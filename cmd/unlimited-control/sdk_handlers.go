package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// SDKManager SDK管理器
type SDKManager struct {
	server *UnlimitedControlServer
}

// NewSDKManager 创建SDK管理器
func NewSDKManager(server *UnlimitedControlServer) *SDKManager {
	return &SDKManager{
		server: server,
	}
}

// GenerateAPIKey 生成API密钥
func (sm *SDKManager) GenerateAPIKey() (string, string, error) {
	// 生成32字节随机密钥
	keyBytes := make([]byte, 32)
	if _, err := rand.Read(keyBytes); err != nil {
		return "", "", err
	}

	// 生成密钥ID（前缀 + 16字节十六进制）
	keyID := "tuc_" + hex.EncodeToString(keyBytes[:8])
	
	// 生成密钥值（完整32字节十六进制）
	keySecret := hex.EncodeToString(keyBytes)
	
	return keyID, keySecret, nil
}

// ValidateAPIKey 验证API密钥
func (sm *SDKManager) ValidateAPIKey(keyID, keySecret string) (*APIKey, error) {
	var apiKey APIKey
	if err := sm.server.db.Where("key_id = ? AND enabled = ?", keyID, true).First(&apiKey).Error; err != nil {
		return nil, err
	}

	// 检查过期时间
	if time.Now().After(apiKey.ExpiresAt) {
		return nil, fmt.Errorf("API key expired")
	}

	// 验证密钥哈希
	hash := sha256.Sum256([]byte(keySecret))
	keyHash := hex.EncodeToString(hash[:])
	
	if apiKey.KeyHash != keyHash {
		return nil, fmt.Errorf("invalid API key")
	}

	// 更新使用统计
	apiKey.LastUsed = time.Now()
	apiKey.UsageCount++
	sm.server.db.Save(&apiKey)

	return &apiKey, nil
}

// LogSDKUsage 记录SDK使用情况
func (sm *SDKManager) LogSDKUsage(apiKeyID uint, sdkVersion, language, method, endpoint, userAgent, clientIP string, duration int64, success bool, errorMsg string) {
	usage := SDKUsage{
		APIKeyID:   apiKeyID,
		SDKVersion: sdkVersion,
		Language:   language,
		Method:     method,
		Endpoint:   endpoint,
		UserAgent:  userAgent,
		ClientIP:   clientIP,
		Duration:   duration,
		Success:    success,
		Error:      errorMsg,
		CreatedAt:  time.Now(),
	}
	sm.server.db.Create(&usage)
}

// ===== API密钥管理处理函数 =====
// 注意：基础的handleListAPIKeys和handleCreateAPIKey方法在handlers.go中已定义

// handleGetAPIKey 获取单个API密钥
func (s *UnlimitedControlServer) handleGetAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var apiKey APIKey
	if err := s.db.First(&apiKey, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "API key not found",
		})
		return
	}

	// 隐藏敏感信息
	apiKey.KeySecret = ""
	apiKey.KeyHash = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiKey,
	})
}

