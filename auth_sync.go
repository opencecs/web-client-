package main

import (
	"crypto/md5"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"sort"
	"strings"
	"sync"
	"time"
)

const mytLoginURL = "https://moyunteng.com/api/sp_api.php"
var mytSignSalt = func() string {
	if s := os.Getenv("MYT_SIGN_SALT"); s != "" {
		return s
	}
	return "454&*&*fsdff"
}()

type MytAuthService struct {
	db            *sql.DB
	deviceAddr    string
	deviceService *DeviceService
	mu            sync.RWMutex
	username   string
	password   string // 明文密码（内存中）
	token      string
	uname      string
	lastSync   time.Time
	autoSync   bool
	ticker     *time.Ticker
	stopChan   chan struct{}
	// 绑定状态缓存
	bindStatus   int    // 0=未绑定, 1=已绑定, 2=他人绑定
	bindDeviceID string
	// 坑位授权缓存
	slotCache   map[string]interface{}
	slotCacheAt time.Time
}

func NewMytAuthService(db *sql.DB, deviceAddr string, deviceService *DeviceService) *MytAuthService {
	// 创建表
	db.Exec(`CREATE TABLE IF NOT EXISTS myt_auth (
		id INTEGER PRIMARY KEY CHECK (id = 1),
		username TEXT NOT NULL DEFAULT '',
		password TEXT NOT NULL DEFAULT '',
		auto_sync BOOLEAN DEFAULT 1,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	s := &MytAuthService{
		db:            db,
		deviceAddr:    deviceAddr,
		deviceService: deviceService,
		autoSync:      true,
		stopChan:      make(chan struct{}),
	}

	// 恢复已保存的凭证
	s.loadCredentials()

	return s
}

// 从数据库加载凭证，如果有则自动启动定时器
func (s *MytAuthService) loadCredentials() {
	var username, encPassword string
	var autoSync bool
	err := s.db.QueryRow("SELECT username, password, auto_sync FROM myt_auth WHERE id = 1").Scan(&username, &encPassword, &autoSync)
	if err != nil || username == "" {
		return
	}
	// 解密密码
	password, err := aesDecryptStr(jwtSecret[:32], encPassword)
	if err != nil {
		// 兼容旧明文密码：如果解密失败，尝试当作明文使用
		password = encPassword
		log.Printf("[MytAuth] 密码解密失败，尝试明文兼容模式")
	}
	s.mu.Lock()
	s.username = username
	s.password = password
	s.autoSync = autoSync
	s.mu.Unlock()

	// 启动时自动登录同步一次
	go func() {
		if err := s.loginAndSync(); err != nil {
			log.Printf("[MytAuth] 启动时自动登录失败: %v", err)
		}
		if autoSync {
			s.startTimer()
		}
	}()
}

// 保存凭证到数据库（密码加密存储）
func (s *MytAuthService) saveCredentials() {
	encPwd, err := aesEncrypt(jwtSecret[:32], []byte(s.password))
	if err != nil {
		log.Printf("[MytAuth] 密码加密失败: %v", err)
		return
	}
	s.db.Exec(`INSERT INTO myt_auth (id, username, password, auto_sync, updated_at)
		VALUES (1, ?, ?, ?, CURRENT_TIMESTAMP)
		ON CONFLICT(id) DO UPDATE SET username=?, password=?, auto_sync=?, updated_at=CURRENT_TIMESTAMP`,
		s.username, encPwd, s.autoSync,
		s.username, encPwd, s.autoSync)
}

// MD5 计算
func md5Hex(s string) string {
	h := md5.Sum([]byte(s))
	return fmt.Sprintf("%x", h)
}

// 调用魔云腾登录接口
func (s *MytAuthService) mytLogin(username, password string) (string, string, error) {
	pwdMD5 := md5Hex(password)
	ts := fmt.Sprintf("%d", time.Now().UnixMilli())

	// 签名计算：key 按字母排序 _ts, password, username
	params := map[string]string{
		"_ts":      ts,
		"password": pwdMD5,
		"username": username,
	}
	keys := make([]string, 0, len(params))
	for k := range params {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	vals := make([]string, 0, len(keys))
	for _, k := range keys {
		vals = append(vals, params[k])
	}
	signStr := strings.Join(vals, "#") + "#" + mytSignSalt
	sign := md5Hex(signStr)

	// 构造请求数据
	data := map[string]interface{}{
		"uname": username,
		"pwd":   pwdMD5,
		"_ts":   ts,
		"_sign": sign,
	}
	dataJSON, _ := json.Marshal(data)

	formData := url.Values{}
	formData.Set("type", "login")
	formData.Set("data", string(dataJSON))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(mytLoginURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return "", "", fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	var result struct {
		Code json.Number `json:"code"`
		Data struct {
			Token string `json:"token"`
			Uname string `json:"uname"`
			UID   string `json:"uid"`
		} `json:"data"`
		Msg string `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", "", fmt.Errorf("解析响应失败: %v", err)
	}
	code, _ := result.Code.Int64()
	if code != 200 {
		return "", "", fmt.Errorf("登录失败: %s", result.Msg)
	}

	return result.Data.Token, result.Data.Uname, nil
}

