<template>
  <div class="mobile-network">
    <van-nav-bar title="虚拟网卡" left-arrow @click-left="$router.back()" :border="false">
      <template #right>
        <van-icon name="plus" size="20" @click="showCreate = true" />
      </template>
    </van-nav-bar>

    <!-- 说明 -->
    <div class="info-banner">
      虚拟网卡为容器提供独立网络通道，不同网卡间的容器网络隔离。
    </div>

    <!-- 网卡列表 -->
    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="bridge-list">
        <van-swipe-cell v-for="b in bridges" :key="b.name">
          <van-cell :title="b.name" :label="b.cidr" @click="startEdit(b)" is-link />
          <template #right>
            <van-button type="danger" square text="删除" class="swipe-btn" @click="deleteBridge(b)" />
          </template>
        </van-swipe-cell>
      </div>
      <van-empty v-if="!bridges.length && !loading" description="暂无虚拟网卡" />
    </van-pull-refresh>

    <!-- 创建弹窗 -->
    <van-dialog v-model:show="showCreate" title="创建虚拟网卡" show-cancel-button @confirm="doCreate">
      <div style="padding: 16px">
        <van-field v-model="form.name" label="名称" placeholder="如 game-net" />
        <van-field v-model="form.cidr" label="IP 段" placeholder="如 172.18.0.0/16" />
      </div>
    </van-dialog>

    <!-- 编辑弹窗 -->
    <van-dialog v-model:show="showEdit" title="编辑网卡" show-cancel-button @confirm="doUpdate">
      <div style="padding: 16px">
        <van-field :model-value="editName" label="名称" readonly />
        <van-field v-model="editCidr" label="IP 段" placeholder="新的 CIDR" />
      </div>
    </van-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, onMounted, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { showToast, showConfirmDialog } from 'vant'

const device = useDeviceStore()
const bridges = ref([])
const loading = ref(false)
const refreshing = ref(false)
const showCreate = ref(false)
const showEdit = ref(false)
const form = reactive({ name: '', cidr: '' })
const editName = ref('')
const editCidr = ref('')

async function fetchBridges() {
  loading.value = true
  try {
    const resp = await device.request('sdk:listBridges')
    const d = resp.data
    bridges.value = Array.isArray(d?.data) ? d.data : Array.isArray(d) ? d : []
  } catch {} finally { loading.value = false }
}

function onRefresh() { fetchBridges(); setTimeout(() => refreshing.value = false, 800) }

async function doCreate() {
  if (!form.name || !form.cidr) { showToast('请填写完整'); return }
  try {
    await device.request('sdk:createBridge', { customName: form.name, cidr: form.cidr })
    showToast('创建成功'); form.name = ''; form.cidr = ''; fetchBridges()
  } catch (e) { showToast(e.message || '创建失败') }
}

function startEdit(b) { editName.value = b.name; editCidr.value = b.cidr; showEdit.value = true }

async function doUpdate() {
  try {
    await device.request('sdk:updateBridge', { name: editName.value, newCidr: editCidr.value })
    showToast('更新成功'); fetchBridges()
  } catch (e) { showToast(e.message || '更新失败') }
}

async function deleteBridge(b) {
  try {
    await showConfirmDialog({ title: '确认', message: `删除网卡 ${b.name}？` })
    await device.request('sdk:deleteBridge', { name: b.name })
    showToast('删除成功'); fetchBridges()
  } catch {}
}

onMounted(() => { if (device.online) fetchBridges() })
watch(() => device.online, (v) => { if (v) fetchBridges() })
</script>

<style scoped>
.mobile-network { background: #0a0a0a; min-height: 100vh; }
.info-banner { margin: 12px; padding: 10px 14px; background: #1a1a1a; border: 1px solid #2a2a2a; border-radius: 10px; font-size: 12px; color: #999; line-height: 1.6; }
.bridge-list { padding: 0 0 24px; }
.swipe-btn { height: 100%; }
</style>
