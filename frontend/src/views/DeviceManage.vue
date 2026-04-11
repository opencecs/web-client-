<template>
  <div style="padding: 24px">
    <h2 style="margin-top: 0; color: #e0e0e0">设备管理</h2>

    <!-- SDK 版本 -->
    <el-card style="background: #1a1a1a; border-color: #2a2a2a">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #e0e0e0; font-weight: bold">SDK 版本</span>
          <el-button type="primary" size="small" :loading="checking || upgrading" @click="handleCheckUpgrade">
            {{ upgrading ? '升级中...' : '检查升级' }}
          </el-button>
        </div>
      </template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="当前版本">{{ versionInfo.currentVersion || '-' }}</el-descriptions-item>
        <el-descriptions-item label="最新版本">{{ versionInfo.latestVersion || '-' }}</el-descriptions-item>
      </el-descriptions>
      <div v-if="upgradeStatus" style="margin-top: 16px">
        <el-progress :percentage="upgradeProgress" :status="upgradeProgressStatus" :stroke-width="16" striped striped-flow>
          <span style="font-size: 12px">{{ upgradeStatus }}</span>
        </el-progress>
      </div>
    </el-card>

    <!-- 面板版本 -->
    <el-card style="background: #1a1a1a; border-color: #2a2a2a; margin-top: 16px">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #e0e0e0; font-weight: bold">面板版本</span>
          <el-space>
            <el-button size="small" :loading="panelChecking" @click="checkPanelUpdate">检查更新</el-button>
            <el-popconfirm v-if="panelUpdateInfo.hasUpdate" title="更新期间面板将短暂断开连接（2-3秒），确认更新？" @confirm="doPanelUpdate">
              <template #reference>
                <el-button type="success" size="small" :loading="panelUpdating">立即更新</el-button>
              </template>
            </el-popconfirm>
          </el-space>
        </div>
      </template>
      <el-descriptions :column="2" border size="small">
        <el-descriptions-item label="当前版本">v{{ panelUpdateInfo.currentVersion || '-' }}</el-descriptions-item>
        <el-descriptions-item label="最新版本">
          <span>v{{ panelUpdateInfo.latestVersion || '-' }}</span>
          <el-tag v-if="panelUpdateInfo.hasUpdate" type="success" size="small" style="margin-left: 8px">有新版本</el-tag>
          <el-tag v-else-if="panelUpdateInfo.latestVersion" type="info" size="small" style="margin-left: 8px">已是最新</el-tag>
        </el-descriptions-item>
      </el-descriptions>
      <div v-if="panelUpdateInfo.changelog" style="margin-top: 12px; background: #141414; border-radius: 6px; padding: 10px 14px; font-size: 12px; color: #aaa; white-space: pre-wrap; line-height: 1.8">{{ panelUpdateInfo.changelog }}</div>
      <div v-if="panelUpdateStatus" style="margin-top: 12px">
        <el-progress :percentage="panelUpdateProgress" :status="panelProgressStatus" :stroke-width="16" striped striped-flow>
          <span style="font-size: 12px">{{ panelUpdateStatus }}</span>
        </el-progress>
      </div>
    </el-card>

    <!-- 网络设置 -->
    <el-card style="background: #1a1a1a; border-color: #2a2a2a; margin-top: 16px">
      <template #header>
        <div style="display: flex; align-items: center; justify-content: space-between">
          <span style="color: #e0e0e0; font-weight: bold">网络设置</span>
          <el-button type="primary" size="small" :loading="savingNetwork" @click="saveNetworkSettings">保存</el-button>
        </div>
      </template>
      <el-form label-width="140px" style="max-width: 500px">
        <el-form-item label="公网UDP端口">
          <el-input v-model="networkSettings.publicUdpPort" placeholder="留空则与网页端口一致" clearable style="width: 200px" />
          <div style="color: #999; font-size: 12px; margin-top: 4px">
            投屏WebRTC媒体流的公网端口。如果公网映射时UDP端口与TCP端口不同，请在此填写实际的UDP公网端口号。
          </div>
        </el-form-item>
      </el-form>
    </el-card>

    <!-- 设备操作 -->
    <el-card style="background: #1a1a1a; border-color: #2a2a2a; margin-top: 16px">
      <template #header><span style="color: #e0e0e0; font-weight: bold">设备操作</span></template>
      <el-space>
        <el-popconfirm title="确认重启设备？" @confirm="handleReboot">
          <template #reference>
            <el-button type="warning">重启设备</el-button>
          </template>
        </el-popconfirm>

        <el-button type="primary" @click="handleLinkSSH">远程SSH</el-button>

        <el-popconfirm
          title="此操作将清空设备所有磁盘数据且不可恢复！设备将重启，耗时5~10分钟。"
          confirm-button-text="确认清空"
          cancel-button-text="取消"
          confirm-button-type="danger"
          icon-color="#f56c6c"
          @confirm="handleCleanDisk"
        >
          <template #reference>
            <el-button type="danger" :loading="cleaning">清空设备磁盘数据</el-button>
          </template>
        </el-popconfirm>
      </el-space>

      <div v-if="cleanStatus" style="margin-top: 16px">
        <el-progress :percentage="cleanProgress" :status="cleanProgressStatus" :stroke-width="16" striped striped-flow>
          <span style="font-size: 12px">{{ cleanStatus }}</span>
        </el-progress>
      </div>
    </el-card>

    <!-- SSH 终端弹窗 -->
    <el-dialog v-model="sshDialogVisible" title="远程SSH" width="850px"
      :close-on-click-modal="false" @close="cleanupSSH" destroy-on-close
      style="--el-dialog-bg-color: #1e1e1e; --el-dialog-border-radius: 8px">
      <div v-if="!sshConnected" style="display: flex; align-items: center; gap: 12px; margin-bottom: 12px">
        <el-select v-model="sshAccount" style="width: 180px" placeholder="选择账号" @change="onSshAccountChange">
          <el-option label="user / myt" value="user:myt" />
          <el-option label="linaro / linaro" value="linaro:linaro" />
          <el-option label="自定义" value="custom" />
        </el-select>
        <el-input v-if="sshAccount === 'custom'" v-model="sshCustomUser" placeholder="用户名" style="width: 120px" />
        <el-input v-if="sshAccount === 'custom'" v-model="sshCustomPass" placeholder="密码" type="password" show-password style="width: 120px" />
        <el-button type="primary" @click="connectSSH">连接</el-button>
      </div>
      <div v-else style="margin-bottom: 8px">
        <el-tag type="success" size="small">已连接: {{ sshConnectedUser }}</el-tag>
      </div>
      <div ref="sshTermRef" style="height: 450px; background: #0c0c0c; border-radius: 4px"></div>
    </el-dialog>

    <!-- 账号授权 -->
    <el-card style="background: #1a1a1a; border-color: #2a2a2a; margin-top: 16px">
      <template #header><span style="color: #e0e0e0; font-weight: bold">魔云腾账号授权</span></template>

      <!-- 未登录：显示账号密码输入 -->
      <div v-if="!mytStatus.loggedIn">
        <el-form :inline="true">
          <el-form-item label="账号">
            <el-input v-model="mytForm.username" placeholder="魔云腾账号" style="width: 180px" />
          </el-form-item>
          <el-form-item label="密码">
            <el-input v-model="mytForm.password" type="password" show-password placeholder="密码" style="width: 180px" />
          </el-form-item>
          <el-form-item>
            <el-button type="primary" :loading="mytLogging" @click="mytLogin">登录并同步</el-button>
          </el-form-item>
        </el-form>
      </div>

      <!-- 已登录：显示状态和操作 -->
      <div v-else>
        <el-descriptions :column="2" border size="small">
          <el-descriptions-item label="登录账号">{{ mytStatus.uname || mytStatus.username }}</el-descriptions-item>
          <el-descriptions-item label="上次同步">{{ mytStatus.lastSync || '未同步' }}</el-descriptions-item>
          <el-descriptions-item label="通讯状态">
            <el-tag :type="mytStatus.hasToken ? 'success' : 'danger'" size="small">
              {{ mytStatus.hasToken ? '正常' : '异常' }}
            </el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="自动同步">
            <el-switch :model-value="mytStatus.autoSync" @change="mytToggleAuto" />
            <span style="color: #999; font-size: 12px; margin-left: 8px">每10分钟</span>
          </el-descriptions-item>
          <el-descriptions-item label="设备绑定">
            <el-tag v-if="bindInfo.bindStatus === 1" type="success" size="small">已绑定</el-tag>
            <el-tag v-else-if="bindInfo.bindStatus === 2" type="warning" size="small">他人绑定</el-tag>
            <el-tag v-else type="info" size="small">未绑定</el-tag>
          </el-descriptions-item>
          <el-descriptions-item label="设备ID">{{ bindInfo.deviceId || '-' }}</el-descriptions-item>
        </el-descriptions>
        <div style="margin-top: 12px">
          <el-button type="primary" :loading="mytSyncing" @click="mytSync">手动同步</el-button>
          <el-button v-if="bindInfo.bindStatus !== 1" type="success" :loading="binding" @click="mytBind">绑定设备</el-button>
          <el-button v-if="bindInfo.bindStatus === 1" type="warning" @click="showUnbindDialog = true">解绑设备</el-button>
          <el-button @click="mytLogout">退出登录</el-button>
        </div>
      </div>
    </el-card>

    <!-- 解绑弹窗 -->
    <el-dialog v-model="showUnbindDialog" title="解绑设备" width="400px">
      <el-form label-width="80px">
        <el-form-item label="手机号">
          <el-input v-model="unbindForm.phone" placeholder="注册手机号" />
        </el-form-item>
        <el-form-item label="验证码">
          <div style="display: flex; gap: 8px">
            <el-input v-model="unbindForm.vcode" placeholder="输入验证码" />
            <el-button :loading="sendingVCode" :disabled="vcodeCountdown > 0" @click="sendVCode">
              {{ vcodeCountdown > 0 ? `${vcodeCountdown}s` : '发送验证码' }}
            </el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showUnbindDialog = false">取消</el-button>
        <el-button type="danger" :loading="unbinding" @click="mytUnbind">确认解绑</el-button>
      </template>
    </el-dialog>

  </div>
