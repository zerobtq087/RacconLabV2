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
        <div class="text-h5 font-weight-bold" :class="`text-${k.color}`">{{ k.valor.toLocaleString('es-MX') }}</div>
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

      <!-- Paginación en el servidor: solo se descargan los usuarios de la página -->
      <v-data-table-server
        v-model:page="pagina"
        v-model:items-per-page="porPagina"
        v-model:sort-by="orden"
        :headers="headers"
        :items="usuarios"
        :items-length="total"
        :loading="cargando"
        :items-per-page-options="[10, 25, 50, 100]"
        item-value="matricula"
        no-data-text="Sin usuarios. Importa un archivo para empezar."
        @update:options="cargar"
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
        <template #item.nombre="{ item }">
          {{ item.nombre }}
          <v-chip v-if="item.protegido" size="x-small" color="error" variant="outlined" prepend-icon="mdi-lock" class="ml-1">
            General
          </v-chip>
        </template>
        <template #item.acciones="{ item }">
          <v-btn
            icon="mdi-pencil" variant="text" size="small"
            :title="item.protegido ? 'El administrador general no se puede modificar' : 'Editar datos'"
            :disabled="item.protegido"
            @click="abrirDatos(item)"
          />
          <v-btn
            icon="mdi-shield-edit" variant="text" size="small"
            :title="item.protegido ? 'El administrador general no se puede modificar' : 'Editar roles'"
            :disabled="item.protegido"
            @click="abrirRoles(item)"
          />
          <v-btn
            icon="mdi-key-variant" variant="text" size="small" color="warning"
            :title="item.protegido && !soyYo(item) ? 'Solo él puede cambiar su contraseña' : 'Restablecer contraseña'"
            :disabled="item.protegido && !soyYo(item)"
            @click="abrirPassword(item)"
          />
          <v-btn
            icon="mdi-delete" variant="text" size="small" color="error"
            :title="item.protegido ? 'El administrador general no se puede eliminar' : soyYo(item) ? 'No puedes eliminarte' : 'Eliminar'"
            :disabled="item.protegido || soyYo(item)"
            @click="abrirEliminar(item)"
          />
        </template>
      </v-data-table-server>
    </v-card-text>
  </v-card>

  <!-- Editar datos -->
  <v-dialog v-model="dlgDatos.show" max-width="460">
    <v-card>
      <v-card-item>
        <template #prepend><v-icon icon="mdi-pencil" color="primary" /></template>
        <v-card-title>Editar datos</v-card-title>
        <v-card-subtitle>{{ ETIQUETA_TIPO[dlgDatos.usuario?.tipo ?? ''] }} · <span class="font-mono">{{ dlgDatos.usuario?.matricula }}</span></v-card-subtitle>
      </v-card-item>

      <v-card-text>
        <v-form ref="formDatosRef" @submit.prevent="guardarDatos">
          <v-text-field
            v-model="dlgDatos.matricula"
            :label="dlgDatos.usuario?.tipo === 'alumnos' ? 'Matrícula' : 'Clave'"
            class="font-mono"
            :rules="[req, reglaMatricula]"
            :disabled="soyYo(dlgDatos.usuario)"
            :hint="soyYo(dlgDatos.usuario) ? 'No puedes cambiar tu propia clave' : 'Es su usuario para entrar'"
            persistent-hint
          />
          <v-text-field v-model="dlgDatos.nombre" label="Nombre completo" class="mt-2" :rules="[req]" counter="120" />
          <v-combobox
            v-if="dlgDatos.usuario?.tipo === 'alumnos'"
            v-model="dlgDatos.grupo"
            :items="grupos"
            label="Grupo"
            class="mt-2 font-mono"
            :rules="[req]"
            hint="Elige uno o escribe uno nuevo (se crea solo)"
            persistent-hint
          />
        </v-form>

        <v-alert v-if="!soyYo(dlgDatos.usuario)" type="warning" variant="tonal" density="compact" class="mt-4"
          text="Al guardar se cierran las sesiones abiertas de este usuario." />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgDatos.show = false">Cancelar</v-btn>
        <v-btn color="primary" :loading="guardando" @click="guardarDatos">Guardar</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Eliminar -->
  <v-dialog v-model="dlgEliminar.show" max-width="440">
    <v-card>
      <v-card-item>
        <template #prepend><v-icon icon="mdi-alert" color="error" /></template>
        <v-card-title>Eliminar usuario</v-card-title>
      </v-card-item>
      <v-card-text>
        Se eliminará a <b>{{ dlgEliminar.usuario?.nombre }}</b>
        (<span class="font-mono">{{ dlgEliminar.usuario?.matricula }}</span>) con sus roles, asignaciones a grupos
        y laboratorios. <b>No se puede deshacer.</b>
        <v-text-field
          v-model="dlgEliminar.confirmacion"
          class="mt-4 font-mono"
          :label="`Escribe ${dlgEliminar.usuario?.matricula} para confirmar`"
          hide-details
        />
      </v-card-text>
      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgEliminar.show = false">Cancelar</v-btn>
        <v-btn
          color="error" :loading="guardando"
          :disabled="dlgEliminar.confirmacion.trim().toLowerCase() !== dlgEliminar.usuario?.matricula"
          @click="eliminar"
        >
          Eliminar
        </v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>

  <!-- Editar roles -->
  <v-dialog v-model="dlg.show" max-width="460">
    <v-card>
      <v-card-item>
        <v-card-title>Roles de {{ dlg.usuario?.nombre }}</v-card-title>
        <v-card-subtitle class="font-mono">{{ dlg.usuario?.matricula }} · {{ ETIQUETA_TIPO[dlg.usuario?.tipo ?? ''] }}</v-card-subtitle>
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
        <v-alert v-else type="warning" variant="tonal" density="compact" class="mt-2"
          text="Al guardar se cierran las sesiones abiertas de este usuario." />

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
          text="La contraseña anterior deja de funcionar al instante y se cierran sus sesiones. Compártela con el usuario por un medio seguro." />
      </v-card-text>

      <v-card-actions>
        <v-spacer />
        <v-btn variant="text" @click="dlgPass.show = false">Cancelar</v-btn>
        <v-btn color="warning" :loading="guardando" @click="guardarPassword">Restablecer</v-btn>
      </v-card-actions>
    </v-card>
  </v-dialog>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, watch, onMounted } from 'vue'
