package service

import (
	"cmp"
	"fmt"
	"math/bits"
	"net/netip"
	"slices"
	"strconv"
	"strings"

	"raccoon-lab/backend/domain"
)

// =====================================================================
// Calculadora de redes IPv4. Todo es cálculo puro (sin base de datos):
// las direcciones se manejan como números de 32 bits (aNumero/aIP están
// en topologia.go) y los tamaños como uint64 para que /0 no se desborde.
// =====================================================================

const maxFilas = 1024 // máximo de filas que se regresan en una tabla

// tam: direcciones de una red con ese prefijo (/24 → 256)
func tam(prefijo int) uint64 { return uint64(1) << (32 - prefijo) }

// usables: hosts que se pueden usar (/31 → 2 en enlaces punto a punto, RFC 3021; /32 → 1)
func usables(prefijo int) uint64 {
	switch prefijo {
	case 32:
		return 1
	case 31:
		return 2
	}
	return tam(prefijo) - 2
}

// binario: 192.168.1.1 → 11000000.10101000.00000001.00000001
func binario(a netip.Addr) string {
	b := a.As4()
	return fmt.Sprintf("%08b.%08b.%08b.%08b", b[0], b[1], b[2], b[3])
}

func leerIP(texto, que string) (netip.Addr, error) {
	a, err := netip.ParseAddr(strings.TrimSpace(texto))
	if err != nil || !a.Is4() {
		return netip.Addr{}, invalido("%s: \"%s\" no es una IPv4 válida (ej. 192.168.1.10)", que, texto)
	}
	return a, nil
}

// bitsDeMascara: 255.255.255.0 → 24. La máscara debe ser unos seguidos de ceros.
func bitsDeMascara(a netip.Addr) (int, bool) {
	n := aNumero(a)
	unos := bits.LeadingZeros32(^n)
	return unos, n == ^uint32(0)<<(32-unos) || unos == 32
}

// leerMascara acepta "24", "/24" o "255.255.255.0"
func leerMascara(texto string) (int, error) {
	t := strings.TrimPrefix(strings.TrimSpace(texto), "/")
	if strings.Contains(t, ".") {
		a, err := leerIP(t, "Máscara")
		if err != nil {
			return 0, err
		}
		p, ok := bitsDeMascara(a)
		if !ok {
			return 0, invalido("La máscara %s no es válida: debe tener los unos seguidos (ej. 255.255.255.0)", t)
		}
		return p, nil
	}
	p, err := strconv.Atoi(t)
	if err != nil || p < 0 || p > 32 {
		return 0, invalido("El prefijo \"%s\" no es válido: debe ser de 0 a 32 (o una máscara como 255.255.255.0)", texto)
	}
	return p, nil
}

// leerRed acepta "192.168.1.0/24" o "192.168.1.0/255.255.255.0" y regresa la red
// (si traía bits de host, se usa la red a la que pertenece: 192.168.1.5/24 → 192.168.1.0/24)
func leerRed(texto, que string) (netip.Prefix, error) {
	ip, masc, ok := strings.Cut(strings.TrimSpace(texto), "/")
	if !ok {
		return netip.Prefix{}, invalido("%s: escribe la red con su prefijo, ej. 192.168.1.0/24", que)
	}
	a, err := leerIP(ip, que)
	if err != nil {
		return netip.Prefix{}, err
	}
	p, err := leerMascara(masc)
	if err != nil {
		return netip.Prefix{}, err
	}
	return netip.PrefixFrom(a, p).Masked(), nil
}

// fila arma una fila de tabla para una subred
func fila(red netip.Prefix) domain.SubredCalc {
	i := infoDe(red.Addr(), red.Bits())
	return domain.SubredCalc{Red: i.Red, Mascara: i.Mascara, PrimerHost: i.PrimerHost,
		UltimoHost: i.UltimoHost, Broadcast: i.Broadcast, Hosts: i.Hosts}
}

