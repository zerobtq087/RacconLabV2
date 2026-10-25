package domain

import "time"

// Despliegue: avance de un despliegue de topología (igual que models/Despliegue.ts → EstadoDespliegue)
type Despliegue struct {
	ID         string    `gorm:"column:ID" json:"id"`
	Origen     string    `gorm:"column:ORIGEN" json:"origen"`
	Proyecto   string    `gorm:"column:PROYECTO" json:"proyecto"`
	Estado     string    `gorm:"column:ESTADO" json:"estado"`
	Porcentaje int       `gorm:"column:PORCENTAJE" json:"porcentaje"`
	Paso       string    `gorm:"column:PASO" json:"paso"`
	Mensaje    string    `gorm:"column:MENSAJE" json:"mensaje"`
	Inicio     time.Time `gorm:"column:INICIO" json:"inicio"`
}

// SolicitudDespliegue: lo que manda Plantillas.vue (igual que models/Despliegue.ts)
type SolicitudDespliegue struct {
	Plantilla       string `json:"plantilla"`        // basica | intermedia | avanzada
	NombreLab       string `json:"nombre_lab"`       // nombre del proyecto en GNS3
	Subnet          string `json:"subnet"`           // red de las LAN, ej. 192.168.0.0/16
	SerialSubnet    string `json:"serial_subnet"`    // red de los enlaces entre routers
	Vlans           []Vlan `json:"vlans"`            // solo la avanzada
	RoutingProtocol string `json:"routing_protocol"` // rip | ospf
	OspfArea        int64  `json:"ospf_area"`
}

type Vlan struct {
	ID      int    `json:"id"`
	Subnet  string `json:"subnet"`
	Gateway string `json:"gateway"`
}

// ---------------------------------------------------------------------
// Topologia: el JSON que entiende deploy_topology.yml.
// Las plantillas lo leen de ansible/topologies/*.json; después la IA podrá
// mandar uno propio con el mismo formato. Los campos "calculados" los llena
// Go (service.ResolverTopologia) antes de mandarlo al laboratorio.
// ---------------------------------------------------------------------
type Topologia struct {
	ProjectName     string           `json:"project_name"`
	ProjectSettings *ProjectSettings `json:"project_settings,omitempty"`
	Nodes           []Nodo           `json:"nodes"`
	Links           []Enlace         `json:"links"`
}

// ProjectSettings: los parámetros del profesor, inyectados por Go (como el original)
type ProjectSettings struct {
	Subnet          string `json:"subnet"`
	SerialSubnet    string `json:"serial_subnet"`
	Vlans           []Vlan `json:"vlans"`
	RoutingProtocol string `json:"routing_protocol"`
	OspfArea        int64  `json:"ospf_area"`
}

// Nodo: router (type "cisco"), switch ("switch") o host ("docker": Ubuntu o VPCS)
type Nodo struct {
	Name     string `json:"name"`
	Template string `json:"template"`
	Type     string `json:"type"`
	X        int    `json:"x"`
	Y        int    `json:"y"`

	Vlans      []VlanSwitch `json:"vlans,omitempty"`      // switch: puertos de acceso por VLAN
	Interfaces []Interfaz   `json:"interfaces,omitempty"` // router

	SubnetTag  string `json:"subnet_tag,omitempty"`  // host: red donde está
	HostNumber int    `json:"host_number,omitempty"` // host: número dentro de la red
	Vlan       int    `json:"vlan,omitempty"`        // host: VLAN (si está en una)

	// Calculados por Go
	IP      string   `json:"ip,omitempty"`       // host
	Netmask string   `json:"netmask,omitempty"`  // host
	Prefix  int      `json:"prefix,omitempty"`   // host (para VPCS: ip X/24)
	Gateway string   `json:"gateway,omitempty"`  // host
	Parents []string `json:"parents,omitempty"`  // router: interfaces físicas de las subinterfaces
	Redes   []Red    `json:"networks,omitempty"` // router: redes para RIP/OSPF
}

type Interfaz struct {
	Name       string `json:"name"`                // ej. FastEthernet1/0, Serial2/0, FastEthernet1/0.10
	SubnetTag  string `json:"subnet_tag"`          // red a la que se conecta
	HostNumber int    `json:"host_number"`         // número del router dentro de esa red
	Vlan       int    `json:"vlan,omitempty"`      // subinterfaz dot1Q
	MaskBits   int    `json:"mask_bits,omitempty"` // tamaño de la red (por omisión /24 LAN, /30 serial)

	// Calculados por Go
	IP      string `json:"ip,omitempty"`
	Netmask string `json:"netmask,omitempty"`
}

type VlanSwitch struct {
	ID    int    `json:"id"`
	Name  string `json:"name"`
	Ports []int  `json:"ports"`
}

// Enlace: cable entre dos nodos. port = adaptador (slot); port_sub = puerto del adaptador
type Enlace struct {
	NodeA    string `json:"node_a"`
	PortA    int    `json:"port_a"`
	PortASub int    `json:"port_a_sub"`
	NodeB    string `json:"node_b"`
	PortB    int    `json:"port_b"`
	PortBSub int    `json:"port_b_sub"`
}

type Red struct {
	Network  string `json:"network"`
	Wildcard string `json:"wildcard"`
}
