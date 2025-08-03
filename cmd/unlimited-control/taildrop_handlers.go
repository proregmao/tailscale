package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// TaildropManager Taildrop管理器
type TaildropManager struct {
	server    *UnlimitedControlServer
	uploadDir string
}

// NewTaildropManager 创建Taildrop管理器
func NewTaildropManager(server *UnlimitedControlServer) *TaildropManager {
	uploadDir := "./uploads"
	os.MkdirAll(uploadDir, 0755)
	
	return &TaildropManager{
		server:    server,
		uploadDir: uploadDir,
	}
}

// GetFileTargets 获取可用的文件传输目标
func (tm *TaildropManager) GetFileTargets(excludeDeviceID uint) ([]map[string]interface{}, error) {
	var devices []Device
	query := tm.server.db.Where("online = ? AND authorized = ?", true, true)
	
	if excludeDeviceID > 0 {
		query = query.Where("id != ?", excludeDeviceID)
	}
	
	if err := query.Preload("User").Find(&devices).Error; err != nil {
		return nil, err
	}

	targets := make([]map[string]interface{}, 0)
	for _, device := range devices {
		var ips []string
		json.Unmarshal([]byte(device.IPAddresses), &ips)

		// 检查设备的Taildrop配置
		var config TaildropConfig
		taildropEnabled := true
		if err := tm.server.db.Where("device_id = ?", device.ID).First(&config).Error; err == nil {
			taildropEnabled = config.Enabled
		}

		if len(ips) > 0 && taildropEnabled {
			target := map[string]interface{}{
				"device_id":    device.ID,
				"hostname":     device.Hostname,
				"given_name":   device.GivenName,
				"ip_addresses": ips,
				"user":         device.User.Name,
				"online":       device.Online,
				"last_seen":    device.LastSeen,
				"peer_api_url": fmt.Sprintf("http://%s:41112", ips[0]),
			}
			targets = append(targets, target)
		}
	}

	return targets, nil
}

// SaveUploadedFile 保存上传的文件
func (tm *TaildropManager) SaveUploadedFile(file *multipart.FileHeader) (string, error) {
	// 生成唯一文件名
	timestamp := time.Now().Unix()
	filename := fmt.Sprintf("%d_%s", timestamp, file.Filename)
	filepath := filepath.Join(tm.uploadDir, filename)

	// 保存文件
	src, err := file.Open()
	if err != nil {
		return "", err
	}
	defer src.Close()

	dst, err := os.Create(filepath)
	if err != nil {
		return "", err
	}
	defer dst.Close()

	if _, err = io.Copy(dst, src); err != nil {
		return "", err
	}

	return filepath, nil
}

// CleanupExpiredTransfers 清理过期的文件传输
func (tm *TaildropManager) CleanupExpiredTransfers() {
	var expiredTransfers []FileTransfer
	tm.server.db.Where("expires_at < ? AND status != ?", time.Now(), "completed").Find(&expiredTransfers)

	for _, transfer := range expiredTransfers {
		// 删除文件
		if transfer.FilePath != "" {
			os.Remove(transfer.FilePath)
		}
		// 删除记录
		tm.server.db.Delete(&transfer)
	}
}

// ===== Taildrop API处理函数 =====

// handleGetTaildropTargets 获取文件传输目标
func (s *UnlimitedControlServer) handleGetTaildropTargets(c *gin.Context) {
	excludeDeviceID := uint(0)
	if deviceIDStr := c.Query("exclude_device"); deviceIDStr != "" {
		if id, err := strconv.ParseUint(deviceIDStr, 10, 32); err == nil {
			excludeDeviceID = uint(id)
		}
	}

	tm := NewTaildropManager(s)
	targets, err := tm.GetFileTargets(excludeDeviceID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to get file targets",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    targets,
		"total":   len(targets),
	})
}

