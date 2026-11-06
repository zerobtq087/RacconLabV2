package service

import (
	"errors"
	"strings"
	"testing"

	"raccoon-lab/backend/domain"
)

func TestSubred(t *testing.T) {
	casos := []struct {
		ip, mascara                                       string
		red, bc, primero, ultimo, masc, wild, clase, tipo string
		hosts                                             uint64
	}{
		{"192.168.1.130", "26", "192.168.1.128/26", "192.168.1.191", "192.168.1.129", "192.168.1.190", "255.255.255.192", "0.0.0.63", "C", "Privada (RFC 1918)", 62},
		{"192.168.1.130", "255.255.255.192", "192.168.1.128/26", "192.168.1.191", "192.168.1.129", "192.168.1.190", "255.255.255.192", "0.0.0.63", "C", "Privada (RFC 1918)", 62},
		{"10.20.30.40/8", "", "10.0.0.0/8", "10.255.255.255", "10.0.0.1", "10.255.255.254", "255.0.0.0", "0.255.255.255", "A", "Privada (RFC 1918)", 16777214},
		{"172.31.255.1", "/12", "172.16.0.0/12", "172.31.255.255", "172.16.0.1", "172.31.255.254", "255.240.0.0", "0.15.255.255", "B", "Privada (RFC 1918)", 1048574},
		{"172.32.0.1", "16", "172.32.0.0/16", "172.32.255.255", "172.32.0.1", "172.32.255.254", "255.255.0.0", "0.0.255.255", "B", "Pública", 65534},
		{"8.8.8.8", "32", "8.8.8.8/32", "8.8.8.8", "8.8.8.8", "8.8.8.8", "255.255.255.255", "0.0.0.0", "A", "Pública", 1},
		{"10.0.0.1", "31", "10.0.0.0/31", "10.0.0.1", "10.0.0.0", "10.0.0.1", "255.255.255.254", "0.0.0.1", "A", "Privada (RFC 1918)", 2},
		{"100.70.1.1", "10", "100.64.0.0/10", "100.127.255.255", "100.64.0.1", "100.127.255.254", "255.192.0.0", "0.63.255.255", "A", "Compartida CGNAT (RFC 6598)", 4194302},
		{"169.254.10.5", "16", "169.254.0.0/16", "169.254.255.255", "169.254.0.1", "169.254.255.254", "255.255.0.0", "0.0.255.255", "B", "Link-local (APIPA)", 65534},
		{"224.0.0.5", "4", "224.0.0.0/4", "239.255.255.255", "224.0.0.1", "239.255.255.254", "240.0.0.0", "15.255.255.255", "D (multicast)", "Multicast", 268435454},
	}
	for _, c := range casos {
		i, err := Subred(c.ip, c.mascara)
		if err != nil {
			t.Errorf("%s %s: %v", c.ip, c.mascara, err)
			continue
		}
		got := []string{i.Red, i.Broadcast, i.PrimerHost, i.UltimoHost, i.Mascara, i.Wildcard, i.Clase, i.Tipo}
		want := []string{c.red, c.bc, c.primero, c.ultimo, c.masc, c.wild, c.clase, c.tipo}
		if strings.Join(got, "|") != strings.Join(want, "|") || i.Hosts != c.hosts {
			t.Errorf("%s/%s\n got  %v hosts=%d\n want %v hosts=%d", c.ip, c.mascara, got, i.Hosts, want, c.hosts)
		}
	}

	i, _ := Subred("192.168.1.130", "26")
	if i.Binario.IP != "11000000.10101000.00000001.10000010" || i.Binario.Mascara != "11111111.11111111.11111111.11000000" {
		t.Errorf("binario: %+v", i.Binario)
	}
	if !i.Privada || i.Direcciones != 64 {
		t.Errorf("privada=%v direcciones=%d", i.Privada, i.Direcciones)
	}
}

func redes(f []domain.SubredCalc) string {
	s := []string{}
	for _, x := range f {
		s = append(s, x.Red)
	}
	return strings.Join(s, " ")
}

