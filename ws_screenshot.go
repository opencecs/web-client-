package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"
	"time"
)

const (
	screenshotInterval    = 2 * time.Second
	screenshotHTTPTimeout = 3 * time.Second
	screenshotMaxWorkers  = 12
	screenshotRetryDelay  = 500 * time.Millisecond
	screenshotMaxRetries  = 3
)

type ScreenshotEntry struct {
	Data    string
	Version int64
}

type ScreenshotCache struct {
	mu      sync.RWMutex
	entries map[int]*ScreenshotEntry
	client  *http.Client
}

func NewScreenshotCache() *ScreenshotCache {
	return &ScreenshotCache{
		entries: make(map[int]*ScreenshotEntry),
		client: &http.Client{
			Timeout: screenshotHTTPTimeout,
			Transport: &http.Transport{
				MaxIdleConns:      20,
				IdleConnTimeout:   30 * time.Second,
				MaxConnsPerHost:   5,
				DisableKeepAlives: false,
			},
		},
	}
}

// ClearSlot 清除指定坑位的截图缓存
func (sc *ScreenshotCache) ClearSlot(indexNum int) bool {
	sc.mu.Lock()
	defer sc.mu.Unlock()
	if _, ok := sc.entries[indexNum]; ok {
		delete(sc.entries, indexNum)
		return true
	}
	return false
}

// screenshotPort 根据坑位号计算截图端口（固定规则）
func screenshotPort(indexNum int) int {
	return 30000 + (indexNum-1)*100 + 1
}

func (h *WSHub) PollScreenshots(cache *ScreenshotCache) {
	// 保存引用，方便连接时推送
	h.ssCache = cache

	// 等待容器列表可用后立即抓取
	for {
		h.containerMu.RLock()
		has := h.containerCache != nil
		h.containerMu.RUnlock()
		if has {
			break
		}
		time.Sleep(200 * time.Millisecond)
	}
	log.Println("[截图] 容器列表已就绪，开始抓取截图")
	h.fetchAndPushScreenshots(cache)

	ticker := time.NewTicker(screenshotInterval)
	defer ticker.Stop()
	for range ticker.C {
		h.fetchAndPushScreenshots(cache)
	}
}

func (h *WSHub) fetchAndPushScreenshots(cache *ScreenshotCache) {
	h.containerMu.RLock()
	raw := h.containerCache
	h.containerMu.RUnlock()
	if raw == nil {
		return
	}

	// 解析运行中的容器坑位
	type cInfo struct {
		IndexNum int    `json:"indexNum"`
		Status   string `json:"status"`
	}
	var resp struct {
		Data struct {
			List []cInfo `json:"list"`
		} `json:"data"`
		List []cInfo `json:"list"`
	}
	if err := json.Unmarshal(raw, &resp); err != nil {
		return
	}
	list := resp.Data.List
	if list == nil {
		list = resp.List
	}

	var runningSlots []int
	for _, c := range list {
		if c.Status == "running" && c.IndexNum > 0 {
			runningSlots = append(runningSlots, c.IndexNum)
		}
	}

	if len(runningSlots) == 0 {
		cache.mu.Lock()
		changed := len(cache.entries) > 0
		cache.entries = make(map[int]*ScreenshotEntry)
		cache.mu.Unlock()
		if changed {
			h.pushScreenshots(cache)
		}
		return
	}

	// 并发抓取，失败重试
	sem := make(chan struct{}, screenshotMaxWorkers)
	var wg sync.WaitGroup
	var updated bool
	var updatedMu sync.Mutex

	for _, slot := range runningSlots {
		wg.Add(1)
		sem <- struct{}{}
		go func(idx int) {
			defer func() { <-sem; wg.Done() }()
			url := fmt.Sprintf("http://127.0.0.1:%d/task=snap&level=1", screenshotPort(idx))

			// 带重试的抓取
			var data string
			for attempt := 0; attempt <= screenshotMaxRetries; attempt++ {
				data = fetchScreenshot(cache.client, url)
				if data != "" {
					break
				}
				if attempt < screenshotMaxRetries {
					time.Sleep(screenshotRetryDelay)
				}
			}
			if data == "" {
				return
			}

			now := time.Now().UnixMilli()
			cache.mu.Lock()
			old := cache.entries[idx]
			if old == nil || len(old.Data) != len(data) {
				cache.entries[idx] = &ScreenshotEntry{Data: data, Version: now}
				updatedMu.Lock()
				updated = true
				updatedMu.Unlock()
			}
			cache.mu.Unlock()
		}(slot)
	}
	wg.Wait()

	// 清理不再运行的坑位
	runMap := make(map[int]bool, len(runningSlots))
	for _, s := range runningSlots {
		runMap[s] = true
	}
	cache.mu.Lock()
	for slot := range cache.entries {
		if !runMap[slot] {
			delete(cache.entries, slot)
			updated = true
		}
	}
	cache.mu.Unlock()

	if updated {
		h.pushScreenshots(cache)
	}
}

// pushScreenshotsToClient 向单个客户端推送截图缓存
func (h *WSHub) pushScreenshotsToClient(client *WSClient, cache *ScreenshotCache) {
	cache.mu.RLock()
	if len(cache.entries) == 0 {
		cache.mu.RUnlock()
		return
	}
	filtered := make(map[string]string)
	for slot, entry := range cache.entries {
		if client.canAccessSlot(slot) {
			filtered[fmt.Sprintf("%d", slot)] = entry.Data
		}
	}
	cache.mu.RUnlock()

	if len(filtered) == 0 {
		return
	}

	msg, _ := json.Marshal(map[string]interface{}{
		"type": "event", "event": "screenshots",
		"data": filtered,
	})
	func() {
		defer func() { recover() }()
		select {
		case client.send <- msg:
		default:
		}
	}()
}

func (h *WSHub) pushScreenshots(cache *ScreenshotCache) {
	cache.mu.RLock()
	all := make(map[string]string, len(cache.entries))
	for slot, entry := range cache.entries {
		all[fmt.Sprintf("%d", slot)] = entry.Data
	}
	cache.mu.RUnlock()

	h.mu.RLock()
	defer h.mu.RUnlock()

	for client := range h.clients {
		filtered := make(map[string]string)
		for slotStr, data := range all {
			var num int
			fmt.Sscanf(slotStr, "%d", &num)
			if client.canAccessSlot(num) {
				filtered[slotStr] = data
			}
		}
		msg, _ := json.Marshal(map[string]interface{}{
			"type": "event", "event": "screenshots",
			"data": filtered,
		})
		func() {
			defer func() { recover() }()
			select {
			case client.send <- msg:
			default:
			}
		}()
	}
}

func fetchScreenshot(client *http.Client, url string) string {
	ctx, cancel := context.WithTimeout(context.Background(), screenshotHTTPTimeout)
	defer cancel()

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return ""
	}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return ""
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil || len(body) == 0 {
		return ""
	}

	contentType := resp.Header.Get("Content-Type")
	if contentType == "" {
		contentType = "image/jpeg"
	}
	for i, c := range contentType {
		if c == ';' || c == ' ' {
			contentType = contentType[:i]
			break
		}
	}
	return "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(body)
}
