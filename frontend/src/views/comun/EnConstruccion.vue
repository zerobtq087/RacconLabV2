<template>
  <div class="mb-6">
    <div class="page-title">{{ route.meta.titulo }}</div>
    <div class="page-subtitle">Esta sección se construye en los siguientes pasos.</div>
  </div>
  <v-card class="pa-10 text-center">
    <v-icon :icon="route.meta.icono" size="64" color="primary" />
    <div class="text-h6 mt-4">En construcción</div>
  </v-card>
</template>

<script lang="ts" setup>
import { useRoute } from 'vue-router'
const route = useRoute()
</script>