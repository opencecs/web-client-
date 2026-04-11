package main

// handleMytAction 处理 MYT 云平台操作
func (c *WSClient) handleMytAction(req WSRequest) {
	switch req.Action {
	case "myt:status":
		data := c.hub.mytAuth.GetStatus()
		c.sendResponse(req.ID, true, "ok", data)

	case "myt:slotStates":
		data, err := c.hub.mytAuth.GetSlotStates()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", data)

	case "myt:login":
		username := getStr(req.Data, "username")
		password := getStr(req.Data, "password")
		result, err := c.hub.mytAuth.DoLogin(username, password)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", result)

	case "myt:logout":
		c.hub.mytAuth.DoLogout()
		c.sendResponse(req.ID, true, "ok", nil)

	case "myt:sync":
		result, err := c.hub.mytAuth.DoSync()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", result)

	case "myt:autoToggle":
		autoSync := getBool(req.Data, "autoSync")
		result, err := c.hub.mytAuth.DoAutoToggle(autoSync)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", result)

	case "myt:bindStatus":
		data, err := c.hub.mytAuth.GetBindStatus()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", data)

	case "myt:bind":
		data, err := c.hub.mytAuth.DoBind()
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", data)

	case "myt:vcode":
		phone := getStr(req.Data, "phone")
		data, err := c.hub.mytAuth.DoGetVCode(phone)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", data)

	case "myt:unbind":
		vcode := getStr(req.Data, "vcode")
		vkey := getStr(req.Data, "vkey")
		err := c.hub.mytAuth.DoUnbind(vcode, vkey)
		if err != nil {
			c.sendResponse(req.ID, false, err.Error(), nil)
			return
		}
		c.sendResponse(req.ID, true, "ok", nil)
	}
}
