package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"
	"tailscale.com/tailcfg"
	"tailscale.com/types/key"
)

// 生成节点密钥
func generateNodeKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "nodekey:" + hex.EncodeToString(bytes)
}

// 生成机器密钥
func generateMachineKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "mkey:" + hex.EncodeToString(bytes)
}

// 生成发现密钥
func generateDiscoKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "discokey:" + hex.EncodeToString(bytes)
}

// handleStats 获取统计信息
func (s *UnlimitedControlServer) handleStats(c *gin.Context) {
	var stats ServerStats
	
	s.db.Model(&Device{}).Count(&stats.TotalDevices)
	s.db.Model(&Device{}).Where("online = ?", true).Count(&stats.OnlineDevices)
	s.db.Model(&User{}).Count(&stats.TotalUsers)
	
	// 模拟活跃会话和网络映射数量
	stats.ActiveSessions = stats.OnlineDevices
	stats.NetworkMaps = stats.TotalUsers
	stats.DERPConnections = stats.OnlineDevices * 2

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// UserWithStats 用户统计信息
type UserWithStats struct {
	User
	OnlineDeviceCount int `json:"online_device_count"`
}

// handleListUsers 获取用户列表
func (s *UnlimitedControlServer) handleListUsers(c *gin.Context) {
	var users []User

	result := s.db.Preload("Devices").Find(&users)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	// 计算每个用户的在线设备数量
	var usersWithStats []UserWithStats
	for _, user := range users {
		onlineCount := 0
		for _, device := range user.Devices {
			if device.Online {
				onlineCount++
			}
		}

		userWithStats := UserWithStats{
			User:              user,
			OnlineDeviceCount: onlineCount,
		}
		// 清空设备列表以减少响应大小
		userWithStats.Devices = nil
		usersWithStats = append(usersWithStats, userWithStats)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usersWithStats,
		"total":   len(usersWithStats),
	})
}

// handleCreateUser 创建用户
func (s *UnlimitedControlServer) handleCreateUser(c *gin.Context) {
	var user User
	
	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	user.CreatedAt = time.Now()
	user.UpdatedAt = time.Now()

	result := s.db.Create(&user)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    user,
	})
}

// handleGetUser 获取用户详情
func (s *UnlimitedControlServer) handleGetUser(c *gin.Context) {
	id := c.Param("id")
	var user User

	result := s.db.Preload("Devices").First(&user, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// handleUpdateUser 更新用户
func (s *UnlimitedControlServer) handleUpdateUser(c *gin.Context) {
	id := c.Param("id")
	var user User

	if err := s.db.First(&user, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&user); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	user.UpdatedAt = time.Now()
	s.db.Save(&user)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    user,
	})
}

// handleDeleteUser 删除用户
func (s *UnlimitedControlServer) handleDeleteUser(c *gin.Context) {
	id := c.Param("id")
	
	result := s.db.Delete(&User{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User deleted successfully",
	})
}

// handleListDevices 获取设备列表
func (s *UnlimitedControlServer) handleListDevices(c *gin.Context) {
	var devices []Device
	
	result := s.db.Preload("User").Find(&devices)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
		"total":   len(devices),
	})
}

// handleCreateDevice 创建设备
func (s *UnlimitedControlServer) handleCreateDevice(c *gin.Context) {
	var device Device

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 自动生成缺失的密钥
	if device.NodeKey == "" {
		device.NodeKey = generateNodeKey()
	}
	if device.MachineKey == "" {
		device.MachineKey = generateMachineKey()
	}
	if device.DiscoKey == "" {
		device.DiscoKey = generateDiscoKey()
	}

	// 处理IP地址
	if device.IPAddresses == "" || device.IPAddresses == "[]" {
		// 如果没有提供IP地址，自动分配
		device.IPAddresses = s.allocateIPAddress()
	} else {
		// 如果提供了IP地址，确保格式为JSON数组
		if !strings.HasPrefix(device.IPAddresses, "[") {
			// 如果不是JSON格式，转换为JSON数组
			ips := []string{device.IPAddresses}
			ipJSON, _ := json.Marshal(ips)
			device.IPAddresses = string(ipJSON)
		}
	}

	// 设置默认值
	device.CreatedAt = time.Now()
	device.UpdatedAt = time.Now()
	device.LastSeen = time.Now()
	if device.Authorized == false && device.Online == false {
		device.Authorized = true // 默认授权
		device.Online = true     // 默认在线
	}

	result := s.db.Create(&device)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	// 为新设备创建MagicDNS记录
	if device.Hostname != "" {
		s.createMagicDNSRecord(&device)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    device,
	})
}

// handleGetDevice 获取设备详情
func (s *UnlimitedControlServer) handleGetDevice(c *gin.Context) {
	id := c.Param("id")
	var device Device

	result := s.db.Preload("User").First(&device, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Device not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}

// handleUpdateDevice 更新设备
func (s *UnlimitedControlServer) handleUpdateDevice(c *gin.Context) {
	id := c.Param("id")
	var device Device

	if err := s.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Device not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&device); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	device.UpdatedAt = time.Now()
	s.db.Save(&device)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}

// handleDeleteDevice 删除设备
func (s *UnlimitedControlServer) handleDeleteDevice(c *gin.Context) {
	id := c.Param("id")

	// 先获取设备信息，用于删除MagicDNS记录
	var device Device
	if err := s.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Device not found",
		})
		return
	}

	// 开始事务
	tx := s.db.Begin()

	// 删除设备的MagicDNS记录
	s.deleteDeviceMagicDNS(&device)

	// 删除设备
	if err := tx.Delete(&device).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 更新用户设备计数
	if device.UserID > 0 {
		if err := tx.Model(&User{}).Where("id = ?", device.UserID).Update("device_count", gorm.Expr("GREATEST(device_count - 1, 0)")).Error; err != nil {
			tx.Rollback()
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}
	}

	// 提交事务
	tx.Commit()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device deleted successfully",
	})
}

