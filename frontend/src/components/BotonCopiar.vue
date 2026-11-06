<template>
  <!-- Copia "texto"; el botón cambia a palomita + "Copiado" durante 2 s (sin ventanas).
       Sin "etiqueta" es solo un ícono; con "etiqueta" (ej. "Copiar tabla") lleva texto. -->
  <v-btn
    :color="copiado ? 'success' : undefined"
    density="comfortable"
    :icon="etiqueta ? undefined : icono"
    :prepend-icon="etiqueta ? icono : undefined"
    size="small"
    :text="etiqueta ? (copiado ? 'Copiado' : etiqueta) : undefined"
    :title="copiado ? 'Copiado' : 'Copiar'"
    variant="text"
    @click="copiar"
  />
</template>

<script lang="ts" setup>
import { computed, onBeforeUnmount, ref } from 'vue'

const props = defineProps<{
  texto: string
  etiqueta?: string // si viene, el botón muestra texto (ej. "Copiar tabla"); si no, solo el ícono
}>()

const copiado = ref(false)
const icono = computed(() => copiado.value ? 'mdi-check' : 'mdi-content-copy')
let timer: ReturnType<typeof setTimeout> | undefined

// navigator.clipboard solo existe en HTTPS o localhost; la página va por http://IP,
// así que si no está se usa el método viejo: un textarea oculto + execCommand('copy')
function copiar () {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(props.texto)
      .then(marcar)
      .catch(() => respaldo() && marcar())
  } else if (respaldo()) {
    marcar()
  }
}

function respaldo (): boolean {
  const area = document.createElement('textarea')
  area.value = props.texto
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.opacity = '0'
  document.body.append(area)
  area.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {
    ok = false
  }
  area.remove()
  return ok
}

function marcar () {
  copiado.value = true
  clearTimeout(timer)
  timer = setTimeout(() => copiado.value = false, 2000)
}

onBeforeUnmount(() => clearTimeout(timer))
</script>
