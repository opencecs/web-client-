<template>
  <div v-if="modelValue && container" class="projection-float" :style="floatStyle"
    @mousedown.stop="onDragStart">
    <!-- 标题栏（可拖动） -->
    <div class="projection-header" @mousedown.stop="onDragStart">
      <span>{{ device.displayName(container.name) }} - 坑位 {{ container.indexNum }}</span>
      <div class="projection-header-btns">
        <span class="projection-btn" title="关闭" @mousedown.stop @click="closeProjection">✕</span>
      </div>
    </div>
    <!-- 中间区域：投屏 + 右侧工具栏 -->
    <div class="projection-main">
      <div class="projection-body">
        <iframe v-if="playerUrl" ref="iframeRef" :src="playerUrl" class="projection-iframe"
          allowfullscreen allow="autoplay *; fullscreen; microphone; camera; display-capture" disablepictureinpicture />
      </div>
      <!-- 右侧工具栏 -->
      <div class="projection-sidebar" @mousedown.stop>
        <div class="sidebar-latency" :style="{ color: latencyColor }">{{ latencyText }}</div>
        <div class="sidebar-divider"></div>
        <button class="sidebar-btn" title="音量+" @click="sendCmd('volUp')">🔊<span>音量+</span></button>
        <button class="sidebar-btn" title="音量-" @click="sendCmd('volDown')">🔉<span>音量-</span></button>
        <div class="sidebar-divider"></div>
        <button class="sidebar-btn" title="摇一摇" @click="doShake">📳<span>摇一摇</span></button>
        <button class="sidebar-btn" title="模拟短信" @click="showSmsDialog = true">💬<span>短信</span></button>
        <div class="sidebar-divider"></div>
        <label class="sidebar-btn" title="上传文件（APK自动安装）">
          📁<span>上传</span><input type="file" style="display:none" @change="doUpload" ref="uploadInput" />
        </label>
      </div>
    </div>
    <!-- 安卓导航按钮 -->
    <div class="projection-footer">
      <button class="android-btn" title="最近任务" @mousedown.stop @click="sendCmd('goClean')">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><rect x="2" y="2" width="12" height="12" rx="1.5" stroke="currentColor" stroke-width="1.6"/></svg>
      </button>
      <button class="android-btn" title="主页" @mousedown.stop @click="sendCmd('goHome')">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><circle cx="8" cy="8" r="6" stroke="currentColor" stroke-width="1.6"/></svg>
      </button>
      <button class="android-btn" title="返回" @mousedown.stop @click="sendCmd('goBack')">
        <svg width="16" height="16" viewBox="0 0 16 16" fill="none"><path d="M10 3L5 8l5 5" stroke="currentColor" stroke-width="1.6" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </button>
    </div>

    <!-- 短信弹窗 -->
    <div v-if="showSmsDialog" class="sms-overlay" @mousedown.stop>
      <div class="sms-dialog">
        <div style="font-size: 13px; color: #e0e0e0; margin-bottom: 8px; font-weight: bold">模拟短信</div>
        <input v-model="smsAddress" placeholder="发送号码" style="width: 100%; margin-bottom: 6px; padding: 6px 8px; background: #333; border: 1px solid #555; border-radius: 4px; color: #e0e0e0; font-size: 12px" />
        <textarea v-model="smsBody" placeholder="短信内容" rows="3" style="width: 100%; margin-bottom: 8px; padding: 6px 8px; background: #333; border: 1px solid #555; border-radius: 4px; color: #e0e0e0; font-size: 12px; resize: none"></textarea>
        <div style="display: flex; gap: 6px; justify-content: flex-end">
          <button class="sidebar-btn" @click="showSmsDialog = false" style="width: auto; padding: 4px 12px; font-size: 12px">取消</button>
          <button class="sidebar-btn" @click="doSendSms" style="width: auto; padding: 4px 12px; font-size: 12px; background: #409eff; color: #fff">发送</button>
        </div>
      </div>
    </div>
    <!-- 底部拖拽调整大小 -->
  </div>
</template>