// handleBatchDeleteDevices 批量删除设备
func (s *UnlimitedControlServer) handleBatchDeleteDevices(c *gin.Context) {
	var req struct {
		DeviceIDs []uint `json:"device_ids" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if len(req.DeviceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "No device IDs provided",
		})
		return
	}

	// 获取要删除的设备信息
	var devices []Device
	if err := s.db.Where("id IN ?", req.DeviceIDs).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 删除每个设备的MagicDNS记录
	for _, device := range devices {
		s.deleteDeviceMagicDNS(&device)
	}

	// 批量删除设备
	result := s.db.Where("id IN ?", req.DeviceIDs).Delete(&Device{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Successfully deleted %d devices", result.RowsAffected),
		"deleted_count": result.RowsAffected,
	})
}

// handleAuthorizeDevice 授权设备
func (s *UnlimitedControlServer) handleAuthorizeDevice(c *gin.Context) {
	id := c.Param("id")
	var device Device

	if err := s.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Device not found",
		})
		return
	}

	device.Authorized = true
	device.UpdatedAt = time.Now()
	s.db.Save(&device)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
		"message": "Device authorized successfully",
	})
}

// handleUpdateRoutes 更新设备路由
func (s *UnlimitedControlServer) handleUpdateRoutes(c *gin.Context) {
	id := c.Param("id")
	var device Device

	if err := s.db.First(&device, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Device not found",
		})
		return
	}

	var routeUpdate struct {
		AdvertiseRoutes []string `json:"advertise_routes"`
		EnabledRoutes   []string `json:"enabled_routes"`
		ExitNode        bool     `json:"exit_node"`
	}

	if err := c.ShouldBindJSON(&routeUpdate); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 转换为JSON字符串存储
	if advertiseRoutesJSON, err := json.Marshal(routeUpdate.AdvertiseRoutes); err == nil {
		device.AdvertiseRoutes = string(advertiseRoutesJSON)
	}
	if enabledRoutesJSON, err := json.Marshal(routeUpdate.EnabledRoutes); err == nil {
		device.EnabledRoutes = string(enabledRoutesJSON)
	}
	device.ExitNode = routeUpdate.ExitNode
	device.UpdatedAt = time.Now()

	s.db.Save(&device)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
		"message": "Routes updated successfully",
	})
}

// ===== DERP服务器管理处理函数 =====

// handleListDERPServers 获取DERP服务器列表
func (s *UnlimitedControlServer) handleListDERPServers(c *gin.Context) {
	var servers []DERPServer

	result := s.db.Find(&servers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"servers": servers,
		"total":   len(servers),
	})
}

// handleCreateDERPServer 创建DERP服务器
func (s *UnlimitedControlServer) handleCreateDERPServer(c *gin.Context) {
	var server DERPServer

	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	server.CreatedAt = time.Now()
	server.UpdatedAt = time.Now()

	result := s.db.Create(&server)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    server,
	})
}

// handleGetDERPServer 获取DERP服务器详情
func (s *UnlimitedControlServer) handleGetDERPServer(c *gin.Context) {
	id := c.Param("id")
	var server DERPServer

	result := s.db.First(&server, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "DERP server not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    server,
	})
}

// handleUpdateDERPServer 更新DERP服务器
func (s *UnlimitedControlServer) handleUpdateDERPServer(c *gin.Context) {
	id := c.Param("id")
	var server DERPServer

	if err := s.db.First(&server, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "DERP server not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&server); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	server.UpdatedAt = time.Now()
	s.db.Save(&server)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    server,
	})
}

// handleDeleteDERPServer 删除DERP服务器
func (s *UnlimitedControlServer) handleDeleteDERPServer(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Delete(&DERPServer{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "DERP server not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DERP server deleted successfully",
	})
}

// handleGetDERPMap 获取DERP地图
func (s *UnlimitedControlServer) handleGetDERPMap(c *gin.Context) {
	var servers []DERPServer

	result := s.db.Where("enabled = ?", true).Find(&servers)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	// 构建DERP地图格式
	derpMap := map[string]interface{}{
		"regions": make(map[int]interface{}),
	}

	regions := make(map[int]interface{})
	for _, server := range servers {
		region := map[string]interface{}{
			"regionid":   server.RegionID,
			"regioncode": server.RegionCode,
			"regionname": server.RegionName,
			"nodes": []map[string]interface{}{
				{
					"name":      server.Hostname,
					"regionid":  server.RegionID,
					"hostname":  server.Hostname,
					"ipv4":      server.IPv4,
					"ipv6":      server.IPv6,
					"stunport":  server.STUNPort,
					"derpport":  server.DERPPort,
					"stunonly":  server.STUNOnly,
				},
			},
		}
		regions[server.RegionID] = region
	}

	derpMap["regions"] = regions

	c.JSON(http.StatusOK, derpMap)
}

// ===== ACL规则管理处理函数 =====

// handleListACLRules 获取ACL规则列表
func (s *UnlimitedControlServer) handleListACLRules(c *gin.Context) {
	var rules []ACLRule

	result := s.db.Order("priority ASC").Find(&rules)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"rules":   rules,
		"total":   len(rules),
	})
}

// handleCreateACLRule 创建ACL规则
func (s *UnlimitedControlServer) handleCreateACLRule(c *gin.Context) {
	var rule ACLRule

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	result := s.db.Create(&rule)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleGetACLRule 获取ACL规则详情
func (s *UnlimitedControlServer) handleGetACLRule(c *gin.Context) {
	id := c.Param("id")
	var rule ACLRule

	result := s.db.First(&rule, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleUpdateACLRule 更新ACL规则
func (s *UnlimitedControlServer) handleUpdateACLRule(c *gin.Context) {
	id := c.Param("id")
	var rule ACLRule

	if err := s.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL rule not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	rule.UpdatedAt = time.Now()
	s.db.Save(&rule)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleDeleteACLRule 删除ACL规则
func (s *UnlimitedControlServer) handleDeleteACLRule(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Delete(&ACLRule{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "ACL rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "ACL rule deleted successfully",
	})
}

// ===== 网络管理处理函数 =====

// handleGetNetworkMap 获取网络映射
func (s *UnlimitedControlServer) handleGetNetworkMap(c *gin.Context) {
	var devices []Device
	var users []User
	var derpServers []DERPServer
	var aclRules []ACLRule

	// 获取所有相关数据
	s.db.Preload("User").Find(&devices)
	s.db.Find(&users)
	s.db.Where("enabled = ?", true).Find(&derpServers)
	s.db.Where("enabled = ?", true).Order("priority ASC").Find(&aclRules)

	// 获取DNS配置
	var dnsConfig DNSConfig
	s.db.First(&dnsConfig)

	// 获取DNS记录
	var dnsRecords []DNSRecord
	s.db.Where("enabled = ?", true).Find(&dnsRecords)

	networkMap := map[string]interface{}{
		"devices":     devices,
		"users":       users,
		"derp_map":    derpServers,
		"acl_rules":   aclRules,
		"dns_config":  dnsConfig,
		"dns_records": dnsRecords,
		"updated_at":  time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    networkMap,
	})
}

// handleUpdateNetworkMap 更新网络映射
func (s *UnlimitedControlServer) handleUpdateNetworkMap(c *gin.Context) {
	// 这里可以触发网络映射的重新计算和分发
	// 在实际实现中，这会通知所有连接的设备更新其网络映射

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Network map update triggered",
		"timestamp": time.Now(),
	})
}

// ===== 网络诊断处理函数 =====

// handlePingDevice 设备ping测试
func (s *UnlimitedControlServer) handlePingDevice(c *gin.Context) {
	var request struct {
		Target string `json:"target" binding:"required"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 模拟ping测试结果
	// 在实际实现中，这里会执行真实的ping测试
	result := map[string]interface{}{
		"target":       request.Target,
		"success":      true,
		"packets_sent": 4,
		"packets_recv": 4,
		"packet_loss":  0,
		"min_rtt":      "1.2ms",
		"max_rtt":      "2.1ms",
		"avg_rtt":      "1.6ms",
		"timestamp":    time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// handleGetNetworkStats 获取网络统计信息
func (s *UnlimitedControlServer) handleGetNetworkStats(c *gin.Context) {
	// 模拟网络统计数据
	// 在实际实现中，这里会收集真实的网络统计信息
	stats := map[string]interface{}{
		"total_traffic": map[string]interface{}{
			"bytes_sent":     1024 * 1024 * 100, // 100MB
			"bytes_received": 1024 * 1024 * 200, // 200MB
			"packets_sent":   50000,
			"packets_recv":   75000,
		},
		"active_connections": 25,
		"derp_usage": map[string]interface{}{
			"direct_connections": 15,
			"relayed_connections": 10,
			"derp_bytes_sent":     1024 * 1024 * 50, // 50MB
			"derp_bytes_recv":     1024 * 1024 * 80, // 80MB
		},
		"timestamp": time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ===== Tailscale协议兼容处理函数 =====

// handleMachineRegister 处理设备注册请求
func (s *UnlimitedControlServer) handleMachineRegister(c *gin.Context) {
	var registerReq tailcfg.RegisterRequest

	if err := c.ShouldBindJSON(&registerReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid register request: " + err.Error(),
		})
		return
	}

	// 查找或创建用户
	var user User
	result := s.db.Where("name = ?", "default-user").First(&user)
	if result.Error != nil {
		// 创建默认用户
		user = User{
			Name:        "default-user",
			Email:       "user@localhost",
			Provider:    "local",
			DisplayName: "Default User",
			Role:        "user",
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}
		s.db.Create(&user)
	}

	// 查找或创建设备
	var device Device
	nodeKeyStr := registerReq.NodeKey.String()
	result = s.db.Where("node_key = ?", nodeKeyStr).First(&device)

	if result.Error != nil {
		// 创建新设备
		device = Device{
			NodeKey:     nodeKeyStr,
			MachineKey:  generateMachineKey(), // 生成独立的机器密钥
			DiscoKey:    generateDiscoKey(),   // 生成发现密钥
			Hostname:    registerReq.Hostinfo.Hostname,
			GivenName:   registerReq.Hostinfo.Hostname, // 设置给定名称
			UserID:      user.ID,
			Online:      true,
			Authorized:  true, // 无限制版本自动授权
			DERP:        "1",  // 默认DERP区域
			LastSeen:    time.Now(),
			CreatedAt:   time.Now(),
			UpdatedAt:   time.Now(),
		}

		// 分配IP地址
		device.IPAddresses = s.allocateIPAddress()

		s.db.Create(&device)

		// 为新设备创建MagicDNS记录
		s.createMagicDNSRecord(&device)

		// 处理路由广播
		s.processDeviceRoutes(&device, &registerReq)
	} else {
		// 更新现有设备
		device.Online = true
		device.LastSeen = time.Now()
		device.UpdatedAt = time.Now()
		if registerReq.Hostinfo.Hostname != "" {
			device.Hostname = registerReq.Hostinfo.Hostname
		}
		s.db.Save(&device)
	}

	// 构建注册响应
	registerResp := tailcfg.RegisterResponse{
		User:              tailcfg.User{ID: tailcfg.UserID(user.ID), DisplayName: user.DisplayName},
		Login:             tailcfg.Login{ID: tailcfg.LoginID(user.ID), DisplayName: user.DisplayName},
		NodeKeyExpired:    false,
		MachineAuthorized: true,
		AuthURL:           "", // 无需认证URL，自动授权
	}

	c.JSON(http.StatusOK, registerResp)
}

// handleMachineMap 处理网络映射请求
func (s *UnlimitedControlServer) handleMachineMap(c *gin.Context) {
	var mapReq tailcfg.MapRequest

	if err := c.ShouldBindJSON(&mapReq); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error": "Invalid map request: " + err.Error(),
		})
		return
	}

	// 获取请求设备信息
	nodeKeyStr := mapReq.NodeKey.String()
	var requestingDevice Device
	result := s.db.Where("node_key = ?", nodeKeyStr).First(&requestingDevice)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"error": "Device not found",
		})
		return
	}

	// 更新设备最后活跃时间
	requestingDevice.LastSeen = time.Now()
	requestingDevice.Online = true
	s.db.Save(&requestingDevice)

	// 获取所有授权设备
	var allDevices []Device
	s.db.Where("authorized = ?", true).Find(&allDevices)

	// 构建网络映射
	var nodes []*tailcfg.Node
	for _, dev := range allDevices {
		node := &tailcfg.Node{
			ID:       tailcfg.NodeID(dev.ID),
			Name:     dev.Hostname,
			User:     tailcfg.UserID(dev.UserID),
			Key:      key.NodePublic{}, // 需要解析存储的key
			Machine:  key.MachinePublic{},
			Online:   &dev.Online,
			LastSeen: &dev.LastSeen,
		}

		// 解析IP地址
		if dev.IPAddresses != "" {
			var ips []string
			if err := json.Unmarshal([]byte(dev.IPAddresses), &ips); err == nil {
				for _, ipStr := range ips {
					if addr, err := netip.ParseAddr(ipStr); err == nil {
						node.Addresses = append(node.Addresses, netip.PrefixFrom(addr, 32))
					}
				}
			}
		}

		nodes = append(nodes, node)
	}

	// 获取DERP地图
	var derpServers []DERPServer
	s.db.Where("enabled = ?", true).Find(&derpServers)

	derpMap := &tailcfg.DERPMap{
		Regions: make(map[int]*tailcfg.DERPRegion),
	}

	for _, server := range derpServers {
		region := &tailcfg.DERPRegion{
			RegionID:   server.RegionID,
			RegionCode: server.RegionCode,
			RegionName: server.RegionName,
			Nodes: []*tailcfg.DERPNode{
				{
					Name:     server.Hostname,
					RegionID: server.RegionID,
					HostName: server.Hostname,
					IPv4:     server.IPv4,
					IPv6:     server.IPv6,
					STUNPort: server.STUNPort,
					DERPPort: server.DERPPort,
					STUNOnly: server.STUNOnly,
				},
			},
		}
		derpMap.Regions[server.RegionID] = region
	}

	// 构建网络映射响应
	mapResp := &tailcfg.MapResponse{
		DERPMap:   derpMap,
		KeepAlive: true,
	}

	// 如果有节点，设置节点信息
	if len(nodes) > 0 {
		mapResp.Node = nodes[0] // 请求设备的节点信息
		if len(nodes) > 1 {
			mapResp.Peers = nodes[1:] // 其他设备
		}
	}

	c.JSON(http.StatusOK, mapResp)
}

