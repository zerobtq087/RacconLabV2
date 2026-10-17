<template>
  <v-container class="h-100 pa-4" fluid>
    <v-row class="h-100 align-center justify-center">
      <v-sheet class="flex-1-1 px-4" color="transparent" max-width="420">
        <v-img class="mx-auto mb-4" height="60" max-width="60" src="@/assets/logo.svg" />

        <div class="text-headline-small text-center mb-8 font-weight-medium">Raccoon Lab</div>

        <v-sheet border="opacity-25 thin" class="overflow-hidden" rounded="lg">
          <v-select
            v-model="credentials.rol"
            class="ma-3"
            hide-details
            item-title="text"
            item-value="value"
            :items="roles"
            placeholder="Rol"
            prepend-inner-icon="mdi-account-key"
            variant="solo"
          />
          <v-divider />
          <v-text-field
            v-model="credentials.matricula"
            autocomplete="username"
            class="ma-3"
            hide-details
            placeholder="Matrícula"
            prepend-inner-icon="mdi-card-account-details"
            variant="solo"
          />
          <v-divider />
          <v-text-field
            v-model="credentials.password"
            autocomplete="current-password"
            class="ma-3"
            hide-details
            placeholder="Contraseña"
            prepend-inner-icon="mdi-lock"
            type="password"
            variant="solo"
            @keyup.enter="login"
          />
        </v-sheet>

        <v-alert v-if="segundos > 0" class="mt-4" type="warning" variant="tonal">
          Cuenta bloqueada por intentos fallidos. Intenta en {{ reloj }}
        </v-alert>

        <v-btn
          block
          class="text-none my-6"
          color="primary"
          :disabled="segundos > 0"
          flat
          :loading="cargando"
          rounded="lg"
          text="Iniciar sesión"
          @click="login"
        />
      </v-sheet>
    </v-row>
  </v-container>
</template>

<script lang="ts" setup>
import axios from 'axios'
import { computed, onBeforeUnmount, reactive, ref } from 'vue'
import { useRouter } from 'vue-router'
import Swal from 'sweetalert2'
import { useAuthStore } from '@/stores/auth'

const router = useRouter()
const authStore = useAuthStore()

const roles = [
  { text: 'Alumno', value: 'ALUMNO' },
  { text: 'Maestro', value: 'PROFESOR' },
  { text: 'Administrador', value: 'ADMIN' },
]

const credentials = reactive({ rol: 'ALUMNO', matricula: '', password: '' })
const cargando = ref(false)

// Cuenta regresiva cuando el backend bloquea la cuenta (HTTP 429)
const segundos = ref(0)
let timer: ReturnType<typeof setInterval> | undefined

const reloj = computed(() => {
  const m = Math.floor(segundos.value / 60)
  const s = segundos.value % 60
  return `${m}:${String(s).padStart(2, '0')}`
})

function iniciarBloqueo (total: number) {
  segundos.value = total
  clearInterval(timer)
  timer = setInterval(() => {
    segundos.value--
    if (segundos.value <= 0) clearInterval(timer)
  }, 1000)
}

onBeforeUnmount(() => clearInterval(timer))

const login = () => {
  if (!credentials.matricula || !credentials.password) {
    Swal.fire({ icon: 'warning', title: 'Escribe tu matrícula y contraseña' })
    return
  }

  cargando.value = true
  axios.post('/api/login', credentials)
    .then(async response => {
      if (response.data == 'success') {
        await authStore.fetchProfile()
        router.push('/')
      }
    })
    .catch(err => {
      if (err.response?.status == 429) {
        iniciarBloqueo(Number(err.response.headers['retry-after']) || 900)
        return
      }
      Swal.fire({ icon: 'error', title: 'No se pudo entrar', text: err.response?.data || 'Sin conexión con el servidor' })
    })
    .finally(() => cargando.value = false)
}
</script>
