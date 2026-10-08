<template>
  <v-card class="pa-3" variant="tonal" :color="ws ? 'success' : 'error'">
    <div class="d-flex align-center ga-3">
      <v-icon :icon="ws ? 'mdi-server-network' : 'mdi-server-off'" />
      <div class="flex-grow-1">
        <div class="text-body-2 font-weight-bold font-mono">{{ ws ? ws.id_workspace : 'Sin laboratorio activo' }}</div>
        <div v-if="ws" class="text-caption">Puerto público: {{ ws.puerto_web }}</div>
        <div v-else class="text-caption">Despliega tu laboratorio en “Mi laboratorio” primero.</div>
      </div>
      <v-btn icon="mdi-refresh" variant="text" size="small" :loading="cargando" @click="verificar" />
    </div>
  </v-card>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { labsApi } from '@/api/labs'

interface Estado {
  running?: boolean
  id_workspace?: string
  puerto_web?: number
  soy_dueno?: boolean
}

const emit = defineEmits<{ cambio: [valor: { container_name: string; puerto_gns3: number } | null] }>()

const ws = ref<Estado | null>(null)
const cargando = ref(false)

const verificar = async () => {
  cargando.value = true
  try {
    const e: Estado = await labsApi.estado()
    // Solo sirve si está corriendo y es SU laboratorio (las plantillas van al lab del dueño)
    ws.value = e?.id_workspace && e.running && e.soy_dueno ? e : null
  } catch {
    ws.value = null
  } finally {
    cargando.value = false
    emit('cambio', ws.value
      ? { container_name: ws.value.id_workspace!, puerto_gns3: (ws.value.puerto_web ?? 10000) - 10000 }
      : null)
  }
}

onMounted(verificar)
defineExpose({ verificar })
</script>
