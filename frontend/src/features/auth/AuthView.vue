<script setup lang="ts">
import { computed, ref } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../../stores/auth'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const mode = ref<'login' | 'register'>(route.path === '/register' ? 'register' : 'login')
const email = ref('')
const name = ref('')
const password = ref('')
const confirmPassword = ref('')
const error = ref('')

const title = computed(() => mode.value === 'login' ? 'Sign in to Web Studio' : 'Create your Web Studio account')

function switchMode(next: 'login' | 'register') {
  mode.value = next; error.value = ''; router.replace(next === 'login' ? '/login' : '/register')
}

async function submit() {
  error.value = ''
  if (!email.value.trim() || !password.value) { error.value = 'Email and password are required'; return }
  if (mode.value === 'register' && !name.value.trim()) { error.value = 'Name is required'; return }
  if (mode.value === 'register' && password.value !== confirmPassword.value) { error.value = 'Passwords do not match'; return }
  try {
    if (mode.value === 'login') await auth.login(email.value, password.value)
    else await auth.register(email.value, name.value, password.value)
    const redirect = typeof route.query.redirect === 'string' ? route.query.redirect : '/projects'
    await router.replace(redirect)
  } catch (err) { error.value = err instanceof Error ? err.message : 'Authentication failed'; ElMessage.error(error.value) }
}
</script>

<template>
  <main class="auth-page">
    <el-card class="auth-card">
      <div class="brand">Web Studio</div>
      <h1>{{ title }}</h1>
      <p class="muted">Your projects and creative assets are private to your account.</p>
      <el-alert v-if="error" :title="error" type="error" show-icon class="error" />
      <el-form label-position="top" @submit.prevent="submit">
        <el-form-item label="Email"><el-input v-model="email" type="email" autocomplete="email" /></el-form-item>
        <el-form-item v-if="mode === 'register'" label="Name"><el-input v-model="name" autocomplete="name" /></el-form-item>
        <el-form-item label="Password"><el-input v-model="password" type="password" show-password autocomplete="current-password" /></el-form-item>
        <el-form-item v-if="mode === 'register'" label="Confirm password"><el-input v-model="confirmPassword" type="password" show-password autocomplete="new-password" /></el-form-item>
        <el-button type="primary" native-type="submit" :loading="auth.loading" class="submit">{{ mode === 'login' ? 'Sign in' : 'Create account' }}</el-button>
      </el-form>
      <div class="switch">
        <span v-if="mode === 'login'">No account?</span><el-button v-if="mode === 'login'" link type="primary" @click="switchMode('register')">Create one</el-button>
        <span v-else>Already have an account?</span><el-button v-if="mode === 'register'" link type="primary" @click="switchMode('login')">Sign in</el-button>
      </div>
    </el-card>
  </main>
</template>

<style scoped>
.auth-page { min-height: 100vh; display: grid; place-items: center; padding: 24px; background: var(--el-bg-color-page); }
.auth-card { width: min(440px, 100%); }
.brand { font-weight: 700; font-size: 18px; }
h1 { margin: 12px 0 8px; }
.muted { color: var(--el-text-color-secondary); margin-bottom: 24px; }
.error { margin-bottom: 16px; }
.submit { width: 100%; }
.switch { margin-top: 20px; display: flex; justify-content: center; gap: 4px; align-items: center; }
</style>