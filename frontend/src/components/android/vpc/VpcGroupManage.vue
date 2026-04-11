<template>
  <div>
    <!-- VPC 分组管理 -->
    <el-card style="background: #1e1e1e; border-color: #333; margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #e0e0e0; font-weight: bold">VPC 分组</span>
          <el-space>
            <el-button size="small" type="primary" @click="openCreateGroup">添加订阅</el-button>
            <el-button size="small" @click="openSocksDialog">添加 SOCKS5</el-button>
            <el-button size="small" :icon="Refresh" @click="$emit('refresh')" :loading="loading" circle />
          </el-space>
        </div>
      </template>

      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        <template #title><span style="font-weight: bold">什么是 VPC？</span></template>
        <div style="line-height: 1.8; color: #b0b0b0">
          VPC（虚拟专用通道）为容器提供独立的网络出口节点，绑定后容器所有流量通过该节点转发，实现 IP 隔离。<br/>
          <b>使用流程：</b>① 创建分组并添加节点 → ② 在「容器 VPC 规则」中将容器绑定到节点 → ③ 容器流量自动走 VPC<br/>
          <b>三种添加方式：</b><br/>
          &nbsp;&nbsp;• <b>订阅地址</b> — 填入机场/服务商提供的订阅 URL，自动拉取全部节点，支持一键刷新更新<br/>
          &nbsp;&nbsp;• <b>手动节点</b> — 粘贴协议链接（ss://、vmess:// 等）或逐项填写服务器信息<br/>
          &nbsp;&nbsp;• <b>SOCKS5</b> — 直接填写 SOCKS5 代理的 IP、端口、账密
        </div>
      </el-alert>

      <el-table :data="groups" v-loading="loading" size="small" stripe row-key="id"
        :expand-row-keys="expandedRows" @expand-change="onExpandChange">
        <el-table-column type="expand">
          <template #default="{ row }">
            <div style="padding: 8px 16px">
              <div style="display: flex; justify-content: flex-end; margin-bottom: 6px">
                <el-button size="small" type="primary" text
                  :loading="batchTesting[row.id]"
                  :disabled="!(row.vpcs?.list?.length)"
                  @click.stop="testAllNodes(row)">
                  {{ batchTesting[row.id] ? `测试中 (${batchTestProgress[row.id] || 0}/${row.vpcs?.list?.length || 0})` : '全部测试' }}
                </el-button>
              </div>
              <el-table :data="row.vpcs?.list || []" size="small" style="background: transparent">
                <el-table-column label="备注" prop="remarks" min-width="150" show-overflow-tooltip />
                <el-table-column label="协议" prop="protocol" width="100">
                  <template #default="{ row: node }">
                    <el-tag size="small" type="info">{{ node.protocol || '-' }}</el-tag>
                  </template>
                </el-table-column>
                <el-table-column label="标签" prop="tag" width="120" show-overflow-tooltip />
                <el-table-column label="延迟" width="120">
                  <template #default="{ row: node }">
                    <el-button v-if="!nodeLatency[node.id]" size="small" text type="primary"
                      :loading="testingNodes[node.id]" @click.stop="testNode(node)">测试</el-button>
                    <span v-else :style="{ color: latencyColor(nodeLatency[node.id]) }">
                      {{ nodeLatency[node.id] }}
                    </span>
                  </template>
                </el-table-column>
                <el-table-column label="操作" width="80">
                  <template #default="{ row: node }">
                    <el-popconfirm title="确认删除此节点？" @confirm="deleteNode(node)">
                      <template #reference>
                        <el-button type="danger" size="small" text>删除</el-button>
                      </template>
                    </el-popconfirm>
                  </template>
                </el-table-column>
              </el-table>
              <el-empty v-if="!(row.vpcs?.list?.length)" description="该分组暂无节点" :image-size="40" />
            </div>
          </template>
        </el-table-column>
        <el-table-column label="别名" prop="alias" min-width="150" show-overflow-tooltip />
        <el-table-column label="类型" width="100">
          <template #default="{ row }">
            <el-tag :type="row.url ? 'primary' : 'warning'" size="small">{{ row.url ? '订阅' : '手动' }}</el-tag>
          </template>
        </el-table-column>
        <el-table-column label="节点数" width="80" align="center">
          <template #default="{ row }">{{ row.vpcs?.vpcCount || row.vpcs?.list?.length || 0 }}</template>
        </el-table-column>
        <el-table-column label="订阅地址" prop="url" min-width="200" show-overflow-tooltip>
          <template #default="{ row }">
            <span style="color: #999">{{ row.url || '-' }}</span>
          </template>
        </el-table-column>
        <el-table-column label="操作" width="220" fixed="right">
          <template #default="{ row }">
            <el-space :size="4" wrap>
              <el-button size="small" text @click="openRename(row)">重命名</el-button>
              <el-button v-if="row.url" size="small" text type="primary"
                :loading="refreshingGroups[row.id]" @click="refreshGroup(row)">刷新</el-button>
              <el-popconfirm title="确认删除此分组及其所有节点？" @confirm="deleteGroup(row)">
                <template #reference>
                  <el-button type="danger" size="small" text>删除</el-button>
                </template>
              </el-popconfirm>
            </el-space>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!groups.length && !loading" description="暂无 VPC 分组，点击「添加订阅」或「添加 SOCKS5」" :image-size="60" />
    </el-card>

    <!-- 创建分组弹窗 -->
    <el-dialog v-model="showCreate" title="添加 VPC 分组" width="600px">
      <el-form label-width="100px">
        <el-form-item label="分组别名">
          <el-input v-model="createForm.alias" placeholder="给分组取个名字，如：美国节点、香港线路" />
        </el-form-item>
        <el-form-item label="添加方式">
          <el-radio-group v-model="createForm.source">
            <el-radio :value="1">订阅地址</el-radio>
            <el-radio :value="2">手动节点</el-radio>
          </el-radio-group>
        </el-form-item>

        <!-- 订阅模式 -->
        <template v-if="createForm.source === 1">
          <el-form-item label="订阅 URL">
            <el-input v-model="createForm.url" placeholder="https://example.com/subscribe/token..." />
            <div style="color: #999; font-size: 11px; margin-top: 2px">填入服务商提供的订阅链接，系统自动解析所有节点</div>
          </el-form-item>
        </template>

        <!-- 手动模式 -->
        <template v-if="createForm.source === 2">
          <el-form-item label="输入方式">
            <el-radio-group v-model="createForm.manualMode" size="small">
              <el-radio-button value="link">粘贴协议链接</el-radio-button>
              <el-radio-button value="form">逐项填写</el-radio-button>
            </el-radio-group>
          </el-form-item>

          <!-- 粘贴链接模式 -->
          <el-form-item v-if="createForm.manualMode === 'link'" label="节点链接">
            <el-input v-model="createForm.addressText" type="textarea" :rows="6"
              placeholder="每行粘贴一个协议链接，例如：&#10;ss://YWVzLTI1Ni1jZmI6cGFzc3dvcmQ@1.2.3.4:8388#节点名&#10;vmess://eyJhZGQiOiIxLjIuMy40IiwicG9ydCI6IjQ0MyJ9&#10;trojan://password@1.2.3.4:443#节点名&#10;vless://uuid@1.2.3.4:443?type=tcp#节点名&#10;socks5://user:pass@1.2.3.4:1080#节点名" />
            <div style="margin-top: 6px; padding: 8px 12px; background: #252525; border-radius: 4px; font-size: 11px; line-height: 1.8; color: #999">
              <b style="color: #b0b0b0">支持的协议格式：</b><br/>
              <code style="color: #67c23a">ss://</code> Shadowsocks — <span style="color: #666">ss://method:password@host:port#name</span><br/>
              <code style="color: #409eff">vmess://</code> VMess — <span style="color: #666">vmess://base64编码的JSON配置</span><br/>
              <code style="color: #e6a23c">trojan://</code> Trojan — <span style="color: #666">trojan://password@host:port#name</span><br/>
              <code style="color: #f56c6c">vless://</code> VLESS — <span style="color: #666">vless://uuid@host:port?type=tcp&security=tls#name</span><br/>
              <code style="color: #909399">hysteria2://</code> Hysteria2 — <span style="color: #666">hysteria2://auth@host:port#name</span><br/>
              <code style="color: #b88dff">socks5://</code> SOCKS5 — <span style="color: #666">socks5://user:password@host:port#name（无认证可省略 user:password@）</span><br/>
              从客户端导出的分享链接可直接粘贴，每行一个
            </div>
          </el-form-item>

          <!-- 逐项填写模式 -->
          <template v-if="createForm.manualMode === 'form'">
            <div v-for="(item, idx) in createForm.formNodes" :key="idx"
              style="border: 1px solid #333; border-radius: 6px; padding: 16px; margin-bottom: 12px; background: #252525">
              <div style="display: flex; justify-content: space-between; align-items: center; margin-bottom: 12px">
                <span style="color: #e0e0e0; font-size: 13px; font-weight: bold">节点 {{ idx + 1 }}</span>
                <el-button :icon="Delete" circle size="small" @click="createForm.formNodes.splice(idx, 1)"
                  :disabled="createForm.formNodes.length <= 1" />
              </div>
              <el-form label-width="80px" label-position="left" style="margin-bottom: 0">
                <el-form-item label="协议" style="margin-bottom: 10px">
                  <el-select v-model="item.protocol" style="width: 100%">
                    <el-option label="Shadowsocks (SS)" value="ss" />
                    <el-option label="VMess" value="vmess" />
                    <el-option label="Trojan" value="trojan" />
                    <el-option label="VLESS" value="vless" />
                    <el-option label="Hysteria2" value="hysteria2" />
                    <el-option label="SOCKS5" value="socks5" />
                  </el-select>
                </el-form-item>
                <el-form-item label="服务器" style="margin-bottom: 10px">
                  <div style="display: flex; gap: 8px; width: 100%">
                    <el-input v-model="item.address" placeholder="IP 或域名，如 1.2.3.4" style="flex: 1" />
                    <el-input-number v-model="item.port" :min="1" :max="65535" placeholder="端口"
                      style="width: 130px" controls-position="right" />
                  </div>
                </el-form-item>
                <el-form-item v-if="item.protocol === 'ss'" label="加密方式" style="margin-bottom: 10px">
                  <el-select v-model="item.method" style="width: 100%">
                    <el-option label="aes-256-gcm" value="aes-256-gcm" />
                    <el-option label="aes-128-gcm" value="aes-128-gcm" />
                    <el-option label="chacha20-ietf-poly1305" value="chacha20-ietf-poly1305" />
                    <el-option label="2022-blake3-aes-256-gcm" value="2022-blake3-aes-256-gcm" />
                    <el-option label="2022-blake3-aes-128-gcm" value="2022-blake3-aes-128-gcm" />
                  </el-select>
                </el-form-item>
                <el-form-item :label="item.protocol === 'vless' || item.protocol === 'vmess' ? 'UUID' : '密码'" style="margin-bottom: 10px">
                  <el-input v-model="item.password"
                    :placeholder="item.protocol === 'vless' || item.protocol === 'vmess' ? '填入 UUID' : item.protocol === 'socks5' ? '无密码可留空' : '填入密码'"
                    :type="item.protocol === 'vless' || item.protocol === 'vmess' ? 'text' : 'password'"
                    :show-password="item.protocol !== 'vless' && item.protocol !== 'vmess'" />
                </el-form-item>
                <el-form-item v-if="item.protocol === 'socks5'" label="用户名" style="margin-bottom: 10px">
                  <el-input v-model="item.socksUser" placeholder="无用户名可留空" />
                </el-form-item>
                <el-form-item label="备注" style="margin-bottom: 0">
                  <el-input v-model="item.remarks" placeholder="给节点取个名，如：香港01（可选）" />
                </el-form-item>
              </el-form>
            </div>
            <el-button size="small" type="primary" text :icon="Plus" @click="addFormNode" style="margin-left: 100px">添加下一个节点</el-button>
          </template>
        </template>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" :loading="creating" @click="doCreateGroup">创建</el-button>
      </template>
    </el-dialog>

    <!-- 重命名弹窗 -->
    <el-dialog v-model="showRename" title="重命名分组" width="400px">
      <el-form label-width="100px">
        <el-form-item label="当前名称">
          <span style="color: #e0e0e0">{{ renameTarget?.alias }}</span>
        </el-form-item>
        <el-form-item label="新名称">
          <el-input v-model="renameAlias" placeholder="输入新的分组别名" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showRename = false">取消</el-button>
        <el-button type="primary" :loading="renaming" @click="doRename">保存</el-button>
      </template>
    </el-dialog>

    <!-- SOCKS5 弹窗 -->
    <el-dialog v-model="showSocks" title="添加 SOCKS5 节点" width="650px">
      <el-form label-width="100px">
        <el-form-item label="目标分组">
          <el-input v-model="socksForm.alias" placeholder="分组别名（不存在则自动创建）" />
        </el-form-item>
        <el-form-item label="节点列表">
          <div style="width: 100%">
            <div v-for="(item, idx) in socksForm.list" :key="idx"
              style="display: flex; gap: 6px; margin-bottom: 8px; align-items: center">
              <el-input v-model="item.remarks" placeholder="备注" style="width: 100px" size="small" />
              <el-input v-model="item.socksIp" placeholder="IP 地址" style="width: 130px" size="small" />
              <el-input-number v-model="item.socksPort" :min="1" :max="65535" placeholder="端口"
                style="width: 100px" size="small" controls-position="right" />
              <el-input v-model="item.socksUser" placeholder="用户名" style="width: 90px" size="small" />
              <el-input v-model="item.socksPassword" placeholder="密码" style="width: 90px" size="small"
                type="password" show-password />
              <el-button :icon="Delete" circle size="small" @click="socksForm.list.splice(idx, 1)"
                :disabled="socksForm.list.length <= 1" />
            </div>
            <el-button size="small" type="primary" text :icon="Plus" @click="addSocksRow">添加节点</el-button>
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showSocks = false">取消</el-button>
        <el-button type="primary" :loading="addingSocks" @click="doAddSocks">提交</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh, Delete, Plus } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../../stores/device.js'

