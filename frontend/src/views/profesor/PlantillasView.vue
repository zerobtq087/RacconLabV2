<template>
  <div class="d-flex flex-wrap align-center justify-space-between ga-4 mb-6">
    <div>
      <div class="page-title">Plantillas de topología</div>
      <div class="page-subtitle">Despliegue parametrizado con Ansible sobre tu workspace GNS3.</div>
    </div>
    <EstadoWorkspace style="min-width: 300px" @cambio="ws = $event" />
  </div>

  <v-row>
    <!-- Formulario -->
    <v-col cols="12" lg="6">
      <v-card title="1. Escenario y parámetros" prepend-icon="mdi-form-select">
        <v-card-text>
          <v-form ref="formRef">
            <v-select
              v-model="tipo"
              :items="PLANTILLAS"
              item-title="titulo"
              item-value="clave"
              label="Plantilla"
              @update:model-value="aplicarPlantilla"
            />

            <template v-if="tipo">
              <v-text-field v-model="form.nombre_lab" label="Nombre del laboratorio" class="font-mono" :rules="[req, nombreValido]" />

              <!-- Red LAN -->
              <div class="text-caption font-weight-bold mb-2">Red LAN</div>
              <v-row dense>
                <v-col cols="8">
                  <v-text-field v-model="lan.ip" label="Dirección de red" class="font-mono" :rules="[req, ipValida]" />
                </v-col>
                <v-col cols="4">
                  <v-select v-model="lan.prefijo" :items="PREFIJOS_LAN" label="Máscara" prefix="/" />
                </v-col>
              </v-row>
              <v-alert v-if="infoLan" density="compact" variant="tonal" :type="infoLan.esRed ? 'info' : 'warning'" class="mb-4 text-caption">
                <template v-if="!infoLan.esRed">
                  {{ lan.ip }} no es dirección de red; se usará <b>{{ infoLan.red }}/{{ lan.prefijo }}</b>.<br />
                </template>
                Máscara {{ infoLan.mascara }} · hosts {{ infoLan.primera }} – {{ infoLan.ultima }} ({{ infoLan.hosts }})
              </v-alert>

              <!-- Seriales -->
              <template v-if="tipo !== 'basica'">
                <div class="text-caption font-weight-bold mb-2">Enlaces entre routers</div>
                <v-row dense>
                  <v-col cols="8">
                    <v-text-field v-model="serial.ip" label="Red para enlaces" class="font-mono" :rules="[req, ipValida]" />
                  </v-col>
                  <v-col cols="4">
                    <v-select v-model="serial.prefijo" :items="[24, 26, 28]" label="Máscara" prefix="/" />
                  </v-col>
                </v-row>
                <div v-if="enlaces.length" class="text-caption text-medium-emphasis mb-4">
                  Se dividirá en /30: <span class="font-mono">{{ enlaces.map((e) => e.red).join(', ') }}…</span>
                </div>

                <!-- Enrutamiento -->
                <v-row dense>
                  <v-col cols="12" sm="7">
                    <v-select v-model="form.routing_protocol" :items="PROTOCOLOS" label="Enrutamiento" />
                  </v-col>
                  <v-col v-if="form.routing_protocol === 'ospf'" cols="12" sm="5">
                    <v-text-field v-model.number="form.ospf_area" type="number" min="0" label="Área OSPF" />
                  </v-col>
                </v-row>
              </template>

              <!-- VLANs -->
              <template v-if="tipo === 'avanzada'">
                <div class="d-flex align-center justify-space-between mb-2">
                  <div class="text-caption font-weight-bold">VLANs</div>
                  <v-btn size="small" variant="text" prepend-icon="mdi-plus" :disabled="form.vlans.length >= 6" @click="agregarVlan">Agregar</v-btn>
                </div>
                <v-row v-for="(v, i) in form.vlans" :key="i" dense align="center">
                  <v-col cols="3"><v-text-field v-model.number="v.id" label="ID" type="number" :rules="[vlanValida]" density="compact" /></v-col>
                  <v-col cols="4"><v-text-field v-model="v.subnet" label="Subred" class="font-mono" density="compact" /></v-col>
                  <v-col cols="4"><v-text-field v-model="v.gateway" label="Gateway" class="font-mono" density="compact" /></v-col>
                  <v-col cols="1">
                    <v-btn icon="mdi-delete" variant="text" size="small" color="error" :disabled="form.vlans.length <= 1" @click="form.vlans.splice(i, 1)" />
                  </v-col>
                </v-row>
              </template>
            </template>

            <v-alert v-if="!ws" type="warning" variant="tonal" density="compact" class="mb-3"
              text="Necesitas un workspace activo para desplegar." />

            <v-btn
              block size="large" color="primary" prepend-icon="mdi-play"
              :loading="desplegando" :disabled="!ws || !tipo" @click="desplegar"
            >
              {{ desplegando ? 'Ejecutando playbook de Ansible…' : 'Desplegar en GNS3' }}
            </v-btn>
          </v-form>
        </v-card-text>
      </v-card>
    </v-col>

    <!-- Diagrama -->
    <v-col cols="12" lg="6">
      <v-card title="2. Diagrama" prepend-icon="mdi-graph-outline" height="100%">
        <v-card-text class="d-flex flex-column align-center justify-center" style="min-height: 380px">
          <template v-if="plantilla">
            <v-img v-if="plantilla.imagen" :src="plantilla.imagen" max-height="400" width="100%" contain>
              <template #error>
                <div class="text-medium-emphasis text-center pa-10">
                  <v-icon icon="mdi-image-off" size="48" /><br />
                  Falta la imagen <span class="font-mono">{{ plantilla.imagen }}</span>
                </div>
              </template>
            </v-img>
            <div class="text-body-2 text-medium-emphasis mt-4 text-center">{{ plantilla.descripcion }}</div>
          </template>
          <div v-else class="text-medium-emphasis">Selecciona una plantilla para ver su diagrama.</div>
        </v-card-text>
      </v-card>
    </v-col>
  </v-row>
