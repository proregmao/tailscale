package main

import (
	"crypto/md5"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SSHManager SSH管理器
type SSHManager struct {
	server *UnlimitedControlServer
}

// NewSSHManager 创建SSH管理器
func NewSSHManager(server *UnlimitedControlServer) *SSHManager {
	return &SSHManager{
		server: server,
	}
}

// ParseSSHPublicKey 解析SSH公钥
func (sm *SSHManager) ParseSSHPublicKey(publicKey string) (keyType, fingerprint, comment string, err error) {
	parts := strings.Fields(strings.TrimSpace(publicKey))
	if len(parts) < 2 {
		return "", "", "", fmt.Errorf("invalid SSH public key format")
	}

	keyType = parts[0]
	keyData := parts[1]
	
	if len(parts) > 2 {
		comment = strings.Join(parts[2:], " ")
	}

	// 解码base64密钥数据
	keyBytes, err := base64.StdEncoding.DecodeString(keyData)
	if err != nil {
		return "", "", "", fmt.Errorf("failed to decode key data: %w", err)
	}

	// 生成MD5指纹
	md5Hash := md5.Sum(keyBytes)
	fingerprint = fmt.Sprintf("MD5:%s", hex.EncodeToString(md5Hash[:]))

	return keyType, fingerprint, comment, nil
}

// ValidateSSHKey 验证SSH密钥
func (sm *SSHManager) ValidateSSHKey(publicKey string) error {
	keyType, _, _, err := sm.ParseSSHPublicKey(publicKey)
	if err != nil {
		return err
	}

	// 检查支持的密钥类型
	supportedTypes := []string{"ssh-rsa", "ssh-ed25519", "ecdsa-sha2-nistp256", "ecdsa-sha2-nistp384", "ecdsa-sha2-nistp521"}
	isSupported := false
	for _, t := range supportedTypes {
		if keyType == t {
			isSupported = true
			break
		}
	}

	if !isSupported {
		return fmt.Errorf("unsupported key type: %s", keyType)
	}

	return nil
}

// GenerateAuthorizedKeys 生成authorized_keys文件内容
func (sm *SSHManager) GenerateAuthorizedKeys(deviceID uint) (string, error) {
	var keys []SSHKey
	if err := sm.server.db.Where("(device_id = ? OR device_id IS NULL) AND enabled = ?", 
		deviceID, true).Find(&keys).Error; err != nil {
		return "", err
	}

	var authorizedKeys []string
	for _, key := range keys {
		authorizedKeys = append(authorizedKeys, key.PublicKey)
	}

	return strings.Join(authorizedKeys, "\n"), nil
}

// ===== SSH密钥管理处理函数 =====

// handleListSSHKeys 获取SSH密钥列表
func (s *UnlimitedControlServer) handleListSSHKeys(c *gin.Context) {
	var keys []SSHKey
	
	query := s.db.Model(&SSHKey{}).Preload("User").Preload("Device")
	
	// 支持按用户过滤
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}
	
	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	
	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch SSH keys",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
		"total":   len(keys),
	})
}

// handleCreateSSHKey 创建SSH密钥
func (s *UnlimitedControlServer) handleCreateSSHKey(c *gin.Context) {
	var req SSHKey
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 验证SSH密钥
	sm := NewSSHManager(s)
	if err := sm.ValidateSSHKey(req.PublicKey); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 解析密钥信息
	keyType, fingerprint, comment, err := sm.ParseSSHPublicKey(req.PublicKey)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查指纹是否已存在
	var existing SSHKey
	if err := s.db.Where("fingerprint = ?", fingerprint).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "SSH key with this fingerprint already exists",
		})
		return
	}

	// 检查用户是否存在
	var user User
	if err := s.db.First(&user, req.UserID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "User not found",
		})
		return
	}

	// 如果指定了设备，检查设备是否存在
	if req.DeviceID > 0 {
		var device Device
		if err := s.db.First(&device, req.DeviceID).Error; err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Device not found",
			})
			return
		}
	}

	// 设置解析的信息
	req.KeyType = keyType
	req.Fingerprint = fingerprint
	if req.Comment == "" {
		req.Comment = comment
	}
	req.Enabled = true

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create SSH key",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetSSHKey 获取单个SSH密钥
func (s *UnlimitedControlServer) handleGetSSHKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var key SSHKey
	if err := s.db.Preload("User").Preload("Device").First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH key not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    key,
	})
}

