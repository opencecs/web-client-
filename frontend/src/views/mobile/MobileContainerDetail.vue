<template>
  <div class="mobile-container-detail">
    <van-nav-bar :title="displayName" left-arrow @click-left="$router.back()" :border="false" />

    <!-- 截图预览 -->
    <div class="preview-section">
      <div class="preview-card">
        <img v-if="screenshot" :src="screenshot" class="preview-img" @click="goProjection" />
        <div v-else class="preview-placeholder" @click="goProjection">
          <van-icon name="play-circle-o" size="48" color="#555" />
          <span>点击投屏</span>
        </div>
      </div>
    </div>

    <!-- 状态信息 -->
    <van-cell-group inset class="info-group">
      <van-cell title="状态">
        <template #value>
          <span style="display: inline-flex; align-items: center; gap: 6px">
            <span class="status-dot" :class="stateClass"></span>
            {{ stateText }}
          </span>
        </template>
      </van-cell>
      <van-cell title="坑位" :value="'#' + container?.indexNum" />
      <van-cell title="备注" :value="device.containerAliases[containerName] || '未设置'" />
      <van-cell title="容器ID" :value="container?.name || '-'" />
      <van-cell title="镜像" :value="imageTag" />
    </van-cell-group>

    <!-- 操作按钮 -->
    <div class="action-section">
      <div class="section-title">操作</div>
      <van-grid :column-num="4" :border="false" class="action-grid">
        <van-grid-item icon="play-circle-o" text="投屏" @click="goProjection" v-if="auth.can('projection')" />
        <van-grid-item icon="play" text="启动" @click="doAction('start')" v-if="auth.can('container_start')" />
        <van-grid-item icon="pause" text="停止" @click="doAction('stop')" v-if="auth.can('container_start')" />
        <van-grid-item icon="replay" text="重启" @click="doAction('restart')" v-if="auth.can('container_restart')" />
        <van-grid-item icon="revoke" text="重置" @click="confirmAction('reset', '确认重置此容器？')" v-if="auth.can('container_reset')" />
        <van-grid-item icon="delete-o" text="删除" @click="confirmAction('delete', '确认删除此容器？')" v-if="auth.can('container_delete')" />
        <van-grid-item icon="edit" text="重命名" @click="showRename = true" v-if="auth.can('container_rename')" />
        <van-grid-item icon="description" text="终端" @click="showTerminalHint" v-if="auth.can('terminal')" />
      </van-grid>
    </div>

    <!-- S5 代理操作 -->
    <div class="action-section" v-if="auth.can('vpc_manage')">
      <div class="section-title">S5 代理</div>
      <van-cell-group inset>
        <van-cell title="代理状态" :value="proxyStatus" is-link @click="queryProxy" />
        <van-cell title="设置代理" is-link @click="showProxyDialog = true" />
        <van-cell title="停止代理" is-link @click="stopProxy" />
      </van-cell-group>
    </div>

    <!-- 重命名弹窗 -->
    <van-dialog v-model:show="showRename" title="重命名容器" show-cancel-button
      @confirm="doRename" :before-close="beforeRenameClose">
      <div style="padding: 16px">
        <van-field v-model="newAlias" placeholder="输入新名称" clearable />
      </div>
    </van-dialog>

    <!-- S5 代理设置弹窗 -->
    <van-dialog v-model:show="showProxyDialog" title="设置 S5 代理" show-cancel-button
      @confirm="setProxy">
      <div style="padding: 16px">
        <van-field v-model="proxyForm.addr" label="地址" placeholder="代理服务器IP" />
        <van-field v-model="proxyForm.port" label="端口" placeholder="端口" type="digit" />
        <van-field v-model="proxyForm.usr" label="用户名" placeholder="用户名（可选）" />
        <van-field v-model="proxyForm.pwd" label="密码" placeholder="密码（可选）" type="password" />
        <van-field label="解析模式">
          <template #input>
            <van-radio-group v-model="proxyForm.type" direction="horizontal">
              <van-radio name="1">本地解析</van-radio>
              <van-radio name="2">服务端解析</van-radio>
            </van-radio-group>
          </template>
        </van-field>
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, computed, reactive, onMounted, watch } from 'vue'
import { useRoute, useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showToast, showConfirmDialog } from 'vant'

const route = useRoute()
const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()

