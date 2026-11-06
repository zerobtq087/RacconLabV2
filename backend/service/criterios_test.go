package service

import (
	"archive/zip"
	"bytes"
	"io"
	"net/netip"
	"strings"
	"testing"
	"time"

	"raccoon-lab/backend/domain"
)

// Salidas reales de un Cisco 7200 (IOS 15.2), VPCS y Ubuntu
const (
	salidaShowIPInt = `FastEthernet0/0 is up, line protocol is up
  Internet address is 192.168.10.1/24
  Broadcast address is 255.255.255.255`

	salidaProtocolos = `*** IP Routing is NSF aware ***

Routing Protocol is "application"
  Sending updates every 0 seconds
Routing Protocol is "ospf 1"
  Outgoing update filter list for all interfaces is not set
  Router ID 10.0.0.1`

	salidaVecinos = `Neighbor ID     Pri   State           Dead Time   Address         Interface
10.0.0.2          1   FULL/DR         00:00:35    10.0.0.2        FastEthernet1/0
10.0.0.3          1   FULL/BDR        00:00:31    10.0.1.3        FastEthernet1/1
10.0.0.4          1   INIT/DROTHER    00:00:39    10.0.2.4        FastEthernet2/0`

	salidaRuta = `Routing entry for 192.168.20.0/24
  Known via "ospf 1", distance 110, metric 2, type intra area
  Last update from 10.0.0.2 on FastEthernet1/0, 00:01:10 ago`

	salidaRutaMayor = `Routing entry for 192.168.0.0/16
  Known via "static", distance 1, metric 0`

	salidaVPCS = `NAME        : PC1[1]
IP/MASK     : 192.168.10.10/24
GATEWAY     : 192.168.10.1
DNS         :
MAC         : 00:50:79:66:68:00`

	salidaVPCSSinIP = `NAME        : PC1[1]
IP/MASK     : 0.0.0.0/0
GATEWAY     : 0.0.0.0`

	salidaLinux = `2: eth0    inet 192.168.10.20/24 brd 192.168.10.255 scope global eth0\       valid_lft forever preferred_lft forever`
)

func TestLectores(t *testing.T) {
	if p, ok := ipDeIOS(salidaShowIPInt); !ok || p.String() != "192.168.10.1/24" {
		t.Errorf("ipDeIOS = %v %v", p, ok)
	}
	if _, ok := ipDeIOS("FastEthernet0/0 is administratively down\n  Internet protocol processing disabled"); ok {
		t.Error("ipDeIOS sin IP debería fallar")
	}
	if p, ok := ipDeVPCS(salidaVPCS); !ok || p.String() != "192.168.10.10/24" {
		t.Errorf("ipDeVPCS = %v %v", p, ok)
	}
	if _, ok := ipDeVPCS(salidaVPCSSinIP); ok {
		t.Error("VPCS con 0.0.0.0 no tiene IP")
	}
	if p, ok := ipDeLinux(salidaLinux); !ok || p.String() != "192.168.10.20/24" {
		t.Errorf("ipDeLinux = %v %v", p, ok)
	}

	act := protocolosActivos(salidaProtocolos)
	if !act["ospf"] || act["rip"] {
		t.Errorf("protocolosActivos = %v", act)
	}
	if n := vecinosFull(salidaVecinos); n != 2 {
		t.Errorf("vecinosFull = %d, quería 2", n)
	}

	red := netip.MustParsePrefix("192.168.20.0/24")
	if origenRuta(salidaRuta) != "ospf" || !rutaExacta(salidaRuta, red) {
		t.Error("la ruta OSPF a 192.168.20.0/24 no se leyó")
	}
	// IOS responde con la ruta que contiene la red: no cuenta como la ruta pedida
	if rutaExacta(salidaRutaMayor, red) {
		t.Error("una ruta /16 no es la ruta /24 pedida")
	}
	if origenRuta("% Network not in table") != "" {
		t.Error("sin ruta debería dar vacío")
	}

	if !pingIOS("Success rate is 66 percent (2/3), round-trip min/avg/max = 1/2/4 ms") {
		t.Error("ping IOS con 66% sí respondió")
	}
	if pingIOS("Success rate is 0 percent (0/3)") {
		t.Error("ping IOS con 0% no respondió")
	}
	if !pingVPCS("84 bytes from 192.168.10.1 icmp_seq=1 ttl=255 time=9.1 ms") || pingVPCS("host (192.168.10.1) not reachable") {
		t.Error("pingVPCS")
	}
	if !dhcpConfigurado("auto eth0\n  iface eth0 inet dhcp\n") || dhcpConfigurado("#iface eth0 inet dhcp\niface eth0 inet static") {
		t.Error("dhcpConfigurado")
	}
}

