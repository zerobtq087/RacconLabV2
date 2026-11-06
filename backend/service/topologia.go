package service

import (
	"cmp"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/bits"
	"net/netip"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"raccoon-lab/backend/domain"
)

// Plantillas del profesor: clave del frontend → archivo en ansible/topologies
var Plantillas = map[string]string{
	"basica":     "basic_lan.json",
	"intermedia": "star_topology.json",
	"avanzada":   "vlan_topology.json",
}

// ErrValidacion: datos incorrectos del usuario (el handler responde 400 con el texto)
type ErrValidacion struct{ Texto string }

func (e *ErrValidacion) Error() string { return e.Texto }

func invalido(formato string, args ...any) error {
	return &ErrValidacion{Texto: fmt.Sprintf(formato, args...)}
}

var (
	nombreSeguro = regexp.MustCompile(`^[A-Za-z0-9_-]{3,40}$`) // proyecto de GNS3
	nodoSeguro   = regexp.MustCompile(`^[A-Za-z0-9_-]{1,30}$`) // nombre de nodo
	tagVlan      = regexp.MustCompile(`^vlan_(\d+)$`)
)

// CargarTopologia lee una topología base (las plantillas están en ansible/topologies)
func CargarTopologia(dir, archivo string) (*domain.Topologia, error) {
	datos, err := os.ReadFile(filepath.Join(dir, archivo))
	if err != nil {
		return nil, fmt.Errorf("no se pudo leer la topología %s: %w", archivo, err)
	}
	var t domain.Topologia
	if err := json.Unmarshal(datos, &t); err != nil {
		return nil, fmt.Errorf("la topología %s no es un JSON válido: %w", archivo, err)
	}
	return &t, nil
}

// ValidarSolicitud revisa los parámetros del profesor (formato; las redes se revisan al resolver)
func ValidarSolicitud(s *domain.SolicitudDespliegue) error {
	s.NombreLab = strings.TrimSpace(s.NombreLab)
	if !nombreSeguro.MatchString(s.NombreLab) {
		return invalido("El nombre del laboratorio debe tener de 3 a 40 letras, números, guion o guion bajo (sin espacios)")
	}
	s.RoutingProtocol = strings.ToLower(strings.TrimSpace(s.RoutingProtocol))
	if s.RoutingProtocol != "rip" && s.RoutingProtocol != "ospf" {
		return invalido("El enrutamiento debe ser RIP u OSPF")
	}
	if s.OspfArea < 0 || s.OspfArea > 4294967295 {
		return invalido("El área OSPF debe estar entre 0 y 4294967295")
	}
	if _, err := red(s.Subnet, "Red LAN"); err != nil {
		return err
	}
	if s.SerialSubnet != "" {
		if _, err := red(s.SerialSubnet, "Red de enlaces seriales"); err != nil {
			return err
		}
	}
	vistas := map[int]bool{}
	for _, v := range s.Vlans {
		if v.ID < 1 || v.ID > 4094 {
			return invalido("El ID de VLAN %d no es válido (debe ser de 1 a 4094)", v.ID)
		}
		if vistas[v.ID] {
			return invalido("La VLAN %d está repetida", v.ID)
		}
		vistas[v.ID] = true
		p, err := red(v.Subnet, fmt.Sprintf("Subred de la VLAN %d", v.ID))
		if err != nil {
			return err
		}
		if _, err := hostDe(p, v.Gateway, fmt.Sprintf("Gateway de la VLAN %d", v.ID)); err != nil {
			return err
		}
	}
	return nil
}

// red acepta CUALQUIER IP con su prefijo y regresa la red a la que pertenece:
// "192.200.100.50/16" → 192.200.0.0/16 (no es error escribir una IP de host).
// Sirve cualquier red IPv4 (privada, pública, clase A/B/C o prefijos poco comunes).
func red(texto, que string) (netip.Prefix, error) {
	p, err := netip.ParsePrefix(strings.TrimSpace(texto))
	if err != nil || !p.Addr().Is4() {
		return netip.Prefix{}, invalido("%s: \"%s\" no es una IP con prefijo válida (ej. 192.168.1.0/24)", que, texto)
	}
	if p.Bits() > 30 {
		return netip.Prefix{}, invalido("%s: /%d es demasiado pequeña: no deja hosts (máximo /30)", que, p.Bits())
	}
	return p.Masked(), nil
}

