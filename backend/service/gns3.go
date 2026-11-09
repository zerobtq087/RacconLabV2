package service

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"time"

	"raccoon-lab/backend/config"
	"raccoon-lab/backend/domain"

	"github.com/google/uuid"
)

// GNS3: cliente mínimo de la API REST del GNS3 de UN laboratorio.
// Go usa el usuario y token del lab (guardados en Firebird); el docente
// que supervisa nunca los recibe.
type GNS3 struct {
	base    string // http://host:puerto/v2
	host    string // para las consolas telnet
	usuario string
	token   string
	http    *http.Client
	largo   *http.Client // exportar/importar pueden tardar
}

func NewGNS3(env *config.Env, lab *domain.Laboratorio) *GNS3 {
	return &GNS3{
		base:    fmt.Sprintf("http://%s:%d/v2", env.GNS3Host, lab.PuertoBase),
		host:    env.GNS3Host,
		usuario: lab.Usuario,
		token:   lab.Token,
		http:    &http.Client{Timeout: 60 * time.Second},
		largo:   &http.Client{Timeout: 10 * time.Minute},
	}
}

// NodoGNS3: lo que se usa de un nodo de GNS3
type NodoGNS3 struct {
	ID          string         `json:"node_id"`
	Nombre      string         `json:"name"`
	Tipo        string         `json:"node_type"` // dynamips, docker, vpcs, ethernet_switch, qemu...
	Estado      string         `json:"status"`    // started, stopped
	Consola     int            `json:"console"`
	Aux         int            `json:"aux"`
	Propiedades map[string]any `json:"properties"`
	Puertos     []struct {
		Nombre  string `json:"name"`
		Adapter int    `json:"adapter_number"`
		Puerto  int    `json:"port_number"`
	} `json:"ports"`
}

func (g *GNS3) peticion(cli *http.Client, metodo, ruta string, cuerpo io.Reader, destino any) ([]byte, error) {
	req, err := http.NewRequest(metodo, g.base+ruta, cuerpo)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(g.usuario, g.token)
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("no se pudo hablar con el GNS3 del laboratorio: %w", err)
	}
	defer resp.Body.Close()
	datos, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	if resp.StatusCode >= 400 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.Unmarshal(datos, &e)
		return nil, fmt.Errorf("GNS3 respondió %d: %s", resp.StatusCode, strings.TrimSpace(e.Message))
	}
	if destino != nil && len(datos) > 0 {
		if err := json.Unmarshal(datos, destino); err != nil {
			return nil, err
		}
	}
	return datos, nil
}

func (g *GNS3) json(metodo, ruta string, cuerpo any, destino any) error {
	var r io.Reader
	if cuerpo != nil {
		b, _ := json.Marshal(cuerpo)
		r = bytes.NewReader(b)
	}
	_, err := g.peticion(g.http, metodo, ruta, r, destino)
	return err
}

// Proyectos: los proyectos del lab con cuántos nodos tienen encendidos
func (g *GNS3) Proyectos() ([]domain.ProyectoGNS3, error) {
	proyectos := []domain.ProyectoGNS3{}
	if err := g.json(http.MethodGet, "/projects", nil, &proyectos); err != nil {
		return nil, err
	}
	for i := range proyectos {
		if proyectos[i].Estado != "opened" {
			continue
		}
		if nodos, err := g.Nodos(proyectos[i].ID); err == nil {
			proyectos[i].Nodos = len(nodos)
			for _, n := range nodos {
				if n.Estado == "started" {
					proyectos[i].Encendidos++
				}
			}
		}
	}
	return proyectos, nil
}

// ProyectoPorNombre: el ID del proyecto con ese nombre ("" si no existe)
func (g *GNS3) ProyectoPorNombre(nombre string) (string, error) {
	proyectos := []domain.ProyectoGNS3{}
	if err := g.json(http.MethodGet, "/projects", nil, &proyectos); err != nil {
		return "", err
	}
	for _, p := range proyectos {
		if p.Nombre == nombre {
			return p.ID, nil
		}
	}
	return "", nil
}

func (g *GNS3) Nodos(proyecto string) ([]NodoGNS3, error) {
	nodos := []NodoGNS3{}
	if err := g.json(http.MethodGet, "/projects/"+proyecto+"/nodes", nil, &nodos); err != nil {
		return nil, err
	}
	return nodos, nil
}

func (g *GNS3) Abrir(proyecto string) error {
	return g.json(http.MethodPost, "/projects/"+proyecto+"/open", map[string]any{}, nil)
}

func (g *GNS3) Cerrar(proyecto string) error {
	return g.json(http.MethodPost, "/projects/"+proyecto+"/close", map[string]any{}, nil)
}

func (g *GNS3) Borrar(proyecto string) error {
	return g.json(http.MethodDelete, "/projects/"+proyecto, nil, nil)
}

func (g *GNS3) Encender(proyecto string) error {
	_, err := g.peticion(g.largo, http.MethodPost, "/projects/"+proyecto+"/nodes/start", strings.NewReader("{}"), nil)
	return err
}

