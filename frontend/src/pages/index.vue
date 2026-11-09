<template>
  <AppFrame>
    <div class="d-flex align-center flex-wrap ga-2 my-4">
      <div class="text-h5">Hola, {{ authStore.userAcc.nombre }}</div>
      <v-chip color="primary" size="small" variant="tonal">{{ nombreRol }}</v-chip>
      <v-chip v-if="authStore.userAcc.grupo" size="small" variant="tonal">Grupo {{ authStore.userAcc.grupo }}</v-chip>
    </div>

    <v-alert v-if="error" class="mb-4" closable :text="error" type="error" variant="tonal" @click:close="error = ''" />
    <v-progress-linear v-if="!datos && !error" color="primary" indeterminate />

    <!-- ============================ ALUMNO ============================ -->
    <v-row v-if="alumno">
      <!-- Mi laboratorio -->
      <v-col cols="12" md="6">
        <v-card border class="h-100" elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center flex-wrap ga-2">
            <v-icon icon="mdi-lan" />
            Mi laboratorio
            <v-spacer />
            <v-chip :color="estadoLab.color" size="small" variant="tonal">{{ estadoLab.texto }}</v-chip>
          </v-card-title>
          <v-card-text>
            <v-alert v-if="alumno.lab.mensaje" class="mb-3" density="compact" type="warning" variant="tonal">
              {{ alumno.lab.mensaje }}
            </v-alert>
            <template v-if="alumno.lab.estado == 'CORRIENDO'">
              <div class="d-flex flex-wrap ga-6">
                <div>
                  <div class="text-caption text-medium-emphasis">Servidor</div>
                  <div class="text-h6">{{ servidor }}</div>
                </div>
                <div>
                  <div class="text-caption text-medium-emphasis">Puerto</div>
                  <div class="text-h6">{{ alumno.lab.puerto }}</div>
                </div>
                <div v-if="alumno.lab.codigo">
                  <div class="text-caption text-medium-emphasis">Código del equipo</div>
                  <div class="text-h6">{{ alumno.lab.codigo }}</div>
                </div>
              </div>
              <div class="text-medium-emphasis mt-2">
                {{ alumno.soy_dueno ? 'Eres el dueño.' : `Laboratorio de ${alumno.lab.dueno}.` }}
                El usuario y la contraseña de GNS3 están en Mi laboratorio.
              </div>
            </template>
            <div v-else-if="alumno.lab.estado == 'CREANDO'" class="text-medium-emphasis">
              Se está preparando; tarda un par de minutos.
            </div>
            <div v-else class="text-medium-emphasis">
              Todavía no tienes laboratorio. Créalo o únete al de tu equipo.
            </div>
          </v-card-text>
          <v-card-actions>
            <v-btn color="primary" prepend-icon="mdi-arrow-right" text="Ir a Mi laboratorio" to="/laboratorio" variant="tonal" />
          </v-card-actions>
        </v-card>
      </v-col>

      <!-- Prácticas pendientes -->
      <v-col cols="12" md="6">
        <v-card border class="h-100" elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center ga-2">
            <v-icon icon="mdi-clipboard-text-clock" />
            Prácticas por entregar
            <v-spacer />
            <v-chip v-if="alumno.pendientes.length" color="warning" size="small" variant="tonal">
              {{ alumno.pendientes.length }}
            </v-chip>
          </v-card-title>
          <v-card-text>
            <div v-if="alumno.pendientes.length == 0" class="text-medium-emphasis">No tienes prácticas pendientes.</div>
            <v-list-item
              v-for="p in alumno.pendientes"
              :key="p.id"
              class="px-0"
              prepend-icon="mdi-file-document-edit-outline"
              :subtitle="p.profesor"
              :title="p.nombre"
            >
              <template #append>
                <v-chip v-if="p.revision && (p.revision.estado == 'EN_COLA' || p.revision.estado == 'REVISANDO')" color="info" size="small" variant="tonal">
                  Revisando
                </v-chip>
              </template>
            </v-list-item>
          </v-card-text>
          <v-card-actions>
            <v-btn color="primary" prepend-icon="mdi-arrow-right" text="Ir a Mis prácticas" to="/mis-practicas" variant="tonal" />
          </v-card-actions>
        </v-card>
      </v-col>

      <!-- Calificaciones -->
      <v-col cols="12" md="6">
        <v-card border class="h-100" elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center ga-2">
            <v-icon icon="mdi-star-check" />
            Mis últimas calificaciones
          </v-card-title>
          <v-card-text>
            <div v-if="alumno.calificaciones.length == 0" class="text-medium-emphasis">Todavía no tienes entregas.</div>
            <v-list-item
              v-for="p in alumno.calificaciones"
              :key="p.id"
              class="px-0"
              :subtitle="p.tarde ? `Entregada ${p.retraso} tarde` : `Entregada a tiempo · ${fecha(p.entregada)}`"
              :title="p.nombre"
            >
              <template #append>
                <v-chip :color="colorCalificacion(p.calificacion ?? 0)" label variant="tonal">{{ p.calificacion }} / 100</v-chip>
              </template>
            </v-list-item>
          </v-card-text>
        </v-card>
      </v-col>

      <!-- Guía para conectar GNS3 (e invitaciones, si hay) -->
      <v-col cols="12" md="6">
        <v-card v-if="alumno.invitaciones.length" border class="mb-4" elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center ga-2">
            <v-icon icon="mdi-email" />
            Invitaciones
          </v-card-title>
          <v-card-text>
            <v-list-item
              v-for="inv in alumno.invitaciones"
              :key="inv.id"
              class="px-0"
              :subtitle="inv.matricula"
              :title="`${inv.emisor} te invitó a su laboratorio`"
            />
          </v-card-text>
          <v-card-actions>
            <v-btn color="primary" text="Responder en Mi laboratorio" to="/laboratorio" variant="tonal" />
          </v-card-actions>
        </v-card>

        <v-card border elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center ga-2">
            <v-icon icon="mdi-help-circle-outline" />
            Conectar GNS3 2.2.59
          </v-card-title>
          <v-card-text>
            <ol class="ml-5 guia">
              <li>Inicia sesión en esta página <strong>desde la misma computadora</strong> donde usas GNS3.</li>
              <li>En GNS3: <strong>Edit → Preferences → Server</strong> y desactiva <em>Enable local server</em>.</li>
              <li>Host: <strong>{{ servidor }}</strong>, Port: <strong>{{ alumno.lab.puerto || 'el de tu laboratorio' }}</strong>, protocolo HTTP.</li>
              <li>Marca <em>Auth</em> y escribe el usuario y la contraseña que aparecen en <strong>Mi laboratorio</strong>.</li>
              <li>Aplica y abre o crea tu proyecto. Las consolas de cada equipo se abren solas al agregarlo.</li>
            </ol>
          </v-card-text>
        </v-card>
      </v-col>
    </v-row>

    <!-- ============================ PROFESOR ============================ -->
    <template v-if="profesor">
      <v-row>
        <v-col v-for="t in tilesProfesor" :key="t.titulo" cols="12" sm="4">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-text class="d-flex align-center ga-4">
              <v-avatar :color="t.color" size="48" variant="tonal"><v-icon :icon="t.icono" /></v-avatar>
              <div>
                <div class="text-h5">{{ t.valor }}</div>
                <div class="text-medium-emphasis">{{ t.titulo }}</div>
              </div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="12" md="7">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-title class="d-flex align-center ga-2">
              <v-icon icon="mdi-clipboard-check-multiple" />
              Mis prácticas activas
            </v-card-title>
            <v-card-text>
              <div v-if="profesor.practicas.length == 0" class="text-medium-emphasis">No tienes prácticas activas.</div>
              <v-list-item
                v-for="p in profesor.practicas"
                :key="p.id"
                class="px-0 mb-2"
                :to="`/practicas/${p.id}`"
              >
                <div class="d-flex align-center flex-wrap ga-2 mb-1">
                  <strong>{{ p.nombre }}</strong>
                  <v-chip size="x-small" variant="tonal">{{ p.grupo }}</v-chip>
                  <v-spacer />
                  <span class="text-body-2">{{ p.entregadas }}/{{ p.alumnos }} entregas</span>
                </div>
                <v-progress-linear color="primary" height="8" :model-value="porcentaje(p.entregadas, p.alumnos)" rounded />
              </v-list-item>
            </v-card-text>
            <v-card-actions>
              <v-btn color="primary" prepend-icon="mdi-arrow-right" text="Ir a Prácticas" to="/practicas" variant="tonal" />
            </v-card-actions>
          </v-card>
        </v-col>

        <v-col cols="12" md="5">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-title class="d-flex align-center ga-2">
              <v-icon icon="mdi-account-group" />
              Mis grupos
            </v-card-title>
            <v-card-text>
              <div v-if="profesor.grupos.length == 0" class="text-medium-emphasis">No tienes grupos asignados.</div>
              <v-list-item
                v-for="g in profesor.grupos"
                :key="g.id"
                class="px-0"
                :subtitle="g.carrera || 'Sin carrera'"
                :title="g.codigo"
              >
                <template #append>
                  <div class="text-right">
                    <div class="text-body-2">{{ g.alumnos }} alumnos</div>
                    <div class="text-caption" :class="g.labs_encendidos ? 'text-success' : 'text-medium-emphasis'">
                      {{ g.labs_encendidos }} con lab encendido
                    </div>
                  </div>
                </template>
              </v-list-item>
            </v-card-text>
            <v-card-actions>
              <v-btn color="primary" prepend-icon="mdi-monitor-eye" text="Supervisar" to="/supervision" variant="tonal" />
            </v-card-actions>
          </v-card>
        </v-col>
      </v-row>
    </template>

    <!-- ============================ ADMIN ============================ -->
    <template v-if="admin">
      <v-row>
        <v-col v-for="t in tilesAdmin" :key="t.titulo" cols="12" lg="3" sm="6">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-text>
              <div class="d-flex align-center ga-2 mb-2">
                <v-icon :color="t.color" :icon="t.icono" />
                <span class="text-medium-emphasis">{{ t.titulo }}</span>
              </div>
              <div class="text-h5 mb-2">{{ t.valor }}</div>
              <v-progress-linear :color="t.color" height="8" :model-value="t.porcentaje" rounded />
              <div class="text-caption text-medium-emphasis mt-1">{{ t.detalle }}</div>
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>

      <v-row>
        <v-col cols="12" md="6" lg="4">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-title class="d-flex align-center ga-2">
              <v-icon icon="mdi-heart-pulse" />
              Servicios
            </v-card-title>
            <v-card-text>
              <v-list-item v-for="sv in admin.servicios" :key="sv.nombre" class="px-0" :subtitle="sv.detalle" :title="sv.nombre">
                <template #prepend>
                  <v-icon :color="sv.bien ? 'success' : 'error'" :icon="sv.bien ? 'mdi-check-circle' : 'mdi-alert-circle'" />
                </template>
              </v-list-item>
            </v-card-text>
          </v-card>
        </v-col>

        <v-col cols="12" md="6" lg="4">
          <v-card border class="h-100" elevation="0" rounded="lg">
            <v-card-title class="d-flex align-center ga-2">
              <v-icon icon="mdi-account-multiple" />
              Usuarios
            </v-card-title>
            <v-card-text>
              <v-list-item v-for="u in admin.usuarios" :key="u.tipo" class="px-0" :title="u.descripcion">
                <template #append>
                  <div class="text-right">
                    <div class="text-h6">{{ u.total }}</div>
                    <div v-if="u.activos != u.total" class="text-caption text-medium-emphasis">{{ u.activos }} activos</div>
                  </div>
                </template>
              </v-list-item>
            </v-card-text>
            <v-card-actions>
              <v-btn color="primary" prepend-icon="mdi-arrow-right" text="Ir a Usuarios" to="/usuarios" variant="tonal" />
            </v-card-actions>
          </v-card>
        </v-col>

        <v-col cols="12" lg="4">
          <v-card border class="mb-4" elevation="0" rounded="lg">
            <v-card-title class="d-flex align-center ga-2">
              <v-icon icon="mdi-file-upload" />
              Última importación
            </v-card-title>
            <v-card-text>
              <div v-if="!admin.ultima_importacion" class="text-medium-emphasis">Todavía no se ha importado nada.</div>
              <template v-else>
                <div class="d-flex align-center flex-wrap ga-2">
                  <strong class="text-truncate">{{ admin.ultima_importacion.archivo }}</strong>
                  <v-chip :color="colorImportacion(admin.ultima_importacion.estado)" size="x-small" variant="tonal">
                    {{ admin.ultima_importacion.estado }}
                  </v-chip>
                </div>
                <div class="text-medium-emphasis">
                  {{ fecha(admin.ultima_importacion.inicio) }} · {{ admin.ultima_importacion.usuario }}
                </div>
                <div class="mt-1">
                  {{ admin.ultima_importacion.insertados }} nuevos ·
                  {{ admin.ultima_importacion.actualizados }} actualizados ·
                  <span :class="admin.ultima_importacion.con_error ? 'text-error' : ''">{{ admin.ultima_importacion.con_error }} con error</span>
                </div>
              </template>
            </v-card-text>
          </v-card>

          <v-card border elevation="0" rounded="lg">
            <v-card-text class="d-flex align-center ga-4">
              <v-avatar color="primary" size="48" variant="tonal"><v-icon icon="mdi-clipboard-check-multiple" /></v-avatar>
              <div>
                <div class="text-h5">{{ admin.practicas_activas }}</div>
                <div class="text-medium-emphasis">Prácticas activas</div>
              </div>
              <v-spacer />
              <v-btn text="Ver" to="/practicas" variant="text" />
            </v-card-text>
          </v-card>
        </v-col>
      </v-row>
    </template>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import axios from 'axios'
