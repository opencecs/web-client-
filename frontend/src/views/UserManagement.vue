<template>
  <div style="padding: var(--space-lg)">
    <div style="display: flex; align-items: center; justify-content: space-between; margin-bottom: var(--space-md)">
      <h2 style="margin: 0; color: var(--text-primary); font-size: 18px; font-weight: 600">用户管理</h2>
      <el-button type="primary" @click="showCreateDialog">新增用户</el-button>
    </div>

    <el-table :data="users" style="width: 100%" stripe>
      <el-table-column prop="id" label="ID" width="60" />
      <el-table-column prop="username" label="用户名" width="150" />
      <el-table-column prop="role" label="角色" width="100">
        <template #default="{ row }">
          <el-tag :type="row.role === 'admin' ? 'danger' : 'info'" size="small">{{ row.role === 'admin' ? '管理员' : '用户' }}</el-tag>
        </template>
      </el-table-column>
      <el-table-column label="到期时间" width="200">
        <template #default="{ row }">
          {{ row.expiresAt ? new Date(row.expiresAt).toLocaleString() : '永不过期' }}
        </template>
      </el-table-column>
      <el-table-column label="状态" width="100">
        <template #default="{ row }">
          <el-switch :model-value="row.enabled" @change="toggleEnabled(row)" :disabled="row.username === 'myt'" />
        </template>
      </el-table-column>
      <el-table-column label="操作">
        <template #default="{ row }">
          <el-button size="small" @click="editUser(row)">编辑</el-button>
          <el-button v-if="row.role === 'user'" size="small" type="warning" @click="editPermissions(row)">权限</el-button>
          <el-popconfirm title="确认删除？" @confirm="deleteUser(row.id)" v-if="row.username !== 'myt'">
            <template #reference>
              <el-button size="small" type="danger">删除</el-button>
            </template>
          </el-popconfirm>
        </template>
      </el-table-column>
    </el-table>

    <!-- 新增/编辑对话框 -->
    <el-dialog v-model="showCreate" :title="editingUser ? '编辑用户' : '新增用户'" width="450px">
      <el-form :model="form" label-width="80px">
        <el-form-item label="用户名">
          <el-input v-model="form.username" :disabled="!!editingUser" />
        </el-form-item>
        <el-form-item label="密码">
          <el-input v-model="form.password" type="password" show-password
            :placeholder="editingUser ? '留空不修改' : '请输入密码'" />
        </el-form-item>
        <el-form-item v-if="form.password" label="确认密码">
          <el-input v-model="form.confirmPassword" type="password" show-password
            placeholder="请再次输入密码" />
        </el-form-item>
        <el-form-item label="角色" v-if="!isMytUser">
          <el-select v-model="form.role" style="width: 100%" @change="onRoleChange">
            <el-option label="管理员" value="admin" />
            <el-option label="普通用户" value="user" />
          </el-select>
        </el-form-item>
        <el-form-item label="到期方式" v-if="!isMytUser && form.role !== 'admin'">
          <el-radio-group v-model="expiryMode">
            <el-radio value="never">永不过期</el-radio>
            <el-radio value="hours">按小时</el-radio>
            <el-radio value="date">按日期</el-radio>
          </el-radio-group>
        </el-form-item>
        <el-form-item v-if="!isMytUser && form.role !== 'admin' && expiryMode === 'hours'" label="小时数">
          <el-input-number v-model="expiryHours" :min="1" :max="8760" />
        </el-form-item>
        <el-form-item v-if="!isMytUser && form.role !== 'admin' && expiryMode === 'date'" label="到期日期">
          <el-date-picker v-model="expiryDate" type="datetime" style="width: 100%" />
        </el-form-item>
      </el-form>
      <template #footer>
        <el-button @click="showCreate = false">取消</el-button>
        <el-button type="primary" @click="submitForm">{{ editingUser ? '保存' : '创建' }}</el-button>
      </template>
    </el-dialog>

    <!-- 权限配置对话框 -->
    <el-dialog v-model="permVisible" :title="`权限配置 — ${permUser?.username}`" width="640px">
      <!-- 坑位权限 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">坑位权限</span>
          <span class="perm-section-desc">允许操作的云机坑位</span>
          <div class="perm-section-actions">
            <el-button size="small" text @click="selectAllSlots">全选</el-button>
            <el-button size="small" text @click="selectNoSlots">清空</el-button>
          </div>
        </div>
        <div class="slot-grid">
          <el-checkbox
            v-for="i in maxSlots"
            :key="i"
            :model-value="permForm.slots.includes(i)"
            @change="toggleSlot(i)"
            :label="'坑位 ' + i"
            border
            size="small"
          />
        </div>
      </div>

      <el-divider style="margin: 12px 0" />

      <!-- 容器管理 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">容器管理</span>
          <span class="perm-section-desc">云机的启停、重置、删除等操作</span>
        </div>
        <div class="perm-grid">
          <el-checkbox v-model="permForm.container_start" label="启动 / 停止" />
          <el-checkbox v-model="permForm.container_restart" label="重启" />
          <el-checkbox v-model="permForm.container_reset" label="重置" />
          <el-checkbox v-model="permForm.container_delete" label="删除" />
          <el-checkbox v-model="permForm.container_rename" label="重命名" />
          <el-checkbox v-model="permForm.container_copy" label="复制" />
          <el-checkbox v-model="permForm.container_create" label="创建容器" />
          <el-checkbox v-model="permForm.switch_model" label="切换机型" />
        </div>
      </div>

      <!-- 远程控制 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">远程控制</span>
          <span class="perm-section-desc">投屏、终端等远程访问功能</span>
        </div>
        <div class="perm-grid">
          <el-checkbox v-model="permForm.projection" label="投屏" />
          <el-checkbox v-model="permForm.terminal" label="终端" />
        </div>
      </div>

      <!-- 数据管理 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">数据管理</span>
          <span class="perm-section-desc">备份、镜像、别名等数据操作</span>
        </div>
        <div class="perm-grid">
          <el-checkbox v-model="permForm.backup_manage" label="备份管理" />
          <el-checkbox v-model="permForm.image_view" label="镜像管理" />
          <el-checkbox v-model="permForm.alias_manage" label="别名管理" />
        </div>
      </div>

      <!-- 网络 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">网络管理</span>
          <span class="perm-section-desc">虚拟网卡、VPC 等网络配置</span>
        </div>
        <div class="perm-grid">
          <el-checkbox v-model="permForm.network_bridge" label="虚拟网卡" />
          <el-checkbox v-model="permForm.vpc_manage" label="VPC 管理" />
        </div>
      </div>

      <!-- 菜单权限 -->
      <div class="perm-section">
        <div class="perm-section-header">
          <span class="perm-section-title">菜单权限</span>
          <span class="perm-section-desc">控制用户可见的导航菜单</span>
        </div>
        <div class="perm-grid">
          <el-checkbox v-model="permForm.menu_dashboard" label="设备概览" />
          <el-checkbox v-model="permForm.menu_device" label="设备管理" />
          <el-checkbox v-model="permForm.menu_android" label="安卓管理" />
          <el-checkbox v-model="permForm.menu_backup" label="备份管理" />
          <el-checkbox v-model="permForm.menu_file" label="文件管理" />
          <el-checkbox v-model="permForm.menu_users" label="用户管理" />
        </div>
      </div>

      <template #footer>
        <el-button @click="permVisible = false">取消</el-button>
        <el-button type="primary" :loading="permSaving" @click="savePermissions">保存</el-button>
      </template>
    </el-dialog>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted } from 'vue'
