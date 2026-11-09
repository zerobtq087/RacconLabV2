<template>
  <AppFrame>
    <v-alert v-if="error" class="my-4" closable :text="error" type="error" variant="tonal" @click:close="error = ''" />
    <v-alert v-if="aviso" class="my-4" closable :text="aviso" type="success" variant="tonal" @click:close="aviso = ''" />

    <v-tabs v-model="tab" class="my-2" color="primary">
      <v-tab text="Laboratorios de mis alumnos" value="labs" />
      <v-tab text="Bitácora" value="bitacora" @click="cargarBitacora" />
    </v-tabs>

    <v-window v-model="tab">
      <v-window-item value="labs">
        <v-row class="my-2">
          <!-- 1. Grupo y alumnos -->
          <v-col cols="12" lg="4">
            <v-card border elevation="0" rounded="lg">
              <v-card-text>
                <v-select
                  v-model="grupo"
                  item-title="codigo"
                  item-value="id"
                  :items="grupos"
                  label="Grupo"
                  no-data-text="No tienes grupos asignados"
                  variant="outlined"
                  @update:model-value="cargarAlumnos"
                />
              </v-card-text>
              <v-list density="compact" nav>
                <v-list-item
                  v-for="a in alumnos"
                  :key="a.id_usuario"
                  :active="alumno?.id_usuario == a.id_usuario"
                  :disabled="!a.id_lab"
                  :subtitle="a.matricula + (a.id_lab ? (a.es_dueno ? ' · dueño del lab' : ' · en equipo') : ' · sin laboratorio')"
                  :title="a.nombre"
                  @click="elegirAlumno(a)"
                >
                  <template #append>
                    <v-chip v-if="a.id_lab" :color="a.estado == 'CORRIENDO' ? 'success' : 'grey'" size="x-small" variant="tonal">
                      {{ a.estado }}
                    </v-chip>
                  </template>
                </v-list-item>
              </v-list>
            </v-card>
          </v-col>

          <!-- 2. Proyectos del lab y sus nodos -->
          <v-col cols="12" lg="8">
            <v-card v-if="alumno" border elevation="0" rounded="lg">
              <v-card-title class="d-flex align-center flex-wrap ga-2">
                Laboratorio de {{ alumno.nombre }}
                <v-spacer />
                <v-btn
                  v-if="!consolas"
                  prepend-icon="mdi-console"
                  size="small"
                  text="Abrir consolas y GNS3 a mi IP (30 min)"
                  variant="tonal"
                  @click="abrirConsolas"
                />
                <v-btn
                  v-else
                  color="warning"
                  prepend-icon="mdi-console"
                  size="small"
                  text="Cerrar acceso"
                  variant="tonal"
                  @click="cerrarConsolas"
                />
              </v-card-title>
              <v-card-text>
                <v-alert v-if="consolas" class="mb-4" density="compact" type="info" variant="tonal">
                  Este laboratorio está abierto para tu IP hasta las {{ consolas.hasta }}: las consolas de sus nodos
                  (<code>telnet {{ servidor }} &lt;puerto&gt;</code>, el puerto de cada nodo está abajo) y el puerto
                  {{ consolas.puerto }} de su GNS3, que sigue pidiendo la contraseña del alumno (tú no la recibes).
                  Al terminar ciérralo para que vuelva a ser solo del alumno.
                </v-alert>

                <v-select
                  v-model="proyecto"
                  item-title="name"
                  item-value="project_id"
                  :items="proyectos"
                  label="Proyecto"
                  no-data-text="El alumno no tiene proyectos"
                  variant="outlined"
                  @update:model-value="cargarNodos(false)"
                >
                  <template #item="{ props: p, item }">
                    <v-list-item v-bind="p" :subtitle="`${item.nodos} nodos · ${item.encendidos} encendidos`" />
                  </template>
                </v-select>

                <template v-if="proyecto">
                  <div class="d-flex flex-wrap ga-2 mb-3">
                    <v-btn-toggle v-model="actual" density="compact" mandatory variant="outlined" @update:model-value="cargarNodos(actual)">
                      <v-btn :value="false" text="Config guardada" />
                      <v-btn :value="true" text="Config actual (consola)" />
                    </v-btn-toggle>
                    <v-spacer />
                    <v-btn prepend-icon="mdi-download" text="Descargar proyecto" variant="tonal" @click="exportar('descargar', false)" />
                    <v-btn
                      v-if="authStore.userAcc.rol == 'PROFESOR'"
                      prepend-icon="mdi-content-copy"
                      text="Copiar a mi laboratorio"
                      variant="tonal"
                      @click="exportar('copiar', false)"
                    />
                  </div>

                  <!-- GNS3 solo exporta proyectos detenidos: se pregunta antes de detener -->
                  <v-alert v-if="pendiente" class="mb-3" type="warning" variant="tonal">
                    {{ pendiente.mensaje }}
                    <template #append>
                      <v-btn class="mr-2" text="Cancelar" variant="text" @click="pendiente = null" />
                      <v-btn color="warning" text="Detener y continuar" variant="flat" @click="exportar(pendiente.que, true)" />
                    </template>
                  </v-alert>

                  <v-progress-linear v-if="cargando" class="mb-2" color="primary" indeterminate />
                  <v-expansion-panels variant="accordion">
                    <v-expansion-panel v-for="n in nodos" :key="n.nombre">
                      <v-expansion-panel-title>
                        <v-icon class="mr-2" :color="n.estado == 'started' ? 'success' : 'grey'" icon="mdi-circle" size="x-small" />
                        {{ n.nombre }}
                        <span class="text-medium-emphasis ml-2">{{ n.tipo }}</span>
                        <v-spacer />
                        <span v-if="n.consola" class="text-medium-emphasis mr-3">consola {{ n.consola }}</span>
                      </v-expansion-panel-title>
                      <v-expansion-panel-text>
                        <v-alert v-if="n.error" class="mb-2" density="compact" :text="n.error" type="warning" variant="tonal" />
                        <pre v-if="n.config" class="config">{{ n.config }}</pre>
                        <div v-else-if="!n.error" class="text-medium-emphasis">Sin configuración guardada.</div>
                      </v-expansion-panel-text>
                    </v-expansion-panel>
                  </v-expansion-panels>
                  <div class="text-caption text-medium-emphasis mt-2">
                    "Config actual" lee la running-config por la consola de cada router encendido. Todo lo que consultas queda en la bitácora.
                  </div>
                </template>
              </v-card-text>
            </v-card>
            <v-alert v-else type="info" variant="tonal">Elige un grupo y un alumno con laboratorio.</v-alert>
          </v-col>
        </v-row>
      </v-window-item>

      <v-window-item value="bitacora">
        <v-data-table :headers="headersBitacora" :items="bitacora" items-per-page="25" no-data-text="Sin registros">
          <template #item.fecha="{ item }">{{ fecha(item.fecha) }}</template>
        </v-data-table>
      </v-window-item>
    </v-window>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { onMounted, ref } from 'vue'
