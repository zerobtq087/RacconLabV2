<template>
  <v-snackbar v-model="noti.visible" :color="noti.color" :timeout="noti.timeout" location="top right" variant="tonal">
    {{ noti.texto }}
    <template #actions>
      <v-btn icon="mdi-close" variant="text" size="small" @click="noti.visible = false" />
    </template>
  </v-snackbar>
</template>

<script lang="ts" setup>
import { useNotificaciones } from '@/stores/notificaciones'
const noti = useNotificaciones()
</script>