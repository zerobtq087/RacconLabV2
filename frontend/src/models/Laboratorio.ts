import type { Persona } from './Grupo'

export class Laboratorio {
  constructor(
    public estado: string = 'NINGUNO',   // NINGUNO | CREANDO | CORRIENDO | ERROR
    public mensaje: string = '',
    public soy_dueno: boolean = false,
    public dueno: string = '',           // nombre del dueño
    public puerto: number = 0,           // puerto público de GNS3 (sin usuario ni contraseña)
    public codigo: string = '',          // código para que otros se unan (solo en equipo)
    public miembros: Persona[] = [],
    public inicio: string = ''
  ) { }
}

export interface Invitacion {
  id: string
  emisor: string                         // nombre de quien invita
  matricula: string
  creada: string
}
