<template>
  <div class="mobile-android">
    <van-nav-bar title="云机管理" :border="false" />

    <!-- 快捷操作网格 -->
    <van-grid :column-num="5" :border="false" class="quick-actions">
      <van-grid-item icon="add-o" text="创建" @click="$router.push('/m/android/create')" v-if="auth.can('container_create')" />
      <van-grid-item icon="photo-o" text="截图" @click="$router.push('/m/screenshots')" />
      <van-grid-item icon="play-circle-o" text="全选" @click="toggleSelectAll" />
      <van-grid-item icon="replay" text="刷新" @click="refresh" />
      <van-grid-item icon="more-o" text="批量" @click="showBatchActions = true" />
    </van-grid>

    <!-- 容器列表 -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="container-list">
        <div v-if="filteredContainers.length === 0" class="empty-state">
          <van-empty description="暂无容器" />
        </div>

        <div v-for="c in filteredContainers" :key="c.name" class="slot-card"
          :class="{ selected: selectedNames.has(c.name) }">
          <!-- 左侧：截图预览 → 点击进入详情 -->
          <div class="slot-preview" @click="goDetail(c)">
            <img v-if="c.status === 'running' && screenshots[c.indexNum]" :src="screenshots[c.indexNum]" class="slot-img" />
            <div v-else class="slot-placeholder">
              <span>{{ c.indexNum }}</span>
            </div>
            <div class="slot-status" :class="stateClass(c)">
              {{ stateText(c) }}
            </div>
          </div>
          <!-- 右侧：信息+勾选 → 点击勾选 -->
          <div class="slot-right" @click="toggleSelect(c.name)">
            <div class="slot-info">
              <div class="slot-name">{{ device.displayName(c.name) }}</div>
              <div class="slot-meta">
                <span class="meta-status" :class="stateClass(c)">{{ stateText(c) }}</span>
                <span class="meta-sep">·</span>
                <span>坑位 {{ c.indexNum }}</span>
              </div>
              <div class="slot-image" v-if="c.image">{{ shortImageTag(c.image) }}</div>
            </div>
            <van-checkbox v-model="selectMap[c.name]" shape="square" icon-size="22px"
              @click.stop @change="onCheckChange(c.name, $event)" />
          </div>
        </div>
      </div>
    </van-pull-refresh>

    <!-- 批量操作面板 -->
    <van-action-sheet v-model:show="showBatchActions" title="批量操作" :actions="batchActions"
      @select="onBatchAction" cancel-text="取消" close-on-click-action />

    <!-- 底部浮动操作条（选中时显示） -->
    <transition name="slide-up">
      <div v-if="selectedNames.size > 0" class="batch-bar">
        <span class="batch-count">已选 {{ selectedNames.size }} 项</span>
        <van-button size="small" type="primary" plain @click="batchStart">启动</van-button>
        <van-button size="small" type="warning" plain @click="batchStop">停止</van-button>
        <van-button size="small" plain @click="clearSelection">取消</van-button>
      </div>
    </transition>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, watch } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showToast, showConfirmDialog } from 'vant'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()

const refreshing = ref(false)
const showBatchActions = ref(false)
const selectedNames = ref(new Set())
const selectMap = reactive({})

const screenshots = computed(() => device.screenshots || {})
const mirrorMap = ref({})

async function fetchMirrorMap() {
  try {
    const resp = await device.request('device:mirrors')
    const list = resp.data || []
    const map = {}
    for (const m of list) {
      if (m.url && m.name) map[m.url] = m.name
    }
    mirrorMap.value = map
  } catch {}
}

function shortImageTag(image) {
  if (!image) return ''
  if (mirrorMap.value[image]) return mirrorMap.value[image]
  const parts = image.split('/')
  return parts[parts.length - 1] || image
}

// 按权限过滤可见容器
const filteredContainers = computed(() => {
  return device.containers.filter(c => auth.canSlot(c.indexNum))
})

const batchActions = [
  { name: '启动选中', value: 'start' },
  { name: '停止选中', value: 'stop' },
  { name: '重启选中', value: 'restart' },
  { name: '重置选中', value: 'reset', color: '#ee0a24' },
]

function stateClass(c) {
  if (c.status === 'running') return 'running'
  return 'stopped'
}

function stateText(c) {
  const map = {
    running: '运行中',
    restarting: '重启中',
    exited: '已停止',
    shutdown: '已停止',
    stopped: '已停止',
    created: '已创建',
    creating: '创建中',
    paused: '已暂停',
    dead: '异常',
  }
  return map[c.status] || c.status || '未知'
}

function goDetail(c) {
  router.push(`/m/android/container/${c.name}`)
}

function toggleSelect(name) {
  const set = new Set(selectedNames.value)
  if (set.has(name)) {
    set.delete(name)
    selectMap[name] = false
  } else {
    set.add(name)
    selectMap[name] = true
  }
  selectedNames.value = set
}

function onCheckChange(name, checked) {
  const set = new Set(selectedNames.value)
  if (checked) set.add(name)
  else set.delete(name)
  selectedNames.value = set
}

function toggleSelectAll() {
  if (selectedNames.value.size === filteredContainers.value.length) {
    clearSelection()
  } else {
    const set = new Set()
    filteredContainers.value.forEach(c => {
      set.add(c.name)
      selectMap[c.name] = true
    })
    selectedNames.value = set
  }
}

function clearSelection() {
  selectedNames.value = new Set()
  Object.keys(selectMap).forEach(k => selectMap[k] = false)
}

const actionLabels = { start: '启动', stop: '停止', restart: '重启', reset: '重置' }