<script setup>
import { ref, computed, reactive, onBeforeUnmount, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { useAuthStore } from '../../stores/auth.js'
import api from '../../api/index.js'

const device = useDeviceStore()
const auth = useAuthStore()

const props = defineProps({
  modelValue: Boolean,
  container: Object,
  // 多窗口位置偏移
  offsetIndex: { type: Number, default: 0 }
})
const emit = defineEmits(['update:modelValue', 'close'])

const iframeRef = ref(null)

// 浮窗位置和大小 - 根据 offsetIndex 错开
const pos = reactive({
  x: 100 + props.offsetIndex * 30,
  y: 60 + props.offsetIndex * 30,
  w: 380,
  h: 700
})
let dragging = false
let startX = 0, startY = 0, startPosX = 0, startPosY = 0

const floatStyle = computed(() => ({
  left: pos.x + 'px',
  top: pos.y + 'px',
  width: pos.w + 'px',
  height: pos.h + 'px',
}))

// 向 iframe 发送安卓控制指令
function sendCmd(action) {
  if (iframeRef.value?.contentWindow) {
    iframeRef.value.contentWindow.postMessage({ action }, location.origin)
  }
}

// 拖动
function onDragStart(e) {
  if (e.target.closest('.projection-btn') || e.target.closest('.projection-footer') || e.target.closest('.projection-sidebar') || e.target.closest('.sms-overlay')) return
  dragging = true
  startX = e.clientX; startY = e.clientY
  startPosX = pos.x; startPosY = pos.y
  document.addEventListener('mousemove', onDragMove)
  document.addEventListener('mouseup', onDragEnd)
}
function onDragMove(e) {
  if (!dragging) return
  pos.x = startPosX + (e.clientX - startX)
  pos.y = startPosY + (e.clientY - startY)
}
function onDragEnd() {
  dragging = false
  document.removeEventListener('mousemove', onDragMove)
  document.removeEventListener('mouseup', onDragEnd)
}


// 端口提取
function getPort(container, containerPort, fallbackOffset) {
  const bindings = container?.portBindings?.[containerPort]
  if (bindings && bindings.length > 0) return bindings[0].HostPort
  const idx = container?.indexNum || 1
  return String(30000 + (idx - 1) * 100 + fallbackOffset)
}

const playerUrl = ref('')

// 异步获取投屏 token 并构建 playerUrl
async function loadPlayerUrl() {
  if (!props.container) { playerUrl.value = ''; return }
  try {
    const resp = await device.requestProjectionToken(props.container.name)
    const projToken = typeof resp === 'string' ? resp : resp?.token
    const udpPort = typeof resp === 'object' ? resp?.udpPort : ''
    if (!projToken) {
      // 投屏 token 获取失败，回退到旧方式（兼容未重新登录的场景）
      console.warn('[投屏] 投屏 token 为空，使用主 token 回退')
      const currentPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
      const params = new URLSearchParams({
        shost: window.location.hostname,
        sport: currentPort,
        q: '5', v: 'h264',
        rtc_i: window.location.hostname,
        rtc_p: udpPort || currentPort,
        container_name: props.container.name || '',
        token: (auth.token || '') + ':' + (props.container.name || '')
      })
      playerUrl.value = `/webplayer/play.html?${params.toString()}`
      return
    }
    const currentPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
    const params = new URLSearchParams({
      shost: window.location.hostname,
      sport: currentPort,
      q: '1',
      v: 'h264',
      rtc_i: window.location.hostname,
      rtc_p: udpPort || currentPort,
      container_name: props.container.name || '',
      token: projToken
    })
    playerUrl.value = `/webplayer/play.html?${params.toString()}`
  } catch (e) {
    console.error('[投屏] 获取 token 失败，使用主 token 回退:', e)
    // 回退到旧方式
    const currentPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
    const params = new URLSearchParams({
      shost: window.location.hostname,
      sport: currentPort,
      q: '5', v: 'h264',
      rtc_i: window.location.hostname,
      rtc_p: currentPort,
      container_name: props.container.name || '',
      token: (auth.token || '') + ':' + (props.container.name || '')
    })
    playerUrl.value = `/webplayer/play.html?${params.toString()}`
  }
}

// 容器变化时重新获取 token
watch(() => props.container?.name, () => {
  if (props.modelValue && props.container) loadPlayerUrl()
}, { immediate: true })

async function openNewWindow() {
  const currentPort = window.location.port || (window.location.protocol === 'https:' ? '443' : '80')
  let tokenParam, udpPort = ''
  try {
    const resp = await device.requestProjectionToken(props.container.name)
    if (typeof resp === 'object' && resp) {
      tokenParam = resp.token
      udpPort = resp.udpPort || ''
    } else {
      tokenParam = resp
    }
    if (!tokenParam) tokenParam = (auth.token || '') + ':' + (props.container.name || '')
  } catch {
    tokenParam = (auth.token || '') + ':' + (props.container.name || '')
  }
  const params = new URLSearchParams({
    shost: window.location.hostname,
    sport: currentPort,
    q: '5', v: 'h264',
    rtc_i: window.location.hostname,
    rtc_p: udpPort || currentPort,
    container_name: props.container.name || '',
    token: tokenParam
  })
  window.open(`/webplayer/play.html?${params.toString()}`, `projection_${props.container?.name}`, 'width=400,height=750,menubar=no,toolbar=no')
  closeProjection()
}

function closeProjection() {
  // 通知 iframe 内 SDK 清理
  if (iframeRef.value?.contentWindow) {
    try { iframeRef.value.contentWindow.globalCleanup?.() } catch {}
  }
  emit('close', props.container?.name)
  emit('update:modelValue', false)
}

// ===== 延迟监测 =====
const latencyMs = ref(-1)
let latencyTimer = null

const latencyText = computed(() => {
  if (latencyMs.value < 0) return '--ms'
  return latencyMs.value + 'ms'
})
const latencyColor = computed(() => {
  if (latencyMs.value < 0) return '#666'
  if (latencyMs.value < 30) return '#67c23a'
  if (latencyMs.value < 80) return '#e6a23c'
  return '#f56c6c'
})

function onLatencyMessage(e) {
  if (e.data?.type === 'latency') {
    latencyMs.value = e.data.rtt ?? -1
  }
}

function startLatencyPoll() {
  window.addEventListener('message', onLatencyMessage)
  // 立即测一次
  measureWsLatency()
  latencyTimer = setInterval(() => {
    if (iframeRef.value?.contentWindow) {
      try {
        iframeRef.value.contentWindow.postMessage({ action: 'getLatency' }, '*')
      } catch {}
    }
    if (latencyMs.value < 0) measureWsLatency()
  }, 3000)
}

function measureWsLatency() {
  const start = performance.now()
  device.request('panel:version', {}, 5000).then(() => {
    latencyMs.value = Math.round(performance.now() - start)
  }).catch(() => {})
}

function stopLatencyPoll() {
  window.removeEventListener('message', onLatencyMessage)
  if (latencyTimer) { clearInterval(latencyTimer); latencyTimer = null }
  latencyMs.value = -1
}

// ===== 工具栏功能 =====
const uploadInput = ref(null)
const certInput = ref(null)
const showSmsDialog = ref(false)
const smsAddress = ref('')
const smsBody = ref('')

function doShake() {
  if (!props.container) return
  device.request('android:shake', { name: props.container.name }).catch(() => {})
}

async function doUpload(e) {
  const file = e.target.files?.[0]
  if (!file || !props.container) return
  const form = new FormData()
  form.append('file', file)
  try {
    await api.post(`/container/${props.container.name}/upload`, form, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 600000 })
  } catch {}
  if (uploadInput.value) uploadInput.value.value = ''
}

