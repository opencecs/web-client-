<template>
  <div>
    <!-- 拉取进度区域（置顶显示） -->
    <el-card v-if="pullTasks.size > 0" style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <span style="color: #e0e0e0; font-weight: bold">镜像拉取任务 ({{ pullTasks.size }})</span>
      </template>
      <div v-for="[url, task] in pullTasks" :key="url" style="margin-bottom: 16px; padding: 12px; background: #252525; border-radius: 6px">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
          <div style="overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 70%;" :title="url">
            <span style="color: #e0e0e0; font-size: 13px; font-weight: 600;">{{ task.name || getImageShortName(url) }}</span>
            <span v-if="task.name" style="color: #888; font-size: 12px; margin-left: 4px;">({{ getImageShortName(url) }})</span>
          </div>
          <el-tag :type="task.phase === 'done' ? 'success' : task.phase === 'failed' ? 'danger' : 'warning'" size="small">
            {{ phaseLabel(task.phase) }}
          </el-tag>
        </div>
        <template v-if="task.phase === 'pulling'">
          <el-progress :percentage="task.percent" :stroke-width="12" :show-text="false" striped striped-flow />
          <div style="color: #999; font-size: 12px; margin-top: 4px">{{ task.text || '准备下载...' }}</div>
        </template>
        <template v-else-if="task.phase === 'extracting'">
          <el-progress :percentage="50" :stroke-width="12" :show-text="false" striped striped-flow :indeterminate="true" />
          <div style="color: #999; font-size: 12px; margin-top: 4px">{{ task.text || '正在解压镜像层...' }}</div>
        </template>
        <template v-else-if="task.phase === 'done'">
          <el-progress :percentage="100" status="success" :stroke-width="12" />
          <div style="color: #67c23a; font-size: 12px; margin-top: 4px">{{ task.text || '拉取完成' }}</div>
        </template>
        <template v-else-if="task.phase === 'failed'">
          <el-progress :percentage="100" status="exception" :stroke-width="12" />
          <div style="color: #f56c6c; font-size: 12px; margin-top: 4px">{{ task.text || '拉取失败' }}</div>
        </template>
      </div>
    </el-card>

    <!-- 在线镜像列表（合并本地状态） -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">在线镜像</span>
          <el-space>
            <el-popconfirm title="确认清理所有未使用的镜像？" @confirm="pruneImages">
              <template #reference>
                <el-button size="small" type="warning">清理未使用</el-button>
              </template>
            </el-popconfirm>
            <el-radio-group v-model="onlineFilter" size="small">
              <el-radio-button value="all">全部</el-radio-button>
              <el-radio-button value="and14">安卓 14</el-radio-button>
              <el-radio-button value="and16">安卓 16</el-radio-button>
            </el-radio-group>
            <el-button size="small" :icon="Refresh" @click="refreshAll" :loading="loadingMirrors" circle />
          </el-space>
        </div>
      </template>
      <el-table :data="filteredMirrors" v-loading="loadingMirrors" size="small" stripe>
        <el-table-column label="镜像名称" prop="name" min-width="250" show-overflow-tooltip />
        <el-table-column label="版本" width="80">
          <template #default="{ row }">
            <el-tag size="small" :type="row.os_ver === 'and16' ? 'success' : 'primary'">{{ row.os_ver === 'and14' ? 'A14' : 'A16' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="状态" width="90">
          <template #default="{ row }">
            <el-tag v-if="pullTasks.has(row.url)" type="warning" size="small">拉取中</el-tag>
            <el-tag v-else-if="isLocalImage(row.url)" type="success" size="small">已拉取</el-tag>
            <el-tag v-else type="info" size="small">未拉取</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="镜像地址" prop="url" min-width="300" show-overflow-tooltip />
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-popconfirm v-if="isLocalImage(row.url) && !pullTasks.has(row.url)" title="确认删除此本地镜像？" @confirm="deleteImageByUrl(row.url)">
              <template #reference>
                <el-button type="danger" size="small" text>删除</el-button>
              </template>
            </el-popconfirm>
            <el-button v-else-if="!pullTasks.has(row.url)" type="primary" size="small" text @click="pullFromMirror(row)">拉取</el-button>
            <span v-else style="color: #999; font-size: 12px">拉取中...</span>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!filteredMirrors.length && !loadingMirrors" description="暂无在线镜像" :image-size="60" />
    </el-card>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../stores/device.js'
import { pullImage } from '../../utils/pullImage.js'

const device = useDeviceStore()

const mirrors = ref([])
const images = ref([])
const loadingMirrors = ref(false)
const loadingImages = ref(false)
const onlineFilter = ref('all')

// 拉取任务映射：imageUrl → { phase, percent, text }
const pullTasks = reactive(new Map())

// 本地镜像URL集合（用于匹配在线镜像是否已拉取）
const localImageUrls = computed(() => {
  const urls = new Set()
  for (const img of images.value) {
    const url = img.imageUrl || img.image || img.name || ''
    if (url) urls.add(url)
  }
  return urls
})

function isLocalImage(url) {
  if (!url) return false
  // 精确匹配
  if (localImageUrls.value.has(url)) return true
  // 部分匹配（本地镜像可能只存了最后一段）
  const shortUrl = url.split('/').pop()
  for (const local of localImageUrls.value) {
    if (local === shortUrl || local.endsWith('/' + shortUrl)) return true
  }
  return false
}

const filteredMirrors = computed(() => {
  let list = mirrors.value
  if (onlineFilter.value !== 'all') {
    list = list.filter(m => m.os_ver === onlineFilter.value)
  }
  // 按名称降序排序（版本号大的排前面）
  return [...list].sort((a, b) => {
    return (b.name || b.url || '').localeCompare(a.name || a.url || '', undefined, { numeric: true })
  })
})

function getImageShortName(url) {
  if (!url) return ''
  const tag = url.split(':').pop()
  return tag || url.split('/').pop() || url
}

function phaseLabel(phase) {
  const labels = { pulling: '下载中', extracting: '解压中', done: '完成', failed: '失败' }
  return labels[phase] || '进行中'
}

async function fetchMirrors() {
  loadingMirrors.value = true
  try {
    const resp = await device.request('device:mirrors')
    mirrors.value = resp.data || []
  } catch {} finally { loadingMirrors.value = false }
}

async function fetchImages() {
  loadingImages.value = true
  try {
    const resp = await device.request('sdk:listImages')
    const d = resp.data
    const list = d?.data?.list || d?.list || d?.data || d || []
    images.value = Array.isArray(list) ? list : []
  } catch {} finally { loadingImages.value = false }
}

function refreshAll() {
  fetchMirrors()
  fetchImages()
}

function pullFromMirror(row) {
  if (pullTasks.has(row.url)) { ElMessage.warning('该镜像正在拉取中'); return }
  pullTasks.set(row.url, { phase: 'pulling', percent: 0, text: '准备下载...', name: row.name || '' })
  pullImage(row.url, {
    onProgress({ percent, text }) {
      const task = pullTasks.get(row.url)
      if (task) { task.phase = 'pulling'; task.percent = percent; task.text = text }
    },
    onExtracting(text) {
      const task = pullTasks.get(row.url)
      if (task) { task.phase = 'extracting'; task.text = text }
    },
    onComplete(text) {
      const task = pullTasks.get(row.url)
      if (task) { task.phase = 'done'; task.text = text }
      // 3秒后清除已完成的任务
      setTimeout(() => { pullTasks.delete(row.url) }, 3000)
    },
    onError(msg) {
      const task = pullTasks.get(row.url)
      if (task) { task.phase = 'failed'; task.text = msg }
      ElMessage.error(msg)
      setTimeout(() => { pullTasks.delete(row.url) }, 5000)
    },
  }).then(ok => {
    if (ok) {
      ElMessage.success('镜像拉取成功')
      fetchImages()
    }
  })
}

// 全局事件监听：刷新页面后自动恢复正在进行的拉取任务
function globalPullHandler(msg) {
  if (msg.event !== 'task:progress') return
  if (msg.data?.action !== 'pullImage') return
  const url = msg.data.imageUrl
  if (!url) return

  // 如果已被 pullImage() 的 handler 管理，跳过
  if (pullTasks.has(url)) return

  // 发现新的后台拉取任务，自动显示
  if (msg.data.done) {
    // 收到完成事件但本地没有记录，说明是刷新前已完成的，不处理
    return
  }
  // 创建新的进度条跟踪
  const mirror = mirrors.value.find(m => m.url === url)
  pullTasks.set(url, { phase: 'pulling', percent: 0, text: '检测到后台拉取任务...', name: mirror?.name || '' })
  // 后续事件会被 pullImage.js 的监听处理不到（因为没有调 pullImage()），
  // 所以这里注册一个专门的监听器
  startTrackingPull(url)
}

function startTrackingPull(url) {
  const handler = (msg) => {
    if (msg.event !== 'task:progress') return
    if (msg.data?.action !== 'pullImage') return
    if (msg.data.imageUrl !== url) return

    if (msg.data.done) {
      const task = pullTasks.get(url)
      if (task) {
        task.phase = 'done'
        task.text = msg.data.exists ? '镜像已存在' : '拉取完成'
      }
      setTimeout(() => { pullTasks.delete(url) }, 3000)
      device.offEvent(handler)
      fetchImages()
      return
    }

    const chunk = msg.data.chunk || ''
    const lines = chunk.split('\n')
    for (const line of lines) {
      let eventData = null
      if (line.startsWith('data: ')) {
        try { eventData = JSON.parse(line.slice(6)) } catch {}
      } else if (line.trim().startsWith('{')) {
        try { eventData = JSON.parse(line.trim()) } catch {}
      }
      if (!eventData) continue

      const task = pullTasks.get(url)
      if (!task) continue

      if (eventData.error) {
        task.phase = 'failed'
        task.text = eventData.error
        setTimeout(() => { pullTasks.delete(url) }, 5000)
        device.offEvent(handler)
        return
      }
      if (eventData.status === 'Downloading' && eventData.progressDetail?.total) {
        const current = eventData.progressDetail.current || 0
        const total = eventData.progressDetail.total
        task.phase = 'pulling'
        task.percent = Math.min(99, Math.round(current / total * 100))
        task.text = `下载中: ${formatBytes(current)} / ${formatBytes(total)}`
      } else if (eventData.status === 'Extracting' || eventData.status === 'Pull complete') {
        task.phase = 'extracting'
        task.text = eventData.status === 'Extracting' ? '正在解压镜像层...' : eventData.status
      } else if (eventData.status === 'No operation' || eventData.message === 'Image already exists') {
        task.phase = 'done'
        task.text = '镜像已存在'
        setTimeout(() => { pullTasks.delete(url) }, 3000)
        device.offEvent(handler)
        fetchImages()
      }
    }
  }
  device.onEvent(handler)
}

function formatBytes(bytes) {
  if (!bytes) return '0 B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}

async function deleteImageByUrl(url) {
  try {
    await device.request('sdk:deleteImage', { image: url })
    ElMessage.success('删除成功')
    fetchImages()
  } catch (e) {
    ElMessage.error(e.message || '删除失败')
  }
}

async function pruneImages() {
  try {
    await device.request('sdk:pruneImages')
    ElMessage.success('清理完成')
    fetchImages()
  } catch (e) {
    ElMessage.error(e.message || '清理失败')
  }
}

onMounted(() => {
  fetchMirrors()
  fetchImages()
  // 注册全局监听，自动发现后台拉取任务
  device.onEvent(globalPullHandler)
})

onBeforeUnmount(() => {
  device.offEvent(globalPullHandler)
})
</script>
