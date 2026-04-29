<template>
  <div style="padding: 24px">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: 16px">
      <div style="display: flex; align-items: center; gap: 12px">
        <h3 style="margin: 0; color: #e0e0e0">备份管理</h3>
        <span style="color: #999; font-size: 13px">共 {{ containerBackups.length }} 个备份</span>
        <el-button size="small" type="danger" :disabled="!selectedRows.length" @click="batchDeleteBackups">
          批量删除{{ selectedRows.length ? ` (${selectedRows.length})` : '' }}
        </el-button>
      </div>
      <el-button size="small" :loading="loading" @click="loadContainerBackups">刷新</el-button>
    </div>

    <div v-for="slot in sortedSlots" :key="slot" style="margin-bottom: 16px">
      <div class="slot-title">坑位 {{ slot }}</div>
      <el-table :data="getSlotContainers(slot)" style="width: 100%"
        row-key="name" ref="tableRefs" @selection-change="(s) => onSelChange(slot, s)">
        <el-table-column type="selection" width="45" />
        <el-table-column label="名称" min-width="140">
          <template #default="{ row }">
            <span style="color: #000">{{ device.displayName(row.name) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="镜像版本" min-width="140" align="center">
          <template #default="{ row }">
            <span style="color: #999; font-size: 13px">{{ matchMirrorName(row.image) }}</span>
          </template>
        </el-table-column>
        <el-table-column label="运行状态" width="100" align="center">
          <template #default="{ row }">
            <el-tag v-if="row.status === 'running'" type="success" size="small">运行</el-tag>
            <el-tag v-else type="info" size="small">停止</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="操作" min-width="280" align="center">
          <template #default="{ row }">
            <div style="display: flex; gap: 4px; flex-wrap: wrap; justify-content: center">
              <el-button size="small" type="success" :disabled="row.status === 'running'"
                :loading="actionLoading[row.name] === 'start'"
                @click="startContainer(row.name)">开机</el-button>
              <el-button size="small" type="warning" :disabled="row.status !== 'running'"
                :loading="actionLoading[row.name] === 'stop'"
                @click="stopContainer(row.name)">关机</el-button>
              <el-button size="small" @click="openRename(row)">修改名称</el-button>
              <el-button size="small" type="primary" :disabled="row.status !== 'running'"
                @click="openProjection(row.name)">投屏</el-button>
              <el-popconfirm title="确认删除该容器？数据将不可恢复" @confirm="deleteContainer(row.name)">
                <template #reference>
                  <el-button size="small" type="danger">删除</el-button>
                </template>
              </el-popconfirm>
            </div>
          </template>
        </el-table-column>
      </el-table>
    </div>

    <el-empty v-if="sortedSlots.length === 0 && !loading" description="暂无容器" />

    <!-- 修改名称弹窗 -->
    <el-dialog v-model="renameVisible" title="修改名称" width="420px">
      <el-form :model="renameForm" :rules="renameRules" ref="renameFormRef" label-width="70px">
        <el-form-item label="容器">
          <span style="color: #999; font-size: 13px">{{ renameTarget }}</span>
        </el-form-item>
        <el-form-item label="别名" prop="alias">
          <el-input v-model="renameForm.alias" placeholder="输入别名（支持中文、空格、符号）" clearable />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="renameVisible = false">取消</el-button>
        <el-button type="primary" :loading="renameSaving" @click="doRename">保存</el-button>
      </template>
    </el-dialog>

    <!-- 投屏窗口 -->
    <ContainerProjection
      v-for="(p, idx) in projections"
      :key="p.name"
      v-model="p.visible"
      :container="p.container"
      :offset-index="idx"
      @close="removeProjection"
    />
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { ElMessage, ElMessageBox } from 'element-plus'
import { useDeviceStore } from '../stores/device.js'
import { useAuthStore } from '../stores/auth.js'
import ContainerProjection from '../components/android/ContainerProjection.vue'

const device = useDeviceStore()
const auth = useAuthStore()
const loading = ref(false)
const containerBackups = ref([])
const mirrorCache = ref([])
const actionLoading = reactive({})

// 选择状态：slotNum -> 容器数组
const slotSels = reactive({})
const selectedRows = computed(() => {
  const all = []
  for (const arr of Object.values(slotSels)) {
    for (const row of arr) all.push(row)
  }
  return all
})

// 缓存每坑位容器引用，避免 filter 产生新数组导致 table 重渲染
const slotContainerCache = reactive({})
function getSlotContainers(slot) {
  // 与 device.containers 同引用
  const current = device.containers.filter(c => (c.indexNum || 0) === slot)
  const cached = slotContainerCache[slot]
  if (cached && cached.length === current.length && cached.every((c, i) => c.name === current[i].name)) {
    return cached
  }
  slotContainerCache[slot] = current
  return current
}

async function loadMirrors() {
  try {
    const resp = await device.request('device:mirrors')
    const raw = resp.data
    mirrorCache.value = Array.isArray(raw) ? raw : (Array.isArray(raw?.data) ? raw.data : [])
  } catch {}
}

function matchMirrorName(url) {
  if (!url) return '-'
  const match = mirrorCache.value.find(m => m.url === url)
  if (match) return match.name
  const parts = url.split('/')
  return parts[parts.length - 1] || url
}

const sortedSlots = computed(() => {
  const slots = new Set()
  for (const c of device.containers) {
    slots.add(c.indexNum || 0)
  }
  return [...slots].sort((a, b) => a - b)
})

function onSelChange(slot, selection) {
  slotSels[slot] = selection
}

// ===== 容器操作 =====
async function startContainer(name) {
  const target = device.containers.find(c => c.name === name)
  if (!target) return
  const running = device.containers.find(c =>
    c.indexNum === target.indexNum && c.status === 'running' && c.name !== name
  )
  if (running) {
    try {
      await ElMessageBox.confirm(
        `坑位 ${target.indexNum} 当前运行的是 ${device.displayName(running.name)}，启动此容器将停止它，是否继续？`,
        '切换容器', { type: 'warning' }
      )
    } catch { return }
    actionLoading[running.name] = 'stop'
    try {
      await device.request('container:stop', { name: running.name })
      await new Promise(r => setTimeout(r, 2000))
    } catch (e) { ElMessage.error(e.message || '停止旧容器失败'); delete actionLoading[running.name]; return }
    finally { delete actionLoading[running.name] }
  }
  actionLoading[name] = 'start'
  try {
    await device.request('container:start', { name })
    ElMessage.success('开机成功')
  } catch (e) { ElMessage.error(e.message || '开机失败') }
  finally { delete actionLoading[name] }
}

async function stopContainer(name) {
  actionLoading[name] = 'stop'
  try {
    await device.request('container:stop', { name })
    ElMessage.success('关机成功')
  } catch (e) { ElMessage.error(e.message || '关机失败') }
  finally { delete actionLoading[name] }
}

async function deleteContainer(name) {
  try {
    await device.request('container:delete', { name })
    ElMessage.success('已删除')
  } catch (e) { ElMessage.error(e.message || '删除失败') }
}

// 改名
const renameVisible = ref(false)
const renameTarget = ref('')
const renameForm = reactive({ alias: '' })
const renameFormRef = ref(null)
const renameSaving = ref(false)
const renameRules = { alias: [{ required: true, message: '请输入别名', trigger: 'blur' }] }

function openRename(row) {
  renameTarget.value = row.name
  renameForm.alias = device.containerAliases[row.name] || ''
  renameVisible.value = true
}

async function doRename() {
  try { await renameFormRef.value?.validate() } catch { return }
  renameSaving.value = true
  try {
    await device.setAlias(renameTarget.value, renameForm.alias.trim())
    ElMessage.success('名称已更新')
    renameVisible.value = false
  } catch (e) { ElMessage.error(e.message || '修改失败') }
  finally { renameSaving.value = false }
}

// 投屏
const projections = reactive([])
function openProjection(name) {
  const container = device.containers.find(c => c.name === name)
  if (!container) return
  const existing = projections.find(p => p.name === name)
  if (existing) { existing.visible = true; existing.container = container; return }
  projections.push({ name, container, visible: true })
}
function removeProjection(name) {
  const idx = projections.findIndex(p => p.name === name)
  if (idx !== -1) projections.splice(idx, 1)
}

// ===== 备份操作 =====
async function batchDeleteBackups() {
  const names = selectedRows.value.map(r => r.name)
  if (!names.length) return
  try {
    await ElMessageBox.confirm(
      `确认删除选中的 ${names.length} 个容器？删除后不可恢复`,
      '批量删除', { type: 'warning' }
    )
  } catch { return }

  // 循环调用单个删除
  for (const name of names) {
    try {
      await device.request('container:delete', { name })
    } catch (e) {
      ElMessage.error(`${name} 删除失败: ${e.message}`)
    }
  }
  ElMessage.success('批量删除完成')
  Object.keys(slotSels).forEach(k => delete slotSels[k])
  loadContainerBackups()
}

async function loadContainerBackups() {
  loading.value = true
  try {
    const resp = await device.request('sdk:listBackups')
    const d = resp.data
    containerBackups.value = Array.isArray(d?.data?.list) ? d.data.list : []
  } catch (e) {
    containerBackups.value = []
    ElMessage.error(e.message || '获取容器备份失败')
  } finally { loading.value = false }
}

onMounted(() => {
  loadMirrors()
  loadContainerBackups()
})
</script>

<style scoped>
.slot-title {
  font-size: 14px;
  font-weight: 600;
  color: #409eff;
  padding: 8px 12px 6px;
  margin-top: 4px;
}
</style>
