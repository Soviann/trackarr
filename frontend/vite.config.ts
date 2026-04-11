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
        manualChunks: (id) => {
          if (id.includes('/pages/Admin')) return 'admin'
          if (id.includes('/pages/MatchReview') || id.includes('/components/MatchReviewCard')) return 'match-review'
          if (id.includes('/pages/Stats')) return 'stats'
          if (id.includes('/components/FilterDrawer')) return 'filter-drawer'
        },
      },
    },
  },
})
