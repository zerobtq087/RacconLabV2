import http from './http'
import { mockImportar, mockListarUsuarios, mockActualizarUsuario } from './mock'
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
   * tipo:   'alumnos' | 'maestros' | 'admins'
   * dryRun: true = solo valida; false = guarda
   */
  async importar(archivo, tipo, dryRun = true) {
    if (MOCK) return mockImportar(archivo, tipo, dryRun)

    const fd = new FormData()
    fd.append('archivo', archivo)
    const { data } = await http.post('/admin/usuarios/importar', fd, {
      params: { tipo, dry_run: dryRun },
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 120000
    })
    return data
  }
}