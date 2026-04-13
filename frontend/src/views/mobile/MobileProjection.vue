<template>
  <div class="mobile-projection">
    <!-- 顶部状态栏 -->
    <div class="projection-top-bar">
      <van-icon name="arrow-left" size="22" color="#fff" @click="goBack" />
      <span class="top-title">{{ displayName }}</span>
      <span class="latency" :style="{ color: latencyColor }">{{ latencyText }}</span>
    </div>

    <!-- 投屏画面 -->
    <div class="projection-screen">
      <iframe v-if="playerUrl" ref="iframeRef" :src="playerUrl" class="projection-iframe"
        allowfullscreen allow="autoplay *; fullscreen; microphone; camera; display-capture; screen-wake-lock"
        loading="eager" importance="high" />
      <div v-else class="loading-hint">
        <van-loading size="36" color="#409eff" />
        <span>正在连接...</span>
      </div>
    </div>

    <!-- 安卓导航栏 -->
    <div class="android-nav-bar">
      <button class="nav-btn" @click="sendCmd('goClean')">□</button>
      <button class="nav-btn home" @click="sendCmd('goHome')">○</button>
      <button class="nav-btn" @click="sendCmd('goBack')">◁</button>
    </div>

    <!-- 浮动工具按钮 -->
    <div class="fab" @click="showTools = true">
      <van-icon name="more-o" size="24" color="#fff" />
    </div>

    <!-- 工具操作面板 -->
    <van-action-sheet v-model:show="showTools" title="工具" :actions="toolActions"
      @select="onToolAction" cancel-text="取消" close-on-click-action />

    <!-- 短信弹窗 -->
    <van-dialog v-model:show="showSms" title="模拟短信" show-cancel-button @confirm="doSendSms">
      <div style="padding: 16px">
        <van-field v-model="smsAddress" label="号码" placeholder="发送号码" />
        <van-field v-model="smsBody" label="内容" placeholder="短信内容" type="textarea" rows="3" />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showToast } from 'vant'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()

const containerName = computed(() => route.params.name)
const displayName = computed(() => device.displayName(containerName.value))

const iframeRef = ref(null)
const playerUrl = ref('')
const showTools = ref(false)
const showSms = ref(false)
const smsAddress = ref('')
const smsBody = ref('')
const isMuted = ref(false)
let wakeLock = null
let loading = false

// 延迟监测
const latencyMs = ref(-1)
let latencyTimer = null

const latencyText = computed(() => latencyMs.value < 0 ? '--ms' : latencyMs.value + 'ms')
const latencyColor = computed(() => {
  if (latencyMs.value < 0) return '#666'
  if (latencyMs.value < 30) return '#67c23a'
  if (latencyMs.value < 80) return '#e6a23c'
  return '#f56c6c'
})

const toolActions = computed(() => [
  { name: isMuted.value ? '取消静音' : '静音', value: 'toggleMute' },
  { name: '音量+', value: 'volUp' },
  { name: '音量-', value: 'volDown' },
  { name: '摇一摇', value: 'shake' },
  { name: '模拟短信', value: 'sms' },
])

function sendCmd(action) {
  if (iframeRef.value?.contentWindow) {
    iframeRef.value.contentWindow.postMessage({ action }, '*')
  }
}

function onToolAction(action) {
  if (action.value === 'sms') { showSms.value = true; return }
  if (action.value === 'shake') {
    device.request('android:shake', { name: containerName.value }).catch(() => {})
    return
  }
  if (action.value === 'toggleMute') {
    sendCmd('toggleMute')
    return
  }
  sendCmd(action.value)
}

function doSendSms() {
  if (!smsAddress.value || !smsBody.value) return
  device.request('android:sms', {
    name: containerName.value, address: smsAddress.value, body: smsBody.value
  }).catch(() => {})
  smsAddress.value = ''; smsBody.value = ''
}

function goBack() { router.back() }

async function loadPlayer() {
  if (loading || playerUrl.value) return
  loading = true
  try {
    const resp = await device.requestProjectionToken(containerName.value)
    const projToken = typeof resp === 'string' ? resp : resp?.token
    const udpPort = typeof resp === 'object' ? resp?.udpPort : ''
    const currentPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
    const token = projToken || ((auth.token || '') + ':' + (containerName.value || ''))
    const params = new URLSearchParams({
      shost: window.location.hostname,
      sport: currentPort,
      q: '1', v: 'h264',
      rtc_i: window.location.hostname,
      rtc_p: udpPort || currentPort,
      container_name: containerName.value || '',
      token
    })
    playerUrl.value = `/webplayer/play.html?${params.toString()}`
  } catch {
    showToast('投屏连接失败')
  } finally {
    loading = false
  }
}

