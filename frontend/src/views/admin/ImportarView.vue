<template>
  <div class="mb-4">
    <div class="page-title">Importar usuarios</div>
    <div class="page-subtitle">Cada tipo de usuario se sube en su propio archivo CSV o Excel.</div>
  </div>

  <v-tabs v-model="tipo" color="primary" class="mb-4" :disabled="paso > 1">
    <v-tab v-for="(t, clave) in TIPOS" :key="clave" :value="clave" :prepend-icon="t.icono">{{ t.titulo }}</v-tab>
  </v-tabs>

  <v-stepper v-model="paso" :items="['Archivo', 'Importando', 'Resultado']" hide-actions flat class="bg-transparent">
    <!-- 1. Archivo -->
    <template #item.1>
      <v-row>
        <v-col cols="12" md="7">
          <v-card class="pa-6">
            <div
              class="zona pa-10 text-center"
              :class="{ activa: arrastrando }"
              @click="inputRef?.click()"
              @dragover.prevent="arrastrando = true"
              @dragleave.prevent="arrastrando = false"
              @drop.prevent="soltar"
            >
              <v-icon :icon="actual.icono" size="56" color="primary" />
              <div class="text-h6 mt-3">Archivo de {{ actual.titulo.toLowerCase() }}</div>
              <div class="text-medium-emphasis">Arrastra o haz clic · .csv o .xlsx · máx. 100 MB</div>
              <input ref="inputRef" type="file" hidden accept=".csv,.xlsx" @change="elegir" />
            </div>

            <v-alert v-if="archivo" type="info" variant="tonal" class="mt-4" :icon="iconoArchivo">
              <div class="d-flex align-center justify-space-between">
                <div>
                  <div class="font-weight-bold">{{ archivo.name }}</div>
                  <div class="text-caption">{{ tamano(archivo.size) }}</div>
                </div>
                <v-btn icon="mdi-close" variant="text" size="small" @click="archivo = null" />
              </div>
            </v-alert>

            <v-btn
              block size="large" color="primary" class="mt-4" prepend-icon="mdi-database-import"
              :disabled="!archivo" @click="importar"
            >
              Importar {{ actual.titulo.toLowerCase() }}
            </v-btn>
          </v-card>
        </v-col>

        <v-col cols="12" md="5">
          <v-card :title="`Formato · ${actual.titulo}`" prepend-icon="mdi-table">
            <v-card-text>
              <v-table density="compact" class="mb-4">
                <thead><tr><th>Columna</th><th>Ejemplo</th></tr></thead>
                <tbody>
                  <tr v-for="c in actual.columnas" :key="c.nombre">
                    <td class="font-mono">{{ c.nombre }}</td>
                    <td class="font-mono text-caption">{{ c.ejemplo }}</td>
                  </tr>
                </tbody>
              </v-table>
              <ul class="text-caption text-medium-emphasis pl-4 mb-4">
                <li>La primera fila puede ser encabezado; acentos y mayúsculas no importan.</li>
                <li>Separador: coma o punto y coma. Excel: se lee la primera hoja.</li>
                <li>Podrán entrar como: <b>{{ actual.acceso }}</b>.</li>
                <li v-if="tipo === 'alumnos'">Los grupos que no existan se crean solos.</li>
                <li v-else>La asignación a grupos se hace en <b>Grupos</b>.</li>
                <li>Si la {{ tipo === 'alumnos' ? 'matrícula' : 'clave' }} ya existe, se actualiza y se cierran sus sesiones.</li>
                <li>La contraseña es obligatoria en cada fila.</li>
              </ul>
              <v-btn variant="tonal" prepend-icon="mdi-download" block @click="descargarPlantilla">
                Descargar plantilla de {{ actual.titulo.toLowerCase() }}
              </v-btn>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </template>

    <!-- 2. Importando (avance en vivo) -->
    <template #item.2>
      <v-card class="pa-6">
        <div class="d-flex align-center mb-2">
          <v-icon :icon="iconoArchivo" class="mr-2" />
          <span class="font-weight-bold">{{ trabajo?.archivo || archivo?.name }}</span>
        </div>

        <!-- Subida -->
        <div class="text-caption mb-1">
          {{ subida < 100 ? 'Subiendo archivo…' : 'Archivo subido' }} ({{ subida }}%)
        </div>
        <v-progress-linear :model-value="subida" color="info" height="8" rounded class="mb-5" />

        <!-- Procesamiento -->
        <div class="text-caption mb-1">
          Procesando: cifrando contraseñas y guardando en la base de datos ({{ trabajo?.porcentaje ?? 0 }}%)
        </div>
        <v-progress-linear
          :model-value="trabajo?.porcentaje ?? 0"
          :indeterminate="subida === 100 && !trabajo"
          color="primary" height="14" rounded striped class="mb-6"
        />

        <v-row>
          <v-col cols="6" md="3"><div class="text-caption">Filas leídas</div><div class="text-h5 font-weight-bold">{{ num(trabajo?.leidas) }}</div></v-col>
          <v-col cols="6" md="3"><div class="text-caption">Nuevos</div><div class="text-h5 font-weight-bold text-success">{{ num(trabajo?.insertados) }}</div></v-col>
          <v-col cols="6" md="3"><div class="text-caption">Actualizados</div><div class="text-h5 font-weight-bold text-warning">{{ num(trabajo?.actualizados) }}</div></v-col>
          <v-col cols="6" md="3"><div class="text-caption">Con error</div><div class="text-h5 font-weight-bold text-error">{{ num(trabajo?.con_error) }}</div></v-col>
        </v-row>

        <div class="text-caption text-medium-emphasis mt-4">
          Puedes salir de esta pantalla: la importación sigue en el servidor y al volver verás su avance.
        </div>
      </v-card>
    </template>

    <!-- 3. Resultado -->
    <template #item.3>
      <v-card class="pa-8 text-center mb-4">
        <template v-if="trabajo?.estado === 'terminado'">
          <v-icon icon="mdi-check-circle" color="success" size="64" />
          <div class="text-h5 mt-4">Importación de {{ TIPOS[trabajo.tipo as TipoClave].titulo.toLowerCase() }} terminada</div>
          <div class="text-caption text-medium-emphasis mb-6">{{ trabajo.archivo }} · {{ duracion }}</div>
        </template>
        <template v-else>
          <v-icon icon="mdi-alert-circle" color="error" size="64" />
          <div class="text-h5 mt-4">La importación falló</div>
          <div class="text-error mb-6">{{ trabajo?.mensaje }}</div>
        </template>

        <v-row justify="center">
          <v-col cols="6" md="2"><div class="text-h4 text-success">{{ num(trabajo?.insertados) }}</div><div class="text-caption">nuevos</div></v-col>
          <v-col cols="6" md="2"><div class="text-h4 text-warning">{{ num(trabajo?.actualizados) }}</div><div class="text-caption">actualizados</div></v-col>
          <v-col cols="6" md="2"><div class="text-h4 text-error">{{ num(trabajo?.con_error) }}</div><div class="text-caption">con error</div></v-col>
        </v-row>
        <v-btn variant="tonal" class="mt-6" @click="reiniciar">Importar otro archivo</v-btn>
      </v-card>

      <v-card v-if="trabajo?.errores?.length" color="error" variant="tonal"
        title="Filas que NO se importaron" prepend-icon="mdi-alert-circle">
        <v-card-text>
          <div v-if="(trabajo.con_error ?? 0) > trabajo.errores.length" class="text-caption mb-2">
            Se muestran los primeros {{ trabajo.errores.length }} de {{ num(trabajo.con_error) }} errores.
          </div>
          <v-data-table :headers="headersErrores" :items="trabajo.errores" density="compact" :items-per-page="10" />
        </v-card-text>
      </v-card>
    </template>
  </v-stepper>
