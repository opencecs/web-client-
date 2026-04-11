<template>
  <div class="slot-actions">
    <el-space wrap>
      <el-button v-if="auth.can('container_create')" type="primary" size="small" @click="$emit('create')">创建容器</el-button>
      <el-button v-if="auth.can('container_start')" type="success" size="small" :disabled="!hasStopped" @click="batchAction('start')">启动</el-button>
      <el-button v-if="auth.can('container_start')" size="small" :disabled="!hasRunning" @click="batchAction('stop')">停止</el-button>
      <el-button v-if="auth.can('container_restart')" size="small" :disabled="!hasContainer" @click="batchAction('restart')">重启</el-button>
      <el-button v-if="auth.can('container_reset')" size="small" :disabled="!hasContainer" @click="batchAction('reset')">重置</el-button>
      <el-button v-if="auth.can('alias_manage')" size="small" :disabled="!hasContainer" @click="$emit('rename', singleContainer)">设置别名</el-button>
      <el-button v-if="auth.can('container_copy')" size="small" :disabled="!singleContainer" @click="$emit('copy', singleContainer)">复制</el-button>
      <el-button v-if="auth.can('projection')" type="primary" size="small" :disabled="!hasRunning" @click="doProjection">投屏</el-button>
      <el-button v-if="auth.can('projection')" size="small" @click="$emit('close-all-projections')">关闭投屏</el-button>
      <el-button v-if="auth.can('terminal')" size="small" :disabled="!singleRunning" @click="$emit('terminal', singleContainer)">终端</el-button>
      <el-button v-if="auth.can('backup_manage')" size="small" :disabled="!singleContainer" @click="$emit('backup-switch', liveSlots[0])">备份切换</el-button>
      <el-popconfirm v-if="auth.can('container_delete')" title="确认删除选中坑位的容器？" @confirm="batchAction('delete')">
        <template #reference>
          <el-button type="danger" size="small" :disabled="!hasContainer">删除</el-button>
        </template>
      </el-popconfirm>
    </el-space>
    <span class="selection-hint" v-if="selected.length > 0">
      已选 {{ selected.length }} 个坑位
      <span v-if="singleContainer"> — {{ device.displayName(singleContainer.name) }}</span>
    </span>
  </div>
</template>

<script setup>
import { computed } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  selected: { type: Array, default: () => [] }
})

const emit = defineEmits(['create', 'projection', 'terminal', 'backup-switch', 'rename', 'copy', 'close-all-projections'])

const auth = useAuthStore()
const device = useDeviceStore()

// 实时从 device store 获取选中坑位的最新状态
const liveSlots = computed(() => {
  return props.selected.map(s => {
    const num = s.num
    const containers = device.containers.filter(c => c.indexNum === num)
    const running = containers.find(c => c.status === 'running')
    const active = running
      || containers.find(c => device.containerAliases[c.name])
      || containers.find(c => c.status === 'exited' || c.status === 'shutdown' || c.status === 'stopped')
      || containers[0] || null
    return {
      num,
      container: active,
      running: active?.status === 'running',
    }
  })
})

const hasContainer = computed(() => liveSlots.value.some(s => s.container))
const hasRunning = computed(() => liveSlots.value.some(s => s.running))
const hasStopped = computed(() => liveSlots.value.some(s => s.container && !s.running))
const isSingle = computed(() => liveSlots.value.length === 1)
const singleContainer = computed(() => isSingle.value ? liveSlots.value[0]?.container : null)
const singleRunning = computed(() => singleContainer.value?.status === 'running')

// 投屏：为每个运行中的容器打开投屏
function doProjection() {
  const runningSlots = liveSlots.value.filter(s => s.running && s.container)
  for (const slot of runningSlots) {
    emit('projection', slot.container)
  }
}

async function batchAction(action) {
  const slotsWithContainers = liveSlots.value.filter(s => s.container)
  if (!slotsWithContainers.length) {
    ElMessage.warning('选中的坑位没有容器')
    return
  }
  if (action === 'reset') {
    try {
      await ElMessageBox.confirm(`确认重置 ${slotsWithContainers.length} 个容器？数据将被清除。`, '批量重置', { type: 'warning' })
    } catch { return }
  }
  let success = 0, fail = 0
  for (const slot of slotsWithContainers) {
    try {
      if (action === 'delete') await device.request('container:delete', { name: slot.container.name })
      else if (action === 'reset') await device.request('container:reset', { name: slot.container.name })
      else await device.request(`container:${action}`, { name: slot.container.name })
      success++
    } catch { fail++ }
  }
  ElMessage.success(`完成: ${success} 成功${fail ? `, ${fail} 失败` : ''}`)
}
</script>

<style scoped>
.slot-actions {
  display: flex;
  align-items: center;
  gap: 12px;
  margin-bottom: 12px;
  flex-wrap: wrap;
}
.selection-hint { color: #999; font-size: 12px; }
</style>
