import { defineStore } from 'pinia'
import http, { setAccessToken, refrescarSesion, registrarExpiracion } from '@/api/http'
import { mockLogin } from '@/api/mock'
import { normalizarRoles } from '@/utils/roles'

// El login tiene su PROPIO interruptor de mock.
// VITE_MOCK_AUTH=true  -> login simulado
// sin definir / false  -> login real (Firebird + Redis)
const MOCK = import.meta.env.VITE_MOCK_AUTH === 'true'
const CLAVE_MOCK = 'raccoon_mock_sesion'

export const useAuth = defineStore('auth', {
  state: () => ({
    usuario: null, // { matricula, nombre, grupo, tipo, roles[], rol_activo }
    restaurada: false
  }),

  getters: {
    autenticado: (s) => !!s.usuario,
    rol: (s) => s.usuario?.rol_activo || null,
    esAdmin: (s) => s.usuario?.rol_activo === 'ADMIN',
    esProfesor: (s) => s.usuario?.rol_activo === 'PROFESOR',
    // Exactamente los roles que el admin le asignó
    rolesDisponibles: (s) => (s.usuario ? normalizarRoles(s.usuario.roles) : [])
  },

  actions: {
    _aplicar(data) {
      setAccessToken(data.access_token)
      this.usuario = data.usuario
      if (MOCK) sessionStorage.setItem(CLAVE_MOCK, JSON.stringify(data.usuario))
    },

    async login(matricula, password, rol) {
      const data = MOCK
        ? await mockLogin(matricula.trim(), password, rol)
        : (await http.post('/auth/login', { matricula: matricula.trim(), password, rol })).data
      this._aplicar(data)
    },

    async restaurar() {
      if (this.restaurada) return
      registrarExpiracion(() => this.limpiar(true))
      try {
        this.usuario = MOCK
          ? JSON.parse(sessionStorage.getItem(CLAVE_MOCK) || 'null')
          : (await refrescarSesion()).usuario
      } catch {
        this.usuario = null
      } finally {
        this.restaurada = true
      }
    },

    async cambiarRol(rol) {
      if (!this.rolesDisponibles.includes(rol)) throw new Error('No tienes asignado ese rol')
      if (MOCK) return this._aplicar({ access_token: 'mock-token', usuario: { ...this.usuario, rol_activo: rol } })
      this._aplicar((await http.post('/auth/cambiar-rol', { rol })).data)
    },

    async logout() {
      if (!MOCK) {
        try { await http.post('/auth/logout') } catch { /* ya no existía */ }
      }
      this.limpiar(false)
    },

    limpiar(redirigir) {
      setAccessToken(null)
      this.usuario = null
      sessionStorage.removeItem(CLAVE_MOCK)
      if (redirigir && window.location.pathname !== '/login') {
        window.location.href = '/login?expirada=1'
      }
    }
  }
})
