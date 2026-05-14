<template>
  <div class="app-container dark">
    <template v-if="auth.isLoggedIn">
      <Sidebar @collapse-change="onSidebarCollapse" />
      <div class="main-wrapper" :style="{ marginLeft: sidebarWidth + 'px' }">
        <header class="app-header">
          <div class="ws-status" :class="device.online ? 'online' : 'offline'">
            <span class="ws-dot"></span>
            <span>{{ device.online ? '已连接' : '未连接' }}</span>
          </div>
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
        </header>
        <main class="app-main">
          <router-view />
        </main>
      </div>
    </template>
    <router-view v-else />
    <!-- 手机 UA 被强制桌面模式时，显示回切按钮 -->
    <div v-if="showMobileSwitch" class="mobile-switch-hint" @click="switchToMobile">
      切换到手机版
    </div>
  </div>
</template>

<script setup>
import { onMounted, computed, onBeforeUnmount, ref } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from './stores/auth.js'
import { useDeviceStore } from './stores/device.js'
import Sidebar from './components/Sidebar.vue'
import { ArrowDown } from '@element-plus/icons-vue'
import { checkIsMobile } from './utils/isMobile.js'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()
const sidebarWidth = ref(64)

function onSidebarCollapse(collapsed) {
  sidebarWidth.value = collapsed ? 64 : 200
}

// 检测：手机 UA 但被 force_platform=desktop 强制到桌面版
const isMobileUA = /Android|iPhone|iPad|iPod|webOS|BlackBerry|IEMobile/i.test(navigator.userAgent)
const showMobileSwitch = computed(() => isMobileUA && localStorage.getItem('force_platform') === 'desktop')

function switchToMobile() {
  localStorage.removeItem('force_platform')
  window.location.href = '/m'
}

function handleCommand(cmd) {
  if (cmd === 'logout') {
    device.disconnect()
    auth.logout()
    router.push('/login')
  }
}

// 监听设备类型变化：PC↔手机切换时自动刷新
let lastMobile = false

function checkAndReload() {
  const nowMobile = checkIsMobile()
  if (nowMobile !== lastMobile) {
    window.location.reload()
  }
}

// resize：窗口大小变化（如DevTools切换设备模拟）
function onResize() {
  checkAndReload()
}

// storage：其他标签页修改 force_platform
function onStorage(e) {
  if (e.key === 'force_platform') {
    checkAndReload()
  }
}

let uaPollTimer = null

onMounted(() => {
  if (auth.isLoggedIn) {
    device.connect()
  }
  lastMobile = checkIsMobile()
  window.addEventListener('resize', onResize)
  window.addEventListener('storage', onStorage)
  // 轮询检测UA变化（DevTools切换设备模拟时UA会变但不触发事件）
  uaPollTimer = setInterval(checkAndReload, 1000)
})

onBeforeUnmount(() => {
  window.removeEventListener('resize', onResize)
  window.removeEventListener('storage', onStorage)
  if (uaPollTimer) clearInterval(uaPollTimer)
})
</script>

<style>
@import './theme.css';

html, body, #app {
  margin: 0;
  padding: 0;
  height: 100%;
  overflow: hidden;
  background: var(--bg-default);
  color: var(--text-primary);
  font-family: var(--font-family);
  -webkit-font-smoothing: antialiased;
  -moz-osx-font-smoothing: grayscale;
}
html.dark {
  color-scheme: dark;
}
.app-container {
  height: 100%;
  overflow: hidden;
}
.main-wrapper {
  height: 100%;
  display: flex;
  flex-direction: column;
  overflow: hidden;
  transition: margin-left var(--transition-normal);
}
.app-header {
  background: rgba(20, 20, 20, 0.85);
  backdrop-filter: blur(12px);
  -webkit-backdrop-filter: blur(12px);
  border-bottom: 1px solid var(--border-color);
  display: flex;
  align-items: center;
  justify-content: space-between;
  height: var(--header-height);
  padding: 0 var(--space-lg);
  position: sticky;
  top: 0;
  z-index: 100;
  flex-shrink: 0;
}
.app-main {
  flex: 1;
  padding: 0;
  background: var(--bg-default);
  overflow-y: auto;
  min-height: 0;
}
.app-main > * {
  animation: fadeIn 0.25s ease both;
}
@keyframes fadeIn {
  from { opacity: 0; }
  to { opacity: 1; }
}
.ws-status {
  display: flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
  color: var(--text-tertiary);
}
.ws-status.online { color: var(--success); }
.ws-status.offline { color: var(--danger); }
.ws-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--text-tertiary);
}
.ws-status.online .ws-dot { background: var(--success); }
.ws-status.offline .ws-dot { background: var(--danger); }
.mobile-switch-hint {
  position: fixed;
  bottom: 20px;
  right: 20px;
  background: var(--accent);
  color: #fff;
  padding: 10px 18px;
  border-radius: 24px;
  font-size: 14px;
  cursor: pointer;
  box-shadow: 0 4px 16px rgba(64, 158, 255, 0.4);
  z-index: 99999;
}
.mobile-switch-hint:active {
  transform: scale(0.95);
}