import axios from 'axios'
import { type AlumnoLab, type EntradaBitacora, fecha, type NodoSupervision, type ProyectoGNS3 } from '@/models/Practica'
import type { Grupo } from '@/models/Grupo'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const servidor = window.location.hostname

const tab = ref('labs')
const grupos = ref<Grupo[]>([])
const grupo = ref('')
const alumnos = ref<AlumnoLab[]>([])
const alumno = ref<AlumnoLab | null>(null)
const proyectos = ref<ProyectoGNS3[]>([])
const proyecto = ref('')
const nodos = ref<NodoSupervision[]>([])
const actual = ref(false)
const cargando = ref(false)
const consolas = ref<{ hasta: string, puerto: number } | null>(null)
const pendiente = ref<{ que: string, mensaje: string } | null>(null)
const bitacora = ref<EntradaBitacora[]>([])
const error = ref('')
const aviso = ref('')

const headersBitacora = [
  { title: 'Fecha', key: 'fecha' },
  { title: 'Docente', key: 'docente' },
  { title: 'Alumno', key: 'alumno' },
  { title: 'Acción', key: 'accion' },
  { title: 'Detalle', key: 'detalle' },
]

function mensaje (err: { response?: { data?: unknown } }) {
  const d = err.response?.data
  return typeof d == 'string' ? d : 'No se pudo completar'
}

