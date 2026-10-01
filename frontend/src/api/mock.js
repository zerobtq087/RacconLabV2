import { normalizarRoles } from '@/utils/roles'

// ===== "Base de datos" en memoria mientras no hay backend =====
export const USUARIOS = [
  { matricula: 'ADMIN', password: 'admin123', nombre: 'Administrador General', grupo: null, tipo: 'admins', roles: ['ADMIN', 'PROFESOR', 'ALUMNO'], activo: true },
  { matricula: 'DOC0045', password: 'maestro123', nombre: 'Juan Pérez Soto', grupo: null, tipo: 'maestros', roles: ['PROFESOR', 'ALUMNO'], activo: true },
  { matricula: '2230123', password: 'alumno123', nombre: 'Ana López Ruiz', grupo: 'IRD-71', tipo: 'alumnos', roles: ['ALUMNO'], activo: true }
]

export const GRUPOS = [
  { codigo: 'IRD-71', docentes: ['DOC0045'] }
]

const error = (status, message) => Object.assign(new Error(message), { response: { status, data: { message } } })
const esperar = (ms) => new Promise((r) => setTimeout(r, ms))
const buscar = (m) => USUARIOS.find((u) => u.matricula.toUpperCase() === String(m).toUpperCase())
const sinPassword = ({ password, ...u }) => ({ ...u, roles: normalizarRoles(u.roles) })

// ===== LOGIN =====
export const mockLogin = async (matricula, password, rol) => {
  await esperar(500)
  const u = buscar(matricula)
  if (!u || u.password !== password) throw error(401, 'Matrícula/clave o contraseña incorrectas')
  if (!u.activo) throw error(403, 'Tu cuenta está desactivada')
  if (!normalizarRoles(u.roles).includes(rol)) {
    throw error(403, 'No tienes asignado ese rol')
  }
  return { access_token: 'mock-token', usuario: { ...sinPassword(u), rol_activo: rol } }
}

// ===== USUARIOS (admin) =====
export const mockListarUsuarios = async () => {
  await esperar(300)
  return USUARIOS.map(sinPassword)
}

export const mockActualizarUsuario = async (matriculaAdmin, { matricula, roles, activo }) => {
  await esperar(400)
  const u = buscar(matricula)
  if (!u) throw error(404, 'Usuario no encontrado')

  const nuevos = normalizarRoles(roles)
  const esYo = u.matricula.toUpperCase() === String(matriculaAdmin).toUpperCase()

  if (esYo && (!nuevos.includes('ADMIN') || activo === false)) {
    throw error(400, 'No puedes quitarte el rol de Admin ni desactivarte a ti mismo')
  }

  const adminsActivos = USUARIOS.filter((x) => x !== u && x.activo && x.roles.includes('ADMIN')).length
  if ((!nuevos.includes('ADMIN') || activo === false) && u.roles.includes('ADMIN') && adminsActivos === 0) {
    throw error(400, 'Debe quedar al menos un admin activo')
  }

  u.roles = nuevos
  if (typeof activo === 'boolean') u.activo = activo
  return sinPassword(u)
}

// ===== IMPORTACIÓN =====
export const TIPOS_IMPORTACION = {
  alumnos: {
    roles: ['ALUMNO'],
    columnas: {
      matricula: ['matricula', 'clave'],
      nombre: ['nombre'],
      password: ['contrasenia', 'contrasena', 'password'],
      grupo: ['grupo']
    }
  },
  maestros: {
    roles: ['PROFESOR', 'ALUMNO'],
    columnas: {
      matricula: ['clave'],
      nombre: ['nombre'],
      password: ['contrasenia', 'contrasena', 'password']
    }
  },
  admins: {
    roles: ['ADMIN', 'PROFESOR', 'ALUMNO'],
    columnas: {
      matricula: ['clave'],
      nombre: ['nombre'],
      password: ['contrasenia', 'contrasena', 'password']
    }
  }
}

const ETIQUETA_TIPO = { alumnos: 'alumno', maestros: 'maestro', admins: 'administrador' }

const normalizar = (s) =>
  String(s ?? '').trim().toLowerCase().normalize('NFD').replace(/[\u0300-\u036f]/g, '')

const parsearCSV = (texto) => {
  texto = texto.replace(/^\uFEFF/, '')
  const primera = texto.split(/\r?\n/)[0] || ''
  const sep = (primera.match(/;/g) || []).length > (primera.match(/,/g) || []).length ? ';' : ','

  const filas = []
  let fila = []
  let celda = ''
  let comillas = false

  for (let i = 0; i < texto.length; i++) {
    const c = texto[i]
    if (comillas) {
      if (c === '"' && texto[i + 1] === '"') { celda += '"'; i++ }
      else if (c === '"') comillas = false
      else celda += c
    } else if (c === '"') comillas = true
    else if (c === sep) { fila.push(celda); celda = '' }
    else if (c === '\n' || c === '\r') {
      if (c === '\r' && texto[i + 1] === '\n') i++
      fila.push(celda); filas.push(fila); fila = []; celda = ''
    } else celda += c
  }
  if (celda || fila.length) { fila.push(celda); filas.push(fila) }
  return filas.filter((f) => f.some((c) => c.trim() !== ''))
}

