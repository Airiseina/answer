import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    port: 3000,
    proxy: {
      '/ws': {
        target: 'ws://localhost:8082',
        ws: true,
      },
      '/login': 'http://localhost:1234',
      '/register': 'http://localhost:1234',
      '/api': 'http://localhost:1234',
    }
  }
})
