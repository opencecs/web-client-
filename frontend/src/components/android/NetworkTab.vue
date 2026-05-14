<template>
  <div>
    <!-- 虚拟网卡 -->
    <el-card style="margin-bottom: 16px">
      <template #header>
        <div style="display: flex; justify-content: space-between; align-items: center">
          <span style="color: #f0f0f0; font-weight: bold">虚拟网卡</span>
          <el-space>
            <el-button size="small" type="primary" @click="showBridgeCreate = true">创建网卡</el-button>
            <el-button size="small" :icon="Refresh" @click="fetchBridges" :loading="loadingBridges" circle />
          </el-space>
        </div>
      </template>

      <!-- 功能说明 -->
      <el-alert type="info" :closable="false" show-icon style="margin-bottom: 12px">
        <template #title>
          <span style="font-weight: bold">什么是虚拟网卡？</span>
        </template>
        <div style="line-height: 1.8; color: #b0b0b0">
          虚拟网卡是为容器分配的独立网络通道，每张网卡拥有独立的 IP 地址段（CIDR），容器通过虚拟网卡实现网络隔离和通信。<br/>
          <b>使用场景：</b>当需要让不同容器在不同网段运行时，可以创建多张虚拟网卡，然后在创建容器时选择对应的网卡。<br/>
          <b>CIDR 格式：</b>例如 <code>172.18.0.0/16</code>，表示该网卡使用 172.18.x.x 网段，可容纳约 65000 个 IP 地址。
        </div>
      </el-alert>

      <el-table :data="bridges" v-loading="loadingBridges" size="small" stripe>
        <el-table-column label="网卡名称" prop="name" width="200" />
        <el-table-column label="IP 地址段 (CIDR)" prop="cidr" min-width="200" />
        <el-table-column label="操作" width="150">
          <template #default="{ row }">
            <el-button size="small" text @click="startEditBridge(row)">编辑</el-button>
            <el-popconfirm title="确认删除此网卡？删除后使用该网卡的容器将失去网络连接。" @confirm="deleteBridge(row)">
              <template #reference><el-button type="danger" size="small" text>删除</el-button></template>
            </el-popconfirm>
          </template>
        </el-table-column>
      </el-table>
      <el-empty v-if="!bridges.length && !loadingBridges" description="暂无虚拟网卡，点击「创建网卡」添加" :image-size="60" />
    </el-card>

    <!-- 创建网卡弹窗 -->
    <el-dialog v-model="showBridgeCreate" title="创建虚拟网卡" width="450px">
      <el-alert type="info" :closable="false" style="margin-bottom: 16px">
        <div style="line-height: 1.6; color: #b0b0b0">
          创建一张虚拟网卡，容器可以通过该网卡获得独立的网络地址。<br/>
          不同网卡之间的容器默认网络隔离，同一网卡下的容器可以互相通信。
        </div>
      </el-alert>
      <el-form label-width="100px">
        <el-form-item label="网卡名称">
          <el-input v-model="bridgeForm.name" placeholder="自定义名称，如 game-net" />
          <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">仅支持英文、数字和短横线，如：my-bridge</div>
        </el-form-item>
        <el-form-item label="IP 地址段">
          <el-input v-model="bridgeForm.cidr" placeholder="如 192.168.0.0（自动补/24）" />
          <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">
            CIDR 格式，不同网卡需使用不同网段，避免 IP 冲突。<br/>
            常用网段：172.18.0.0/16、172.19.0.0/16、10.10.0.0/16
          </div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBridgeCreate = false">取消</el-button>
        <el-button type="primary" :loading="creatingBridge" @click="createBridge">创建</el-button>
      </template>
    </el-dialog>

    <!-- 编辑网卡弹窗 -->
    <el-dialog v-model="showBridgeEdit" title="编辑虚拟网卡" width="450px">
      <el-form label-width="100px">
        <el-form-item label="网卡名称">
          <span style="color: #f0f0f0">{{ editBridgeName }}</span>
        </el-form-item>
        <el-form-item label="IP 地址段">
          <el-input v-model="editBridgeCidr" placeholder="新的 CIDR 地址段" />
          <div style="color: #b0b0b0; font-size: 11px; margin-top: 2px">修改后，已连接该网卡的容器可能需要重启才能生效</div>
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showBridgeEdit = false">取消</el-button>
        <el-button type="primary" :loading="updatingBridge" @click="updateBridge">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { Refresh } from '@element-plus/icons-vue'
import { useDeviceStore } from '../../stores/device.js'

const device = useDeviceStore()

// 虚拟网卡
const bridges = ref([])
const loadingBridges = ref(false)
const showBridgeCreate = ref(false)
const showBridgeEdit = ref(false)
const creatingBridge = ref(false)
const updatingBridge = ref(false)
const bridgeForm = reactive({ name: '', cidr: '' })
const editBridgeName = ref('')
const editBridgeCidr = ref('')

async function fetchBridges() {
  loadingBridges.value = true
  try {
    const resp = await device.request('sdk:listBridges')
    const d = resp.data
    const list = d?.data?.list || d?.list || d?.data || d
    bridges.value = Array.isArray(list) ? list : []
  } catch {} finally { loadingBridges.value = false }
}

async function createBridge() {
  if (!bridgeForm.name || !bridgeForm.cidr) { ElMessage.warning('请填写完整'); return }
  // 自动补全子网掩码：用户只输入 IP 时默认加 /24
  let cidr = bridgeForm.cidr.trim()
  if (!cidr.includes('/')) cidr += '/24'
  creatingBridge.value = true
  try {
    const resp = await device.request('sdk:createBridge', { customName: bridgeForm.name, cidr })
    const d = resp.data
    if (d?.code !== undefined && d.code !== 0) {
      ElMessage.error(d?.message || '创建失败')
      return
    }
    ElMessage.success('创建成功')
    showBridgeCreate.value = false
    bridgeForm.name = ''; bridgeForm.cidr = ''
    fetchBridges()
  } catch (e) { ElMessage.error(e.message || '创建失败') }
  finally { creatingBridge.value = false }
}

function startEditBridge(row) {
  editBridgeName.value = row.name
  editBridgeCidr.value = row.cidr
  showBridgeEdit.value = true
}

async function updateBridge() {
  updatingBridge.value = true
  try {
    const resp = await device.request('sdk:updateBridge', { name: editBridgeName.value, newCidr: editBridgeCidr.value })
    const d = resp.data
    if (d?.code !== undefined && d.code !== 0) {
      ElMessage.error(d?.message || '更新失败')
      return
    }
    ElMessage.success('更新成功')
    showBridgeEdit.value = false
    fetchBridges()
  } catch (e) { ElMessage.error(e.message || '更新失败') }
  finally { updatingBridge.value = false }
}

async function deleteBridge(row) {
  try {
    const resp = await device.request('sdk:deleteBridge', { name: row.name })
    const d = resp.data
    if (d?.code !== undefined && d.code !== 0) {
      ElMessage.error(d?.message || '删除失败')
      return
    }
    ElMessage.success('删除成功')
    fetchBridges()
  } catch (e) { ElMessage.error('删除失败') }
}

// 暴露 bridges 供外部使用（如创建容器时选择网卡）
defineExpose({ bridges, fetchBridges })

onMounted(() => { fetchBridges() })
</script>
