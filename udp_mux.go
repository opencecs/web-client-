package main

import (
	"encoding/binary"
	"log"
	"net"
	"strings"
	"sync"
	"sync/atomic"
	"time"
)

// MuxSession 单个投屏 UDP 会话
type MuxSession struct {
	ContainerUDPPort int          // 容器 UDP 端口，如 30008
	ContainerAddr    *net.UDPAddr // 127.0.0.1:<port>
	BrowserAddr      *net.UDPAddr // 首次 STUN 匹配后填充
	Ufrag            string       // 容器 SDP 中的 ice-ufrag
	CreatedAt        time.Time
	lastActivity     atomic.Int64 // unix nano，原子操作避免竞态
	LocalConn        *net.UDPConn // 会话专属本地 UDP socket（与容器通信）
	MuxConn          *net.UDPConn // 主 Mux conn 引用（向浏览器回包）
	closed           bool
}

// touch 更新最后活跃时间
func (s *MuxSession) touch() {
	s.lastActivity.Store(time.Now().UnixNano())
}

// lastActive 获取最后活跃时间
func (s *MuxSession) lastActive() time.Time {
	ns := s.lastActivity.Load()
	if ns == 0 {
		return s.CreatedAt
	}
	return time.Unix(0, ns)
}

// SessionRegistry 管理 ufrag → 会话 的映射（支持同一容器多会话）
type SessionRegistry struct {
	mu              sync.RWMutex
	ufragToSession  map[string]*MuxSession   // ice-ufrag → 会话（STUN 慢路径）
	addrToSession   map[string]*MuxSession   // 浏览器地址 → 会话（快路径）
	portToSessions  map[int][]*MuxSession    // 容器 UDP 端口 → 多个会话
}

func NewSessionRegistry() *SessionRegistry {
	return &SessionRegistry{
		ufragToSession: make(map[string]*MuxSession),
		addrToSession:  make(map[string]*MuxSession),
		portToSessions: make(map[int][]*MuxSession),
	}
}

// Register 注册新会话，为每个会话创建独立的本地 UDP socket
func (r *SessionRegistry) Register(ufrag string, containerUDPPort int, muxConn *net.UDPConn) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 如果相同 ufrag 已存在，先清理
	if old, ok := r.ufragToSession[ufrag]; ok {
		r.removeSessionLocked(old)
	}

	// 创建独立的本地 UDP socket（随机端口，与容器通信）
	localConn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: 0})
	if err != nil {
		log.Printf("[UDP Mux] 创建本地 socket 失败: %v", err)
		return
	}
	localConn.SetReadBuffer(512 * 1024)

	session := &MuxSession{
		ContainerUDPPort: containerUDPPort,
		ContainerAddr:    &net.UDPAddr{IP: net.IPv4(127, 0, 0, 1), Port: containerUDPPort},
		Ufrag:            ufrag,
		CreatedAt:        time.Now(),
		LocalConn:        localConn,
		MuxConn:          muxConn,
	}
	session.touch()
	r.ufragToSession[ufrag] = session
	r.portToSessions[containerUDPPort] = append(r.portToSessions[containerUDPPort], session)

	localPort := localConn.LocalAddr().(*net.UDPAddr).Port
	log.Printf("[UDP Mux] 注册会话: ufrag=%s → 容器端口 %d (本地端口 %d)", ufrag, containerUDPPort, localPort)

	// 启动容器回包读协程：容器 → LocalConn → MuxConn → 浏览器
	go func() {
		buf := make([]byte, 1500)
		for {
			n, _, err := localConn.ReadFromUDP(buf)
			if err != nil {
				if !session.closed {
					log.Printf("[UDP Mux] 本地 socket 读取结束: ufrag=%s", ufrag)
				}
				return
			}
			r.mu.RLock()
			browserAddr := session.BrowserAddr
			r.mu.RUnlock()
			if browserAddr != nil {
				muxConn.WriteToUDP(buf[:n], browserAddr)
				session.touch()
			}
		}
	}()
}

// RemoveByUfrag 移除会话并关闭其本地 socket
func (r *SessionRegistry) RemoveByUfrag(ufrag string) {
	if ufrag == "" {
		return
	}
	r.mu.Lock()
	defer r.mu.Unlock()

	s, ok := r.ufragToSession[ufrag]
	if !ok {
		return
	}
	r.removeSessionLocked(s)
}

// removeSessionLocked 内部清理（调用时已持有写锁）
func (r *SessionRegistry) removeSessionLocked(s *MuxSession) {
	if s.BrowserAddr != nil {
		delete(r.addrToSession, s.BrowserAddr.String())
	}
	delete(r.ufragToSession, s.Ufrag)

	// 从 portToSessions 列表中移除
	sessions := r.portToSessions[s.ContainerUDPPort]
	for i, ss := range sessions {
		if ss == s {
			r.portToSessions[s.ContainerUDPPort] = append(sessions[:i], sessions[i+1:]...)
			break
		}
	}
	if len(r.portToSessions[s.ContainerUDPPort]) == 0 {
		delete(r.portToSessions, s.ContainerUDPPort)
	}

	// 关闭本地 socket
	s.closed = true
	if s.LocalConn != nil {
		s.LocalConn.Close()
	}
	log.Printf("[UDP Mux] 移除会话: ufrag=%s port=%d", s.Ufrag, s.ContainerUDPPort)
}

