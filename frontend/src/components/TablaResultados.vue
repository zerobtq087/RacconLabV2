<template>
  <!-- Resultado de cada criterio (para el docente: qué se pedía y qué se encontró) -->
  <v-table density="compact">
    <thead>
      <tr>
        <th />
        <th>Criterio</th>
        <th>Se encontró</th>
        <th class="text-right">Puntos</th>
      </tr>
    </thead>
    <tbody>
      <tr v-for="r in resultados" :key="r.id_criterio">
        <td>
          <v-icon :color="r.cumple ? 'success' : 'error'" :icon="r.cumple ? 'mdi-check-circle' : 'mdi-close-circle'" />
        </td>
        <td>
          {{ r.descripcion }}
          <div v-if="!r.cumple" class="text-caption text-medium-emphasis">El alumno ve: {{ r.mensaje }}</div>
        </td>
        <td class="text-medium-emphasis">{{ r.obtenido }}</td>
        <td class="text-right">{{ r.puntos }} / {{ r.peso }}</td>
      </tr>
    </tbody>
  </v-table>
</template>

<script lang="ts" setup>
import type { Resultado } from '@/models/Practica'

defineProps<{ resultados: Resultado[] }>()
</script>
