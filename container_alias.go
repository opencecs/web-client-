package main

import (
	"database/sql"
	"log"
	"strings"
	"sync"
	"time"
)

type ContainerAliasService struct {
	db       *sql.DB
	migrated sync.Once
}

func NewContainerAliasService(db *sql.DB) *ContainerAliasService {
	s := &ContainerAliasService{db: db}
	s.initTable()
	return s
}

func (s *ContainerAliasService) initTable() {
	// 以容器名为主键（别名跟随具体容器，容器删除后别名消失）
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS container_name_aliases (
			container_name TEXT PRIMARY KEY,
			alias TEXT NOT NULL,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		log.Fatal("[Alias] 创建 container_name_aliases 表失败:", err)
	}
}

// MigrateFromSlotBased 从旧的 slot_aliases 表迁移到容器名模式（只执行一次）
func (s *ContainerAliasService) MigrateFromSlotBased(containers []ParsedContainer) {
	s.migrated.Do(func() {
		// 检查旧表是否存在
		var tableName string
		err := s.db.QueryRow("SELECT name FROM sqlite_master WHERE type='table' AND name='slot_aliases'").Scan(&tableName)
		if err != nil {
			return // 旧表不存在，无需迁移
		}

		// 读取旧别名 slot_num → alias
		rows, err := s.db.Query("SELECT slot_num, alias FROM slot_aliases")
		if err != nil {
			return
		}
		defer rows.Close()

		// 构建坑位号 → 容器名映射
		slotToName := make(map[int]string, len(containers))
		for _, c := range containers {
			if c.IndexNum > 0 {
				slotToName[c.IndexNum] = c.Name
			}
		}

		migrated := 0
		for rows.Next() {
			var slot int
			var alias string
			if rows.Scan(&slot, &alias) != nil {
				continue
			}
			name, ok := slotToName[slot]
			if !ok || name == "" {
				continue
			}
			// 仅迁移新表中不存在的容器
			var exists int
			s.db.QueryRow("SELECT COUNT(*) FROM container_name_aliases WHERE container_name = ?", name).Scan(&exists)
			if exists == 0 {
				s.db.Exec("INSERT INTO container_name_aliases (container_name, alias, updated_at) VALUES (?, ?, ?)",
					name, alias, time.Now())
				migrated++
			}
		}

		if migrated > 0 {
			log.Printf("[Alias] 从坑位模式迁移了 %d 条别名到容器名模式", migrated)
		}

		// 迁移完成后删除旧表
		s.db.Exec("DROP TABLE IF EXISTS slot_aliases")
	})
}

// SetAlias 设置容器别名
func (s *ContainerAliasService) SetAlias(containerName, alias string) error {
	containerName = strings.TrimSpace(containerName)
	alias = strings.TrimSpace(alias)
	if containerName == "" || alias == "" {
		return nil
	}
	_, err := s.db.Exec(
		`INSERT INTO container_name_aliases (container_name, alias, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(container_name) DO UPDATE SET alias = excluded.alias, updated_at = excluded.updated_at`,
		containerName, alias, time.Now(),
	)
	return err
}

// DeleteAlias 删除容器别名
func (s *ContainerAliasService) DeleteAlias(containerName string) error {
	if containerName == "" {
		return nil
	}
	_, err := s.db.Exec("DELETE FROM container_name_aliases WHERE container_name = ?", containerName)
	return err
}

// GetAliases 返回 containerName → alias 映射
func (s *ContainerAliasService) GetAliases() map[string]string {
	aliases := make(map[string]string)
	rows, err := s.db.Query("SELECT container_name, alias FROM container_name_aliases")
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

// BuildAliasMap 返回 containerName → alias 映射（前端兼容格式）
func (s *ContainerAliasService) BuildAliasMap(containers []ParsedContainer) map[string]string {
	return s.GetAliases()
}
