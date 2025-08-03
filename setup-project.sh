#!/bin/bash

# Tailscale Unlimited 项目初始化脚本
# 用于快速搭建开发环境

set -e

echo "🚀 开始初始化 Tailscale Unlimited 项目..."

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 打印带颜色的消息
print_status() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

print_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

print_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

print_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 检查必要的工具
check_requirements() {
    print_status "检查系统要求..."
    
    # 检查Docker
    if ! command -v docker &> /dev/null; then
        print_error "Docker 未安装，请先安装 Docker"
        exit 1
    fi
    
    # 检查Docker Compose
    if ! command -v docker-compose &> /dev/null; then
        print_error "Docker Compose 未安装，请先安装 Docker Compose"
        exit 1
    fi
    
    # 检查Go
    if ! command -v go &> /dev/null; then
        print_error "Go 未安装，请先安装 Go 1.23+"
        exit 1
    fi
    
    # 检查Node.js
    if ! command -v node &> /dev/null; then
        print_error "Node.js 未安装，请先安装 Node.js 18+"
        exit 1
    fi
    
    # 检查Git
    if ! command -v git &> /dev/null; then
        print_error "Git 未安装，请先安装 Git"
        exit 1
    fi
    
    print_success "系统要求检查通过"
}

# 创建项目目录结构
create_project_structure() {
    print_status "创建项目目录结构..."
    
    # 创建主目录
    PROJECT_NAME="tailscale-unlimited"
    
    if [ -d "$PROJECT_NAME" ]; then
        print_warning "项目目录已存在，是否继续？(y/n)"
        read -r response
        if [[ ! "$response" =~ ^[Yy]$ ]]; then
            exit 0
        fi
    fi
    
    mkdir -p $PROJECT_NAME
    cd $PROJECT_NAME
    
    # 后端目录结构
    mkdir -p {cmd,internal,pkg,api,configs,scripts,deployments,tests}
    mkdir -p internal/{auth,device,user,derp,network,monitor,websocket}
    mkdir -p pkg/{database,redis,logger,utils,middleware}
    mkdir -p cmd/{server,cli,derp-server}
    mkdir -p api/{v1,docs}
    mkdir -p configs/{dev,prod,test}
    mkdir -p deployments/{docker,k8s,scripts}
    mkdir -p tests/{unit,integration,e2e}
    
    # 前端目录结构
    mkdir -p web/{src,public,dist,tests}
    mkdir -p web/src/{components,pages,hooks,stores,utils,types,services}
    mkdir -p web/src/components/{common,layout,forms,charts}
    mkdir -p web/src/pages/{dashboard,devices,users,derp,acl,network,settings}
    
    print_success "项目目录结构创建完成"
}

# 初始化Go模块
init_go_module() {
    print_status "初始化Go模块..."
    
    # 创建go.mod
    cat > go.mod << 'EOF'
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
    github.com/stretchr/testify v1.8.4
    github.com/swaggo/gin-swagger v1.6.0
    github.com/swaggo/files v1.0.1
    github.com/swaggo/swag v1.16.2
)
EOF
    
    # 创建main.go
    cat > cmd/server/main.go << 'EOF'
package main

import (
    "log"
    "tailscale-unlimited/internal/server"
)

func main() {
    srv := server.NewServer()
    if err := srv.Run(); err != nil {
        log.Fatal("Server failed to start:", err)
    }
}
EOF
    
    print_success "Go模块初始化完成"
}

