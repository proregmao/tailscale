package main

import (
	"context"
	"crypto/tls"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"gorm.io/driver/sqlite"
	"gorm.io/gorm"
)

// UnlimitedControlServer 无限制控制服务器
type UnlimitedControlServer struct {
	db             *gorm.DB
	router         *gin.Engine
	server         *http.Server
	alertEngine    *AlertEngine
	localAPIServer *LocalAPIServer

	// 配置
	listenAddr string
	dbPath     string

	// 统计信息
	stats ServerStats
}

// ServerStats 服务器统计信息
type ServerStats struct {
	TotalDevices    int64 `json:"total_devices"`
	OnlineDevices   int64 `json:"online_devices"`
	TotalUsers      int64 `json:"total_users"`
	ActiveSessions  int64 `json:"active_sessions"`
	NetworkMaps     int64 `json:"network_maps"`
	DERPConnections int64 `json:"derp_connections"`
}

// Device 设备模型
type Device struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	NodeKey         string    `gorm:"uniqueIndex" json:"node_key"`
	MachineKey      string    `gorm:"uniqueIndex" json:"machine_key"`
	DiscoKey        string    `json:"disco_key"`
	Hostname        string    `json:"hostname"`
	GivenName       string    `json:"given_name"`
	UserID          uint      `json:"user_id"`
	IPAddresses     string    `json:"ip_addresses"` // JSON array
	Endpoints       string    `json:"endpoints"`    // JSON array
	DERP            string    `json:"derp"`
	LastSeen        time.Time `json:"last_seen"`
	Online          bool      `json:"online"`
	Authorized      bool      `json:"authorized"`
	Tags            string    `json:"tags"` // JSON array
	ForcedTags      string    `json:"forced_tags"` // JSON array
	AdvertiseRoutes string    `json:"advertise_routes"` // JSON array
	EnabledRoutes   string    `json:"enabled_routes"`   // JSON array
	ExitNode        bool      `json:"exit_node"`
	ExitNodeRoute   bool      `json:"exit_node_route"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
	
	// 关联
	User            User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// User 用户模型
type User struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex" json:"name"`
	Email       string    `gorm:"uniqueIndex" json:"email"`
	Phone       string    `gorm:"uniqueIndex" json:"phone"`
	Password    string    `json:"-"` // 密码哈希，不在JSON中返回
	Provider    string    `json:"provider"`
	ProviderId  string    `json:"provider_id"`
	DisplayName string    `json:"display_name"`
	AvatarURL   string    `json:"avatar_url"`
	Role        string    `json:"role"` // admin, user
	Active      bool      `json:"active"` // 用户是否激活
	DeviceLimit int       `gorm:"default:0" json:"device_limit"` // 设备数量限制，0表示无限制
	DeviceCount int       `gorm:"default:0" json:"device_count"` // 当前设备数量
	Remark      string    `json:"remark"` // 用户备注
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Devices     []Device  `gorm:"foreignKey:UserID" json:"devices,omitempty"`
}

// DERPServer DERP服务器模型
type DERPServer struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	RegionID    int       `gorm:"uniqueIndex" json:"region_id"`
	RegionCode  string    `json:"region_code"`
	RegionName  string    `json:"region_name"`
	Hostname    string    `json:"hostname"`
	IPv4        string    `json:"ipv4"`
	IPv6        string    `json:"ipv6"`
	STUNPort    int       `json:"stun_port"`
	DERPPort    int       `json:"derp_port"`
	STUNOnly    bool      `json:"stun_only"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// DNSConfig DNS配置模型
type DNSConfig struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	MagicDNSEnabled bool      `json:"magic_dns_enabled"`
	MagicDNSSuffix  string    `json:"magic_dns_suffix"`
	SearchDomains   string    `json:"search_domains"`   // JSON array
	Nameservers     string    `json:"nameservers"`      // JSON array
	GlobalDNS       string    `json:"global_dns"`       // JSON array
	RestrictedDNS   string    `json:"restricted_dns"`   // JSON object
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`
}

// DNSRecord DNS记录模型
type DNSRecord struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Name      string    `gorm:"index" json:"name"`
	Type      string    `json:"type"`      // A, AAAA, CNAME, MX, TXT, etc.
	Value     string    `json:"value"`     // IP地址或其他值
	TTL       int       `json:"ttl"`       // 生存时间
	Enabled   bool      `json:"enabled"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Route 路由模型
type Route struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	DeviceID    uint      `gorm:"index" json:"device_id"`
	Prefix      string    `gorm:"index" json:"prefix"`      // CIDR格式，如 192.168.1.0/24
	Advertised  bool      `json:"advertised"`               // 是否被设备广播
	Enabled     bool      `json:"enabled"`                  // 是否启用
	Primary     bool      `json:"primary"`                  // 是否为主路由
	ExitNode    bool      `json:"exit_node"`                // 是否为出口节点路由
	Description string    `json:"description"`              // 路由描述
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Device      Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// ExitNodeConfig 出口节点配置
type ExitNodeConfig struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	DeviceID              uint      `gorm:"uniqueIndex" json:"device_id"`
	Enabled               bool      `json:"enabled"`                 // 是否启用出口节点功能
	AllowLANAccess        bool      `json:"allow_lan_access"`        // 是否允许访问本地网络
	AdvertiseDefaultRoute bool      `json:"advertise_default_route"` // 是否广播默认路由
	DNSConfig             string    `json:"dns_config"`              // DNS配置 (JSON)
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`

	// 关联
	Device                Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// FileTransfer 文件传输记录
type FileTransfer struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	SenderID     uint      `gorm:"index" json:"sender_id"`       // 发送方设备ID
	ReceiverID   uint      `gorm:"index" json:"receiver_id"`     // 接收方设备ID
	FileName     string    `json:"file_name"`                    // 文件名
	FileSize     int64     `json:"file_size"`                    // 文件大小（字节）
	FilePath     string    `json:"file_path"`                    // 服务器上的文件路径
	MimeType     string    `json:"mime_type"`                    // MIME类型
	Status       string    `json:"status"`                       // pending, transferring, completed, failed
	Progress     float64   `json:"progress"`                     // 传输进度 (0-100)
	TransferURL  string    `json:"transfer_url,omitempty"`       // 传输URL
	ExpiresAt    time.Time `json:"expires_at"`                   // 过期时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Sender       Device    `gorm:"foreignKey:SenderID" json:"sender,omitempty"`
	Receiver     Device    `gorm:"foreignKey:ReceiverID" json:"receiver,omitempty"`
}

// TaildropConfig Taildrop配置
type TaildropConfig struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	DeviceID          uint      `gorm:"uniqueIndex" json:"device_id"`
	Enabled           bool      `json:"enabled"`             // 是否启用Taildrop
	AutoAccept        bool      `json:"auto_accept"`         // 是否自动接受文件
	SavePath          string    `json:"save_path"`           // 文件保存路径
	MaxFileSize       int64     `json:"max_file_size"`       // 最大文件大小（字节）
	AllowedMimeTypes  string    `json:"allowed_mime_types"`  // 允许的MIME类型 (JSON array)
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// 关联
	Device            Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// SSHKey SSH密钥模型
