<template>
  <div class="d-flex flex-wrap align-center justify-space-between ga-4 mb-6">
    <div>
      <div class="page-title">Workspaces activos</div>
      <div class="page-subtitle">Contenedores GNS3 corriendo en el servidor. Se actualiza cada 30 s.</div>
    </div>
    <v-btn prepend-icon="mdi-refresh" variant="tonal" :loading="cargando" @click="cargar">Actualizar</v-btn>
  </div>

  <v-row class="mb-2">
    <v-col cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">Activos</div><div class="text-h5 font-weight-bold text-success">{{ items.length }}</div></v-card>
    </v-col>
    <v-col cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">De equipo</div><div class="text-h5 font-weight-bold text-secondary">{{ colaborativos }}</div></v-card>
    </v-col>
    <v-col cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">Usuarios conectados</div><div class="text-h5 font-weight-bold text-primary">{{ usuarios }}</div></v-card>
    </v-col>
  </v-row>

  <v-card>
    <v-card-text>
      <v-data-table
        :headers="headers"
        :items="items"
        :loading="cargando"
        item-value="id_workspace"
        no-data-text="No hay workspaces activos"
      >
        <template #item.id_workspace="{ item }"><span class="font-mono">{{ item.id_workspace }}</span></template>
        <template #item.creado_por="{ item }"><span class="font-mono">{{ item.creado_por }}</span></template>
        <template #item.miembros="{ item }">
          <span v-if="!item.miembros?.length" class="text-medium-emphasis">—</span>
          <v-chip v-for="m in item.miembros" :key="m" size="x-small" class="mr-1 font-mono">{{ m }}</v-chip>
        </template>
        <template #item.codigo_lab="{ item }">
          <v-chip v-if="item.codigo_lab" size="x-small" color="secondary" label class="font-mono">{{ item.codigo_lab }}</v-chip>
          <span v-else class="text-medium-emphasis text-caption">individual</span>
        </template>
        <template #item.puerto_base="{ item }"><span class="font-mono">{{ item.puerto_base }} → {{ item.puerto_base + 10000 }}</span></template>
        <template #item.fecha_inicio="{ item }">{{ tiempo(item.fecha_inicio) }}</template>
        <template #item.acciones="{ item }">
          <v-btn icon="mdi-stop-circle" variant="text" size="small" color="error" title="Cerrar laboratorio" @click="confirmar = item" />
        </template>
      </v-data-table>
    </v-card-text>
  </v-card>

  <v-dialog :model-value="!!confirmar" max-width="440" @update:model-value="confirmar = null">
    <v-card title="¿Cerrar este laboratorio?" prepend-icon="mdi-alert">
      <v-card-text>
        Se detendrá <span class="font-mono">{{ confirmar?.id_workspace }}</span> de
        <b>{{ confirmar?.creado_por }}</b>
        <template v-if="confirmar?.miembros?.length"> y de {{ confirmar.miembros.length }} compañero(s)</template>.
        Los proyectos guardados no se borran.
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="confirmar = null">Cancelar</v-btn>
        <v-btn color="error" :loading="cerrando" @click="cerrar">Cerrar laboratorio</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount } from 'vue'
import { monitorApi } from '@/api/monitor'
import { mensajeError } from '@/api/http'
import { useNotificaciones } from '@/stores/notificaciones'

const noti = useNotificaciones()

const headers = [
  { title: 'Workspace', key: 'id_workspace' },
  { title: 'Dueño', key: 'creado_por' },
  { title: 'Invitados', key: 'miembros', sortable: false },
  { title: 'Equipo', key: 'codigo_lab' },
  { title: 'Puerto GNS3 → público', key: 'puerto_base' },
  { title: 'Activo desde', key: 'fecha_inicio' },
  { title: '', key: 'acciones', sortable: false, align: 'end' }
]

const items = ref([])
const cargando = ref(false)
const cerrando = ref(false)
const confirmar = ref(null)
let timer = null

const colaborativos = computed(() => items.value.filter((w) => w.codigo_lab).length)
const usuarios = computed(() => items.value.reduce((a, w) => a + 1 + (w.miembros?.length || 0), 0))

const tiempo = (iso) => {
  if (!iso) return '—'
  const min = Math.floor((Date.now() - new Date(iso).getTime()) / 60000)
  if (min < 1) return 'hace un momento'
  if (min < 60) return `hace ${min} min`
  return `hace ${Math.floor(min / 60)} h ${min % 60} min`
}

const cargar = async () => {
  cargando.value = true
  try {
    items.value = await monitorApi.listar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron consultar los workspaces'))
  } finally {
    cargando.value = false
  }
}

const cerrar = async () => {
  cerrando.value = true
  try {
    await monitorApi.cerrar(confirmar.value)
    noti.exito('Laboratorio cerrado')
    confirmar.value = null
    cargar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo cerrar el laboratorio'))
  } finally {
    cerrando.value = false
  }
}

onMounted(() => {
  cargar()
  timer = setInterval(cargar, 30000)
})
onBeforeUnmount(() => clearInterval(timer))
</script>