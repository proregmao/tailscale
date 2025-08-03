package main

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/url"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
)

// ServeManager 服务暴露管理器
type ServeManager struct {
	server *UnlimitedControlServer
}

// NewServeManager 创建服务暴露管理器
func NewServeManager(server *UnlimitedControlServer) *ServeManager {
	return &ServeManager{
		server: server,
	}
}

// GenerateSelfSignedCert 生成自签名证书
func (sm *ServeManager) GenerateSelfSignedCert(domain string) (*TLSCertificate, error) {
	// 生成私钥
	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return nil, err
	}

	// 创建证书模板
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			Organization:  []string{"Tailscale Unlimited Control"},
			Country:       []string{"US"},
			Province:      []string{""},
			Locality:      []string{""},
			StreetAddress: []string{""},
			PostalCode:    []string{""},
		},
		NotBefore:    time.Now(),
		NotAfter:     time.Now().Add(365 * 24 * time.Hour), // 1年有效期
		KeyUsage:     x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:  []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		IPAddresses:  nil,
		DNSNames:     []string{domain},
	}

	// 生成证书
	certDER, err := x509.CreateCertificate(rand.Reader, &template, &template, &privateKey.PublicKey, privateKey)
	if err != nil {
		return nil, err
	}

	// 编码证书
	certPEM := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certDER})
	keyPEM := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(privateKey)})

	cert := &TLSCertificate{
		Domain:       domain,
		CertData:     string(certPEM),
		KeyData:      string(keyPEM),
		Issuer:       "Self-Signed",
		Subject:      fmt.Sprintf("CN=%s", domain),
		SerialNumber: "1",
		NotBefore:    template.NotBefore,
		NotAfter:     template.NotAfter,
		AutoRenew:    false,
		Status:       "active",
	}

	return cert, nil
}

// ValidateServeConfig 验证服务配置
func (sm *ServeManager) ValidateServeConfig(config *ServeConfig) error {
	if config.ServiceName == "" {
		return fmt.Errorf("service name is required")
	}

	if config.ServiceType != "serve" && config.ServiceType != "funnel" {
		return fmt.Errorf("service type must be 'serve' or 'funnel'")
	}

	if config.Protocol != "http" && config.Protocol != "https" && config.Protocol != "tcp" {
		return fmt.Errorf("protocol must be 'http', 'https', or 'tcp'")
	}

	if config.LocalPort <= 0 || config.LocalPort > 65535 {
		return fmt.Errorf("local port must be between 1 and 65535")
	}

	if config.ExternalPort <= 0 || config.ExternalPort > 65535 {
		return fmt.Errorf("external port must be between 1 and 65535")
	}

	if config.TargetURL != "" {
		if _, err := url.Parse(config.TargetURL); err != nil {
			return fmt.Errorf("invalid target URL: %v", err)
		}
	}

	return nil
}

// LogAccess 记录访问日志
func (sm *ServeManager) LogAccess(serveID uint, clientIP, userAgent, method, path string, statusCode int, responseSize, duration int64) {
	log := AccessLog{
		ServeID:      serveID,
		ClientIP:     clientIP,
		UserAgent:    userAgent,
		Method:       method,
		Path:         path,
		StatusCode:   statusCode,
		ResponseSize: responseSize,
		Duration:     duration,
		CreatedAt:    time.Now(),
	}
	sm.server.db.Create(&log)
}

// ===== 服务配置管理处理函数 =====

// handleListServeConfigs 获取服务配置列表
func (s *UnlimitedControlServer) handleListServeConfigs(c *gin.Context) {
	var configs []ServeConfig
	
	query := s.db.Model(&ServeConfig{}).Preload("Device")
	
	// 支持按设备过滤
	if deviceID := c.Query("device_id"); deviceID != "" {
		query = query.Where("device_id = ?", deviceID)
	}
	
	// 支持按服务类型过滤
	if serviceType := c.Query("service_type"); serviceType != "" {
		query = query.Where("service_type = ?", serviceType)
	}
	
	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&configs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch serve configs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    configs,
		"total":   len(configs),
	})
}

