import http from './http'
import { mockListarUsuarios, mockActualizarUsuario } from './mock'
import { mockResetPassword } from './mockAdmin'
import { useAuth } from '@/stores/auth'

const MOCK = import.meta.env.VITE_MOCK === 'true'

export const usuariosApi = {
  async listar() {
    if (MOCK) return mockListarUsuarios()
    return (await http.get('/admin/usuarios')).data
  },

  /** cambios = { matricula, roles: ['ADMIN','PROFESOR','ALUMNO'], activo } */
  async actualizar(cambios) {
    if (MOCK) return mockActualizarUsuario(useAuth().usuario.matricula, cambios)
    return (await http.put('/admin/usuarios/editar', cambios)).data
  },

  /** El admin pone una contraseña nueva sin conocer la anterior */
  async resetPassword(matricula, nueva) {
    if (MOCK) return mockResetPassword(matricula, nueva)
    return (await http.put('/admin/usuarios/password', { matricula, nueva_password: nueva })).data
  },

  /**
   * Importación masiva (SIEMPRE real: el backend ya existe).
   * tipo:     'alumnos' | 'maestros' | 'admins'
   * onSubida: callback con el % de subida (0-100)
   * Responde de inmediato con el trabajo { id, estado: 'procesando', ... }
   */
  async importar(archivo, tipo, onSubida) {
    const fd = new FormData()
    fd.append('archivo', archivo)
    const { data } = await http.post(`/admin/importar/${tipo}`, fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 0, // 100 MB pueden tardar en subir
      onUploadProgress: (e) => {
        if (onSubida && e.total) onSubida(Math.round((e.loaded * 100) / e.total))
      }
    })
    return data
  },

  /** Avance en vivo de una importación */
  async estadoImportacion(id) {
    return (await http.get(`/admin/importar/${id}`)).data
  }
}
