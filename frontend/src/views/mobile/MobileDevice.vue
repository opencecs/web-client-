<template>
  <div class="mobile-device">
    <van-nav-bar title="设备管理" left-arrow @click-left="$router.back()" :border="false" />

    <!-- SDK 版本 -->
    <div class="section-title">SDK 版本</div>
    <van-cell-group inset>
      <van-cell title="当前版本" :value="versionLoading ? '加载中...' : (versionInfo.currentVersion || '-')" />
      <van-cell title="最新版本" :value="versionLoading ? '' : (versionInfo.latestVersion || '-')" />
      <van-cell>
        <van-button type="primary" size="small" block :loading="checking || upgrading"
          @click="handleCheckUpgrade">
          {{ upgrading ? '升级中...' : '检查升级' }}
        </van-button>
      </van-cell>
    </van-cell-group>
    <div v-if="upgradeStatus" class="progress-area">
      <van-progress :percentage="upgradeProgress"
        :color="upgradeProgressOk ? '#67c23a' : upgradeProgressFail ? '#f56c6c' : '#409eff'"
        stroke-width="6" track-color="#2a2a2a" :show-pivot="false" />
      <div class="progress-text">{{ upgradeStatus }}</div>
    </div>

    <!-- 面板版本 -->
    <div class="section-title">面板版本</div>
    <van-cell-group inset>
      <van-cell title="当前版本" :value="'v' + (panelInfo.currentVersion || '-')" />
      <van-cell title="最新版本">
        <template #value>
          <span>v{{ panelInfo.latestVersion || '-' }}</span>
          <van-tag v-if="panelInfo.hasUpdate" type="success" size="medium" style="margin-left: 4px">有新版</van-tag>
          <van-tag v-else-if="panelInfo.latestVersion" type="default" size="medium" style="margin-left: 4px">已最新</van-tag>
        </template>
      </van-cell>
      <van-cell v-if="panelInfo.changelog">
        <div style="font-size: 12px; color: #aaa; white-space: pre-wrap; line-height: 1.6">{{ panelInfo.changelog }}</div>
      </van-cell>
      <van-cell>
        <div style="display: flex; gap: 8px">
          <van-button size="small" :loading="panelChecking" @click="checkPanelUpdate">检查更新</van-button>
          <van-button v-if="panelInfo.hasUpdate" type="success" size="small"
            :loading="panelUpdating" @click="doPanelUpdate">立即更新</van-button>
        </div>
      </van-cell>
    </van-cell-group>
    <div v-if="panelUpdateStatus" class="progress-area">
      <van-progress :percentage="panelUpdateProgress"
        :color="panelProgressOk ? '#67c23a' : panelProgressFail ? '#f56c6c' : '#409eff'"
        stroke-width="6" track-color="#2a2a2a" :show-pivot="false" />
      <div class="progress-text">{{ panelUpdateStatus }}</div>
    </div>

    <!-- 网络设置 -->
    <div class="section-title">网络设置</div>
    <van-cell-group inset>
      <van-field v-model="networkSettings.publicUdpPort" label="公网UDP端口"
        :placeholder="networkLoading ? '加载中...' : '留空则与网页端口一致'" :readonly="networkLoading" />
      <van-cell>
        <van-button type="primary" size="small" :loading="savingNetwork" @click="saveNetworkSettings">保存</van-button>
      </van-cell>
    </van-cell-group>

    <!-- 魔云腾账号 -->
    <div class="section-title">魔云腾账号</div>
    <van-cell-group inset v-if="mytLoading">
      <van-cell><van-loading size="20" style="margin: 8px 0">加载中...</van-loading></van-cell>
    </van-cell-group>
    <van-cell-group inset v-else-if="!mytStatus.loggedIn">
      <van-field v-model="mytForm.username" label="账号" placeholder="魔云腾账号" />
      <van-field v-model="mytForm.password" label="密码" type="password" placeholder="密码" />
      <van-cell>
        <van-button type="primary" size="small" :loading="mytLogging" @click="mytLogin">登录并同步</van-button>
      </van-cell>
    </van-cell-group>
    <van-cell-group inset v-else>
      <van-cell title="登录账号" :value="mytStatus.uname || mytStatus.username || '-'" />
      <van-cell title="通讯状态">
        <template #value>
          <van-tag :type="mytStatus.hasToken ? 'success' : 'danger'" size="medium">
            {{ mytStatus.hasToken ? '正常' : '异常' }}
          </van-tag>
        </template>
      </van-cell>
      <van-cell title="设备绑定">
        <template #value>
          <van-tag v-if="bindInfo.bindStatus === 1" type="success" size="medium">已绑定</van-tag>
          <van-tag v-else-if="bindInfo.bindStatus === 2" type="warning" size="medium">他人绑定</van-tag>
          <van-tag v-else type="default" size="medium">未绑定</van-tag>
        </template>
      </van-cell>
      <van-cell title="自动同步">
        <template #right-icon>
          <van-switch :model-value="mytStatus.autoSync" size="20px" @change="mytToggleAuto" />
        </template>
      </van-cell>
      <van-cell>
        <div style="display: flex; flex-wrap: wrap; gap: 8px">
          <van-button size="small" type="primary" :loading="mytSyncing" @click="mytSync">手动同步</van-button>
          <van-button v-if="bindInfo.bindStatus !== 1" size="small" type="success" :loading="binding" @click="mytBind">绑定设备</van-button>
          <van-button v-if="bindInfo.bindStatus === 1" size="small" type="warning" @click="mytUnbindConfirm">解绑设备</van-button>
          <van-button size="small" @click="mytLogout">退出登录</van-button>
        </div>
      </van-cell>
    </van-cell-group>

    <!-- 设备操作 -->
    <div class="section-title">设备操作</div>
    <van-cell-group inset>
      <van-cell title="重启设备" is-link @click="confirmReboot" />
      <van-cell title="清空设备磁盘" is-link @click="confirmCleanDisk">
        <template #label><span style="color: #f56c6c; font-size: 11px">清空所有数据且不可恢复，设备将重启</span></template>
      </van-cell>
    </van-cell-group>
    <div v-if="cleanStatus" class="progress-area">
      <van-progress :percentage="cleanProgress"
        :color="cleanProgress >= 100 ? '#67c23a' : '#409eff'"
        stroke-width="6" track-color="#2a2a2a" :show-pivot="false" />
      <div class="progress-text">{{ cleanStatus }}</div>
    </div>

    <div style="height: 24px"></div>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { showToast, showConfirmDialog } from 'vant'

