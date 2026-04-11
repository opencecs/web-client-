<template>
  <div>
    <!-- 容器 VPC 规则 -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">容器 VPC 规则</span>
          <el-space>
            <el-button size="small" type="primary" @click="openAssign()">分配 VPC</el-button>
            <el-button size="small" @click="openBatchAssign">批量分配</el-button>
            <el-button size="small" @click="openBatchRemove">批量移除</el-button>
            <el-button size="small" :icon="Refresh" @click="$emit('refresh')" :loading="loading" circle />
          </el-space>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        <template #title><span style="font-weight: bold">容器规则说明</span></template>
        <div style="line-height: 1.8; color: #b0b0b0">
          为容器分配 VPC 节点后，该容器的<b>所有网络流量</b>将通过指定节点转发，实现独立 IP 出口。<br/>
          • 每个容器只能绑定<b>一个</b>节点，重新分配会自动覆盖旧规则<br/>
          • <b>批量分配</b>可同时为多个容器指定同一节点；<b>批量移除</b>可一次清除多个容器的 VPC<br/>
          • <b>DNS 白名单</b>开启后，白名单内的 DNS 请求不经过 VPC，直连解析
        </div>
      </el-alert>

      <el-table :data="rules" v-loading="loading" size="small" stripe>
        <el-table-column label="容器" min-width="140" show-overflow-tooltip>
          <template #default="{ row }">
            <span>{{ device.displayName(row.containerName) }}</span>
            <span v-if="device.displayName(row.containerName) !== row.containerName"
              style="color: #666; font-size: 11px; margin-left: 4px">({{ row.containerName }})</span>
          </template>
        </el-table-column>
        <el-table-column label="IP" prop="containerIP" width="130" />
        <el-table-column label="状态" width="80" align="center">
          <template #default="{ row }">
            <el-tag :type="row.containerState === 'running' ? 'success' : 'info'" size="small">
              {{ row.containerState || '-' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="分组" prop="groupName" width="120" show-overflow-tooltip />
        <el-table-column label="节点" prop="vpcRemarks" min-width="150" show-overflow-tooltip />
        <el-table-column label="VPC 状态" width="90" align="center">
          <template #default="{ row }">
            <el-tag :type="row.status === 1 ? 'success' : 'danger'" size="small">
              {{ row.status === 1 ? '生效' : '未生效' }}
            </el-tag>
          </template>
        </el-table-column>
        <el-table-column label="DNS 白名单" width="110" align="center">
          <template #default="{ row }">
            <el-switch :model-value="!!(row.WhiteListDns && row.WhiteListDns.length)"
              size="small" @change="val => toggleDns(row, val)" :loading="togglingDns[row.id]" />
          </template>
        </el-table-column>
        <el-table-column label="操作" width="140" fixed="right">
          <template #default="{ row }">
            <el-button size="small" text type="primary" @click="openAssign(row)">更换</el-button>
            <el-popconfirm title="确认移除此容器的 VPC？" @confirm="removeRule(row)">
              <template #reference>
                <el-button type="danger" size="small" text>移除</el-button>
              </template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!rules.length && !loading" description="暂无容器绑定 VPC 节点" :image-size="60" />
    </el-card>

    <!-- 分配 VPC 弹窗 -->
    <el-dialog v-model="showAssign" :title="assignTarget ? '更换 VPC 节点' : '分配 VPC 节点'" width="800px"
      style="--el-dialog-padding-primary: 16px">
      <div v-if="assignTarget" style="margin-bottom: 12px">
        <span style="color: #999">容器：</span>
        <span style="color: #e0e0e0">{{ device.displayName(assignTarget.containerName) }}</span>
      </div>
      <div v-else style="margin-bottom: 12px">
        <div style="color: #999; margin-bottom: 6px">选择容器</div>
        <el-select v-model="assignForm.name" filterable placeholder="选择容器" style="width: 100%">
          <el-option v-for="c in availableContainers" :key="c.name" :label="device.displayName(c.name)" :value="c.name" />
        </el-select>
      </div>

      <div style="color: #999; margin-bottom: 6px">选择分组</div>
      <el-select v-model="assignForm.groupId" placeholder="先选择 VPC 分组" style="width: 100%; margin-bottom: 12px"
        @change="assignForm.vpcID = null">
        <el-option v-for="g in groups" :key="g.id" :label="`${g.alias}（${g.vpcs?.list?.length || 0} 个节点）`" :value="g.id" />
      </el-select>

      <template v-if="assignForm.groupId != null">
        <div style="color: #999; margin-bottom: 6px">选择节点</div>
        <div style="max-height: 280px; overflow-y: auto">
          <div v-for="node in assignGroupNodes" :key="node.id"
            :style="{
              display: 'flex', alignItems: 'center', padding: '10px 12px', marginBottom: '6px',
              borderRadius: '6px', cursor: 'pointer',
              border: assignForm.vpcID === node.id ? '1px solid #409eff' : '1px solid #333',
              background: assignForm.vpcID === node.id ? '#1a2b3d' : '#252525'
            }"
            @click="assignForm.vpcID = node.id">
            <el-radio :model-value="assignForm.vpcID" :value="node.id" style="margin-right: 10px" />
            <span style="color: #e0e0e0; margin-right: 8px">{{ node.remarks || '未命名节点' }}</span>
            <el-tag size="small" :type="protocolTagType(node.protocol)">{{ node.protocol || '-' }}</el-tag>
            <el-tag v-if="node.tag" size="small" type="info" style="margin-left: 4px">{{ node.tag }}</el-tag>
          </div>
          <el-empty v-if="!assignGroupNodes.length" description="该分组暂无节点" :image-size="40" />
        </div>
      </template>

      <template #footer>
        <el-button @click="showAssign = false">取消</el-button>
        <el-button type="primary" :loading="assigning" @click="doAssign">确认</el-button>
      </template>
    </el-dialog>

    <!-- 批量分配弹窗 -->
    <el-dialog v-model="showBatchAssign" title="批量分配 VPC" width="800px"
      style="--el-dialog-padding-primary: 16px">
      <div style="color: #999; margin-bottom: 6px">选择容器</div>
      <div style="max-height: 160px; overflow-y: auto; border: 1px solid #333; border-radius: 4px; padding: 8px; margin-bottom: 12px">
        <el-checkbox-group v-model="batchAssignNames">
          <el-checkbox v-for="c in availableContainers" :key="c.name" :label="c.name" :value="c.name"
            style="display: block; margin: 4px 0; color: #e0e0e0">
            {{ device.displayName(c.name) }}
          </el-checkbox>
        </el-checkbox-group>
      </div>
      <div style="color: #999; font-size: 11px; margin-bottom: 12px">已选 {{ batchAssignNames.length }} 个</div>

      <div style="color: #999; margin-bottom: 6px">选择分组</div>
      <el-select v-model="batchAssignGroupId" placeholder="先选择 VPC 分组" style="width: 100%; margin-bottom: 12px"
        @change="batchAssignVpcID = null">
        <el-option v-for="g in groups" :key="g.id" :label="`${g.alias}（${g.vpcs?.list?.length || 0} 个节点）`" :value="g.id" />
      </el-select>

      <template v-if="batchAssignGroupId != null">
        <div style="color: #999; margin-bottom: 6px">选择节点</div>
        <div style="max-height: 200px; overflow-y: auto">
          <div v-for="node in batchAssignGroupNodes" :key="node.id"
            :style="{
              display: 'flex', alignItems: 'center', padding: '10px 12px', marginBottom: '6px',
              borderRadius: '6px', cursor: 'pointer',
              border: batchAssignVpcID === node.id ? '1px solid #409eff' : '1px solid #333',
              background: batchAssignVpcID === node.id ? '#1a2b3d' : '#252525'
            }"
            @click="batchAssignVpcID = node.id">
            <el-radio :model-value="batchAssignVpcID" :value="node.id" style="margin-right: 10px" />
            <span style="color: #e0e0e0; margin-right: 8px">{{ node.remarks || '未命名节点' }}</span>
            <el-tag size="small" :type="protocolTagType(node.protocol)">{{ node.protocol || '-' }}</el-tag>
            <el-tag v-if="node.tag" size="small" type="info" style="margin-left: 4px">{{ node.tag }}</el-tag>
          </div>
          <el-empty v-if="!batchAssignGroupNodes.length" description="该分组暂无节点" :image-size="40" />
        </div>
      </template>

      <template #footer>
        <el-button @click="showBatchAssign = false">取消</el-button>
        <el-button type="primary" :loading="batchAssigning" @click="doBatchAssign">确认分配</el-button>
      </template>
    </el-dialog>

    <!-- 批量移除弹窗 -->
    <el-dialog v-model="showBatchRemove" title="批量移除 VPC" width="500px"
      style="--el-dialog-padding-primary: 16px">
      <template v-if="rules.length">
        <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 8px">
          <span style="color: #999">选择要移除 VPC 的容器</span>
          <el-checkbox :model-value="batchRemoveNames.length === rules.length && rules.length > 0"
            :indeterminate="batchRemoveNames.length > 0 && batchRemoveNames.length < rules.length"
            @change="val => batchRemoveNames = val ? rules.map(r => r.containerName) : []"
            style="color: #999">全选</el-checkbox>
        </div>
        <div style="max-height: 350px; overflow-y: auto">
          <div v-for="r in rules" :key="r.containerName"
            :style="{
              display: 'flex', alignItems: 'center', padding: '8px 12px', marginBottom: '6px',
              borderRadius: '6px', cursor: 'pointer',
              border: batchRemoveNames.includes(r.containerName) ? '1px solid #f56c6c' : '1px solid #333',
              background: batchRemoveNames.includes(r.containerName) ? '#2d1a1a' : '#252525'
            }"
            @click="toggleBatchRemove(r.containerName)">
            <el-checkbox :model-value="batchRemoveNames.includes(r.containerName)" style="margin-right: 10px; pointer-events: none" />
            <div style="flex: 1; min-width: 0">
              <div style="color: #e0e0e0">{{ device.displayName(r.containerName) }}</div>
              <div style="color: #666; font-size: 11px; margin-top: 2px">{{ r.groupName }} / {{ r.vpcRemarks || '-' }}</div>
            </div>
            <el-tag :type="r.containerState === 'running' ? 'success' : 'info'" size="small">{{ r.containerState || '-' }}</el-tag>
          </div>
        </div>
        <div style="color: #999; font-size: 12px; margin-top: 8px">已选 {{ batchRemoveNames.length }} / {{ rules.length }}</div>
      </template>
      <el-empty v-else description="当前没有容器绑定了 VPC，无需移除" :image-size="80" />

      <template #footer>
        <el-button @click="showBatchRemove = false">{{ rules.length ? '取消' : '关闭' }}</el-button>
        <el-button v-if="rules.length" type="danger" :loading="batchRemoving" :disabled="!batchRemoveNames.length" @click="doBatchRemove">
          移除{{ batchRemoveNames.length ? ` (${batchRemoveNames.length})` : '' }}
        </el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../../stores/device.js'

