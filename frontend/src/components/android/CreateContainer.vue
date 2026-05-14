<template>
  <el-dialog :modelValue="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    title="创建容器" width="950px" :close-on-click-modal="false" destroy-on-close>
    <el-form :model="form" label-width="100px" size="small">
      <el-row :gutter="24">
        <!-- 左侧 -->
        <el-col :span="12">
          <el-form-item label="容器别名">
            <el-input v-model="form.alias" placeholder="可选，创建后自动设置别名" clearable />
            <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">容器名称自动生成，别名用于显示</div>
          </el-form-item>

          <el-form-item label="坑位号">
            <div style="margin-bottom: 4px">
              <el-button size="small" text @click="selectAllSlots">全选</el-button>
              <el-button size="small" text @click="clearSlots">清空</el-button>
              <span style="color: #b0b0b0; font-size: 11px; margin-left: 8px" v-if="selectedSlots.size > 0">
                已选 {{ selectedSlots.size }} 个坑位
              </span>
            </div>
            <div style="display: flex; flex-wrap: wrap; gap: 6px">
              <div v-for="slot in slotList" :key="slot.num"
                :class="['slot-item', getSlotClass(slot.num), { active: selectedSlots.has(slot.num) }]"
                @click="toggleSlot(slot.num)"
                :title="getSlotTitle(slot)">
                {{ slot.num }}
              </div>
            </div>
            <div style="color: #b0b0b0; font-size: 11px; margin-top: 4px">
              支持多选，每个坑位创建一个容器；同一坑位可创建多个容器，但同时只能运行一个
            </div>
            <div v-if="hasOccupiedSlot" style="color: #E6A23C; font-size: 11px; margin-top: 4px">
              ⚠ 部分选中坑位已有运行中容器，新容器将以关机状态创建（不自动启动）
            </div>
          </el-form-item>

          <el-form-item label="安卓版本" required>
            <el-radio-group v-model="androidVersion" @change="onAndroidVersionChange">
              <el-radio value="and16">Android 16</el-radio>
              <el-radio value="and14">Android 14</el-radio>
            </el-radio-group>
          </el-form-item>

          <el-form-item label="镜像来源" required>
            <el-select v-model="form.imageUrl" filterable
              placeholder="选择镜像" style="width: 100%" :loading="loadingMirrors">
              <el-option v-for="img in filteredMirrors" :key="img.url" :label="img.name" :value="img.url" />
            </el-select>
          </el-form-item>

          <el-form-item label="DNS">
            <el-select v-model="dnsOption" style="width: 100%" @change="onDnsChange">
              <el-option label="223.5.5.5 (阿里DNS)" value="223.5.5.5" />
              <el-option label="8.8.8.8 (Google DNS)" value="8.8.8.8" />
              <el-option label="114.114.114.114" value="114.114.114.114" />
              <el-option label="自定义" value="custom" />
            </el-select>
            <el-input v-if="dnsOption === 'custom'" v-model="form.dns" placeholder="输入DNS地址"
              style="margin-top: 6px" />
          </el-form-item>

          <el-form-item label="沙盒大小">
            <el-select v-model="form.sandboxSize" style="width: 100%">
              <el-option label="16 GB" value="16GB" />
              <el-option label="32 GB" value="32GB" />
              <el-option label="64 GB" value="64GB" />
              <el-option label="128 GB" value="128GB" />
            </el-select>
          </el-form-item>

          <el-form-item label="锁屏密码">
            <el-input v-model="form.PINCode" placeholder="4-8位数字，留空不设" maxlength="8" />
          </el-form-item>

          <el-form-item label="自动启动">
            <el-switch v-model="form.start" />
          </el-form-item>

          <el-form-item label="国家代码">
            <el-select v-model="form.countryCode" filterable clearable placeholder="留空默认"
              style="width: 100%" :loading="loadingCountryCodes">
              <el-option v-for="c in countryOptions" :key="c.code" :label="c.label" :value="c.code" />
            </el-select>
          </el-form-item>

          <!-- GPS 定位 -->
          <el-divider content-position="left">GPS 定位</el-divider>
          <el-form-item label="快捷定位">
            <el-select v-model="gpsPreset" clearable placeholder="选择城市预设" style="width: 100%"
              @change="onGpsPreset">
              <el-option label="不设置" value="" />
              <el-option label="纽约 (美国)" value="40.7128,-74.0060" />
              <el-option label="洛杉矶 (美国)" value="34.0522,-118.2437" />
              <el-option label="伦敦 (英国)" value="51.5074,-0.1278" />
              <el-option label="东京 (日本)" value="35.6762,139.6503" />
              <el-option label="首尔 (韩国)" value="37.5665,126.9780" />
              <el-option label="新加坡" value="1.3521,103.8198" />
              <el-option label="悉尼 (澳大利亚)" value="33.8688,151.2093" />
              <el-option label="迪拜 (阿联酋)" value="25.2048,55.2708" />
              <el-option label="自定义坐标" value="custom" />
            </el-select>
          </el-form-item>
          <template v-if="gpsPreset === 'custom'">
            <el-form-item label="纬度">
              <el-input v-model="form.latitude" placeholder="如 39.916527" />
            </el-form-item>
            <el-form-item label="经度">
              <el-input v-model="form.longitude" placeholder="如 116.397128" />
            </el-form-item>
          </template>
          <div v-else-if="form.latitude && form.longitude"
            style="padding: 0 0 8px 100px; color: #b0b0b0; font-size: 12px">
            坐标: {{ form.latitude }}, {{ form.longitude }}
          </div>
        </el-col>

        <!-- 右侧 -->
        <el-col :span="12">
          <el-form-item label="分辨率">
            <el-select v-model="resolutionPreset" placeholder="选择分辨率" style="width: 100%"
              @change="onResolutionPreset">
              <el-option label="机型默认分辨率" value="default" />
              <el-option label="720 × 1280" value="720x1280x320" />
              <el-option label="1080 × 1920" value="1080x1920x420" />
              <el-option label="1200 × 1920（平板）" value="1200x1920x240" />
              <el-option label="1600 × 2560（平板）" value="1600x2560x320" />
              <el-option label="自定义分辨率" value="custom" />
            </el-select>
          </el-form-item>
          <template v-if="resolutionPreset === 'custom'">
            <el-form-item label="宽">
              <el-input v-model="form.doboxWidth" placeholder="如 720" />
            </el-form-item>
            <el-form-item label="高">
              <el-input v-model="form.doboxHeight" placeholder="如 1280" />
            </el-form-item>
            <el-form-item label="DPI">
              <el-input v-model="form.doboxDpi" placeholder="如 320" />
            </el-form-item>
          </template>
          <el-form-item label="帧率">
            <el-input v-model="form.doboxFps" placeholder="默认 60" />
          </el-form-item>

          <el-divider content-position="left">机型设置</el-divider>
          <el-form-item label="手机型号">
            <el-select v-model="form.modelId" filterable clearable placeholder="留空随机分配"
              style="width: 100%" @change="onModelChange">
              <el-option v-for="m in filteredPhoneModels" :key="m.id || m.modelId"
                :label="m.name || m.modelName" :value="m.id || m.modelId" />
            </el-select>
            <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">
              已过滤为 {{ androidVersion === 'and16' ? 'Android 16' : 'Android 14' }} 机型
              （{{ filteredPhoneModels.length }} 个）
            </div>
          </el-form-item>

          <el-divider content-position="left">专项功能</el-divider>
          <el-form-item label="Magisk">
            <el-switch v-model="mgEnabled" />
            <span class="feature-desc">内置 Magisk 框架，用于模块管理和系统修改，不代表已获取 Root 权限</span>
          </el-form-item>
          <el-form-item label="GMS">
            <el-switch v-model="gmsEnabled" />
            <span class="feature-desc">Google 移动服务框架，提供 Play 商店、推送通知等 Google 服务支持</span>
          </el-form-item>
          <el-form-item label="安全模式">
            <el-switch v-model="enforceEnabled" />
            <span class="feature-desc">启用 SELinux 严格模式，拦截不安全的权限操作，增强系统安全性</span>
          </el-form-item>
          <el-form-item label="随机文件">
            <el-switch v-model="form.randomFile" />
            <span class="feature-desc">开启后系统程序的文件哈希值将在每次创建时重新生成，增强设备唯一性</span>
          </el-form-item>

          <el-divider content-position="left">SOCKS5 代理</el-divider>
          <el-form-item label="代理类型">
            <el-select v-model="form.s5Type" style="width: 100%">
              <el-option label="不开启代理" value="0" />
              <el-option label="本地域名解析 (tun2socks)" value="1" />
              <el-option label="服务器域名解析 (tun2proxy)" value="2" />
            </el-select>
          </el-form-item>
          <template v-if="form.s5Type !== '0'">
            <el-form-item label="代理 IP">
              <el-input v-model="form.s5IP" placeholder="代理服务器 IP" />
            </el-form-item>
            <el-form-item label="代理端口">
              <el-input v-model="form.s5Port" placeholder="代理端口" />
            </el-form-item>
            <el-form-item label="用户名">
              <el-input v-model="form.s5User" />
            </el-form-item>
            <el-form-item label="密码">
              <el-input v-model="form.s5Password" type="password" show-password />
            </el-form-item>
          </template>

          <el-divider content-position="left">虚拟网卡</el-divider>
          <el-form-item label="网卡选择">
            <el-select v-model="form.bridge" clearable placeholder="默认网卡（不指定）" style="width: 100%"
              :loading="loadingBridges">
              <el-option v-for="b in bridgeOptions" :key="b.name" :label="`${b.name}（${b.cidr}）`" :value="b.name" />
            </el-select>
            <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">
              为容器指定独立的虚拟网卡，不同网卡的容器网络互相隔离。留空则使用系统默认网卡。
            </div>
          </el-form-item>
        </el-col>
      </el-row>
    </el-form>

    <!-- 任务进度 -->
    <div v-if="taskPhase" style="margin-top: 12px; padding: 16px; background: #252525; border-radius: 6px">
      <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 10px">
        <span style="color: #f0f0f0; font-weight: bold">{{ phaseLabel }}</span>
        <el-space>
          <span v-if="!taskDone" style="color: #b0b0b0; font-size: 12px">已等待 {{ elapsedTime }}</span>
          <el-tag :type="taskTagType" size="small">{{ phaseTag }}</el-tag>
        </el-space>
      </div>
      <template v-if="taskPhase === 'pulling'">
        <el-progress :percentage="pullPercent" :stroke-width="12" :show-text="false" striped striped-flow />
        <div style="color: #b0b0b0; font-size: 12px; margin-top: 6px">{{ pullStatusText }}</div>
      </template>
      <template v-else-if="taskPhase === 'extracting'">
        <el-progress :percentage="50" :stroke-width="12" :show-text="false" striped striped-flow :indeterminate="true" />
        <div style="color: #b0b0b0; font-size: 12px; margin-top: 6px">正在解压镜像层...</div>
      </template>
      <template v-else-if="taskPhase === 'creating'">
        <el-progress :percentage="createTotal ? Math.round(createCurrent / createTotal * 100) : 0" :stroke-width="12" :show-text="false" striped striped-flow />
        <div style="color: #b0b0b0; font-size: 12px; margin-top: 6px">
          {{ createSlotNum > 0 ? `正在创建 ${createCurrent}/${createTotal} (坑位 ${createSlotNum})...` : `等待设备就绪，准备创建下一个 (${createCurrent}/${createTotal})...` }}
        </div>
      </template>
      <template v-else-if="taskPhase === 'done' || taskPhase === 'failed'">
        <el-progress :percentage="100" :status="taskPhase === 'failed' ? 'exception' : 'success'" :stroke-width="12" />
      </template>
      <div v-if="taskError" style="margin-top: 8px">
        <el-alert type="error" :closable="false" :title="taskError" :description="taskErrorDetail" show-icon />
      </div>
    </div>

    <template #footer>
      <span v-if="createTip" style="color: #e6a23c; font-size: 13px; margin-right: 12px">{{ createTip }}</span>
      <el-button @click="onClose">{{ taskDone ? '关闭' : '取消' }}</el-button>
      <el-button v-if="!taskPhase" type="primary" :loading="creating" @click="doCreate">创建</el-button>
    </template>
  </el-dialog>