type SSHKey struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	UserID      uint      `gorm:"index" json:"user_id"`
	DeviceID    uint      `gorm:"index" json:"device_id,omitempty"`
	KeyType     string    `json:"key_type"`        // ssh-rsa, ssh-ed25519, etc.
	PublicKey   string    `gorm:"type:text" json:"public_key"`
	Fingerprint string    `gorm:"uniqueIndex" json:"fingerprint"`
	Comment     string    `json:"comment"`
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	User        User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Device      Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// SSHSession SSH会话记录
type SSHSession struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"user_id"`
	SourceDeviceID uint     `gorm:"index" json:"source_device_id"`
	TargetDeviceID uint     `gorm:"index" json:"target_device_id"`
	Username      string    `json:"username"`           // SSH用户名
	Command       string    `json:"command,omitempty"`  // 执行的命令
	Status        string    `json:"status"`             // active, completed, failed
	StartTime     time.Time `json:"start_time"`
	EndTime       time.Time `json:"end_time,omitempty"`
	Duration      int64     `json:"duration"`           // 会话持续时间（秒）
	BytesIn       int64     `json:"bytes_in"`           // 输入字节数
	BytesOut      int64     `json:"bytes_out"`          // 输出字节数
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 关联
	User          User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
	SourceDevice  Device    `gorm:"foreignKey:SourceDeviceID" json:"source_device,omitempty"`
	TargetDevice  Device    `gorm:"foreignKey:TargetDeviceID" json:"target_device,omitempty"`
}

// SSHConfig SSH配置模型
type SSHConfig struct {
	ID                uint      `gorm:"primaryKey" json:"id"`
	DeviceID          uint      `gorm:"uniqueIndex" json:"device_id"`
	Enabled           bool      `json:"enabled"`              // 是否启用SSH服务
	Port              int       `json:"port"`                 // SSH端口
	AllowedUsers      string    `json:"allowed_users"`        // 允许的用户列表 (JSON array)
	AuthorizedKeys    string    `gorm:"type:text" json:"authorized_keys"` // 授权密钥
	PasswordAuth      bool      `json:"password_auth"`        // 是否允许密码认证
	KeyAuth           bool      `json:"key_auth"`             // 是否允许密钥认证
	RootLogin         bool      `json:"root_login"`           // 是否允许root登录
	ForwardAgent      bool      `json:"forward_agent"`        // 是否允许代理转发
	ForwardX11        bool      `json:"forward_x11"`          // 是否允许X11转发
	CreatedAt         time.Time `json:"created_at"`
	UpdatedAt         time.Time `json:"updated_at"`

	// 关联
	Device            Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// OAuthProvider OAuth提供商配置
type OAuthProvider struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"uniqueIndex" json:"name"`        // 提供商名称 (google, microsoft, github, etc.)
	DisplayName  string    `json:"display_name"`                   // 显示名称
	ClientID     string    `json:"client_id"`                      // 客户端ID
	ClientSecret string    `json:"client_secret"`                  // 客户端密钥
	AuthURL      string    `json:"auth_url"`                       // 授权URL
	TokenURL     string    `json:"token_url"`                      // 令牌URL
	UserInfoURL  string    `json:"user_info_url"`                  // 用户信息URL
	Scopes       string    `json:"scopes"`                         // 权限范围 (JSON array)
	Enabled      bool      `json:"enabled"`                        // 是否启用
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// MFADevice MFA设备
type MFADevice struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	UserID     uint      `gorm:"index" json:"user_id"`
	DeviceType string    `json:"device_type"`           // totp, sms, email, hardware
	DeviceName string    `json:"device_name"`           // 设备名称
	Secret     string    `json:"secret,omitempty"`      // TOTP密钥
	Phone      string    `json:"phone,omitempty"`       // 手机号码
	Email      string    `json:"email,omitempty"`       // 邮箱地址
	BackupCodes string   `json:"backup_codes"`          // 备用代码 (JSON array)
	Enabled    bool      `json:"enabled"`               // 是否启用
	Verified   bool      `json:"verified"`              // 是否已验证
	LastUsed   time.Time `json:"last_used,omitempty"`   // 最后使用时间
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`

	// 关联
	User       User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// AuthSession 认证会话
type AuthSession struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	UserID        uint      `gorm:"index" json:"user_id"`
	SessionToken  string    `gorm:"uniqueIndex" json:"session_token"`
	Provider      string    `json:"provider"`              // local, google, microsoft, etc.
	IPAddress     string    `json:"ip_address"`            // 登录IP
	UserAgent     string    `json:"user_agent"`            // 用户代理
	MFARequired   bool      `json:"mfa_required"`          // 是否需要MFA
	MFACompleted  bool      `json:"mfa_completed"`         // MFA是否完成
	ExpiresAt     time.Time `json:"expires_at"`            // 过期时间
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 关联
	User          User      `gorm:"foreignKey:UserID" json:"user,omitempty"`
}

// SecurityPolicy 安全策略
type SecurityPolicy struct {
	ID                  uint      `gorm:"primaryKey" json:"id"`
	Name                string    `gorm:"uniqueIndex" json:"name"`
	RequireMFA          bool      `json:"require_mfa"`           // 是否要求MFA
	AllowedProviders    string    `json:"allowed_providers"`     // 允许的认证提供商 (JSON array)
	SessionTimeout      int       `json:"session_timeout"`       // 会话超时时间（分钟）
	MaxDevicesPerUser   int       `json:"max_devices_per_user"`  // 每用户最大设备数
	AllowedIPRanges     string    `json:"allowed_ip_ranges"`     // 允许的IP范围 (JSON array)
	BlockedCountries    string    `json:"blocked_countries"`     // 禁止的国家 (JSON array)
	RequireDeviceAuth   bool      `json:"require_device_auth"`   // 是否要求设备认证
	Enabled             bool      `json:"enabled"`               // 是否启用
	CreatedAt           time.Time `json:"created_at"`
	UpdatedAt           time.Time `json:"updated_at"`
}

// KeyRotationPolicy 密钥轮换策略
type KeyRotationPolicy struct {
	ID                    uint      `gorm:"primaryKey" json:"id"`
	Name                  string    `gorm:"uniqueIndex" json:"name"`
	RotationInterval      int       `json:"rotation_interval"`      // 轮换间隔（小时）
	GracePeriod           int       `json:"grace_period"`           // 宽限期（小时）
	AutoRotate            bool      `json:"auto_rotate"`            // 是否自动轮换
	NotifyBeforeRotation  int       `json:"notify_before_rotation"` // 轮换前通知时间（小时）
	MaxKeyAge             int       `json:"max_key_age"`            // 密钥最大年龄（小时）
	TargetDeviceGroups    string    `json:"target_device_groups"`   // 目标设备组 (JSON array)
	Enabled               bool      `json:"enabled"`                // 是否启用
	CreatedAt             time.Time `json:"created_at"`
	UpdatedAt             time.Time `json:"updated_at"`
}

// DeviceKey 设备密钥
type DeviceKey struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	DeviceID     uint      `gorm:"index" json:"device_id"`
	KeyVersion   int       `json:"key_version"`           // 密钥版本
	PublicKey    string    `gorm:"type:text" json:"public_key"`
	PrivateKey   string    `gorm:"type:text" json:"private_key,omitempty"` // 仅在生成时返回
	KeyType      string    `json:"key_type"`              // curve25519, ed25519
	Status       string    `json:"status"`                // active, pending, expired, revoked
	IssuedAt     time.Time `json:"issued_at"`             // 签发时间
	ExpiresAt    time.Time `json:"expires_at"`            // 过期时间
	ActivatedAt  time.Time `json:"activated_at,omitempty"` // 激活时间
	RevokedAt    time.Time `json:"revoked_at,omitempty"`  // 撤销时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Device       Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// KeyRotationJob 密钥轮换任务
type KeyRotationJob struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	PolicyID     uint      `gorm:"index" json:"policy_id"`
	DeviceID     uint      `gorm:"index" json:"device_id"`
	OldKeyID     uint      `json:"old_key_id,omitempty"`
	NewKeyID     uint      `json:"new_key_id,omitempty"`
	Status       string    `json:"status"`               // pending, running, completed, failed
	ScheduledAt  time.Time `json:"scheduled_at"`         // 计划执行时间
	StartedAt    time.Time `json:"started_at,omitempty"` // 开始时间
	CompletedAt  time.Time `json:"completed_at,omitempty"` // 完成时间
	ErrorMessage string    `json:"error_message,omitempty"` // 错误信息
	RetryCount   int       `json:"retry_count"`          // 重试次数
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Policy       KeyRotationPolicy `gorm:"foreignKey:PolicyID" json:"policy,omitempty"`
	Device       Device            `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
	OldKey       DeviceKey         `gorm:"foreignKey:OldKeyID" json:"old_key,omitempty"`
	NewKey       DeviceKey         `gorm:"foreignKey:NewKeyID" json:"new_key,omitempty"`
}

