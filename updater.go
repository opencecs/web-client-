package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

// 更新锁，防止并发触发
var updateMu sync.Mutex
var updating bool

// UpdateInfo 更新服务器返回的版本信息
type UpdateInfo struct {
	LatestVersion string `json:"latest_version"`
	DownloadURL   string `json:"download_url"`
	SHA256        string `json:"sha256"`
	Changelog     string `json:"changelog"`
	ForceUpdate   bool   `json:"force_update"`
	FileSize      int64  `json:"file_size"`
}

// handlePanelVersion 返回当前面板版本
func (c *WSClient) handlePanelVersion(req WSRequest) {
	c.sendResponse(req.ID, true, "ok", map[string]interface{}{
		"version": Version,
		"arch":    runtime.GOARCH,
		"device":  Device,
	})
}

// handlePanelCheckUpdate 检查面板更新
func (c *WSClient) handlePanelCheckUpdate(req WSRequest) {
	info, err := checkPanelUpdate()
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}

	hasUpdate := info.DownloadURL != "" && compareVersion(info.LatestVersion, Version) > 0
	c.sendResponse(req.ID, true, "ok", map[string]interface{}{
		"hasUpdate":     hasUpdate,
		"currentVersion": Version,
		"latestVersion": info.LatestVersion,
		"changelog":     info.Changelog,
		"fileSize":      info.FileSize,
		"forceUpdate":   info.ForceUpdate,
	})
}

// handlePanelDoUpdate 执行面板更新（手动触发）
func (c *WSClient) handlePanelDoUpdate(req WSRequest) {
	updateMu.Lock()
	if updating {
		updateMu.Unlock()
		c.sendResponse(req.ID, false, "更新正在进行中", nil)
		return
	}
	updating = true
	updateMu.Unlock()

	c.sendResponse(req.ID, true, "updating", nil)

	err := doUpdateInternal(c.hub)
	if err != nil {
		updateMu.Lock()
		updating = false
		updateMu.Unlock()
		c.hub.Broadcast("task:progress", map[string]interface{}{
			"action": "panel:update", "phase": "error", "message": err.Error(),
		})
	}
	// 成功时 doUpdateInternal 会重启进程，不会走到这里
}

// doUpdateInternal 核心更新逻辑（检查→下载→校验→替换→重启）
// 供手动更新和自动更新共用，通过 hub.Broadcast 发送进度
func doUpdateInternal(hub *WSHub) error {
	// 1. 检查更新
	hub.Broadcast("task:progress", map[string]interface{}{
		"action": "panel:update", "phase": "checking", "message": "正在检查更新...",
	})

	info, err := checkPanelUpdate()
	if err != nil {
		return fmt.Errorf("检查更新失败: %v", err)
	}

	if info.DownloadURL == "" || compareVersion(info.LatestVersion, Version) <= 0 {
		return fmt.Errorf("已是最新版本")
	}

	log.Printf("[更新] 发现新版本: %s → %s", Version, info.LatestVersion)

	// 2. 下载新二进制
	hub.Broadcast("task:progress", map[string]interface{}{
		"action": "panel:update", "phase": "downloading", "message": "正在下载新版本...",
		"progress": 0, "fileSize": info.FileSize,
	})

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("获取程序路径失败: %v", err)
	}
	exePath, _ = filepath.EvalSymlinks(exePath)
	newPath := exePath + ".new"

	err = downloadWithProgress(info.DownloadURL, newPath, info.FileSize, func(downloaded, total int64) {
		progress := 0
		if total > 0 {
			progress = int(downloaded * 100 / total)
		}
		hub.Broadcast("task:progress", map[string]interface{}{
			"action": "panel:update", "phase": "downloading",
			"progress": progress, "downloaded": downloaded, "fileSize": total,
		})
	})
	if err != nil {
		os.Remove(newPath)
		return fmt.Errorf("下载失败: %v", err)
	}

	// 3. SHA256 校验
	hub.Broadcast("task:progress", map[string]interface{}{
		"action": "panel:update", "phase": "verifying", "message": "正在校验文件...", "progress": 100,
	})

	if info.SHA256 != "" {
		actualHash, err := fileSHA256(newPath)
		if err != nil {
			os.Remove(newPath)
			return fmt.Errorf("校验失败: %v", err)
		}
		if !strings.EqualFold(actualHash, info.SHA256) {
			os.Remove(newPath)
			return fmt.Errorf("校验不匹配: 期望 %s, 实际 %s", info.SHA256[:16]+"...", actualHash[:16]+"...")
		}
		log.Printf("[更新] SHA256 校验通过: %s", actualHash[:16])
	}

	// 4. 原子替换
	hub.Broadcast("task:progress", map[string]interface{}{
		"action": "panel:update", "phase": "replacing", "message": "正在替换文件...", "progress": 100,
	})

	os.Chmod(newPath, 0755)

	backupPath := exePath + ".bak"
	os.Remove(backupPath)
	if err := os.Rename(exePath, backupPath); err != nil {
		os.Remove(newPath)
		return fmt.Errorf("备份旧文件失败: %v", err)
	}

	if err := os.Rename(newPath, exePath); err != nil {
		os.Rename(backupPath, exePath)
		return fmt.Errorf("替换文件失败: %v", err)
	}

	if info.SHA256 != "" {
		os.WriteFile(exePath+".sha256", []byte(info.SHA256), 0644)
	}

	log.Printf("[更新] 面板已更新: %s → %s", Version, info.LatestVersion)

	// 5. 通知并重启
	hub.Broadcast("task:progress", map[string]interface{}{
		"action": "panel:update", "phase": "restarting",
		"message": fmt.Sprintf("更新完成 v%s → v%s，正在重启...", Version, info.LatestVersion),
		"progress": 100, "done": true,
	})

	go func() {
		time.Sleep(2 * time.Second)
		log.Printf("[更新] 正在重启...")
		args := os.Args
		env := os.Environ()
		if err := syscall.Exec(exePath, args, env); err != nil {
			log.Printf("[更新] syscall.Exec 失败: %v，尝试 os.Exit", err)
			os.Exit(0)
		}
	}()

	return nil
}

