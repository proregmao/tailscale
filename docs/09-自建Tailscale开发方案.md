# 09-自建Tailscale无限制版开发方案

## 技术选型推荐

### 🎯 **核心技术栈**

#### 后端技术栈
```yaml
语言: Go 1.23+
框架: Gin + GORM
数据库: PostgreSQL 15+
缓存: Redis 7+
消息队列: NATS
认证: JWT + OAuth2
API风格: RESTful + WebSocket
```

#### 前端技术栈
```yaml
框架: React 18 + TypeScript
状态管理: Zustand
UI组件: Ant Design + Tailwind CSS
图表: ECharts + D3.js
实时通信: Socket.IO
构建工具: Vite
地图: Leaflet.js
```

#### 基础设施
```yaml
容器化: Docker + Docker Compose
反向代理: Nginx
监控: Prometheus + Grafana
日志: ELK Stack
CI/CD: GitHub Actions
```

### 🏗️ **架构设计**

```
┌─────────────────────────────────────────────────────────┐
│                    前端架构                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   设备管理  │  │   网络诊断  │  │   DERP管理  │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │   用户管理  │  │   ACL编辑   │  │   监控面板  │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                            │
                    ┌───────────────┐
                    │   API网关     │
                    └───────────────┘
                            │
┌─────────────────────────────────────────────────────────┐
│                    后端架构                              │
├─────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  控制平面   │  │  DERP服务   │  │  认证服务   │     │
│  │   服务      │  │    管理     │  │             │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────┐     │
│  │  设备管理   │  │  网络诊断   │  │  监控服务   │     │
│  │   服务      │  │    服务     │  │             │     │
│  └─────────────┘  └─────────────┘  └─────────────┘     │
└─────────────────────────────────────────────────────────┘
                            │
                    ┌───────────────┐
                    │   数据层      │
                    │ PostgreSQL    │
                    │   Redis       │
                    └───────────────┘
```

## 详细开发计划

### 📅 **开发阶段规划**

#### 第一阶段：基础架构 (4周)
- **Week 1**: 项目初始化和基础架构
- **Week 2**: 数据库设计和基础API
- **Week 3**: 认证系统和权限管理
- **Week 4**: 前端基础框架和路由

#### 第二阶段：核心功能 (6周)
- **Week 5-6**: 设备管理和用户管理
- **Week 7-8**: 控制平面核心逻辑
- **Week 9-10**: 前端核心页面开发

#### 第三阶段：高级功能 (6周)
- **Week 11-12**: DERP服务管理
- **Week 13-14**: 网络诊断工具
- **Week 15-16**: ACL可视化编辑器

#### 第四阶段：监控和优化 (4周)
- **Week 17-18**: 监控系统和仪表板
- **Week 19-20**: 性能优化和测试

## 第一阶段：基础架构开发

### Week 1: 项目初始化

#### 1.1 创建项目结构
```bash
mkdir tailscale-unlimited
cd tailscale-unlimited

# 后端项目结构
mkdir -p {cmd,internal,pkg,api,configs,scripts,deployments}
mkdir -p internal/{auth,device,user,derp,network,monitor}
mkdir -p pkg/{database,redis,logger,utils}

# 前端项目结构
mkdir -p web/{src,public,dist}
mkdir -p web/src/{components,pages,hooks,stores,utils,types}
```

#### 1.2 初始化Go模块
```go
// go.mod
module tailscale-unlimited

go 1.23

require (
    github.com/gin-gonic/gin v1.9.1
    github.com/golang-jwt/jwt/v5 v5.0.0
    gorm.io/gorm v1.25.5
    gorm.io/driver/postgres v1.5.4
    github.com/redis/go-redis/v9 v9.3.0
    github.com/gorilla/websocket v1.5.1
    github.com/spf13/viper v1.17.0
    github.com/sirupsen/logrus v1.9.3
)
```

#### 1.3 Docker环境配置
```yaml
# docker-compose.yml
version: '3.8'
services:
  postgres:
    image: postgres:15
    environment:
      POSTGRES_DB: tailscale_unlimited
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: password
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data

  redis:
    image: redis:7-alpine
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data

  backend:
    build: .
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      - DB_HOST=postgres
      - REDIS_HOST=redis

volumes:
  postgres_data:
  redis_data:
```