func TestDividir(t *testing.T) {
	casos := []struct {
		red, modo string
		cantidad  int
		redes     string
		hosts     uint64
	}{
		{"192.168.0.0/24", "subredes", 4, "192.168.0.0/26 192.168.0.64/26 192.168.0.128/26 192.168.0.192/26", 62},
		{"192.168.0.0/24", "subredes", 3, "192.168.0.0/26 192.168.0.64/26 192.168.0.128/26 192.168.0.192/26", 62}, // 3 → 4
		{"192.168.0.0/24", "hosts", 50, "192.168.0.0/26 192.168.0.64/26 192.168.0.128/26 192.168.0.192/26", 62},
		{"192.168.0.0/24", "hosts", 63, "192.168.0.0/25 192.168.0.128/25", 126},
		{"192.168.0.77/24", "hosts", 126, "192.168.0.0/25 192.168.0.128/25", 126}, // se usa la red
		{"10.0.0.0/16", "subredes", 1, "10.0.0.0/16", 65534},
	}
	for _, c := range casos {
		r, err := Dividir(&domain.SolicitudDivision{Red: c.red, Modo: c.modo, Cantidad: c.cantidad})
		if err != nil {
			t.Errorf("%v: %v", c, err)
			continue
		}
		if redes(r.Subredes) != c.redes || r.HostsPorRed != c.hosts {
			t.Errorf("%s %s %d: %s (hosts %d)", c.red, c.modo, c.cantidad, redes(r.Subredes), r.HostsPorRed)
		}
	}
	r, _ := Dividir(&domain.SolicitudDivision{Red: "10.0.0.0/8", Modo: "subredes", Cantidad: 5000})
	if r.Total != 8192 || len(r.Subredes) != 1024 || !strings.Contains(r.Nota, "primeras 1024") {
		t.Errorf("tabla grande: total=%d filas=%d nota=%q", r.Total, len(r.Subredes), r.Nota)
	}
}

