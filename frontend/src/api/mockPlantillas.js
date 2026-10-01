const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))

export const mockDesplegarPlantilla = async (payload) => {
  await esperar(2500) // simula Ansible
  if (!payload.container_name) throw error(400, 'No hay workspace activo')
  if (!payload.subnet) throw error(400, 'Falta la red base')
  return {
    status: 'success',
    message: `Topología "${payload.nombre_lab}" desplegada (${payload.routing_protocol.toUpperCase()}, ${payload.vlans.length} VLANs)`
  }
}