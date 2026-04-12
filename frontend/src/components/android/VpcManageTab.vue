<template>
  <el-row :gutter="16">
    <el-col :span="12">
      <VpcGroupManage :groups="groups" :loading="loadingGroups" @refresh="fetchGroups" />
    </el-col>
    <el-col :span="12">
      <VpcContainerRules :groups="groups" :rules="rules" :loading="loadingRules" @refresh="fetchRules" />
      <VpcDomainManage :rules="rules" />
    </el-col>
  </el-row>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { ElMessage } from 'element-plus'
import { useDeviceStore } from '../../stores/device.js'
import VpcGroupManage from './vpc/VpcGroupManage.vue'
import VpcContainerRules from './vpc/VpcContainerRules.vue'
import VpcDomainManage from './vpc/VpcDomainManage.vue'

const device = useDeviceStore()

const groups = ref([])
const rules = ref([])
const loadingGroups = ref(false)
const loadingRules = ref(false)

async function fetchGroups() {
  loadingGroups.value = true
  try {
    const resp = await device.request('sdk:listVpcGroups')
    const d = resp.data
    groups.value = Array.isArray(d?.data?.list) ? d.data.list : Array.isArray(d?.list) ? d.list : []
  } catch (e) { groups.value = []; ElMessage.error('加载 VPC 分组失败') }
  finally { loadingGroups.value = false }
}

async function fetchRules() {
  loadingRules.value = true
  try {
    const resp = await device.request('sdk:listContainerRules')
    const d = resp.data
    rules.value = Array.isArray(d?.data?.list) ? d.data.list : Array.isArray(d?.list) ? d.list : []
  } catch (e) { rules.value = []; ElMessage.error('加载容器规则失败') }
  finally { loadingRules.value = false }
}

onMounted(() => {
  Promise.allSettled([fetchGroups(), fetchRules()])
})
</script>
