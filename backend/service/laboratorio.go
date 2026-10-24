package service

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"
	"raccoon-lab/backend/repository"
)

// Laboratorios crea y destruye los contenedores GNS3.
// Go NO abre puertos: Docker publica el bloque de cada laboratorio
// y el propio GNS3 server pide usuario y contraseña (token).
type Laboratorios struct {
	repository *repository.LaboratorioRepository
	docker     *Docker
	env        *config.Env
}

func NewLaboratorios(r *repository.LaboratorioRepository, d *Docker, env *config.Env) *Laboratorios {
	return &Laboratorios{repository: r, docker: d, env: env}
}

// Create aparta el bloque de puertos y levanta el contenedor en segundo plano
func (s *Laboratorios) Create(idUsuario string, colaborativo bool) error {
	lab, err := s.repository.Reserve(idUsuario, colaborativo, s.env.GNS3Port, s.env.LabPuertos, s.env.LabsMax)
	if err != nil {
		return err
	}

	// Ya con el registro guardado, se trae con el nombre y la matrícula del dueño
	if completo := s.repository.ByID(lab.ID); completo != nil {
		lab = completo
	}

	go s.levantar(lab)
	return nil
}

// levantar crea el contenedor, espera a GNS3 y carga las plantillas de routers
func (s *Laboratorios) levantar(lab *domain.Laboratorio) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	base := lab.PuertoBase
	vol := s.env.GNS3Volumes

	c := &Contenedor{
		Nombre: lab.Contenedor,
		Imagen: s.env.GNS3Image,
		Env: []string{
			"GNS3_PORT=" + strconv.Itoa(base),
			"GNS3_CONSOLE_START=" + strconv.Itoa(base+1),
			"GNS3_CONSOLE_END=" + strconv.Itoa(base+s.env.LabPuertos-1),
			"GNS3_USER=" + lab.Usuario,
			"GNS3_PASSWORD=" + lab.Token,
		},
		Binds: []string{
			vol + "/gns3_images/QEMU:/root/GNS3/images/QEMU:ro",
			vol + "/gns3_images/IOS:/root/GNS3/images/IOS:ro",
			vol + "/gns3_images/Docker:/root/GNS3/images/Docker:ro",           // ubuntu-gns3.tar (host de las prácticas)
			vol + "/gns3_projects/" + lab.Contenedor + ":/root/GNS3/projects", // los proyectos sobreviven a la limpieza
			vol + "/ansible:/root/ansible:ro",
		},
		PuertoBase: base,
		Puertos:    s.env.LabPuertos,
		ConKVM:     true,
	}

	// Por si quedó un contenedor huérfano con el mismo nombre
	_ = s.docker.Remove(ctx, c.Nombre)

	err := s.docker.Create(ctx, c)
	if err != nil && strings.Contains(strings.ToLower(err.Error()), "kvm") {
		// Sin /dev/kvm (p. ej. VM sin virtualización anidada): sin aceleración
		log.Printf("[LABS] sin /dev/kvm en el host; %s se crea sin KVM", c.Nombre)
		c.ConKVM = false
		err = s.docker.Create(ctx, c)
	}
	if err != nil {
		log.Printf("[LABS] %s: %v", c.Nombre, err)
		s.repository.SetEstado(lab.ID, "ERROR", "No se pudo crear el contenedor del laboratorio")
		return
	}

	if !s.esperarGNS3(ctx, lab) {
		s.repository.SetEstado(lab.ID, "ERROR", "GNS3 no arrancó a tiempo; limpia el laboratorio e intenta de nuevo")
		return
	}

	aviso := s.cargarPlantillas(ctx, lab)
	if aviso == "" {
		aviso = s.revisarImagenes(ctx, lab)
	}
	s.repository.SetEstado(lab.ID, "CORRIENDO", aviso)
	log.Printf("[LABS] %s listo en el puerto %d", c.Nombre, base)
}

// esperarGNS3 pregunta a /v2/version (con el usuario y token) hasta que responda
func (s *Laboratorios) esperarGNS3(ctx context.Context, lab *domain.Laboratorio) bool {
	url := fmt.Sprintf("http://%s:%d/v2/version", s.env.GNS3Host, lab.PuertoBase)
	cli := &http.Client{Timeout: 3 * time.Second}

	for range 150 { // hasta ~5 minutos (antes de GNS3 se carga la imagen Ubuntu)
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		req.SetBasicAuth(lab.Usuario, lab.Token)
		if resp, err := cli.Do(req); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				return true
			}
		}
		select {
		case <-ctx.Done():
			return false
		case <-time.After(2 * time.Second):
		}
	}
	return false
}

