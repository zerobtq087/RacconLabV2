<template>
  <div class="d-flex flex-wrap align-center justify-space-between ga-4 mb-6">
    <div>
      <div class="page-title">Usuarios y roles</div>
      <div class="page-subtitle">Asigna roles y restablece contraseñas de cualquier usuario.</div>
    </div>
    <v-btn color="primary" prepend-icon="mdi-file-upload" :to="{ name: 'importar' }">Importar</v-btn>
  </div>

  <v-row class="mb-2">
    <v-col v-for="k in kpis" :key="k.titulo" cols="6" md="3">
      <v-card class="pa-4">
        <div class="text-caption">{{ k.titulo }}</div>
        <div class="text-h5 font-weight-bold" :class="`text-${k.color}`">{{ k.valor }}</div>
      </v-card>
    </v-col>
  </v-row>

  <v-card>
    <v-card-text>
      <v-row dense class="mb-2">
        <v-col cols="12" md="5">
          <v-text-field v-model="busqueda" prepend-inner-icon="mdi-magnify" label="Nombre, matrícula o clave" hide-details clearable />
        </v-col>
        <v-col cols="6" md="2">
          <v-select v-model="filtroTipo" :items="TIPOS" label="Tipo" hide-details />
        </v-col>
        <v-col cols="6" md="3">
          <v-select v-model="filtroRol" :items="FILTRO_ROL" label="Tiene el rol" hide-details />
        </v-col>
        <v-col cols="12" md="2">
          <v-select v-model="filtroGrupo" :items="['TODOS', ...grupos]" label="Grupo" hide-details />
        </v-col>
      </v-row>

      <v-data-table
        :headers="headers"
        :items="filtrados"
        :loading="cargando"
        item-value="matricula"
        no-data-text="Sin usuarios. Importa un archivo para empezar."
      >
        <template #item.matricula="{ item }"><span class="font-mono">{{ item.matricula }}</span></template>
        <template #item.tipo="{ item }">
          <span class="text-caption text-medium-emphasis">{{ ETIQUETA_TIPO[item.tipo] }}</span>
        </template>
        <template #item.grupo="{ item }">{{ item.grupo || '—' }}</template>
        <template #item.roles="{ item }">
          <v-chip v-for="r in item.roles" :key="r" :color="colorRol(r)" size="x-small" label class="mr-1">{{ etiquetaRol(r) }}</v-chip>
        </template>
        <template #item.activo="{ item }">
          <v-icon :icon="item.activo ? 'mdi-check-circle' : 'mdi-cancel'" :color="item.activo ? 'success' : 'grey'" />
        </template>
        <template #item.acciones="{ item }">
          <v-btn icon="mdi-shield-edit" variant="text" size="small" title="Editar roles" @click="abrirRoles(item)" />
          <v-btn icon="mdi-key-variant" variant="text" size="small" color="warning" title="Restablecer contraseña" @click="abrirPassword(item)" />
        </template>
      </v-data-table>
    </v-card-text>
  </v-card>

  <!-- Editar roles -->
  <v-dialog v-model="dlg.show" max-width="460">
    <v-card>
      <v-card-item>
        <v-card-title>Roles de {{ dlg.usuario?.nombre }}</v-card-title>
        <v-card-subtitle class="font-mono">{{ dlg.usuario?.matricula }} · {{ ETIQUETA_TIPO[dlg.usuario?.tipo] }}</v-card-subtitle>
      </v-card-item>

      <v-card-text>
        <v-list density="comfortable" class="bg-transparent">
          <v-list-item prepend-icon="mdi-school" title="Alumno" subtitle="Todos los usuarios lo tienen">
            <template #append><v-switch model-value disabled color="success" hide-details inset /></template>
          </v-list-item>
          <v-list-item prepend-icon="mdi-human-male-board" title="Maestro" subtitle="Grupos, plantillas y asistente IA">
            <template #append><v-switch v-model="dlg.profesor" color="secondary" hide-details inset /></template>
          </v-list-item>
          <v-list-item prepend-icon="mdi-shield-crown" title="Admin" subtitle="Usuarios, importación y configuración">
            <template #append>
              <v-switch v-model="dlg.admin" color="error" hide-details inset :disabled="esYo" />
            </template>
          </v-list-item>
        </v-list>

        <v-divider class="my-2" />

        <v-switch v-model="dlg.activo" color="primary" label="Cuenta activa" inset hide-details :disabled="esYo" />

        <v-alert v-if="esYo" type="info" variant="tonal" density="compact" class="mt-2"
          text="No puedes quitarte el rol de Admin ni desactivar tu propia cuenta." />

        <div class="text-caption text-medium-emphasis mt-3">
          Podrá entrar como: <b>{{ rolesResultantes.map(etiquetaRol).join(', ') }}</b>
        </div>
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlg.show = false">Cancelar</v-btn>
        <v-btn color="primary" :loading="guardando" @click="guardarRoles">Guardar</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Restablecer contraseña -->
  <v-dialog v-model="dlgPass.show" max-width="460">
    <v-card>
      <v-card-item>
        <template #prepend><v-icon icon="mdi-key-variant" color="warning" /></template>
        <v-card-title>Restablecer contraseña</v-card-title>
        <v-card-subtitle>{{ dlgPass.usuario?.nombre }} · <span class="font-mono">{{ dlgPass.usuario?.matricula }}</span></v-card-subtitle>
      </v-card-item>

      <v-card-text>
        <v-form ref="formPassRef" @submit.prevent="guardarPassword">
          <v-text-field
            v-model="dlgPass.nueva"
            label="Nueva contraseña"
            class="font-mono"
            :type="dlgPass.ver ? 'text' : 'password'"
            :rules="[req, min8]"
            autocomplete="new-password"
          >
            <template #append-inner>
              <v-btn :icon="dlgPass.ver ? 'mdi-eye-off' : 'mdi-eye'" variant="text" size="small" @click="dlgPass.ver = !dlgPass.ver" />
              <v-btn icon="mdi-dice-5" variant="text" size="small" title="Generar" @click="generar" />
              <v-btn icon="mdi-content-copy" variant="text" size="small" title="Copiar" :disabled="!dlgPass.nueva" @click="copiar" />
            </template>
          </v-text-field>
        </v-form>

        <v-alert type="info" variant="tonal" density="compact"
          text="La contraseña anterior deja de funcionar al instante. Compártela con el usuario por un medio seguro." />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgPass.show = false">Cancelar</v-btn>
        <v-btn color="warning" :loading="guardando" @click="guardarPassword">Restablecer</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { usuariosApi } from '@/api/usuarios'
