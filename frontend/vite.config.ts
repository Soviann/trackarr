import { defineConfig } from 'vite'
import preact from '@preact/preset-vite'

export default defineConfig({
  plugins: [preact({ devToolsEnabled: false })],
  cacheDir: '/tmp/vite-cache',
  server: {
    host: '0.0.0.0',
    port: 5174,
    proxy: {
      '/api': 'http://localhost:8080',
    },
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        manualChunks: {
          admin: ['./src/pages/Admin.tsx', './src/pages/AdminNotifications.tsx', './src/pages/AdminTasks.tsx'],
        },
      },
    },
  },
})