// LookupByAddr 根据浏览器地址查找会话（快路径）
func (r *SessionRegistry) LookupByAddr(addr string) *MuxSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.addrToSession[addr]
}

// LookupByUfrag 根据 ufrag 查找会话（STUN 慢路径）
func (r *SessionRegistry) LookupByUfrag(ufrag string) *MuxSession {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.ufragToSession[ufrag]
}

// BindBrowserAddr 绑定浏览器地址到会话
func (r *SessionRegistry) BindBrowserAddr(addr *net.UDPAddr, session *MuxSession) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// 清理旧绑定
	if session.BrowserAddr != nil {
		delete(r.addrToSession, session.BrowserAddr.String())
	}
	session.BrowserAddr = addr
	r.addrToSession[addr.String()] = session
	log.Printf("[UDP Mux] 绑定浏览器 %s → 容器端口 %d (ufrag=%s)", addr.String(), session.ContainerUDPPort, session.Ufrag)
}

// cleanupStale 清理过期会话
func (r *SessionRegistry) cleanupStale(maxAge time.Duration) {
	r.mu.Lock()
	defer r.mu.Unlock()

	now := time.Now()
	for ufrag, s := range r.ufragToSession {
		if now.Sub(s.lastActive()) > maxAge {
			log.Printf("[UDP Mux] 过期清理: ufrag=%s port=%d", ufrag, s.ContainerUDPPort)
			r.removeSessionLocked(s)
		}
	}
}

// StartUDPMux 启动 UDP 复用器，监听指定端口
func StartUDPMux(port int, registry *SessionRegistry) error {
	addr := &net.UDPAddr{Port: port, IP: net.IPv4zero}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	conn.SetReadBuffer(2 * 1024 * 1024)

	// 保存全局引用，供 Register 使用
	muxConn = conn

	log.Printf("[UDP Mux] 监听 UDP :%d（多会话模式）", port)

	// 过期清理协程
	go func() {
		ticker := time.NewTicker(30 * time.Second)
		defer ticker.Stop()
		for range ticker.C {
			registry.cleanupStale(2 * time.Minute)
		}
	}()

	// 收包主循环（只处理浏览器 → 容器方向）
	go func() {
		buf := make([]byte, 1500)
		for {
			n, remoteAddr, err := conn.ReadFromUDP(buf)
			if err != nil {
				log.Printf("[UDP Mux] 读取错误: %v", err)
				continue
			}
			packet := buf[:n]

			// 已知浏览器地址 → 快路径直接转发
			session := registry.LookupByAddr(remoteAddr.String())
			if session != nil {
				session.LocalConn.WriteToUDP(packet, session.ContainerAddr)
				session.touch()
				continue
			}

			// 未知地址 → 尝试 STUN 解析
			if isSTUNMessage(packet) {
				ufrag := parseSTUNUsername(packet)
				if ufrag != "" {
					session = registry.LookupByUfrag(ufrag)
					if session != nil {
						registry.BindBrowserAddr(remoteAddr, session)
						session.LocalConn.WriteToUDP(packet, session.ContainerAddr)
						session.touch()
						continue
					}
				}
			}

			// 未匹配 → 静默丢弃
		}
	}()

	return nil
}

// muxConn 全局 UDP 连接引用，供 Register 创建会话时使用
var muxConn *net.UDPConn

// GetMuxConn 获取主 Mux UDP 连接
func GetMuxConn() *net.UDPConn {
	return muxConn
}

// isSTUNMessage 判断是否为 STUN 消息（RFC 5389）
func isSTUNMessage(data []byte) bool {
	if len(data) < 20 {
		return false
	}
	// 前两位必须为 0
	if data[0]&0xC0 != 0 {
		return false
	}
	// Magic Cookie: 0x2112A442
	return data[4] == 0x21 && data[5] == 0x12 && data[6] == 0xA4 && data[7] == 0x42
}

// parseSTUNUsername 从 STUN 消息中提取 USERNAME 属性的 ufrag 部分
// USERNAME 格式：remote_ufrag:local_ufrag，取冒号前部分
func parseSTUNUsername(data []byte) string {
	if len(data) < 20 {
		return ""
	}
	msgLen := int(binary.BigEndian.Uint16(data[2:4]))
	offset := 20
	end := 20 + msgLen
	if end > len(data) {
		end = len(data)
	}

	for offset+4 <= end {
		attrType := binary.BigEndian.Uint16(data[offset:])
		attrLen := int(binary.BigEndian.Uint16(data[offset+2:]))
		if offset+4+attrLen > len(data) {
			break
		}
		if attrType == 0x0006 { // USERNAME
			username := string(data[offset+4 : offset+4+attrLen])
			parts := strings.SplitN(username, ":", 2)
			if len(parts) >= 1 {
				return parts[0]
			}
		}
		// 属性按 4 字节对齐
		offset += 4 + ((attrLen + 3) &^ 3)
	}
	return ""
}