// UDP 同步 token 到设备
func (s *MytAuthService) syncTokenToDevice(token string) error {
	host, _, _ := net.SplitHostPort(s.deviceAddr)
	if host == "" {
		host = s.deviceAddr
	}

	conn, err := net.ListenUDP("udp", &net.UDPAddr{IP: net.IPv4zero, Port: 0})
	if err != nil {
		return fmt.Errorf("udp error: %v", err)
	}
	defer conn.Close()

	msg := []byte("lgtoken:" + token)
	_, err = conn.WriteToUDP(msg, &net.UDPAddr{
		IP:   net.ParseIP(host),
		Port: 7678,
	})
	if err != nil {
		return fmt.Errorf("send failed: %v", err)
	}
	return nil
}

// 登录并同步
func (s *MytAuthService) loginAndSync() error {
	s.mu.RLock()
	username := s.username
	password := s.password
	s.mu.RUnlock()

	if username == "" || password == "" {
		return fmt.Errorf("未配置账号")
	}

	token, uname, err := s.mytLogin(username, password)
	if err != nil {
		return err
	}

	s.mu.Lock()
	s.token = token
	s.uname = uname
	s.mu.Unlock()

	if err := s.syncTokenToDevice(token); err != nil {
		return err
	}

	s.mu.Lock()
	s.lastSync = time.Now()
	s.mu.Unlock()

	// 同步后刷新绑定状态
	go s.refreshBindStatus()

	log.Printf("[MytAuth] 登录同步成功: %s", uname)
	return nil
}

// 启动定时器
func (s *MytAuthService) startTimer() {
	s.stopTimer()
	s.ticker = time.NewTicker(10 * time.Minute)
	s.stopChan = make(chan struct{})
	go func() {
		for {
			select {
			case <-s.ticker.C:
				if err := s.loginAndSync(); err != nil {
					log.Printf("[MytAuth] 自动同步失败: %v", err)
				}
			case <-s.stopChan:
				return
			}
		}
	}()
	log.Println("[MytAuth] 自动同步定时器已启动 (每10分钟)")
}

// 停止定时器
func (s *MytAuthService) stopTimer() {
	if s.ticker != nil {
		s.ticker.Stop()
		close(s.stopChan)
		s.ticker = nil
	}
}

// --- HTTP Handlers ---

// POST /api/myt/login
func (s *MytAuthService) HandleMytLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, "请输入账号和密码", 400)
		return
	}

	// 先尝试登录
	token, uname, err := s.mytLogin(req.Username, req.Password)
	if err != nil {
		jsonError(w, err.Error(), 401)
		return
	}

	// 保存凭证
	s.mu.Lock()
	s.username = req.Username
	s.password = req.Password
	s.token = token
	s.uname = uname
	s.mu.Unlock()
	s.saveCredentials()

	// 同步到设备
	if err := s.syncTokenToDevice(token); err != nil {
		log.Printf("[MytAuth] 同步失败: %v", err)
	}
	s.mu.Lock()
	s.lastSync = time.Now()
	autoSync := s.autoSync
	s.mu.Unlock()

	// 启动定时器
	if autoSync {
		s.startTimer()
	}

	// 登录后自动查询绑定状态
	go s.refreshBindStatus()

	jsonResponse(w, map[string]interface{}{
		"ok":       true,
		"uname":    uname,
		"lastSync": s.lastSync.Format("2006-01-02 15:04:05"),
	})
}

