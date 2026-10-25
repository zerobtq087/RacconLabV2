package service

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// Docker: cliente mínimo de la API de Docker por el socket unix (solo HTTP,
// sin llamar al comando docker). Los laboratorios GNS3 viven en Docker.
type Docker struct {
	http *http.Client
}

func NewDocker(socket string) *Docker {
	tr := &http.Transport{
		DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, "unix", socket)
		},
		MaxIdleConns:    10,
		IdleConnTimeout: 30 * time.Second,
	}
	return &Docker{http: &http.Client{Transport: tr}}
}

// peticion hace la llamada HTTP a Docker y decodifica la respuesta en destino (si no es nil)
func (d *Docker) peticion(ctx context.Context, metodo, ruta string, cuerpo any, destino any) (int, error) {
	var r io.Reader
	if cuerpo != nil {
		b, err := json.Marshal(cuerpo)
		if err != nil {
			return 0, err
		}
		r = bytes.NewReader(b)
	}

	req, err := http.NewRequestWithContext(ctx, metodo, "http://docker"+ruta, r)
	if err != nil {
		return 0, err
	}
	if cuerpo != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := d.http.Do(req)
	if err != nil {
		return 0, fmt.Errorf("no se pudo hablar con Docker: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return resp.StatusCode, fmt.Errorf("docker respondió %d: %s", resp.StatusCode, strings.TrimSpace(e.Message))
	}
	if destino != nil {
		if err := json.NewDecoder(resp.Body).Decode(destino); err != nil && err != io.EOF {
			return resp.StatusCode, err
		}
	}
	return resp.StatusCode, nil
}

// Contenedor: lo necesario para crear el contenedor de un laboratorio
type Contenedor struct {
	Nombre     string
	Imagen     string
	Env        []string
	Binds      []string // "origen_en_host:destino[:ro]"
	PuertoBase int      // se publican PuertoBase ... PuertoBase+Puertos-1 (mismo número adentro y afuera)
	Puertos    int
	ConKVM     bool
}

type puertoHost struct {
	HostIP   string `json:"HostIp"`
	HostPort string `json:"HostPort"`
}

// Create crea el contenedor y lo arranca
func (d *Docker) Create(ctx context.Context, c *Contenedor) error {
	expuestos := map[string]struct{}{}
	publicados := map[string][]puertoHost{}
	for p := c.PuertoBase; p < c.PuertoBase+c.Puertos; p++ {
		clave := strconv.Itoa(p) + "/tcp"
		expuestos[clave] = struct{}{}
		publicados[clave] = []puertoHost{{HostIP: "0.0.0.0", HostPort: strconv.Itoa(p)}}
	}

	hostConfig := map[string]any{
		"Privileged":    true, // GNS3 necesita ubridge, dynamips y su propio dockerd
		"Binds":         c.Binds,
		"RestartPolicy": map[string]string{"Name": "unless-stopped"},
		"PortBindings":  publicados,
		// El Docker interno guarda sus imágenes en un volumen propio del lab:
		// así puede usar overlay2 (sobre la capa del contenedor solo funciona vfs,
		// que copia la imagen completa por cada capa y cada nodo)
		"Mounts": []map[string]string{
			{"Type": "volume", "Source": volumenDocker(c.Nombre), "Target": "/var/lib/docker"},
		},
	}
	if c.ConKVM {
		hostConfig["Devices"] = []map[string]string{
			{"PathOnHost": "/dev/kvm", "PathInContainer": "/dev/kvm", "CgroupPermissions": "rwm"},
		}
	}

	var creado struct {
		ID string `json:"Id"`
	}
	_, err := d.peticion(ctx, http.MethodPost, "/containers/create?name="+url.QueryEscape(c.Nombre), map[string]any{
		"Image":        c.Imagen,
		"Env":          c.Env,
		"Labels":       map[string]string{"raccoon.lab": "true"},
		"ExposedPorts": expuestos,
		"HostConfig":   hostConfig,
	}, &creado)
	if err != nil {
		return fmt.Errorf("creando %s: %w", c.Nombre, err)
	}

	if _, err := d.peticion(ctx, http.MethodPost, "/containers/"+creado.ID+"/start", nil, nil); err != nil {
		_ = d.Remove(ctx, c.Nombre)
		return fmt.Errorf("arrancando %s: %w", c.Nombre, err)
	}
	return nil
}

// volumenDocker: nombre del volumen con el Docker interno del laboratorio
func volumenDocker(contenedor string) string {
	return contenedor + "-docker"
}

// Remove detiene y borra el contenedor y su volumen del Docker interno
// (si no existen, no es error)
func (d *Docker) Remove(ctx context.Context, nombre string) error {
	codigo, err := d.peticion(ctx, http.MethodDelete, "/containers/"+url.PathEscape(nombre)+"?force=true&v=true", nil, nil)
	if err != nil && codigo != http.StatusNotFound {
		return err
	}

	// El volumen se borra aparte (con nombre no se va con v=true); también limpia
	// el de un laboratorio cuyo contenedor ya no existía
	codigo, err = d.peticion(ctx, http.MethodDelete, "/volumes/"+url.PathEscape(volumenDocker(nombre))+"?force=true", nil, nil)
	if codigo == http.StatusNotFound {
		return nil
	}
	return err
}

// Running dice si el contenedor existe y está en ejecución
func (d *Docker) Running(ctx context.Context, nombre string) bool {
	var info struct {
		State struct {
			Running bool `json:"Running"`
		} `json:"State"`
	}
	if _, err := d.peticion(ctx, http.MethodGet, "/containers/"+url.PathEscape(nombre)+"/json", nil, &info); err != nil {
		return false
	}
	return info.State.Running
}

// Exec corre un comando dentro del contenedor (como "docker exec")
// y regresa la salida y el código de salida
func (d *Docker) Exec(ctx context.Context, nombre string, cmd []string) (string, int, error) {
	return d.ExecStream(ctx, nombre, cmd, nil)
}

// ExecStream es Exec, pero además llama a porLinea con cada línea de salida
// en cuanto llega (así se reporta el avance de ansible-playbook)
func (d *Docker) ExecStream(ctx context.Context, nombre string, cmd []string, porLinea func(string)) (string, int, error) {
	var ex struct {
		ID string `json:"Id"`
	}
	if _, err := d.peticion(ctx, http.MethodPost, "/containers/"+url.PathEscape(nombre)+"/exec",
		map[string]any{"AttachStdout": true, "AttachStderr": true, "Cmd": cmd}, &ex); err != nil {
		return "", -1, err
	}

	// Arrancar y leer la salida (formato multiplexado de Docker)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "http://docker/exec/"+ex.ID+"/start",
		strings.NewReader(`{"Detach":false,"Tty":false}`))
	if err != nil {
		return "", -1, err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := d.http.Do(req)
	if err != nil {
		return "", -1, err
	}
	salida := demultiplexar(resp.Body, porLinea)
	resp.Body.Close()

	var estado struct {
		ExitCode int `json:"ExitCode"`
	}
	if _, err := d.peticion(ctx, http.MethodGet, "/exec/"+ex.ID+"/json", nil, &estado); err != nil {
		return salida, -1, err
	}
	return salida, estado.ExitCode, nil
}

// demultiplexar quita los encabezados de 8 bytes que Docker pone a stdout/stderr.
// Si porLinea no es nil, la llama con cada línea completa. Regresa la salida
// (si pasa de 1 MB, solo el final: ahí están los errores).
func demultiplexar(r io.Reader, porLinea func(string)) string {
	const maximo = 1 << 20
	var out, pendiente []byte
	cab := make([]byte, 8)
	for {
		if _, err := io.ReadFull(r, cab); err != nil {
			break
		}
		trozo := make([]byte, binary.BigEndian.Uint32(cab[4:]))
		if _, err := io.ReadFull(r, trozo); err != nil {
			break
		}
		out = append(out, trozo...)
		if len(out) > maximo {
			out = out[len(out)-maximo:]
		}
		if porLinea != nil {
			pendiente = append(pendiente, trozo...)
			for {
				i := bytes.IndexByte(pendiente, '\n')
				if i < 0 {
					break
				}
				porLinea(strings.TrimRight(string(pendiente[:i]), "\r"))
				pendiente = pendiente[i+1:]
			}
		}
	}
	if porLinea != nil && len(pendiente) > 0 {
		porLinea(string(pendiente))
	}
	return string(out)
}

// CopyFile escribe un archivo dentro del contenedor (como "docker cp"):
// la API de Docker recibe un .tar y lo extrae en la carpeta indicada
func (d *Docker) CopyFile(ctx context.Context, nombre, carpeta, archivo string, contenido []byte) error {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	if err := tw.WriteHeader(&tar.Header{Name: archivo, Mode: 0o644, Size: int64(len(contenido)), ModTime: time.Now()}); err != nil {
		return err
	}
	if _, err := tw.Write(contenido); err != nil {
		return err
	}
	if err := tw.Close(); err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPut,
		"http://docker/containers/"+url.PathEscape(nombre)+"/archive?path="+url.QueryEscape(carpeta), &buf)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/x-tar")
	resp, err := d.http.Do(req)
	if err != nil {
		return fmt.Errorf("no se pudo hablar con Docker: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		msg, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("docker respondió %d al copiar %s: %s", resp.StatusCode, archivo, strings.TrimSpace(string(msg)))
	}
	return nil
}