// rangoUsable: "192.168.10.1 a 192.168.10.254"
func rangoUsable(p netip.Prefix) string {
	return fmt.Sprintf("%s a %s", aIP(aNumero(p.Addr())+1), aIP(aNumero(ultima(p))-1))
}

// hostDe valida que una IP sea un host (ni red ni broadcast) dentro del prefijo;
// si no, dice qué IPs serían válidas
func hostDe(p netip.Prefix, texto, que string) (netip.Addr, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(texto))
	if err != nil || !a.Is4() {
		return netip.Addr{}, invalido("%s: \"%s\" no es una IP válida", que, texto)
	}
	switch {
	case !p.Contains(a):
		return netip.Addr{}, invalido("%s: %s no está dentro de %s; usa una IP de %s (por ejemplo %s)",
			que, a, p, rangoUsable(p), aIP(aNumero(p.Addr())+1))
	case a == p.Addr():
		return netip.Addr{}, invalido("%s: %s es la dirección de red de %s; usa una IP de %s", que, a, p, rangoUsable(p))
	case a == ultima(p):
		return netip.Addr{}, invalido("%s: %s es el broadcast de %s; usa una IP de %s", que, a, p, rangoUsable(p))
	}
	return a, nil
}

// InfoDeRed: lo que se muestra debajo de cada campo (red calculada, máscara y rango usable)
func InfoDeRed(texto, que string) (*domain.InfoRed, error) {
	p, err := red(texto, que)
	if err != nil {
		return nil, err
	}
	return &domain.InfoRed{
		Red: p.String(), Mascara: mascara(p.Bits()), PrimerHost: aIP(aNumero(p.Addr()) + 1).String(),
		UltimoHost: aIP(aNumero(ultima(p)) - 1).String(), Hosts: tamano(p) - 2,
	}, nil
}

// ---------------------------------------------------------------------
// Cuentas con IPv4 (como uint32)
// ---------------------------------------------------------------------

func aNumero(a netip.Addr) uint32 { b := a.As4(); return binary.BigEndian.Uint32(b[:]) }

func aIP(n uint32) netip.Addr {
	var b [4]byte
	binary.BigEndian.PutUint32(b[:], n)
	return netip.AddrFrom4(b)
}

func tamano(p netip.Prefix) uint32 { return uint32(1) << (32 - p.Bits()) }

func ultima(p netip.Prefix) netip.Addr { return aIP(aNumero(p.Addr()) + tamano(p) - 1) }

func mascara(bits int) string { return aIP(^uint32(0) << (32 - bits)).String() }

func comodin(bits int) string { return aIP(^(^uint32(0) << (32 - bits))).String() }

// enesimo: el host número n de la red (n=1 → primera IP usable)
func enesimo(p netip.Prefix, n int, que string) (netip.Addr, error) {
	if n < 1 || uint32(n) >= tamano(p)-1 {
		return netip.Addr{}, invalido("%s: el número de host %d no cabe en %s", que, n, p)
	}
	return aIP(aNumero(p.Addr()) + uint32(n)), nil
}

func traslapan(a, b netip.Prefix) bool { return a.Overlaps(b) }

// bolsa reparte subredes consecutivas de un bloque (LAN o seriales)
type bolsa struct {
	bloque    netip.Prefix
	siguiente uint64 // como uint64 para detectar que ya se acabó el bloque
	usado     bool
	nombre    string
}

