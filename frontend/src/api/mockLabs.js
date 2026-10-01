import { USUARIOS } from './mock'

const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))
const mismo = (a, b) => String(a).toUpperCase() === String(b).toUpperCase()
const usuario = (m) => USUARIOS.find((u) => mismo(u.matricula, m))
const aleatorio = (n, c = 'ABCDEFGHJKLMNPQRSTUVWXYZ23456789') =>
  [...crypto.getRandomValues(new Uint32Array(n))].map((x) => c[x % c.length]).join('')

// Estado en memoria
const WORKSPACES = []   // { id, dueno, miembros[], colaborativo, codigo, puerto, token, inicio }
const INVITACIONES = [] // { id, id_workspace, emisor, receptor, estado }
let siguientePuerto = 30000

const workspaceDe = (m) => WORKSPACES.find((w) => mismo(w.dueno, m) || w.miembros.some((x) => mismo(x, m)))

const aEstado = (w) => ({
  id_workspace: w.id,
  running: true,
  status: { running: true },
  ip_real_host: window.location.hostname,
  puerto_web: w.puerto + 10000, // puerto público del proxy
  token_acceso: w.token,
  es_colaborativo: w.colaborativo,
  codigo_lab: w.codigo,
  soy_dueno: false
})

export const mockEstado = async (matricula) => {
  await esperar(300)
  const w = workspaceDe(matricula)
  if (!w) return { running: false, status: { running: false } }
  return { ...aEstado(w), soy_dueno: mismo(w.dueno, matricula) }
}

export const mockCrear = async (matricula, rol, { es_colaborativo, codigo_lab }) => {
  await esperar(1500) // simula levantar el contenedor
  const u = usuario(matricula)
  if (rol === 'ALUMNO' && u.tipo === 'alumnos' && !u.grupo) {
    throw error(403, 'Debes pertenecer a un grupo para desplegar un laboratorio')
  }
  if (workspaceDe(matricula)) throw error(409, 'Ya tienes un laboratorio activo')

  // Unirse por código
  if (codigo_lab) {
    const w = WORKSPACES.find((x) => x.codigo === codigo_lab.trim().toUpperCase())
    if (!w) throw error(404, 'El código no existe o el laboratorio ya se cerró')
    w.miembros.push(u.matricula)
    return aEstado(w)
  }

  const w = {
    id: `ws-${aleatorio(6).toLowerCase()}`,
    dueno: u.matricula,
    miembros: [],
    colaborativo: !!es_colaborativo,
    codigo: es_colaborativo ? `COLAB-${aleatorio(4)}` : null,
    puerto: (siguientePuerto += 200),
    token: aleatorio(8),
    inicio: new Date().toISOString()
  }
  WORKSPACES.push(w)
  return aEstado(w)
}

export const mockLimpiar = async (matricula) => {
  await esperar(800)
  const w = workspaceDe(matricula)
  if (!w) return { message: 'No había laboratorio activo' }
  if (mismo(w.dueno, matricula)) {
    WORKSPACES.splice(WORKSPACES.indexOf(w), 1)
    // borrar invitaciones pendientes de ese workspace
    for (let i = INVITACIONES.length - 1; i >= 0; i--) {
      if (INVITACIONES[i].id_workspace === w.id) INVITACIONES.splice(i, 1)
    }
    return { message: 'Laboratorio cerrado para todo el equipo' }
  }
  w.miembros = w.miembros.filter((x) => !mismo(x, matricula))
  return { message: 'Saliste del laboratorio del equipo' }
}

export const mockCompaneros = async (matricula) => {
  await esperar(300)
  const u = usuario(matricula)
  if (!u?.grupo) return []
  return USUARIOS
    .filter((x) => x.grupo === u.grupo && !mismo(x.matricula, matricula) && x.activo)
    .map((x) => ({ matricula: x.matricula, nombre: x.nombre, ocupado: !!workspaceDe(x.matricula) }))
}

export const mockInvitar = async (matricula, receptor) => {
  await esperar(400)
  const w = workspaceDe(matricula)
  if (!w || !w.colaborativo) throw error(400, 'Necesitas un laboratorio colaborativo activo')
  if (!mismo(w.dueno, matricula)) throw error(403, 'Solo el dueño del laboratorio puede invitar')
  if (workspaceDe(receptor)) throw error(409, 'Ese compañero ya está en un laboratorio')
  if (INVITACIONES.some((i) => i.id_workspace === w.id && mismo(i.receptor, receptor) && i.estado === 'PENDIENTE')) {
    throw error(409, 'Ya le enviaste una invitación')
  }
  INVITACIONES.push({ id: aleatorio(10), id_workspace: w.id, emisor: w.dueno, receptor: usuario(receptor).matricula, estado: 'PENDIENTE' })
}

export const mockPendientes = async (matricula) => {
  await esperar(200)
  return INVITACIONES
    .filter((i) => mismo(i.receptor, matricula) && i.estado === 'PENDIENTE')
    .map((i) => ({ id_invitacion: i.id, id_workspace: i.id_workspace, emisor: i.emisor, nombre_emisor: usuario(i.emisor)?.nombre }))
}

export const mockResponder = async (matricula, idInvitacion, aceptar) => {
  await esperar(500)
  const inv = INVITACIONES.find((i) => i.id === idInvitacion && mismo(i.receptor, matricula))
  if (!inv) throw error(404, 'Invitación no encontrada')
  if (!aceptar) { inv.estado = 'RECHAZADA'; return }

  if (workspaceDe(matricula)) throw error(409, 'Primero limpia tu laboratorio actual')
  const w = WORKSPACES.find((x) => x.id === inv.id_workspace)
  if (!w) { inv.estado = 'EXPIRADA'; throw error(404, 'Ese laboratorio ya se cerró') }
  w.miembros.push(usuario(matricula).matricula)
  inv.estado = 'ACEPTADA'
}

// Para la página de monitor del admin (paso 12)
export const mockListarWorkspaces = async () => {
  await esperar(300)
  return WORKSPACES.map((w) => ({
    id_workspace: w.id,
    creado_por: w.dueno,
    miembros: w.miembros,
    puerto_base: w.puerto,
    codigo_lab: w.codigo,
    fecha_inicio: w.inicio
  }))
}