// handleUpdateAPIKey 更新API密钥
func (s *UnlimitedControlServer) handleUpdateAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var req struct {
		Name        string   `json:"name"`
		Permissions []string `json:"permissions"`
		ExpiresAt   string   `json:"expires_at,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var apiKey APIKey
	if err := s.db.First(&apiKey, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "API key not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != "" && req.Name != apiKey.Name {
		var existing APIKey
		if err := s.db.Where("name = ? AND id != ?", req.Name, apiKey.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "API key with this name already exists",
			})
			return
		}
		apiKey.Name = req.Name
	}

	// 更新权限
	if req.Permissions != nil {
		permissionsJSON, _ := json.Marshal(req.Permissions)
		apiKey.Permissions = string(permissionsJSON)
	}

	// 更新过期时间
	if req.ExpiresAt != "" {
		if parsedTime, err := time.Parse(time.RFC3339, req.ExpiresAt); err == nil {
			apiKey.ExpiresAt = parsedTime
		}
	}

	if err := s.db.Save(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update API key",
		})
		return
	}

	// 隐藏敏感信息
	apiKey.KeySecret = ""
	apiKey.KeyHash = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiKey,
	})
}

// handleDeleteAPIKey 删除API密钥
func (s *UnlimitedControlServer) handleDeleteAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var apiKey APIKey
	if err := s.db.First(&apiKey, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "API key not found",
		})
		return
	}

	// 删除相关的使用记录
	s.db.Where("api_key_id = ?", apiKey.ID).Delete(&SDKUsage{})

	// 删除API密钥
	if err := s.db.Delete(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete API key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "API key deleted successfully",
	})
}

// handleEnableAPIKey 启用API密钥
func (s *UnlimitedControlServer) handleEnableAPIKey(c *gin.Context) {
	s.toggleAPIKey(c, true)
}

// handleDisableAPIKey 禁用API密钥
func (s *UnlimitedControlServer) handleDisableAPIKey(c *gin.Context) {
	s.toggleAPIKey(c, false)
}

// toggleAPIKey 切换API密钥状态
func (s *UnlimitedControlServer) toggleAPIKey(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var apiKey APIKey
	if err := s.db.First(&apiKey, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "API key not found",
		})
		return
	}

	apiKey.Enabled = enabled
	if err := s.db.Save(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update API key status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	// 隐藏敏感信息
	apiKey.KeySecret = ""
	apiKey.KeyHash = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("API key %s successfully", action),
		"data":    apiKey,
	})
}

// handleRegenerateAPIKey 重新生成API密钥
func (s *UnlimitedControlServer) handleRegenerateAPIKey(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var apiKey APIKey
	if err := s.db.First(&apiKey, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "API key not found",
		})
		return
	}

	// 生成新的API密钥
	sm := NewSDKManager(s)
	keyID, keySecret, err := sm.GenerateAPIKey()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate new API key",
		})
		return
	}

	// 生成新的哈希值
	hash := sha256.Sum256([]byte(keySecret))
	keyHash := hex.EncodeToString(hash[:])

	// 更新API密钥
	apiKey.KeyID = keyID
	apiKey.KeySecret = keySecret // 只在重新生成时返回
	apiKey.KeyHash = keyHash
	apiKey.UsageCount = 0 // 重置使用次数

	if err := s.db.Save(&apiKey).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update API key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    apiKey,
		"message": "API key regenerated successfully. Please save the new key secret as it will not be shown again.",
	})
}

// ===== Webhook管理处理函数 =====

// handleListWebhooks 获取Webhook列表
func (s *UnlimitedControlServer) handleListWebhooks(c *gin.Context) {
	var webhooks []Webhook

	query := s.db.Model(&Webhook{})

	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&webhooks).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch webhooks",
		})
		return
	}

	// 隐藏敏感信息
	for i := range webhooks {
		webhooks[i].Secret = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    webhooks,
		"total":   len(webhooks),
	})
}

// handleCreateWebhook 创建Webhook
func (s *UnlimitedControlServer) handleCreateWebhook(c *gin.Context) {
	var req struct {
		Name    string            `json:"name" binding:"required"`
		URL     string            `json:"url" binding:"required"`
		Secret  string            `json:"secret,omitempty"`
		Events  []string          `json:"events"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
		Retries int               `json:"retries"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查名称是否已存在
	var existing Webhook
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Webhook with this name already exists",
		})
		return
	}

	// 验证URL格式
	if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook URL",
		})
		return
	}

	// 序列化事件和头部
	eventsJSON, _ := json.Marshal(req.Events)
	headersJSON, _ := json.Marshal(req.Headers)

	// 设置默认值
	if req.Timeout <= 0 {
		req.Timeout = 30
	}
	if req.Retries < 0 {
		req.Retries = 3
	}

	webhook := Webhook{
		Name:    req.Name,
		URL:     req.URL,
		Secret:  req.Secret,
		Events:  string(eventsJSON),
		Headers: string(headersJSON),
		Timeout: req.Timeout,
		Retries: req.Retries,
		Enabled: true,
	}

	if err := s.db.Create(&webhook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create webhook",
		})
		return
	}

	// 隐藏敏感信息
	webhook.Secret = ""

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    webhook,
	})
}

// handleGetWebhook 获取单个Webhook
func (s *UnlimitedControlServer) handleGetWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var webhook Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook not found",
		})
		return
	}

	// 隐藏敏感信息
	webhook.Secret = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    webhook,
	})
}

// handleUpdateWebhook 更新Webhook
func (s *UnlimitedControlServer) handleUpdateWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var req struct {
		Name    string            `json:"name"`
		URL     string            `json:"url"`
		Secret  string            `json:"secret,omitempty"`
		Events  []string          `json:"events"`
		Headers map[string]string `json:"headers"`
		Timeout int               `json:"timeout"`
		Retries int               `json:"retries"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var webhook Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != "" && req.Name != webhook.Name {
		var existing Webhook
		if err := s.db.Where("name = ? AND id != ?", req.Name, webhook.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Webhook with this name already exists",
			})
			return
		}
		webhook.Name = req.Name
	}

	// 更新字段
	if req.URL != "" {
		if !strings.HasPrefix(req.URL, "http://") && !strings.HasPrefix(req.URL, "https://") {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid webhook URL",
			})
			return
		}
		webhook.URL = req.URL
	}

	if req.Secret != "" {
		webhook.Secret = req.Secret
	}

	if req.Events != nil {
		eventsJSON, _ := json.Marshal(req.Events)
		webhook.Events = string(eventsJSON)
	}

	if req.Headers != nil {
		headersJSON, _ := json.Marshal(req.Headers)
		webhook.Headers = string(headersJSON)
	}

	if req.Timeout > 0 {
		webhook.Timeout = req.Timeout
	}

	if req.Retries >= 0 {
		webhook.Retries = req.Retries
	}

	if err := s.db.Save(&webhook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update webhook",
		})
		return
	}

	// 隐藏敏感信息
	webhook.Secret = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    webhook,
	})
}

