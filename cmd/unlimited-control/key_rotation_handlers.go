package main

import (
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"golang.org/x/crypto/curve25519"
)

// KeyRotationManager 密钥轮换管理器
type KeyRotationManager struct {
	server *UnlimitedControlServer
}

// NewKeyRotationManager 创建密钥轮换管理器
func NewKeyRotationManager(server *UnlimitedControlServer) *KeyRotationManager {
	return &KeyRotationManager{
		server: server,
	}
}

// GenerateKeyPair 生成密钥对
func (krm *KeyRotationManager) GenerateKeyPair(keyType string) (publicKey, privateKey string, err error) {
	switch keyType {
	case "curve25519":
		// 生成Curve25519密钥对
		var private [32]byte
		if _, err := rand.Read(private[:]); err != nil {
			return "", "", err
		}
		
		var public [32]byte
		curve25519.ScalarBaseMult(&public, &private)
		
		publicKey = base64.StdEncoding.EncodeToString(public[:])
		privateKey = base64.StdEncoding.EncodeToString(private[:])
		
	case "ed25519":
		// 生成Ed25519密钥对
		pub, priv, err := ed25519.GenerateKey(rand.Reader)
		if err != nil {
			return "", "", err
		}
		
		publicKey = base64.StdEncoding.EncodeToString(pub)
		privateKey = base64.StdEncoding.EncodeToString(priv)
		
	default:
		return "", "", fmt.Errorf("unsupported key type: %s", keyType)
	}
	
	return publicKey, privateKey, nil
}

// ScheduleRotationForDevice 为设备安排密钥轮换
func (krm *KeyRotationManager) ScheduleRotationForDevice(deviceID uint, policyID uint) error {
	var policy KeyRotationPolicy
	if err := krm.server.db.First(&policy, policyID).Error; err != nil {
		return err
	}

	if !policy.Enabled || !policy.AutoRotate {
		return fmt.Errorf("policy is not enabled or auto-rotate is disabled")
	}

	// 计算下次轮换时间
	scheduledAt := time.Now().Add(time.Duration(policy.RotationInterval) * time.Hour)

	// 创建轮换任务
	job := KeyRotationJob{
		PolicyID:    policyID,
		DeviceID:    deviceID,
		Status:      "pending",
		ScheduledAt: scheduledAt,
	}

	return krm.server.db.Create(&job).Error
}

// ExecuteKeyRotation 执行密钥轮换
func (krm *KeyRotationManager) ExecuteKeyRotation(jobID uint) error {
	var job KeyRotationJob
	if err := krm.server.db.Preload("Policy").Preload("Device").First(&job, jobID).Error; err != nil {
		return err
	}

	if job.Status != "pending" {
		return fmt.Errorf("job is not in pending status")
	}

	// 更新任务状态
	job.Status = "running"
	job.StartedAt = time.Now()
	krm.server.db.Save(&job)

	// 记录开始日志
	krm.logRotationAction(job.ID, job.DeviceID, "start", "success", "Key rotation started")

	// 获取当前活跃密钥
	var currentKey DeviceKey
	err := krm.server.db.Where("device_id = ? AND status = ?", job.DeviceID, "active").First(&currentKey).Error
	if err != nil {
		// 如果没有活跃密钥，创建第一个密钥
		return krm.createInitialKey(job)
	}

	// 生成新密钥
	newKey, err := krm.generateNewKey(job.DeviceID, currentKey.KeyVersion+1)
	if err != nil {
		krm.failJob(job, fmt.Sprintf("Failed to generate new key: %v", err))
		return err
	}

	// 更新任务信息
	job.OldKeyID = currentKey.ID
	job.NewKeyID = newKey.ID

	// 激活新密钥
	if err := krm.activateKey(newKey.ID); err != nil {
		krm.failJob(job, fmt.Sprintf("Failed to activate new key: %v", err))
		return err
	}

	// 等待宽限期后撤销旧密钥
	go krm.scheduleKeyRevocation(currentKey.ID, job.Policy.GracePeriod)

	// 完成任务
	job.Status = "completed"
	job.CompletedAt = time.Now()
	krm.server.db.Save(&job)

	krm.logRotationAction(job.ID, job.DeviceID, "complete", "success", "Key rotation completed successfully")

	return nil
}