### Week 2: 数据库设计

#### 2.1 核心数据模型
```go
// internal/models/user.go
type User struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Email       string    `gorm:"uniqueIndex" json:"email"`
    Name        string    `json:"name"`
    Avatar      string    `json:"avatar"`
    Role        string    `json:"role"` // admin, user
    Status      string    `json:"status"` // active, inactive
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    
    // 关联
    Devices     []Device  `gorm:"foreignKey:UserID" json:"devices"`
    ACLRules    []ACLRule `gorm:"foreignKey:UserID" json:"acl_rules"`
}

// internal/models/device.go
type Device struct {
    ID              uint      `gorm:"primaryKey" json:"id"`
    UserID          uint      `json:"user_id"`
    NodeKey         string    `gorm:"uniqueIndex" json:"node_key"`
    MachineKey      string    `gorm:"uniqueIndex" json:"machine_key"`
    Hostname        string    `json:"hostname"`
    OS              string    `json:"os"`
    IPAddress       string    `json:"ip_address"`
    LastSeen        time.Time `json:"last_seen"`
    Online          bool      `json:"online"`
    Tags            []string  `gorm:"type:text[]" json:"tags"`
    AdvertiseRoutes []string  `gorm:"type:text[]" json:"advertise_routes"`
    ExitNode        bool      `json:"exit_node"`
    CreatedAt       time.Time `json:"created_at"`
    UpdatedAt       time.Time `json:"updated_at"`
    
    // 关联
    User            User      `gorm:"foreignKey:UserID" json:"user"`
}

// internal/models/derp.go
type DERPServer struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    Name        string    `json:"name"`
    RegionID    int       `gorm:"uniqueIndex" json:"region_id"`
    Hostname    string    `json:"hostname"`
    IPv4        string    `json:"ipv4"`
    IPv6        string    `json:"ipv6"`
    STUNPort    int       `json:"stun_port"`
    DERPPort    int       `json:"derp_port"`
    Enabled     bool      `json:"enabled"`
    Location    string    `json:"location"`
    Latitude    float64   `json:"latitude"`
    Longitude   float64   `json:"longitude"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
}

// internal/models/acl.go
type ACLRule struct {
    ID          uint      `gorm:"primaryKey" json:"id"`
    UserID      uint      `json:"user_id"`
    Name        string    `json:"name"`
    Action      string    `json:"action"` // accept, deny
    Sources     []string  `gorm:"type:text[]" json:"sources"`
    Destinations []string `gorm:"type:text[]" json:"destinations"`
    Ports       []string  `gorm:"type:text[]" json:"ports"`
    Protocols   []string  `gorm:"type:text[]" json:"protocols"`
    Priority    int       `json:"priority"`
    Enabled     bool      `json:"enabled"`
    CreatedAt   time.Time `json:"created_at"`
    UpdatedAt   time.Time `json:"updated_at"`
    
    // 关联
    User        User      `gorm:"foreignKey:UserID" json:"user"`
}
```

#### 2.2 数据库迁移
```go
// pkg/database/migrate.go
func AutoMigrate(db *gorm.DB) error {
    return db.AutoMigrate(
        &models.User{},
        &models.Device{},
        &models.DERPServer{},
        &models.ACLRule{},
        &models.NetworkMap{},
        &models.AuditLog{},
    )
}
```

### Week 3: 认证系统

#### 3.1 JWT认证实现
```go
// internal/auth/jwt.go
type JWTManager struct {
    secretKey string
    tokenDuration time.Duration
}

func NewJWTManager(secretKey string, tokenDuration time.Duration) *JWTManager {
    return &JWTManager{secretKey, tokenDuration}
}

