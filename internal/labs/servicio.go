// Package labs: ciclo de vida de los laboratorios GNS3 (contenedores Docker).
package labs

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"log"
	"math/big"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"raccoon_lab/internal/config"
	"raccoon_lab/internal/docker"
	"raccoon_lab/internal/repositorio"
)

const (
	tamBloque       = 200   // puertos por laboratorio: base (web) + consolas
	maxLaboratorios = 100   // bloques disponibles
	desfasePublico  = 10000 // puerto público = base + 10000
)

var (
	ErrYaActivo       = errors.New("ya tienes o estás en un laboratorio activo; límpialo o sal antes de crear otro")
	ErrSinLaboratorio = errors.New("no estás en ningún laboratorio")
	ErrCodigoInvalido = errors.New("el código no existe o el laboratorio ya se cerró")
	ErrSinPuertos     = errors.New("no hay puertos libres para más laboratorios")
)

// Servicio coordina Docker, Firebird y el proxy. Se crea una vez y se comparte por puntero.
type Servicio struct {
	Cfg    *config.Config
	Docker *docker.Cliente
	Repo   *repositorio.WorkspaceRepo
	Proxy  *Proxy

	mu sync.Mutex // una creación a la vez (evita choques de puertos)
}

func Nuevo(cfg *config.Config, dk *docker.Cliente, repo *repositorio.WorkspaceRepo) *Servicio {
	return &Servicio{Cfg: cfg, Docker: dk, Repo: repo, Proxy: NuevoProxy()}
}

// Estado es lo que ve el frontend
type Estado struct {
	Running        bool       `json:"running"`
	IDWorkspace    string     `json:"id_workspace,omitempty"`
	IPRealHost     string     `json:"ip_real_host,omitempty"`
	PuertoWeb      int        `json:"puerto_web,omitempty"`
	TokenAcceso    string     `json:"token_acceso,omitempty"`
	EsColaborativo bool       `json:"es_colaborativo"`
	SoyDueno       bool       `json:"soy_dueno"`
	CodigoLab      string     `json:"codigo_lab,omitempty"`
	Dueno          string     `json:"dueno,omitempty"`
	Inicio         *time.Time `json:"inicio,omitempty"`
	Aviso          string     `json:"aviso,omitempty"`
}

func aleatorio(n int, alfabeto string) string {
	b := make([]byte, n)
	for i := range b {
		k, _ := rand.Int(rand.Reader, big.NewInt(int64(len(alfabeto))))
		b[i] = alfabeto[k.Int64()]
	}
	return string(b)
}

func (s *Servicio) nombreContenedor(matricula string) string {
	return fmt.Sprintf("rl-%s-%s", s.Cfg.Entorno, matricula)
}

func (s *Servicio) estadoDe(w *repositorio.Workspace, soyDueno bool, host string, corriendo bool) *Estado {
	e := &Estado{
		Running:     corriendo,
		IDWorkspace: w.ID,
		IPRealHost:  host,
		PuertoWeb:   w.PuertoBase + desfasePublico,
		TokenAcceso: w.Token,
		SoyDueno:    soyDueno,
		Dueno:       w.Dueno,
		Inicio:      &w.Inicio,
	}
	if w.CodigoColab != nil {
		e.EsColaborativo, e.CodigoLab = true, *w.CodigoColab
	}
	if !corriendo {
		e.Aviso = "El contenedor no está en ejecución. Límpialo y vuelve a desplegarlo."
	}
	return e
}

// Estado del laboratorio del usuario (host = IP/nombre con el que entró a la plataforma)
func (s *Servicio) Estado(ctx context.Context, matricula, host string) (*Estado, error) {
	w, soyDueno, err := s.Repo.DeUsuario(ctx, matricula)
	if err != nil {
		return nil, err
	}
	if w == nil {
		return &Estado{Running: false}, nil
	}
	corriendo, err := s.Docker.Corriendo(ctx, w.ID)
	if err != nil {
		return nil, err
	}
	return s.estadoDe(w, soyDueno, host, corriendo), nil
}

// puertoLibre: el bloque no está en la BD ni ocupado en el host (p. ej. por el otro entorno)
func puertoLibre(base int) bool {
	for _, dir := range []string{fmt.Sprintf("127.0.0.1:%d", base), fmt.Sprintf(":%d", base+desfasePublico)} {
		ln, err := net.Listen("tcp", dir)
		if err != nil {
			return false
		}
		ln.Close()
	}
	return true
}

func (s *Servicio) asignarPuerto(ctx context.Context) (int, error) {
	usados, err := s.Repo.PuertosUsados(ctx)
	if err != nil {
		return 0, err
	}
	for i := range maxLaboratorios {
		base := s.Cfg.GNS3.PuertoBase + i*tamBloque
		if !usados[base] && puertoLibre(base) {
			return base, nil
		}
	}
	return 0, ErrSinPuertos
}

