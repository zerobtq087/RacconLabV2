package service

import (
	"errors"
	"strings"
	"testing"

	"raccoon-lab/backend/domain"
)

const dirTopologias = "../../opt/container-environment/production-system/raccoon.lab/gns3/volumes/ansible/topologies"

func cargar(t *testing.T, plantilla string) *domain.Topologia {
	t.Helper()
	topo, err := CargarTopologia(dirTopologias, Plantillas[plantilla])
	if err != nil {
		t.Fatal(err)
	}
	return topo
}

func nodo(t *testing.T, topo *domain.Topologia, nombre string) *domain.Nodo {
	t.Helper()
	for i := range topo.Nodes {
		if topo.Nodes[i].Name == nombre {
			return &topo.Nodes[i]
		}
	}
	t.Fatalf("no existe el nodo %s", nombre)
	return nil
}

func ip(t *testing.T, topo *domain.Topologia, router, iface string) string {
	t.Helper()
	for _, i := range nodo(t, topo, router).Interfaces {
		if i.Name == iface {
			return i.IP + " " + i.Netmask
		}
	}
	t.Fatalf("%s no tiene %s", router, iface)
	return ""
}

func resolver(t *testing.T, plantilla string, s domain.SolicitudDespliegue) (*domain.Topologia, error) {
	t.Helper()
	if err := ValidarSolicitud(&s); err != nil {
		return nil, err
	}
	topo := cargar(t, plantilla)
	return topo, ResolverTopologia(topo, &s)
}

