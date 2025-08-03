package main

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/pquerna/otp"
	"github.com/pquerna/otp/totp"
	"golang.org/x/crypto/bcrypt"
)

// AuthManager 认证管理器
type AuthManager struct {
	server *UnlimitedControlServer
}

// NewAuthManager 创建认证管理器
func NewAuthManager(server *UnlimitedControlServer) *AuthManager {
	return &AuthManager{
		server: server,
	}
}

// RegisterRequest 注册请求结构
type RegisterRequest struct {
	Username        string `json:"username" binding:"required,min=3,max=50"`
	Email           string `json:"email" binding:"required,email"`
	Phone           string `json:"phone" binding:"required"`
	Password        string `json:"password" binding:"required,min=6"`
	ConfirmPassword string `json:"confirm_password" binding:"required"`
}

// LoginRequest 登录请求结构
type LoginRequest struct {
	Identifier string `json:"identifier" binding:"required"` // 用户名、邮箱或手机号
	Password   string `json:"password" binding:"required"`
	RememberMe bool   `json:"remember_me"`
}

// LoginResponse 登录响应结构
type LoginResponse struct {
	Success      bool   `json:"success"`
	Message      string `json:"message"`
	Token        string `json:"token,omitempty"`
	User         User   `json:"user,omitempty"`
	RequiresMFA  bool   `json:"requires_mfa,omitempty"`
	SessionID    string `json:"session_id,omitempty"`
}

// GenerateSessionToken 生成会话令牌
func (am *AuthManager) GenerateSessionToken() (string, error) {
	bytes := make([]byte, 32)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes), nil
}

// GenerateTOTPSecret 生成TOTP密钥
func (am *AuthManager) GenerateTOTPSecret() (string, error) {
	bytes := make([]byte, 20)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base32.StdEncoding.EncodeToString(bytes), nil
}

// GenerateBackupCodes 生成备用代码
func (am *AuthManager) GenerateBackupCodes() ([]string, error) {
	codes := make([]string, 10)
	for i := 0; i < 10; i++ {
		bytes := make([]byte, 4)
		if _, err := rand.Read(bytes); err != nil {
			return nil, err
		}
		codes[i] = fmt.Sprintf("%08x", sha256.Sum256(bytes))[:8]
	}
	return codes, nil
}

// ValidateTOTP 验证TOTP代码
func (am *AuthManager) ValidateTOTP(secret, code string) bool {
	return totp.Validate(code, secret)
}

// HashPassword 哈希密码
func (am *AuthManager) HashPassword(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	return string(bytes), err
}

// CheckPassword 验证密码
func (am *AuthManager) CheckPassword(hashedPassword, password string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hashedPassword), []byte(password))
	return err == nil
}

// ValidateEmail 验证邮箱格式
func (am *AuthManager) ValidateEmail(email string) bool {
	emailRegex := regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)
	return emailRegex.MatchString(email)
}

// ValidatePhone 验证手机号格式
func (am *AuthManager) ValidatePhone(phone string) bool {
	phoneRegex := regexp.MustCompile(`^1[3-9]\d{9}$`)
	return phoneRegex.MatchString(phone)
}

// ===== 用户注册和登录处理函数 =====

// handleRegister 处理用户注册
func (s *UnlimitedControlServer) handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求数据格式错误",
			"error":   err.Error(),
		})
		return
	}

	am := NewAuthManager(s)

	// 验证密码确认
	if req.Password != req.ConfirmPassword {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "两次输入的密码不一致",
		})
		return
	}

	// 验证邮箱格式
	if !am.ValidateEmail(req.Email) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "邮箱格式不正确",
		})
		return
	}

	// 验证手机号格式
	if !am.ValidatePhone(req.Phone) {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "手机号格式不正确",
		})
		return
	}

	// 检查用户名是否已存在
	var existingUser User
	if err := s.db.Where("name = ?", req.Username).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "用户名已存在",
		})
		return
	}

	// 检查邮箱是否已存在
	if err := s.db.Where("email = ?", req.Email).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "邮箱已被注册",
		})
		return
	}

	// 检查手机号是否已存在
	if err := s.db.Where("phone = ?", req.Phone).First(&existingUser).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "手机号已被注册",
		})
		return
	}

	// 哈希密码
	hashedPassword, err := am.HashPassword(req.Password)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "密码加密失败",
		})
		return
	}

	// 创建新用户
	user := User{
		Name:        req.Username,
		Email:       req.Email,
		Phone:       req.Phone,
		Password:    hashedPassword,
		Provider:    "local",
		DisplayName: req.Username,
		Role:        "user",
		Active:      true,
		CreatedAt:   time.Now(),
		UpdatedAt:   time.Now(),
	}

	if err := s.db.Create(&user).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "用户创建失败",
			"error":   err.Error(),
		})
		return
	}

	// 返回成功响应（不包含密码）
	user.Password = ""
	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"message": "注册成功",
		"data":    user,
	})
}