const props = defineProps({
  groups: { type: Array, default: () => [] },
  rules: { type: Array, default: () => [] },
  loading: Boolean
})
const emit = defineEmits(['refresh'])
const device = useDeviceStore()

// 可分配的容器列表（排除已有规则的）
const ruleNames = computed(() => new Set(props.rules.map(r => r.containerName)))
const availableContainers = computed(() =>
  device.containers.filter(c => !ruleNames.value.has(c.name))
)

// 协议标签颜色
function protocolTagType(protocol) {
  const p = (protocol || '').toLowerCase()
  if (p.includes('ss') || p === 'shadowsocks') return 'success'
  if (p.includes('vmess')) return ''
  if (p.includes('trojan')) return 'warning'
  if (p.includes('vless')) return 'danger'
  if (p.includes('hysteria')) return 'info'
  if (p.includes('socks')) return 'info'
  return 'info'
}

// 单个分配
const showAssign = ref(false)
const assigning = ref(false)
const assignTarget = ref(null)
const assignForm = reactive({ name: '', groupId: null, vpcID: null })

const assignGroupNodes = computed(() => {
  const g = props.groups.find(g => g.id === assignForm.groupId)
  return g?.vpcs?.list || []
})

function openAssign(rule) {
  assignTarget.value = rule || null
  assignForm.name = rule?.containerName || ''
  assignForm.groupId = null
  assignForm.vpcID = null
  showAssign.value = true
}
async function doAssign() {
  const name = assignTarget.value?.containerName || assignForm.name
  if (!name) { ElMessage.warning('请选择容器'); return }
  if (!assignForm.vpcID) { ElMessage.warning('请选择 VPC 节点'); return }
  assigning.value = true
  try {
    await device.request('sdk:addVpcRule', { name, vpcID: assignForm.vpcID })
    ElMessage.success('分配成功')
    showAssign.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '分配失败') }
  finally { assigning.value = false }
}