// KeyRotationLog 密钥轮换日志
type KeyRotationLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	JobID     uint      `gorm:"index" json:"job_id"`
	DeviceID  uint      `gorm:"index" json:"device_id"`
	Action    string    `json:"action"`    // generate, activate, revoke, notify
	Status    string    `json:"status"`    // success, failed
	Message   string    `json:"message"`   // 详细信息
	Details   string    `json:"details"`   // 额外详情 (JSON)
	CreatedAt time.Time `json:"created_at"`

	// 关联
	Job       KeyRotationJob `gorm:"foreignKey:JobID" json:"job,omitempty"`
	Device    Device         `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// ServeConfig 服务暴露配置
type ServeConfig struct {
	ID            uint      `gorm:"primaryKey" json:"id"`
	DeviceID      uint      `gorm:"index" json:"device_id"`
	ServiceName   string    `json:"service_name"`           // 服务名称
	ServiceType   string    `json:"service_type"`           // serve, funnel
	Protocol      string    `json:"protocol"`               // http, https, tcp
	LocalPort     int       `json:"local_port"`             // 本地端口
	ExternalPort  int       `json:"external_port"`          // 外部端口
	Domain        string    `json:"domain,omitempty"`       // 自定义域名
	Path          string    `json:"path"`                   // URL路径
	TargetURL     string    `json:"target_url"`             // 目标URL
	Enabled       bool      `json:"enabled"`                // 是否启用
	HTTPSEnabled  bool      `json:"https_enabled"`          // 是否启用HTTPS
	AuthRequired  bool      `json:"auth_required"`          // 是否需要认证
	AllowedUsers  string    `json:"allowed_users"`          // 允许的用户 (JSON array)
	RateLimitRPS  int       `json:"rate_limit_rps"`         // 速率限制（请求/秒）
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at"`

	// 关联
	Device        Device    `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// TLSCertificate TLS证书
type TLSCertificate struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Domain       string    `gorm:"uniqueIndex" json:"domain"`
	CertData     string    `gorm:"type:text" json:"cert_data,omitempty"`     // 证书数据
	KeyData      string    `gorm:"type:text" json:"key_data,omitempty"`      // 私钥数据
	CertChain    string    `gorm:"type:text" json:"cert_chain,omitempty"`    // 证书链
	Issuer       string    `json:"issuer"`                                   // 签发者
	Subject      string    `json:"subject"`                                  // 主题
	SerialNumber string    `json:"serial_number"`                            // 序列号
	NotBefore    time.Time `json:"not_before"`                               // 生效时间
	NotAfter     time.Time `json:"not_after"`                                // 过期时间
	AutoRenew    bool      `json:"auto_renew"`                               // 是否自动续期
	Status       string    `json:"status"`                                   // active, expired, revoked
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// ProxyRule 代理规则
type ProxyRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ServeID     uint      `gorm:"index" json:"serve_id"`
	RuleType    string    `json:"rule_type"`        // path, header, query, method
	Pattern     string    `json:"pattern"`          // 匹配模式
	Action      string    `json:"action"`           // proxy, redirect, block
	Target      string    `json:"target"`           // 目标地址
	Priority    int       `json:"priority"`         // 优先级
	Enabled     bool      `json:"enabled"`          // 是否启用
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Serve       ServeConfig `gorm:"foreignKey:ServeID" json:"serve,omitempty"`
}

// AccessLog 访问日志
type AccessLog struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ServeID      uint      `gorm:"index" json:"serve_id"`
	ClientIP     string    `json:"client_ip"`           // 客户端IP
	UserAgent    string    `json:"user_agent"`          // 用户代理
	Method       string    `json:"method"`              // HTTP方法
	Path         string    `json:"path"`                // 请求路径
	StatusCode   int       `json:"status_code"`         // 状态码
	ResponseSize int64     `json:"response_size"`       // 响应大小
	Duration     int64     `json:"duration"`            // 请求耗时（毫秒）
	Referer      string    `json:"referer,omitempty"`   // 来源页面
	Country      string    `json:"country,omitempty"`   // 国家
	CreatedAt    time.Time `json:"created_at"`

	// 关联
	Serve        ServeConfig `gorm:"foreignKey:ServeID" json:"serve,omitempty"`
}

// K8sCluster Kubernetes集群
type K8sCluster struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Name         string    `gorm:"uniqueIndex" json:"name"`
	Description  string    `json:"description"`
	KubeConfig   string    `gorm:"type:text" json:"kubeconfig,omitempty"`    // Kubernetes配置
	APIServer    string    `json:"api_server"`                               // API服务器地址
	Token        string    `json:"token,omitempty"`                          // 访问令牌
	CACert       string    `gorm:"type:text" json:"ca_cert,omitempty"`       // CA证书
	Namespace    string    `json:"namespace"`                                // 默认命名空间
	CNIEnabled   bool      `json:"cni_enabled"`                              // 是否启用CNI
	CNIConfig    string    `gorm:"type:text" json:"cni_config,omitempty"`    // CNI配置
	PodCIDR      string    `json:"pod_cidr"`                                 // Pod网络CIDR
	ServiceCIDR  string    `json:"service_cidr"`                             // Service网络CIDR
	Status       string    `json:"status"`                                   // connected, disconnected, error
	LastSync     time.Time `json:"last_sync"`                                // 最后同步时间
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// K8sPod Kubernetes Pod
type K8sPod struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClusterID   uint      `gorm:"index" json:"cluster_id"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	PodIP       string    `json:"pod_ip"`
	HostIP      string    `json:"host_ip"`
	NodeName    string    `json:"node_name"`
	Phase       string    `json:"phase"`                    // Pending, Running, Succeeded, Failed, Unknown
	Labels      string    `gorm:"type:text" json:"labels"`  // JSON格式的标签
	Annotations string    `gorm:"type:text" json:"annotations"` // JSON格式的注解
	TailscaleIP string    `json:"tailscale_ip,omitempty"`   // 分配的Tailscale IP
	DeviceID    uint      `json:"device_id,omitempty"`      // 关联的设备ID
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Cluster     K8sCluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	Device      Device     `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// K8sService Kubernetes Service
type K8sService struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClusterID   uint      `gorm:"index" json:"cluster_id"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	Type        string    `json:"type"`                     // ClusterIP, NodePort, LoadBalancer, ExternalName
	ClusterIP   string    `json:"cluster_ip"`
	ExternalIP  string    `json:"external_ip,omitempty"`
	Ports       string    `gorm:"type:text" json:"ports"`   // JSON格式的端口配置
	Selector    string    `gorm:"type:text" json:"selector"` // JSON格式的选择器
	TailscaleIP string    `json:"tailscale_ip,omitempty"`   // 分配的Tailscale IP
	Exposed     bool      `json:"exposed"`                  // 是否通过Tailscale暴露
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Cluster     K8sCluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// K8sNode Kubernetes节点
type K8sNode struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	ClusterID    uint      `gorm:"index" json:"cluster_id"`
	Name         string    `json:"name"`
	InternalIP   string    `json:"internal_ip"`
	ExternalIP   string    `json:"external_ip,omitempty"`
	Hostname     string    `json:"hostname"`
	OSImage      string    `json:"os_image"`
	KernelVersion string   `json:"kernel_version"`
	ContainerRuntime string `json:"container_runtime"`
	KubeletVersion string  `json:"kubelet_version"`
	Ready        bool      `json:"ready"`
	TailscaleIP  string    `json:"tailscale_ip,omitempty"`   // 分配的Tailscale IP
	DeviceID     uint      `json:"device_id,omitempty"`      // 关联的设备ID
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`

	// 关联
	Cluster      K8sCluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
	Device       Device     `gorm:"foreignKey:DeviceID" json:"device,omitempty"`
}

// K8sNetworkPolicy 网络策略
type K8sNetworkPolicy struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	ClusterID   uint      `gorm:"index" json:"cluster_id"`
	Name        string    `json:"name"`
	Namespace   string    `json:"namespace"`
	PolicyType  string    `json:"policy_type"`              // Ingress, Egress, Both
	PodSelector string    `gorm:"type:text" json:"pod_selector"` // JSON格式的Pod选择器
	Ingress     string    `gorm:"type:text" json:"ingress"`      // JSON格式的入站规则
	Egress      string    `gorm:"type:text" json:"egress"`       // JSON格式的出站规则
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Cluster     K8sCluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// K8sOperatorConfig Operator配置
type K8sOperatorConfig struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	ClusterID       uint      `gorm:"index" json:"cluster_id"`
	OperatorImage   string    `json:"operator_image"`           // Operator镜像
	OperatorVersion string    `json:"operator_version"`         // Operator版本
	Namespace       string    `json:"namespace"`                // 部署命名空间
	ServiceAccount  string    `json:"service_account"`          // 服务账户
	ClusterRole     string    `json:"cluster_role"`             // 集群角色
	ConfigMap       string    `gorm:"type:text" json:"config_map"` // 配置映射
	Secret          string    `gorm:"type:text" json:"secret"`     // 密钥配置
	AutoSync        bool      `json:"auto_sync"`                // 自动同步
	SyncInterval    int       `json:"sync_interval"`            // 同步间隔（秒）
	LogLevel        string    `json:"log_level"`                // 日志级别
	Enabled         bool      `json:"enabled"`
	CreatedAt       time.Time `json:"created_at"`
	UpdatedAt       time.Time `json:"updated_at"`

	// 关联
	Cluster         K8sCluster `gorm:"foreignKey:ClusterID" json:"cluster,omitempty"`
}

// Webhook Webhook配置
type Webhook struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `gorm:"uniqueIndex" json:"name"`
	URL         string    `json:"url"`                          // Webhook URL
	Secret      string    `json:"secret,omitempty"`             // 签名密钥
	Events      string    `gorm:"type:text" json:"events"`      // 订阅的事件类型 (JSON array)
	Headers     string    `gorm:"type:text" json:"headers"`     // 自定义头部 (JSON object)
	Timeout     int       `json:"timeout"`                      // 超时时间（秒）
	Retries     int       `json:"retries"`                      // 重试次数
	Enabled     bool      `json:"enabled"`                      // 是否启用
	LastTrigger time.Time `json:"last_trigger"`                 // 最后触发时间
	SuccessCount int64    `json:"success_count"`                // 成功次数
	FailureCount int64    `json:"failure_count"`                // 失败次数
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// WebhookDelivery Webhook投递记录
type WebhookDelivery struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	WebhookID    uint      `gorm:"index" json:"webhook_id"`
	EventType    string    `json:"event_type"`                   // 事件类型
	EventData    string    `gorm:"type:text" json:"event_data"`  // 事件数据 (JSON)
	RequestURL   string    `json:"request_url"`                  // 请求URL
	RequestHeaders string  `gorm:"type:text" json:"request_headers"` // 请求头部 (JSON)
	RequestBody  string    `gorm:"type:text" json:"request_body"`    // 请求体
	ResponseCode int       `json:"response_code"`                // 响应状态码
	ResponseHeaders string `gorm:"type:text" json:"response_headers"` // 响应头部 (JSON)
	ResponseBody string    `gorm:"type:text" json:"response_body"`    // 响应体
	Duration     int64     `json:"duration"`                     // 请求耗时（毫秒）
	Success      bool      `json:"success"`                      // 是否成功
	Error        string    `json:"error,omitempty"`              // 错误信息
	Attempt      int       `json:"attempt"`                      // 尝试次数
	CreatedAt    time.Time `json:"created_at"`

	// 关联
	Webhook      Webhook   `gorm:"foreignKey:WebhookID" json:"webhook,omitempty"`
}

// APIKey API密钥
type APIKey struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`                         // 密钥名称
	KeyID       string    `gorm:"uniqueIndex" json:"key_id"`    // 密钥ID
	KeySecret   string    `json:"key_secret,omitempty"`         // 密钥值（创建时返回）
	KeyHash     string    `json:"key_hash"`                     // 密钥哈希值
	Permissions string    `gorm:"type:text" json:"permissions"` // 权限列表 (JSON array)
	ExpiresAt   time.Time `json:"expires_at"`                   // 过期时间
	LastUsed    time.Time `json:"last_used"`                    // 最后使用时间
	UsageCount  int64     `json:"usage_count"`                  // 使用次数
	Enabled     bool      `json:"enabled"`                      // 是否启用
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// PreAuthKey 预授权密钥模型
type PreAuthKey struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	Key        string    `gorm:"uniqueIndex;not null" json:"key"`
	UserID     uint      `gorm:"not null" json:"user_id"`
	User       *User     `gorm:"foreignKey:UserID" json:"user,omitempty"`
	Reusable   bool      `gorm:"default:false" json:"reusable"`
	Ephemeral  bool      `gorm:"default:false" json:"ephemeral"`
	Used       bool      `gorm:"default:false" json:"used"`
	CreatedAt  time.Time `json:"created_at"`
	ExpiresAt  time.Time `json:"expires_at"`
	UsedAt     *time.Time `json:"used_at,omitempty"`
	UsedBy     string    `json:"used_by,omitempty"` // 使用该密钥的设备标识
}

