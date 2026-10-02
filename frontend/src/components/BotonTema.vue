<template>
  <v-tooltip :text="esOscuro ? 'Cambiar a modo día' : 'Cambiar a modo noche'" location="bottom">
    <template #activator="{ props }">
      <v-btn
        v-bind="props"
        :icon="esOscuro ? 'mdi-weather-sunny' : 'mdi-weather-night'"
        variant="text"
        @click="alternar"
      />
    </template>
  </v-tooltip>
</template>

<script lang="ts" setup>
import { computed } from 'vue'
import { useTheme } from 'vuetify'

const tema = useTheme()

const esOscuro = computed(() => tema.global.current.value.dark)

const alternar = () => {
  const nuevo = esOscuro.value ? 'raccoonLight' : 'raccoonDark'
  tema.global.name.value = nuevo
  localStorage.setItem('raccoon_theme', nuevo)
}
</script>