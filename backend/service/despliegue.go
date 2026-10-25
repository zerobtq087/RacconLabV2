package service

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

var (
	ErrLabNoCorriendo = errors.New("tu laboratorio no está en ejecución: créalo o espera a que termine de arrancar")
	ErrProyectoExiste = errors.New("ya existe un proyecto con ese nombre en tu laboratorio; usa otro nombre")
)

// Rutas DENTRO del contenedor del laboratorio (ansible está montado en /root/ansible)
const (
	playbookTopologia = "/root/ansible/topologies/deploy_topology.yml"
	carpetaTemporal   = "/tmp"
)

// Despliegues monta topologías en el GNS3 del laboratorio con Ansible, en segundo plano.
// El avance queda en Firebird (tabla DESPLIEGUES) porque hay 2 réplicas de la API.
type Despliegues struct {
	repository *repository.DespliegueRepository
	labs       *repository.LaboratorioRepository
	docker     *Docker
	env        *config.Env
}

func NewDespliegues(r *repository.DespliegueRepository, labs *repository.LaboratorioRepository, d *Docker, env *config.Env) *Despliegues {
	return &Despliegues{repository: r, labs: labs, docker: d, env: env}
}

// dirTopologias: donde están los JSON de las plantillas (la API ve GNS3_VOLUMES en la misma ruta)
func (s *Despliegues) dirTopologias() string {
	return filepath.Join(s.env.GNS3Volumes, "ansible", "topologies")
}

// DesplegarPlantilla: una de las 3 plantillas del profesor
func (s *Despliegues) DesplegarPlantilla(idUsuario string, sol *domain.SolicitudDespliegue) error {
	archivo, ok := Plantillas[sol.Plantilla]
	if !ok {
		return invalido("La plantilla \"%s\" no existe (basica, intermedia o avanzada)", sol.Plantilla)
	}
	topo, err := CargarTopologia(s.dirTopologias(), archivo)
	if err != nil {
		return err
	}
	return s.DesplegarTopologia(idUsuario, sol.Plantilla, topo, sol)
}

// DesplegarTopologia valida, reserva y lanza el despliegue de CUALQUIER topología.
// Es el camino común: las plantillas lo usan hoy y la IA podrá mandar la suya
// (origen "ia") con sus parámetros.
func (s *Despliegues) DesplegarTopologia(idUsuario, origen string, topo *domain.Topologia, sol *domain.SolicitudDespliegue) error {
	if err := ValidarSolicitud(sol); err != nil {
		return err
	}

	// El contenedor y el puerto los saca Go del laboratorio del usuario (nunca del frontend)
	lab, dueno := s.labs.ByUser(idUsuario)
	if lab == nil {
		return repository.ErrSinLaboratorio
	}
	if !dueno {
		return repository.ErrNoEsDueno
	}
	if lab.Estado != "CORRIENDO" {
		return ErrLabNoCorriendo
	}

	if err := ResolverTopologia(topo, sol); err != nil {
		return err
	}
	if s.proyectoExiste(lab, sol.NombreLab) {
		return ErrProyectoExiste
	}

	id, err := s.repository.Reserve(lab.ID, idUsuario, origen, sol.NombreLab)
	if err != nil {
		return err
	}
	go s.ejecutar(id, lab, topo)
	return nil
}

// Estado: el último despliegue del laboratorio del usuario
func (s *Despliegues) Estado(idUsuario string) *domain.Despliegue {
	if lab, _ := s.labs.ByUser(idUsuario); lab != nil {
		if d := s.repository.Last(lab.ID); d != nil {
			return d
		}
	}
	return &domain.Despliegue{Estado: "NINGUNO"}
}