// handleLogin 处理用户登录
func (s *UnlimitedControlServer) handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "请求数据格式错误",
		})
		return
	}

	am := NewAuthManager(s)

	// 查找用户（支持用户名、邮箱、手机号登录）
	var user User
	query := s.db.Where("name = ? OR email = ? OR phone = ?", req.Identifier, req.Identifier, req.Identifier)
	if err := query.First(&user).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "用户名、邮箱或手机号不存在",
		})
		return
	}

	// 检查用户是否激活
	if !user.Active {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "账户已被禁用，请联系管理员",
		})
		return
	}

	// 验证密码
	if !am.CheckPassword(user.Password, req.Password) {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "密码错误",
		})
		return
	}

	// 生成会话令牌
	sessionToken, err := am.GenerateSessionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "会话创建失败",
		})
		return
	}

	// 创建认证会话
	session := AuthSession{
		UserID:       user.ID,
		SessionToken: sessionToken,
		Provider:     "local",
		IPAddress:    c.ClientIP(),
		UserAgent:    c.GetHeader("User-Agent"),
		MFARequired:  false, // 暂时不要求MFA
		MFACompleted: true,
		ExpiresAt:    time.Now().Add(time.Hour * 24 * 7), // 7天过期
		CreatedAt:    time.Now(),
		UpdatedAt:    time.Now(),
	}

	if err := s.db.Create(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "会话保存失败",
		})
		return
	}

	// 返回登录成功响应
	user.Password = "" // 不返回密码
	c.JSON(http.StatusOK, LoginResponse{
		Success:   true,
		Message:   "登录成功",
		Token:     sessionToken,
		User:      user,
		SessionID: fmt.Sprintf("%d", session.ID),
	})
}

// handleLogout 处理用户登出
func (s *UnlimitedControlServer) handleLogout(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.Query("token")
	}

	if token != "" {
		// 删除会话
		s.db.Where("session_token = ?", token).Delete(&AuthSession{})
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "登出成功",
	})
}

// handleRefreshToken 刷新令牌
func (s *UnlimitedControlServer) handleRefreshToken(c *gin.Context) {
	token := c.GetHeader("Authorization")
	if token == "" {
		token = c.Query("token")
	}

	if token == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "缺少认证令牌",
		})
		return
	}

	// 查找现有会话
	var session AuthSession
	if err := s.db.Where("session_token = ?", token).First(&session).Error; err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "无效的认证令牌",
		})
		return
	}

	// 检查会话是否过期
	if time.Now().After(session.ExpiresAt) {
		s.db.Delete(&session)
		c.JSON(http.StatusUnauthorized, gin.H{
			"success": false,
			"message": "会话已过期",
		})
		return
	}

	am := NewAuthManager(s)

	// 生成新的会话令牌
	newToken, err := am.GenerateSessionToken()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "令牌生成失败",
		})
		return
	}

	// 更新会话
	session.SessionToken = newToken
	session.ExpiresAt = time.Now().Add(time.Hour * 24 * 7) // 延长7天
	session.UpdatedAt = time.Now()

	if err := s.db.Save(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "会话更新失败",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "令牌刷新成功",
		"token":   newToken,
	})
}

// ===== OAuth提供商管理处理函数 =====

// handleListOAuthProviders 获取OAuth提供商列表
func (s *UnlimitedControlServer) handleListOAuthProviders(c *gin.Context) {
	var providers []OAuthProvider
	
	query := s.db.Model(&OAuthProvider{})
	
	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&providers).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch OAuth providers",
		})
		return
	}

	// 隐藏客户端密钥
	for i := range providers {
		if providers[i].ClientSecret != "" {
			providers[i].ClientSecret = "***"
		}
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    providers,
		"total":   len(providers),
	})
}