// POST /api/myt/sync
func (s *MytAuthService) HandleMytSync(w http.ResponseWriter, r *http.Request) {
	if err := s.loginAndSync(); err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	// 同步后刷新绑定状态
	go s.refreshBindStatus()

	s.mu.RLock()
	lastSync := s.lastSync
	s.mu.RUnlock()
	jsonResponse(w, map[string]interface{}{
		"ok":       true,
		"lastSync": lastSync.Format("2006-01-02 15:04:05"),
	})
}

// GET /api/myt/status
func (s *MytAuthService) HandleMytStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lastSync := ""
	if !s.lastSync.IsZero() {
		lastSync = s.lastSync.Format("2006-01-02 15:04:05")
	}

	jsonResponse(w, map[string]interface{}{
		"loggedIn":     s.username != "",
		"username":     s.username,
		"uname":        s.uname,
		"hasToken":     s.token != "",
		"lastSync":     lastSync,
		"autoSync":     s.autoSync,
		"bindStatus":   s.bindStatus,
		"bindDeviceID": s.bindDeviceID,
	})
}

// POST /api/myt/logout
func (s *MytAuthService) HandleMytLogout(w http.ResponseWriter, r *http.Request) {
	s.stopTimer()
	s.mu.Lock()
	s.username = ""
	s.password = ""
	s.token = ""
	s.uname = ""
	s.lastSync = time.Time{}
	s.autoSync = true
	s.bindStatus = 0
	s.bindDeviceID = ""
	s.mu.Unlock()

	s.db.Exec("DELETE FROM myt_auth WHERE id = 1")

	jsonResponse(w, map[string]interface{}{"ok": true})
}

// POST /api/myt/auto
func (s *MytAuthService) HandleMytAutoToggle(w http.ResponseWriter, r *http.Request) {
	var req struct {
		AutoSync bool `json:"autoSync"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	s.mu.Lock()
	s.autoSync = req.AutoSync
	s.mu.Unlock()
	s.saveCredentials()

	if req.AutoSync {
		s.startTimer()
	} else {
		s.stopTimer()
	}

	jsonResponse(w, map[string]interface{}{"ok": true, "autoSync": req.AutoSync})
}

// --- 通用魔云腾 API 调用 ---

const mytAPIURL = "https://www.moyunteng.com/api/api.php"

func (s *MytAuthService) mytAPICall(apiType string, data interface{}) (json.RawMessage, error) {
	dataJSON, _ := json.Marshal(data)
	formData := url.Values{}
	formData.Set("type", apiType)
	formData.Set("data", string(dataJSON))

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Post(mytAPIURL, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	var result struct {
		Code json.Number     `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败: %v", err)
	}
	code, _ := result.Code.Int64()
	if code != 200 {
		return nil, fmt.Errorf("%s", result.Msg)
	}
	return result.Data, nil
}

func (s *MytAuthService) getDeviceID() string {
	return s.deviceService.GetDeviceID()
}

// 查询绑定状态并缓存
func (s *MytAuthService) refreshBindStatus() {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		return
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		return
	}

	hostJSON, _ := json.Marshal(map[string]interface{}{"host": []string{deviceID}})
	data := map[string]interface{}{
		"act":   "get",
		"data":  string(hostJSON),
		"token": token,
	}

	respData, err := s.mytAPICall("user_host_oper", data)
	if err != nil {
		log.Printf("[MytAuth] 查询绑定状态失败: %v", err)
		return
	}

	var statusMap map[string]int
	if err := json.Unmarshal(respData, &statusMap); err != nil {
		log.Printf("[MytAuth] 解析绑定状态失败: %v", err)
		return
	}

	s.mu.Lock()
	s.bindStatus = statusMap[deviceID]
	s.bindDeviceID = deviceID
	s.mu.Unlock()

	log.Printf("[MytAuth] 绑定状态已更新: deviceId=%s, status=%d", deviceID, statusMap[deviceID])
}

