<template>
  <div class="login-container">
    <!-- 背景装饰 -->
    <div class="bg-decoration">
      <div class="bg-circle bg-circle-1"></div>
      <div class="bg-circle bg-circle-2"></div>
      <div class="bg-circle bg-circle-3"></div>
    </div>

    <div class="login-wrapper">
      <!-- Logo 区域 -->
      <div class="login-header">
        <div class="logo-icon">
          <img src="/favicon.ico" alt="魔云互联" class="logo-img" />
        </div>
        <h1 class="login-title">魔云互联</h1>
        <p class="login-subtitle">云手机管理平台</p>
      </div>

      <!-- 登录卡片 -->
      <el-card class="login-card" :body-style="{ padding: '32px 32px 24px' }">
        <el-form @submit.prevent="handleLogin" :model="form">
          <el-form-item>
            <el-input v-model="form.username" placeholder="请输入用户名" prefix-icon="User" size="large"
              :class="{ 'is-error': errorMsg }" @input="errorMsg = ''" />
          </el-form-item>
          <el-form-item>
            <el-input v-model="form.password" placeholder="请输入密码" type="password" prefix-icon="Lock"
              size="large" show-password :class="{ 'is-error': errorMsg }"
              @keyup.enter="handleLogin" @input="errorMsg = ''" />
          </el-form-item>
          <div v-if="errorMsg" class="login-error">{{ errorMsg }}</div>
          <el-form-item style="margin-bottom: 8px">
            <div style="display: flex; align-items: center; width: 100%">
              <el-checkbox v-model="rememberMe" label="记住账号密码" />
            </div>
          </el-form-item>
          <el-form-item style="margin-bottom: 12px">
            <el-button type="primary" size="large" class="login-btn" :loading="loading" @click="handleLogin">
              {{ loading ? '登录中...' : '登 录' }}
            </el-button>
          </el-form-item>
        </el-form>
        <div class="login-hint">
          <el-icon :size="14" style="margin-right: 4px; vertical-align: -2px"><InfoFilled /></el-icon>
          默认管理账号：<span class="hint-code">myt</span> / <span class="hint-code">myt</span>
        </div>
      </el-card>

      <!-- 启动参数说明 -->
      <div class="startup-info">
        <div class="info-toggle" @click="showInfo = !showInfo">
          {{ showInfo ? '收起' : '使用说明' }}
        </div>
        <div v-if="showInfo" class="info-content">
          <div class="info-title">适配机型</div>
          <div class="info-note">目前仅适配 <b>C1 / Q1 / R1S</b> 最新固件</div>
          <div class="info-title" style="margin-top: 10px">自定义端口</div>
          <code class="info-code">./myt-panel -port 9090</code>
          <div class="info-note" style="margin-top: 6px">默认端口 <b>8181</b>，TCP/UDP 共用同一端口</div>
        </div>
      </div>

      <!-- 底部版本 -->
      <div class="login-footer">v{{ panelVersion }}</div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../stores/auth.js'
import { useDeviceStore } from '../stores/device.js'
import { InfoFilled } from '@element-plus/icons-vue'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()
const loading = ref(false)
const form = reactive({ username: '', password: '' })
const rememberMe = ref(false)
const showInfo = ref(false)
const panelVersion = ref('...')
const errorMsg = ref('')

// 加载记住的用户名密码 + 获取版本号
onMounted(async () => {
  // 获取面板版本号（不需要登录）
  try {
    const resp = await fetch('/api/version')
    const data = await resp.json()
    panelVersion.value = data.version || 'dev'
  } catch {
    panelVersion.value = 'dev'
  }
  // 迁移旧格式（清除明文密码）
  const oldSaved = localStorage.getItem('saved_credentials')
  if (oldSaved) {
    try {
      const { username, password } = JSON.parse(oldSaved)
      if (username) localStorage.setItem('saved_username', username)
      if (password) localStorage.setItem('saved_password', btoa(password))
    } catch {}
    localStorage.removeItem('saved_credentials')
  }
  const savedUser = localStorage.getItem('saved_username')
  const savedPass = localStorage.getItem('saved_password')
  if (savedUser) {
    form.username = savedUser
    rememberMe.value = true
  }
  if (savedPass) {
    try { form.password = atob(savedPass) } catch {}
  }
})