import { useDeviceStore } from '../stores/device.js'
import { useAuthStore } from '../stores/auth.js'
import { ElMessage, ElMessageBox } from 'element-plus'

const device = useDeviceStore()
const auth = useAuthStore()

const users = ref([])
const showCreate = ref(false)
const editingUser = ref(null)
const expiryMode = ref('hours')
const expiryHours = ref(1)
const expiryDate = ref(null)
const form = reactive({ username: '', password: '', confirmPassword: '', role: 'user' })

const isMytUser = computed(() => editingUser.value?.username === 'myt')

// 坑位数量
const maxSlots = computed(() => {
  const model = (device.status?.model || '').toLowerCase()
  return model.includes('p1') ? 24 : 12
})

function showCreateDialog() {
  editingUser.value = null
  form.username = ''
  form.password = ''
  form.confirmPassword = ''
  form.role = 'user'
  expiryMode.value = 'hours'
  expiryHours.value = 1
  expiryDate.value = null
  showCreate.value = true
}

function onRoleChange(val) {
  if (val === 'admin') {
    expiryMode.value = 'never'
  }
}

async function loadUsers() {
  const resp = await device.request('user:list')
  users.value = resp.data
}

function editUser(user) {
  editingUser.value = user
  form.username = user.username
  form.password = ''
  form.confirmPassword = ''
  form.role = user.role
  if (user.expiresAt) {
    expiryMode.value = 'date'
    expiryDate.value = new Date(user.expiresAt)
  } else {
    expiryMode.value = 'never'
  }
  showCreate.value = true
}

function getExpiresAt() {
  if (expiryMode.value === 'never') return ''
  if (expiryMode.value === 'hours') {
    return new Date(Date.now() + expiryHours.value * 3600000).toISOString()
  }
  return expiryDate.value ? new Date(expiryDate.value).toISOString() : ''
}

