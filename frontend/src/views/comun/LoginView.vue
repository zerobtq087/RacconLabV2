<template>
  <v-main class="d-flex align-center justify-center login-fondo">
    <div class="boton-tema"><BotonTema /></div>

    <v-card class="pa-8 mx-4" width="100%" max-width="440">
      <div class="text-center mb-6">
        <v-icon icon="mdi-lan" size="56" color="primary" class="mb-2" />
        <div class="page-title">Raccoon Lab</div>
        <div class="page-subtitle">Plataforma de laboratorios de redes</div>
      </div>

      <v-alert v-if="route.query.expirada" type="warning" variant="tonal" density="compact" class="mb-4"
        text="Tu sesión expiró. Vuelve a iniciar sesión." />

      <v-form ref="formRef" @submit.prevent="entrar">
        <div class="text-caption font-weight-bold mb-2">Entrar como</div>
        <v-btn-toggle v-model="form.rol" mandatory color="primary" variant="outlined" divided class="w-100 mb-5">
          <v-btn v-for="r in ROLES_UI" :key="r.value" :value="r.value" :prepend-icon="r.icon" class="flex-grow-1 px-2">
            {{ r.title }}
          </v-btn>
        </v-btn-toggle>

        <v-text-field
          v-model="form.matricula"
          label="Matrícula o clave"
          prepend-inner-icon="mdi-card-account-details"
          autocomplete="username"
          :rules="[requerido]"
          autofocus
        />

        <v-text-field
          v-model="form.password"
          label="Contraseña"
          prepend-inner-icon="mdi-lock"
          :type="verPassword ? 'text' : 'password'"
          :append-inner-icon="verPassword ? 'mdi-eye-off' : 'mdi-eye'"
          autocomplete="current-password"
          :rules="[requerido]"
          @click:append-inner="verPassword = !verPassword"
        />

        <!-- Bloqueo con cuenta regresiva en tiempo real -->
        <v-alert v-if="bloqueada" type="warning" variant="tonal" class="mb-4" icon="mdi-lock-clock">
          <div>Cuenta bloqueada por intentos fallidos.</div>
          <div class="d-flex align-center mt-1">
            <span class="mr-2">Podrás intentar de nuevo en</span>
            <span class="text-h5 font-weight-bold reloj">{{ tiempoRestante }}</span>
          </div>
        </v-alert>

        <v-alert v-else-if="error" type="error" variant="tonal" density="compact" class="mb-4" :text="error" />

        <v-btn type="submit" color="primary" size="large" block :loading="cargando" :disabled="bloqueada">
          {{ bloqueada ? 'Bloqueado' : 'Iniciar sesión' }}
        </v-btn>
      </v-form>

      <div class="text-caption text-medium-emphasis text-center mt-6">
        Las cuentas las da de alta el administrador.<br />
        Si no puedes entrar, contacta a tu maestro.
      </div>
    </v-card>
  </v-main>
</template>

<script lang="ts" setup>
import { ref, reactive, computed, onBeforeUnmount } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import type { VForm } from 'vuetify/components'
import BotonTema from '@/components/BotonTema.vue'
import { useAuth } from '@/stores/auth'
import { mensajeError } from '@/api/http'
import { ROLES_UI } from '@/utils/roles'

type Rol = 'ALUMNO' | 'PROFESOR' | 'ADMIN'

interface ErrorApi {
  response?: {
    status?: number
    headers?: Record<string, string>
    data?: { message?: string; bloqueo_s?: number }
  }
}

const BLOQUEO_POR_DEFECTO = 15 * 60

const auth = useAuth()
const route = useRoute()
const router = useRouter()

const formRef = ref<InstanceType<typeof VForm> | null>(null)
const verPassword = ref(false)
const cargando = ref(false)
const error = ref('')
const form = reactive<{ rol: Rol; matricula: string; password: string }>({
  rol: 'ALUMNO',
  matricula: '',
  password: ''
})

const requerido = (v: string) => !!(v && String(v).trim()) || 'Campo obligatorio'

// ---- Bloqueo: matrícula bloqueada + segundos restantes ----
const matriculaBloqueada = ref('')
const segundos = ref(0)
let reloj: ReturnType<typeof setInterval> | null = null

const normalizar = (m: string) => m.trim().toLowerCase()

// El bloqueo es por matrícula: si escribe otra, puede intentar
const bloqueada = computed(
  () => segundos.value > 0 && normalizar(form.matricula) === matriculaBloqueada.value
)

const tiempoRestante = computed(() => {
  const m = Math.floor(segundos.value / 60)
  const s = segundos.value % 60
  return `${String(m).padStart(2, '0')}:${String(s).padStart(2, '0')}`
})

const detenerReloj = () => {
  if (reloj) clearInterval(reloj)
  reloj = null
}

const iniciarBloqueo = (matricula: string, seg: number) => {
  detenerReloj()
  matriculaBloqueada.value = normalizar(matricula)
  // Se calcula contra la hora de fin: no se desfasa si la pestaña se duerme
  const fin = Date.now() + seg * 1000
  segundos.value = seg
  reloj = setInterval(() => {
    segundos.value = Math.max(0, Math.ceil((fin - Date.now()) / 1000))
    if (segundos.value === 0) {
      detenerReloj()
      matriculaBloqueada.value = ''
      error.value = ''
    }
  }, 1000)
}

onBeforeUnmount(detenerReloj)

// Segundos de bloqueo: del JSON, del header Retry-After o 15 min
const segundosDeBloqueo = (err: ErrorApi): number => {
  const delJson = Number(err.response?.data?.bloqueo_s)
  if (delJson > 0) return delJson
  const delHeader = Number(err.response?.headers?.['retry-after'])
  if (delHeader > 0) return delHeader
  return BLOQUEO_POR_DEFECTO
}

const entrar = async () => {
  const resultado = await formRef.value?.validate()
  if (!resultado?.valid || bloqueada.value) return

  cargando.value = true
  error.value = ''
  try {
    await auth.login(form.matricula, form.password, form.rol)
    router.replace((route.query.redirect as string) || { name: 'laboratorio' })
  } catch (e) {
    const err = e as ErrorApi
    if (err.response?.status === 429) {
      iniciarBloqueo(form.matricula, segundosDeBloqueo(err))
      form.password = ''
    } else {
      error.value = mensajeError(e, 'Matrícula o contraseña incorrectas')
    }
  } finally {
    cargando.value = false
  }
}
</script>

<style scoped>
.login-fondo {
  min-height: 100vh;
  background:
    radial-gradient(circle at 20% 20%, rgba(249, 115, 22, 0.12), transparent 40%),
    radial-gradient(circle at 80% 80%, rgba(56, 189, 248, 0.1), transparent 40%);
}
.boton-tema {
  position: fixed;
  top: 12px;
  right: 12px;
  z-index: 10;
}
.reloj {
  font-variant-numeric: tabular-nums;
}
</style>