// ---------------------------------------------------------------------
// 1) Subred: IP + prefijo o máscara
// ---------------------------------------------------------------------

// Subred: si mascara viene vacía, ip puede traer el prefijo ("192.168.1.10/24")
func Subred(ip, mascara string) (*domain.InfoSubred, error) {
	if mascara == "" {
		if antes, despues, ok := strings.Cut(ip, "/"); ok {
			ip, mascara = antes, despues
		} else {
			return nil, invalido("Escribe el prefijo o la máscara (ej. 24 o 255.255.255.0)")
		}
	}
	a, err := leerIP(ip, "IP")
	if err != nil {
		return nil, err
	}
	p, err := leerMascara(mascara)
	if err != nil {
		return nil, err
	}
	i := infoDe(a, p)
	return &i, nil
}

func infoDe(a netip.Addr, p int) domain.InfoSubred {
	n := uint64(aNumero(a))
	red := n &^ (tam(p) - 1)
	bc := red + tam(p) - 1
	primero, ultimo := red+1, bc-1
	switch p {
	case 32:
		primero, ultimo = red, red
	case 31:
		primero, ultimo = red, bc
	}
	ip := func(x uint64) netip.Addr { return aIP(uint32(x)) }
	clase, tipo, privada := clasificar(a)

	return domain.InfoSubred{
		IP:          a.String(),
		Red:         fmt.Sprintf("%s/%d", ip(red), p),
		Broadcast:   ip(bc).String(),
		PrimerHost:  ip(primero).String(),
		UltimoHost:  ip(ultimo).String(),
		Hosts:       usables(p),
		Direcciones: tam(p),
		Prefijo:     p,
		Mascara:     mascara(p),
		Wildcard:    comodin(p),
		Clase:       clase,
		Tipo:        tipo,
		Privada:     privada,
		Binario: domain.Binario{
			IP:        binario(a),
			Mascara:   binario(aIP(^uint32(0) << (32 - p))),
			Red:       binario(ip(red)),
			Broadcast: binario(ip(bc)),
		},
	}
}

// clasificar: clase por el primer octeto y tipo de dirección (RFC 1918, 6598...)
func clasificar(a netip.Addr) (clase, tipo string, privada bool) {
	o := a.As4()[0]
	switch {
	case o < 128:
		clase = "A"
	case o < 192:
		clase = "B"
	case o < 224:
		clase = "C"
	case o < 240:
		clase = "D (multicast)"
	default:
		clase = "E (reservada)"
	}

	en := func(red string) bool { return netip.MustParsePrefix(red).Contains(a) }
	switch {
	case en("10.0.0.0/8"), en("172.16.0.0/12"), en("192.168.0.0/16"):
		return clase, "Privada (RFC 1918)", true
	case a == netip.MustParseAddr("255.255.255.255"):
		return clase, "Broadcast limitado", false
	case en("127.0.0.0/8"):
		return clase, "Loopback", false
	case en("169.254.0.0/16"):
		return clase, "Link-local (APIPA)", false
	case en("100.64.0.0/10"):
		return clase, "Compartida CGNAT (RFC 6598)", false
	case en("0.0.0.0/8"):
		return clase, "Esta red (no se usa como destino)", false
	case en("224.0.0.0/4"):
		return clase, "Multicast", false
	case en("240.0.0.0/4"):
		return clase, "Reservada", false
	}
	return clase, "Pública", false
}

// ---------------------------------------------------------------------
// 2) Dividir una red en subredes iguales (por número de subredes o de hosts)
// ---------------------------------------------------------------------