async function handleLogin() {
  if (!form.username || !form.password) {
    errorMsg.value = '请输入用户名和密码'
    return
  }
  loading.value = true
  errorMsg.value = ''
  try {
    await auth.login(form.username, form.password)
    // 记住/清除账号密码
    if (rememberMe.value) {
      localStorage.setItem('saved_username', form.username)
      localStorage.setItem('saved_password', btoa(form.password))
    } else {
      localStorage.removeItem('saved_username')
      localStorage.removeItem('saved_password')
    }
    device.connect()
    router.push('/')
  } catch (e) {
    const msg = e.response?.data?.error
    if (msg === 'invalid credentials') {
      errorMsg.value = '用户名或密码错误'
    } else if (msg === 'account disabled') {
      errorMsg.value = '账号已禁用'
    } else if (msg === 'account expired') {
      errorMsg.value = '账号已过期'
    } else {
      errorMsg.value = '登录失败'
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.login-container {
  height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0a0a0a;
  position: relative;
  overflow: hidden;
}

/* 背景装饰圆 */
.bg-decoration {
  position: absolute;
  inset: 0;
  pointer-events: none;
  overflow: hidden;
}
.bg-circle {
  position: absolute;
  border-radius: 50%;
  filter: blur(80px);
  opacity: 0.08;
}
.bg-circle-1 {
  width: 400px; height: 400px;
  background: #409eff;
  top: -100px; left: -100px;
}
.bg-circle-2 {
  width: 300px; height: 300px;
  background: #67c23a;
  bottom: -80px; right: -80px;
}
.bg-circle-3 {
  width: 200px; height: 200px;
  background: #e6a23c;
  top: 50%; left: 60%;
}

.login-wrapper {
  position: relative;
  z-index: 1;
  display: flex;
  flex-direction: column;
  align-items: center;
}

/* Logo 头部 */
.login-header {
  text-align: center;
  margin-bottom: 28px;
}
.logo-icon {
  margin-bottom: 12px;
}
.logo-img {
  width: 64px;
  height: 64px;
  object-fit: contain;
}
.login-title {
  font-size: 28px;
  font-weight: 700;
  color: #e0e0e0;
  margin: 0 0 6px;
  letter-spacing: 2px;
}
.login-subtitle {
  font-size: 13px;
  color: #666;
  margin: 0;
  letter-spacing: 1px;
}

/* 卡片 */
.login-card {
  width: 380px;
  background: #1a1a1a;
  border: 1px solid #2a2a2a;
  border-radius: 12px;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.4);
}

.login-btn {
  width: 100%;
  font-size: 15px;
  letter-spacing: 4px;
  border-radius: 8px;
}

.login-error {
  color: #f56c6c;
  font-size: 13px;
  text-align: center;
  margin-bottom: 12px;
}

:deep(.is-error .el-input__wrapper) {
  box-shadow: 0 0 0 1px #f56c6c inset;
}

/* 默认账号提示 */
.login-hint {
  text-align: center;
  font-size: 12px;
  color: #666;
  padding: 10px 0 0;
  border-top: 1px solid #2a2a2a;
}
.hint-code {
  display: inline-block;
  background: #252525;
  color: #409eff;
  padding: 1px 8px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 13px;
  font-weight: 600;
}

/* 启动参数说明 */
.startup-info {
  width: 380px;
  margin-top: 16px;
}
.info-toggle {
  text-align: center;
  font-size: 12px;
  color: #555;
  cursor: pointer;
  padding: 4px 0;
}
.info-toggle:hover { color: #409eff; }
.info-content {
  margin-top: 8px;
  background: #141414;
  border: 1px solid #2a2a2a;
  border-radius: 8px;
  padding: 14px 16px;
  font-size: 12px;
  color: #999;
}
.info-title {
  color: #bbb;
  font-weight: 600;
  margin-bottom: 6px;
  font-size: 12px;
}
.info-code {
  display: block;
  background: #1a1a1a;
  color: #67c23a;
  padding: 6px 10px;
  border-radius: 4px;
  font-family: monospace;
  font-size: 12px;
  word-break: break-all;
}
.info-note {
  color: #888;
  font-size: 12px;
  line-height: 1.6;
}
.info-note b {
  color: #e6a23c;
}

/* 底部版本号 */
.login-footer {
  margin-top: 20px;
  font-size: 11px;
  color: #444;
}
</style>
