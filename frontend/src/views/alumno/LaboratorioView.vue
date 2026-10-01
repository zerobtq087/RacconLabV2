<template>
  <div class="d-flex flex-wrap align-center justify-space-between ga-4 mb-6">
    <div>
      <div class="page-title">Mi laboratorio GNS3</div>
      <div class="page-subtitle">Levanta tu entorno aislado o únete al de tu equipo.</div>
    </div>
    <v-btn prepend-icon="mdi-account-group" variant="tonal" @click="abrirEquipo">
      Equipo e invitaciones
      <v-badge v-if="invitaciones.length" :content="invitaciones.length" color="error" inline />
    </v-btn>
  </div>

  <v-row>
    <!-- Controles -->
    <v-col cols="12" md="5">
      <v-card title="Controles" prepend-icon="mdi-tune-variant" :loading="consultando">
        <v-card-text>
          <v-switch
            v-model="colaborativo"
            :disabled="activo"
            color="primary"
            inset
            label="Modo colaborativo (equipo)"
            hide-details
            class="mb-2"
          />

          <v-expand-transition>
            <div v-if="!activo && colaborativo" class="mb-4">
              <div class="text-caption font-weight-bold mb-2">¿Unirte a un laboratorio existente?</div>
              <div class="d-flex ga-2">
                <v-text-field v-model="codigoUnirse" label="COLAB-XXXX" hide-details class="font-mono" />
                <v-btn color="secondary" height="48" :loading="creando" :disabled="!codigoUnirse.trim()" @click="unirse">
                  Unirme
                </v-btn>
              </div>
            </div>
          </v-expand-transition>

          <v-alert v-if="activo && estado.codigo_lab" type="info" variant="tonal" class="mb-4">
            <div class="text-caption">{{ estado.soy_dueno ? 'Comparte este código con tu equipo' : 'Estás en el laboratorio del equipo' }}</div>
            <div class="text-h6 font-mono">{{ estado.codigo_lab }}</div>
          </v-alert>

          <v-btn
            block size="large" color="primary" class="mb-3" prepend-icon="mdi-rocket-launch"
            :loading="creando" :disabled="activo" @click="desplegar"
          >
            {{ activo ? 'Laboratorio activo' : colaborativo ? 'Crear laboratorio de equipo' : 'Desplegar laboratorio' }}
          </v-btn>

          <v-btn
            block variant="outlined" color="error"
            :prepend-icon="activo && !estado.soy_dueno ? 'mdi-exit-run' : 'mdi-broom'"
            :loading="limpiando" :disabled="!activo" @click="confirmar = true"
          >
            {{ activo && !estado.soy_dueno ? 'Salir del laboratorio' : 'Limpiar entorno' }}
          </v-btn>
        </v-card-text>
      </v-card>
    </v-col>

    <!-- Credenciales -->
    <v-col cols="12" md="7">
      <v-card title="Acceso a GNS3" prepend-icon="mdi-key-chain">
        <template #append>
          <v-chip :color="activo ? 'success' : 'grey'" size="small" label>
            <v-icon start :icon="activo ? 'mdi-circle' : 'mdi-circle-outline'" size="10" />
            {{ activo ? 'En ejecución' : 'Detenido' }}
          </v-chip>
        </template>
        <v-card-text>
          <div v-if="!activo" class="text-medium-emphasis py-6 text-center">
            <v-icon icon="mdi-server-off" size="48" class="mb-2" />
            <div>No hay un laboratorio activo.</div>
            <div class="text-caption">Al desplegarlo verás aquí la IP, el puerto y el token.</div>
          </div>
          <v-row v-else dense>
            <v-col cols="12" sm="7"><CampoCopiable label="Dirección IP" :valor="estado.ip_real_host" /></v-col>
            <v-col cols="12" sm="5"><CampoCopiable label="Puerto" :valor="estado.puerto_web" /></v-col>
            <v-col cols="12"><CampoCopiable label="Token de acceso" :valor="estado.token_acceso" secreto /></v-col>
            <v-col cols="12">
              <v-btn :href="urlWeb" target="_blank" color="secondary" variant="tonal" prepend-icon="mdi-open-in-new" block>
                Abrir GNS3 Web UI
              </v-btn>
            </v-col>
          </v-row>
        </v-card-text>
      </v-card>
    </v-col>
  </v-row>

  <!-- Panel de equipo -->
  <v-navigation-drawer v-model="panelEquipo" location="right" temporary width="380">
    <v-toolbar flat density="comfortable" title="Equipo">
      <v-btn icon="mdi-refresh" @click="cargarEquipo" />
      <v-btn icon="mdi-close" @click="panelEquipo = false" />
    </v-toolbar>

    <v-list density="comfortable">
      <v-list-subheader>Invitaciones recibidas</v-list-subheader>
      <v-list-item v-if="!invitaciones.length" class="text-medium-emphasis" title="Sin invitaciones pendientes" />
      <v-list-item
        v-for="inv in invitaciones"
        :key="inv.id_invitacion"
        :title="inv.nombre_emisor || inv.emisor"
        subtitle="Te invita a su laboratorio"
        prepend-icon="mdi-email-outline"
      >
        <template #append>
          <v-btn icon="mdi-check" color="success" variant="text" size="small" @click="responder(inv, true)" />
          <v-btn icon="mdi-close" color="error" variant="text" size="small" @click="responder(inv, false)" />
        </template>
      </v-list-item>

      <v-divider class="my-2" />
      <v-list-subheader>Compañeros de tu grupo</v-list-subheader>
      <v-list-item v-if="!companeros.length" class="text-medium-emphasis" title="No hay compañeros en tu grupo" />
      <v-list-item
        v-for="c in companeros"
        :key="c.matricula"
        :title="c.nombre"
        :subtitle="c.ocupado ? `${c.matricula} · en un laboratorio` : c.matricula"
        prepend-icon="mdi-account"
      >
        <template #append>
          <v-btn
            v-if="puedeInvitar"
            size="small" color="primary" variant="tonal"
            :disabled="c.ocupado" :loading="invitando === c.matricula"
            @click="invitar(c)"
          >
            Invitar
          </v-btn>
        </template>
      </v-list-item>
    </v-list>

    <div v-if="!puedeInvitar" class="text-caption text-medium-emphasis pa-4">
      Para invitar, crea un laboratorio en modo colaborativo.
    </div>
  </v-navigation-drawer>

  <!-- Confirmar limpieza -->
  <v-dialog v-model="confirmar" max-width="420">
    <v-card :title="estado.soy_dueno ? '¿Limpiar el entorno?' : '¿Salir del laboratorio?'" prepend-icon="mdi-alert">
      <v-card-text>
        <template v-if="estado.soy_dueno && estado.es_colaborativo">
          Se cerrará el laboratorio para <b>todo el equipo</b>.
        </template>
        <template v-else-if="estado.soy_dueno">
          Se detendrá tu contenedor GNS3 y se liberarán los recursos.
        </template>
        <template v-else>
          Saldrás del laboratorio; el resto del equipo puede seguir trabajando.
        </template>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="confirmar = false">Cancelar</v-btn>
        <v-btn color="error" @click="limpiar">Confirmar</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed, onMounted } from 'vue'