const containerName = computed(() => route.params.name)
const container = computed(() => device.containers.find(c => c.name === containerName.value))
const displayName = computed(() => device.displayName(containerName.value))
const screenshot = computed(() => {
  const idx = container.value?.indexNum
  return idx ? device.screenshots[idx] : null
})

// 镜像 URL → 中文名映射
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

const imageTag = computed(() => {
  const image = container.value?.image
  if (!image) return '-'
  if (mirrorMap.value[image]) return mirrorMap.value[image]
  const parts = image.split('/')
  return parts[parts.length - 1] || image
})

const stateClass = computed(() => {
  if (container.value?.status === 'running') return 'running'
  return 'stopped'
})
const stateText = computed(() => {
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
  return map[container.value?.status] || container.value?.status || '未知'
})

const showRename = ref(false)
const newAlias = ref('')
const showProxyDialog = ref(false)
const proxyStatus = ref('未知')
const proxyForm = reactive({ addr: '', port: '', usr: '', pwd: '', type: '1' })

function goProjection() {
  if (!auth.can('projection')) { showToast('无投屏权限'); return }
  router.push(`/m/android/projection/${containerName.value}`)
}

async function doAction(action) {
  try {
    showToast({ message: `正在${action === 'start' ? '启动' : action === 'stop' ? '停止' : '重启'}...`, type: 'loading', duration: 0 })
    await device.request(`container:${action}`, { name: containerName.value })
    showToast('操作成功')
    device.refreshContainers()
  } catch (e) {
    showToast(e.message || '操作失败')
  }
}

async function confirmAction(action, message) {
  try {
    await showConfirmDialog({ title: '确认', message })
    await doAction(action)
  } catch {}
}

async function doRename() {
  if (!newAlias.value.trim()) return
  try {
    await device.setAlias(containerName.value, newAlias.value.trim())
    showToast('重命名成功')
  } catch (e) {
    showToast(e.message || '重命名失败')
  }
}

function beforeRenameClose(action) {
  if (action === 'confirm' && !newAlias.value.trim()) return false
  return true
}

function showTerminalHint() {
  showToast('终端功能请在桌面端使用')
}

async function queryProxy() {
  try {
    const resp = await device.request('proxy:status', { name: containerName.value })
    const data = resp.data?.data || resp.data
    proxyStatus.value = data?.status === 1 ? `已连接 ${data.addr || ''}` : '未连接'
  } catch {
    proxyStatus.value = '查询失败'
  }
}

async function setProxy() {
  if (!proxyForm.addr || !proxyForm.port) { showToast('请填写地址和端口'); return }
  try {
    await device.request('proxy:set', {
      name: containerName.value,
      addr: proxyForm.addr,
      port: proxyForm.port,
      usr: proxyForm.usr,
      pwd: proxyForm.pwd,
      type: proxyForm.type
    })
    showToast('代理设置成功')
    await queryProxy()
  } catch (e) {
    showToast(e.message || '设置失败')
  }
}

async function stopProxy() {
  try {
    await device.request('proxy:stop', { name: containerName.value })
    showToast('代理已停止')
    proxyStatus.value = '未连接'
  } catch (e) {
    showToast(e.message || '停止失败')
  }
}

onMounted(() => {
  if (container.value) {
    newAlias.value = device.containerAliases[containerName.value] || ''
  }
  if (device.online) fetchMirrorMap()
})
watch(() => device.online, (v) => { if (v && !Object.keys(mirrorMap.value).length) fetchMirrorMap() })
</script>

<style scoped>
.mobile-container-detail {
  background: #0a0a0a;
  min-height: 100vh;
}

.preview-section {
  padding: 12px 16px 0;
  margin-bottom: 16px;
  display: flex;
  justify-content: center;
}
.preview-card {
  background: #141414;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  overflow: hidden;
  width: 180px;
  aspect-ratio: 9/16;
  display: flex;
  align-items: center;
  justify-content: center;
}
.preview-img {
  width: 100%;
  height: 100%;
  object-fit: contain;
  cursor: pointer;
}
.preview-placeholder {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 8px;
  color: #555;
  font-size: 14px;
  cursor: pointer;
}

.info-group {
  margin-bottom: 16px;
}

.action-section {
  margin-bottom: 16px;
}
.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #e0e0e0;
  padding: 8px 16px;
}

.action-grid {
  margin: 0 12px;
  background: #1a1a1a;
  border-radius: 12px;
  overflow: hidden;
}
</style>
