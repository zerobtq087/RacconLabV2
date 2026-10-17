export interface ErrorFila {
  fila: number
  mensaje: string
}

export class Importacion {
  constructor(
    public id: string = '',
    public tipo: string = '',            // alumnos | maestros | admins
    public archivo: string = '',
    public estado: string = '',          // PROCESANDO | TERMINADO | ERROR
    public mensaje: string = '',
    public porcentaje: number = 0,
    public leidas: number = 0,
    public insertados: number = 0,
    public actualizados: number = 0,
    public con_error: number = 0,
    public errores: ErrorFila[] = []
  ) { }
}