// handleCreateServeConfig 创建服务配置
func (s *UnlimitedControlServer) handleCreateServeConfig(c *gin.Context) {
	var req ServeConfig
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

	// 验证配置
	sm := NewServeManager(s)
	if err := sm.ValidateServeConfig(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查端口冲突
	var existing ServeConfig
	if err := s.db.Where("device_id = ? AND external_port = ? AND enabled = ?", 
		req.DeviceID, req.ExternalPort, true).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "External port is already in use on this device",
		})
		return
	}

	// 设置默认值
	if req.Path == "" {
		req.Path = "/"
	}
	if req.RateLimitRPS <= 0 {
		req.RateLimitRPS = 100
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create serve config",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetServeConfig 获取单个服务配置
func (s *UnlimitedControlServer) handleGetServeConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config ServeConfig
	if err := s.db.Preload("Device").First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleUpdateServeConfig 更新服务配置
func (s *UnlimitedControlServer) handleUpdateServeConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var req ServeConfig
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var config ServeConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	// 验证配置
	sm := NewServeManager(s)
	if err := sm.ValidateServeConfig(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": err.Error(),
		})
		return
	}

	// 检查端口冲突（排除当前配置）
	if req.ExternalPort != config.ExternalPort {
		var existing ServeConfig
		if err := s.db.Where("device_id = ? AND external_port = ? AND enabled = ? AND id != ?", 
			req.DeviceID, req.ExternalPort, true, config.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "External port is already in use on this device",
			})
			return
		}
	}

	// 更新字段
	config.ServiceName = req.ServiceName
	config.ServiceType = req.ServiceType
	config.Protocol = req.Protocol
	config.LocalPort = req.LocalPort
	config.ExternalPort = req.ExternalPort
	config.Domain = req.Domain
	config.Path = req.Path
	config.TargetURL = req.TargetURL
	config.Enabled = req.Enabled
	config.HTTPSEnabled = req.HTTPSEnabled
	config.AuthRequired = req.AuthRequired
	config.AllowedUsers = req.AllowedUsers
	config.RateLimitRPS = req.RateLimitRPS

	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update serve config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    config,
	})
}

// handleDeleteServeConfig 删除服务配置
func (s *UnlimitedControlServer) handleDeleteServeConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config ServeConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	// 删除相关的代理规则
	s.db.Where("serve_id = ?", config.ID).Delete(&ProxyRule{})

	// 删除配置
	if err := s.db.Delete(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete serve config",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Serve config deleted successfully",
	})
}

// handleEnableServeConfig 启用服务配置
func (s *UnlimitedControlServer) handleEnableServeConfig(c *gin.Context) {
	s.toggleServeConfig(c, true)
}

// handleDisableServeConfig 禁用服务配置
func (s *UnlimitedControlServer) handleDisableServeConfig(c *gin.Context) {
	s.toggleServeConfig(c, false)
}

// toggleServeConfig 切换服务配置状态
func (s *UnlimitedControlServer) toggleServeConfig(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config ServeConfig
	if err := s.db.First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	config.Enabled = enabled
	if err := s.db.Save(&config).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update config status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Serve config %s successfully", action),
		"data":    config,
	})
}

// ===== TLS证书管理处理函数 =====

// handleListTLSCertificates 获取TLS证书列表
func (s *UnlimitedControlServer) handleListTLSCertificates(c *gin.Context) {
	var certificates []TLSCertificate

	query := s.db.Model(&TLSCertificate{})

	// 支持按状态过滤
	if status := c.Query("status"); status != "" {
		query = query.Where("status = ?", status)
	}

	// 支持按域名过滤
	if domain := c.Query("domain"); domain != "" {
		query = query.Where("domain LIKE ?", "%"+domain+"%")
	}

	if err := query.Order("created_at DESC").Find(&certificates).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch TLS certificates",
		})
		return
	}

	// 隐藏私钥数据
	for i := range certificates {
		certificates[i].KeyData = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    certificates,
		"total":   len(certificates),
	})
}