# 创建Docker配置
create_docker_config() {
    print_status "创建Docker配置..."
    
    # 创建Dockerfile
    cat > Dockerfile << 'EOF'
# 构建阶段
FROM golang:1.23-alpine AS builder

WORKDIR /app
COPY go.mod go.sum ./
RUN go mod download

COPY . .
RUN CGO_ENABLED=0 GOOS=linux go build -o main cmd/server/main.go

# 运行阶段
FROM alpine:latest

RUN apk --no-cache add ca-certificates
WORKDIR /root/

COPY --from=builder /app/main .
COPY --from=builder /app/configs ./configs

EXPOSE 8080

CMD ["./main"]
EOF
    
    # 创建docker-compose.yml
    cat > docker-compose.yml << 'EOF'
version: '3.8'

services:
  postgres:
    image: postgres:15
    container_name: tailscale-postgres
    environment:
      POSTGRES_DB: tailscale_unlimited
      POSTGRES_USER: admin
      POSTGRES_PASSWORD: password123
    ports:
      - "5432:5432"
    volumes:
      - postgres_data:/var/lib/postgresql/data
      - ./scripts/init.sql:/docker-entrypoint-initdb.d/init.sql
    networks:
      - tailscale-network

  redis:
    image: redis:7-alpine
    container_name: tailscale-redis
    ports:
      - "6379:6379"
    volumes:
      - redis_data:/data
    networks:
      - tailscale-network

  backend:
    build: .
    container_name: tailscale-backend
    ports:
      - "8080:8080"
    depends_on:
      - postgres
      - redis
    environment:
      - DB_HOST=postgres
      - DB_PORT=5432
      - DB_USER=admin
      - DB_PASSWORD=password123
      - DB_NAME=tailscale_unlimited
      - REDIS_HOST=redis
      - REDIS_PORT=6379
      - JWT_SECRET=your-super-secret-jwt-key
      - GIN_MODE=debug
    volumes:
      - ./configs:/app/configs
      - ./logs:/app/logs
    networks:
      - tailscale-network

  frontend:
    image: node:18-alpine
    container_name: tailscale-frontend
    working_dir: /app
    ports:
      - "3000:3000"
    volumes:
      - ./web:/app
    command: sh -c "npm install && npm run dev"
    networks:
      - tailscale-network

volumes:
  postgres_data:
  redis_data:

networks:
  tailscale-network:
    driver: bridge
EOF
    
    # 创建.dockerignore
    cat > .dockerignore << 'EOF'
.git
.gitignore
README.md
Dockerfile
docker-compose.yml
.env
.env.local
node_modules
web/node_modules
web/dist
logs
*.log
EOF
    
    print_success "Docker配置创建完成"
}

# 初始化前端项目
init_frontend() {
    print_status "初始化前端项目..."
    
    cd web
    
    # 创建package.json
    cat > package.json << 'EOF'
{
  "name": "tailscale-unlimited-web",
  "version": "1.0.0",
  "type": "module",
  "scripts": {
    "dev": "vite --host 0.0.0.0",
    "build": "tsc && vite build",
    "preview": "vite preview",
    "test": "vitest",
    "lint": "eslint . --ext ts,tsx --report-unused-disable-directives --max-warnings 0"
  },
  "dependencies": {
    "react": "^18.2.0",
    "react-dom": "^18.2.0",
    "react-router-dom": "^6.8.0",
    "antd": "^5.12.0",
    "@ant-design/icons": "^5.2.0",
    "zustand": "^4.4.0",
    "axios": "^1.6.0",
    "socket.io-client": "^4.7.0",
    "leaflet": "^1.9.0",
    "echarts": "^5.4.0",
    "echarts-for-react": "^3.0.0",
    "dayjs": "^1.11.0"
  },
  "devDependencies": {
    "@types/react": "^18.2.0",
    "@types/react-dom": "^18.2.0",
    "@types/leaflet": "^1.9.0",
    "@typescript-eslint/eslint-plugin": "^6.0.0",
    "@typescript-eslint/parser": "^6.0.0",
    "@vitejs/plugin-react": "^4.0.0",
    "eslint": "^8.45.0",
    "eslint-plugin-react-hooks": "^4.6.0",
    "eslint-plugin-react-refresh": "^0.4.0",
    "typescript": "^5.0.2",
    "vite": "^4.4.0",
    "vitest": "^0.34.0"
  }
}
EOF
    
    # 创建vite.config.ts
    cat > vite.config.ts << 'EOF'
import { defineConfig } from 'vite'
import react from '@vitejs/plugin-react'

export default defineConfig({
  plugins: [react()],
  server: {
    host: '0.0.0.0',
    port: 3000,
    proxy: {
      '/api': {
        target: 'http://backend:8080',
        changeOrigin: true,
      },
      '/ws': {
        target: 'ws://backend:8080',
        ws: true,
      },
    },
  },
  build: {
    outDir: 'dist',
    sourcemap: true,
  },
})
EOF
    
    # 创建基础的index.html
    cat > index.html << 'EOF'
<!DOCTYPE html>
<html lang="zh-CN">
  <head>
    <meta charset="UTF-8" />
    <link rel="icon" type="image/svg+xml" href="/vite.svg" />
    <meta name="viewport" content="width=device-width, initial-scale=1.0" />
    <title>Tailscale Unlimited</title>
  </head>
  <body>
    <div id="root"></div>
    <script type="module" src="/src/main.tsx"></script>
  </body>
</html>
EOF
    
    cd ..
    print_success "前端项目初始化完成"
}

