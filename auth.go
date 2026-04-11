package main

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/golang-jwt/jwt/v5"
	"golang.org/x/crypto/bcrypt"
	_ "modernc.org/sqlite"
)

type contextKey string

const userContextKey contextKey = "user"

var jwtSecret = func() []byte {
	// 从程序目录下的 .jwt_secret 文件读取，不存在则自动生成
	exePath, _ := os.Executable()
	secretFile := filepath.Join(filepath.Dir(exePath), ".jwt_secret")
	if data, err := os.ReadFile(secretFile); err == nil && len(data) >= 32 {
		return data
	}
	// 生成 32 字节随机密钥
	key := make([]byte, 32)
	io.ReadFull(rand.Reader, key)
	os.WriteFile(secretFile, key, 0600)
	log.Printf("[Auth] JWT 密钥已自动生成: %s", secretFile)
	return key
}()

type User struct {
	ID           int64      `json:"id"`
	Username     string     `json:"username"`
	PasswordHash string     `json:"-"`
	Role         string     `json:"role"`
	ExpiresAt    *time.Time `json:"expiresAt"`
	Enabled      bool       `json:"enabled"`
	CreatedAt    time.Time  `json:"createdAt"`
	UpdatedAt    time.Time  `json:"updatedAt"`
}

type AuthService struct {
	db              *sql.DB
	sessionKeys     sync.Map // username → []byte (AES session key)
	tokenInvalidAt  sync.Map // username → time.Time (该时间之前签发的 token 视为失效)
	wsHub           interface{ KickUser(string); KickUserWithReason(string, string) } // 延迟注入，避免循环依赖
}

// storeSessionKey 缓存用户的会话密钥
func (s *AuthService) storeSessionKey(username string, key []byte) {
	s.sessionKeys.Store(username, key)
}

// getSessionKey 获取用户的会话密钥
func (s *AuthService) getSessionKey(username string) []byte {
	if val, ok := s.sessionKeys.Load(username); ok {
		return val.([]byte)
	}
	return nil
}

func InitDB(path string) *sql.DB {
	db, err := sql.Open("sqlite", path)
	if err != nil {
		log.Fatal("Failed to open database:", err)
	}

	// 启用 WAL 模式：允许并发读写，写操作不阻塞读
	db.Exec("PRAGMA journal_mode=WAL")
	// 写操作遇到锁时等待 5 秒而非立即失败
	db.Exec("PRAGMA busy_timeout=5000")
	// SQLite 单写多读，限制连接数避免 SQLITE_BUSY
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(2)

	db.Exec(`CREATE TABLE IF NOT EXISTS users (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		username TEXT UNIQUE NOT NULL,
		password_hash TEXT NOT NULL,
		role TEXT NOT NULL DEFAULT 'user',
		expires_at DATETIME,
		enabled BOOLEAN DEFAULT 1,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	)`)

	db.Exec(`CREATE TABLE IF NOT EXISTS user_permissions (
		user_id INTEGER PRIMARY KEY,
		slots TEXT DEFAULT '',
		container_start BOOLEAN DEFAULT 0,
		container_restart BOOLEAN DEFAULT 0,
		container_reset BOOLEAN DEFAULT 0,
		container_delete BOOLEAN DEFAULT 0,
		container_rename BOOLEAN DEFAULT 0,
		container_copy BOOLEAN DEFAULT 0,
		container_create BOOLEAN DEFAULT 0,
		alias_manage BOOLEAN DEFAULT 0,
		backup_manage BOOLEAN DEFAULT 0,
		image_view BOOLEAN DEFAULT 0,
		projection BOOLEAN DEFAULT 0,
		terminal BOOLEAN DEFAULT 0,
		network_bridge BOOLEAN DEFAULT 0,
		vpc_manage BOOLEAN DEFAULT 0,
		FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE CASCADE
	)`)

	// 系统设置表
	db.Exec(`CREATE TABLE IF NOT EXISTS settings (
		key TEXT PRIMARY KEY,
		value TEXT NOT NULL DEFAULT ''
	)`)

	// Create default admin if not exists
	var count int
	db.QueryRow("SELECT COUNT(*) FROM users WHERE username = 'myt'").Scan(&count)
	if count == 0 {
		hash, _ := bcrypt.GenerateFromPassword([]byte("myt"), bcrypt.DefaultCost)
		db.Exec("INSERT INTO users (username, password_hash, role, enabled) VALUES (?, ?, 'admin', 1)", "myt", string(hash))
		log.Println("[Auth] ⚠ 默认管理员账户已创建: myt/myt —— 请立即登录后修改密码！")
	}

	return db
}

