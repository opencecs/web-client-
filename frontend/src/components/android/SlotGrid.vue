<template>
  <div>
    <div class="grid-toolbar">
      <el-button size="small" text @click="selectAll">全选</el-button>
      <el-button size="small" text @click="selectNone">取消全选</el-button>
      <el-button size="small" text @click="invertSelection">反选</el-button>
    </div>
    <div class="slot-grid">
      <div
        v-for="slot in slots"
        :key="slot.num"
        :class="['slot-card', { selected: isSelected(slot.num), running: slot.running }]"
        @click="toggleSelect(slot.num)"
      >
        <div class="slot-header">
          <span class="slot-num">坑位{{ slot.num }}</span>
          <span class="slot-auth">{{ authText(slot.num) }}</span>
        </div>
        <div class="slot-name" :title="slot.containerName">{{ slot.containerName || '空闲' }}</div>
        <div v-if="slot.imageTag" class="slot-image" :title="slot.imageTag">{{ slot.imageTag }}</div>
        <div class="slot-status">
          <span :class="['status-dot', slot.running ? 'dot-running' : 'dot-stopped']"></span>
          <span class="status-text">{{ slot.statusText }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  maxSlots: { type: Number, default: 12 }
})

const emit = defineEmits(['selection-change'])

const auth = useAuthStore()
const device = useDeviceStore()
const selected = ref(new Set())
const slotStates = ref({})
const mirrorMap = ref({}) // 镜像 URL → 中文名

async function fetchSlotStates() {
  try {
    const resp = await device.request('myt:slotStates')
    slotStates.value = resp.data?.slots || {}
  } catch {}
}

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

// 允许的坑位列表
const allowedSlots = computed(() => {
  const all = []
  for (let i = 1; i <= props.maxSlots; i++) {
    if (auth.canSlot(i)) all.push(i)
  }
  return all
})

const slots = computed(() => {
  const list = []
  for (const i of allowedSlots.value) {
    const containers = device.containers.filter(c => c.indexNum === i)
    const runningContainer = containers.find(c => c.status === 'running')
    // 优先 running，其次有别名的，最后第一个
    const activeContainer = runningContainer
      || containers.find(c => device.containerAliases[c.name])
      || containers.find(c => c.status === 'exited' || c.status === 'shutdown' || c.status === 'stopped')
      || containers[0] || null
    list.push({
      num: i,
      containerName: activeContainer ? device.displayName(activeContainer.name) : '',
      running: activeContainer?.status === 'running',
      statusText: activeContainer ? statusText(activeContainer.status) : '空闲',
      imageTag: runningContainer ? shortImageTag(runningContainer.image) : '',
      container: activeContainer
    })
  }
  return list
})

// 镜像简称：优先中文名，降级取标签
function shortImageTag(image) {
  if (!image) return ''
  if (mirrorMap.value[image]) return mirrorMap.value[image]
  const parts = image.split('/')
  return parts[parts.length - 1] || image
}

function statusText(s) {
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
  return map[s] || s || '未知'
}

function authText(num) {
  const info = slotStates.value[String(num)]
  if (!info) return '' // 数据未加载时不显示"未授权"
  if (info.state === 0) return '正常'
  if (info.state === 1) return '即将到期'
  if (info.state === 2) return '已到期'
  return '未知'
}

function isSelected(num) {
  return selected.value.has(num)
}

// 直接点击切换选中（无需 Ctrl）
function toggleSelect(num) {
  const newSet = new Set(selected.value)
  if (newSet.has(num)) newSet.delete(num)
  else newSet.add(num)
  selected.value = newSet
  emitSelection()
}

function selectAll() {
  const newSet = new Set()
  for (const i of allowedSlots.value) newSet.add(i)
  selected.value = newSet
  emitSelection()
}

function selectNone() {
  selected.value = new Set()
  emitSelection()
}

function invertSelection() {
  const newSet = new Set()
  for (const i of allowedSlots.value) {
    if (!selected.value.has(i)) newSet.add(i)
  }
  selected.value = newSet
  emitSelection()
}

function emitSelection() {
  const selectedSlots = slots.value.filter(s => selected.value.has(s.num))
  emit('selection-change', selectedSlots)
}

defineExpose({
  refreshSlotStates: fetchSlotStates,
  clearSelection() { selected.value = new Set(); emitSelection() }
})

function loadData() {
  fetchSlotStates()
  fetchMirrorMap()
}

onMounted(() => { if (device.online) loadData() })
watch(() => device.online, (v) => { if (v) loadData() })
</script>

<style scoped>
.grid-toolbar {
  margin-bottom: 8px;
}
.slot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(130px, 1fr));
  gap: 10px;
  padding: 4px;
}
.slot-card {
  position: relative;
  padding: 10px;
  border-radius: 8px;
  background: #1e1e1e;
  border: 2px solid #333;
  cursor: pointer;
  transition: all 0.15s;
  user-select: none;
  min-height: 80px;
  display: flex;
  flex-direction: column;
  justify-content: space-between;
}
.slot-card:hover { border-color: #555; }
.slot-card.selected { border-color: #409eff; box-shadow: 0 0 8px rgba(64, 158, 255, 0.3); }
.slot-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
}
.slot-num { font-size: 12px; color: #888; font-weight: bold; }
.slot-auth { font-size: 10px; color: #666; }
.slot-name {
  font-size: 13px; color: #e0e0e0;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  margin: 4px 0;
}
.slot-image {
  font-size: 10px; color: #67c23a;
  overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  margin-bottom: 2px;
}
.slot-status { display: flex; align-items: center; gap: 4px; }
.status-dot { width: 8px; height: 8px; border-radius: 50%; flex-shrink: 0; }
.dot-running { background: #67c23a; box-shadow: 0 0 4px #67c23a; }
.dot-stopped { background: #666; }
.status-text { font-size: 11px; color: #999; }
</style>
