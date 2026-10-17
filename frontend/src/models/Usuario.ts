export class Usuario {
  constructor(
    public id: string = '',
    public matricula: string = '',
    public nombre: string = '',
    public tipo: string = '',            // alumnos | maestros | admins
    public grupo: string = '',           // código del grupo (solo alumnos)
    public activo: boolean = true,
    public roles: string[] = [],
    public protegido: boolean = false    // admin general: no se puede editar ni borrar
  ) { }
}
