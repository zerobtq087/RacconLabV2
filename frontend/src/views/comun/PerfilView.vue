<template>
  <div class="mb-6">
    <div class="page-title">Mi perfil</div>
    <div class="page-subtitle">Tus datos los carga el administrador; aquí solo puedes cambiar tu contraseña.</div>
  </div>

  <v-row>
    <!-- Datos -->
    <v-col cols="12" md="5">
      <v-card :loading="cargando">
        <v-card-item>
          <template #prepend>
            <v-avatar color="primary" size="56">
              <span class="text-h6 font-weight-bold">{{ iniciales }}</span>
            </v-avatar>
          </template>
          <v-card-title>{{ perfil.nombre || auth.usuario?.nombre }}</v-card-title>
          <v-card-subtitle class="font-mono">{{ perfil.matricula || auth.usuario?.matricula }}</v-card-subtitle>
        </v-card-item>

        <v-divider />

        <v-list density="comfortable" class="bg-transparent">
          <v-list-item prepend-icon="mdi-card-account-details-outline" title="Tipo de cuenta" :subtitle="ETIQUETA_TIPO[perfil.tipo] || '—'" />

          <v-list-item v-if="perfil.tipo === 'alumnos'" prepend-icon="mdi-account-group-outline" title="Grupo" :subtitle="perfil.grupo || '—'" />

          <v-list-item v-if="perfil.roles?.includes('PROFESOR')" prepend-icon="mdi-human-male-board" title="Grupos que imparto">
            <template #subtitle>
              <span v-if="!perfil.grupos_docente?.length">Aún no tienes grupos asignados</span>
              <v-chip v-for="g in perfil.grupos_docente" :key="g" size="small" label class="mr-1 mt-1 font-mono">{{ g }}</v-chip>
            </template>
          </v-list-item>

          <v-list-item prepend-icon="mdi-shield-account-outline" title="Roles asignados">
            <template #subtitle>
              <v-chip v-for="r in perfil.roles || []" :key="r" :color="colorRol(r)" size="small" label class="mr-1 mt-1">
                {{ etiquetaRol(r) }}
              </v-chip>
            </template>
          </v-list-item>

          <v-list-item prepend-icon="mdi-login" title="Sesión actual como">
            <template #subtitle>
              <v-chip :color="colorRol(auth.rol)" size="small" label class="mt-1">{{ etiquetaRol(auth.rol) }}</v-chip>
            </template>
          </v-list-item>
        </v-list>
      </v-card>
    </v-col>

    <!-- Contraseña -->
    <v-col cols="12" md="7">
      <v-card title="Cambiar contraseña" prepend-icon="mdi-lock-reset">
        <v-card-text>
          <v-form ref="formRef" @submit.prevent="guardar">
            <v-text-field
              v-model="form.actual"
              label="Contraseña actual"
              :type="ver.actual ? 'text' : 'password'"
              :append-inner-icon="ver.actual ? 'mdi-eye-off' : 'mdi-eye'"
              autocomplete="current-password"
              :rules="[req]"
              @click:append-inner="ver.actual = !ver.actual"
            />
            <v-text-field
              v-model="form.nueva"
              label="Nueva contraseña"
              :type="ver.nueva ? 'text' : 'password'"
              :append-inner-icon="ver.nueva ? 'mdi-eye-off' : 'mdi-eye'"
              autocomplete="new-password"
              :rules="[req, min8, distinta]"
              @click:append-inner="ver.nueva = !ver.nueva"
            />

            <v-progress-linear :model-value="fuerza.valor" :color="fuerza.color" height="6" rounded class="mb-1" />
            <div class="text-caption mb-4" :class="`text-${fuerza.color}`">{{ fuerza.texto }}</div>

            <v-text-field
              v-model="form.confirmar"
              label="Confirmar nueva contraseña"
              :type="ver.nueva ? 'text' : 'password'"
              autocomplete="new-password"
              :rules="[req, coincide]"
            />

            <v-btn type="submit" color="primary" prepend-icon="mdi-content-save" :loading="guardando">
              Actualizar contraseña
            </v-btn>
          </v-form>
        </v-card-text>
      </v-card>
    </v-col>
  </v-row>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { perfilApi } from '@/api/perfil'
import { mensajeError } from '@/api/http'
import { useAuth } from '@/stores/auth'
import { useNotificaciones } from '@/stores/notificaciones'
import { etiquetaRol, colorRol } from '@/utils/roles'

const auth = useAuth()
const noti = useNotificaciones()

const ETIQUETA_TIPO = { alumnos: 'Alumno', maestros: 'Maestro', admins: 'Administrador' }

const perfil = ref({})
const cargando = ref(false)
const guardando = ref(false)
const formRef = ref(null)
const form = reactive({ actual: '', nueva: '', confirmar: '' })
const ver = reactive({ actual: false, nueva: false })

const iniciales = computed(() =>
  (perfil.value.nombre || auth.usuario?.nombre || '?')
    .split(' ').filter(Boolean).slice(0, 2).map((p) => p[0]).join('').toUpperCase()
)

const req = (v) => !!v || 'Campo obligatorio'
const min8 = (v) => (v && v.length >= 8) || 'Mínimo 8 caracteres'
const distinta = (v) => v !== form.actual || 'Debe ser distinta a la actual'
const coincide = (v) => v === form.nueva || 'Las contraseñas no coinciden'

// Indicador simple de fuerza
const fuerza = computed(() => {
  const p = form.nueva || ''
  let puntos = 0
  if (p.length >= 8) puntos++
  if (p.length >= 12) puntos++
  if (/[a-z]/.test(p) && /[A-Z]/.test(p)) puntos++
  if (/\d/.test(p)) puntos++
  if (/[^A-Za-z0-9]/.test(p)) puntos++
  if (!p) return { valor: 0, color: 'grey', texto: 'Usa 8+ caracteres, mayúsculas, números y símbolos' }
  if (puntos <= 2) return { valor: 33, color: 'error', texto: 'Débil' }
  if (puntos <= 3) return { valor: 66, color: 'warning', texto: 'Aceptable' }
  return { valor: 100, color: 'success', texto: 'Fuerte' }
})

const cargar = async () => {
  cargando.value = true
  try {
    perfil.value = await perfilApi.obtener()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo cargar tu perfil'))
  } finally {
    cargando.value = false
  }
}

const guardar = async () => {
  const { valid } = await formRef.value.validate()
  if (!valid) return
  guardando.value = true
  try {
    await perfilApi.cambiarPassword(form.actual, form.nueva)
    noti.exito('Contraseña actualizada')
    formRef.value.reset()
  } catch (e) {
    noti.error(mensajeError(e, 'No se pudo actualizar la contraseña'))
  } finally {
    guardando.value = false
  }
}

onMounted(cargar)
</script>