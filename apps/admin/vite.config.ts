import { fileURLToPath, URL } from 'node:url'

import { defineConfig, loadEnv } from 'vite'
import vue from '@vitejs/plugin-vue'

// https://vite.dev/config/
export default defineConfig(({ mode }) => {
  const env = loadEnv(mode, process.cwd(), '')
  const port = Number(env.VITE_PORT || '80')
  const host = env.VITE_HOST || '0.0.0.0'
  const server = {
    host,
    port: Number.isFinite(port) && port > 0 ? port : 5173,
  }

  return {
    plugins: [vue()],
    resolve: {
      alias: {
        '@': fileURLToPath(new URL('./src', import.meta.url)),
      },
    },
    server,
    preview: server,
    build: {
      // Emit a single JavaScript bundle (inline all dynamic imports —
      // route-level lazy views and the async value editor).
      chunkSizeWarningLimit: 5000,
      rollupOptions: {
        output: {
          inlineDynamicImports: true,
        },
      },
    },
  }
})
