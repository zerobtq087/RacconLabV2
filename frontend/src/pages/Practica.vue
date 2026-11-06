<template>
  <AppFrame>
    <div class="d-flex align-center flex-wrap ga-2 my-4">
      <v-btn icon="mdi-arrow-left" to="/practicas" variant="text" />
      <span class="text-h6">{{ practica.nombre }}</span>
      <v-chip :color="estadosPractica[practica.estado]?.color" size="small" variant="tonal">
        {{ estadosPractica[practica.estado]?.texto }}
      </v-chip>
      <span class="text-medium-emphasis">{{ practica.grupo }} · {{ practica.profesor }}</span>
      <v-spacer />
      <v-btn
        v-if="practica.estado == 'BORRADOR'"
        color="success"
        prepend-icon="mdi-play"
        text="Activar"
        variant="flat"
        @click="confirmar = 'activar'"
      />
      <v-btn
        v-if="practica.estado == 'ACTIVA'"
        color="warning"
        prepend-icon="mdi-lock"
        text="Cerrar"
        variant="flat"
        @click="confirmar = 'cerrar'"
      />
      <v-btn
        v-if="!practica.tiene_entregas"
        color="error"
        prepend-icon="mdi-delete"
        text="Borrar"
        variant="text"
        @click="confirmar = 'borrar'"
      />
    </div>

    <!-- Confirmaciones dentro de la página -->
    <v-alert v-if="confirmar" class="mb-4" :type="confirmar == 'activar' ? 'info' : 'warning'" variant="tonal">
      {{ textosConfirmar[confirmar] }}
      <template #append>
        <v-btn class="mr-2" text="Cancelar" variant="text" @click="confirmar = ''" />
        <v-btn color="primary" text="Sí, continuar" variant="flat" @click="accion(confirmar)" />
      </template>
    </v-alert>
    <v-alert v-if="error" class="mb-4" closable :text="error" type="error" variant="tonal" @click:close="error = ''" />
    <v-alert v-if="aviso" class="mb-4" closable :text="aviso" type="success" variant="tonal" @click:close="aviso = ''" />

    <v-alert v-if="practica.estado == 'EXPORTANDO'" class="mb-4" type="info" variant="tonal">
      Exportando tu proyecto: {{ practica.paso }}…
    </v-alert>
    <v-alert v-if="practica.estado == 'ERROR'" class="mb-4" :text="practica.mensaje" type="error" variant="tonal" />
    <v-alert v-if="practica.cerrada" class="mb-4" density="compact" type="warning" variant="tonal">
      Cerrada el {{ fecha(practica.cerrada) }}: quien no había entregado aún puede hacerlo (contará como tarde).
    </v-alert>

    <v-tabs v-model="tab" class="mb-4" color="primary">
      <v-tab text="Criterios" value="criterios" />
      <v-tab text="Entregas" value="entregas" />
      <v-tab text="Datos" value="datos" />
    </v-tabs>

    <v-window v-model="tab">
      <!-- ===================== CRITERIOS ===================== -->
      <v-window-item value="criterios">
        <v-alert v-if="practica.tiene_entregas" class="mb-4" density="compact" type="info" variant="tonal">
          Ya hay entregas: los criterios quedaron bloqueados.
        </v-alert>

        <v-card v-for="(c, i) in criterios" :key="i" border class="mb-3" elevation="0" rounded="lg">
          <v-card-text>
            <v-row dense>
              <v-col cols="12" md="4">
                <v-select
                  v-model="c.tipo"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :items="tiposCriterio"
                  :label="`Criterio ${i + 1}`"
                  variant="outlined"
                  @update:model-value="cambiarTipo(c)"
                />
              </v-col>
              <v-col cols="6" md="3">
                <v-select
                  v-model="c.nodo"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :items="nodosPara(c.tipo)"
                  label="Nodo"
                  variant="outlined"
                />
              </v-col>
              <v-col v-if="pideInterfaz(c)" cols="6" md="3">
                <v-select
                  v-model="c.interfaz"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :items="puertosDe(c.nodo)"
                  :label="c.tipo.startsWith('VLAN') || c.tipo == 'TRUNK' ? 'Puerto' : 'Interfaz'"
                  variant="outlined"
                />
              </v-col>
              <v-col v-if="c.tipo == 'PING'" cols="6" md="3">
                <v-combobox
                  v-model="c.destino"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :items="practica.nodos.filter(n => n.tipo != 'ethernet_switch').map(n => n.nombre)"
                  label="Destino (nodo o IP)"
                  variant="outlined"
                />
              </v-col>
              <v-col v-if="pideValor(c.tipo)" cols="6" md="3">
                <v-text-field
                  v-model="c.valor"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :label="etiquetaValor[c.tipo]"
                  variant="outlined"
                />
              </v-col>
              <v-col v-if="modosCriterio[c.tipo]" cols="6" md="3">
                <v-select
                  v-model="c.modo"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  :items="modosCriterio[c.tipo]"
                  label="Cómo"
                  variant="outlined"
                />
              </v-col>
              <v-col cols="4" md="2">
                <v-text-field
                  v-model.number="c.peso"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  label="Puntos"
                  max="100"
                  min="1"
                  type="number"
                  variant="outlined"
                />
              </v-col>
              <v-col cols="12" md="9">
                <v-text-field
                  v-model="c.mensaje"
                  density="compact"
                  :disabled="bloqueados"
                  hide-details
                  label="Mensaje para el alumno si falla (opcional; sin dar la respuesta)"
                  variant="outlined"
                />
              </v-col>
              <v-col class="d-flex align-center" cols="12" md="1">
                <v-btn
                  v-if="!bloqueados"
                  color="error"
                  icon="mdi-delete"
                  size="small"
                  variant="text"
                  @click="criterios.splice(i, 1)"
                />
              </v-col>
            </v-row>
          </v-card-text>
        </v-card>

        <div class="d-flex flex-wrap ga-2 mb-6">
          <v-btn
            v-if="!bloqueados"
            prepend-icon="mdi-plus"
            text="Agregar criterio"
            variant="tonal"
            @click="criterios.push(new Criterio())"
          />
          <v-btn
            v-if="!bloqueados"
            color="primary"
            :loading="guardando"
            prepend-icon="mdi-content-save"
            text="Guardar criterios"
            variant="flat"
            @click="guardarCriterios"
          />
          <v-spacer />
          <span class="align-self-center text-medium-emphasis">Total: {{ totalPuntos }} puntos (la calificación es el % obtenido)</span>
        </div>

        <!-- Probar los criterios contra mi propio proyecto (la solución) -->
        <v-card border elevation="0" rounded="lg" title="Probar con mi solución">
          <v-card-text>
            <div class="text-medium-emphasis mb-3">
              Revisa los criterios guardados contra tu proyecto "{{ practica.proyecto_origen }}" en tu laboratorio
              (lo enciende si está apagado). Sirve para comprobar que tu solución saca 100.
            </div>
            <template v-if="prueba?.revision">
              <v-progress-linear
                v-if="enCurso(prueba.revision)"
                class="mb-2"
                color="primary"
                height="22"
                :model-value="prueba.revision.porcentaje"
                rounded
                striped
              >
                <strong>{{ prueba.revision.porcentaje }}%</strong>
              </v-progress-linear>
              <div v-if="enCurso(prueba.revision)" class="text-medium-emphasis">{{ prueba.revision.paso }}…</div>
              <v-alert v-else-if="prueba.revision.estado == 'ERROR'" :text="prueba.revision.mensaje" type="error" variant="tonal" />
              <template v-else>
                <div class="text-h6 mb-2">Calificación: {{ prueba.revision.calificacion }} / 100</div>
                <TablaResultados :resultados="prueba.resultados" />
              </template>
            </template>
          </v-card-text>
          <v-card-actions>
            <v-btn
              color="primary"
              :disabled="practica.criterios.length == 0 || (!!prueba?.revision && enCurso(prueba.revision))"
              prepend-icon="mdi-check-all"
              text="Probar ahora"
              variant="tonal"
              @click="probar"
            />
          </v-card-actions>
        </v-card>
      </v-window-item>

      <!-- ===================== ENTREGAS ===================== -->
      <v-window-item value="entregas">
        <div class="d-flex mb-3">
          <span class="text-medium-emphasis align-self-center">{{ practica.entregadas }} de {{ practica.alumnos }} alumnos entregaron</span>
          <v-spacer />
          <v-btn prepend-icon="mdi-microsoft-excel" text="Descargar Excel" variant="tonal" :href="`/api/practicas/${id}/excel`" />
        </div>
        <v-data-table
          :headers="headersEntregas"
          hover
          :items="entregas"
          items-per-page="50"
          no-data-text="El grupo no tiene alumnos"
          @click:row="(_e: unknown, { item }: { item: FilaEntrega }) => verEntrega(item)"
        >
          <template #item.calificacion="{ item }">
            <strong v-if="item.calificacion != null">{{ item.calificacion }}</strong>
          </template>
          <template #item.estado="{ item }">
            <v-chip v-if="item.estado == 'A_TIEMPO'" color="success" size="small" variant="tonal">A tiempo</v-chip>
            <v-chip v-else-if="item.estado == 'TARDE'" color="error" size="small" variant="tonal">{{ item.retraso }} tarde</v-chip>
            <v-chip v-else size="small" variant="tonal">Pendiente</v-chip>
          </template>
          <template #item.entregada="{ item }">{{ fecha(item.entregada) }}</template>
        </v-data-table>

        <!-- Detalle de la entrega de un alumno -->
        <v-card v-if="detalle" border class="mt-4" elevation="0" rounded="lg">
          <v-card-title class="d-flex align-center">
            {{ detalle.fila.matricula }} · {{ detalle.fila.nombre }}
            <v-spacer />
            <v-btn icon="mdi-close" size="small" variant="text" @click="detalle = null" />
          </v-card-title>
          <v-card-text>
            <div class="mb-3">
              <strong class="text-h6">{{ detalle.fila.calificacion }} / 100</strong>
              · entregó {{ detalle.fila.entrego }} el {{ fecha(detalle.fila.entregada) }}
              <span v-if="detalle.integrantes.length > 1">
                · contó para: {{ detalle.integrantes.map(p => p.nombre).join(', ') }}
              </span>
            </div>
            <TablaResultados :resultados="detalle.resultados" />

            <div class="text-subtitle-1 mt-4 mb-2">Configuración de los routers al entregar</div>
            <v-expansion-panels variant="accordion">
              <v-expansion-panel v-for="cfg in detalle.configs" :key="cfg.nodo" :title="cfg.nodo">
                <v-expansion-panel-text><pre class="config">{{ cfg.config }}</pre></v-expansion-panel-text>
              </v-expansion-panel>
            </v-expansion-panels>
          </v-card-text>
          <v-card-actions v-if="detalle.con_proyecto" class="flex-wrap ga-2">
            <v-btn
              prepend-icon="mdi-download"
              text="Descargar proyecto"
              variant="tonal"
              :href="`/api/practicas/${id}/entregas/${detalle.fila.id_usuario}/proyecto`"
            />
            <v-btn
              v-if="authStore.userAcc.rol == 'PROFESOR'"
              :loading="abriendo"
              prepend-icon="mdi-open-in-app"
              text="Abrir en mi laboratorio"
              variant="tonal"
              @click="abrirEntrega"
            />
          </v-card-actions>
        </v-card>
      </v-window-item>

      <!-- ===================== DATOS ===================== -->
      <v-window-item value="datos">
        <v-card border elevation="0" rounded="lg">
          <v-card-text>
            <v-text-field v-model="datos.nombre" counter="80" label="Nombre" variant="outlined" />
            <v-select
              v-model="datos.id_grupo"
              :disabled="practica.tiene_entregas"
              item-title="codigo"
              item-value="id"
              :items="grupos"
              label="Grupo"
              variant="outlined"
            />
            <v-text-field v-model="datos.enable_password" label="Contraseña de enable (opcional)" variant="outlined" />
            <v-textarea v-model="datos.instrucciones" auto-grow label="Instrucciones" rows="5" variant="outlined" />
            <div class="text-caption text-medium-emphasis">Proyecto de origen: {{ practica.proyecto_origen }}</div>
          </v-card-text>
          <v-card-actions>
            <v-btn color="primary" prepend-icon="mdi-content-save" text="Guardar" variant="flat" @click="guardarDatos" />
          </v-card-actions>
        </v-card>
      </v-window-item>
    </v-window>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import TablaResultados from '../components/TablaResultados.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import axios from 'axios'