</template>

<script setup>
import { ref, reactive, onMounted, onBeforeUnmount, nextTick } from 'vue'
import { useDeviceStore } from '../stores/device.js'
import { ElMessage } from 'element-plus'
import { Terminal } from 'xterm'
import { FitAddon } from '@xterm/addon-fit'
import 'xterm/css/xterm.css'

const device = useDeviceStore()

const versionInfo = ref({})
const checking = ref(false)
const upgrading = ref(false)
const upgradeProgress = ref(0)
const upgradeStatus = ref('')
const upgradeProgressStatus = ref('')
// 面板更新
const panelChecking = ref(false)
const panelUpdating = ref(false)
const panelUpdateInfo = ref({})
const panelUpdateProgress = ref(0)
const panelUpdateStatus = ref('')
const panelProgressStatus = ref('')
const cleaning = ref(false)
const cleanProgress = ref(0)
const cleanStatus = ref('')
const cleanProgressStatus = ref('')
const mytForm = reactive({ username: '', password: '' })
const mytStatus = ref({})
const mytLogging = ref(false)
const mytSyncing = ref(false)
const bindInfo = ref({})
const binding = ref(false)
const showUnbindDialog = ref(false)
const unbindForm = reactive({ phone: '', vcode: '' })
const unbindVkey = ref('')
const sendingVCode = ref(false)
const vcodeCountdown = ref(0)
let vcodeTimer = null
const unbinding = ref(false)
const sshDialogVisible = ref(false)
const sshAccount = ref('user:myt')
const sshCustomUser = ref('')
const sshCustomPass = ref('')
const sshConnected = ref(false)
const sshConnectedUser = ref('')
const sshTermRef = ref(null)
let sshTerm = null
let sshSocket = null
let sshFitAddon = null
let sshHeartbeat = null

