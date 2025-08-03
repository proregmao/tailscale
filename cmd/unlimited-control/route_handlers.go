package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"

	"github.com/gin-gonic/gin"
)

// RouteManager 路由管理器
type RouteManager struct {
	server *UnlimitedControlServer
}

// NewRouteManager 创建路由管理器
func NewRouteManager(server *UnlimitedControlServer) *RouteManager {
	return &RouteManager{
		server: server,
	}
}

// ValidateRoute 验证路由前缀
func (rm *RouteManager) ValidateRoute(prefix string) error {
	// 检查是否为有效的CIDR格式
	_, _, err := net.ParseCIDR(prefix)
	if err != nil {
		return fmt.Errorf("invalid CIDR format: %s", prefix)
	}
	return nil
}

// IsDefaultRoute 检查是否为默认路由
func (rm *RouteManager) IsDefaultRoute(prefix string) bool {
	return prefix == "0.0.0.0/0" || prefix == "::/0"
}

// UpdateDeviceRoutes 更新设备的路由信息
func (rm *RouteManager) UpdateDeviceRoutes(deviceID uint) error {
	var routes []Route
	if err := rm.server.db.Where("device_id = ? AND enabled = ?", deviceID, true).Find(&routes).Error; err != nil {
		return err
	}

	// 构建启用的路由列表
	enabledRoutes := make([]string, 0)
	advertiseRoutes := make([]string, 0)
	
	for _, route := range routes {
		if route.Enabled {
			enabledRoutes = append(enabledRoutes, route.Prefix)
		}
		if route.Advertised {
			advertiseRoutes = append(advertiseRoutes, route.Prefix)
		}
	}

	// 更新设备记录
	enabledRoutesJSON, _ := json.Marshal(enabledRoutes)
	advertiseRoutesJSON, _ := json.Marshal(advertiseRoutes)

	return rm.server.db.Model(&Device{}).Where("id = ?", deviceID).Updates(map[string]interface{}{
		"enabled_routes":   string(enabledRoutesJSON),
		"advertise_routes": string(advertiseRoutesJSON),
	}).Error
}

// ===== 路由管理处理函数 =====

// handleListRoutes 获取路由列表
func (s *UnlimitedControlServer) handleListRoutes(c *gin.Context) {
	var routes []Route
	
	query := s.db.Model(&Route{}).Preload("Device").Preload("Device.User")
	
	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	
	// 支持按类型过滤
	if exitNode := c.Query("exit_node"); exitNode == "true" {
		query = query.Where("exit_node = ?", true)
	}
	
	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch routes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    routes,
		"total":   len(routes),
	})
}

// handleCreateRoute 创建路由
func (s *UnlimitedControlServer) handleCreateRoute(c *gin.Context) {
	var req Route
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 验证路由
	rm := NewRouteManager(s)
	if err := rm.ValidateRoute(req.Prefix); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查设备是否存在
	var device Device
	if err := s.db.First(&device, req.DeviceID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Device not found",
		})
		return
	}

	// 检查路由是否已存在
	var existing Route
	if err := s.db.Where("device_id = ? AND prefix = ?", req.DeviceID, req.Prefix).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Route already exists for this device",
		})
		return
	}

	// 检查是否为默认路由（出口节点）
	if rm.IsDefaultRoute(req.Prefix) {
		req.ExitNode = true
	}

	// 设置默认值
	if req.Description == "" {
		if req.ExitNode {
			req.Description = "Exit node route"
		} else {
			req.Description = fmt.Sprintf("Subnet route for %s", req.Prefix)
		}
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create route",
		})
		return
	}

	// 更新设备路由信息
	rm.UpdateDeviceRoutes(req.DeviceID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetRoute 获取单个路由
func (s *UnlimitedControlServer) handleGetRoute(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid route ID",
		})
		return
	}

	var route Route
	if err := s.db.Preload("Device").Preload("Device.User").First(&route, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Route not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    route,
	})
}

// handleUpdateRoute 更新路由
func (s *UnlimitedControlServer) handleUpdateRoute(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid route ID",
		})
		return
	}

	var req Route
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var route Route
	if err := s.db.First(&route, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Route not found",
		})
		return
	}

	// 验证新的路由前缀
	if req.Prefix != route.Prefix {
		rm := NewRouteManager(s)
		if err := rm.ValidateRoute(req.Prefix); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		// 检查新路由是否已存在
		var existing Route
		if err := s.db.Where("device_id = ? AND prefix = ? AND id != ?", 
			route.DeviceID, req.Prefix, route.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Route already exists for this device",
			})
			return
		}
	}

	// 更新路由
	route.Prefix = req.Prefix
	route.Advertised = req.Advertised
	route.Enabled = req.Enabled
	route.Primary = req.Primary
	route.Description = req.Description

	// 检查是否为默认路由
	rm := NewRouteManager(s)
	route.ExitNode = rm.IsDefaultRoute(route.Prefix)

	if err := s.db.Save(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update route",
		})
		return
	}

	// 更新设备路由信息
	rm.UpdateDeviceRoutes(route.DeviceID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    route,
	})
}