</template>

<script lang="ts" setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { usuariosApi } from '@/api/usuarios'
import { mensajeError } from '@/api/http'
import { useNotificaciones } from '@/stores/notificaciones'

type TipoClave = 'alumnos' | 'maestros' | 'admins'

interface ErrorFila { fila: number; mensaje: string }

interface Trabajo {
  id: string
  tipo: string
  archivo: string
  estado: 'procesando' | 'terminado' | 'error'
  mensaje?: string
  porcentaje: number
  leidas: number
  insertados: number
  actualizados: number
  con_error: number
  errores: ErrorFila[]
  inicio: string
  fin?: string
}

interface DefTipo {
  titulo: string
  icono: string
  acceso: string
  columnas: { nombre: string; ejemplo: string }[]
  ejemplos: string[]
}

const MAX_BYTES = 100 * 1024 * 1024
const CLAVE_TRABAJO = 'raccoon_importacion' // para retomar el avance si sales de la pantalla

const TIPOS: Record<TipoClave, DefTipo> = {
  alumnos: {
    titulo: 'Alumnos',
    icono: 'mdi-school',
    acceso: 'Alumno',
    columnas: [
      { nombre: 'matricula', ejemplo: '2230200' },
      { nombre: 'nombre', ejemplo: 'Luis Martínez Gómez' },
      { nombre: 'contrasenia', ejemplo: 'Tula2026!' },
      { nombre: 'grupo', ejemplo: 'IRD-71' }
    ],
    ejemplos: [
      '2230200,Luis Martínez Gómez,Tula2026!,IRD-71',
      '2230201,María Hernández Cruz,Tula2026!,IRD-72'
    ]
  },
  maestros: {
    titulo: 'Maestros',
    icono: 'mdi-human-male-board',
    acceso: 'Maestro o Alumno',
    columnas: [
      { nombre: 'clave', ejemplo: 'doc0050' },
      { nombre: 'nombre', ejemplo: 'Rosa Jiménez Vega' },
      { nombre: 'contrasenia', ejemplo: 'Maestra2026!' }
    ],
    ejemplos: ['doc0050,Rosa Jiménez Vega,Maestra2026!']
  },
  admins: {
    titulo: 'Admins',
    icono: 'mdi-shield-crown',
    acceso: 'Admin, Maestro o Alumno',
    columnas: [
      { nombre: 'clave', ejemplo: 'adm002' },
      { nombre: 'nombre', ejemplo: 'Carlos Ramírez Ortiz' },
      { nombre: 'contrasenia', ejemplo: 'Admin2026!' }
    ],
    ejemplos: ['adm002,Carlos Ramírez Ortiz,Admin2026!']
  }
}