async function batchCommand(action) {
  const names = [...selectedNames.value]
  if (names.length === 0) { showToast('请先选择容器'); return }
  const label = actionLabels[action] || action
  try {
    await showConfirmDialog({
      title: `批量${label}`,
      message: `确认${label}选中的 ${names.length} 个容器？`
    })
  } catch { return }
  try {
    showToast({ message: `正在${label}...`, type: 'loading', duration: 0 })
    for (const name of names) {
      await device.request(`container:${action}`, { name })
    }
    showToast(`${label}完成`)
    device.refreshContainers()
    clearSelection()
  } catch (e) {
    showToast(e.message || '操作失败')
  }
}

async function batchStart() {
  const names = [...selectedNames.value]
  if (names.length === 0) { showToast('请先选择容器'); return }

  // 找出选中的停止状态容器，以及同坑位需要先停掉的运行中容器
  const toStart = []
  const toStopFirst = []
  for (const name of names) {
    const c = device.containers.find(x => x.name === name)
    if (!c) continue
    if (c.status === 'running') continue // 已经在跑，跳过
    toStart.push(c)
    // 同坑位有运行中的容器，需要先停掉
    const running = device.containers.find(x => x.indexNum === c.indexNum && x.status === 'running' && x.name !== name)
    if (running && !toStopFirst.find(x => x.name === running.name)) {
      toStopFirst.push(running)
    }
  }

  if (toStart.length === 0) { showToast('选中的容器都已在运行'); return }

  let msg = `启动 ${toStart.length} 个容器？`
  if (toStopFirst.length > 0) {
    msg = `将停止 ${toStopFirst.length} 个运行中的容器，然后启动 ${toStart.length} 个容器，是否继续？`
  }
  try {
    await showConfirmDialog({ title: '启动', message: msg })
  } catch { return }

  try {
    showToast({ message: '正在切换...', type: 'loading', duration: 0 })
    // 先停掉同坑位运行中的
    for (const c of toStopFirst) {
      await device.request('container:stop', { name: c.name })
    }
    if (toStopFirst.length > 0) {
      await new Promise(r => setTimeout(r, 2000))
    }
    // 再启动选中的
    for (const c of toStart) {
      await device.request('container:start', { name: c.name })
    }
    showToast('启动请求已发送，等待开机...')
    device.refreshContainers()
    clearSelection()
  } catch (e) {
    showToast(e.message || '操作失败')
  }
}
async function batchStop() { await batchCommand('stop') }

async function onBatchAction(action) {
  if (selectedNames.value.size === 0) { showToast('请先选择容器'); return }
  await batchCommand(action.value)
}

function onRefresh() {
  device.refreshContainers()
  setTimeout(() => { refreshing.value = false }, 800)
}

function refresh() {
  device.refreshContainers()
  showToast('已刷新')
}

function loadData() {
  device.refreshContainers()
  fetchMirrorMap()
}

onMounted(() => { if (device.online) loadData() })
watch(() => device.online, (v) => { if (v) loadData() })
</script>

<style scoped>
.mobile-android {
  background: #0a0a0a;
  min-height: 100vh;
}

.quick-actions {
  margin: 8px 12px;
  background: #1a1a1a;
  border-radius: 12px;
  overflow: hidden;
}

.container-list {
  padding: 0 12px 80px;
}

.slot-card {
  display: flex;
  align-items: center;
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  padding: 10px;
  margin-bottom: 8px;
  transition: border-color 0.2s;
}
.slot-card.selected {
  border-color: #409eff;
  background: rgba(64, 158, 255, 0.05);
}

.slot-preview {
  width: 54px;
  height: 96px;
  border-radius: 8px;
  overflow: hidden;
  position: relative;
  flex-shrink: 0;
  background: #141414;
  cursor: pointer;
}
.slot-img {
  width: 100%;
  height: 100%;
  object-fit: cover;
}
.slot-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 18px;
  font-weight: 700;
  color: #555;
}

.slot-status {
  position: absolute;
  bottom: 0;
  left: 0;
  right: 0;
  font-size: 10px;
  text-align: center;
  padding: 2px 0;
  color: #fff;
}
.slot-status.running { background: rgba(103, 194, 58, 0.8); }
.slot-status.stopped { background: rgba(153, 153, 153, 0.8); }

.slot-right {
  flex: 1;
  display: flex;
  align-items: center;
  padding: 8px 12px;
  min-width: 0;
  cursor: pointer;
}

.slot-info {
  flex: 1;
  min-width: 0;
}
.slot-name {
  font-size: 14px;
  font-weight: 600;
  color: #e0e0e0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.slot-meta {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
  display: flex;
  align-items: center;
  gap: 4px;
}
.meta-status { font-weight: 600; }
.meta-status.running { color: #67c23a; }
.meta-status.stopped { color: #999; }
.meta-sep { color: #555; }

.slot-image {
  font-size: 11px;
  color: #666;
  margin-top: 2px;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* 底部批量操作条（在 TabBar 上方） */
.batch-bar {
  position: fixed;
  bottom: calc(50px + constant(safe-area-inset-bottom) + 8px);
  bottom: calc(50px + env(safe-area-inset-bottom) + 8px);
  left: 12px;
  right: 12px;
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  padding: 10px 16px;
  display: flex;
  align-items: center;
  gap: 8px;
  z-index: 100;
}
.batch-count {
  flex: 1;
  font-size: 13px;
  color: #e0e0e0;
}

.slide-up-enter-active, .slide-up-leave-active {
  transition: transform 0.25s, opacity 0.25s;
}
.slide-up-enter-from, .slide-up-leave-to {
  transform: translateY(100%);
  opacity: 0;
}

.empty-state {
  padding: 60px 0;
}
</style>