// handleCreateOAuthProvider 创建OAuth提供商
func (s *UnlimitedControlServer) handleCreateOAuthProvider(c *gin.Context) {
	var req OAuthProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查名称是否已存在
	var existing OAuthProvider
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "OAuth provider with this name already exists",
		})
		return
	}

	// 验证必填字段
	if req.Name == "" || req.ClientID == "" || req.ClientSecret == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Name, client ID and client secret are required",
		})
		return
	}

	// 设置默认值
	if req.DisplayName == "" {
		req.DisplayName = strings.Title(req.Name)
	}

	// 预设常见提供商的配置
	switch req.Name {
	case "google":
		if req.AuthURL == "" {
			req.AuthURL = "https://accounts.google.com/o/oauth2/auth"
		}
		if req.TokenURL == "" {
			req.TokenURL = "https://oauth2.googleapis.com/token"
		}
		if req.UserInfoURL == "" {
			req.UserInfoURL = "https://www.googleapis.com/oauth2/v2/userinfo"
		}
		if req.Scopes == "" {
			req.Scopes = `["openid", "email", "profile"]`
		}
	case "microsoft":
		if req.AuthURL == "" {
			req.AuthURL = "https://login.microsoftonline.com/common/oauth2/v2.0/authorize"
		}
		if req.TokenURL == "" {
			req.TokenURL = "https://login.microsoftonline.com/common/oauth2/v2.0/token"
		}
		if req.UserInfoURL == "" {
			req.UserInfoURL = "https://graph.microsoft.com/v1.0/me"
		}
		if req.Scopes == "" {
			req.Scopes = `["openid", "email", "profile"]`
		}
	case "github":
		if req.AuthURL == "" {
			req.AuthURL = "https://github.com/login/oauth/authorize"
		}
		if req.TokenURL == "" {
			req.TokenURL = "https://github.com/login/oauth/access_token"
		}
		if req.UserInfoURL == "" {
			req.UserInfoURL = "https://api.github.com/user"
		}
		if req.Scopes == "" {
			req.Scopes = `["user:email"]`
		}
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create OAuth provider",
		})
		return
	}

	// 隐藏客户端密钥
	req.ClientSecret = "***"

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetOAuthProvider 获取单个OAuth提供商
func (s *UnlimitedControlServer) handleGetOAuthProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid provider ID",
		})
		return
	}

	var provider OAuthProvider
	if err := s.db.First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found",
		})
		return
	}

	// 隐藏客户端密钥
	provider.ClientSecret = "***"

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    provider,
	})
}

// handleUpdateOAuthProvider 更新OAuth提供商
func (s *UnlimitedControlServer) handleUpdateOAuthProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid provider ID",
		})
		return
	}

	var req OAuthProvider
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var provider OAuthProvider
	if err := s.db.First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != provider.Name {
		var existing OAuthProvider
		if err := s.db.Where("name = ? AND id != ?", req.Name, provider.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "OAuth provider with this name already exists",
			})
			return
		}
	}

	// 更新字段
	provider.Name = req.Name
	provider.DisplayName = req.DisplayName
	provider.ClientID = req.ClientID
	if req.ClientSecret != "***" && req.ClientSecret != "" {
		provider.ClientSecret = req.ClientSecret
	}
	provider.AuthURL = req.AuthURL
	provider.TokenURL = req.TokenURL
	provider.UserInfoURL = req.UserInfoURL
	provider.Scopes = req.Scopes
	provider.Enabled = req.Enabled

	if err := s.db.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update OAuth provider",
		})
		return
	}

	// 隐藏客户端密钥
	provider.ClientSecret = "***"

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    provider,
	})
}

// handleDeleteOAuthProvider 删除OAuth提供商
func (s *UnlimitedControlServer) handleDeleteOAuthProvider(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid provider ID",
		})
		return
	}

	var provider OAuthProvider
	if err := s.db.First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found",
		})
		return
	}

	if err := s.db.Delete(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete OAuth provider",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OAuth provider deleted successfully",
	})
}

// handleEnableOAuthProvider 启用OAuth提供商
func (s *UnlimitedControlServer) handleEnableOAuthProvider(c *gin.Context) {
	s.toggleOAuthProvider(c, true)
}