const device = useDeviceStore()

// === SDK 版本 ===
const versionInfo = ref({})
const versionLoading = ref(true)
const checking = ref(false)
const upgrading = ref(false)
const upgradeProgress = ref(0)
const upgradeStatus = ref('')
const upgradeProgressOk = ref(false)
const upgradeProgressFail = ref(false)

async function fetchVersion() {
  try {
    const resp = await device.request('device:version')
    const d = resp.data?.data || resp.data
    versionInfo.value = d || {}
  } catch {}
  finally { versionLoading.value = false }
}

async function handleCheckUpgrade() {
  checking.value = true
  upgradeStatus.value = ''
  upgradeProgress.value = 0
  upgradeProgressOk.value = false
  upgradeProgressFail.value = false
  await fetchVersion()
  checking.value = false

  const v = versionInfo.value
  if (v.currentVersion && v.latestVersion && v.currentVersion >= v.latestVersion) {
    showToast('已是最新版本')
    upgradeStatus.value = '已是最新版本'
    upgradeProgress.value = 100
    upgradeProgressOk.value = true
    return
  }

  upgrading.value = true
  upgradeStatus.value = '正在升级...'
  upgradeProgress.value = 10

  let totalChunks = 0
  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'device:upgrade') {
      if (msg.data.done) {
        upgradeProgress.value = 100
        upgradeStatus.value = '升级完成'
        upgradeProgressOk.value = true
        upgrading.value = false
        device.offEvent(handler)
        fetchVersion()
        return
      }
      totalChunks++
      const raw = (msg.data.chunk || '').trim()
      if (!raw) return
      let text = raw
      try {
        const obj = JSON.parse(raw)
        if (obj.error) { upgradeStatus.value = obj.message || '升级失败'; upgradeProgressFail.value = true; return }
        text = obj.message || obj.msg || raw
      } catch {}
      if (text.includes('最新版本')) {
        upgradeStatus.value = '已是最新版本'; upgradeProgress.value = 100; upgradeProgressOk.value = true
      } else {
        upgradeProgress.value = Math.min(10 + totalChunks * 15, 95)
        upgradeStatus.value = text.substring(0, 50)
      }
    }
  }
  device.onEvent(handler)

  try {
    await device.request('device:upgrade', {}, 120000)
  } catch (e) {
    upgradeStatus.value = '升级失败: ' + e.message
    upgradeProgressFail.value = true
    upgrading.value = false
    device.offEvent(handler)
  }
}

