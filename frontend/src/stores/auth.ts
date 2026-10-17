// Utilities
import { UserAccount } from '../models/UserAccount'
import axios from 'axios'
import { defineStore } from 'pinia'
import { ref } from 'vue'

export const useAuthStore = defineStore('auth', () => {
  const userAcc = ref<UserAccount>(new UserAccount())
  const isLoading = ref(false)
  const error = ref<string | null>(null)

  const fetchProfile = async () => {
    isLoading.value = true
    try {
      const response = await axios.get('/api/current-user-acc')
      // Solo es válido si de verdad llegó un usuario (con id)
      userAcc.value = response.data?.id ? response.data : new UserAccount()
    } catch {
      userAcc.value = new UserAccount()
      error.value = 'No autorizado'
    } finally {
      isLoading.value = false
    }
  }

  const clear = () => {
    userAcc.value = new UserAccount()
  }

  return { userAcc, fetchProfile, clear, isLoading, error }
})
