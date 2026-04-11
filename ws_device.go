package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// handleDeviceAction 处理设备管理操作
func (c *WSClient) handleDeviceAction(req WSRequest) {
	switch req.Action {
	case "device:info":
		c.hub.device.cacheMu.RLock()
		cached := c.hub.device.cachedInfo
		c.hub.device.cacheMu.RUnlock()
		if cached != nil {
			c.sendResponse(req.ID, true, "ok", json.RawMessage(cached))
			return
		}
		raw, err := c.hub.deviceRequestRaw("GET", "/info/device", nil)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))

	case "device:version":
		raw, err := c.hub.deviceRequestRaw("GET", "/info", nil)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))

	case "device:mirrors":
		raw, err := c.hub.device.GetMirrorsJSON()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))

	case "device:reboot":
		raw, err := c.hub.deviceRequestRaw("GET", "/server/reboot", nil)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))

	case "device:upgrade":
		go c.handleDeviceStream(req, "GET", "/server/upgrade", 120*time.Second)

	case "device:cleanDisk":
		go c.handleDeviceStream(req, "POST", "/server/device/reset", 600*time.Second)
	}
}

// handleDeviceStream 流式设备操作（upgrade / cleanDisk）
func (c *WSClient) handleDeviceStream(req WSRequest, method, path string, timeout time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()

	reqURL := fmt.Sprintf("http://%s%s", c.hub.deviceAddr, path)
	httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, nil)
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}

	resp, err := c.hub.streamClient.Do(httpReq)
	if err != nil {
		c.sendResponse(req.ID, false, "设备连接失败: "+err.Error(), nil)
		return
	}
	defer resp.Body.Close()

	// 检查 HTTP 状态码
	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		errMsg := extractDeviceError(body, resp.StatusCode)
		c.sendResponse(req.ID, false, errMsg, nil)
		return
	}

	c.sendResponse(req.ID, true, "started", nil)

	action := req.Action

	// SSE 逐行解析
	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		data := parseSSELine(line)
		if data == "" {
			continue
		}
		c.hub.Broadcast("task:progress", map[string]interface{}{
			"action": action,
			"chunk":  data,
		})
	}

	c.hub.Broadcast("task:progress", map[string]interface{}{"action": action, "done": true})
}

// parseSSELine 解析 SSE 行，提取 data: 后的内容
func parseSSELine(line string) string {
	line = strings.TrimSpace(line)
	if line == "" || strings.HasPrefix(line, ":") {
		return "" // 空行或 SSE 注释
	}
	if strings.HasPrefix(line, "data:") {
		return strings.TrimSpace(strings.TrimPrefix(line, "data:"))
	}
	// 非 SSE 格式的行也透传（兼容非 SSE 响应）
	return line
}

// extractDeviceError 从错误响应中提取可读消息
func extractDeviceError(body []byte, statusCode int) string {
	var errResp struct {
		Message string `json:"message"`
		Error   string `json:"error"`
		Msg     string `json:"msg"`
	}
	if json.Unmarshal(body, &errResp) == nil {
		if errResp.Message != "" {
			return errResp.Message
		}
		if errResp.Error != "" {
			return errResp.Error
		}
		if errResp.Msg != "" {
			return errResp.Msg
		}
	}
	if len(body) > 0 && len(body) < 200 {
		return string(body)
	}
	return fmt.Sprintf("设备返回错误 (HTTP %d)", statusCode)
}
