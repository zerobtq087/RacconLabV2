export class Usuario {
  constructor(
    public id: string = '',
    public matricula: string = '',
    public nombre: string = '',
    public tipo: string = '',            // alumnos | maestros | admins
    public grupo: string = '',           // código del grupo (solo alumnos)
    public carrera: string = '',         // carrera de su grupo
    public activo: boolean = true,
    public roles: string[] = [],
    public protegido: boolean = false    // admin general: no se puede editar ni borrar
  ) { }
}

// Lo que se manda al agregar (POST) o editar (PUT) un usuario (igual que domain.UsuarioForm)
export class UsuarioForm {
  constructor(
    public matricula: string = '',
    public nombre: string = '',
    public password: string = '',        // al editar: vacía = no se cambia
    public tipo: string = 'alumnos',
    public roles: string[] = [],
    public grupo: string = '',
    public carrera: string = '',
    public activo: boolean = true
  ) { }
}

// Catálogo de tipos con los roles que da cada uno (GET /api/usuarios/tipos)
export interface TipoUsuario {
  clave: string                          // alumnos | maestros | admins
  descripcion: string
  roles: string[]
}
