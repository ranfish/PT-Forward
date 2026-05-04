import { defineStore } from 'pinia'
import { ref } from 'vue'
import { authApi } from '@/api/auth'
import router from '@/router'

export const useAuthStore = defineStore('auth', () => {
  const accessToken = ref(localStorage.getItem('pt-forward-access-token') || '')
  const isLoggedIn = ref(!!accessToken.value)

  async function login(username: string, password: string) {
    const resp = await authApi.login({ username, password })
    const data = resp.data.data
    accessToken.value = data.accessToken
    isLoggedIn.value = true
    localStorage.setItem('pt-forward-access-token', data.accessToken)
    localStorage.setItem('pt-forward-refresh-token', data.refreshToken)
  }

  function logout() {
    accessToken.value = ''
    isLoggedIn.value = false
    localStorage.removeItem('pt-forward-access-token')
    localStorage.removeItem('pt-forward-refresh-token')
    router.push({ name: 'Login' })
  }

  return { accessToken, isLoggedIn, login, logout }
})
