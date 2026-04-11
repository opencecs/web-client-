<template>
  <div class="app-container dark">
    <el-container v-if="auth.isLoggedIn" style="height: 100vh">
      <Sidebar />
      <el-container direction="vertical">
        <el-header style="background: #141414; border-bottom: 1px solid #2a2a2a; display: flex; align-items: center; justify-content: flex-end; height: 50px; padding: 0 20px">
          <el-dropdown trigger="click" @command="handleCommand">
            <span style="color: #bbb; cursor: pointer; display: flex; align-items: center; gap: 8px">
              <el-avatar :size="28" style="background: #409eff">{{ auth.username?.charAt(0)?.toUpperCase() }}</el-avatar>
              <span>{{ auth.username }}</span>
              <el-icon><ArrowDown /></el-icon>
            </span>
            <template #dropdown>
              <el-dropdown-menu>
                <el-dropdown-item disabled>
                  <el-tag size="small" :type="auth.role === 'admin' ? 'danger' : 'info'">{{ auth.role === 'admin' ? '管理员' : '用户' }}</el-tag>
                </el-dropdown-item>
                <el-dropdown-item divided command="logout">退出登录</el-dropdown-item>
              </el-dropdown-menu>
            </template>
          </el-dropdown>
        </el-header>
        <el-main style="padding: 0; background: #0a0a0a; overflow-y: auto">
          <router-view />
        </el-main>
      </el-container>
    </el-container>
    <router-view v-else />
  </div>
</template>

<script setup>
import { onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth.js'
import { useDeviceStore } from './stores/device.js'
import Sidebar from './components/Sidebar.vue'
import { ArrowDown } from '@element-plus/icons-vue'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()

onMounted(() => {
  if (auth.isLoggedIn) {
    device.connect()
  }
})

function handleCommand(cmd) {
  if (cmd === 'logout') {
    device.disconnect()
    auth.logout()
    router.push('/login')
  }
}
</script>

<style>
html, body {
  margin: 0;
  padding: 0;
  background: #0a0a0a;
  color: #e0e0e0;
}
html.dark {
  color-scheme: dark;
}
.app-container {
  height: 100vh;
}
</style>