func TestLineaPresente(t *testing.T) {
	config := "hostname R1\n!\nrouter ospf 1\n network 10.0.0.0   0.0.0.255 area 0\n!\nip route 0.0.0.0 0.0.0.0 10.0.0.2"
	casos := []struct {
		linea, modo string
		esta        bool
	}{
		{"network 10.0.0.0 0.0.0.255 area 0", "EXACTA", true}, // espacios de más no importan
		{"NETWORK 10.0.0.0 0.0.0.255 AREA 0", "EXACTA", true}, // ni mayúsculas
		{"network 10.0.0.0", "EXACTA", false},
		{"network 10.0.0.0", "EMPIEZA", true},
		{"area 0", "CONTIENE", true},
		{"area 1", "CONTIENE", false},
		{"ip route 0.0.0.0", "EMPIEZA", true},
	}
	for _, c := range casos {
		if lineaPresente(config, c.linea, c.modo) != c.esta {
			t.Errorf("lineaPresente(%q, %s) != %v", c.linea, c.modo, c.esta)
		}
	}
}

func TestCalificar(t *testing.T) {
	res := []domain.Resultado{{Peso: 30, Puntos: 30}, {Peso: 50, Puntos: 0}, {Peso: 20, Puntos: 20}}
	if c := Calificar(res); c != 50 {
		t.Errorf("Calificar = %d, quería 50", c)
	}
	if c := Calificar([]domain.Resultado{{Peso: 1, Puntos: 1}, {Peso: 2}}); c != 33 {
		t.Errorf("Calificar redondeo = %d, quería 33", c)
	}
	if Calificar(nil) != 0 {
		t.Error("sin criterios = 0")
	}
}

func TestFormatoRetraso(t *testing.T) {
	casos := map[time.Duration]string{
		30 * time.Second:                "menos de 1 min",
		45 * time.Minute:                "45 min",
		2 * time.Hour:                   "2 h",
		2*time.Hour + 15*time.Minute:    "2 h 15 min",
		3 * 24 * time.Hour:              "3 d",
		3*24*time.Hour + 4*time.Hour:    "3 d 4 h",
		26*time.Hour + 59*time.Minute:   "1 d 2 h",
		59*time.Minute + 59*time.Second: "59 min",
	}
	for d, quiere := range casos {
		if got := FormatoRetraso(d); got != quiere {
			t.Errorf("FormatoRetraso(%v) = %q, quería %q", d, got, quiere)
		}
	}
}

func TestValidarCriterio(t *testing.T) {
	nodos := map[string]domain.NodoPractica{
		"R1":  {Nombre: "R1", Tipo: "dynamips", Puertos: []string{"FastEthernet0/0", "FastEthernet1/0"}},
		"PC1": {Nombre: "PC1", Tipo: "vpcs", Puertos: []string{"Ethernet0"}},
		"SW1": {Nombre: "SW1", Tipo: "ethernet_switch", Puertos: []string{"Ethernet0", "Ethernet1"}},
	}
	buenos := []domain.Criterio{
		{Tipo: "PING", Peso: 10, Nodo: "PC1", Destino: "R1", Modo: "DEBE"},
		{Tipo: "PING", Peso: 10, Nodo: "R1", Destino: "8.8.8.8", Modo: "NO_DEBE"},
		{Tipo: "IP_INTERFAZ", Peso: 10, Nodo: "R1", Interfaz: "FastEthernet0/0", Valor: "192.168.1.1/24", Modo: "EXACTA"},
		{Tipo: "RUTA", Peso: 10, Nodo: "R1", Valor: "192.168.2.77/24", Modo: "OSPF"},
		{Tipo: "VLAN_PUERTO", Peso: 10, Nodo: "SW1", Interfaz: "Ethernet1", Valor: "20"},
		{Tipo: "TRUNK", Peso: 10, Nodo: "SW1", Interfaz: "Ethernet0"},
		{Tipo: "LINEA_CONFIG", Peso: 10, Nodo: "R1", Valor: "router ospf 1", Modo: "EXACTA"},
		{Tipo: "DHCP", Peso: 10, Nodo: "PC1", Valor: "10.0.0.9/24"},
	}
	for i := range buenos {
		if err := ValidarCriterio(&buenos[i], nodos); err != nil {
			t.Errorf("criterio %d: %v", i, err)
		}
	}
	if buenos[3].Valor != "192.168.2.0/24" || buenos[7].Valor != "10.0.0.0/24" {
		t.Errorf("las redes se normalizan: %s %s", buenos[3].Valor, buenos[7].Valor)
	}

	malos := []domain.Criterio{
		{Tipo: "PING", Peso: 0, Nodo: "PC1", Destino: "R1", Modo: "DEBE"},  // peso 0
		{Tipo: "PING", Peso: 10, Nodo: "PC9", Destino: "R1", Modo: "DEBE"}, // nodo que no existe
		{Tipo: "PING", Peso: 10, Nodo: "PC1", Destino: "R7", Modo: "DEBE"}, // destino que no existe
		{Tipo: "PROTOCOLO", Peso: 10, Nodo: "PC1", Modo: "OSPF"},           // no aplica a VPCS
		{Tipo: "IP_INTERFAZ", Peso: 10, Nodo: "R1", Interfaz: "Gi0/9", Valor: "192.168.1.1/24", Modo: "EXACTA"},
		{Tipo: "IP_INTERFAZ", Peso: 10, Nodo: "R1", Interfaz: "FastEthernet0/0", Valor: "192.168.1.1", Modo: "EXACTA"},
		{Tipo: "VLAN_PUERTO", Peso: 10, Nodo: "SW1", Interfaz: "Ethernet1", Valor: "5000"},
		{Tipo: "VECINOS_OSPF", Peso: 10, Nodo: "R1", Valor: "0"},
		{Tipo: "LINEA_CONFIG", Peso: 10, Nodo: "R1", Valor: "", Modo: "EXACTA"},
		{Tipo: "NADA", Peso: 10, Nodo: "R1"},
	}
	for i := range malos {
		if ValidarCriterio(&malos[i], nodos) == nil {
			t.Errorf("el criterio malo %d pasó", i)
		}
	}
}

