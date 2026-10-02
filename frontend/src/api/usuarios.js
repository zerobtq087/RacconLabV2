import http from './http'

/*
 * Usuarios: TODO es real (Firebird). Ya no usa mock.
 */
export const usuariosApi = {
  /**
   * Página de usuarios (la pagina Firebird).
   * params = { pagina, por_pagina, q, tipo, rol, grupo, orden, desc }
   * -> { items: [...], total }
   */
  async listar(params = {}) {
    return (await http.get('/admin/usuarios', { params })).data
  },

  /** -> { usuarios, profesores, admins, inactivos, grupos: [] } */
  async resumen() {
    return (await http.get('/admin/usuarios/resumen')).data
  },

  /** cambios = { matricula, roles: ['ADMIN','PROFESOR','ALUMNO'], activo } */
  async actualizar(cambios) {
    return (await http.put('/admin/usuarios/editar', cambios)).data
  },

  /** El admin pone una contraseña nueva sin conocer la anterior */
  async resetPassword(matricula, nueva) {
    await http.put('/admin/usuarios/password', { matricula, nueva_password: nueva })
  },

  /**
   * Importación masiva.
   * tipo:     'alumnos' | 'maestros' | 'admins'
   * onSubida: callback con el % de subida (0-100)
   * Responde de inmediato con el trabajo { id, estado: 'procesando', ... }
   */
  async importar(archivo, tipo, onSubida) {
    const fd = new FormData()
    fd.append('archivo', archivo)
    const { data } = await http.post(`/admin/importar/${tipo}`, fd, {
      headers: { 'Content-Type': 'multipart/form-data' },
      timeout: 0, // 100 MB pueden tardar en subir
      onUploadProgress: (e) => {
        if (onSubida && e.total) onSubida(Math.round((e.loaded * 100) / e.total))
      }
    })
    return data
  },

  /** Avance en vivo de una importación */
  async estadoImportacion(id) {
    return (await http.get(`/admin/importar/${id}`)).data
  }
}