func NewAuthService(db *sql.DB) *AuthService {
	return &AuthService{db: db}
}

// --- JWT ---

type Claims struct {
	UserID   int64  `json:"uid"`
	Username string `json:"username"`
	Role     string `json:"role"`
	jwt.RegisteredClaims
}

func (s *AuthService) generateToken(user *User) (string, error) {
	claims := &Claims{
		UserID:   user.ID,
		Username: user.Username,
		Role:     user.Role,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(24 * time.Hour)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

func (s *AuthService) parseToken(tokenStr string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &Claims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	return token.Claims.(*Claims), nil
}

// --- 投屏专用 Token ---

// ProjectionClaims 投屏专用 JWT Claims（短期一次性）
type ProjectionClaims struct {
	UserID        int64  `json:"uid"`
	Username      string `json:"username"`
	Role          string `json:"role"`
	Purpose       string `json:"purpose"`
	ContainerName string `json:"container"`
	jwt.RegisteredClaims
}

// generateProjectionToken 生成投屏专用短期 token（60秒有效）
func (s *AuthService) generateProjectionToken(userID int64, username, role, containerName string) (string, error) {
	claims := &ProjectionClaims{
		UserID:        userID,
		Username:      username,
		Role:          role,
		Purpose:       "projection",
		ContainerName: containerName,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(time.Now().Add(60 * time.Second)),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
		},
	}
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	return token.SignedString(jwtSecret)
}

// parseProjectionToken 解析投屏专用 token
func (s *AuthService) parseProjectionToken(tokenStr string) (*ProjectionClaims, error) {
	token, err := jwt.ParseWithClaims(tokenStr, &ProjectionClaims{}, func(token *jwt.Token) (interface{}, error) {
		return jwtSecret, nil
	})
	if err != nil {
		return nil, err
	}
	claims := token.Claims.(*ProjectionClaims)
	if claims.Purpose != "projection" {
		return nil, fmt.Errorf("invalid token purpose")
	}
	return claims, nil
}

// --- Session Key 加密 ---

// generateSessionKey 生成 32 字节随机会话密钥
func generateSessionKey() ([]byte, error) {
	key := make([]byte, 32)
	if _, err := io.ReadFull(rand.Reader, key); err != nil {
		return nil, err
	}
	return key, nil
}

// aesEncrypt AES-GCM 加密，返回 base64(nonce+ciphertext)
func aesEncrypt(key []byte, plaintext []byte) (string, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// aesDecryptStr AES-GCM 解密，输入 base64(nonce+ciphertext)，返回明文字符串
func aesDecryptStr(key []byte, encrypted string) (string, error) {
	data, err := base64.StdEncoding.DecodeString(encrypted)
	if err != nil {
		return "", err
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	if len(data) < gcm.NonceSize() {
		return "", fmt.Errorf("ciphertext too short")
	}
	nonce, ciphertext := data[:gcm.NonceSize()], data[gcm.NonceSize():]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", err
	}
	return string(plaintext), nil
}

// --- Middleware ---

func (s *AuthService) JWTMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		tokenStr := ""

		// From header
		auth := r.Header.Get("Authorization")
		if strings.HasPrefix(auth, "Bearer ") {
			tokenStr = auth[7:]
		}
		// From query param (for WebSocket)
		if tokenStr == "" {
			tokenStr = r.URL.Query().Get("token")
		}

		if tokenStr == "" {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
			return
		}

		claims, err := s.parseToken(tokenStr)
		if err != nil {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		// 检查 token 是否因用户登出而失效
		if invalidAt, ok := s.tokenInvalidAt.Load(claims.Username); ok {
			if claims.IssuedAt != nil && claims.IssuedAt.Time.Before(invalidAt.(time.Time)) {
				http.Error(w, `{"error":"token revoked"}`, http.StatusUnauthorized)
				return
			}
		}

		// Check if user still enabled and not expired
		user := s.getUserByID(claims.UserID)
		if user == nil || !user.Enabled {
			http.Error(w, `{"error":"account disabled"}`, http.StatusForbidden)
			return
		}
		if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
			http.Error(w, `{"error":"account expired"}`, http.StatusForbidden)
			return
		}

		ctx := context.WithValue(r.Context(), userContextKey, claims)
		next.ServeHTTP(w, r.WithContext(ctx))
	})
}

func (s *AuthService) AdminOnly(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		claims := r.Context().Value(userContextKey).(*Claims)
		if claims.Role != "admin" {
			http.Error(w, `{"error":"admin required"}`, http.StatusForbidden)
			return
		}
		next.ServeHTTP(w, r)
	})
}

