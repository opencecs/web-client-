package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

// activeSession 当前活跃的投屏会话
type activeSession struct {
	frontConn  *websocket.Conn
	deviceConn *websocket.Conn
	username   string
	done       chan struct{} // 关闭时通知
}

// closeDeviceConn 关闭与容器的 WebSocket 连接
// 先发送 Close 帧通知容器释放信令会话，再用 SO_LINGER(0) 强制 RST
// 避免容器未完成关闭握手导致 FIN_WAIT2 僵尸连接（会阻塞后续投屏）
func closeDeviceConn(conn *websocket.Conn) {
	if conn == nil {
		return
	}
	// 发送 WebSocket Close 帧，通知容器释放信令会话
	conn.WriteControl(websocket.CloseMessage,
		websocket.FormatCloseMessage(websocket.CloseNormalClosure, ""),
		time.Now().Add(2*time.Second))
	// 等待容器接收 Close 帧（localhost 几乎无延迟）
	time.Sleep(50 * time.Millisecond)
	// 强制 RST 关闭，避免 FIN_WAIT2（容器信令服务经常不完成关闭握手）
	if tcpConn, ok := conn.UnderlyingConn().(*net.TCPConn); ok {
		tcpConn.SetLinger(0)
	}
	conn.Close()
}

// warmConn 预热连接（保持容器投屏服务活跃）
type warmConn struct {
	conn   *websocket.Conn
	cancel context.CancelFunc
	done   chan struct{}
}

// ProjectionProxy 投屏 WebSocket 信令代理
// 每个坑位同时只允许一个投屏连接（互斥），新连接会踢掉旧连接
type ProjectionProxy struct {
	auth      *AuthService
	hub       *WSHub
	registry  *SessionRegistry
	active    sync.Map // indexNum(int) → *activeSession
	warmPool  sync.Map // indexNum(int) → *warmConn（预热连接池）
	warmLock  sync.Map // indexNum(int) → *sync.Mutex（每个坑位的连接锁）
	warming   sync.Map // indexNum(int) → bool（标记正在重连循环中的坑位）
	refreshCh chan struct{} // 容器列表变化通知
}

func NewProjectionProxy(auth *AuthService, hub *WSHub, registry *SessionRegistry) *ProjectionProxy {
	return &ProjectionProxy{
		auth:      auth,
		hub:       hub,
		registry:  registry,
		refreshCh: make(chan struct{}, 1),
	}
}

// getSlotLock 获取坑位级别的互斥锁（防止同一坑位并发连接）
func (p *ProjectionProxy) getSlotLock(indexNum int) *sync.Mutex {
	val, _ := p.warmLock.LoadOrStore(indexNum, &sync.Mutex{})
	return val.(*sync.Mutex)
}

// evictExisting 踢掉坑位上已有的连接
func (p *ProjectionProxy) evictExisting(indexNum int) {
	if val, ok := p.active.Load(indexNum); ok {
		old := val.(*activeSession)
		log.Printf("[投屏代理] 踢掉坑位 %d 的旧连接 (用户 %s)", indexNum, old.username)
		// 发送自定义文本消息通知旧客户端被接管（比 close 帧更可靠）
		old.frontConn.WriteMessage(websocket.TextMessage, []byte(`{"id":"evicted","data":"投屏已被其他窗口接管"}`))
		// 等浏览器收到消息
		time.Sleep(200 * time.Millisecond)
		// 关闭连接（先优雅关闭容器端，避免 FIN_WAIT2）
		closeDeviceConn(old.deviceConn)
		old.frontConn.Close()
		<-old.done
		p.active.Delete(indexNum)
	}
}

// EvictUser 踢掉指定用户的所有投屏连接（账号过期/禁用时调用）
func (p *ProjectionProxy) EvictUser(username string) {
	p.active.Range(func(key, val interface{}) bool {
		sess := val.(*activeSession)
		if sess.username == username {
			indexNum := key.(int)
			log.Printf("[投屏代理] 踢掉用户 %s 的投屏 (坑位 %d)", username, indexNum)
			sess.frontConn.WriteMessage(websocket.TextMessage, []byte(`{"id":"evicted","data":"账号已过期"}`))
			time.Sleep(200 * time.Millisecond)
			closeDeviceConn(sess.deviceConn)
			sess.frontConn.Close()
			p.active.Delete(key)
		}
		return true
	})
}

// --- 预热连接池 ---