</template>

<script setup>
import { ref, reactive, computed, watch, onBeforeUnmount } from 'vue'
import { ElMessage } from 'element-plus'
import { countryMap } from '../../utils/countryData.js'
import { pullImage } from '../../utils/pullImage.js'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  modelValue: Boolean,
  maxSlots: { type: Number, default: 12 },
  defaultSlot: { type: Number, default: 1 }
})
const emit = defineEmits(['update:modelValue', 'created'])

// 生成随机名称：8位字母+数字
function generateName() {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let name = ''
  for (let i = 0; i < 8; i++) name += chars[Math.floor(Math.random() * chars.length)]
  return name
}

// 表单默认值
const defaultForm = () => ({
  alias: '',
  imageUrl: '',
  dns: '223.5.5.5',
  sandboxSize: '32GB',
  doboxWidth: '',
  doboxHeight: '',
  doboxFps: '60',
  doboxDpi: '',
  modelId: '',
  modelName: '',
  PINCode: '',
  start: true,
  countryCode: '',
  s5Type: '0',
  s5IP: '',
  s5Port: '',
  s5User: '',
  s5Password: '',
  randomFile: true,
  longitude: '',
  latitude: '',
  bridge: '',
})

const form = reactive(defaultForm())
const selectedSlots = ref(new Set([props.defaultSlot || 1]))
const device = useDeviceStore()

