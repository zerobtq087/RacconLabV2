import http from './http'
import { mockListarWorkspaces, mockLimpiar } from './mockLabs'

const MOCK = import.meta.env.VITE_MOCK === 'true'

export const monitorApi = {
  async listar() {
    if (MOCK) return mockListarWorkspaces()
    return (await http.get('/admin/workspaces')).data
  },

  /** El admin cierra un laboratorio (se cierra para todo el equipo) */
  async cerrar(ws) {
    if (MOCK) return mockLimpiar(ws.creado_por)
    return (await http.delete(`/admin/workspaces/${encodeURIComponent(ws.id_workspace)}`)).data
  }
}