// allocateIPAddress 分配IP地址
func (s *UnlimitedControlServer) allocateIPAddress() string {
	// 简化的IP分配逻辑
	// 在实际实现中，这里应该有更复杂的IP地址管理
	var deviceCount int64
	s.db.Model(&Device{}).Count(&deviceCount)

	// 使用100.64.0.0/10网段（Tailscale默认网段）
	ip := fmt.Sprintf("100.64.0.%d", deviceCount+1)
	ips := []string{ip}

	ipJSON, _ := json.Marshal(ips)
	return string(ipJSON)
}

// handleListAPIKeys 获取API密钥列表
func (s *UnlimitedControlServer) handleListAPIKeys(c *gin.Context) {
	// 返回空的API密钥列表，因为我们的无限制版本不需要API密钥认证
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    []map[string]interface{}{},
		"total":   0,
	})
}

// handleCreateAPIKey 创建API密钥
func (s *UnlimitedControlServer) handleCreateAPIKey(c *gin.Context) {
	// 返回一个模拟的API密钥，因为我们的无限制版本不需要真实的API密钥
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"apiKey": "unlimited-control-key-" + fmt.Sprintf("%d", time.Now().Unix()),
		},
	})
}

// handleExpireAPIKey 使API密钥过期
func (s *UnlimitedControlServer) handleExpireAPIKey(c *gin.Context) {
	// 模拟API密钥过期操作
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key expired successfully",
	})
}

