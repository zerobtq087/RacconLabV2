<template>
  <!-- Debajo de un campo de red: la red que calculó el backend o el error en rojo -->
  <div v-if="info" class="text-caption mt-1">
    <span v-if="info.error" class="text-error">{{ info.error }}</span>
    <span v-else class="text-medium-emphasis">
      Red <strong>{{ info.red }}</strong> · máscara {{ info.mascara }} ·
      usables {{ info.primer_host }} a {{ info.ultimo_host }} ({{ info.hosts.toLocaleString('es-MX') }} hosts)
    </span>
  </div>
</template>

<script lang="ts" setup>
import type { InfoRed } from '@/models/Despliegue'

defineProps<{ info?: InfoRed | null }>()
</script>
