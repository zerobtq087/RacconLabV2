<template>
  <AppFrame>
    <v-row>
      <v-col v-for="(item, i) in statItems" :key="i" cols="6" md="3">
        <v-list border elevation="0" lines="two" rounded="lg">
          <v-list-item>
            <v-list-item-title class="text-body-small">{{ item.subtitle }}</v-list-item-title>
            <v-list-item-title>{{ item.title }}</v-list-item-title>
            <template #append>
              <v-icon :color="item.color" :icon="item.icon" size="30" />
            </template>
          </v-list-item>
        </v-list>
      </v-col>
    </v-row>

    <v-card border class="my-5" elevation="0" rounded="lg" title="Laboratorios activos">
      <template #append>
        <v-btn :loading="cargando" prepend-icon="mdi-refresh" text="Actualizar" variant="text" @click="load" />
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

        <v-data-table class="mt-4" :headers="headers" :items="labs" :loading="cargando" :search="search">
          <template #item.nombre="{ item }">
            <div>{{ item.nombre }}</div>
            <div class="text-body-small text-medium-emphasis">{{ item.matricula }}</div>
          </template>

          <template #item.codigo="{ item }">
            <v-chip
              :color="item.codigo ? 'primary' : 'grey'"
              label
              size="x-small"
              :text="item.codigo ? `Equipo (${item.miembros + 1})` : 'Individual'"
            />
          </template>

          <template #item.inicio="{ item }">
            {{ new Date(item.inicio).toLocaleString() }}
          </template>

          <template #item.actions="{ item }">
            <div class="d-flex justify-end">
              <v-icon color="error" icon="mdi-stop-circle-outline" size="small" @click="cerrar(item)" />
            </div>
          </template>
        </v-data-table>
      </template>
    </v-card>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, onBeforeUnmount, onMounted, ref, shallowRef } from 'vue'
import type { DataTableHeader } from 'vuetify'
import axios from 'axios'
import Swal from 'sweetalert2'
import { LabActivo } from '@/models/LabActivo'

const headers: DataTableHeader[] = [
  { title: 'Alumno', key: 'nombre', align: 'start' },
  { title: 'Grupo', key: 'grupo', align: 'center' },
  { title: 'Tipo', key: 'codigo', align: 'center' },
  { title: 'Puerto', key: 'puerto', align: 'center' },
  { title: 'Inicio', key: 'inicio', align: 'start' },
  { title: 'Cerrar', key: 'actions', align: 'end', sortable: false },
]

const search = shallowRef('')
const labs = ref<LabActivo[]>([])
const cargando = ref(false)
let timer: ReturnType<typeof setInterval> | undefined

const statItems = computed(() => [
  { subtitle: 'Laboratorios activos', title: labs.value.length, icon: 'mdi-lan', color: 'blue' },
  { subtitle: 'Individuales', title: labs.value.filter(l => !l.codigo).length, icon: 'mdi-account', color: 'green' },
  { subtitle: 'En equipo', title: labs.value.filter(l => l.codigo).length, icon: 'mdi-account-group', color: 'orange' },
  { subtitle: 'Alumnos conectados', title: labs.value.reduce((t, l) => t + 1 + l.miembros, 0), icon: 'mdi-school', color: 'red' },
])

function load () {
  cargando.value = true
  axios.get('/api/monitor')
    .then(response => labs.value = response.data)
    .finally(() => cargando.value = false)
}

function cerrar (item: LabActivo) {
  Swal.fire({
    icon: 'warning',
    title: '¿Cerrar laboratorio?',
    text: `Se borrará el laboratorio de "${item.nombre}" y todo su trabajo.`,
    showCancelButton: true,
    confirmButtonText: 'Sí, cerrar',
    cancelButtonText: 'Cancelar',
    confirmButtonColor: '#d33',
  }).then(result => {
    if (!result.isConfirmed) return
    axios.delete(`/api/monitor/${item.id}`).then(() => {
      load()
      Swal.fire({ icon: 'success', title: 'Cerrado', timer: 1500, showConfirmButton: false })
    }).catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo cerrar.' })
    })
  })
}

onMounted(() => {
  load()
  timer = setInterval(load, 15000)   // se actualiza solo cada 15 s
})

onBeforeUnmount(() => clearInterval(timer))
</script>
