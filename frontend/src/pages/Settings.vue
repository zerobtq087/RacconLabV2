<template>
  <AppFrame>
    <v-row class="my-2">
      <v-col cols="12" md="5">
        <v-card border class="h-100" elevation="0" rounded="lg" title="Mi cuenta">
          <template #text>
            <v-list-item class="px-0" prepend-icon="mdi-card-account-details" subtitle="Matrícula" :title="authStore.userAcc.matricula" />
            <v-list-item class="px-0" prepend-icon="mdi-account" subtitle="Nombre" :title="authStore.userAcc.nombre" />
            <v-list-item v-if="authStore.userAcc.grupo" class="px-0" prepend-icon="mdi-account-group" subtitle="Grupo" :title="authStore.userAcc.grupo" />
            <v-list-item class="px-0" prepend-icon="mdi-account-key" subtitle="Roles">
              <template #title>
                <v-chip v-for="rol in authStore.userAcc.roles" :key="rol" class="me-1" label size="x-small" :text="rol" />
              </template>
            </v-list-item>
          </template>
        </v-card>
      </v-col>

      <v-col cols="12" md="7">
        <v-card border class="h-100" elevation="0" rounded="lg" title="Cambiar contraseña">
          <template #text>
            <v-text-field
              v-model="form.currentPasswd"
              autocomplete="current-password"
              class="mb-2"
              label="Contraseña actual"
              type="password"
              variant="outlined"
            />
            <v-text-field
              v-model="form.newPasswd"
              autocomplete="new-password"
              class="mb-2"
              hint="Mínimo 8 caracteres"
              label="Nueva contraseña"
              persistent-hint
              type="password"
              variant="outlined"
            />
            <v-text-field
              v-model="form.confirmPasswd"
              autocomplete="new-password"
              :error-messages="noCoinciden ? 'No coincide con la nueva contraseña' : ''"
              label="Confirmar nueva contraseña"
              type="password"
              variant="outlined"
            />
          </template>
          <template #actions>
            <v-spacer />
            <v-btn color="primary" :disabled="!valido" :loading="guardando" text="Cambiar contraseña" @click="save" />
          </template>
        </v-card>
      </v-col>
    </v-row>
  </AppFrame>
</template>

<script lang="ts" setup>
import AppFrame from '../components/AppFrame.vue'
import { computed, reactive, ref } from 'vue'
import axios from 'axios'
import Swal from 'sweetalert2'
import { useAuthStore } from '@/stores/auth'

const authStore = useAuthStore()

const form = reactive({ currentPasswd: '', newPasswd: '', confirmPasswd: '' })
const guardando = ref(false)

const noCoinciden = computed(() => form.confirmPasswd != '' && form.confirmPasswd != form.newPasswd)
const valido = computed(() => form.currentPasswd != '' && form.newPasswd.length >= 8 && form.newPasswd == form.confirmPasswd)

function save () {
  guardando.value = true
  axios.post('/api/update-passwd', form)
    .then(() => {
      form.currentPasswd = ''
      form.newPasswd = ''
      form.confirmPasswd = ''
      Swal.fire({ icon: 'success', title: 'Contraseña cambiada', text: 'Se cerraron tus sesiones en otros equipos.' })
    })
    .catch(err => {
      Swal.fire({ icon: 'error', title: 'Error', text: err.response?.data || 'No se pudo cambiar la contraseña.' })
    })
    .finally(() => guardando.value = false)
}
</script>
