package service

import (
	"errors"
	"fmt"
	"log"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

var (
	ErrSinPermiso       = errors.New("no tienes permiso para esta práctica")
	ErrYaEntregaste     = errors.New("ya entregaste esta práctica")
	ErrPracticaNoActiva = errors.New("la práctica no está activa")
	ErrConEntregas      = errors.New("la práctica ya tiene entregas: los criterios y el grupo ya no se pueden cambiar")
	ErrAbrirPrimero     = errors.New("primero abre la práctica en tu laboratorio")
)

// ErrOtrosAbiertos: el alumno tiene otros proyectos con nodos encendidos
type ErrOtrosAbiertos struct{ Proyectos []string }

func (e *ErrOtrosAbiertos) Error() string {
	return "Tienes otra topología encendida en tu laboratorio (" + strings.Join(e.Proyectos, ", ") +
		"). Para abrir la práctica hay que detenerla y cerrarla."
}

// Practicas: prácticas con calificación. Lo pesado (exportar y revisar) va en
// segundo plano y su avance queda en Firebird (hay 2 réplicas de la API).
type Practicas struct {
	repository *repository.PracticaRepository
	labs       *repository.LaboratorioRepository
	docker     *Docker
	env        *config.Env
}

func NewPracticas(r *repository.PracticaRepository, labs *repository.LaboratorioRepository, d *Docker, env *config.Env) *Practicas {
	s := &Practicas{repository: r, labs: labs, docker: d, env: env}
	go s.atenderCola()
	return s
}

// nombreProyecto: así se llama la práctica en el GNS3 del alumno
func nombreProyecto(p *domain.Practica) string { return "Práctica - " + p.Nombre }

// labCorriendo: el lab del usuario (dueño o miembro) en ejecución
func (s *Practicas) labCorriendo(idUsuario string) (*domain.Laboratorio, error) {
	lab, _ := s.labs.ByUser(idUsuario)
	if lab == nil {
		return nil, repository.ErrSinLaboratorio
	}
	if lab.Estado != "CORRIENDO" {
		return nil, ErrLabNoCorriendo
	}
	return lab, nil
}

// practicaDe: la práctica si el usuario la puede administrar (su profesor o un admin)
func (s *Practicas) practicaDe(idUsuario, id string) (*domain.Practica, error) {
	p := s.repository.PorID(id)
	if p == nil {
		return nil, repository.ErrPracticaNoEsta
	}
	if p.IDProfesor != idUsuario && !s.repository.EsAdmin(idUsuario) {
		return nil, ErrSinPermiso
	}
	return p, nil
}

// =====================================================================
// Profesor
// =====================================================================

func (s *Practicas) Listar(idUsuario string) []domain.Practica {
	return s.repository.Listar(idUsuario, s.repository.EsAdmin(idUsuario))
}

func (s *Practicas) Detalle(idUsuario, id string) (*domain.Practica, error) {
	return s.practicaDe(idUsuario, id)
}

// ProyectosDeMiLab: los proyectos que el profesor puede convertir en práctica
func (s *Practicas) ProyectosDeMiLab(idUsuario string) ([]domain.ProyectoGNS3, error) {
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return nil, err
	}
	return NewGNS3(s.env, lab).Proyectos()
}

func (s *Practicas) validarSolicitud(idUsuario string, sol *domain.SolicitudPractica) error {
	sol.Nombre = strings.Join(strings.Fields(sol.Nombre), " ")
	sol.Instrucciones = strings.TrimSpace(sol.Instrucciones)
	if n := utf8.RuneCountInString(sol.Nombre); n < 3 || n > 80 {
		return invalido("El nombre debe tener de 3 a 80 caracteres")
	}
	if strings.ContainsAny(sol.Nombre, `/\"`) {
		return invalido("El nombre no puede llevar / \\ ni comillas")
	}
	if sol.IDGrupo == "" {
		return invalido("Elige el grupo")
	}
	if !s.repository.EsAdmin(idUsuario) && !s.repository.EsDocente(idUsuario, sol.IDGrupo) {
		return invalido("Solo puedes crear prácticas para tus grupos")
	}
	return nil
}

