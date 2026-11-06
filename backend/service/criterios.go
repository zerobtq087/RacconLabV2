package service

import (
	"fmt"
	"math"
	"net/netip"
	"regexp"
	"strconv"
	"strings"
	"time"

	"raccoon-lab/backend/domain"
)

// =====================================================================
// Criterios de las prácticas: validación, lectura de salidas (funciones
// puras con pruebas) y el revisor que se conecta al lab del alumno.
// =====================================================================

// Qué tipos de nodo sirven para cada criterio
var nodosDeCriterio = map[string][]string{
	"PING":         {"docker", "vpcs", "dynamips"},
	"IP_INTERFAZ":  {"docker", "vpcs", "dynamips"},
	"PROTOCOLO":    {"dynamips"},
	"VECINOS_OSPF": {"dynamips"},
	"RUTA":         {"dynamips"},
	"VLAN_PUERTO":  {"ethernet_switch"},
	"TRUNK":        {"ethernet_switch"},
	"LINEA_CONFIG": {"dynamips"},
	"DHCP":         {"docker", "vpcs"},
}

// ValidarCriterio revisa que el criterio tenga sentido con los nodos del proyecto base
func ValidarCriterio(c *domain.Criterio, nodos map[string]domain.NodoPractica) error {
	tipos, ok := nodosDeCriterio[c.Tipo]
	if !ok {
		return invalido("Tipo de criterio desconocido: %s", c.Tipo)
	}
	que := fmt.Sprintf("Criterio %d", c.Orden)
	if c.Peso < 1 || c.Peso > 100 {
		return invalido("%s: el peso debe ser de 1 a 100", que)
	}
	n, ok := nodos[c.Nodo]
	if !ok {
		return invalido("%s: el nodo \"%s\" no está en el proyecto", que, c.Nodo)
	}
	if !contiene(tipos, n.Tipo) {
		return invalido("%s: este criterio no aplica a %s (es %s)", que, c.Nodo, n.Tipo)
	}
	tienePuerto := func(p string) bool { return contiene(n.Puertos, p) }
	c.Valor = strings.TrimSpace(c.Valor)
	c.Mensaje = strings.TrimSpace(c.Mensaje)

	switch c.Tipo {
	case "PING":
		c.Destino = strings.TrimSpace(c.Destino)
		if _, err := netip.ParseAddr(c.Destino); err != nil {
			if _, ok := nodos[c.Destino]; !ok {
				return invalido("%s: el destino debe ser una IP o un nodo del proyecto", que)
			}
		}
		if c.Modo != "DEBE" && c.Modo != "NO_DEBE" {
			return invalido("%s: indica si el ping debe o no debe funcionar", que)
		}
	case "IP_INTERFAZ":
		if n.Tipo == "dynamips" && !tienePuerto(c.Interfaz) {
			return invalido("%s: elige una interfaz de %s", que, c.Nodo)
		}
		p, err := netip.ParsePrefix(c.Valor)
		if err != nil || !p.Addr().Is4() {
			return invalido("%s: escribe la IP con su prefijo (ej. 192.168.1.1/24)", que)
		}
		if c.Modo != "EXACTA" && c.Modo != "RED" {
			return invalido("%s: elige si la IP debe ser exacta o estar dentro de la red", que)
		}
		if c.Modo == "RED" {
			c.Valor = p.Masked().String()
		}
	case "PROTOCOLO":
		if c.Modo != "OSPF" && c.Modo != "RIP" {
			return invalido("%s: elige OSPF o RIP", que)
		}
	case "VECINOS_OSPF":
		if v, err := strconv.Atoi(c.Valor); err != nil || v < 1 || v > 50 {
			return invalido("%s: el número de vecinos debe ser de 1 a 50", que)
		}
	case "RUTA":
		p, err := netip.ParsePrefix(c.Valor)
		if err != nil || !p.Addr().Is4() {
			return invalido("%s: escribe la red de la ruta (ej. 192.168.2.0/24)", que)
		}
		c.Valor = p.Masked().String()
		if c.Modo != "OSPF" && c.Modo != "RIP" && c.Modo != "ESTATICA" {
			return invalido("%s: elige si la ruta es por OSPF, RIP o estática", que)
		}
	case "VLAN_PUERTO", "TRUNK":
		if !tienePuerto(c.Interfaz) {
			return invalido("%s: elige un puerto de %s", que, c.Nodo)
		}
		if c.Tipo == "VLAN_PUERTO" {
			if v, err := strconv.Atoi(c.Valor); err != nil || v < 1 || v > 4094 {
				return invalido("%s: la VLAN debe ser de 1 a 4094", que)
			}
		}
	case "LINEA_CONFIG":
		c.Valor = strings.Join(strings.Fields(c.Valor), " ")
		if c.Valor == "" || len(c.Valor) > 200 {
			return invalido("%s: escribe la línea que debe aparecer en la running-config", que)
		}
		if c.Modo != "EXACTA" && c.Modo != "EMPIEZA" && c.Modo != "CONTIENE" {
			return invalido("%s: elige cómo comparar la línea", que)
		}
	case "DHCP":
		if c.Valor != "" {
			p, err := netip.ParsePrefix(c.Valor)
			if err != nil {
				return invalido("%s: la red esperada debe ser como 192.168.1.0/24 (o vacía)", que)
			}
			c.Valor = p.Masked().String()
		}
	}
	return nil
}

