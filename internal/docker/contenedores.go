package docker

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
)

// Especificacion: lo necesario para crear un contenedor de workspace
type Especificacion struct {
	Nombre    string
	Imagen    string
	Env       []string
	Etiquetas map[string]string
	Binds     []string // "origen_en_host:destino[:ro]"
	// Puerto que se publica SOLO en 127.0.0.1 del host (el proxy con token lo expone)
	PuertoLocal int
	ConKVM      bool
}

type puertoHost struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

type dispositivo struct {
	PathOnHost        string `json:"PathOnHost"`
	PathInContainer   string `json:"PathInContainer"`
	CgroupPermissions string `json:"CgroupPermissions"`
}

// Crear crea el contenedor y lo arranca
func (c *Cliente) Crear(ctx context.Context, e *Especificacion) error {
	puerto := strconv.Itoa(e.PuertoLocal) + "/tcp"

	hostConfig := map[string]any{
		"Privileged":    true, // GNS3 necesita ubridge, dynamips y su propio dockerd
		"Binds":         e.Binds,
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		"PortBindings": map[string][]puertoHost{
			puerto: {{HostIP: "127.0.0.1", HostPort: strconv.Itoa(e.PuertoLocal)}},
		},
	}
	if e.ConKVM {
		hostConfig["Devices"] = []dispositivo{{"/dev/kvm", "/dev/kvm", "rwm"}}
	}

	cuerpo, err := json.Marshal(map[string]any{
		"Image":        e.Imagen,
		"Env":          e.Env,
		"Labels":       e.Etiquetas,
		"ExposedPorts": map[string]struct{}{puerto: {}},
		"HostConfig":   hostConfig,
	})
	if err != nil {
		return err
	}

	var creado struct {
		ID string `json:"Id"`
	}
	ruta := "/containers/create?name=" + url.QueryEscape(e.Nombre)
	if _, err := c.peticion(ctx, http.MethodPost, ruta, bytes.NewReader(cuerpo), &creado); err != nil {
		return fmt.Errorf("creando %s: %w", e.Nombre, err)
	}
	if _, err := c.peticion(ctx, http.MethodPost, "/containers/"+creado.ID+"/start", nil, nil); err != nil {
		_ = c.Eliminar(ctx, e.Nombre)
		return fmt.Errorf("arrancando %s: %w", e.Nombre, err)
	}
	return nil
}

// Eliminar detiene y borra el contenedor (si no existe, no es error)
func (c *Cliente) Eliminar(ctx context.Context, nombre string) error {
	codigo, err := c.peticion(ctx, http.MethodDelete, "/containers/"+url.PathEscape(nombre)+"?force=true&v=true", nil, nil)
	if codigo == http.StatusNotFound {
		return nil
	}
	return err
}

// Corriendo dice si el contenedor existe y está en ejecución
func (c *Cliente) Corriendo(ctx context.Context, nombre string) (bool, error) {
	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	codigo, err := c.peticion(ctx, http.MethodGet, "/containers/"+url.PathEscape(nombre)+"/json", nil, &info)
	if codigo == http.StatusNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return info.State.Running, nil
}

// Ejecutar corre un comando dentro del contenedor (como "docker exec") y regresa
// la salida combinada y el código de salida. Usa la API, no el comando docker.
func (c *Cliente) Ejecutar(ctx context.Context, nombre string, cmd []string) (string, int, error) {
	cuerpo, _ := json.Marshal(map[string]any{
		"AttachStdout": true,
		"AttachStderr": true,
		"Cmd":          cmd,
	})
	var ex struct {
		ID string `json:"Id"`
	}
	if _, err := c.peticion(ctx, http.MethodPost, "/containers/"+url.PathEscape(nombre)+"/exec", bytes.NewReader(cuerpo), &ex); err != nil {
		return "", -1, err
	}

	// Arrancar y leer la salida (formato multiplexado de Docker)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/exec/"+ex.ID+"/start",
		strings.NewReader(`{"Detach":false,"Tty":false}`))
	if err != nil {
		return "", -1, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return "", -1, err
	}
	salida := demultiplexar(resp.Body)
	resp.Body.Close()

	var estado struct {
		ExitCode int `json:"ExitCode"`
	}
	if _, err := c.peticion(ctx, http.MethodGet, "/exec/"+ex.ID+"/json", nil, &estado); err != nil {
		return salida, -1, err
	}
	return salida, estado.ExitCode, nil
}

// demultiplexar quita los encabezados de 8 bytes que Docker pone a stdout/stderr
func demultiplexar(r io.Reader) string {
	var out strings.Builder
	cab := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, cab); err != nil {
			break
		}
		n := binary.BigEndian.Uint32(cab[4:])
		if _, err := io.CopyN(&out, r, int64(n)); err != nil {
			break
		}
		if out.Len() > 1<<20 { // máximo 1 MB de salida
			break
		}
	}
	return out.String()
}
