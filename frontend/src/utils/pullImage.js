// 拉取镜像 WS 流式模块
// 通过 WS 调用 sdk:pullImage，监听 task:progress 事件获取实时进度

import { useDeviceStore } from '../stores/device.js'

/**
 * 拉取镜像（WS 流式进度）
 * @param {string} imageUrl - 镜像地址
 * @param {object} callbacks - 回调函数
 * @param {function} callbacks.onProgress - 下载进度回调 ({ current, total, percent, text })
 * @param {function} callbacks.onExtracting - 解压阶段回调 (statusText)
 * @param {function} callbacks.onComplete - 完成回调 (statusText)
 * @param {function} callbacks.onError - 错误回调 (errorMessage)
 * @param {AbortSignal} [signal] - 用于取消请求的 AbortSignal
 * @returns {Promise<boolean>} 是否成功
 */
export async function pullImage(imageUrl, callbacks = {}, signal) {
  const { onProgress, onExtracting, onComplete, onError } = callbacks
  const device = useDeviceStore()

  return new Promise((resolve) => {
    let hasError = false
    let resolved = false

    function done(ok) {
      if (resolved) return
      resolved = true
      device.offEvent(handler)
      resolve(ok)
    }

    // 通过 device store 的事件机制监听（跨 WS 重连有效）
    const handler = (msg) => {
      if (msg.event !== 'task:progress') return
      if (msg.data?.action !== 'pullImage') return
      // 按 imageUrl 过滤，支持多镜像并发拉取
      if (msg.data.imageUrl && msg.data.imageUrl !== imageUrl) return

      if (msg.data.done) {
        if (!hasError) {
          onComplete?.(msg.data.exists ? '镜像已存在，无需下载' : '拉取完成')
        }
        done(!hasError)
        return
      }

      // 解析 chunk 中的 SSE 数据
      const chunk = msg.data.chunk || ''
      const lines = chunk.split('\n')
      for (const line of lines) {
        let eventData = null
        if (line.startsWith('data: ')) {
          try { eventData = JSON.parse(line.slice(6)) } catch {}
        } else if (line.trim().startsWith('{')) {
          try { eventData = JSON.parse(line.trim()) } catch {}
        }

        if (eventData) {
          if (eventData.error) {
            hasError = true
            onError?.(eventData.error)
            continue
          }
          // 兜底处理"镜像已存在"
          if (eventData.status === 'No operation' || eventData.message === 'Image already exists') {
            onComplete?.('镜像已存在，无需下载')
            continue
          }
          handleEvent(eventData, callbacks)
        }
      }
    }

    device.onEvent(handler)

    // 取消信号
    if (signal) {
      signal.addEventListener('abort', () => {
        onError?.('已取消')
        done(false)
      })
    }

    // 发送拉取请求
    device.request('sdk:pullImage', { imageUrl }, 600000).catch((e) => {
      if (!resolved) {
        onError?.(`拉取镜像失败: ${e.message}`)
        done(false)
      }
    })
  })
}

// 处理单条 SSE 事件
function handleEvent(event, callbacks) {
  const { onProgress, onExtracting, onComplete } = callbacks
  const status = event.status || ''

  if (status === 'Downloading') {
    const detail = event.progressDetail || {}
    if (detail.total && onProgress) {
      const current = detail.current || 0
      const total = detail.total
      const percent = Math.min(99, Math.round(current / total * 100))
      const text = `下载中: ${formatBytes(current)} / ${formatBytes(total)}`
      onProgress({ current, total, percent, text })
    }
  } else if (status === 'Extracting' || status === 'Pull complete' || status === 'Verifying Checksum') {
    onExtracting?.(status === 'Extracting' ? '正在解压镜像层...' : status)
  } else if (status.startsWith('Digest:') || status.startsWith('Status:')) {
    onComplete?.(status)
  } else if (status.includes('Pulling')) {
    onProgress?.({ current: 0, total: 0, percent: 0, text: status + (event.id ? ` ${event.id}` : '') })
  }
}

// 格式化字节数
export function formatBytes(bytes) {
  if (!bytes) return '0 B'
  if (bytes < 1024 * 1024) return (bytes / 1024).toFixed(1) + ' KB'
  if (bytes < 1024 * 1024 * 1024) return (bytes / 1024 / 1024).toFixed(1) + ' MB'
  return (bytes / 1024 / 1024 / 1024).toFixed(2) + ' GB'
}
