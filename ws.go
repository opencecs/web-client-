package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// ParsedContainer 容器结构化缓存（一次解析，多处使用）
type ParsedContainer struct {
	Name     string
	IndexNum int
	Status   string
	Raw      json.RawMessage // 单个容器的原始 JSON，保留所有字段
}

// parseContainerList 从 SDK 响应中一次性解析容器列表
func parseContainerList(raw []byte) []ParsedContainer {
	var resp struct {
		Data struct {
			List []json.RawMessage `json:"list"`
		} `json:"data"`
		List []json.RawMessage `json:"list"`
	}
	if json.Unmarshal(raw, &resp) != nil {
		return nil
	}
	srcList := resp.Data.List
	if srcList == nil {
		srcList = resp.List
	}
	result := make([]ParsedContainer, 0, len(srcList))
	for _, item := range srcList {
		var c struct {
			Name     string `json:"name"`
			IndexNum int    `json:"indexNum"`
			Status   string `json:"status"`
		}
		if json.Unmarshal(item, &c) == nil {
			result = append(result, ParsedContainer{
				Name: c.Name, IndexNum: c.IndexNum, Status: c.Status, Raw: item,
			})
		}
	}
	return result
}

var upgrader = websocket.Upgrader{
	CheckOrigin: func(r *http.Request) bool {
		// 允许同源和局域网访问
		origin := r.Header.Get("Origin")
		if origin == "" {
			return true // 无 Origin 头（非浏览器请求）
		}
		host := r.Host
		// 同源检查
		if strings.Contains(origin, host) {
			return true
		}
		// 允许局域网 IP
		for _, prefix := range []string{"http://192.168.", "http://10.", "http://172.", "http://127.", "http://localhost"} {
			if strings.HasPrefix(origin, prefix) {
				return true
			}
		}
		return false
	},
}

type WSClient struct {
	hub         *WSHub
	conn        *websocket.Conn
	send        chan []byte
	username    string
	userID      int64
	isAdmin     bool
	permissions *UserPermissions // nil = admin（全部权限）
	sessionKey  []byte           // AES 会话密钥
	tokenExp    time.Time        // token 过期时间
}

// WSRequest 客户端发来的请求
type WSRequest struct {
	Action string                 `json:"action"`
	ID     string                 `json:"id"`
	Data   map[string]interface{} `json:"data"`
}

type WSHub struct {
	clients    map[*WSClient]bool
	broadcast  chan []byte
	register   chan *WSClient
	unregister chan *WSClient
	mu         sync.RWMutex

	// 服务引用
	auth    *AuthService
	alias   *ContainerAliasService
	device  *DeviceService
	mytAuth *MytAuthService

	// 容器列表轮询
	deviceAddr     string
	httpClient     *http.Client
	streamClient   *http.Client // 无超时，用于流式代理
	containerCache json.RawMessage    // 原始 JSON（admin 推送用）
	parsedContainers []ParsedContainer // 结构化缓存（避免重复 JSON 解析）
	containerMu    sync.RWMutex
	refreshCh      chan struct{}

	// 截图缓存引用
	ssCache *ScreenshotCache

	// 投屏代理引用
	projProxy *ProjectionProxy
}

func NewWSHub(auth *AuthService, alias *ContainerAliasService, device *DeviceService, mytAuth *MytAuthService, deviceAddr string) *WSHub {
	return &WSHub{
		clients:    make(map[*WSClient]bool),
		broadcast:  make(chan []byte, 256),
		register:   make(chan *WSClient),
		unregister: make(chan *WSClient),
		auth:       auth,
		alias:      alias,
		device:     device,
		mytAuth:    mytAuth,
		deviceAddr: deviceAddr,
		httpClient: &http.Client{
			Timeout: 10 * time.Second,
			Transport: &http.Transport{
				MaxIdleConns:        20,
				MaxIdleConnsPerHost: 10,
				IdleConnTimeout:     60 * time.Second,
			},
		},
		streamClient: &http.Client{
			Timeout: 0,
			Transport: &http.Transport{
				MaxIdleConns:    10,
				IdleConnTimeout: 120 * time.Second,
			},
		},
		refreshCh: make(chan struct{}, 1),
	}
}

