package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// handleSDKAction 处理 SDK 相关操作（镜像、网络）
func (c *WSClient) handleSDKAction(req WSRequest) {
	switch req.Action {
	// 容器创建
	case "sdk:createContainer":
		dataJSON, _ := json.Marshal(req.Data)
		log.Printf("[SDK] 创建容器请求: %s", string(dataJSON))
		c.sdkAction(req, "POST", "/android", req.Data, nil, 120*time.Second)

	// 镜像
	case "sdk:listImages":
		q := url.Values{}
		if name := getStr(req.Data, "imageName"); name != "" {
			q.Set("imageName", name)
		}
		c.sdkQuery(req, "GET", "/android/image", q)
	case "sdk:deleteImage":
		q := url.Values{}
		q.Set("image", getStr(req.Data, "image"))
		c.sdkQuery(req, "DELETE", "/android/image", q)
	case "sdk:pullImage":
		go c.handlePullImage(req)
	case "sdk:pruneImages":
		c.sdkQuery(req, "POST", "/android/pruneImages", nil)

	// 机型
	case "sdk:getPhoneModels":
		c.sdkQuery(req, "GET", "/android/phoneModel", nil)
	case "sdk:getCountryCodes":
		c.sdkQuery(req, "GET", "/android/countryCode", nil)

	// 网络
	case "sdk:listBridges":
		c.sdkQuery(req, "GET", "/mytBridge", nil)
	case "sdk:createBridge":
		c.sdkAction(req, "POST", "/mytBridge", req.Data, nil, 0)
	case "sdk:updateBridge":
		c.sdkAction(req, "PUT", "/mytBridge", req.Data, nil, 0)
	case "sdk:deleteBridge":
		q := url.Values{}
		q.Set("name", getStr(req.Data, "name"))
		c.sdkQuery(req, "DELETE", "/mytBridge", q)
	case "sdk:listVpcGroups":
		q := url.Values{}
		if alias := getStr(req.Data, "alias"); alias != "" {
			q.Set("alias", alias)
		}
		c.sdkQuery(req, "GET", "/mytVpc/group", q)
	case "sdk:createVpcGroup":
		c.sdkAction(req, "POST", "/mytVpc/group", req.Data, nil, 0)
	case "sdk:deleteVpcGroup":
		q := url.Values{}
		q.Set("id", getStr(req.Data, "id"))
		c.sdkQuery(req, "DELETE", "/mytVpc/group", q)
	case "sdk:listContainerRules":
		c.sdkQuery(req, "GET", "/mytVpc/containerRule", nil)
	case "sdk:addVpcRule":
		c.sdkAction(req, "POST", "/mytVpc/addRule", req.Data, nil, 0)
	case "sdk:removeVpcRule":
		c.sdkAction(req, "POST", "/mytVpc/delRule", req.Data, nil, 0)

	// 备份管理
	case "sdk:listBackups":
		c.sdkQuery(req, "GET", "/backup", nil)
	case "sdk:deleteBackup":
		q := url.Values{}
		q.Set("name", getStr(req.Data, "name"))
		c.sdkQuery(req, "DELETE", "/backup", q)
	case "sdk:listModelBackups":
		c.sdkQuery(req, "GET", "/android/backup/model", nil)
	case "sdk:deleteModelBackup":
		q := url.Values{}
		q.Set("name", getStr(req.Data, "name"))
		c.sdkQuery(req, "DELETE", "/android/backup/model", q)
	case "sdk:batchChangeImage":
		c.sdkAction(req, "POST", "/android/change-image", req.Data, nil, 120*time.Second)
	}
}

// sdkQuery 通用 SDK GET/DELETE 查询
func (c *WSClient) sdkQuery(req WSRequest, method, path string, query url.Values) {
	raw, err := c.hub.sdkRequest(method, path, nil, query)
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}
	c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))
}

