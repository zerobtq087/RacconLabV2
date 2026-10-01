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

        <v-alert v-if="error" type="error" variant="tonal" density="compact" class="mb-4" :text="error" />

        <v-btn type="submit" color="primary" size="large" block :loading="cargando">Iniciar sesión</v-btn>
      </v-form>

      <div class="text-caption text-medium-emphasis text-center mt-6">
        Las cuentas las da de alta el administrador.<br />
        Si no puedes entrar, contacta a tu maestro.
      </div>
    </v-card>
  </v-main>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import BotonTema from '@/components/BotonTema.vue'
import { useAuth } from '@/stores/auth'
import { mensajeError } from '@/api/http'
import { ROLES_UI } from '@/utils/roles'

const auth = useAuth()
const route = useRoute()
const router = useRouter()

const formRef = ref(null)
const verPassword = ref(false)
const cargando = ref(false)
const error = ref('')
const form = reactive({ rol: 'ALUMNO', matricula: '', password: '' })

const requerido = (v) => !!(v && String(v).trim()) || 'Campo obligatorio'

const entrar = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return

  cargando.value = true
  error.value = ''
  try {
    await auth.login(form.matricula, form.password, form.rol)
    router.replace(route.query.redirect || { name: 'laboratorio' })
  } catch (e) {
    error.value = mensajeError(e, 'Matrícula o contraseña incorrectas')
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
</style>