func (h *WSHub) Run() {
	for {
		select {
		case client := <-h.register:
			h.mu.Lock()
			h.clients[client] = true
			h.mu.Unlock()
			log.Printf("[WS] 客户端连接: %s (admin=%v)", client.username, client.isAdmin)
			// 推送容器缓存（按坑位权限过滤）
			h.containerMu.RLock()
			cached := h.containerCache
			parsed := h.parsedContainers
			h.containerMu.RUnlock()
			if cached != nil {
				var filtered json.RawMessage
				if client.permissions == nil {
					filtered = cached
				} else {
					filtered = filterByParsedSlots(parsed, client.permissions.AllowedSlotsMap())
				}
				msg, _ := json.Marshal(map[string]interface{}{
					"type": "event", "event": "containers:list",
					"data": json.RawMessage(filtered),
				})
				select {
				case client.send <- msg:
				default:
				}
			}
			// 推送别名数据
			if h.alias != nil {
				aliases := h.alias.GetAllAliases()
				msg, _ := json.Marshal(map[string]interface{}{
					"type": "event", "event": "aliases:list",
					"data": aliases,
				})
				select {
				case client.send <- msg:
				default:
				}
			}
			// 推送权限信息（非 admin）
			if client.permissions != nil {
				msg, _ := json.Marshal(map[string]interface{}{
					"type": "event", "event": "user:permissions",
					"data": client.permissions,
				})
				select {
				case client.send <- msg:
				default:
				}
			}
			// 推送截图缓存
			if h.ssCache != nil {
				h.pushScreenshotsToClient(client, h.ssCache)
			}

		case client := <-h.unregister:
			h.mu.Lock()
			if _, ok := h.clients[client]; ok {
				delete(h.clients, client)
				close(client.send)
			}
			h.mu.Unlock()
			log.Printf("[WS] 客户端断开: %s", client.username)

		case message := <-h.broadcast:
			h.mu.RLock()
			var stale []*WSClient
			for client := range h.clients {
				select {
				case client.send <- message:
				default:
					stale = append(stale, client)
				}
			}
			h.mu.RUnlock()
			// 用写锁批量清理发送失败的客户端
			if len(stale) > 0 {
				h.mu.Lock()
				for _, client := range stale {
					if _, ok := h.clients[client]; ok {
						delete(h.clients, client)
						close(client.send)
					}
				}
				h.mu.Unlock()
			}
		}
	}
}

func (h *WSHub) Broadcast(event string, data interface{}) {
	msg, _ := json.Marshal(map[string]interface{}{
		"type": "event", "event": event, "data": data,
	})
	h.broadcast <- msg
}

func (c *WSClient) SendJSON(data interface{}) {
	msg, _ := json.Marshal(data)
	defer func() { recover() }() // 防止 send on closed channel
	select {
	case c.send <- msg:
	default:
	}
}

func (h *WSHub) KickUser(username string) {
	h.KickUserWithReason(username, "expired")
}

func (h *WSHub) KickUserWithReason(username, reason string) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		if client.username == username {
			msg, _ := json.Marshal(map[string]interface{}{
				"type": "event", "event": "user:kicked",
				"data": map[string]string{"reason": reason},
			})
			select {
			case client.send <- msg:
			default:
			}
			client.conn.Close()
		}
	}
}

func (h *WSHub) HandleWS(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(userContextKey).(*Claims)
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[WS] Upgrade error: %v", err)
		return
	}

	isAdmin := claims.Role == "admin"
	var perms *UserPermissions
	if !isAdmin {
		perms = h.auth.GetUserPermissions(claims.UserID)
	}

	client := &WSClient{
		hub: h, conn: conn, send: make(chan []byte, 256),
		username: claims.Username, userID: claims.UserID,
		isAdmin: isAdmin, permissions: perms,
		sessionKey: h.auth.getSessionKey(claims.Username),
		tokenExp:   claims.ExpiresAt.Time,
	}
	h.register <- client
	go client.writePump()
	go client.readPump()
}