// Crear: registra la práctica y exporta el proyecto del profesor en segundo plano
func (s *Practicas) Crear(idUsuario string, sol *domain.SolicitudPractica) (string, error) {
	if err := s.validarSolicitud(idUsuario, sol); err != nil {
		return "", err
	}
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return "", err
	}
	g := NewGNS3(s.env, lab)
	proyectos, err := g.Proyectos()
	if err != nil {
		return "", err
	}
	var origen *domain.ProyectoGNS3
	for i := range proyectos {
		if proyectos[i].ID == sol.Proyecto {
			origen = &proyectos[i]
		}
	}
	if origen == nil {
		return "", invalido("Elige uno de los proyectos de tu laboratorio")
	}

	id, err := s.repository.Crear(idUsuario, sol, origen.Nombre)
	if err != nil {
		return "", err
	}
	go s.exportar(id, g, origen.ID)
	return id, nil
}

// exportar: guarda la configuración de los routers, detiene el proyecto y lo exporta
func (s *Practicas) exportar(id string, g *GNS3, proyecto string) {
	falla := func(msg string, err error) {
		log.Printf("[PRACTICAS] %s: %s: %v", id, msg, err)
		s.repository.FallaExportar(id, fmt.Sprintf("%s: %v", msg, err))
	}
	if err := g.Abrir(proyecto); err != nil {
		falla("No se pudo abrir tu proyecto", err)
		return
	}

	// write memory en cada router encendido: así su configuración entra en la exportación.
	// Todavía no hay criterios (ni sus contraseñas): si un router pide contraseña de
	// enable se usa la configuración que el profesor ya guardó y queda en el log.
	s.repository.Paso(id, "Guardando la configuración de los routers")
	if r, err := nuevoRevisor(g, s.docker, "", proyecto, nil); err == nil {
		for _, router := range r.Routers() {
			if err := r.GuardarConfig(router); err != nil {
				log.Printf("[PRACTICAS] %s: no se guardó la config de %s: %v", id, router, err)
			}
		}
		r.cerrar()
	}

	s.repository.Paso(id, "Deteniendo el proyecto")
	if err := g.Detener(proyecto); err != nil {
		falla("No se pudo detener tu proyecto", err)
		return
	}

	s.repository.Paso(id, "Exportando el proyecto")
	zip, err := g.Exportar(proyecto)
	if err != nil {
		falla("No se pudo exportar tu proyecto", err)
		return
	}
	if max := s.env.PracticaMaxMB; max > 0 && len(zip) > max<<20 {
		falla("El proyecto es muy grande", fmt.Errorf("%d MB (máximo %d MB)", len(zip)>>20, max))
		return
	}

	nodosGNS3, err := g.Nodos(proyecto)
	if err != nil {
		falla("No se pudieron leer los nodos", err)
		return
	}
	nodos := []domain.NodoPractica{}
	routers, hosts := 0, 0
	for _, n := range nodosGNS3 {
		np := domain.NodoPractica{Nombre: n.Nombre, Tipo: n.Tipo, Puertos: []string{}}
		for _, p := range n.Puertos {
			np.Puertos = append(np.Puertos, p.Nombre)
		}
		nodos = append(nodos, np)
		switch n.Tipo {
		case "dynamips", "qemu":
			routers++
		case "docker", "vpcs":
			hosts++
		}
	}
	if max := s.env.PracticaMaxRouters; max > 0 && routers > max {
		falla("Demasiados routers", fmt.Errorf("%d (máximo %d)", routers, max))
		return
	}
	if max := s.env.PracticaMaxHosts; max > 0 && hosts > max {
		falla("Demasiados hosts", fmt.Errorf("%d (máximo %d)", hosts, max))
		return
	}
	if err := s.repository.Exportada(id, zip, nodos); err != nil {
		falla("No se pudo guardar el proyecto", err)
		return
	}
	log.Printf("[PRACTICAS] %s exportada (%d KB, %d nodos)", id, len(zip)>>10, len(nodos))
}

