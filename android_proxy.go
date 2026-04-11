package main

import (
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"

	"github.com/gorilla/websocket"
)

// SDKProxy 通用 SDK API 反向代理
// 将 /api/sdk/* 请求透传到设备 http://device:port/*
type SDKProxy struct {
	deviceAddr string
	httpClient *http.Client
	// 长连接客户端，用于 SSE 流式请求（pullImage 等），无超时限制
	streamClient *http.Client
}

func NewSDKProxy(deviceAddr string) *SDKProxy {
	noRedirect := func(req *http.Request, via []*http.Request) error {
		return http.ErrUseLastResponse
	}
	return &SDKProxy{
		deviceAddr: deviceAddr,
		httpClient: &http.Client{
			Timeout:       300 * time.Second,
			CheckRedirect: noRedirect,
		},
		streamClient: &http.Client{
			Timeout:       0, // 无超时，SSE 流可能持续很长时间
			CheckRedirect: noRedirect,
		},
	}
}

// HandleProxy 处理所有 /api/sdk/* 请求
func (p *SDKProxy) HandleProxy(w http.ResponseWriter, r *http.Request) {
	// 提取 /api/sdk/ 后面的路径
	path := strings.TrimPrefix(r.URL.Path, "/api/sdk")
	if path == "" {
		path = "/"
	}

	// WebSocket 请求走专门的代理
	if websocket.IsWebSocketUpgrade(r) {
		p.proxyWebSocket(w, r, path)
		return
	}

	// 构建目标 URL
	targetURL := fmt.Sprintf("http://%s%s", p.deviceAddr, path)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	log.Printf("[SDKProxy] %s %s → %s", r.Method, r.URL.Path, targetURL)

	// 创建代理请求
	proxyReq, err := http.NewRequestWithContext(r.Context(), r.Method, targetURL, r.Body)
	if err != nil {
		log.Printf("[SDKProxy] 创建请求失败: %v", err)
		jsonError(w, "proxy error: "+err.Error(), 500)
		return
	}

	// 透传 Content-Type 等请求头
	if ct := r.Header.Get("Content-Type"); ct != "" {
		proxyReq.Header.Set("Content-Type", ct)
	}
	if cl := r.Header.Get("Content-Length"); cl != "" {
		proxyReq.Header.Set("Content-Length", cl)
	}

	// 已知的流式请求路径使用无超时的 streamClient
	client := p.httpClient
	isStreamPath := strings.Contains(path, "pullImage") || strings.Contains(path, "export")
	if isStreamPath {
		client = p.streamClient
		log.Printf("[SDKProxy] 使用流式客户端（无超时）: %s", path)
	}

	startTime := time.Now()

	// 执行请求
	resp, err := client.Do(proxyReq)
	if err != nil {
		log.Printf("[SDKProxy] 请求设备失败: %s %v", targetURL, err)
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	// 透传响应头
	for k, vv := range resp.Header {
		for _, v := range vv {
			w.Header().Add(k, v)
		}
	}

	// 检查是否是 SSE 流式响应
	contentType := resp.Header.Get("Content-Type")
	isStream := strings.Contains(contentType, "text/event-stream") ||
		strings.Contains(contentType, "application/octet-stream")

	w.WriteHeader(resp.StatusCode)

	if isStream {
		log.Printf("[SDKProxy] SSE 流式转发开始: %s (Content-Type: %s)", path, contentType)
		// 流式转发
		flusher, _ := w.(http.Flusher)
		buf := make([]byte, 4096)
		totalBytes := 0
		for {
			n, err := resp.Body.Read(buf)
			if n > 0 {
				w.Write(buf[:n])
				totalBytes += n
				if flusher != nil {
					flusher.Flush()
				}
			}
			if err != nil {
				break
			}
		}
		log.Printf("[SDKProxy] SSE 流式转发结束: %s, 总计 %d 字节, 耗时 %v", path, totalBytes, time.Since(startTime))
	} else {
		// 普通响应直接拷贝
		written, _ := io.Copy(w, resp.Body)
		log.Printf("[SDKProxy] 响应完成: %s %d %dB %v", path, resp.StatusCode, written, time.Since(startTime))
	}
}

// proxyWebSocket 代理 WebSocket 连接
func (p *SDKProxy) proxyWebSocket(w http.ResponseWriter, r *http.Request, path string) {
	// 升级前端连接
	frontConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[SDKProxy] WebSocket 升级失败: %v", err)
		return
	}
	defer frontConn.Close()

	// 连接设备
	deviceWSURL := fmt.Sprintf("ws://%s%s", p.deviceAddr, path)
	if r.URL.RawQuery != "" {
		deviceWSURL += "?" + r.URL.RawQuery
	}
	deviceConn, _, err := websocket.DefaultDialer.Dial(deviceWSURL, nil)
	if err != nil {
		log.Printf("[SDKProxy] 连接设备 WebSocket 失败: %v", err)
		frontConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "无法连接设备"))
		return
	}
	defer deviceConn.Close()

	log.Printf("[SDKProxy] WebSocket 代理已建立: %s", path)

	done := make(chan struct{})

	// 设备 → 前端
	go func() {
		defer close(done)
		for {
			msgType, data, err := deviceConn.ReadMessage()
			if err != nil {
				break
			}
			if err := frontConn.WriteMessage(msgType, data); err != nil {
				break
			}
		}
	}()

	// 前端 → 设备
	go func() {
		for {
			msgType, data, err := frontConn.ReadMessage()
			if err != nil {
				deviceConn.Close()
				break
			}
			if err := deviceConn.WriteMessage(msgType, data); err != nil {
				break
			}
		}
	}()

	<-done
}
