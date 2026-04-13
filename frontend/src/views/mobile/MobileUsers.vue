<template>
  <div class="mobile-users">
    <van-nav-bar title="用户管理" left-arrow @click-left="$router.back()" :border="false">
      <template #right>
        <van-icon name="plus" size="20" @click="showCreateDialog" />
      </template>
    </van-nav-bar>

    <van-pull-refresh v-model="refreshing" @refresh="onRefresh">
      <div class="user-list">
        <div v-for="u in users" :key="u.id" class="user-card" @click="editUser(u)">
          <div class="user-top">
            <div class="user-name">{{ u.username }}</div>
            <div class="user-tags">
              <van-tag :type="u.role === 'admin' ? 'danger' : 'primary'" size="medium">
                {{ u.role === 'admin' ? '管理员' : '用户' }}
              </van-tag>
              <van-tag v-if="!u.enabled" type="default" size="medium">已禁用</van-tag>
            </div>
          </div>
          <div class="user-meta">
            {{ u.expiresAt ? '到期: ' + new Date(u.expiresAt).toLocaleString() : '永不过期' }}
          </div>
          <!-- 操作按钮行 -->
          <div class="user-actions">
            <van-button size="small" type="primary" plain @click.stop="editUser(u)">编辑</van-button>
            <van-button v-if="u.role === 'user'" size="small" type="warning" plain
              @click.stop="editPermissions(u)">权限配置</van-button>
            <van-button v-if="u.username !== 'myt'" size="small" type="danger" plain
              @click.stop="deleteUser(u)">删除</van-button>
          </div>
        </div>
      </div>
      <van-empty v-if="!users.length" description="暂无用户" />
    </van-pull-refresh>

    <!-- 创建/编辑弹窗 -->
    <van-popup v-model:show="showForm" position="bottom" round safe-area-inset-bottom
      style="max-height: 85vh; overflow-y: auto">
      <div class="popup-content">
        <div class="popup-title">{{ editing ? '编辑用户' : '新增用户' }}</div>

        <van-field v-model="form.username" label="用户名" :readonly="!!editing"
          placeholder="请输入用户名" />
        <van-field v-model="form.password" label="密码" type="password"
          :placeholder="editing ? '留空不修改' : '请输入密码'" />
        <van-field v-if="form.password" v-model="form.confirmPassword" label="确认密码"
          type="password" placeholder="再次输入密码" />

        <template v-if="!isMytEdit">
          <div class="form-section">角色</div>
          <van-radio-group v-model="form.role" direction="horizontal" class="form-radio-row">
            <van-radio name="admin">管理员</van-radio>
            <van-radio name="user">普通用户</van-radio>
          </van-radio-group>
        </template>

        <template v-if="form.role === 'user'">
          <div class="form-section">到期方式</div>
          <van-radio-group v-model="expiryMode" direction="horizontal" class="form-radio-row">
            <van-radio name="never">永不过期</van-radio>
            <van-radio name="hours">按小时</van-radio>
          </van-radio-group>
          <van-field v-if="expiryMode === 'hours'" v-model="expiryHours" label="有效小时"
            type="digit" placeholder="如 24" />
        </template>

        <template v-if="editing && !isMytEdit">
          <div class="form-section">状态</div>
          <div class="form-switch-row">
            <span>启用账号</span>
            <van-switch v-model="form.enabled" size="20px" />
          </div>
        </template>

        <div class="popup-actions">
          <van-button plain block @click="showForm = false">取消</van-button>
          <van-button type="primary" block :loading="submitting" @click="submitForm">
            {{ editing ? '保存' : '创建' }}
          </van-button>
        </div>
      </div>
    </van-popup>

    <!-- 权限配置 -->
    <van-popup v-model:show="showPerms" position="bottom" round safe-area-inset-bottom
      style="max-height: 90vh; overflow-y: auto">
      <div class="popup-content">
        <div class="popup-title">权限配置 — {{ permUser?.username }}</div>

        <!-- 坑位权限 -->
        <div class="form-section" style="display: flex; align-items: center; gap: 8px">
          <span>坑位权限</span>
          <van-button size="mini" plain @click="selectAllSlots">全选</van-button>
          <van-button size="mini" plain @click="permForm.slots = []">清空</van-button>
        </div>
        <div class="slot-grid">
          <div v-for="i in maxSlots" :key="i"
            :class="['slot-item', { active: permForm.slots.includes(i) }]"
            @click="togglePermSlot(i)">{{ i }}</div>
        </div>

        <!-- 功能权限 -->
        <div class="form-section" style="display: flex; align-items: center; gap: 8px">
          <span>功能权限</span>
          <van-button size="mini" plain @click="selectAllPerms">全选</van-button>
          <van-button size="mini" plain @click="selectNoPerms">清空</van-button>
        </div>
        <div class="perm-list">
          <div v-for="p in permList" :key="p.key" class="perm-item">
            <span>{{ p.label }}</span>
            <van-switch v-model="permForm[p.key]" size="20px" />
          </div>
        </div>

        <div class="popup-actions">
          <van-button plain block @click="showPerms = false">取消</van-button>
          <van-button type="primary" block :loading="savingPerms" @click="savePerms">保存权限</van-button>
        </div>
      </div>
    </van-popup>
  </div>