import type { VForm } from 'vuetify/components'
import { usuariosApi } from '@/api/usuarios'
import { mensajeError } from '@/api/http'
import { useAuth } from '@/stores/auth'
import { useNotificaciones } from '@/stores/notificaciones'
import { etiquetaRol, colorRol, normalizarRoles } from '@/utils/roles'

type Tipo = 'alumnos' | 'maestros' | 'admins'

interface Usuario {
  matricula: string
  nombre: string
  tipo: Tipo
  grupo: string | null
  roles: string[]
  activo: boolean
  protegido: boolean // admin general: nadie lo puede modificar
}

interface Resumen {
  usuarios: number
  profesores: number
  admins: number
  inactivos: number
  grupos: string[]
}

const auth = useAuth()
const noti = useNotificaciones()

const ETIQUETA_TIPO: Record<string, string> = { alumnos: 'Alumno', maestros: 'Maestro', admins: 'Admin' }
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
  { title: 'Activo', key: 'activo', align: 'center' as const },
  { title: '', key: 'acciones', sortable: false, align: 'end' as const }
]

// ---- Tabla (paginada en el servidor) ----
const usuarios = ref<Usuario[]>([])
const total = ref(0)
const pagina = ref(1)
const porPagina = ref(10)
const orden = ref<{ key: string; order?: 'asc' | 'desc' | boolean }[]>([])
const cargando = ref(false)
const guardando = ref(false)

const busqueda = ref('')
const filtroTipo = ref('TODOS')
const filtroRol = ref('TODOS')
const filtroGrupo = ref('TODOS')

const resumen = ref<Resumen>({ usuarios: 0, profesores: 0, admins: 0, inactivos: 0, grupos: [] })
const grupos = computed(() => resumen.value.grupos)

const kpis = computed(() => [
  { titulo: 'Usuarios', valor: resumen.value.usuarios, color: 'primary' },
  { titulo: 'Con rol Maestro', valor: resumen.value.profesores, color: 'secondary' },
  { titulo: 'Con rol Admin', valor: resumen.value.admins, color: 'error' },
  { titulo: 'Inactivos', valor: resumen.value.inactivos, color: 'warning' }
])

const cargar = async () => {
  cargando.value = true
  try {
    const o = orden.value[0]
    const datos = await usuariosApi.listar({
      pagina: pagina.value,
      por_pagina: porPagina.value,
      q: busqueda.value?.trim() || undefined,
      tipo: filtroTipo.value === 'TODOS' ? undefined : filtroTipo.value,
      rol: filtroRol.value === 'TODOS' ? undefined : filtroRol.value,
      grupo: filtroGrupo.value === 'TODOS' ? undefined : filtroGrupo.value,
      orden: o?.key,
      desc: o?.order === 'desc' ? true : undefined
    })
    usuarios.value = datos.items
    total.value = datos.total
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron cargar los usuarios'))
  } finally {
    cargando.value = false
  }
}

const cargarResumen = async () => {
  try {
    resumen.value = await usuariosApi.resumen()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo cargar el resumen'))
  }
}

