package service

import (
	"errors"
	"fmt"
	"sort"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

// DuracionSupervision: cuánto tiempo se abren las consolas de un lab al docente
const DuracionSupervision = 30 * time.Minute

// ErrHayQueDetener: GNS3 solo exporta proyectos detenidos
var ErrHayQueDetener = errors.New("el proyecto tiene nodos encendidos: para copiarlo hay que detenerlo (el alumno pierde lo que no haya guardado)")

// Supervision: el docente (o admin) revisa los labs de sus alumnos desde la página.
// Go habla con el GNS3 del alumno; el docente nunca recibe el token del alumno.
// Todo lo que consulta queda en la bitácora (SUPERVISIONES).
type Supervision struct {
	practicas *repository.PracticaRepository
	accesos   *repository.AccesoRepository
	labs      *repository.LaboratorioRepository
	docker    *Docker
	env       *config.Env
}

func NewSupervision(p *repository.PracticaRepository, a *repository.AccesoRepository, l *repository.LaboratorioRepository, d *Docker, env *config.Env) *Supervision {
	return &Supervision{practicas: p, accesos: a, labs: l, docker: d, env: env}
}

// puedeVerGrupo: admin, o docente del grupo
func (s *Supervision) puedeVerGrupo(idUsuario, idGrupo string) bool {
	return s.practicas.EsAdmin(idUsuario) || s.practicas.EsDocente(idUsuario, idGrupo)
}

// labSupervisado: el lab si el usuario es admin o docente de alguien del lab
func (s *Supervision) labSupervisado(idUsuario, idLab string) (*domain.Laboratorio, error) {
	lab := s.labs.ByID(idLab)
	if lab == nil {
		return nil, repository.ErrSinLaboratorio
	}
	permitido := s.practicas.EsAdmin(idUsuario)
	for _, g := range s.practicas.GruposDelLab(idLab) {
		permitido = permitido || s.practicas.EsDocente(idUsuario, g)
	}
	if !permitido {
		return nil, ErrSinPermiso
	}
	if lab.Estado != "CORRIENDO" {
		return nil, ErrLabNoCorriendo
	}
	return lab, nil
}

func (s *Supervision) Alumnos(idUsuario, idGrupo string) ([]domain.AlumnoLab, error) {
	if !s.puedeVerGrupo(idUsuario, idGrupo) {
		return nil, ErrSinPermiso
	}
	return s.practicas.AlumnosConLab(idGrupo), nil
}

func (s *Supervision) Proyectos(idUsuario, idLab string) ([]domain.ProyectoGNS3, error) {
	lab, err := s.labSupervisado(idUsuario, idLab)
	if err != nil {
		return nil, err
	}
	s.accesos.Bitacora(idUsuario, lab.IDDueno, "PROYECTOS", lab.Contenedor)
	return NewGNS3(s.env, lab).Proyectos()
}

// Nodos: los nodos de un proyecto con su configuración guardada (archivos del
// proyecto) o la actual (running-config por la consola; solo routers encendidos)
func (s *Supervision) Nodos(idUsuario, idLab, proyecto string, actual bool) ([]domain.NodoSupervision, error) {
	lab, err := s.labSupervisado(idUsuario, idLab)
	if err != nil {
		return nil, err
	}
	g := NewGNS3(s.env, lab)
	nodos, err := g.Nodos(proyecto)
	if err != nil {
		return nil, err
	}
	accion, detalle := "CONFIG_GUARDADA", proyecto
	if actual {
		accion = "CONFIG_ACTUAL"
	}
	if lista, err := g.Proyectos(); err == nil { // en la bitácora va el nombre, no el ID
		for _, p := range lista {
			if p.ID == proyecto {
				detalle = p.Nombre
			}
		}
	}
	s.accesos.Bitacora(idUsuario, lab.IDDueno, accion, detalle)

	var r *revisor
	if actual {
		if r, err = nuevoRevisor(g, s.docker, lab.Contenedor, proyecto, ""); err != nil {
			return nil, err
		}
		defer r.cerrar()
	}
	lista := []domain.NodoSupervision{}
	for _, n := range nodos {
		ns := domain.NodoSupervision{Nombre: n.Nombre, Tipo: n.Tipo, Estado: n.Estado, Consola: n.Consola}
		var e error
		if actual && n.Tipo == "dynamips" && n.Estado == "started" {
			ns.Config, e = r.RunningConfig(n.Nombre)
		} else {
			ns.Config, e = g.ConfigGuardada(proyecto, n)
		}
		if e != nil {
			ns.Error = e.Error()
		}
		lista = append(lista, ns)
	}
	sort.Slice(lista, func(i, j int) bool { return lista[i].Nombre < lista[j].Nombre })
	return lista, nil
}

// exportar: la copia del proyecto del alumno (si hay nodos encendidos solo con "detener")
func (s *Supervision) exportar(idUsuario, idLab, proyecto string, detener bool) (*domain.Laboratorio, string, []byte, error) {
	lab, err := s.labSupervisado(idUsuario, idLab)
	if err != nil {
		return nil, "", nil, err
	}
	g := NewGNS3(s.env, lab)
	proyectos, err := g.Proyectos()
	if err != nil {
		return nil, "", nil, err
	}
	var p *domain.ProyectoGNS3
	for i := range proyectos {
		if proyectos[i].ID == proyecto {
			p = &proyectos[i]
		}
	}
	if p == nil {
		return nil, "", nil, invalido("Ese proyecto ya no está en el laboratorio")
	}
	if p.Encendidos > 0 {
		if !detener {
			return nil, "", nil, ErrHayQueDetener
		}
		s.accesos.Bitacora(idUsuario, lab.IDDueno, "DETENER", p.Nombre)
		if err := g.Detener(proyecto); err != nil {
			return nil, "", nil, err
		}
	}
	zip, err := g.Exportar(proyecto)
	if err != nil {
		return nil, "", nil, err
	}
	return lab, p.Nombre, zip, nil
}

func (s *Supervision) Descargar(idUsuario, idLab, proyecto string, detener bool) ([]byte, string, error) {
	lab, nombre, zip, err := s.exportar(idUsuario, idLab, proyecto, detener)
	if err != nil {
		return nil, "", err
	}
	s.accesos.Bitacora(idUsuario, lab.IDDueno, "DESCARGAR", nombre)
	return zip, fmt.Sprintf("%s - %s.gns3project", lab.Usuario, nombre), nil
}

// CopiarAMiLab: importa la copia del proyecto del alumno en el lab del docente
func (s *Supervision) CopiarAMiLab(idUsuario, idLab, proyecto string, detener bool) (string, error) {
	mio, _ := s.labs.ByUser(idUsuario)
	if mio == nil || mio.Estado != "CORRIENDO" {
		return "", invalido("Primero crea tu laboratorio (Mi laboratorio)")
	}
	lab, nombre, zip, err := s.exportar(idUsuario, idLab, proyecto, detener)
	if err != nil {
		return "", err
	}
	g := NewGNS3(s.env, mio)
	copia := fmt.Sprintf("Copia %s - %s", lab.Usuario, nombre)
	for i := 2; ; i++ {
		ya, err := g.ProyectoPorNombre(copia)
		if err != nil {
			return "", err
		}
		if ya == "" {
			break
		}
		copia = fmt.Sprintf("Copia %s - %s (%d)", lab.Usuario, nombre, i)
	}
	if _, err := g.Importar(copia, zip); err != nil {
		return "", err
	}
	s.accesos.Bitacora(idUsuario, lab.IDDueno, "COPIAR", nombre)
	return copia, nil
}

// Supervisar: abre en el firewall las consolas del lab para la IP del docente
// (30 min, se renueva con otra llamada). Sigue sin recibir el token de GNS3.
func (s *Supervision) Supervisar(idUsuario, idLab string) (int, time.Time, error) {
	lab, err := s.labSupervisado(idUsuario, idLab)
	if err != nil {
		return 0, time.Time{}, err
	}
	s.accesos.Supervisar(idUsuario, idLab, DuracionSupervision)
	s.accesos.Bitacora(idUsuario, lab.IDDueno, "CONSOLAS", lab.Contenedor)
	return lab.PuertoBase, time.Now().Add(DuracionSupervision), nil
}

func (s *Supervision) DejarDeSupervisar(idUsuario, idLab string) {
	s.accesos.DejarDeSupervisar(idUsuario, idLab)
}

// Bitacora: el admin ve todo; el docente solo lo suyo
func (s *Supervision) Bitacora(idUsuario string) []domain.EntradaBitacora {
	if s.practicas.EsAdmin(idUsuario) {
		return s.accesos.Bitacoras("", 300)
	}
	return s.accesos.Bitacoras(idUsuario, 300)
}
