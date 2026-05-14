package main

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"golang.org/x/crypto/bcrypt"
)

// handleUserAction 处理用户管理操作
func (c *WSClient) handleUserAction(req WSRequest) {
	switch req.Action {
	case "user:list":
		users, err := c.hub.auth.ListUsers()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", users)

	case "user:create":
		username := getStr(req.Data, "username")
		password := getStr(req.Data, "password")
		role := getStr(req.Data, "role")
		expiresAt := getStr(req.Data, "expiresAt")
		result, err := c.hub.auth.CreateUser(username, password, role, expiresAt)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", result)

	case "user:update":
		id := getNum(req.Data, "id")
		password := getStr(req.Data, "password")
		err := c.hub.auth.UpdateUser(id, req.Data)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		// 改密码后踢掉该用户所有连接，并使旧 token 失效
		if password != "" {
			var username string
			c.hub.auth.db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&username)
			if username != "" {
				// 先发响应给操作者，再踢人
				c.sendResponse(req.ID, true, "密码修改成功，该用户已被强制下线", nil)
				c.hub.auth.tokenInvalidAt.Store(username, time.Now())
				c.hub.auth.sessionKeys.Delete(username)
				// 延迟踢人，确保响应先到达
				go func() {
					time.Sleep(500 * time.Millisecond)
					c.hub.KickUserWithReason(username, "password_changed")
				}()
				return
			}
		}
		c.sendResponse(req.ID, true, "ok", nil)

	case "user:delete":
		id := getNum(req.Data, "id")
		// 先查用户名，用于踢在线连接
		var delUsername string
		c.hub.auth.db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&delUsername)
		// 删除用户时同时删除权限记录
		err := c.hub.auth.DeleteUser(id)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.hub.auth.DeleteUserPermissions(int64(id))
		// 踢掉该用户的在线连接
		if delUsername != "" {
			c.hub.auth.tokenInvalidAt.Store(delUsername, time.Now())
			c.hub.KickUserWithReason(delUsername, "deleted")
		}
		c.sendResponse(req.ID, true, "ok", nil)

	case "user:getPermissions":
		id := int64(getNum(req.Data, "id"))
		perms := c.hub.auth.GetUserPermissions(id)
		c.sendResponse(req.ID, true, "ok", perms)

	case "user:setPermissions":
		id := int64(getNum(req.Data, "id"))
		perms := parsePermissions(req.Data)
		err := c.hub.auth.SaveUserPermissions(id, perms)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		// 刷新该用户的 WS 连接权限
		c.hub.refreshUserPermissions(id, perms)
		c.sendResponse(req.ID, true, "ok", nil)
	}
}

// parsePermissions 从请求数据中解析权限配置
func parsePermissions(data map[string]interface{}) *UserPermissions {
	p := &UserPermissions{}

	// 解析 slots
	if slotsRaw, ok := data["slots"]; ok {
		switch v := slotsRaw.(type) {
		case []interface{}:
			for _, s := range v {
				switch n := s.(type) {
				case float64:
					p.Slots = append(p.Slots, int(n))
				}
			}
		}
	}
	if p.Slots == nil {
		p.Slots = []int{}
	}

	p.ContainerStart = getBool(data, "container_start")
	p.ContainerRestart = getBool(data, "container_restart")
	p.ContainerReset = getBool(data, "container_reset")
	p.ContainerDelete = getBool(data, "container_delete")
	p.ContainerRename = getBool(data, "container_rename")
	p.ContainerCopy = getBool(data, "container_copy")
	p.ContainerCreate = getBool(data, "container_create")
	p.AliasManage = getBool(data, "alias_manage")
	p.BackupManage = getBool(data, "backup_manage")
	p.ImageView = getBool(data, "image_view")
	p.Projection = getBool(data, "projection")
	p.Terminal = getBool(data, "terminal")
	p.NetworkBridge = getBool(data, "network_bridge")
	p.VpcManage = getBool(data, "vpc_manage")

	p.MenuDashboard = getBool(data, "menu_dashboard")
	p.MenuDevice = getBool(data, "menu_device")
	p.MenuAndroid = getBool(data, "menu_android")
	p.MenuBackup = getBool(data, "menu_backup")
	p.MenuFile = getBool(data, "menu_file")
	p.MenuUsers = getBool(data, "menu_users")
	p.SwitchModel = getBool(data, "switch_model")

	return p
}

