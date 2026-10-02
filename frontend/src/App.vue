<template>
  <v-app>
    <router-view />
    <AppNotificaciones />
  </v-app>
</template>

<script lang="ts" setup>
import AppNotificaciones from '@/components/AppNotificaciones.vue'
</script>