<template>
  <div class="mobile-app">
    <!-- 已登录：TabBar 布局 -->
    <template v-if="auth.isLoggedIn">
      <!-- 全局连接状态条 -->
      <transition name="slide-down">
        <div v-if="showConnBar" class="conn-bar" :class="connBarClass">
          <van-icon v-if="!device.online" name="signal" size="15" color="#e6a23c" />
          <van-icon v-else name="checked" size="15" color="#67c23a" />
          <span>{{ connBarText }}</span>
        </div>
      </transition>

      <div class="mobile-content">
        <router-view />
      </div>
      <!-- 底部 TabBar -->
      <van-tabbar v-if="showTabbar" v-model="activeTab" @change="onTabChange" fixed placeholder safe-area-inset-bottom>
        <van-tabbar-item name="/m" icon="bar-chart-o">概览</van-tabbar-item>
        <van-tabbar-item name="/m/android" icon="phone-o">云机</van-tabbar-item>
        <van-tabbar-item name="/m/manage" icon="setting-o">管理</van-tabbar-item>
        <van-tabbar-item name="/m/profile" icon="contact-o">我的</van-tabbar-item>
      </van-tabbar>
    </template>
    <!-- 未登录：直接显示路由 -->
    <router-view v-else />
  </div>
</template>

<script setup>
import { ref, computed, watch, onMounted, onBeforeUnmount } from 'vue'
import { useRouter, useRoute } from 'vue-router'
import { useAuthStore } from './stores/auth.js'
import { useDeviceStore } from './stores/device.js'
import { checkIsMobile } from './utils/isMobile.js'

const router = useRouter()
const route = useRoute()
const auth = useAuthStore()
const device = useDeviceStore()

const activeTab = ref('/m')
const showTabbar = computed(() => route.meta.tabbar === true)

watch(() => route.path, (path) => {
  const tabPaths = ['/m', '/m/android', '/m/manage', '/m/profile']
  if (tabPaths.includes(path)) {
    activeTab.value = path
  }
}, { immediate: true })

function onTabChange(name) {
  router.push(name)
}

// === 连接状态 ===
const wasOnline = ref(false)
const justReconnected = ref(false)
let reconnectedTimer = null

const showConnBar = computed(() => {
  if (!auth.isLoggedIn) return false
  // 未连接 或 刚恢复连接（短暂显示"已连接"）
  return !device.online || justReconnected.value
})

const connBarClass = computed(() => {
  if (justReconnected.value) return 'connected'
  return 'connecting'
})

const connBarText = computed(() => {
  if (justReconnected.value) return '已连接'
  return '正在连接服务器...'
})

// 监听连接状态变化
watch(() => device.online, (online) => {
  if (online && wasOnline.value === false) {
    // 从离线恢复 → 短暂显示"已连接"
    justReconnected.value = true
    if (reconnectedTimer) clearTimeout(reconnectedTimer)
    reconnectedTimer = setTimeout(() => { justReconnected.value = false }, 2000)
  }
  wasOnline.value = online
})

// 监听设备类型变化：手机↔PC切换时自动刷新
let lastMobile = true
let uaPollTimer = null

function checkAndReload() {
  const nowMobile = checkIsMobile()
  if (nowMobile !== lastMobile) {
    window.location.reload()
  }
}

function onResize() {
  checkAndReload()
}

function onStorage(e) {
  if (e.key === 'force_platform') {
    checkAndReload()
  }
}

onMounted(() => {
  if (auth.isLoggedIn) {
    device.connect()
  }
  lastMobile = checkIsMobile()
  window.addEventListener('resize', onResize)
  window.addEventListener('storage', onStorage)
  uaPollTimer = setInterval(checkAndReload, 1000)
})

onBeforeUnmount(() => {
  if (reconnectedTimer) clearTimeout(reconnectedTimer)
  window.removeEventListener('resize', onResize)
  window.removeEventListener('storage', onStorage)
  if (uaPollTimer) clearInterval(uaPollTimer)
})
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  background: #0a0a0a;
  color: #e0e0e0;
}
html.dark {
  color-scheme: dark;
}
</style>

<style scoped>
.mobile-app {
  min-height: 100vh;
  background: #0a0a0a;
}

.mobile-content {
  min-height: 100vh;
}

/* 连接状态条 - 右侧小胶囊 */
.conn-bar {
  position: fixed;
  top: calc(10px + constant(safe-area-inset-top));
  top: calc(10px + env(safe-area-inset-top));
  right: 12px;
  z-index: 9998;
  display: flex;
  align-items: center;
  gap: 6px;
  padding: 6px 12px;
  font-size: 12px;
  font-weight: 500;
  color: #e0e0e0;
  border-radius: 20px;
  backdrop-filter: blur(16px);
  -webkit-backdrop-filter: blur(16px);
  box-shadow: 0 2px 12px rgba(0, 0, 0, 0.3);
  transition: all 0.3s ease;
}
.conn-bar.connecting {
  background: rgba(230, 162, 60, 0.2);
  border: 1px solid rgba(230, 162, 60, 0.35);
  animation: pulse-conn 2s ease-in-out infinite;
}
.conn-bar.connected {
  background: rgba(103, 194, 58, 0.2);
  border: 1px solid rgba(103, 194, 58, 0.35);
}
@keyframes pulse-conn {
  0%, 100% { opacity: 1; }
  50% { opacity: 0.65; }
}

.slide-down-enter-active, .slide-down-leave-active {
  transition: transform 0.3s, opacity 0.3s;
}
.slide-down-enter-from, .slide-down-leave-to {
  transform: translateY(-100%);
  opacity: 0;
}
</style>