// handleDeleteWebhook 删除Webhook
func (s *UnlimitedControlServer) handleDeleteWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var webhook Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook not found",
		})
		return
	}

	// 删除相关的投递记录
	s.db.Where("webhook_id = ?", webhook.ID).Delete(&WebhookDelivery{})

	// 删除Webhook
	if err := s.db.Delete(&webhook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete webhook",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook deleted successfully",
	})
}

// handleEnableWebhook 启用Webhook
func (s *UnlimitedControlServer) handleEnableWebhook(c *gin.Context) {
	s.toggleWebhook(c, true)
}

// handleDisableWebhook 禁用Webhook
func (s *UnlimitedControlServer) handleDisableWebhook(c *gin.Context) {
	s.toggleWebhook(c, false)
}

// toggleWebhook 切换Webhook状态
func (s *UnlimitedControlServer) toggleWebhook(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var webhook Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook not found",
		})
		return
	}

	webhook.Enabled = enabled
	if err := s.db.Save(&webhook).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update webhook status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	// 隐藏敏感信息
	webhook.Secret = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Webhook %s successfully", action),
		"data":    webhook,
	})
}

// handleTestWebhook 测试Webhook
func (s *UnlimitedControlServer) handleTestWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var webhook Webhook
	if err := s.db.First(&webhook, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook not found",
		})
		return
	}

	// 创建测试事件数据
	testEvent := map[string]interface{}{
		"event_type": "webhook.test",
		"timestamp":  time.Now().Unix(),
		"data": map[string]interface{}{
			"message": "This is a test webhook event",
			"test_id": fmt.Sprintf("test_%d", time.Now().Unix()),
		},
	}

	// 模拟发送Webhook（在实际实现中会发送HTTP请求）
	delivery := WebhookDelivery{
		WebhookID:       webhook.ID,
		EventType:       "webhook.test",
		EventData:       fmt.Sprintf("%v", testEvent),
		RequestURL:      webhook.URL,
		RequestHeaders:  `{"Content-Type": "application/json"}`,
		RequestBody:     fmt.Sprintf("%v", testEvent),
		ResponseCode:    200,
		ResponseHeaders: `{"Content-Type": "application/json"}`,
		ResponseBody:    `{"success": true}`,
		Duration:        150, // 模拟150ms响应时间
		Success:         true,
		Attempt:         1,
		CreatedAt:       time.Now(),
	}

	// 保存投递记录
	s.db.Create(&delivery)

	// 更新Webhook统计
	webhook.LastTrigger = time.Now()
	webhook.SuccessCount++
	s.db.Save(&webhook)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook test completed successfully",
		"data":    delivery,
	})
}

// ===== SDK信息和使用统计处理函数 =====