// handleCreateTLSCertificate 创建TLS证书
func (s *UnlimitedControlServer) handleCreateTLSCertificate(c *gin.Context) {
	var req struct {
		Domain    string `json:"domain" binding:"required"`
		AutoRenew bool   `json:"auto_renew"`
		CertData  string `json:"cert_data,omitempty"`
		KeyData   string `json:"key_data,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查域名是否已存在
	var existing TLSCertificate
	if err := s.db.Where("domain = ?", req.Domain).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Certificate for this domain already exists",
		})
		return
	}

	var cert *TLSCertificate
	var err error

	if req.CertData != "" && req.KeyData != "" {
		// 使用提供的证书数据
		cert = &TLSCertificate{
			Domain:    req.Domain,
			CertData:  req.CertData,
			KeyData:   req.KeyData,
			AutoRenew: req.AutoRenew,
			Status:    "active",
		}

		// 解析证书信息
		block, _ := pem.Decode([]byte(req.CertData))
		if block != nil {
			if x509Cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				cert.Issuer = x509Cert.Issuer.String()
				cert.Subject = x509Cert.Subject.String()
				cert.SerialNumber = x509Cert.SerialNumber.String()
				cert.NotBefore = x509Cert.NotBefore
				cert.NotAfter = x509Cert.NotAfter
			}
		}
	} else {
		// 生成自签名证书
		sm := NewServeManager(s)
		cert, err = sm.GenerateSelfSignedCert(req.Domain)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"success": false,
				"message": "Failed to generate certificate",
			})
			return
		}
		cert.AutoRenew = req.AutoRenew
	}

	if err := s.db.Create(cert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create TLS certificate",
		})
		return
	}

	// 隐藏私钥
	cert.KeyData = ""

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    cert,
	})
}

// handleGetTLSCertificate 获取单个TLS证书
func (s *UnlimitedControlServer) handleGetTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid certificate ID",
		})
		return
	}

	var cert TLSCertificate
	if err := s.db.First(&cert, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "TLS certificate not found",
		})
		return
	}

	// 隐藏私钥
	cert.KeyData = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cert,
	})
}

// handleUpdateTLSCertificate 更新TLS证书
func (s *UnlimitedControlServer) handleUpdateTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid certificate ID",
		})
		return
	}

	var req struct {
		AutoRenew bool   `json:"auto_renew"`
		CertData  string `json:"cert_data,omitempty"`
		KeyData   string `json:"key_data,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var cert TLSCertificate
	if err := s.db.First(&cert, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "TLS certificate not found",
		})
		return
	}

	// 更新字段
	cert.AutoRenew = req.AutoRenew

	if req.CertData != "" && req.KeyData != "" {
		cert.CertData = req.CertData
		cert.KeyData = req.KeyData

		// 重新解析证书信息
		block, _ := pem.Decode([]byte(req.CertData))
		if block != nil {
			if x509Cert, err := x509.ParseCertificate(block.Bytes); err == nil {
				cert.Issuer = x509Cert.Issuer.String()
				cert.Subject = x509Cert.Subject.String()
				cert.SerialNumber = x509Cert.SerialNumber.String()
				cert.NotBefore = x509Cert.NotBefore
				cert.NotAfter = x509Cert.NotAfter
			}
		}
	}

	if err := s.db.Save(&cert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update TLS certificate",
		})
		return
	}

	// 隐藏私钥
	cert.KeyData = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cert,
	})
}

// handleDeleteTLSCertificate 删除TLS证书
func (s *UnlimitedControlServer) handleDeleteTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid certificate ID",
		})
		return
	}

	var cert TLSCertificate
	if err := s.db.First(&cert, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "TLS certificate not found",
		})
		return
	}

	if err := s.db.Delete(&cert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete TLS certificate",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "TLS certificate deleted successfully",
	})
}

// handleRenewTLSCertificate 续期TLS证书
func (s *UnlimitedControlServer) handleRenewTLSCertificate(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid certificate ID",
		})
		return
	}

	var cert TLSCertificate
	if err := s.db.First(&cert, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "TLS certificate not found",
		})
		return
	}

	// 生成新的自签名证书
	sm := NewServeManager(s)
	newCert, err := sm.GenerateSelfSignedCert(cert.Domain)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to renew certificate",
		})
		return
	}

	// 更新证书数据
	cert.CertData = newCert.CertData
	cert.KeyData = newCert.KeyData
	cert.Issuer = newCert.Issuer
	cert.Subject = newCert.Subject
	cert.SerialNumber = newCert.SerialNumber
	cert.NotBefore = newCert.NotBefore
	cert.NotAfter = newCert.NotAfter
	cert.Status = "active"

	if err := s.db.Save(&cert).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to save renewed certificate",
		})
		return
	}

	// 隐藏私钥
	cert.KeyData = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "TLS certificate renewed successfully",
		"data":    cert,
	})
}