// 多选坑位
function toggleSlot(num) {
  const newSet = new Set(selectedSlots.value)
  if (newSet.has(num)) newSet.delete(num)
  else newSet.add(num)
  selectedSlots.value = newSet
}
function selectAllSlots() {
  const newSet = new Set()
  for (let i = 1; i <= props.maxSlots; i++) newSet.add(i)
  selectedSlots.value = newSet
}
function clearSlots() {
  selectedSlots.value = new Set()
}

// 检测选中坑位是否有运行中的容器
const hasOccupiedSlot = computed(() => {
  return [...selectedSlots.value].some(num =>
    device.containers.some(c => c.indexNum === num && c.status === 'running')
  )
})

// 状态
const creating = ref(false)
const createTip = ref('')
const taskPhase = ref(null)
const taskDone = ref(false)
const taskError = ref('')
const taskErrorDetail = ref('')
const elapsedSeconds = ref(0)
let elapsedTimer = null
let abortController = null
const pullCurrent = ref(0)
const pullTotal = ref(0)
const pullStatusText = ref('')
const createCurrent = ref(0)
const createTotal = ref(0)
const createSlotNum = ref(0)

const androidVersion = ref('and16')
const dnsOption = ref('223.5.5.5')
const gpsPreset = ref('')
const resolutionPreset = ref('default')
const mgEnabled = ref(false)
const gmsEnabled = ref(false)
const enforceEnabled = ref(true)

