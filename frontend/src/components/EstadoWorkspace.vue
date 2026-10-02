<template>
  <v-card class="pa-3" variant="tonal" :color="ws ? 'success' : 'error'">
    <div class="d-flex align-center ga-3">
      <v-icon :icon="ws ? 'mdi-server-network' : 'mdi-server-off'" />
      <div class="flex-grow-1">
        <div class="text-body-2 font-weight-bold font-mono">{{ ws ? ws.container_name : 'Sin workspace activo' }}</div>
        <div v-if="ws" class="text-caption">Puerto GNS3: {{ ws.puerto_gns3 }}</div>
        <div v-else class="text-caption">
          Despliégalo en <router-link :to="{ name: 'laboratorio' }">Mi laboratorio</router-link>.
        </div>
      </div>
      <v-btn icon="mdi-refresh" variant="text" size="small" :loading="cargando" @click="verificar" />
    </div>
  </v-card>
</template>

<script lang="ts" setup>
import { ref, onMounted } from 'vue'
import { labsApi } from '@/api/labs'

const emit = defineEmits(['cambio'])
const ws = ref(null)
const cargando = ref(false)

const verificar = async () => {
  cargando.value = true
  try {
    const d = await labsApi.estado()
    const corriendo = d?.running || d?.status?.running
    // el proxy público = puerto GNS3 + 10000
    ws.value = corriendo ? { container_name: d.id_workspace, puerto_gns3: d.puerto_web - 10000 } : null
  } catch {
    ws.value = null
  } finally {
    cargando.value = false
    emit('cambio', ws.value)
  }
}

onMounted(verificar)
defineExpose({ verificar })
</script>