// ===== 预授权密钥管理 =====

// generatePreAuthKey 生成预授权密钥
func generatePreAuthKey() string {
	bytes := make([]byte, 32)
	rand.Read(bytes)
	return "tskey-auth-" + hex.EncodeToString(bytes)
}

// handleListPreAuthKeys 获取预授权密钥列表
func (s *UnlimitedControlServer) handleListPreAuthKeys(c *gin.Context) {
	userID := c.Query("user")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "user parameter is required",
		})
		return
	}

	var preAuthKeys []PreAuthKey
	query := s.db.Preload("User").Where("user_id = ?", userID)

	if err := query.Find(&preAuthKeys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success":     true,
		"preAuthKeys": preAuthKeys,
	})
}

// handleCreatePreAuthKey 创建预授权密钥
func (s *UnlimitedControlServer) handleCreatePreAuthKey(c *gin.Context) {
	var req struct {
		User       string `json:"user" binding:"required"`
		Expiration string `json:"expiration" binding:"required"`
		Reusable   bool   `json:"reusable"`
		Ephemeral  bool   `json:"ephemeral"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 解析过期时间
	expiresAt, err := time.Parse(time.RFC3339, req.Expiration)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid expiration time format",
		})
		return
	}

	// 查找用户
	var user User
	if err := s.db.Where("name = ?", req.User).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	// 创建预授权密钥
	preAuthKey := PreAuthKey{
		Key:       generatePreAuthKey(),
		UserID:    user.ID,
		Reusable:  req.Reusable,
		Ephemeral: req.Ephemeral,
		Used:      false,
		CreatedAt: time.Now(),
		ExpiresAt: expiresAt,
	}

	if err := s.db.Create(&preAuthKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 重新加载用户信息
	s.db.Preload("User").First(&preAuthKey, preAuthKey.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success":    true,
		"preAuthKey": preAuthKey,
	})
}

// handleExpirePreAuthKey 使预授权密钥过期
func (s *UnlimitedControlServer) handleExpirePreAuthKey(c *gin.Context) {
	var req struct {
		User string `json:"user" binding:"required"`
		Key  string `json:"key" binding:"required"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 查找并删除预授权密钥
	result := s.db.Where("key = ?", req.Key).Delete(&PreAuthKey{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "PreAuth key not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "PreAuth key expired successfully",
	})
}

// ===== 用户设备限制管理 =====

// handleSetUserDeviceLimit 设置用户设备限制
func (s *UnlimitedControlServer) handleSetUserDeviceLimit(c *gin.Context) {
	var req struct {
		UserID      uint `json:"user_id" binding:"required"`
		DeviceLimit int  `json:"device_limit" binding:"min=0"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 更新用户设备限制
	result := s.db.Model(&User{}).Where("id = ?", req.UserID).Update("device_limit", req.DeviceLimit)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User device limit updated successfully",
	})
}

// handleUserDeviceRegistration 用户设备注册（基于用户名的自动授权）
func (s *UnlimitedControlServer) handleUserDeviceRegistration(c *gin.Context) {
	var req struct {
		Username    string `json:"username" binding:"required"`
		Hostname    string `json:"hostname" binding:"required"`
		MachineKey  string `json:"machine_key" binding:"required"`
		NodeKey     string `json:"node_key" binding:"required"`
		DiscoKey    string `json:"disco_key" binding:"required"`
		IPAddresses string `json:"ip_addresses"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 查找用户
	var user User
	if err := s.db.Where("name = ?", req.Username).First(&user).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found: " + req.Username,
		})
		return
	}

	// 检查用户是否激活
	if !user.Active {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   "User is not active: " + req.Username,
		})
		return
	}

	// 检查设备是否已存在（基于MachineKey）
	var existingDevice Device
	if err := s.db.Where("machine_key = ?", req.MachineKey).First(&existingDevice).Error; err == nil {
		// 设备已存在，更新信息
		existingDevice.Hostname = req.Hostname
		existingDevice.NodeKey = req.NodeKey
		existingDevice.DiscoKey = req.DiscoKey
		existingDevice.LastSeen = time.Now()
		existingDevice.Online = true

		if err := s.db.Save(&existingDevice).Error; err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"error":   err.Error(),
			})
			return
		}

		c.JSON(http.StatusOK, gin.H{
			"success": true,
			"message": "Device updated successfully",
			"data":    existingDevice,
		})
		return
	}

	// 检查设备数量限制
	if user.DeviceLimit > 0 && user.DeviceCount >= user.DeviceLimit {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"error":   fmt.Sprintf("User %s has reached device limit (%d/%d)",
				req.Username, user.DeviceCount, user.DeviceLimit),
		})
		return
	}

	// 创建新设备
	device := Device{
		UserID:      user.ID,
		Hostname:    req.Hostname,
		MachineKey:  req.MachineKey,
		NodeKey:     req.NodeKey,
		DiscoKey:    req.DiscoKey,
		IPAddresses: req.IPAddresses,
		Authorized:  true,  // 自动授权
		Online:      true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
		LastSeen:    time.Now(),
	}

	// 如果没有提供IP地址，自动分配
	if device.IPAddresses == "" || device.IPAddresses == "[]" {
		device.IPAddresses = s.allocateIPAddress()
	} else {
		// 确保IP地址格式为JSON数组
		if !strings.HasPrefix(device.IPAddresses, "[") {
			ips := []string{device.IPAddresses}
			ipJSON, _ := json.Marshal(ips)
			device.IPAddresses = string(ipJSON)
		}
	}

	// 开始事务
	tx := s.db.Begin()

	// 创建设备
	if err := tx.Create(&device).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 更新用户设备计数
	if err := tx.Model(&user).Update("device_count", gorm.Expr("device_count + ?", 1)).Error; err != nil {
		tx.Rollback()
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 提交事务
	tx.Commit()

	// 重新加载设备信息
	s.db.Preload("User").First(&device, device.ID)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "Device registered successfully",
		"data":    device,
	})
}

