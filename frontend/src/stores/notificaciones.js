import { defineStore } from 'pinia'

export const useNotificaciones = defineStore('notificaciones', {
  state: () => ({ visible: false, texto: '', color: 'error', timeout: 4500 }),
  actions: {
    mostrar(texto, color = 'error', timeout = 4500) {
      Object.assign(this, { texto, color, timeout, visible: true })
    },
    exito(texto) { this.mostrar(texto, 'success') },
    error(texto) { this.mostrar(texto, 'error', 6000) },
    info(texto) { this.mostrar(texto, 'info') }
  }
})