func (h *WSHub) TriggerRefresh() {
	select {
	case h.refreshCh <- struct{}{}:
	default:
	}
}

func (h *WSHub) PollContainers(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	h.fetchAndBroadcastContainers()
	for {
		select {
		case <-ticker.C:
			h.fetchAndBroadcastContainers()
		case <-h.refreshCh:
			h.fetchAndBroadcastContainers()
		}
	}
}

func (h *WSHub) fetchAndBroadcastContainers() {
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	url := fmt.Sprintf("http://%s/android", h.deviceAddr)
	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	// 一次性解析容器列表（后续所有查找/过滤都用结构化数据）
	parsed := parseContainerList(body)

	h.containerMu.Lock()
	h.containerCache = body
	h.parsedContainers = parsed
	h.containerMu.Unlock()

	// 预构建 admin 完整消息
	adminMsg, _ := json.Marshal(map[string]interface{}{
		"type": "event", "event": "containers:list",
		"data": json.RawMessage(body),
	})

	// 锁内只收集客户端列表，锁外构建消息并发送
	h.mu.RLock()
	type clientInfo struct {
		client *WSClient
		perms  *UserPermissions
	}
	clients := make([]clientInfo, 0, len(h.clients))
	for client := range h.clients {
		clients = append(clients, clientInfo{client, client.permissions})
	}
	h.mu.RUnlock()

	// 锁外构建消息并发送（JSON 序列化不持锁）
	for _, ci := range clients {
		var msg []byte
		if ci.perms == nil {
			msg = adminMsg
		} else {
			filtered := filterByParsedSlots(parsed, ci.perms.AllowedSlotsMap())
			msg, _ = json.Marshal(map[string]interface{}{
				"type": "event", "event": "containers:list",
				"data": json.RawMessage(filtered),
			})
		}
		func() {
			defer func() { recover() }()
			select {
			case ci.client.send <- msg:
			default:
			}
		}()
	}

	// 容器列表更新后立即触发截图抓取
	if h.ssCache != nil {
		go h.fetchAndPushScreenshots(h.ssCache)
	}

	// 通知投屏代理刷新预热连接池
	if h.projProxy != nil {
		h.projProxy.TriggerWarmRefresh()
	}
}

// deviceRequest 向设备 SDK 发送 HTTP 请求
func (h *WSHub) deviceRequest(method, path string, jsonBody interface{}) ([]byte, int, error) {
	reqURL := fmt.Sprintf("http://%s%s", h.deviceAddr, path)
	var bodyReader io.Reader
	if jsonBody != nil {
		data, _ := json.Marshal(jsonBody)
		bodyReader = strings.NewReader(string(data))
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	req, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
	if err != nil {
		return nil, 0, err
	}
	if jsonBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	resp, err := h.httpClient.Do(req)
	if err != nil {
		return nil, 0, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
	return body, resp.StatusCode, nil
}

// deviceRequestRaw 返回原始 JSON 数据
func (h *WSHub) deviceRequestRaw(method, path string, jsonBody interface{}) (json.RawMessage, error) {
	body, status, err := h.deviceRequest(method, path, jsonBody)
	if err != nil {
		return nil, fmt.Errorf("设备连接失败: %v", err)
	}
	if status >= 400 {
		return nil, fmt.Errorf("设备返回错误 (HTTP %d)", status)
	}
	return json.RawMessage(body), nil
}

// sdkRequest 通用 SDK 代理
func (h *WSHub) sdkRequest(method, path string, jsonBody interface{}, query url.Values) (json.RawMessage, error) {
	fullPath := path
	if query != nil && len(query) > 0 {
		fullPath = path + "?" + query.Encode()
	}
	return h.deviceRequestRaw(method, fullPath, jsonBody)
}

func (c *WSClient) writePump() {
	ticker := time.NewTicker(30 * time.Second)
	refreshTicker := time.NewTicker(10 * time.Minute) // 每 10 分钟检查一次 token 是否需要刷新
	defer func() { ticker.Stop(); refreshTicker.Stop(); c.conn.Close() }()
	for {
		select {
		case message, ok := <-c.send:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return
			}
			c.conn.WriteMessage(websocket.TextMessage, message)
		case <-ticker.C:
			c.conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
				return
			}
		case <-refreshTicker.C:
			// 距离 token 过期不到 2 小时时自动刷新
			if time.Until(c.tokenExp) < 2*time.Hour {
				c.refreshToken()
			}
		}
	}
}