async function doCertUpload(e) {
  const file = e.target.files?.[0]
  if (!file || !props.container) return
  const form = new FormData()
  form.append('file', file)
  try {
    await api.post(`/container/${props.container.name}/cert`, form, { headers: { 'Content-Type': 'multipart/form-data' }, timeout: 60000 })
  } catch {}
  if (certInput.value) certInput.value.value = ''
}

function doSendSms() {
  if (!props.container || !smsAddress.value || !smsBody.value) return
  device.request('android:sms', { name: props.container.name, address: smsAddress.value, body: smsBody.value }).catch(() => {})
  showSmsDialog.value = false
  smsAddress.value = ''; smsBody.value = ''
}

// 剪贴板快捷键：Ctrl+V 粘贴到安卓，Ctrl+Shift+X 从安卓复制
function onClipboardKey(e) {
  if (!props.modelValue || !props.container) return
  if (!e.ctrlKey && !e.metaKey) return

  if (e.code === 'KeyV' && !e.shiftKey) {
    // Ctrl+V: 电脑剪贴板 → 安卓剪贴板
    e.preventDefault()
    navigator.clipboard.readText().then(text => {
      if (!text) return
      // 只通过 WS 写入安卓剪贴板，不直接调 iframe（避免文字出现两次）
      device.request('clipboard:set', { name: props.container.name, text }).catch(() => {})
    }).catch(() => {})
  } else if (e.code === 'KeyX' && e.shiftKey) {
    // Ctrl+Shift+X: 安卓剪贴板 → 电脑剪贴板
    e.preventDefault()
    e.stopPropagation()
    device.request('clipboard:get', { name: props.container.name }).then(resp => {
      const text = resp.data?.data?.text || resp.data?.text || ''
      if (text) {
        navigator.clipboard.writeText(text).then(() => {
          // 短暂提示
          document.title = '已复制: ' + text.substring(0, 20) + (text.length > 20 ? '...' : '')
          setTimeout(() => { document.title = 'myt-panel' }, 2000)
        }).catch(() => {})
      }
    }).catch(() => {})
  }
}

