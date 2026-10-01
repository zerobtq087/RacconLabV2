// Roles independientes: cada usuario tiene una lista explícita.
// ALUMNO siempre está incluido; PROFESOR y ADMIN se asignan por separado.
export const ROLES_UI = [
  { value: 'ALUMNO', title: 'Alumno', icon: 'mdi-school' },
  { value: 'PROFESOR', title: 'Maestro', icon: 'mdi-human-male-board' },
  { value: 'ADMIN', title: 'Admin', icon: 'mdi-shield-crown' }
]

const ORDEN = { ADMIN: 0, PROFESOR: 1, ALUMNO: 2 }

/** Normaliza: sin repetidos, siempre con ALUMNO, ordenado ADMIN > PROFESOR > ALUMNO */
export const normalizarRoles = (roles = []) =>
  [...new Set([...roles, 'ALUMNO'])]
    .filter((r) => r in ORDEN)
    .sort((a, b) => ORDEN[a] - ORDEN[b])

export const etiquetaRol = (rol) => ROLES_UI.find((r) => r.value === rol)?.title || rol

export const colorRol = (rol) =>
  ({ ADMIN: 'error', PROFESOR: 'secondary', ALUMNO: 'success' })[rol] || 'grey'