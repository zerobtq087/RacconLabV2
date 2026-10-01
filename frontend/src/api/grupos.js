import http from './http'
import { useAuth } from '@/stores/auth'
import {
  mockListarGrupos, mockMaestrosDisponibles, mockCrearGrupo,
  mockEliminarGrupo, mockAsignarDocentes, mockAlumnosDeGrupo
} from './mockGrupos'

const MOCK = import.meta.env.VITE_MOCK === 'true'
const sesion = () => {
  const a = useAuth()
  return [a.rol, a.usuario?.matricula]
}

export const gruposApi = {
  // Admin: todos | Maestro: solo los suyos (el backend decide por el rol del token)
  async listar() {
    if (MOCK) return mockListarGrupos(...sesion())
    return (await http.get('/grupos')).data
  },

  async alumnos(codigo) {
    if (MOCK) return mockAlumnosDeGrupo(codigo, ...sesion())
    return (await http.get(`/grupos/${encodeURIComponent(codigo)}/alumnos`)).data
  },

  // ----- Solo admin -----
  async maestrosDisponibles() {
    if (MOCK) return mockMaestrosDisponibles()
    return (await http.get('/admin/maestros')).data
  },

  async crear(codigo) {
    if (MOCK) return mockCrearGrupo(codigo)
    return (await http.post('/admin/grupos', { codigo })).data
  },

  async eliminar(codigo) {
    if (MOCK) return mockEliminarGrupo(codigo)
    return (await http.delete(`/admin/grupos/${encodeURIComponent(codigo)}`)).data
  },

  /** docentes = ['DOC0045', 'DOC0050'] (reemplaza la lista completa) */
  async asignarDocentes(codigo, docentes) {
    if (MOCK) return mockAsignarDocentes(codigo, docentes)
    return (await http.put(`/admin/grupos/${encodeURIComponent(codigo)}/docentes`, { docentes })).data
  }
}