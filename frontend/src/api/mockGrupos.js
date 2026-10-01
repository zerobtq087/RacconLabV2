import { USUARIOS, GRUPOS } from './mock'

const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))
const mismo = (a, b) => String(a).toUpperCase() === String(b).toUpperCase()
const usuario = (m) => USUARIOS.find((u) => mismo(u.matricula, m))

const alumnosDe = (codigo) => USUARIOS.filter((u) => u.tipo === 'alumnos' && u.grupo === codigo)

const aDTO = (g) => ({
  codigo: g.codigo,
  docentes: g.docentes
    .map(usuario)
    .filter(Boolean)
    .map((u) => ({ matricula: u.matricula, nombre: u.nombre, activo: u.activo && u.roles.includes('PROFESOR') })),
  total_alumnos: alumnosDe(g.codigo).length
})

// Admin ve todos; maestro solo donde está asignado
export const mockListarGrupos = async (rol, matricula) => {
  await esperar(300)
  const lista = rol === 'ADMIN' ? GRUPOS : GRUPOS.filter((g) => g.docentes.some((d) => mismo(d, matricula)))
  return lista.map(aDTO).sort((a, b) => a.codigo.localeCompare(b.codigo))
}

export const mockMaestrosDisponibles = async () => {
  await esperar(200)
  return USUARIOS
    .filter((u) => u.activo && u.roles.includes('PROFESOR'))
    .map((u) => ({ matricula: u.matricula, nombre: u.nombre }))
    .sort((a, b) => a.nombre.localeCompare(b.nombre))
}

export const mockCrearGrupo = async (codigo) => {
  await esperar(300)
  codigo = String(codigo || '').trim().toUpperCase()
  if (!/^[A-Z0-9-]{2,15}$/.test(codigo)) throw error(400, 'Código inválido (ej. IRD-71)')
  if (GRUPOS.some((g) => g.codigo === codigo)) throw error(409, `El grupo ${codigo} ya existe`)
  GRUPOS.push({ codigo, docentes: [] })
  return { codigo }
}

export const mockEliminarGrupo = async (codigo) => {
  await esperar(300)
  const i = GRUPOS.findIndex((g) => g.codigo === codigo)
  if (i < 0) throw error(404, 'Grupo no encontrado')
  if (alumnosDe(codigo).length) throw error(400, 'No se puede eliminar: el grupo tiene alumnos')
  GRUPOS.splice(i, 1)
}

export const mockAsignarDocentes = async (codigo, docentes) => {
  await esperar(400)
  const g = GRUPOS.find((x) => x.codigo === codigo)
  if (!g) throw error(404, 'Grupo no encontrado')
  for (const m of docentes) {
    const u = usuario(m)
    if (!u || !u.roles.includes('PROFESOR')) throw error(400, `${m} no tiene el rol de Maestro`)
  }
  g.docentes = [...new Set(docentes.map((m) => usuario(m).matricula))]
  return aDTO(g)
}

export const mockAlumnosDeGrupo = async (codigo, rol, matricula) => {
  await esperar(300)
  const g = GRUPOS.find((x) => x.codigo === codigo)
  if (!g) throw error(404, 'Grupo no encontrado')
  if (rol !== 'ADMIN' && !g.docentes.some((d) => mismo(d, matricula))) {
    throw error(403, 'No estás asignado a este grupo')
  }
  return alumnosDe(codigo)
    .map((u) => ({ matricula: u.matricula, nombre: u.nombre, activo: u.activo }))
    .sort((a, b) => a.nombre.localeCompare(b.nombre))
}