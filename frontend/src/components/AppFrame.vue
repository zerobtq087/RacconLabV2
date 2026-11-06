<template>
  <v-app>
    <v-navigation-drawer v-model="drawer">
      <template #prepend>
        <v-list-item
          lines="two"
          prepend-icon="mdi-account-circle"
          :subtitle="`${authStore.userAcc.matricula} · ${nombreRol}`"
          :title="authStore.userAcc.nombre"
        />
      </template>

      <v-divider />
      <v-list>
        <v-list-item
          v-for="(mnu, idx) in menuVisible"
          :key="idx"
          :prepend-icon="mnu.icon"
          :title="mnu.text"
          :to="mnu.to"
        />
      </v-list>
    </v-navigation-drawer>

    <v-app-bar color="primary">
      <v-app-bar-nav-icon @click="drawer = !drawer" />

      <v-app-bar-title>Raccoon Lab</v-app-bar-title>

      <template #append>
        <v-btn icon="mdi-theme-light-dark" @click="toggleTheme" />
        <v-btn icon="mdi-exit-to-app" @click="logout" />
      </template>
    </v-app-bar>

    <v-main>
      <v-slide-x-reverse-transition mode="out-in" group>
        <v-container fluid>
          <slot />
        </v-container>
      </v-slide-x-reverse-transition>
    </v-main>
  </v-app>
</template>

<script lang="ts" setup>
import axios from 'axios'
import { computed, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useTheme } from 'vuetify'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()
const drawer = ref(true)
const theme = useTheme()

interface menuItem {
  icon: string
  text: string
  to: string
  roles: string[]
}

// Cada opción del menú dice qué roles la pueden ver
const menuItems: menuItem[] = [
  { icon: 'mdi-view-dashboard-outline', text: 'Inicio', to: '/', roles: ['ADMIN', 'PROFESOR', 'ALUMNO'] },
  { icon: 'mdi-account-multiple-outline', text: 'Usuarios', to: '/usuarios', roles: ['ADMIN'] },
  { icon: 'mdi-file-upload-outline', text: 'Importar usuarios', to: '/importar', roles: ['ADMIN'] },
  { icon: 'mdi-account-group-outline', text: 'Grupos', to: '/grupos', roles: ['ADMIN', 'PROFESOR'] },
  { icon: 'mdi-lan', text: 'Mi laboratorio', to: '/laboratorio', roles: ['ALUMNO', 'PROFESOR'] },
  { icon: 'mdi-sitemap-outline', text: 'Plantillas', to: '/plantillas', roles: ['PROFESOR'] },
  { icon: 'mdi-monitor-eye', text: 'Monitor', to: '/monitor', roles: ['ADMIN'] },
  { icon: 'mdi-cog-outline', text: 'Mi cuenta', to: '/settings', roles: ['ADMIN', 'PROFESOR', 'ALUMNO'] },
]

const menuVisible = computed(() =>
  menuItems.filter(m => m.roles.includes(authStore.userAcc.rol)),
)

const nombres: Record<string, string> = { ADMIN: 'Administrador', PROFESOR: 'Maestro', ALUMNO: 'Alumno' }
const nombreRol = computed(() => nombres[authStore.userAcc.rol] ?? '')

function toggleTheme () {
  theme.global.name.value = theme.global.current.value.dark ? 'light' : 'dark'
}

const logout = () => {
  axios.post('/api/logout')
    .finally(() => {
      authStore.clear()
      router.push('/login')
    })
}
</script>