// 网络设置
const networkSettings = reactive({ publicUdpPort: '' })
const savingNetwork = ref(false)

// 监听 WS 流式进度事件
let _progressHandler = null
function listenProgress(action, onChunk, onDone) {
  // 注册到 device store 的 ws 消息监听
  return { action, onChunk, onDone }
}

onMounted(async () => {
  await fetchVersion()
  await fetchMytStatus()
  checkPanelUpdate() // 自动检查面板更新
  fetchNetworkSettings() // 加载网络设置
})

async function fetchVersion() {
  try {
    const resp = await device.request('device:version')
    const d = resp.data?.data || resp.data
    versionInfo.value = d
  } catch (e) {}
}

// 面板更新
async function checkPanelUpdate() {
  panelChecking.value = true
  try {
    const resp = await device.request('panel:checkUpdate')
    panelUpdateInfo.value = resp.data || {}
  } catch (e) {
    // 未配置更新服务器时静默失败
    panelUpdateInfo.value = {}
    try {
      const vResp = await device.request('panel:version')
      panelUpdateInfo.value = { currentVersion: vResp.data?.version || 'dev' }
    } catch {}
  } finally {
    panelChecking.value = false
  }
}

async function doPanelUpdate() {
  panelUpdating.value = true
  panelUpdateProgress.value = 0
  panelUpdateStatus.value = '正在准备更新...'
  panelProgressStatus.value = ''

  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'panel:update') {
      const d = msg.data
      if (d.phase === 'error') {
        panelUpdateStatus.value = d.message
        panelProgressStatus.value = 'exception'
        panelUpdating.value = false
        device.offEvent(handler)
        return
      }
      if (d.phase === 'restarting') {
        panelUpdateProgress.value = 100
        panelUpdateStatus.value = d.message
        panelProgressStatus.value = 'success'
        panelUpdating.value = false
        device.offEvent(handler)
        ElMessage.success('更新完成，面板正在重启...')
        // 等待后端重启完成后自动刷新页面
        setTimeout(() => { window.location.reload() }, 4000)
        return
      }
      if (d.progress !== undefined) {
        panelUpdateProgress.value = d.progress
      }
      if (d.message) {
        panelUpdateStatus.value = d.message
      }
    }
  }
  device.onEvent(handler)

  try {
    await device.request('panel:doUpdate', {}, 600000)
  } catch (e) {
    if (!panelProgressStatus.value) {
      panelUpdateStatus.value = '更新失败: ' + e.message
      panelProgressStatus.value = 'exception'
      panelUpdating.value = false
      device.offEvent(handler)
    }
  }
}

