<template>
  <div class="mobile-create">
    <van-nav-bar title="创建容器" left-arrow @click-left="onBack" :border="false" />

    <!-- 分步向导 -->
    <van-steps :active="step" active-color="#409eff" class="create-steps">
      <van-step>基础</van-step>
      <van-step>系统</van-step>
      <van-step>高级</van-step>
      <van-step>确认</van-step>
    </van-steps>

    <div class="step-content">
      <!-- Step 0: 基础配置 -->
      <div v-show="step === 0">
        <van-cell-group inset>
          <van-field v-model="form.alias" label="容器别名" placeholder="可选，用于显示" clearable />
          <van-cell title="安卓版本">
            <template #value>
              <van-radio-group v-model="androidVersion" direction="horizontal">
                <van-radio name="and16">Android 16</van-radio>
                <van-radio name="and14">Android 14</van-radio>
              </van-radio-group>
            </template>
          </van-cell>
          <van-field :model-value="selectedImageName" is-link readonly label="镜像" placeholder="选择镜像"
            @click="showImagePicker = true" />
        </van-cell-group>

        <!-- 坑位选择 -->
        <div class="section-title">选择坑位</div>
        <div class="slot-grid">
          <div v-for="i in maxSlots" :key="i"
            :class="['slot-item', { active: selectedSlots.has(i) }]"
            @click="toggleSlot(i)">{{ i }}</div>
        </div>
        <div class="slot-hint">已选 {{ selectedSlots.size }} 个坑位，每个坑位创建一个容器</div>
      </div>

      <!-- Step 1: 系统配置 -->
      <div v-show="step === 1">
        <van-cell-group inset>
          <van-cell title="DNS">
            <template #value>
              <van-radio-group v-model="form.dns" direction="horizontal">
                <van-radio name="223.5.5.5">阿里</van-radio>
                <van-radio name="8.8.8.8">Google</van-radio>
              </van-radio-group>
            </template>
          </van-cell>
          <van-cell title="沙盒大小">
            <template #value>
              <van-radio-group v-model="form.sandboxSize" direction="horizontal">
                <van-radio name="16GB">16G</van-radio>
                <van-radio name="32GB">32G</van-radio>
                <van-radio name="64GB">64G</van-radio>
              </van-radio-group>
            </template>
          </van-cell>
          <van-field v-model="form.PINCode" label="锁屏密码" placeholder="4-8位数字（可选）" type="digit" maxlength="8" />
          <van-cell title="自动启动">
            <template #right-icon><van-switch v-model="form.start" size="20px" /></template>
          </van-cell>
        </van-cell-group>
      </div>

      <!-- Step 2: 高级配置 -->
      <div v-show="step === 2">
        <van-cell-group inset>
          <van-cell title="Magisk">
            <template #right-icon><van-switch v-model="mgEnabled" size="20px" /></template>
          </van-cell>
          <van-cell title="GMS (谷歌服务)">
            <template #right-icon><van-switch v-model="gmsEnabled" size="20px" /></template>
          </van-cell>
          <van-cell title="安全模式 (SELinux)">
            <template #right-icon><van-switch v-model="enforceEnabled" size="20px" /></template>
          </van-cell>
          <van-cell title="随机文件">
            <template #right-icon><van-switch v-model="form.randomFile" size="20px" /></template>
          </van-cell>
        </van-cell-group>

        <div class="section-title">SOCKS5 代理</div>
        <van-cell-group inset>
          <van-cell title="代理类型">
            <template #value>
              <van-radio-group v-model="form.s5Type" direction="horizontal">
                <van-radio name="0">关闭</van-radio>
                <van-radio name="1">本地</van-radio>
                <van-radio name="2">远程</van-radio>
              </van-radio-group>
            </template>
          </van-cell>
          <template v-if="form.s5Type !== '0'">
            <van-field v-model="form.s5IP" label="代理 IP" placeholder="服务器 IP" />
            <van-field v-model="form.s5Port" label="端口" placeholder="端口" type="digit" />
            <van-field v-model="form.s5User" label="用户名" placeholder="可选" />
            <van-field v-model="form.s5Password" label="密码" placeholder="可选" type="password" />
          </template>
        </van-cell-group>
      </div>

      <!-- Step 3: 确认 -->
      <div v-show="step === 3">
        <van-cell-group inset>
          <van-cell title="别名" :value="form.alias || '(无)'" />
          <van-cell title="安卓版本" :value="androidVersion === 'and16' ? 'Android 16' : 'Android 14'" />
          <van-cell title="坑位" :value="[...selectedSlots].sort((a,b)=>a-b).join(', ')" />
          <van-cell title="DNS" :value="form.dns" />
          <van-cell title="沙盒" :value="form.sandboxSize" />
          <van-cell title="Magisk" :value="mgEnabled ? '开' : '关'" />
          <van-cell title="GMS" :value="gmsEnabled ? '开' : '关'" />
          <van-cell title="代理" :value="form.s5Type === '0' ? '不开启' : form.s5IP + ':' + form.s5Port" />
        </van-cell-group>

        <!-- 进度 -->
        <div v-if="taskPhase" class="progress-section">
          <div class="progress-label">{{ progressLabel }}</div>
          <van-progress :percentage="progressPercent" :color="progressColor" stroke-width="8"
            track-color="#2a2a2a" :show-pivot="false" />
          <div class="progress-text">{{ progressText }}</div>
        </div>
      </div>
    </div>

    <!-- 底部按钮 -->
    <div class="bottom-bar">
      <van-button v-if="step > 0 && !taskPhase" plain @click="step--">上一步</van-button>
      <van-button v-if="step < 3" type="primary" block @click="nextStep">下一步</van-button>
      <van-button v-if="step === 3 && !taskPhase" type="primary" block :loading="creating" @click="doCreate">
        开始创建
      </van-button>
      <van-button v-if="taskDone" type="primary" block @click="$router.back()">完成</van-button>
    </div>

    <!-- 镜像选择 -->
    <van-action-sheet v-model:show="showImagePicker" title="选择镜像">
      <div class="image-picker">
        <van-cell v-for="img in filteredMirrors" :key="img.url" :title="img.name"
          @click="selectImage(img)" clickable>
          <template #right-icon>
            <van-icon v-if="form.imageUrl === img.url" name="success" color="#409eff" />
          </template>
        </van-cell>
        <van-empty v-if="!filteredMirrors.length" description="无可用镜像" :image-size="40" />
      </div>
    </van-action-sheet>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useDeviceStore } from '../../stores/device.js'
