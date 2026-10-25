<template>
  <AppFrame>
    <v-alert v-if="!labListo" class="my-5" type="info" variant="tonal">
      Para desplegar una plantilla necesitas tu laboratorio en ejecución.
      <template #append>
        <v-btn color="primary" text="Ir a mi laboratorio" to="/laboratorio" />
      </template>
    </v-alert>

    <!-- Avance y resultado del despliegue (dentro de la página, sin ventanas) -->
    <v-card v-if="estado.estado != 'NINGUNO'" border class="my-4" elevation="0" rounded="lg">
      <v-card-text>
        <div class="d-flex align-center mb-2">
          <v-icon class="mr-2" :color="colorEstado" :icon="iconoEstado" />
          <span class="text-subtitle-1 font-weight-medium">{{ tituloEstado }}</span>
          <v-spacer />
          <span class="text-medium-emphasis">{{ estado.proyecto }}</span>
        </div>
        <v-progress-linear
          v-if="estado.estado == 'PROCESANDO'"
          color="primary"
          height="22"
          :model-value="estado.porcentaje"
          rounded
          striped
        >
          <strong>{{ estado.porcentaje }}%</strong>
        </v-progress-linear>
        <div v-if="estado.estado == 'PROCESANDO'" class="text-medium-emphasis mt-2">
          {{ estado.paso }}…
        </div>
        <v-alert
          v-else
          class="mt-2"
          :text="estado.mensaje"
          :type="estado.estado == 'TERMINADO' ? 'success' : 'error'"
          variant="tonal"
        />
      </v-card-text>
    </v-card>

    <v-alert
      v-if="errorEnvio"
      class="my-4"
      closable
      :text="errorEnvio"
      type="error"
      variant="tonal"
      @click:close="errorEnvio = ''"
    />

    <v-row class="my-2">
      <v-col cols="12" lg="6">
        <v-card border class="h-100" elevation="0" rounded="lg" title="1. Escenario y parámetros">
          <template #text>
            <v-select
              v-model="form.plantilla"
              item-title="titulo"
              item-value="clave"
              :items="plantillas"
              label="Plantilla"
              variant="outlined"
              @update:model-value="aplicarPlantilla"
            />

            <template v-if="form.plantilla">
              <v-text-field
                v-model="form.nombre_lab"
                hint="Letras, números, guion o guion bajo (sin espacios)"
                label="Nombre del laboratorio"
                persistent-hint
                variant="outlined"
              />

              <v-row class="mt-1" dense>
                <v-col cols="7">
                  <v-select v-model="form.routing_protocol" item-title="text" item-value="value" :items="protocolos" label="Enrutamiento" variant="outlined" />
                </v-col>
                <v-col v-if="form.routing_protocol == 'ospf'" cols="5">
                  <v-text-field v-model.number="form.ospf_area" label="Área OSPF" min="0" type="number" variant="outlined" />
                </v-col>
              </v-row>

              <v-row dense>
                <v-col cols="8">
                  <v-text-field v-model="lan.ip" label="Red LAN" :rules="[ipValida]" variant="outlined" />
                </v-col>
                <v-col cols="4">
                  <v-select v-model="lan.cidr" :items="[16, 20, 22, 24, 26, 28]" label="Prefijo" prefix="/" variant="outlined" />
                </v-col>
              </v-row>

              <v-row v-if="form.plantilla != 'basica'" dense>
                <v-col cols="8">
                  <v-text-field v-model="serial.ip" label="Red de enlaces seriales" :rules="[ipValida]" variant="outlined" />
                </v-col>
                <v-col cols="4">
                  <v-select v-model="serial.cidr" :items="[24, 26, 27, 28, 29]" label="Prefijo" prefix="/" variant="outlined" />
                </v-col>
              </v-row>

              <template v-if="form.plantilla == 'avanzada'">
                <div class="text-subtitle-2">VLANs</div>
                <div class="text-caption text-medium-emphasis mb-2">
                  La topología avanzada usa las VLAN 10 y 20 (switch y subinterfaces de R1); elige su subred y gateway.
                </div>
                <v-row v-for="(v, i) in form.vlans" :key="i" dense>
                  <v-col cols="3"><v-text-field v-model.number="v.id" label="ID" readonly type="number" variant="outlined" /></v-col>
                  <v-col cols="5"><v-text-field v-model="v.subnet" label="Subred" variant="outlined" /></v-col>
                  <v-col cols="4"><v-text-field v-model="v.gateway" label="Gateway" variant="outlined" /></v-col>
                </v-row>
              </template>
            </template>
          </template>

          <template #actions>
            <v-btn
              block
              color="primary"
              :disabled="!labListo || !form.plantilla || ocupado"
              :loading="enviando || ocupado"
              prepend-icon="mdi-play"
              size="large"
              text="Desplegar en GNS3"
              variant="flat"
              @click="desplegar"
            />
          </template>
        </v-card>
      </v-col>

      <v-col cols="12" lg="6">
        <v-card border class="h-100" elevation="0" rounded="lg" title="2. Diagrama">
          <template #text>
            <div class="d-flex align-center justify-center" style="min-height: 380px">
              <v-img v-if="plantilla" contain max-height="420" :src="plantilla.imagen" />
              <div v-else class="text-medium-emphasis">Selecciona una plantilla para ver su diagrama.</div>
            </div>
          </template>
        </v-card>
      </v-col>
    </v-row>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onBeforeUnmount, onMounted, reactive, ref } from 'vue'
