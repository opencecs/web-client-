<template>
  <div>
    <!-- 工具栏 -->
    <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 16px">
      <el-space>
        <el-button type="primary" @click="showCreate = true">创建容器</el-button>
        <el-dropdown @command="handleBatch" :disabled="!selected.length" trigger="click">
          <el-button :disabled="!selected.length">
            批量操作 ({{ selected.length }}) <el-icon><ArrowDown /></el-icon>
          </el-button>
          <template #dropdown>
            <el-dropdown-menu>
              <el-dropdown-item command="start">批量启动</el-dropdown-item>
              <el-dropdown-item command="stop">批量停止</el-dropdown-item>
              <el-dropdown-item command="restart">批量重启</el-dropdown-item>
              <el-dropdown-item command="reset">批量重置</el-dropdown-item>
              <el-dropdown-item command="changeImage">批量切换镜像</el-dropdown-item>
              <el-dropdown-item command="delete" divided style="color: #f56c6c">批量删除</el-dropdown-item>
            </el-dropdown-menu>
          </template>
        </el-dropdown>
      </el-space>
      <el-button :icon="Refresh" circle @click="device.refreshContainers()" />
    </div>

    <!-- 容器表格 -->
    <el-table :data="device.containers" @selection-change="onSelect"
      style="width: 100%" row-key="name" size="small" stripe>
      <el-table-column type="selection" width="45" />
      <el-table-column label="坑位" prop="indexNum" width="60" sortable />
      <el-table-column label="名称" prop="name" min-width="120" show-overflow-tooltip />
      <el-table-column label="状态" width="90">
        <template #default="{ row }">
          <el-tag :type="statusType(row.status)" size="small">{{ statusText(row.status) }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="IP" prop="ip" width="130" />
      <el-table-column label="镜像" min-width="150" show-overflow-tooltip>
        <template #default="{ row }">{{ shortImage(row.image) }}</template>
      </el-table-column>
      <el-table-column label="创建时间" width="160">
        <template #default="{ row }">{{ formatTime(row.created) }}</template>
      </el-table-column>
      <el-table-column label="操作" width="320" fixed="right">
        <template #default="{ row }">
          <el-space wrap :size="4">
            <el-button v-if="row.status !== 'running'" type="success" size="small" text @click="doAction('start', row)">启动</el-button>
            <el-button v-if="row.status === 'running'" type="warning" size="small" text @click="doAction('stop', row)">停止</el-button>
            <el-button size="small" text @click="doAction('restart', row)">重启</el-button>
            <el-button size="small" text @click="doReset(row)">重置</el-button>
            <el-button size="small" text @click="startRename(row)">重命名</el-button>
            <el-button size="small" text @click="startCopy(row)">复制</el-button>
            <el-button v-if="row.status === 'running'" type="primary" size="small" text @click="$emit('projection', row)">投屏</el-button>
            <el-button v-if="row.status === 'running'" type="primary" size="small" text @click="$emit('terminal', row)">终端</el-button>
            <el-popconfirm title="确认删除此容器？" @confirm="doDelete(row)">
              <template #reference>
                <el-button type="danger" size="small" text>删除</el-button>
              </template>
            </el-popconfirm>
          </el-space>
        </template>
      </el-table-column>
    </el-table>

    <!-- 创建容器弹窗 -->
    <CreateContainer v-model="showCreate" :max-slots="maxSlots" :used-slots="usedSlots" @created="device.refreshContainers()" />

    <!-- 重命名弹窗 -->
    <el-dialog v-model="renameVisible" title="重命名容器" width="400px">
      <el-form label-width="80px">
        <el-form-item label="原名称">{{ renameTarget?.name }}</el-form-item>
        <el-form-item label="新名称">
          <el-input v-model="newName" placeholder="输入新名称" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="doRename">确认</el-button>
      </template>
    </el-dialog>

    <!-- 复制弹窗 -->
    <el-dialog v-model="copyVisible" title="复制容器" width="400px">
      <el-form label-width="90px">
        <el-form-item label="源容器">{{ copyTarget?.name }}</el-form-item>
        <el-form-item label="目标坑位">
          <el-input-number v-model="copySlot" :min="0" :max="maxSlots" :step="1" />
          <span style="color: #999; margin-left: 8px; font-size: 12px">0 = 自动分配</span>
        </el-form-item>
        <el-form-item label="复制数量">
          <el-input-number v-model="copyCount" :min="1" :max="20" :step="1" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="copyVisible = false">取消</el-button>
        <el-button type="primary" :loading="copying" @click="doCopy">确认</el-button>
      </template>
    </el-dialog>

    <!-- 批量切换镜像弹窗 -->
    <el-dialog v-model="changeImageVisible" title="批量切换镜像" width="500px">
      <el-form label-width="90px">
        <el-form-item label="目标容器">
          <el-tag v-for="c in selected" :key="c.name" size="small" style="margin: 2px">{{ c.name }}</el-tag>
        </el-form-item>
        <el-form-item label="新镜像">
          <el-input v-model="newImageUrl" placeholder="输入镜像 URL" />
        </el-form-item>
      </el-form>
      <div v-if="changeImageTaskId" style="margin-top: 12px">
        <el-progress :percentage="changeImageProgress" :status="changeImageProgress >= 100 ? 'success' : ''" />
        <p style="color: #999; font-size: 12px; margin-top: 4px">{{ changeImageStatus }}</p>
      </div>
      <template #footer>
        <el-button @click="changeImageVisible = false">取消</el-button>
        <el-button type="primary" :loading="changingImage" @click="doBatchChangeImage">执行</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, watch } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { ArrowDown, Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../stores/device.js'
import CreateContainer from './CreateContainer.vue'

const props = defineProps({
  maxSlots: { type: Number, default: 12 }
})

const emit = defineEmits(['projection', 'terminal'])

const device = useDeviceStore()
const selected = ref([])
const showCreate = ref(false)
const mirrorMap = ref({}) // url -> name 映射

const usedSlots = computed(() => device.containers.filter(c => c.indexNum > 0).map(c => ({ num: c.indexNum, name: c.name })))

// 状态
function statusType(s) {
  if (s === 'running') return 'success'
  if (s === 'restarting') return 'warning'
  return 'info'
}
function statusText(s) {
  if (s === 'running') return '运行中'
  if (s === 'restarting') return '重启中'
  if (s === 'exited' || s === 'shutdown') return '已停止'
  return s || '未知'
}

// 加载镜像名称映射
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

// 镜像名简化：优先显示在线镜像的中文名
function shortImage(url) {
  if (!url) return '-'
  if (mirrorMap.value[url]) return mirrorMap.value[url]
  const parts = url.split('/')
  return parts[parts.length - 1] || url
}

// 时间格式化
function formatTime(t) {
  if (!t) return '-'
  return t.replace('T', ' ').replace(/\.\d+.*/, '')
}

function onSelect(rows) {
  selected.value = rows
}

// 单个操作 - 通过 WS
async function doAction(action, row) {
  try {
    await device.request(`container:${action}`, { name: row.name })
    ElMessage.success(`${row.name} ${action} 成功`)
  } catch (e) {
    ElMessage.error(e.message || `${action} 失败`)
  }
}

// 重置
async function doReset(row) {
  try {
    await ElMessageBox.confirm(`确认重置容器 ${row.name}？数据将被清除。`, '重置容器', { type: 'warning' })
    await device.request('container:reset', { name: row.name })
    ElMessage.success('重置成功')
  } catch (e) {
    if (e !== 'cancel' && e?.message !== 'cancel') ElMessage.error(e.message || '重置失败')
  }
}

// 删除
async function doDelete(row) {
  try {
    await device.request('container:delete', { name: row.name })
    ElMessage.success('删除成功')
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

// 重命名
const renameVisible = ref(false)
const renameTarget = ref(null)
const newName = ref('')
const renaming = ref(false)

function startRename(row) {
  renameTarget.value = row
  newName.value = row.name
  renameVisible.value = true
}

async function doRename() {
  if (!newName.value) return
  renaming.value = true
  try {
    await device.request('container:rename', { name: renameTarget.value.name, newName: newName.value })
    ElMessage.success('重命名成功')
    renameVisible.value = false
  } catch (e) {
    ElMessage.error(e.message || '重命名失败')
  } finally {
    renaming.value = false
  }
}

// 复制
const copyVisible = ref(false)
const copyTarget = ref(null)
const copySlot = ref(0)
const copyCount = ref(1)
const copying = ref(false)

function startCopy(row) {
  copyTarget.value = row
  copySlot.value = 0
  copyCount.value = 1
  copyVisible.value = true
}

async function doCopy() {
  copying.value = true
  try {
    await device.request('container:copy', {
      name: copyTarget.value.name,
      indexNum: copySlot.value,
      count: copyCount.value
    })
    ElMessage.success('复制成功')
    copyVisible.value = false
  } catch (e) {
    ElMessage.error(e.message || '复制失败')
  } finally {
    copying.value = false
  }
}

// 批量操作
async function handleBatch(cmd) {
  const names = selected.value.map(c => c.name)
  if (!names.length) return

  if (cmd === 'changeImage') {
    changeImageVisible.value = true
    return
  }

  if (cmd === 'delete') {
    try {
      await ElMessageBox.confirm(`确认删除 ${names.length} 个容器？`, '批量删除', { type: 'warning' })
    } catch { return }
  }

  if (cmd === 'reset') {
    try {
      await ElMessageBox.confirm(`确认重置 ${names.length} 个容器？`, '批量重置', { type: 'warning' })
    } catch { return }
  }

  let successCount = 0
  let failCount = 0

  for (const name of names) {
    try {
      if (cmd === 'delete') await device.request('container:delete', { name })
      else if (cmd === 'reset') await device.request('container:reset', { name })
      else await device.request(`container:${cmd}`, { name })
      successCount++
    } catch { failCount++ }
  }

  ElMessage.success(`完成: ${successCount} 成功, ${failCount} 失败`)
}

// 批量切换镜像
const changeImageVisible = ref(false)
const newImageUrl = ref('')
const changingImage = ref(false)
const changeImageTaskId = ref('')
const changeImageProgress = ref(0)
const changeImageStatus = ref('')

async function doBatchChangeImage() {
  if (!newImageUrl.value) { ElMessage.warning('请输入镜像 URL'); return }
  changingImage.value = true
  changeImageProgress.value = 0
  changeImageStatus.value = '提交中...'
  try {
    const names = selected.value.map(c => c.name)
    await device.request('sdk:batchChangeImage', {
      containerNames: names,
      image: newImageUrl.value
    }, 120000)
    ElMessage.success('切换完成')
    changeImageVisible.value = false
    device.refreshContainers()
  } catch (e) {
    ElMessage.error(e.message || '切换失败')
  } finally {
    changingImage.value = false
  }
}

onMounted(() => {
  if (device.online) fetchMirrorMap()
})
// 刷新页面后 WS 重连时重新获取镜像名称映射
watch(() => device.online, (v) => { if (v && !Object.keys(mirrorMap.value).length) fetchMirrorMap() })

defineExpose({
  fetchList: () => device.refreshContainers()
})
</script>
