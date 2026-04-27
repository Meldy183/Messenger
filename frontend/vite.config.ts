import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

// auth-service: port 8080, chat-service: port 8081 (matches service configs)
export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/api/v1/auth': {
        target: 'http://localhost:8080',
        changeOrigin: true,
      },
      '/api/v1': {
        target: 'http://localhost:8081',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8081',
        ws: true,
      },
    },
  },
})
