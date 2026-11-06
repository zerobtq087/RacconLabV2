package service

import (
	"context"
	"errors"
	"fmt"
	"net/netip"
	"strings"
	"time"

	"raccoon-lab/backend/domain"
)

// revisor revisa los criterios dentro de UN proyecto del lab (del alumno o del profesor):
//   - routers y VPCS por su consola telnet,
//   - hosts Ubuntu con "docker exec" dentro del contenedor del lab,
//   - switches de GNS3 por la API (ports_mapping).
//
// Los nodos se buscan por NOMBRE: al importar, GNS3 les pone IDs nuevos.
type revisor struct {
	gns3       *GNS3
	docker     *Docker
	contenedor string // contenedor Docker del lab
	proyecto   string
	enable     string // contraseña de enable de la práctica (si la pide)
	nodos      map[string]NodoGNS3
	consolas   map[string]*Consola
}

func nuevoRevisor(g *GNS3, d *Docker, contenedor, proyecto, enable string) (*revisor, error) {
	nodos, err := g.Nodos(proyecto)
	if err != nil {
		return nil, err
	}
	r := &revisor{gns3: g, docker: d, contenedor: contenedor, proyecto: proyecto, enable: enable,
		nodos: map[string]NodoGNS3{}, consolas: map[string]*Consola{}}
	for _, n := range nodos {
		r.nodos[n.Nombre] = n
	}
	return r, nil
}

func (r *revisor) cerrar() {
	for _, c := range r.consolas {
		c.Close()
	}
}

func (r *revisor) nodo(nombre string) (NodoGNS3, error) {
	n, ok := r.nodos[nombre]
	if !ok {
		return n, fmt.Errorf("no se encontró el nodo %s en el proyecto", nombre)
	}
	if n.Estado != "started" && n.Tipo != "ethernet_switch" {
		return n, fmt.Errorf("%s está apagado", nombre)
	}
	return n, nil
}

// consola abre (una vez) la consola del nodo y la deja lista para comandos
func (r *revisor) consola(nombre string) (*Consola, error) {
	if c, ok := r.consolas[nombre]; ok {
		return c, nil
	}
	n, err := r.nodo(nombre)
	if err != nil {
		return nil, err
	}
	c, err := AbrirConsola(r.gns3.host, n.Consola)
	if err != nil {
		return nil, err
	}
	if n.Tipo == "vpcs" {
		err = c.PrepararVPCS()
	} else {
		err = c.PrepararIOS(r.enable, 3*time.Minute)
	}
	if err != nil {
		c.Close()
		return nil, fmt.Errorf("%s: %w", nombre, err)
	}
	r.consolas[nombre] = c
	return c, nil
}

func (r *revisor) comando(nombre, cmd string) (string, error) {
	c, err := r.consola(nombre)
	if err != nil {
		return "", err
	}
	return c.Comando(cmd, 30*time.Second)
}