async function handleCheckUpgrade() {
  checking.value = true
  upgradeStatus.value = ''
  upgradeProgress.value = 0
  upgradeProgressStatus.value = ''
  await fetchVersion()
  checking.value = false

  const v = versionInfo.value
  if (v.currentVersion && v.latestVersion && v.currentVersion >= v.latestVersion) {
    ElMessage.success('当前已是最新版本')
    upgradeStatus.value = '已是最新版本'
    upgradeProgress.value = 100
    upgradeProgressStatus.value = 'success'
    return
  }

  // 开始升级（WS 流式）
  upgrading.value = true
  upgradeStatus.value = '正在升级...'
  upgradeProgress.value = 10

  // 监听进度事件（通过 store 事件机制，跨 WS 重连有效）
  let totalChunks = 0
  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'device:upgrade') {
      if (msg.data.done) {
        if (upgradeProgress.value < 100) {
          upgradeProgress.value = 100
          upgradeStatus.value = '升级完成'
          upgradeProgressStatus.value = 'success'
        }
        ElMessage.success(upgradeStatus.value)
        upgrading.value = false
        device.offEvent(handler)
        fetchVersion()
        return
      }
      totalChunks++
      const raw = (msg.data.chunk || '').trim()
      if (!raw) return

      // 尝试 JSON 解析，提取有效消息
      let text = raw
      try {
        const obj = JSON.parse(raw)
        if (obj.error || (obj.code && obj.code !== 0)) {
          upgradeStatus.value = obj.message || obj.error || obj.msg || '升级失败'
          upgradeProgressStatus.value = 'exception'
          return
        }
        text = obj.message || obj.msg || obj.data || raw
      } catch {
        // 非 JSON，使用原始文本
      }

      if (text.includes('最新版本')) {
        upgradeStatus.value = '已是最新版本'
        upgradeProgress.value = 100
        upgradeProgressStatus.value = 'success'
      } else {
        upgradeProgress.value = Math.min(10 + totalChunks * 15, 95)
        upgradeStatus.value = text.substring(0, 60) || '正在升级...'
      }
    }
  }
  device.onEvent(handler)

  try {
    await device.request('device:upgrade', {}, 120000)
  } catch (e) {
    upgradeStatus.value = '升级失败: ' + e.message
    upgradeProgressStatus.value = 'exception'
    ElMessage.error('升级失败: ' + e.message)
    upgrading.value = false
    device.offEvent(handler)
  }
}

async function handleReboot() {
  try {
    await device.request('device:reboot')
    ElMessage.success('重启命令已发送')
  } catch (e) {
    ElMessage.error('重启失败')
  }
}