const headersErrores = [
  { title: 'Fila', key: 'fila', width: 80 },
  { title: 'Problema', key: 'mensaje' }
]

const noti = useNotificaciones()

const tipo = ref<TipoClave>('alumnos')
const paso = ref(1)
const inputRef = ref<HTMLInputElement | null>(null)
const archivo = ref<File | null>(null)
const arrastrando = ref(false)
const subida = ref(0)
const trabajo = ref<Trabajo | null>(null)
let sondeo: ReturnType<typeof setInterval> | null = null

const actual = computed(() => TIPOS[tipo.value])

const iconoArchivo = computed(() => {
  const nombre = trabajo.value?.archivo || archivo.value?.name || ''
  return nombre.toLowerCase().endsWith('.csv') ? 'mdi-file-delimited' : 'mdi-file-excel'
})

const duracion = computed(() => {
  if (!trabajo.value?.fin) return ''
  const s = Math.round((Date.parse(trabajo.value.fin) - Date.parse(trabajo.value.inicio)) / 1000)
  return s < 60 ? `${s} s` : `${Math.floor(s / 60)} min ${s % 60} s`
})

const num = (n?: number) => (n ?? 0).toLocaleString('es-MX')
const tamano = (b: number) => (b < 1024 * 1024 ? `${(b / 1024).toFixed(1)} KB` : `${(b / 1024 / 1024).toFixed(1)} MB`)

// Al cambiar de pestaña se descarta el archivo elegido
watch(tipo, () => { if (paso.value === 1) archivo.value = null })

// ---- Elegir archivo ----
const aceptar = (f?: File | null) => {
  if (!f) return
  if (!/\.(csv|xlsx)$/i.test(f.name)) {
    noti.error('Solo se aceptan .csv o .xlsx (si es .xls, ábrelo en Excel y guárdalo como .xlsx)')
    return
  }
  if (f.size > MAX_BYTES) {
    noti.error('El archivo pesa más de 100 MB')
    return
  }
  archivo.value = f
}
const elegir = (e: Event) => {
  const input = e.target as HTMLInputElement
  aceptar(input.files?.[0])
  input.value = ''
}
const soltar = (e: DragEvent) => {
  arrastrando.value = false
  aceptar(e.dataTransfer?.files?.[0])
}

// ---- Avance en vivo: pregunta al servidor cada segundo ----
const detenerSondeo = () => {
  if (sondeo) clearInterval(sondeo)
  sondeo = null
}

const consultar = async (id: string) => {
  try {
    trabajo.value = await usuariosApi.estadoImportacion(id)
    if (trabajo.value?.estado !== 'procesando') {
      detenerSondeo()
      sessionStorage.removeItem(CLAVE_TRABAJO)
      paso.value = 3
      if (trabajo.value?.estado === 'terminado') noti.exito('Importación terminada')
    }
  } catch {
    // La app se reinició o el trabajo ya no existe
    detenerSondeo()
    sessionStorage.removeItem(CLAVE_TRABAJO)
    if (paso.value === 2) {
      noti.error('Se perdió el seguimiento de la importación')
      reiniciar()
    }
  }
}

const seguir = (id: string) => {
  sessionStorage.setItem(CLAVE_TRABAJO, id)
  paso.value = 2
  detenerSondeo()
  consultar(id)
  sondeo = setInterval(() => consultar(id), 1000)
}

// ---- Importar ----
const importar = async () => {
  if (!archivo.value) return
  subida.value = 0
  trabajo.value = null
  paso.value = 2
  try {
    const t: Trabajo = await usuariosApi.importar(archivo.value, tipo.value, (p: number) => { subida.value = p })
    subida.value = 100
    trabajo.value = t
    seguir(t.id)
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo subir el archivo'))
    paso.value = 1
  }
}

function reiniciar() {
  detenerSondeo()
  sessionStorage.removeItem(CLAVE_TRABAJO)
  archivo.value = null
  trabajo.value = null
  subida.value = 0
  paso.value = 1
}

// Si había una importación en curso, retomarla
onMounted(() => {
  const id = sessionStorage.getItem(CLAVE_TRABAJO)
  if (id) {
    subida.value = 100
    seguir(id)
  }
})
onBeforeUnmount(detenerSondeo)

// ---- Plantilla ----
const descargarPlantilla = () => {
  const filas = [actual.value.columnas.map((c) => c.nombre).join(','), ...actual.value.ejemplos]
  // El BOM (marca UTF-8) hace que Excel respete los acentos
  const bom = String.fromCharCode(0xfeff)
  const blob = new Blob([bom + filas.join('\r\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `plantilla_${tipo.value}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}
</script>