/* Element Plus 暗色主题补丁 */
.el-card {
  background: var(--bg-card) !important;
  border-color: var(--border-color) !important;
  border-radius: var(--radius-md) !important;
  color: var(--text-primary) !important;
}
.el-card__header {
  color: var(--text-primary) !important;
  border-bottom-color: var(--border-color) !important;
}
.el-tabs--border-card {
  background: var(--bg-card) !important;
  border-color: var(--border-color) !important;
  color: var(--text-primary) !important;
}
.el-tabs--border-card > .el-tabs__header {
  background: var(--bg-elevated) !important;
  border-bottom: 1px solid var(--border-color) !important;
}
.el-tabs--border-card > .el-tabs__item {
  color: var(--text-secondary) !important;
}
.el-tabs--border-card > .el-tabs__item.is-active {
  color: var(--accent) !important;
}

/* 表格 */
.el-table {
  --el-table-bg-color: var(--bg-card) !important;
  --el-table-tr-bg-color: var(--bg-card) !important;
  --el-table-header-bg-color: var(--bg-elevated) !important;
  --el-table-row-hover-bg-color: var(--bg-hover) !important;
  --el-table-border-color: var(--border-color) !important;
  --el-table-text-color: var(--text-primary) !important;
  --el-table-header-text-color: var(--text-secondary) !important;
  --el-table-current-row-bg-color: var(--bg-hover) !important;
  --el-table-stripe-bg-color: var(--bg-elevated) !important;
}
.el-table__empty-text {
  color: var(--text-tertiary) !important;
}
.el-table__body tr.el-table__row--striped td.el-table__cell {
  background: var(--bg-elevated) !important;
}

/* 弹窗 — 仅覆盖颜色，不干涉 Element Plus 自带布局 */
.el-dialog {
  --el-dialog-bg-color: var(--bg-card) !important;
  border-radius: var(--radius-lg) !important;
  color: var(--text-primary) !important;
}
.el-dialog__title {
  color: var(--text-primary) !important;
}
.el-dialog__body {
  color: var(--text-primary) !important;
}

/* Popconfirm / Popover */
.el-popconfirm__main {
  color: var(--text-primary) !important;
}
.el-popconfirm__action .el-button--small {
  color: var(--text-primary) !important;
}
.el-popover.el-popper {
  background: var(--bg-elevated) !important;
  border-color: var(--border-color) !important;
  color: var(--text-primary) !important;
}
.el-popover__title {
  color: var(--text-primary) !important;
}

/* MessageBox */
.el-message-box {
  --el-messagebox-title-color: var(--text-primary) !important;
  --el-messagebox-content-color: var(--text-secondary) !important;
  background-color: var(--bg-card) !important;
  border-color: var(--border-color) !important;
}

/* 表单 */
.el-form-item__label {
  color: var(--text-secondary) !important;
}
.el-input__inner {
  color: var(--text-primary) !important;
}
.el-input__wrapper {
  background-color: var(--bg-elevated) !important;
  box-shadow: 0 0 0 1px var(--border-light) inset !important;
}
.el-input__wrapper:hover {
  box-shadow: 0 0 0 1px var(--border-light) inset !important;
}
.el-input__wrapper.is-focus {
  box-shadow: 0 0 0 1px var(--accent) inset !important;
}
.el-textarea__inner {
  color: var(--text-primary) !important;
  background-color: var(--bg-elevated) !important;
}
.el-input-number {
  --el-input-number-unit-offset: 0 !important;
}
.el-input-number .el-input__wrapper {
  background-color: var(--bg-elevated) !important;
}

