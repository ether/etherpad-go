import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'
import tailwindcss from '@tailwindcss/vite'
import path from 'node:path'

export default defineConfig({
  plugins: [react(), tailwindcss()],
  resolve: {
    alias: {
      '@': path.resolve(__dirname, './src'),
    },
  },
  build: {
    outDir: '../assets/js/admin',
    emptyOutDir: true,
  },
  server: {
    proxy: {
      '/admin/ws': { target: 'http://localhost:9001', ws: true },
      '/admin/api': 'http://localhost:9001',
      '/admin/validate': 'http://localhost:9001',
    },
  },
})