func (s *Practicas) Editar(idUsuario, id string, sol *domain.SolicitudPractica) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if err := s.validarSolicitud(idUsuario, sol); err != nil {
		return err
	}
	if p.TieneEntregas && sol.IDGrupo != p.IDGrupo {
		return ErrConEntregas
	}
	return s.repository.Actualizar(id, sol)
}

// GuardarCriterios valida y reemplaza los criterios (bloqueados si ya hay entregas)
func (s *Practicas) GuardarCriterios(idUsuario, id string, criterios []domain.Criterio) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if p.TieneEntregas {
		return ErrConEntregas
	}
	if p.Estado == "EXPORTANDO" || p.Estado == "ERROR" {
		return invalido("La práctica todavía no tiene su proyecto listo")
	}
	nodos := map[string]domain.NodoPractica{}
	for _, n := range p.Nodos {
		nodos[n.Nombre] = n
	}
	if len(criterios) > 100 {
		return invalido("Máximo 100 criterios")
	}
	for i := range criterios {
		criterios[i].Orden = i + 1
		if err := ValidarCriterio(&criterios[i], nodos); err != nil {
			return err
		}
	}
	return s.repository.GuardarCriterios(id, criterios)
}

func (s *Practicas) Activar(idUsuario, id string) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if p.Estado != "BORRADOR" {
		return invalido("Solo se activa una práctica en borrador")
	}
	if len(p.Criterios) == 0 {
		return invalido("Agrega al menos un criterio antes de activarla")
	}
	return s.repository.Activar(id)
}

func (s *Practicas) Cerrar(idUsuario, id string) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if p.Estado != "ACTIVA" {
		return invalido("Solo se cierra una práctica activa")
	}
	return s.repository.Cerrar(id)
}

func (s *Practicas) Borrar(idUsuario, id string) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if p.TieneEntregas {
		return invalido("La práctica ya tiene entregas: no se puede borrar")
	}
	return s.repository.Borrar(id)
}

// Probar: revisa los criterios contra el proyecto del profesor (su solución)
func (s *Practicas) Probar(idUsuario, id string) error {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return err
	}
	if len(p.Criterios) == 0 {
		return invalido("Agrega al menos un criterio")
	}
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return err
	}
	revID, err := s.repository.ReservarRevision(id, idUsuario, lab.ID, true, false)
	if err != nil {
		return err
	}
	go s.ejecutarRevision(revID)
	return nil
}

// Prueba: la última prueba del profesor, con el detalle de cada criterio
func (s *Practicas) Prueba(idUsuario, id string) (*domain.Prueba, error) {
	if _, err := s.practicaDe(idUsuario, id); err != nil {
		return nil, err
	}
	pr := &domain.Prueba{Resultados: []domain.Resultado{}}
	if pr.Revision = s.repository.UltimaRevision(id, idUsuario, true); pr.Revision != nil {
		pr.Resultados = s.resultados(id, pr.Revision.ID, true)
	}
	return pr, nil
}

// resultados con la descripción de cada criterio (para el docente) o el mensaje (alumno)
func (s *Practicas) resultados(idPractica, idRevision string, docente bool) []domain.Resultado {
	criterios := map[string]domain.Criterio{}
	for _, c := range s.repository.Criterios(idPractica) {
		criterios[c.ID] = c
	}
	res := s.repository.Resultados(idRevision)
	for i := range res {
		c := criterios[res[i].IDCriterio]
		res[i].Mensaje = MensajeAlumno(&c)
		if docente {
			res[i].Descripcion = Descripcion(&c)
		} else {
			res[i].Obtenido = ""
		}
	}
	return res
}