import { useAuthStore } from '@/stores/auth'
import { colorUso, type Inicio, porcentaje, tamano } from '@/models/Inicio'
import { fecha } from '@/models/Practica'

const authStore = useAuthStore()
const datos = ref<Inicio | null>(null)
const error = ref('')
const servidor = window.location.hostname   // el GNS3 está en el mismo servidor que la página
let temporizador: ReturnType<typeof setInterval> | undefined

const alumno = computed(() => datos.value?.alumno)
const profesor = computed(() => datos.value?.profesor)
const admin = computed(() => datos.value?.admin)

const nombreRol = computed(() => ({ ADMIN: 'Administrador', PROFESOR: 'Profesor', ALUMNO: 'Alumno' } as Record<string, string>)[authStore.userAcc.rol] ?? authStore.userAcc.rol)

const estadoLab = computed(() => {
  switch (alumno.value?.lab.estado) {
    case 'CORRIENDO': { return { texto: 'En ejecución', color: 'success' } }
    case 'CREANDO': { return { texto: 'Creando', color: 'info' } }
    case 'ERROR': { return { texto: 'Error', color: 'error' } }
    default: { return { texto: 'Sin laboratorio', color: 'grey' } }
  }
})

const tilesProfesor = computed(() => {
  const p = profesor.value
  if (!p) return []
  return [
    { titulo: 'Prácticas activas', valor: p.practicas.length, icono: 'mdi-clipboard-check-multiple', color: 'primary' },
    { titulo: 'Alumnos en mis grupos', valor: p.alumnos, icono: 'mdi-account-school', color: 'info' },
    { titulo: 'Con el lab encendido', valor: `${p.labs_encendidos}/${p.alumnos}`, icono: 'mdi-lan-connect', color: 'success' },
  ]
})

