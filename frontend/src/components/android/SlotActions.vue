<template>
  <div class="slot-actions">
    <el-space wrap>
      <el-button v-if="auth.can('container_create')" type="primary" size="small" @click="$emit('create')">创建容器</el-button>
      <el-button v-if="auth.can('container_start')" type="success" size="small" :disabled="!hasStopped" @click="confirmAction('start')">启动</el-button>
      <el-button v-if="auth.can('container_start')" size="small" :disabled="!hasRunning" @click="confirmAction('stop')">停止</el-button>
      <el-button v-if="auth.can('container_restart')" size="small" :disabled="!hasContainer" @click="confirmAction('restart')">重启</el-button>
      <el-button v-if="auth.can('container_reset')" type="warning" size="small" :disabled="!hasContainer" @click="confirmAction('reset')">重置</el-button>
      <el-button v-if="auth.can('alias_manage')" size="small" :disabled="!hasContainer" @click="$emit('rename', singleContainer)">设置别名</el-button>
      <el-button v-if="auth.can('container_copy')" size="small" :disabled="!singleContainer" @click="$emit('copy', singleContainer)">复制</el-button>
      <el-button v-if="auth.can('projection')" type="primary" size="small" :disabled="!hasRunning" @click="doProjection">投屏</el-button>
      <el-button v-if="auth.can('projection')" size="small" @click="$emit('close-all-projections')">关闭投屏</el-button>
      <el-button v-if="auth.can('terminal')" size="small" :disabled="!singleRunning" @click="$emit('terminal', singleContainer)">终端</el-button>
      <el-button v-if="auth.can('container_start')" size="small" :disabled="!singleRunning" @click="$emit('s5proxy', singleContainer)">S5 代理</el-button>
      <el-button v-if="auth.can('backup_manage')" size="small" :disabled="!singleContainer" @click="$emit('backup-switch', liveSlots[0])">备份切换</el-button>
      <el-button v-if="auth.can('container_delete')" type="danger" size="small" :disabled="!hasContainer" @click="confirmAction('delete')">删除</el-button>
    </el-space>
    <span class="selection-hint" v-if="selected.length > 0">
      已选 {{ selected.length }} 个坑位
      <span v-if="singleContainer"> — {{ device.displayName(singleContainer.name) }}</span>
    </span>

    <!-- 确认弹窗 -->
    <el-dialog v-model="confirmVisible" :title="confirmTitle" width="360px" :close-on-click-modal="false" align-center>
      <p style="margin: 0; color: #ccc; font-size: 14px;">{{ confirmMsg }}</p>
      <template #footer>
        <el-button @click="confirmVisible = false">取消</el-button>
        <el-button :type="confirmDanger ? 'danger' : 'primary'" :loading="executing" @click="doConfirmedAction">确认</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  selected: { type: Array, default: () => [] }
})

const emit = defineEmits(['create', 'projection', 'terminal', 'backup-switch', 'rename', 'copy', 'close-all-projections', 's5proxy'])

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

// 投屏
function doProjection() {
  const runningSlots = liveSlots.value.filter(s => s.running && s.container)
  for (const slot of runningSlots) {
    emit('projection', slot.container)
  }
}

// 确认弹窗状态
const confirmVisible = ref(false)
const confirmTitle = ref('')
const confirmMsg = ref('')
const confirmDanger = ref(false)
const pendingAction = ref('')
const executing = ref(false)

const actionLabels = { start: '启动', stop: '停止', restart: '重启', reset: '重置', delete: '删除' }

function confirmAction(action) {
  const slotsWithContainers = liveSlots.value.filter(s => s.container)
  if (!slotsWithContainers.length) {
    ElMessage.warning('选中的坑位没有容器')
    return
  }
  const label = actionLabels[action]
  const count = slotsWithContainers.length
  confirmTitle.value = `批量${label}`
  confirmDanger.value = action === 'reset' || action === 'delete'
  if (action === 'reset') {
    confirmMsg.value = `确认重置 ${count} 个容器？数据将被清除，不可恢复。`
  } else if (action === 'delete') {
    confirmMsg.value = `确认删除 ${count} 个容器？`
  } else {
    confirmMsg.value = `确认${label} ${count} 个容器？`
  }
  pendingAction.value = action
  confirmVisible.value = true
}

async function doConfirmedAction() {
  const action = pendingAction.value
  const label = actionLabels[action]
  const slotsWithContainers = liveSlots.value.filter(s => s.container)
  executing.value = true
  let success = 0, fail = 0
  for (const slot of slotsWithContainers) {
    try {
      await device.request(`container:${action}`, { name: slot.container.name })
      success++
    } catch { fail++ }
  }
  executing.value = false
  confirmVisible.value = false
  ElMessage.success(`${label}完成: ${success} 成功${fail ? `, ${fail} 失败` : ''}`)
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