// SDKUsage SDK使用统计
type SDKUsage struct {
	ID         uint      `gorm:"primaryKey" json:"id"`
	APIKeyID   uint      `gorm:"index" json:"api_key_id"`
	SDKVersion string    `json:"sdk_version"`                  // SDK版本
	Language   string    `json:"language"`                     // 编程语言
	Method     string    `json:"method"`                       // API方法
	Endpoint   string    `json:"endpoint"`                     // API端点
	UserAgent  string    `json:"user_agent"`                   // 用户代理
	ClientIP   string    `json:"client_ip"`                    // 客户端IP
	Duration   int64     `json:"duration"`                     // 请求耗时（毫秒）
	Success    bool      `json:"success"`                      // 是否成功
	Error      string    `json:"error,omitempty"`              // 错误信息
	CreatedAt  time.Time `json:"created_at"`

	// 关联
	APIKey     APIKey    `gorm:"foreignKey:APIKeyID" json:"api_key,omitempty"`
}

// ACLRule ACL规则模型
type ACLRule struct {
	ID           uint      `gorm:"primaryKey" json:"id"`
	Action       string    `json:"action"` // accept, deny
	Sources      string    `json:"sources"` // JSON array
	Destinations string    `json:"destinations"` // JSON array
	Ports        string    `json:"ports"` // JSON array
	Protocols    string    `json:"protocols"` // JSON array
	Priority     int       `json:"priority"`
	Comment      string    `json:"comment"`
	Enabled      bool      `json:"enabled"`
	CreatedAt    time.Time `json:"created_at"`
	UpdatedAt    time.Time `json:"updated_at"`
}