func (b *bolsa) tomar(bits int, cuantas int) (netip.Prefix, error) {
	if bits < b.bloque.Bits() {
		return netip.Prefix{}, invalido("%s %s es más chica que una red /%d", b.nombre, b.bloque, bits)
	}
	t := uint64(1) << (32 - bits)
	inicio := (b.siguiente + t - 1) / t * t // alinear al tamaño de la subred
	fin := uint64(aNumero(b.bloque.Addr())) + uint64(tamano(b.bloque))
	if inicio+t > fin {
		return netip.Prefix{}, invalido("%s %s no alcanza: se necesitan %d redes /%d", b.nombre, b.bloque, cuantas, bits)
	}
	b.siguiente = inicio + t
	b.usado = true
	return netip.PrefixFrom(aIP(uint32(inicio)), bits), nil
}

// ---------------------------------------------------------------------
// ResolverTopologia: valida la topología y calcula TODAS las direcciones
// (routers, hosts y redes para RIP/OSPF) con los parámetros del profesor.
// Go es el único que hace estas cuentas; el playbook solo las aplica.
// ---------------------------------------------------------------------
func ResolverTopologia(t *domain.Topologia, s *domain.SolicitudDespliegue) error {
	if err := validarEstructura(t); err != nil {
		return err
	}

	lan, err := red(s.Subnet, "Red LAN")
	if err != nil {
		return err
	}
	bolsaLAN := &bolsa{bloque: lan, siguiente: uint64(aNumero(lan.Addr())), nombre: "La red LAN"}
	var bolsaSerial *bolsa
	if s.SerialSubnet != "" {
		serial, err := red(s.SerialSubnet, "Red de enlaces seriales")
		if err != nil {
			return err
		}
		bolsaSerial = &bolsa{bloque: serial, siguiente: uint64(aNumero(serial.Addr())), nombre: "La red de enlaces seriales"}
	}

	vlans := map[int]domain.Vlan{}
	for _, v := range s.Vlans {
		vlans[v.ID] = v
	}

	// 1) Cuántas redes de cada tipo hay (para dar mensajes claros si no alcanza)
	clase := map[string]string{} // tag → "lan" | "serial" | "vlan"
	var orden []string
	cuenta := map[string]int{}
	for _, n := range t.Nodes {
		for _, i := range n.Interfaces {
			if _, ya := clase[i.SubnetTag]; ya || i.SubnetTag == "" {
				continue
			}
			c := claseDe(i)
			clase[i.SubnetTag] = c
			orden = append(orden, i.SubnetTag)
			cuenta[c]++
		}
	}

	// Tamaño de cada LAN: /24 si la red alcanza; si es más chica, se divide
	// en partes iguales (ej. una /24 para 4 LAN → 4 redes /26)
	bitsLAN := max(24, lan.Bits()+bits.Len(uint(max(cuenta["lan"], 1)-1)))
	if err := revisarCapacidad(t, s, lan, bitsLAN, clase, orden, vlans); err != nil {
		return err
	}

	// 2) Repartir las redes en el orden en que aparecen (como el original)
	redes := map[string]netip.Prefix{}
	gateways := map[string]netip.Addr{} // VLAN: el gateway lo da el profesor
	for _, tag := range orden {
		ifz := primeraInterfaz(t, tag)
		switch clase[tag] {
		case "vlan":
			id := vlanDe(ifz)
			v, ok := vlans[id]
			if !ok {
				return invalido("La topología usa la VLAN %d: agrégala en la sección de VLANs", id)
			}
			p, _ := red(v.Subnet, "")
			g, _ := netip.ParseAddr(strings.TrimSpace(v.Gateway))
			redes[tag], gateways[tag] = p, g
		case "serial":
			if bolsaSerial == nil {
				return invalido("Esta topología tiene enlaces entre routers: indica la red de enlaces seriales")
			}
			bits := ifz.MaskBits
			if bits == 0 {
				bits = 30
			}
			p, err := bolsaSerial.tomar(bits, cuenta["serial"])
			if err != nil {
				return err
			}
			redes[tag] = p
		default:
			bits := ifz.MaskBits
			if bits == 0 {
				bits = bitsLAN
			}
			p, err := bolsaLAN.tomar(bits, cuenta["lan"])
			if err != nil {
				return err
			}
			redes[tag] = p
		}
	}

	// Las redes no deben encimarse entre sí
	for i, a := range orden {
		for _, b := range orden[i+1:] {
			if traslapan(redes[a], redes[b]) {
				return invalido("Las redes %s y %s se enciman: revisa la red LAN, la serial y las VLANs", redes[a], redes[b])
			}
		}
	}

	// 3) Routers: IP de cada interfaz y redes para RIP/OSPF
	usadas := map[netip.Addr]string{}
	ipRouter := map[string]netip.Addr{} // tag → IP del router (gateway de los hosts)
	for n := range t.Nodes {
		nodo := &t.Nodes[n]
		if nodo.Type != "cisco" {
			continue
		}
		vistas := map[string]bool{}
		padres := map[string]bool{}
		for k := range nodo.Interfaces {
			ifz := &nodo.Interfaces[k]
			if padre, _, esSub := strings.Cut(ifz.Name, "."); esSub && !padres[padre] {
				padres[padre] = true
				nodo.Parents = append(nodo.Parents, padre)
			}
			if ifz.SubnetTag == "" {
				continue
			}
			p := redes[ifz.SubnetTag]
			var ip netip.Addr
			if g, esVlan := gateways[ifz.SubnetTag]; esVlan {
				ip = g // la subinterfaz de la VLAN es su gateway
			} else {
				num := ifz.HostNumber
				if num == 0 {
					num = 1
				}
				if ip, err = enesimo(p, num, nodo.Name+" "+ifz.Name); err != nil {
					return err
				}
			}
			if otro, repetida := usadas[ip]; repetida {
				return invalido("%s %s tendría la IP %s, que ya usa %s", nodo.Name, ifz.Name, ip, otro)
			}
			usadas[ip] = nodo.Name + " " + ifz.Name
			ifz.IP, ifz.Netmask = ip.String(), mascara(p.Bits())
			if _, ya := ipRouter[ifz.SubnetTag]; !ya {
				ipRouter[ifz.SubnetTag] = ip
			}
			if !vistas[p.String()] {
				vistas[p.String()] = true
				nodo.Redes = append(nodo.Redes, domain.Red{Network: p.Addr().String(), Wildcard: comodin(p.Bits())})
			}
		}
	}

	// 4) Hosts: IP, máscara y gateway (el router de su red)
	for n := range t.Nodes {
		nodo := &t.Nodes[n]
		if nodo.Type != "docker" {
			continue
		}
		tag := nodo.SubnetTag
		if tag == "" && nodo.Vlan > 0 {
			tag = fmt.Sprintf("vlan_%d", nodo.Vlan)
		}
		p, ok := redes[tag]
		if !ok {
			return invalido("El host %s está en la red \"%s\", pero ningún router tiene esa red", nodo.Name, tag)
		}
		num := nodo.HostNumber
		if num == 0 {
			num = 10
		}
		ip, err := enesimo(p, num, "Host "+nodo.Name)
		if err != nil {
			return err
		}
		if otro, repetida := usadas[ip]; repetida {
			return invalido("El host %s tendría la IP %s, que ya usa %s", nodo.Name, ip, otro)
		}
		usadas[ip] = nodo.Name
		nodo.IP, nodo.Netmask, nodo.Prefix = ip.String(), mascara(p.Bits()), p.Bits()
		if g, ok := ipRouter[tag]; ok {
			nodo.Gateway = g.String()
		}
	}

	// 5) Parámetros del profesor dentro del JSON (como el original) y nombre del proyecto
	t.ProjectName = s.NombreLab
	// Se guardan las redes ya calculadas (192.200.100.50/16 → 192.200.0.0/16)
	vlansCalc := []domain.Vlan{}
	for _, v := range s.Vlans {
		p, _ := red(v.Subnet, "")
		vlansCalc = append(vlansCalc, domain.Vlan{ID: v.ID, Subnet: p.String(), Gateway: strings.TrimSpace(v.Gateway)})
	}
	serialCalc := ""
	if bolsaSerial != nil {
		serialCalc = bolsaSerial.bloque.String()
	}
	t.ProjectSettings = &domain.ProjectSettings{
		Subnet:          lan.String(),
		SerialSubnet:    serialCalc,
		Vlans:           vlansCalc,
		RoutingProtocol: s.RoutingProtocol,
		OspfArea:        s.OspfArea,
	}
	return nil
}