// refreshToken 生成新 token 并通过 WS 加密推送
func (c *WSClient) refreshToken() {
	user := c.hub.auth.getUserByID(c.userID)
	if user == nil || !user.Enabled {
		return
	}
	newToken, err := c.hub.auth.generateToken(user)
	if err != nil {
		log.Printf("[WS] token 刷新失败: %v", err)
		return
	}
	// 更新过期时间
	c.tokenExp = time.Now().Add(24 * time.Hour)

	// 构建刷新消息
	payload, _ := json.Marshal(map[string]interface{}{
		"type":  "event",
		"event": "token:refresh",
		"data":  map[string]string{"token": newToken},
	})

	// 如果有会话密钥则加密
	if c.sessionKey != nil {
		encrypted, err := aesEncrypt(c.sessionKey, payload)
		if err == nil {
			msg, _ := json.Marshal(map[string]string{
				"type": "encrypted",
				"data": encrypted,
			})
			c.send <- msg
			log.Printf("[WS] token 已加密刷新: %s", c.username)
			return
		}
	}
	// 无密钥则明文发送
	c.send <- payload
	log.Printf("[WS] token 已刷新: %s", c.username)
}

func (c *WSClient) readPump() {
	defer func() { c.hub.unregister <- c; c.conn.Close() }()
	c.conn.SetReadLimit(65536)
	c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
	c.conn.SetPongHandler(func(string) error {
		c.conn.SetReadDeadline(time.Now().Add(60 * time.Second))
		return nil
	})
	for {
		_, msg, err := c.conn.ReadMessage()
		if err != nil {
			break
		}
		var req WSRequest
		if err := json.Unmarshal(msg, &req); err != nil {
			continue
		}
		go c.handleRequest(req)
	}
}

// handleRequest 路由分发
func (c *WSClient) handleRequest(req WSRequest) {
	// 权限检查
	if !c.hasPermission(req.Action) {
		c.sendResponse(req.ID, false, "无权限", nil)
		return
	}

	switch req.Action {
	// 容器操作
	case "containers:refresh":
		c.hub.TriggerRefresh()
	case "container:start", "container:stop", "container:restart",
		"container:reset", "container:delete", "container:rename", "container:copy":
		c.handleContainerAction(req)
	// 别名
	case "alias:list", "alias:set", "alias:delete":
		c.handleAliasAction(req)
	// SDK 操作
	case "sdk:createContainer", "sdk:listImages", "sdk:deleteImage",
		"sdk:pullImage", "sdk:pruneImages",
		"sdk:getPhoneModels", "sdk:getCountryCodes",
		"sdk:listBridges", "sdk:createBridge", "sdk:updateBridge", "sdk:deleteBridge",
		"sdk:listVpcGroups", "sdk:createVpcGroup", "sdk:deleteVpcGroup",
		"sdk:renameVpcGroup", "sdk:refreshVpcGroup", "sdk:deleteVpcNode",
		"sdk:addVpcSocks", "sdk:testVpcNode",
		"sdk:listContainerRules", "sdk:addVpcRule", "sdk:removeVpcRule",
		"sdk:addVpcRuleBatch", "sdk:removeVpcRuleBatch",
		"sdk:toggleWhiteListDns",
		"sdk:getDomainDirect", "sdk:setDomainDirect", "sdk:deleteDomainDirect",
		"sdk:getDomainFilter", "sdk:setDomainFilter", "sdk:deleteDomainFilter",
		"sdk:getGlobalDomainFilter", "sdk:setGlobalDomainFilter", "sdk:deleteGlobalDomainFilter",
		"sdk:listBackups", "sdk:deleteBackup",
		"sdk:listModelBackups", "sdk:deleteModelBackup",
		"sdk:batchChangeImage":
		c.handleSDKAction(req)
	// 设备管理
	case "device:info", "device:version", "device:mirrors", "device:reboot",
		"device:upgrade", "device:cleanDisk":
		c.handleDeviceAction(req)
	// MYT 云平台
	case "myt:status", "myt:slotStates", "myt:login", "myt:logout",
		"myt:sync", "myt:autoToggle", "myt:bindStatus",
		"myt:bind", "myt:vcode", "myt:unbind":
		c.handleMytAction(req)
	// 用户管理
	case "user:list", "user:create", "user:update", "user:delete",
		"user:getPermissions", "user:setPermissions":
		c.handleUserAction(req)
	// 投屏 token
	case "projection:token":
		c.handleProjectionToken(req)
	// 系统设置
	case "settings:get", "settings:set":
		c.handleSettingsAction(req)
	// 面板更新
	case "panel:version":
		c.handlePanelVersion(req)
	case "panel:checkUpdate":
		c.handlePanelCheckUpdate(req)
	case "panel:doUpdate":
		go c.handlePanelDoUpdate(req)
	default:
		c.sendResponse(req.ID, false, "未知操作: "+req.Action, nil)
	}
}