// handleUpdateSSHKey 更新SSH密钥
func (s *UnlimitedControlServer) handleUpdateSSHKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var req SSHKey
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var key SSHKey
	if err := s.db.First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH key not found",
		})
		return
	}

	// 如果更新了公钥，需要重新验证和解析
	if req.PublicKey != key.PublicKey {
		sm := NewSSHManager(s)
		if err := sm.ValidateSSHKey(req.PublicKey); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		keyType, fingerprint, comment, err := sm.ParseSSHPublicKey(req.PublicKey)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": err.Error(),
			})
			return
		}

		// 检查新指纹是否已存在
		var existing SSHKey
		if err := s.db.Where("fingerprint = ? AND id != ?", fingerprint, key.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "SSH key with this fingerprint already exists",
			})
			return
		}

		key.PublicKey = req.PublicKey
		key.KeyType = keyType
		key.Fingerprint = fingerprint
		if req.Comment == "" {
			key.Comment = comment
		} else {
			key.Comment = req.Comment
		}
	} else {
		// 只更新注释
		key.Comment = req.Comment
	}

	key.Enabled = req.Enabled

	if err := s.db.Save(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update SSH key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    key,
	})
}

// handleDeleteSSHKey 删除SSH密钥
func (s *UnlimitedControlServer) handleDeleteSSHKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var key SSHKey
	if err := s.db.First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH key not found",
		})
		return
	}

	if err := s.db.Delete(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete SSH key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSH key deleted successfully",
	})
}

// handleEnableSSHKey 启用SSH密钥
func (s *UnlimitedControlServer) handleEnableSSHKey(c *gin.Context) {
	s.toggleSSHKey(c, true)
}

// handleDisableSSHKey 禁用SSH密钥
func (s *UnlimitedControlServer) handleDisableSSHKey(c *gin.Context) {
	s.toggleSSHKey(c, false)
}

// toggleSSHKey 切换SSH密钥状态
func (s *UnlimitedControlServer) toggleSSHKey(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var key SSHKey
	if err := s.db.First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH key not found",
		})
		return
	}

	key.Enabled = enabled
	if err := s.db.Save(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update key status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("SSH key %s successfully", action),
		"data":    key,
	})
}

// ===== SSH配置管理处理函数 =====

// handleGetSSHConfig 获取SSH配置
func (s *UnlimitedControlServer) handleGetSSHConfig(c *gin.Context) {
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

	var config SSHConfig
	if err := s.db.Where("device_id = ?", uint(deviceID)).First(&config).Error; err != nil {
		// 创建默认配置
		allowedUsers := []string{"root", "admin"}
		allowedUsersJSON, _ := json.Marshal(allowedUsers)

		config = SSHConfig{
			DeviceID:     uint(deviceID),
			Enabled:      false,
			Port:         22,
			AllowedUsers: string(allowedUsersJSON),
			PasswordAuth: false,
			KeyAuth:      true,
			RootLogin:    false,
			ForwardAgent: false,
			ForwardX11:   false,
		}
		s.db.Create(&config)
	}

	// 生成authorized_keys
	sm := NewSSHManager(s)
	authorizedKeys, _ := sm.GenerateAuthorizedKeys(uint(deviceID))
	config.AuthorizedKeys = authorizedKeys

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleUpdateSSHConfig 更新SSH配置
func (s *UnlimitedControlServer) handleUpdateSSHConfig(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var req SSHConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
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

	var config SSHConfig
	if err := s.db.Where("device_id = ?", uint(deviceID)).First(&config).Error; err != nil {
		// 创建新配置
		config = SSHConfig{
			DeviceID: uint(deviceID),
		}
	}

	// 更新配置
	config.Enabled = req.Enabled
	config.Port = req.Port
	config.AllowedUsers = req.AllowedUsers
	config.PasswordAuth = req.PasswordAuth
	config.KeyAuth = req.KeyAuth
	config.RootLogin = req.RootLogin
	config.ForwardAgent = req.ForwardAgent
	config.ForwardX11 = req.ForwardX11

	// 验证端口范围
	if config.Port < 1 || config.Port > 65535 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid port number",
		})
		return
	}

	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update SSH configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// ===== SSH会话管理处理函数 =====

// handleListSSHSessions 获取SSH会话列表
func (s *UnlimitedControlServer) handleListSSHSessions(c *gin.Context) {
	var sessions []SSHSession

	query := s.db.Model(&SSHSession{}).Preload("User").Preload("SourceDevice").Preload("TargetDevice")

	// 支持按用户过滤
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// 支持按源设备过滤
	if sourceDeviceID := c.Query("source_device_id"); sourceDeviceID != "" {
		query = query.Where("source_device_id = ?", sourceDeviceID)
	}

	// 支持按目标设备过滤
	if targetDeviceID := c.Query("target_device_id"); targetDeviceID != "" {
		query = query.Where("target_device_id = ?", targetDeviceID)
	}

	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("start_time DESC").Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch SSH sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sessions,
		"total":   len(sessions),
	})
}