// tamanoMinimo: direcciones que necesita una red para que quepa el host número n
// (más red y broadcast), redondeado a potencia de 2. Ej. hasta el host 20 → 32 (/27)
func tamanoMinimo(n int) uint64 { return max(4, uint64(1)<<bits.Len(uint(n+1))) }

// prefijoPara: el prefijo de una red con ese número de direcciones (potencia de 2)
func prefijoPara(direcciones uint64) int { return 33 - bits.Len64(direcciones) }

// revisarCapacidad: antes de repartir, ¿alcanzan la red LAN, la serial y cada VLAN?
// Si no, dice cuántas direcciones faltan y qué prefijo usar.
func revisarCapacidad(t *domain.Topologia, s *domain.SolicitudDespliegue, lan netip.Prefix, bitsLAN int,
	clase map[string]string, orden []string, vlans map[int]domain.Vlan) error {

	// El número de host más alto que se usa en cada red (routers: 1 por omisión; hosts: 10)
	maxHost := map[string]int{}
	for _, n := range t.Nodes {
		for _, i := range n.Interfaces {
			if i.SubnetTag != "" {
				maxHost[i.SubnetTag] = max(maxHost[i.SubnetTag], cmp.Or(i.HostNumber, 1))
			}
		}
		if n.Type == "docker" {
			tag := n.SubnetTag
			if tag == "" && n.Vlan > 0 {
				tag = fmt.Sprintf("vlan_%d", n.Vlan)
			}
			maxHost[tag] = max(maxHost[tag], cmp.Or(n.HostNumber, 10))
		}
	}

	var lanes, seriales []string
	for _, tag := range orden {
		switch clase[tag] {
		case "lan":
			lanes = append(lanes, tag)
		case "serial":
			seriales = append(seriales, tag)
		case "vlan":
			v, ok := vlans[vlanDe(primeraInterfaz(t, tag))]
			if !ok {
				continue // ResolverTopologia dice qué VLAN falta
			}
			p, _ := red(v.Subnet, "")
			if necesita := tamanoMinimo(maxHost[tag]); uint64(tamano(p)) < necesita {
				return invalido("La VLAN %d (%s) es muy chica: sus hosts usan hasta el número %d y se necesitan "+
					"al menos %d direcciones (/%d). Tiene %d: faltan %d.",
					v.ID, p, maxHost[tag], necesita, prefijoPara(necesita), tamano(p), necesita-uint64(tamano(p)))
			}
		}
	}

	// LAN: todas del mismo tamaño
	if len(lanes) > 0 {
		porLAN := uint64(4)
		for _, tag := range lanes {
			porLAN = max(porLAN, tamanoMinimo(maxHost[tag]))
		}
		if bitsLAN > 30 || uint64(1)<<(32-bitsLAN) < porLAN {
			total := porLAN << bits.Len(uint(len(lanes)-1)) // LANs redondeadas a potencia de 2
			cuantas := fmt.Sprintf("las %d redes LAN de la topología: cada una necesita", len(lanes))
			if len(lanes) == 1 {
				cuantas = "la red LAN de la topología: necesita"
			}
			return invalido("La red LAN %s no alcanza para %s al menos %d direcciones (/%d) y en total se necesitan %d. "+
				"Tiene %d: faltan %d direcciones. Usa un prefijo /%d o menor (una red más grande).",
				lan, cuantas, porLAN, prefijoPara(porLAN), total, tamano(lan), total-uint64(tamano(lan)), prefijoPara(total))
		}
	}

	// Enlaces seriales: /30 cada uno (o el tamaño que diga la topología)
	if len(seriales) > 0 && s.SerialSubnet != "" {
		serial, _ := red(s.SerialSubnet, "")
		var total uint64
		for _, tag := range seriales {
			total += uint64(1) << (32 - cmp.Or(primeraInterfaz(t, tag).MaskBits, 30))
		}
		if total > uint64(tamano(serial)) {
			cuantos := fmt.Sprintf("los %d enlaces entre routers: cada uno usa", len(seriales))
			if len(seriales) == 1 {
				cuantos = "el enlace entre routers: usa"
			}
			return invalido("La red de enlaces seriales %s no alcanza para %s una red /30 (4 direcciones) y en total "+
				"se necesitan %d. Tiene %d: faltan %d direcciones. Usa un prefijo /%d o menor.",
				serial, cuantos, total, tamano(serial), total-uint64(tamano(serial)), prefijoPara(total))
		}
	}
	return nil
}