// handleSendFile 发送文件
func (s *UnlimitedControlServer) handleSendFile(c *gin.Context) {
	// 解析表单数据
	senderIDStr := c.PostForm("sender_id")
	receiverIDStr := c.PostForm("receiver_id")

	senderID, err := strconv.ParseUint(senderIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid sender ID",
		})
		return
	}

	receiverID, err := strconv.ParseUint(receiverIDStr, 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid receiver ID",
		})
		return
	}

	// 检查设备是否存在
	var sender, receiver Device
	if err := s.db.First(&sender, uint(senderID)).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Sender device not found",
		})
		return
	}

	if err := s.db.First(&receiver, uint(receiverID)).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Receiver device not found",
		})
		return
	}

	// 获取上传的文件
	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "No file uploaded",
		})
		return
	}

	// 检查文件大小限制
	maxFileSize := int64(100 * 1024 * 1024) // 默认100MB
	var receiverConfig TaildropConfig
	if err := s.db.Where("device_id = ?", receiverID).First(&receiverConfig).Error; err == nil {
		if receiverConfig.MaxFileSize > 0 {
			maxFileSize = receiverConfig.MaxFileSize
		}
	}

	if file.Size > maxFileSize {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": fmt.Sprintf("File too large. Maximum size: %d bytes", maxFileSize),
		})
		return
	}

	// 保存文件
	tm := NewTaildropManager(s)
	filePath, err := tm.SaveUploadedFile(file)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save file",
		})
		return
	}

	// 创建文件传输记录
	transfer := FileTransfer{
		SenderID:   uint(senderID),
		ReceiverID: uint(receiverID),
		FileName:   file.Filename,
		FileSize:   file.Size,
		FilePath:   filePath,
		MimeType:   file.Header.Get("Content-Type"),
		Status:     "pending",
		Progress:   0,
		ExpiresAt:  time.Now().Add(24 * time.Hour), // 24小时过期
	}

	if err := s.db.Create(&transfer).Error; err != nil {
		// 删除已保存的文件
		os.Remove(filePath)
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create transfer record",
		})
		return
	}

	// 生成传输URL
	transfer.TransferURL = fmt.Sprintf("/api/v1/taildrop/download/%d", transfer.ID)
	s.db.Save(&transfer)

	// 检查是否自动接受
	if receiverConfig.AutoAccept {
		transfer.Status = "completed"
		transfer.Progress = 100
		s.db.Save(&transfer)
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    transfer,
	})
}

// handleListFileTransfers 获取文件传输列表
func (s *UnlimitedControlServer) handleListFileTransfers(c *gin.Context) {
	var transfers []FileTransfer
	
	query := s.db.Model(&FileTransfer{}).Preload("Sender").Preload("Receiver")
	
	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("sender_id = ? OR receiver_id = ?", deviceID, deviceID)
	}
	
	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	if err := query.Order("created_at DESC").Find(&transfers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch file transfers",
		})
		return
	}

	// 清理过期传输
	tm := NewTaildropManager(s)
	go tm.CleanupExpiredTransfers()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    transfers,
		"total":   len(transfers),
	})
}

// handleGetFileTransfer 获取单个文件传输
func (s *UnlimitedControlServer) handleGetFileTransfer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transfer ID",
		})
		return
	}

	var transfer FileTransfer
	if err := s.db.Preload("Sender").Preload("Receiver").First(&transfer, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File transfer not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    transfer,
	})
}

// handleAcceptFileTransfer 接受文件传输
func (s *UnlimitedControlServer) handleAcceptFileTransfer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transfer ID",
		})
		return
	}

	var transfer FileTransfer
	if err := s.db.First(&transfer, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File transfer not found",
		})
		return
	}

	if transfer.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Transfer is not in pending status",
		})
		return
	}

	// 更新状态
	transfer.Status = "completed"
	transfer.Progress = 100
	if err := s.db.Save(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update transfer status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File transfer accepted",
		"data":    transfer,
	})
}