func (manager *JWTManager) Generate(userID uint, email string, role string) (string, error) {
    claims := &Claims{
        UserID: userID,
        Email:  email,
        Role:   role,
        RegisteredClaims: jwt.RegisteredClaims{
            ExpiresAt: jwt.NewNumericDate(time.Now().Add(manager.tokenDuration)),
            IssuedAt:  jwt.NewNumericDate(time.Now()),
        },
    }

    token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
    return token.SignedString([]byte(manager.secretKey))
}

func (manager *JWTManager) Verify(accessToken string) (*Claims, error) {
    token, err := jwt.ParseWithClaims(
        accessToken,
        &Claims{},
        func(token *jwt.Token) (interface{}, error) {
            return []byte(manager.secretKey), nil
        },
    )

    if err != nil {
        return nil, err
    }

    claims, ok := token.Claims.(*Claims)
    if !ok {
        return nil, fmt.Errorf("invalid token claims")
    }

    return claims, nil
}
```

#### 3.2 OAuth2集成
```go
// internal/auth/oauth.go
type OAuthProvider struct {
    config *oauth2.Config
    state  string
}

func NewOAuthProvider(clientID, clientSecret, redirectURL string) *OAuthProvider {
    config := &oauth2.Config{
        ClientID:     clientID,
        ClientSecret: clientSecret,
        RedirectURL:  redirectURL,
        Scopes:       []string{"openid", "profile", "email"},
        Endpoint: oauth2.Endpoint{
            AuthURL:  "https://accounts.google.com/o/oauth2/auth",
            TokenURL: "https://oauth2.googleapis.com/token",
        },
    }

    return &OAuthProvider{
        config: config,
        state:  generateRandomState(),
    }
}
```

### Week 4: 前端基础框架

#### 4.1 React项目初始化
```bash
cd web
npm create vite@latest . -- --template react-ts
npm install

# 安装依赖
npm install @ant-design/icons antd
npm install zustand
npm install axios socket.io-client
npm install leaflet @types/leaflet
npm install echarts echarts-for-react
npm install tailwindcss
```

#### 4.2 基础组件和路由
```typescript
// src/App.tsx
import React from 'react';
import { BrowserRouter as Router, Routes, Route } from 'react-router-dom';
import { ConfigProvider } from 'antd';
import Layout from './components/Layout';
import Dashboard from './pages/Dashboard';
import Devices from './pages/Devices';
import Users from './pages/Users';
import DERP from './pages/DERP';
import ACL from './pages/ACL';
import Network from './pages/Network';
import Settings from './pages/Settings';

function App() {
  return (
    <ConfigProvider theme={{ token: { colorPrimary: '#1890ff' } }}>
      <Router>
        <Layout>
          <Routes>
            <Route path="/" element={<Dashboard />} />
            <Route path="/devices" element={<Devices />} />
            <Route path="/users" element={<Users />} />
            <Route path="/derp" element={<DERP />} />
            <Route path="/acl" element={<ACL />} />
            <Route path="/network" element={<Network />} />
            <Route path="/settings" element={<Settings />} />
          </Routes>
        </Layout>
      </Router>
    </ConfigProvider>
  );
}

export default App;
```

#### 4.3 状态管理
```typescript
// src/stores/authStore.ts
import { create } from 'zustand';

interface User {
  id: number;
  email: string;
  name: string;
  role: string;
}

interface AuthState {
  user: User | null;
  token: string | null;
  isAuthenticated: boolean;
  login: (email: string, password: string) => Promise<void>;
  logout: () => void;
  setUser: (user: User) => void;
}

export const useAuthStore = create<AuthState>((set, get) => ({
  user: null,
  token: localStorage.getItem('token'),
  isAuthenticated: !!localStorage.getItem('token'),
  
  login: async (email: string, password: string) => {
    try {
      const response = await fetch('/api/auth/login', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ email, password }),
      });
      
      const data = await response.json();
      
      if (response.ok) {
        localStorage.setItem('token', data.token);
        set({ 
          user: data.user, 
          token: data.token, 
          isAuthenticated: true 
        });
      }
    } catch (error) {
      console.error('Login failed:', error);
    }
  },
  
  logout: () => {
    localStorage.removeItem('token');
    set({ user: null, token: null, isAuthenticated: false });
  },
  
  setUser: (user: User) => set({ user }),
}));
```

## 开发进度跟踪

### 📊 **进度管理工具**

我建议使用以下工具来跟踪开发进度：

1. **项目管理**: GitHub Projects 或 Notion
2. **代码管理**: Git + GitHub
3. **文档管理**: 继续使用docs目录
4. **测试管理**: Jest + Cypress
5. **部署管理**: Docker + GitHub Actions

### 📝 **开发日志模板**

```markdown
## 开发日志 - Week X