// handleDisableOAuthProvider 禁用OAuth提供商
func (s *UnlimitedControlServer) handleDisableOAuthProvider(c *gin.Context) {
	s.toggleOAuthProvider(c, false)
}

// toggleOAuthProvider 切换OAuth提供商状态
func (s *UnlimitedControlServer) toggleOAuthProvider(c *gin.Context, enabled bool) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid provider ID",
		})
		return
	}

	var provider OAuthProvider
	if err := s.db.First(&provider, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found",
		})
		return
	}

	provider.Enabled = enabled
	if err := s.db.Save(&provider).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update provider status",
		})
		return
	}

	action := "disabled"
	if enabled {
		action = "enabled"
	}

	// 隐藏客户端密钥
	provider.ClientSecret = "***"

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("OAuth provider %s successfully", action),
		"data":    provider,
	})
}

// ===== OAuth认证流程处理函数 =====

// handleOAuthLogin 处理OAuth登录
func (s *UnlimitedControlServer) handleOAuthLogin(c *gin.Context) {
	providerName := c.Param("provider")

	var provider OAuthProvider
	if err := s.db.Where("name = ? AND enabled = ?", providerName, true).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found or disabled",
		})
		return
	}

	// 生成state参数用于防止CSRF攻击
	state, _ := s.generateRandomString(32)

	// 构建授权URL
	authURL, _ := url.Parse(provider.AuthURL)
	params := url.Values{}
	params.Add("client_id", provider.ClientID)
	params.Add("redirect_uri", fmt.Sprintf("%s/api/v1/auth/oauth/%s/callback", s.getBaseURL(), providerName))
	params.Add("response_type", "code")
	params.Add("state", state)

	var scopes []string
	json.Unmarshal([]byte(provider.Scopes), &scopes)
	params.Add("scope", strings.Join(scopes, " "))

	authURL.RawQuery = params.Encode()

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"auth_url": authURL.String(),
			"state":    state,
		},
	})
}

// handleOAuthCallback 处理OAuth回调
func (s *UnlimitedControlServer) handleOAuthCallback(c *gin.Context) {
	providerName := c.Param("provider")
	code := c.Query("code")
	state := c.Query("state")

	if code == "" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Authorization code is required",
		})
		return
	}

	var provider OAuthProvider
	if err := s.db.Where("name = ? AND enabled = ?", providerName, true).First(&provider).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "OAuth provider not found or disabled",
		})
		return
	}

	// 这里应该实现完整的OAuth流程
	// 1. 使用code换取access_token
	// 2. 使用access_token获取用户信息
	// 3. 创建或更新用户记录
	// 4. 创建认证会话

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "OAuth callback received",
		"data": map[string]interface{}{
			"provider": providerName,
			"code":     code,
			"state":    state,
		},
	})
}

// ===== MFA设备管理处理函数 =====

// handleListMFADevices 获取MFA设备列表
func (s *UnlimitedControlServer) handleListMFADevices(c *gin.Context) {
	var devices []MFADevice

	query := s.db.Model(&MFADevice{}).Preload("User")

	// 支持按用户过滤
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// 支持按设备类型过滤
	if deviceType := c.Query("device_type"); deviceType != "" {
		query = query.Where("device_type = ?", deviceType)
	}

	// 支持按状态过滤
	if enabled := c.Query("enabled"); enabled != "" {
		if enabled == "true" {
			query = query.Where("enabled = ?", true)
		} else if enabled == "false" {
			query = query.Where("enabled = ?", false)
		}
	}

	if err := query.Order("created_at DESC").Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch MFA devices",
		})
		return
	}

	// 隐藏敏感信息
	for i := range devices {
		devices[i].Secret = ""
		devices[i].BackupCodes = ""
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    devices,
		"total":   len(devices),
	})
}