function onParentMessage(e) {
  if (e.data?.type === 'latency') latencyMs.value = e.data.rtt ?? -1
  if (e.data?.type === 'muteState') isMuted.value = !!e.data.muted
}

function startLatencyPoll() {
  window.addEventListener('message', onParentMessage)
  latencyTimer = setInterval(() => {
    if (iframeRef.value?.contentWindow) {
      try { iframeRef.value.contentWindow.postMessage({ action: 'getLatency' }, '*') } catch {}
    }
    if (latencyMs.value < 0) {
      const start = performance.now()
      device.request('panel:version', {}, 5000).then(() => {
        latencyMs.value = Math.round(performance.now() - start)
      }).catch(() => {})
    }
  }, 3000)
}

// Screen Wake Lock - 防止投屏时手机熄屏
async function requestWakeLock() {
  if (!('wakeLock' in navigator)) return
  try {
    wakeLock = await navigator.wakeLock.request('screen')
    wakeLock.addEventListener('release', () => { wakeLock = null })
  } catch {}
}
function onVisibilityChange() {
  if (document.visibilityState === 'visible' && !wakeLock) requestWakeLock()
}

onMounted(() => {
  if (device.online) {
    loadPlayer()
    startLatencyPoll()
  }
  requestWakeLock()
  document.addEventListener('visibilitychange', onVisibilityChange)
})

watch(() => device.online, (v) => {
  if (v && !playerUrl.value) {
    loadPlayer()
    startLatencyPoll()
  }
})

onBeforeUnmount(() => {
  window.removeEventListener('message', onParentMessage)
  document.removeEventListener('visibilitychange', onVisibilityChange)
  if (latencyTimer) clearInterval(latencyTimer)
  if (wakeLock) { wakeLock.release().catch(() => {}); wakeLock = null }
  if (iframeRef.value?.contentWindow) {
    try { iframeRef.value.contentWindow.globalCleanup?.() } catch {}
  }
})
</script>

<style scoped>
.mobile-projection {
  position: fixed;
  inset: 0;
  background: #000;
  display: flex;
  flex-direction: column;
  z-index: 9999;
  /* GPU 加速整个投屏容器 */
  transform: translateZ(0);
  -webkit-transform: translateZ(0);
  will-change: transform;
  -webkit-overflow-scrolling: touch;
}

.projection-top-bar {
  display: flex;
  align-items: center;
  padding: 8px 12px;
  padding-top: calc(12px + constant(safe-area-inset-top));
  padding-top: calc(12px + env(safe-area-inset-top));
  background: rgba(20, 20, 20, 0.9);
  backdrop-filter: blur(8px);
  gap: 12px;
  flex-shrink: 0;
  min-height: 44px;
  box-sizing: border-box;
}
.top-title {
  flex: 1;
  color: #e0e0e0;
  font-size: 14px;
  font-weight: 600;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.latency {
  font-size: 12px;
  font-family: monospace;
  font-weight: bold;
}

.projection-screen {
  flex: 1;
  overflow: hidden;
  /* GPU 加速视频区域 */
  transform: translateZ(0);
  -webkit-transform: translateZ(0);
  contain: strict;
}
.projection-iframe {
  width: 100%;
  height: 100%;
  border: none;
  /* GPU 合成层 */
  transform: translateZ(0);
  -webkit-transform: translateZ(0);
  will-change: contents;
}
.loading-hint {
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  height: 100%;
  gap: 12px;
  color: #666;
  font-size: 14px;
}

.android-nav-bar {
  display: flex;
  justify-content: space-around;
  align-items: center;
  height: 44px;
  background: rgba(20, 20, 20, 0.85);
  backdrop-filter: blur(8px);
  flex-shrink: 0;
  padding-bottom: constant(safe-area-inset-bottom);
  padding-bottom: env(safe-area-inset-bottom);
}
.nav-btn {
  background: none;
  border: none;
  color: #aaa;
  font-size: 20px;
  width: 56px;
  height: 40px;
  border-radius: 8px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.nav-btn:active { background: rgba(255, 255, 255, 0.1); color: #fff; }

.fab {
  position: fixed;
  right: 16px;
  bottom: 100px;
  width: 48px;
  height: 48px;
  border-radius: 50%;
  background: rgba(64, 158, 255, 0.85);
  display: flex;
  align-items: center;
  justify-content: center;
  box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
  z-index: 10000;
}
.fab:active { transform: scale(0.9); }

.fade-enter-active, .fade-leave-active { transition: opacity 0.3s; }
.fade-enter-from, .fade-leave-to { opacity: 0; }
</style>
