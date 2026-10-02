import http from './http'
import { useAuth } from '@/stores/auth'
import * as mock from './mockLabs'

// El laboratorio propio ya es REAL (Docker + GNS3).
// Compañeros e invitaciones siguen simulados hasta el paso 9c.
const MOCK = import.meta.env.VITE_MOCK === 'true'
const yo = () => useAuth().usuario.matricula

export const labsApi = {
  async estado() {
    return (await http.get('/labs/estado')).data
  },

  /** { es_colaborativo: bool } o { codigo_lab: 'COLAB-XXXX' } */
  async crear(payload) {
    // Levantar GNS3 y cargar plantillas puede tardar ~2 min
    return (await http.post('/labs/crear', payload, { timeout: 300000 })).data
  },

  async limpiar() {
    return (await http.post('/labs/limpiar', {}, { timeout: 120000 })).data
  },

  // ----- Equipo (paso 9c) -----
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