import CampoCopiable from '@/components/CampoCopiable.vue'
import { labsApi } from '@/api/labs'
import { mensajeError } from '@/api/http'
import { useNotificaciones } from '@/stores/notificaciones'

const noti = useNotificaciones()

const estado = ref({})
const colaborativo = ref(false)
const codigoUnirse = ref('')
const consultando = ref(false)
const creando = ref(false)
const limpiando = ref(false)
const confirmar = ref(false)

const panelEquipo = ref(false)
const companeros = ref([])
const invitaciones = ref([])
const invitando = ref(null)

const activo = computed(() => !!(estado.value.running || estado.value.status?.running))
const puedeInvitar = computed(() => activo.value && estado.value.es_colaborativo && estado.value.soy_dueno)
const urlWeb = computed(() => `http://${estado.value.ip_real_host}:${estado.value.puerto_web}/`)

const consultar = async () => {
  consultando.value = true
  try {
    estado.value = await labsApi.estado()
    if (activo.value) colaborativo.value = !!estado.value.es_colaborativo
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo consultar el laboratorio'))
  } finally {
    consultando.value = false
  }
}

const desplegar = async () => {
  creando.value = true
  try {
    await labsApi.crear({ es_colaborativo: colaborativo.value })
    await consultar()
    noti.exito('Laboratorio desplegado')
  } catch (e) {
    if (e.response?.status === 409) await consultar()
    noti.error(mensajeError(e, 'No se pudo desplegar el laboratorio'))
  } finally {
    creando.value = false
  }
}

const unirse = async () => {
  creando.value = true
  try {
    await labsApi.crear({ codigo_lab: codigoUnirse.value.trim().toUpperCase() })
    codigoUnirse.value = ''
    await consultar()
    noti.exito('Te uniste al laboratorio del equipo')
  } catch (e) {
    noti.error(mensajeError(e, 'Código inválido o laboratorio cerrado'))
  } finally {
    creando.value = false
  }
}

const limpiar = async () => {
  confirmar.value = false
  limpiando.value = true
  try {
    const r = await labsApi.limpiar()
    colaborativo.value = false
    await consultar()
    noti.exito(r?.message || 'Entorno limpiado')
  } catch (e) {
    await consultar()
    noti.error(mensajeError(e, 'No se pudo limpiar el entorno'))
  } finally {
    limpiando.value = false
  }
}

const cargarEquipo = async () => {
  const [c, i] = await Promise.allSettled([labsApi.companeros(), labsApi.pendientes()])
  companeros.value = c.value || []
  invitaciones.value = i.value || []
}

const abrirEquipo = () => {
  panelEquipo.value = true
  cargarEquipo()
}

const invitar = async (c) => {
  invitando.value = c.matricula
  try {
    await labsApi.invitar(c.matricula)
    noti.exito(`Invitación enviada a ${c.nombre}`)
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo invitar'))
  } finally {
    invitando.value = null
  }
}

const responder = async (inv, aceptar) => {
  try {
    await labsApi.responder(inv.id_invitacion, aceptar)
    if (aceptar) {
      panelEquipo.value = false
      await consultar()
      noti.exito('Te uniste al laboratorio')
    } else {
      noti.info('Invitación rechazada')
    }
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo responder'))
  } finally {
    cargarEquipo()
  }
}

onMounted(async () => {
  await consultar()
  cargarEquipo()
})
</script>