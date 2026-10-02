import http from './http'

/*
 * Grupos: TODO es real (Firebird). Ya no usa mock.
 * El backend decide qué ve cada quien por el rol activo de la sesión.
 */
export const gruposApi = {
  // Admin: todos | Maestro: solo los suyos
  async listar() {
    return (await http.get('/grupos')).data
  },

  async alumnos(codigo) {
    return (await http.get(`/grupos/${encodeURIComponent(codigo)}/alumnos`)).data
  },

  // ----- Solo admin -----
  async maestrosDisponibles() {
    return (await http.get('/admin/maestros')).data
  },

  async crear(codigo) {
    return (await http.post('/admin/grupos', { codigo })).data
  },

  async eliminar(codigo) {
    await http.delete(`/admin/grupos/${encodeURIComponent(codigo)}`)
  },

  /** docentes = ['doc0045', 'doc0050'] (reemplaza la lista completa) */
  async asignarDocentes(codigo, docentes) {
    await http.put(`/admin/grupos/${encodeURIComponent(codigo)}/docentes`, { docentes })
  }
}
