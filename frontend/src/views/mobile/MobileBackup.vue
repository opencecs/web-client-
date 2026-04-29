<template>
  <div class="mobile-backup">
    <van-nav-bar title="备份管理" left-arrow @click-left="$router.back()" :border="false" />

    <!-- 顶部操作栏 -->
    <div class="top-bar">
      <span style="color: #999; font-size: 13px">共 {{ containerBackups.length }} 个备份</span>
      <div style="display: flex; gap: 8px">
        <van-button size="small" type="danger" :disabled="!selectedNames.size" @click="batchDeleteBackups">
          批量删除{{ selectedNames.size ? ` (${selectedNames.size})` : '' }}
        </van-button>
        <van-button size="small" :loading="loading" @click="loadContainerBackups">刷新</van-button>
      </div>
    </div>

    <!-- 按坑位分组列表 -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="slot-list">
        <div v-for="slot in slotGroups" :key="slot.slotNum" class="slot-group">
          <div class="slot-header">坑位 {{ slot.slotNum }}</div>
          <div v-for="c in slot.containers" :key="c.name" class="container-card">
            <div class="container-info">
              <div class="container-top">
                <div class="container-name">
                  <van-checkbox :model-value="selectedNames.has(c.name)" shape="square" icon-size="18px"
                    @update:model-value="toggleSelect(c.name)" />
                  <span>{{ device.displayName(c.name) }}</span>
                  <van-tag :type="c.status === 'running' ? 'success' : 'default'" size="medium" style="margin-left: 6px">
                    {{ c.status === 'running' ? '运行' : '停止' }}
                  </van-tag>
                </div>
              </div>
              <div class="container-image">{{ matchMirrorName(c.image) }}</div>
              <div class="container-actions">
                <van-button size="mini" type="success"
                  :disabled="c.status === 'running' || actionLoading[c.name]"
                  :loading="actionLoading[c.name] === 'start'"
                  @click="startContainer(c.name)">开机</van-button>
                <van-button size="mini" type="warning"
                  :disabled="c.status !== 'running' || actionLoading[c.name]"
                  :loading="actionLoading[c.name] === 'stop'"
                  @click="stopContainer(c.name)">关机</van-button>
                <van-button size="mini" @click="openRename(c)">修改名称</van-button>
                <van-button size="mini" type="primary" :disabled="c.status !== 'running'"
                  @click="openProjection(c.name)">投屏</van-button>
                <van-button size="mini" type="danger" @click="confirmDeleteContainer(c.name)">删除</van-button>
              </div>
            </div>
            <!-- 备份列表 -->
            <div class="backup-list" v-if="getContainerBackups(c.name).length">
              <div v-for="b in getContainerBackups(c.name)" :key="b.name || b.Name" class="backup-item">
                <span class="backup-name">{{ b.name || b.Name }}</span>
                <van-button size="mini" type="danger" plain @click.stop="deleteBackup(b.name || b.Name)">删除</van-button>
              </div>
            </div>
            <div class="backup-empty" v-else>
              <span style="color: #666; font-size: 12px">无备份</span>
            </div>
          </div>
        </div>
        <van-empty v-if="slotGroups.length === 0" description="暂无容器" />
      </div>
    </van-pull-refresh>

    <!-- 修改名称弹窗 -->
    <van-dialog v-model:show="renameVisible" title="修改名称" show-cancel-button
      @confirm="doRename" :before-close="beforeRenameClose">
      <div style="padding: 16px">
        <van-field v-model="renameInput" label="别名" placeholder="输入别名（支持中文、空格、符号）" clearable :rules="[{ required: true, message: '请输入别名' }]" />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, onMounted, reactive } from 'vue'
import { useRouter } from 'vue-router'
import { showToast, showConfirmDialog } from 'vant'
import { useDeviceStore } from '../../stores/device.js'
import { useAuthStore } from '../../stores/auth.js'

const router = useRouter()
const device = useDeviceStore()
const auth = useAuthStore()
const loading = ref(false)
const refreshing = ref(false)
const containerBackups = ref([])
const selectedNames = ref(new Set())
const mirrorCache = ref([])
const actionLoading = reactive({})

// 改名
const renameVisible = ref(false)
const renameTarget = ref('')
const renameInput = ref('')

function openRename(c) {
  renameTarget.value = c.name
  renameInput.value = device.containerAliases[c.name] || ''
  renameVisible.value = true
}

async function doRename() {
  const alias = renameInput.value.trim()
  if (!alias) return
  try {
    await device.setAlias(renameTarget.value, alias)
    showToast('名称已更新')
  } catch (e) { showToast(e.message || '修改失败') }
}

function beforeRenameClose(action) {
  if (action === 'confirm' && !renameInput.value.trim()) return false
  return true
}

// 镜像名称
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

// 按坑位分组
const slotGroups = computed(() => {
  const slotMap = {}
  for (const c of device.containers) {
    const slot = c.indexNum || 0
    if (!slotMap[slot]) slotMap[slot] = []
    slotMap[slot].push(c)
  }
  return Object.keys(slotMap).map(Number).sort((a, b) => a - b).map(slot => ({
    slotNum: slot,
    containers: slotMap[slot]
  }))
})

