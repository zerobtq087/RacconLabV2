import http from './http'
import { useAuth } from '@/stores/auth'
import { mockObtenerPerfil, mockCambiarPassword } from './mockPerfil'

const MOCK = import.meta.env.VITE_MOCK === 'true'

export const perfilApi = {
  async obtener() {
    if (MOCK) return mockObtenerPerfil(useAuth().usuario.matricula)
    return (await http.get('/usuario/perfil')).data
  },

  async cambiarPassword(actual, nueva) {
    if (MOCK) return mockCambiarPassword(useAuth().usuario.matricula, actual, nueva)
    return (await http.put('/usuario/perfil/password', { password_actual: actual, nueva_password: nueva })).data
  }
}