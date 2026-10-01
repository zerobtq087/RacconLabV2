<template>
  <v-app>
    <router-view />
    <AppNotificaciones />
  </v-app>
</template>

<script setup>
import AppNotificaciones from '@/components/AppNotificaciones.vue'
</script>