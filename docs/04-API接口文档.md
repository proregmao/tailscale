# 04-Tailscale API接口文档

## API概述

Tailscale提供了多层次的API接口，包括控制平面API、本地API和内部API，支持各种管理和集成需求。

## 控制平面API

### 1. 基础信息
- **基础URL**: `https://api.tailscale.com`
- **API版本**: v2
- **认证方式**: API Key (HTTP Basic Auth)
- **数据格式**: JSON

### 2. 认证
```http
Authorization: Basic <base64(api_key:)>
```

### 3. 主要端点

#### 设备管理
```http
# 获取设备列表
GET /api/v2/tailnet/{tailnet}/devices

# 获取设备详情
GET /api/v2/tailnet/{tailnet}/devices/{deviceId}

# 删除设备
DELETE /api/v2/tailnet/{tailnet}/devices/{deviceId}

# 更新设备
POST /api/v2/tailnet/{tailnet}/devices/{deviceId}
```

#### 用户管理
```http
# 获取用户列表
GET /api/v2/tailnet/{tailnet}/users

# 获取用户详情
GET /api/v2/tailnet/{tailnet}/users/{userId}
```

#### DNS配置
```http
# 获取DNS配置
GET /api/v2/tailnet/{tailnet}/dns

# 更新DNS配置
POST /api/v2/tailnet/{tailnet}/dns
```

#### ACL管理
```http
# 获取ACL
GET /api/v2/tailnet/{tailnet}/acl

# 更新ACL
POST /api/v2/tailnet/{tailnet}/acl

# 验证ACL
POST /api/v2/tailnet/{tailnet}/acl/validate
```

#### 密钥管理
```http
# 创建认证密钥
POST /api/v2/tailnet/{tailnet}/keys

# 获取密钥列表
GET /api/v2/tailnet/{tailnet}/keys

# 删除密钥
DELETE /api/v2/tailnet/{tailnet}/keys/{keyId}
```

## 本地API (LocalAPI)

### 1. 基础信息
- **基础URL**: `http://local-tailscaled.sock`
- **传输方式**: Unix Socket 或 HTTP
- **认证**: 基于进程权限

### 2. 主要端点

#### 状态查询
```http
# 获取状态
GET /localapi/v0/status

# 获取详细状态
GET /localapi/v0/status?peers=true

# 获取健康状态
GET /localapi/v0/health
```

#### 网络操作
```http
# 启动Tailscale
POST /localapi/v0/up

# 停止Tailscale
POST /localapi/v0/down

# 登录
POST /localapi/v0/login

# 登出
POST /localapi/v0/logout
```

#### 配置管理
```http
# 获取配置
GET /localapi/v0/prefs

# 更新配置
PATCH /localapi/v0/prefs

# 重新加载配置
POST /localapi/v0/reload-config
```

#### 网络诊断
```http
# Ping测试
POST /localapi/v0/ping

# 网络检查
GET /localapi/v0/netcheck

# WhoIs查询
GET /localapi/v0/whois?addr={ip}
```

#### 文件操作 (Taildrop)
```http
# 获取文件目标
GET /localapi/v0/file-targets

# 发送文件
PUT /localapi/v0/file-put/{target}/{filename}

# 获取等待文件
GET /localapi/v0/files/
```

#### 证书管理
```http
# 获取证书
GET /localapi/v0/cert/{domain}

# 获取证书状态
GET /localapi/v0/cert/{domain}/status
```

## Web客户端API

### 1. 认证相关
```typescript
// 登录
POST /api/up
{
  "Reauthenticate"?: boolean,
  "ControlURL"?: string,
  "AuthKey"?: string
}

// 登出
POST /api/logout

// 新建认证会话
GET /api/auth/session/new
```

### 2. 配置管理
```typescript
// 更新偏好设置
PATCH /api/local/v0/prefs
{
  "RunSSHSet"?: boolean,
  "RunSSH"?: boolean
}

// 更新路由
POST /api/routes
{
  routes: SubnetRoute[]
}

// 更新出口节点
POST /api/exit-node
{
  exitNode: ExitNode
}
```

## 内部客户端API

### 1. VIP服务
```go
// 获取VIP服务
func (client *Client) GetVIPService(ctx context.Context, name tailcfg.ServiceName) (*VIPService, error)

// 列出VIP服务
func (client *Client) ListVIPServices(ctx context.Context) (*VIPServiceList, error)
```

### 2. Tailnet管理
```go
// 删除Tailnet
func (c *Client) TailnetDeleteRequest(ctx context.Context, tailnetID string) error
```

## 日志服务API

### 1. 基础信息
- **基础URL**: `https://log.tailscale.com`
- **认证**: API Key (HTTP Basic Auth)

### 2. 主要端点
```http
# 查询集合
GET /collections?collection-name={name}

# 上传日志
POST /c/{collection}/{instance}
Content-Type: application/json
Content-Encoding: zstd (可选)
```

## Kubernetes API

### 1. 自定义资源
```yaml
# Connector资源
apiVersion: tailscale.com/v1alpha1
kind: Connector

# ProxyClass资源
apiVersion: tailscale.com/v1alpha1
kind: ProxyClass

# DNSConfig资源
apiVersion: tailscale.com/v1alpha1
kind: DNSConfig
```

### 2. 操作器API
- 自动管理Tailscale代理
- 服务暴露和负载均衡
- 网络策略集成

## 错误处理

### 1. HTTP状态码
- `200 OK`: 请求成功
- `400 Bad Request`: 请求参数错误
- `401 Unauthorized`: 认证失败
- `403 Forbidden`: 权限不足
- `404 Not Found`: 资源不存在
- `429 Too Many Requests`: 请求频率限制
- `500 Internal Server Error`: 服务器内部错误

### 2. 错误响应格式
```json
{
  "message": "错误描述",
  "data": [
    {
      "user": "用户信息",
      "errors": ["具体错误列表"]
    }
  ]
}
```

## 使用示例

### 1. Go客户端示例
```go
import "tailscale.com/client/tailscale"

client := tailscale.NewClient("example.com", tailscale.APIKey("your-api-key"))

// 获取设备列表
devices, err := client.Devices(ctx)
if err != nil {
    log.Fatal(err)
}
```

### 2. curl示例
```bash
# 获取设备列表
curl -u "your-api-key:" \
  https://api.tailscale.com/api/v2/tailnet/example.com/devices

# 本地状态查询
curl http://local-tailscaled.sock/localapi/v0/status
```

### 3. JavaScript示例
```javascript
// Web客户端API调用
const response = await fetch('/api/status', {
  method: 'GET',
  headers: {
    'Content-Type': 'application/json'
  }
});
const status = await response.json();
```

## 限制和配额

### 1. 速率限制
- API调用频率限制
- 并发连接数限制
- 数据传输量限制

### 2. 资源限制
- 设备数量限制
- 用户数量限制
- ACL规则数量限制

### 3. 功能限制
- 免费版功能限制
- 企业版高级功能
- API访问权限控制
