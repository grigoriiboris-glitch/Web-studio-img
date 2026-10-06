import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const apiProxyTarget = (env.VITE_API_PROXY_TARGET ?? process.env.VITE_API_PROXY_TARGET)?.trim()

  return {
    plugins: [vue()],
    server: {
      port: 5173,
      host: '0.0.0.0',
      proxy: apiProxyTarget ? { '/api': { target: apiProxyTarget, changeOrigin: true } } : undefined,
    },
    test: { environment: 'jsdom' },
  }
})