// handleGetCertificateByDomain 根据域名获取证书
func (s *UnlimitedControlServer) handleGetCertificateByDomain(c *gin.Context) {
	domain := c.Param("domain")
	if domain == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Domain is required",
		})
		return
	}

	var cert TLSCertificate
	if err := s.db.Where("domain = ?", domain).First(&cert).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Certificate not found for this domain",
		})
		return
	}

	// 隐藏私钥
	cert.KeyData = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    cert,
	})
}

// ===== 代理规则管理处理函数 =====

// handleListProxyRules 获取代理规则列表
func (s *UnlimitedControlServer) handleListProxyRules(c *gin.Context) {
	var rules []ProxyRule

	query := s.db.Model(&ProxyRule{}).Preload("Serve")

	// 支持按服务过滤
	if serveID := c.Query("serve_id"); serveID != "" {
		query = query.Where("serve_id = ?", serveID)
	}

	// 支持按规则类型过滤
	if ruleType := c.Query("rule_type"); ruleType != "" {
		query = query.Where("rule_type = ?", ruleType)
	}

	if err := query.Order("priority ASC, created_at DESC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch proxy rules",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"total":   len(rules),
	})
}

// handleCreateProxyRule 创建代理规则
func (s *UnlimitedControlServer) handleCreateProxyRule(c *gin.Context) {
	var req ProxyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查服务是否存在
	var serve ServeConfig
	if err := s.db.First(&serve, req.ServeID).Error; err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	// 验证规则类型
	validTypes := []string{"path", "header", "query", "method"}
	isValidType := false
	for _, t := range validTypes {
		if req.RuleType == t {
			isValidType = true
			break
		}
	}

	if !isValidType {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid rule type",
		})
		return
	}

	// 验证动作类型
	validActions := []string{"proxy", "redirect", "block"}
	isValidAction := false
	for _, a := range validActions {
		if req.Action == a {
			isValidAction = true
			break
		}
	}

	if !isValidAction {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid action type",
		})
		return
	}

	// 设置默认优先级
	if req.Priority <= 0 {
		var maxPriority int
		s.db.Model(&ProxyRule{}).Where("serve_id = ?", req.ServeID).Select("COALESCE(MAX(priority), 0)").Scan(&maxPriority)
		req.Priority = maxPriority + 10
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create proxy rule",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetProxyRule 获取单个代理规则
func (s *UnlimitedControlServer) handleGetProxyRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid rule ID",
		})
		return
	}

	var rule ProxyRule
	if err := s.db.Preload("Serve").First(&rule, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Proxy rule not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleUpdateProxyRule 更新代理规则
func (s *UnlimitedControlServer) handleUpdateProxyRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid rule ID",
		})
		return
	}

	var req ProxyRule
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var rule ProxyRule
	if err := s.db.First(&rule, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Proxy rule not found",
		})
		return
	}

	// 更新字段
	rule.RuleType = req.RuleType
	rule.Pattern = req.Pattern
	rule.Action = req.Action
	rule.Target = req.Target
	rule.Priority = req.Priority
	rule.Enabled = req.Enabled

	if err := s.db.Save(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update proxy rule",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rule,
	})
}

// handleDeleteProxyRule 删除代理规则
func (s *UnlimitedControlServer) handleDeleteProxyRule(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid rule ID",
		})
		return
	}

	var rule ProxyRule
	if err := s.db.First(&rule, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Proxy rule not found",
		})
		return
	}

	if err := s.db.Delete(&rule).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete proxy rule",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Proxy rule deleted successfully",
	})
}

// handleGetRulesByServe 获取服务的代理规则
func (s *UnlimitedControlServer) handleGetRulesByServe(c *gin.Context) {
	serveID, err := strconv.ParseUint(c.Param("serve_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid serve ID",
		})
		return
	}

	var rules []ProxyRule
	if err := s.db.Where("serve_id = ?", uint(serveID)).Order("priority ASC").Find(&rules).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch proxy rules",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    rules,
		"total":   len(rules),
	})
}