// handleGetSSHSession 获取单个SSH会话
func (s *UnlimitedControlServer) handleGetSSHSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid session ID",
		})
		return
	}

	var session SSHSession
	if err := s.db.Preload("User").Preload("SourceDevice").Preload("TargetDevice").First(&session, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH session not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    session,
	})
}

// handleTerminateSSHSession 终止SSH会话
func (s *UnlimitedControlServer) handleTerminateSSHSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid session ID",
		})
		return
	}

	var session SSHSession
	if err := s.db.First(&session, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "SSH session not found",
		})
		return
	}

	if session.Status != "active" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Session is not active",
		})
		return
	}

	// 更新会话状态
	session.Status = "terminated"
	session.EndTime = time.Now()
	session.Duration = int64(session.EndTime.Sub(session.StartTime).Seconds())

	if err := s.db.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to terminate session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "SSH session terminated successfully",
		"data":    session,
	})
}

// ===== SSH连接处理函数 =====

// handleSSHConnect 处理SSH连接请求
func (s *UnlimitedControlServer) handleSSHConnect(c *gin.Context) {
	var req struct {
		SourceDeviceID uint   `json:"source_device_id" binding:"required"`
		TargetDeviceID uint   `json:"target_device_id" binding:"required"`
		Username       string `json:"username" binding:"required"`
		Command        string `json:"command,omitempty"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查源设备
	var sourceDevice Device
	if err := s.db.Preload("User").First(&sourceDevice, req.SourceDeviceID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Source device not found",
		})
		return
	}

	// 检查目标设备
	var targetDevice Device
	if err := s.db.First(&targetDevice, req.TargetDeviceID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Target device not found",
		})
		return
	}

	// 检查目标设备的SSH配置
	var sshConfig SSHConfig
	if err := s.db.Where("device_id = ?", req.TargetDeviceID).First(&sshConfig).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SSH not configured on target device",
		})
		return
	}

	if !sshConfig.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "SSH is disabled on target device",
		})
		return
	}

	// 检查用户是否被允许
	var allowedUsers []string
	json.Unmarshal([]byte(sshConfig.AllowedUsers), &allowedUsers)

	userAllowed := false
	for _, user := range allowedUsers {
		if user == req.Username {
			userAllowed = true
			break
		}
	}

	if !userAllowed {
		c.JSON(http.StatusForbidden, gin.H{
			"success": false,
			"message": "User not allowed to SSH to this device",
		})
		return
	}

	// 创建SSH会话记录
	session := SSHSession{
		UserID:         sourceDevice.UserID,
		SourceDeviceID: req.SourceDeviceID,
		TargetDeviceID: req.TargetDeviceID,
		Username:       req.Username,
		Command:        req.Command,
		Status:         "active",
		StartTime:      time.Now(),
	}

	if err := s.db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create session record",
		})
		return
	}

	// 获取目标设备IP
	var targetIPs []string
	json.Unmarshal([]byte(targetDevice.IPAddresses), &targetIPs)

	if len(targetIPs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Target device has no IP addresses",
		})
		return
	}

	// 构建SSH连接信息
	connectionInfo := map[string]interface{}{
		"session_id":    session.ID,
		"target_ip":     targetIPs[0],
		"target_port":   sshConfig.Port,
		"username":      req.Username,
		"command":       req.Command,
		"ssh_command":   fmt.Sprintf("ssh %s@%s -p %d", req.Username, targetIPs[0], sshConfig.Port),
		"target_device": targetDevice.Hostname,
		"source_device": sourceDevice.Hostname,
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    connectionInfo,
	})
}

// handleGetSSHHosts 获取SSH主机列表
func (s *UnlimitedControlServer) handleGetSSHHosts(c *gin.Context) {
	var devices []Device
	s.db.Where("online = ? AND authorized = ?", true, true).Preload("User").Find(&devices)

	hosts := make([]map[string]interface{}, 0)
	for _, device := range devices {
		// 检查设备是否启用了SSH
		var sshConfig SSHConfig
		sshEnabled := false
		if err := s.db.Where("device_id = ? AND enabled = ?", device.ID, true).First(&sshConfig).Error; err == nil {
			sshEnabled = true
		}

		if sshEnabled {
			var ips []string
			json.Unmarshal([]byte(device.IPAddresses), &ips)

			if len(ips) > 0 {
				var allowedUsers []string
				json.Unmarshal([]byte(sshConfig.AllowedUsers), &allowedUsers)

				host := map[string]interface{}{
					"device_id":     device.ID,
					"hostname":      device.Hostname,
					"given_name":    device.GivenName,
					"ip_addresses":  ips,
					"ssh_port":      sshConfig.Port,
					"allowed_users": allowedUsers,
					"user":          device.User.Name,
					"online":        device.Online,
					"last_seen":     device.LastSeen,
				}
				hosts = append(hosts, host)
			}
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    hosts,
		"total":   len(hosts),
	})
}
