<template>
  <AppFrame>
    <!-- Avisos de la página (sin ventanas emergentes) -->
    <v-alert
      v-if="aviso.texto"
      class="mt-5"
      closable
      :text="aviso.texto"
      :type="aviso.tipo"
      variant="tonal"
      @click:close="aviso.texto = ''"
    />

    <!-- Confirmar borrado -->
    <v-alert v-if="porBorrar" class="mt-5" type="warning" variant="tonal">
      ¿Borrar a <strong>{{ porBorrar.matricula }} - {{ porBorrar.nombre }}</strong>? Esto no se puede deshacer.
      <template #append>
        <v-btn class="me-2" text="Cancelar" variant="text" @click="porBorrar = null" />
        <v-btn color="error" :loading="borrando" text="Sí, borrar" variant="flat" @click="remove" />
      </template>
    </v-alert>

    <!-- Formulario para agregar o editar (dentro de la página) -->
    <v-card v-if="editando" border class="my-5" elevation="0" rounded="lg" :title="editandoId ? `Editar ${original}` : 'Agregar usuario'">
      <template #text>
        <v-alert v-if="errorForm" class="mb-4" density="compact" :text="errorForm" type="error" variant="tonal" />

        <v-row dense>
          <v-col cols="12" md="4">
            <v-text-field v-model="form.matricula" hint="Es su usuario para entrar" label="Matrícula" variant="outlined" />
          </v-col>
          <v-col cols="12" md="8">
            <v-text-field v-model="form.nombre" label="Nombre completo" variant="outlined" />
          </v-col>

          <v-col cols="12" md="4">
            <v-select
              v-model="form.tipo"
              item-title="descripcion"
              item-value="clave"
              :items="tipos"
              label="Tipo"
              variant="outlined"
              @update:model-value="cambiarTipo"
            />
          </v-col>
          <template v-if="form.tipo == 'alumnos'">
            <v-col cols="12" md="3">
              <v-combobox
                v-model="form.grupo"
                hint="Si no existe, se crea"
                :items="grupos.map(g => g.codigo)"
                label="Grupo"
                variant="outlined"
                @update:model-value="sugerirCarrera"
              />
            </v-col>
            <v-col cols="12" md="5">
              <v-combobox
                v-model="form.carrera"
                hint="Es la carrera del grupo"
                :items="carreras"
                label="Carrera (opcional)"
                variant="outlined"
              />
            </v-col>
          </template>

          <v-col cols="12" md="6">
            <v-text-field
              v-model="form.password"
              :append-inner-icon="verPassword ? 'mdi-eye-off' : 'mdi-eye'"
              autocomplete="new-password"
              :hint="editandoId ? 'Déjala vacía para no cambiarla (mínimo 8 caracteres)' : 'Mínimo 8 caracteres'"
              :label="editandoId ? 'Restablecer contraseña' : 'Contraseña'"
              persistent-hint
              :type="verPassword ? 'text' : 'password'"
              variant="outlined"
              @click:append-inner="verPassword = !verPassword"
            />
          </v-col>
          <v-col cols="12" md="6">
            <div class="text-subtitle-2">Roles</div>
            <div class="d-flex flex-wrap ga-4">
              <v-checkbox
                v-for="rol in roles"
                :key="rol.value"
                v-model="form.roles"
                density="compact"
                hide-details
                :label="rol.text"
                :value="rol.value"
              />
            </div>
            <v-switch v-model="form.activo" color="primary" density="compact" hide-details inset label="Cuenta activa" />
          </v-col>
        </v-row>

        <div class="text-caption text-medium-emphasis mt-2">
          Si cambias la contraseña, el tipo, los roles o la matrícula, o desactivas la cuenta, se cierran sus sesiones.
        </div>
      </template>

      <template #actions>
        <v-spacer />
        <v-btn text="Cancelar" @click="cerrarForm" />
        <v-btn color="primary" :loading="guardando" :text="editandoId ? 'Guardar cambios' : 'Agregar'" variant="flat" @click="save" />
      </template>
    </v-card>

    <v-card border class="my-5" elevation="0" rounded="lg" title="Usuarios">
      <template #append>
        <v-btn class="me-2" color="primary" prepend-icon="mdi-account-plus" text="Agregar usuario" variant="flat" @click="openCreate" />
        <v-btn color="primary" prepend-icon="mdi-file-upload-outline" text="Importar" to="/importar" variant="tonal" />
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
              item-title="descripcion"
              item-value="clave"
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
              :items="grupos.map(g => g.codigo)"
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
              <v-icon color="error" icon="mdi-delete" size="small" @click="porBorrar = item" />
            </div>
          </template>
        </v-data-table-server>
      </template>
    </v-card>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { onMounted, reactive, ref, watch } from 'vue'
import type { DataTableHeader } from 'vuetify'
import axios from 'axios'
import { type TipoUsuario, Usuario, UsuarioForm } from '@/models/Usuario'
import { Grupo } from '@/models/Grupo'