// ===== 通用响应 =====

func (c *WSClient) sendResponse(reqID string, ok bool, message string, data interface{}) {
	resp := map[string]interface{}{
		"type": "response", "id": reqID, "ok": ok, "message": message,
	}
	if data != nil {
		resp["data"] = data
	}
	c.SendJSON(resp)
}

// ===== 工具函数 =====

// handleProjectionToken 处理投屏 token 请求
func (c *WSClient) handleProjectionToken(req WSRequest) {
	containerName, _ := req.Data["container_name"].(string)
	if containerName == "" {
		c.sendResponse(req.ID, false, "缺少 container_name", nil)
		return
	}

	// 权限检查：非 admin 需要投屏权限 + 坑位权限
	if !c.isAdmin {
		if c.permissions == nil || !c.permissions.Projection {
			c.sendResponse(req.ID, false, "无投屏权限", nil)
			return
		}
		// 查找容器的坑位号并检查权限
		indexNum := c.hub.projProxy.findContainerIndex(containerName)
		if indexNum <= 0 {
			c.sendResponse(req.ID, false, "容器不存在", nil)
			return
		}
		hasSlot := false
		for _, s := range c.permissions.Slots {
			if s == indexNum {
				hasSlot = true
				break
			}
		}
		if !hasSlot {
			c.sendResponse(req.ID, false, "无此坑位权限", nil)
			return
		}
	}

	// 生成投屏专用短期 token
	token, err := c.hub.auth.generateProjectionToken(c.userID, c.username, func() string {
		if c.isAdmin {
			return "admin"
		}
		return "user"
	}(), containerName)
	if err != nil {
		c.sendResponse(req.ID, false, "token 生成失败", nil)
		return
	}

	// 构建响应（投屏 token 明文返回，本身是60秒短期token，无需加密）
	respData := map[string]string{"token": token}
	// 附带公网 UDP 端口配置（空则前端使用网页端口）
	if udpPort := c.hub.auth.GetSetting("public_udp_port"); udpPort != "" {
		respData["udpPort"] = udpPort
	}
	c.sendResponse(req.ID, true, "", respData)
}

func getStr(m map[string]interface{}, key string) string {
	if m == nil {
		return ""
	}
	if v, ok := m[key].(string); ok {
		return v
	}
	return ""
}

func getNum(m map[string]interface{}, key string) int {
	if m == nil {
		return 0
	}
	switch v := m[key].(type) {
	case float64:
		return int(v)
	case int:
		return v
	}
	return 0
}

func getBool(m map[string]interface{}, key string) bool {
	if m == nil {
		return false
	}
	if v, ok := m[key].(bool); ok {
		return v
	}
	return false
}
