<template>
  <AppFrame>
    <v-card border class="my-5" elevation="0" rounded="lg" :title="esAdmin ? 'Grupos' : 'Mis grupos'">
      <template #append>
        <v-btn v-if="esAdmin" color="primary" prepend-icon="mdi-plus" text="Nuevo" @click="openCreate" />
      </template>

      <template #text>
        <v-text-field
          v-model="search"
          autocomplete="off"
          hide-details
          placeholder="Buscar"
          prepend-inner-icon="mdi-magnify"
          rounded
          variant="outlined"
        />

        <v-data-table class="mt-4" :headers="headers" :items="grupos" :loading="cargando" :search="search">
          <template #item.docentes="{ item }">
            <v-chip
              v-for="d in item.docentes"
              :key="d.id"
              class="me-1"
              label
              size="x-small"
              :text="d.nombre"
            />
            <span v-if="item.docentes.length == 0" class="text-medium-emphasis">Sin maestro</span>
          </template>

          <template #item.actions="{ item }">
            <div class="d-flex justify-end ga-1">
              <v-icon icon="mdi-account-school-outline" size="small" @click="openAlumnos(item)" />
              <template v-if="esAdmin">
                <v-icon icon="mdi-account-tie" size="small" @click="openDocentes(item)" />
                <v-icon icon="mdi-pencil" size="small" @click="openEdit(item)" />
                <v-icon color="error" icon="mdi-delete" size="small" @click="remove(item)" />
              </template>
            </div>
          </template>
        </v-data-table>
      </template>
    </v-card>

    <!-- Dialogo Crear/Editar -->
    <v-dialog v-model="dialog" max-width="500">
      <v-card rounded="lg" :title="form.id ? 'Editar grupo' : 'Nuevo grupo'">
        <template #text>
          <v-text-field v-model="form.codigo" class="mb-2" label="Código (ej. IRD-71)" variant="outlined" />
          <v-combobox v-model="form.carrera" :items="carreras" label="Carrera" variant="outlined" />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cancelar" @click="dialog = false" />
          <v-btn color="primary" text="Guardar" @click="save" />
        </template>
      </v-card>
    </v-dialog>

    <!-- Dialogo Maestros -->
    <v-dialog v-model="dialogDocentes" max-width="500">
      <v-card rounded="lg" :title="`Maestros de ${form.codigo}`">
        <template #text>
          <v-autocomplete
            v-model="docentesSel"
            chips
            closable-chips
            item-title="nombre"
            item-value="id"
            :items="maestros"
            label="Maestros"
            multiple
            variant="outlined"
          />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cancelar" @click="dialogDocentes = false" />
          <v-btn color="primary" text="Guardar" @click="saveDocentes" />
        </template>
      </v-card>
    </v-dialog>

    <!-- Dialogo Alumnos -->
    <v-dialog v-model="dialogAlumnos" max-width="600">
      <v-card rounded="lg" :title="`Alumnos de ${form.codigo}`">
        <template #text>
          <v-data-table density="compact" :headers="headersAlumnos" :items="alumnos" :loading="cargandoAlumnos" />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cerrar" @click="dialogAlumnos = false" />
        </template>
      </v-card>
    </v-dialog>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onMounted, ref, shallowRef } from 'vue'
import type { DataTableHeader } from 'vuetify'
import axios from 'axios'
import Swal from 'sweetalert2'
import { Grupo, type Persona } from '@/models/Grupo'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
const esAdmin = computed(() => authStore.userAcc.rol == 'ADMIN')

const headers: DataTableHeader[] = [
  { title: 'Grupo', key: 'codigo', align: 'start' },
  { title: 'Carrera', key: 'carrera', align: 'start' },
  { title: 'Alumnos', key: 'alumnos', align: 'center' },
  { title: 'Maestros', key: 'docentes', align: 'start', sortable: false },
  { title: 'Acciones', key: 'actions', align: 'end', sortable: false },
]

const headersAlumnos: DataTableHeader[] = [
  { title: 'Matrícula', key: 'matricula', align: 'start' },
  { title: 'Nombre', key: 'nombre', align: 'start' },
]

const search = shallowRef('')
const grupos = ref<Grupo[]>([])
const carreras = ref<string[]>([])
const maestros = ref<Persona[]>([])
const alumnos = ref<Persona[]>([])
const cargando = ref(false)
const cargandoAlumnos = ref(false)

const dialog = ref(false)
const dialogDocentes = ref(false)
const dialogAlumnos = ref(false)
const form = ref<Grupo>(new Grupo())
const docentesSel = ref<string[]>([])

// El backend regresa todos los grupos al admin y solo los suyos al maestro
function load () {
  cargando.value = true
  axios.get('/api/grupos')
    .then(response => grupos.value = response.data)
    .finally(() => cargando.value = false)
}

function loadCatalogos () {
  if (!esAdmin.value) return
  axios.get('/api/carreras').then(response => {
    carreras.value = response.data.map((c: { nombre: string }) => c.nombre)
  })
  axios.get('/api/maestros').then(response => maestros.value = response.data)
}

function openCreate () {
  form.value = new Grupo()
  dialog.value = true
}

function openEdit (item: Grupo) {
  form.value = { ...item }
  dialog.value = true
}

function openDocentes (item: Grupo) {
  form.value = { ...item }
  docentesSel.value = item.docentes.map(d => d.id)
  dialogDocentes.value = true
}

function openAlumnos (item: Grupo) {
  form.value = { ...item }
  alumnos.value = []
  dialogAlumnos.value = true
  cargandoAlumnos.value = true
  axios.get(`/api/grupos/${item.id}/alumnos`)
    .then(response => alumnos.value = response.data)
    .finally(() => cargandoAlumnos.value = false)
}

function save () {
  const editing = !!form.value.id
  const req = editing
    ? axios.put(`/api/grupos/${form.value.id}`, form.value)
    : axios.post('/api/grupos', form.value)

  req.then(() => {
    dialog.value = false
    load()
    loadCatalogos()
    Swal.fire({ icon: 'success', title: editing ? 'Actualizado' : 'Creado', timer: 1500, showConfirmButton: false })
  }).catch(err => {
    Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo guardar.' })
  })
}

function saveDocentes () {
  axios.put(`/api/grupos/${form.value.id}/docentes`, { docentes: docentesSel.value })
    .then(() => {
      dialogDocentes.value = false
      load()
      Swal.fire({ icon: 'success', title: 'Maestros asignados', timer: 1500, showConfirmButton: false })
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo guardar.' })
    })
}

function remove (item: Grupo) {
  Swal.fire({
    icon: 'warning',
    title: '¿Borrar grupo?',
    text: `Se eliminará "${item.codigo}". Solo se puede si no tiene alumnos.`,
    showCancelButton: true,
    confirmButtonText: 'Sí, borrar',
    cancelButtonText: 'Cancelar',
    confirmButtonColor: '#d33',
  }).then(result => {
    if (!result.isConfirmed) return
    axios.delete(`/api/grupos/${item.id}`).then(() => {
      load()
      Swal.fire({ icon: 'success', title: 'Borrado', timer: 1500, showConfirmButton: false })
    }).catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo borrar.' })
    })
  })
}

onMounted(() => {
  load()
  loadCatalogos()
})
</script>