const props = defineProps({
  groups: { type: Array, default: () => [] },
  loading: Boolean
})
const emit = defineEmits(['refresh'])
const device = useDeviceStore()

// 展开行
const expandedRows = ref([])
function onExpandChange(row) {
  if (expandedRows.value.includes(row.id)) {
    expandedRows.value = []
  } else {
    expandedRows.value = [row.id]
  }
}

// 延迟测试
const nodeLatency = reactive({})
const testingNodes = reactive({})
const batchTesting = reactive({})
const batchTestProgress = reactive({})

async function testNode(node) {
  const addr = node.outConfig || node.remarks || ''
  if (!addr) { ElMessage.warning('节点无可测试地址'); return }
  testingNodes[node.id] = true
  try {
    const resp = await device.request('sdk:testVpcNode', { address: addr })
    const d = resp.data?.data || resp.data || {}
    nodeLatency[node.id] = d.latency || d.msg || '超时'
  } catch { nodeLatency[node.id] = '失败' }
  finally { testingNodes[node.id] = false }
}

async function testAllNodes(group) {
  const nodes = group.vpcs?.list || []
  if (!nodes.length) return
  batchTesting[group.id] = true
  batchTestProgress[group.id] = 0
  for (const node of nodes) {
    await testNode(node)
    batchTestProgress[group.id]++
  }
  batchTesting[group.id] = false
}
function latencyColor(val) {
  if (!val || val === '失败' || val === '超时') return '#f56c6c'
  const ms = parseInt(val)
  if (isNaN(ms)) return '#e0e0e0'
  if (ms < 200) return '#67c23a'
  if (ms < 500) return '#e6a23c'
  return '#f56c6c'
}