// --- Handlers ---

func (s *AuthService) HandleLogin(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username string `json:"username"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	user := s.getUserByName(req.Username)
	if user == nil {
		jsonError(w, "invalid credentials", 401)
		return
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		jsonError(w, "invalid credentials", 401)
		return
	}

	if !user.Enabled {
		jsonError(w, "account disabled", 403)
		return
	}
	if user.ExpiresAt != nil && user.ExpiresAt.Before(time.Now()) {
		jsonError(w, "account expired", 403)
		return
	}

	token, err := s.generateToken(user)
	if err != nil {
		jsonError(w, "token generation failed", 500)
		return
	}

	// 生成会话密钥用于 WS 加密通信
	sessionKey, err := generateSessionKey()
	if err != nil {
		jsonError(w, "session key generation failed", 500)
		return
	}

	// 缓存会话密钥（以用户名为 key，WS 连接时使用）
	s.storeSessionKey(user.Username, sessionKey)

	jsonResponse(w, map[string]interface{}{
		"token":       token,
		"role":        user.Role,
		"username":    user.Username,
		"session_key": base64.StdEncoding.EncodeToString(sessionKey),
	})
}

func (s *AuthService) HandleMe(w http.ResponseWriter, r *http.Request) {
	claims := r.Context().Value(userContextKey).(*Claims)
	resp := map[string]interface{}{
		"id":       claims.UserID,
		"username": claims.Username,
		"role":     claims.Role,
	}
	// 非 admin 用户返回权限信息
	if claims.Role != "admin" {
		resp["permissions"] = s.GetUserPermissions(claims.UserID)
	}
	jsonResponse(w, resp)
}

func (s *AuthService) HandleLogout(w http.ResponseWriter, r *http.Request) {
	// 从 header 提取 token，尽力失效，不强制要求有效
	tokenStr := ""
	if auth := r.Header.Get("Authorization"); strings.HasPrefix(auth, "Bearer ") {
		tokenStr = auth[7:]
	}
	if tokenStr != "" {
		if claims, err := s.parseToken(tokenStr); err == nil {
			s.tokenInvalidAt.Store(claims.Username, time.Now())
			s.sessionKeys.Delete(claims.Username)
			if s.wsHub != nil {
				s.wsHub.KickUserWithReason(claims.Username, "logout")
			}
			log.Printf("[Auth] 用户 '%s' 已登出", claims.Username)
		}
	}
	jsonResponse(w, map[string]interface{}{"ok": true})
}

func (s *AuthService) HandleListUsers(w http.ResponseWriter, r *http.Request) {
	rows, err := s.db.Query("SELECT id, username, role, expires_at, enabled, created_at, updated_at FROM users ORDER BY id")
	if err != nil {
		jsonError(w, err.Error(), 500)
		return
	}
	defer rows.Close()

	var users []User
	for rows.Next() {
		var u User
		var expiresAt sql.NullTime
		rows.Scan(&u.ID, &u.Username, &u.Role, &expiresAt, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
		if expiresAt.Valid {
			u.ExpiresAt = &expiresAt.Time
		}
		users = append(users, u)
	}
	if users == nil {
		users = []User{}
	}
	jsonResponse(w, users)
}

func (s *AuthService) HandleCreateUser(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Username  string  `json:"username"`
		Password  string  `json:"password"`
		Role      string  `json:"role"`
		ExpiresAt *string `json:"expiresAt"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}
	if req.Username == "" || req.Password == "" {
		jsonError(w, "username and password required", 400)
		return
	}
	if req.Role == "" {
		req.Role = "user"
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)

	var expiresAt *time.Time
	if req.ExpiresAt != nil && *req.ExpiresAt != "" {
		t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
		if err == nil {
			expiresAt = &t
		}
	}

	result, err := s.db.Exec("INSERT INTO users (username, password_hash, role, expires_at, enabled) VALUES (?, ?, ?, ?, 1)",
		req.Username, string(hash), req.Role, expiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			jsonError(w, "username already exists", 409)
			return
		}
		jsonError(w, err.Error(), 500)
		return
	}

	id, _ := result.LastInsertId()
	jsonResponse(w, map[string]interface{}{"id": id, "username": req.Username, "role": req.Role})
}