### 本周目标
- [ ] 目标1
- [ ] 目标2
- [ ] 目标3

### 完成情况
- ✅ 已完成的任务
- 🔄 进行中的任务
- ❌ 遇到问题的任务

### 技术难点
1. 问题描述
2. 解决方案
3. 经验总结

### 下周计划
- 下周要完成的任务
- 需要重点关注的问题
```

## 第二阶段：核心功能开发

### Week 5-6: 设备管理和用户管理

#### 5.1 设备管理API实现
```go
// internal/device/service.go
type DeviceService struct {
    db    *gorm.DB
    redis *redis.Client
}

func (s *DeviceService) RegisterDevice(req *RegisterDeviceRequest) (*Device, error) {
    device := &Device{
        UserID:          req.UserID,
        NodeKey:         req.NodeKey,
        MachineKey:      req.MachineKey,
        Hostname:        req.Hostname,
        OS:              req.OS,
        IPAddress:       s.allocateIP(),
        Online:          true,
        AdvertiseRoutes: req.AdvertiseRoutes,
        ExitNode:        req.ExitNode,
    }

    if err := s.db.Create(device).Error; err != nil {
        return nil, err
    }

    // 更新网络映射
    s.updateNetworkMap(device.UserID)

    return device, nil
}

func (s *DeviceService) GetDevices(userID uint) ([]Device, error) {
    var devices []Device
    err := s.db.Where("user_id = ?", userID).Find(&devices).Error
    return devices, err
}

func (s *DeviceService) UpdateDeviceStatus(deviceID uint, online bool) error {
    return s.db.Model(&Device{}).Where("id = ?", deviceID).
        Updates(map[string]interface{}{
            "online":    online,
            "last_seen": time.Now(),
        }).Error
}
```

#### 5.2 前端设备管理页面
```typescript
// src/pages/Devices.tsx
import React, { useState, useEffect } from 'react';
import { Table, Button, Tag, Space, Modal, Form, Input, Switch } from 'antd';
import { PlusOutlined, EditOutlined, DeleteOutlined } from '@ant-design/icons';

interface Device {
  id: number;
  hostname: string;
  os: string;
  ip_address: string;
  online: boolean;
  last_seen: string;
  tags: string[];
  exit_node: boolean;
}

const Devices: React.FC = () => {
  const [devices, setDevices] = useState<Device[]>([]);
  const [loading, setLoading] = useState(false);
  const [modalVisible, setModalVisible] = useState(false);
  const [editingDevice, setEditingDevice] = useState<Device | null>(null);

  const columns = [
    {
      title: '设备名称',
      dataIndex: 'hostname',
      key: 'hostname',
    },
    {
      title: '操作系统',
      dataIndex: 'os',
      key: 'os',
    },
    {
      title: 'IP地址',
      dataIndex: 'ip_address',
      key: 'ip_address',
    },
    {
      title: '状态',
      dataIndex: 'online',
      key: 'online',
      render: (online: boolean) => (
        <Tag color={online ? 'green' : 'red'}>
          {online ? '在线' : '离线'}
        </Tag>
      ),
    },
    {
      title: '标签',
      dataIndex: 'tags',
      key: 'tags',
      render: (tags: string[]) => (
        <>
          {tags.map(tag => (
            <Tag key={tag}>{tag}</Tag>
          ))}
        </>
      ),
    },
    {
      title: '出口节点',
      dataIndex: 'exit_node',
      key: 'exit_node',
      render: (exitNode: boolean) => (
        <Switch checked={exitNode} disabled />
      ),
    },
    {
      title: '操作',
      key: 'action',
      render: (_, record: Device) => (
        <Space size="middle">
          <Button
            icon={<EditOutlined />}
            onClick={() => handleEdit(record)}
          >
            编辑
          </Button>
          <Button
            icon={<DeleteOutlined />}
            danger
            onClick={() => handleDelete(record.id)}
          >
            删除
          </Button>
        </Space>
      ),
    },
  ];

  const fetchDevices = async () => {
    setLoading(true);
    try {
      const response = await fetch('/api/devices');
      const data = await response.json();
      setDevices(data);
    } catch (error) {
      console.error('Failed to fetch devices:', error);
    } finally {
      setLoading(false);
    }
  };

  useEffect(() => {
    fetchDevices();
  }, []);

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button
          type="primary"
          icon={<PlusOutlined />}
          onClick={() => setModalVisible(true)}
        >
          添加设备
        </Button>
      </div>

      <Table
        columns={columns}
        dataSource={devices}
        loading={loading}
        rowKey="id"
      />
    </div>
  );
};

