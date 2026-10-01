<template>
  <v-navigation-drawer v-model="drawer">
    <v-list-item class="py-4" prepend-icon="mdi-lan" title="Raccoon Lab" subtitle="v2" nav />

    <v-divider />

    <v-list nav density="comfortable" color="primary">
      <template v-for="grupo in menu" :key="grupo.seccion">
        <v-list-subheader v-if="grupo.seccion">{{ grupo.seccion }}</v-list-subheader>
        <v-list-item
          v-for="item in grupo.items"
          :key="item.name"
          :to="{ name: item.name }"
          :prepend-icon="item.meta.icono"
          :title="item.meta.titulo"
        />
      </template>
    </v-list>
  </v-navigation-drawer>

  <v-app-bar flat border="b">
    <v-app-bar-nav-icon @click="drawer = !drawer" />
    <v-app-bar-title class="font-weight-bold">{{ route.meta.titulo || 'Raccoon Lab' }}</v-app-bar-title>

    <template #append>
      <BotonTema />

      <v-menu location="bottom end">
        <template #activator="{ props }">
          <v-btn v-bind="props" variant="text" class="ml-2">
            <v-avatar color="primary" size="32" class="mr-2">
              <span class="text-caption font-weight-bold">{{ iniciales }}</span>
            </v-avatar>
            <div class="text-left d-none d-sm-block">
              <div class="text-body-2 font-weight-bold">{{ auth.usuario?.nombre }}</div>
              <div class="text-caption text-medium-emphasis">{{ auth.usuario?.matricula }}</div>
            </div>
            <v-chip :color="colorRol(auth.rol)" size="x-small" class="ml-3" label>{{ etiquetaRol(auth.rol) }}</v-chip>
          </v-btn>
        </template>

        <v-list density="compact" min-width="220">
          <template v-if="auth.rolesDisponibles.length > 1">
            <v-list-subheader>Cambiar rol</v-list-subheader>
            <v-list-item
              v-for="r in auth.rolesDisponibles"
              :key="r"
              :active="r === auth.rol"
              :title="etiquetaRol(r)"
              prepend-icon="mdi-account-switch"
              @click="cambiarRol(r)"
            />
            <v-divider class="my-1" />
          </template>
          <v-list-item prepend-icon="mdi-account-circle" title="Mi perfil" :to="{ name: 'perfil' }" />
          <v-list-item prepend-icon="mdi-logout" title="Cerrar sesión" base-color="error" @click="salir" />
        </v-list>
      </v-menu>
    </template>
  </v-app-bar>

  <v-main>
    <v-container fluid class="pa-4 pa-md-6" style="max-width: 1400px">
      <router-view />
    </v-container>
  </v-main>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useDisplay } from 'vuetify'
import BotonTema from '@/components/BotonTema.vue'
import { useAuth } from '@/stores/auth'
import { useNotificaciones } from '@/stores/notificaciones'
import { mensajeError } from '@/api/http'
import { etiquetaRol, colorRol } from '@/utils/roles'

const auth = useAuth()
const noti = useNotificaciones()
const route = useRoute()
const router = useRouter()
const { mobile } = useDisplay()

// null = Vuetify decide: abierto en escritorio, cerrado en móvil (Baseline)
const drawer = ref(null)

// En móvil, cerrar el menú después de elegir una opción
watch(() => route.fullPath, () => {
  if (mobile.value) drawer.value = false
})

const iniciales = computed(() =>
  (auth.usuario?.nombre || '?').split(' ').filter(Boolean).slice(0, 2).map((p) => p[0]).join('').toUpperCase()
)

// El menú sale de las rutas y se filtra por el rol activo
const menu = computed(() => {
  const hijos = router.options.routes.find((r) => r.path === '/')?.children || []
  const visibles = hijos.filter((r) => r.meta?.menu && (!r.meta.roles || r.meta.roles.includes(auth.rol)))
  const secciones = new Map()
  for (const r of visibles) {
    const s = r.meta.seccion || ''
    if (!secciones.has(s)) secciones.set(s, [])
    secciones.get(s).push(r)
  }
  return [...secciones.entries()].map(([seccion, items]) => ({ seccion, items }))
})

const cambiarRol = async (rol) => {
  if (rol === auth.rol) return
  try {
    await auth.cambiarRol(rol)
    noti.exito(`Ahora estás como ${etiquetaRol(rol)}`)
    router.push({ name: 'laboratorio' })
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo cambiar de rol'))
  }
}

const salir = async () => {
  await auth.logout()
  router.replace({ name: 'login' })
}
</script>