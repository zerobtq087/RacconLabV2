<template>
  <AppFrame>
    <v-alert
      v-if="error"
      class="my-4"
      closable
      :text="error"
      type="error"
      variant="tonal"
      @click:close="error = ''"
    />

    <v-row class="my-2">
      <!-- Nueva práctica: sale de un proyecto de MI laboratorio -->
      <v-col cols="12" lg="5">
        <v-card border elevation="0" rounded="lg" title="Nueva práctica">
          <template #text>
            <v-alert v-if="!labListo" class="mb-4" density="compact" type="info" variant="tonal">
              La práctica se arma con un proyecto de tu laboratorio: necesitas tu laboratorio en ejecución
              (entra como maestro).
            </v-alert>
            <v-select
              v-model="form.proyecto"
              :disabled="!labListo"
              item-title="name"
              item-value="project_id"
              :items="proyectos"
              label="Proyecto de mi GNS3"
              no-data-text="No tienes proyectos en tu laboratorio"
              variant="outlined"
            >
              <template #item="{ props: p, item }">
                <v-list-item v-bind="p" :subtitle="`${item.nodos} nodos · ${item.encendidos} encendidos`" />
              </template>
            </v-select>
            <v-text-field v-model="form.nombre" counter="80" label="Nombre de la práctica" variant="outlined" />
            <v-select
              v-model="form.id_grupo"
              item-title="codigo"
              item-value="id"
              :items="grupos"
              label="Grupo"
              no-data-text="No tienes grupos asignados"
              variant="outlined"
            />
            <v-textarea
              v-model="form.instrucciones"
              auto-grow
              hint="Escribe aquí todo lo que deben configurar, contraseñas incluidas (ej. enable secret Redes2026)"
              label="Instrucciones para el alumno"
              persistent-hint
              rows="4"
              variant="outlined"
            />
            <div class="text-caption text-medium-emphasis">
              Al crearla se guarda la configuración de tus routers (write memory), se detiene el proyecto y se exporta.
              Después agregas los criterios y la activas.
            </div>
          </template>
          <template #actions>
            <v-btn
              block
              color="primary"
              :disabled="!labListo || !form.proyecto || !form.nombre || !form.id_grupo"
              :loading="creando"
              prepend-icon="mdi-plus"
              size="large"
              text="Crear práctica"
              variant="flat"
              @click="crear"
            />
          </template>
        </v-card>
      </v-col>

      <v-col cols="12" lg="7">
        <v-card border elevation="0" rounded="lg" title="Mis prácticas">
          <v-data-table
            :headers="headers"
            hover
            :items="practicas"
            items-per-page="10"
            no-data-text="Todavía no hay prácticas"
            @click:row="(_e: unknown, { item }: { item: Practica }) => router.push(`/practicas/${item.id}`)"
          >
            <template #item.estado="{ item }">
              <v-chip :color="estadosPractica[item.estado]?.color" size="small" variant="tonal">
                {{ estadosPractica[item.estado]?.texto }}
              </v-chip>
              <div v-if="item.estado == 'EXPORTANDO'" class="text-caption text-medium-emphasis">{{ item.paso }}…</div>
              <div v-if="item.estado == 'ERROR'" class="text-caption text-error">{{ item.mensaje }}</div>
            </template>
            <template #item.entregadas="{ item }">{{ item.entregadas }} / {{ item.alumnos }}</template>
            <template #item.creada="{ item }">{{ fecha(item.creada) }}</template>
          </v-data-table>
        </v-card>
      </v-col>
    </v-row>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { onBeforeUnmount, onMounted, ref } from 'vue'
import { useRouter } from 'vue-router'
import axios from 'axios'
import { estadosPractica, fecha, Practica, type ProyectoGNS3, SolicitudPractica } from '@/models/Practica'
import type { Grupo } from '@/models/Grupo'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const practicas = ref<Practica[]>([])
const proyectos = ref<ProyectoGNS3[]>([])
const grupos = ref<Grupo[]>([])
const form = ref(new SolicitudPractica())
const labListo = ref(false)
const creando = ref(false)
const error = ref('')
let temporizador: ReturnType<typeof setInterval> | undefined

const headers = [
  { title: 'Práctica', key: 'nombre' },
  { title: 'Grupo', key: 'grupo' },
  { title: 'Estado', key: 'estado' },
  { title: 'Entregas', key: 'entregadas' },
  { title: 'Creada', key: 'creada' },
]

function cargar () {
  axios.get('/api/practicas').then(response => {
    practicas.value = response.data
    // mientras alguna se exporta, se pregunta cada 2 s
    const exportando = practicas.value.some(p => p.estado == 'EXPORTANDO')
    if (exportando && !temporizador) temporizador = setInterval(cargar, 2000)
    if (!exportando && temporizador) {
      clearInterval(temporizador)
      temporizador = undefined
    }
  })
}

function cargarProyectos () {
  axios.get('/api/practicas/proyectos')
    .then(response => {
      proyectos.value = response.data
      labListo.value = true
    })
    .catch(() => labListo.value = false)
}

function crear () {
  creando.value = true
  error.value = ''
  axios.post('/api/practicas', form.value)
    .then(() => {
      form.value = new SolicitudPractica()
      cargar()
    })
    .catch(err => error.value = err.response?.data || 'No se pudo crear la práctica')
    .finally(() => creando.value = false)
}

onMounted(() => {
  cargar()
  axios.get('/api/grupos').then(response => grupos.value = response.data)
  if (authStore.userAcc.rol == 'PROFESOR') cargarProyectos()
})
onBeforeUnmount(() => clearInterval(temporizador))
</script>
