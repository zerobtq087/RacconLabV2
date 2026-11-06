package domain

// ---------------------------------------------------------------------
// Calculadora de redes (igual que models/Calculadora.ts)
// ---------------------------------------------------------------------

// InfoSubred: todo sobre una IP con su prefijo o máscara
type InfoSubred struct {
	IP          string  `json:"ip"`
	Red         string  `json:"red"` // ej. 192.168.1.128/26
	Broadcast   string  `json:"broadcast"`
	PrimerHost  string  `json:"primer_host"`
	UltimoHost  string  `json:"ultimo_host"`
	Hosts       uint64  `json:"hosts"`       // hosts que se pueden usar
	Direcciones uint64  `json:"direcciones"` // total de la red
	Prefijo     int     `json:"prefijo"`
	Mascara     string  `json:"mascara"`
	Wildcard    string  `json:"wildcard"`
	Clase       string  `json:"clase"` // A, B, C, D o E
	Tipo        string  `json:"tipo"`  // Privada (RFC 1918), Pública, Loopback...
	Privada     bool    `json:"privada"`
	Binario     Binario `json:"binario"`
}

// Binario: las direcciones en bits (8.8.8.8 → 00001000.00001000...)
type Binario struct {
	IP        string `json:"ip"`
	Mascara   string `json:"mascara"`
	Red       string `json:"red"`
	Broadcast string `json:"broadcast"`
}

// SubredCalc: una fila de las tablas (dividir y VLSM)
type SubredCalc struct {
	Nombre       string `json:"nombre,omitempty"`        // VLSM
	HostsPedidos int    `json:"hosts_pedidos,omitempty"` // VLSM
	Red          string `json:"red"`
	Mascara      string `json:"mascara"`
	PrimerHost   string `json:"primer_host"`
	UltimoHost   string `json:"ultimo_host"`
	Broadcast    string `json:"broadcast"`
	Hosts        uint64 `json:"hosts"`
	Desperdicio  uint64 `json:"desperdicio,omitempty"` // VLSM: hosts que sobran en esa subred
}

type SolicitudDivision struct {
	Red      string `json:"red"`      // ej. 192.168.0.0/24
	Modo     string `json:"modo"`     // subredes | hosts
	Cantidad int    `json:"cantidad"` // número de subredes o de hosts por subred
}

type ResultadoDivision struct {
	Red          string       `json:"red"`
	NuevoPrefijo int          `json:"nuevo_prefijo"`
	Mascara      string       `json:"mascara"`
	Total        int          `json:"total"` // subredes que salen
	HostsPorRed  uint64       `json:"hosts_por_red"`
	Subredes     []SubredCalc `json:"subredes"` // máximo 1024 en la lista
	Nota         string       `json:"nota"`
}

type Necesidad struct {
	Nombre string `json:"nombre"`
	Hosts  int    `json:"hosts"`
}

type SolicitudVLSM struct {
	Red         string      `json:"red"`
	Necesidades []Necesidad `json:"necesidades"`
}

type ResultadoVLSM struct {
	Red             string       `json:"red"`
	Subredes        []SubredCalc `json:"subredes"` // de mayor a menor
	Total           uint64       `json:"total"`    // direcciones de la red
	Usadas          uint64       `json:"usadas"`
	Libres          uint64       `json:"libres"`
	PorcentajeLibre float64      `json:"porcentaje_libre"`
	BloquesLibres   []string     `json:"bloques_libres"` // lo que sobra, en redes CIDR
}

type SolicitudEnlaces struct {
	Red      string `json:"red"`
	Prefijo  int    `json:"prefijo"`  // 30 o 31
	Cantidad int    `json:"cantidad"` // 0 = todos los que quepan
}

type EnlaceP2P struct {
	Red       string `json:"red"`
	IPA       string `json:"ip_a"`
	IPB       string `json:"ip_b"`
	Broadcast string `json:"broadcast,omitempty"` // /31 no tiene
}

type ResultadoEnlaces struct {
	Red     string      `json:"red"`
	Prefijo int         `json:"prefijo"`
	Mascara string      `json:"mascara"`
	Caben   int         `json:"caben"` // enlaces que caben en la red
	Enlaces []EnlaceP2P `json:"enlaces"`
	Nota    string      `json:"nota"`
}

type SolicitudSumarizacion struct {
	Redes []string `json:"redes"`
}

type ResultadoSumarizacion struct {
	Redes            []string `json:"redes"`   // ya normalizadas
	Resumen          string   `json:"resumen"` // la superred
	Mascara          string   `json:"mascara"`
	Wildcard         string   `json:"wildcard"`
	Exacta           bool     `json:"exacta"`            // la superred no incluye direcciones de más
	DireccionesExtra uint64   `json:"direcciones_extra"` // direcciones que cubre de más
}

type SolicitudConversion struct {
	Tipo  string `json:"tipo"` // prefijo | mascara | wildcard
	Valor string `json:"valor"`
}

type ResultadoConversion struct {
	Prefijo        int    `json:"prefijo"`
	Mascara        string `json:"mascara"`
	Wildcard       string `json:"wildcard"`
	MascaraBinaria string `json:"mascara_binaria"`
	Direcciones    uint64 `json:"direcciones"`
	Hosts          uint64 `json:"hosts"`
}