# 创建配置文件
create_config_files() {
    print_status "创建配置文件..."
    
    # 创建开发环境配置
    cat > configs/dev/config.yaml << 'EOF'
server:
  port: 8080
  mode: debug

database:
  host: localhost
  port: 5432
  user: admin
  password: password123
  dbname: tailscale_unlimited
  sslmode: disable

redis:
  host: localhost
  port: 6379
  password: ""
  db: 0

jwt:
  secret: your-super-secret-jwt-key
  expire: 24h

oauth:
  google:
    client_id: ""
    client_secret: ""
    redirect_url: "http://localhost:8080/auth/google/callback"

derp:
  stun_port: 3478
  derp_port: 443
  
logging:
  level: debug
  file: logs/app.log
EOF
    
    # 创建.env.example
    cat > .env.example << 'EOF'
# 数据库配置
DB_HOST=localhost
DB_PORT=5432
DB_USER=admin
DB_PASSWORD=password123
DB_NAME=tailscale_unlimited

# Redis配置
REDIS_HOST=localhost
REDIS_PORT=6379

# JWT配置
JWT_SECRET=your-super-secret-jwt-key

# OAuth配置
GOOGLE_CLIENT_ID=your-google-client-id
GOOGLE_CLIENT_SECRET=your-google-client-secret

# 服务器配置
SERVER_PORT=8080
GIN_MODE=debug
EOF
    
    print_success "配置文件创建完成"
}

# 创建README文件
create_readme() {
    print_status "创建README文件..."
    
    cat > README.md << 'EOF'
# Tailscale Unlimited

基于Tailscale的无限制私有网络解决方案

## 功能特性

- 🚀 无设备数量限制
- 👥 无用户数量限制  
- 🔧 自定义DERP服务器
- 🎛️ 可视化管理界面
- 📊 实时监控和诊断
- 🔒 企业级安全控制

## 快速开始

### 环境要求

- Docker & Docker Compose
- Go 1.23+
- Node.js 18+
- Git

### 启动开发环境

```bash
# 克隆项目
git clone <your-repo-url>
cd tailscale-unlimited

# 启动服务
docker-compose up -d

# 查看日志
docker-compose logs -f
```

### 访问地址

- 前端界面: http://localhost:3000
- 后端API: http://localhost:8080
- API文档: http://localhost:8080/swagger/index.html

## 开发指南

详细的开发指南请参考 [docs/08-开发指南.md](docs/08-开发指南.md)

## 项目结构

```
tailscale-unlimited/
├── cmd/                    # 命令行工具
├── internal/               # 内部包
├── pkg/                    # 公共包
├── web/                    # 前端代码
├── configs/                # 配置文件
├── deployments/            # 部署文件
├── docs/                   # 文档
└── tests/                  # 测试文件
```

## 贡献指南

1. Fork 项目
2. 创建功能分支
3. 提交更改
4. 推送到分支
5. 创建 Pull Request

## 许可证

MIT License
EOF
    
    print_success "README文件创建完成"
}

# 创建Git配置
init_git() {
    print_status "初始化Git仓库..."
    
    # 创建.gitignore
    cat > .gitignore << 'EOF'
# Go
*.exe
*.exe~
*.dll
*.so
*.dylib
*.test
*.out
go.work

# 依赖
vendor/

# IDE
.vscode/
.idea/
*.swp
*.swo

# 日志
*.log
logs/

# 环境变量
.env
.env.local

# 数据库
*.db
*.sqlite

# 前端
web/node_modules/
web/dist/
web/.vite/

# Docker
.docker/

# 临时文件
tmp/
temp/

# 操作系统
.DS_Store
Thumbs.db
EOF
    
    # 初始化Git仓库
    git init
    git add .
    git commit -m "Initial commit: Project setup"
    
    print_success "Git仓库初始化完成"
}

# 主函数
main() {
    echo "🎯 Tailscale Unlimited 项目初始化"
    echo "=================================="
    
    check_requirements
    create_project_structure
    init_go_module
    create_docker_config
    init_frontend
    create_config_files
    create_readme
    init_git
    
    echo ""
    echo "🎉 项目初始化完成！"
    echo ""
    echo "下一步操作："
    echo "1. cd tailscale-unlimited"
    echo "2. 复制 .env.example 到 .env 并配置"
    echo "3. docker-compose up -d"
    echo "4. 访问 http://localhost:3000"
    echo ""
    echo "📚 更多信息请查看 docs/ 目录下的文档"
}

# 运行主函数
main "$@"
