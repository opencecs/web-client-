package main

import (
	"encoding/json"
	"fmt"
	"net/url"
	"time"
)

// handleContainerAction 处理容器相关操作
func (c *WSClient) handleContainerAction(req WSRequest) {
	name := getStr(req.Data, "name")

	// 非 admin 用户检查坑位权限
	if name != "" {
		if msg := c.checkContainerSlotAccess(name); msg != "" {
			c.sendResponse(req.ID, false, msg, nil)
			return
		}
	}

	switch req.Action {
	case "container:start":
		c.containerSimple(req, "POST", "/android/start")
	case "container:stop":
		c.containerSimple(req, "POST", "/android/stop")
	case "container:restart":
		c.containerSimple(req, "POST", "/android/restart")
	case "container:reset":
		name := getStr(req.Data, "name")
		body, status, err := c.hub.deviceRequest("PUT", "/android", map[string]interface{}{"name": name, "start": true})
		c.handleDeviceResponse(req.ID, body, status, err)
		c.clearSlotScreenshot(name)
	case "container:delete":
		name := getStr(req.Data, "name")
		c.clearSlotScreenshot(name)
		_, status, err := c.hub.deviceRequest("DELETE", "/android?name="+url.QueryEscape(name), nil)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
		} else if status >= 400 {
			c.sendResponse(req.ID, false, "删除失败", nil)
		} else {
			c.sendResponse(req.ID, true, "删除成功", nil)
		}
		c.hub.TriggerRefresh()
	case "container:rename":
		name := getStr(req.Data, "name")
		newName := getStr(req.Data, "newName")
		body, status, err := c.hub.deviceRequest("POST", "/android/rename", map[string]interface{}{"name": name, "newName": newName})
		c.handleDeviceResponse(req.ID, body, status, err)
	case "container:copy":
		name := getStr(req.Data, "name")
		indexNum := getNum(req.Data, "indexNum")
		count := getNum(req.Data, "count")
		// 检查目标坑位权限
		if !c.isAdmin && c.permissions != nil {
			for i := 0; i < count; i++ {
				if !c.canAccessSlot(indexNum + i) {
					c.sendResponse(req.ID, false, fmt.Sprintf("无权操作坑位 %d", indexNum+i), nil)
					return
				}
			}
		}
		path := fmt.Sprintf("/android/copy?name=%s&indexNum=%d&count=%d", url.QueryEscape(name), indexNum, count)
		body, status, err := c.hub.deviceRequest("GET", path, nil)
		c.handleDeviceResponse(req.ID, body, status, err)
	}
}

// handleAliasAction 处理别名操作
func (c *WSClient) handleAliasAction(req WSRequest) {
	// alias:set 和 alias:delete 需要检查坑位权限
	if req.Action != "alias:list" {
		name := getStr(req.Data, "name")
		if name != "" {
			if msg := c.checkContainerSlotAccess(name); msg != "" {
				c.sendResponse(req.ID, false, msg, nil)
				return
			}
		}
	}

	switch req.Action {
	case "alias:list":
		aliases := c.hub.alias.GetAllAliases()
		c.sendResponse(req.ID, true, "ok", map[string]interface{}{"aliases": aliases})
	case "alias:set":
		name := getStr(req.Data, "name")
		alias := getStr(req.Data, "alias")
		if err := c.hub.alias.SetAlias(name, alias); err != nil {
			c.sendResponse(req.ID, false, "设置别名失败: "+err.Error(), nil)
		} else {
			c.sendResponse(req.ID, true, "ok", nil)
			c.hub.Broadcast("aliases:list", c.hub.alias.GetAllAliases())
		}
	case "alias:delete":
		name := getStr(req.Data, "name")
		if err := c.hub.alias.DeleteAlias(name); err != nil {
			c.sendResponse(req.ID, false, "删除别名失败: "+err.Error(), nil)
		} else {
			c.sendResponse(req.ID, true, "ok", nil)
			c.hub.Broadcast("aliases:list", c.hub.alias.GetAllAliases())
		}
	}
}

// containerSimple 简单容器操作（start/stop/restart）
func (c *WSClient) containerSimple(req WSRequest, method, path string) {
	name := getStr(req.Data, "name")
	body, status, err := c.hub.deviceRequest(method, path, map[string]interface{}{"name": name})
	c.handleDeviceResponse(req.ID, body, status, err)
	// 非 start 操作清除截图缓存
	if req.Action != "container:start" {
		c.clearSlotScreenshot(name)
	}
	time.AfterFunc(1*time.Second, func() { c.hub.TriggerRefresh() })
}

// handleDeviceResponse 处理设备响应
func (c *WSClient) handleDeviceResponse(reqID string, body []byte, status int, err error) {
	if err != nil {
		c.sendResponse(reqID, false, "设备连接失败: "+err.Error(), nil)
		return
	}
	if status >= 400 {
		var errResp map[string]interface{}
		if json.Unmarshal(body, &errResp) == nil {
			if msg, ok := errResp["message"].(string); ok {
				c.sendResponse(reqID, false, msg, nil)
				return
			}
			if msg, ok := errResp["error"].(string); ok {
				c.sendResponse(reqID, false, msg, nil)
				return
			}
		}
		c.sendResponse(reqID, false, fmt.Sprintf("操作失败 (HTTP %d)", status), nil)
		return
	}
	c.sendResponse(reqID, true, "操作成功", nil)
}

// clearSlotScreenshot 清除容器对应坑位的截图缓存并推送更新
func (c *WSClient) clearSlotScreenshot(containerName string) {
	if c.hub.ssCache == nil {
		return
	}
	slot := c.getContainerSlot(containerName)
	if slot <= 0 {
		return
	}
	if c.hub.ssCache.ClearSlot(slot) {
		c.hub.pushScreenshots(c.hub.ssCache)
	}
}
