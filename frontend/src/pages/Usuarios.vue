<template>
  <AppFrame>
    <v-card border class="my-5" elevation="0" rounded="lg" title="Usuarios">
      <template #append>
        <v-btn color="primary" prepend-icon="mdi-file-upload-outline" text="Importar" to="/importar" />
      </template>

      <template #text>
        <v-row>
          <v-col cols="12" md="6">
            <v-text-field
              v-model="filtros.buscar"
              autocomplete="off"
              clearable
              hide-details
              placeholder="Buscar por matrícula o nombre"
              prepend-inner-icon="mdi-magnify"
              rounded
              variant="outlined"
            />
          </v-col>
          <v-col cols="6" md="3">
            <v-select
              v-model="filtros.tipo"
              clearable
              hide-details
              item-title="text"
              item-value="value"
              :items="tipos"
              label="Tipo"
              variant="outlined"
            />
          </v-col>
          <v-col cols="6" md="3">
            <v-select
              v-model="filtros.grupo"
              clearable
              hide-details
              :items="grupos"
              label="Grupo"
              variant="outlined"
            />
          </v-col>
        </v-row>

        <v-data-table-server
          v-model:items-per-page="porPagina"
          class="mt-4"
          :headers="headers"
          :items="usuarios"
          :items-length="total"
          :loading="cargando"
          @update:options="cargar"
        >
          <template #item.nombre="{ item }">
            <div class="d-flex align-center ga-2">
              <span>{{ item.nombre }}</span>
              <v-chip v-if="item.protegido" color="primary" label size="x-small" text="General" />
            </div>
          </template>

          <template #item.roles="{ item }">
            <v-chip
              v-for="rol in item.roles"
              :key="rol"
              class="me-1"
              :color="colores[rol]"
              label
              size="x-small"
              :text="rol"
            />
          </template>

          <template #item.activo="{ item }">
            <v-icon :color="item.activo ? 'success' : 'grey'" :icon="item.activo ? 'mdi-check-circle' : 'mdi-cancel'" />
          </template>

          <template #item.actions="{ item }">
            <div v-if="!item.protegido" class="d-flex justify-end ga-1">
              <v-icon icon="mdi-pencil" size="small" @click="openEdit(item)" />
              <v-icon icon="mdi-lock-reset" size="small" @click="openPassword(item)" />
              <v-icon color="error" icon="mdi-delete" size="small" @click="remove(item)" />
            </div>
          </template>
        </v-data-table-server>
      </template>
    </v-card>

    <!-- Dialogo Editar -->
    <v-dialog v-model="dialog" max-width="500">
      <v-card rounded="lg" :title="`Editar ${form.matricula}`">
        <template #text>
          <v-text-field v-model="form.matricula" class="mb-2" label="Matrícula" variant="outlined" />
          <v-text-field v-model="form.nombre" class="mb-2" label="Nombre" variant="outlined" />
          <v-select
            v-if="form.tipo == 'alumnos'"
            v-model="form.grupo"
            class="mb-2"
            clearable
            :items="grupos"
            label="Grupo"
            variant="outlined"
          />
          <div class="text-subtitle-2 mb-1">Roles</div>
          <v-checkbox
            v-for="rol in roles"
            :key="rol.value"
            v-model="form.roles"
            density="compact"
            hide-details
            :label="rol.text"
            :value="rol.value"
          />
          <v-switch v-model="form.activo" class="mt-2" color="primary" hide-details inset label="Cuenta activa" />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cancelar" @click="dialog = false" />
          <v-btn color="primary" text="Guardar" @click="save" />
        </template>
      </v-card>
    </v-dialog>

    <!-- Dialogo Contraseña -->
    <v-dialog v-model="dialogPassword" max-width="420">
      <v-card rounded="lg" :title="`Nueva contraseña para ${form.matricula}`">
        <template #text>
          <v-text-field
            v-model="nuevaPassword"
            :append-inner-icon="verPassword ? 'mdi-eye-off' : 'mdi-eye'"
            hint="Mínimo 8 caracteres"
            label="Contraseña"
            persistent-hint
            :type="verPassword ? 'text' : 'password'"
            variant="outlined"
            @click:append-inner="verPassword = !verPassword"
          />
        </template>
        <template #actions>
          <v-spacer />
          <v-btn text="Cancelar" @click="dialogPassword = false" />
          <v-btn color="primary" :disabled="nuevaPassword.length < 8" text="Cambiar" @click="savePassword" />
        </template>
      </v-card>
    </v-dialog>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { onMounted, reactive, ref, watch } from 'vue'