import {
  Criterio, type DetalleEntrega, estadosPractica, fecha, type FilaEntrega, modosCriterio,
  Practica, type Prueba, type Revision, SolicitudPractica, tiposCriterio,
} from '@/models/Practica'
import type { Grupo } from '@/models/Grupo'
import { useAuthStore } from '@/stores/auth'

const route = useRoute()
const router = useRouter()
const authStore = useAuthStore()
const id = route.params.id as string

const practica = ref(new Practica())
const criterios = ref<Criterio[]>([])
const datos = ref(new SolicitudPractica())
const grupos = ref<Grupo[]>([])
const entregas = ref<FilaEntrega[]>([])
const detalle = ref<DetalleEntrega | null>(null)
const prueba = ref<Prueba | null>(null)
const tab = ref('criterios')
const error = ref('')
const aviso = ref('')
const confirmar = ref('')
const guardando = ref(false)
const abriendo = ref(false)
let temporizador: ReturnType<typeof setInterval> | undefined

const textosConfirmar: Record<string, string> = {
  activar: 'Al activarla, los alumnos del grupo la verán y podrán abrirla en su laboratorio.',
  cerrar: 'Al cerrarla se guarda la hora de cierre. No se puede reabrir: quien no haya entregado aún podrá entregar, pero marcado como tarde.',
  borrar: 'Se borra la práctica con su proyecto y criterios.',
}

