<template>
  <AppFrame>
    <!-- Sin laboratorio -->
    <v-row v-if="lab.estado == 'NINGUNO' || lab.estado == 'ERROR'" class="my-2">
      <v-col cols="12">
        <v-alert v-if="lab.estado == 'ERROR'" type="error" variant="tonal">{{ lab.mensaje }}</v-alert>
      </v-col>

      <v-col cols="12" md="6">
        <v-card border class="h-100" elevation="0" rounded="lg" title="Crear laboratorio">
          <template #text>
            <p class="mb-4 text-medium-emphasis">
              Se crea un entorno GNS3 solo para ti. En modo equipo se genera un código para que tus compañeros se unan.
            </p>
            <v-switch v-model="colaborativo" color="primary" hide-details inset label="Trabajar en equipo" />
          </template>
          <template #actions>
            <v-btn color="primary" :loading="procesando" prepend-icon="mdi-rocket-launch" text="Crear" @click="crear" />
          </template>
        </v-card>
      </v-col>

      <v-col cols="12" md="6">
        <v-card border class="h-100" elevation="0" rounded="lg" title="Unirse a un equipo">
          <template #text>
            <p class="mb-4 text-medium-emphasis">Escribe el código que te compartió tu compañero.</p>
            <v-text-field v-model="codigo" hide-details label="Código" variant="outlined" />
          </template>
          <template #actions>
            <v-btn color="primary" :disabled="!codigo" :loading="procesando" prepend-icon="mdi-account-plus" text="Unirse" @click="unirse" />
          </template>
        </v-card>
      </v-col>
    </v-row>

    <!-- Creando -->
    <v-card v-else-if="lab.estado == 'CREANDO'" border class="my-5 text-center pa-8" elevation="0" rounded="lg">
      <v-progress-circular color="primary" indeterminate size="64" />
      <div class="text-title-large mt-4">Preparando tu laboratorio…</div>
      <div class="text-medium-emphasis">Esto puede tardar un par de minutos.</div>
    </v-card>

    <!-- Corriendo -->
    <template v-else>
      <v-card border class="my-5" elevation="0" rounded="lg" title="Mi laboratorio">
        <template #append>
          <v-chip color="success" label size="small" text="En ejecución" />
        </template>

        <template #text>
          <v-row>
            <v-col cols="12" md="6">
              <v-text-field
                append-inner-icon="mdi-content-copy"
                label="Dirección de GNS3"
                :model-value="urlGns3"
                readonly
                variant="outlined"
                @click:append-inner="copiar(urlGns3)"
              />
            </v-col>
            <v-col cols="6" md="3">
              <v-text-field
                append-inner-icon="mdi-content-copy"
                label="Usuario"
                :model-value="lab.usuario"
                readonly
                variant="outlined"
                @click:append-inner="copiar(lab.usuario)"
              />
            </v-col>
            <v-col cols="6" md="3">
              <v-text-field
                append-inner-icon="mdi-content-copy"
                label="Contraseña"
                :model-value="lab.token"
                readonly
                variant="outlined"
                @click:append-inner="copiar(lab.token)"
              />
            </v-col>
            <v-col v-if="lab.codigo" cols="12" md="6">
              <v-text-field
                append-inner-icon="mdi-content-copy"
                hint="Compártelo con tu equipo para que se unan"
                label="Código del equipo"
                :model-value="lab.codigo"
                persistent-hint
                readonly
                variant="outlined"
                @click:append-inner="copiar(lab.codigo)"
              />
            </v-col>
            <v-col v-if="!lab.soy_dueno" cols="12" md="6">
              <v-text-field label="Dueño" :model-value="lab.dueno" readonly variant="outlined" />
            </v-col>
          </v-row>
        </template>

        <template #actions>
          <v-btn color="primary" :href="urlGns3" prepend-icon="mdi-open-in-new" target="_blank" text="Abrir GNS3" />
          <v-spacer />
          <v-btn
            color="error"
            :loading="procesando"
            :prepend-icon="lab.soy_dueno ? 'mdi-delete' : 'mdi-logout'"
            :text="lab.soy_dueno ? 'Limpiar entorno' : 'Salir del equipo'"
            @click="salir"
          />
        </template>
      </v-card>

      <!-- Equipo -->
      <v-card v-if="lab.codigo" border class="my-5" elevation="0" rounded="lg" title="Equipo">
        <template #append>
          <v-btn v-if="lab.soy_dueno" color="primary" prepend-icon="mdi-email-plus" text="Invitar" @click="openInvitar" />
        </template>
        <template #text>
          <v-list-item
            v-for="m in lab.miembros"
            :key="m.id"
            class="px-0"
            prepend-icon="mdi-account"
            :subtitle="m.matricula"
            :title="m.nombre"
          />
          <div v-if="lab.miembros.length == 0" class="text-medium-emphasis">Todavía no se une nadie.</div>
        </template>
      </v-card>
    </template>

    <!-- Invitaciones recibidas -->
    <v-card v-if="invitaciones.length" border class="my-5" elevation="0" rounded="lg" title="Invitaciones">
      <template #text>
        <v-list-item
          v-for="inv in invitaciones"
          :key="inv.id"
          class="px-0"
          prepend-icon="mdi-email"
          :subtitle="inv.matricula"
          :title="`${inv.emisor} te invitó a su laboratorio`"
        >
          <template #append>
            <v-btn class="me-2" color="success" size="small" text="Aceptar" @click="responder(inv, true)" />
            <v-btn size="small" text="Rechazar" @click="responder(inv, false)" />
          </template>
        </v-list-item>
      </template>
    </v-card>

    <!-- Dialogo Invitar -->
    <v-dialog v-model="dialogInvitar" max-width="500">
      <v-card rounded="lg" title="Invitar compañero">
        <template #text>
          <v-autocomplete
            v-model="invitado"
            item-title="nombre"
            item-value="id"
            :items="companeros"
            label="Compañero de tu grupo"
            variant="outlined"
          />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cancelar" @click="dialogInvitar = false" />
          <v-btn color="primary" :disabled="!invitado" text="Invitar" @click="invitar" />
        </template>
      </v-card>
    </v-dialog>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onBeforeUnmount, onMounted, ref } from 'vue'