// handleSyncUserDeviceCount 同步用户设备计数
func (s *UnlimitedControlServer) handleSyncUserDeviceCount(c *gin.Context) {
	userID := c.Param("id")
	if userID == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "User ID is required",
		})
		return
	}

	// 查询用户的实际设备数量
	var deviceCount int64
	if err := s.db.Model(&Device{}).Where("user_id = ?", userID).Count(&deviceCount).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 更新用户的设备计数
	result := s.db.Model(&User{}).Where("id = ?", userID).Update("device_count", deviceCount)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "User not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "User device count synchronized successfully",
		"device_count": deviceCount,
	})
}

// handleSyncAllUserDeviceCount 同步所有用户的设备计数
func (s *UnlimitedControlServer) handleSyncAllUserDeviceCount(c *gin.Context) {
	// 获取所有用户
	var users []User
	if err := s.db.Find(&users).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	syncCount := 0
	for _, user := range users {
		// 查询每个用户的实际设备数量
		var deviceCount int64
		if err := s.db.Model(&Device{}).Where("user_id = ?", user.ID).Count(&deviceCount).Error; err != nil {
			continue
		}

		// 更新用户的设备计数
		if err := s.db.Model(&User{}).Where("id = ?", user.ID).Update("device_count", deviceCount).Error; err != nil {
			continue
		}

		syncCount++
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "All user device counts synchronized successfully",
		"synced_users": syncCount,
	})
}

// ============ 告警管理 API ============

// handleListAlertRules 获取告警规则列表
func (s *UnlimitedControlServer) handleListAlertRules(c *gin.Context) {
	var rules []AlertRule

	result := s.db.Preload("Notifications").Find(&rules)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"total":   len(rules),
	})
}

// handleCreateAlertRule 创建告警规则
func (s *UnlimitedControlServer) handleCreateAlertRule(c *gin.Context) {
	var rule AlertRule

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	rule.CreatedAt = time.Now()
	rule.UpdatedAt = time.Now()

	result := s.db.Create(&rule)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	// 记录日志
	s.logSystemEvent("info", "alert", fmt.Sprintf("Created alert rule: %s", rule.Name), nil, nil)

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleGetAlertRule 获取告警规则详情
func (s *UnlimitedControlServer) handleGetAlertRule(c *gin.Context) {
	id := c.Param("id")
	var rule AlertRule

	result := s.db.Preload("Notifications").First(&rule, id)
	if result.Error != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Alert rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleUpdateAlertRule 更新告警规则
func (s *UnlimitedControlServer) handleUpdateAlertRule(c *gin.Context) {
	id := c.Param("id")
	var rule AlertRule

	if err := s.db.First(&rule, id).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Alert rule not found",
		})
		return
	}

	if err := c.ShouldBindJSON(&rule); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	rule.UpdatedAt = time.Now()
	s.db.Save(&rule)

	// 记录日志
	s.logSystemEvent("info", "alert", fmt.Sprintf("Updated alert rule: %s", rule.Name), nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleDeleteAlertRule 删除告警规则
func (s *UnlimitedControlServer) handleDeleteAlertRule(c *gin.Context) {
	id := c.Param("id")

	// 先删除相关的通知配置
	s.db.Where("alert_rule_id = ?", id).Delete(&AlertNotification{})

	result := s.db.Delete(&AlertRule{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Alert rule not found",
		})
		return
	}

	// 记录日志
	s.logSystemEvent("info", "alert", fmt.Sprintf("Deleted alert rule ID: %s", id), nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Alert rule deleted successfully",
	})
}