// handleCreateMFADevice 创建MFA设备
func (s *UnlimitedControlServer) handleCreateMFADevice(c *gin.Context) {
	var req MFADevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
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

	// 验证设备类型
	validTypes := []string{"totp", "sms", "email", "hardware"}
	isValidType := false
	for _, t := range validTypes {
		if req.DeviceType == t {
			isValidType = true
			break
		}
	}

	if !isValidType {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device type",
		})
		return
	}

	am := NewAuthManager(s)

	// 根据设备类型设置相应字段
	switch req.DeviceType {
	case "totp":
		if req.Secret == "" {
			secret, err := am.GenerateTOTPSecret()
			if err != nil {
				c.JSON(http.StatusInternalServerError, gin.H{
					"success": false,
					"message": "Failed to generate TOTP secret",
				})
				return
			}
			req.Secret = secret
		}
	case "sms":
		if req.Phone == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Phone number is required for SMS MFA",
			})
			return
		}
	case "email":
		if req.Email == "" {
			c.JSON(http.StatusBadRequest, gin.H{
				"success": false,
				"message": "Email address is required for email MFA",
			})
			return
		}
	}

	// 生成备用代码
	backupCodes, err := am.GenerateBackupCodes()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate backup codes",
		})
		return
	}
	backupCodesJSON, _ := json.Marshal(backupCodes)
	req.BackupCodes = string(backupCodesJSON)

	// 设置默认值
	req.Enabled = false // 需要验证后才能启用
	req.Verified = false

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create MFA device",
		})
		return
	}

	// 返回响应（包含备用代码，但隐藏密钥）
	response := req
	response.Secret = ""
	if req.DeviceType == "totp" {
		response.Secret = req.Secret // TOTP需要返回密钥用于设置
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    response,
		"backup_codes": backupCodes,
	})
}

// generateRandomString 生成随机字符串
func (s *UnlimitedControlServer) generateRandomString(length int) (string, error) {
	bytes := make([]byte, length)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.URLEncoding.EncodeToString(bytes)[:length], nil
}

// getBaseURL 获取基础URL
func (s *UnlimitedControlServer) getBaseURL() string {
	return "http://localhost:8080" // 应该从配置中获取
}

// handleGetMFADevice 获取单个MFA设备
func (s *UnlimitedControlServer) handleGetMFADevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var device MFADevice
	if err := s.db.Preload("User").First(&device, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MFA device not found",
		})
		return
	}

	// 隐藏敏感信息
	device.Secret = ""
	device.BackupCodes = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}

// handleUpdateMFADevice 更新MFA设备
func (s *UnlimitedControlServer) handleUpdateMFADevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var req MFADevice
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var device MFADevice
	if err := s.db.First(&device, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MFA device not found",
		})
		return
	}

	// 更新允许的字段
	device.DeviceName = req.DeviceName
	device.Phone = req.Phone
	device.Email = req.Email
	device.Enabled = req.Enabled

	if err := s.db.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update MFA device",
		})
		return
	}

	// 隐藏敏感信息
	device.Secret = ""
	device.BackupCodes = ""

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    device,
	})
}

// handleDeleteMFADevice 删除MFA设备
func (s *UnlimitedControlServer) handleDeleteMFADevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var device MFADevice
	if err := s.db.First(&device, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MFA device not found",
		})
		return
	}

	if err := s.db.Delete(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete MFA device",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MFA device deleted successfully",
	})
}

// handleVerifyMFADevice 验证MFA设备
func (s *UnlimitedControlServer) handleVerifyMFADevice(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var req struct {
		Code string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Verification code is required",
		})
		return
	}

	var device MFADevice
	if err := s.db.First(&device, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MFA device not found",
		})
		return
	}

	am := NewAuthManager(s)
	var isValid bool

	switch device.DeviceType {
	case "totp":
		isValid = am.ValidateTOTP(device.Secret, req.Code)
	case "sms", "email":
		// 这里应该验证发送的验证码
		// 暂时简单验证
		isValid = req.Code == "123456"
	default:
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Unsupported device type",
		})
		return
	}

	if !isValid {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid verification code",
		})
		return
	}

	// 标记为已验证并启用
	device.Verified = true
	device.Enabled = true
	device.LastUsed = time.Now()

	if err := s.db.Save(&device).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to verify MFA device",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MFA device verified successfully",
		"data":    device,
	})
}