// cargarPlantillas corre el playbook que registra los routers en GNS3.
// Si falla, el laboratorio sigue vivo y solo se avisa.
func (s *Laboratorios) cargarPlantillas(ctx context.Context, lab *domain.Laboratorio) string {
	salida, codigo, err := s.docker.Exec(ctx, lab.Contenedor, []string{
		"ansible-playbook", "-i", "127.0.0.1,", "-c", "local",
		"/root/ansible/global/deploy_templates.yml",
		"-e", fmt.Sprintf("gns3_url=http://127.0.0.1:%d", lab.PuertoBase),
		"-e", "gns3_user=" + lab.Usuario,
		"-e", "gns3_password=" + lab.Token,
		"-e", "target_host=127.0.0.1",
	})
	if err != nil || codigo != 0 {
		if len(salida) > 1500 {
			salida = salida[len(salida)-1500:]
		}
		log.Printf("[LABS] %s: plantillas fallaron (código %d, %v): %s", lab.Contenedor, codigo, err, salida)
		return "El laboratorio está activo, pero no se cargaron las plantillas de routers"
	}
	return ""
}

const (
	// imagenPrincipal: el router c7200 (Dynamips) de las plantillas
	imagenPrincipal = "/root/GNS3/images/IOS/c7200-adventerprisek9-mz.124-24.T5.bin"
	// imagenUbuntu: host de las prácticas; entrypoint.sh la carga de gns3_images/Docker/ubuntu-gns3.tar
	imagenUbuntu = "raccoon/ubuntu-host:latest"
)

// revisarImagenes ve DENTRO del contenedor si están el router principal y el host Ubuntu
// (la API puede correr en otro contenedor y no ver gns3_images del host).
// Si falta alguna, el laboratorio sigue vivo y solo se avisa.
func (s *Laboratorios) revisarImagenes(ctx context.Context, lab *domain.Laboratorio) string {
	var faltan []string

	if _, codigo, err := s.docker.Exec(ctx, lab.Contenedor, []string{"test", "-f", imagenPrincipal}); err != nil || codigo != 0 {
		faltan = append(faltan, "router c7200 en gns3_images/IOS")
	}
	if _, codigo, err := s.docker.Exec(ctx, lab.Contenedor, []string{"docker", "image", "inspect", imagenUbuntu}); err != nil || codigo != 0 {
		faltan = append(faltan, "host Ubuntu en gns3_images/Docker (se usará VPCS)")
	}

	if len(faltan) == 0 {
		return ""
	}
	log.Printf("[LABS] %s: faltan imágenes: %s", lab.Contenedor, strings.Join(faltan, "; "))
	return "Faltan imágenes: " + strings.Join(faltan, " y ") + "; avisa al administrador"
}

// Status: el laboratorio del usuario (dueño o miembro)
func (s *Laboratorios) Status(idUsuario string) *domain.EstadoLab {
	lab, dueno := s.repository.ByUser(idUsuario)
	if lab == nil {
		return &domain.EstadoLab{
			Laboratorio: &domain.Laboratorio{Estado: "NINGUNO"},
			Miembros:    []domain.Persona{},
		}
	}
	return &domain.EstadoLab{
		Laboratorio: lab,
		SoyDueno:    dueno,
		Miembros:    s.repository.Miembros(lab.ID),
	}
}

// Leave: el dueño destruye el laboratorio; un miembro solo se sale del equipo
func (s *Laboratorios) Leave(ctx context.Context, idUsuario string) error {
	lab, dueno := s.repository.ByUser(idUsuario)
	if lab == nil {
		return repository.ErrSinLaboratorio
	}
	if !dueno {
		return s.repository.Leave(idUsuario)
	}
	return s.Destroy(ctx, lab)
}

// Destroy borra el contenedor y el registro (lo usa también el monitor del admin)
func (s *Laboratorios) Destroy(ctx context.Context, lab *domain.Laboratorio) error {
	if err := s.docker.Remove(ctx, lab.Contenedor); err != nil {
		return fmt.Errorf("no se pudo borrar el contenedor: %w", err)
	}
	log.Printf("[LABS] %s eliminado", lab.Contenedor)
	return s.repository.Delete(lab.ID)
}