func TestVLSM(t *testing.T) {
	// Ejemplo clásico: 192.168.1.0/24 con 100, 50, 25 y 2 hosts (en desorden)
	r, err := VLSM(&domain.SolicitudVLSM{Red: "192.168.1.0/24", Necesidades: []domain.Necesidad{
		{Nombre: "C", Hosts: 25}, {Nombre: "Enlace", Hosts: 2}, {Nombre: "A", Hosts: 100}, {Nombre: "B", Hosts: 50},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "192.168.1.0/25 192.168.1.128/26 192.168.1.192/27 192.168.1.224/30"
	if redes(r.Subredes) != want || r.Subredes[0].Nombre != "A" || r.Subredes[3].Nombre != "Enlace" {
		t.Fatalf("orden/redes: %s (%s primero)", redes(r.Subredes), r.Subredes[0].Nombre)
	}
	if r.Subredes[0].Desperdicio != 26 || r.Subredes[1].UltimoHost != "192.168.1.190" {
		t.Errorf("desperdicio A=%d, último de B=%s", r.Subredes[0].Desperdicio, r.Subredes[1].UltimoHost)
	}
	if r.Usadas != 228 || r.Libres != 28 || strings.Join(r.BloquesLibres, " ") != "192.168.1.228/30 192.168.1.232/29 192.168.1.240/28" {
		t.Errorf("usadas=%d libres=%d bloques=%v", r.Usadas, r.Libres, r.BloquesLibres)
	}

	if _, err := VLSM(&domain.SolicitudVLSM{Red: "192.168.1.0/24", Necesidades: []domain.Necesidad{{Hosts: 200}, {Hosts: 60}}}); err == nil ||
		!strings.Contains(err.Error(), "No alcanza") {
		t.Errorf("200+60 no caben en /24: %v", err)
	}
	if _, err := VLSM(&domain.SolicitudVLSM{Red: "192.168.1.0/24", Necesidades: []domain.Necesidad{{Nombre: "X", Hosts: 300}}}); err == nil ||
		!strings.Contains(err.Error(), "X necesita 300") {
		t.Errorf("300 hosts en /24: %v", err)
	}
}

func TestEnlaces(t *testing.T) {
	r, err := Enlaces(&domain.SolicitudEnlaces{Red: "10.0.0.0/28", Prefijo: 30})
	if err != nil {
		t.Fatal(err)
	}
	if r.Caben != 4 || len(r.Enlaces) != 4 || r.Enlaces[3].IPA != "10.0.0.13" || r.Enlaces[3].IPB != "10.0.0.14" || r.Enlaces[3].Broadcast != "10.0.0.15" {
		t.Errorf("/30: %+v", r.Enlaces)
	}
	r, _ = Enlaces(&domain.SolicitudEnlaces{Red: "10.0.0.0/28", Prefijo: 31, Cantidad: 3})
	if r.Caben != 8 || len(r.Enlaces) != 3 || r.Enlaces[2].Red != "10.0.0.4/31" || r.Enlaces[2].IPA != "10.0.0.4" || r.Enlaces[2].Broadcast != "" {
		t.Errorf("/31: %+v", r.Enlaces)
	}
	if _, err := Enlaces(&domain.SolicitudEnlaces{Red: "10.0.0.0/29", Prefijo: 30, Cantidad: 3}); err == nil {
		t.Error("3 enlaces /30 no caben en /29")
	}
}

func TestSumarizar(t *testing.T) {
	casos := []struct {
		redes   []string
		resumen string
		exacta  bool
		extra   uint64
	}{
		{[]string{"172.16.0.0/24", "172.16.1.0/24", "172.16.2.0/24", "172.16.3.0/24"}, "172.16.0.0/22", true, 0},
		{[]string{"192.168.1.0/24", "192.168.4.0/24"}, "192.168.0.0/21", false, 1536},
		{[]string{"10.1.0.0/16", "10.1.128.0/17"}, "10.1.0.0/16", true, 0}, // una dentro de otra
		{[]string{"192.168.10.0/24", "192.168.11.0/24", "192.168.12.0/24", "192.168.13.0/24"}, "192.168.8.0/21", false, 1024},
	}
	for _, c := range casos {
		r, err := Sumarizar(&domain.SolicitudSumarizacion{Redes: c.redes})
		if err != nil {
			t.Errorf("%v: %v", c.redes, err)
			continue
		}
		if r.Resumen != c.resumen || r.Exacta != c.exacta || r.DireccionesExtra != c.extra {
			t.Errorf("%v: %s exacta=%v extra=%d", c.redes, r.Resumen, r.Exacta, r.DireccionesExtra)
		}
	}
	r, _ := Sumarizar(&domain.SolicitudSumarizacion{Redes: []string{"172.16.0.0/24", "172.16.1.0/24", "172.16.2.0/24", "172.16.3.0/24"}})
	if r.Mascara != "255.255.252.0" || r.Wildcard != "0.0.3.255" {
		t.Errorf("máscara %s wildcard %s", r.Mascara, r.Wildcard)
	}
}

func TestConvertir(t *testing.T) {
	casos := []struct {
		tipo, valor, masc, wild string
		prefijo                 int
		hosts                   uint64
	}{
		{"prefijo", "/20", "255.255.240.0", "0.0.15.255", 20, 4094},
		{"prefijo", "0", "0.0.0.0", "255.255.255.255", 0, 4294967294},
		{"mascara", "255.255.255.224", "255.255.255.224", "0.0.0.31", 27, 30},
		{"mascara", "255.255.255.255", "255.255.255.255", "0.0.0.0", 32, 1},
		{"wildcard", "0.0.0.7", "255.255.255.248", "0.0.0.7", 29, 6},
	}
	for _, c := range casos {
		r, err := Convertir(&domain.SolicitudConversion{Tipo: c.tipo, Valor: c.valor})
		if err != nil {
			t.Errorf("%s %s: %v", c.tipo, c.valor, err)
			continue
		}
		if r.Prefijo != c.prefijo || r.Mascara != c.masc || r.Wildcard != c.wild || r.Hosts != c.hosts {
			t.Errorf("%s %s: %+v", c.tipo, c.valor, r)
		}
	}
	r, _ := Convertir(&domain.SolicitudConversion{Tipo: "prefijo", Valor: "26"})
	if r.MascaraBinaria != "11111111.11111111.11111111.11000000" {
		t.Errorf("binario: %s", r.MascaraBinaria)
	}
}

func TestErroresCalculadora(t *testing.T) {
	errores := []error{
		func() error { _, e := Subred("192.168.1.300", "24"); return e }(),
		func() error { _, e := Subred("192.168.1.1", "33"); return e }(),
		func() error { _, e := Subred("192.168.1.1", "255.0.255.0"); return e }(), // unos no seguidos
		func() error { _, e := Subred("192.168.1.1", ""); return e }(),
		func() error {
			_, e := Dividir(&domain.SolicitudDivision{Red: "192.168.1.0", Modo: "hosts", Cantidad: 10})
			return e
		}(),
		func() error {
			_, e := Dividir(&domain.SolicitudDivision{Red: "192.168.1.0/24", Modo: "subredes", Cantidad: 128})
			return e
		}(),
		func() error {
			_, e := Dividir(&domain.SolicitudDivision{Red: "192.168.1.0/24", Modo: "hosts", Cantidad: 300})
			return e
		}(),
		func() error { _, e := Enlaces(&domain.SolicitudEnlaces{Red: "10.0.0.0/24", Prefijo: 29}); return e }(),
		func() error {
			_, e := Sumarizar(&domain.SolicitudSumarizacion{Redes: []string{"10.0.0.0/8"}})
			return e
		}(),
		func() error {
			_, e := Convertir(&domain.SolicitudConversion{Tipo: "wildcard", Valor: "0.0.255.0"})
			return e
		}(),
	}
	for i, err := range errores {
		var ev *ErrValidacion
		if !errors.As(err, &ev) {
			t.Errorf("caso %d: se esperaba un error de validación y salió %v", i, err)
		}
	}
}
