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
          <v-alert v-if="lab.mensaje" class="mb-4" type="warning" variant="tonal">{{ lab.mensaje }}</v-alert>

          <p class="mb-4 text-medium-emphasis">
            Abre <strong>GNS3</strong> en tu computadora, ve a <strong>Edit → Preferences → Server</strong>,
            desactiva <em>Enable local server</em> y escribe estos datos:
          </p>

          <v-row>
            <v-col v-for="campo in campos" :key="campo.label" cols="12" :md="campo.md">
              <v-text-field
                :hint="campo.hint"
                :label="campo.label"
                :model-value="campo.valor"
                :persistent-hint="!!campo.hint"
                readonly
                variant="outlined"
              >
                <!-- Al copiar, el botón muestra una palomita y "Copiado" durante 2 s -->
                <template #append-inner>
                  <v-btn
                    :color="copiado == campo.label ? 'success' : undefined"
                    density="comfortable"
                    :prepend-icon="copiado == campo.label ? 'mdi-check' : 'mdi-content-copy'"
                    size="small"
                    :text="copiado == campo.label ? 'Copiado' : 'Copiar'"
                    variant="text"
                    @click="copiar(campo.label, String(campo.valor))"
                  />
                </template>
              </v-text-field>
            </v-col>
            <v-col v-if="!lab.soy_dueno" cols="12" md="6">
              <v-text-field label="Dueño" :model-value="lab.dueno" readonly variant="outlined" />
            </v-col>
          </v-row>
        </template>

        <template #actions>
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

const copiado = ref('')   // campo que se acaba de copiar (su botón dice "Copiado")

let timerLab: ReturnType<typeof setTimeout> | undefined
let timerCopiado: ReturnType<typeof setTimeout> | undefined
let timerInv: ReturnType<typeof setInterval> | undefined

// Datos para conectar GNS3 de escritorio (el servidor es el mismo de esta página)
const campos = computed(() => {
  const lista = [
    { label: 'Servidor (Host)', valor: window.location.hostname, md: 6, hint: '' },
    { label: 'Puerto', valor: lab.value.puerto, md: 6, hint: `Consolas de tus equipos: ${lab.value.puerto + 1} en adelante` },
    { label: 'Usuario', valor: lab.value.usuario, md: 6, hint: '' },
    { label: 'Contraseña', valor: lab.value.token, md: 6, hint: '' },
  ]
  if (lab.value.codigo) {
    lista.push({ label: 'Código del equipo', valor: lab.value.codigo, md: 12, hint: 'Compártelo con tu equipo para que se unan' })
  }
  return lista
})

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

// navigator.clipboard solo existe en HTTPS o localhost; la página va por http://IP,
// así que si no está se usa el método viejo: un textarea oculto + execCommand('copy')
function copiar (campo: string, texto: string) {
  if (navigator.clipboard && window.isSecureContext) {
    navigator.clipboard.writeText(texto)
      .then(() => marcarCopiado(campo))
      .catch(() => copiarRespaldo(texto) && marcarCopiado(campo))
  } else if (copiarRespaldo(texto)) {
    marcarCopiado(campo)
  }
}

function copiarRespaldo (texto: string): boolean {
  const area = document.createElement('textarea')
  area.value = texto
  area.setAttribute('readonly', '')
  area.style.position = 'fixed'
  area.style.opacity = '0'
  document.body.append(area)
  area.select()
  let ok = false
  try {
    ok = document.execCommand('copy')
  } catch {
    ok = false
  }
  area.remove()
  return ok
}

// El botón del campo dice "Copiado" y a los 2 segundos vuelve a la normalidad
function marcarCopiado (campo: string) {
  copiado.value = campo
  clearTimeout(timerCopiado)
  timerCopiado = setTimeout(() => copiado.value = '', 2000)
}

onMounted(() => {
  load()
  loadInvitaciones()
  timerInv = setInterval(loadInvitaciones, 20000)   // revisa invitaciones cada 20 s
})

onBeforeUnmount(() => {
  clearTimeout(timerLab)
  clearTimeout(timerCopiado)
  clearInterval(timerInv)
})
</script>