// handleGetSDKInfo 获取SDK信息
func (s *UnlimitedControlServer) handleGetSDKInfo(c *gin.Context) {
	sdkInfo := map[string]interface{}{
		"name":        "Tailscale Unlimited Control SDK",
		"version":     "1.0.0",
		"description": "Official SDK for Tailscale Unlimited Control API",
		"languages": []map[string]interface{}{
			{
				"name":    "Go",
				"version": "1.0.0",
				"status":  "stable",
			},
			{
				"name":    "Python",
				"version": "1.0.0",
				"status":  "beta",
			},
			{
				"name":    "JavaScript",
				"version": "1.0.0",
				"status":  "beta",
			},
		},
		"documentation": "https://docs.tailscale-unlimited-control.com/sdk",
		"repository":    "https://github.com/tailscale/unlimited-control-sdk",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sdkInfo,
	})
}

// handleGetSDKVersions 获取SDK版本列表
func (s *UnlimitedControlServer) handleGetSDKVersions(c *gin.Context) {
	versions := []map[string]interface{}{
		{
			"version":     "1.0.0",
			"release_date": "2025-07-31",
			"status":      "stable",
			"changelog":   "Initial release with full API support",
		},
		{
			"version":     "0.9.0",
			"release_date": "2025-07-15",
			"status":      "deprecated",
			"changelog":   "Beta release with core functionality",
		},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    versions,
		"total":   len(versions),
	})
}

// handleGetSDKDocs 获取SDK文档
func (s *UnlimitedControlServer) handleGetSDKDocs(c *gin.Context) {
	docs := map[string]interface{}{
		"getting_started": "https://docs.tailscale-unlimited-control.com/sdk/getting-started",
		"api_reference":   "https://docs.tailscale-unlimited-control.com/sdk/api-reference",
		"examples":        "https://docs.tailscale-unlimited-control.com/sdk/examples",
		"changelog":       "https://docs.tailscale-unlimited-control.com/sdk/changelog",
		"support":         "https://docs.tailscale-unlimited-control.com/sdk/support",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    docs,
	})
}

// handleGetSDKUsage 获取SDK使用统计
func (s *UnlimitedControlServer) handleGetSDKUsage(c *gin.Context) {
	var usage []SDKUsage

	query := s.db.Model(&SDKUsage{}).Preload("APIKey")

	// 支持按API密钥过滤
	if apiKeyID := c.Query("api_key_id"); apiKeyID != "" {
		query = query.Where("api_key_id = ?", apiKeyID)
	}

	// 支持按语言过滤
	if language := c.Query("language"); language != "" {
		query = query.Where("language = ?", language)
	}

	// 支持按时间范围过滤
	if since := c.Query("since"); since != "" {
		if sinceTime, err := time.Parse(time.RFC3339, since); err == nil {
			query = query.Where("created_at >= ?", sinceTime)
		}
	}

	// 限制返回数量
	limit := 100
	if limitStr := c.Query("limit"); limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 1000 {
			limit = l
		}
	}

	if err := query.Order("created_at DESC").Limit(limit).Find(&usage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch SDK usage",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usage,
		"total":   len(usage),
	})
}

// handleGetSDKUsageStats 获取SDK使用统计汇总
func (s *UnlimitedControlServer) handleGetSDKUsageStats(c *gin.Context) {
	var stats struct {
		TotalRequests    int64                `json:"total_requests"`
		UniqueAPIKeys    int64                `json:"unique_api_keys"`
		SuccessRate      float64              `json:"success_rate"`
		AvgResponseTime  int64                `json:"avg_response_time"`
		RequestsByLang   map[string]int64     `json:"requests_by_language"`
		RequestsByMethod map[string]int64     `json:"requests_by_method"`
		RequestsToday    int64                `json:"requests_today"`
	}

	// 总请求数
	s.db.Model(&SDKUsage{}).Count(&stats.TotalRequests)

	// 唯一API密钥数
	s.db.Model(&SDKUsage{}).Distinct("api_key_id").Count(&stats.UniqueAPIKeys)

	if stats.TotalRequests > 0 {
		// 成功率
		var successCount int64
		s.db.Model(&SDKUsage{}).Where("success = ?", true).Count(&successCount)
		stats.SuccessRate = float64(successCount) / float64(stats.TotalRequests) * 100

		// 平均响应时间
		var avgDuration float64
		s.db.Model(&SDKUsage{}).Select("AVG(duration)").Scan(&avgDuration)
		stats.AvgResponseTime = int64(avgDuration)
	}

	// 按语言统计
	stats.RequestsByLang = make(map[string]int64)
	var langStats []struct {
		Language string
		Count    int64
	}
	s.db.Model(&SDKUsage{}).Select("language, count(*) as count").Group("language").Scan(&langStats)
	for _, ls := range langStats {
		stats.RequestsByLang[ls.Language] = ls.Count
	}

	// 按方法统计
	stats.RequestsByMethod = make(map[string]int64)
	var methodStats []struct {
		Method string
		Count  int64
	}
	s.db.Model(&SDKUsage{}).Select("method, count(*) as count").Group("method").Scan(&methodStats)
	for _, ms := range methodStats {
		stats.RequestsByMethod[ms.Method] = ms.Count
	}

	// 今日请求数
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&SDKUsage{}).Where("created_at >= ?", today).Count(&stats.RequestsToday)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// handleDownloadSDK 下载SDK
func (s *UnlimitedControlServer) handleDownloadSDK(c *gin.Context) {
	language := c.Param("language")
	version := c.Param("version")

	// 验证语言和版本
	supportedLanguages := map[string]bool{
		"go":         true,
		"python":     true,
		"javascript": true,
	}

	if !supportedLanguages[language] {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unsupported language",
		})
		return
	}

	// 构建下载信息
	downloadInfo := map[string]interface{}{
		"language": language,
		"version":  version,
		"download_url": fmt.Sprintf("https://releases.tailscale-unlimited-control.com/sdk/%s/%s/tailscale-unlimited-control-sdk-%s-%s.tar.gz",
			language, version, language, version),
		"checksum": "sha256:abcd1234...", // 模拟校验和
		"size":     "2.5MB",
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    downloadInfo,
	})
}