func TestBasica(t *testing.T) {
	topo, err := resolver(t, "basica", domain.SolicitudDespliegue{
		NombreLab: "Lab_Basico", Subnet: "192.168.1.0/24", RoutingProtocol: "rip",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ip(t, topo, "R1", "FastEthernet0/0"); got != "192.168.1.1 255.255.255.0" {
		t.Errorf("R1 Fa0/0 = %s", got)
	}
	h := nodo(t, topo, "Host02")
	if h.IP != "192.168.1.20" || h.Gateway != "192.168.1.1" || h.Prefix != 24 {
		t.Errorf("Host02 = %s/%d gw %s", h.IP, h.Prefix, h.Gateway)
	}
	if topo.ProjectName != "Lab_Basico" || topo.ProjectSettings.RoutingProtocol != "rip" {
		t.Errorf("project_settings mal: %+v", topo.ProjectSettings)
	}
}

func TestIntermedia(t *testing.T) {
	topo, err := resolver(t, "intermedia", domain.SolicitudDespliegue{
		NombreLab: "Lab_Intermedio", Subnet: "192.168.0.0/16", SerialSubnet: "10.0.0.0/24", RoutingProtocol: "ospf",
	})
	if err != nil {
		t.Fatal(err)
	}
	// Enlaces /30 consecutivos: R2 es .1 y cada router de borde .2
	casos := map[string]string{
		"R2 Serial2/0":       "10.0.0.1 255.255.255.252",
		"R1 Serial2/0":       "10.0.0.2 255.255.255.252",
		"R2 Serial2/3":       "10.0.0.13 255.255.255.252",
		"R5 Serial2/3":       "10.0.0.14 255.255.255.252",
		"R1 FastEthernet1/0": "192.168.0.1 255.255.255.0", // lan_e es la primera LAN
		"R3 FastEthernet1/0": "192.168.1.1 255.255.255.0", // lan_g
		"R4 FastEthernet1/0": "192.168.2.1 255.255.255.0", // lan_f
	}
	for donde, esperado := range casos {
		r, i, _ := strings.Cut(donde, " ")
		if got := ip(t, topo, r, i); got != esperado {
			t.Errorf("%s = %s, se esperaba %s", donde, got, esperado)
		}
	}
	h := nodo(t, topo, "Host02") // en lan_f, detrás de R4
	if h.IP != "192.168.2.10" || h.Gateway != "192.168.2.1" {
		t.Errorf("Host02 = %s gw %s", h.IP, h.Gateway)
	}
	if n := len(nodo(t, topo, "R2").Redes); n != 4 {
		t.Errorf("R2 debe anunciar 4 redes y anuncia %d", n)
	}
}

func TestAvanzadaGatewaysCoinciden(t *testing.T) {
	topo, err := resolver(t, "avanzada", domain.SolicitudDespliegue{
		NombreLab: "Lab_VLAN", Subnet: "192.168.0.0/16", SerialSubnet: "10.0.0.0/24", RoutingProtocol: "ospf",
		Vlans: []domain.Vlan{
			{ID: 10, Subnet: "192.168.10.0/24", Gateway: "192.168.10.1"},
			{ID: 20, Subnet: "192.168.20.0/24", Gateway: "192.168.20.1"},
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if got := ip(t, topo, "R1", "FastEthernet1/0.10"); got != "192.168.10.1 255.255.255.0" {
		t.Errorf("R1 Fa1/0.10 = %s", got)
	}
	if p := nodo(t, topo, "R1").Parents; len(p) != 1 || p[0] != "FastEthernet1/0" {
		t.Errorf("R1 debe encender FastEthernet1/0, parents = %v", p)
	}
	// El host de lan_d y su gateway (R4) quedan en la MISMA red (en el original no)
	h5, r4 := nodo(t, topo, "Host05"), ip(t, topo, "R4", "FastEthernet1/0")
	if h5.Gateway+" 255.255.255.0" != r4 || !strings.HasPrefix(h5.IP, "192.168.0.") {
		t.Errorf("Host05 %s gw %s, pero R4 Fa1/0 = %s", h5.IP, h5.Gateway, r4)
	}
	if h3 := nodo(t, topo, "Host03"); h3.IP != "192.168.20.10" || h3.Gateway != "192.168.20.1" {
		t.Errorf("Host03 = %s gw %s", h3.IP, h3.Gateway)
	}
}

func TestErrores(t *testing.T) {
	base := domain.SolicitudDespliegue{NombreLab: "Lab_X", Subnet: "192.168.0.0/16", SerialSubnet: "10.0.0.0/24", RoutingProtocol: "ospf"}
	casos := []struct {
		nombre, plantilla, contiene string
		cambiar                     func(*domain.SolicitudDespliegue)
	}{
		{"nombre con espacios", "basica", "nombre del laboratorio", func(s *domain.SolicitudDespliegue) { s.NombreLab = "mi lab" }},
		{"nombre con comillas", "basica", "nombre del laboratorio", func(s *domain.SolicitudDespliegue) { s.NombreLab = "lab';rm" }},
		{"protocolo", "basica", "RIP u OSPF", func(s *domain.SolicitudDespliegue) { s.RoutingProtocol = "eigrp" }},
		{"área negativa", "basica", "área OSPF", func(s *domain.SolicitudDespliegue) { s.OspfArea = -1 }},
		{"CIDR inválido", "basica", "no es una red IPv4", func(s *domain.SolicitudDespliegue) { s.Subnet = "192.168.1.300/24" }},
		{"no es red", "basica", "usa 192.168.1.0/24", func(s *domain.SolicitudDespliegue) { s.Subnet = "192.168.1.5/24" }},
		{"serial chica", "intermedia", "no alcanza", func(s *domain.SolicitudDespliegue) { s.SerialSubnet = "10.0.0.0/29" }},
		{"sin serial", "intermedia", "enlaces seriales", func(s *domain.SolicitudDespliegue) { s.SerialSubnet = "" }},
		{"LAN chica", "intermedia", "no alcanza", func(s *domain.SolicitudDespliegue) { s.Subnet = "192.168.0.0/23" }},
		{"falta VLAN", "avanzada", "VLAN 10", func(s *domain.SolicitudDespliegue) {}},
		{"VLAN fuera de rango", "avanzada", "1 a 4094", func(s *domain.SolicitudDespliegue) {
			s.Vlans = []domain.Vlan{{ID: 5000, Subnet: "192.168.10.0/24", Gateway: "192.168.10.1"}}
		}},
		{"gateway fuera", "avanzada", "no es un host", func(s *domain.SolicitudDespliegue) {
			s.Vlans = []domain.Vlan{{ID: 10, Subnet: "192.168.10.0/24", Gateway: "192.168.11.1"}}
		}},
		{"redes encimadas", "avanzada", "se enciman", func(s *domain.SolicitudDespliegue) {
			s.Vlans = []domain.Vlan{
				{ID: 10, Subnet: "192.168.0.0/24", Gateway: "192.168.0.1"}, // choca con lan_d
				{ID: 20, Subnet: "192.168.20.0/24", Gateway: "192.168.20.1"},
			}
		}},
	}
	for _, c := range casos {
		t.Run(c.nombre, func(t *testing.T) {
			s := base
			c.cambiar(&s)
			_, err := resolver(t, c.plantilla, s)
			var ev *ErrValidacion
			if !errors.As(err, &ev) || !strings.Contains(err.Error(), c.contiene) {
				t.Errorf("error = %v; se esperaba uno que dijera %q", err, c.contiene)
			}
		})
	}
}
