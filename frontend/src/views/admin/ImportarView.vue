<template>
  <div class="mb-4">
    <div class="page-title">Importar usuarios</div>
    <div class="page-subtitle">Cada tipo de usuario se sube en su propio archivo CSV o Excel.</div>
  </div>

  <v-tabs v-model="tipo" color="primary" class="mb-4" :disabled="paso > 1">
    <v-tab v-for="(t, clave) in TIPOS" :key="clave" :value="clave" :prepend-icon="t.icono">{{ t.titulo }}</v-tab>
  </v-tabs>

  <v-stepper v-model="paso" :items="['Archivo', 'Validación', 'Resultado']" hide-actions flat class="bg-transparent">
    <!-- 1. Archivo -->
    <template #item.1>
      <v-row>
        <v-col cols="12" md="7">
          <v-card class="pa-6">
            <div
              class="zona pa-10 text-center"
              :class="{ activa: arrastrando }"
              @click="inputRef.click()"
              @dragover.prevent="arrastrando = true"
              @dragleave.prevent="arrastrando = false"
              @drop.prevent="soltar"
            >
              <v-icon :icon="actual.icono" size="56" color="primary" />
              <div class="text-h6 mt-3">Archivo de {{ actual.titulo.toLowerCase() }}</div>
              <div class="text-medium-emphasis">Arrastra o haz clic · .csv, .xlsx, .xls · máx. 5 MB</div>
              <input ref="inputRef" type="file" hidden accept=".csv,.xlsx,.xls" @change="elegir" />
            </div>

            <v-alert v-if="archivo" type="info" variant="tonal" class="mt-4" :icon="iconoArchivo">
              <div class="d-flex align-center justify-space-between">
                <div>
                  <div class="font-weight-bold">{{ archivo.name }}</div>
                  <div class="text-caption">{{ (archivo.size / 1024).toFixed(1) }} KB</div>
                </div>
                <v-btn icon="mdi-close" variant="text" size="small" @click="archivo = null" />
              </div>
            </v-alert>

            <v-btn
              block size="large" color="primary" class="mt-4" prepend-icon="mdi-check-decagram"
              :disabled="!archivo" :loading="cargando" @click="validar"
            >
              Validar archivo
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
                <li>La primera fila es el encabezado; acentos y mayúsculas no importan.</li>
                <li>Podrán entrar como: <b>{{ actual.acceso }}</b>.</li>
                <li v-if="tipo === 'alumnos'">Los grupos que no existan se crean solos.</li>
                <li v-else>La asignación a grupos se hace en <b>Grupos</b>.</li>
                <li>Si la {{ tipo === 'alumnos' ? 'matrícula' : 'clave' }} ya existe, se actualiza; la contraseña solo si viene.</li>
                <li>Contraseña mínima de 8 caracteres.</li>
              </ul>
              <v-btn variant="tonal" prepend-icon="mdi-download" block @click="descargarPlantilla">
                Descargar plantilla de {{ actual.titulo.toLowerCase() }}
              </v-btn>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </template>

    <!-- 2. Validación -->
    <template #item.2>
      <v-row class="mb-2">
        <v-col cols="6" md="3">
          <v-card class="pa-4"><div class="text-caption">Filas leídas</div><div class="text-h5 font-weight-bold">{{ validacion.total }}</div></v-card>
        </v-col>
        <v-col cols="6" md="3">
          <v-card class="pa-4"><div class="text-caption">Válidas</div><div class="text-h5 font-weight-bold text-success">{{ validacion.validos }}</div></v-card>
        </v-col>
        <v-col cols="6" md="3">
          <v-card class="pa-4"><div class="text-caption">Con errores</div><div class="text-h5 font-weight-bold text-error">{{ validacion.errores.length }}</div></v-card>
        </v-col>
        <v-col v-if="tipo === 'alumnos'" cols="6" md="3">
          <v-card class="pa-4"><div class="text-caption">Grupos nuevos</div><div class="text-h5 font-weight-bold text-info">{{ validacion.grupos_nuevos.length }}</div></v-card>
        </v-col>
      </v-row>

      <v-alert v-if="validacion.grupos_nuevos.length" type="info" variant="tonal" class="mb-4">
        Se crearán los grupos: <b>{{ validacion.grupos_nuevos.join(', ') }}</b>. Después asígnales maestro en <b>Grupos</b>.
      </v-alert>

      <v-card v-if="validacion.errores.length" class="mb-4" color="error" variant="tonal"
        title="Errores (estas filas NO se importarán)" prepend-icon="mdi-alert-circle">
        <v-card-text>
          <v-data-table :headers="headersErrores" :items="validacion.errores" density="compact" items-per-page="5" />
        </v-card-text>
      </v-card>

      <v-card title="Vista previa" prepend-icon="mdi-eye">
        <v-card-text>
          <v-data-table :headers="headersPreview" :items="validacion.vista_previa" density="compact" items-per-page="10">
            <template #item.matricula="{ item }"><span class="font-mono">{{ item.matricula }}</span></template>
            <template #item.accion="{ item }">
              <v-chip :color="item.accion === 'CREAR' ? 'success' : 'warning'" size="x-small" variant="outlined">{{ item.accion }}</v-chip>
            </template>
          </v-data-table>
        </v-card-text>
        <v-card-actions>
          <v-btn variant="text" prepend-icon="mdi-arrow-left" @click="paso = 1">Cambiar archivo</v-btn>
          <v-spacer />
          <v-btn color="primary" prepend-icon="mdi-database-import" :disabled="!validacion.validos" :loading="cargando" @click="importar">
            Importar {{ validacion.validos }} {{ actual.titulo.toLowerCase() }}
          </v-btn>
        </v-card-actions>
      </v-card>
    </template>

    <!-- 3. Resultado -->
    <template #item.3>
      <v-card class="pa-8 text-center">
        <v-icon icon="mdi-check-circle" color="success" size="64" />
        <div class="text-h5 mt-4 mb-6">Importación de {{ actual.titulo.toLowerCase() }} terminada</div>
        <v-row justify="center">
          <v-col cols="6" md="2"><div class="text-h4 text-success">{{ resultado.creados }}</div><div class="text-caption">creados</div></v-col>
          <v-col cols="6" md="2"><div class="text-h4 text-warning">{{ resultado.actualizados }}</div><div class="text-caption">actualizados</div></v-col>
          <v-col v-if="tipo === 'alumnos'" cols="6" md="2"><div class="text-h4 text-info">{{ resultado.grupos_creados || 0 }}</div><div class="text-caption">grupos creados</div></v-col>
          <v-col cols="6" md="2"><div class="text-h4 text-error">{{ resultado.errores?.length || 0 }}</div><div class="text-caption">con error</div></v-col>
        </v-row>
        <v-btn variant="tonal" class="mt-6" @click="reiniciar">Importar otro archivo</v-btn>
      </v-card>
    </template>
  </v-stepper>