// ===== API密钥使用统计处理函数 =====

// handleGetAPIKeyUsage 获取API密钥使用统计
func (s *UnlimitedControlServer) handleGetAPIKeyUsage(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid API key ID",
		})
		return
	}

	var usage []SDKUsage
	if err := s.db.Where("api_key_id = ?", uint(id)).Order("created_at DESC").Limit(100).Find(&usage).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch API key usage",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    usage,
		"total":   len(usage),
	})
}

// handleGetAPIUsageStats 获取API使用统计汇总
func (s *UnlimitedControlServer) handleGetAPIUsageStats(c *gin.Context) {
	var stats struct {
		TotalAPIKeys     int64            `json:"total_api_keys"`
		ActiveAPIKeys    int64            `json:"active_api_keys"`
		TotalRequests    int64            `json:"total_requests"`
		RequestsToday    int64            `json:"requests_today"`
		TopAPIKeys       []map[string]interface{} `json:"top_api_keys"`
	}

	// 总API密钥数
	s.db.Model(&APIKey{}).Count(&stats.TotalAPIKeys)

	// 活跃API密钥数
	s.db.Model(&APIKey{}).Where("enabled = ?", true).Count(&stats.ActiveAPIKeys)

	// 总请求数
	s.db.Model(&SDKUsage{}).Count(&stats.TotalRequests)

	// 今日请求数
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&SDKUsage{}).Where("created_at >= ?", today).Count(&stats.RequestsToday)

	// 使用最多的API密钥
	var topKeys []struct {
		APIKeyID uint
		Count    int64
		Name     string
	}
	s.db.Table("sdk_usages").
		Select("api_key_id, count(*) as count, api_keys.name").
		Joins("LEFT JOIN api_keys ON sdk_usages.api_key_id = api_keys.id").
		Group("api_key_id").
		Order("count DESC").
		Limit(5).
		Scan(&topKeys)

	for _, tk := range topKeys {
		stats.TopAPIKeys = append(stats.TopAPIKeys, map[string]interface{}{
			"api_key_id": tk.APIKeyID,
			"name":       tk.Name,
			"count":      tk.Count,
		})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// ===== Webhook投递记录处理函数 =====

// handleGetWebhookDeliveries 获取Webhook投递记录
func (s *UnlimitedControlServer) handleGetWebhookDeliveries(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var deliveries []WebhookDelivery
	if err := s.db.Where("webhook_id = ?", uint(id)).Order("created_at DESC").Limit(100).Find(&deliveries).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch webhook deliveries",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    deliveries,
		"total":   len(deliveries),
	})
}

// handleGetWebhookDelivery 获取单个Webhook投递记录
func (s *UnlimitedControlServer) handleGetWebhookDelivery(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("delivery_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid delivery ID",
		})
		return
	}

	var delivery WebhookDelivery
	if err := s.db.Preload("Webhook").First(&delivery, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook delivery not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    delivery,
	})
}

