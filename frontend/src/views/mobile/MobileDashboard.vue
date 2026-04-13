<template>
  <div class="mobile-dashboard">
    <van-nav-bar title="设备概览" :border="false" />

    <!-- 设备在线状态 -->
    <div class="status-banner" :class="{ online: device.online }">
      <span class="status-dot" :class="device.online ? 'running' : 'stopped'"></span>
      {{ device.online ? '设备在线' : '设备离线' }}
    </div>

    <!-- 状态卡片网格 -->
    <div class="stats-grid">
      <div class="stat-card">
        <div class="stat-value" :class="cpuColor">{{ info.cputemp || 0 }}<span class="stat-unit">°C</span></div>
        <div class="stat-label">CPU 温度</div>
        <div class="stat-bar"><div class="stat-bar-fill" :style="{ width: cpuPercent + '%' }" :class="cpuColor"></div></div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ memUsed }}<span class="stat-unit">MB</span></div>
        <div class="stat-label">内存 / {{ info.memtotal || 0 }} MB</div>
        <div class="stat-bar"><div class="stat-bar-fill" :style="{ width: memPercent + '%' }"></div></div>
      </div>
      <div class="stat-card">
        <div class="stat-value">{{ storageUsedGB }}<span class="stat-unit">GB</span></div>
        <div class="stat-label">存储 / {{ storageTotalGB }} GB</div>
        <div class="stat-bar"><div class="stat-bar-fill" :style="{ width: storagePercent + '%' }"></div></div>
      </div>
      <div class="stat-card">
        <div class="stat-value uptime">{{ uptimeStr }}</div>
        <div class="stat-label">运行时间</div>
      </div>
    </div>

    <!-- 设备信息 -->
    <div class="info-section">
      <div class="section-title">设备信息</div>
      <van-cell-group inset>
        <van-cell title="SDK 版本" :value="info.version || '-'" />
        <van-cell title="设备型号" :value="info.model || '-'" />
        <van-cell title="设备 ID" :value="info.deviceId || '-'" />
        <van-cell title="IP 地址" :value="info.ip || '-'" />
        <van-cell title="MAC 地址" :value="info.hwaddr || '-'" />
        <van-cell title="网络速度" :value="(info.speed || '-') + ' Mbps'" />
      </van-cell-group>
    </div>

    <!-- 存储信息 -->
    <div class="info-section">
      <div class="section-title">存储信息</div>
      <van-cell-group inset>
        <van-cell title="硬盘型号" :value="info.mmcmodel || '-'" />
        <van-cell title="硬盘温度" :value="(info.mmctemp || '-') + '°C'" />
        <van-cell title="读取速度" :value="info.mmcread || '-'" />
        <van-cell title="写入速度" :value="info.mmcwrite || '-'" />
        <van-cell title="CPU 负载" :value="info.cpuload || '-'" />
      </van-cell-group>
    </div>
  </div>
</template>

<script setup>
import { computed, onMounted, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'

const device = useDeviceStore()
const info = computed(() => device.status || {})
const memUsed = computed(() => info.value.memuse || 0)
const storageUsedGB = computed(() => (Number(info.value.mmcuse) / 1024).toFixed(1))
const storageTotalGB = computed(() => (Number(info.value.mmctotal) / 1024).toFixed(1))

const cpuPercent = computed(() => Math.min(100, (Number(info.value.cputemp) / 80) * 100))
const memPercent = computed(() => {
  const total = Number(info.value.memtotal) || 1
  return Math.min(100, (Number(info.value.memuse) / total) * 100)
})
const storagePercent = computed(() => {
  const total = Number(info.value.mmctotal) || 1
  return Math.min(100, (Number(info.value.mmcuse) / total) * 100)
})

const cpuColor = computed(() => {
  const temp = Number(info.value.cputemp) || 0
  if (temp >= 80) return 'danger'
  if (temp >= 60) return 'warning'
  return ''
})

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

async function fetchDeviceInfo() {
  try {
    const resp = await device.request('device:info')
    device.status = resp.data?.data || resp.data
  } catch {}
}

onMounted(() => { if (device.online) fetchDeviceInfo() })
watch(() => device.online, (v) => { if (v) fetchDeviceInfo() })
</script>

<style scoped>
.mobile-dashboard {
  background: #0a0a0a;
  min-height: 100vh;
}

.status-banner {
  margin: 12px 16px;
  padding: 10px 16px;
  border-radius: 10px;
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  font-size: 14px;
  color: #999;
  display: flex;
  align-items: center;
}
.status-banner.online {
  border-color: rgba(103, 194, 58, 0.3);
}

.stats-grid {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 10px;
  padding: 0 16px;
  margin-bottom: 16px;
}

.stat-card {
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  padding: 14px;
}

.stat-value {
  font-size: 24px;
  font-weight: 700;
  color: #e0e0e0;
  line-height: 1.2;
}
.stat-value.danger { color: #f56c6c; }
.stat-value.warning { color: #e6a23c; }
.stat-value.uptime { font-size: 16px; }
.stat-unit {
  font-size: 12px;
  color: #999;
  font-weight: 400;
  margin-left: 2px;
}

.stat-label {
  font-size: 12px;
  color: #888;
  margin-top: 4px;
}

.stat-bar {
  height: 4px;
  background: #2a2a2a;
  border-radius: 2px;
  margin-top: 10px;
  overflow: hidden;
}
.stat-bar-fill {
  height: 100%;
  border-radius: 2px;
  background: #409eff;
  transition: width 0.3s;
}
.stat-bar-fill.warning { background: #e6a23c; }
.stat-bar-fill.danger { background: #f56c6c; }

.info-section {
  margin-bottom: 16px;
}

.section-title {
  font-size: 14px;
  font-weight: 600;
  color: #e0e0e0;
  padding: 8px 16px;
}
</style>