// 数据
const mirrors = ref([])
const phoneModels = ref([])
const countryCodes = ref([])
const slotStates = ref({})
const loadingMirrors = ref(false)
const loadingCountryCodes = ref(false)
const bridgeOptions = ref([])
const loadingBridges = ref(false)

// 坑位列表
const slotList = computed(() => {
  const list = []
  for (let i = 1; i <= props.maxSlots; i++) {
    list.push({ num: i })
  }
  return list
})

function getSlotClass(num) {
  const info = slotStates.value[String(num)]
  if (!info) return 'slot-gray'
  if (info.state === 0) return 'slot-blue'
  if (info.state === 1) return 'slot-yellow'
  if (info.state === 2) return 'slot-red'
  return 'slot-gray'
}

function getSlotTitle(slot) {
  const info = slotStates.value[String(slot.num)]
  if (!info) return `坑位 ${slot.num} (未授权)`
  if (info.state === 0) return `坑位 ${slot.num} (正常)`
  if (info.state === 1) return `坑位 ${slot.num} (即将到期)`
  if (info.state === 2) return `坑位 ${slot.num} (已到期)`
  return `坑位 ${slot.num}`
}

// 按安卓版本过滤镜像，版本号大的排前面
const filteredMirrors = computed(() => {
  return mirrors.value
    .filter(m => m.os_ver === androidVersion.value)
    .sort((a, b) => (b.name || '').localeCompare(a.name || '', undefined, { numeric: true }))
})