/* Select 下拉 */
.el-select-dropdown {
  background-color: var(--bg-elevated) !important;
  border-color: var(--border-color) !important;
}
.el-select-dropdown__item {
  color: var(--text-primary) !important;
}
.el-select-dropdown__item.hover,
.el-select-dropdown__item:hover {
  background-color: var(--bg-hover) !important;
}
.el-select-dropdown__item.selected {
  color: var(--accent) !important;
}

/* Checkbox / Radio */
.el-checkbox__label {
  color: var(--text-primary) !important;
}
.el-radio__label {
  color: var(--text-primary) !important;
}
.el-radio-button__inner {
  color: var(--text-primary) !important;
}

/* Descriptions */
.el-descriptions {
  --el-descriptions-table-bg: var(--bg-card) !important;
  --el-descriptions-item-bg: var(--bg-card) !important;
  --el-descriptions-item-label-bg: var(--bg-elevated) !important;
  --el-descriptions-border-color: var(--border-color) !important;
}
.el-descriptions__label {
  color: var(--text-secondary) !important;
}
.el-descriptions__content {
  color: var(--text-primary) !important;
}
.el-descriptions .el-descriptions__body {
  background-color: var(--bg-card) !important;
  color: var(--text-primary) !important;
}
.el-descriptions .is-bordered .el-descriptions__cell {
  background-color: var(--bg-card) !important;
}

/* Tag */
.el-tag {
  --el-tag-bg-color: transparent !important;
}

/* Empty */
.el-empty__description p {
  color: var(--text-tertiary) !important;
}

/* Alert */
.el-alert .el-alert__title {
  color: var(--text-primary) !important;
}
.el-alert .el-alert__description {
  color: var(--text-secondary) !important;
}
.el-alert--info {
  --el-alert-bg-color: var(--bg-elevated) !important;
  border-color: var(--border-light) !important;
}
.el-alert--error {
  --el-alert-bg-color: #2a1515 !important;
  border-color: #5c2a2a !important;
}
.el-alert--error .el-alert__title {
  color: #f56c6c !important;
}
.el-alert--error .el-alert__description {
  color: #f0a0a0 !important;
  font-size: 12px;
  word-break: break-all;
  white-space: pre-wrap;
}

/* Progress */
.el-progress__text {
  color: var(--text-primary) !important;
}

/* Switch */
.el-switch__label {
  color: var(--text-secondary) !important;
}
.el-switch__label.is-active {
  color: var(--accent) !important;
}

/* DatePicker */
.el-date-picker,
.el-date-editor {
  --el-datepicker-border-color: var(--border-color) !important;
}
.el-picker-panel {
  background-color: var(--bg-elevated) !important;
  color: var(--text-primary) !important;
}
.el-picker-panel__body {
  color: var(--text-primary) !important;
}

/* Divider */
.el-divider__text {
  background-color: var(--bg-card) !important;
  color: var(--text-secondary) !important;
}
.el-divider {
  border-color: var(--border-color) !important;
}

/* Dropdown */
.el-dropdown-menu {
  background-color: var(--bg-elevated) !important;
  border-color: var(--border-color) !important;
}
.el-dropdown-menu__item {
  color: var(--text-primary) !important;
}
.el-dropdown-menu__item:hover {
  background-color: var(--bg-hover) !important;
  color: var(--accent) !important;
}

/* Pagination (if used) */
.el-pagination {
  --el-pagination-bg-color: transparent !important;
  --el-pagination-text-color: var(--text-secondary) !important;
  --el-pagination-button-bg-color: var(--bg-elevated) !important;
}

/* Loading */
.el-loading-text {
  color: var(--text-secondary) !important;
}

/* Space 内文字 */
.el-space > * {
  color: var(--text-primary);
}
</style>