// handleDeleteRoute 删除路由
func (s *UnlimitedControlServer) handleDeleteRoute(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid route ID",
		})
		return
	}

	var route Route
	if err := s.db.First(&route, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Route not found",
		})
		return
	}

	deviceID := route.DeviceID

	if err := s.db.Delete(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete route",
		})
		return
	}

	// 更新设备路由信息
	rm := NewRouteManager(s)
	rm.UpdateDeviceRoutes(deviceID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Route deleted successfully",
	})
}

// handleEnableRoute 启用路由
func (s *UnlimitedControlServer) handleEnableRoute(c *gin.Context) {
	s.toggleRoute(c, true)
}

// handleDisableRoute 禁用路由
func (s *UnlimitedControlServer) handleDisableRoute(c *gin.Context) {
	s.toggleRoute(c, false)
}

// toggleRoute 切换路由状态
func (s *UnlimitedControlServer) toggleRoute(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid route ID",
		})
		return
	}

	var route Route
	if err := s.db.First(&route, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Route not found",
		})
		return
	}

	route.Enabled = enabled
	if err := s.db.Save(&route).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update route status",
		})
		return
	}

	// 更新设备路由信息
	rm := NewRouteManager(s)
	rm.UpdateDeviceRoutes(route.DeviceID)

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Route %s successfully", action),
		"data":    route,
	})
}

// handleGetDeviceRoutes 获取设备的所有路由
func (s *UnlimitedControlServer) handleGetDeviceRoutes(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	// 检查设备是否存在
	var device Device
	if err := s.db.First(&device, uint(deviceID)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Device not found",
		})
		return
	}

	var routes []Route
	if err := s.db.Where("device_id = ?", uint(deviceID)).Order("created_at DESC").Find(&routes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch device routes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    routes,
		"total":   len(routes),
		"device":  device,
	})
}

// ===== 出口节点管理处理函数 =====

// handleListExitNodes 获取出口节点列表
func (s *UnlimitedControlServer) handleListExitNodes(c *gin.Context) {
	var exitNodes []ExitNodeConfig

	query := s.db.Model(&ExitNodeConfig{}).Preload("Device").Preload("Device.User")

	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&exitNodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch exit nodes",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    exitNodes,
		"total":   len(exitNodes),
	})
}

// handleCreateExitNode 创建出口节点配置
func (s *UnlimitedControlServer) handleCreateExitNode(c *gin.Context) {
	var req ExitNodeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查设备是否存在
	var device Device
	if err := s.db.First(&device, req.DeviceID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Device not found",
		})
		return
	}

	// 检查是否已存在出口节点配置
	var existing ExitNodeConfig
	if err := s.db.Where("device_id = ?", req.DeviceID).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Exit node configuration already exists for this device",
		})
		return
	}

	// 设置默认DNS配置
	if req.DNSConfig == "" {
		defaultDNS := map[string]interface{}{
			"nameservers": []string{"8.8.8.8", "8.8.4.4"},
			"search_domains": []string{},
		}
		dnsJSON, _ := json.Marshal(defaultDNS)
		req.DNSConfig = string(dnsJSON)
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create exit node configuration",
		})
		return
	}

	// 如果启用了出口节点，创建默认路由
	if req.Enabled && req.AdvertiseDefaultRoute {
		s.createDefaultRoutes(req.DeviceID)
	}

	// 更新设备的出口节点状态
	s.db.Model(&device).Update("exit_node", req.Enabled)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// createDefaultRoutes 为出口节点创建默认路由
func (s *UnlimitedControlServer) createDefaultRoutes(deviceID uint) {
	// 创建IPv4默认路由
	ipv4Route := Route{
		DeviceID:    deviceID,
		Prefix:      "0.0.0.0/0",
		Advertised:  true,
		Enabled:     true,
		Primary:     true,
		ExitNode:    true,
		Description: "IPv4 default route (exit node)",
	}

	// 检查是否已存在
	var existing Route
	if err := s.db.Where("device_id = ? AND prefix = ?", deviceID, "0.0.0.0/0").First(&existing).Error; err != nil {
		s.db.Create(&ipv4Route)
	}

	// 创建IPv6默认路由
	ipv6Route := Route{
		DeviceID:    deviceID,
		Prefix:      "::/0",
		Advertised:  true,
		Enabled:     true,
		Primary:     true,
		ExitNode:    true,
		Description: "IPv6 default route (exit node)",
	}

	if err := s.db.Where("device_id = ? AND prefix = ?", deviceID, "::/0").First(&existing).Error; err != nil {
		s.db.Create(&ipv6Route)
	}

	// 更新设备路由信息
	rm := NewRouteManager(s)
	rm.UpdateDeviceRoutes(deviceID)
}

// handleGetExitNode 获取单个出口节点配置
func (s *UnlimitedControlServer) handleGetExitNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid exit node ID",
		})
		return
	}

	var exitNode ExitNodeConfig
	if err := s.db.Preload("Device").Preload("Device.User").First(&exitNode, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Exit node configuration not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    exitNode,
	})
}