// 按安卓版本过滤机型
const selectedAndroidVer = computed(() => androidVersion.value === 'and14' ? '14' : '16')
const filteredPhoneModels = computed(() => {
  if (!selectedAndroidVer.value) return phoneModels.value
  return phoneModels.value.filter(m => m.android_version === selectedAndroidVer.value)
})

// 切换安卓版本时清空镜像和机型选择
function onAndroidVersionChange() {
  form.imageUrl = ''
  form.modelId = ''
  form.modelName = ''
}

function onModelChange(id) {
  const model = phoneModels.value.find(m => (m.id || m.modelId) === id)
  form.modelName = model ? (model.name || model.modelName || '') : ''
}

function onDnsChange(val) {
  if (val !== 'custom') form.dns = val
  else form.dns = ''
}

function onResolutionPreset(val) {
  if (val === 'default' || val === 'custom') {
    form.doboxWidth = ''; form.doboxHeight = ''; form.doboxDpi = ''
    return
  }
  const parts = val.split('x')
  if (parts.length === 3) {
    form.doboxWidth = parts[0]; form.doboxHeight = parts[1]; form.doboxDpi = parts[2]
  }
}

function onGpsPreset(val) {
  if (!val || val === 'custom') {
    if (val !== 'custom') { form.latitude = ''; form.longitude = '' }
    return
  }
  const [lat, lng] = val.split(',')
  form.latitude = lat; form.longitude = lng
}

const countryOptions = computed(() => {
  return countryCodes.value.map(c => {
    const local = countryMap[c.countryCode]
    const en = local ? local.en : ''
    const label = en ? `${c.countryName} / ${en} (${c.countryCode})` : `${c.countryName} (${c.countryCode})`
    return { code: c.countryCode, label }
  })
})

// 计算属性
const elapsedTime = computed(() => {
  const s = elapsedSeconds.value
  const min = Math.floor(s / 60)
  const sec = s % 60
  return min > 0 ? `${min}分${sec}秒` : `${sec}秒`
})
const pullPercent = computed(() => {
  if (!pullTotal.value) return 0
  return Math.min(99, Math.round(pullCurrent.value / pullTotal.value * 100))
})
const phaseLabel = computed(() => {
  if (taskPhase.value === 'creating' && createTotal.value > 1) {
    return `正在批量创建容器 (${createCurrent.value}/${createTotal.value})...`
  }
  const map = { pulling: '正在下载镜像...', extracting: '正在解压镜像...', creating: '正在创建容器...', done: '创建完成', failed: '创建失败' }
  return map[taskPhase.value] || ''
})
const phaseTag = computed(() => {
  if (taskPhase.value === 'done') return '完成'
  if (taskPhase.value === 'failed') return '失败'
  return '进行中'
})
const taskTagType = computed(() => {
  if (taskPhase.value === 'done') return 'success'
  if (taskPhase.value === 'failed') return 'danger'
  return ''
})

// 打开弹窗时重置
watch(() => props.modelValue, (val) => {
  if (val) {
    Object.assign(form, defaultForm())
    selectedSlots.value = new Set()
    androidVersion.value = 'and16'
    dnsOption.value = '223.5.5.5'
    gpsPreset.value = ''
    resolutionPreset.value = 'default'
    mgEnabled.value = false
    gmsEnabled.value = false
    enforceEnabled.value = true
    taskPhase.value = null
    taskDone.value = false
    taskError.value = ''
    taskErrorDetail.value = ''
    pullCurrent.value = 0; pullTotal.value = 0; pullStatusText.value = ''
    stopTask()
    loadData()
  }
})