func contiene(lista []string, x string) bool {
	for _, v := range lista {
		if v == x {
			return true
		}
	}
	return false
}

// MensajeAlumno: lo que ve el alumno si falla (el del profesor, o uno que NO da la respuesta)
func MensajeAlumno(c *domain.Criterio) string {
	if c.Mensaje != "" {
		return c.Mensaje
	}
	destino := c.Destino
	if _, err := netip.ParseAddr(destino); err == nil {
		destino = "el destino indicado"
	}
	switch c.Tipo {
	case "PING":
		if c.Modo == "NO_DEBE" {
			return fmt.Sprintf("Desde %s hay conectividad con %s y no debería", c.Nodo, destino)
		}
		return fmt.Sprintf("Desde %s no hay conectividad con %s", c.Nodo, destino)
	case "IP_INTERFAZ":
		return fmt.Sprintf("%s: la dirección IP no es la que pide la práctica", strings.TrimSpace(c.Nodo+" "+c.Interfaz))
	case "PROTOCOLO":
		return fmt.Sprintf("%s: no está activo el enrutamiento que pide la práctica", c.Nodo)
	case "VECINOS_OSPF":
		return fmt.Sprintf("%s: no tiene los vecinos OSPF esperados", c.Nodo)
	case "RUTA":
		return fmt.Sprintf("%s: le falta una ruta que pide la práctica", c.Nodo)
	case "VLAN_PUERTO":
		return fmt.Sprintf("%s %s: el puerto no está en la VLAN que pide la práctica", c.Nodo, c.Interfaz)
	case "TRUNK":
		return fmt.Sprintf("%s %s: el puerto no está configurado como lo pide la práctica", c.Nodo, c.Interfaz)
	case "LINEA_CONFIG":
		return fmt.Sprintf("%s: falta una configuración que pide la práctica", c.Nodo)
	case "DHCP":
		return fmt.Sprintf("%s: no obtiene su dirección como lo pide la práctica", c.Nodo)
	}
	return "Un criterio no se cumplió"
}

// Descripcion: qué se pedía (solo lo ve el docente)
func Descripcion(c *domain.Criterio) string {
	switch c.Tipo {
	case "PING":
		que := "debe responder"
		if c.Modo == "NO_DEBE" {
			que = "NO debe responder"
		}
		return fmt.Sprintf("Ping de %s a %s: %s", c.Nodo, c.Destino, que)
	case "IP_INTERFAZ":
		donde := strings.TrimSpace(c.Nodo + " " + c.Interfaz)
		if c.Modo == "RED" {
			return fmt.Sprintf("%s con una IP dentro de %s", donde, c.Valor)
		}
		return fmt.Sprintf("%s = %s", donde, c.Valor)
	case "PROTOCOLO":
		return fmt.Sprintf("%s con %s activo", c.Nodo, c.Modo)
	case "VECINOS_OSPF":
		return fmt.Sprintf("%s con al menos %s vecinos OSPF en FULL", c.Nodo, c.Valor)
	case "RUTA":
		return fmt.Sprintf("%s con ruta %s a %s", c.Nodo, strings.ToLower(c.Modo), c.Valor)
	case "VLAN_PUERTO":
		return fmt.Sprintf("%s %s en la VLAN %s", c.Nodo, c.Interfaz, c.Valor)
	case "TRUNK":
		return fmt.Sprintf("%s %s como trunk (dot1q)", c.Nodo, c.Interfaz)
	case "LINEA_CONFIG":
		return fmt.Sprintf("%s: línea que %s \"%s\"", c.Nodo, map[string]string{"EXACTA": "es", "EMPIEZA": "empieza con", "CONTIENE": "contiene"}[c.Modo], c.Valor)
	case "DHCP":
		if c.Valor != "" {
			return fmt.Sprintf("%s con IP por DHCP dentro de %s", c.Nodo, c.Valor)
		}
		return fmt.Sprintf("%s con IP por DHCP", c.Nodo)
	}
	return c.Tipo
}

// Calificar: porcentaje de puntos obtenidos (0-100), redondeado
func Calificar(resultados []domain.Resultado) int {
	total, obtenidos := 0, 0
	for _, r := range resultados {
		total += r.Peso
		obtenidos += r.Puntos
	}
	if total == 0 {
		return 0
	}
	return int(math.Round(float64(obtenidos) * 100 / float64(total)))
}