function cargarAlumnos () {
  alumno.value = null
  axios.get(`/api/supervision/grupos/${grupo.value}`)
    .then(response => alumnos.value = response.data)
    .catch(err => error.value = mensaje(err))
}

function elegirAlumno (a: AlumnoLab) {
  alumno.value = a
  proyecto.value = ''
  nodos.value = []
  consolas.value = null
  pendiente.value = null
  error.value = ''
  axios.get(`/api/supervision/labs/${a.id_lab}/proyectos`)
    .then(response => proyectos.value = response.data)
    .catch(err => {
      proyectos.value = []
      error.value = mensaje(err)
    })
}

function cargarNodos (deConsola: boolean) {
  if (!alumno.value || !proyecto.value) return
  actual.value = deConsola
  cargando.value = true
  nodos.value = []
  pendiente.value = null
  axios.get(`/api/supervision/labs/${alumno.value.id_lab}/proyectos/${proyecto.value}/nodos`, { params: { actual: deConsola ? 1 : 0 } })
    .then(response => nodos.value = response.data)
    .catch(err => error.value = mensaje(err))
    .finally(() => cargando.value = false)
}

// Descargar o copiar a mi lab; si el proyecto está encendido, primero se pregunta
function exportar (que: string, detener: boolean) {
  if (!alumno.value) return
  const ruta = `/api/supervision/labs/${alumno.value.id_lab}/proyectos/${proyecto.value}/${que}`
  pendiente.value = null
  error.value = ''
  aviso.value = ''
  cargando.value = true
  const peticion = que == 'descargar'
    ? axios.get(ruta, { params: { detener: detener ? 1 : 0 }, responseType: 'blob' })
    : axios.post(ruta, null, { params: { detener: detener ? 1 : 0 } })
  peticion
    .then(response => {
      if (que == 'copiar') {
        aviso.value = `Se copió como "${response.data.proyecto}" en tu laboratorio: ábrelo en GNS3 (File → Open project).`
        return
      }
      const nombre = proyectos.value.find(p => p.project_id == proyecto.value)?.name || 'proyecto'
      const a = document.createElement('a')
      a.href = URL.createObjectURL(response.data)
      a.download = `${alumno.value?.matricula} - ${nombre}.gns3project`
      a.click()
      URL.revokeObjectURL(a.href)
    })
    .catch(async err => {
      // con responseType blob, el texto del error llega como Blob
      const texto = err.response?.data instanceof Blob ? await err.response.data.text() : mensaje(err)
      if (err.response?.status == 409 && texto.includes('detener')) {
        pendiente.value = { que, mensaje: texto }
        return
      }
      error.value = texto
    })
    .finally(() => {
      cargando.value = false
      if (detener) elegirProyectoOtraVez()
    })
}

function elegirProyectoOtraVez () {
  if (!alumno.value) return
  axios.get(`/api/supervision/labs/${alumno.value.id_lab}/proyectos`).then(response => proyectos.value = response.data)
  cargarNodos(false)
}

function abrirConsolas () {
  if (!alumno.value) return
  axios.post(`/api/supervision/labs/${alumno.value.id_lab}/consolas`)
    .then(response => consolas.value = {
      hasta: new Date(response.data.hasta).toLocaleTimeString('es-MX', { timeStyle: 'short' }),
      puerto: response.data.puerto,
    })
    .catch(err => error.value = mensaje(err))
}

function cerrarConsolas () {
  if (!alumno.value) return
  axios.delete(`/api/supervision/labs/${alumno.value.id_lab}/consolas`).then(() => consolas.value = null)
}

function cargarBitacora () {
  axios.get('/api/supervision/bitacora').then(response => bitacora.value = response.data)
}

onMounted(() => {
  axios.get('/api/grupos').then(response => grupos.value = response.data)
})
</script>

<style scoped>
.config {
  font-family: monospace;
  font-size: 0.8rem;
  white-space: pre-wrap;
  max-height: 400px;
  overflow: auto;
}
</style>