</template>

<script setup>
import { ref, reactive, computed, onMounted, watch } from 'vue'
import { useDeviceStore } from '../../stores/device.js'
import { useAuthStore } from '../../stores/auth.js'
import { showToast, showConfirmDialog } from 'vant'

const device = useDeviceStore()
const auth = useAuthStore()

const users = ref([])
const refreshing = ref(false)
const showForm = ref(false)
const editing = ref(null)
const isMytEdit = ref(false)
const submitting = ref(false)
const expiryMode = ref('never')
const expiryHours = ref(1)
const form = reactive({ username: '', password: '', confirmPassword: '', role: 'user', enabled: true })

const maxSlots = computed(() => {
  const model = (device.status?.model || '').toLowerCase()
  return model.includes('p1') ? 24 : 12
})

const showPerms = ref(false)
const permUser = ref(null)
const savingPerms = ref(false)
const permForm = reactive({
  slots: [],
  container_start: false, container_restart: false, container_reset: false,
  container_delete: false, container_rename: false, container_copy: false,
  container_create: false, alias_manage: false, backup_manage: false,
  image_view: false, projection: false, terminal: false,
  network_bridge: false, vpc_manage: false,
})
const permKeys = [
  'container_start', 'container_restart', 'container_reset', 'container_delete',
  'container_rename', 'container_copy', 'container_create', 'alias_manage',
  'backup_manage', 'image_view', 'projection', 'terminal', 'network_bridge', 'vpc_manage'
]
const permList = [
  { key: 'container_start', label: '启动/停止' },
  { key: 'container_restart', label: '重启' },
  { key: 'container_reset', label: '重置' },
  { key: 'container_delete', label: '删除' },
  { key: 'container_create', label: '创建容器' },
  { key: 'container_rename', label: '重命名' },
  { key: 'container_copy', label: '复制' },
  { key: 'projection', label: '投屏' },
  { key: 'terminal', label: '终端' },
  { key: 'image_view', label: '镜像管理' },
  { key: 'network_bridge', label: '虚拟网卡' },
  { key: 'vpc_manage', label: 'VPC 管理' },
  { key: 'alias_manage', label: '别名管理' },
  { key: 'backup_manage', label: '备份管理' },
]

async function loadUsers() {
  try {
    const resp = await device.request('user:list')
    users.value = resp.data || []
  } catch {}
}

function onRefresh() { loadUsers(); setTimeout(() => refreshing.value = false, 800) }

function showCreateDialog() {
  editing.value = null; isMytEdit.value = false
  form.username = ''; form.password = ''; form.confirmPassword = ''; form.role = 'user'; form.enabled = true
  expiryMode.value = 'never'; expiryHours.value = 1
  showForm.value = true
}

function editUser(u) {
  editing.value = u; isMytEdit.value = u.username === 'myt'
  form.username = u.username; form.password = ''; form.confirmPassword = ''
  form.role = u.role; form.enabled = u.enabled
  if (u.expiresAt) { expiryMode.value = 'hours'; expiryHours.value = 1 }
  else expiryMode.value = 'never'
  showForm.value = true
}

function getExpiresAt() {
  if (expiryMode.value === 'never') return ''
  return new Date(Date.now() + Number(expiryHours.value) * 3600000).toISOString()
}