// handleListAlertHistory 获取告警历史
func (s *UnlimitedControlServer) handleListAlertHistory(c *gin.Context) {
	var history []AlertHistory

	result := s.db.Preload("AlertRule").Order("created_at DESC").Limit(100).Find(&history)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    history,
		"total":   len(history),
	})
}

// handleTestAlert 测试告警
func (s *UnlimitedControlServer) handleTestAlert(c *gin.Context) {
	var request struct {
		RuleID uint `json:"rule_id"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 创建测试告警历史记录
	history := AlertHistory{
		AlertRuleID: request.RuleID,
		Message:     "Test alert triggered manually",
		Severity:    "info",
		Value:       0,
		Resolved:    false,
		CreatedAt:   time.Now(),
	}

	s.db.Create(&history)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Test alert sent successfully",
		"data":    history,
	})
}

// ============ 日志管理 API ============

// handleListLogs 获取系统日志列表
func (s *UnlimitedControlServer) handleListLogs(c *gin.Context) {
	var logs []SystemLog

	// 获取查询参数
	level := c.Query("level")
	component := c.Query("component")
	limitStr := c.DefaultQuery("limit", "100")

	// 解析limit参数
	limitInt := 100
	if limitStr != "" {
		if parsed, err := strconv.Atoi(limitStr); err == nil && parsed > 0 {
			limitInt = parsed
		}
	}

	query := s.db.Model(&SystemLog{})

	if level != "" {
		query = query.Where("level = ?", level)
	}
	if component != "" {
		query = query.Where("component = ?", component)
	}

	result := query.Order("created_at DESC").Limit(limitInt).Find(&logs)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// handleCreateLog 创建系统日志
func (s *UnlimitedControlServer) handleCreateLog(c *gin.Context) {
	var log SystemLog

	if err := c.ShouldBindJSON(&log); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	log.CreatedAt = time.Now()

	result := s.db.Create(&log)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    log,
	})
}

// handleDeleteLog 删除日志
func (s *UnlimitedControlServer) handleDeleteLog(c *gin.Context) {
	id := c.Param("id")

	result := s.db.Delete(&SystemLog{}, id)
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	if result.RowsAffected == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"error":   "Log not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Log deleted successfully",
	})
}

// handleClearLogs 清空日志
func (s *UnlimitedControlServer) handleClearLogs(c *gin.Context) {
	var request struct {
		Level     string `json:"level"`
		Component string `json:"component"`
		Days      int    `json:"days"` // 清空多少天前的日志
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	query := s.db.Model(&SystemLog{})

	if request.Level != "" {
		query = query.Where("level = ?", request.Level)
	}
	if request.Component != "" {
		query = query.Where("component = ?", request.Component)
	}
	if request.Days > 0 {
		cutoff := time.Now().AddDate(0, 0, -request.Days)
		query = query.Where("created_at < ?", cutoff)
	}

	result := query.Delete(&SystemLog{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"error":   result.Error.Error(),
		})
		return
	}

	// 记录清理操作
	s.logSystemEvent("info", "system", fmt.Sprintf("Cleared %d log entries", result.RowsAffected), nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Cleared %d log entries", result.RowsAffected),
		"count":   result.RowsAffected,
	})
}

// logSystemEvent 记录系统事件的辅助函数
func (s *UnlimitedControlServer) logSystemEvent(level, component, message string, userID, deviceID *uint) {
	log := SystemLog{
		Level:     level,
		Component: component,
		Message:   message,
		UserID:    userID,
		DeviceID:  deviceID,
		CreatedAt: time.Now(),
	}
	s.db.Create(&log)
}

// ============ 报表管理 API ============

// handleUsageReport 获取使用情况报表
func (s *UnlimitedControlServer) handleUsageReport(c *gin.Context) {
	var stats struct {
		TotalUsers       int64 `json:"total_users"`
		TotalDevices     int64 `json:"total_devices"`
		OnlineDevices    int64 `json:"online_devices"`
		TotalDERPServers int64 `json:"total_derp_servers"`
		TotalACLRules    int64 `json:"total_acl_rules"`
		TotalAlerts      int64 `json:"total_alerts"`
		TotalLogs        int64 `json:"total_logs"`
	}

	s.db.Model(&User{}).Count(&stats.TotalUsers)
	s.db.Model(&Device{}).Count(&stats.TotalDevices)
	s.db.Model(&Device{}).Where("online = ?", true).Count(&stats.OnlineDevices)
	s.db.Model(&DERPServer{}).Count(&stats.TotalDERPServers)
	s.db.Model(&ACLRule{}).Count(&stats.TotalACLRules)
	s.db.Model(&AlertHistory{}).Count(&stats.TotalAlerts)
	s.db.Model(&SystemLog{}).Count(&stats.TotalLogs)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// handlePerformanceReport 获取性能报表
func (s *UnlimitedControlServer) handlePerformanceReport(c *gin.Context) {
	// 模拟性能数据
	report := gin.H{
		"cpu_usage":    75.5,
		"memory_usage": 68.2,
		"disk_usage":   45.8,
		"network_io": gin.H{
			"bytes_in":  1024000,
			"bytes_out": 2048000,
		},
		"api_response_time": gin.H{
			"avg": 85.5,
			"p95": 150.2,
			"p99": 280.1,
		},
		"database_performance": gin.H{
			"connections": 5,
			"query_time":  12.5,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// handleDeviceReport 获取设备报表
func (s *UnlimitedControlServer) handleDeviceReport(c *gin.Context) {
	var devices []Device
	s.db.Preload("User").Find(&devices)

	// 统计设备状态
	var onlineCount, offlineCount int64
	s.db.Model(&Device{}).Where("online = ?", true).Count(&onlineCount)
	s.db.Model(&Device{}).Where("online = ?", false).Count(&offlineCount)

	report := gin.H{
		"total_devices":  len(devices),
		"online_devices": onlineCount,
		"offline_devices": offlineCount,
		"devices":        devices,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// handleExportReport 导出报表
func (s *UnlimitedControlServer) handleExportReport(c *gin.Context) {
	reportType := c.Param("type")

	switch reportType {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=report.csv")
		c.String(http.StatusOK, "Device Name,Status,User,Last Seen\ntest-device,online,admin,2025-01-30")
	case "json":
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=report.json")
		c.JSON(http.StatusOK, gin.H{
			"success":     true,
			"export_time": time.Now(),
			"data":        "Report data here",
		})
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Unsupported export type",
		})
	}
}

// ============ 网络诊断 API ============

// handleNetworkPing 网络Ping测试
func (s *UnlimitedControlServer) handleNetworkPing(c *gin.Context) {
	var request struct {
		Target string `json:"target"`
		Count  int    `json:"count"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	if request.Count == 0 {
		request.Count = 4
	}

	// 模拟ping结果
	result := gin.H{
		"target": request.Target,
		"count":  request.Count,
		"results": []gin.H{
			{"seq": 1, "time": 12.5, "status": "success"},
			{"seq": 2, "time": 15.2, "status": "success"},
			{"seq": 3, "time": 11.8, "status": "success"},
			{"seq": 4, "time": 13.1, "status": "success"},
		},
		"statistics": gin.H{
			"packets_sent":     request.Count,
			"packets_received": request.Count,
			"packet_loss":      0,
			"min_time":         11.8,
			"max_time":         15.2,
			"avg_time":         13.15,
		},
	}

	// 记录诊断日志
	s.logSystemEvent("info", "network", fmt.Sprintf("Ping test to %s completed", request.Target), nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// handleNetworkStats 获取网络统计信息
func (s *UnlimitedControlServer) handleNetworkStats(c *gin.Context) {
	// 模拟网络统计数据
	stats := gin.H{
		"total_connections": 25,
		"active_sessions":   18,
		"bandwidth_usage": gin.H{
			"inbound":  "125.5 MB",
			"outbound": "89.2 MB",
		},
		"derp_usage": gin.H{
			"total_relayed": 1250,
			"direct_connections": 18,
			"relayed_connections": 7,
		},
		"latency": gin.H{
			"avg": 45.2,
			"min": 12.1,
			"max": 156.8,
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// handleTraceroute 路由追踪
func (s *UnlimitedControlServer) handleTraceroute(c *gin.Context) {
	var request struct {
		Target string `json:"target"`
	}

	if err := c.ShouldBindJSON(&request); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   err.Error(),
		})
		return
	}

	// 模拟traceroute结果
	result := gin.H{
		"target": request.Target,
		"hops": []gin.H{
			{"hop": 1, "ip": "192.168.1.1", "hostname": "gateway", "time": 1.2},
			{"hop": 2, "ip": "10.0.0.1", "hostname": "isp-router", "time": 15.5},
			{"hop": 3, "ip": "203.0.113.1", "hostname": "backbone", "time": 45.8},
			{"hop": 4, "ip": request.Target, "hostname": "", "time": 62.1},
		},
	}

	// 记录诊断日志
	s.logSystemEvent("info", "network", fmt.Sprintf("Traceroute to %s completed", request.Target), nil, nil)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// handleConnectionQuality 连接质量分析
func (s *UnlimitedControlServer) handleConnectionQuality(c *gin.Context) {
	// 模拟连接质量数据
	quality := gin.H{
		"overall_score": 85,
		"metrics": gin.H{
			"latency": gin.H{
				"score": 90,
				"value": "45ms",
				"status": "good",
			},
			"packet_loss": gin.H{
				"score": 95,
				"value": "0.1%",
				"status": "excellent",
			},
			"jitter": gin.H{
				"score": 80,
				"value": "5.2ms",
				"status": "good",
			},
			"bandwidth": gin.H{
				"score": 75,
				"value": "100 Mbps",
				"status": "fair",
			},
		},
		"recommendations": []string{
			"Consider upgrading bandwidth for better performance",
			"Network latency is within acceptable range",
			"Packet loss is minimal, connection is stable",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    quality,
	})
}

// ===== MagicDNS辅助函数 =====

// createMagicDNSRecord 为设备创建MagicDNS记录
func (s *UnlimitedControlServer) createMagicDNSRecord(device *Device) error {
	// 获取DNS配置
	var dnsConfig DNSConfig
	if err := s.db.First(&dnsConfig).Error; err != nil {
		return err
	}

	// 如果MagicDNS未启用，跳过
	if !dnsConfig.MagicDNSEnabled {
		return nil
	}

	// 解析设备IP地址
	var ips []string
	if err := json.Unmarshal([]byte(device.IPAddresses), &ips); err != nil || len(ips) == 0 {
		return fmt.Errorf("no valid IP addresses for device")
	}

	// 创建A记录（主机名）
	hostname := device.Hostname
	if hostname == "" {
		hostname = device.GivenName
	}

	if hostname != "" {
		record := DNSRecord{
			Name:    hostname,
			Type:    "A",
			Value:   ips[0],
			TTL:     300,
			Enabled: true,
		}

		// 检查是否已存在
		var existing DNSRecord
		if err := s.db.Where("name = ? AND type = ?", hostname, "A").First(&existing).Error; err != nil {
			// 不存在，创建新记录
			s.db.Create(&record)
		} else {
			// 已存在，更新IP地址
			existing.Value = ips[0]
			s.db.Save(&existing)
		}
	}

	return nil
}

// updateDeviceMagicDNS 更新设备的MagicDNS记录
func (s *UnlimitedControlServer) updateDeviceMagicDNS(device *Device) error {
	return s.createMagicDNSRecord(device)
}

// deleteDeviceMagicDNS 删除设备的MagicDNS记录
func (s *UnlimitedControlServer) deleteDeviceMagicDNS(device *Device) error {
	hostname := device.Hostname
	if hostname == "" {
		hostname = device.GivenName
	}

	if hostname != "" {
		s.db.Where("name = ? AND type = ?", hostname, "A").Delete(&DNSRecord{})
	}

	return nil
}

// processDeviceRoutes 处理设备路由广播
func (s *UnlimitedControlServer) processDeviceRoutes(device *Device, registerReq *tailcfg.RegisterRequest) {
	// 检查是否有路由广播请求
	if registerReq.Hostinfo == nil {
		return
	}

	// 处理广播的路由
	if len(registerReq.Hostinfo.RoutableIPs) > 0 {
		for _, routePrefix := range registerReq.Hostinfo.RoutableIPs {
			// 验证路由格式
			if routePrefix.String() == "" {
				continue
			}

			// 检查路由是否已存在
			var existingRoute Route
			if err := s.db.Where("device_id = ? AND prefix = ?", device.ID, routePrefix.String()).First(&existingRoute).Error; err != nil {
				// 创建新路由
				route := Route{
					DeviceID:    device.ID,
					Prefix:      routePrefix.String(),
					Advertised:  true,
					Enabled:     false, // 默认禁用，需要管理员手动启用
					Primary:     false,
					ExitNode:    isDefaultRoute(routePrefix.String()),
					Description: fmt.Sprintf("Auto-discovered route from %s", device.Hostname),
				}

				s.db.Create(&route)
			} else {
				// 更新现有路由为广播状态
				existingRoute.Advertised = true
				s.db.Save(&existingRoute)
			}
		}

		// 更新设备的路由信息
		rm := NewRouteManager(s)
		rm.UpdateDeviceRoutes(device.ID)
	}

	// 检查是否请求作为出口节点
	if registerReq.Hostinfo.RequestTags != nil {
		for _, tag := range registerReq.Hostinfo.RequestTags {
			if tag == "tag:exit-node" {
				// 创建或更新出口节点配置
				var exitNodeConfig ExitNodeConfig
				if err := s.db.Where("device_id = ?", device.ID).First(&exitNodeConfig).Error; err != nil {
					// 创建新的出口节点配置
					exitNodeConfig = ExitNodeConfig{
						DeviceID:              device.ID,
						Enabled:               false, // 默认禁用，需要管理员手动启用
						AllowLANAccess:        false,
						AdvertiseDefaultRoute: true,
						DNSConfig:             `{"nameservers": ["8.8.8.8", "8.8.4.4"], "search_domains": []}`,
					}
					s.db.Create(&exitNodeConfig)
				}
				break
			}
		}
	}
}

// isDefaultRoute 检查是否为默认路由
func isDefaultRoute(prefix string) bool {
	return prefix == "0.0.0.0/0" || prefix == "::/0"
}

// handleGetNotificationSettings 获取通知设置
func (s *UnlimitedControlServer) handleGetNotificationSettings(c *gin.Context) {
	// 返回默认的通知设置
	settings := gin.H{
		"email": gin.H{
			"smtp_server": "",
			"port":        587,
			"username":    "",
			"password":    "",
		},
		"webhook": gin.H{
			"url":    "",
			"secret": "",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    settings,
	})
}

// handleUpdateNotificationSettings 更新通知设置
func (s *UnlimitedControlServer) handleUpdateNotificationSettings(c *gin.Context) {
	var settings map[string]interface{}
	if err := c.ShouldBindJSON(&settings); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"error":   "Invalid JSON format",
		})
		return
	}

	// 这里可以将设置保存到数据库或配置文件
	// 目前只是简单返回成功
	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Notification settings updated successfully",
	})
}