// 删除节点
async function deleteNode(node) {
  try {
    await device.request('sdk:deleteVpcNode', { vpcID: node.id })
    ElMessage.success('节点已删除')
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '删除失败') }
}

// 创建分组
const showCreate = ref(false)
const creating = ref(false)
const createForm = reactive({ alias: '', source: 1, url: '', addressText: '', manualMode: 'link', formNodes: [newFormNode()] })

function newFormNode() {
  return { protocol: 'ss', address: '', port: null, password: '', method: 'aes-256-gcm', remarks: '', socksUser: '' }
}
function addFormNode() {
  createForm.formNodes.push(newFormNode())
}

function openCreateGroup() {
  createForm.alias = ''; createForm.source = 1; createForm.url = ''; createForm.addressText = ''
  createForm.manualMode = 'link'; createForm.formNodes = [newFormNode()]
  showCreate.value = true
}

// 将表单节点转成协议链接
function formNodesToAddresses() {
  return createForm.formNodes.filter(n => n.address && n.port).map(n => {
    const name = n.remarks ? '#' + encodeURIComponent(n.remarks) : ''
    switch (n.protocol) {
      case 'ss': {
        const userinfo = btoa(`${n.method}:${n.password}`)
        return `ss://${userinfo}@${n.address}:${n.port}${name}`
      }
      case 'trojan':
        return `trojan://${encodeURIComponent(n.password)}@${n.address}:${n.port}${name}`
      case 'vless':
        return `vless://${n.password}@${n.address}:${n.port}?type=tcp${name}`
      case 'vmess': {
        const cfg = btoa(JSON.stringify({ v: '2', add: n.address, port: String(n.port), id: n.password, ps: n.remarks || '' }))
        return `vmess://${cfg}`
      }
      case 'hysteria2':
        return `hysteria2://${encodeURIComponent(n.password)}@${n.address}:${n.port}${name}`
      case 'socks5': {
        const auth = n.socksUser ? `${encodeURIComponent(n.socksUser)}:${encodeURIComponent(n.password)}@` : n.password ? `${encodeURIComponent(n.password)}@` : ''
        return `socks5://${auth}${n.address}:${n.port}${name}`
      }
      default:
        return `${n.protocol}://${n.address}:${n.port}`
    }
  })
}
async function doCreateGroup() {
  if (!createForm.alias) { ElMessage.warning('请输入分组别名'); return }
  if (createForm.source === 1 && !createForm.url) { ElMessage.warning('请输入订阅地址'); return }
  if (createForm.source === 2) {
    if (createForm.manualMode === 'link' && !createForm.addressText.trim()) { ElMessage.warning('请输入节点链接'); return }
    if (createForm.manualMode === 'form') {
      const valid = createForm.formNodes.filter(n => n.address && n.port)
      if (!valid.length) { ElMessage.warning('请至少填写一个有效节点（地址 + 端口）'); return }
    }
  }
  creating.value = true
  try {
    const data = { alias: createForm.alias, source: createForm.source }
    if (createForm.source === 1) {
      data.url = createForm.url
    } else {
      if (createForm.manualMode === 'link') {
        data.addresses = createForm.addressText.split('\n').map(s => s.trim()).filter(Boolean)
      } else {
        data.addresses = formNodesToAddresses()
      }
    }
    await device.request('sdk:createVpcGroup', data)
    ElMessage.success('分组创建成功')
    showCreate.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '创建失败') }
  finally { creating.value = false }
}