</template>

<script lang="ts" setup>
import { ref, computed, watch } from 'vue'
import { usuariosApi } from '@/api/usuarios'
import { mensajeError } from '@/api/http'
import { useNotificaciones } from '@/stores/notificaciones'

const noti = useNotificaciones()

const TIPOS = {
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
      { nombre: 'clave', ejemplo: 'DOC0050' },
      { nombre: 'nombre', ejemplo: 'Rosa Jiménez Vega' },
      { nombre: 'contrasenia', ejemplo: 'Maestra2026!' }
    ],
    ejemplos: ['DOC0050,Rosa Jiménez Vega,Maestra2026!']
  },
  admins: {
    titulo: 'Admins',
    icono: 'mdi-shield-crown',
    acceso: 'Admin, Maestro o Alumno',
    columnas: [
      { nombre: 'clave', ejemplo: 'ADM002' },
      { nombre: 'nombre', ejemplo: 'Carlos Ramírez Ortiz' },
      { nombre: 'contrasenia', ejemplo: 'Admin2026!' }
    ],
    ejemplos: ['ADM002,Carlos Ramírez Ortiz,Admin2026!']
  }
}

const headersErrores = [
  { title: 'Fila', key: 'fila', width: 70 },
  { title: 'Matrícula / clave', key: 'matricula' },
  { title: 'Problema', key: 'mensaje' }
]

const vacia = () => ({ total: 0, validos: 0, errores: [], grupos_nuevos: [], vista_previa: [] })

const tipo = ref('alumnos')
const paso = ref(1)
const inputRef = ref(null)
const archivo = ref(null)
const arrastrando = ref(false)
const cargando = ref(false)
const validacion = ref(vacia())
const resultado = ref({ creados: 0, actualizados: 0, errores: [] })

const actual = computed(() => TIPOS[tipo.value])

const headersPreview = computed(() => [
  { title: 'Fila', key: 'fila', width: 70 },
  { title: tipo.value === 'alumnos' ? 'Matrícula' : 'Clave', key: 'matricula' },
  { title: 'Nombre', key: 'nombre' },
  ...(tipo.value === 'alumnos' ? [{ title: 'Grupo', key: 'grupo' }] : []),
  { title: 'Acción', key: 'accion' }
])

const iconoArchivo = computed(() =>
  archivo.value?.name.toLowerCase().endsWith('.csv') ? 'mdi-file-delimited' : 'mdi-file-excel'
)

// Al cambiar de pestaña se descarta el archivo elegido
watch(tipo, () => reiniciar())

const aceptar = (f) => {
  if (!f) return
  if (!/\.(csv|xlsx|xls)$/i.test(f.name)) return noti.error('Solo se aceptan archivos .csv, .xlsx o .xls')
  if (f.size > 5 * 1024 * 1024) return noti.error('El archivo pesa más de 5 MB')
  archivo.value = f
}
const elegir = (e) => { aceptar(e.target.files[0]); e.target.value = '' }
const soltar = (e) => { arrastrando.value = false; aceptar(e.dataTransfer.files[0]) }

const validar = async () => {
  cargando.value = true
  try {
    validacion.value = { ...vacia(), ...(await usuariosApi.importar(archivo.value, tipo.value, true)) }
    paso.value = 2
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo leer el archivo'))
  } finally {
    cargando.value = false
  }
}

const importar = async () => {
  cargando.value = true
  try {
    resultado.value = await usuariosApi.importar(archivo.value, tipo.value, false)
    paso.value = 3
    noti.exito(`${actual.value.titulo} importados`)
  } catch (e) {
    noti.error(mensajeError(e, 'Falló la importación'))
  } finally {
    cargando.value = false
  }
}

function reiniciar() {
  archivo.value = null
  validacion.value = vacia()
  paso.value = 1
}

const descargarPlantilla = () => {
  const filas = [actual.value.columnas.map((c) => c.nombre).join(','), ...actual.value.ejemplos]
  // El BOM (\uFEFF) hace que Excel respete los acentos
  const blob = new Blob(['\uFEFF' + filas.join('\r\n')], { type: 'text/csv;charset=utf-8' })
  const a = document.createElement('a')
  a.href = URL.createObjectURL(blob)
  a.download = `plantilla_${tipo.value}.csv`
  a.click()
  URL.revokeObjectURL(a.href)
}
</script>