async function handleCleanDisk() {
  cleaning.value = true
  cleanProgress.value = 5
  cleanStatus.value = '正在清空磁盘数据...'
  cleanProgressStatus.value = ''
  const totalSteps = 6

  const handler = (msg) => {
    if (msg.event === 'task:progress' && msg.data?.action === 'device:cleanDisk') {
      if (msg.data.done) {
        if (cleanProgress.value < 100) {
          cleanProgress.value = 100
          cleanStatus.value = '清空完成'
          cleanProgressStatus.value = 'success'
        }
        cleaning.value = false
        device.offEvent(handler)
        return
      }
      const raw = (msg.data.chunk || '').trim()
      if (!raw) return
      // 尝试 JSON 解析
      let text = raw
      try {
        const obj = JSON.parse(raw)
        if (obj.error || (obj.code && obj.code !== 0)) {
          cleanStatus.value = obj.message || obj.error || '清空失败'
          cleanProgressStatus.value = 'exception'
          return
        }
        text = obj.message || obj.msg || obj.data || raw
      } catch {
        // 非 JSON，使用原始文本
      }
      const lines = text.split('\n')
      for (const line of lines) {
        if (!line.trim()) continue
        const stepMatch = line.match(/\[STEP\s+(\d+)\]/i)
        if (stepMatch) {
          const step = parseInt(stepMatch[1])
          cleanProgress.value = Math.round((step / totalSteps) * 100)
          cleanStatus.value = line.trim()
        }
        if (/Reset sequence completed|Rebooting/i.test(line)) {
          cleanProgress.value = 100
          cleanStatus.value = '清空完成，设备正在重启...'
          cleanProgressStatus.value = 'success'
          ElMessage.success('设备磁盘数据已清空，正在重启')
        }
      }
    }
  }
  device.onEvent(handler)

  try {
    await device.request('device:cleanDisk', {}, 600000)
  } catch (e) {
    cleanStatus.value = '清空失败: ' + e.message
    cleanProgressStatus.value = 'exception'
    ElMessage.error('清空失败: ' + e.message)
    cleaning.value = false
    device.offEvent(handler)
  }
}

function handleLinkSSH() {
  sshDialogVisible.value = true
}

function onSshAccountChange() {}

async function connectSSH() {
  let user, pass
  if (sshAccount.value === 'custom') {
    user = sshCustomUser.value
    pass = sshCustomPass.value
    if (!user || !pass) { ElMessage.warning('请输入用户名和密码'); return }
  } else {
    [user, pass] = sshAccount.value.split(':')
  }
  await nextTick()

  sshTerm = new Terminal({
    cursorBlink: true,
    fontSize: 14,
    theme: { background: '#0c0c0c' }
  })
  sshFitAddon = new FitAddon()
  sshTerm.loadAddon(sshFitAddon)
  sshTerm.open(sshTermRef.value)
  sshFitAddon.fit()
  sshTerm.focus()
  sshTerm.write('\r\n\x1b[32m正在连接...\x1b[0m\r\n')

  const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
  const token = localStorage.getItem('token')
  sshSocket = new WebSocket(`${proto}//${location.host}/ws/ssh?token=${token}`)
  sshSocket.binaryType = 'arraybuffer'

  sshSocket.onopen = () => {
    sshConnected.value = true
    sshConnectedUser.value = user
    sshSocket.send(JSON.stringify({
      type: 'login',
      ip: '127.0.0.1',
      port: 22,
      user,
      password: pass
    }))
    const dims = sshFitAddon.proposeDimensions()
    if (dims) {
      sshSocket.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }))
    }
    sshHeartbeat = setInterval(() => {
      if (sshSocket?.readyState === WebSocket.OPEN) {
        sshSocket.send(JSON.stringify({ type: 'heartbeat' }))
      }
    }, 30000)
  }

  sshSocket.onmessage = (event) => {
    if (event.data instanceof ArrayBuffer) {
      sshTerm.write(new Uint8Array(event.data))
    } else {
      sshTerm.write(event.data)
    }
  }

  sshSocket.onclose = () => {
    sshTerm?.write('\r\n\x1b[31m连接已断开\x1b[0m\r\n')
    sshConnected.value = false
  }

  sshSocket.onerror = () => {
    sshTerm?.write('\r\n\x1b[31m连接错误\x1b[0m\r\n')
  }

  sshTerm.onData((data) => {
    if (sshSocket?.readyState === WebSocket.OPEN) {
      sshSocket.send(JSON.stringify({ type: 'stdin', data }))
    }
  })

  window._sshResize = () => {
    if (sshFitAddon && sshTerm) {
      sshFitAddon.fit()
      const dims = sshFitAddon.proposeDimensions()
      if (dims && sshSocket?.readyState === WebSocket.OPEN) {
        sshSocket.send(JSON.stringify({ type: 'resize', cols: dims.cols, rows: dims.rows }))
      }
    }
  }
  window.addEventListener('resize', window._sshResize)
}

