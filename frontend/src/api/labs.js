import http from './http'
import { useAuth } from '@/stores/auth'
import * as mock from './mockLabs'

const MOCK = import.meta.env.VITE_MOCK === 'true'
const yo = () => useAuth().usuario.matricula

export const labsApi = {
  async estado() {
    if (MOCK) return mock.mockEstado(yo())
    return (await http.get('/labs/estado')).data
  },

  /** { es_colaborativo: bool } o { codigo_lab: 'COLAB-XXXX' } */
  async crear(payload) {
    if (MOCK) return mock.mockCrear(yo(), useAuth().rol, payload)
    return (await http.post('/labs/crear', payload, { timeout: 180000 })).data
  },

  async limpiar() {
    if (MOCK) return mock.mockLimpiar(yo())
    return (await http.post('/labs/limpiar', {}, { timeout: 60000 })).data
  },

  async companeros() {
    if (MOCK) return mock.mockCompaneros(yo())
    return (await http.get('/labs/companeros')).data
  },

  async invitar(matricula) {
    if (MOCK) return mock.mockInvitar(yo(), matricula)
    return (await http.post('/labs/invitaciones/invitar', { invitados_ids: [matricula] })).data
  },

  async pendientes() {
    if (MOCK) return mock.mockPendientes(yo())
    return (await http.get('/labs/invitaciones/pendientes')).data
  },

  async responder(idInvitacion, aceptar) {
    if (MOCK) return mock.mockResponder(yo(), idInvitacion, aceptar)
    return (await http.post('/labs/invitaciones/responder', { id_invitacion: idInvitacion, aceptar })).data
  }
}