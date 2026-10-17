export interface Vlan {
  id: number
  subnet: string
  gateway: string
}

export class Despliegue {
  constructor(
    public plantilla: string = '',        // basica | intermedia | avanzada
    public nombre_lab: string = '',
    public subnet: string = '',           // red LAN, ej. 192.168.1.0/24
    public serial_subnet: string = '',    // red de enlaces entre routers
    public routing_protocol: string = 'ospf',
    public ospf_area: number = 0,
    public vlans: Vlan[] = []
  ) { }
}
