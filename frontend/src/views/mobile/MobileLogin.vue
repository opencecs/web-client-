<template>
  <div class="mobile-login">
    <!-- 背景装饰 -->
    <div class="bg-decoration">
      <div class="bg-circle bg-circle-1"></div>
      <div class="bg-circle bg-circle-2"></div>
    </div>

    <div class="login-wrapper">
      <!-- Logo -->
      <div class="login-header">
        <img src="/favicon.ico" alt="魔云互联" class="logo-img" />
        <h1 class="login-title">魔云互联</h1>
        <p class="login-subtitle">云手机管理平台</p>
      </div>

      <!-- 登录表单 -->
      <div class="login-form">
        <van-cell-group inset>
          <van-field v-model="form.username" placeholder="请输入用户名" left-icon="contact-o"
            clearable autocomplete="username" />
          <van-field v-model="form.password" placeholder="请输入密码" left-icon="lock"
            :type="showPwd ? 'text' : 'password'" autocomplete="current-password"
            :right-icon="showPwd ? 'eye-o' : 'closed-eye'" @click-right-icon="showPwd = !showPwd"
            @keyup.enter="handleLogin" />
        </van-cell-group>

        <div class="remember-row">
          <van-checkbox v-model="rememberMe" shape="square" icon-size="16px">记住账号密码</van-checkbox>
        </div>

        <div style="padding: 0 16px">
          <van-button type="primary" block round size="large" :loading="loading"
            loading-text="登录中..." @click="handleLogin">
            登 录
          </van-button>
        </div>

        <div class="login-hint">
          默认管理账号：<span class="hint-code">myt</span> / <span class="hint-code">myt</span>
        </div>
      </div>

      <!-- 版本号 -->
      <div class="login-footer">v{{ panelVersion }}</div>
    </div>
  </div>
</template>

<script setup>
import { reactive, ref, onMounted } from 'vue'
import { useRouter } from 'vue-router'
import { useAuthStore } from '../../stores/auth.js'
import { useDeviceStore } from '../../stores/device.js'
import { showToast } from 'vant'

const router = useRouter()
const auth = useAuthStore()
const device = useDeviceStore()
const loading = ref(false)
const showPwd = ref(false)
const form = reactive({ username: '', password: '' })
const rememberMe = ref(false)
const panelVersion = ref('...')

onMounted(async () => {
  try {
    const resp = await fetch('/api/version')
    const data = await resp.json()
    panelVersion.value = data.version || 'dev'
  } catch {
    panelVersion.value = 'dev'
  }

  // 加载记住的凭据
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
    showToast('请输入用户名和密码')
    return
  }
  loading.value = true
  try {
    await auth.login(form.username, form.password)
    if (rememberMe.value) {
      localStorage.setItem('saved_username', form.username)
      localStorage.setItem('saved_password', btoa(form.password))
    } else {
      localStorage.removeItem('saved_username')
      localStorage.removeItem('saved_password')
    }
    device.connect()
    router.push('/m')
  } catch (e) {
    const msg = e.response?.data?.error
    if (msg === 'invalid credentials') {
      showToast('用户名或密码错误')
    } else if (msg === 'account disabled') {
      showToast('账号已禁用')
    } else if (msg === 'account expired') {
      showToast('账号已过期')
    } else {
      showToast('登录失败')
    }
  } finally {
    loading.value = false
  }
}
</script>

<style scoped>
.mobile-login {
  min-height: 100vh;
  display: flex;
  align-items: center;
  justify-content: center;
  background: #0a0a0a;
  position: relative;
  overflow: hidden;
  padding: 24px;
}

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
  width: 300px; height: 300px;
  background: #409eff;
  top: -80px; left: -60px;
}
.bg-circle-2 {
  width: 200px; height: 200px;
  background: #67c23a;
  bottom: -60px; right: -40px;
}

.login-wrapper {
  position: relative;
  z-index: 1;
  width: 100%;
  max-width: 400px;
}

.login-header {
  text-align: center;
  margin-bottom: 32px;
}
.logo-img {
  width: 56px;
  height: 56px;
  object-fit: contain;
}
.login-title {
  font-size: 24px;
  font-weight: 700;
  color: #e0e0e0;
  margin: 12px 0 6px;
  letter-spacing: 2px;
}
.login-subtitle {
  font-size: 13px;
  color: #666;
  margin: 0;
}

.login-form {
  background: #1a1a1a;
  border-radius: 16px;
  border: 1px solid #2a2a2a;
  padding: 24px 0 20px;
}

.remember-row {
  padding: 12px 24px 16px;
}

.login-hint {
  text-align: center;
  font-size: 12px;
  color: #666;
  margin-top: 16px;
  padding-top: 12px;
  border-top: 1px solid #2a2a2a;
}
.hint-code {
  background: #252525;
  color: #409eff;
  padding: 1px 6px;
  border-radius: 4px;
  font-family: monospace;
  font-weight: 600;
}

.login-footer {
  text-align: center;
  margin-top: 24px;
  font-size: 11px;
  color: #444;
}
</style>
