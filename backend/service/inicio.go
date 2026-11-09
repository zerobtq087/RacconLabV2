package service

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"syscall"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

// Inicio arma el tablero de la página de Inicio según el rol con el que se
// entró (el de la sesión, no todos los que tiene el usuario).
type Inicio struct {
	repo      *repository.InicioRepository
	labs      *repository.LaboratorioRepository
	practicas *Practicas
	pracRepo  *repository.PracticaRepository
	accesos   *repository.AccesoRepository
	docker    *Docker
	env       *config.Env
}

func NewInicio(r *repository.InicioRepository, l *repository.LaboratorioRepository, p *Practicas,
	pr *repository.PracticaRepository, a *repository.AccesoRepository, d *Docker, env *config.Env) *Inicio {
	return &Inicio{repo: r, labs: l, practicas: p, pracRepo: pr, accesos: a, docker: d, env: env}
}

func (s *Inicio) Tablero(idUsuario, rol string) *domain.Inicio {
	t := &domain.Inicio{Rol: rol}
	switch rol {
	case "ALUMNO":
		t.Alumno = s.alumno(idUsuario)
	case "PROFESOR":
		t.Profesor = s.profesor(idUsuario)
	case "ADMIN":
		t.Admin = s.admin()
	}
	return t
}

// alumno: su lab, lo pendiente, sus últimas 5 calificaciones y sus invitaciones
func (s *Inicio) alumno(idUsuario string) *domain.InicioAlumno {
	a := &domain.InicioAlumno{Pendientes: []domain.MiPractica{}, Calificaciones: []domain.MiPractica{}}
	a.Lab, a.SoyDueno = s.labs.ByUser(idUsuario)
	if a.Lab == nil {
		a.Lab = &domain.Laboratorio{Estado: "NINGUNO"}
	}
	for _, p := range s.practicas.MisPracticas(idUsuario) {
		p.Instrucciones = "" // en Inicio solo el resumen
		switch {
		case p.Entregada != nil:
			a.Calificaciones = append(a.Calificaciones, p)
		case p.Estado == "ACTIVA":
			a.Pendientes = append(a.Pendientes, p)
		}
	}
	sort.Slice(a.Calificaciones, func(i, j int) bool { return a.Calificaciones[i].Entregada.After(*a.Calificaciones[j].Entregada) })
	if len(a.Calificaciones) > 5 {
		a.Calificaciones = a.Calificaciones[:5]
	}
	a.Invitaciones = s.labs.Invitations(idUsuario)
	return a
}

// profesor: SUS prácticas activas (entregas/total) y sus grupos
func (s *Inicio) profesor(idUsuario string) *domain.InicioProfesor {
	p := &domain.InicioProfesor{Practicas: []domain.ResumenPractica{}}
	for _, x := range s.pracRepo.Listar(idUsuario, false) {
		if x.Estado == "ACTIVA" {
			p.Practicas = append(p.Practicas, domain.ResumenPractica{
				ID: x.ID, Nombre: x.Nombre, Grupo: x.Grupo, Entregadas: x.Entregadas, Alumnos: x.Alumnos})
		}
	}
	p.Grupos = s.repo.GruposDelDocente(idUsuario)
	for _, g := range p.Grupos {
		p.Alumnos += g.Alumnos
		p.LabsEncendidos += g.LabsEncendidos
	}
	return p
}

func (s *Inicio) admin() *domain.InicioAdmin {
	a := &domain.InicioAdmin{LabsMax: s.env.LabsMax}
	a.LabsActivos, a.LabsCreando = s.repo.Labs()
	a.Servidor = recursos(s.env.GNS3Volumes)
	a.Usuarios = s.repo.UsuariosPorTipo()
	a.UltimaImportacion = s.repo.UltimaImportacion()
	a.PracticasActivas = s.repo.PracticasActivas()
	a.Servicios = s.servicios()
	return a
}

// servicios: lo que la API puede comprobar desde dentro
func (s *Inicio) servicios() []domain.EstadoServicio {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	lista := []domain.EstadoServicio{
		{Nombre: "API", Bien: true, Detalle: "respondió " + s.env.AppID},
		{Nombre: "Firebird", Bien: s.repo.FirebirdResponde(), Detalle: "base de datos"},
		// si esta petición llegó aquí, el middleware ya validó la sesión en Redis
		{Nombre: "Redis", Bien: true, Detalle: "sesiones (tu sesión se validó ahí)"},
	}
	docker := s.docker.Ping(ctx)
	lista = append(lista, domain.EstadoServicio{Nombre: "Docker", Bien: docker, Detalle: "contenedores de los labs"})
	if docker {
		lista = append(lista, domain.EstadoServicio{Nombre: "Imagen de los labs",
			Bien: s.docker.ImagenExiste(ctx, s.env.GNS3Image), Detalle: s.env.GNS3Image})
	}
	agente := domain.EstadoServicio{Nombre: "Firewall (raccoon-accesos)", Detalle: "nunca ha preguntado"}
	if visto := s.accesos.UltimoLatido("raccoon-accesos"); visto != nil {
		hace := time.Since(*visto).Round(time.Second)
		agente.Bien = hace < 30*time.Second
		agente.Detalle = fmt.Sprintf("última consulta hace %s", hace)
	}
	return append(lista, agente)
}

// recursos: CPU, RAM y disco del servidor. /proc/stat y /proc/meminfo dentro
// del contenedor son los del servidor; el disco es el de GNS3_VOLUMES (los labs).
func recursos(ruta string) domain.RecursosServidor {
	r := domain.RecursosServidor{Nucleos: runtime.NumCPU(), DiscoRuta: ruta}

	ocupado1, total1, err1 := cpuStat()
	time.Sleep(250 * time.Millisecond)
	ocupado2, total2, err2 := cpuStat()
	if err1 == nil && err2 == nil && total2 > total1 {
		r.CPU = float64(int(1000*float64(ocupado2-ocupado1)/float64(total2-total1))) / 10
	}

	if datos, err := os.ReadFile("/proc/meminfo"); err == nil {
		campos := map[string]uint64{}
		for _, l := range strings.Split(string(datos), "\n") {
			if f := strings.Fields(l); len(f) >= 2 {
				n, _ := strconv.ParseUint(f[1], 10, 64)
				campos[strings.TrimSuffix(f[0], ":")] = n * 1024 // viene en kB
			}
		}
		r.RAMTotal = campos["MemTotal"]
		r.RAMUsada = campos["MemTotal"] - campos["MemAvailable"]
	}

	var fs syscall.Statfs_t
	if err := syscall.Statfs(ruta, &fs); err == nil {
		r.DiscoTotal = fs.Blocks * uint64(fs.Bsize)
		r.DiscoUsado = (fs.Blocks - fs.Bfree) * uint64(fs.Bsize)
	} else {
		r.Error = "no se pudo leer el disco de " + ruta
	}
	return r
}

// cpuStat: tiempo ocupado y total de la CPU (primera línea de /proc/stat)
func cpuStat() (ocupado, total uint64, err error) {
	datos, err := os.ReadFile("/proc/stat")
	if err != nil {
		return 0, 0, err
	}
	linea, _, _ := strings.Cut(string(datos), "\n")
	f := strings.Fields(linea) // cpu user nice system idle iowait irq softirq steal
	for i, v := range f[1:] {
		n, _ := strconv.ParseUint(v, 10, 64)
		total += n
		if i != 3 && i != 4 { // idle e iowait no cuentan como ocupado
			ocupado += n
		}
	}
	return ocupado, total, nil
}