// GET /api/myt/bind-status - 返回缓存的绑定状态
func (s *MytAuthService) HandleMytBindStatus(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	token := s.token
	bindStatus := s.bindStatus
	bindDeviceID := s.bindDeviceID
	s.mu.RUnlock()

	if token == "" {
		jsonError(w, "未登录", 401)
		return
	}

	// 如果没有缓存，实时查询一次
	if bindDeviceID == "" {
		deviceID := s.getDeviceID()
		if deviceID == "" {
			jsonError(w, "无法获取设备ID", 500)
			return
		}

		hostJSON, _ := json.Marshal(map[string]interface{}{"host": []string{deviceID}})
		data := map[string]interface{}{
			"act":   "get",
			"data":  string(hostJSON),
			"token": token,
		}

		respData, err := s.mytAPICall("user_host_oper", data)
		if err != nil {
			jsonError(w, err.Error(), 500)
			return
		}

		var statusMap map[string]int
		if err := json.Unmarshal(respData, &statusMap); err != nil {
			jsonError(w, "解析绑定状态失败", 500)
			return
		}

		bindStatus = statusMap[deviceID]
		bindDeviceID = deviceID

		// 缓存
		s.mu.Lock()
		s.bindStatus = bindStatus
		s.bindDeviceID = bindDeviceID
		s.mu.Unlock()
	}

	statusText := "未绑定"
	switch bindStatus {
	case 1:
		statusText = "已绑定"
	case 2:
		statusText = "他人绑定"
	}

	jsonResponse(w, map[string]interface{}{
		"deviceId":   bindDeviceID,
		"bindStatus": bindStatus,
		"statusText": statusText,
	})
}

// POST /api/myt/bind
func (s *MytAuthService) HandleMytBind(w http.ResponseWriter, r *http.Request) {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		jsonError(w, "未登录", 401)
		return
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		jsonError(w, "无法获取设备ID", 500)
		return
	}

	hostJSON, _ := json.Marshal(map[string]interface{}{"host": []string{deviceID}})
	data := map[string]interface{}{
		"act":   "batchBind",
		"data":  string(hostJSON),
		"token": token,
	}

	_, err := s.mytAPICall("user_host_oper", data)
	if err != nil {
		jsonError(w, "绑定失败: "+err.Error(), 500)
		return
	}

	// 绑定成功，更新缓存
	s.mu.Lock()
	s.bindStatus = 1
	s.bindDeviceID = deviceID
	s.mu.Unlock()

	jsonResponse(w, map[string]interface{}{"ok": true, "deviceId": deviceID})
}

// POST /api/myt/vcode
func (s *MytAuthService) HandleMytGetVCode(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Phone string `json:"phone"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		jsonError(w, "未登录", 401)
		return
	}

	data := map[string]interface{}{
		"act":   "com",
		"phone": req.Phone,
		"plat":  "",
		"token": token,
	}

	respData, err := s.mytAPICall("get_phone_vcode", data)
	if err != nil {
		jsonError(w, "获取验证码失败: "+err.Error(), 500)
		return
	}

	var vkeyResp struct {
		Vkey string `json:"vkey"`
	}
	json.Unmarshal(respData, &vkeyResp)

	jsonResponse(w, map[string]interface{}{"ok": true, "vkey": vkeyResp.Vkey})
}

// GET /api/myt/slot-states - 查询坑位授权到期状态（无需登录魔云腾）
func (s *MytAuthService) HandleSlotStates(w http.ResponseWriter, r *http.Request) {
	// 检查缓存（24小时有效）
	s.mu.RLock()
	cached := s.slotCache
	cacheValid := s.slotCache != nil && time.Since(s.slotCacheAt) < 24*time.Hour
	s.mu.RUnlock()

	if cacheValid {
		jsonResponse(w, map[string]interface{}{"slots": cached})
		return
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		jsonError(w, "无法获取设备ID", 500)
		return
	}

	// 按旧客户端方式构造请求：host 为 JSON 数组字符串
	hostJSON, _ := json.Marshal([]string{deviceID})
	hostStr := string(hostJSON)
	tsInt := time.Now().Unix()
	ts := fmt.Sprintf("%d", tsInt)

	// 签名：key 排序 (_ts, host)，值用 # 拼接，盐值不同于登录接口
	const slotSignSalt = "@#1234A98413G=--..234"
	signStr := ts + "#" + hostStr + "#" + slotSignSalt
	sign := md5Hex(signStr)

	dataMap := map[string]interface{}{
		"host":  hostStr,
		"_ts":   tsInt,
		"_sign": sign,
	}
	dataJSON, _ := json.Marshal(dataMap)

	formData := url.Values{}
	formData.Set("type", "term_info_nologn")
	formData.Set("data", string(dataJSON))

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", mytAPIURL, strings.NewReader(formData.Encode()))
	if err != nil {
		jsonError(w, "创建请求失败", 500)
		return
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		jsonError(w, "请求失败: "+err.Error(), 500)
		return
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	var result struct {
		Code json.Number     `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		jsonError(w, "解析响应失败", 500)
		return
	}
	code, _ := result.Code.Int64()
	if code != 200 {
		jsonError(w, "查询失败: "+result.Msg, 500)
		return
	}

	// respData 格式: [{ "child": { "1": "1735689600", "2": "0", ... }, ... }]
	var list []struct {
		Child map[string]interface{} `json:"child"`
	}
	if err := json.Unmarshal(result.Data, &list); err != nil {
		jsonResponse(w, map[string]interface{}{"slots": map[string]interface{}{}})
		return
	}

	slots := map[string]interface{}{}
	if len(list) > 0 && list[0].Child != nil {
		now := time.Now().Unix()
		warnThreshold := int64(3 * 24 * 3600) // 3天
		for slot, expireVal := range list[0].Child {
			var expireTs int64
			switch v := expireVal.(type) {
			case string:
				fmt.Sscanf(v, "%d", &expireTs)
			case float64:
				expireTs = int64(v)
			}

			var state int // 0=正常有效, 1=即将到期(3天内), 2=已到期
			if expireTs == 0 || expireTs < now {
				state = 2
			} else if expireTs-now < warnThreshold {
				state = 1
			} else {
				state = 0
			}
			slots[slot] = map[string]interface{}{
				"state":    state,
				"expireTs": expireTs,
			}
		}
	}

	// 更新缓存
	s.mu.Lock()
	s.slotCache = slots
	s.slotCacheAt = time.Now()
	s.mu.Unlock()

	log.Printf("[MytAuth] 坑位授权状态已缓存，共 %d 个坑位", len(slots))
	jsonResponse(w, map[string]interface{}{"slots": slots})
}

