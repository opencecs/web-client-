package main

import (
	"encoding/json"
)

// hasPermission 检查客户端是否有权限执行指定操作
func (c *WSClient) hasPermission(action string) bool {
	// admin 拥有全部权限
	if c.permissions == nil {
		return true
	}

	p := c.permissions

	switch action {
	// 容器操作
	case "container:start", "container:stop":
		return p.ContainerStart
	case "container:restart":
		return p.ContainerRestart
	case "container:reset":
		return p.ContainerReset
	case "container:delete":
		return p.ContainerDelete
	case "container:rename":
		return p.ContainerRename
	case "container:copy":
		return p.ContainerCopy
	case "container:changeImage":
		return p.ContainerStart // 切换镜像需要启停权限

	// 创建容器
	case "sdk:createContainer":
		return p.ContainerCreate

	// 别名
	case "alias:list", "alias:set", "alias:delete":
		return p.AliasManage

	// 备份
	case "sdk:listBackups", "sdk:deleteBackup",
		"sdk:listModelBackups", "sdk:deleteModelBackup",
		"sdk:batchChangeImage":
		return p.BackupManage

	// 镜像
	case "sdk:listImages", "sdk:deleteImage",
		"sdk:pullImage", "sdk:pruneImages":
		return p.ImageView

	// 机型/国家码（创建容器时需要）
	case "sdk:getPhoneModels", "sdk:getCountryCodes":
		return p.ContainerCreate

	// 网络
	case "sdk:listBridges", "sdk:createBridge", "sdk:updateBridge", "sdk:deleteBridge":
		return p.NetworkBridge
	case "sdk:listVpcGroups", "sdk:createVpcGroup", "sdk:deleteVpcGroup",
		"sdk:renameVpcGroup", "sdk:refreshVpcGroup", "sdk:deleteVpcNode",
		"sdk:addVpcSocks", "sdk:testVpcNode",
		"sdk:listContainerRules", "sdk:addVpcRule", "sdk:removeVpcRule",
		"sdk:addVpcRuleBatch", "sdk:removeVpcRuleBatch",
		"sdk:toggleWhiteListDns",
		"sdk:getDomainDirect", "sdk:setDomainDirect", "sdk:deleteDomainDirect",
		"sdk:getDomainFilter", "sdk:setDomainFilter", "sdk:deleteDomainFilter",
		"sdk:getGlobalDomainFilter", "sdk:setGlobalDomainFilter", "sdk:deleteGlobalDomainFilter":
		return p.VpcManage

	// 设备管理（仅 admin）
	case "device:version", "device:reboot", "device:upgrade", "device:cleanDisk":
		return false

	// 面板更新（仅 admin）
	case "panel:version":
		return true // 版本号所有人可查
	case "panel:checkUpdate", "panel:doUpdate":
		return false

	// MYT 云平台（仅 admin）
	case "myt:status", "myt:login", "myt:logout", "myt:sync",
		"myt:autoToggle", "myt:bindStatus", "myt:bind", "myt:vcode", "myt:unbind":
		return false

	// 系统设置（仅 admin）
	case "settings:get", "settings:set":
		return false

	// 用户管理（仅 admin）
	case "user:list", "user:create", "user:update", "user:delete",
		"user:getPermissions", "user:setPermissions":
		return false

	// 公开操作
	case "containers:refresh", "device:info", "device:mirrors", "myt:slotStates":
		return true

	// 投屏 token（具体坑位权限在 handleProjectionToken 中检查）
	case "projection:token":
		return p.Projection

	default:
		return false
	}
}

// canAccessSlot 检查客户端是否有权限操作指定坑位
func (c *WSClient) canAccessSlot(indexNum int) bool {
	if c.permissions == nil {
		return true // admin
	}
	for _, s := range c.permissions.Slots {
		if s == indexNum {
			return true
		}
	}
	return false
}

// getContainerSlot 从容器缓存中查找容器名对应的坑位号（使用预解析数据）
func (c *WSClient) getContainerSlot(containerName string) int {
	c.hub.containerMu.RLock()
	parsed := c.hub.parsedContainers
	c.hub.containerMu.RUnlock()

	for _, ct := range parsed {
		if ct.Name == containerName {
			return ct.IndexNum
		}
	}
	return -1
}

// checkContainerSlotAccess 检查容器操作的坑位权限，返回错误消息或空字符串
func (c *WSClient) checkContainerSlotAccess(containerName string) string {
	if c.permissions == nil {
		return "" // admin
	}
	slot := c.getContainerSlot(containerName)
	if slot <= 0 {
		return "找不到容器"
	}
	if !c.canAccessSlot(slot) {
		return "无权操作该坑位"
	}
	return ""
}

// filterByParsedSlots 根据坑位权限过滤预解析的容器列表（无需重复 JSON 解析）
func filterByParsedSlots(parsed []ParsedContainer, allowedSlots map[int]bool) json.RawMessage {
	if len(allowedSlots) == 0 {
		empty, _ := json.Marshal(map[string]interface{}{"code": 0, "data": map[string]interface{}{"list": []interface{}{}}})
		return empty
	}

	var filtered []json.RawMessage
	for _, item := range parsed {
		if allowedSlots[item.IndexNum] {
			filtered = append(filtered, item.Raw)
		}
	}

	if filtered == nil {
		filtered = []json.RawMessage{}
	}
	result, _ := json.Marshal(map[string]interface{}{
		"code": 0, "message": "OK",
		"data": map[string]interface{}{"count": len(filtered), "list": filtered},
	})
	return result
}