// createInitialKey 创建初始密钥
func (krm *KeyRotationManager) createInitialKey(job KeyRotationJob) error {
	newKey, err := krm.generateNewKey(job.DeviceID, 1)
	if err != nil {
		krm.failJob(job, fmt.Sprintf("Failed to generate initial key: %v", err))
		return err
	}

	// 立即激活初始密钥
	if err := krm.activateKey(newKey.ID); err != nil {
		krm.failJob(job, fmt.Sprintf("Failed to activate initial key: %v", err))
		return err
	}

	// 更新任务
	job.NewKeyID = newKey.ID
	job.Status = "completed"
	job.CompletedAt = time.Now()
	krm.server.db.Save(&job)

	krm.logRotationAction(job.ID, job.DeviceID, "initial", "success", "Initial key created and activated")

	return nil
}

// generateNewKey 生成新密钥
func (krm *KeyRotationManager) generateNewKey(deviceID uint, version int) (*DeviceKey, error) {
	publicKey, privateKey, err := krm.GenerateKeyPair("curve25519")
	if err != nil {
		return nil, err
	}

	key := DeviceKey{
		DeviceID:   deviceID,
		KeyVersion: version,
		PublicKey:  publicKey,
		PrivateKey: privateKey,
		KeyType:    "curve25519",
		Status:     "pending",
		IssuedAt:   time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour), // 24小时后过期
	}

	if err := krm.server.db.Create(&key).Error; err != nil {
		return nil, err
	}

	krm.logRotationAction(0, deviceID, "generate", "success", fmt.Sprintf("New key generated (version %d)", version))

	return &key, nil
}

// activateKey 激活密钥
func (krm *KeyRotationManager) activateKey(keyID uint) error {
	var key DeviceKey
	if err := krm.server.db.First(&key, keyID).Error; err != nil {
		return err
	}

	// 将设备的其他密钥设为非活跃
	krm.server.db.Model(&DeviceKey{}).Where("device_id = ? AND id != ?", key.DeviceID, keyID).Update("status", "expired")

	// 激活新密钥
	key.Status = "active"
	key.ActivatedAt = time.Now()
	if err := krm.server.db.Save(&key).Error; err != nil {
		return err
	}

	krm.logRotationAction(0, key.DeviceID, "activate", "success", fmt.Sprintf("Key activated (version %d)", key.KeyVersion))

	return nil
}

// scheduleKeyRevocation 安排密钥撤销
func (krm *KeyRotationManager) scheduleKeyRevocation(keyID uint, gracePeriodHours int) {
	time.Sleep(time.Duration(gracePeriodHours) * time.Hour)
	
	var key DeviceKey
	if err := krm.server.db.First(&key, keyID).Error; err != nil {
		return
	}

	key.Status = "revoked"
	key.RevokedAt = time.Now()
	krm.server.db.Save(&key)

	krm.logRotationAction(0, key.DeviceID, "revoke", "success", fmt.Sprintf("Old key revoked (version %d)", key.KeyVersion))
}

// failJob 标记任务失败
func (krm *KeyRotationManager) failJob(job KeyRotationJob, errorMessage string) {
	job.Status = "failed"
	job.ErrorMessage = errorMessage
	job.CompletedAt = time.Now()
	job.RetryCount++
	krm.server.db.Save(&job)

	krm.logRotationAction(job.ID, job.DeviceID, "fail", "failed", errorMessage)
}

// logRotationAction 记录轮换操作日志
func (krm *KeyRotationManager) logRotationAction(jobID, deviceID uint, action, status, message string) {
	log := KeyRotationLog{
		JobID:     jobID,
		DeviceID:  deviceID,
		Action:    action,
		Status:    status,
		Message:   message,
		CreatedAt: time.Now(),
	}
	krm.server.db.Create(&log)
}

// ===== 密钥轮换策略管理处理函数 =====

// handleListKeyRotationPolicies 获取密钥轮换策略列表
func (s *UnlimitedControlServer) handleListKeyRotationPolicies(c *gin.Context) {
	var policies []KeyRotationPolicy
	
	query := s.db.Model(&KeyRotationPolicy{})
	
	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&policies).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch key rotation policies",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policies,
		"total":   len(policies),
	})
}

