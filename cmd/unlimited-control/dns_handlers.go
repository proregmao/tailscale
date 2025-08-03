package main

import (
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
)

// MagicDNSResolver MagicDNS解析器
type MagicDNSResolver struct {
	server *UnlimitedControlServer
	config *DNSConfig
}

// NewMagicDNSResolver 创建MagicDNS解析器
func NewMagicDNSResolver(server *UnlimitedControlServer) *MagicDNSResolver {
	return &MagicDNSResolver{
		server: server,
	}
}

// LoadConfig 加载DNS配置
func (r *MagicDNSResolver) LoadConfig() error {
	var config DNSConfig
	if err := r.server.db.First(&config).Error; err != nil {
		return err
	}
	r.config = &config
	return nil
}

// ResolveName 解析设备名称到IP地址
func (r *MagicDNSResolver) ResolveName(name string) (string, error) {
	if r.config == nil {
		if err := r.LoadConfig(); err != nil {
			return "", err
		}
	}

	// 如果MagicDNS未启用，返回错误
	if !r.config.MagicDNSEnabled {
		return "", fmt.Errorf("MagicDNS is disabled")
	}

	// 规范化名称
	name = strings.ToLower(strings.TrimSuffix(name, "."))
	
	// 移除MagicDNS后缀
	suffix := r.config.MagicDNSSuffix
	if strings.HasSuffix(name, "."+suffix) {
		name = strings.TrimSuffix(name, "."+suffix)
	}

	// 首先检查自定义DNS记录
	var record DNSRecord
	if err := r.server.db.Where("name = ? AND type = ? AND enabled = ?", 
		name, "A", true).First(&record).Error; err == nil {
		return record.Value, nil
	}

	// 然后查找设备
	var device Device
	if err := r.server.db.Where("hostname = ? OR given_name = ?", 
		name, name).First(&device).Error; err == nil {
		// 解析IP地址
		var ips []string
		if err := json.Unmarshal([]byte(device.IPAddresses), &ips); err == nil && len(ips) > 0 {
			return ips[0], nil
		}
	}

	return "", fmt.Errorf("name not found: %s", name)
}

// GenerateDeviceFQDN 为设备生成完整域名
func (r *MagicDNSResolver) GenerateDeviceFQDN(hostname string) string {
	if r.config == nil {
		r.LoadConfig()
	}
	
	if r.config != nil && r.config.MagicDNSEnabled {
		return fmt.Sprintf("%s.%s", hostname, r.config.MagicDNSSuffix)
	}
	
	return hostname
}

// ===== DNS配置管理处理函数 =====

// handleGetDNSConfig 获取DNS配置
func (s *UnlimitedControlServer) handleGetDNSConfig(c *gin.Context) {
	var config DNSConfig
	if err := s.db.First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "DNS configuration not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleUpdateDNSConfig 更新DNS配置
func (s *UnlimitedControlServer) handleUpdateDNSConfig(c *gin.Context) {
	var req DNSConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var config DNSConfig
	if err := s.db.First(&config).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "DNS configuration not found",
		})
		return
	}

	// 更新配置
	config.MagicDNSEnabled = req.MagicDNSEnabled
	config.MagicDNSSuffix = req.MagicDNSSuffix
	config.SearchDomains = req.SearchDomains
	config.Nameservers = req.Nameservers
	config.GlobalDNS = req.GlobalDNS
	config.RestrictedDNS = req.RestrictedDNS

	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update DNS configuration",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// ===== DNS记录管理处理函数 =====

// handleListDNSRecords 获取DNS记录列表
func (s *UnlimitedControlServer) handleListDNSRecords(c *gin.Context) {
	var records []DNSRecord
	
	query := s.db.Model(&DNSRecord{})
	
	// 支持按类型过滤
	if recordType := c.Query("type"); recordType != "" {
		query = query.Where("type = ?", recordType)
	}
	
	// 支持按名称搜索
	if name := c.Query("name"); name != "" {
		query = query.Where("name LIKE ?", "%"+name+"%")
	}
	
	// 支持按启用状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("name ASC").Find(&records).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch DNS records",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    records,
		"total":   len(records),
	})
}

