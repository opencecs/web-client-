<template>
  <div class="mobile-images">
    <van-nav-bar title="镜像管理" left-arrow @click-left="$router.back()" :border="false">
      <template #right>
        <van-icon name="replay" size="20" @click="refreshAll" />
      </template>
    </van-nav-bar>

    <!-- 拉取进度 -->
    <div v-if="pullTasks.size > 0" class="pull-section">
      <div class="section-title">拉取任务 ({{ pullTasks.size }})</div>
      <div v-for="[url, task] in pullTasks" :key="url" class="pull-card">
        <div class="pull-header">
          <span class="pull-name">{{ getShortName(url) }}</span>
          <van-tag :type="task.phase === 'done' ? 'success' : task.phase === 'failed' ? 'danger' : 'warning'" size="medium">
            {{ phaseLabel(task.phase) }}
          </van-tag>
        </div>
        <van-progress v-if="task.phase === 'pulling'" :percentage="task.percent"
          stroke-width="6" color="#409eff" track-color="#2a2a2a" :show-pivot="false" />
        <van-progress v-else-if="task.phase === 'done'" :percentage="100"
          stroke-width="6" color="#67c23a" track-color="#2a2a2a" :show-pivot="false" />
        <div class="pull-text">{{ task.text }}</div>
      </div>
    </div>

    <!-- 版本过滤 -->
    <div class="filter-row">
      <van-tabs v-model:active="filterTab" shrink>
        <van-tab title="全部" name="all" />
        <van-tab title="Android 14" name="and14" />
        <van-tab title="Android 16" name="and16" />
      </van-tabs>
      <van-button size="small" type="warning" plain @click="pruneImages" style="margin-right: 12px">
        清理未使用
      </van-button>
    </div>

    <!-- 镜像列表 -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="image-list">
        <div v-for="img in filteredMirrors" :key="img.url" class="image-card">
          <div class="image-header">
            <span class="image-name">{{ img.name }}</span>
            <van-tag v-if="pullTasks.has(img.url)" type="warning" size="medium">拉取中</van-tag>
            <van-tag v-else-if="isLocal(img.url)" type="success" size="medium">已拉取</van-tag>
            <van-tag v-else type="default" size="medium">未拉取</van-tag>
          </div>
          <div class="image-url">{{ img.url }}</div>
          <div class="image-actions">
            <van-button v-if="!isLocal(img.url) && !pullTasks.has(img.url)"
              type="primary" size="small" plain @click="pullFromMirror(img)">拉取</van-button>
            <van-button v-if="isLocal(img.url) && !pullTasks.has(img.url)"
              type="danger" size="small" plain @click="deleteImage(img.url)">删除</van-button>
            <span v-if="pullTasks.has(img.url)" style="color: #999; font-size: 12px">拉取中...</span>
          </div>
        </div>
      </div>
      <van-empty v-if="!filteredMirrors.length && !loading" description="暂无在线镜像" />
    </van-pull-refresh>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, onBeforeUnmount, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { pullImage } from '../../utils/pullImage.js'
import { showToast, showConfirmDialog } from 'vant'

const device = useDeviceStore()

const mirrors = ref([])
const images = ref([])
const loading = ref(false)
const refreshing = ref(false)
const filterTab = ref('all')
const pullTasks = reactive(new Map())

const localUrls = computed(() => {
  const urls = new Set()
  for (const img of images.value) {
    const url = img.imageUrl || img.image || img.name || ''
    if (url) urls.add(url)
  }
  return urls
})

function isLocal(url) {
  if (!url) return false
  if (localUrls.value.has(url)) return true
  const short = url.split('/').pop()
  for (const local of localUrls.value) {
    if (local === short || local.endsWith('/' + short)) return true
  }
  return false
}

const filteredMirrors = computed(() => {
  let list = mirrors.value
  if (filterTab.value !== 'all') list = list.filter(m => m.os_ver === filterTab.value)
  return [...list].sort((a, b) => (b.name || '').localeCompare(a.name || '', undefined, { numeric: true }))
})