import type { DataTableHeader } from 'vuetify'
import axios from 'axios'
import Swal from 'sweetalert2'
import { Usuario } from '@/models/Usuario'

const headers: DataTableHeader[] = [
  { title: 'Matrícula', key: 'matricula', align: 'start', sortable: false },
  { title: 'Nombre', key: 'nombre', align: 'start', sortable: false },
  { title: 'Grupo', key: 'grupo', align: 'center', sortable: false },
  { title: 'Roles', key: 'roles', align: 'start', sortable: false },
  { title: 'Activo', key: 'activo', align: 'center', sortable: false },
  { title: 'Acciones', key: 'actions', align: 'end', sortable: false },
]

const tipos = [
  { text: 'Alumnos', value: 'alumnos' },
  { text: 'Maestros', value: 'maestros' },
  { text: 'Administradores', value: 'admins' },
]

const roles = [
  { text: 'Administrador', value: 'ADMIN' },
  { text: 'Maestro', value: 'PROFESOR' },
  { text: 'Alumno', value: 'ALUMNO' },
]

const colores: Record<string, string> = { ADMIN: 'red', PROFESOR: 'orange', ALUMNO: 'blue' }

const usuarios = ref<Usuario[]>([])
const grupos = ref<string[]>([])
const total = ref(0)
const cargando = ref(false)
const pagina = ref(1)
const porPagina = ref(10)
const filtros = reactive({ buscar: '', tipo: null as string | null, grupo: null as string | null })

const dialog = ref(false)
const dialogPassword = ref(false)
const form = ref<Usuario>(new Usuario())
const nuevaPassword = ref('')
const verPassword = ref(false)

// La tabla pide al backend solo la página que se ve (sirve para miles de alumnos)
function cargar (opciones?: { page: number, itemsPerPage: number }) {
  if (opciones) {
    pagina.value = opciones.page
    porPagina.value = opciones.itemsPerPage
  }
  cargando.value = true
  axios.get('/api/usuarios', {
    params: {
      pagina: pagina.value,
      por_pagina: porPagina.value,
      buscar: filtros.buscar || undefined,
      tipo: filtros.tipo || undefined,
      grupo: filtros.grupo || undefined,
    },
  })
    .then(response => {
      usuarios.value = response.data.usuarios
      total.value = response.data.total
    })
    .finally(() => cargando.value = false)
}

function cargarGrupos () {
  axios.get('/api/grupos').then(response => {
    grupos.value = response.data.map((g: { codigo: string }) => g.codigo)
  })
}

// Al cambiar un filtro, vuelve a la primera página (espera a que termine de escribir)
let espera: ReturnType<typeof setTimeout> | undefined
watch(filtros, () => {
  clearTimeout(espera)
  espera = setTimeout(() => {
    pagina.value = 1
    cargar()
  }, 400)
})

function openEdit (item: Usuario) {
  form.value = { ...item, roles: [...item.roles] }
  dialog.value = true
}

function openPassword (item: Usuario) {
  form.value = { ...item }
  nuevaPassword.value = ''
  verPassword.value = false
  dialogPassword.value = true
}

function save () {
  if (form.value.roles.length == 0) {
    Swal.fire({ icon: 'warning', title: 'El usuario debe tener al menos un rol' })
    return
  }
  axios.put(`/api/usuarios/${form.value.id}`, form.value)
    .then(() => {
      dialog.value = false
      cargar()
      Swal.fire({ icon: 'success', title: 'Actualizado', timer: 1500, showConfirmButton: false })
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo guardar.' })
    })
}

function savePassword () {
  axios.put(`/api/usuarios/${form.value.id}/password`, { password: nuevaPassword.value })
    .then(() => {
      dialogPassword.value = false
      Swal.fire({ icon: 'success', title: 'Contraseña cambiada', timer: 1500, showConfirmButton: false })
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo cambiar.' })
    })
}

function remove (item: Usuario) {
  Swal.fire({
    icon: 'warning',
    title: '¿Borrar usuario?',
    text: `Se eliminará "${item.matricula} - ${item.nombre}".`,
    showCancelButton: true,
    confirmButtonText: 'Sí, borrar',
    cancelButtonText: 'Cancelar',
    confirmButtonColor: '#d33',
  }).then(result => {
    if (!result.isConfirmed) return
    axios.delete(`/api/usuarios/${item.id}`).then(() => {
      cargar()
      Swal.fire({ icon: 'success', title: 'Borrado', timer: 1500, showConfirmButton: false })
    }).catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo borrar.' })
    })
  })
}

onMounted(cargarGrupos)
</script>
