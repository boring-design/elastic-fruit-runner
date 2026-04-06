import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  server: {
    proxy: {
      '/controlplane.v1.ControlPlaneService': {
        target: 'http://localhost:8080',
      },
    },
  },
})
