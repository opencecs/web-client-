import { defineConfig } from 'vite'
import vue from '@vitejs/plugin-vue'
import AutoImport from 'unplugin-auto-import/vite'
import Components from 'unplugin-vue-components/vite'
import { ElementPlusResolver } from 'unplugin-vue-components/resolvers'
import { VantResolver } from '@vant/auto-import-resolver'

export default defineConfig({
  plugins: [
    vue(),
    // Element Plus + Vant 按需自动导入
    AutoImport({
      resolvers: [ElementPlusResolver(), VantResolver()],
    }),
    Components({
      resolvers: [ElementPlusResolver(), VantResolver()],
    }),
  ],
  server: {
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://localhost:8181',
        timeout: 600000, // 10分钟，大文件上传需要
      },
      '/ws': { target: 'ws://localhost:8181', ws: true },
      '/lgcloud': { target: 'ws://localhost:8181', ws: true }
    }
  },
  build: {
    outDir: 'dist',
    emptyOutDir: true,
    rollupOptions: {
      output: {
        // Vendor 分包：拆分大型第三方库
        manualChunks: {
          'vue-vendor': ['vue', 'vue-router', 'pinia'],
          'element-plus': ['element-plus'],
          'vant': ['vant'],
          'xterm': ['xterm', '@xterm/addon-fit'],
        }
      }
    }
  }
})