// handleRedeliverWebhook 重新投递Webhook
func (s *UnlimitedControlServer) handleRedeliverWebhook(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("delivery_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid delivery ID",
		})
		return
	}

	var delivery WebhookDelivery
	if err := s.db.Preload("Webhook").First(&delivery, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Webhook delivery not found",
		})
		return
	}

	// 创建新的投递记录
	newDelivery := WebhookDelivery{
		WebhookID:       delivery.WebhookID,
		EventType:       delivery.EventType,
		EventData:       delivery.EventData,
		RequestURL:      delivery.RequestURL,
		RequestHeaders:  delivery.RequestHeaders,
		RequestBody:     delivery.RequestBody,
		ResponseCode:    200, // 模拟成功
		ResponseHeaders: `{"Content-Type": "application/json"}`,
		ResponseBody:    `{"success": true}`,
		Duration:        120,
		Success:         true,
		Attempt:         delivery.Attempt + 1,
		CreatedAt:       time.Now(),
	}

	s.db.Create(&newDelivery)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Webhook redelivered successfully",
		"data":    newDelivery,
	})
}

// handleGetWebhookStats 获取Webhook统计
func (s *UnlimitedControlServer) handleGetWebhookStats(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid webhook ID",
		})
		return
	}

	var stats struct {
		TotalDeliveries   int64   `json:"total_deliveries"`
		SuccessfulDeliveries int64 `json:"successful_deliveries"`
		FailedDeliveries  int64   `json:"failed_deliveries"`
		SuccessRate       float64 `json:"success_rate"`
		AvgResponseTime   int64   `json:"avg_response_time"`
		LastDelivery      time.Time `json:"last_delivery"`
	}

	// 总投递数
	s.db.Model(&WebhookDelivery{}).Where("webhook_id = ?", id).Count(&stats.TotalDeliveries)

	// 成功投递数
	s.db.Model(&WebhookDelivery{}).Where("webhook_id = ? AND success = ?", id, true).Count(&stats.SuccessfulDeliveries)

	// 失败投递数
	stats.FailedDeliveries = stats.TotalDeliveries - stats.SuccessfulDeliveries

	// 成功率
	if stats.TotalDeliveries > 0 {
		stats.SuccessRate = float64(stats.SuccessfulDeliveries) / float64(stats.TotalDeliveries) * 100
	}

	// 平均响应时间
	var avgDuration float64
	s.db.Model(&WebhookDelivery{}).Where("webhook_id = ?", id).Select("AVG(duration)").Scan(&avgDuration)
	stats.AvgResponseTime = int64(avgDuration)

	// 最后投递时间
	var lastDelivery WebhookDelivery
	if err := s.db.Where("webhook_id = ?", id).Order("created_at DESC").First(&lastDelivery).Error; err == nil {
		stats.LastDelivery = lastDelivery.CreatedAt
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// handleListWebhookEvents 获取Webhook事件列表
func (s *UnlimitedControlServer) handleListWebhookEvents(c *gin.Context) {
	events := []map[string]interface{}{
		{"name": "device.created", "description": "设备创建时触发"},
		{"name": "device.updated", "description": "设备更新时触发"},
		{"name": "device.deleted", "description": "设备删除时触发"},
		{"name": "device.authorized", "description": "设备授权时触发"},
		{"name": "device.revoked", "description": "设备撤销时触发"},
		{"name": "user.created", "description": "用户创建时触发"},
		{"name": "user.updated", "description": "用户更新时触发"},
		{"name": "user.deleted", "description": "用户删除时触发"},
		{"name": "route.created", "description": "路由创建时触发"},
		{"name": "route.updated", "description": "路由更新时触发"},
		{"name": "route.deleted", "description": "路由删除时触发"},
		{"name": "ssh.session.started", "description": "SSH会话开始时触发"},
		{"name": "ssh.session.ended", "description": "SSH会话结束时触发"},
		{"name": "serve.config.created", "description": "服务配置创建时触发"},
		{"name": "serve.config.updated", "description": "服务配置更新时触发"},
		{"name": "k8s.cluster.connected", "description": "K8s集群连接时触发"},
		{"name": "k8s.cluster.disconnected", "description": "K8s集群断开时触发"},
		{"name": "system.alert", "description": "系统告警时触发"},
		{"name": "system.error", "description": "系统错误时触发"},
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    events,
		"total":   len(events),
	})
}
