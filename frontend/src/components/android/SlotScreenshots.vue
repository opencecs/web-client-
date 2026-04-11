<template>
  <div class="screenshot-grid">
    <div
      v-for="slot in slots"
      :key="slot.num"
      class="screenshot-card"
      :class="{ clickable: slot.hasScreenshot && slot.status === 'running' }"
      @click="onClickSlot(slot)"
    >
      <div class="screenshot-header">
        <span class="slot-label">坑位{{ slot.num }}</span>
        <span class="container-name" :title="slot.name">{{ slot.displayName }}</span>
      </div>
      <div class="screenshot-body">
        <img v-if="slot.hasScreenshot && slot.status === 'running'" :src="slot.screenshot" class="screenshot-img" />
        <div v-else class="screenshot-placeholder">
          <span v-if="!slot.hasContainer">空闲</span>
          <span v-else>{{ statusLabel(slot.status) }}</span>
        </div>
      </div>
    </div>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  maxSlots: { type: Number, default: 12 }
})

const emit = defineEmits(['projection'])

const auth = useAuthStore()
const device = useDeviceStore()

const slots = computed(() => {
  const list = []
  for (let i = 1; i <= props.maxSlots; i++) {
    if (!auth.canSlot(i)) continue
    const containers = device.containers.filter(c => c.indexNum === i)
    const running = containers.find(c => c.status === 'running')
    // 优先 running，其次有别名的，最后第一个
    const active = running
      || containers.find(c => device.containerAliases[c.name])
      || containers.find(c => c.status === 'exited' || c.status === 'shutdown' || c.status === 'stopped')
      || containers[0] || null
    const screenshot = device.screenshots[String(i)] || ''
    list.push({
      num: i,
      name: active?.name || '',
      displayName: active ? device.displayName(active.name) : '空闲',
      hasContainer: !!active,
      status: active?.status || '',
      container: active,
      hasScreenshot: !!screenshot,
      screenshot
    })
  }
  return list
})

function statusLabel(s) {
  const map = {
    running: '开机中...',
    restarting: '重启中...',
    creating: '创建中...',
    created: '已创建',
    exited: '已停止',
    shutdown: '已停止',
    stopped: '已停止',
    paused: '已暂停',
    dead: '异常',
  }
  return map[s] || s || '未知'
}

function onClickSlot(slot) {
  if (slot.hasScreenshot && slot.status === 'running' && slot.container) {
    if (!auth.can('projection')) {
      ElMessage.warning('无投屏权限，请联系管理员开通')
      return
    }
    emit('projection', slot.container)
  }
}
</script>

<style scoped>
.screenshot-grid {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 10px;
  padding: 4px;
}
.screenshot-card {
  background: #1e1e1e;
  border: 1px solid #333;
  border-radius: 8px;
  overflow: hidden;
  transition: border-color 0.15s;
}
.screenshot-card.clickable {
  cursor: pointer;
}
.screenshot-card.clickable:hover {
  border-color: #409eff;
}
.screenshot-header {
  display: flex;
  justify-content: space-between;
  align-items: center;
  padding: 6px 10px;
  background: #252525;
}
.slot-label {
  font-size: 12px;
  color: #888;
  font-weight: bold;
}
.container-name {
  font-size: 11px;
  color: #bbb;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  max-width: 100px;
}
.screenshot-body {
  position: relative;
  width: 100%;
  aspect-ratio: 9 / 16;
  background: #111;
}
.screenshot-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
}
.screenshot-placeholder {
  width: 100%;
  height: 100%;
  display: flex;
  align-items: center;
  justify-content: center;
  color: #555;
  font-size: 13px;
}
</style>