func Dividir(s *domain.SolicitudDivision) (*domain.ResultadoDivision, error) {
	red, err := leerRed(s.Red, "Red")
	if err != nil {
		return nil, err
	}
	if s.Cantidad < 1 {
		return nil, invalido("La cantidad debe ser mayor que 0")
	}

	var nuevo int
	switch s.Modo {
	case "subredes":
		// bits que se toman prestados: los necesarios para tener al menos N subredes
		nuevo = red.Bits() + bits.Len(uint(s.Cantidad-1))
		if nuevo > 30 {
			return nil, invalido("No se puede: %s en %d subredes daría redes /%d sin hosts útiles", red, s.Cantidad, nuevo)
		}
	case "hosts":
		// bits de host para que quepan los hosts + red + broadcast
		nuevo = 32 - bits.Len64(uint64(s.Cantidad)+1)
		if nuevo < red.Bits() {
			return nil, invalido("No caben: %s da como máximo %d hosts por red", red, usables(red.Bits()))
		}
	default:
		return nil, invalido("Elige dividir por número de subredes o de hosts")
	}

	total := uint64(1) << (nuevo - red.Bits())
	r := &domain.ResultadoDivision{
		Red: red.String(), NuevoPrefijo: nuevo, Mascara: mascara(nuevo),
		Total: int(min(total, 1<<30)), HostsPorRed: usables(nuevo), Subredes: []domain.SubredCalc{},
	}
	inicio := uint64(aNumero(red.Addr()))
	for k := uint64(0); k < total && k < maxFilas; k++ {
		r.Subredes = append(r.Subredes, fila(netip.PrefixFrom(aIP(uint32(inicio+k*tam(nuevo))), nuevo)))
	}

	if s.Modo == "subredes" && total > uint64(s.Cantidad) {
		r.Nota = fmt.Sprintf("Pediste %d subredes: con %d bits prestados salen %d (siempre es potencia de 2); las demás quedan libres.",
			s.Cantidad, nuevo-red.Bits(), total)
	}
	if total > maxFilas {
		r.Nota += fmt.Sprintf(" Se muestran las primeras %d de %d.", maxFilas, total)
	}
	return r, nil
}

// ---------------------------------------------------------------------
// 3) VLSM: subredes de distinto tamaño, de la más grande a la más chica
// ---------------------------------------------------------------------

func VLSM(s *domain.SolicitudVLSM) (*domain.ResultadoVLSM, error) {
	red, err := leerRed(s.Red, "Red")
	if err != nil {
		return nil, err
	}
	if len(s.Necesidades) == 0 {
		return nil, invalido("Agrega al menos una subred con su número de hosts")
	}
	if len(s.Necesidades) > 200 {
		return nil, invalido("Máximo 200 subredes")
	}

	type pedido struct {
		domain.Necesidad
		prefijo int
	}
	pedidos := make([]pedido, len(s.Necesidades))
	for i, n := range s.Necesidades {
		n.Nombre = strings.TrimSpace(n.Nombre)
		if n.Nombre == "" {
			n.Nombre = fmt.Sprintf("Red %d", i+1)
		}
		if n.Hosts < 1 {
			return nil, invalido("%s: el número de hosts debe ser mayor que 0", n.Nombre)
		}
		p := 32 - bits.Len64(uint64(n.Hosts)+1) // la red más chica donde caben
		if p < red.Bits() {
			return nil, invalido("%s necesita %d hosts y %s solo tiene %d", n.Nombre, n.Hosts, red, usables(red.Bits()))
		}
		pedidos[i] = pedido{n, min(p, 30)}
	}
	// De mayor a menor: así cada subred queda alineada y no hay traslapes
	slices.SortStableFunc(pedidos, func(a, b pedido) int { return cmp.Compare(b.Hosts, a.Hosts) })

	inicio := uint64(aNumero(red.Addr()))
	fin := inicio + tam(red.Bits())
	siguiente := inicio
	var necesarias uint64
	for _, p := range pedidos {
		necesarias += tam(p.prefijo)
	}
	if necesarias > tam(red.Bits()) {
		return nil, invalido("No alcanza: las subredes necesitan %d direcciones y %s tiene %d", necesarias, red, tam(red.Bits()))
	}

	r := &domain.ResultadoVLSM{Red: red.String(), Total: tam(red.Bits()), Subredes: []domain.SubredCalc{}}
	for _, p := range pedidos {
		f := fila(netip.PrefixFrom(aIP(uint32(siguiente)), p.prefijo))
		f.Nombre, f.HostsPedidos, f.Desperdicio = p.Nombre, p.Hosts, usables(p.prefijo)-uint64(p.Hosts)
		r.Subredes = append(r.Subredes, f)
		siguiente += tam(p.prefijo)
	}
	r.Usadas = siguiente - inicio
	r.Libres = r.Total - r.Usadas
	r.PorcentajeLibre = float64(r.Libres) * 100 / float64(r.Total)
	r.BloquesLibres = bloquesCIDR(siguiente, fin)
	return r, nil
}