import { mensajeError } from '@/api/http'
import { useAuth } from '@/stores/auth'
import { useNotificaciones } from '@/stores/notificaciones'
import { etiquetaRol, colorRol, normalizarRoles } from '@/utils/roles'

const auth = useAuth()
const noti = useNotificaciones()

const ETIQUETA_TIPO = { alumnos: 'Alumno', maestros: 'Maestro', admins: 'Admin' }
const TIPOS = [
  { title: 'Todos', value: 'TODOS' },
  { title: 'Alumnos', value: 'alumnos' },
  { title: 'Maestros', value: 'maestros' },
  { title: 'Admins', value: 'admins' }
]
const FILTRO_ROL = [
  { title: 'Cualquiera', value: 'TODOS' },
  { title: 'Maestro', value: 'PROFESOR' },
  { title: 'Admin', value: 'ADMIN' }
]

const headers = [
  { title: 'Matrícula / clave', key: 'matricula' },
  { title: 'Nombre', key: 'nombre' },
  { title: 'Tipo', key: 'tipo' },
  { title: 'Grupo', key: 'grupo' },
  { title: 'Roles', key: 'roles', sortable: false },
  { title: 'Activo', key: 'activo', align: 'center' },
  { title: '', key: 'acciones', sortable: false, align: 'end' }
]

const usuarios = ref([])
const cargando = ref(false)
const guardando = ref(false)
const busqueda = ref('')
const filtroTipo = ref('TODOS')
const filtroRol = ref('TODOS')
const filtroGrupo = ref('TODOS')
const formPassRef = ref(null)