// ===== 访问日志处理函数 =====

// handleListAccessLogs 获取访问日志列表
func (s *UnlimitedControlServer) handleListAccessLogs(c *gin.Context) {
	var logs []AccessLog

	query := s.db.Model(&AccessLog{}).Preload("Serve")

	// 支持按服务过滤
	if serveID := c.Query("serve_id"); serveID != "" {
		query = query.Where("serve_id = ?", serveID)
	}

	// 支持按状态码过滤
	if statusCode := c.Query("status_code"); statusCode != "" {
		query = query.Where("status_code = ?", statusCode)
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

	if err := query.Order("created_at DESC").Limit(limit).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch access logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// handleGetAccessLogsByServe 获取服务的访问日志
func (s *UnlimitedControlServer) handleGetAccessLogsByServe(c *gin.Context) {
	serveID, err := strconv.ParseUint(c.Param("serve_id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid serve ID",
		})
		return
	}

	var logs []AccessLog
	if err := s.db.Where("serve_id = ?", uint(serveID)).Order("created_at DESC").Limit(100).Find(&logs).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch access logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    logs,
		"total":   len(logs),
	})
}

// handleGetAccessStats 获取访问统计
func (s *UnlimitedControlServer) handleGetAccessStats(c *gin.Context) {
	var stats struct {
		TotalRequests   int64 `json:"total_requests"`
		UniqueIPs       int64 `json:"unique_ips"`
		SuccessRequests int64 `json:"success_requests"`
		ErrorRequests   int64 `json:"error_requests"`
		AvgResponseTime int64 `json:"avg_response_time"`
	}

	// 总请求数
	s.db.Model(&AccessLog{}).Count(&stats.TotalRequests)

	// 唯一IP数
	s.db.Model(&AccessLog{}).Distinct("client_ip").Count(&stats.UniqueIPs)

	// 成功请求数（2xx状态码）
	s.db.Model(&AccessLog{}).Where("status_code >= ? AND status_code < ?", 200, 300).Count(&stats.SuccessRequests)

	// 错误请求数（4xx和5xx状态码）
	s.db.Model(&AccessLog{}).Where("status_code >= ?", 400).Count(&stats.ErrorRequests)

	// 平均响应时间
	var avgDuration float64
	s.db.Model(&AccessLog{}).Select("AVG(duration)").Scan(&avgDuration)
	stats.AvgResponseTime = int64(avgDuration)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    stats,
	})
}

// handleClearAccessLogs 清除访问日志
func (s *UnlimitedControlServer) handleClearAccessLogs(c *gin.Context) {
	var req struct {
		Before string `json:"before,omitempty"` // 清除此时间之前的日志
	}
	c.ShouldBindJSON(&req)

	query := s.db.Model(&AccessLog{})

	if req.Before != "" {
		if beforeTime, err := time.Parse(time.RFC3339, req.Before); err == nil {
			query = query.Where("created_at < ?", beforeTime)
		}
	}

	result := query.Delete(&AccessLog{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to clear access logs",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Cleared %d access logs", result.RowsAffected),
	})
}

// ===== 服务状态和监控处理函数 =====

// handleGetServeStatus 获取服务状态
func (s *UnlimitedControlServer) handleGetServeStatus(c *gin.Context) {
	var status struct {
		TotalConfigs    int64 `json:"total_configs"`
		ActiveConfigs   int64 `json:"active_configs"`
		TotalCerts      int64 `json:"total_certs"`
		ExpiringSoon    int64 `json:"expiring_soon"`
		TotalRequests   int64 `json:"total_requests"`
		ActiveSessions  int64 `json:"active_sessions"`
	}

	// 总配置数
	s.db.Model(&ServeConfig{}).Count(&status.TotalConfigs)

	// 活跃配置数
	s.db.Model(&ServeConfig{}).Where("enabled = ?", true).Count(&status.ActiveConfigs)

	// 总证书数
	s.db.Model(&TLSCertificate{}).Count(&status.TotalCerts)

	// 即将过期的证书数（30天内）
	expiryThreshold := time.Now().Add(30 * 24 * time.Hour)
	s.db.Model(&TLSCertificate{}).Where("not_after <= ? AND status = ?", expiryThreshold, "active").Count(&status.ExpiringSoon)

	// 总请求数（今天）
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&AccessLog{}).Where("created_at >= ?", today).Count(&status.TotalRequests)

	// 活跃会话数（假设值）
	status.ActiveSessions = status.ActiveConfigs

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    status,
	})
}

