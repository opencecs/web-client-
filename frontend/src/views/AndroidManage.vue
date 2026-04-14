<template>
  <div style="padding: 24px">
    <el-tabs v-model="activeTab" type="border-card" style="background: #1a1a1a; border-color: #2a2a2a">
      <el-tab-pane label="云机管理" name="slots">
        <!-- 操作栏 -->
        <SlotActions
          :selected="selectedSlots"
          @create="openCreate"
          @projection="openProjection"
          @close-all-projections="closeAllProjections"
          @terminal="openTerminal"
          @backup-switch="openBackupSwitch"
          @rename="openAlias"
          @copy="openCopy"
          @s5proxy="openS5Proxy"
        />
        <!-- 方块网格 -->
        <SlotGrid ref="slotGridRef" :max-slots="maxSlots" @selection-change="onSelectionChange" />

        <!-- 截图预览 -->
        <div style="margin-top: 20px">
          <h4 style="color: #e0e0e0; margin-bottom: 10px">实时截图</h4>
          <SlotScreenshots :max-slots="maxSlots" @projection="openProjection" />
        </div>
      </el-tab-pane>
      <el-tab-pane v-if="auth.can('image_view')" label="镜像管理" name="images" lazy>
        <ImageManage />
      </el-tab-pane>
      <el-tab-pane v-if="auth.can('network_bridge')" label="虚拟网卡" name="network" lazy>
        <NetworkTab />
      </el-tab-pane>
      <el-tab-pane v-if="auth.can('vpc_manage')" label="VPC 管理" name="vpc" lazy>
        <VpcManageTab />
      </el-tab-pane>
    </el-tabs>

    <!-- 创建容器弹窗 -->
    <CreateContainer v-model="createVisible" :max-slots="maxSlots" :default-slot="createDefaultSlot"
      @created="device.refreshContainers()" />

    <!-- 备份切换弹窗 -->
    <BackupSwitch v-model="backupSwitchVisible" :slot-num="backupSwitchSlot" />

    <!-- 设置别名弹窗 -->
    <el-dialog v-model="aliasVisible" title="设置别名" width="400px">
      <el-form label-width="80px">
        <el-form-item v-if="aliasTarget" label="容器 ID">
          <el-input :model-value="aliasTarget.name" readonly />
        </el-form-item>
        <el-form-item v-if="isBatchAlias" label="批量设置">
          <span style="color: #e6a23c; font-size: 13px">将为 {{ aliasBatchTargets.length }} 个容器设置别名（自动加坑位号后缀）</span>
        </el-form-item>
        <el-form-item v-if="aliasTarget && currentAlias" label="当前别名">
          <span style="color: #999">{{ currentAlias }}</span>
        </el-form-item>
        <el-form-item label="新别名">
          <el-input v-model="aliasInput" :placeholder="isBatchAlias ? '输入别名前缀（如：游戏）' : '输入别名（支持中文、空格、符号）'" clearable />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="aliasVisible = false">取消</el-button>
        <el-button v-if="currentAlias || isBatchAlias" type="danger" :loading="aliasSaving" @click="doRemoveAlias">清除别名</el-button>
        <el-button type="primary" :loading="aliasSaving" @click="doSetAlias">保存</el-button>
      </template>
    </el-dialog>

    <!-- 复制弹窗 -->
    <el-dialog v-model="copyVisible" title="复制容器" width="400px">
      <el-form label-width="90px">
        <el-form-item label="源容器">{{ device.displayName(copyTarget?.name) }}</el-form-item>
        <el-form-item label="目标坑位">
          <el-input-number v-model="copySlot" :min="1" :max="maxSlots" :step="1" />
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

    <!-- 多投屏窗口 -->
    <ContainerProjection
      v-for="(p, idx) in projections"
      :key="p.name"
      v-model="p.visible"
      :container="p.container"
      :offset-index="idx"
      @close="removeProjection"
    />

    <!-- 终端弹窗 -->
    <ContainerTerminal v-model="terminalVisible" :container="terminalContainer" />

    <!-- S5 代理弹窗 -->
    <el-dialog v-model="s5Visible" title="S5 代理管理" width="500px" @opened="fetchS5Status">
      <div v-if="s5Container" style="margin-bottom: 12px">
        <span style="color: #999">容器：</span>
        <span style="color: #e0e0e0">{{ device.displayName(s5Container.name) }}</span>
      </div>

      <!-- 当前状态 -->
      <div v-if="s5Status.status === 1" style="padding: 14px 16px; border-radius: 6px; margin-bottom: 16px; background: #162312; border: 1px solid #67c23a">
        <div style="display: flex; align-items: center; justify-content: space-between">
          <div style="display: flex; align-items: center; gap: 10px">
            <el-tag type="success" size="small" effect="dark">已启动</el-tag>
            <span style="color: #e0e0e0; font-size: 13px; font-family: monospace">{{ s5Status.addr }}</span>
          </div>
          <el-button type="danger" size="small" :loading="s5Stopping" @click="doStopS5">停止代理</el-button>
        </div>
      </div>
      <div v-else style="padding: 14px 16px; border-radius: 6px; margin-bottom: 16px; background: #1e1e1e; border: 1px dashed #555; text-align: center">
        <el-tag type="info" size="small">未启动</el-tag>
        <span style="color: #666; margin-left: 8px; font-size: 13px">当前未配置 S5 代理</span>
      </div>

      <!-- 快捷解析 -->
      <div style="display: flex; gap: 8px; margin-bottom: 14px; align-items: center">
        <div style="color: #999; font-size: 12px; white-space: nowrap">S5信息</div>
        <el-input v-model="s5QuickInput" placeholder="格式: 地址:端口:用户名:密码（用户名密码可省略）" size="small" />
        <el-button size="small" @click="parseS5Quick">解析并填写</el-button>
      </div>

      <!-- 设置表单 -->
      <div style="color: #e0e0e0; font-weight: bold; margin-bottom: 10px">{{ s5Status.status === 1 ? '修改代理' : '设置代理' }}</div>
      <div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px; margin-bottom: 12px">
        <div>
          <div style="color: #999; font-size: 12px; margin-bottom: 4px">服务器 IP</div>
          <el-input v-model="s5Form.addr" placeholder="如 1.2.3.4" />
        </div>
        <div>
          <div style="color: #999; font-size: 12px; margin-bottom: 4px">端口</div>
          <el-input v-model="s5Form.port" placeholder="如 1080" />
        </div>
        <div>
          <div style="color: #999; font-size: 12px; margin-bottom: 4px">用户名</div>
          <el-input v-model="s5Form.usr" placeholder="无认证可留空" />
        </div>
        <div>
          <div style="color: #999; font-size: 12px; margin-bottom: 4px">密码</div>
          <el-input v-model="s5Form.pwd" placeholder="无认证可留空" type="password" show-password />
        </div>
      </div>
      <div style="margin-bottom: 12px">
        <div style="color: #999; font-size: 12px; margin-bottom: 4px">域名解析模式</div>
        <el-radio-group v-model="s5Form.type">
          <el-radio value="1">本地解析</el-radio>
          <el-radio value="2">服务端解析</el-radio>
        </el-radio-group>
      </div>

      <template #footer>
        <el-button @click="s5Visible = false">关闭</el-button>
        <el-button type="primary" :loading="s5Setting" @click="doSetS5">
          {{ s5Status.status === 1 ? '更新代理' : '启动代理' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from '../stores/auth.js'
import { useDeviceStore } from '../stores/device.js'
import SlotGrid from '../components/android/SlotGrid.vue'
import SlotActions from '../components/android/SlotActions.vue'
import SlotScreenshots from '../components/android/SlotScreenshots.vue'
import CreateContainer from '../components/android/CreateContainer.vue'
import BackupSwitch from '../components/android/BackupSwitch.vue'
import ImageManage from '../components/android/ImageManage.vue'
import NetworkTab from '../components/android/NetworkTab.vue'
import VpcManageTab from '../components/android/VpcManageTab.vue'
import ContainerProjection from '../components/android/ContainerProjection.vue'
import ContainerTerminal from '../components/android/ContainerTerminal.vue'
import { reactive } from 'vue'

const auth = useAuthStore()
const device = useDeviceStore()
const activeTab = ref('slots')
const slotGridRef = ref(null)
const selectedSlots = ref([])

const maxSlots = computed(() => {
  const model = (device.status?.model || '').toLowerCase()
  return model.includes('p1') ? 24 : 12
})

function onSelectionChange(slots) {
  selectedSlots.value = slots
}

// 创建容器
const createVisible = ref(false)
const createDefaultSlot = ref(1)
function openCreate() {
  const single = selectedSlots.value.length === 1 ? selectedSlots.value[0] : null
  createDefaultSlot.value = single?.num || 1
  createVisible.value = true
}

// 备份切换
const backupSwitchVisible = ref(false)
const backupSwitchSlot = ref(0)
function openBackupSwitch(slot) {
  backupSwitchSlot.value = slot.num
  backupSwitchVisible.value = true
}

// 设置别名
const aliasVisible = ref(false)
const aliasTarget = ref(null) // 单选时为容器对象，多选时为 null
const aliasBatchTargets = ref([]) // 多选时的容器列表
const aliasInput = ref('')
const aliasSaving = ref(false)
const isBatchAlias = computed(() => aliasBatchTargets.value.length > 1)
const currentAlias = computed(() => {
  if (!aliasTarget.value) return ''
  return device.containerAliases[aliasTarget.value.name] || ''
})
function openAlias(container) {
  if (container) {
    // 单选
    aliasTarget.value = container
    aliasBatchTargets.value = [container]
    aliasInput.value = device.containerAliases[container.name] || ''
  } else {
    // 多选：从选中坑位取每个坑位的代表容器
    const targets = []
    for (const slot of selectedSlots.value) {
      const containers = device.containers.filter(c => c.indexNum === slot.num)
      const running = containers.find(c => c.status === 'running')
      const active = running || containers[0]
      if (active) targets.push(active)
    }
    if (!targets.length) { ElMessage.warning('选中的坑位没有容器'); return }
    aliasTarget.value = targets.length === 1 ? targets[0] : null
    aliasBatchTargets.value = targets
    aliasInput.value = ''
  }
  aliasVisible.value = true
}
async function doSetAlias() {
  const alias = aliasInput.value.trim()
  if (!alias) { aliasVisible.value = false; return }
  aliasSaving.value = true
  try {
    if (aliasBatchTargets.value.length === 1) {
      await device.setAlias(aliasBatchTargets.value[0].name, alias)
    } else {
      // 批量：别名 + 坑位号后缀
      for (const c of aliasBatchTargets.value) {
        await device.setAlias(c.name, `${alias}-${c.indexNum}`)
      }
    }
    ElMessage.success('别名设置成功')
    aliasVisible.value = false
  } catch (e) {
    ElMessage.error(e.message || '设置失败')
  } finally { aliasSaving.value = false }
}
async function doRemoveAlias() {
  aliasSaving.value = true
  try {
    for (const c of aliasBatchTargets.value) {
      if (device.containerAliases[c.name]) {
        await device.removeAlias(c.name)
      }
    }
    ElMessage.success('别名已清除')
    aliasVisible.value = false
  } catch (e) {
    ElMessage.error(e.message || '清除失败')
  } finally { aliasSaving.value = false }
}

// 复制
const copyVisible = ref(false)
const copyTarget = ref(null)
const copySlot = ref(1)
const copyCount = ref(1)
const copying = ref(false)
function openCopy(container) {
  copyTarget.value = container
  copySlot.value = container.indexNum || 1
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
  } finally { copying.value = false }
}

// 多投屏
const projections = reactive([])
function openProjection(container) {
  const existing = projections.find(p => p.name === container.name)
  if (existing) { existing.visible = true; existing.container = container; return }
  projections.push({ name: container.name, container, visible: true })
}
function removeProjection(name) {
  const idx = projections.findIndex(p => p.name === name)
  if (idx !== -1) projections.splice(idx, 1)
}
function closeAllProjections() {
  projections.splice(0, projections.length)
}

// 终端
const terminalVisible = ref(false)
const terminalContainer = ref(null)
function openTerminal(container) {
  terminalContainer.value = container
  terminalVisible.value = true
}

// S5 代理
const s5Visible = ref(false)
const s5Container = ref(null)
const s5Status = reactive({ status: 0, statusText: '未启动', addr: '', type: 0 })
const s5Form = reactive({ addr: '', port: '', usr: '', pwd: '', type: '1' })
const s5Setting = ref(false)
const s5Stopping = ref(false)
const s5QuickInput = ref('')

function parseS5Quick() {
  const raw = s5QuickInput.value.trim()
  if (!raw) { ElMessage.warning('请输入 S5 信息'); return }
  const parts = raw.split(':')
  if (parts.length < 2) { ElMessage.warning('格式错误，至少需要 地址:端口'); return }
  s5Form.addr = parts[0]
  s5Form.port = parts[1]
  s5Form.usr = parts[2] || ''
  s5Form.pwd = parts[3] || ''
  ElMessage.success('已解析并填写')
}

function openS5Proxy(container) {
  if (!container) { ElMessage.warning('请选择一个运行中的容器'); return }
  s5Container.value = container
  s5Form.addr = ''; s5Form.port = ''; s5Form.usr = ''; s5Form.pwd = ''; s5Form.type = '1'
  s5QuickInput.value = ''
  Object.assign(s5Status, { status: 0, statusText: '未启动', addr: '', type: 0 })
  s5Visible.value = true
}

async function fetchS5Status() {
  if (!s5Container.value) return
  try {
    const resp = await device.request('proxy:status', { name: s5Container.value.name })
    const d = resp.data?.data || resp.data || {}
    Object.assign(s5Status, { status: d.status || 0, statusText: d.statusText || '未启动', addr: d.addr || '', type: d.type || 0 })
  } catch {}
}

async function doSetS5() {
  if (!s5Form.addr || !s5Form.port) { ElMessage.warning('请填写 IP 和端口'); return }
  s5Setting.value = true
  try {
    await device.request('proxy:set', {
      name: s5Container.value.name,
      addr: s5Form.addr,
      port: s5Form.port,
      usr: s5Form.usr,
      pwd: s5Form.pwd,
      type: s5Form.type
    })
    ElMessage.success('代理设置成功')
    await fetchS5Status()
  } catch (e) { ElMessage.error(e.message || '设置失败') }
  finally { s5Setting.value = false }
}

async function doStopS5() {
  s5Stopping.value = true
  try {
    await device.request('proxy:stop', { name: s5Container.value.name })
    ElMessage.success('代理已停止')
    await fetchS5Status()
  } catch (e) { ElMessage.error(e.message || '停止失败') }
  finally { s5Stopping.value = false }
}
</script>
