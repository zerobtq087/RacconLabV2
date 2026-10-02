<template>
  <v-text-field
    :model-value="valor"
    :label="label"
    :type="secreto && !visible ? 'password' : 'text'"
    readonly
    class="font-mono"
    hide-details
  >
    <template #append-inner>
      <v-btn v-if="secreto" :icon="visible ? 'mdi-eye-off' : 'mdi-eye'" variant="text" size="small" @click="visible = !visible" />
      <v-btn icon="mdi-content-copy" variant="text" size="small" @click="copiar" />
    </template>
  </v-text-field>
</template>

<script lang="ts" setup>
import { ref } from 'vue'
import { useNotificaciones } from '@/stores/notificaciones'

const props = defineProps({
  valor: { type: [String, Number], default: '' },
  label: { type: String, default: '' },
  secreto: { type: Boolean, default: false }
})

const noti = useNotificaciones()
const visible = ref(false)

// navigator.clipboard solo funciona en HTTPS/localhost; en la LAN por HTTP usa el respaldo
const copiar = async () => {
  const texto = String(props.valor ?? '')
  if (!texto) return
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(texto)
    } else {
      const ta = document.createElement('textarea')
      ta.value = texto
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    noti.exito('Copiado')
  } catch {
    noti.error('No se pudo copiar')
  }
}
</script>