// marcar: a tiempo, tarde (con el retraso) o pendiente
func marcar(f *domain.FilaEntrega, cerrada *time.Time) {
	switch {
	case f.Entregada == nil:
		f.Estado = "PENDIENTE"
	case cerrada != nil && f.Entregada.After(*cerrada):
		f.Estado, f.Retraso = "TARDE", FormatoRetraso(f.Entregada.Sub(*cerrada))
	default:
		f.Estado = "A_TIEMPO"
	}
}

func (s *Practicas) Entregas(idUsuario, id string) ([]domain.FilaEntrega, error) {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return nil, err
	}
	filas := s.repository.TablaGrupo(id)
	for i := range filas {
		marcar(&filas[i], p.Cerrada)
	}
	return filas, nil
}

// Excel: matrícula, nombre, calificación, a tiempo o tarde, retraso y fecha de entrega
func (s *Practicas) Excel(idUsuario, id string) ([]byte, string, error) {
	filas, err := s.Entregas(idUsuario, id)
	if err != nil {
		return nil, "", err
	}
	p := s.repository.PorID(id)
	estado := map[string]string{"A_TIEMPO": "A tiempo", "TARDE": "Tarde", "PENDIENTE": "Pendiente"}
	tabla := [][]string{{"Matrícula", "Nombre", "Calificación", "Estado", "Retraso", "Fecha de entrega"}}
	for _, f := range filas {
		cal, fecha := "", ""
		if f.Calificacion != nil {
			cal = fmt.Sprint(*f.Calificacion)
		}
		if f.Entregada != nil {
			fecha = f.Entregada.Format("2006-01-02 15:04")
		}
		tabla = append(tabla, []string{f.Matricula, f.Nombre, cal, estado[f.Estado], f.Retraso, fecha})
	}
	datos, err := escribirXLSX("calificaciones", tabla, map[int]bool{2: true})
	nombre := "calificaciones_" + strings.ReplaceAll(strings.ToLower(p.Nombre), " ", "_") + ".xlsx"
	return datos, nombre, err
}

// filaDe: la entrega de un alumno (si no entregó, error)
func (s *Practicas) filaDe(idUsuario, id, idAlumno string) (*domain.Practica, *domain.FilaEntrega, error) {
	p, err := s.practicaDe(idUsuario, id)
	if err != nil {
		return nil, nil, err
	}
	for _, f := range s.repository.TablaGrupo(id) {
		if f.IDUsuario == idAlumno && f.IDRevision != "" {
			marcar(&f, p.Cerrada)
			return p, &f, nil
		}
	}
	return nil, nil, invalido("Ese alumno no ha entregado")
}

func (s *Practicas) DetalleEntrega(idUsuario, id, idAlumno string) (*domain.DetalleEntrega, error) {
	_, f, err := s.filaDe(idUsuario, id, idAlumno)
	if err != nil {
		return nil, err
	}
	return &domain.DetalleEntrega{
		Fila:        *f,
		Integrantes: s.repository.Integrantes(f.IDRevision),
		Resultados:  s.resultados(id, f.IDRevision, true),
		Configs:     s.repository.Configs(f.IDRevision),
		ConProyecto: s.repository.TieneProyecto(f.IDRevision),
	}, nil
}

// ProyectoEntrega: la copia del proyecto que se guardó al entregar
func (s *Practicas) ProyectoEntrega(idUsuario, id, idAlumno string) ([]byte, string, error) {
	p, f, err := s.filaDe(idUsuario, id, idAlumno)
	if err != nil {
		return nil, "", err
	}
	zip := s.repository.ProyectoRevision(f.IDRevision)
	if len(zip) == 0 {
		return nil, "", invalido("Esa entrega no tiene copia del proyecto")
	}
	return zip, fmt.Sprintf("%s - %s.gns3project", p.Nombre, f.Matricula), nil
}