// proyectoExiste pregunta al GNS3 del laboratorio (con el usuario y token del dueño)
func (s *Despliegues) proyectoExiste(lab *domain.Laboratorio, nombre string) bool {
	req, err := http.NewRequest(http.MethodGet, fmt.Sprintf("http://%s:%d/v2/projects", s.env.GNS3Host, lab.PuertoBase), nil)
	if err != nil {
		return false
	}
	req.SetBasicAuth(lab.Usuario, lab.Token)
	resp, err := (&http.Client{Timeout: 10 * time.Second}).Do(req)
	if err != nil {
		return false // si no responde, el playbook dará el error
	}
	defer resp.Body.Close()
	var proyectos []struct {
		Name string `json:"name"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&proyectos)
	for _, p := range proyectos {
		if strings.EqualFold(p.Name, nombre) {
			return true
		}
	}
	return false
}

// ejecutar manda la topología al laboratorio y corre deploy_topology.yml,
// guardando el avance con cada tarea de Ansible
func (s *Despliegues) ejecutar(id string, lab *domain.Laboratorio, topo *domain.Topologia) {
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Minute)
	defer cancel()

	// Latido: si un paso tarda (arrancar routers), la otra réplica no lo da por muerto
	listo := make(chan struct{})
	defer close(listo)
	go func() {
		t := time.NewTicker(30 * time.Second)
		defer t.Stop()
		for {
			select {
			case <-t.C:
				s.repository.Heartbeat(id)
			case <-listo:
				return
			}
		}
	}()

	// 1) La topología ya resuelta (con todas las IPs) se copia al contenedor
	s.repository.Progress(id, 2, "Enviando la topología al laboratorio")
	datos, _ := json.MarshalIndent(topo, "", "  ")
	archivo := "raccoon-despliegue-" + id + ".json"
	if err := s.docker.CopyFile(ctx, lab.Contenedor, carpetaTemporal, archivo, datos); err != nil {
		log.Printf("[DESPLIEGUE] %s: %v", lab.Contenedor, err)
		s.repository.Finish(id, "ERROR", "No se pudo enviar la topología al laboratorio")
		return
	}

	// 2) Ansible, contando sus tareas para el porcentaje
	total := contarTareas(filepath.Join(s.dirTopologias(), "deploy_topology.yml"))
	hechas := 0
	paso := ""
	salida, codigo, err := s.docker.ExecStream(ctx, lab.Contenedor, []string{
		"ansible-playbook", playbookTopologia,
		"-e", fmt.Sprintf("gns3_url=http://127.0.0.1:%d", lab.PuertoBase),
		"-e", "gns3_user=" + lab.Usuario,
		"-e", "gns3_password=" + lab.Token,
		"-e", "project_name=" + topo.ProjectName,
		"-e", "topology_file=" + carpetaTemporal + "/" + archivo,
	}, func(linea string) {
		// Cada tarea empieza con "TASK [nombre] ****"
		if nombre, ok := strings.CutPrefix(linea, "TASK ["); ok {
			paso, _, _ = strings.Cut(nombre, "]")
			hechas++
			s.repository.Progress(id, min(95, 5+hechas*90/max(total, 1)), paso)
		}
	})
	_, _, _ = s.docker.Exec(ctx, lab.Contenedor, []string{"rm", "-f", carpetaTemporal + "/" + archivo})

	if err != nil || codigo != 0 {
		log.Printf("[DESPLIEGUE] %s falló (código %d, %v): %s", lab.Contenedor, codigo, err, ultimas(salida, 2000))
		s.repository.Finish(id, "ERROR", fmt.Sprintf("Falló en «%s»: %s", paso, resumenError(salida, err)))
		return
	}

	routers, hosts := 0, 0
	for _, n := range topo.Nodes {
		switch n.Type {
		case "cisco":
			routers++
		case "docker":
			hosts++
		}
	}
	s.repository.Finish(id, "TERMINADO", fmt.Sprintf(
		"Topología «%s» desplegada: %d routers y %d hosts. Los routers tardan 1 o 2 minutos en arrancar; "+
			"después ábrela en GNS3 (File → Open project).", topo.ProjectName, routers, hosts))
	log.Printf("[DESPLIEGUE] %s: %s listo", lab.Contenedor, topo.ProjectName)
}

// contarTareas cuenta las tareas del playbook ("- name:" debajo de "tasks:")
func contarTareas(ruta string) int {
	f, err := os.Open(ruta)
	if err != nil {
		return 20
	}
	defer f.Close()
	n, enTareas := 0, false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		l := strings.TrimSpace(sc.Text())
		if l == "tasks:" {
			enTareas = true
		} else if enTareas && strings.HasPrefix(l, "- name:") {
			n++
		}
	}
	return n
}

// resumenError saca de la salida de Ansible la línea que explica el error
func resumenError(salida string, err error) string {
	if err != nil && salida == "" {
		return err.Error()
	}
	for _, l := range strings.Split(salida, "\n") {
		l = strings.TrimSpace(l)
		if texto, ok := strings.CutPrefix(l, "[ERROR]:"); ok {
			return strings.TrimSpace(texto)
		}
		if _, json0, ok := strings.Cut(l, "FAILED! => "); ok {
			var r struct {
				Msg string `json:"msg"`
			}
			if json.Unmarshal([]byte(json0), &r) == nil && r.Msg != "" {
				return r.Msg
			}
		}
	}
	return ultimas(salida, 300)
}

func ultimas(s string, n int) string {
	s = strings.TrimSpace(s)
	if len(s) > n {
		return "…" + s[len(s)-n:]
	}
	return s
}
