import { defineStore } from 'pinia'
import { ref } from 'vue'
import { ElMessage } from 'element-plus'
import { useAuthStore } from './auth.js'

// AES-GCM 解密（配合后端 aesEncrypt）
async function aesDecrypt(sessionKeyBase64, encryptedBase64) {
  const keyBytes = Uint8Array.from(atob(sessionKeyBase64), c => c.charCodeAt(0))
  const key = await crypto.subtle.importKey('raw', keyBytes, 'AES-GCM', false, ['decrypt'])
  const data = Uint8Array.from(atob(encryptedBase64), c => c.charCodeAt(0))
  // 前 12 字节是 nonce，后面是密文
  const nonce = data.slice(0, 12)
  const ciphertext = data.slice(12)
  const plaintext = await crypto.subtle.decrypt({ name: 'AES-GCM', iv: nonce }, key, ciphertext)
  return new TextDecoder().decode(plaintext)
}

export const useDeviceStore = defineStore('device', () => {
  const status = ref(null)
  const online = ref(false)
  const containers = ref([])
  const containerAliases = ref({})
  const screenshots = ref({}) // { "坑位号": "data:image/jpeg;base64,..." }
  let ws = null
  let _reqId = 0
  let _kicked = false // 被踢标志，阻止自动重连
  let _reconnectDelay = 1000 // 重连延迟，指数退避

  // 暴露 ws 引用给需要监听原始事件的组件
  const _ws = { get value() { return ws } }

  // 等待响应的回调 map: id -> { resolve, reject, timer }
  const pendingRequests = new Map()

  // 外部事件监听器（跨重连保持）
  const eventListeners = new Set()

  function connect() {
    const auth = useAuthStore()
    if (!auth.token) return
    _kicked = false

    const proto = location.protocol === 'https:' ? 'wss:' : 'ws:'
    const url = `${proto}//${location.host}/ws?token=${auth.token}`

    ws = new WebSocket(url)

    ws.onmessage = (event) => {
      try {
        const msg = JSON.parse(event.data)

        // 处理加密消息
        if (msg.type === 'encrypted' && msg.data && auth.sessionKey) {
          aesDecrypt(auth.sessionKey, msg.data).then(plaintext => {
            try {
              const decrypted = JSON.parse(plaintext)
              handleMessage(decrypted)
            } catch {}
          }).catch(() => {})
          return
        }

        handleMessage(msg)
      } catch (e) {}
    }

    function handleMessage(msg) {
        // 事件推送
        if (msg.type === 'event') {
          if (msg.event === 'device:status') {
            status.value = msg.data?.data || msg.data
            online.value = msg.data?.online ?? true
          }
          if (msg.event === 'containers:list') {
            const raw = msg.data
            const list = raw?.list || raw?.data?.list || []
            containers.value = Array.isArray(list) ? list.sort((a, b) => a.indexNum - b.indexNum) : []
          }
          if (msg.event === 'aliases:list') {
            containerAliases.value = msg.data || {}
          }
          if (msg.event === 'user:kicked') {
            _kicked = true
            const reason = msg.data?.reason
            if (reason === 'password_changed') {
              alert('密码已被修改，请重新登录')
            } else if (reason === 'logout') {
              // 自己退出的，不弹提示
            } else {
              alert('账号已被强制下线')
            }
            auth.logout()
            window.location.href = '/login'
            return
          }
          if (msg.event === 'user:permissions') {
            auth.setPermissions(msg.data)
          }
          if (msg.event === 'screenshots') {
            screenshots.value = msg.data || {}
          }
          // token 刷新
          if (msg.event === 'token:refresh' && msg.data?.token) {
            auth.token = msg.data.token
            localStorage.setItem('token', msg.data.token)
            console.log('[WS] token 已刷新')
          }

          // 通知外部事件监听器
          for (const listener of eventListeners) {
            try { listener(msg) } catch {}
          }
        }

        // 请求响应
        if (msg.type === 'response' && msg.id) {
          const pending = pendingRequests.get(msg.id)
          if (pending) {
            clearTimeout(pending.timer)
            pendingRequests.delete(msg.id)
            if (msg.ok) {
              pending.resolve(msg)
            } else {
              pending.reject(new Error(msg.message || '操作失败'))
            }
          }
        }
    }

    ws.onopen = () => {
      _reconnectDelay = 1000 // 连接成功，重置退避
    }

    ws.onclose = () => {
      online.value = false
      if (_kicked) return // 被踢后不重连
      const delay = _reconnectDelay
      _reconnectDelay = Math.min(_reconnectDelay * 2, 30000) // 指数退避，最大 30 秒
      setTimeout(() => connect(), delay)
    }

    ws.onerror = () => ws.close()
  }

  function disconnect() {
    // 清理所有待处理请求
    for (const [id, pending] of pendingRequests) {
      clearTimeout(pending.timer)
      pending.reject(new Error('连接已断开'))
    }
    pendingRequests.clear()
    if (ws) {
      ws.close()
      ws = null
    }
  }

  /**
   * 通过 WS 发送请求并等待响应
   * @param {string} action - 操作类型，如 'container:start'
   * @param {object} data - 请求数据
   * @param {number} timeout - 超时毫秒数，默认 30000
   * @returns {Promise}
   */
  function request(action, data = {}, timeout = 30000) {
    return new Promise((resolve, reject) => {
      if (!ws || ws.readyState !== WebSocket.OPEN) {
        reject(new Error('WebSocket 未连接'))
        return
      }

      const id = String(++_reqId)
      const timer = setTimeout(() => {
        pendingRequests.delete(id)
        reject(new Error('请求超时'))
      }, timeout)

      pendingRequests.set(id, { resolve, reject, timer })

      ws.send(JSON.stringify({ action, id, data }))
    })
  }

  // 请求刷新容器列表（不等待响应，服务端会广播新数据）
  function refreshContainers() {
    if (ws && ws.readyState === WebSocket.OPEN) {
      ws.send(JSON.stringify({ action: 'containers:refresh', id: String(++_reqId) }))
    }
  }

  // ========== 容器别名（公共方法） ==========

  // 获取容器显示名称：有别名返回别名，否则返回原始名
  function displayName(name) {
    if (!name) return ''
    return containerAliases.value[name] || name
  }

  // 设置别名
  async function setAlias(name, alias) {
    await request('alias:set', { name, alias })
    containerAliases.value = { ...containerAliases.value, [name]: alias }
  }

  // 删除别名
  async function removeAlias(name) {
    await request('alias:delete', { name })
    const copy = { ...containerAliases.value }
    delete copy[name]
    containerAliases.value = copy
  }

  // 注册/注销事件监听器（跨 WS 重连保持有效）
  function onEvent(listener) {
    eventListeners.add(listener)
  }
  function offEvent(listener) {
    eventListeners.delete(listener)
  }

  // 请求投屏专用 token
  async function requestProjectionToken(containerName) {
    const auth = useAuthStore()
    const resp = await request('projection:token', { container_name: containerName })
    const data = resp.data
    // 加密响应需要解密
    if (data?.encrypted && data?.data) {
      if (auth.sessionKey) {
        try {
          const plaintext = await aesDecrypt(auth.sessionKey, data.data)
          const parsed = JSON.parse(plaintext)
          return { token: parsed.token, udpPort: parsed.udpPort || '' }
        } catch {
          console.warn('[投屏] sessionKey 解密失败')
        }
      }
      // sessionKey 不存在或解密失败，无法获取投屏 token
      console.warn('[投屏] 无 sessionKey，无法解密投屏 token')
      return null
    }
    return { token: data?.token, udpPort: data?.udpPort || '' }
  }

  return {
    status, online, containers, containerAliases, screenshots,
    connect, disconnect, request, refreshContainers, requestProjectionToken,
    displayName, setAlias, removeAlias, onEvent, offEvent,
    get _ws() { return ws },
  }
})