// AbrirEntregaEnMiLab: importa la copia de la entrega en el lab del docente
func (s *Practicas) AbrirEntregaEnMiLab(idUsuario, id, idAlumno string) (string, error) {
	zip, _, err := s.ProyectoEntrega(idUsuario, id, idAlumno)
	if err != nil {
		return "", err
	}
	p, f, _ := s.filaDe(idUsuario, id, idAlumno)
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return "", err
	}
	g := NewGNS3(s.env, lab)
	nombre := fmt.Sprintf("Entrega %s - %s", f.Matricula, p.Nombre)
	if ya, _ := g.ProyectoPorNombre(nombre); ya != "" {
		return "", invalido("Ya tienes \"%s\" en tu laboratorio; ábrelo desde GNS3 (File → Open project) o bórralo", nombre)
	}
	if _, err := g.Importar(nombre, zip); err != nil {
		return "", err
	}
	return nombre, nil
}

// =====================================================================
// Alumno
// =====================================================================

// MisPracticas: después de entregar, el alumno solo ve su resultado
func (s *Practicas) MisPracticas(idUsuario string) []domain.MiPractica {
	lista := s.repository.MisPracticas(idUsuario)
	lab, _ := s.labs.ByUser(idUsuario)
	for i := range lista {
		m := &lista[i]
		m.Fallidos = []string{}
		if m.IDRevision != "" {
			m.Instrucciones = "" // ya entregó: no vuelve a ver la práctica
			if m.Cerrada != nil && m.Entregada != nil && m.Entregada.After(*m.Cerrada) {
				m.Tarde, m.Retraso = true, FormatoRetraso(m.Entregada.Sub(*m.Cerrada))
			}
			for _, r := range s.resultados(m.ID, m.IDRevision, false) {
				if !r.Cumple {
					m.Fallidos = append(m.Fallidos, r.Mensaje)
				}
			}
			continue
		}
		if lab != nil {
			if rev := s.repository.RevisionEnCurso(lab.ID, m.ID); rev != nil {
				m.Revision = rev
			} else if rev := s.repository.UltimaRevision(m.ID, idUsuario, false); rev != nil && rev.Estado == "ERROR" {
				m.Revision = rev // la última falló (no contó como entrega): se muestra el error
			}
		}
	}
	return lista
}

// paraAlumno: la práctica si el alumno la puede abrir o entregar
func (s *Practicas) paraAlumno(idUsuario, id string) (*domain.Practica, error) {
	p := s.repository.PorID(id)
	if p == nil || s.repository.GrupoDe(idUsuario) != p.IDGrupo {
		return nil, repository.ErrPracticaNoEsta
	}
	if p.Estado != "ACTIVA" && p.Estado != "CERRADA" {
		return nil, ErrPracticaNoActiva
	}
	if s.repository.YaEntrego(id, idUsuario) {
		return nil, ErrYaEntregaste
	}
	return p, nil
}

// Abrir: importa una copia independiente de la práctica en el lab del alumno
// (si ya estaba, solo la abre y conserva su trabajo)
func (s *Practicas) Abrir(idUsuario, id string, sol *domain.SolicitudAbrir) (string, error) {
	p, err := s.paraAlumno(idUsuario, id)
	if err != nil {
		return "", err
	}
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return "", err
	}
	if s.repository.RevisandoEnLab(lab.ID) {
		return "", repository.ErrRevisionEnCurso // no se toca el proyecto mientras se revisa
	}
	g := NewGNS3(s.env, lab)
	nombre := nombreProyecto(p)
	proyectos, err := g.Proyectos()
	if err != nil {
		return "", err
	}

	// ¿Otra topología encendida? Se avisa antes de abrir (no se toca sin permiso)
	var otros []string
	for _, x := range proyectos {
		if x.Nombre != nombre && x.Encendidos > 0 {
			otros = append(otros, x.Nombre)
		}
	}
	if len(otros) > 0 && !sol.DetenerOtros {
		return "", &ErrOtrosAbiertos{Proyectos: otros}
	}
	for _, x := range proyectos {
		if x.Nombre != nombre && x.Encendidos > 0 {
			_ = g.Detener(x.ID)
			_ = g.Cerrar(x.ID)
		}
	}

	pid := ""
	for _, x := range proyectos {
		if x.Nombre == nombre {
			pid = x.ID
		}
	}
	if pid != "" && sol.EmpezarDeNuevo {
		_ = g.Borrar(pid)
		pid = ""
	}
	if pid == "" {
		zip := s.repository.ProyectoBase(id)
		if len(zip) == 0 {
			return "", invalido("La práctica no tiene su proyecto")
		}
		if pid, err = g.Importar(nombre, zip); err != nil {
			return "", err
		}
	}
	if err := g.Abrir(pid); err != nil {
		return "", err
	}
	if err := g.Encender(pid); err != nil {
		return "", err
	}
	return nombre, nil
}

