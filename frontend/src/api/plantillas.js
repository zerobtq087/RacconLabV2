import http from './http'

/*
 * Plantillas Ansible: real. Ya no usa mock.
 * Se despliegan en el laboratorio del maestro (el de "Mi laboratorio").
 */
export const plantillasApi = {
  async desplegar(payload) {
    // Ansible crea nodos, espera a que arranquen los routers y los configura: varios minutos
    return (await http.post('/labs/profesor/despliegue', payload, { timeout: 600000 })).data
  }
}
