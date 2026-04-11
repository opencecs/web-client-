import { createRouter, createWebHistory } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import api from '../api/index.js'

const routes = [
  { path: '/login', name: 'Login', component: () => import('../views/Login.vue'), meta: { guest: true } },
  { path: '/', name: 'Dashboard', component: () => import('../views/Dashboard.vue') },
  { path: '/device', name: 'DeviceManage', component: () => import('../views/DeviceManage.vue'), meta: { admin: true } },
  { path: '/android', name: 'AndroidManage', component: () => import('../views/AndroidManage.vue') },

  { path: '/users', name: 'UserManagement', component: () => import('../views/UserManagement.vue'), meta: { admin: true } },
]

const router = createRouter({
  history: createWebHistory(),
  routes
})

router.beforeEach(async (to, from, next) => {
  const auth = useAuthStore()
  if (to.meta.guest) {
    // 已登录用户访问登录页，重定向到首页
    if (auth.token) {
      next('/')
    } else {
      next()
    }
  } else if (!auth.token) {
    next('/login')
  } else if (to.meta.admin) {
    // 每次进入管理员页面都从后端验证角色
    try {
      const { data } = await api.get('/auth/me')
      if (data.role !== 'admin') {
        next('/')
        return
      }
    } catch {
      next('/login')
      return
    }
    next()
  } else {
    next()
  }
})

export default router