const dlg = reactive({ show: false, usuario: null, profesor: false, admin: false, activo: true })
const dlgPass = reactive({ show: false, usuario: null, nueva: '', ver: true })

const req = (v) => !!v || 'Campo obligatorio'
const min8 = (v) => (v && v.length >= 8) || 'Mínimo 8 caracteres'

const grupos = computed(() => [...new Set(usuarios.value.map((u) => u.grupo).filter(Boolean))].sort())

const kpis = computed(() => [
  { titulo: 'Usuarios', valor: usuarios.value.length, color: 'primary' },
  { titulo: 'Con rol Maestro', valor: usuarios.value.filter((u) => u.roles.includes('PROFESOR')).length, color: 'secondary' },
  { titulo: 'Con rol Admin', valor: usuarios.value.filter((u) => u.roles.includes('ADMIN')).length, color: 'error' },
  { titulo: 'Inactivos', valor: usuarios.value.filter((u) => !u.activo).length, color: 'warning' }
])

const filtrados = computed(() => {
  const q = (busqueda.value || '').trim().toLowerCase()
  return usuarios.value.filter((u) =>
    (filtroTipo.value === 'TODOS' || u.tipo === filtroTipo.value) &&
    (filtroRol.value === 'TODOS' || u.roles.includes(filtroRol.value)) &&
    (filtroGrupo.value === 'TODOS' || u.grupo === filtroGrupo.value) &&
    (!q || u.nombre?.toLowerCase().includes(q) || u.matricula?.toLowerCase().includes(q))
  )
})

const esYo = computed(() =>
  dlg.usuario?.matricula?.toUpperCase() === auth.usuario?.matricula?.toUpperCase()
)

const rolesResultantes = computed(() =>
  normalizarRoles([...(dlg.admin ? ['ADMIN'] : []), ...(dlg.profesor ? ['PROFESOR'] : [])])
)

const cargar = async () => {
  cargando.value = true
  try {
    usuarios.value = await usuariosApi.listar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron cargar los usuarios'))
  } finally {
    cargando.value = false
  }
}

// ----- Roles -----
const abrirRoles = (u) => {
  Object.assign(dlg, {
    show: true,
    usuario: u,
    profesor: u.roles.includes('PROFESOR'),
    admin: u.roles.includes('ADMIN'),
    activo: u.activo
  })
}

const guardarRoles = async () => {
  guardando.value = true
  try {
    await usuariosApi.actualizar({
      matricula: dlg.usuario.matricula,
      roles: rolesResultantes.value,
      activo: dlg.activo
    })
    noti.exito('Roles actualizados')
    dlg.show = false
    cargar()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron guardar los roles'))
  } finally {
    guardando.value = false
  }
}

// ----- Contraseña -----
const generar = () => {
  // Sin caracteres confusos (0/O, 1/l/I)
  const c = 'ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnpqrstuvwxyz23456789'
  const arr = crypto.getRandomValues(new Uint32Array(10))
  dlgPass.nueva = [...arr].map((n) => c[n % c.length]).join('')
  dlgPass.ver = true
}

const copiar = async () => {
  try {
    if (navigator.clipboard && window.isSecureContext) {
      await navigator.clipboard.writeText(dlgPass.nueva)
    } else {
      const ta = document.createElement('textarea')
      ta.value = dlgPass.nueva
      ta.style.position = 'fixed'
      ta.style.opacity = '0'
      document.body.appendChild(ta)
      ta.select()
      document.execCommand('copy')
      document.body.removeChild(ta)
    }
    noti.exito('Contraseña copiada')
  } catch {
    noti.error('No se pudo copiar')
  }
}

const abrirPassword = (u) => {
  Object.assign(dlgPass, { show: true, usuario: u, nueva: '', ver: true })
}

const guardarPassword = async () => {
  const { valid } = await formPassRef.value.validate()
  if (!valid) return
  guardando.value = true
  try {
    await usuariosApi.resetPassword(dlgPass.usuario.matricula, dlgPass.nueva)
    noti.exito(`Contraseña de ${dlgPass.usuario.matricula} restablecida`)
    dlgPass.show = false
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo restablecer la contraseña'))
  } finally {
    guardando.value = false
  }
}

onMounted(cargar)
</script>