</template>

<script lang="ts" setup>
import { ref, reactive, computed } from 'vue'
import EstadoWorkspace from '@/components/EstadoWorkspace.vue'
import { plantillasApi } from '@/api/plantillas'
import { mensajeError } from '@/api/http'
import { useNotificaciones } from '@/stores/notificaciones'
import { esIPValida, calcularRed, subdividir } from '@/utils/ipv4'

const noti = useNotificaciones()

const PLANTILLAS = [
  {
    clave: 'basica', titulo: 'Básica (1 router)', archivo: 'basic_lan.json',
    imagen: '/img/topologies/topologia_basica.png',
    descripcion: 'Un router con una LAN y PCs. Ideal para practicar direccionamiento.'
  },
  {
    clave: 'intermedia', titulo: 'Intermedia (multi-router)', archivo: 'star_topology.json',
    imagen: '/img/topologies/topologia_intermedia.png',
    descripcion: 'Varios routers en estrella con enrutamiento estático, RIPv2 u OSPF.'
  },
  {
    clave: 'avanzada', titulo: 'Avanzada (multi-router + VLANs)', archivo: 'vlan_topology.json',
    imagen: '/img/topologies/topologia_avanzada.png',
    descripcion: 'Routers + switch con VLANs, trunk e inter-VLAN routing.'
  }
]
const PROTOCOLOS = [
  { title: 'Rutas estáticas', value: 'static' },
  { title: 'RIP v2', value: 'rip' },
  { title: 'OSPF', value: 'ospf' }
]
const PREFIJOS_LAN = [16, 20, 22, 23, 24, 25, 26, 27, 28]

const ws = ref(null)
const tipo = ref(null)
const desplegando = ref(false)
const formRef = ref(null)
const lan = reactive({ ip: '192.168.1.0', prefijo: 24 })
const serial = reactive({ ip: '10.0.0.0', prefijo: 24 })
const form = reactive({ nombre_lab: '', routing_protocol: 'ospf', ospf_area: 0, vlans: [] })

const plantilla = computed(() => PLANTILLAS.find((p) => p.clave === tipo.value))
const infoLan = computed(() => calcularRed(lan.ip, lan.prefijo))
const enlaces = computed(() => subdividir(serial.ip, serial.prefijo, 30, 3))

const req = (v) => (v !== '' && v !== null && v !== undefined) || 'Obligatorio'
const ipValida = (v) => esIPValida(v) || 'IPv4 inválida'
const nombreValido = (v) => /^[A-Za-z0-9_-]{3,40}$/.test(v) || 'Solo letras, números, - y _'
const vlanValida = (v) => (v >= 2 && v <= 4094) || '2-4094'

const aplicarPlantilla = (clave) => {
  if (clave === 'basica') {
    Object.assign(form, { nombre_lab: 'Lab_Basico', routing_protocol: 'ospf', vlans: [] })
    Object.assign(lan, { ip: '192.168.1.0', prefijo: 24 })
  } else if (clave === 'intermedia') {
    Object.assign(form, { nombre_lab: 'Lab_Intermedio', routing_protocol: 'ospf', vlans: [] })
    Object.assign(lan, { ip: '192.168.0.0', prefijo: 16 })
  } else {
    Object.assign(form, {
      nombre_lab: 'Lab_Avanzado_VLAN',
      routing_protocol: 'ospf',
      vlans: [
        { id: 10, subnet: '192.168.10.0/24', gateway: '192.168.10.1' },
        { id: 20, subnet: '192.168.20.0/24', gateway: '192.168.20.1' }
      ]
    })
  }
}

const agregarVlan = () => {
  const id = Math.max(0, ...form.vlans.map((v) => v.id)) + 10
  form.vlans.push({ id, subnet: `192.168.${id}.0/24`, gateway: `192.168.${id}.1` })
}

const desplegar = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return
  desplegando.value = true
  try {
    const r = await plantillasApi.desplegar({
      container_name: ws.value.container_name,
      puerto_gns3: ws.value.puerto_gns3,
      nombre_lab: form.nombre_lab,
      archivo_json: plantilla.value.archivo,
      subnet: `${infoLan.value.red}/${lan.prefijo}`,
      serial_subnet: tipo.value !== 'basica' ? `${serial.ip}/${serial.prefijo}` : '',
      routing_protocol: form.routing_protocol,
      ospf_area: Number(form.ospf_area) || 0,
      vlans: tipo.value === 'avanzada' ? form.vlans : []
    })
    noti.exito(r?.message || 'Topología desplegada')
  } catch (e) {
    noti.error(mensajeError(e, 'Falló el despliegue con Ansible'))
  } finally {
    desplegando.value = false
  }
}
</script>