func (s *Servicio) nuevoCodigo(ctx context.Context) (string, error) {
	for range 20 {
		c := "COLAB-" + aleatorio(4, "ABCDEFGHJKLMNPQRSTUVWXYZ23456789")
		existe, err := s.Repo.CodigoExiste(ctx, c)
		if err != nil {
			return "", err
		}
		if !existe {
			return c, nil
		}
	}
	return "", errors.New("no se pudo generar un código único")
}

// Crear levanta un laboratorio nuevo para el usuario
func (s *Servicio) Crear(ctx context.Context, matricula, host string, colaborativo bool) (*Estado, error) {
	w, err := s.crearContenedor(ctx, matricula, colaborativo)
	if err != nil {
		return nil, err
	}
	log.Printf("[LABS] %s creó %s (puerto %d, colaborativo=%v)", matricula, w.ID, w.PuertoBase+desfasePublico, colaborativo)
	e := s.estadoDe(w, true, host, true)

	// Esperar a GNS3 y cargar las plantillas (~1 min). Fuera del candado: no frena a los demás.
	if aviso := s.prepararGNS3(ctx, w); aviso != "" {
		e.Aviso = aviso
	}
	return e, nil
}

// crearContenedor: la parte que no puede ir en paralelo (puertos, nombre, registro)
func (s *Servicio) crearContenedor(ctx context.Context, matricula string, colaborativo bool) (*repositorio.Workspace, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, _, err := s.Repo.DeUsuario(ctx, matricula); err != nil {
		return nil, err
	} else if w != nil {
		return nil, ErrYaActivo
	}

	base, err := s.asignarPuerto(ctx)
	if err != nil {
		return nil, err
	}

	w := &repositorio.Workspace{
		ID:         s.nombreContenedor(matricula),
		Dueno:      matricula,
		PuertoBase: base,
		Token:      aleatorio(12, "abcdefghijkmnpqrstuvwxyzABCDEFGHJKLMNPQRSTUVWXYZ23456789"),
		Inicio:     time.Now(),
	}
	if colaborativo {
		codigo, err := s.nuevoCodigo(ctx)
		if err != nil {
			return nil, err
		}
		w.CodigoColab = &codigo
	}

	// Por si quedó un contenedor huérfano con el mismo nombre
	_ = s.Docker.Eliminar(ctx, w.ID)

	vol := s.Cfg.GNS3.Volumes
	esp := &docker.Especificacion{
		Nombre: w.ID,
		Imagen: s.Cfg.GNS3.Imagen,
		Env: []string{
			fmt.Sprintf("GNS3_WEB_PORT=%d", base),
			fmt.Sprintf("GNS3_CONSOLE_START=%d", base+1),
			fmt.Sprintf("GNS3_CONSOLE_END=%d", base+150),
		},
		Etiquetas: map[string]string{
			"raccoon.entorno": s.Cfg.Entorno,
			"raccoon.dueno":   matricula,
		},
		Binds: []string{
			vol + "/gns3_images/QEMU:/root/GNS3/images/QEMU:ro",
			vol + "/gns3_images/IOS:/root/GNS3/images/IOS:ro",
			vol + "/gns3_projects/" + w.ID + ":/root/GNS3/projects", // los proyectos sobreviven a la limpieza
			vol + "/ansible:/root/ansible:ro",
		},
		PuertoLocal: base,
		ConKVM:      true,
	}

	if err := s.Docker.Crear(ctx, esp); err != nil {
		// Sin /dev/kvm (p. ej. VM sin virtualización anidada): sin KVM
		if strings.Contains(strings.ToLower(err.Error()), "kvm") {
			log.Printf("[LABS] sin /dev/kvm en el host; %s se crea sin aceleración", w.ID)
			esp.ConKVM = false
			err = s.Docker.Crear(ctx, esp)
		}
		if err != nil {
			return nil, err
		}
	}

	if err := s.Repo.Guardar(ctx, w); err != nil {
		_ = s.Docker.Eliminar(context.Background(), w.ID)
		return nil, fmt.Errorf("guardando el laboratorio: %w", err)
	}
	if err := s.Proxy.Abrir(w.ID, base+desfasePublico, base, w.Token); err != nil {
		s.destruir(context.Background(), w)
		return nil, err
	}
	return w, nil
}