// claseDe decide si una interfaz va en una VLAN, en un enlace serial o en una LAN
func claseDe(i domain.Interfaz) string {
	switch {
	case vlanDe(&i) > 0:
		return "vlan"
	case strings.HasPrefix(i.Name, "Serial") || i.MaskBits == 30:
		return "serial"
	default:
		return "lan"
	}
}

func vlanDe(i *domain.Interfaz) int {
	if i.Vlan > 0 {
		return i.Vlan
	}
	if m := tagVlan.FindStringSubmatch(i.SubnetTag); m != nil {
		id, _ := strconv.Atoi(m[1])
		return id
	}
	return 0
}

func primeraInterfaz(t *domain.Topologia, tag string) *domain.Interfaz {
	for n := range t.Nodes {
		for k := range t.Nodes[n].Interfaces {
			if t.Nodes[n].Interfaces[k].SubnetTag == tag {
				return &t.Nodes[n].Interfaces[k]
			}
		}
	}
	return nil
}

// validarEstructura: nombres, tipos, plantillas y que los cables apunten a nodos que existen
func validarEstructura(t *domain.Topologia) error {
	if len(t.Nodes) == 0 {
		return invalido("La topología no tiene nodos")
	}
	if len(t.Nodes) > 30 {
		return invalido("La topología tiene %d nodos; el máximo es 30 (el lab tiene 39 consolas)", len(t.Nodes))
	}
	nodos := map[string]bool{}
	for _, n := range t.Nodes {
		if !nodoSeguro.MatchString(n.Name) {
			return invalido("El nombre de nodo \"%s\" no es válido (letras, números, guion o guion bajo)", n.Name)
		}
		if nodos[n.Name] {
			return invalido("El nodo %s está repetido", n.Name)
		}
		nodos[n.Name] = true
		switch n.Type {
		case "cisco", "docker", "switch":
		default:
			return invalido("El nodo %s tiene un tipo desconocido \"%s\" (cisco, switch o docker)", n.Name, n.Type)
		}
		if n.Template == "" {
			return invalido("El nodo %s no indica su plantilla de GNS3", n.Name)
		}
	}
	puertos := map[string]bool{}
	for _, e := range t.Links {
		for _, extremo := range []struct {
			nodo        string
			puerto, sub int
		}{{e.NodeA, e.PortA, e.PortASub}, {e.NodeB, e.PortB, e.PortBSub}} {
			if !nodos[extremo.nodo] {
				return invalido("Un cable usa el nodo %s, que no existe", extremo.nodo)
			}
			clave := fmt.Sprintf("%s/%d/%d", extremo.nodo, extremo.puerto, extremo.sub)
			if puertos[clave] {
				return invalido("El puerto %d/%d de %s tiene dos cables", extremo.puerto, extremo.sub, extremo.nodo)
			}
			puertos[clave] = true
		}
	}
	return nil
}