// handleLogStream WebSocket日志流
func (s *UnlimitedControlServer) handleLogStream(c *gin.Context) {
	// 简单返回WebSocket不支持的错误
	c.JSON(http.StatusNotImplemented, gin.H{
		"success": false,
		"error":   "WebSocket streaming not implemented yet",
	})
}

// handleExportLogs 导出日志
func (s *UnlimitedControlServer) handleExportLogs(c *gin.Context) {
	format := c.DefaultQuery("format", "json")

	// 模拟日志数据
	logs := []gin.H{
		{
			"id":         1,
			"level":      "info",
			"component":  "api",
			"message":    "API request processed",
			"created_at": "2025-08-01T08:00:00Z",
		},
		{
			"id":         2,
			"level":      "error",
			"component":  "auth",
			"message":    "Authentication failed",
			"created_at": "2025-08-01T08:01:00Z",
		},
	}

	switch format {
	case "csv":
		c.Header("Content-Type", "text/csv")
		c.Header("Content-Disposition", "attachment; filename=logs.csv")
		c.String(http.StatusOK, "id,level,component,message,created_at\n1,info,api,API request processed,2025-08-01T08:00:00Z\n2,error,auth,Authentication failed,2025-08-01T08:01:00Z\n")
	default:
		c.Header("Content-Type", "application/json")
		c.Header("Content-Disposition", "attachment; filename=logs.json")
		c.JSON(http.StatusOK, logs)
	}
}

// handleNetworkReport 网络报表
func (s *UnlimitedControlServer) handleNetworkReport(c *gin.Context) {
	// 模拟网络报表数据
	report := gin.H{
		"total_inbound":       1024 * 1024 * 100, // 100MB
		"total_outbound":      1024 * 1024 * 80,  // 80MB
		"active_connections":  25,
		"avg_latency":         45.5,
		"min_latency":         12.3,
		"max_latency":         156.7,
		"packet_loss":         0.02,
		"bandwidth_utilization": 65.4,
		"total_connections":   150,
		"successful_connections": 145,
		"failed_connections":  5,
		"inbound":  []float64{10, 15, 20, 25, 30, 35, 40},
		"outbound": []float64{8, 12, 16, 20, 24, 28, 32},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    report,
	})
}

// handleHealth 健康检查
func (s *UnlimitedControlServer) handleHealth(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{
		"status":    "healthy",
		"timestamp": time.Now(),
		"version":   "1.0.0",
		"uptime":    time.Since(time.Now().Add(-time.Hour * 24)), // 模拟运行时间
	})
}
