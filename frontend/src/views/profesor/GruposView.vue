<template>
  <div class="d-flex flex-wrap align-center justify-space-between ga-4 mb-6">
    <div>
      <div class="page-title">{{ auth.esAdmin ? 'Grupos' : 'Mis grupos' }}</div>
      <div class="page-subtitle">
        {{ auth.esAdmin ? 'Asigna uno o varios maestros a cada grupo.' : 'Grupos donde estás asignado como maestro.' }}
      </div>
    </div>
    <v-btn v-if="auth.esAdmin" color="primary" prepend-icon="mdi-plus" @click="dlgNuevo.show = true">Nuevo grupo</v-btn>
  </div>

  <v-row class="mb-2">
    <v-col cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">Grupos</div><div class="text-h5 font-weight-bold text-primary">{{ grupos.length }}</div></v-card>
    </v-col>
    <v-col cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">Alumnos</div><div class="text-h5 font-weight-bold text-success">{{ totalAlumnos }}</div></v-card>
    </v-col>
    <v-col v-if="auth.esAdmin" cols="6" md="3">
      <v-card class="pa-4"><div class="text-caption">Sin maestro</div><div class="text-h5 font-weight-bold text-warning">{{ sinMaestro }}</div></v-card>
    </v-col>
  </v-row>

  <v-card>
    <v-card-text>
      <v-text-field v-model="busqueda" prepend-inner-icon="mdi-magnify" label="Buscar grupo o maestro" hide-details clearable class="mb-4" />

      <v-data-table
        :headers="headers"
        :items="filtrados"
        :loading="cargando"
        item-value="codigo"
        :no-data-text="auth.esAdmin ? 'No hay grupos. Se crean al importar alumnos o con “Nuevo grupo”.' : 'Aún no te asignan grupos.'"
      >
        <template #item.codigo="{ item }"><span class="font-mono font-weight-bold">{{ item.codigo }}</span></template>

        <template #item.docentes="{ item }">
          <span v-if="!item.docentes.length" class="text-warning text-caption">
            <v-icon icon="mdi-alert" size="small" /> Sin maestro
          </span>
          <v-chip
            v-for="d in item.docentes"
            :key="d.matricula"
            size="small"
            :color="d.activo ? 'secondary' : 'grey'"
            class="mr-1 mb-1"
            :title="d.activo ? d.matricula : 'Sin rol de Maestro o inactivo'"
          >
            {{ d.nombre }}
          </v-chip>
        </template>

        <template #item.total_alumnos="{ item }"><v-chip size="small" label>{{ item.total_alumnos }}</v-chip></template>

        <template #item.acciones="{ item }">
          <v-btn icon="mdi-account-multiple" variant="text" size="small" title="Ver alumnos" @click="verAlumnos(item)" />
          <template v-if="auth.esAdmin">
            <v-btn icon="mdi-account-tie" variant="text" size="small" title="Asignar maestros" @click="abrirAsignar(item)" />
            <v-btn icon="mdi-delete" variant="text" size="small" color="error" title="Eliminar" @click="eliminar(item)" />
          </template>
        </template>
      </v-data-table>
    </v-card-text>
  </v-card>

  <!-- Nuevo grupo -->
  <v-dialog v-model="dlgNuevo.show" max-width="400">
    <v-card title="Nuevo grupo" prepend-icon="mdi-account-group">
      <v-card-text>
        <v-text-field v-model="dlgNuevo.codigo" label="Código (ej. IRD-71)" class="font-mono" autofocus @keyup.enter="crear" />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgNuevo.show = false">Cancelar</v-btn>
        <v-btn color="primary" :loading="guardando" :disabled="!dlgNuevo.codigo" @click="crear">Crear</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Asignar maestros: lista con casillas (sin menú flotante) -->
  <v-dialog v-model="dlgAsignar.show" max-width="520" scrollable>
    <v-card>
      <v-card-item>
        <v-card-title>Maestros de <span class="font-mono">{{ dlgAsignar.grupo?.codigo }}</span></v-card-title>
        <v-card-subtitle>{{ dlgAsignar.seleccion.length }} seleccionado(s) · solo usuarios activos con rol Maestro</v-card-subtitle>
      </v-card-item>

      <v-card-text class="pt-0">
        <v-text-field
          v-model="dlgAsignar.busqueda"
          prepend-inner-icon="mdi-magnify"
          label="Buscar maestro"
          hide-details
          clearable
          class="mb-2"
        />

        <v-list density="compact" class="lista-maestros bg-transparent">
          <v-list-item v-if="!maestrosFiltrados.length" title="No hay usuarios con rol Maestro" class="text-medium-emphasis" />
          <v-list-item
            v-for="m in maestrosFiltrados"
            :key="m.matricula"
            :title="m.nombre"
            :subtitle="m.matricula"
            @click="alternar(m.matricula)"
          >
            <template #prepend>
              <v-checkbox-btn
                :model-value="dlgAsignar.seleccion.includes(m.matricula)"
                color="primary"
                @update:model-value="alternar(m.matricula)"
                @click.stop
              />
            </template>
          </v-list-item>
        </v-list>
      </v-card-text>

      <v-card-actions>
        <v-btn variant="text" :disabled="!dlgAsignar.seleccion.length" @click="dlgAsignar.seleccion = []">Quitar todos</v-btn>
        <v-spacer />
        <v-btn variant="text" @click="dlgAsignar.show = false">Cancelar</v-btn>
        <v-btn color="primary" :loading="guardando" @click="guardarAsignacion">Guardar</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Alumnos -->
  <v-dialog v-model="dlgAlumnos.show" max-width="640" scrollable>
    <v-card>
      <v-card-item>
        <v-card-title>Alumnos de <span class="font-mono">{{ dlgAlumnos.grupo?.codigo }}</span></v-card-title>
        <v-card-subtitle>{{ dlgAlumnos.lista.length }} alumno(s)</v-card-subtitle>
      </v-card-item>
      <v-card-text>
        <v-text-field v-model="dlgAlumnos.busqueda" prepend-inner-icon="mdi-magnify" label="Buscar" hide-details class="mb-3" />
        <v-data-table
          :headers="headersAlumnos"
          :items="dlgAlumnos.lista"
          :search="dlgAlumnos.busqueda"
          :loading="dlgAlumnos.cargando"
          density="compact"
          items-per-page="10"
          no-data-text="Sin alumnos en este grupo"
        >
          <template #item.matricula="{ item }"><span class="font-mono">{{ item.matricula }}</span></template>
          <template #item.activo="{ item }">
            <v-icon :icon="item.activo ? 'mdi-check-circle' : 'mdi-cancel'" :color="item.activo ? 'success' : 'grey'" size="small" />
          </template>
        </v-data-table>
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgAlumnos.show = false">Cerrar</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { gruposApi } from '@/api/grupos'
import { mensajeError } from '@/api/http'
import { useAuth } from '@/stores/auth'
import { useNotificaciones } from '@/stores/notificaciones'

