<template>
  <div>
    <!-- 全局域名过滤 -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">全局域名过滤</span>
          <el-space>
            <el-button size="small" type="primary" @click="openAddGlobal">添加规则</el-button>
            <el-button size="small" :icon="Refresh" @click="fetchGlobalFilter" :loading="loadingGlobal" circle />
          </el-space>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        <template #title><span style="font-weight: bold">域名过滤说明</span></template>
        <div style="line-height: 1.8; color: #b0b0b0">
          全局域名过滤对<b>所有使用 VPC 的容器</b>生效。匹配的域名流量将<b>不经过 VPC</b>，直接走本地网络（常用于国内网站直连）。<br/>
          支持四种匹配模式：<br/>
          &nbsp;&nbsp;• <b>domain:</b> 域名后缀匹配 — 如 <code>baidu.com</code> 匹配 <code>*.baidu.com</code><br/>
          &nbsp;&nbsp;• <b>full:</b> 精确匹配 — 如 <code>full:www.baidu.com</code> 仅匹配该地址<br/>
          &nbsp;&nbsp;• <b>keyword:</b> 关键字匹配 — 如 <code>keyword:baidu</code> 匹配含 baidu 的所有域名<br/>
          &nbsp;&nbsp;• <b>regexp:</b> 正则匹配 — 如 <code>regexp:^.*\.cn$</code> 匹配所有 .cn 域名<br/>
          不加前缀默认为 domain 模式。
        </div>
      </el-alert>

      <div v-if="globalDomains.length" style="margin-bottom: 8px">
        <el-tag v-for="(d, i) in globalDomains" :key="i" closable :type="domainTagType(d)"
          @close="removeGlobalDomain(i)" style="margin: 2px 4px">{{ d }}</el-tag>
      </div>
      <el-empty v-else-if="!loadingGlobal" description="暂无全局过滤规则" :image-size="40" />

      <div v-if="globalDomains.length" style="margin-top: 8px">
        <el-popconfirm title="确认清空所有全局域名过滤规则？" @confirm="clearGlobalFilter">
          <template #reference>
            <el-button type="danger" size="small" text>清空全部</el-button>
          </template>
        </el-popconfirm>
      </div>
    </el-card>

    <!-- 容器域名管理 -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">容器域名规则</span>
          <el-select v-if="selectedContainer" v-model="selectedContainer" filterable clearable placeholder="切换容器" size="small"
            style="width: 250px" @change="onContainerChange">
            <el-option v-for="r in rules" :key="r.containerName"
              :label="device.displayName(r.containerName)" :value="r.containerName" />
          </el-select>
        </div>
      </template>

      <!-- 未选择容器：醒目引导 -->
      <div v-if="!selectedContainer">
        <div v-if="rules.length" style="text-align: center; padding: 30px 0">
          <div style="color: #e0e0e0; font-size: 15px; margin-bottom: 16px">选择一个已绑定 VPC 的容器，管理其域名规则</div>
          <el-select v-model="selectedContainer" filterable placeholder="点击选择容器" size="large"
            style="width: 350px" @change="onContainerChange">
            <el-option v-for="r in rules" :key="r.containerName" :value="r.containerName">
              <div style="display: flex; justify-content: space-between; align-items: center; width: 100%">
                <span>{{ device.displayName(r.containerName) }}</span>
                <span style="color: #999; font-size: 11px">{{ r.groupName }} / {{ r.vpcRemarks || '-' }}</span>
              </div>
            </el-option>
          </el-select>
        </div>
        <el-empty v-else description="暂无容器绑定 VPC，请先在「容器 VPC 规则」中分配" :image-size="60" />
      </div>

      <!-- 已选择容器 -->
      <div v-else>
        <!-- 域名直连 -->
        <div style="margin-bottom: 20px">
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
            <span style="color: #e0e0e0; font-weight: bold; font-size: 13px">域名直连</span>
            <el-space>
              <el-button size="small" type="primary" text @click="openAddDirect">添加</el-button>
              <el-popconfirm v-if="directDomains.length" title="清空域名直连列表？" @confirm="clearDirect">
                <template #reference>
                  <el-button size="small" type="danger" text>清空</el-button>
                </template>
              </el-popconfirm>
            </el-space>
          </div>
          <div style="color: #999; font-size: 11px; margin-bottom: 6px">
            直连域名的流量<b>不走 VPC</b>，直接使用本地网络
          </div>
          <div v-if="directDomains.length" v-loading="loadingDirect">
            <el-tag v-for="(d, i) in directDomains" :key="i" closable type="success"
              @close="removeDirectDomain(i)" style="margin: 2px 4px">{{ d }}</el-tag>
          </div>
          <el-empty v-else-if="!loadingDirect" description="无域名直连规则" :image-size="30" />
        </div>

        <el-divider style="border-color: #333; margin: 12px 0" />

        <!-- 域名过滤 -->
        <div>
          <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
            <span style="color: #e0e0e0; font-weight: bold; font-size: 13px">域名过滤</span>
            <el-space>
              <el-button size="small" type="primary" text @click="openAddFilter">添加</el-button>
              <el-popconfirm v-if="filterDomains.length" title="清空域名过滤列表？" @confirm="clearFilter">
                <template #reference>
                  <el-button size="small" type="danger" text>清空</el-button>
                </template>
              </el-popconfirm>
            </el-space>
          </div>
          <div style="color: #999; font-size: 11px; margin-bottom: 6px">
            过滤域名的流量将被<b>拦截丢弃</b>，容器无法访问这些域名
          </div>
          <div v-if="filterDomains.length" v-loading="loadingFilter">
            <el-tag v-for="(d, i) in filterDomains" :key="i" closable type="danger"
              @close="removeFilterDomain(i)" style="margin: 2px 4px">{{ d }}</el-tag>
          </div>
          <el-empty v-else-if="!loadingFilter" description="无域名过滤规则" :image-size="30" />
        </div>
      </div>
    </el-card>

    <!-- 添加域名弹窗（通用） -->
    <el-dialog v-model="showAddDomain" :title="addDomainTitle" width="520px">
      <template v-if="addDomainType === 'global'">
        <div style="color: #999; margin-bottom: 8px">匹配模式</div>
        <div style="margin-bottom: 16px">
          <div v-for="opt in matchModes" :key="opt.value"
            :style="{
              display: 'flex', alignItems: 'flex-start', padding: '10px 12px', marginBottom: '6px',
              borderRadius: '6px', cursor: 'pointer',
              border: addDomainPrefix === opt.value ? '1px solid #409eff' : '1px solid #333',
              background: addDomainPrefix === opt.value ? '#1a2b3d' : '#252525'
            }"
            @click="addDomainPrefix = opt.value">
            <el-radio :model-value="addDomainPrefix" :value="opt.value" style="margin-right: 10px; margin-top: 1px" />
            <div>
              <div style="color: #e0e0e0; font-size: 13px">{{ opt.label }}</div>
              <div style="color: #666; font-size: 11px; margin-top: 2px">{{ opt.desc }}</div>
            </div>
          </div>
        </div>
      </template>

      <div style="color: #999; margin-bottom: 6px">域名列表</div>
      <el-input v-model="addDomainText" type="textarea" :rows="5"
        :placeholder="addDomainType === 'global' ? matchModePlaceholder : '每行一个域名，如：\ngoogle.com\nfacebook.com'" />
      <div style="color: #999; font-size: 11px; margin-top: 4px">每行一个，添加后立即生效</div>

      <template #footer>
        <el-button @click="showAddDomain = false">取消</el-button>
        <el-button type="primary" :loading="addingDomain" @click="doAddDomain">添加</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../../stores/device.js'

