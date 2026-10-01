import { USUARIOS, GRUPOS } from './mock'
import { normalizarRoles } from '@/utils/roles'

const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))
const mismo = (a, b) => String(a).toUpperCase() === String(b).toUpperCase()
const buscar = (m) => USUARIOS.find((u) => mismo(u.matricula, m))

export const mockObtenerPerfil = async (matricula) => {
  await esperar(300)
  const u = buscar(matricula)
  if (!u) throw error(404, 'Usuario no encontrado')
  return {
    matricula: u.matricula,
    nombre: u.nombre,
    tipo: u.tipo,
    grupo: u.grupo,
    roles: normalizarRoles(u.roles),
    activo: u.activo,
    grupos_docente: GRUPOS.filter((g) => g.docentes.some((d) => mismo(d, u.matricula))).map((g) => g.codigo)
  }
}

export const mockCambiarPassword = async (matricula, actual, nueva) => {
  await esperar(500)
  const u = buscar(matricula)
  if (!u) throw error(404, 'Usuario no encontrado')
  if (u.password !== actual) throw error(400, 'La contraseña actual es incorrecta')
  if (nueva.length < 8) throw error(400, 'La nueva contraseña debe tener al menos 8 caracteres')
  if (nueva === actual) throw error(400, 'La nueva contraseña debe ser distinta a la actual')
  u.password = nueva
}