// 重命名
const showRename = ref(false)
const renaming = ref(false)
const renameTarget = ref(null)
const renameAlias = ref('')
function openRename(row) {
  renameTarget.value = row
  renameAlias.value = row.alias
  showRename.value = true
}
async function doRename() {
  if (!renameAlias.value.trim()) { ElMessage.warning('请输入新名称'); return }
  renaming.value = true
  try {
    await device.request('sdk:renameVpcGroup', { id: renameTarget.value.id, newAlias: renameAlias.value.trim() })
    ElMessage.success('重命名成功')
    showRename.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '重命名失败') }
  finally { renaming.value = false }
}

// 刷新订阅
const refreshingGroups = reactive({})
async function refreshGroup(row) {
  refreshingGroups[row.id] = true
  try {
    await device.request('sdk:refreshVpcGroup', { id: row.id }, 60000)
    ElMessage.success('订阅已更新')
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '刷新失败') }
  finally { refreshingGroups[row.id] = false }
}

// 删除分组
async function deleteGroup(row) {
  try {
    await device.request('sdk:deleteVpcGroup', { id: String(row.id) })
    ElMessage.success('分组已删除')
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '删除失败') }
}

// SOCKS5
const showSocks = ref(false)
const addingSocks = ref(false)
const socksForm = reactive({
  alias: '',
  list: [{ remarks: '', socksIp: '', socksPort: null, socksUser: '', socksPassword: '' }]
})
function openSocksDialog() {
  socksForm.alias = ''
  socksForm.list = [{ remarks: '', socksIp: '', socksPort: null, socksUser: '', socksPassword: '' }]
  showSocks.value = true
}
function addSocksRow() {
  socksForm.list.push({ remarks: '', socksIp: '', socksPort: null, socksUser: '', socksPassword: '' })
}
async function doAddSocks() {
  if (!socksForm.alias) { ElMessage.warning('请输入分组别名'); return }
  const valid = socksForm.list.filter(s => s.socksIp && s.socksPort)
  if (!valid.length) { ElMessage.warning('请至少填写一个有效节点（IP + 端口）'); return }
  addingSocks.value = true
  try {
    await device.request('sdk:addVpcSocks', { alias: socksForm.alias, list: valid })
    ElMessage.success('SOCKS5 节点添加成功')
    showSocks.value = false
    emit('refresh')
  } catch (e) { ElMessage.error(e.message || '添加失败') }
  finally { addingSocks.value = false }
}
</script>
