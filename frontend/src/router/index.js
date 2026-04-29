import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { isMobile } from '../utils/isMobile.js'
import { mobileRoutes } from './mobileRoutes.js'
import api from '../api/index.js'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { guest: true } },
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue') },
  { path: '/device', name: 'DeviceManage', component: () => import('../views/DeviceManage.vue'), meta: { admin: true } },
  { path: '/android', name: 'AndroidManage', component: () => import('../views/AndroidManage.vue') },
  { path: '/backup', name: 'BackupManage', component: () => import('../views/BackupManage.vue'), meta: { perm: 'backup_manage' } },

  { path: '/users', name: 'UserManagement', component: () => import('../views/UserManagement.vue'), meta: { admin: true } },

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
  } else if (to.meta.admin) {
    // 每次进入管理员页面都从后端验证角色
    try {
      const { data } = await api.get('/auth/me')
      if (data.role !== 'admin') {
        next(homePath)
        return
      }
    } catch {
      next(loginPath)
      return
    }
    next()
  } else if (to.meta.perm) {
    // 功能权限检查
    if (!auth.can(to.meta.perm)) {
      next(homePath)
      return
    }
    next()
  } else {
    next()
  }
})

export default router