// handleRejectFileTransfer 拒绝文件传输
func (s *UnlimitedControlServer) handleRejectFileTransfer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transfer ID",
		})
		return
	}

	var transfer FileTransfer
	if err := s.db.First(&transfer, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File transfer not found",
		})
		return
	}

	if transfer.Status != "pending" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Transfer is not in pending status",
		})
		return
	}

	// 删除文件
	if transfer.FilePath != "" {
		os.Remove(transfer.FilePath)
	}

	// 更新状态
	transfer.Status = "failed"
	if err := s.db.Save(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update transfer status",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File transfer rejected",
		"data":    transfer,
	})
}

// handleDeleteFileTransfer 删除文件传输记录
func (s *UnlimitedControlServer) handleDeleteFileTransfer(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transfer ID",
		})
		return
	}

	var transfer FileTransfer
	if err := s.db.First(&transfer, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File transfer not found",
		})
		return
	}

	// 删除文件
	if transfer.FilePath != "" {
		os.Remove(transfer.FilePath)
	}

	// 删除记录
	if err := s.db.Delete(&transfer).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete transfer record",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "File transfer deleted successfully",
	})
}

// handleDownloadFile 下载文件
func (s *UnlimitedControlServer) handleDownloadFile(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid transfer ID",
		})
		return
	}

	var transfer FileTransfer
	if err := s.db.First(&transfer, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File transfer not found",
		})
		return
	}

	// 检查文件是否存在
	if _, err := os.Stat(transfer.FilePath); os.IsNotExist(err) {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "File not found",
		})
		return
	}

	// 检查是否已过期
	if time.Now().After(transfer.ExpiresAt) {
		c.JSON(http.StatusGone, gin.H{
			"success": false,
			"message": "File transfer has expired",
		})
		return
	}

	// 设置响应头
	c.Header("Content-Description", "File Transfer")
	c.Header("Content-Transfer-Encoding", "binary")
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", transfer.FileName))
	c.Header("Content-Type", "application/octet-stream")

	// 发送文件
	c.File(transfer.FilePath)
}

// ===== Taildrop配置管理 =====

// handleGetTaildropConfig 获取Taildrop配置
func (s *UnlimitedControlServer) handleGetTaildropConfig(c *gin.Context) {
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

	var config TaildropConfig
	if err := s.db.Where("device_id = ?", uint(deviceID)).First(&config).Error; err != nil {
		// 创建默认配置
		allowedTypes := []string{"*/*"} // 允许所有类型
		allowedTypesJSON, _ := json.Marshal(allowedTypes)

		config = TaildropConfig{
			DeviceID:         uint(deviceID),
			Enabled:          true,
			AutoAccept:       false,
			SavePath:         "./downloads",
			MaxFileSize:      100 * 1024 * 1024, // 100MB
			AllowedMimeTypes: string(allowedTypesJSON),
		}
		s.db.Create(&config)
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleUpdateTaildropConfig 更新Taildrop配置
func (s *UnlimitedControlServer) handleUpdateTaildropConfig(c *gin.Context) {
	deviceID, err := strconv.ParseUint(c.Param("device_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var req TaildropConfig
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

	var config TaildropConfig
	if err := s.db.Where("device_id = ?", uint(deviceID)).First(&config).Error; err != nil {
		// 创建新配置
		config = TaildropConfig{
			DeviceID: uint(deviceID),
		}
	}

	// 更新配置
	config.Enabled = req.Enabled
	config.AutoAccept = req.AutoAccept
	config.SavePath = req.SavePath
	config.MaxFileSize = req.MaxFileSize
	config.AllowedMimeTypes = req.AllowedMimeTypes

	// 验证保存路径
	if config.SavePath != "" {
		if err := os.MkdirAll(config.SavePath, 0755); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Invalid save path",
			})
			return
		}
	}

	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}
