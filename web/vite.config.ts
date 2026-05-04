import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import VueI18nPlugin from '@intlify/unplugin-vue-i18n/vite'
import { resolve } from 'path'

export default defineConfig({
  plugins: [
    vue(),
    VueI18nPlugin({
      include: resolve(__dirname, './src/locales/**'),
    }),
  ],
  resolve: {
    alias: {
      '@': resolve(__dirname, './src'),
    },
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
          if (id.includes('node_modules')) return 'vendor-core'
        },
      },
    },
    chunkSizeWarningLimit: 600,
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