function getContainerBackups(name) {
  return containerBackups.value.filter(b =>
    b.containerName === name || b.ContainerName === name || b.name === name || b.Name === name
  )
}

// 勾选（按容器名）
function toggleSelect(name) {
  const set = new Set(selectedNames.value)
  if (set.has(name)) set.delete(name)
  else set.add(name)
  selectedNames.value = set
}

// 容器操作（与 PC 端一致）
async function startContainer(name) {
  const target = device.containers.find(c => c.name === name)
  if (!target) return
  // 同坑位有运行中容器时先停
  const running = device.containers.find(c =>
    c.indexNum === target.indexNum && c.status === 'running' && c.name !== name
  )
  if (running) {
    try {
      await showConfirmDialog({
        title: '切换容器',
        message: `坑位 ${target.indexNum} 当前运行的是 ${device.displayName(running.name)}，启动此容器将停止它，是否继续？`
      })
    } catch { return }
    actionLoading[running.name] = 'stop'
    try {
      showToast({ message: '正在停止旧容器...', type: 'loading', duration: 0 })
      await device.request('container:stop', { name: running.name })
      await new Promise(r => setTimeout(r, 2000))
    } catch (e) { showToast(e.message || '停止旧容器失败'); delete actionLoading[running.name]; return }
    finally { delete actionLoading[running.name] }
  }
  actionLoading[name] = 'start'
  try {
    showToast({ message: '正在开机...', type: 'loading', duration: 0 })
    await device.request('container:start', { name })
    showToast('开机成功')
  } catch (e) { showToast(e.message || '开机失败') }
  finally { delete actionLoading[name] }
}

async function stopContainer(name) {
  actionLoading[name] = 'stop'
  try {
    await device.request('container:stop', { name })
    showToast('关机成功')
  } catch (e) { showToast(e.message || '关机失败') }
  finally { delete actionLoading[name] }
}

async function confirmDeleteContainer(name) {
  try {
    await showConfirmDialog({ title: '确认删除', message: '删除容器后数据不可恢复，确认删除？' })
    await device.request('container:delete', { name })
    showToast('已删除')
  } catch {}
}

function openProjection(name) {
  router.push(`/m/android/projection/${name}`)
}

// 备份操作
async function loadContainerBackups() {
  loading.value = true
  try {
    const resp = await device.request('sdk:listBackups')
    const d = resp.data
    containerBackups.value = Array.isArray(d?.data?.list) ? d.data.list : []
  } catch (e) {
    containerBackups.value = []
    showToast(e.message || '获取备份失败')
  } finally { loading.value = false }
}

async function deleteBackup(name) {
  try {
    await showConfirmDialog({ title: '确认删除', message: '删除后不可恢复，确认删除该备份？' })
    await device.request('sdk:deleteBackup', { name })
    showToast('已删除')
    loadContainerBackups()
  } catch {}
}

async function batchDeleteBackups() {
  if (!selectedNames.value.size) return
  try {
    await showConfirmDialog({
      title: '批量删除',
      message: `确认删除选中的 ${selectedNames.value.size} 个容器？删除后不可恢复`
    })
  } catch { return }

  const names = [...selectedNames.value]
  for (const name of names) {
    try {
      await device.request('container:delete', { name })
    } catch {}
  }
  showToast('批量删除完成')
  selectedNames.value = new Set()
}

function onRefresh() {
  loadContainerBackups()
  setTimeout(() => { refreshing.value = false }, 500)
}

onMounted(() => {
  loadMirrors()
  loadContainerBackups()
})
</script>

<style scoped>
.mobile-backup {
  background: #0a0a0a;
  min-height: 100vh;
}

.top-bar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 16px;
}

.slot-list {
  padding: 0 12px 80px;
}

.slot-group {
  margin-bottom: 12px;
}

.slot-header {
  font-size: 14px;
  font-weight: 600;
  color: #409eff;
  padding: 8px 4px 4px;
}

.container-card {
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 10px;
  padding: 12px;
  margin-bottom: 6px;
}

.container-info {
  margin-bottom: 8px;
}

.container-top {
  display: flex;
  align-items: center;
  justify-content: space-between;
}

.container-name {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 14px;
  font-weight: 600;
  color: #e0e0e0;
}

.container-image {
  font-size: 12px;
  color: #666;
  margin: 4px 0 8px;
  padding-left: 26px;
}

.container-actions {
  display: flex;
  gap: 4px;
  flex-wrap: wrap;
}

.backup-list {
  border-top: 1px solid #2a2a2a;
  padding-top: 8px;
}

.backup-item {
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 6px 0;
  border-bottom: 1px solid #1e1e1e;
}
.backup-item:last-child { border-bottom: none; }

.backup-name {
  flex: 1;
  font-size: 13px;
  color: #ccc;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.backup-empty {
  padding: 4px 0;
}
</style>