// FormatoRetraso: "2 h 15 min", "3 d 4 h", "45 min", "menos de 1 min"
func FormatoRetraso(d time.Duration) string {
	min := int(d.Minutes())
	switch {
	case min < 1:
		return "menos de 1 min"
	case min < 60:
		return fmt.Sprintf("%d min", min)
	case min < 24*60:
		if min%60 == 0 {
			return fmt.Sprintf("%d h", min/60)
		}
		return fmt.Sprintf("%d h %d min", min/60, min%60)
	}
	dias, horas := min/(24*60), (min%(24*60))/60
	if horas == 0 {
		return fmt.Sprintf("%d d", dias)
	}
	return fmt.Sprintf("%d d %d h", dias, horas)
}

// ---------------------------------------------------------------------
// Lectores de salidas (puros: se prueban con texto real)
// ---------------------------------------------------------------------

var (
	reIOSAddress = regexp.MustCompile(`Internet address is (\d+\.\d+\.\d+\.\d+/\d+)`)
	reProtocolo  = regexp.MustCompile(`Routing Protocol is "(\w+)`)
	reKnownVia   = regexp.MustCompile(`Known via "(\w+)`)
	reVPCSIP     = regexp.MustCompile(`IP/MASK\s*:\s*(\d+\.\d+\.\d+\.\d+/\d+)`)
	reLinuxIP    = regexp.MustCompile(`inet (\d+\.\d+\.\d+\.\d+/\d+)`)
	reExito      = regexp.MustCompile(`Success rate is (\d+) percent`)
	reDHCP       = regexp.MustCompile(`(?m)^\s*iface\s+eth0\s+inet\s+dhcp\b`)
)

// ipDeIOS: "show ip interface X" → 192.168.1.1/24
func ipDeIOS(out string) (netip.Prefix, bool) { return prefijoDe(reIOSAddress, out) }

// ipDeVPCS: "show ip" del VPCS → IP/MASK : 10.0.0.10/24 (0.0.0.0/0 = sin IP)
func ipDeVPCS(out string) (netip.Prefix, bool) {
	p, ok := prefijoDe(reVPCSIP, out)
	return p, ok && p.Addr() != netip.IPv4Unspecified()
}

// ipDeLinux: "ip -4 -o addr show eth0" → 192.168.1.10/24
func ipDeLinux(out string) (netip.Prefix, bool) { return prefijoDe(reLinuxIP, out) }

func prefijoDe(re *regexp.Regexp, out string) (netip.Prefix, bool) {
	m := re.FindStringSubmatch(out)
	if m == nil {
		return netip.Prefix{}, false
	}
	p, err := netip.ParsePrefix(m[1])
	return p, err == nil
}

// protocolosActivos: "show ip protocols" → {ospf, rip}
func protocolosActivos(out string) map[string]bool {
	activos := map[string]bool{}
	for _, m := range reProtocolo.FindAllStringSubmatch(out, -1) {
		activos[strings.ToLower(m[1])] = true
	}
	return activos
}

// vecinosFull: líneas de "show ip ospf neighbor" en estado FULL
func vecinosFull(out string) int {
	n := 0
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "FULL") {
			n++
		}
	}
	return n
}

// origenRuta: "show ip route <red> <máscara>" → ospf, rip, static, connected ("" si no hay)
func origenRuta(out string) string {
	if m := reKnownVia.FindStringSubmatch(out); m != nil {
		return strings.ToLower(m[1])
	}
	return ""
}

// rutaExacta: que la entrada sea de ESA red (y no de una que la contiene)
func rutaExacta(out string, red netip.Prefix) bool {
	return strings.Contains(out, "Routing entry for "+red.String())
}

// lineaPresente: ¿la running-config tiene la línea? (sin importar mayúsculas ni espacios de más)
func lineaPresente(config, linea, modo string) bool {
	buscar := strings.ToLower(strings.Join(strings.Fields(linea), " "))
	for _, l := range strings.Split(config, "\n") {
		l = strings.ToLower(strings.Join(strings.Fields(l), " "))
		switch {
		case modo == "EXACTA" && l == buscar,
			modo == "EMPIEZA" && strings.HasPrefix(l, buscar),
			modo == "CONTIENE" && strings.Contains(l, buscar):
			return true
		}
	}
	return false
}

// pingIOS: "Success rate is 66 percent (2/3)" → hubo respuesta
func pingIOS(out string) bool {
	m := reExito.FindStringSubmatch(out)
	return m != nil && m[1] != "0"
}

// pingVPCS: "84 bytes from 10.0.0.20 icmp_seq=1 ..."
func pingVPCS(out string) bool { return strings.Contains(out, "bytes from") }

// dhcpConfigurado: el Edit config del host Ubuntu dice "iface eth0 inet dhcp"
func dhcpConfigurado(interfaces string) bool { return reDHCP.MatchString(interfaces) }