// El mensaje para el alumno no debe llevar la respuesta (IPs, redes, VLAN, líneas)
func TestMensajeAlumnoSinRespuesta(t *testing.T) {
	criterios := []domain.Criterio{
		{Tipo: "PING", Nodo: "PC1", Destino: "192.168.2.10", Modo: "DEBE"},
		{Tipo: "IP_INTERFAZ", Nodo: "R1", Interfaz: "Fa0/0", Valor: "192.168.1.1/24"},
		{Tipo: "RUTA", Nodo: "R1", Valor: "192.168.2.0/24", Modo: "OSPF"},
		{Tipo: "VLAN_PUERTO", Nodo: "SW1", Interfaz: "Ethernet1", Valor: "20"},
		{Tipo: "LINEA_CONFIG", Nodo: "R1", Valor: "router ospf 1"},
	}
	for _, c := range criterios {
		m := MensajeAlumno(&c)
		for _, secreto := range []string{c.Valor, c.Destino, "ospf", "OSPF"} {
			if secreto != "" && secreto != c.Nodo && strings.Contains(m, secreto) {
				t.Errorf("%s: el mensaje %q revela %q", c.Tipo, m, secreto)
			}
		}
	}
}

// El .ghost de Dynamips no se guarda; lo demás queda igual
func TestSinGhost(t *testing.T) {
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	for nombre, contenido := range map[string]string{
		"project.gns3": `{"name": "x"}`,
		"project-files/dynamips/c7200-adventerprisek9-mz.124-24.T5.bin-512.ghost": strings.Repeat("\x00", 4096),
		"project-files/dynamips/abc/configs/i1_startup-config.cfg":                "hostname R1\n",
	} {
		w, _ := zw.Create(nombre)
		_, _ = w.Write([]byte(contenido))
	}
	_ = zw.Close()

	limpio, err := sinGhost(buf.Bytes())
	if err != nil {
		t.Fatal(err)
	}
	zr, err := zip.NewReader(bytes.NewReader(limpio), int64(len(limpio)))
	if err != nil {
		t.Fatal(err)
	}
	quedan := map[string]string{}
	for _, f := range zr.File {
		r, _ := f.Open()
		b, _ := io.ReadAll(r)
		quedan[f.Name] = string(b)
	}
	if len(quedan) != 2 || quedan["project.gns3"] != `{"name": "x"}` ||
		quedan["project-files/dynamips/abc/configs/i1_startup-config.cfg"] != "hostname R1\n" {
		t.Errorf("quedaron: %v", quedan)
	}
	if _, err := sinGhost([]byte("no es zip")); err == nil {
		t.Error("un archivo que no es zip debe dar error")
	}
}