// AlertRule 告警规则模型
type AlertRule struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	Name        string    `json:"name"`
	Description string    `json:"description"`
	Metric      string    `json:"metric"`      // cpu_usage, memory_usage, device_offline, etc.
	Operator    string    `json:"operator"`    // >, <, >=, <=, ==, !=
	Threshold   float64   `json:"threshold"`   // 阈值
	Duration    int       `json:"duration"`    // 持续时间(秒)
	Severity    string    `json:"severity"`    // critical, warning, info
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`

	// 关联
	Notifications []AlertNotification `gorm:"foreignKey:AlertRuleID" json:"notifications,omitempty"`
}

// AlertNotification 告警通知配置
type AlertNotification struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AlertRuleID uint      `json:"alert_rule_id"`
	Type        string    `json:"type"`        // email, webhook, slack
	Target      string    `json:"target"`      // 邮箱地址、Webhook URL等
	Enabled     bool      `json:"enabled"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

// AlertHistory 告警历史记录
type AlertHistory struct {
	ID          uint      `gorm:"primaryKey" json:"id"`
	AlertRuleID uint      `json:"alert_rule_id"`
	Message     string    `json:"message"`
	Severity    string    `json:"severity"`
	Value       float64   `json:"value"`       // 触发时的实际值
	Resolved    bool      `json:"resolved"`    // 是否已解决
	CreatedAt   time.Time `json:"created_at"`
	ResolvedAt  *time.Time `json:"resolved_at,omitempty"`

	// 关联
	AlertRule   AlertRule `gorm:"foreignKey:AlertRuleID" json:"alert_rule,omitempty"`
}

