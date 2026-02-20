import { fileURLToPath, URL } from 'node:url'

import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import vueDevTools from 'vite-plugin-vue-devtools'

// https://vite.dev/config/
export default defineConfig({
  base: process.env.VITE_BASE_PATH || '/',
  plugins: [
    vue(),
    vueDevTools(),
  ],
  resolve: {
    alias: {
      '@': fileURLToPath(new URL('./src', import.meta.url))
    },
  },
  server: {
    // 开发时将常用后端路由代理到本地后端，避免跨域或错发到 Vite dev server
    proxy: {
      // Yggdrasil / auth APIs
      '^/authserver': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        rewrite: (path) => path,
      },
      // Session APIs
      '^/sessionserver': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        rewrite: (path) => path,
      },
      // API routes that might conflict with frontend routes
      // When a browser refreshes on these paths, it should serve index.html instead of proxying to the backend
      '^/(admin|register|reset-password|site-login|me|public|microsoft|textures|send-verification-code)': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
        bypass: (req) => {
          if (req.headers.accept?.indexOf('text/html') !== -1) {
            return '/index.html';
          }
        }
      },
      '^/static/textures': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
      '^/static/carousel': {
        target: 'http://127.0.0.1:8000',
        changeOrigin: true,
      },
    }
  }
})