async function loadData() {
  // 并行加载所有数据
  const [slotResp, mirrorResp, countryResp, phoneResp, bridgeResp] = await Promise.allSettled([
    device.request('myt:slotStates'),
    (loadingMirrors.value = true, device.request('device:mirrors')),
    (loadingCountryCodes.value = true, device.request('sdk:getCountryCodes')),
    device.request('sdk:getPhoneModels'),
    (loadingBridges.value = true, device.request('sdk:listBridges')),
  ])
  // 坑位授权
  if (slotResp.status === 'fulfilled') {
    slotStates.value = slotResp.value.data?.slots || {}
  }
  // 镜像
  if (mirrorResp.status === 'fulfilled') {
    mirrors.value = mirrorResp.value.data || []
  }
  loadingMirrors.value = false
  // 国家代码
  if (countryResp.status === 'fulfilled') {
    const d = countryResp.value.data
    const cl = d?.data?.list || d?.list || d?.data || d || []
    countryCodes.value = Array.isArray(cl) ? cl : []
  }
  loadingCountryCodes.value = false
  // 机型
  if (phoneResp.status === 'fulfilled') {
    const d = phoneResp.value.data
    const pl = d?.data?.list || d?.list || d?.data || d || []
    phoneModels.value = Array.isArray(pl) ? pl : []
  }
  // 虚拟网卡
  if (bridgeResp.status === 'fulfilled') {
    const d = bridgeResp.value.data
    const bl = d?.data?.list || d?.list || d?.data || d
    bridgeOptions.value = Array.isArray(bl) ? bl : []
  }
  loadingBridges.value = false
}

async function doCreate() {
  createTip.value = ''
  if (selectedSlots.value.size === 0 && !form.imageUrl) { createTip.value = '请选择坑位和镜像'; return }
  if (selectedSlots.value.size === 0) { createTip.value = '请选择坑位'; return }
  if (!form.imageUrl) { createTip.value = '请选择镜像'; return }
  creating.value = true
  try {
    // 先拉取镜像（只需一次）
    elapsedSeconds.value = 0
    elapsedTimer = setInterval(() => elapsedSeconds.value++, 1000)
    taskPhase.value = 'pulling'
    pullCurrent.value = 0; pullTotal.value = 0
    pullStatusText.value = '正在连接镜像仓库...'
    const pullOk = await doPullImage(form.imageUrl)
    if (!pullOk) return

    // 逐坑位创建容器
    taskPhase.value = 'creating'
    const slots = [...selectedSlots.value].sort((a, b) => a - b)
    const alias = form.alias?.trim()
    let success = 0, fail = 0
    const failedSlots = []
    let lastFailMsg = ''
    createCurrent.value = 0
    createTotal.value = slots.length
    for (let i = 0; i < slots.length; i++) {
      const slotNum = slots[i]
      createCurrent.value = i + 1
      createSlotNum.value = slotNum
      try {
        const body = buildBody()
        body.name = generateName()
        body.indexNum = slotNum
        const hasRunning = device.containers.some(c => c.indexNum === slotNum && c.status === 'running')
        if (hasRunning) body.start = false
        const res = await device.request('sdk:createContainer', body, 120000)
        const resData = res.data || {}
        if (resData.code && resData.code !== 0) {
          fail++; failedSlots.push(slotNum)
          lastFailMsg = resData.message || resData.msg || resData.error || `错误码 ${resData.code}`
          continue
        }
        if (alias) {
          const displayAlias = slots.length > 1 ? `${alias}-${slotNum}` : alias
          try { await device.setAlias(body.name, displayAlias) } catch {}
        }
        success++
      } catch (e) {
        fail++; failedSlots.push(slotNum)
        lastFailMsg = e?.message || e?.toString() || '未知错误'
      }
      // 非最后一个坑位，等待设备就绪
      if (i < slots.length - 1) {
        createSlotNum.value = -1
        await new Promise(r => setTimeout(r, 2000))
      }
    }
    if (fail === 0) {
      setDone()
      if (slots.length > 1) ElMessage.success(`${success} 个容器全部创建完成`)
    } else {
      const failInfo = failedSlots.length > 0 ? ` (坑位 ${failedSlots.join(', ')})` : ''
      setFailed(`完成: ${success} 成功, ${fail} 失败${failInfo}`, lastFailMsg)
    }
  } catch (e) {
    setFailed('创建失败', e?.message || '未知错误')
  } finally {
    creating.value = false
    device.refreshContainers()
  }
}

