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
      <el-menu-item v-if="auth.can('menu_dashboard')" index="/">
        <el-icon><Monitor /></el-icon>
        <template #title>设备概览</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('menu_device')" index="/device">
        <el-icon><Cpu /></el-icon>
        <template #title>设备管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('menu_android')" index="/android">
        <el-icon><Cellphone /></el-icon>
        <template #title>安卓管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('menu_backup')" index="/backup">
        <el-icon><FolderOpened /></el-icon>
        <template #title>备份管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('menu_file')" index="/files">
        <el-icon><Document /></el-icon>
        <template #title>文件管理</template>
      </el-menu-item>
      <el-menu-item v-if="auth.can('menu_users')" index="/users">
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
import { Monitor, Cpu, Cellphone, FolderOpened, Document, User, DArrowLeft, DArrowRight } from '@element-plus/icons-vue'

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
  border-right: 1px solid var(--border-color);
  transition: width var(--transition-normal);
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
  padding: var(--space-md);
  cursor: pointer;
  white-space: nowrap;
  overflow: hidden;
  border-bottom: 1px solid var(--border-color);
}
.sidebar-header:hover .collapse-icon {
  color: var(--accent);
}
.sidebar-title {
  color: var(--accent);
  margin: 0;
  font-size: 18px;
}
.sidebar-ver {
  font-size: 11px;
  color: var(--text-tertiary);
  margin-left: 4px;
}
.collapse-icon {
  color: var(--text-tertiary);
  flex-shrink: 0;
  transition: color var(--transition-fast);
}
.collapse-icon-center {
  color: var(--text-secondary);
  margin: 0 auto;
  transition: color var(--transition-fast);
}
.sidebar-header:hover .collapse-icon-center {
  color: var(--accent);
}
</style>