// handleCreateKeyRotationPolicy 创建密钥轮换策略
func (s *UnlimitedControlServer) handleCreateKeyRotationPolicy(c *gin.Context) {
	var req KeyRotationPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查名称是否已存在
	var existing KeyRotationPolicy
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Key rotation policy with this name already exists",
		})
		return
	}

	// 验证必填字段
	if req.Name == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Policy name is required",
		})
		return
	}

	// 设置默认值
	if req.RotationInterval <= 0 {
		req.RotationInterval = 168 // 7天
	}
	if req.GracePeriod <= 0 {
		req.GracePeriod = 24 // 1天
	}
	if req.MaxKeyAge <= 0 {
		req.MaxKeyAge = 720 // 30天
	}
	if req.NotifyBeforeRotation <= 0 {
		req.NotifyBeforeRotation = 24 // 1天
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create key rotation policy",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetKeyRotationPolicy 获取单个密钥轮换策略
func (s *UnlimitedControlServer) handleGetKeyRotationPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy KeyRotationPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleUpdateKeyRotationPolicy 更新密钥轮换策略
func (s *UnlimitedControlServer) handleUpdateKeyRotationPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var req KeyRotationPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var policy KeyRotationPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation policy not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != policy.Name {
		var existing KeyRotationPolicy
		if err := s.db.Where("name = ? AND id != ?", req.Name, policy.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Key rotation policy with this name already exists",
			})
			return
		}
	}

	// 更新字段
	policy.Name = req.Name
	policy.RotationInterval = req.RotationInterval
	policy.GracePeriod = req.GracePeriod
	policy.AutoRotate = req.AutoRotate
	policy.NotifyBeforeRotation = req.NotifyBeforeRotation
	policy.MaxKeyAge = req.MaxKeyAge
	policy.TargetDeviceGroups = req.TargetDeviceGroups
	policy.Enabled = req.Enabled

	if err := s.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update key rotation policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleDeleteKeyRotationPolicy 删除密钥轮换策略
func (s *UnlimitedControlServer) handleDeleteKeyRotationPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy KeyRotationPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation policy not found",
		})
		return
	}

	if err := s.db.Delete(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete key rotation policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Key rotation policy deleted successfully",
	})
}

// handleEnableKeyRotationPolicy 启用密钥轮换策略
func (s *UnlimitedControlServer) handleEnableKeyRotationPolicy(c *gin.Context) {
	s.toggleKeyRotationPolicy(c, true)
}

// handleDisableKeyRotationPolicy 禁用密钥轮换策略
func (s *UnlimitedControlServer) handleDisableKeyRotationPolicy(c *gin.Context) {
	s.toggleKeyRotationPolicy(c, false)
}

// toggleKeyRotationPolicy 切换密钥轮换策略状态
func (s *UnlimitedControlServer) toggleKeyRotationPolicy(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy KeyRotationPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation policy not found",
		})
		return
	}

	policy.Enabled = enabled
	if err := s.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update policy status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Key rotation policy %s successfully", action),
		"data":    policy,
	})
}

// ===== 设备密钥管理处理函数 =====

// handleListDeviceKeys 获取设备密钥列表
func (s *UnlimitedControlServer) handleListDeviceKeys(c *gin.Context) {
	var keys []DeviceKey

	query := s.db.Model(&DeviceKey{}).Preload("Device")

	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch device keys",
		})
		return
	}

	// 隐藏私钥
	for i := range keys {
		keys[i].PrivateKey = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
		"total":   len(keys),
	})
}

// handleCreateDeviceKey 创建设备密钥
func (s *UnlimitedControlServer) handleCreateDeviceKey(c *gin.Context) {
	var req struct {
		DeviceID uint   `json:"device_id" binding:"required"`
		KeyType  string `json:"key_type"`
	}
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

	// 设置默认密钥类型
	if req.KeyType == "" {
		req.KeyType = "curve25519"
	}

	// 获取下一个版本号
	var lastKey DeviceKey
	version := 1
	if err := s.db.Where("device_id = ?", req.DeviceID).Order("key_version DESC").First(&lastKey).Error; err == nil {
		version = lastKey.KeyVersion + 1
	}

	krm := NewKeyRotationManager(s)
	newKey, err := krm.generateNewKey(req.DeviceID, version)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate device key",
		})
		return
	}

	// 返回包含私钥的响应（仅此一次）
	response := *newKey
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    response,
	})
}

// handleGetDeviceKey 获取单个设备密钥
func (s *UnlimitedControlServer) handleGetDeviceKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var key DeviceKey
	if err := s.db.Preload("Device").First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Device key not found",
		})
		return
	}

	// 隐藏私钥
	key.PrivateKey = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    key,
	})
}

// handleActivateDeviceKey 激活设备密钥
func (s *UnlimitedControlServer) handleActivateDeviceKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	krm := NewKeyRotationManager(s)
	if err := krm.activateKey(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to activate device key",
		})
		return
	}

	var key DeviceKey
	s.db.First(&key, uint(id))
	key.PrivateKey = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device key activated successfully",
		"data":    key,
	})
}

