<template>
  <div>
    <!-- 云机备份 -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">云机备份</span>
          <el-button size="small" :icon="Refresh" @click="fetchBackups" :loading="loadingBackups" circle />
        </div>
      </template>
      <el-table :data="backups" v-loading="loadingBackups" size="small" stripe>
        <el-table-column label="名称" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <div>{{ row.name }}</div>
            <div style="display: flex; align-items: center; gap: 4px; margin-top: 2px">
              <el-tag v-if="device.containerAliases[row.name]" size="small" type="info" closable
                @close="removeBackupAlias(row.name)">
                {{ device.containerAliases[row.name] }}
              </el-tag>
              <el-button v-else size="small" text type="primary" @click="openAliasEdit(row.name)">
                设置别名
              </el-button>
              <el-button v-if="device.containerAliases[row.name]" size="small" text type="primary"
                @click="openAliasEdit(row.name)">修改</el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="大小" width="120">
          <template #default="{ row }">{{ formatSize(row.size) }}</template>
        </el-table-column>
        <el-table-column label="时间" width="180">
          <template #default="{ row }">{{ formatTs(row.mtimestamp) }}</template>
        </el-table-column>
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button type="primary" size="small" text @click="downloadBackup(row)">下载</el-button>
            <el-popconfirm title="确认删除？" @confirm="deleteBackup(row)">
              <template #reference><el-button type="danger" size="small" text>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!backups.length && !loadingBackups" description="暂无备份" :image-size="60" />
    </el-card>

    <!-- 机型备份 -->
    <el-card style="background: #1e1e1e; border-color: #333">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">机型备份</span>
          <el-button size="small" :icon="Refresh" @click="fetchModelBackups" :loading="loadingModels" circle />
        </div>
      </template>
      <el-table :data="modelBackups" v-loading="loadingModels" size="small" stripe>
        <el-table-column label="名称" min-width="250" show-overflow-tooltip>
          <template #default="{ row }">
            <div>{{ row.name }}</div>
            <div style="display: flex; align-items: center; gap: 4px; margin-top: 2px">
              <el-tag v-if="device.containerAliases[row.name]" size="small" type="info" closable
                @close="removeBackupAlias(row.name)">
                {{ device.containerAliases[row.name] }}
              </el-tag>
              <el-button v-else size="small" text type="primary" @click="openAliasEdit(row.name)">
                设置别名
              </el-button>
              <el-button v-if="device.containerAliases[row.name]" size="small" text type="primary"
                @click="openAliasEdit(row.name)">修改</el-button>
            </div>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="100">
          <template #default="{ row }">
            <el-popconfirm title="确认删除？" @confirm="deleteModelBackup(row)">
              <template #reference><el-button type="danger" size="small" text>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!modelBackups.length && !loadingModels" description="暂无机型备份" :image-size="60" />
    </el-card>

    <!-- 别名编辑弹窗 -->
    <el-dialog v-model="aliasVisible" title="设置备份别名" width="380px">
      <el-form label-width="70px">
        <el-form-item label="别名">
          <el-input v-model="aliasInput" placeholder="输入备份别名" clearable />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="aliasVisible = false">取消</el-button>
        <el-button type="primary" :loading="aliasSaving" @click="saveAlias">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../stores/device.js'

const device = useDeviceStore()

const backups = ref([])
const modelBackups = ref([])
const loadingBackups = ref(false)
const loadingModels = ref(false)

// 别名编辑
const aliasVisible = ref(false)
const aliasInput = ref('')
const aliasTarget = ref('')
const aliasSaving = ref(false)

function openAliasEdit(name) {
  aliasTarget.value = name
  aliasInput.value = device.containerAliases[name] || ''
  aliasVisible.value = true
}

async function saveAlias() {
  const alias = aliasInput.value.trim()
  if (!alias) { ElMessage.warning('请输入别名'); return }
  aliasSaving.value = true
  try {
    await device.setAlias(aliasTarget.value, alias)
    ElMessage.success('别名已保存')
    aliasVisible.value = false
  } catch { ElMessage.error('保存失败') }
  finally { aliasSaving.value = false }
}

async function removeBackupAlias(name) {
  try {
    await device.removeAlias(name)
    ElMessage.success('别名已清除')
  } catch { ElMessage.error('清除失败') }
}

async function fetchBackups() {
  loadingBackups.value = true
  try {
    const resp = await device.request('sdk:listBackups')
    const d = resp.data
    backups.value = Array.isArray(d?.data?.list) ? d.data.list : Array.isArray(d?.data) ? d.data : Array.isArray(d) ? d : []
  } catch {} finally { loadingBackups.value = false }
}

async function fetchModelBackups() {
  loadingModels.value = true
  try {
    const resp = await device.request('sdk:listModelBackups')
    const d = resp.data
    modelBackups.value = Array.isArray(d?.data) ? d.data : Array.isArray(d) ? d : []
  } catch {} finally { loadingModels.value = false }
}

function downloadBackup(row) {
  // 文件下载保留 HTTP
  const token = localStorage.getItem('token')
  window.open(`/api/sdk/backup/download?name=${encodeURIComponent(row.name)}&token=${token}`, '_blank')
}

async function deleteBackup(row) {
  try {
    await device.request('sdk:deleteBackup', { name: row.name })
    ElMessage.success('删除成功')
    fetchBackups()
  } catch (e) { ElMessage.error('删除失败') }
}

async function deleteModelBackup(row) {
  try {
    await device.request('sdk:deleteModelBackup', { name: row.name })
    ElMessage.success('删除成功')
    fetchModelBackups()
  } catch (e) { ElMessage.error('删除失败') }
}

function formatSize(bytes) {
  if (!bytes) return '-'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / (1024 * 1024)).toFixed(1) + ' MB'
  return (bytes / (1024 * 1024 * 1024)).toFixed(2) + ' GB'
}

function formatTs(ts) {
  if (!ts) return '-'
  return new Date(ts * 1000).toLocaleString('zh-CN')
}

onMounted(() => { fetchBackups(); fetchModelBackups() })
</script>