// StartWarmPool 启动预热连接池，监听容器列表变化并维护连接
func (p *ProjectionProxy) StartWarmPool() {
	// 等待容器缓存首次加载（最多等 30 秒）
	for i := 0; i < 30; i++ {
		time.Sleep(1 * time.Second)
		p.hub.containerMu.RLock()
		cached := p.hub.containerCache
		p.hub.containerMu.RUnlock()
		if cached != nil {
			break
		}
	}
	log.Printf("[投屏预热] 连接池启动")
	p.syncWarmPool()

	for range p.refreshCh {
		p.syncWarmPool()
	}
}

// TriggerWarmRefresh 通知预热连接池刷新
func (p *ProjectionProxy) TriggerWarmRefresh() {
	select {
	case p.refreshCh <- struct{}{}:
	default:
	}
}

// syncWarmPool 同步预热连接池与当前运行中的容器列表
func (p *ProjectionProxy) syncWarmPool() {
	running := p.getRunningSlots()
	runningMap := make(map[int]bool, len(running))
	for _, idx := range running {
		runningMap[idx] = true
	}

	// 关闭不再运行的容器的预热连接
	p.warmPool.Range(func(key, value interface{}) bool {
		idx := key.(int)
		if !runningMap[idx] {
			w := value.(*warmConn)
			log.Printf("[投屏预热] 关闭坑位 %d 的预热连接（容器已停止）", idx)
			w.cancel()
			<-w.done
			p.warmPool.Delete(idx)
		}
		return true
	})

	// 为运行中容器建立预热连接（跳过已有预热、活跃连接或正在重连的坑位）
	for _, idx := range running {
		if _, hasWarm := p.warmPool.Load(idx); hasWarm {
			continue
		}
		if _, hasActive := p.active.Load(idx); hasActive {
			continue
		}
		if _, isWarming := p.warming.Load(idx); isWarming {
			continue
		}
		idx := idx // 闭包变量
		go p.warmConnect(idx, 0)
	}
}

// getRunningSlots 从结构化缓存获取所有运行中的容器坑位号
func (p *ProjectionProxy) getRunningSlots() []int {
	p.hub.containerMu.RLock()
	parsed := p.hub.parsedContainers
	p.hub.containerMu.RUnlock()

	var slots []int
	for _, c := range parsed {
		if c.Status == "running" && c.IndexNum > 0 {
			slots = append(slots, c.IndexNum)
		}
	}
	return slots
}

