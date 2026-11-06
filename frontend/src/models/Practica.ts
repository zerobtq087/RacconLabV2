// Prácticas con calificación (igual que backend/domain/practica.go)

export interface NodoPractica {
  nombre: string
  tipo: string            // dynamips, docker, vpcs, ethernet_switch...
  puertos: string[]
}

export class Criterio {
  constructor(
    public id: string = '',
    public orden: number = 0,
    public tipo: string = 'PING',   // PING, IP_INTERFAZ, PROTOCOLO, VECINOS_OSPF, RUTA, VLAN_PUERTO, TRUNK, LINEA_CONFIG, DHCP
    public peso: number = 10,
    public nodo: string = '',
    public interfaz: string = '',
    public destino: string = '',    // PING: nodo o IP
    public valor: string = '',      // IP/red, línea, vecinos, VLAN...
    public modo: string = 'DEBE',
    public mensaje: string = ''     // lo que ve el alumno si falla (opcional)
  ) { }
}

export class Practica {
  constructor(
    public id: string = '',
    public profesor: string = '',
    public id_grupo: string = '',
    public grupo: string = '',
    public nombre: string = '',
    public instrucciones: string = '',
    public enable_password: string = '',
    public proyecto_origen: string = '',
    public estado: string = '',     // EXPORTANDO, BORRADOR, ACTIVA, CERRADA, ERROR
    public paso: string = '',
    public mensaje: string = '',
    public creada: string = '',
    public activada: string | null = null,
    public cerrada: string | null = null,
    public entregadas: number = 0,
    public alumnos: number = 0,
    public tiene_entregas: boolean = false,
    public nodos: NodoPractica[] = [],
    public criterios: Criterio[] = []
  ) { }
}

// Lo que se manda al crear o editar
export class SolicitudPractica {
  constructor(
    public proyecto: string = '',   // ID del proyecto en mi GNS3 (solo al crear)
    public nombre: string = '',
    public id_grupo: string = '',
    public instrucciones: string = '',
    public enable_password: string = ''
  ) { }
}

export interface ProyectoGNS3 {
  project_id: string
  name: string
  status: string
  nodos: number
  encendidos: number
}

export interface Revision {
  id: string
  es_prueba: boolean
  estado: string          // EN_COLA, REVISANDO, CALIFICADA, ERROR
  porcentaje: number
  paso: string
  mensaje: string
  calificacion: number | null
  inicio: string
  entrego: string
}

export interface Resultado {
  id_criterio: string
  tipo: string
  nodo: string
  peso: number
  cumple: boolean
  puntos: number
  mensaje: string
  obtenido?: string       // solo docente
  descripcion?: string    // solo docente
}

export interface Prueba {
  revision: Revision | null
  resultados: Resultado[]
}

export interface FilaEntrega {
  id_usuario: string
  matricula: string
  nombre: string
  id_revision: string
  entregada: string | null
  calificacion: number | null
  entrego: string
  estado: string          // A_TIEMPO, TARDE, PENDIENTE
  retraso: string
}

export interface ConfigNodo {
  nodo: string
  config: string
}

export interface DetalleEntrega {
  fila: FilaEntrega
  integrantes: { id: string, matricula: string, nombre: string }[]
  resultados: Resultado[]
  configs: ConfigNodo[]
  con_proyecto: boolean
}

export interface MiPractica {
  id: string
  nombre: string
  instrucciones?: string
  estado: string          // ACTIVA o CERRADA
  cerrada: string | null
  profesor: string
  entregada: string | null
  calificacion: number | null
  tarde: boolean
  retraso: string
  fallidos: string[]
  revision: Revision | null
}

// Supervisión
export interface AlumnoLab {
  id_usuario: string
  matricula: string
  nombre: string
  id_lab: string
  puerto: number
  estado: string
  es_dueno: boolean
}

export interface NodoSupervision {
  nombre: string
  tipo: string
  estado: string
  consola: number
  config: string
  error?: string
}

export interface EntradaBitacora {
  fecha: string
  docente: string
  alumno: string
  accion: string
  detalle: string
}

// Tipos de criterio para el formulario: qué campos pide cada uno
export const tiposCriterio = [
  { value: 'PING', title: 'Ping entre nodos', nodos: ['docker', 'vpcs', 'dynamips'] },
  { value: 'IP_INTERFAZ', title: 'IP de una interfaz', nodos: ['docker', 'vpcs', 'dynamips'] },
  { value: 'PROTOCOLO', title: 'Protocolo de enrutamiento activo', nodos: ['dynamips'] },
  { value: 'VECINOS_OSPF', title: 'Vecinos OSPF en FULL', nodos: ['dynamips'] },
  { value: 'RUTA', title: 'Ruta en la tabla', nodos: ['dynamips'] },
  { value: 'VLAN_PUERTO', title: 'Puerto del switch en una VLAN', nodos: ['ethernet_switch'] },
  { value: 'TRUNK', title: 'Puerto del switch como trunk', nodos: ['ethernet_switch'] },
  { value: 'LINEA_CONFIG', title: 'Línea en la running-config', nodos: ['dynamips'] },
  { value: 'DHCP', title: 'Host con IP por DHCP', nodos: ['docker', 'vpcs'] },
]

export const modosCriterio: Record<string, { value: string, title: string }[]> = {
  PING: [{ value: 'DEBE', title: 'Debe responder' }, { value: 'NO_DEBE', title: 'NO debe responder' }],
  IP_INTERFAZ: [{ value: 'EXACTA', title: 'IP exacta' }, { value: 'RED', title: 'Cualquier IP de la red' }],
  PROTOCOLO: [{ value: 'OSPF', title: 'OSPF' }, { value: 'RIP', title: 'RIP' }],
  RUTA: [{ value: 'OSPF', title: 'Aprendida por OSPF' }, { value: 'RIP', title: 'Aprendida por RIP' }, { value: 'ESTATICA', title: 'Estática' }],
  LINEA_CONFIG: [{ value: 'EXACTA', title: 'Línea exacta' }, { value: 'EMPIEZA', title: 'Empieza con' }, { value: 'CONTIENE', title: 'Contiene' }],
}

export const estadosPractica: Record<string, { color: string, texto: string }> = {
  EXPORTANDO: { color: 'info', texto: 'Exportando' },
  BORRADOR: { color: 'grey', texto: 'Borrador' },
  ACTIVA: { color: 'success', texto: 'Activa' },
  CERRADA: { color: 'warning', texto: 'Cerrada' },
  ERROR: { color: 'error', texto: 'Error' },
}

// Fecha legible: 06/11/2026 14:05
export function fecha (f: string | null | undefined): string {
  if (!f) return ''
  return new Date(f).toLocaleString('es-MX', { dateStyle: 'short', timeStyle: 'short' })
}