const headers: DataTableHeader[] = [
  { title: 'Matrícula', key: 'matricula', align: 'start', sortable: false },
  { title: 'Nombre', key: 'nombre', align: 'start', sortable: false },
  { title: 'Grupo', key: 'grupo', align: 'center', sortable: false },
  { title: 'Roles', key: 'roles', align: 'start', sortable: false },
  { title: 'Activo', key: 'activo', align: 'center', sortable: false },
  { title: 'Acciones', key: 'actions', align: 'end', sortable: false },
]

const roles = [
  { text: 'Administrador', value: 'ADMIN' },
  { text: 'Maestro', value: 'PROFESOR' },
  { text: 'Alumno', value: 'ALUMNO' },
]

const colores: Record<string, string> = { ADMIN: 'red', PROFESOR: 'orange', ALUMNO: 'blue' }

const usuarios = ref<Usuario[]>([])
const tipos = ref<TipoUsuario[]>([])
const grupos = ref<Grupo[]>([])
const carreras = ref<string[]>([])
const total = ref(0)
const cargando = ref(false)
const pagina = ref(1)
const porPagina = ref(10)
const filtros = reactive({ buscar: '', tipo: null as string | null, grupo: null as string | null })

// Formulario
const editando = ref(false)
const editandoId = ref('')         // '' = agregar
const original = ref('')           // matrícula antes de editar (para el título)
const form = ref<UsuarioForm>(new UsuarioForm())
const verPassword = ref(false)
const guardando = ref(false)
const errorForm = ref('')

// Avisos y borrado
const aviso = reactive({ tipo: 'success' as 'success' | 'error', texto: '' })
const porBorrar = ref<Usuario | null>(null)
const borrando = ref(false)
let timerAviso: ReturnType<typeof setTimeout> | undefined

function avisar (tipo: 'success' | 'error', texto: string) {
  aviso.tipo = tipo
  aviso.texto = texto
  clearTimeout(timerAviso)
  if (tipo == 'success') {
    timerAviso = setTimeout(() => aviso.texto = '', 4000)
  }
}

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

function cargarCatalogos () {
  axios.get('/api/usuarios/tipos').then(response => tipos.value = response.data)
  axios.get('/api/grupos').then(response => grupos.value = response.data)
  axios.get('/api/carreras').then(response => {
    carreras.value = response.data.map((c: { nombre: string }) => c.nombre)
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

// Al elegir el tipo se ponen los roles que da ese tipo (se pueden ajustar)
function cambiarTipo (clave: string) {
  form.value.roles = [...(tipos.value.find(t => t.clave == clave)?.roles || [])]
}

// Si el grupo ya existe, se propone su carrera
function sugerirCarrera (codigo: string) {
  const grupo = grupos.value.find(g => g.codigo.toUpperCase() == (codigo || '').toUpperCase())
  if (grupo?.carrera) {
    form.value.carrera = grupo.carrera
  }
}

function openCreate () {
  form.value = new UsuarioForm()
  cambiarTipo(form.value.tipo)
  editandoId.value = ''
  abrirForm()
}

function openEdit (item: Usuario) {
  form.value = new UsuarioForm(item.matricula, item.nombre, '', item.tipo, [...item.roles], item.grupo, item.carrera, item.activo)
  editandoId.value = item.id
  original.value = item.matricula
  abrirForm()
}

function abrirForm () {
  errorForm.value = ''
  verPassword.value = false
  porBorrar.value = null
  editando.value = true
  window.scrollTo({ top: 0, behavior: 'smooth' })
}

function cerrarForm () {
  editando.value = false
  errorForm.value = ''
}

function save () {
  errorForm.value = ''
  if (form.value.roles.length == 0) {
    errorForm.value = 'El usuario debe tener al menos un rol'
    return
  }

  guardando.value = true
  const peticion = editandoId.value
    ? axios.put(`/api/usuarios/${editandoId.value}`, form.value)
    : axios.post('/api/usuarios', form.value)

  peticion
    .then(() => {
      avisar('success', editandoId.value ? `Se guardaron los cambios de ${form.value.matricula}.` : `Se agregó a ${form.value.matricula}.`)
      cerrarForm()
      cargar()
      cargarCatalogos() // pudo crearse un grupo o una carrera
    })
    .catch(err => {
      errorForm.value = err.response?.data || 'No se pudo guardar.'
    })
    .finally(() => guardando.value = false)
}

function remove () {
  if (!porBorrar.value) return
  const item = porBorrar.value
  borrando.value = true
  axios.delete(`/api/usuarios/${item.id}`)
    .then(() => {
      avisar('success', `Se borró a ${item.matricula}.`)
      cargar()
    })
    .catch(err => {
      avisar('error', err.response?.data || 'No se pudo borrar.')
    })
    .finally(() => {
      borrando.value = false
      porBorrar.value = null
    })
}

onMounted(cargarCatalogos)
</script>