// === 面板版本 ===
const panelInfo = ref({})
const panelChecking = ref(false)
const panelUpdating = ref(false)
const panelUpdateProgress = ref(0)
const panelUpdateStatus = ref('')
const panelProgressOk = ref(false)
const panelProgressFail = ref(false)

async function checkPanelUpdate() {
  panelChecking.value = true
  try {
    const resp = await device.request('panel:checkUpdate')
    panelInfo.value = resp.data || {}
  } catch {
    panelInfo.value = {}
    try {
      const vResp = await device.request('panel:version')
      panelInfo.value = { currentVersion: vResp.data?.version || 'dev' }
    } catch {}
  } finally { panelChecking.value = false }
}

async function doPanelUpdate() {
  try {
    await showConfirmDialog({ title: '确认', message: '更新期间面板将短暂断开（2-3秒），确认？' })
  } catch { return }

  panelUpdating.value = true
  panelUpdateProgress.value = 0
  panelUpdateStatus.value = '正在更新...'
  panelProgressOk.value = false
  panelProgressFail.value = false

  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'panel:update') {
      const d = msg.data
      if (d.phase === 'error') {
        panelUpdateStatus.value = d.message
        panelProgressFail.value = true
        panelUpdating.value = false
        device.offEvent(handler)
        return
      }
      if (d.phase === 'restarting') {
        panelUpdateProgress.value = 100
        panelUpdateStatus.value = d.message
        panelProgressOk.value = true
        panelUpdating.value = false
        device.offEvent(handler)
        showToast('更新完成，正在重启...')
        setTimeout(() => window.location.reload(), 4000)
        return
      }
      if (d.progress !== undefined) panelUpdateProgress.value = d.progress
      if (d.message) panelUpdateStatus.value = d.message
    }
  }
  device.onEvent(handler)

  try {
    await device.request('panel:doUpdate', {}, 600000)
  } catch (e) {
    if (!panelProgressOk.value) {
      panelUpdateStatus.value = '更新失败: ' + e.message
      panelProgressFail.value = true
      panelUpdating.value = false
      device.offEvent(handler)
    }
  }
}

// === 网络设置 ===
const networkSettings = reactive({ publicUdpPort: '' })
const savingNetwork = ref(false)
const networkLoading = ref(true)

async function fetchNetworkSettings() {
  try {
    const resp = await device.request('settings:get')
    const data = resp.data || {}
    networkSettings.publicUdpPort = data.public_udp_port || ''
  } catch {}
  finally { networkLoading.value = false }
}

async function saveNetworkSettings() {
  savingNetwork.value = true
  try {
    await device.request('settings:set', { key: 'public_udp_port', value: networkSettings.publicUdpPort || '' })
    showToast('已保存')
  } catch (e) { showToast(e.message || '保存失败') }
  finally { savingNetwork.value = false }
}

// === 魔云腾账号 ===
const mytForm = reactive({ username: '', password: '' })
const mytStatus = ref({})
const mytLoading = ref(true)
const mytLogging = ref(false)
const mytSyncing = ref(false)
const bindInfo = ref({})
const binding = ref(false)

async function fetchMytStatus() {
  mytLoading.value = true
  try {
    const resp = await device.request('myt:status')
    mytStatus.value = resp.data || {}
    if (resp.data?.loggedIn && resp.data.bindDeviceID) {
      bindInfo.value = { deviceId: resp.data.bindDeviceID, bindStatus: resp.data.bindStatus }
    } else if (resp.data?.loggedIn) {
      try {
        const br = await device.request('myt:bindStatus')
        bindInfo.value = br.data || {}
      } catch {}
    }
  } catch {}
  finally { mytLoading.value = false }
}