// Entregar: una sola vez. Se revisa en segundo plano (o espera turno en la cola).
func (s *Practicas) Entregar(idUsuario, id string) error {
	p, err := s.paraAlumno(idUsuario, id)
	if err != nil {
		return err
	}
	if len(p.Criterios) == 0 {
		return invalido("La práctica no tiene criterios")
	}
	lab, err := s.labCorriendo(idUsuario)
	if err != nil {
		return err
	}
	if pid, err := NewGNS3(s.env, lab).ProyectoPorNombre(nombreProyecto(p)); err != nil {
		return err
	} else if pid == "" {
		return ErrAbrirPrimero
	}
	enCola := s.env.RevisionesSimultaneas > 0 && s.repository.Revisando() >= s.env.RevisionesSimultaneas
	revID, err := s.repository.ReservarRevision(id, idUsuario, lab.ID, false, enCola)
	if err != nil {
		return err
	}
	if !enCola {
		go s.ejecutarRevision(revID)
	}
	return nil
}

// atenderCola: cada réplica toma revisiones en cola cuando hay lugar
func (s *Practicas) atenderCola() {
	for range time.Tick(3 * time.Second) {
		for {
			limite := s.env.RevisionesSimultaneas
			if limite > 0 && s.repository.Revisando() >= limite {
				break
			}
			id := s.repository.TomarDeCola()
			if id == "" {
				break
			}
			go s.ejecutarRevision(id)
		}
	}
}