import { pullImage } from '../../utils/pullImage.js'
import { showToast } from 'vant'

const router = useRouter()
const device = useDeviceStore()

const step = ref(0)
const maxSlots = 12
const selectedSlots = ref(new Set([1]))
const androidVersion = ref('and16')
const mgEnabled = ref(false)
const gmsEnabled = ref(false)
const enforceEnabled = ref(true)
const creating = ref(false)
const showImagePicker = ref(false)
const mirrors = ref([])
const phoneModels = ref([])

const form = reactive({
  alias: '', imageUrl: '', dns: '223.5.5.5', sandboxSize: '32GB',
  PINCode: '', start: true, s5Type: '0', s5IP: '', s5Port: '',
  s5User: '', s5Password: '', randomFile: true,
})

const taskPhase = ref(null)
const taskDone = ref(false)
const progressPercent = ref(0)
const progressText = ref('')
const progressLabel = computed(() => {
  const map = { pulling: '下载镜像中...', extracting: '解压中...', creating: '创建容器中...', done: '创建完成', failed: '创建失败' }
  return map[taskPhase.value] || ''
})
const progressColor = computed(() => {
  if (taskPhase.value === 'done') return '#67c23a'
  if (taskPhase.value === 'failed') return '#f56c6c'
  return '#409eff'
})