const validar = (filas, tipo) => {
  const def = TIPOS_IMPORTACION[tipo]
  const encabezado = filas[0].map(normalizar)

  const idx = {}
  const faltantes = []
  for (const [campo, alias] of Object.entries(def.columnas)) {
    idx[campo] = encabezado.findIndex((h) => alias.includes(h))
    if (idx[campo] < 0) faltantes.push(alias[0])
  }
  if (faltantes.length) throw error(400, `Faltan columnas para ${tipo}: ${faltantes.join(', ')}`)

  const valor = (f, campo) => (idx[campo] >= 0 ? String(f[idx[campo]] ?? '').trim() : '')

  const errores = []
  const validas = []
  const vistas = new Set()

  filas.slice(1).forEach((f, i) => {
    const fila = i + 2
    const r = {
      fila,
      matricula: valor(f, 'matricula').toUpperCase(),
      nombre: valor(f, 'nombre'),
      password: valor(f, 'password'),
      grupo: tipo === 'alumnos' ? valor(f, 'grupo').toUpperCase() : null
    }
    const existente = buscar(r.matricula)

    let problema = null
    if (!r.matricula) problema = 'Matrícula/clave vacía'
    else if (!/^[A-Z0-9_-]{3,20}$/.test(r.matricula)) problema = 'Matrícula/clave inválida (3-20 letras o números)'
    else if (vistas.has(r.matricula)) problema = 'Repetida en el archivo'
    else if (existente && existente.tipo !== tipo) problema = `Ya existe como ${ETIQUETA_TIPO[existente.tipo]}`
    else if (!r.nombre) problema = 'Nombre vacío'
    else if (tipo === 'alumnos' && !r.grupo) problema = 'Grupo vacío'
    else if (tipo === 'alumnos' && !/^[A-Z0-9-]{2,15}$/.test(r.grupo)) problema = 'Grupo inválido (ej. IRD-71)'
    else if (!existente && !r.password) problema = 'Contraseña obligatoria para usuarios nuevos'
    else if (r.password && r.password.length < 8) problema = 'Contraseña de menos de 8 caracteres'

    if (problema) errores.push({ fila, matricula: r.matricula, mensaje: problema })
    else {
      vistas.add(r.matricula)
      validas.push({ ...r, accion: existente ? 'ACTUALIZAR' : 'CREAR' })
    }
  })

  const gruposNuevos = [...new Set(validas.map((r) => r.grupo).filter(Boolean))]
    .filter((g) => !GRUPOS.some((x) => x.codigo === g))

  return { total: filas.length - 1, validas, errores, gruposNuevos }
}

export const mockImportar = async (archivo, tipo, dryRun) => {
  await esperar(700)
  if (!TIPOS_IMPORTACION[tipo]) throw error(400, 'Tipo de importación no válido')
  if (!archivo.name.toLowerCase().endsWith('.csv')) {
    throw error(400, 'En modo simulado solo se lee CSV. Excel lo procesará el backend Go.')
  }

  const filas = parsearCSV(await archivo.text())
  if (filas.length < 2) throw error(400, 'El archivo no tiene filas de datos')

  const { total, validas, errores, gruposNuevos } = validar(filas, tipo)

  if (dryRun) {
    return {
      total,
      validos: validas.length,
      errores,
      grupos_nuevos: gruposNuevos,
      vista_previa: validas.map(({ password, ...r }) => r)
    }
  }

  gruposNuevos.forEach((codigo) => GRUPOS.push({ codigo, docentes: [] }))

  let creados = 0
  let actualizados = 0
  for (const r of validas) {
    const u = buscar(r.matricula)
    if (u) {
      // Los roles NO se tocan: pudieron ser ajustados por el admin
      u.nombre = r.nombre
      if (tipo === 'alumnos') u.grupo = r.grupo
      if (r.password) u.password = r.password
      actualizados++
    } else {
      USUARIOS.push({
        matricula: r.matricula, password: r.password, nombre: r.nombre,
        grupo: r.grupo, tipo, roles: [...TIPOS_IMPORTACION[tipo].roles], activo: true
      })
      creados++
    }
  }
  return { creados, actualizados, grupos_creados: gruposNuevos.length, errores }
}