// bloquesCIDR parte el rango [desde, hasta) en las redes CIDR más grandes posibles
func bloquesCIDR(desde, hasta uint64) []string {
	bloques := []string{}
	for desde < hasta {
		// La red más grande que puede empezar en "desde" (por su alineación)...
		p := 32 - min(bits.TrailingZeros64(desde), 32)
		// ...y que no se pase del final
		for tam(p) > hasta-desde {
			p++
		}
		bloques = append(bloques, fmt.Sprintf("%s/%d", aIP(uint32(desde)), p))
		desde += tam(p)
	}
	return bloques
}

// ---------------------------------------------------------------------
// 4) Enlaces punto a punto (/30 con red y broadcast, o /31 sin ellos)
// ---------------------------------------------------------------------

func Enlaces(s *domain.SolicitudEnlaces) (*domain.ResultadoEnlaces, error) {
	red, err := leerRed(s.Red, "Red")
	if err != nil {
		return nil, err
	}
	if s.Prefijo != 30 && s.Prefijo != 31 {
		return nil, invalido("Los enlaces punto a punto son /30 o /31")
	}
	if red.Bits() > s.Prefijo {
		return nil, invalido("%s es más chica que un enlace /%d", red, s.Prefijo)
	}
	caben := tam(red.Bits()) / tam(s.Prefijo)
	if s.Cantidad < 0 || uint64(s.Cantidad) > caben {
		return nil, invalido("En %s caben %d enlaces /%d", red, caben, s.Prefijo)
	}
	cuantos := caben
	if s.Cantidad > 0 {
		cuantos = uint64(s.Cantidad)
	}

	r := &domain.ResultadoEnlaces{Red: red.String(), Prefijo: s.Prefijo, Mascara: mascara(s.Prefijo),
		Caben: int(min(caben, 1<<30)), Enlaces: []domain.EnlaceP2P{}}
	inicio := uint64(aNumero(red.Addr()))
	for k := uint64(0); k < cuantos && k < maxFilas; k++ {
		base := inicio + k*tam(s.Prefijo)
		e := domain.EnlaceP2P{Red: fmt.Sprintf("%s/%d", aIP(uint32(base)), s.Prefijo)}
		if s.Prefijo == 30 {
			e.IPA, e.IPB, e.Broadcast = aIP(uint32(base+1)).String(), aIP(uint32(base+2)).String(), aIP(uint32(base+3)).String()
		} else {
			e.IPA, e.IPB = aIP(uint32(base)).String(), aIP(uint32(base+1)).String()
		}
		r.Enlaces = append(r.Enlaces, e)
	}
	if s.Prefijo == 31 {
		r.Nota = "Con /31 (RFC 3021) no hay red ni broadcast: las 2 direcciones son de los routers. Ahorra la mitad que /30."
	}
	if cuantos > maxFilas {
		r.Nota += fmt.Sprintf(" Se muestran los primeros %d de %d.", maxFilas, cuantos)
	}
	return r, nil
}

// ---------------------------------------------------------------------
// 5) Sumarización (superred): la red más chica que contiene a todas
// ---------------------------------------------------------------------

