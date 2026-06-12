/// <reference types="vitest" />
import { defineConfig } from 'vitest/config'
import vue from '@vitejs/plugin-vue'
import { resolve } from 'path'
import pkg from './package.json'

export default defineConfig({
  define: {
    __APP_VERSION__: JSON.stringify(pkg.version),
  },
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
      'vue-i18n': 'vue-i18n/dist/vue-i18n.mjs',
    },
  },
  plugins: [
    vue(),
  ],
  test: {
    environment: 'happy-dom',
    include: ['src/**/*.{test,spec}.{ts,tsx}'],
  },
  css: {
    preprocessorOptions: {
      less: {
        javascriptEnabled: true,
      },
    },
  },
  build: {
    rollupOptions: {
      output: {
        manualChunks(id) {
          if (id.includes('node_modules/@ant-design/icons-vue')) return 'vendor-icons'
          if (id.includes('node_modules/ant-design-vue')) return 'vendor-antd'
          if (id.includes('node_modules/echarts')) return 'vendor-echarts'
          if (id.includes('node_modules/vue')) return 'vendor-vue'
          if (id.includes('node_modules/axios')) return 'vendor-libs'
          if (id.includes('node_modules/vue-router')) return 'vendor-libs'
          if (id.includes('node_modules/vue-i18n')) return 'vendor-libs'
          if (id.includes('node_modules')) return 'vendor-libs'
        },
      },
    },
    chunkSizeWarningLimit: 1500,
  },
  server: {
    port: 5173,
    proxy: {
      '/api': {
        target: 'http://localhost:8765',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://localhost:8765',
        ws: true,
      },
    },
  },
})