// handleRevokeDeviceKey 撤销设备密钥
func (s *UnlimitedControlServer) handleRevokeDeviceKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid key ID",
		})
		return
	}

	var key DeviceKey
	if err := s.db.First(&key, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Device key not found",
		})
		return
	}

	if key.Status == "revoked" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Key is already revoked",
		})
		return
	}

	key.Status = "revoked"
	key.RevokedAt = time.Now()
	if err := s.db.Save(&key).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to revoke device key",
		})
		return
	}

	krm := NewKeyRotationManager(s)
	krm.logRotationAction(0, key.DeviceID, "revoke", "success", fmt.Sprintf("Key manually revoked (version %d)", key.KeyVersion))

	key.PrivateKey = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Device key revoked successfully",
		"data":    key,
	})
}

// handleGetDeviceKeysByDevice 获取设备的所有密钥
func (s *UnlimitedControlServer) handleGetDeviceKeysByDevice(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var keys []DeviceKey
	if err := s.db.Where("device_id = ?", uint(deviceID)).Order("key_version DESC").Find(&keys).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch device keys",
		})
		return
	}

	// 隐藏私钥
	for i := range keys {
		keys[i].PrivateKey = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    keys,
		"total":   len(keys),
	})
}

// ===== 轮换任务管理处理函数 =====

// handleListKeyRotationJobs 获取密钥轮换任务列表
func (s *UnlimitedControlServer) handleListKeyRotationJobs(c *gin.Context) {
	var jobs []KeyRotationJob

	query := s.db.Model(&KeyRotationJob{}).Preload("Policy").Preload("Device").Preload("OldKey").Preload("NewKey")

	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}

	if err := query.Order("created_at DESC").Find(&jobs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch key rotation jobs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    jobs,
		"total":   len(jobs),
	})
}

// handleCreateKeyRotationJob 创建密钥轮换任务
func (s *UnlimitedControlServer) handleCreateKeyRotationJob(c *gin.Context) {
	var req struct {
		PolicyID    uint      `json:"policy_id" binding:"required"`
		DeviceID    uint      `json:"device_id" binding:"required"`
		ScheduledAt time.Time `json:"scheduled_at"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查策略是否存在
	var policy KeyRotationPolicy
	if err := s.db.First(&policy, req.PolicyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Policy not found",
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

	// 设置默认计划时间
	if req.ScheduledAt.IsZero() {
		req.ScheduledAt = time.Now()
	}

	job := KeyRotationJob{
		PolicyID:    req.PolicyID,
		DeviceID:    req.DeviceID,
		Status:      "pending",
		ScheduledAt: req.ScheduledAt,
	}

	if err := s.db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create key rotation job",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    job,
	})
}

// handleGetKeyRotationJob 获取单个密钥轮换任务
func (s *UnlimitedControlServer) handleGetKeyRotationJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid job ID",
		})
		return
	}

	var job KeyRotationJob
	if err := s.db.Preload("Policy").Preload("Device").Preload("OldKey").Preload("NewKey").First(&job, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation job not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    job,
	})
}

// handleExecuteKeyRotationJob 执行密钥轮换任务
func (s *UnlimitedControlServer) handleExecuteKeyRotationJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid job ID",
		})
		return
	}

	krm := NewKeyRotationManager(s)
	if err := krm.ExecuteKeyRotation(uint(id)); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to execute key rotation",
			"error":   err.Error(),
		})
		return
	}

	var job KeyRotationJob
	s.db.Preload("Policy").Preload("Device").First(&job, uint(id))

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Key rotation executed successfully",
		"data":    job,
	})
}

// handleCancelKeyRotationJob 取消密钥轮换任务
func (s *UnlimitedControlServer) handleCancelKeyRotationJob(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid job ID",
		})
		return
	}

	var job KeyRotationJob
	if err := s.db.First(&job, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Key rotation job not found",
		})
		return
	}

	if job.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Can only cancel pending jobs",
		})
		return
	}

	if err := s.db.Delete(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to cancel key rotation job",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Key rotation job cancelled successfully",
	})
}

// handleScheduleKeyRotation 安排密钥轮换
func (s *UnlimitedControlServer) handleScheduleKeyRotation(c *gin.Context) {
	var req struct {
		PolicyID uint `json:"policy_id" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 获取策略
	var policy KeyRotationPolicy
	if err := s.db.First(&policy, req.PolicyID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Policy not found",
		})
		return
	}

	if !policy.Enabled {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Policy is not enabled",
		})
		return
	}

	// 获取目标设备
	var devices []Device
	query := s.db.Where("authorized = ?", true)

	// 如果指定了设备组，过滤设备
	if policy.TargetDeviceGroups != "" && policy.TargetDeviceGroups != "[]" {
		var groups []string
		json.Unmarshal([]byte(policy.TargetDeviceGroups), &groups)
		if len(groups) > 0 {
			// 这里可以根据设备组过滤，暂时跳过
		}
	}

	if err := query.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch devices",
		})
		return
	}

	krm := NewKeyRotationManager(s)
	var createdJobs []KeyRotationJob

	// 为每个设备创建轮换任务
	for _, device := range devices {
		if err := krm.ScheduleRotationForDevice(device.ID, policy.ID); err == nil {
			var job KeyRotationJob
			s.db.Where("device_id = ? AND policy_id = ? AND status = ?",
				device.ID, policy.ID, "pending").Last(&job)
			createdJobs = append(createdJobs, job)
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Scheduled key rotation for %d devices", len(createdJobs)),
		"data":    createdJobs,
	})
}

