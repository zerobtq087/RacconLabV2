<template>
  <AppFrame>
    <v-alert v-if="!labListo" class="my-5" type="info" variant="tonal">
      Para desplegar una plantilla necesitas tu laboratorio en ejecución.
      <template #append>
        <v-btn color="primary" text="Ir a mi laboratorio" to="/laboratorio" />
      </template>
    </v-alert>

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
              <v-text-field v-model="form.nombre_lab" label="Nombre del laboratorio" variant="outlined" />

              <v-row v-if="form.plantilla != 'basica'" dense>
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
                  <v-select v-model="serial.cidr" :items="[24, 29, 30]" label="Prefijo" prefix="/" variant="outlined" />
                </v-col>
              </v-row>

              <template v-if="form.plantilla == 'avanzada'">
                <div class="text-subtitle-2 mb-2">VLANs</div>
                <v-row v-for="(v, i) in form.vlans" :key="i" dense>
                  <v-col cols="3"><v-text-field v-model.number="v.id" label="ID" type="number" variant="outlined" /></v-col>
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
              :disabled="!labListo || !form.plantilla"
              :loading="cargando"
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
import { computed, onMounted, reactive, ref } from 'vue'
import axios from 'axios'
import Swal from 'sweetalert2'
import { Despliegue } from '@/models/Despliegue'

const plantillas = [
  { clave: 'basica', titulo: 'Básica (1 router)', imagen: '/img/topologies/topologia_basica.png' },
  { clave: 'intermedia', titulo: 'Intermedia (varios routers)', imagen: '/img/topologies/topologia_intermedia.png' },
  { clave: 'avanzada', titulo: 'Avanzada (varios routers + VLANs)', imagen: '/img/topologies/topologia_avanzada.png' },
]

const protocolos = [
  { text: 'Rutas estáticas', value: 'static' },
  { text: 'OSPF', value: 'ospf' },
  { text: 'RIP v2', value: 'rip' },
]

const form = ref<Despliegue>(new Despliegue())
const lan = reactive({ ip: '192.168.1.0', cidr: 24 })
const serial = reactive({ ip: '10.0.0.0', cidr: 24 })
const cargando = ref(false)
const labListo = ref(false)

const plantilla = computed(() => plantillas.find(p => p.clave == form.value.plantilla))

const ipValida = (v: string) =>
  /^(25[0-5]|2[0-4]\d|1?\d?\d)(\.(25[0-5]|2[0-4]\d|1?\d?\d)){3}$/.test(v) || 'IPv4 inválida'

// Valores de ejemplo de cada plantilla
function aplicarPlantilla (clave: string) {
  if (clave == 'basica') {
    form.value = new Despliegue(clave, 'Lab_Basico')
    Object.assign(lan, { ip: '192.168.1.0', cidr: 24 })
  } else if (clave == 'intermedia') {
    form.value = new Despliegue(clave, 'Lab_Intermedio')
    Object.assign(lan, { ip: '192.168.0.0', cidr: 16 })
  } else {
    form.value = new Despliegue(clave, 'Lab_Avanzado_VLAN', '', '', 'ospf', 0, [
      { id: 10, subnet: '192.168.10.0/24', gateway: '192.168.10.1' },
      { id: 20, subnet: '192.168.20.0/24', gateway: '192.168.20.1' },
    ])
  }
}

// Solo se puede desplegar sobre el laboratorio propio en ejecución
function revisarLab () {
  axios.get('/api/labs').then(response => {
    labListo.value = response.data.estado == 'CORRIENDO' && response.data.soy_dueno
  })
}

function desplegar () {
  const datos: Despliegue = {
    ...form.value,
    subnet: `${lan.ip}/${lan.cidr}`,
    serial_subnet: form.value.plantilla != 'basica' ? `${serial.ip}/${serial.cidr}` : '',
    vlans: form.value.plantilla == 'avanzada' ? form.value.vlans : [],
  }

  cargando.value = true
  axios.post('/api/plantillas/despliegue', datos)
    .then(() => {
      Swal.fire({ icon: 'success', title: 'Topología desplegada', text: 'Ábrela desde tu laboratorio en GNS3.' })
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'Falló el despliegue', text: err.response?.data || 'Error al ejecutar Ansible.' })
    })
    .finally(() => cargando.value = false)
}

onMounted(revisarLab)
</script>