// warmConnect 建立预热连接并维持心跳（带坑位级互斥锁）
// retryCount 用于限制连续失败重试次数
func (p *ProjectionProxy) warmConnect(indexNum int, retryCount int) {
	const maxRetries = 5
	mu := p.getSlotLock(indexNum)
	if !mu.TryLock() {
		return
	}
	defer mu.Unlock()

	// 二次检查
	if _, hasWarm := p.warmPool.Load(indexNum); hasWarm {
		return
	}
	if _, hasActive := p.active.Load(indexNum); hasActive {
		return
	}

	// 标记正在处理该坑位的重连
	p.warming.Store(indexNum, true)

	targetPort := 30000 + (indexNum-1)*100 + 7
	targetURL := fmt.Sprintf("ws://127.0.0.1:%d/lgcloud?user=warm&os=mobile&type=1&quality=1&platform=1&dm=0&width=1280&height=720", targetPort)

	conn, _, err := websocket.DefaultDialer.Dial(targetURL, nil)
	if err != nil {
		log.Printf("[投屏预热] 连接坑位 %d 失败 (第%d次): %v", indexNum, retryCount+1, err)
		if retryCount+1 >= maxRetries {
			log.Printf("[投屏预热] 坑位 %d 连续失败 %d 次，停止重试", indexNum, maxRetries)
			p.warming.Delete(indexNum)
			return
		}
		// 指数退避重试：30s, 60s, 120s, 240s
		delay := time.Duration(30<<retryCount) * time.Second
		go func() {
			time.Sleep(delay)
			p.warming.Delete(indexNum)
			p.warmConnect(indexNum, retryCount+1)
		}()
		return
	}

	ctx, cancel := context.WithCancel(context.Background())
	done := make(chan struct{})
	w := &warmConn{conn: conn, cancel: cancel, done: done}
	p.warmPool.Store(indexNum, w)
	p.warming.Delete(indexNum) // 连接成功，清除重连标记

	log.Printf("[投屏预热] 坑位 %d 预热连接已建立 (端口 %d)", indexNum, targetPort)

	go func() {
		defer func() {
			closeDeviceConn(conn)
			p.warmPool.CompareAndDelete(indexNum, w)
			close(done)
		}()

		// 心跳发送协程
		heartDone := make(chan struct{})
		go func() {
			defer close(heartDone)
			ticker := time.NewTicker(1 * time.Second)
			defer ticker.Stop()
			for {
				select {
				case <-ctx.Done():
					return
				case <-ticker.C:
					if err := conn.WriteMessage(websocket.TextMessage, []byte(`{"id":"heart","data":"1"}`)); err != nil {
						return
					}
				}
			}
		}()

		// 读取协程（消费容器消息，保持 WebSocket 活跃）
		// 增加初始健康检测：10 秒内必须收到至少一条消息，否则说明安卓未就绪
		firstMsg := make(chan struct{}, 1)
		readDone := make(chan struct{})
		go func() {
			defer close(readDone)
			for {
				_, _, err := conn.ReadMessage()
				if err != nil {
					return
				}
				select {
				case firstMsg <- struct{}{}:
				default:
				}
			}
		}()

		// 等待初始健康检测通过（10秒内收到任意消息）
		select {
		case <-ctx.Done():
			return
		case <-firstMsg:
			// 信令正常，继续保持连接
			log.Printf("[投屏预热] 坑位 %d 信令健康检测通过", indexNum)
		case <-time.After(10 * time.Second):
			// 安卓未就绪或信令卡死，关闭后延迟重试
			log.Printf("[投屏预热] 坑位 %d 未收到消息（安卓未就绪），断开重试", indexNum)
			// defer 会关闭连接，下面触发延迟重连
			p.warming.Store(indexNum, true)
			go func() {
				time.Sleep(30 * time.Second)
				p.warming.Delete(indexNum)
				p.warmConnect(indexNum, 0)
			}()
			return
		case <-heartDone:
		case <-readDone:
		}

		// 持久保持连接，等待任一方退出
		select {
		case <-ctx.Done():
		case <-heartDone:
		case <-readDone:
		}

		// 异常断开 → 延迟重连
		select {
		case <-ctx.Done():
			// 被主动取消（takeWarm / syncWarmPool），不自动重连
		default:
			log.Printf("[投屏预热] 坑位 %d 预热连接断开，5秒后重连", indexNum)
			p.warming.Store(indexNum, true)
			go func() {
				time.Sleep(5 * time.Second)
				p.warming.Delete(indexNum)
				p.warmConnect(indexNum, 0)
			}()
		}
	}()
}

// takeWarm 取出并关闭预热连接（用户投屏前调用），返回是否实际释放了连接
// 不等待协程退出，直接关闭底层连接以加速释放
func (p *ProjectionProxy) takeWarm(indexNum int) bool {
	if val, ok := p.warmPool.LoadAndDelete(indexNum); ok {
		w := val.(*warmConn)
		w.cancel()
		closeDeviceConn(w.conn) // 优雅关闭，避免僵尸连接
		log.Printf("[投屏预热] 坑位 %d 预热连接已释放", indexNum)
		return true
	}
	return false
}

// returnWarm 延迟后重新建立预热连接（用户断开投屏后调用）
func (p *ProjectionProxy) returnWarm(indexNum int) {
	go func() {
		time.Sleep(2 * time.Second)
		p.warmConnect(indexNum, 0)
	}()
}

