package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type DeviceService struct {
	addr          string
	httpClient    *http.Client
	cacheMu       sync.RWMutex
	cachedInfo    []byte    // raw JSON from /info/device
	cachedVersion []byte    // raw JSON from /info
	versionCacheT time.Time // when version was last fetched
}

func NewDeviceService(addr string) *DeviceService {
	return &DeviceService{
		addr: addr,
		httpClient: &http.Client{
			Transport: &http.Transport{
				MaxIdleConns:        100,
				MaxIdleConnsPerHost: 2,
				IdleConnTimeout:     120 * time.Second,
				DialContext: (&net.Dialer{
					Timeout:   5 * time.Second,
					KeepAlive: 60 * time.Second,
				}).DialContext,
				ForceAttemptHTTP2: false,
			},
		},
	}
}

func (d *DeviceService) deviceURL(path string) string {
	return fmt.Sprintf("http://%s%s", d.addr, path)
}

func (d *DeviceService) proxyGet(w http.ResponseWriter, path string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", d.deviceURL(path), nil)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	w.Write(body)
}

// GET /api/device/info -> return cached device info (updated every 4s by poller)
func (d *DeviceService) HandleInfo(w http.ResponseWriter, r *http.Request) {
	d.cacheMu.RLock()
	data := d.cachedInfo
	d.cacheMu.RUnlock()

	if data != nil {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}
	// No cache yet, proxy directly
	d.proxyGet(w, "/info/device", 15*time.Second)
}

// GET /api/device/version -> cached /info (4s TTL)
func (d *DeviceService) HandleVersion(w http.ResponseWriter, r *http.Request) {
	d.cacheMu.RLock()
	data := d.cachedVersion
	t := d.versionCacheT
	d.cacheMu.RUnlock()

	if data != nil && time.Since(t) < 4*time.Second {
		w.Header().Set("Content-Type", "application/json")
		w.Write(data)
		return
	}

	// Fetch fresh
	resp, err := d.httpClient.Get(d.deviceURL("/info"))
	if err != nil {
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	d.cacheMu.Lock()
	d.cachedVersion = body
	d.versionCacheT = time.Now()
	d.cacheMu.Unlock()

	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// POST /api/device/upgrade -> proxy to device /server/upgrade (SSE)
func (d *DeviceService) HandleUpgrade(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", d.deviceURL("/server/upgrade"), nil)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	// Forward SSE response as-is
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}

// POST /api/device/reboot
func (d *DeviceService) HandleReboot(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", d.deviceURL("/server/reboot"), nil)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	w.Header().Set("Content-Type", "application/json")
	w.Write(body)
}

// POST /api/device/clean-disk -> proxy to device /server/device/reset (SSE stream)
func (d *DeviceService) HandleCleanDisk(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(context.Background(), 600*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "POST", d.deviceURL("/server/device/reset"), nil)
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}

	resp, err := d.httpClient.Do(req)
	if err != nil {
		jsonError(w, "device unreachable: "+err.Error(), 502)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	flusher, _ := w.(http.Flusher)
	buf := make([]byte, 4096)
	for {
		n, err := resp.Body.Read(buf)
		if n > 0 {
			w.Write(buf[:n])
			if flusher != nil {
				flusher.Flush()
			}
		}
		if err != nil {
			break
		}
	}
}
// GET /ws/ssh -> WebSocket 代理到设备 SSH 终端
func (d *DeviceService) HandleSSHProxy(w http.ResponseWriter, r *http.Request) {
	// 升级前端连接为 WebSocket
	frontConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[SSH] 前端 WebSocket 升级失败: %v", err)
		return
	}
	defer frontConn.Close()

	// 连接设备的 SSH WebSocket
	deviceWSURL := fmt.Sprintf("ws://%s/link/ssh", d.addr)
	deviceConn, _, err := websocket.DefaultDialer.Dial(deviceWSURL, nil)
	if err != nil {
		log.Printf("[SSH] 连接设备 WebSocket 失败: %v", err)
		frontConn.WriteMessage(websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "无法连接设备"))
		return
	}
	defer deviceConn.Close()

	log.Printf("[SSH] WebSocket 代理已建立")

	done := make(chan struct{})

	// 设备→前端: 转发终端输出
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

	// 前端→设备: 转发用户输入
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

func (d *DeviceService) HandleAuthSync(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Token string `json:"token"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	// Extract IP from device address
	host, _, _ := net.SplitHostPort(d.addr)
	if host == "" {
		host = d.addr
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		jsonError(w, "udp error: "+err.Error(), 500)
		return
	}
	defer conn.Close()

	msg := []byte("lgtoken:" + req.Token)
	_, err = conn.WriteToUDP(msg, &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: 7678,
	})
	if err != nil {
		jsonError(w, "send failed: "+err.Error(), 500)
		return
	}

	jsonResponse(w, map[string]interface{}{"ok": true})
}

// GetDeviceID 从缓存中提取设备ID
func (d *DeviceService) GetDeviceID() string {
	d.cacheMu.RLock()
	data := d.cachedInfo
	d.cacheMu.RUnlock()
	if data == nil {
		return ""
	}
	var info struct {
		Data struct {
			DeviceID string `json:"deviceId"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &info) == nil && info.Data.DeviceID != "" {
		return info.Data.DeviceID
	}
	// 可能没有 data 包裹
	var flat struct {
		DeviceID string `json:"deviceId"`
	}
	if json.Unmarshal(data, &flat) == nil {
		return flat.DeviceID
	}
	return ""
}

// PollStatus fetches device info periodically and broadcasts via WebSocket
func (d *DeviceService) PollStatus(hub *WSHub, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		req, err := http.NewRequestWithContext(ctx, "GET", d.deviceURL("/info/device"), nil)
		if err != nil {
			cancel()
			continue
		}

		resp, err := d.httpClient.Do(req)
		if err != nil {
			cancel()
			hub.Broadcast("device:status", map[string]interface{}{"online": false, "error": err.Error()})
			continue
		}

		body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
		resp.Body.Close()
		cancel()

		// Update cache
		d.cacheMu.Lock()
		d.cachedInfo = body
		d.cacheMu.Unlock()

		var data map[string]interface{}
		if err := json.Unmarshal(body, &data); err == nil {
			data["online"] = true
			hub.Broadcast("device:status", data)
		} else {
			log.Printf("[Device] Failed to parse status: %v", err)
		}
	}
}