export default Devices;
```

### Week 7-8: 控制平面核心逻辑

#### 7.1 网络映射管理
```go
// internal/network/networkmap.go
type NetworkMapService struct {
    db    *gorm.DB
    redis *redis.Client
}

func (s *NetworkMapService) GenerateNetworkMap(userID uint) (*NetworkMap, error) {
    // 获取用户的所有设备
    var devices []Device
    if err := s.db.Where("user_id = ?", userID).Find(&devices).Error; err != nil {
        return nil, err
    }

    // 获取ACL规则
    var aclRules []ACLRule
    if err := s.db.Where("user_id = ? AND enabled = ?", userID, true).
        Order("priority ASC").Find(&aclRules).Error; err != nil {
        return nil, err
    }

    // 获取DERP服务器列表
    var derpServers []DERPServer
    if err := s.db.Where("enabled = ?", true).Find(&derpServers).Error; err != nil {
        return nil, err
    }

    networkMap := &NetworkMap{
        UserID:      userID,
        Devices:     devices,
        ACLRules:    aclRules,
        DERPServers: derpServers,
        UpdatedAt:   time.Now(),
    }

    // 缓存网络映射
    s.cacheNetworkMap(userID, networkMap)

    return networkMap, nil
}

func (s *NetworkMapService) UpdateNetworkMap(userID uint) error {
    networkMap, err := s.GenerateNetworkMap(userID)
    if err != nil {
        return err
    }

    // 通知所有相关设备更新网络映射
    return s.notifyDevices(userID, networkMap)
}
```

#### 7.2 实时通信实现
```go
// internal/websocket/hub.go
type Hub struct {
    clients    map[*Client]bool
    broadcast  chan []byte
    register   chan *Client
    unregister chan *Client
}

type Client struct {
    hub    *Hub
    conn   *websocket.Conn
    send   chan []byte
    userID uint
}

func (h *Hub) Run() {
    for {
        select {
        case client := <-h.register:
            h.clients[client] = true
            log.Printf("Client connected: %d", client.userID)

        case client := <-h.unregister:
            if _, ok := h.clients[client]; ok {
                delete(h.clients, client)
                close(client.send)
                log.Printf("Client disconnected: %d", client.userID)
            }

        case message := <-h.broadcast:
            for client := range h.clients {
                select {
                case client.send <- message:
                default:
                    close(client.send)
                    delete(h.clients, client)
                }
            }
        }
    }
}
```

### Week 9-10: 前端核心页面开发

#### 9.1 仪表板页面
```typescript
// src/pages/Dashboard.tsx
import React, { useState, useEffect } from 'react';
import { Row, Col, Card, Statistic, Progress, List, Tag } from 'antd';
import { UserOutlined, LaptopOutlined, GlobalOutlined, SafetyOutlined } from '@ant-design/icons';
import ReactECharts from 'echarts-for-react';

interface DashboardStats {
  totalDevices: number;
  onlineDevices: number;
  totalUsers: number;
  activeConnections: number;
  networkTraffic: {
    upload: number;
    download: number;
  };
}