// ===== WSHub 方法 =====

// refreshUserPermissions 刷新指定用户所有连接的权限
func (h *WSHub) refreshUserPermissions(userID int64, perms *UserPermissions) {
	h.mu.RLock()
	var targets []*WSClient
	for client := range h.clients {
		if client.userID == userID && !client.isAdmin {
			targets = append(targets, client)
		}
	}
	h.mu.RUnlock()
	// 释放锁后再修改 client 字段
	for _, client := range targets {
		client.permissions = perms
		client.SendJSON(map[string]interface{}{
			"type": "event", "event": "user:permissions",
			"data": perms,
		})
	}
	// 触发容器列表刷新，让用户看到更新后的坑位
	h.TriggerRefresh()
}

// ===== AuthService 的 WS 可用方法 =====

// ListUsers 返回所有用户列表
func (s *AuthService) ListUsers() ([]User, error) {
	rows, err := s.db.Query("SELECT id, username, role, expires_at, enabled, created_at, updated_at FROM users ORDER BY id")
	if err != nil {
		return nil, err
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
	return users, nil
}

// CreateUser 创建用户
func (s *AuthService) CreateUser(username, password, role, expiresAtStr string) (map[string]interface{}, error) {
	if username == "" || password == "" {
		return nil, fmt.Errorf("用户名和密码不能为空")
	}
	if role == "" {
		role = "user"
	}
	if role != "admin" && role != "user" {
		return nil, fmt.Errorf("角色只能为 admin 或 user")
	}

	hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)

	var expiresAt *time.Time
	if expiresAtStr != "" {
		t, err := time.Parse(time.RFC3339, expiresAtStr)
		if err == nil {
			expiresAt = &t
		}
	}

	result, err := s.db.Exec("INSERT INTO users (username, password_hash, role, expires_at, enabled) VALUES (?, ?, ?, ?, 1)",
		username, string(hash), role, expiresAt)
	if err != nil {
		if strings.Contains(err.Error(), "UNIQUE") {
			return nil, fmt.Errorf("用户名已存在")
		}
		return nil, err
	}

	id, _ := result.LastInsertId()
	return map[string]interface{}{"id": id, "username": username, "role": role}, nil
}

// UpdateUser 更新用户
func (s *AuthService) UpdateUser(id int, data map[string]interface{}) error {
	if password := getStr(data, "password"); password != "" {
		hash, _ := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if _, err := s.db.Exec("UPDATE users SET password_hash = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", string(hash), id); err != nil {
			return fmt.Errorf("更新密码失败: %v", err)
		}
	}
	if role := getStr(data, "role"); role != "" {
		if role != "admin" && role != "user" {
			return fmt.Errorf("角色只能为 admin 或 user")
		}
		if _, err := s.db.Exec("UPDATE users SET role = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", role, id); err != nil {
			return fmt.Errorf("更新角色失败: %v", err)
		}
	}
	if expiresAt := getStr(data, "expiresAt"); expiresAt != "" {
		if expiresAt == "null" {
			if _, err := s.db.Exec("UPDATE users SET expires_at = NULL, updated_at = CURRENT_TIMESTAMP WHERE id = ?", id); err != nil {
				return fmt.Errorf("更新过期时间失败: %v", err)
			}
		} else {
			t, err := time.Parse(time.RFC3339, expiresAt)
			if err == nil {
				if _, err := s.db.Exec("UPDATE users SET expires_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", t, id); err != nil {
					return fmt.Errorf("更新过期时间失败: %v", err)
				}
			}
		}
	}
	if enabled, ok := data["enabled"]; ok {
		if _, err := s.db.Exec("UPDATE users SET enabled = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?", enabled, id); err != nil {
			return fmt.Errorf("更新状态失败: %v", err)
		}
	}
	return nil
}

// DeleteUser 删除用户
func (s *AuthService) DeleteUser(id int) error {
	var username string
	s.db.QueryRow("SELECT username FROM users WHERE id = ?", id).Scan(&username)
	if username == "myt" {
		return fmt.Errorf("不能删除默认管理员")
	}
	s.db.Exec("DELETE FROM users WHERE id = ?", id)
	return nil
}