// handleVerifyMFA 验证MFA代码
func (s *UnlimitedControlServer) handleVerifyMFA(c *gin.Context) {
	var req struct {
		UserID   uint   `json:"user_id" binding:"required"`
		Code     string `json:"code" binding:"required"`
		DeviceID uint   `json:"device_id,omitempty"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var devices []MFADevice
	query := s.db.Where("user_id = ? AND enabled = ? AND verified = ?", req.UserID, true, true)

	if req.DeviceID > 0 {
		query = query.Where("id = ?", req.DeviceID)
	}

	if err := query.Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch MFA devices",
		})
		return
	}

	if len(devices) == 0 {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "No MFA devices found",
		})
		return
	}

	am := NewAuthManager(s)
	var validDevice *MFADevice

	// 尝试验证所有设备
	for i := range devices {
		device := &devices[i]
		var isValid bool

		switch device.DeviceType {
		case "totp":
			isValid = am.ValidateTOTP(device.Secret, req.Code)
		case "sms", "email":
			// 这里应该验证发送的验证码
			isValid = req.Code == "123456"
		}

		if isValid {
			validDevice = device
			break
		}
	}

	if validDevice == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid MFA code",
		})
		return
	}

	// 更新最后使用时间
	validDevice.LastUsed = time.Now()
	s.db.Save(validDevice)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "MFA verification successful",
		"data": map[string]interface{}{
			"device_id":   validDevice.ID,
			"device_type": validDevice.DeviceType,
		},
	})
}

// handleVerifyBackupCode 验证备用代码
func (s *UnlimitedControlServer) handleVerifyBackupCode(c *gin.Context) {
	var req struct {
		UserID uint   `json:"user_id" binding:"required"`
		Code   string `json:"code" binding:"required"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var devices []MFADevice
	if err := s.db.Where("user_id = ? AND enabled = ? AND verified = ?",
		req.UserID, true, true).Find(&devices).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch MFA devices",
		})
		return
	}

	var validDevice *MFADevice
	for i := range devices {
		device := &devices[i]
		var backupCodes []string
		json.Unmarshal([]byte(device.BackupCodes), &backupCodes)

		for j, code := range backupCodes {
			if code == req.Code {
				// 移除已使用的备用代码
				backupCodes = append(backupCodes[:j], backupCodes[j+1:]...)
				newBackupCodesJSON, _ := json.Marshal(backupCodes)
				device.BackupCodes = string(newBackupCodesJSON)
				validDevice = device
				break
			}
		}

		if validDevice != nil {
			break
		}
	}

	if validDevice == nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid backup code",
		})
		return
	}

	// 更新设备信息
	validDevice.LastUsed = time.Now()
	s.db.Save(validDevice)

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Backup code verification successful",
		"data": map[string]interface{}{
			"device_id":        validDevice.ID,
			"remaining_codes":  len(strings.Split(validDevice.BackupCodes, ",")),
		},
	})
}

// handleGetMFAQRCode 获取MFA二维码
func (s *UnlimitedControlServer) handleGetMFAQRCode(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid device ID",
		})
		return
	}

	var device MFADevice
	if err := s.db.Preload("User").First(&device, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "MFA device not found",
		})
		return
	}

	if device.DeviceType != "totp" {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "QR code is only available for TOTP devices",
		})
		return
	}

	// 生成TOTP URL
	issuer := "Tailscale Unlimited Control"
	accountName := device.User.Email
	if accountName == "" {
		accountName = device.User.Name
	}

	key, err := otp.NewKeyFromURL(fmt.Sprintf("otpauth://totp/%s:%s?secret=%s&issuer=%s",
		issuer, accountName, device.Secret, issuer))
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to generate TOTP key",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data": map[string]interface{}{
			"qr_url":    key.URL(),
			"secret":    device.Secret,
			"issuer":    issuer,
			"account":   accountName,
		},
	})
}

// ===== 认证会话管理处理函数 =====

// handleListAuthSessions 获取认证会话列表
func (s *UnlimitedControlServer) handleListAuthSessions(c *gin.Context) {
	var sessions []AuthSession

	query := s.db.Model(&AuthSession{}).Preload("User")

	// 支持按用户过滤
	if userID := c.Query("user_id"); userID != "" {
		query = query.Where("user_id = ?", userID)
	}

	// 支持按提供商过滤
	if provider := c.Query("provider"); provider != "" {
		query = query.Where("provider = ?", provider)
	}

	if err := query.Order("created_at DESC").Find(&sessions).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to fetch auth sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    sessions,
		"total":   len(sessions),
	})
}

