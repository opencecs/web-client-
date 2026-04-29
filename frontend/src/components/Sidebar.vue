<template>
  <el-aside :width="collapsed ? '64px' : '200px'" class="sidebar">
    <div class="sidebar-header" @click="collapsed = !collapsed">
      <template v-if="!collapsed">
        <h2 class="sidebar-title">魔云互联<span class="sidebar-ver">v{{ panelVersion }}</span></h2>
        <el-icon class="collapse-icon" :size="18"><DArrowLeft /></el-icon>
      </template>
      <template v-else>
        <el-icon class="collapse-icon-center" :size="20"><DArrowRight /></el-icon>
      </template>
    </div>

    <el-menu :default-active="route.path" router background-color="#141414" text-color="#bbb" active-text-color="#409eff" :collapse="collapsed">
      <el-menu-item index="/">
        <el-icon><Monitor /></el-icon>
        <template #title>设备概览</template>
      </el-menu-item>
      <el-menu-item v-if="auth.isAdmin" index="/device">
        <el-icon><Cpu /></el-icon>
        <template #title>设备管理</template>
      </el-menu-item>
      <el-menu-item index="/android">
        <el-icon><Cellphone /></el-icon>
        <template #title>安卓管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('backup_manage')" index="/backup">
        <el-icon><FolderOpened /></el-icon>
        <template #title>备份管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.isAdmin" index="/users">
        <el-icon><User /></el-icon>
        <template #title>用户管理</template>
      </el-menu-item>
    </el-menu>
  </el-aside>
</template>

<script setup>
import { ref, onMounted, watch } from 'vue'
import { useRoute } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { useDeviceStore } from '../stores/device.js'
import { Monitor, Cpu, Cellphone, FolderOpened, User, DArrowLeft, DArrowRight } from '@element-plus/icons-vue'

const emit = defineEmits(['collapse-change'])
const route = useRoute()
const auth = useAuthStore()
const device = useDeviceStore()
const collapsed = ref(true)
const panelVersion = ref('...')

watch(collapsed, (val) => {
  emit('collapse-change', val)
}, { immediate: true })

onMounted(() => {
  const fetchVersion = async () => {
    try {
      const resp = await device.request('panel:version')
      panelVersion.value = resp.data?.version || 'dev'
    } catch {
      // WebSocket 可能还未就绪，1秒后重试
      setTimeout(fetchVersion, 1000)
    }
  }
  fetchVersion()
})
</script>

<style scoped>
.sidebar {
  background: #141414;
  border-right: 1px solid #2a2a2a;
  transition: width 0.2s;
  overflow: hidden;
  position: fixed;
  top: 0;
  left: 0;
  bottom: 0;
  z-index: 200;
}
.sidebar-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 16px;
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  border-bottom: 1px solid #2a2a2a;
}
.sidebar-header:hover .collapse-icon {
  color: #409eff;
}
.sidebar-title {
  color: #409eff;
  margin: 0;
  font-size: 18px;
}
.sidebar-ver {
  font-size: 11px;
  color: #666;
  margin-left: 4px;
}
.collapse-icon {
  color: #666;
  flex-shrink: 0;
  transition: color 0.2s;
}
.collapse-icon-center {
  color: #888;
  margin: 0 auto;
  transition: color 0.2s;
}
.sidebar-header:hover .collapse-icon-center {
  color: #409eff;
}
</style>