const props = defineProps({
  rules: { type: Array, default: () => [] }
})
const device = useDeviceStore()

// ===== 全局域名过滤 =====
const globalDomains = ref([])
const loadingGlobal = ref(false)

async function fetchGlobalFilter() {
  loadingGlobal.value = true
  try {
    const resp = await device.request('sdk:getGlobalDomainFilter')
    const d = resp.data?.data || resp.data || {}
    globalDomains.value = Array.isArray(d.domains) ? d.domains : []
  } catch { globalDomains.value = [] }
  finally { loadingGlobal.value = false }
}

function removeGlobalDomain(idx) {
  const updated = [...globalDomains.value]
  updated.splice(idx, 1)
  saveGlobalFilter(updated)
}

async function clearGlobalFilter() {
  try {
    await device.request('sdk:deleteGlobalDomainFilter')
    ElMessage.success('全局过滤已清空')
    globalDomains.value = []
  } catch (e) { ElMessage.error(e.message || '清空失败') }
}

async function saveGlobalFilter(domains) {
  try {
    await device.request('sdk:setGlobalDomainFilter', { domains })
    ElMessage.success('规则已更新')
    globalDomains.value = domains
  } catch (e) { ElMessage.error(e.message || '更新失败') }
}

// 域名标签颜色
function domainTagType(domain) {
  if (domain.startsWith('full:')) return 'success'
  if (domain.startsWith('keyword:')) return 'warning'
  if (domain.startsWith('regexp:')) return 'danger'
  return '' // domain 模式用默认蓝色
}

// ===== 容器域名管理 =====
const selectedContainer = ref('')
const directDomains = ref([])
const filterDomains = ref([])
const loadingDirect = ref(false)
const loadingFilter = ref(false)