const Dashboard: React.FC = () => {
  const [stats, setStats] = useState<DashboardStats | null>(null);
  const [recentActivity, setRecentActivity] = useState([]);

  const trafficOption = {
    title: { text: '网络流量' },
    tooltip: { trigger: 'axis' },
    xAxis: { type: 'category', data: ['00:00', '04:00', '08:00', '12:00', '16:00', '20:00'] },
    yAxis: { type: 'value' },
    series: [
      {
        name: '上传',
        type: 'line',
        data: [120, 132, 101, 134, 90, 230],
      },
      {
        name: '下载',
        type: 'line',
        data: [220, 182, 191, 234, 290, 330],
      },
    ],
  };

  useEffect(() => {
    // 获取仪表板数据
    fetchDashboardData();
  }, []);

  const fetchDashboardData = async () => {
    try {
      const response = await fetch('/api/dashboard/stats');
      const data = await response.json();
      setStats(data);
    } catch (error) {
      console.error('Failed to fetch dashboard data:', error);
    }
  };

  if (!stats) return <div>Loading...</div>;

  return (
    <div>
      <Row gutter={16} style={{ marginBottom: 24 }}>
        <Col span={6}>
          <Card>
            <Statistic
              title="总设备数"
              value={stats.totalDevices}
              prefix={<LaptopOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="在线设备"
              value={stats.onlineDevices}
              prefix={<GlobalOutlined />}
              valueStyle={{ color: '#3f8600' }}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="用户数量"
              value={stats.totalUsers}
              prefix={<UserOutlined />}
            />
          </Card>
        </Col>
        <Col span={6}>
          <Card>
            <Statistic
              title="活跃连接"
              value={stats.activeConnections}
              prefix={<SafetyOutlined />}
            />
          </Card>
        </Col>
      </Row>

      <Row gutter={16}>
        <Col span={16}>
          <Card title="网络流量监控">
            <ReactECharts option={trafficOption} style={{ height: 400 }} />
          </Card>
        </Col>
        <Col span={8}>
          <Card title="最近活动">
            <List
              dataSource={recentActivity}
              renderItem={(item: any) => (
                <List.Item>
                  <List.Item.Meta
                    title={item.title}
                    description={item.description}
                  />
                  <Tag color={item.type === 'success' ? 'green' : 'blue'}>
                    {item.time}
                  </Tag>
                </List.Item>
              )}
            />
          </Card>
        </Col>
      </Row>
    </div>
  );
};

export default Dashboard;
```

## 第三阶段：高级功能开发

### Week 11-12: DERP服务管理

#### 11.1 DERP服务器管理API
```go
// internal/derp/service.go
type DERPService struct {
    db    *gorm.DB
    redis *redis.Client
}

func (s *DERPService) AddDERPServer(req *AddDERPServerRequest) (*DERPServer, error) {
    server := &DERPServer{
        Name:      req.Name,
        RegionID:  req.RegionID,
        Hostname:  req.Hostname,
        IPv4:      req.IPv4,
        IPv6:      req.IPv6,
        STUNPort:  req.STUNPort,
        DERPPort:  req.DERPPort,
        Enabled:   true,
        Location:  req.Location,
        Latitude:  req.Latitude,
        Longitude: req.Longitude,
    }

    if err := s.db.Create(server).Error; err != nil {
        return nil, err
    }

    // 测试DERP服务器连接
    if err := s.testDERPServer(server); err != nil {
        log.Printf("DERP server test failed: %v", err)
    }

    // 更新DERP映射
    s.updateDERPMap()

    return server, nil
}

func (s *DERPService) testDERPServer(server *DERPServer) error {
    // 实现DERP服务器连接测试
    conn, err := net.DialTimeout("tcp",
        fmt.Sprintf("%s:%d", server.Hostname, server.DERPPort),
        5*time.Second)
    if err != nil {
        return err
    }
    defer conn.Close()

    return nil
}
```

#### 11.2 DERP管理前端页面
```typescript
// src/pages/DERP.tsx
import React, { useState, useEffect } from 'react';
import { Table, Button, Modal, Form, Input, InputNumber, Switch, message } from 'antd';
import { MapContainer, TileLayer, Marker, Popup } from 'react-leaflet';
import 'leaflet/dist/leaflet.css';

interface DERPServer {
  id: number;
  name: string;
  region_id: number;
  hostname: string;
  ipv4: string;
  location: string;
  enabled: boolean;
  latitude: number;
  longitude: number;
  status: 'online' | 'offline' | 'testing';
}

const DERP: React.FC = () => {
  const [servers, setServers] = useState<DERPServer[]>([]);
  const [modalVisible, setModalVisible] = useState(false);
  const [form] = Form.useForm();

  const columns = [
    { title: '名称', dataIndex: 'name', key: 'name' },
    { title: '区域ID', dataIndex: 'region_id', key: 'region_id' },
    { title: '主机名', dataIndex: 'hostname', key: 'hostname' },
    { title: 'IPv4', dataIndex: 'ipv4', key: 'ipv4' },
    { title: '位置', dataIndex: 'location', key: 'location' },
    {
      title: '状态',
      dataIndex: 'status',
      key: 'status',
      render: (status: string) => (
        <span style={{
          color: status === 'online' ? 'green' :
                 status === 'offline' ? 'red' : 'orange'
        }}>
          {status === 'online' ? '在线' :
           status === 'offline' ? '离线' : '测试中'}
        </span>
      ),
    },
    {
      title: '启用',
      dataIndex: 'enabled',
      key: 'enabled',
      render: (enabled: boolean, record: DERPServer) => (
        <Switch
          checked={enabled}
          onChange={(checked) => handleToggleServer(record.id, checked)}
        />
      ),
    },
  ];

  const handleAddServer = async (values: any) => {
    try {
      const response = await fetch('/api/derp/servers', {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify(values),
      });

      if (response.ok) {
        message.success('DERP服务器添加成功');
        setModalVisible(false);
        form.resetFields();
        fetchServers();
      }
    } catch (error) {
      message.error('添加失败');
    }
  };

  return (
    <div>
      <div style={{ marginBottom: 16 }}>
        <Button type="primary" onClick={() => setModalVisible(true)}>
          添加DERP服务器
        </Button>
      </div>

      <Table columns={columns} dataSource={servers} rowKey="id" />

      <div style={{ marginTop: 24, height: 400 }}>
        <MapContainer center={[39.9042, 116.4074]} zoom={2} style={{ height: '100%' }}>
          <TileLayer url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png" />
          {servers.map(server => (
            <Marker key={server.id} position={[server.latitude, server.longitude]}>
              <Popup>
                <div>
                  <h4>{server.name}</h4>
                  <p>位置: {server.location}</p>
                  <p>状态: {server.status}</p>
                </div>
              </Popup>
            </Marker>
          ))}
        </MapContainer>
      </div>

      <Modal
        title="添加DERP服务器"
        open={modalVisible}
        onCancel={() => setModalVisible(false)}
        footer={null}
      >
        <Form form={form} onFinish={handleAddServer} layout="vertical">
          <Form.Item name="name" label="服务器名称" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="region_id" label="区域ID" rules={[{ required: true }]}>
            <InputNumber min={1} />
          </Form.Item>
          <Form.Item name="hostname" label="主机名" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="ipv4" label="IPv4地址" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="location" label="地理位置" rules={[{ required: true }]}>
            <Input />
          </Form.Item>
          <Form.Item name="latitude" label="纬度" rules={[{ required: true }]}>
            <InputNumber step={0.000001} />
          </Form.Item>
          <Form.Item name="longitude" label="经度" rules={[{ required: true }]}>
            <InputNumber step={0.000001} />
          </Form.Item>
          <Form.Item>
            <Button type="primary" htmlType="submit">
              添加
            </Button>
          </Form.Item>
        </Form>
      </Modal>
    </div>
  );
};

export default DERP;
```

这个开发方案为您提供了一个完整的技术路线图和详细的实现步骤。您觉得这个方案如何？需要我详细展开某个特定阶段的实现细节吗？
