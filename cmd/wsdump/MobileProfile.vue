<template>
  <div class="mobile-profile">
    <van-nav-bar title="我的" :border="false" />

    <!-- 用户信息 -->
    <div class="profile-card">
      <div class="avatar">{{ auth.username?.charAt(0)?.toUpperCase() }}</div>
      <div class="profile-info">
        <div class="profile-name">{{ auth.username }}</div>
        <van-tag :type="auth.role === 'admin' ? 'danger' : 'primary'" size="medium">
          {{ auth.role === 'admin' ? '管理员' : '用户' }}
        </van-tag>
      </div>
    </div>

    <!-- 设备状态 -->
    <van-cell-group inset>
      <van-cell title="设备状态" :value="device.online ? '在线' : '离线'">
        <template #icon>
          <span class="status-dot" :class="device.online ? 'running' : 'stopped'" style="margin-right: 8px"></span>
        </template>
      </van-cell>
      <van-cell title="容器数量" :value="device.containers.length + ' 个'" />
    </van-cell-group>

    <!-- 面板信息 -->
    <van-cell-group inset style="margin-top: 12px">
      <van-cell title="面板版本" :value="'v' + panelVersion" />
      <van-cell title="切换到桌面版" is-link @click="switchToDesktop" />
    </van-cell-group>

    <!-- 退出登录 -->
    <div style="padding: 32px 16px">
      <van-button type="danger" block round @click="handleLogout">退出登录</van-button>
    </div>
  </div>
</template>

<script setup>
import { ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showConfirmDialog } from 'vant'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()
const panelVersion = ref('...')

function switchToDesktop() {
  localStorage.setItem('force_platform', 'desktop')
  window.location.href = '/'
}

async function handleLogout() {
  try {
    await showConfirmDialog({ title: '确认', message: '确定退出登录？' })
    device.disconnect()
    auth.logout()
    router.push('/m/login')
  } catch {}
}

onMounted(async () => {
  try {
    const resp = await fetch('/api/version')
    const data = await resp.json()
    panelVersion.value = data.version || 'dev'
  } catch { panelVersion.value = 'dev' }
})
</script>

<style scoped>
.mobile-profile { background: #0a0a0a; min-height: 100vh; }

.profile-card {
  display: flex;
  align-items: center;
  padding: 20px 16px;
  gap: 16px;
}

.avatar {
  width: 56px;
  height: 56px;
  border-radius: 50%;
  background: #409eff;
  color: #fff;
  font-size: 24px;
  font-weight: 700;
  display: flex;
  align-items: center;
  justify-content: center;
}

.profile-info {
  display: flex;
  flex-direction: column;
  gap: 6px;
}

.profile-name {
  font-size: 18px;
  font-weight: 600;
  color: #e0e0e0;
}
</style>