// ejecutarRevision: revisa los criterios en el lab y, si es entrega, guarda las
// configuraciones y una copia del proyecto, registra la entrega y quita la
// práctica del lab del alumno. Si algo falla, NO cuenta como entrega.
func (s *Practicas) ejecutarRevision(idRevision string) {
	rev := s.repository.Revision(idRevision)
	if rev == nil {
		return
	}
	p := s.repository.PorID(rev.IDPractica)
	lab := s.labs.ByID(rev.Lab)
	falla := func(msg string) {
		log.Printf("[PRACTICAS] revisión %s: %s", idRevision, msg)
		s.repository.TerminarRevision(idRevision, "ERROR", nil, msg)
	}
	if p == nil || lab == nil {
		falla("La práctica o el laboratorio ya no existen")
		return
	}

	listo := make(chan struct{})
	defer close(listo)
	go func() { // latido: que la otra réplica no la dé por muerta
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.repository.Latido(idRevision)
			case <-listo:
				return
			}
		}
	}()

	g := NewGNS3(s.env, lab)
	nombre := nombreProyecto(p)
	if rev.EsPrueba {
		nombre = p.ProyectoOrigen
	}
	s.repository.Avance(idRevision, 3, "Abriendo el proyecto")
	pid, err := g.ProyectoPorNombre(nombre)
	if err != nil || pid == "" {
		falla(fmt.Sprintf("No se encontró el proyecto \"%s\" en el laboratorio", nombre))
		return
	}
	_ = g.Abrir(pid)

	// Si algo estaba apagado se enciende (los criterios lo necesitan) y se espera
	// a que los routers arranquen y converja el enrutamiento
	apagados := 0
	if nodos, err := g.Nodos(pid); err == nil {
		for _, n := range nodos {
			if n.Estado != "started" {
				apagados++
			}
		}
	}
	if apagados > 0 {
		s.repository.Avance(idRevision, 4, "Encendiendo la topología")
		_ = g.Encender(pid)
	}

	r, err := nuevoRevisor(g, s.docker, lab.Contenedor, pid, p.Criterios)
	if err != nil {
		falla("No se pudo leer el proyecto: " + err.Error())
		return
	}
	defer r.cerrar()
	if apagados > 0 {
		s.repository.Avance(idRevision, 4, "Esperando a que arranquen los routers")
		for _, router := range r.Routers() {
			_, _ = r.consola(router) // espera al prompt (hasta 3 min por router)
		}
		time.Sleep(45 * time.Second) // OSPF/RIP necesitan unos segundos para converger
	}

	// 1) Criterios (el paso NO dice qué se revisa, para no dar la respuesta)
	resultados := []domain.Resultado{}
	for i, c := range p.Criterios {
		s.repository.Avance(idRevision, 5+i*70/len(p.Criterios), fmt.Sprintf("Revisando criterio %d de %d", i+1, len(p.Criterios)))
		cumple, obtenido := r.Evaluar(&c)
		res := domain.Resultado{IDCriterio: c.ID, Peso: c.Peso, Cumple: cumple, Obtenido: obtenido}
		if cumple {
			res.Puntos = c.Peso
		}
		resultados = append(resultados, res)
	}
	s.repository.GuardarResultados(idRevision, resultados)
	calificacion := Calificar(resultados)

	if !rev.EsPrueba {
		// 2) Configuración de cada router en este momento (la ve el docente)
		s.repository.Avance(idRevision, 78, "Guardando la configuración de tus routers")
		routers := r.Routers()
		sort.Strings(routers)
		configs := []domain.ConfigNodo{}
		for _, router := range routers {
			// Con enable: running-config y write memory (la copia del proyecto la incluye).
			// Sin enable (el alumno puso otra contraseña): lo que dan los show, con un aviso.
			config, completa := r.ConfigParaDocente(router)
			if completa {
				_ = r.GuardarConfig(router)
			}
			configs = append(configs, domain.ConfigNodo{Nodo: router, Config: config})
		}
		s.repository.GuardarConfigs(idRevision, configs)
		r.cerrar()

		// 3) Copia del proyecto (GNS3 solo exporta proyectos detenidos)
		s.repository.Avance(idRevision, 86, "Guardando una copia de tu proyecto")
		if err := g.Detener(pid); err != nil {
			falla("No se pudo detener el proyecto: " + err.Error())
			return
		}
		zip, err := g.Exportar(pid)
		if err != nil {
			falla("No se pudo copiar el proyecto: " + err.Error())
			return
		}
		if err := s.repository.GuardarProyectoRevision(idRevision, zip); err != nil {
			falla("No se pudo guardar la copia del proyecto")
			return
		}

		// 4) Cuenta para quien entregó y su equipo de ESTE momento (los que aún no entregaban)
		s.repository.Avance(idRevision, 95, "Registrando la entrega")
		var alumnos []string
		for _, u := range append([]domain.Persona{{ID: lab.IDDueno}}, s.labs.Miembros(lab.ID)...) {
			if s.repository.GrupoDe(u.ID) == p.IDGrupo && !s.repository.YaEntrego(p.ID, u.ID) {
				alumnos = append(alumnos, u.ID)
			}
		}
		if s.repository.RegistrarEntregas(p.ID, idRevision, alumnos) == 0 {
			falla("Tu equipo ya había entregado esta práctica")
			return
		}

		// 5) La práctica sale del lab: después de entregar ya no se ve ni se modifica
		if err := g.Borrar(pid); err != nil {
			log.Printf("[PRACTICAS] no se pudo quitar %s del lab %s: %v", nombre, lab.Contenedor, err)
		}
	}

	s.repository.TerminarRevision(idRevision, "CALIFICADA", &calificacion, "")
	log.Printf("[PRACTICAS] revisión %s: %d/100 (prueba=%v)", idRevision, calificacion, rev.EsPrueba)
}
