package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

const mytMirrorAPI = "http://api.moyunteng.com/api.php"

// MirrorImage 在线镜像
type MirrorImage struct {
	ID      string   `json:"id"`
	Name    string   `json:"name"`
	URL     string   `json:"url"`
	OsVer   string   `json:"os_ver"`
	SysVer  string   `json:"sys_ver"`
	SysDesc string   `json:"sys_ver_des"`
	TType2  []string `json:"ttype2"`
}

// 镜像缓存
var (
	mirrorCacheMu sync.RWMutex
	mirrorCache   []MirrorImage
	mirrorCacheAt time.Time
	mirrorCacheTTL = 30 * time.Minute
)

// 镜像黑名单（名称包含以下关键词的镜像不显示）
var mirrorBlacklist = []string{
	"CQR16-ALL-v0.1.0",
	"CQR16-ALL-v0.1.1",
}

// isBlacklisted 检查镜像是否在黑名单中
func isBlacklisted(name string) bool {
	for _, kw := range mirrorBlacklist {
		if strings.Contains(name, kw) {
			return true
		}
	}
	return false
}

// HandleMirrorList 获取在线镜像列表（带缓存）
func (d *DeviceService) HandleMirrorList(w http.ResponseWriter, r *http.Request) {
	deviceModel := strings.ToLower(d.getDeviceModel())

	// 检查缓存
	mirrorCacheMu.RLock()
	cached := mirrorCache
	cacheValid := time.Since(mirrorCacheAt) < mirrorCacheTTL && cached != nil
	mirrorCacheMu.RUnlock()

	if !cacheValid {
		// 从魔云腾 API 获取
		fresh, err := fetchMirrorList()
		if err != nil {
			// 缓存过期但有旧数据，降级使用旧缓存
			if cached != nil {
				log.Printf("[Mirror] 刷新失败，使用旧缓存: %v", err)
			} else {
				jsonError(w, "获取镜像列表失败: "+err.Error(), 500)
				return
			}
		} else {
			cached = fresh
			mirrorCacheMu.Lock()
			mirrorCache = fresh
			mirrorCacheAt = time.Now()
			mirrorCacheMu.Unlock()
			log.Printf("[Mirror] 镜像列表已缓存，共 %d 条", len(fresh))
		}
	}

	// 筛选：sys_ver=5 (ALL版) + 安卓14或16 + 匹配设备型号
	var filtered []MirrorImage
	for _, img := range cached {
		if img.SysVer != "5" {
			continue
		}
		if img.OsVer != "and14" && img.OsVer != "and16" {
			continue
		}
		if !matchDeviceType(img.TType2, deviceModel) {
			continue
		}
		// 排除 dev 版本镜像
		if strings.Contains(strings.ToLower(img.Name), "dev") {
			continue
		}
		// 排除黑名单镜像
		if isBlacklisted(img.Name) {
			continue
		}
		filtered = append(filtered, img)
	}

	jsonResponse(w, filtered)
}

// fetchMirrorList 从魔云腾 API 拉取完整镜像列表
func fetchMirrorList() ([]MirrorImage, error) {
	client := &http.Client{Timeout: 15 * time.Second}
	formData := url.Values{}
	formData.Set("type", "get_mirror_list2")

	resp, err := client.Post(mytMirrorAPI, "application/x-www-form-urlencoded", strings.NewReader(formData.Encode()))
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))

	var result struct {
		Code json.Number     `json:"code"`
		Data json.RawMessage `json:"data"`
		Msg  string          `json:"msg"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return nil, err
	}

	var images []MirrorImage
	json.Unmarshal(result.Data, &images)
	return images, nil
}

// matchDeviceType 判断镜像是否支持当前设备
func matchDeviceType(ttype2 []string, deviceModel string) bool {
	var needTag string
	if strings.Contains(deviceModel, "p1") {
		needTag = "p1_v3"
	} else if strings.Contains(deviceModel, "r1s") || strings.Contains(deviceModel, "r1") {
		needTag = "r1s_v3"
	} else if strings.Contains(deviceModel, "c1") {
		needTag = "c1_v3"
	} else {
		needTag = "q1_v3"
	}

	for _, t := range ttype2 {
		if strings.ToLower(t) == needTag {
			return true
		}
	}
	return false
}

// getDeviceModel 从缓存中获取设备型号
func (d *DeviceService) getDeviceModel() string {
	d.cacheMu.RLock()
	data := d.cachedInfo
	d.cacheMu.RUnlock()
	if data == nil {
		return ""
	}
	var info struct {
		Data struct {
			Model string `json:"model"`
		} `json:"data"`
	}
	if json.Unmarshal(data, &info) == nil && info.Data.Model != "" {
		return info.Data.Model
	}
	var flat struct {
		Model string `json:"model"`
	}
	if json.Unmarshal(data, &flat) == nil {
		return flat.Model
	}
	return ""
}

// GetMirrorsJSON 返回过滤后的镜像列表 JSON（供 WS 调用）
func (d *DeviceService) GetMirrorsJSON() (json.RawMessage, error) {
	deviceModel := strings.ToLower(d.getDeviceModel())

	mirrorCacheMu.RLock()
	cached := mirrorCache
	cacheValid := time.Since(mirrorCacheAt) < mirrorCacheTTL && cached != nil
	mirrorCacheMu.RUnlock()

	if !cacheValid {
		fresh, err := fetchMirrorList()
		if err != nil {
			if cached != nil {
				log.Printf("[Mirror] 刷新失败，使用旧缓存: %v", err)
			} else {
				return nil, fmt.Errorf("获取镜像列表失败: %v", err)
			}
		} else {
			cached = fresh
			mirrorCacheMu.Lock()
			mirrorCache = fresh
			mirrorCacheAt = time.Now()
			mirrorCacheMu.Unlock()
			log.Printf("[Mirror] 镜像列表已缓存，共 %d 条", len(fresh))
		}
	}

	var filtered []MirrorImage
	for _, img := range cached {
		if img.SysVer != "5" {
			continue
		}
		if img.OsVer != "and14" && img.OsVer != "and16" {
			continue
		}
		if !matchDeviceType(img.TType2, deviceModel) {
			continue
		}
		if strings.Contains(strings.ToLower(img.Name), "dev") {
			continue
		}
		// 排除黑名单镜像
		if isBlacklisted(img.Name) {
			continue
		}
		filtered = append(filtered, img)
	}

	data, err := json.Marshal(filtered)
	if err != nil {
		return nil, err
	}
	return json.RawMessage(data), nil
}