const etiquetaValor: Record<string, string> = {
  IP_INTERFAZ: 'IP/prefijo (ej. 192.168.1.1/24)',
  VECINOS_OSPF: 'Mínimo de vecinos',
  RUTA: 'Red (ej. 192.168.2.0/24)',
  VLAN_PUERTO: 'VLAN',
  LINEA_CONFIG: 'Línea (ej. router ospf 1)',
  DHCP: 'Red esperada (opcional)',
}

const headersEntregas = [
  { title: 'Matrícula', key: 'matricula' },
  { title: 'Nombre', key: 'nombre' },
  { title: 'Calificación', key: 'calificacion' },
  { title: 'Estado', key: 'estado' },
  { title: 'Entregada', key: 'entregada' },
]

const bloqueados = computed(() => practica.value.tiene_entregas ||
  practica.value.estado == 'EXPORTANDO' || practica.value.estado == 'ERROR')
const totalPuntos = computed(() => criterios.value.reduce((s, c) => s + (Number(c.peso) || 0), 0))

function nodosPara (tipo: string) {
  const tipos = tiposCriterio.find(t => t.value == tipo)?.nodos || []
  return practica.value.nodos.filter(n => tipos.includes(n.tipo)).map(n => n.nombre)
}
function nodo (nombre: string) {
  return practica.value.nodos.find(n => n.nombre == nombre)
}
function puertosDe (nombre: string) {
  return nodo(nombre)?.puertos || []
}
// La interfaz solo hace falta en routers (IP) y en puertos de switch
function pideInterfaz (c: Criterio) {
  return c.tipo == 'VLAN_PUERTO' || c.tipo == 'TRUNK' || (c.tipo == 'IP_INTERFAZ' && nodo(c.nodo)?.tipo == 'dynamips')
}
function pideValor (tipo: string) {
  return tipo in etiquetaValor
}
function cambiarTipo (c: Criterio) {
  c.modo = modosCriterio[c.tipo]?.[0]?.value || ''
  c.valor = c.tipo == 'VECINOS_OSPF' ? '1' : ''
  c.interfaz = ''
  c.destino = ''
  if (!nodosPara(c.tipo).includes(c.nodo)) c.nodo = ''
}
function enCurso (r: Revision) {
  return r.estado == 'EN_COLA' || r.estado == 'REVISANDO'
}