func (s *AuthService) HandleUpdateUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	var req struct {
		Password  *string `json:"password"`
		Role      *string `json:"role"`
		ExpiresAt *string `json:"expiresAt"`
		Enabled   *bool   `json:"enabled"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonError(w, "invalid request", 400)
		return
	}

	if req.Password != nil {
		hash, _ := bcrypt.GenerateFromPassword([]byte(*req.Password), bcrypt.DefaultCost)
		if _, err := s.db.Exec("UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(hash), id); err != nil {
			jsonError(w, "更新密码失败: "+err.Error(), 500)
			return
		}
	}
	if req.Role != nil {
		if *req.Role != "admin" && *req.Role != "user" {
			jsonError(w, "角色只能为 admin 或 user", 400)
			return
		}
		if _, err := s.db.Exec("UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *req.Role, id); err != nil {
			jsonError(w, "更新角色失败: "+err.Error(), 500)
			return
		}
	}
	if req.ExpiresAt != nil {
		if *req.ExpiresAt == "" {
			if _, err := s.db.Exec("UPDATE users SET expires_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
				jsonError(w, "更新过期时间失败: "+err.Error(), 500)
				return
			}
		} else {
			t, err := time.Parse(time.RFC3339, *req.ExpiresAt)
			if err == nil {
				if _, err := s.db.Exec("UPDATE users SET expires_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", t, id); err != nil {
					jsonError(w, "更新过期时间失败: "+err.Error(), 500)
					return
				}
			}
		}
	}
	if req.Enabled != nil {
		if _, err := s.db.Exec("UPDATE users SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", *req.Enabled, id); err != nil {
			jsonError(w, "更新状态失败: "+err.Error(), 500)
			return
		}
	}

	jsonResponse(w, map[string]interface{}{"ok": true})
}

func (s *AuthService) HandleDeleteUser(w http.ResponseWriter, r *http.Request) {
	id, _ := strconv.ParseInt(chi.URLParam(r, "id"), 10, 64)

	// Prevent deleting the default admin
	var username string
	s.db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&username)
	if username == "myt" {
		jsonError(w, "cannot delete default admin", 403)
		return
	}

	s.db.Exec("DELETE FROM users WHERE id = ?", id)
	jsonResponse(w, map[string]interface{}{"ok": true})
}

// --- User expiry checker ---

func (s *AuthService) CheckExpiry(hub *WSHub, interval time.Duration) {
	ticker := time.NewTicker(interval)
	for range ticker.C {
		rows, err := s.db.Query("SELECT id, username FROM users WHERE enabled = 1 AND expires_at IS NOT NULL AND expires_at < ?", time.Now())
		if err != nil {
			continue
		}
		for rows.Next() {
			var id int64
			var username string
			rows.Scan(&id, &username)
			s.db.Exec("UPDATE users SET enabled = 0, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id)
			log.Printf("[Auth] User '%s' expired, disabled", username)
			hub.Broadcast("user:kicked", map[string]interface{}{"username": username, "reason": "expired"})
			hub.KickUser(username)
		}
		rows.Close()
	}
}

// --- User Permissions ---

// UserPermissions 用户权限配置（仅普通用户使用，admin 默认全部权限）
type UserPermissions struct {
	Slots           []int `json:"slots"`            // 允许的坑位列表
	ContainerStart  bool  `json:"container_start"`  // 启动/停止
	ContainerRestart bool `json:"container_restart"` // 重启
	ContainerReset  bool  `json:"container_reset"`  // 重置
	ContainerDelete bool  `json:"container_delete"` // 删除
	ContainerRename bool  `json:"container_rename"` // 重命名
	ContainerCopy   bool  `json:"container_copy"`   // 复制
	ContainerCreate bool  `json:"container_create"` // 创建容器
	AliasManage     bool  `json:"alias_manage"`     // 别名管理
	BackupManage    bool  `json:"backup_manage"`    // 备份管理
	ImageView       bool  `json:"image_view"`       // 查看镜像
	Projection      bool  `json:"projection"`       // 投屏
	Terminal        bool  `json:"terminal"`         // 终端
	NetworkBridge   bool  `json:"network_bridge"`   // 虚拟内置网卡
	VpcManage       bool  `json:"vpc_manage"`       // VPC
}

// AllowedSlotsMap 返回坑位集合，用于快速查找
func (p *UserPermissions) AllowedSlotsMap() map[int]bool {
	m := make(map[int]bool, len(p.Slots))
	for _, s := range p.Slots {
		m[s] = true
	}
	return m
}

// GetUserPermissions 获取用户权限
func (s *AuthService) GetUserPermissions(userID int64) *UserPermissions {
	var slotsStr string
	var p UserPermissions
	err := s.db.QueryRow(`SELECT slots, container_start, container_restart, container_reset, container_delete,
		container_rename, container_copy, container_create, alias_manage, backup_manage,
		image_view, projection, terminal, network_bridge, vpc_manage
		FROM user_permissions WHERE user_id = ?`, userID).Scan(
		&slotsStr, &p.ContainerStart, &p.ContainerRestart, &p.ContainerReset, &p.ContainerDelete,
		&p.ContainerRename, &p.ContainerCopy, &p.ContainerCreate, &p.AliasManage, &p.BackupManage,
		&p.ImageView, &p.Projection, &p.Terminal, &p.NetworkBridge, &p.VpcManage)
	if err != nil {
		// 没有记录，返回空权限
		return &UserPermissions{}
	}
	// 解析 slots
	if slotsStr != "" {
		for _, part := range strings.Split(slotsStr, ",") {
			part = strings.TrimSpace(part)
			if n, err := strconv.Atoi(part); err == nil {
				p.Slots = append(p.Slots, n)
			}
		}
	}
	if p.Slots == nil {
		p.Slots = []int{}
	}
	return &p
}

// SaveUserPermissions 保存用户权限
func (s *AuthService) SaveUserPermissions(userID int64, p *UserPermissions) error {
	// 构建 slots 字符串
	parts := make([]string, len(p.Slots))
	for i, n := range p.Slots {
		parts[i] = strconv.Itoa(n)
	}
	slotsStr := strings.Join(parts, ",")

	_, err := s.db.Exec(`INSERT INTO user_permissions (user_id, slots, container_start, container_restart,
		container_reset, container_delete, container_rename, container_copy, container_create,
		alias_manage, backup_manage, image_view, projection, terminal, network_bridge, vpc_manage)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(user_id) DO UPDATE SET
		slots=excluded.slots, container_start=excluded.container_start, container_restart=excluded.container_restart,
		container_reset=excluded.container_reset, container_delete=excluded.container_delete,
		container_rename=excluded.container_rename, container_copy=excluded.container_copy,
		container_create=excluded.container_create, alias_manage=excluded.alias_manage,
		backup_manage=excluded.backup_manage, image_view=excluded.image_view,
		projection=excluded.projection, terminal=excluded.terminal,
		network_bridge=excluded.network_bridge, vpc_manage=excluded.vpc_manage`,
		userID, slotsStr, p.ContainerStart, p.ContainerRestart,
		p.ContainerReset, p.ContainerDelete, p.ContainerRename, p.ContainerCopy, p.ContainerCreate,
		p.AliasManage, p.BackupManage, p.ImageView, p.Projection, p.Terminal, p.NetworkBridge, p.VpcManage)
	return err
}

// DeleteUserPermissions 删除用户权限记录
func (s *AuthService) DeleteUserPermissions(userID int64) {
	s.db.Exec("DELETE FROM user_permissions WHERE user_id = ?", userID)
}

// --- Helpers ---

func (s *AuthService) getUserByName(username string) *User {
	u := &User{}
	var expiresAt sql.NullTime
	err := s.db.QueryRow("SELECT id, username, password_hash, role, expires_at, enabled, created_at, updated_at FROM users WHERE username = ?", username).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &expiresAt, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil
	}
	if expiresAt.Valid {
		u.ExpiresAt = &expiresAt.Time
	}
	return u
}

func (s *AuthService) getUserByID(id int64) *User {
	u := &User{}
	var expiresAt sql.NullTime
	err := s.db.QueryRow("SELECT id, username, password_hash, role, expires_at, enabled FROM users WHERE id = ?", id).
		Scan(&u.ID, &u.Username, &u.PasswordHash, &u.Role, &expiresAt, &u.Enabled)
	if err != nil {
		return nil
	}
	if expiresAt.Valid {
		u.ExpiresAt = &expiresAt.Time
	}
	return u
}

// GetSetting 读取系统设置
func (s *AuthService) GetSetting(key string) string {
	var value string
	err := s.db.QueryRow("SELECT value FROM settings WHERE key = ?", key).Scan(&value)
	if err != nil {
		return ""
	}
	return value
}

// SetSetting 保存系统设置（upsert）
func (s *AuthService) SetSetting(key, value string) error {
	_, err := s.db.Exec(`INSERT INTO settings (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value=excluded.value`, key, value)
	return err
}

// GetAllSettings 获取所有系统设置
func (s *AuthService) GetAllSettings() map[string]string {
	result := make(map[string]string)
	rows, err := s.db.Query("SELECT key, value FROM settings")
	if err != nil {
		return result
	}
	defer rows.Close()
	for rows.Next() {
		var k, v string
		if rows.Scan(&k, &v) == nil {
			result[k] = v
		}
	}
	return result
}

func jsonResponse(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func jsonError(w http.ResponseWriter, msg string, code int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	json.NewEncoder(w).Encode(map[string]string{"error": msg})
}
