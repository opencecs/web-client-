import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { isMobile } from '../utils/isMobile.js'
import { mobileRoutes } from './mobileRoutes.js'
import api from '../api/index.js'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { guest: true } },
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue') },
  { path: '/device', name: 'DeviceManage', component: () => import('../views/DeviceManage.vue'), meta: { perm: 'menu_device' } },
  { path: '/android', name: 'AndroidManage', component: () => import('../views/AndroidManage.vue'), meta: { perm: 'menu_android' } },
  { path: '/backup', name: 'BackupManage', component: () => import('../views/BackupManage.vue'), meta: { perm: 'menu_backup' } },
  { path: '/files', name: 'FileManage', component: () => import('../views/FileManage.vue'), meta: { perm: 'menu_file' } },
  { path: '/users', name: 'UserManagement', component: () => import('../views/UserManagement.vue'), meta: { perm: 'menu_users' } },

  // 移动端路由
  ...mobileRoutes,
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const auth = useAuthStore()

  // 移动端自动重定向：桌面路由 → /m/ 路由
  if (isMobile && !to.path.startsWith('/m')) {
    const mobileMap = {
      '/': '/m',
      '/login': '/m/login',
      '/android': '/m/android',
      '/device': '/m/device',
      '/backup': '/m/backup',
      '/files': '/m/files',
      '/users': '/m/users',
    }
    const target = mobileMap[to.path]
    if (target) { next(target); return }
  }
  // 桌面端自动重定向：/m/ 路由 → 桌面路由
  if (!isMobile && to.path.startsWith('/m')) {
    const desktopMap = {
      '/m': '/',
      '/m/login': '/login',
      '/m/android': '/android',
      '/m/device': '/device',
      '/m/backup': '/backup',
      '/m/files': '/files',
      '/m/users': '/users',
    }
    const target = desktopMap[to.path]
    if (target) { next(target); return }
  }

  // 移动端登录页路径
  const loginPath = isMobile ? '/m/login' : '/login'
  const homePath = isMobile ? '/m' : '/'

  if (to.meta.guest) {
    if (auth.token) {
      next(homePath)
    } else {
      next()
    }
  } else if (!auth.token) {
    next(loginPath)
  } else if (to.meta.perm) {
    // 权限检查（admin 自动通过）
    if (!auth.can(to.meta.perm)) {
      // 跳到第一个有权限的菜单，避免死循环
      const menuMap = [
        { perm: 'menu_dashboard', path: '/' },
        { perm: 'menu_device', path: '/device' },
        { perm: 'menu_android', path: '/android' },
        { perm: 'menu_backup', path: '/backup' },
        { perm: 'menu_file', path: '/files' },
        { perm: 'menu_users', path: '/users' },
      ]
      const first = menuMap.find(m => auth.can(m.perm))
      next(first ? first.path : '/')
      return
    }
    next()
  } else {
    next()
  }
})

export default router