const auth = useAuth()
const noti = useNotificaciones()

const headers = [
  { title: 'Grupo', key: 'codigo' },
  { title: 'Maestros', key: 'docentes', sortable: false },
  { title: 'Alumnos', key: 'total_alumnos', align: 'center' },
  { title: '', key: 'acciones', sortable: false, align: 'end' }
]
const headersAlumnos = [
  { title: 'Matrícula', key: 'matricula' },
  { title: 'Nombre', key: 'nombre' },
  { title: 'Activo', key: 'activo', align: 'center' }
]

const grupos = ref([])
const maestros = ref([])
const cargando = ref(false)
const guardando = ref(false)
const busqueda = ref('')

const dlgNuevo = reactive({ show: false, codigo: '' })
const dlgAsignar = reactive({ show: false, grupo: null, seleccion: [], busqueda: '' })
const dlgAlumnos = reactive({ show: false, grupo: null, lista: [], busqueda: '', cargando: false })

const totalAlumnos = computed(() => grupos.value.reduce((a, g) => a + g.total_alumnos, 0))
const sinMaestro = computed(() => grupos.value.filter((g) => !g.docentes.length).length)

const filtrados = computed(() => {
  const q = (busqueda.value || '').trim().toLowerCase()
  if (!q) return grupos.value
  return grupos.value.filter((g) =>
    g.codigo.toLowerCase().includes(q) ||
    g.docentes.some((d) => d.nombre.toLowerCase().includes(q) || d.matricula.toLowerCase().includes(q))
  )
})

const maestrosFiltrados = computed(() => {
  const q = (dlgAsignar.busqueda || '').trim().toLowerCase()
  if (!q) return maestros.value
  return maestros.value.filter((m) => m.nombre.toLowerCase().includes(q) || m.matricula.toLowerCase().includes(q))
})

const alternar = (matricula) => {
  const i = dlgAsignar.seleccion.indexOf(matricula)
  if (i >= 0) dlgAsignar.seleccion.splice(i, 1)
  else dlgAsignar.seleccion.push(matricula)
}

const cargar = async () => {
  cargando.value = true
  try {
    grupos.value = await gruposApi.listar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron cargar los grupos'))
  } finally {
    cargando.value = false
  }
}

const crear = async () => {
  guardando.value = true
  try {
    await gruposApi.crear(dlgNuevo.codigo)
    noti.exito('Grupo creado')
    Object.assign(dlgNuevo, { show: false, codigo: '' })
    cargar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo crear el grupo'))
  } finally {
    guardando.value = false
  }
}

const eliminar = async (g) => {
  if (!confirm(`¿Eliminar el grupo ${g.codigo}?`)) return
  try {
    await gruposApi.eliminar(g.codigo)
    noti.exito('Grupo eliminado')
    cargar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo eliminar'))
  }
}

const abrirAsignar = async (g) => {
  try {
    maestros.value = await gruposApi.maestrosDisponibles()
  } catch (e) {
    return noti.error(mensajeError(e, 'No se pudo cargar la lista de maestros'))
  }
  Object.assign(dlgAsignar, {
    show: true,
    grupo: { codigo: g.codigo },
    seleccion: g.docentes.map((d) => d.matricula),
    busqueda: ''
  })
}

const guardarAsignacion = async () => {
  guardando.value = true
  try {
    await gruposApi.asignarDocentes(dlgAsignar.grupo.codigo, [...dlgAsignar.seleccion])
    noti.exito('Maestros asignados')
    dlgAsignar.show = false
    cargar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo guardar la asignación'))
  } finally {
    guardando.value = false
  }
}

const verAlumnos = async (g) => {
  Object.assign(dlgAlumnos, { show: true, grupo: { codigo: g.codigo }, lista: [], busqueda: '', cargando: true })
  try {
    dlgAlumnos.lista = await gruposApi.alumnos(g.codigo)
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron cargar los alumnos'))
  } finally {
    dlgAlumnos.cargando = false
  }
}

onMounted(cargar)
</script>

<style scoped>
.lista-maestros {
  max-height: 340px;
  overflow-y: auto;
}
</style>