// prepararGNS3 espera a que GNS3 responda y corre el playbook de plantillas.
// Si falla, el laboratorio sigue vivo y se regresa un aviso.
func (s *Servicio) prepararGNS3(ctx context.Context, w *repositorio.Workspace) string {
	urlVersion := fmt.Sprintf("http://127.0.0.1:%d/v2/version", w.PuertoBase)
	cli := &http.Client{Timeout: 3 * time.Second}

	listo := false
	for range 60 { // hasta ~2 minutos
		if resp, err := cli.Get(urlVersion); err == nil {
			resp.Body.Close()
			if resp.StatusCode == http.StatusOK {
				listo = true
				break
			}
		}
		select {
		case <-ctx.Done():
			return "Se canceló la espera de GNS3"
		case <-time.After(2 * time.Second):
		}
	}
	if !listo {
		return "GNS3 tardó demasiado en arrancar; espera un momento y recarga"
	}

	salida, codigo, err := s.Docker.Ejecutar(ctx, w.ID, []string{
		"ansible-playbook", "/root/ansible/global/deploy_templates.yml",
		"-e", fmt.Sprintf("gns3_url=http://127.0.0.1:%d", w.PuertoBase),
		"-e", "gns3_host=127.0.0.1",
		"-e", "target_host=127.0.0.1",
	})
	if err != nil || codigo != 0 {
		log.Printf("[LABS] %s: plantillas fallaron (código %d, err %v): %s", w.ID, codigo, err, ultimas(salida, 1500))
		return "El laboratorio está activo, pero no se cargaron las plantillas de routers"
	}
	return ""
}

func ultimas(s string, n int) string {
	if len(s) > n {
		return s[len(s)-n:]
	}
	return s
}

// Unirse al laboratorio de un equipo con su código
func (s *Servicio) Unirse(ctx context.Context, matricula, host, codigo string) (*Estado, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	if w, _, err := s.Repo.DeUsuario(ctx, matricula); err != nil {
		return nil, err
	} else if w != nil {
		return nil, ErrYaActivo
	}
	w, err := s.Repo.PorCodigo(ctx, strings.ToUpper(strings.TrimSpace(codigo)))
	if err != nil {
		return nil, err
	}
	if w == nil {
		return nil, ErrCodigoInvalido
	}
	if err := s.Repo.AgregarMiembro(ctx, w.ID, matricula); err != nil {
		return nil, err
	}
	log.Printf("[LABS] %s se unió a %s", matricula, w.ID)
	corriendo, _ := s.Docker.Corriendo(ctx, w.ID)
	return s.estadoDe(w, false, host, corriendo), nil
}

// Limpiar: el dueño destruye el laboratorio; un miembro solo se sale
func (s *Servicio) Limpiar(ctx context.Context, matricula string) (string, error) {
	w, soyDueno, err := s.Repo.DeUsuario(ctx, matricula)
	if err != nil {
		return "", err
	}
	if w == nil {
		return "", ErrSinLaboratorio
	}
	if !soyDueno {
		if err := s.Repo.QuitarMiembro(ctx, matricula); err != nil {
			return "", err
		}
		log.Printf("[LABS] %s salió de %s", matricula, w.ID)
		return "Saliste del laboratorio del equipo", nil
	}
	s.destruir(ctx, w)
	log.Printf("[LABS] %s limpió %s", matricula, w.ID)
	return "Laboratorio eliminado y recursos liberados", nil
}

func (s *Servicio) destruir(ctx context.Context, w *repositorio.Workspace) {
	s.Proxy.Cerrar(w.ID)
	if err := s.Docker.Eliminar(ctx, w.ID); err != nil {
		log.Printf("[LABS] eliminando contenedor %s: %v", w.ID, err)
	}
	if err := s.Repo.Eliminar(ctx, w.ID); err != nil {
		log.Printf("[LABS] eliminando registro %s: %v", w.ID, err)
	}
}

// Restaurar se llama al arrancar la app: vuelve a abrir los proxies de los
// laboratorios que siguen corriendo y borra los registros de los que ya no existen.
func (s *Servicio) Restaurar(ctx context.Context) {
	lista, err := s.Repo.Todos(ctx)
	if err != nil {
		log.Printf("[LABS] restaurando: %v", err)
		return
	}
	for _, w := range lista {
		corriendo, err := s.Docker.Corriendo(ctx, w.ID)
		if err != nil {
			log.Printf("[LABS] restaurando %s: %v", w.ID, err)
			continue
		}
		if !corriendo {
			s.destruir(ctx, w)
			log.Printf("[LABS] %s ya no existía; registro eliminado", w.ID)
			continue
		}
		if err := s.Proxy.Abrir(w.ID, w.PuertoBase+desfasePublico, w.PuertoBase, w.Token); err != nil {
			log.Printf("[LABS] restaurando proxy %s: %v", w.ID, err)
		}
	}
	if len(lista) > 0 {
		log.Printf("[LABS] %d laboratorio(s) revisado(s) al arrancar", len(lista))
	}
}