function cargar () {
  axios.get(`/api/practicas/${id}`).then(response => {
    practica.value = Object.assign(new Practica(), response.data)
    criterios.value = practica.value.criterios.map(c => Object.assign(new Criterio(), c))
    datos.value = new SolicitudPractica('', practica.value.nombre, practica.value.id_grupo,
      practica.value.instrucciones, practica.value.enable_password)
    vigilar()
  }).catch(err => {
    if (err.response?.status == 404 || err.response?.status == 403) router.push('/practicas')
  })
}
function cargarEntregas () {
  axios.get(`/api/practicas/${id}/entregas`).then(response => entregas.value = response.data)
}
function cargarPrueba () {
  axios.get(`/api/practicas/${id}/prueba`).then(response => {
    prueba.value = response.data
    vigilar()
  })
}

// Mientras se exporta o se prueba, se pregunta cada 2 s
function vigilar () {
  const ocupado = practica.value.estado == 'EXPORTANDO' || (!!prueba.value?.revision && enCurso(prueba.value.revision))
  if (ocupado && !temporizador) {
    temporizador = setInterval(() => {
      if (practica.value.estado == 'EXPORTANDO') cargar()
      cargarPrueba()
    }, 2000)
  }
  if (!ocupado && temporizador) {
    clearInterval(temporizador)
    temporizador = undefined
  }
}

