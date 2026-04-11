<template>
  <div style="padding: 24px">
    <h2 style="margin-top: 0; color: #e0e0e0">设备概览</h2>

    <el-row :gutter="16">
      <el-col :span="6">
        <DeviceStatusCard title="CPU 温度" :value="info.cputemp || 0" unit="°C"
          :max="80" show-progress :danger-threshold="80" :warning-threshold="60" />
      </el-col>
      <el-col :span="6">
        <DeviceStatusCard title="内存使用" :value="memUsed" unit="MB"
          :subtitle="`/ ${info.memtotal || 0} MB`" :max="Number(info.memtotal) || 1" show-progress />
      </el-col>
      <el-col :span="6">
        <DeviceStatusCard title="存储使用" :value="storageUsedGB" unit="GB"
          :subtitle="`/ ${storageTotalGB} GB`" :max="Number(storageTotalGB) || 1" show-progress />
      </el-col>
      <el-col :span="6">
        <DeviceStatusCard title="运行时间" :value="uptimeStr" subtitle="系统在线时长" />
      </el-col>
    </el-row>

    <el-row :gutter="16" style="margin-top: 16px">
      <el-col :span="12">
        <el-card style="background: #1a1a1a; border-color: #2a2a2a">
          <template #header><span style="color: #e0e0e0; font-weight: bold">设备信息</span></template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="SDK 版本">{{ info.version || '-' }}</el-descriptions-item>
            <el-descriptions-item label="设备型号">{{ info.model || '-' }}</el-descriptions-item>
            <el-descriptions-item label="设备 ID">{{ info.deviceId || '-' }}</el-descriptions-item>
            <el-descriptions-item label="IP 地址">{{ info.ip || '-' }}</el-descriptions-item>
            <el-descriptions-item label="MAC 地址">{{ info.hwaddr || '-' }}</el-descriptions-item>
            <el-descriptions-item label="网络速度">{{ info.speed || '-' }} Mbps</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
      <el-col :span="12">
        <el-card style="background: #1a1a1a; border-color: #2a2a2a">
          <template #header><span style="color: #e0e0e0; font-weight: bold">存储信息</span></template>
          <el-descriptions :column="1" border size="small">
            <el-descriptions-item label="硬盘型号">{{ info.mmcmodel || '-' }}</el-descriptions-item>
            <el-descriptions-item label="硬盘温度">{{ info.mmctemp || '-' }}°C</el-descriptions-item>
            <el-descriptions-item label="读取速度">{{ info.mmcread || '-' }}</el-descriptions-item>
            <el-descriptions-item label="写入速度">{{ info.mmcwrite || '-' }}</el-descriptions-item>
            <el-descriptions-item label="CPU 负载">{{ info.cpuload || '-' }}</el-descriptions-item>
          </el-descriptions>
        </el-card>
      </el-col>
    </el-row>
  </div>
</template>

<script setup>
import { computed, onMounted } from 'vue'
import { useDeviceStore } from '../stores/device.js'
import DeviceStatusCard from '../components/DeviceStatusCard.vue'

const device = useDeviceStore()
const info = computed(() => device.status || {})
const memUsed = computed(() => info.value.memuse || 0)
const storageUsed = computed(() => info.value.mmcuse || 0)
const storageUsedGB = computed(() => (Number(info.value.mmcuse) / 1024).toFixed(1))
const storageTotalGB = computed(() => (Number(info.value.mmctotal) / 1024).toFixed(1))

const uptimeStr = computed(() => {
  const raw = String(info.value.sysuptime || '')
  const sec = parseInt(raw.replace(/[^0-9]/g, ''))
  if (!sec && sec !== 0) return '-'
  const days = Math.floor(sec / 86400)
  const hours = Math.floor((sec % 86400) / 3600)
  const minutes = Math.floor((sec % 3600) / 60)
  const parts = []
  if (days > 0) parts.push(`${days}天`)
  if (hours > 0) parts.push(`${hours}小时`)
  parts.push(`${minutes}分钟`)
  return parts.join(' ')
})

onMounted(async () => {
  try {
    const resp = await device.request('device:info')
    device.status = resp.data?.data || resp.data
  } catch (e) {}
})
</script>