function getShortName(url) {
  return url?.split(':').pop() || url?.split('/').pop() || url
}

function phaseLabel(phase) {
  return { pulling: '下载中', extracting: '解压中', done: '完成', failed: '失败' }[phase] || '进行中'
}

async function fetchMirrors() {
  loading.value = true
  try {
    const resp = await device.request('device:mirrors')
    mirrors.value = resp.data || []
  } catch {} finally { loading.value = false }
}

async function fetchImages() {
  try {
    const resp = await device.request('sdk:listImages')
    const d = resp.data
    images.value = Array.isArray(d?.data?.list || d?.list || d?.data || d) ? (d?.data?.list || d?.list || d?.data || d) : []
  } catch {}
}

function refreshAll() { fetchMirrors(); fetchImages() }
function onRefresh() { refreshAll(); setTimeout(() => refreshing.value = false, 800) }

function pullFromMirror(row) {
  if (pullTasks.has(row.url)) return
  pullTasks.set(row.url, { phase: 'pulling', percent: 0, text: '准备下载...' })
  pullImage(row.url, {
    onProgress({ percent, text }) {
      const t = pullTasks.get(row.url); if (t) { t.phase = 'pulling'; t.percent = percent; t.text = text }
    },
    onExtracting(text) { const t = pullTasks.get(row.url); if (t) { t.phase = 'extracting'; t.text = text } },
    onComplete(text) {
      const t = pullTasks.get(row.url); if (t) { t.phase = 'done'; t.text = text }
      setTimeout(() => pullTasks.delete(row.url), 3000)
    },
    onError(msg) {
      const t = pullTasks.get(row.url); if (t) { t.phase = 'failed'; t.text = msg }
      showToast(msg)
      setTimeout(() => pullTasks.delete(row.url), 5000)
    },
  }).then(ok => { if (ok) { showToast('拉取成功'); fetchImages() } })
}

async function deleteImage(url) {
  try {
    await showConfirmDialog({ title: '确认', message: '删除此本地镜像？' })
    const short = url.split('/').pop()
    const match = images.value.find(img => {
      const local = img.imageUrl || img.image || img.name || ''
      return local === url || local === short || local.endsWith('/' + short)
    })
    const name = match ? (match.imageUrl || match.image || match.name) : url
    await device.request('sdk:deleteImage', { image: name })
    showToast('删除成功'); fetchImages()
  } catch {}
}

async function pruneImages() {
  try {
    await showConfirmDialog({ title: '确认', message: '清理所有未使用的镜像？' })
    await device.request('sdk:pruneImages')
    showToast('清理完成'); fetchImages()
  } catch {}
}

onMounted(() => { if (device.online) refreshAll() })
watch(() => device.online, (v) => { if (v) refreshAll() })
</script>

<style scoped>
.mobile-images { background: #0a0a0a; min-height: 100vh; }

.pull-section { padding: 0 12px; margin-bottom: 8px; }
.section-title { font-size: 14px; font-weight: 600; color: #e0e0e0; padding: 8px 4px; }
.pull-card { background: #1a1a1a; border: 1px solid #2a2a2a; border-radius: 10px; padding: 12px; margin-bottom: 8px; }
.pull-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px; }
.pull-name { font-size: 13px; color: #ccc; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 60%; }
.pull-text { font-size: 12px; color: #999; margin-top: 6px; }

.filter-row { display: flex; align-items: center; justify-content: space-between; }

.image-list { padding: 8px 12px 24px; }
.image-card { background: #1a1a1a; border: 1px solid #2a2a2a; border-radius: 10px; padding: 12px; margin-bottom: 8px; }
.image-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 4px; }
.image-name { font-size: 14px; font-weight: 600; color: #e0e0e0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; max-width: 65%; }
.image-url { font-size: 11px; color: #666; margin-bottom: 8px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.image-actions { display: flex; gap: 8px; }
</style>
