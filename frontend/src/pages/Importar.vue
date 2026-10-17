<template>
  <AppFrame>
    <v-card border class="my-5" elevation="0" rounded="lg" title="Importar usuarios">
      <template #text>
        <p class="mb-4 text-medium-emphasis">
          Sube un archivo CSV o Excel. Columnas esperadas:
          <strong>{{ columnas[form.tipo] }}</strong>.
          Si la matrícula ya existe, se actualizan sus datos.
        </p>

        <v-row>
          <v-col cols="12" md="4">
            <v-select
              v-model="form.tipo"
              item-title="text"
              item-value="value"
              :items="tipos"
              label="Tipo de usuarios"
              variant="outlined"
            />
          </v-col>
          <v-col cols="12" md="8">
            <v-file-input
              v-model="form.archivo"
              accept=".csv,.xlsx"
              label="Archivo (.csv o .xlsx)"
              prepend-icon="mdi-file-delimited"
              show-size
              variant="outlined"
            />
          </v-col>
        </v-row>

        <v-btn
          color="primary"
          :disabled="!form.archivo || enCurso"
          :loading="subiendo"
          prepend-icon="mdi-upload"
          text="Importar"
          @click="subir"
        />
      </template>
    </v-card>

    <!-- Avance de la importación -->
    <v-card v-if="trabajo.id" border class="my-5" elevation="0" rounded="lg" :title="`Archivo: ${trabajo.archivo}`">
      <template #append>
        <v-chip :color="colorEstado" :text="trabajo.estado" label size="small" />
      </template>

      <template #text>
        <v-progress-linear class="mb-4" color="primary" height="10" :model-value="trabajo.porcentaje" rounded />

        <v-row>
          <v-col v-for="(item, i) in resumen" :key="i" cols="6" md="3">
            <v-list border elevation="0" lines="two" rounded="lg">
              <v-list-item>
                <v-list-item-title class="text-body-small">{{ item.titulo }}</v-list-item-title>
                <v-list-item-title>{{ item.valor }}</v-list-item-title>
                <template #append>
                  <v-icon :color="item.color" :icon="item.icon" size="30" />
                </template>
              </v-list-item>
            </v-list>
          </v-col>
        </v-row>

        <v-alert v-if="trabajo.mensaje" class="mt-4" type="error" variant="tonal">{{ trabajo.mensaje }}</v-alert>

        <v-data-table
          v-if="trabajo.errores.length"
          class="mt-4"
          density="compact"
          :headers="headersErrores"
          :items="trabajo.errores"
          items-per-page="10"
        />
      </template>
    </v-card>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import type { DataTableHeader } from 'vuetify'
import axios from 'axios'
import Swal from 'sweetalert2'
import { Importacion } from '@/models/Importacion'

const tipos = [
  { text: 'Alumnos', value: 'alumnos' },
  { text: 'Maestros', value: 'maestros' },
  { text: 'Administradores', value: 'admins' },
]

const columnas: Record<string, string> = {
  alumnos: 'matricula, nombre, contrasenia, grupo, carrera',
  maestros: 'clave, nombre, contrasenia',
  admins: 'clave, nombre, contrasenia',
}

const headersErrores: DataTableHeader[] = [
  { title: 'Fila', key: 'fila', align: 'start', width: 100 },
  { title: 'Error', key: 'mensaje', align: 'start' },
]

const form = reactive<{ tipo: string, archivo: File | null }>({ tipo: 'alumnos', archivo: null })
const subiendo = ref(false)
const trabajo = ref<Importacion>(new Importacion())
let timer: ReturnType<typeof setInterval> | undefined

const enCurso = computed(() => trabajo.value.estado == 'PROCESANDO')

const colorEstado = computed(() => {
  if (trabajo.value.estado == 'TERMINADO') return 'success'
  if (trabajo.value.estado == 'ERROR') return 'error'
  return 'info'
})

const resumen = computed(() => [
  { titulo: 'Filas leídas', valor: trabajo.value.leidas, icon: 'mdi-file-document-outline', color: 'blue' },
  { titulo: 'Nuevos', valor: trabajo.value.insertados, icon: 'mdi-account-plus-outline', color: 'green' },
  { titulo: 'Actualizados', valor: trabajo.value.actualizados, icon: 'mdi-account-sync-outline', color: 'orange' },
  { titulo: 'Con error', valor: trabajo.value.con_error, icon: 'mdi-alert-circle-outline', color: 'red' },
])

function subir () {
  if (!form.archivo) return

  const datos = new FormData()
  datos.append('tipo', form.tipo)
  datos.append('archivo', form.archivo)

  subiendo.value = true
  axios.post('/api/importar', datos)
    .then(response => {
      trabajo.value = new Importacion(response.data.id, form.tipo, form.archivo?.name, 'PROCESANDO')
      form.archivo = null
      consultar()
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'No se pudo importar', text: err.response?.data || 'Error al subir el archivo.' })
    })
    .finally(() => subiendo.value = false)
}

// Pregunta el avance cada segundo hasta que termine
function consultar () {
  clearInterval(timer)
  timer = setInterval(() => {
    axios.get(`/api/importar/${trabajo.value.id}`)
      .then(response => {
        trabajo.value = response.data
        if (trabajo.value.estado != 'PROCESANDO') {
          clearInterval(timer)
          Swal.fire({
            icon: trabajo.value.estado == 'TERMINADO' ? 'success' : 'error',
            title: trabajo.value.estado == 'TERMINADO' ? 'Importación terminada' : 'La importación falló',
            timer: 2000,
            showConfirmButton: false,
          })
        }
      })
      .catch(() => clearInterval(timer))
  }, 1000)
}

onBeforeUnmount(() => clearInterval(timer))
</script>
