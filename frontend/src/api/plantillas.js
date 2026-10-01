import http from './http'
import { mockDesplegarPlantilla } from './mockPlantillas'

const MOCK = import.meta.env.VITE_MOCK === 'true'

export const plantillasApi = {
  async desplegar(payload) {
    if (MOCK) return mockDesplegarPlantilla(payload)
    return (await http.post('/labs/profesor/despliegue', payload, { timeout: 180000 })).data
  }
}