// ===== 轮换日志处理函数 =====

// handleListKeyRotationLogs 获取密钥轮换日志列表
func (s *UnlimitedControlServer) handleListKeyRotationLogs(c *gin.Context) {
	var logs []KeyRotationLog

	query := s.db.Model(&KeyRotationLog{}).Preload("Job").Preload("Device")

	// 支持按操作过滤
	if action := c.Query("action"); action != "" {
		query = query.Where("action = ?", action)
	}

	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch key rotation logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// handleGetKeyRotationLogsByJob 获取任务的轮换日志
func (s *UnlimitedControlServer) handleGetKeyRotationLogsByJob(c *gin.Context) {
	jobID, err := strconv.ParseUint(c.Param("job_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid job ID",
		})
		return
	}

	var logs []KeyRotationLog
	if err := s.db.Where("job_id = ?", uint(jobID)).Order("created_at ASC").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch job logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// handleGetKeyRotationLogsByDevice 获取设备的轮换日志
func (s *UnlimitedControlServer) handleGetKeyRotationLogsByDevice(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var logs []KeyRotationLog
	if err := s.db.Where("device_id = ?", uint(deviceID)).Order("created_at DESC").Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch device logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// ===== 手动轮换处理函数 =====

// handleManualKeyRotation 手动密钥轮换
func (s *UnlimitedControlServer) handleManualKeyRotation(c *gin.Context) {
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
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Device not found",
		})
		return
	}

	// 创建手动轮换任务
	job := KeyRotationJob{
		PolicyID:    0, // 手动轮换不关联策略
		DeviceID:    uint(deviceID),
		Status:      "pending",
		ScheduledAt: time.Now(),
	}

	if err := s.db.Create(&job).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create rotation job",
		})
		return
	}

	// 立即执行轮换
	krm := NewKeyRotationManager(s)
	if err := krm.ExecuteKeyRotation(job.ID); err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to execute key rotation",
			"error":   err.Error(),
		})
		return
	}

	// 重新加载任务信息
	s.db.Preload("Device").First(&job, job.ID)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Manual key rotation completed successfully",
		"data":    job,
	})
}

// handleBatchKeyRotation 批量密钥轮换
func (s *UnlimitedControlServer) handleBatchKeyRotation(c *gin.Context) {
	var req struct {
		DeviceIDs []uint `json:"device_ids" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	if len(req.DeviceIDs) == 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No device IDs provided",
		})
		return
	}

	krm := NewKeyRotationManager(s)
	var results []map[string]interface{}

	for _, deviceID := range req.DeviceIDs {
		// 检查设备是否存在
		var device Device
		if err := s.db.First(&device, deviceID).Error; err != nil {
			results = append(results, map[string]interface{}{
				"device_id": deviceID,
				"success":   false,
				"message":   "Device not found",
			})
			continue
		}

		// 创建轮换任务
		job := KeyRotationJob{
			PolicyID:    0,
			DeviceID:    deviceID,
			Status:      "pending",
			ScheduledAt: time.Now(),
		}

		if err := s.db.Create(&job).Error; err != nil {
			results = append(results, map[string]interface{}{
				"device_id": deviceID,
				"success":   false,
				"message":   "Failed to create rotation job",
			})
			continue
		}

		// 执行轮换
		if err := krm.ExecuteKeyRotation(job.ID); err != nil {
			results = append(results, map[string]interface{}{
				"device_id": deviceID,
				"success":   false,
				"message":   err.Error(),
			})
		} else {
			results = append(results, map[string]interface{}{
				"device_id": deviceID,
				"success":   true,
				"message":   "Key rotation completed",
				"job_id":    job.ID,
			})
		}
	}

	successCount := 0
	for _, result := range results {
		if result["success"].(bool) {
			successCount++
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Batch key rotation completed: %d/%d successful", successCount, len(req.DeviceIDs)),
		"data":    results,
	})
}
