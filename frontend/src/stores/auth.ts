import { computed, ref } from 'vue'
import { defineStore } from 'pinia'
import { authApi, type AuthUser } from '../api/client'

const TOKEN_KEY = 'web-studio-access-token'

export const useAuthStore = defineStore('auth', () => {
  const user = ref<AuthUser | null>(null)
  const initialized = ref(false)
  const loading = ref(false)

  const isAuthenticated = computed(() => Boolean(user.value))

  async function initialize() {
    if (initialized.value) return
    const token = localStorage.getItem(TOKEN_KEY)
    if (!token) { initialized.value = true; return }
    try { user.value = await authApi.me() } catch { localStorage.removeItem(TOKEN_KEY); user.value = null }
    finally { initialized.value = true }
  }

  async function login(email: string, password: string) {
    loading.value = true
    try { const session = await authApi.login(email, password); localStorage.setItem(TOKEN_KEY, session.token); user.value = session.user; initialized.value = true; return session.user }
    finally { loading.value = false }
  }

  async function register(email: string, name: string, password: string) {
    loading.value = true
    try { const session = await authApi.register(email, name, password); localStorage.setItem(TOKEN_KEY, session.token); user.value = session.user; initialized.value = true; return session.user }
    finally { loading.value = false }
  }

  async function logout() {
    loading.value = true
    try { if (localStorage.getItem(TOKEN_KEY)) await authApi.logout() } catch { /* local logout still clears the client session */ }
    finally { localStorage.removeItem(TOKEN_KEY); user.value = null; initialized.value = true; loading.value = false }
  }

  return { user, initialized, loading, isAuthenticated, initialize, login, register, logout }
})