async function mytLogin() {
  if (!mytForm.username || !mytForm.password) { showToast('请输入账号和密码'); return }
  mytLogging.value = true
  try {
    await device.request('myt:login', { username: mytForm.username, password: mytForm.password })
    showToast('登录成功'); mytForm.username = ''; mytForm.password = ''
    await fetchMytStatus()
  } catch (e) { showToast(e.message || '登录失败') }
  finally { mytLogging.value = false }
}

async function mytSync() {
  mytSyncing.value = true
  try {
    await device.request('myt:sync')
    showToast('同步成功'); await fetchMytStatus()
  } catch (e) { showToast(e.message || '同步失败') }
  finally { mytSyncing.value = false }
}

async function mytToggleAuto(val) {
  try { await device.request('myt:autoToggle', { autoSync: val }); await fetchMytStatus() }
  catch { showToast('操作失败') }
}

async function mytLogout() {
  try { await device.request('myt:logout'); mytStatus.value = {}; bindInfo.value = {}; showToast('已退出') }
  catch { showToast('操作失败') }
}

async function mytBind() {
  binding.value = true
  try { await device.request('myt:bind'); showToast('绑定成功'); await fetchMytStatus() }
  catch (e) { showToast(e.message || '绑定失败') }
  finally { binding.value = false }
}

async function mytUnbindConfirm() {
  try {
    await showConfirmDialog({ title: '确认', message: '解绑设备需要在桌面端操作（需要手机验证码）' })
  } catch {}
}

// === 设备操作 ===
const cleaning = ref(false)
const cleanProgress = ref(0)
const cleanStatus = ref('')

async function confirmReboot() {
  try {
    await showConfirmDialog({ title: '确认', message: '确认重启设备？' })
    await device.request('device:reboot')
    showToast('重启命令已发送')
  } catch {}
}

async function confirmCleanDisk() {
  try {
    await showConfirmDialog({
      title: '危险操作',
      message: '此操作将清空设备所有磁盘数据且不可恢复！设备将重启，耗时5~10分钟。',
      confirmButtonColor: '#f56c6c'
    })
  } catch { return }

  cleaning.value = true
  cleanProgress.value = 5
  cleanStatus.value = '正在清空磁盘数据...'

  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'device:cleanDisk') {
      if (msg.data.done) {
        cleanProgress.value = 100; cleanStatus.value = '清空完成'
        cleaning.value = false; device.offEvent(handler)
        return
      }
      const raw = (msg.data.chunk || '').trim()
      if (!raw) return
      let text = raw
      try { const obj = JSON.parse(raw); text = obj.message || obj.msg || raw } catch {}
      const stepMatch = text.match(/\[STEP\s+(\d+)\]/i)
      if (stepMatch) {
        cleanProgress.value = Math.round((parseInt(stepMatch[1]) / 6) * 100)
        cleanStatus.value = text.trim()
      }
      if (/Reset sequence completed|Rebooting/i.test(text)) {
        cleanProgress.value = 100; cleanStatus.value = '清空完成，设备正在重启...'
        showToast('设备磁盘已清空，正在重启')
      }
    }
  }
  device.onEvent(handler)

  try {
    await device.request('device:cleanDisk', {}, 600000)
  } catch (e) {
    cleanStatus.value = '清空失败: ' + e.message
    cleaning.value = false; device.offEvent(handler)
  }
}

function loadAll() {
  fetchVersion()
  fetchMytStatus()
  checkPanelUpdate()
  fetchNetworkSettings()
}

onMounted(() => {
  // WS 已连接则立即加载，否则等连上再加载
  if (device.online) {
    loadAll()
  }
})

watch(() => device.online, (online) => {
  if (online) loadAll()
})
</script>

<style scoped>
.mobile-device { background: #0a0a0a; min-height: 100vh; }
.section-title { font-size: 14px; font-weight: 600; color: #e0e0e0; padding: 12px 16px 8px; }
.progress-area { padding: 8px 16px; }
.progress-text { font-size: 12px; color: #999; margin-top: 4px; }
</style>
