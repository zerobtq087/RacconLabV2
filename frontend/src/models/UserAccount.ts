export class UserAccount {
   constructor(
      public id: string = '',
      public matricula: string = '',
      public nombre: string = '',
      public rol: string = '',          // rol con el que entró: ADMIN | PROFESOR | ALUMNO
      public roles: string[] = [],      // todos los roles que tiene
      public grupo: string = ''
   ) { }
}