watch(() => props.modelValue, (visible) => {
  if (visible) {
    window.addEventListener('keydown', onClipboardKey)
    startLatencyPoll()
  } else {
    window.removeEventListener('keydown', onClipboardKey)
    stopLatencyPoll()
  }
}, { immediate: true })

onBeforeUnmount(() => {
  window.removeEventListener('keydown', onClipboardKey)
  stopLatencyPoll()
  if (iframeRef.value?.contentWindow) {
    try { iframeRef.value.contentWindow.globalCleanup?.() } catch {}
  }
})
</script>

<style scoped>
.projection-float {
  position: fixed;
  z-index: 9999;
  background: #1a1a1a;
  border: 1px solid #444;
  border-radius: 8px;
  box-shadow: 0 8px 32px rgba(0,0,0,0.6);
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transform: translateZ(0);
  will-change: transform;
  backface-visibility: hidden;
  contain: layout style;
}
.projection-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: #252525;
  cursor: move;
  user-select: none;
  color: #e0e0e0;
  font-size: 13px;
  flex-shrink: 0;
}
.projection-header-btns {
  display: flex;
  gap: 8px;
}
.projection-btn {
  cursor: pointer;
  font-size: 14px;
  color: #999;
  width: 22px;
  height: 22px;
  display: flex;
  align-items: center;
  justify-content: center;
  border-radius: 4px;
}
.projection-btn:hover { background: #333; color: #fff; }
.projection-iframe {
  width: 100%;
  height: 100%;
  border: none;
  transform: translateZ(0);
}
.projection-main {
  flex: 1;
  display: flex;
  overflow: hidden;
}
.projection-body {
  flex: 1;
  overflow: hidden;
  background: #000;
  transform: translateZ(0);
  contain: strict;
}
.projection-sidebar {
  width: 42px;
  background: #1a1a1a;
  border-left: 1px solid #333;
  display: flex;
  flex-direction: column;
  align-items: center;
  padding: 6px 0;
  gap: 3px;
  flex-shrink: 0;
}
.sidebar-btn {
  width: 36px;
  padding: 3px 0;
  background: none;
  border: 1px solid transparent;
  border-radius: 6px;
  cursor: pointer;
  display: flex;
  flex-direction: column;
  align-items: center;
  justify-content: center;
  font-size: 13px;
  color: #ccc;
  transition: all 0.15s;
  gap: 0px;
}
.sidebar-btn span {
  font-size: 8px;
  color: #888;
  line-height: 1;
}
.sidebar-btn:hover { background: #333; border-color: #555; }
.sidebar-btn:hover span { color: #ccc; }
.sidebar-btn:active { background: #444; }
.sidebar-divider {
  width: 30px;
  height: 1px;
  background: #333;
  margin: 2px 0;
}
.sidebar-latency {
  font-size: 10px;
  font-weight: bold;
  font-family: monospace;
  text-align: center;
  padding: 2px 0;
  line-height: 1;
}
.projection-footer {
  display: flex;
  justify-content: space-around;
  align-items: center;
  height: 36px;
  background: #1e1e1e;
  border-top: 1px solid #333;
  flex-shrink: 0;
}
.sms-overlay {
  position: absolute;
  inset: 0;
  background: rgba(0,0,0,0.6);
  display: flex;
  align-items: center;
  justify-content: center;
  z-index: 10;
}
.sms-dialog {
  background: #1e1e1e;
  border: 1px solid #444;
  border-radius: 8px;
  padding: 14px;
  width: 260px;
}
.android-btn {
  background: none;
  border: none;
  color: #aaa;
  font-size: 18px;
  cursor: pointer;
  width: 48px;
  height: 32px;
  border-radius: 6px;
  display: flex;
  align-items: center;
  justify-content: center;
}
.android-btn:hover { background: #333; color: #fff; }
.android-btn:active { background: #444; }
</style>
