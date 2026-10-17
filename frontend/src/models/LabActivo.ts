export class LabActivo {
  constructor(
    public id: string = '',              // nombre del contenedor
    public matricula: string = '',       // dueño
    public nombre: string = '',
    public grupo: string = '',
    public puerto: number = 0,
    public codigo: string = '',          // vacío = individual
    public miembros: number = 0,
    public inicio: string = ''
  ) { }
}