// Detener apaga todos los nodos. Al detenerse, los routers guardan su NVRAM
// como startup-config (y GNS3 solo exporta proyectos detenidos).
func (g *GNS3) Detener(proyecto string) error {
	_, err := g.peticion(g.largo, http.MethodPost, "/projects/"+proyecto+"/nodes/stop", strings.NewReader("{}"), nil)
	return err
}

// Exportar: el proyecto como .gns3project (zip), sin imágenes (cada lab ya las tiene)
func (g *GNS3) Exportar(proyecto string) ([]byte, error) {
	datos, err := g.peticion(g.largo, http.MethodGet,
		"/projects/"+proyecto+"/export?include_images=no&include_snapshots=no&reset_mac_addresses=yes", nil, nil)
	if err != nil {
		return nil, err
	}
	return sinGhost(datos)
}

// sinGhost quita del .gns3project los archivos .ghost de Dynamips: son la RAM
// "fantasma" del IOS (256 MB, ~45 MB comprimidos) y Dynamips los vuelve a crear
// al encender. Así cada práctica o entrega pesa KB en lugar de 45 MB.
// Las demás entradas se copian tal cual (sin descomprimir).
func sinGhost(datos []byte) ([]byte, error) {
	zr, err := zip.NewReader(bytes.NewReader(datos), int64(len(datos)))
	if err != nil {
		return nil, fmt.Errorf("la exportación no es un zip: %w", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for _, f := range zr.File {
		if strings.HasSuffix(f.Name, ".ghost") {
			continue
		}
		r, err := f.OpenRaw()
		if err != nil {
			return nil, err
		}
		w, err := zw.CreateRaw(&f.FileHeader)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(w, r); err != nil {
			return nil, err
		}
	}
	if err := zw.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Importar: crea un proyecto NUEVO con ese nombre a partir de un .gns3project.
// GNS3 genera IDs nuevos para los nodos (por eso los criterios usan el nombre del nodo).
func (g *GNS3) Importar(nombre string, zip []byte) (string, error) {
	id := uuid.NewString()
	var p domain.ProyectoGNS3
	datos, err := g.peticion(g.largo, http.MethodPost,
		"/projects/"+id+"/import?name="+url.QueryEscape(nombre), bytes.NewReader(zip), nil)
	if err != nil {
		return "", err
	}
	_ = json.Unmarshal(datos, &p)
	if p.ID != "" {
		id = p.ID
	}
	return id, nil
}

// Archivo: un archivo de un nodo (lo mismo que "Edit config"); "" si no existe
func (g *GNS3) Archivo(proyecto, nodo, ruta string) (string, error) {
	datos, err := g.peticion(g.http, http.MethodGet, "/projects/"+proyecto+"/nodes/"+nodo+"/files/"+ruta, nil, nil)
	if err != nil {
		if strings.Contains(err.Error(), "404") {
			return "", nil
		}
		return "", err
	}
	return string(datos), nil
}

// ConfigGuardada: la configuración que el nodo tiene guardada (sin tocar su consola)
func (g *GNS3) ConfigGuardada(proyecto string, n NodoGNS3) (string, error) {
	switch n.Tipo {
	case "dynamips":
		id, _ := n.Propiedades["dynamips_id"].(float64)
		return g.Archivo(proyecto, n.ID, fmt.Sprintf("configs/i%d_startup-config.cfg", int(id)))
	case "docker":
		return g.Archivo(proyecto, n.ID, "etc/network/interfaces")
	case "vpcs":
		return g.Archivo(proyecto, n.ID, "startup.vpc")
	case "ethernet_switch":
		b, _ := json.MarshalIndent(n.Propiedades["ports_mapping"], "", "  ")
		return string(b), nil
	}
	return "", nil
}

// PuertosConsola: los puertos de consola (y aux) de todos los nodos de los
// proyectos abiertos del lab, solo los que caen en [desde, hasta]. Para el
// firewall una respuesta tardía no sirve: tiempo límite corto.
func (g *GNS3) PuertosConsola(desde, hasta int) ([]int, error) {
	rapido := &http.Client{Timeout: 2 * time.Second}
	var proyectos []domain.ProyectoGNS3
	if _, err := g.peticion(rapido, http.MethodGet, "/projects", nil, &proyectos); err != nil {
		return nil, err
	}
	puertos := []int{}
	for _, p := range proyectos {
		if p.Estado != "opened" {
			continue // un proyecto cerrado no tiene consolas escuchando
		}
		var nodos []NodoGNS3
		if _, err := g.peticion(rapido, http.MethodGet, "/projects/"+p.ID+"/nodes", nil, &nodos); err != nil {
			return nil, err
		}
		for _, n := range nodos {
			aux, _ := n.Propiedades["aux"].(float64) // Docker y Dynamips pueden tener consola auxiliar
			for _, puerto := range []int{n.Consola, n.Aux, int(aux)} {
				if puerto >= desde && puerto <= hasta && !slices.Contains(puertos, puerto) {
					puertos = append(puertos, puerto)
				}
			}
		}
	}
	slices.Sort(puertos)
	return puertos, nil
}