function resultado (promesa: Promise<unknown>, ok: string, despues?: () => void) {
  error.value = ''
  aviso.value = ''
  return promesa
    .then(() => {
      aviso.value = ok
      despues?.()
    })
    .catch(err => error.value = typeof err.response?.data == 'string' ? err.response.data : 'No se pudo completar')
}

function accion (que: string) {
  confirmar.value = ''
  if (que == 'borrar') {
    resultado(axios.delete(`/api/practicas/${id}`), 'Práctica borrada', () => router.push('/practicas'))
    return
  }
  resultado(axios.post(`/api/practicas/${id}/${que}`),
    que == 'activar' ? 'Práctica activa: los alumnos ya la ven' : 'Práctica cerrada', cargar)
}

function guardarCriterios () {
  guardando.value = true
  resultado(axios.put(`/api/practicas/${id}/criterios`, criterios.value), 'Criterios guardados', cargar)
    .finally(() => guardando.value = false)
}

function guardarDatos () {
  resultado(axios.put(`/api/practicas/${id}`, datos.value), 'Datos guardados', cargar)
}

function probar () {
  resultado(axios.post(`/api/practicas/${id}/prueba`), 'Revisando tu proyecto…', cargarPrueba)
}

function verEntrega (f: FilaEntrega) {
  detalle.value = null
  if (!f.id_revision) return
  axios.get(`/api/practicas/${id}/entregas/${f.id_usuario}`).then(response => detalle.value = response.data)
}

function abrirEntrega () {
  if (!detalle.value) return
  abriendo.value = true
  error.value = ''
  aviso.value = ''
  axios.post(`/api/practicas/${id}/entregas/${detalle.value.fila.id_usuario}/abrir`)
    .then(response => aviso.value = `Se copió como "${response.data.proyecto}" en tu laboratorio: ábrelo en GNS3 (File → Open project).`)
    .catch(err => error.value = err.response?.data || 'No se pudo copiar')
    .finally(() => abriendo.value = false)
}

onMounted(() => {
  cargar()
  cargarEntregas()
  cargarPrueba()
  axios.get('/api/grupos').then(response => grupos.value = response.data)
})
onBeforeUnmount(() => clearInterval(temporizador))
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
