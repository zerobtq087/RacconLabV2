export interface Persona {
  id: string
  matricula: string
  nombre: string
}

export class Grupo {
  constructor(
    public id: string = '',
    public codigo: string = '',          // IRD-71
    public carrera: string = '',
    public alumnos: number = 0,          // cuántos alumnos tiene
    public docentes: Persona[] = []      // maestros asignados
  ) { }
}
