import { defineStore } from 'pinia'
import { ref, reactive, computed } from 'vue'
import api from '../api/index.js'

export const useAuthStore = defineStore('auth', () => {
  const token = ref(localStorage.getItem('token') || '')
  const username = ref(localStorage.getItem('username') || '')
  const role = ref(localStorage.getItem('role') || '')
  const sessionKey = ref(null) // AES 会话密钥（仅存内存，不持久化）

  // 用户权限（非 admin 用户从后端获取）
  const permissions = reactive({
    slots: [],
    container_start: false,
    container_restart: false,
    container_reset: false,
    container_delete: false,
    container_rename: false,
    container_copy: false,
    container_create: false,
    alias_manage: false,
    backup_manage: false,
    image_view: false,
    projection: false,
    terminal: false,
    network_bridge: false,
    vpc_manage: false,
    // 菜单权限
    menu_dashboard: false,
    menu_device: false,
    menu_android: false,
    menu_backup: false,
    menu_file: false,
    menu_users: false,
    switch_model: false
  })

  const isAdmin = computed(() => role.value === 'admin')
  const isLoggedIn = computed(() => !!token.value)

  // 检查功能权限
  function can(perm) {
    if (isAdmin.value) return true
    return !!permissions[perm]
  }

  // 检查坑位权限
  function canSlot(num) {
    if (isAdmin.value) return true
    return permissions.slots.includes(num)
  }

  // 更新权限数据
  function setPermissions(perms) {
    if (!perms) return
    Object.assign(permissions, perms)
    if (perms.slots) permissions.slots = [...perms.slots]
  }

  async function login(user, pass) {
    const { data } = await api.post('/auth/login', { username: user, password: pass })
    token.value = data.token
    username.value = data.username
    role.value = data.role
    localStorage.setItem('token', data.token)
    localStorage.setItem('username', data.username)
    localStorage.setItem('role', data.role)

    // 保存会话密钥（仅内存）
    if (data.session_key) {
      sessionKey.value = data.session_key
    }

    // 非 admin 用户获取权限
    if (data.role !== 'admin') {
      try {
        const me = await api.get('/auth/me')
        if (me.data.permissions) {
          setPermissions(me.data.permissions)
        }
      } catch {}
    }

    return data
  }

  function logout() {
    // 通知后端使 token 失效并断开 WS
    if (token.value) api.post('/auth/logout').catch(() => {})
    token.value = ''
    username.value = ''
    role.value = ''
    sessionKey.value = null
    localStorage.removeItem('token')
    localStorage.removeItem('username')
    localStorage.removeItem('role')
    // 重置权限
    Object.keys(permissions).forEach(k => {
      if (k === 'slots') permissions[k] = []
      else permissions[k] = false
    })
  }

  return { token, username, role, sessionKey, permissions, isAdmin, isLoggedIn, can, canSlot, setPermissions, login, logout }
})