function cleanupSSH() {
  if (sshHeartbeat) { clearInterval(sshHeartbeat); sshHeartbeat = null }
  if (sshSocket) { sshSocket.close(); sshSocket = null }
  if (sshTerm) { sshTerm.dispose(); sshTerm = null }
  if (window._sshResize) { window.removeEventListener('resize', window._sshResize); window._sshResize = null }
  sshFitAddon = null
  sshConnected.value = false
}

onBeforeUnmount(() => {
  cleanupSSH()
  if (vcodeTimer) {
    clearInterval(vcodeTimer)
    vcodeTimer = null
  }
})

async function fetchNetworkSettings() {
  try {
    const resp = await device.request('settings:get')
    const data = resp.data || {}
    networkSettings.publicUdpPort = data.public_udp_port || ''
  } catch {}
}

async function saveNetworkSettings() {
  savingNetwork.value = true
  try {
    await device.request('settings:set', { key: 'public_udp_port', value: networkSettings.publicUdpPort || '' })
    ElMessage.success('网络设置已保存')
  } catch (e) {
    ElMessage.error('保存失败: ' + (e.message || ''))
  } finally {
    savingNetwork.value = false
  }
}

async function fetchMytStatus() {
  try {
    const resp = await device.request('myt:status')
    mytStatus.value = resp.data
    if (resp.data.loggedIn && resp.data.bindDeviceID) {
      bindInfo.value = {
        deviceId: resp.data.bindDeviceID,
        bindStatus: resp.data.bindStatus
      }
    } else if (resp.data.loggedIn) {
      await fetchBindStatus()
    }
  } catch (e) {}
}

async function fetchBindStatus() {
  try {
    const resp = await device.request('myt:bindStatus')
    bindInfo.value = resp.data
  } catch (e) {}
}

async function mytLogin() {
  if (!mytForm.username || !mytForm.password) { ElMessage.warning('请输入账号和密码'); return }
  mytLogging.value = true
  try {
    await device.request('myt:login', { username: mytForm.username, password: mytForm.password })
    ElMessage.success('登录并同步成功')
    mytForm.username = ''
    mytForm.password = ''
    await fetchMytStatus()
  } catch (e) {
    ElMessage.error(e.message || '登录失败')
  } finally {
    mytLogging.value = false
  }
}

async function mytSync() {
  mytSyncing.value = true
  try {
    await device.request('myt:sync')
    ElMessage.success('同步成功')
    await fetchMytStatus()
  } catch (e) {
    ElMessage.error(e.message || '同步失败')
  } finally {
    mytSyncing.value = false
  }
}

async function mytToggleAuto(val) {
  try {
    await device.request('myt:autoToggle', { autoSync: val })
    await fetchMytStatus()
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function mytLogout() {
  try {
    await device.request('myt:logout')
    mytStatus.value = {}
    bindInfo.value = {}
    ElMessage.success('已退出')
  } catch (e) {
    ElMessage.error('操作失败')
  }
}

async function mytBind() {
  binding.value = true
  try {
    await device.request('myt:bind')
    ElMessage.success('绑定成功')
    await fetchMytStatus()
  } catch (e) {
    ElMessage.error(e.message || '绑定失败')
  } finally {
    binding.value = false
  }
}

async function sendVCode() {
  if (!unbindForm.phone) { ElMessage.warning('请输入手机号'); return }
  sendingVCode.value = true
  try {
    const resp = await device.request('myt:vcode', { phone: unbindForm.phone })
    unbindVkey.value = resp.data.vkey
    ElMessage.success('验证码已发送')
    vcodeCountdown.value = 60
    if (vcodeTimer) clearInterval(vcodeTimer)
    vcodeTimer = setInterval(() => {
      vcodeCountdown.value--
      if (vcodeCountdown.value <= 0) {
        clearInterval(vcodeTimer)
        vcodeTimer = null
      }
    }, 1000)
  } catch (e) {
    ElMessage.error(e.message || '发送失败')
  } finally {
    sendingVCode.value = false
  }
}

async function mytUnbind() {
  if (!unbindForm.vcode) { ElMessage.warning('请输入验证码'); return }
  unbinding.value = true
  try {
    await device.request('myt:unbind', { vcode: unbindForm.vcode, vkey: unbindVkey.value })
    ElMessage.success('解绑成功')
    showUnbindDialog.value = false
    unbindForm.phone = ''
    unbindForm.vcode = ''
    unbindVkey.value = ''
    await fetchMytStatus()
  } catch (e) {
    ElMessage.error(e.message || '解绑失败')
  } finally {
    unbinding.value = false
  }
}

</script>