func Sumarizar(s *domain.SolicitudSumarizacion) (*domain.ResultadoSumarizacion, error) {
	if len(s.Redes) < 2 {
		return nil, invalido("Escribe al menos 2 redes para sumarizar")
	}
	if len(s.Redes) > 256 {
		return nil, invalido("Máximo 256 redes")
	}

	type rango struct{ desde, hasta uint64 } // [desde, hasta]
	rangos := []rango{}
	r := &domain.ResultadoSumarizacion{Redes: []string{}}
	for i, texto := range s.Redes {
		red, err := leerRed(texto, fmt.Sprintf("Red %d", i+1))
		if err != nil {
			return nil, err
		}
		r.Redes = append(r.Redes, red.String())
		d := uint64(aNumero(red.Addr()))
		rangos = append(rangos, rango{d, d + tam(red.Bits()) - 1})
	}

	// La superred: los bits en común entre la dirección más baja y la más alta
	bajo := slices.MinFunc(rangos, func(a, b rango) int { return cmp.Compare(a.desde, b.desde) }).desde
	alto := slices.MaxFunc(rangos, func(a, b rango) int { return cmp.Compare(a.hasta, b.hasta) }).hasta
	p := bits.LeadingZeros32(uint32(bajo) ^ uint32(alto))
	resumen := netip.PrefixFrom(aIP(uint32(bajo)), p).Masked()
	r.Resumen, r.Mascara, r.Wildcard = resumen.String(), mascara(p), comodin(p)

	// ¿Cubre direcciones de más? Se suman las redes sin contar dos veces lo que se encima
	slices.SortFunc(rangos, func(a, b rango) int { return cmp.Compare(a.desde, b.desde) })
	var cubiertas uint64
	actual := rangos[0]
	for _, x := range rangos[1:] {
		if x.desde <= actual.hasta+1 {
			actual.hasta = max(actual.hasta, x.hasta)
			continue
		}
		cubiertas += actual.hasta - actual.desde + 1
		actual = x
	}
	cubiertas += actual.hasta - actual.desde + 1
	r.DireccionesExtra = tam(p) - cubiertas
	r.Exacta = r.DireccionesExtra == 0
	return r, nil
}

// ---------------------------------------------------------------------
// 6) Conversión entre prefijo, máscara y wildcard
// ---------------------------------------------------------------------

func Convertir(s *domain.SolicitudConversion) (*domain.ResultadoConversion, error) {
	valor := strings.TrimSpace(s.Valor)
	var p int
	switch s.Tipo {
	case "prefijo":
		n, err := strconv.Atoi(strings.TrimPrefix(valor, "/"))
		if err != nil || n < 0 || n > 32 {
			return nil, invalido("El prefijo debe ser un número de 0 a 32 (ej. /24)")
		}
		p = n
	case "mascara":
		a, err := leerIP(valor, "Máscara")
		if err != nil {
			return nil, err
		}
		var ok bool
		if p, ok = bitsDeMascara(a); !ok {
			return nil, invalido("La máscara %s no es válida: debe tener los unos seguidos (ej. 255.255.240.0)", valor)
		}
	case "wildcard":
		a, err := leerIP(valor, "Wildcard")
		if err != nil {
			return nil, err
		}
		var ok bool
		if p, ok = bitsDeMascara(aIP(^aNumero(a))); !ok {
			return nil, invalido("La wildcard %s no es válida: debe tener los ceros seguidos (ej. 0.0.15.255)", valor)
		}
	default:
		return nil, invalido("Elige qué vas a convertir: prefijo, máscara o wildcard")
	}
	return &domain.ResultadoConversion{
		Prefijo: p, Mascara: mascara(p), Wildcard: comodin(p),
		MascaraBinaria: binario(aIP(^uint32(0) << (32 - p))),
		Direcciones:    tam(p), Hosts: usables(p),
	}, nil
}
