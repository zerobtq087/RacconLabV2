<template>
  <AppFrame>
    <v-alert v-if="error" class="my-4" closable :text="error" type="error" variant="tonal" @click:close="error = ''" />
    <v-alert v-if="aviso" class="my-4" closable :text="aviso" type="success" variant="tonal" @click:close="aviso = ''" />

    <v-alert v-if="cargado && practicas.length == 0" class="my-5" type="info" variant="tonal">
      Tu profesor todavía no ha activado prácticas para tu grupo.
    </v-alert>

    <v-card v-for="p in practicas" :key="p.id" border class="my-4" elevation="0" rounded="lg">
      <v-card-title class="d-flex align-center flex-wrap ga-2">
        {{ p.nombre }}
        <v-chip v-if="p.entregada && !p.tarde" color="success" size="small" variant="tonal">Entregada a tiempo</v-chip>
        <v-chip v-else-if="p.entregada" color="error" size="small" variant="tonal">Entregada {{ p.retraso }} tarde</v-chip>
        <v-chip v-else-if="p.estado == 'CERRADA'" color="warning" size="small" variant="tonal">Cerrada: tu entrega contará como tarde</v-chip>
        <v-chip v-else color="info" size="small" variant="tonal">Pendiente</v-chip>
        <v-spacer />
        <span class="text-body-2 text-medium-emphasis">{{ p.profesor }}</span>
      </v-card-title>

      <!-- Ya entregó: solo su resultado (ya no ve la práctica) -->
      <v-card-text v-if="p.entregada">
        <div class="text-h5 mb-1">{{ p.calificacion }} / 100</div>
        <div class="text-medium-emphasis mb-3">Entregada el {{ fecha(p.entregada) }}</div>
        <template v-if="p.fallidos.length">
          <div class="text-subtitle-2 mb-1">Lo que no se cumplió:</div>
          <ul class="ml-5">
            <li v-for="(f, i) in p.fallidos" :key="i">{{ f }}</li>
          </ul>
        </template>
        <div v-else class="text-success">Cumpliste todos los criterios.</div>
      </v-card-text>

      <template v-else>
        <v-card-text>
          <div class="instrucciones mb-4">{{ p.instrucciones || 'Sin instrucciones.' }}</div>

          <!-- Revisión en curso (o error de la última) -->
          <template v-if="p.revision">
            <template v-if="enCurso(p)">
              <v-progress-linear color="primary" height="22" :model-value="p.revision.porcentaje" rounded striped>
                <strong>{{ p.revision.porcentaje }}%</strong>
              </v-progress-linear>
              <div class="text-medium-emphasis mt-2">
                {{ p.revision.estado == 'EN_COLA' ? 'Esperando turno para revisar' : p.revision.paso }}…
                (entregó {{ p.revision.entrego }}; no muevas tu topología)
              </div>
            </template>
            <v-alert v-else-if="p.revision.estado == 'ERROR'" class="mb-2" type="error" variant="tonal">
              No se pudo revisar tu entrega: {{ p.revision.mensaje }}. No contó como entrega; puedes volver a entregar.
            </v-alert>
          </template>

          <!-- Confirmaciones dentro de la página -->
          <v-alert v-if="pregunta?.id == p.id" class="mt-3" type="warning" variant="tonal">
            {{ pregunta.texto }}
            <template #append>
              <v-btn class="mr-2" text="Cancelar" variant="text" @click="pregunta = null" />
              <v-btn color="primary" text="Sí, continuar" variant="flat" @click="pregunta.si()" />
            </template>
          </v-alert>
        </v-card-text>

        <v-card-actions v-if="!enCurso(p)" class="flex-wrap ga-2">
          <v-btn
            :disabled="!labListo"
            :loading="ocupado == p.id + 'abrir'"
            prepend-icon="mdi-open-in-app"
            text="Abrir en mi laboratorio"
            variant="tonal"
            @click="abrir(p, {})"
          />
          <v-btn
            :disabled="!labListo"
            prepend-icon="mdi-restart"
            text="Empezar de nuevo"
            variant="text"
            @click="preguntar(p, 'Se borra tu copia de la práctica (con todo lo que hayas hecho) y se vuelve a abrir la original.', () => abrir(p, { empezar_de_nuevo: true }))"
          />
          <v-spacer />
          <v-btn
            color="primary"
            :disabled="!labListo"
            :loading="ocupado == p.id + 'entregar'"
            prepend-icon="mdi-send"
            text="Entregar"
            variant="flat"
            @click="preguntar(p, textoEntregar, () => entregar(p))"
          />
        </v-card-actions>
      </template>
    </v-card>

    <v-alert v-if="cargado && practicas.length && !labListo" class="my-4" type="info" variant="tonal">
      Para abrir o entregar una práctica necesitas tu laboratorio en ejecución.
      <template #append>
        <v-btn color="primary" text="Ir a mi laboratorio" to="/laboratorio" />
      </template>
    </v-alert>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import axios from 'axios'
