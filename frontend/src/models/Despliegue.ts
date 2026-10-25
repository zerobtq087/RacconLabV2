export interface Vlan {
  id: number
  subnet: string
  gateway: string
}

// Lo que se manda a POST /api/plantillas/despliegue (igual que domain.SolicitudDespliegue)
export class Despliegue {
  constructor(
    public plantilla: string = '',        // basica | intermedia | avanzada
    public nombre_lab: string = '',
    public subnet: string = '',           // red LAN, ej. 192.168.1.0/24
    public serial_subnet: string = '',    // red de enlaces entre routers
    public routing_protocol: string = 'ospf',  // rip | ospf
    public ospf_area: number = 0,
    public vlans: Vlan[] = []
  ) { }
}

// Avance del último despliegue (GET /api/plantillas/despliegue, igual que domain.Despliegue)
export class EstadoDespliegue {
  constructor(
    public id: string = '',
    public origen: string = '',           // basica | intermedia | avanzada (después: ia)
    public proyecto: string = '',
    public estado: string = 'NINGUNO',    // NINGUNO | PROCESANDO | TERMINADO | ERROR
    public porcentaje: number = 0,
    public paso: string = '',
    public mensaje: string = '',
    public inicio: string = ''
  ) { }
}