// linux corre un comando en el host Ubuntu (docker exec dentro del lab)
func (r *revisor) linux(nombre, cmd string) (string, int, error) {
	n, err := r.nodo(nombre)
	if err != nil {
		return "", -1, err
	}
	id, _ := n.Propiedades["container_id"].(string)
	if id == "" {
		return "", -1, fmt.Errorf("%s no tiene contenedor", nombre)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	return r.docker.Exec(ctx, r.contenedor, []string{"docker", "exec", id, "sh", "-c", cmd})
}

// ipDe: la IP actual de una interfaz del nodo
func (r *revisor) ipDe(nombre, interfaz string) (netip.Prefix, error) {
	n, err := r.nodo(nombre)
	if err != nil {
		return netip.Prefix{}, err
	}
	var p netip.Prefix
	var ok bool
	switch n.Tipo {
	case "docker":
		out, _, err := r.linux(nombre, "ip -4 -o addr show eth0")
		if err != nil {
			return p, err
		}
		p, ok = ipDeLinux(out)
	case "vpcs":
		out, err := r.comando(nombre, "show ip")
		if err != nil {
			return p, err
		}
		p, ok = ipDeVPCS(out)
	case "dynamips":
		out, err := r.comando(nombre, "show ip interface "+interfaz)
		if err != nil {
			return p, err
		}
		p, ok = ipDeIOS(out)
	default:
		return p, fmt.Errorf("%s no tiene IP", nombre)
	}
	if !ok {
		return p, errors.New("sin IP")
	}
	return p, nil
}

// ping desde un nodo a una IP
func (r *revisor) ping(nombre string, destino netip.Addr) (bool, error) {
	n, err := r.nodo(nombre)
	if err != nil {
		return false, err
	}
	switch n.Tipo {
	case "docker":
		_, codigo, err := r.linux(nombre, "ping -c 3 -W 2 "+destino.String())
		return err == nil && codigo == 0, err
	case "vpcs":
		out, err := r.comando(nombre, "ping "+destino.String()+" -c 3")
		return pingVPCS(out), err
	default:
		out, err := r.comando(nombre, "ping "+destino.String()+" repeat 3 timeout 1")
		return pingIOS(out), err
	}
}

// Evaluar revisa un criterio: si cumple y qué se encontró (solo para el docente)
func (r *revisor) Evaluar(c *domain.Criterio) (bool, string) {
	cumple, obtenido, err := r.evaluar(c)
	if err != nil {
		return false, err.Error()
	}
	return cumple, obtenido
}

func (r *revisor) evaluar(c *domain.Criterio) (bool, string, error) {
	switch c.Tipo {
	case "PING":
		destino, err := netip.ParseAddr(c.Destino)
		if err != nil { // el destino es un nodo: su IP actual
			p, e := r.ipDe(c.Destino, c.Interfaz)
			if e != nil {
				return false, "destino " + c.Destino + ": " + e.Error(), nil
			}
			destino = p.Addr()
		}
		// "NO debe" solo cuenta si el origen sí tiene IP (si no, pasaría sin hacer nada)
		if r.nodos[c.Nodo].Tipo != "dynamips" {
			if _, e := r.ipDe(c.Nodo, ""); e != nil {
				return false, c.Nodo + ": " + e.Error(), nil
			}
		}
		responde, err := r.ping(c.Nodo, destino)
		if err != nil {
			return false, "", err
		}
		if !responde && c.Modo == "DEBE" {
			// el primer ping a veces se pierde mientras se resuelve ARP: un reintento
			time.Sleep(3 * time.Second)
			if responde, err = r.ping(c.Nodo, destino); err != nil {
				return false, "", err
			}
		}
		texto := fmt.Sprintf("ping a %s: %s", destino, map[bool]string{true: "responde", false: "no responde"}[responde])
		return responde == (c.Modo == "DEBE"), texto, nil

	case "IP_INTERFAZ":
		p, err := r.ipDe(c.Nodo, c.Interfaz)
		if err != nil {
			return false, "", err
		}
		esperado, _ := netip.ParsePrefix(c.Valor)
		if c.Modo == "RED" {
			return p.Bits() == esperado.Bits() && esperado.Contains(p.Addr()), p.String(), nil
		}
		return p == esperado, p.String(), nil

	case "PROTOCOLO":
		out, err := r.comando(c.Nodo, "show ip protocols")
		if err != nil {
			return false, "", err
		}
		activos := protocolosActivos(out)
		return activos[strings.ToLower(c.Modo)], fmt.Sprintf("activos: %v", llaves(activos)), nil

	case "VECINOS_OSPF":
		out, err := r.comando(c.Nodo, "show ip ospf neighbor")
		if err != nil {
			return false, "", err
		}
		n := vecinosFull(out)
		var minimo int
		fmt.Sscan(c.Valor, &minimo)
		return n >= minimo, fmt.Sprintf("%d vecinos en FULL", n), nil

	case "RUTA":
		red, _ := netip.ParsePrefix(c.Valor)
		out, err := r.comando(c.Nodo, fmt.Sprintf("show ip route %s %s", red.Addr(), mascara(red.Bits())))
		if err != nil {
			return false, "", err
		}
		origen := origenRuta(out)
		if !rutaExacta(out, red) {
			origen = ""
		}
		quiere := map[string]string{"OSPF": "ospf", "RIP": "rip", "ESTATICA": "static"}[c.Modo]
		if origen == "" {
			return false, "sin ruta a " + red.String(), nil
		}
		return origen == quiere, "ruta por " + origen, nil

	case "VLAN_PUERTO", "TRUNK":
		n, err := r.nodo(c.Nodo)
		if err != nil {
			return false, "", err
		}
		tipo, vlan := puertoSwitch(n, c.Interfaz)
		obtenido := fmt.Sprintf("%s VLAN %d", tipo, vlan)
		if c.Tipo == "TRUNK" {
			return tipo == "dot1q", obtenido, nil
		}
		return tipo == "access" && fmt.Sprint(vlan) == c.Valor, obtenido, nil

	case "LINEA_CONFIG":
		config, err := r.RunningConfig(c.Nodo)
		if err != nil {
			return false, "", err
		}
		esta := lineaPresente(config, c.Valor, c.Modo)
		return esta, map[bool]string{true: "la línea está", false: "la línea no está"}[esta], nil

	case "DHCP":
		n, err := r.nodo(c.Nodo)
		if err != nil {
			return false, "", err
		}
		var dhcp bool
		if n.Tipo == "docker" {
			config, _ := r.gns3.Archivo(r.proyecto, n.ID, "etc/network/interfaces")
			dhcp = dhcpConfigurado(config)
		} else {
			out, _ := r.comando(c.Nodo, "show ip")
			dhcp = strings.Contains(strings.ToUpper(out), "DHCP")
		}
		p, err := r.ipDe(c.Nodo, "")
		if !dhcp {
			return false, "no está configurado por DHCP", nil
		}
		if err != nil {
			return false, "DHCP configurado pero sin IP", nil
		}
		if c.Valor != "" {
			red, _ := netip.ParsePrefix(c.Valor)
			return red.Contains(p.Addr()), "IP " + p.String(), nil
		}
		return true, "IP " + p.String(), nil
	}
	return false, "", fmt.Errorf("tipo desconocido %s", c.Tipo)
}

// RunningConfig: "show running-config" del router (necesita modo enable)
func (r *revisor) RunningConfig(nombre string) (string, error) {
	out, err := r.comando(nombre, "show running-config")
	if err != nil {
		return "", err
	}
	if i := strings.Index(out, "Building configuration"); i >= 0 {
		out = out[i:]
	}
	return out, nil
}

// GuardarConfig: "write memory" para que la configuración quede en la NVRAM
// (al detener el nodo, GNS3 la guarda en el proyecto y entra en la exportación)
func (r *revisor) GuardarConfig(nombre string) error {
	_, err := r.comando(nombre, "write memory")
	return err
}

// Routers: los nodos Dynamips encendidos del proyecto
func (r *revisor) Routers() []string {
	var nombres []string
	for nombre, n := range r.nodos {
		if n.Tipo == "dynamips" && n.Estado == "started" {
			nombres = append(nombres, nombre)
		}
	}
	return nombres
}

// puertoSwitch: tipo (access/dot1q) y VLAN de un puerto del switch de GNS3
func puertoSwitch(n NodoGNS3, puerto string) (string, int) {
	numero := -1
	for _, p := range n.Puertos {
		if p.Nombre == puerto {
			numero = p.Puerto
		}
	}
	lista, _ := n.Propiedades["ports_mapping"].([]any)
	for _, x := range lista {
		m, _ := x.(map[string]any)
		if num, _ := m["port_number"].(float64); int(num) == numero {
			tipo, _ := m["type"].(string)
			vlan, _ := m["vlan"].(float64)
			return tipo, int(vlan)
		}
	}
	return "sin puerto", 0
}

func llaves(m map[string]bool) []string {
	var l []string
	for k := range m {
		l = append(l, k)
	}
	return l
}