import { fecha, type MiPractica } from '@/models/Practica'

interface Pregunta { id: string, texto: string, si: () => void }

const practicas = ref<MiPractica[]>([])
const labListo = ref(false)
const cargado = ref(false)
const error = ref('')
const aviso = ref('')
const ocupado = ref('')
const pregunta = ref<Pregunta | null>(null)
let temporizador: ReturnType<typeof setInterval> | undefined

const textoEntregar = 'Solo se entrega UNA vez (también cuenta para tu equipo). Se revisa tu topología tal como está ahora; ' +
  'al terminar, la práctica sale de tu laboratorio y ya no podrás verla ni modificarla.'

function enCurso (p: MiPractica) {
  return p.revision?.estado == 'EN_COLA' || p.revision?.estado == 'REVISANDO'
}

function cargar () {
  axios.get('/api/mis-practicas').then(response => {
    practicas.value = response.data
    cargado.value = true
    // mientras se revisa una entrega, se pregunta cada 2 s
    const revisando = practicas.value.some(enCurso)
    if (revisando && !temporizador) temporizador = setInterval(cargar, 2000)
    if (!revisando && temporizador) {
      clearInterval(temporizador)
      temporizador = undefined
    }
  })
}

function preguntar (p: MiPractica, texto: string, si: () => void) {
  pregunta.value = { id: p.id, texto, si: () => {
    pregunta.value = null
    si()
  } }
}

function mensaje (err: { response?: { data?: unknown } }) {
  const d = err.response?.data
  return typeof d == 'string' ? d : 'No se pudo completar'
}

function abrir (p: MiPractica, opciones: { detener_otros?: boolean, empezar_de_nuevo?: boolean }) {
  ocupado.value = p.id + 'abrir'
  error.value = ''
  aviso.value = ''
  axios.post(`/api/mis-practicas/${p.id}/abrir`, opciones)
    .then(response => aviso.value = `Listo: "${response.data.proyecto}" está abierta y encendida en tu laboratorio. ` +
      'En GNS3 de escritorio ábrela con File → Open project (o recarga la lista de proyectos).')
    .catch(err => {
      // Otra topología encendida: se pregunta antes de detenerla
      if (err.response?.status == 409 && err.response.data?.proyectos) {
        preguntar(p, err.response.data.mensaje + ' ¿Detenerla y abrir la práctica?',
          () => abrir(p, { ...opciones, detener_otros: true }))
        return
      }
      error.value = mensaje(err)
    })
    .finally(() => ocupado.value = '')
}

function entregar (p: MiPractica) {
  ocupado.value = p.id + 'entregar'
  error.value = ''
  aviso.value = ''
  axios.post(`/api/mis-practicas/${p.id}/entregar`)
    .then(cargar)
    .catch(err => error.value = mensaje(err))
    .finally(() => ocupado.value = '')
}

onMounted(() => {
  cargar()
  axios.get('/api/labs').then(response => labListo.value = response.data.estado == 'CORRIENDO').catch(() => {})
})
onBeforeUnmount(() => clearInterval(temporizador))
</script>

<style scoped>
.instrucciones {
  white-space: pre-wrap;
}
</style>
