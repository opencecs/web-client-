<template>
  <div v-if="modelValue && container" class="projection-float" :style="floatStyle"
    @mousedown.stop="onDragStart">
    <!-- 标题栏（可拖动） -->
    <div class="projection-header" @mousedown.stop="onDragStart">
      <span>{{ device.displayName(container.name) }} - 坑位 {{ container.indexNum }}</span>
      <div class="projection-header-btns">
        <span class="projection-btn" title="新窗口打开" @mousedown.stop @click="openNewWindow">⬗</span>
        <span class="projection-btn" title="关闭" @mousedown.stop @click="closeProjection">✕</span>
      </div>
    </div>
    <!-- 投屏内容 - iframe 隔离 SDK 实例 -->
    <div class="projection-body">
      <iframe v-if="playerUrl" ref="iframeRef" :src="playerUrl" class="projection-iframe"
        allowfullscreen allow="autoplay; fullscreen" />
    </div>
    <!-- 安卓控制按钮 - 标准顺序：最近任务 | 主页 | 返回 -->
    <div class="projection-footer">
      <button class="android-btn" title="最近任务" @mousedown.stop @click="sendCmd('goClean')">□</button>
      <button class="android-btn" title="主页" @mousedown.stop @click="sendCmd('goHome')">○</button>
      <button class="android-btn" title="返回" @mousedown.stop @click="sendCmd('goBack')">◁</button>
    </div>
    <!-- 底部拖拽调整大小 -->
    <div class="projection-resize" @mousedown.stop="onResizeStart"></div>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onBeforeUnmount, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { useAuthStore } from '../../stores/auth.js'

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
let resizing = false
let startX = 0, startY = 0, startPosX = 0, startPosY = 0, startW = 0, startH = 0

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
  if (e.target.closest('.projection-btn') || e.target.closest('.projection-resize') || e.target.closest('.projection-footer')) return
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

// 缩放
function onResizeStart(e) {
  resizing = true
  startX = e.clientX; startY = e.clientY
  startW = pos.w; startH = pos.h
  document.addEventListener('mousemove', onResizeMove)
  document.addEventListener('mouseup', onResizeEnd)
}
function onResizeMove(e) {
  if (!resizing) return
  pos.w = Math.max(280, startW + (e.clientX - startX))
  pos.h = Math.max(400, startH + (e.clientY - startY))
}
function onResizeEnd() {
  resizing = false
  document.removeEventListener('mousemove', onResizeMove)
  document.removeEventListener('mouseup', onResizeEnd)
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
        q: '1', v: 'h264',
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
      q: '1', v: 'h264',
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
    q: '1', v: 'h264',
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

onBeforeUnmount(() => {
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
.projection-body {
  flex: 1;
  overflow: hidden;
  background: #000;
}
.projection-iframe {
  width: 100%;
  height: 100%;
  border: none;
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
.projection-resize {
  position: absolute;
  right: 0;
  bottom: 0;
  width: 16px;
  height: 16px;
  cursor: nwse-resize;
  background: linear-gradient(135deg, transparent 50%, #666 50%);
  border-radius: 0 0 8px 0;
}
</style>