const tilesAdmin = computed(() => {
  const a = admin.value
  if (!a) return []
  const s = a.servidor
  const labs = porcentaje(a.labs_activos, a.labs_max)
  const ram = porcentaje(s.ram_usada, s.ram_total)
  const disco = porcentaje(s.disco_usado, s.disco_total)
  return [
    { titulo: 'Labs activos', valor: `${a.labs_activos} / ${a.labs_max}`, porcentaje: labs, color: colorUso(labs), icono: 'mdi-lan',
      detalle: a.labs_creando ? `${a.labs_creando} creándose` : 'máximo del .env (LABS_MAX)' },
    { titulo: 'CPU', valor: `${s.cpu}%`, porcentaje: s.cpu, color: colorUso(s.cpu), icono: 'mdi-cpu-64-bit', detalle: `${s.nucleos} núcleos` },
    { titulo: 'RAM', valor: `${ram}%`, porcentaje: ram, color: colorUso(ram), icono: 'mdi-memory',
      detalle: `${tamano(s.ram_usada)} de ${tamano(s.ram_total)}` },
    { titulo: 'Disco de los labs', valor: `${disco}%`, porcentaje: disco, color: colorUso(disco), icono: 'mdi-harddisk',
      detalle: s.error || `${tamano(s.disco_usado)} de ${tamano(s.disco_total)}` },
  ]
})

function colorCalificacion (c: number) {
  if (c >= 80) return 'success'
  if (c >= 60) return 'warning'
  return 'error'
}

function colorImportacion (estado: string) {
  return ({ TERMINADO: 'success', PROCESANDO: 'info', ERROR: 'error' } as Record<string, string>)[estado] ?? 'grey'
}

function cargar () {
  axios.get('/api/inicio')
    .then(response => {
      datos.value = response.data
      error.value = ''
    })
    .catch(() => error.value = 'No se pudo cargar el tablero')
}

onMounted(() => {
  cargar()
  temporizador = setInterval(cargar, 15_000) // se actualiza solo cada 15 s
})
onBeforeUnmount(() => clearInterval(temporizador))
</script>

<style scoped>
.guia li {
  margin-bottom: 4px;
}
</style>