// ===== WS 可用方法 =====

// GetStatus 返回当前登录状态
func (s *MytAuthService) GetStatus() map[string]interface{} {
	s.mu.RLock()
	defer s.mu.RUnlock()

	lastSync := ""
	if !s.lastSync.IsZero() {
		lastSync = s.lastSync.Format("2006-01-02 15:04:05")
	}

	return map[string]interface{}{
		"loggedIn":     s.username != "",
		"username":     s.username,
		"uname":        s.uname,
		"hasToken":     s.token != "",
		"lastSync":     lastSync,
		"autoSync":     s.autoSync,
		"bindStatus":   s.bindStatus,
		"bindDeviceID": s.bindDeviceID,
	}
}

// DoLogin 登录并同步
func (s *MytAuthService) DoLogin(username, password string) (map[string]interface{}, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("请输入账号和密码")
	}

	token, uname, err := s.mytLogin(username, password)
	if err != nil {
		return nil, err
	}

	s.mu.Lock()
	s.username = username
	s.password = password
	s.token = token
	s.uname = uname
	s.mu.Unlock()
	s.saveCredentials()

	if err := s.syncTokenToDevice(token); err != nil {
		log.Printf("[MytAuth] 同步失败: %v", err)
	}
	s.mu.Lock()
	s.lastSync = time.Now()
	autoSync := s.autoSync
	s.mu.Unlock()

	if autoSync {
		s.startTimer()
	}

	go s.refreshBindStatus()

	return map[string]interface{}{
		"ok":       true,
		"uname":    uname,
		"lastSync": s.lastSync.Format("2006-01-02 15:04:05"),
	}, nil
}

// DoLogout 登出
func (s *MytAuthService) DoLogout() {
	s.stopTimer()
	s.mu.Lock()
	s.username = ""
	s.password = ""
	s.token = ""
	s.uname = ""
	s.lastSync = time.Time{}
	s.autoSync = true
	s.bindStatus = 0
	s.bindDeviceID = ""
	s.mu.Unlock()
	s.db.Exec("DELETE FROM myt_auth WHERE id = 1")
}

// DoSync 手动同步
func (s *MytAuthService) DoSync() (map[string]interface{}, error) {
	if err := s.loginAndSync(); err != nil {
		return nil, err
	}
	go s.refreshBindStatus()
	s.mu.RLock()
	lastSync := s.lastSync
	s.mu.RUnlock()
	return map[string]interface{}{
		"ok":       true,
		"lastSync": lastSync.Format("2006-01-02 15:04:05"),
	}, nil
}