// handleCreateDNSRecord 创建DNS记录
func (s *UnlimitedControlServer) handleCreateDNSRecord(c *gin.Context) {
	var req DNSRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 验证DNS记录
	if err := s.validateDNSRecord(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查是否已存在相同的记录
	var existing DNSRecord
	if err := s.db.Where("name = ? AND type = ?", req.Name, req.Type).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "DNS record already exists",
		})
		return
	}

	// 设置默认值
	if req.TTL == 0 {
		req.TTL = 300 // 默认5分钟
	}
	req.Enabled = true

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create DNS record",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// validateDNSRecord 验证DNS记录
func (s *UnlimitedControlServer) validateDNSRecord(record *DNSRecord) error {
	if record.Name == "" {
		return fmt.Errorf("name is required")
	}
	
	if record.Type == "" {
		return fmt.Errorf("type is required")
	}
	
	if record.Value == "" {
		return fmt.Errorf("value is required")
	}

	// 验证记录类型
	validTypes := []string{"A", "AAAA", "CNAME", "MX", "TXT", "PTR", "SRV"}
	isValidType := false
	for _, t := range validTypes {
		if record.Type == t {
			isValidType = true
			break
		}
	}
	if !isValidType {
		return fmt.Errorf("invalid record type: %s", record.Type)
	}

	// 验证A记录的IP地址
	if record.Type == "A" {
		if net.ParseIP(record.Value) == nil {
			return fmt.Errorf("invalid IPv4 address: %s", record.Value)
		}
	}

	// 验证AAAA记录的IPv6地址
	if record.Type == "AAAA" {
		ip := net.ParseIP(record.Value)
		if ip == nil || ip.To16() == nil {
			return fmt.Errorf("invalid IPv6 address: %s", record.Value)
		}
	}

	return nil
}

// handleGetDNSRecord 获取单个DNS记录
func (s *UnlimitedControlServer) handleGetDNSRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid record ID",
		})
		return
	}

	var record DNSRecord
	if err := s.db.First(&record, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "DNS record not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    record,
	})
}

// handleUpdateDNSRecord 更新DNS记录
func (s *UnlimitedControlServer) handleUpdateDNSRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid record ID",
		})
		return
	}

	var req DNSRecord
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var record DNSRecord
	if err := s.db.First(&record, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "DNS record not found",
		})
		return
	}

	// 验证DNS记录
	if err := s.validateDNSRecord(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 更新记录
	record.Name = req.Name
	record.Type = req.Type
	record.Value = req.Value
	record.TTL = req.TTL
	record.Enabled = req.Enabled

	if err := s.db.Save(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update DNS record",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    record,
	})
}

// handleDeleteDNSRecord 删除DNS记录
func (s *UnlimitedControlServer) handleDeleteDNSRecord(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid record ID",
		})
		return
	}

	var record DNSRecord
	if err := s.db.First(&record, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "DNS record not found",
		})
		return
	}

	if err := s.db.Delete(&record).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete DNS record",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "DNS record deleted successfully",
	})
}

// ===== DNS解析和状态处理函数 =====

// handleDNSResolve DNS解析测试
func (s *UnlimitedControlServer) handleDNSResolve(c *gin.Context) {
	var req struct {
		Name string `json:"name" binding:"required"`
		Type string `json:"type"`
	}

	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	if req.Type == "" {
		req.Type = "A"
	}

	resolver := NewMagicDNSResolver(s)

	result := map[string]interface{}{
		"name":      req.Name,
		"type":      req.Type,
		"timestamp": time.Now(),
	}

	// 尝试MagicDNS解析
	if ip, err := resolver.ResolveName(req.Name); err == nil {
		result["resolved"] = true
		result["ip"] = ip
		result["source"] = "MagicDNS"
	} else {
		// 尝试系统DNS解析
		if ips, err := net.LookupIP(req.Name); err == nil && len(ips) > 0 {
			result["resolved"] = true
			result["ip"] = ips[0].String()
			result["source"] = "System DNS"
		} else {
			result["resolved"] = false
			result["error"] = "Name not found"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    result,
	})
}

// handleDNSStatus 获取DNS状态
func (s *UnlimitedControlServer) handleDNSStatus(c *gin.Context) {
	var config DNSConfig
	s.db.First(&config)

	var recordCount int64
	s.db.Model(&DNSRecord{}).Count(&recordCount)

	var enabledRecordCount int64
	s.db.Model(&DNSRecord{}).Where("enabled = ?", true).Count(&enabledRecordCount)

	var deviceCount int64
	s.db.Model(&Device{}).Count(&deviceCount)

	status := map[string]interface{}{
		"magic_dns_enabled":    config.MagicDNSEnabled,
		"magic_dns_suffix":     config.MagicDNSSuffix,
		"total_records":        recordCount,
		"enabled_records":      enabledRecordCount,
		"registered_devices":   deviceCount,
		"dns_server_ip":        "100.100.100.100", // Tailscale标准DNS服务器IP
		"last_updated":         time.Now(),
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}
