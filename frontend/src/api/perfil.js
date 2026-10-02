import http from './http'

/*
 * Mi perfil: real (Firebird). Ya no usa mock.
 */
export const perfilApi = {
  /** -> { matricula, nombre, tipo, grupo, roles, grupos_docente } */
  async obtener() {
    return (await http.get('/usuario/perfil')).data
  },

  async cambiarPassword(actual, nueva) {
    await http.put('/usuario/perfil/password', { password_actual: actual, nueva_password: nueva })
  }
}