// handleUpdateExitNode 更新出口节点配置
func (s *UnlimitedControlServer) handleUpdateExitNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid exit node ID",
		})
		return
	}

	var req ExitNodeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var exitNode ExitNodeConfig
	if err := s.db.First(&exitNode, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Exit node configuration not found",
		})
		return
	}

	// 更新配置
	wasEnabled := exitNode.Enabled
	exitNode.Enabled = req.Enabled
	exitNode.AllowLANAccess = req.AllowLANAccess
	exitNode.AdvertiseDefaultRoute = req.AdvertiseDefaultRoute
	exitNode.DNSConfig = req.DNSConfig

	if err := s.db.Save(&exitNode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update exit node configuration",
		})
		return
	}

	// 如果状态发生变化，更新相关路由
	if !wasEnabled && exitNode.Enabled && exitNode.AdvertiseDefaultRoute {
		s.createDefaultRoutes(exitNode.DeviceID)
	} else if wasEnabled && !exitNode.Enabled {
		// 禁用默认路由
		s.db.Model(&Route{}).Where("device_id = ? AND exit_node = ?", exitNode.DeviceID, true).Update("enabled", false)
		rm := NewRouteManager(s)
		rm.UpdateDeviceRoutes(exitNode.DeviceID)
	}

	// 更新设备的出口节点状态
	s.db.Model(&Device{}).Where("id = ?", exitNode.DeviceID).Update("exit_node", exitNode.Enabled)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    exitNode,
	})
}

// handleDeleteExitNode 删除出口节点配置
func (s *UnlimitedControlServer) handleDeleteExitNode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid exit node ID",
		})
		return
	}

	var exitNode ExitNodeConfig
	if err := s.db.First(&exitNode, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Exit node configuration not found",
		})
		return
	}

	deviceID := exitNode.DeviceID

	// 删除相关的默认路由
	s.db.Where("device_id = ? AND exit_node = ?", deviceID, true).Delete(&Route{})

	// 删除出口节点配置
	if err := s.db.Delete(&exitNode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete exit node configuration",
		})
		return
	}

	// 更新设备状态
	s.db.Model(&Device{}).Where("id = ?", deviceID).Update("exit_node", false)

	// 更新设备路由信息
	rm := NewRouteManager(s)
	rm.UpdateDeviceRoutes(deviceID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Exit node configuration deleted successfully",
	})
}

// handleEnableExitNode 启用出口节点
func (s *UnlimitedControlServer) handleEnableExitNode(c *gin.Context) {
	s.toggleExitNode(c, true)
}

// handleDisableExitNode 禁用出口节点
func (s *UnlimitedControlServer) handleDisableExitNode(c *gin.Context) {
	s.toggleExitNode(c, false)
}

// toggleExitNode 切换出口节点状态
func (s *UnlimitedControlServer) toggleExitNode(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid exit node ID",
		})
		return
	}

	var exitNode ExitNodeConfig
	if err := s.db.First(&exitNode, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Exit node configuration not found",
		})
		return
	}

	exitNode.Enabled = enabled
	if err := s.db.Save(&exitNode).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update exit node status",
		})
		return
	}

	// 更新相关路由
	if enabled && exitNode.AdvertiseDefaultRoute {
		s.createDefaultRoutes(exitNode.DeviceID)
	} else if !enabled {
		s.db.Model(&Route{}).Where("device_id = ? AND exit_node = ?", exitNode.DeviceID, true).Update("enabled", false)
		rm := NewRouteManager(s)
		rm.UpdateDeviceRoutes(exitNode.DeviceID)
	}

	// 更新设备状态
	s.db.Model(&Device{}).Where("id = ?", exitNode.DeviceID).Update("exit_node", enabled)

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Exit node %s successfully", action),
		"data":    exitNode,
	})
}

// handleGetAvailableExitNodes 获取可用的出口节点
func (s *UnlimitedControlServer) handleGetAvailableExitNodes(c *gin.Context) {
	var exitNodes []ExitNodeConfig

	if err := s.db.Where("enabled = ?", true).Preload("Device").Preload("Device.User").Find(&exitNodes).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch available exit nodes",
		})
		return
	}

	// 构建可用出口节点列表
	availableNodes := make([]map[string]interface{}, 0)
	for _, exitNode := range exitNodes {
		var ips []string
		json.Unmarshal([]byte(exitNode.Device.IPAddresses), &ips)

		node := map[string]interface{}{
			"id":          exitNode.ID,
			"device_id":   exitNode.DeviceID,
			"hostname":    exitNode.Device.Hostname,
			"given_name":  exitNode.Device.GivenName,
			"ip_addresses": ips,
			"user":        exitNode.Device.User.Name,
			"online":      exitNode.Device.Online,
			"allow_lan_access": exitNode.AllowLANAccess,
			"last_seen":   exitNode.Device.LastSeen,
		}
		availableNodes = append(availableNodes, node)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    availableNodes,
		"total":   len(availableNodes),
	})
}
