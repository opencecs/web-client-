<template>
  <div class="mobile-vpc">
    <van-nav-bar title="VPC 管理" left-arrow @click-left="$router.back()" :border="false" />

    <van-tabs v-model:active="activeTab" sticky>
      <!-- VPC 分组 -->
      <van-tab title="分组" name="groups">
        <van-pull-refresh v-model="refreshingGroups" @refresh="onRefreshGroups">
          <div class="list-section">
            <div v-for="g in groups" :key="g.id" class="vpc-card">
              <div class="vpc-header">
                <span class="vpc-name">{{ g.alias || g.id }}</span>
                <van-tag type="primary" size="medium">{{ g.vpcs?.vpcCount || 0 }} 节点</van-tag>
              </div>
              <div v-if="g.url" class="vpc-url">{{ g.url }}</div>
              <div v-if="g.vpcs?.list?.length" class="vpc-nodes">
                <div v-for="v in g.vpcs.list" :key="v.id" class="node-item">
                  <span class="node-name">{{ v.remarks || v.id }}</span>
                  <span class="node-proto">{{ v.protocol }}</span>
                </div>
              </div>
              <div class="vpc-actions">
                <van-button size="small" plain @click="deleteGroup(g)">删除</van-button>
              </div>
            </div>
          </div>
          <van-empty v-if="!groups.length && !loadingGroups" description="暂无 VPC 分组" />
        </van-pull-refresh>
      </van-tab>

      <!-- 容器规则 -->
      <van-tab title="规则" name="rules">
        <van-pull-refresh v-model="refreshingRules" @refresh="onRefreshRules">
          <div class="list-section">
            <div v-for="r in rules" :key="r.id" class="rule-card">
              <div class="rule-header">
                <span class="rule-name">{{ r.containerName || r.containerID }}</span>
                <van-tag :type="r.status === 'active' ? 'success' : 'default'" size="medium">
                  {{ r.status === 'active' ? '已生效' : '未生效' }}
                </van-tag>
              </div>
              <div class="rule-detail">
                <span>分组: {{ r.groupName || '-' }}</span>
                <span>节点: {{ r.vpcRemarks || '-' }}</span>
              </div>
              <div class="rule-actions">
                <van-button size="small" type="danger" plain @click="removeRule(r)">清除规则</van-button>
              </div>
            </div>
          </div>
          <van-empty v-if="!rules.length && !loadingRules" description="暂无容器规则" />
        </van-pull-refresh>
      </van-tab>
    </van-tabs>
  </div>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { showToast, showConfirmDialog } from 'vant'

const device = useDeviceStore()

const activeTab = ref('groups')
const groups = ref([])
const rules = ref([])
const loadingGroups = ref(false)
const loadingRules = ref(false)
const refreshingGroups = ref(false)
const refreshingRules = ref(false)

async function fetchGroups() {
  loadingGroups.value = true
  try {
    const resp = await device.request('sdk:listVpcGroups')
    const d = resp.data
    groups.value = Array.isArray(d?.data?.list) ? d.data.list : Array.isArray(d?.list) ? d.list : []
  } catch {} finally { loadingGroups.value = false }
}

async function fetchRules() {
  loadingRules.value = true
  try {
    const resp = await device.request('sdk:listContainerRules')
    const d = resp.data
    rules.value = Array.isArray(d?.data?.list) ? d.data.list : Array.isArray(d?.list) ? d.list : []
  } catch {} finally { loadingRules.value = false }
}

function onRefreshGroups() { fetchGroups(); setTimeout(() => refreshingGroups.value = false, 800) }
function onRefreshRules() { fetchRules(); setTimeout(() => refreshingRules.value = false, 800) }

async function deleteGroup(g) {
  try {
    await showConfirmDialog({ title: '确认', message: `删除分组 ${g.alias || g.id}？` })
    await device.request('sdk:deleteVpcGroup', { id: g.id })
    showToast('删除成功'); fetchGroups()
  } catch {}
}

async function removeRule(r) {
  try {
    await showConfirmDialog({ title: '确认', message: `清除容器 ${r.containerName} 的 VPC 规则？` })
    await device.request('sdk:removeVpcRule', { name: r.containerName })
    showToast('已清除'); fetchRules()
  } catch {}
}

function loadData() { fetchGroups(); fetchRules() }
onMounted(() => { if (device.online) loadData() })
watch(() => device.online, (v) => { if (v) loadData() })
</script>

<style scoped>
.mobile-vpc { background: #0a0a0a; min-height: 100vh; }

.list-section { padding: 12px; }

.vpc-card, .rule-card {
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 10px;
  padding: 12px;
  margin-bottom: 8px;
}
.vpc-header, .rule-header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 6px; }
.vpc-name, .rule-name { font-size: 14px; font-weight: 600; color: #e0e0e0; }
.vpc-url { font-size: 11px; color: #666; margin-bottom: 6px; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }

.vpc-nodes { margin: 8px 0; }
.node-item { display: flex; justify-content: space-between; padding: 4px 8px; background: #141414; border-radius: 6px; margin-bottom: 4px; font-size: 12px; }
.node-name { color: #bbb; }
.node-proto { color: #666; }

.vpc-actions, .rule-actions { display: flex; gap: 8px; margin-top: 8px; }

.rule-detail { display: flex; gap: 16px; font-size: 12px; color: #999; margin-bottom: 4px; }
</style>