async function onContainerChange(name) {
  if (!name) { directDomains.value = []; filterDomains.value = []; return }
  // 并行加载直连和过滤
  loadingDirect.value = true; loadingFilter.value = true
  const [directRes, filterRes] = await Promise.allSettled([
    device.request('sdk:getDomainDirect', { containerID: name }),
    device.request('sdk:getDomainFilter', { containerID: name })
  ])
  if (directRes.status === 'fulfilled') {
    const d = directRes.value.data?.data || directRes.value.data || {}
    directDomains.value = Array.isArray(d.domains) ? d.domains : []
  } else { directDomains.value = [] }
  if (filterRes.status === 'fulfilled') {
    const d = filterRes.value.data?.data || filterRes.value.data || {}
    filterDomains.value = Array.isArray(d.domains) ? d.domains : []
  } else { filterDomains.value = [] }
  loadingDirect.value = false; loadingFilter.value = false
}

function removeDirectDomain(idx) {
  const updated = [...directDomains.value]
  updated.splice(idx, 1)
  saveDirect(updated)
}

function removeFilterDomain(idx) {
  const updated = [...filterDomains.value]
  updated.splice(idx, 1)
  saveFilter(updated)
}

async function saveDirect(domains) {
  try {
    await device.request('sdk:setDomainDirect', { containerID: selectedContainer.value, domains })
    directDomains.value = domains
    ElMessage.success('域名直连已更新')
  } catch (e) { ElMessage.error(e.message || '更新失败') }
}

async function saveFilter(domains) {
  try {
    await device.request('sdk:setDomainFilter', { containerID: selectedContainer.value, domains })
    filterDomains.value = domains
    ElMessage.success('域名过滤已更新')
  } catch (e) { ElMessage.error(e.message || '更新失败') }
}

async function clearDirect() {
  try {
    await device.request('sdk:deleteDomainDirect', { containerID: selectedContainer.value })
    directDomains.value = []
    ElMessage.success('域名直连已清空')
  } catch (e) { ElMessage.error(e.message || '清空失败') }
}

async function clearFilter() {
  try {
    await device.request('sdk:deleteDomainFilter', { containerID: selectedContainer.value })
    filterDomains.value = []
    ElMessage.success('域名过滤已清空')
  } catch (e) { ElMessage.error(e.message || '清空失败') }
}

// ===== 通用添加域名弹窗 =====
const showAddDomain = ref(false)
const addDomainText = ref('')
const addDomainPrefix = ref('')
const addDomainType = ref('')
const addingDomain = ref(false)

const matchModes = [
  { value: '',        label: '域名匹配（推荐）', desc: '匹配域名及其所有子域名。填 baidu.com 会同时匹配 www.baidu.com、tieba.baidu.com 等' },
  { value: 'full:',   label: '精确匹配',         desc: '只匹配完全一致的域名。填 www.baidu.com 就只匹配这一个地址' },
  { value: 'keyword:', label: '关键字匹配',       desc: '域名中包含该关键字即匹配。填 google 会匹配所有含 google 的域名' },
  { value: 'regexp:', label: '正则匹配（高级）',   desc: '使用正则表达式匹配，适合复杂规则。如 ^.*\\.cn$ 匹配所有 .cn 域名' }
]

const matchModePlaceholder = computed(() => {
  if (addDomainPrefix.value === '') return '每行一个域名，如：\nbaidu.com\ntaobao.com\nqq.com'
  if (addDomainPrefix.value === 'full:') return '每行一个完整域名，如：\nwww.baidu.com\nwww.google.com'
  if (addDomainPrefix.value === 'keyword:') return '每行一个关键字，如：\ngoogle\nfacebook\ntiktok'
  return '每行一个正则表达式，如：\n^.*\\.cn$\n^.*\\.com\\.cn$'
})

const addDomainTitle = computed(() => {
  if (addDomainType.value === 'global') return '添加全局过滤规则'
  if (addDomainType.value === 'direct') return '添加域名直连'
  return '添加域名过滤'
})

function openAddGlobal() {
  addDomainType.value = 'global'; addDomainText.value = ''; addDomainPrefix.value = ''
  showAddDomain.value = true
}
function openAddDirect() {
  addDomainType.value = 'direct'; addDomainText.value = ''
  showAddDomain.value = true
}
function openAddFilter() {
  addDomainType.value = 'filter'; addDomainText.value = ''
  showAddDomain.value = true
}

async function doAddDomain() {
  const lines = addDomainText.value.split('\n').map(s => s.trim()).filter(Boolean)
  if (!lines.length) { ElMessage.warning('请输入至少一个域名'); return }
  addingDomain.value = true
  try {
    if (addDomainType.value === 'global') {
      const prefixed = lines.map(l => addDomainPrefix.value + l)
      const merged = [...globalDomains.value, ...prefixed]
      await saveGlobalFilter(merged)
    } else if (addDomainType.value === 'direct') {
      const merged = [...directDomains.value, ...lines]
      await saveDirect(merged)
    } else {
      const merged = [...filterDomains.value, ...lines]
      await saveFilter(merged)
    }
    showAddDomain.value = false
  } catch {} // saveXxx 内部已处理错误
  finally { addingDomain.value = false }
}

// 初始加载
import { onMounted } from 'vue'
onMounted(() => { fetchGlobalFilter() })
</script>
