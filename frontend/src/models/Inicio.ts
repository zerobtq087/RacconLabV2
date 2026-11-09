import type { Invitacion, Laboratorio } from './Laboratorio'
import type { MiPractica } from './Practica'

// Tablero de Inicio (igual que domain/inicio.go). GET /api/inicio llena SOLO
// la parte del rol con el que se entró.
export interface Inicio {
  rol: string
  alumno?: InicioAlumno
  profesor?: InicioProfesor
  admin?: InicioAdmin
}

export interface InicioAlumno {
  lab: Laboratorio
  soy_dueno: boolean
  pendientes: MiPractica[]        // activas sin entregar
  calificaciones: MiPractica[]    // las últimas entregadas
  invitaciones: Invitacion[]
}

export interface ResumenPractica {
  id: string
  nombre: string
  grupo: string
  entregadas: number
  alumnos: number
}

export interface ResumenGrupo {
  id: string
  codigo: string
  carrera: string
  alumnos: number
  labs_encendidos: number
}

export interface InicioProfesor {
  practicas: ResumenPractica[]
  grupos: ResumenGrupo[]
  alumnos: number
  labs_encendidos: number
}

export interface RecursosServidor {
  cpu: number                     // %
  nucleos: number
  ram_total: number               // bytes
  ram_usada: number
  disco_total: number
  disco_usado: number
  disco_ruta: string
  error: string
}

export interface UsuariosPorTipo {
  tipo: string
  descripcion: string
  total: number
  activos: number
}

export interface ResumenImportacion {
  archivo: string
  tipo: string
  estado: string
  insertados: number
  actualizados: number
  con_error: number
  inicio: string
  usuario: string
}

export interface EstadoServicio {
  nombre: string
  bien: boolean
  detalle: string
}

export interface InicioAdmin {
  labs_activos: number
  labs_creando: number
  labs_max: number
  servidor: RecursosServidor
  usuarios: UsuariosPorTipo[]
  ultima_importacion: ResumenImportacion | null
  practicas_activas: number
  servicios: EstadoServicio[]
}

// 1536 → "1.5 KB"
export function tamano (bytes: number): string {
  const unidades = ['B', 'KB', 'MB', 'GB', 'TB']
  let i = 0
  let n = bytes
  while (n >= 1024 && i < unidades.length - 1) {
    n /= 1024
    i++
  }
  return `${n.toFixed(i >= 3 ? 1 : 0)} ${unidades[i]}`
}

// porcentaje entero (0 si el total es 0)
export function porcentaje (parte: number, total: number): number {
  return total > 0 ? Math.round(100 * parte / total) : 0
}

// verde, naranja o rojo según qué tan lleno está
export function colorUso (p: number): string {
  if (p >= 90) return 'error'
  if (p >= 70) return 'warning'
  return 'success'
}
