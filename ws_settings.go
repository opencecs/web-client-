package main

// handleSettingsAction 处理系统设置相关的 WS 请求
func (c *WSClient) handleSettingsAction(req WSRequest) {
	switch req.Action {
	case "settings:get":
		settings := c.hub.auth.GetAllSettings()
		c.sendResponse(req.ID, true, "", settings)

	case "settings:set":
		key, _ := req.Data["key"].(string)
		value, _ := req.Data["value"].(string)
		if key == "" {
			c.sendResponse(req.ID, false, "缺少 key", nil)
			return
		}
		if err := c.hub.auth.SetSetting(key, value); err != nil {
			c.sendResponse(req.ID, false, "保存失败: "+err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "保存成功", nil)
	}
}