// handleTestServeConfig 测试服务配置
func (s *UnlimitedControlServer) handleTestServeConfig(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var config ServeConfig
	if err := s.db.Preload("Device").First(&config, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Serve config not found",
		})
		return
	}

	// 构建测试URL
	protocol := config.Protocol
	if protocol == "tcp" {
		protocol = "http" // TCP服务用HTTP测试
	}

	var testURL string
	if config.Device.ID != 0 {
		// 从IP地址数组中提取第一个IP
		var deviceIP string = "localhost"
		if config.Device.IPAddresses != "" {
			// 解析IP地址JSON数组，取第一个
			if config.Device.IPAddresses[0] == '[' {
				// 简单解析JSON数组格式 ["100.64.0.2"]
				start := 2 // 跳过 ["
				end := len(config.Device.IPAddresses) - 2 // 跳过 "]
				if end > start {
					deviceIP = config.Device.IPAddresses[start:end]
				}
			} else {
				deviceIP = config.Device.IPAddresses
			}
		}
		testURL = fmt.Sprintf("%s://%s:%d%s", protocol, deviceIP, config.ExternalPort, config.Path)
	} else {
		testURL = fmt.Sprintf("%s://localhost:%d%s", protocol, config.ExternalPort, config.Path)
	}

	// 执行HTTP测试
	client := &http.Client{
		Timeout: 10 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	resp, err := client.Get(testURL)
	var testResult struct {
		URL        string `json:"url"`
		Success    bool   `json:"success"`
		StatusCode int    `json:"status_code,omitempty"`
		Error      string `json:"error,omitempty"`
		Duration   int64  `json:"duration"`
	}

	start := time.Now()
	testResult.URL = testURL
	testResult.Duration = time.Since(start).Milliseconds()

	if err != nil {
		testResult.Success = false
		testResult.Error = err.Error()
	} else {
		testResult.Success = true
		testResult.StatusCode = resp.StatusCode
		resp.Body.Close()
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    testResult,
	})
}

// handleGetServeMetrics 获取服务指标
func (s *UnlimitedControlServer) handleGetServeMetrics(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid config ID",
		})
		return
	}

	var metrics struct {
		TotalRequests   int64   `json:"total_requests"`
		SuccessRate     float64 `json:"success_rate"`
		AvgResponseTime int64   `json:"avg_response_time"`
		TotalBytes      int64   `json:"total_bytes"`
		UniqueIPs       int64   `json:"unique_ips"`
		RequestsToday   int64   `json:"requests_today"`
	}

	// 总请求数
	s.db.Model(&AccessLog{}).Where("serve_id = ?", id).Count(&metrics.TotalRequests)

	if metrics.TotalRequests > 0 {
		// 成功率
		var successCount int64
		s.db.Model(&AccessLog{}).Where("serve_id = ? AND status_code >= ? AND status_code < ?",
			id, 200, 300).Count(&successCount)
		metrics.SuccessRate = float64(successCount) / float64(metrics.TotalRequests) * 100

		// 平均响应时间
		var avgDuration float64
		s.db.Model(&AccessLog{}).Where("serve_id = ?", id).Select("AVG(duration)").Scan(&avgDuration)
		metrics.AvgResponseTime = int64(avgDuration)

		// 总字节数
		s.db.Model(&AccessLog{}).Where("serve_id = ?", id).Select("SUM(response_size)").Scan(&metrics.TotalBytes)

		// 唯一IP数
		s.db.Model(&AccessLog{}).Where("serve_id = ?", id).Distinct("client_ip").Count(&metrics.UniqueIPs)
	}

	// 今日请求数
	today := time.Now().Truncate(24 * time.Hour)
	s.db.Model(&AccessLog{}).Where("serve_id = ? AND created_at >= ?", id, today).Count(&metrics.RequestsToday)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    metrics,
	})
}