// 移除规则
async function removeRule(row) {
  try {
    await device.request('sdk:removeVpcRule', { name: row.containerName })
    ElMessage.success('已移除')
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '移除失败') }
}

// DNS 白名单切换
const togglingDns = reactive({})
async function toggleDns(row, val) {
  togglingDns[row.id] = true
  try {
    await device.request('sdk:toggleWhiteListDns', {
      ruleID: row.id,
      enable: val,
      whiteListDns: row.WhiteListDns || []
    })
    ElMessage.success(val ? 'DNS 白名单已开启' : 'DNS 白名单已关闭')
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '操作失败') }
  finally { togglingDns[row.id] = false }
}

// 批量分配
const showBatchAssign = ref(false)
const batchAssigning = ref(false)
const batchAssignNames = ref([])
const batchAssignGroupId = ref(null)
const batchAssignVpcID = ref(null)

const batchAssignGroupNodes = computed(() => {
  const g = props.groups.find(g => g.id === batchAssignGroupId.value)
  return g?.vpcs?.list || []
})

function openBatchAssign() {
  batchAssignNames.value = []
  batchAssignGroupId.value = null
  batchAssignVpcID.value = null
  showBatchAssign.value = true
}
async function doBatchAssign() {
  if (!batchAssignNames.value.length) { ElMessage.warning('请选择容器'); return }
  if (!batchAssignVpcID.value) { ElMessage.warning('请选择 VPC 节点'); return }
  batchAssigning.value = true
  try {
    await device.request('sdk:addVpcRuleBatch', {
      names: batchAssignNames.value,
      vpcID: batchAssignVpcID.value
    })
    ElMessage.success(`已为 ${batchAssignNames.value.length} 个容器分配 VPC`)
    showBatchAssign.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '批量分配失败') }
  finally { batchAssigning.value = false }
}

// 批量移除
const showBatchRemove = ref(false)
const batchRemoving = ref(false)
const batchRemoveNames = ref([])
function openBatchRemove() {
  batchRemoveNames.value = []
  showBatchRemove.value = true
}
function toggleBatchRemove(name) {
  const idx = batchRemoveNames.value.indexOf(name)
  if (idx >= 0) batchRemoveNames.value.splice(idx, 1)
  else batchRemoveNames.value.push(name)
}
async function doBatchRemove() {
  if (!batchRemoveNames.value.length) { ElMessage.warning('请选择容器'); return }
  batchRemoving.value = true
  try {
    await device.request('sdk:removeVpcRuleBatch', { name: batchRemoveNames.value })
    ElMessage.success(`已移除 ${batchRemoveNames.value.length} 个容器的 VPC`)
    showBatchRemove.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '批量移除失败') }
  finally { batchRemoving.value = false }
}
</script>

<style scoped>
:deep(.el-dialog__body) {
  padding: 12px 20px;
}
</style>
