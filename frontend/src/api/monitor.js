import http from './http'

/*
 * Monitor de laboratorios (solo admin): real. Ya no usa mock.
 */
export const monitorApi = {
  /** [{ id_workspace, creado_por, miembros, puerto_base, codigo_lab, fecha_inicio, running }] */
  async listar() {
    return (await http.get('/admin/workspaces')).data
  },

  /** El admin cierra un laboratorio (se cierra para todo el equipo) */
  async cerrar(ws) {
    await http.delete(`/admin/workspaces/${encodeURIComponent(ws.id_workspace)}`, { timeout: 120000 })
  }
}