const filteredMirrors = computed(() =>
  mirrors.value.filter(m => m.os_ver === androidVersion.value)
    .sort((a, b) => (b.name || '').localeCompare(a.name || '', undefined, { numeric: true }))
)

const selectedImageName = computed(() => {
  if (!form.imageUrl) return ''
  const m = mirrors.value.find(m => m.url === form.imageUrl)
  return m?.name || form.imageUrl.split('/').pop() || form.imageUrl
})

function toggleSlot(n) {
  const s = new Set(selectedSlots.value)
  if (s.has(n)) s.delete(n); else s.add(n)
  selectedSlots.value = s
}

function selectImage(img) { form.imageUrl = img.url; showImagePicker.value = false }

function nextStep() {
  if (step.value === 0) {
    if (selectedSlots.value.size === 0 && !form.imageUrl) { showToast('请选择坑位和镜像'); return }
    if (selectedSlots.value.size === 0) { showToast('请选择坑位'); return }
    if (!form.imageUrl) { showToast('请选择镜像'); return }
  }
  step.value++
}

function onBack() {
  if (taskPhase.value) return
  router.back()
}

function generateName() {
  const chars = 'abcdefghijklmnopqrstuvwxyz0123456789'
  let name = ''; for (let i = 0; i < 8; i++) name += chars[Math.floor(Math.random() * chars.length)]
  return name
}

function buildBody(slotNum) {
  const body = { ...form }
  delete body.alias
  body.name = generateName()
  body.indexNum = slotNum
  body.mgenable = mgEnabled.value ? '1' : '0'
  body.gmsenable = gmsEnabled.value ? '1' : '0'
  body.enforce = enforceEnabled.value
  if (body.s5Type === '0') {
    delete body.s5Type; delete body.s5IP; delete body.s5Port
    delete body.s5User; delete body.s5Password
  }
  if (!body.PINCode) delete body.PINCode
  // 随机分配机型
  const androidVer = androidVersion.value === 'and14' ? '14' : '16'
  const candidates = phoneModels.value.filter(m => m.android_version === androidVer)
  const pool = candidates.length ? candidates : phoneModels.value
  if (pool.length) {
    const rand = pool[Math.floor(Math.random() * pool.length)]
    body.modelId = rand.id || rand.modelId || ''
    body.modelName = rand.name || rand.modelName || ''
  }
  if (!body.modelId) { delete body.modelId; delete body.modelName }
  if (!body.sandboxSize) delete body.sandboxSize
  return body
}

async function doCreate() {
  creating.value = true
  try {
    // 拉取镜像
    taskPhase.value = 'pulling'; progressPercent.value = 0; progressText.value = '连接镜像仓库...'
    const pullOk = await pullImage(form.imageUrl, {
      onProgress({ percent, text }) { progressPercent.value = percent; progressText.value = text },
      onExtracting(text) { taskPhase.value = 'extracting'; progressText.value = text },
      onComplete(text) { progressText.value = text },
      onError(msg) { taskPhase.value = 'failed'; progressText.value = msg; taskDone.value = true },
    })
    if (!pullOk) return

    // 创建容器
    taskPhase.value = 'creating'
    const slots = [...selectedSlots.value].sort((a, b) => a - b)
    const alias = form.alias?.trim()
    let success = 0, lastErr = '', failedSlots = []
    for (let i = 0; i < slots.length; i++) {
      const slotNum = slots[i]
      progressPercent.value = Math.round((i / slots.length) * 100)
      progressText.value = `创建 ${i + 1}/${slots.length} (坑位 ${slotNum})...`
      try {
        const body = buildBody(slotNum)
        // 坑位上有运行中的容器时不自动启动（和PC端一致）
        const hasRunning = device.containers.some(c => c.indexNum === slotNum && c.status === 'running')
        if (hasRunning) body.start = false
        const resp = await device.request('sdk:createContainer', body, 120000)
        const respData = resp?.data
        if (respData?.code !== undefined && respData.code !== 0) {
          lastErr = respData.message || respData.msg || '创建返回错误'
          failedSlots.push(slotNum)
          continue
        }
        if (alias) {
          const displayAlias = slots.length > 1 ? `${alias}-${slotNum}` : alias
          try { await device.setAlias(body.name, displayAlias) } catch {}
        }
        success++
      } catch (e) {
        lastErr = e.message || '请求失败'
        failedSlots.push(slotNum)
      }
      if (i < slots.length - 1) await new Promise(r => setTimeout(r, 2000))
    }
    if (success > 0) {
      taskPhase.value = 'done'; taskDone.value = true
      progressPercent.value = 100
      if (failedSlots.length) {
        progressText.value = `${success} 成功, ${failedSlots.length} 失败 (坑位 ${failedSlots.join(', ')})`
      } else {
        progressText.value = `${success} 个容器创建完成`
      }
      showToast('创建完成')
    } else {
      taskPhase.value = 'failed'; taskDone.value = true
      progressPercent.value = 100
      progressText.value = lastErr || '所有容器创建失败'
      showToast('创建失败: ' + lastErr)
    }
    device.refreshContainers()
  } catch (e) {
    taskPhase.value = 'failed'; taskDone.value = true; progressText.value = e.message || '创建失败'
  } finally { creating.value = false }
}