// DoAutoToggle 切换自动同步
func (s *MytAuthService) DoAutoToggle(autoSync bool) (map[string]interface{}, error) {
	s.mu.Lock()
	s.autoSync = autoSync
	s.mu.Unlock()
	s.saveCredentials()

	if autoSync {
		s.startTimer()
	} else {
		s.stopTimer()
	}
	return map[string]interface{}{"ok": true, "autoSync": autoSync}, nil
}

// GetSlotStates 获取坑位授权状态
func (s *MytAuthService) GetSlotStates() (map[string]interface{}, error) {
	// 检查缓存（24小时有效）
	s.mu.RLock()
	cached := s.slotCache
	cacheValid := s.slotCache != nil && time.Since(s.slotCacheAt) < 24*time.Hour
	s.mu.RUnlock()

	if cacheValid {
		return map[string]interface{}{"slots": cached}, nil
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		return nil, fmt.Errorf("无法获取设备ID")
	}

	hostJSON, _ := json.Marshal([]string{deviceID})
	hostStr := string(hostJSON)
	tsInt := time.Now().Unix()
	ts := fmt.Sprintf("%d", tsInt)

	const slotSignSalt = "@#1234A98413G=--..234"
	signStr := ts + "#" + hostStr + "#" + slotSignSalt
	sign := md5Hex(signStr)

	dataMap := map[string]interface{}{
		"host":  hostStr,
		"_ts":   tsInt,
		"_sign": sign,
	}
	dataJSON, _ := json.Marshal(dataMap)

	formData := url.Values{}
	formData.Set("type", "term_info_nologn")
	formData.Set("data", string(dataJSON))

	client := &http.Client{Timeout: 15 * time.Second}
	req, err := http.NewRequest("POST", mytAPIURL, strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, fmt.Errorf("创建请求失败")
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("请求失败: %v", err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	var result struct {
		Code json.Number     `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, fmt.Errorf("解析响应失败")
	}
	code, _ := result.Code.Int64()
	if code != 200 {
		return nil, fmt.Errorf("查询失败: %s", result.Msg)
	}

	var list []struct {
		Child map[string]interface{} `json:"child"`
	}
	if err := json.Unmarshal(result.Data, &list); err != nil {
		return map[string]interface{}{"slots": map[string]interface{}{}}, nil
	}

	slots := map[string]interface{}{}
	if len(list) > 0 && list[0].Child != nil {
		now := time.Now().Unix()
		warnThreshold := int64(3 * 24 * 3600)
		log.Printf("[MytAuth] 坑位原始数据 keys: %v", func() []string {
			keys := make([]string, 0, len(list[0].Child))
			for k := range list[0].Child {
				keys = append(keys, k)
			}
			return keys
		}())
		for slot, expireVal := range list[0].Child {
			var expireTs int64
			switch v := expireVal.(type) {
			case string:
				fmt.Sscanf(v, "%d", &expireTs)
			case float64:
				expireTs = int64(v)
			}

			var state int
			if expireTs == 0 || expireTs < now {
				state = 2
			} else if expireTs-now < warnThreshold {
				state = 1
			} else {
				state = 0
			}
			slots[slot] = map[string]interface{}{
				"state":    state,
				"expireTs": expireTs,
			}
		}
	}

	s.mu.Lock()
	s.slotCache = slots
	s.slotCacheAt = time.Now()
	s.mu.Unlock()

	log.Printf("[MytAuth] 坑位授权状态已缓存，共 %d 个坑位", len(slots))
	return map[string]interface{}{"slots": slots}, nil
}

// GetBindStatus 获取绑定状态
func (s *MytAuthService) GetBindStatus() (map[string]interface{}, error) {
	s.mu.RLock()
	token := s.token
	bindStatus := s.bindStatus
	bindDeviceID := s.bindDeviceID
	s.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("未登录")
	}

	if bindDeviceID == "" {
		deviceID := s.getDeviceID()
		if deviceID == "" {
			return nil, fmt.Errorf("无法获取设备ID")
		}

		hostJSON, _ := json.Marshal(map[string]interface{}{"host": []string{deviceID}})
		data := map[string]interface{}{
			"act":   "get",
			"data":  string(hostJSON),
			"token": token,
		}

		respData, err := s.mytAPICall("user_host_oper", data)
		if err != nil {
			return nil, err
		}

		var statusMap map[string]int
		if err := json.Unmarshal(respData, &statusMap); err != nil {
			return nil, fmt.Errorf("解析绑定状态失败")
		}

		bindStatus = statusMap[deviceID]
		bindDeviceID = deviceID

		s.mu.Lock()
		s.bindStatus = bindStatus
		s.bindDeviceID = bindDeviceID
		s.mu.Unlock()
	}

	statusText := "未绑定"
	switch bindStatus {
	case 1:
		statusText = "已绑定"
	case 2:
		statusText = "他人绑定"
	}

	return map[string]interface{}{
		"deviceId":   bindDeviceID,
		"bindStatus": bindStatus,
		"statusText": statusText,
	}, nil
}

// DoBind 绑定设备
func (s *MytAuthService) DoBind() (map[string]interface{}, error) {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("未登录")
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		return nil, fmt.Errorf("无法获取设备ID")
	}

	hostJSON, _ := json.Marshal(map[string]interface{}{"host": []string{deviceID}})
	data := map[string]interface{}{
		"act":   "batchBind",
		"data":  string(hostJSON),
		"token": token,
	}

	_, err := s.mytAPICall("user_host_oper", data)
	if err != nil {
		return nil, fmt.Errorf("绑定失败: %v", err)
	}

	s.mu.Lock()
	s.bindStatus = 1
	s.bindDeviceID = deviceID
	s.mu.Unlock()

	return map[string]interface{}{"ok": true, "deviceId": deviceID}, nil
}

// DoGetVCode 获取验证码
func (s *MytAuthService) DoGetVCode(phone string) (map[string]interface{}, error) {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		return nil, fmt.Errorf("未登录")
	}

	data := map[string]interface{}{
		"act":   "com",
		"phone": phone,
		"plat":  "",
		"token": token,
	}

	respData, err := s.mytAPICall("get_phone_vcode", data)
	if err != nil {
		return nil, fmt.Errorf("获取验证码失败: %v", err)
	}

	var vkeyResp struct {
		Vkey string `json:"vkey"`
	}
	json.Unmarshal(respData, &vkeyResp)

	return map[string]interface{}{"ok": true, "vkey": vkeyResp.Vkey}, nil
}

// DoUnbind 解绑设备
func (s *MytAuthService) DoUnbind(vcode, vkey string) error {
	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		return fmt.Errorf("未登录")
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		return fmt.Errorf("无法获取设备ID")
	}

	innerData, _ := json.Marshal(map[string]interface{}{
		"hlist": []string{deviceID},
		"vcode": vcode,
		"vkey":  vkey,
	})

	data := map[string]interface{}{
		"act":   "batchUnbind",
		"data":  string(innerData),
		"token": token,
	}

	_, err := s.mytAPICall("user_host_oper", data)
	if err != nil {
		return fmt.Errorf("解绑失败: %v", err)
	}

	s.mu.Lock()
	s.bindStatus = 0
	s.bindDeviceID = deviceID
	s.mu.Unlock()

	return nil
}

// POST /api/myt/unbind
func (s *MytAuthService) HandleMytUnbind(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Vcode string `json:"vcode"`
		Vkey  string `json:"vkey"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	s.mu.RLock()
	token := s.token
	s.mu.RUnlock()

	if token == "" {
		jsonError(w, "未登录", 401)
		return
	}

	deviceID := s.getDeviceID()
	if deviceID == "" {
		jsonError(w, "无法获取设备ID", 500)
		return
	}

	innerData, _ := json.Marshal(map[string]interface{}{
		"hlist": []string{deviceID},
		"vcode": req.Vcode,
		"vkey":  req.Vkey,
	})

	data := map[string]interface{}{
		"act":   "batchUnbind",
		"data":  string(innerData),
		"token": token,
	}

	_, err := s.mytAPICall("user_host_oper", data)
	if err != nil {
		jsonError(w, "解绑失败: "+err.Error(), 500)
		return
	}

	// 解绑成功，更新缓存
	s.mu.Lock()
	s.bindStatus = 0
	s.bindDeviceID = deviceID
	s.mu.Unlock()

	jsonResponse(w, map[string]interface{}{"ok": true})
}