function buildBody() {
  const body = { ...form }
  delete body.alias // 别名由前端单独处理，不发给 SDK
  body.mgenable = mgEnabled.value ? '1' : '0'
  body.gmsenable = gmsEnabled.value ? '1' : '0'
  body.enforce = enforceEnabled.value
  if (body.s5Type === '0') {
    delete body.s5Type; delete body.s5IP; delete body.s5Port
    delete body.s5User; delete body.s5Password
  }
  if (!body.PINCode) delete body.PINCode
  // 未选机型时随机
  if (!body.modelId) {
    const candidates = filteredPhoneModels.value.length ? filteredPhoneModels.value : phoneModels.value
    if (candidates.length) {
      const rand = candidates[Math.floor(Math.random() * candidates.length)]
      body.modelId = rand.id || rand.modelId || ''
      body.modelName = rand.name || rand.modelName || ''
    }
  }
  if (!body.modelId) { delete body.modelId; delete body.modelName }
  if (!body.sandboxSize) delete body.sandboxSize
  if (!body.doboxWidth) delete body.doboxWidth
  if (!body.doboxHeight) delete body.doboxHeight
  if (!body.doboxFps) delete body.doboxFps
  if (!body.doboxDpi) delete body.doboxDpi
  if (!body.countryCode) delete body.countryCode
  if (!body.longitude) delete body.longitude
  if (!body.latitude) delete body.latitude
  // 网卡参数名映射：前端 form.bridge → SDK mytBridgeName
  if (body.bridge) {
    body.mytBridgeName = body.bridge
  }
  delete body.bridge
  return body
}

async function doPullImage(imageUrl) {
  abortController = new AbortController()
  return await pullImage(imageUrl, {
    onProgress({ current, total, text }) {
      taskPhase.value = 'pulling'
      pullCurrent.value = current; pullTotal.value = total; pullStatusText.value = text
    },
    onExtracting(text) { taskPhase.value = 'extracting'; pullStatusText.value = text },
    onComplete(text) { pullStatusText.value = text },
    onError(msg) { setFailed(msg, '') },
  }, abortController.signal)
}

function setDone() {
  taskPhase.value = 'done'; taskDone.value = true; stopTimer()
  abortController = null
  ElMessage.success('容器创建完成'); emit('created')
  // 自动关闭对话框
  setTimeout(() => emit('update:modelValue', false), 800)
}
function setFailed(msg, detail) {
  taskPhase.value = 'failed'; taskDone.value = true; taskError.value = msg; taskErrorDetail.value = detail || ''; stopTimer()
  ElMessage.error(msg)
  emit('created')
}
function stopTimer() { if (elapsedTimer) { clearInterval(elapsedTimer); elapsedTimer = null } }
function stopTask() {
  if (abortController) { abortController.abort(); abortController = null }
  stopTimer()
}
function onClose() {
  stopTask(); emit('update:modelValue', false)
  if (taskDone.value) emit('created')
}

onBeforeUnmount(() => stopTask())
</script>

<style scoped>
.slot-item {
  width: 40px; height: 32px; line-height: 32px; text-align: center;
  border-radius: 4px; cursor: pointer; font-size: 13px;
  background: #2a2a2a; color: #ccc; border: 1px solid #444;
  transition: all 0.15s; user-select: none;
}
.slot-item:hover { border-color: #409eff; color: #fff; }
.slot-item.active { background: #409eff; color: #fff; border-color: #409eff; }
.slot-item.slot-blue { border-color: #409EFF; color: #409EFF; }
.slot-item.slot-yellow { border-color: #E6A23C; color: #E6A23C; }
.slot-item.slot-red { border-color: #F56C6C; color: #F56C6C; }
.slot-item.slot-gray { border-color: #909399; color: #909399; }
.slot-item.active.slot-blue { background: #409EFF; color: #fff; }
.slot-item.active.slot-yellow { background: #E6A23C; color: #fff; border-color: #E6A23C; }
.slot-item.active.slot-red { background: #F56C6C; color: #fff; border-color: #F56C6C; }
.slot-item.active.slot-gray { background: #909399; color: #fff; border-color: #909399; }
.feature-desc {
  color: #b0b0b0;
  font-size: 11px;
  margin-left: 8px;
  line-height: 1.4;
}
</style>