// StartAutoUpdate 后台自动更新（仅正式版，每 5 分钟检查）
func StartAutoUpdate(hub *WSHub) {
	if Version == "dev" {
		log.Printf("[自动更新] 当前为开发版本，跳过自动更新")
		return
	}

	log.Printf("[自动更新] 已启动，当前版本 v%s，每 5 分钟检查", Version)

	ticker := time.NewTicker(5 * time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		info, err := checkPanelUpdate()
		if err != nil {
			log.Printf("[自动更新] 检查失败: %v", err)
			continue
		}

		if info.DownloadURL == "" || compareVersion(info.LatestVersion, Version) <= 0 {
			continue
		}

		log.Printf("[自动更新] 发现新版本 v%s → v%s，开始自动更新", Version, info.LatestVersion)

		updateMu.Lock()
		if updating {
			updateMu.Unlock()
			log.Printf("[自动更新] 已有更新在进行中，跳过")
			continue
		}
		updating = true
		updateMu.Unlock()

		err = doUpdateInternal(hub)
		if err != nil {
			updateMu.Lock()
			updating = false
			updateMu.Unlock()
			log.Printf("[自动更新] 更新失败: %v", err)
		}
		// 成功时进程会重启，不会走到这里
	}
}

// checkPanelUpdate 向更新服务器查询最新版本
func checkPanelUpdate() (*UpdateInfo, error) {
	if UpdateURL == "" {
		return nil, fmt.Errorf("未配置更新服务器地址")
	}

	url := fmt.Sprintf("%s/update/check?version=%s&arch=%s&device=%s",
		strings.TrimRight(UpdateURL, "/"), Version, runtime.GOARCH, Device)

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连接更新服务器失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("更新服务器返回 HTTP %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(io.LimitReader(resp.Body, 1*1024*1024))

	// 尝试解析带 data 包装的格式：{"code_id":200,"msg":"success","data":{...}}
	var wrapper struct {
		CodeID int        `json:"code_id"`
		Data   UpdateInfo `json:"data"`
	}
	if err := json.Unmarshal(body, &wrapper); err == nil && wrapper.CodeID == 200 && wrapper.Data.LatestVersion != "" {
		return &wrapper.Data, nil
	}

	// 兼容直接返回 UpdateInfo 的格式
	var info UpdateInfo
	if err := json.Unmarshal(body, &info); err != nil {
		return nil, fmt.Errorf("解析版本信息失败: %v", err)
	}

	return &info, nil
}

// downloadWithProgress 带进度回调的文件下载
func downloadWithProgress(url, destPath string, expectedSize int64, onProgress func(downloaded, total int64)) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return err
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return fmt.Errorf("下载失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return fmt.Errorf("下载返回 HTTP %d", resp.StatusCode)
	}

	total := resp.ContentLength
	if total <= 0 && expectedSize > 0 {
		total = expectedSize
	}

	f, err := os.Create(destPath)
	if err != nil {
		return fmt.Errorf("创建文件失败: %v", err)
	}
	defer f.Close()

	buf := make([]byte, 64*1024)
	var downloaded int64
	lastReport := time.Now()

	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			if _, err := f.Write(buf[:n]); err != nil {
				return fmt.Errorf("写入文件失败: %v", err)
			}
			downloaded += int64(n)

			// 每 500ms 报告一次进度
			if time.Since(lastReport) > 500*time.Millisecond {
				if onProgress != nil {
					onProgress(downloaded, total)
				}
				lastReport = time.Now()
			}
		}
		if readErr != nil {
			if readErr == io.EOF {
				break
			}
			return fmt.Errorf("读取失败: %v", readErr)
		}
	}

	// 最终进度
	if onProgress != nil {
		onProgress(downloaded, total)
	}

	return nil
}

// fileSHA256 计算文件 SHA256
func fileSHA256(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()

	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// compareVersion 比较语义化版本号，返回 >0 表示 a > b
func compareVersion(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")

	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			numB, _ = strconv.Atoi(partsB[i])
		}
		if numA != numB {
			return numA - numB
		}
	}
	return 0
}
