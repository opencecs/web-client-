package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// containerAPIPort 计算容器内部安卓 API 端口（非桥接模式）
// 复用 upload_proxy.go 中的同名函数
func (c *WSClient) containerAPIPort(containerName string) int {
	return containerAPIPort(c.hub, containerName)
}

// handleProxyAction 处理容器 S5 代理操作
func (c *WSClient) handleProxyAction(req WSRequest) {
	name := getStr(req.Data, "name")
	if name == "" {
		c.sendResponse(req.ID, false, "缺少容器名称", nil)
		return
	}

	// 坑位权限检查
	if msg := c.checkContainerSlotAccess(name); msg != "" {
		c.sendResponse(req.ID, false, msg, nil)
		return
	}

	port := c.containerAPIPort(name)
	if port == 0 {
		c.sendResponse(req.ID, false, "找不到容器或无法计算端口", nil)
		return
	}

	switch req.Action {
	case "proxy:status":
		c.proxyRequest(req, port, "GET", fmt.Sprintf("/proxy?cmd=1"), nil)

	case "proxy:set":
		addr := getStr(req.Data, "addr")
		s5Port := getStr(req.Data, "port")
		usr := getStr(req.Data, "usr")
		pwd := getStr(req.Data, "pwd")
		s5Type := getStr(req.Data, "type")
		if addr == "" || s5Port == "" {
			c.sendResponse(req.ID, false, "IP 和端口不能为空", nil)
			return
		}
		if s5Type == "" {
			s5Type = "1"
		}
		q := url.Values{}
		q.Set("cmd", "2")
		q.Set("addr", addr)
		q.Set("port", s5Port)
		q.Set("usr", usr)
		q.Set("pwd", pwd)
		q.Set("type", s5Type)
		c.proxyRequest(req, port, "GET", "/proxy?"+q.Encode(), nil)

	case "proxy:stop":
		c.proxyRequest(req, port, "GET", "/proxy?cmd=3", nil)

	case "clipboard:get":
		c.proxyRequest(req, port, "GET", "/clipboard?cmd=1", nil)

	case "clipboard:set":
		text := getStr(req.Data, "text")
		q := url.Values{}
		q.Set("cmd", "2")
		q.Set("text", text)
		c.proxyRequest(req, port, "GET", "/clipboard?"+q.Encode(), nil)

	case "android:shake":
		c.proxyRequest(req, port, "GET", "/modifydev?cmd=17&shake=1", nil)

	case "android:ping":
		// 轻量 ping：请求容器 API 测延迟
		start := time.Now()
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%d/proxy?cmd=1", port))
		latency := time.Since(start).Milliseconds()
		if err != nil {
			c.sendResponse(req.ID, true, "ok", map[string]interface{}{"latency": -1})
			return
		}
		resp.Body.Close()
		c.sendResponse(req.ID, true, "ok", map[string]interface{}{"latency": latency})

	case "android:sms":
		address := getStr(req.Data, "address")
		body := getStr(req.Data, "body")
		if address == "" || body == "" {
			c.sendResponse(req.ID, false, "发送号码和内容不能为空", nil)
			return
		}
		// POST /sms?cmd=4 + JSON body {"address":"xxx","body":"xxx"}
		smsBody := map[string]string{"address": address, "body": body}
		c.proxyRequest(req, port, "POST", "/sms?cmd=4", smsBody)
	}
}

// proxyRequest 向容器内部 API 发送 HTTP 请求
func (c *WSClient) proxyRequest(req WSRequest, port int, method, path string, body interface{}) {
	reqURL := fmt.Sprintf("http://%s:%d%s", "127.0.0.1", port, path)

	var bodyReader io.Reader
	if body != nil {
		data, _ := json.Marshal(body)
		bodyReader = strings.NewReader(string(data))
	}

	client := &http.Client{Timeout: 10 * time.Second}
	httpReq, err := http.NewRequest(method, reqURL, bodyReader)
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}
	if body != nil {
		httpReq.Header.Set("Content-Type", "application/json")
	}

	resp, err := client.Do(httpReq)
	if err != nil {
		c.sendResponse(req.ID, false, "容器未响应: "+err.Error(), nil)
		return
	}
	defer resp.Body.Close()

	respBody, _ := io.ReadAll(resp.Body)
	log.Printf("[Proxy] %s %s → HTTP %d", method, reqURL, resp.StatusCode)

	if resp.StatusCode >= 400 {
		c.sendResponse(req.ID, false, fmt.Sprintf("容器返回错误 (HTTP %d)", resp.StatusCode), nil)
		return
	}
	c.sendResponse(req.ID, true, "ok", json.RawMessage(respBody))
}