// sdkAction 通用 SDK POST/PUT 操作
func (c *WSClient) sdkAction(req WSRequest, method, path string, body interface{}, query url.Values, timeout time.Duration) {
	if timeout > 0 {
		reqURL := fmt.Sprintf("http://%s%s", c.hub.deviceAddr, path)
		if query != nil && len(query) > 0 {
			reqURL += "?" + query.Encode()
		}
		var bodyReader io.Reader
		if body != nil {
			data, _ := json.Marshal(body)
			bodyReader = strings.NewReader(string(data))
		}
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		httpReq, err := http.NewRequestWithContext(ctx, method, reqURL, bodyReader)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		if body != nil {
			httpReq.Header.Set("Content-Type", "application/json")
		}
		resp, err := c.hub.httpClient.Do(httpReq)
		if err != nil {
			c.sendResponse(req.ID, false, "设备连接失败: "+err.Error(), nil)
			return
		}
		defer resp.Body.Close()
		respBody, _ := io.ReadAll(resp.Body)
		log.Printf("[SDK] %s %s → HTTP %d, 响应: %s", method, path, resp.StatusCode, string(respBody))
		if resp.StatusCode >= 400 {
			c.sendResponse(req.ID, false, fmt.Sprintf("操作失败 (HTTP %d)", resp.StatusCode), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", json.RawMessage(respBody))
		return
	}
	raw, err := c.hub.sdkRequest(method, path, body, query)
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}
	c.sendResponse(req.ID, true, "ok", json.RawMessage(raw))
}

// handlePullImage 流式拉取镜像
func (c *WSClient) handlePullImage(req WSRequest) {
	imageUrl := getStr(req.Data, "imageUrl")
	if imageUrl == "" {
		c.sendResponse(req.ID, false, "缺少 imageUrl", nil)
		return
	}

	reqURL := fmt.Sprintf("http://%s/android/pullImage", c.hub.deviceAddr)
	data, _ := json.Marshal(map[string]string{"imageUrl": imageUrl})
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	httpReq, err := http.NewRequestWithContext(ctx, "POST", reqURL, strings.NewReader(string(data)))
	if err != nil {
		c.sendResponse(req.ID, false, err.Error(), nil)
		return
	}
	httpReq.Header.Set("Content-Type", "application/json")

	resp, err := c.hub.streamClient.Do(httpReq)
	if err != nil {
		c.sendResponse(req.ID, false, "设备连接失败: "+err.Error(), nil)
		return
	}
	defer resp.Body.Close()

	log.Printf("[PullImage] 开始拉取: %s (HTTP %d)", imageUrl, resp.StatusCode)

	if resp.StatusCode >= 400 {
		body, _ := io.ReadAll(resp.Body)
		errMsg := fmt.Sprintf("拉取失败 (HTTP %d): %s", resp.StatusCode, string(body))
		log.Printf("[PullImage] %s", errMsg)
		c.sendResponse(req.ID, false, errMsg, nil)
		return
	}

	c.sendResponse(req.ID, true, "pulling", nil)

	buf := make([]byte, 4096)
	for {
		n, readErr := resp.Body.Read(buf)
		if n > 0 {
			chunk := string(buf[:n])
			log.Printf("[PullImage] chunk(%d bytes): %s", n, chunk)

			// 检测镜像已存在，直接完成
			if strings.Contains(chunk, "Image already exists") || strings.Contains(chunk, "No operation") {
				log.Printf("[PullImage] 镜像已存在，跳过下载")
				c.hub.Broadcast("task:progress", map[string]interface{}{"action": "pullImage", "imageUrl": imageUrl, "done": true, "exists": true})
				return
			}

			c.hub.Broadcast("task:progress", map[string]interface{}{
				"action":   "pullImage",
				"imageUrl": imageUrl,
				"chunk":    chunk,
			})
		}
		if readErr != nil {
			log.Printf("[PullImage] 流结束: %v", readErr)
			break
		}
	}

	log.Printf("[PullImage] 完成: %s", imageUrl)
	c.hub.Broadcast("task:progress", map[string]interface{}{"action": "pullImage", "imageUrl": imageUrl, "done": true})
}