// HandleProjection 处理 /lgcloud WebSocket 代理请求
func (p *ProjectionProxy) HandleProjection(w http.ResponseWriter, r *http.Request) {
	// 验证投屏专用 token
	rawToken := r.URL.Query().Get("token")
	if rawToken == "" {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		return
	}

	// 优先尝试投屏专用 token
	var containerName string
	var userID int64
	var username, role string

	projClaims, err := p.auth.parseProjectionToken(rawToken)
	if err == nil {
		// 投屏专用 token
		containerName = projClaims.ContainerName
		userID = projClaims.UserID
		username = projClaims.Username
		role = projClaims.Role
	} else {
		// 兼容旧格式：jwt_token:container_name
		tokenStr, cName := splitProjectionToken(rawToken)
		if tokenStr == "" || cName == "" {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		claims, err := p.auth.parseToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}
		containerName = cName
		userID = claims.UserID
		username = claims.Username
		role = claims.Role
	}

	user := p.auth.getUserByID(userID)
	if user == nil || !user.Enabled {
		http.Error(w, `{"error":"account disabled"}`, http.StatusForbidden)
		return
	}
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		http.Error(w, `{"error":"account expired"}`, http.StatusForbidden)
		return
	}

	indexNum := p.findContainerIndex(containerName)
	if indexNum <= 0 {
		http.Error(w, `{"error":"container not found"}`, http.StatusNotFound)
		return
	}

	// 权限检查
	if role != "admin" {
		perms := p.auth.GetUserPermissions(userID)
		if perms == nil || !perms.Projection {
			http.Error(w, `{"error":"no projection permission"}`, http.StatusForbidden)
			return
		}
		allowed := false
		for _, s := range perms.Slots {
			if s == indexNum {
				allowed = true
				break
			}
		}
		if !allowed {
			http.Error(w, `{"error":"no access to this slot"}`, http.StatusForbidden)
			return
		}
	}

	// 计算端口
	targetPort := 30000 + (indexNum-1)*100 + 7
	containerUDPPort := 30000 + (indexNum-1)*100 + 8
	targetURL := fmt.Sprintf("ws://127.0.0.1:%d/lgcloud", targetPort)
	if r.URL.RawQuery != "" {
		targetURL += "?" + r.URL.RawQuery
	}

	// 升级浏览器 WebSocket
	frontConn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		log.Printf("[投屏代理] WebSocket 升级失败: %v", err)
		return
	}
	defer frontConn.Close()

	// 释放该坑位的预热连接（token 请求时已提前释放，这里兜底）
	p.takeWarm(indexNum)

	// 踢掉该坑位上已有的连接
	p.evictExisting(indexNum)

	// 等容器清理信令会话（预热连接可能刚被 handleProjectionToken 异步释放）
	time.Sleep(300 * time.Millisecond)

	// 连接容器（大缓冲低延迟，服务端重试一次）
	projDialer := websocket.Dialer{
		ReadBufferSize:  32 * 1024,
		WriteBufferSize: 32 * 1024,
	}
	deviceConn, _, err := projDialer.Dial(targetURL, nil)
	if err != nil {
		time.Sleep(500 * time.Millisecond)
		deviceConn, _, err = projDialer.Dial(targetURL, nil)
		if err != nil {
			log.Printf("[投屏代理] 连接容器失败 (坑位 %d): %v", indexNum, err)
			frontConn.WriteMessage(websocket.CloseMessage,
				websocket.FormatCloseMessage(websocket.CloseInternalServerErr, "无法连接容器"))
			return
		}
	}
	defer closeDeviceConn(deviceConn)

	// 设置浏览器端 pong 处理器：收到 pong 时重置读超时
	const pongWait = 60 * time.Second
	frontConn.SetReadDeadline(time.Now().Add(pongWait))
	frontConn.SetPongHandler(func(string) error {
		frontConn.SetReadDeadline(time.Now().Add(pongWait))
		return nil
	})

	// 设置容器端读超时（容器的信令心跳间隔较短，给足够余量）
	const deviceReadWait = 120 * time.Second
	deviceConn.SetReadDeadline(time.Now().Add(deviceReadWait))

	// 注册当前活跃会话
	sess := &activeSession{
		frontConn:  frontConn,
		deviceConn: deviceConn,
		username:   username,
		done:       make(chan struct{}),
	}
	p.active.Store(indexNum, sess)
	defer func() {
		p.active.CompareAndDelete(indexNum, sess)
		close(sess.done)
		// 投屏断开后恢复预热连接
		p.returnWarm(indexNum)
	}()

	log.Printf("[投屏代理] 用户 %s 连接坑位 %d (端口 %d, 容器 %s)", username, indexNum, targetPort, containerName)

	var sessionUfrag string
	var deviceWriteMu sync.Mutex // 保护 deviceConn 并发写入
	var frontWriteMu sync.Mutex  // 保护 frontConn 并发写入

	// 信令超时检测：10 秒内必须收到 SDP offer（注册 UDP 会话），
	// 否则说明安卓未就绪或信令卡死，强制断开让浏览器重连
	signalingTimer := time.AfterFunc(10*time.Second, func() {
		if sessionUfrag == "" {
			log.Printf("[投屏代理] 坑位 %d 信令超时（未收到 SDP offer），强制断开", indexNum)
			closeDeviceConn(deviceConn)
			frontConn.Close()
		}
	})

	// 浏览器端 ping 心跳（30秒一次）
	pingDone := make(chan struct{})
	go func() {
		defer close(pingDone)
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				frontWriteMu.Lock()
				frontConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				err := frontConn.WriteMessage(websocket.PingMessage, nil)
				frontWriteMu.Unlock()
				if err != nil {
					return
				}
			}
		}
	}()

	// 容器端心跳（防止浏览器后台时容器读超时断开）
	// 延迟 15 秒启动，等待 SDP 信令交换完成，避免干扰容器信令状态
	deviceHeartDone := make(chan struct{})
	go func() {
		defer close(deviceHeartDone)
		time.Sleep(15 * time.Second)
		ticker := time.NewTicker(2 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ticker.C:
				deviceWriteMu.Lock()
				deviceConn.SetWriteDeadline(time.Now().Add(5 * time.Second))
				err := deviceConn.WriteMessage(websocket.TextMessage, []byte(`{"id":"heart","data":"1"}`))
				deviceWriteMu.Unlock()
				if err != nil {
					return
				}
				// 心跳写入成功，重置容器端读超时
				deviceConn.SetReadDeadline(time.Now().Add(deviceReadWait))
			}
		}
	}()

	// 容器 → 浏览器
	deviceDone := make(chan struct{})
	go func() {
		defer close(deviceDone)
		for {
			msgType, data, err := deviceConn.ReadMessage()
			if err != nil {
				return
			}
			// 收到容器消息，重置容器端读超时
			deviceConn.SetReadDeadline(time.Now().Add(deviceReadWait))
			// 拦截 SDP offer 提取 ufrag，注册 UDP 会话
			if msgType == websocket.TextMessage && p.registry != nil {
				if ufrag := extractUfragFromOffer(data); ufrag != "" {
					sessionUfrag = ufrag
					signalingTimer.Stop() // 收到 offer，取消信令超时
					p.registry.Register(ufrag, containerUDPPort, GetMuxConn())
					log.Printf("[投屏代理] 注册 UDP 会话: ufrag=%s → 端口 %d", ufrag, containerUDPPort)
				}
			}
			frontWriteMu.Lock()
			frontConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err = frontConn.WriteMessage(msgType, data)
			frontWriteMu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// 浏览器 → 容器
	clientDone := make(chan struct{})
	go func() {
		defer close(clientDone)
		for {
			msgType, data, err := frontConn.ReadMessage()
			if err != nil {
				return
			}
			deviceWriteMu.Lock()
			deviceConn.SetWriteDeadline(time.Now().Add(10 * time.Second))
			err = deviceConn.WriteMessage(msgType, data)
			deviceWriteMu.Unlock()
			if err != nil {
				return
			}
		}
	}()

	// 等待任一方断开
	select {
	case <-deviceDone:
	case <-clientDone:
	case <-pingDone:
	case <-deviceHeartDone:
	}

	// 停止信令超时检测
	signalingTimer.Stop()

	// 清理 UDP 会话
	if p.registry != nil && sessionUfrag != "" {
		p.registry.RemoveByUfrag(sessionUfrag)
	}

	log.Printf("[投屏代理] 代理断开: 用户 %s → 坑位 %d", username, indexNum)
}

// findContainerIndex 从结构化缓存中查找容器名对应的坑位号
func (p *ProjectionProxy) findContainerIndex(containerName string) int {
	p.hub.containerMu.RLock()
	parsed := p.hub.parsedContainers
	p.hub.containerMu.RUnlock()

	for _, c := range parsed {
		if c.Name == containerName {
			return c.IndexNum
		}
	}
	return -1
}

// splitProjectionToken 分离投屏 token 格式：jwt_token:container_name
func splitProjectionToken(raw string) (token, containerName string) {
	idx := strings.LastIndex(raw, ":")
	if idx <= 0 || idx >= len(raw)-1 {
		return "", ""
	}
	return raw[:idx], raw[idx+1:]
}

// extractUfragFromOffer 从信令消息中提取 SDP offer 的 ice-ufrag
func extractUfragFromOffer(data []byte) string {
	var msg struct {
		ID   string `json:"id"`
		Data string `json:"data"`
	}
	if err := json.Unmarshal(data, &msg); err != nil || msg.ID != "offer" {
		return ""
	}

	decoded, err := base64.StdEncoding.DecodeString(msg.Data)
	if err != nil {
		return ""
	}

	sdp := string(decoded)
	var sdpWrap struct {
		SDP string `json:"sdp"`
	}
	if json.Unmarshal(decoded, &sdpWrap) == nil && sdpWrap.SDP != "" {
		sdp = sdpWrap.SDP
	}

	for _, line := range strings.Split(sdp, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "a=ice-ufrag:") {
			return strings.TrimPrefix(line, "a=ice-ufrag:")
		}
	}
	return ""
}
