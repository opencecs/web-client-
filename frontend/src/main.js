import { createApp } from 'vue'
import { createPinia } from 'pinia'
import { isMobile } from './utils/isMobile.js'

async function bootstrap() {
  if (isMobile) {
    // 移动端：加载 Vant + 暗色主题
    const [{ default: App }, { default: router }] = await Promise.all([
      import('./AppMobile.vue'),
      import('./router/index.js'),
    ])
    await import('vant/lib/index.css')
    await import('./mobile-theme.css')

    const app = createApp(App)
    app.use(createPinia())
    app.use(router)
    app.mount('#app')
  } else {
    // 桌面端：加载 Element Plus（原逻辑）
    const [{ default: App }, { default: router }] = await Promise.all([
      import('./App.vue'),
      import('./router/index.js'),
    ])
    await import('element-plus/theme-chalk/dark/css-vars.css')

    const app = createApp(App)
    app.use(createPinia())
    app.use(router)
    app.mount('#app')
  }
}

bootstrap()