import axios from 'axios'
import { Despliegue, EstadoDespliegue } from '@/models/Despliegue'

const plantillas = [
  { clave: 'basica', titulo: 'Básica (1 router)', imagen: '/img/topologies/topologia_basica.png' },
  { clave: 'intermedia', titulo: 'Intermedia (varios routers)', imagen: '/img/topologies/topologia_intermedia.png' },
  { clave: 'avanzada', titulo: 'Avanzada (varios routers + VLANs)', imagen: '/img/topologies/topologia_avanzada.png' },
]

const protocolos = [
  { text: 'RIP v2', value: 'rip' },
  { text: 'OSPF', value: 'ospf' },
]

const form = ref<Despliegue>(new Despliegue())
const lan = reactive({ ip: '192.168.1.0', cidr: 24 })
const serial = reactive({ ip: '10.0.0.0', cidr: 24 })
const enviando = ref(false)
const labListo = ref(false)
const estado = ref<EstadoDespliegue>(new EstadoDespliegue())
const errorEnvio = ref('')

let timerEstado: ReturnType<typeof setTimeout> | undefined

const plantilla = computed(() => plantillas.find(p => p.clave == form.value.plantilla))
const ocupado = computed(() => estado.value.estado == 'PROCESANDO')

const tituloEstado = computed(() => ({
  PROCESANDO: 'Desplegando la topología',
  TERMINADO: 'Despliegue terminado',
  ERROR: 'El despliegue falló',
} as Record<string, string>)[estado.value.estado] || '')
const iconoEstado = computed(() => ({ PROCESANDO: 'mdi-progress-clock', TERMINADO: 'mdi-check-circle', ERROR: 'mdi-alert-circle' } as Record<string, string>)[estado.value.estado])
const colorEstado = computed(() => ({ PROCESANDO: 'primary', TERMINADO: 'success', ERROR: 'error' } as Record<string, string>)[estado.value.estado])

const ipValida = (v: string) =>
  /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/.test(v) || 'IPv4 inválida'

// Valores de ejemplo de cada plantilla (como el original)
function aplicarPlantilla (clave: string) {
  if (clave == 'basica') {
    form.value = new Despliegue(clave, 'Lab_Basico')
    Object.assign(lan, { ip: '192.168.1.0', cidr: 24 })
  } else if (clave == 'intermedia') {
    form.value = new Despliegue(clave, 'Lab_Intermedio')
    Object.assign(lan, { ip: '192.168.0.0', cidr: 16 })
    Object.assign(serial, { ip: '10.0.0.0', cidr: 24 })
  } else {
    form.value = new Despliegue(clave, 'Lab_Avanzado_VLAN', '', '', 'ospf', 0, [
      { id: 10, subnet: '192.168.10.0/24', gateway: '192.168.10.1' },
      { id: 20, subnet: '192.168.20.0/24', gateway: '192.168.20.1' },
    ])
    Object.assign(lan, { ip: '192.168.0.0', cidr: 24 })
    Object.assign(serial, { ip: '10.0.0.0', cidr: 24 })
  }
}

// Solo se puede desplegar sobre el laboratorio propio en ejecución
function revisarLab () {
  axios.get('/api/labs').then(response => {
    labListo.value = response.data.estado == 'CORRIENDO' && response.data.soy_dueno
  })
}

// Avance del despliegue: mientras está en proceso se pregunta cada 2 segundos
function cargarEstado () {
  clearTimeout(timerEstado)
  axios.get('/api/plantillas/despliegue')
    .then(response => {
      estado.value = Object.assign(new EstadoDespliegue(), response.data)
      if (estado.value.estado == 'PROCESANDO') {
        timerEstado = setTimeout(cargarEstado, 2000)
      }
    })
    .catch(() => {
      timerEstado = setTimeout(cargarEstado, 5000)
    })
}

function desplegar () {
  const datos: Despliegue = {
    ...form.value,
    subnet: `${lan.ip}/${lan.cidr}`,
    serial_subnet: form.value.plantilla == 'basica' ? '' : `${serial.ip}/${serial.cidr}`,
    vlans: form.value.plantilla == 'avanzada' ? form.value.vlans : [],
  }

  enviando.value = true
  errorEnvio.value = ''
  axios.post('/api/plantillas/despliegue', datos)
    .then(() => cargarEstado())
    .catch(err => {
      errorEnvio.value = err.response?.data || 'No se pudo iniciar el despliegue.'
    })
    .finally(() => enviando.value = false)
}

onMounted(() => {
  revisarLab()
  cargarEstado()
})

onBeforeUnmount(() => clearTimeout(timerEstado))
</script>
