<template>
  <AppFrame>
    <v-card border class="my-5" elevation="0" rounded="lg">
      <v-card-title>Bienvenido, {{ authStore.userAcc.nombre }}</v-card-title>
      <v-card-text>
        Entraste como <strong>{{ authStore.userAcc.rol }}</strong>
        <span v-if="authStore.userAcc.grupo"> · Grupo {{ authStore.userAcc.grupo }}</span>
      </v-card-text>
    </v-card>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()
</script>