async function submitForm() {
  if (form.password && form.password !== form.confirmPassword) {
    showToast('两次密码不一致'); return
  }
  submitting.value = true
  try {
    if (editing.value) {
      const body = {}
      if (form.password) body.password = form.password
      if (!isMytEdit.value) {
        body.role = form.role
        body.enabled = form.enabled
        body.expiresAt = form.role === 'admin' ? '' : getExpiresAt()
      }
      await device.request('user:update', { id: editing.value.id, ...body })
      if (form.password && editing.value.username === auth.username) {
        showToast('密码已修改，请重新登录')
        auth.logout()
        window.location.href = '/m/login'
        return
      }
      showToast('保存成功')
    } else {
      if (!form.username || !form.password) { showToast('请填写完整'); return }
      if (form.password !== form.confirmPassword) { showToast('两次密码不一致'); return }
      await device.request('user:create', {
        username: form.username,
        password: form.password,
        role: form.role,
        expiresAt: form.role === 'admin' ? '' : getExpiresAt()
      })
      showToast('创建成功')
    }
    showForm.value = false
    loadUsers()
  } catch (e) {
    showToast(e.message || '操作失败')
  } finally { submitting.value = false }
}

async function deleteUser(u) {
  try {
    await showConfirmDialog({ title: '确认', message: `删除用户 ${u.username}？` })
    await device.request('user:delete', { id: u.id })
    showToast('已删除'); loadUsers()
  } catch {}
}

// ===== 权限管理 =====

async function editPermissions(u) {
  permUser.value = u
  try {
    const resp = await device.request('user:getPermissions', { id: u.id })
    const data = resp.data || {}
    permForm.slots = [...(data.slots || [])]
    for (const k of permKeys) permForm[k] = !!data[k]
  } catch {
    permForm.slots = []
    for (const k of permKeys) permForm[k] = false
  }
  showPerms.value = true
}

function togglePermSlot(n) {
  const idx = permForm.slots.indexOf(n)
  if (idx >= 0) permForm.slots.splice(idx, 1)
  else permForm.slots.push(n)
}

function selectAllSlots() {
  permForm.slots = []
  for (let i = 1; i <= maxSlots.value; i++) permForm.slots.push(i)
}

function selectAllPerms() { for (const k of permKeys) permForm[k] = true }
function selectNoPerms() { for (const k of permKeys) permForm[k] = false }

async function savePerms() {
  savingPerms.value = true
  try {
    const data = { id: permUser.value.id, slots: [...permForm.slots] }
    for (const k of permKeys) data[k] = permForm[k]
    await device.request('user:setPermissions', data)
    showToast('权限已保存'); showPerms.value = false
  } catch (e) { showToast(e.message || '保存失败') }
  finally { savingPerms.value = false }
}

onMounted(() => { if (device.online) loadUsers() })
watch(() => device.online, (v) => { if (v) loadUsers() })
</script>

<style scoped>
.mobile-users { background: #0a0a0a; min-height: 100vh; }

.user-list { padding: 8px 12px 24px; }

.user-card {
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  padding: 14px;
  margin-bottom: 8px;
}
.user-top {
  display: flex;
  justify-content: space-between;
  align-items: center;
  margin-bottom: 4px;
}
.user-name {
  font-size: 15px;
  font-weight: 600;
  color: #e0e0e0;
}
.user-tags {
  display: flex;
  gap: 4px;
}
.user-meta {
  font-size: 12px;
  color: #888;
  margin-bottom: 10px;
}
.user-actions {
  display: flex;
  gap: 8px;
}

/* 弹窗通用 */
.popup-content {
  padding: 20px 16px;
}
.popup-title {
  font-size: 17px;
  font-weight: 600;
  color: #e0e0e0;
  margin-bottom: 16px;
}
.popup-actions {
  display: flex;
  gap: 8px;
  margin-top: 20px;
}

/* 表单区块标题 */
.form-section {
  font-size: 13px;
  font-weight: 600;
  color: #bbb;
  margin: 16px 0 8px;
}
.form-radio-row {
  padding: 0 0 4px;
}
.form-switch-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 8px 0;
  font-size: 14px;
  color: #e0e0e0;
}

/* 坑位网格 */
.slot-grid {
  display: flex;
  flex-wrap: wrap;
  gap: 8px;
  margin-bottom: 8px;
}
.slot-item {
  width: 42px; height: 34px; line-height: 34px; text-align: center;
  border-radius: 8px; font-size: 14px; background: #141414; color: #ccc;
  border: 1px solid #2a2a2a; user-select: none;
}
.slot-item.active { background: #409eff; color: #fff; border-color: #409eff; }

/* 功能权限列表 */
.perm-list {
  background: #141414;
  border-radius: 12px;
  overflow: hidden;
}
.perm-item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 12px 16px;
  font-size: 14px;
  color: #e0e0e0;
  border-bottom: 1px solid #222;
}
.perm-item:last-child { border-bottom: none; }
</style>