// SystemLog 系统日志模型
type SystemLog struct {
	ID        uint      `gorm:"primaryKey" json:"id"`
	Level     string    `json:"level"`     // debug, info, warn, error
	Component string    `json:"component"` // api, auth, device, etc.
	Message   string    `json:"message"`
	Data      string    `json:"data"`      // JSON格式的额外数据
	UserID    *uint     `json:"user_id,omitempty"`
	DeviceID  *uint     `json:"device_id,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

func main() {
	// 检查是否是LocalAPI工具调用
	if len(os.Args) > 1 && os.Args[1] == "localapi" {
		runLocalAPITool()
		return
	}

	var (
		listenAddr = flag.String("listen", ":8080", "HTTP listen address")
		dbPath     = flag.String("db", "unlimited.db", "SQLite database path")
		debug      = flag.Bool("debug", false, "Enable debug mode")
	)
	flag.Parse()

	if !*debug {
		gin.SetMode(gin.ReleaseMode)
	}

	server := &UnlimitedControlServer{
		listenAddr: *listenAddr,
		dbPath:     *dbPath,
	}

	if err := server.Initialize(); err != nil {
		log.Fatalf("Failed to initialize server: %v", err)
	}

	// 启动告警引擎
	server.alertEngine.Start()

	// 启动服务器
	go func() {
		log.Printf("🚀 Tailscale Unlimited Control Server starting on %s", *listenAddr)
		if err := server.Start(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Failed to start server: %v", err)
		}
	}()

	// 等待中断信号
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Println("🛑 Shutting down server...")

	// 优雅关闭
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %v", err)
	}

	log.Println("✅ Server exited")
}

// Initialize 初始化服务器
func (s *UnlimitedControlServer) Initialize() error {
	// 初始化数据库
	if err := s.initDatabase(); err != nil {
		return fmt.Errorf("failed to init database: %w", err)
	}

	// 初始化路由
	s.initRoutes()

	// 初始化告警引擎
	s.alertEngine = NewAlertEngine(s)

	return nil
}

// initDatabase 初始化数据库
func (s *UnlimitedControlServer) initDatabase() error {
	var err error
	s.db, err = gorm.Open(sqlite.Open(s.dbPath), &gorm.Config{})
	if err != nil {
		return err
	}

	// 自动迁移
	err = s.db.AutoMigrate(&User{}, &Device{}, &DERPServer{}, &ACLRule{},
		&AlertRule{}, &AlertNotification{}, &AlertHistory{}, &SystemLog{},
		&DNSConfig{}, &DNSRecord{}, &Route{}, &ExitNodeConfig{},
		&FileTransfer{}, &TaildropConfig{}, &SSHKey{}, &SSHSession{}, &SSHConfig{},
		&OAuthProvider{}, &MFADevice{}, &AuthSession{}, &SecurityPolicy{},
		&KeyRotationPolicy{}, &DeviceKey{}, &KeyRotationJob{}, &KeyRotationLog{},
		&ServeConfig{}, &TLSCertificate{}, &ProxyRule{}, &AccessLog{},
		&K8sCluster{}, &K8sPod{}, &K8sService{}, &K8sNode{}, &K8sNetworkPolicy{}, &K8sOperatorConfig{},
		&Webhook{}, &WebhookDelivery{}, &APIKey{}, &PreAuthKey{}, &SDKUsage{})
	if err != nil {
		return err
	}

	// 创建默认管理员用户
	var adminCount int64
	s.db.Model(&User{}).Where("role = ?", "admin").Count(&adminCount)
	if adminCount == 0 {
		admin := User{
			Name:        "admin",
			Email:       "admin@localhost",
			Provider:    "local",
			DisplayName: "Administrator",
			Role:        "admin",
		}
		s.db.Create(&admin)
		log.Println("✅ Created default admin user")
	}

	// 创建默认DERP服务器
	var derpCount int64
	s.db.Model(&DERPServer{}).Count(&derpCount)
	if derpCount == 0 {
		defaultDERP := DERPServer{
			RegionID:   1,
			RegionCode: "local",
			RegionName: "Local DERP",
			Hostname:   "localhost",
			IPv4:       "127.0.0.1",
			STUNPort:   3478,
			DERPPort:   443,
			Enabled:    true,
		}
		s.db.Create(&defaultDERP)
		log.Println("✅ Created default DERP server")
	}

	// 创建默认DNS配置
	var dnsCount int64
	s.db.Model(&DNSConfig{}).Count(&dnsCount)
	if dnsCount == 0 {
		searchDomains := `["local"]`
		nameservers := `["8.8.8.8", "8.8.4.4"]`
		globalDNS := `["1.1.1.1", "1.0.0.1"]`

		defaultDNS := DNSConfig{
			MagicDNSEnabled: true,
			MagicDNSSuffix:  "ts.net",
			SearchDomains:   searchDomains,
			Nameservers:     nameservers,
			GlobalDNS:       globalDNS,
			RestrictedDNS:   `{}`,
		}
		s.db.Create(&defaultDNS)
		log.Println("✅ Created default DNS configuration")
	}

	return nil
}

// initRoutes 初始化路由
func (s *UnlimitedControlServer) initRoutes() {
	s.router = gin.New()
	s.router.Use(gin.Logger(), gin.Recovery())

	// CORS中间件
	s.router.Use(func(c *gin.Context) {
		c.Header("Access-Control-Allow-Origin", "*")
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if c.Request.Method == "OPTIONS" {
			c.AbortWithStatus(204)
			return
		}
		
		c.Next()
	})

	// 健康检查
	s.router.GET("/api/v1/health", s.handleHealth)

	// API路由组
	api := s.router.Group("/api/v1")
	{
		// 认证相关（无需认证）
		auth := api.Group("/auth")
		{
			auth.POST("/register", s.handleRegister)
			auth.POST("/login", s.handleLogin)
			auth.POST("/logout", s.handleLogout)
			auth.POST("/refresh", s.handleRefreshToken)
		}

		// 统计信息
		api.GET("/stats", s.handleStats)

		// 用户管理
		users := api.Group("/users")
		{
			users.GET("", s.handleListUsers)
			users.POST("", s.handleCreateUser)
			users.GET("/:id", s.handleGetUser)
			users.PUT("/:id", s.handleUpdateUser)
			users.DELETE("/:id", s.handleDeleteUser)
		}

		// 设备管理
		devices := api.Group("/devices")
		{
			devices.POST("/batch-delete", s.handleBatchDeleteDevices)
			devices.GET("", s.handleListDevices)
			devices.POST("", s.handleCreateDevice)
			devices.GET("/:id", s.handleGetDevice)
			devices.PUT("/:id", s.handleUpdateDevice)
			devices.DELETE("/:id", s.handleDeleteDevice)
			devices.POST("/:id/authorize", s.handleAuthorizeDevice)
			devices.POST("/:id/routes", s.handleUpdateRoutes)
		}



		// DERP服务器管理
		derp := api.Group("/derp")
		{
			derp.GET("/servers", s.handleListDERPServers)
			derp.POST("/servers", s.handleCreateDERPServer)
			derp.GET("/servers/:id", s.handleGetDERPServer)
			derp.PUT("/servers/:id", s.handleUpdateDERPServer)
			derp.DELETE("/servers/:id", s.handleDeleteDERPServer)
			derp.GET("/map", s.handleGetDERPMap)
		}

		// ACL管理
		acl := api.Group("/acl")
		{
			acl.GET("/rules", s.handleListACLRules)
			acl.POST("/rules", s.handleCreateACLRule)
			acl.GET("/rules/:id", s.handleGetACLRule)
			acl.PUT("/rules/:id", s.handleUpdateACLRule)
			acl.DELETE("/rules/:id", s.handleDeleteACLRule)
		}

		// 网络映射
		api.GET("/network-map", s.handleGetNetworkMap)
		api.POST("/network-map/update", s.handleUpdateNetworkMap)

		// API密钥管理
		api.GET("/apikey", s.handleListAPIKeys)
		api.POST("/apikey", s.handleCreateAPIKey)
		api.POST("/apikey/expire", s.handleExpireAPIKey)

		// 预授权密钥管理
		api.GET("/preauthkey", s.handleListPreAuthKeys)
		api.POST("/preauthkey", s.handleCreatePreAuthKey)
		api.POST("/preauthkey/expire", s.handleExpirePreAuthKey)

		// 用户设备限制管理
		api.POST("/users/device-limit", s.handleSetUserDeviceLimit)
		api.POST("/devices/register", s.handleUserDeviceRegistration)
		api.POST("/users/:id/sync-device-count", s.handleSyncUserDeviceCount)
		api.POST("/users/sync-all-device-count", s.handleSyncAllUserDeviceCount)

		// 告警管理
		alerts := api.Group("/alerts")
		{
			alerts.GET("/rules", s.handleListAlertRules)
			alerts.POST("/rules", s.handleCreateAlertRule)
			alerts.GET("/rules/:id", s.handleGetAlertRule)
			alerts.PUT("/rules/:id", s.handleUpdateAlertRule)
			alerts.DELETE("/rules/:id", s.handleDeleteAlertRule)
			alerts.GET("/history", s.handleListAlertHistory)
			alerts.POST("/test", s.handleTestAlert)
			alerts.GET("/notifications", s.handleGetNotificationSettings)
			alerts.PUT("/notifications", s.handleUpdateNotificationSettings)
		}

		// 日志管理
		logs := api.Group("/logs")
		{
			logs.GET("", s.handleListLogs)
			logs.POST("", s.handleCreateLog)
			logs.DELETE("/:id", s.handleDeleteLog)
			logs.DELETE("", s.handleClearLogs)
			logs.GET("/stream", s.handleLogStream)
			logs.GET("/export", s.handleExportLogs)
		}

		// 报表管理
		reports := api.Group("/reports")
		{
			reports.GET("/usage", s.handleUsageReport)
			reports.GET("/performance", s.handlePerformanceReport)
			reports.GET("/devices", s.handleDeviceReport)
			reports.GET("/network", s.handleNetworkReport)
			reports.GET("/export/:type", s.handleExportReport)
		}

		// 网络诊断
		network := api.Group("/network")
		{
			network.POST("/ping", s.handleNetworkPing)
			network.GET("/stats", s.handleNetworkStats)
			network.POST("/traceroute", s.handleTraceroute)
			network.GET("/quality", s.handleConnectionQuality)
		}

		// DNS管理
		dns := api.Group("/dns")
		{
			dns.GET("/config", s.handleGetDNSConfig)
			dns.PUT("/config", s.handleUpdateDNSConfig)
			dns.GET("/records", s.handleListDNSRecords)
			dns.POST("/records", s.handleCreateDNSRecord)
			dns.GET("/records/:id", s.handleGetDNSRecord)
			dns.PUT("/records/:id", s.handleUpdateDNSRecord)
			dns.DELETE("/records/:id", s.handleDeleteDNSRecord)
			dns.POST("/resolve", s.handleDNSResolve)
			dns.GET("/status", s.handleDNSStatus)
		}

		// 路由管理
		routes := api.Group("/routes")
		{
			routes.GET("", s.handleListRoutes)
			routes.POST("", s.handleCreateRoute)
			routes.GET("/:id", s.handleGetRoute)
			routes.PUT("/:id", s.handleUpdateRoute)
			routes.DELETE("/:id", s.handleDeleteRoute)
			routes.POST("/:id/enable", s.handleEnableRoute)
			routes.POST("/:id/disable", s.handleDisableRoute)
			routes.GET("/device/:device_id", s.handleGetDeviceRoutes)
		}

		// 出口节点管理
		exitNodes := api.Group("/exit-nodes")
		{
			exitNodes.GET("", s.handleListExitNodes)
			exitNodes.POST("", s.handleCreateExitNode)
			exitNodes.GET("/:id", s.handleGetExitNode)
			exitNodes.PUT("/:id", s.handleUpdateExitNode)
			exitNodes.DELETE("/:id", s.handleDeleteExitNode)
			exitNodes.POST("/:id/enable", s.handleEnableExitNode)
			exitNodes.POST("/:id/disable", s.handleDisableExitNode)
			exitNodes.GET("/available", s.handleGetAvailableExitNodes)
		}

		// Taildrop文件共享
		taildrop := api.Group("/taildrop")
		{
			taildrop.GET("/targets", s.handleGetTaildropTargets)
			taildrop.POST("/send", s.handleSendFile)
			taildrop.GET("/transfers", s.handleListFileTransfers)
			taildrop.GET("/transfers/:id", s.handleGetFileTransfer)
			taildrop.POST("/transfers/:id/accept", s.handleAcceptFileTransfer)
			taildrop.POST("/transfers/:id/reject", s.handleRejectFileTransfer)
			taildrop.DELETE("/transfers/:id", s.handleDeleteFileTransfer)
			taildrop.GET("/config/:device_id", s.handleGetTaildropConfig)
			taildrop.PUT("/config/:device_id", s.handleUpdateTaildropConfig)
			taildrop.GET("/download/:id", s.handleDownloadFile)
		}

		// SSH访问代理
		ssh := api.Group("/ssh")
		{
			// SSH密钥管理
			ssh.GET("/keys", s.handleListSSHKeys)
			ssh.POST("/keys", s.handleCreateSSHKey)
			ssh.GET("/keys/:id", s.handleGetSSHKey)
			ssh.PUT("/keys/:id", s.handleUpdateSSHKey)
			ssh.DELETE("/keys/:id", s.handleDeleteSSHKey)
			ssh.POST("/keys/:id/enable", s.handleEnableSSHKey)
			ssh.POST("/keys/:id/disable", s.handleDisableSSHKey)

			// SSH配置管理
			ssh.GET("/config/:device_id", s.handleGetSSHConfig)
			ssh.PUT("/config/:device_id", s.handleUpdateSSHConfig)

			// SSH会话管理
			ssh.GET("/sessions", s.handleListSSHSessions)
			ssh.GET("/sessions/:id", s.handleGetSSHSession)
			ssh.POST("/sessions/:id/terminate", s.handleTerminateSSHSession)

			// SSH连接
			ssh.POST("/connect", s.handleSSHConnect)
			ssh.GET("/hosts", s.handleGetSSHHosts)
		}

		// SSO和认证管理
		sso := api.Group("/sso")
		{
			// OAuth提供商管理
			sso.GET("/providers", s.handleListOAuthProviders)
			sso.POST("/providers", s.handleCreateOAuthProvider)
			sso.GET("/providers/:id", s.handleGetOAuthProvider)
			sso.PUT("/providers/:id", s.handleUpdateOAuthProvider)
			sso.DELETE("/providers/:id", s.handleDeleteOAuthProvider)
			sso.POST("/providers/:id/enable", s.handleEnableOAuthProvider)
			sso.POST("/providers/:id/disable", s.handleDisableOAuthProvider)

			// OAuth认证流程
			sso.GET("/oauth/:provider", s.handleOAuthLogin)
			sso.GET("/oauth/:provider/callback", s.handleOAuthCallback)

			// MFA设备管理
			sso.GET("/mfa/devices", s.handleListMFADevices)
			sso.POST("/mfa/devices", s.handleCreateMFADevice)
			sso.GET("/mfa/devices/:id", s.handleGetMFADevice)
			sso.PUT("/mfa/devices/:id", s.handleUpdateMFADevice)
			sso.DELETE("/mfa/devices/:id", s.handleDeleteMFADevice)
			sso.POST("/mfa/devices/:id/verify", s.handleVerifyMFADevice)

			// MFA认证
			sso.POST("/mfa/verify", s.handleVerifyMFA)
			sso.POST("/mfa/backup", s.handleVerifyBackupCode)
			sso.GET("/mfa/qr/:id", s.handleGetMFAQRCode)

			// 会话管理
			sso.GET("/sessions", s.handleListAuthSessions)
			sso.DELETE("/sessions/:id", s.handleDeleteAuthSession)
			sso.POST("/sessions/revoke-all", s.handleRevokeAllSessions)

			// 安全策略
			sso.GET("/policies", s.handleListSecurityPolicies)
			sso.POST("/policies", s.handleCreateSecurityPolicy)
			sso.GET("/policies/:id", s.handleGetSecurityPolicy)
			sso.PUT("/policies/:id", s.handleUpdateSecurityPolicy)
			sso.DELETE("/policies/:id", s.handleDeleteSecurityPolicy)
		}

		// 密钥轮换管理
		keyRotation := api.Group("/key-rotation")
		{
			// 轮换策略管理
			keyRotation.GET("/policies", s.handleListKeyRotationPolicies)
			keyRotation.POST("/policies", s.handleCreateKeyRotationPolicy)
			keyRotation.GET("/policies/:id", s.handleGetKeyRotationPolicy)
			keyRotation.PUT("/policies/:id", s.handleUpdateKeyRotationPolicy)
			keyRotation.DELETE("/policies/:id", s.handleDeleteKeyRotationPolicy)
			keyRotation.POST("/policies/:id/enable", s.handleEnableKeyRotationPolicy)
			keyRotation.POST("/policies/:id/disable", s.handleDisableKeyRotationPolicy)

			// 设备密钥管理
			keyRotation.GET("/keys", s.handleListDeviceKeys)
			keyRotation.POST("/keys", s.handleCreateDeviceKey)
			keyRotation.GET("/keys/:id", s.handleGetDeviceKey)
			keyRotation.POST("/keys/:id/activate", s.handleActivateDeviceKey)
			keyRotation.POST("/keys/:id/revoke", s.handleRevokeDeviceKey)
			keyRotation.GET("/keys/device/:device_id", s.handleGetDeviceKeysByDevice)

			// 轮换任务管理
			keyRotation.GET("/jobs", s.handleListKeyRotationJobs)
			keyRotation.POST("/jobs", s.handleCreateKeyRotationJob)
			keyRotation.GET("/jobs/:id", s.handleGetKeyRotationJob)
			keyRotation.POST("/jobs/:id/execute", s.handleExecuteKeyRotationJob)
			keyRotation.POST("/jobs/:id/cancel", s.handleCancelKeyRotationJob)
			keyRotation.POST("/jobs/schedule", s.handleScheduleKeyRotation)

			// 轮换日志
			keyRotation.GET("/logs", s.handleListKeyRotationLogs)
			keyRotation.GET("/logs/job/:job_id", s.handleGetKeyRotationLogsByJob)
			keyRotation.GET("/logs/device/:device_id", s.handleGetKeyRotationLogsByDevice)

			// 手动轮换
			keyRotation.POST("/rotate/:device_id", s.handleManualKeyRotation)
			keyRotation.POST("/rotate/batch", s.handleBatchKeyRotation)
		}

		// 服务暴露管理
		serve := api.Group("/serve")
		{
			// 服务配置管理
			serve.GET("/configs", s.handleListServeConfigs)
			serve.POST("/configs", s.handleCreateServeConfig)
			serve.GET("/configs/:id", s.handleGetServeConfig)
			serve.PUT("/configs/:id", s.handleUpdateServeConfig)
			serve.DELETE("/configs/:id", s.handleDeleteServeConfig)
			serve.POST("/configs/:id/enable", s.handleEnableServeConfig)
			serve.POST("/configs/:id/disable", s.handleDisableServeConfig)

			// TLS证书管理
			serve.GET("/certificates", s.handleListTLSCertificates)
			serve.POST("/certificates", s.handleCreateTLSCertificate)
			serve.GET("/certificates/:id", s.handleGetTLSCertificate)
			serve.PUT("/certificates/:id", s.handleUpdateTLSCertificate)
			serve.DELETE("/certificates/:id", s.handleDeleteTLSCertificate)
			serve.POST("/certificates/:id/renew", s.handleRenewTLSCertificate)
			serve.GET("/certificates/domain/:domain", s.handleGetCertificateByDomain)

			// 代理规则管理
			serve.GET("/rules", s.handleListProxyRules)
			serve.POST("/rules", s.handleCreateProxyRule)
			serve.GET("/rules/:id", s.handleGetProxyRule)
			serve.PUT("/rules/:id", s.handleUpdateProxyRule)
			serve.DELETE("/rules/:id", s.handleDeleteProxyRule)
			serve.GET("/rules/serve/:serve_id", s.handleGetRulesByServe)

			// 访问日志
			serve.GET("/logs", s.handleListAccessLogs)
			serve.GET("/logs/serve/:serve_id", s.handleGetAccessLogsByServe)
			serve.GET("/logs/stats", s.handleGetAccessStats)
			serve.DELETE("/logs", s.handleClearAccessLogs)

			// 服务状态和监控
			serve.GET("/status", s.handleGetServeStatus)
			serve.POST("/test/:id", s.handleTestServeConfig)
			serve.GET("/metrics/:id", s.handleGetServeMetrics)
		}

		// Kubernetes集成管理
		k8s := api.Group("/k8s")
		{
			// 集群管理
			k8s.GET("/clusters", s.handleListK8sClusters)
			k8s.POST("/clusters", s.handleCreateK8sCluster)
			k8s.GET("/clusters/:id", s.handleGetK8sCluster)
			k8s.PUT("/clusters/:id", s.handleUpdateK8sCluster)
			k8s.DELETE("/clusters/:id", s.handleDeleteK8sCluster)
			k8s.POST("/clusters/:id/connect", s.handleConnectK8sCluster)
			k8s.POST("/clusters/:id/disconnect", s.handleDisconnectK8sCluster)
			k8s.POST("/clusters/:id/sync", s.handleSyncK8sCluster)

			// Pod管理
			k8s.GET("/pods", s.handleListK8sPods)
			k8s.GET("/pods/:id", s.handleGetK8sPod)
			k8s.POST("/pods/:id/assign-ip", s.handleAssignPodIP)
			k8s.DELETE("/pods/:id/release-ip", s.handleReleasePodIP)
			k8s.GET("/pods/cluster/:cluster_id", s.handleGetPodsByCluster)

			// Service管理
			k8s.GET("/services", s.handleListK8sServices)
			k8s.GET("/services/:id", s.handleGetK8sService)
			k8s.POST("/services/:id/expose", s.handleExposeK8sService)
			k8s.DELETE("/services/:id/unexpose", s.handleUnexposeK8sService)
			k8s.GET("/services/cluster/:cluster_id", s.handleGetServicesByCluster)

			// 节点管理
			k8s.GET("/nodes", s.handleListK8sNodes)
			k8s.GET("/nodes/:id", s.handleGetK8sNode)
			k8s.POST("/nodes/:id/register", s.handleRegisterK8sNode)
			k8s.DELETE("/nodes/:id/unregister", s.handleUnregisterK8sNode)
			k8s.GET("/nodes/cluster/:cluster_id", s.handleGetNodesByCluster)

			// 网络策略管理
			k8s.GET("/policies", s.handleListK8sNetworkPolicies)
			k8s.POST("/policies", s.handleCreateK8sNetworkPolicy)
			k8s.GET("/policies/:id", s.handleGetK8sNetworkPolicy)
			k8s.PUT("/policies/:id", s.handleUpdateK8sNetworkPolicy)
			k8s.DELETE("/policies/:id", s.handleDeleteK8sNetworkPolicy)
			k8s.POST("/policies/:id/enable", s.handleEnableK8sNetworkPolicy)
			k8s.POST("/policies/:id/disable", s.handleDisableK8sNetworkPolicy)

			// Operator配置管理
			k8s.GET("/operator", s.handleListK8sOperatorConfigs)
			k8s.POST("/operator", s.handleCreateK8sOperatorConfig)
			k8s.GET("/operator/:id", s.handleGetK8sOperatorConfig)
			k8s.PUT("/operator/:id", s.handleUpdateK8sOperatorConfig)
			k8s.DELETE("/operator/:id", s.handleDeleteK8sOperatorConfig)
			k8s.POST("/operator/:id/deploy", s.handleDeployK8sOperator)
			k8s.DELETE("/operator/:id/undeploy", s.handleUndeployK8sOperator)

			// 状态和监控
			k8s.GET("/status", s.handleGetK8sStatus)
			k8s.GET("/metrics", s.handleGetK8sMetrics)
			k8s.GET("/events", s.handleGetK8sEvents)
		}

		// Webhook管理
		webhooks := api.Group("/webhooks")
		{
			// Webhook配置管理
			webhooks.GET("", s.handleListWebhooks)
			webhooks.POST("", s.handleCreateWebhook)
			webhooks.GET("/:id", s.handleGetWebhook)
			webhooks.PUT("/:id", s.handleUpdateWebhook)
			webhooks.DELETE("/:id", s.handleDeleteWebhook)
			webhooks.POST("/:id/enable", s.handleEnableWebhook)
			webhooks.POST("/:id/disable", s.handleDisableWebhook)
			webhooks.POST("/:id/test", s.handleTestWebhook)

			// Webhook投递记录
			webhooks.GET("/:id/deliveries", s.handleGetWebhookDeliveries)
			webhooks.GET("/deliveries/:delivery_id", s.handleGetWebhookDelivery)
			webhooks.POST("/deliveries/:delivery_id/redeliver", s.handleRedeliverWebhook)

			// Webhook统计
			webhooks.GET("/:id/stats", s.handleGetWebhookStats)
			webhooks.GET("/events", s.handleListWebhookEvents)
		}

		// API密钥管理
		apiKeys := api.Group("/api-keys")
		{
			apiKeys.GET("", s.handleListAPIKeys)
			apiKeys.POST("", s.handleCreateAPIKey)
			apiKeys.GET("/:id", s.handleGetAPIKey)
			apiKeys.PUT("/:id", s.handleUpdateAPIKey)
			apiKeys.DELETE("/:id", s.handleDeleteAPIKey)
			apiKeys.POST("/:id/enable", s.handleEnableAPIKey)
			apiKeys.POST("/:id/disable", s.handleDisableAPIKey)
			apiKeys.POST("/:id/regenerate", s.handleRegenerateAPIKey)

			// API密钥使用统计
			apiKeys.GET("/:id/usage", s.handleGetAPIKeyUsage)
			apiKeys.GET("/usage/stats", s.handleGetAPIUsageStats)
		}

		// SDK管理
		sdk := api.Group("/sdk")
		{
			// SDK信息
			sdk.GET("/info", s.handleGetSDKInfo)
			sdk.GET("/versions", s.handleGetSDKVersions)
			sdk.GET("/docs", s.handleGetSDKDocs)

			// SDK使用统计
			sdk.GET("/usage", s.handleGetSDKUsage)
			sdk.GET("/usage/stats", s.handleGetSDKUsageStats)

			// SDK下载
			sdk.GET("/download/:language/:version", s.handleDownloadSDK)
		}
	}

	// Tailscale协议兼容API
	control := s.router.Group("/machine")
	{
		control.POST("/register", s.handleMachineRegister)
		control.POST("/map", s.handleMachineMap)
	}

	// 静态文件服务 (Web界面)
	s.router.Static("/static", "./headscale-ui/build")
	s.router.StaticFile("/", "./headscale-ui/build/index.html")
	s.router.NoRoute(func(c *gin.Context) {
		c.File("./headscale-ui/build/index.html")
	})
}

// Start 启动服务器
func (s *UnlimitedControlServer) Start() error {
	// 初始化并启动LocalAPI服务器
	s.localAPIServer = NewLocalAPIServer(s)
	if err := s.localAPIServer.Start(); err != nil {
		log.Printf("Failed to start LocalAPI server: %v", err)
	}

	s.server = &http.Server{
		Addr:    s.listenAddr,
		Handler: s.router,
		TLSConfig: &tls.Config{
			MinVersion: tls.VersionTLS12,
		},
	}

	return s.server.ListenAndServe()
}

// Shutdown 关闭服务器
func (s *UnlimitedControlServer) Shutdown(ctx context.Context) error {
	// 停止LocalAPI服务器
	if s.localAPIServer != nil {
		s.localAPIServer.Stop()
	}

	// 停止告警引擎
	if s.alertEngine != nil {
		s.alertEngine.Stop()
	}

	return s.server.Shutdown(ctx)
}