async function submitForm() {
  // 密码一致性校验
  if (form.password && form.password !== form.confirmPassword) {
    ElMessage.warning('两次输入的密码不一致')
    return
  }
  try {
    if (editingUser.value) {
      const body = {}
      if (form.password) body.password = form.password
      if (!isMytUser.value) {
        body.role = form.role
        body.expiresAt = form.role === 'admin' ? '' : getExpiresAt()
      }
      await device.request('user:update', { id: editingUser.value.id, ...body })
      if (form.password) {
        // 如果改的是当前登录用户自己的密码，直接跳登录页
        if (editingUser.value.username === auth.username) {
          alert('密码修改成功，请重新登录')
          auth.logout()
          window.location.href = '/login'
          return
        }
        ElMessage.success('密码修改成功，该用户已被强制下线')
      } else {
        ElMessage.success('更新成功')
      }
    } else {
      if (!form.username) { ElMessage.warning('请输入用户名'); return }
      if (!form.password) { ElMessage.warning('请输入密码'); return }
      if (form.password !== form.confirmPassword) { ElMessage.warning('两次输入的密码不一致'); return }
      await device.request('user:create', {
        username: form.username,
        password: form.password,
        role: form.role,
        expiresAt: form.role === 'admin' ? '' : (getExpiresAt() || '')
      })
      ElMessage.success('创建成功')
    }
    showCreate.value = false
    editingUser.value = null
    loadUsers()
  } catch (e) {
    ElMessage.error(e.message || '操作失败')
  }
}

async function toggleEnabled(user) {
  await device.request('user:update', { id: user.id, enabled: !user.enabled })
  loadUsers()
}

async function deleteUser(id) {
  await device.request('user:delete', { id })
  ElMessage.success('已删除')
  loadUsers()
}

// ===== 权限管理 =====

const permVisible = ref(false)
const permUser = ref(null)
const permSaving = ref(false)
const permForm = reactive({
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

const permKeys = [
  'container_start', 'container_restart', 'container_reset', 'container_delete',
  'container_rename', 'container_copy', 'container_create', 'alias_manage',
  'backup_manage', 'image_view', 'projection', 'terminal', 'network_bridge', 'vpc_manage',
  'menu_dashboard', 'menu_device', 'menu_android', 'menu_backup', 'menu_file', 'menu_users',
  'switch_model'
]

async function editPermissions(user) {
  permUser.value = user
  // 加载权限
  try {
    const resp = await device.request('user:getPermissions', { id: user.id })
    const data = resp.data || {}
    permForm.slots = [...(data.slots || [])]
    for (const k of permKeys) {
      permForm[k] = !!data[k]
    }
  } catch {
    // 没有权限记录，全部为 false
    permForm.slots = []
    for (const k of permKeys) permForm[k] = false
  }
  permVisible.value = true
}

function toggleSlot(num) {
  const idx = permForm.slots.indexOf(num)
  if (idx >= 0) permForm.slots.splice(idx, 1)
  else permForm.slots.push(num)
}

function selectAllSlots() {
  permForm.slots = []
  for (let i = 1; i <= maxSlots.value; i++) permForm.slots.push(i)
}

function selectNoSlots() {
  permForm.slots = []
}

function selectAllPerms() {
  for (const k of permKeys) permForm[k] = true
}

function selectNoPerms() {
  for (const k of permKeys) permForm[k] = false
}

async function savePermissions() {
  // 检查：勾了功能但没选坑位
  const hasAnyPerm = permKeys.some(k => permForm[k])
  if (hasAnyPerm && permForm.slots.length === 0) {
    try {
      await ElMessageBox.confirm(
        '未选择任何坑位，用户将无法操作任何容器。确定保存？',
        '提示', { type: 'warning' }
      )
    } catch { return }
  }
  permSaving.value = true
  try {
    const data = { id: permUser.value.id, slots: [...permForm.slots] }
    for (const k of permKeys) data[k] = permForm[k]
    await device.request('user:setPermissions', data)
    ElMessage.success('权限保存成功')
    permVisible.value = false
  } catch (e) {
    ElMessage.error(e.message || '保存失败')
  } finally {
    permSaving.value = false
  }
}

onMounted(loadUsers)
</script>

<style scoped>
.perm-section {
  margin-bottom: 16px;
}
.perm-section-header {
  display: flex;
  align-items: baseline;
  gap: 8px;
  margin-bottom: 8px;
}
.perm-section-title {
  font-size: 14px;
  font-weight: 600;
  color: var(--text-primary);
}
.perm-section-desc {
  font-size: 12px;
  color: var(--text-tertiary);
}
.perm-section-actions {
  margin-left: auto;
}
.slot-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
}
.perm-grid {
  display: grid;
  grid-template-columns: repeat(3, 1fr);
  gap: 8px;
}
</style>
