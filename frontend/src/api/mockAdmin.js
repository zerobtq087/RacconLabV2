import { USUARIOS } from './mock'

const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))

// El admin cambia la contraseña de cualquier usuario sin conocer la anterior
export const mockResetPassword = async (matricula, nueva) => {
  await esperar(400)
  const u = USUARIOS.find((x) => x.matricula.toUpperCase() === String(matricula).toUpperCase())
  if (!u) throw error(404, 'Usuario no encontrado')
  if (!nueva || nueva.length < 8) throw error(400, 'La contraseña debe tener al menos 8 caracteres')
  u.password = nueva
}