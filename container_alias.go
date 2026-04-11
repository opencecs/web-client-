package main

import (
	"database/sql"
	"log"
	"strings"
	"time"
)

type ContainerAliasService struct {
	db *sql.DB
}

func NewContainerAliasService(db *sql.DB) *ContainerAliasService {
	s := &ContainerAliasService{db: db}
	s.initTable()
	return s
}

func (s *ContainerAliasService) initTable() {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS container_aliases (
			name TEXT PRIMARY KEY,
			alias TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal("[Alias] 创建表失败:", err)
	}
}

// GetAllAliases 返回全部别名映射
func (s *ContainerAliasService) GetAllAliases() map[string]string {
	aliases := make(map[string]string)
	rows, err := s.db.Query("SELECT name, alias FROM container_aliases")
	if err != nil {
		log.Printf("[Alias] 查询失败: %v", err)
		return aliases
	}
	defer rows.Close()
	for rows.Next() {
		var name, alias string
		if rows.Scan(&name, &alias) == nil {
			aliases[name] = alias
		}
	}
	return aliases
}

// SetAlias 设置或更新别名
func (s *ContainerAliasService) SetAlias(name, alias string) error {
	name = strings.TrimSpace(name)
	alias = strings.TrimSpace(alias)
	if name == "" || alias == "" {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO container_aliases (name, alias, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET alias = excluded.alias, updated_at = excluded.updated_at`,
		name, alias, time.Now(),
	)
	return err
}

// DeleteAlias 删除别名
func (s *ContainerAliasService) DeleteAlias(name string) error {
	name = strings.TrimSpace(name)
	if name == "" {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM container_aliases WHERE name = ?", name)
	return err
}