// handleDeleteAuthSession 删除认证会话
func (s *UnlimitedControlServer) handleDeleteAuthSession(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid session ID",
		})
		return
	}

	var session AuthSession
	if err := s.db.First(&session, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Auth session not found",
		})
		return
	}

	if err := s.db.Delete(&session).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete auth session",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Auth session deleted successfully",
	})
}

// handleRevokeAllSessions 撤销所有会话
func (s *UnlimitedControlServer) handleRevokeAllSessions(c *gin.Context) {
	var req struct {
		UserID uint `json:"user_id,omitempty"`
	}
	c.ShouldBindJSON(&req)

	query := s.db.Model(&AuthSession{})
	if req.UserID > 0 {
		query = query.Where("user_id = ?", req.UserID)
	}

	result := query.Delete(&AuthSession{})
	if result.Error != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to revoke sessions",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": fmt.Sprintf("Revoked %d sessions", result.RowsAffected),
	})
}

// ===== 安全策略管理处理函数 =====

// handleListSecurityPolicies 获取安全策略列表
func (s *UnlimitedControlServer) handleListSecurityPolicies(c *gin.Context) {
	var policies []SecurityPolicy

	query := s.db.Model(&SecurityPolicy{})

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
			"message": "Failed to fetch security policies",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policies,
		"total":   len(policies),
	})
}

// handleCreateSecurityPolicy 创建安全策略
func (s *UnlimitedControlServer) handleCreateSecurityPolicy(c *gin.Context) {
	var req SecurityPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	// 检查名称是否已存在
	var existing SecurityPolicy
	if err := s.db.Where("name = ?", req.Name).First(&existing).Error; err == nil {
		c.JSON(http.StatusConflict, gin.H{
			"success": false,
			"message": "Security policy with this name already exists",
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
	if req.SessionTimeout <= 0 {
		req.SessionTimeout = 480 // 8小时
	}
	if req.MaxDevicesPerUser <= 0 {
		req.MaxDevicesPerUser = 10
	}

	if err := s.db.Create(&req).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to create security policy",
		})
		return
	}

	c.JSON(http.StatusCreated, gin.H{
		"success": true,
		"data":    req,
	})
}

// handleGetSecurityPolicy 获取单个安全策略
func (s *UnlimitedControlServer) handleGetSecurityPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy SecurityPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Security policy not found",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleUpdateSecurityPolicy 更新安全策略
func (s *UnlimitedControlServer) handleUpdateSecurityPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var req SecurityPolicy
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid request data",
		})
		return
	}

	var policy SecurityPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Security policy not found",
		})
		return
	}

	// 检查名称冲突
	if req.Name != policy.Name {
		var existing SecurityPolicy
		if err := s.db.Where("name = ? AND id != ?", req.Name, policy.ID).First(&existing).Error; err == nil {
			c.JSON(http.StatusConflict, gin.H{
				"success": false,
				"message": "Security policy with this name already exists",
			})
			return
		}
	}

	// 更新字段
	policy.Name = req.Name
	policy.RequireMFA = req.RequireMFA
	policy.AllowedProviders = req.AllowedProviders
	policy.SessionTimeout = req.SessionTimeout
	policy.MaxDevicesPerUser = req.MaxDevicesPerUser
	policy.AllowedIPRanges = req.AllowedIPRanges
	policy.BlockedCountries = req.BlockedCountries
	policy.RequireDeviceAuth = req.RequireDeviceAuth
	policy.Enabled = req.Enabled

	if err := s.db.Save(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to update security policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"data":    policy,
	})
}

// handleDeleteSecurityPolicy 删除安全策略
func (s *UnlimitedControlServer) handleDeleteSecurityPolicy(c *gin.Context) {
	id, err := strconv.ParseUint(c.Param("id"), 10, 32)
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"success": false,
			"message": "Invalid policy ID",
		})
		return
	}

	var policy SecurityPolicy
	if err := s.db.First(&policy, uint(id)).Error; err != nil {
		c.JSON(http.StatusNotFound, gin.H{
			"success": false,
			"message": "Security policy not found",
		})
		return
	}

	if err := s.db.Delete(&policy).Error; err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{
			"success": false,
			"message": "Failed to delete security policy",
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": true,
		"message": "Security policy deleted successfully",
	})
}
