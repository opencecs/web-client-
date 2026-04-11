<template>
  <el-dialog :modelValue="modelValue" @update:modelValue="$emit('update:modelValue', $event)"
    title="备份切换" width="600px" destroy-on-close>
    <p style="color: #999; font-size: 13px; margin-bottom: 12px">
      坑位 {{ slotNum }} 的所有容器（同一坑位只能同时运行一个，点击选择要切换的容器）
    </p>
    <div class="backup-list">
      <div v-for="c in slotContainers" :key="c.name"
        :class="['backup-item', { selected: selectedRow?.name === c.name, running: c.status === 'running' }]"
        @click="onSelect(c)">
        <div class="backup-main">
          <div class="backup-name">{{ device.displayName(c.name) }}</div>
          <el-tag :type="c.status === 'running' ? 'success' : 'info'" size="small" style="margin-left: 8px">
            {{ c.status === 'running' ? '运行中' : '已停止' }}
          </el-tag>
          <el-button size="small" text type="primary" style="margin-left: auto" @click.stop="openAliasEdit(c)">
            {{ device.containerAliases[c.name] ? '修改别名' : '设置别名' }}
          </el-button>
        </div>
        <div class="backup-meta">
          <span>镜像: {{ matchMirrorName(c.image) }}</span>
          <span>创建: {{ formatTime(c.created) }}</span>
        </div>
      </div>
      <el-empty v-if="!slotContainers.length" description="该坑位暂无容器" :image-size="60" />
    </div>
    <template #footer>
      <el-button @click="$emit('update:modelValue', false)">取消</el-button>
      <el-button type="primary" :loading="switching" :disabled="!selectedRow" @click="doSwitch">
        切换到此容器
      </el-button>
    </template>

    <!-- 别名编辑弹窗 -->
    <el-dialog v-model="aliasVisible" title="设置别名" width="360px" append-to-body>
      <el-form label-width="70px">
        <el-form-item label="容器">
          <span style="color: #999; font-size: 12px">{{ aliasTargetName }}</span>
        </el-form-item>
        <el-form-item label="别名">
          <el-input v-model="aliasInput" placeholder="输入别名" clearable />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="aliasVisible = false">取消</el-button>
        <el-button v-if="device.containerAliases[aliasTargetName]" type="danger" :loading="aliasSaving" @click="doRemoveAlias">清除</el-button>
        <el-button type="primary" :loading="aliasSaving" @click="doSaveAlias">保存</el-button>
      </template>
    </el-dialog>
  </el-dialog>
</template>

<script setup>
import { ref, computed, watch } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../../stores/device.js'

const props = defineProps({
  modelValue: Boolean,
  slotNum: { type: Number, default: 0 }
})
const emit = defineEmits(['update:modelValue'])

const device = useDeviceStore()
const selectedRow = ref(null)
const switching = ref(false)
const mirrorCache = ref([])

// 别名编辑
const aliasVisible = ref(false)
const aliasInput = ref('')
const aliasTargetName = ref('')
const aliasSaving = ref(false)

function openAliasEdit(container) {
  aliasTargetName.value = container.name
  aliasInput.value = device.containerAliases[container.name] || ''
  aliasVisible.value = true
}

async function doSaveAlias() {
  const alias = aliasInput.value.trim()
  if (!alias) { ElMessage.warning('请输入别名'); return }
  aliasSaving.value = true
  try {
    await device.setAlias(aliasTargetName.value, alias)
    ElMessage.success('别名已保存')
    aliasVisible.value = false
  } catch { ElMessage.error('保存失败') }
  finally { aliasSaving.value = false }
}

async function doRemoveAlias() {
  aliasSaving.value = true
  try {
    await device.removeAlias(aliasTargetName.value)
    ElMessage.success('别名已清除')
    aliasVisible.value = false
  } catch { ElMessage.error('清除失败') }
  finally { aliasSaving.value = false }
}
// 打开时加载镜像缓存
watch(() => props.modelValue, (val) => {
  if (val) {
    selectedRow.value = null
    loadMirrors()
  }
})

async function loadMirrors() {
  try {
    const resp = await device.request('device:mirrors')
    mirrorCache.value = resp.data || []
  } catch {}
}

// 镜像URL匹配简称
function matchMirrorName(url) {
  if (!url) return '-'
  const match = mirrorCache.value.find(m => m.url === url)
  if (match) return match.name
  // 降级：取URL最后一段
  const parts = url.split('/')
  return parts[parts.length - 1] || url
}

// 筛选同一坑位的所有容器
const slotContainers = computed(() => {
  return device.containers.filter(c => c.indexNum === props.slotNum)
})

function onSelect(row) {
  selectedRow.value = row
}

function formatTime(t) {
  if (!t) return '-'
  return t.replace('T', ' ').replace(/\.\d+.*/, '')
}

async function doSwitch() {
  if (!selectedRow.value) return
  const target = selectedRow.value

  if (target.status === 'running') {
    ElMessage.info('该容器已在运行中')
    return
  }

  switching.value = true
  try {
    // 先停止当前坑位中正在运行的容器
    const runningOne = slotContainers.value.find(c => c.status === 'running')
    if (runningOne) {
      await device.request('container:stop', { name: runningOne.name })
    }
    // 启动选中的容器
    await device.request('container:start', { name: target.name })
    ElMessage.success(`已切换到 ${target.name}`)
    emit('update:modelValue', false)
  } catch (e) {
    ElMessage.error(e.message || '切换失败')
  } finally {
    switching.value = false
  }
}
</script>

<style scoped>
.backup-list {
  display: flex;
  flex-direction: column;
  gap: 8px;
  max-height: 400px;
  overflow-y: auto;
}
.backup-item {
  padding: 12px 16px;
  border-radius: 8px;
  background: #1e1e1e;
  border: 2px solid #333;
  cursor: pointer;
  transition: all 0.15s;
}
.backup-item:hover {
  border-color: #555;
}
.backup-item.selected {
  border-color: #409eff;
  background: #1a2a3a;
  box-shadow: 0 0 8px rgba(64, 158, 255, 0.3);
}
.backup-item.running {
  border-left: 3px solid #67c23a;
}
.backup-main {
  display: flex;
  align-items: center;
  margin-bottom: 6px;
}
.backup-name {
  font-size: 14px;
  font-weight: bold;
  color: #e0e0e0;
}
.backup-meta {
  display: flex;
  gap: 16px;
  font-size: 12px;
  color: #999;
}
</style>