import axios from 'axios'
import Swal from 'sweetalert2'
import { type Invitacion, Laboratorio } from '@/models/Laboratorio'
import type { Persona } from '@/models/Grupo'

const lab = ref<Laboratorio>(new Laboratorio())
const invitaciones = ref<Invitacion[]>([])
const companeros = ref<Persona[]>([])
const colaborativo = ref(false)
const codigo = ref('')
const procesando = ref(false)
const dialogInvitar = ref(false)
const invitado = ref<string | null>(null)

let timerLab: ReturnType<typeof setTimeout> | undefined
let timerInv: ReturnType<typeof setInterval> | undefined

// GNS3 se abre en el mismo servidor, en el puerto del laboratorio
const urlGns3 = computed(() => `http://${window.location.hostname}:${lab.value.puerto}`)

function load () {
  clearTimeout(timerLab)
  axios.get('/api/labs').then(response => {
    lab.value = response.data
    // Mientras se crea, vuelve a preguntar cada 3 segundos
    if (lab.value.estado == 'CREANDO') {
      timerLab = setTimeout(load, 3000)
    }
  })
}

function loadInvitaciones () {
  axios.get('/api/labs/invitaciones').then(response => invitaciones.value = response.data)
}

function error (err: any, texto: string) {
  Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || texto })
}

function crear () {
  procesando.value = true
  axios.post('/api/labs', { colaborativo: colaborativo.value })
    .then(() => load())
    .catch(err => error(err, 'No se pudo crear el laboratorio.'))
    .finally(() => procesando.value = false)
}

function unirse () {
  procesando.value = true
  axios.post('/api/labs/unirse', { codigo: codigo.value.trim() })
    .then(() => {
      codigo.value = ''
      load()
    })
    .catch(err => error(err, 'No se pudo unir al equipo.'))
    .finally(() => procesando.value = false)
}

function salir () {
  Swal.fire({
    icon: 'warning',
    title: lab.value.soy_dueno ? '¿Limpiar el entorno?' : '¿Salir del equipo?',
    text: lab.value.soy_dueno ? 'Se borrará el laboratorio y todo lo que tenga, también para tu equipo.' : 'Podrás volver a unirte con el código.',
    showCancelButton: true,
    confirmButtonText: 'Sí',
    cancelButtonText: 'Cancelar',
    confirmButtonColor: '#d33',
  }).then(result => {
    if (!result.isConfirmed) return
    procesando.value = true
    axios.delete('/api/labs')
      .then(() => load())
      .catch(err => error(err, 'No se pudo completar.'))
      .finally(() => procesando.value = false)
  })
}

function openInvitar () {
  invitado.value = null
  dialogInvitar.value = true
  axios.get('/api/labs/companeros').then(response => companeros.value = response.data)
}

function invitar () {
  axios.post('/api/labs/invitaciones', { id_usuario: invitado.value })
    .then(() => {
      dialogInvitar.value = false
      Swal.fire({ icon: 'success', title: 'Invitación enviada', timer: 1500, showConfirmButton: false })
    })
    .catch(err => error(err, 'No se pudo invitar.'))
}

function responder (inv: Invitacion, aceptar: boolean) {
  axios.put(`/api/labs/invitaciones/${inv.id}`, { aceptar })
    .then(() => {
      loadInvitaciones()
      load()
    })
    .catch(err => error(err, 'No se pudo responder.'))
}

function copiar (texto: string) {
  navigator.clipboard.writeText(texto)
  Swal.fire({ icon: 'success', title: 'Copiado', timer: 800, showConfirmButton: false })
}

onMounted(() => {
  load()
  loadInvitaciones()
  timerInv = setInterval(loadInvitaciones, 20000)   // revisa invitaciones cada 20 s
})

onBeforeUnmount(() => {
  clearTimeout(timerLab)
  clearInterval(timerInv)
})
</script>