onMounted(async () => {
  try {
    const [mirrorResp, modelResp] = await Promise.allSettled([
      device.request('device:mirrors'),
      device.request('sdk:getPhoneModels'),
    ])
    if (mirrorResp.status === 'fulfilled') mirrors.value = mirrorResp.value.data || []
    if (modelResp.status === 'fulfilled') {
      const d = modelResp.value.data
      const pl = d?.data?.list || d?.list || d?.data || d || []
      phoneModels.value = Array.isArray(pl) ? pl : []
    }
  } catch {}
})
</script>

<style scoped>
.mobile-create { background: #0a0a0a; min-height: 100vh; padding-bottom: 90px; }

.create-steps { padding: 20px 16px 12px; }

.step-content { padding: 8px 0 20px; }

.section-title { font-size: 14px; font-weight: 600; color: #e0e0e0; padding: 16px 16px 10px; }

.slot-grid {
  display: flex; flex-wrap: wrap; gap: 10px; padding: 0 16px;
}
.slot-item {
  width: 48px; height: 40px; line-height: 40px; text-align: center;
  border-radius: 8px; font-size: 15px; font-weight: 500; background: #1a1a1a; color: #ccc;
  border: 1px solid #2a2a2a; user-select: none; transition: all 0.15s;
}
.slot-item.active { background: #409eff; color: #fff; border-color: #409eff; }
.slot-hint { font-size: 12px; color: #888; padding: 10px 16px 0; }

.bottom-bar {
  position: fixed; bottom: 0; left: 0; right: 0;
  padding: 12px 16px; background: #141414; border-top: 1px solid #2a2a2a;
  display: flex; gap: 10px;
  padding-bottom: calc(12px + constant(safe-area-inset-bottom));
  padding-bottom: calc(12px + env(safe-area-inset-bottom));
}

.progress-section { padding: 20px 16px; }
.progress-label { font-size: 14px; font-weight: 600; color: #e0e0e0; margin-bottom: 10px; }
.progress-text { font-size: 12px; color: #999; margin-top: 8px; }

.image-picker { max-height: 60vh; overflow-y: auto; }

/* 单选组横排时防止挤压 */
:deep(.van-radio-group--horizontal) { flex-wrap: wrap; gap: 8px 12px; }
:deep(.van-radio) { margin-right: 0; }
/* 表单行间距 */
:deep(.van-cell-group--inset) { margin: 0 12px 12px; border-radius: 12px; overflow: hidden; }
:deep(.van-cell) { padding: 12px 16px; }
:deep(.van-field) { padding: 12px 16px; }
</style>
