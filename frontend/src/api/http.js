import axios from 'axios'

// Access token SOLO en memoria; refresh token = cookie HttpOnly que pone la API
let accessToken = null
let refreshEnCurso = null
let alExpirarSesion = () => {}

export const setAccessToken = (t) => { accessToken = t }
export const registrarExpiracion = (fn) => { alExpirarSesion = fn }

const http = axios.create({
  baseURL: import.meta.env.VITE_API_BASE_URL || '/api',
  withCredentials: true,
  timeout: 30000,
  headers: { 'Content-Type': 'application/json' }
})

http.interceptors.request.use((config) => {
  if (accessToken) config.headers.Authorization = `Bearer ${accessToken}`
  return config
})

export const refrescarSesion = async () => {
  if (!refreshEnCurso) {
    refreshEnCurso = axios
      .post(`${http.defaults.baseURL}/auth/refresh`, {}, { withCredentials: true })
      .then((r) => { setAccessToken(r.data.access_token); return r.data })
      .finally(() => { refreshEnCurso = null })
  }
  return refreshEnCurso
}

// Si una petición da 401, intenta renovar la sesión UNA vez y reintenta
http.interceptors.response.use(
  (r) => r,
  async (error) => {
    const original = error.config || {}
    const esAuth = ['/auth/login', '/auth/refresh', '/auth/logout'].some((p) => original.url?.includes(p))
    if (error.response?.status === 401 && !original._reintento && !esAuth) {
      original._reintento = true
      try {
        await refrescarSesion()
        return http(original)
      } catch {
        alExpirarSesion()
      }
    }
    return Promise.reject(error)
  }
)

export const mensajeError = (err, porDefecto = 'Ocurrió un error inesperado') =>
  err?.response?.data?.message ||
  err?.response?.data?.error ||
  (err?.code === 'ERR_NETWORK' ? 'No hay conexión con el servidor' : null) ||
  err?.message ||
  porDefecto

export default http