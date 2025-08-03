// Package tailscale provides a Go SDK for Tailscale Unlimited Control
package tailscale

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"time"
)

const (
	// DefaultBaseURL 默认API基础URL
	DefaultBaseURL = "http://localhost:8080/api/v1"
	
	// DefaultTimeout 默认超时时间
	DefaultTimeout = 30 * time.Second
	
	// SDKVersion SDK版本
	SDKVersion = "1.0.0"
	
	// UserAgent 用户代理
	UserAgent = "tailscale-unlimited-control-go-sdk/" + SDKVersion
)

// Client Tailscale Unlimited Control API客户端
type Client struct {
	BaseURL    string
	APIKey     string
	APISecret  string
	HTTPClient *http.Client
}

// NewClient 创建新的API客户端
func NewClient(baseURL, apiKey, apiSecret string) *Client {
	if baseURL == "" {
		baseURL = DefaultBaseURL
	}
	
	return &Client{
		BaseURL:   baseURL,
		APIKey:    apiKey,
		APISecret: apiSecret,
		HTTPClient: &http.Client{
			Timeout: DefaultTimeout,
		},
	}
}

// SetTimeout 设置请求超时时间
func (c *Client) SetTimeout(timeout time.Duration) {
	c.HTTPClient.Timeout = timeout
}

// APIResponse API响应结构
type APIResponse struct {
	Success bool        `json:"success"`
	Message string      `json:"message,omitempty"`
	Data    interface{} `json:"data,omitempty"`
	Total   int         `json:"total,omitempty"`
	Error   string      `json:"error,omitempty"`
}

// Device 设备结构
type Device struct {
	ID           uint      `json:"id"`
	Hostname     string    `json:"hostname"`
	IPAddresses  string    `json:"ip_addresses"`
	MachineKey   string    `json:"machine_key"`
	NodeKey      string    `json:"node_key"`
	DiscoKey     string    `json:"disco_key"`
	Authorized   bool      `json:"authorized"`
	LastSeen     time.Time `json:"last_seen"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// User 用户结构
type User struct {
	ID        uint      `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	Role      string    `json:"role"`
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Route 路由结构
type Route struct {
	ID          uint      `json:"id"`
	DeviceID    uint      `json:"device_id"`
	Prefix      string    `json:"prefix"`
	Advertised  bool      `json:"advertised"`
	Enabled     bool      `json:"enabled"`
	IsPrimary   bool      `json:"is_primary"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// makeRequest 发送HTTP请求
func (c *Client) makeRequest(method, endpoint string, body interface{}) (*APIResponse, error) {
	// 构建完整URL
	fullURL, err := url.JoinPath(c.BaseURL, endpoint)
	if err != nil {
		return nil, fmt.Errorf("invalid URL: %w", err)
	}

	// 序列化请求体
	var reqBody io.Reader
	if body != nil {
		jsonData, err := json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
		reqBody = bytes.NewBuffer(jsonData)
	}

	// 创建HTTP请求
	req, err := http.NewRequest(method, fullURL, reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	// 设置请求头
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", UserAgent)
	
	// 设置API认证
	if c.APIKey != "" && c.APISecret != "" {
		req.Header.Set("X-API-Key", c.APIKey)
		req.Header.Set("X-API-Secret", c.APISecret)
	}

	// 发送请求
	resp, err := c.HTTPClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request: %w", err)
	}
	defer resp.Body.Close()

	// 读取响应体
	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	// 解析响应
	var apiResp APIResponse
	if err := json.Unmarshal(respBody, &apiResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	// 检查HTTP状态码
	if resp.StatusCode >= 400 {
		return &apiResp, fmt.Errorf("API error (status %d): %s", resp.StatusCode, apiResp.Message)
	}

	return &apiResp, nil
}

// ===== 设备管理方法 =====

// ListDevices 获取设备列表
func (c *Client) ListDevices() ([]Device, error) {
	resp, err := c.makeRequest("GET", "/devices", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析设备数据
	var devices []Device
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &devices); err != nil {
		return nil, fmt.Errorf("failed to unmarshal devices: %w", err)
	}

	return devices, nil
}

// GetDevice 获取单个设备
func (c *Client) GetDevice(deviceID uint) (*Device, error) {
	endpoint := fmt.Sprintf("/devices/%d", deviceID)
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析设备数据
	var device Device
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &device); err != nil {
		return nil, fmt.Errorf("failed to unmarshal device: %w", err)
	}

	return &device, nil
}

// CreateDevice 创建设备
func (c *Client) CreateDevice(hostname string, ipAddresses []string, authorized bool) (*Device, error) {
	reqBody := map[string]interface{}{
		"hostname":     hostname,
		"ip_addresses": ipAddresses,
		"authorized":   authorized,
	}

	resp, err := c.makeRequest("POST", "/devices", reqBody)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析设备数据
	var device Device
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal device data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &device); err != nil {
		return nil, fmt.Errorf("failed to unmarshal device: %w", err)
	}

	return &device, nil
}

// AuthorizeDevice 授权设备
func (c *Client) AuthorizeDevice(deviceID uint) error {
	endpoint := fmt.Sprintf("/devices/%d/authorize", deviceID)
	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("API error: %s", resp.Message)
	}

	return nil
}

// RevokeDevice 撤销设备授权
func (c *Client) RevokeDevice(deviceID uint) error {
	endpoint := fmt.Sprintf("/devices/%d/revoke", deviceID)
	resp, err := c.makeRequest("POST", endpoint, nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("API error: %s", resp.Message)
	}

	return nil
}

// DeleteDevice 删除设备
func (c *Client) DeleteDevice(deviceID uint) error {
	endpoint := fmt.Sprintf("/devices/%d", deviceID)
	resp, err := c.makeRequest("DELETE", endpoint, nil)
	if err != nil {
		return err
	}

	if !resp.Success {
		return fmt.Errorf("API error: %s", resp.Message)
	}

	return nil
}

// ===== 用户管理方法 =====

// ListUsers 获取用户列表
func (c *Client) ListUsers() ([]User, error) {
	resp, err := c.makeRequest("GET", "/users", nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析用户数据
	var users []User
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &users); err != nil {
		return nil, fmt.Errorf("failed to unmarshal users: %w", err)
	}

	return users, nil
}

// GetUser 获取单个用户
func (c *Client) GetUser(userID uint) (*User, error) {
	endpoint := fmt.Sprintf("/users/%d", userID)
	resp, err := c.makeRequest("GET", endpoint, nil)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析用户数据
	var user User
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}

// CreateUser 创建用户
func (c *Client) CreateUser(username, email, password, role string) (*User, error) {
	reqBody := map[string]interface{}{
		"username": username,
		"email":    email,
		"password": password,
		"role":     role,
	}

	resp, err := c.makeRequest("POST", "/users", reqBody)
	if err != nil {
		return nil, err
	}

	if !resp.Success {
		return nil, fmt.Errorf("API error: %s", resp.Message)
	}

	// 解析用户数据
	var user User
	dataBytes, err := json.Marshal(resp.Data)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal user data: %w", err)
	}

	if err := json.Unmarshal(dataBytes, &user); err != nil {
		return nil, fmt.Errorf("failed to unmarshal user: %w", err)
	}

	return &user, nil
}