// Al cambiar un filtro se vuelve a la página 1 (la búsqueda espera a que dejes de escribir)
let espera: ReturnType<typeof setTimeout> | null = null
const recargarDesdeInicio = () => {
  if (pagina.value !== 1) pagina.value = 1 // dispara @update:options
  else cargar()
}
watch([filtroTipo, filtroRol, filtroGrupo], recargarDesdeInicio)
watch(busqueda, () => {
  if (espera) clearTimeout(espera)
  espera = setTimeout(recargarDesdeInicio, 400)
})

onMounted(cargarResumen) // la tabla se carga sola con @update:options

// ---- Roles ----
const dlg = reactive<{ show: boolean; usuario: Usuario | null; profesor: boolean; admin: boolean; activo: boolean }>({
  show: false, usuario: null, profesor: false, admin: false, activo: true
})

const soyYo = (u?: Usuario | null) =>
  !!u && u.matricula.toLowerCase() === auth.usuario?.matricula?.toLowerCase()

const esYo = computed(() => soyYo(dlg.usuario))

const rolesResultantes = computed<string[]>(() =>
  normalizarRoles([...(dlg.admin ? ['ADMIN'] : []), ...(dlg.profesor ? ['PROFESOR'] : [])])
)

const abrirRoles = (u: Usuario) => {
  Object.assign(dlg, {
    show: true,
    usuario: u,
    profesor: u.roles.includes('PROFESOR'),
    admin: u.roles.includes('ADMIN'),
    activo: u.activo
  })
}

const guardarRoles = async () => {
  if (!dlg.usuario) return
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
    cargarResumen()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron guardar los roles'))
  } finally {
    guardando.value = false
  }
}

// ---- Datos (matrícula, nombre, grupo) ----
const formDatosRef = ref<InstanceType<typeof VForm> | null>(null)
const dlgDatos = reactive<{ show: boolean; usuario: Usuario | null; matricula: string; nombre: string; grupo: string | null }>({
  show: false, usuario: null, matricula: '', nombre: '', grupo: null
})

const reglaMatricula = (v: string) =>
  /^[a-z0-9._-]{1,20}$/.test((v || '').trim().toLowerCase()) || 'Solo letras, números, . _ - (máx. 20)'

const abrirDatos = (u: Usuario) => {
  Object.assign(dlgDatos, { show: true, usuario: u, matricula: u.matricula, nombre: u.nombre, grupo: u.grupo })
}

const guardarDatos = async () => {
  const resultado = await formDatosRef.value?.validate()
  if (!resultado?.valid || !dlgDatos.usuario) return
  guardando.value = true
  try {
    await usuariosApi.editarDatos({
      matricula: dlgDatos.usuario.matricula,
      nueva_matricula: dlgDatos.matricula.trim().toLowerCase(),
      nombre: dlgDatos.nombre,
      grupo: dlgDatos.usuario.tipo === 'alumnos' ? dlgDatos.grupo : null
    })
    noti.exito('Datos actualizados')
    dlgDatos.show = false
    cargar()
    cargarResumen()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudieron guardar los datos'))
  } finally {
    guardando.value = false
  }
}

// ---- Eliminar ----
const dlgEliminar = reactive<{ show: boolean; usuario: Usuario | null; confirmacion: string }>({
  show: false, usuario: null, confirmacion: ''
})

const abrirEliminar = (u: Usuario) => {
  Object.assign(dlgEliminar, { show: true, usuario: u, confirmacion: '' })
}

const eliminar = async () => {
  if (!dlgEliminar.usuario) return
  guardando.value = true
  try {
    await usuariosApi.eliminar(dlgEliminar.usuario.matricula)
    noti.exito(`${dlgEliminar.usuario.matricula} eliminado`)
    dlgEliminar.show = false
    // Si era el último de la página, regresar una página
    if (usuarios.value.length === 1 && pagina.value > 1) pagina.value--
    else cargar()
    cargarResumen()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo eliminar'))
  } finally {
    guardando.value = false
  }
}

// ---- Contraseña ----
const formPassRef = ref<InstanceType<typeof VForm> | null>(null)
const dlgPass = reactive<{ show: boolean; usuario: Usuario | null; nueva: string; ver: boolean }>({
  show: false, usuario: null, nueva: '', ver: true
})

const req = (v: string) => !!v || 'Campo obligatorio'
const min8 = (v: string) => (v && v.length >= 8) || 'Mínimo 8 caracteres'

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
      // En http (sin https) el portapapeles moderno no existe
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

const abrirPassword = (u: Usuario) => {
  Object.assign(dlgPass, { show: true, usuario: u, nueva: '', ver: true })
}

const guardarPassword = async () => {
  const resultado = await formPassRef.value?.validate()
  if (!resultado?.valid || !dlgPass.usuario) return
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
</script>
