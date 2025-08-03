#!/bin/bash

# 全面测试所有页面和功能的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 检测前端端口
FRONTEND_PORT=""
for port in 5174 5173 5175 8082 8083; do
    if curl -s "http://localhost:$port" > /dev/null 2>&1; then
        FRONTEND_PORT=$port
        break
    fi
done

if [ -z "$FRONTEND_PORT" ]; then
    echo -e "${RED}❌ 无法检测到前端服务端口${NC}"
    exit 1
fi

echo -e "${BLUE}🧪 开始全面测试 - 前端端口: $FRONTEND_PORT${NC}"
echo -e "${BLUE}================================================${NC}"

# 测试页面列表
declare -A pages=(
    ["仪表板"]="/"
    ["设备管理"]="/machines.html"
    ["用户管理"]="/users.html"
    ["DERP管理"]="/derp.html"
    ["ACL管理"]="/acl.html"
    ["路由管理"]="/routes.html"
    ["DNS管理"]="/dns.html"
    ["认证管理"]="/auth.html"
    ["SSH管理"]="/ssh.html"
    ["服务暴露"]="/serve.html"
    ["Kubernetes"]="/k8s.html"
    ["密钥轮换"]="/key-rotation.html"
    ["SDK集成"]="/sdk.html"
    ["Taildrop"]="/taildrop.html"
    ["告警管理"]="/alerts.html"
    ["日志管理"]="/logs.html"
    ["报表管理"]="/reports.html"
    ["系统设置"]="/settings.html"
)

# 测试API接口列表
declare -A apis=(
    ["健康检查"]="/api/v1/health"
    ["设备列表"]="/api/v1/machine"
    ["用户列表"]="/api/v1/user"
    ["DERP状态"]="/api/v1/derp/status"
    ["ACL规则"]="/api/v1/acl"
    ["路由列表"]="/api/v1/routes"
    ["DNS配置"]="/api/v1/dns"
    ["认证配置"]="/api/v1/auth"
    ["SSH配置"]="/api/v1/ssh"
    ["服务配置"]="/api/v1/serve"
    ["告警规则"]="/api/v1/alerts/rules"
    ["系统信息"]="/api/v1/system/info"
)

# 测试函数
test_page() {
    local name=$1
    local path=$2
    local url="http://localhost:$FRONTEND_PORT$path"
    
    echo -n "测试 $name ... "
    
    # 使用 curl 测试页面
    if curl -s -f "$url" > /dev/null 2>&1; then
        echo -e "${GREEN}✅ 通过${NC}"
        return 0
    else
        echo -e "${RED}❌ 失败${NC}"
        return 1
    fi
}

test_api() {
    local name=$1
    local path=$2
    local url="http://localhost:8080$path"
    
    echo -n "测试 $name API ... "
    
    # 使用 curl 测试API
    local response=$(curl -s -w "%{http_code}" "$url" 2>/dev/null)
    local http_code="${response: -3}"
    
    if [[ "$http_code" =~ ^[2-3][0-9][0-9]$ ]]; then
        echo -e "${GREEN}✅ 通过 ($http_code)${NC}"
        return 0
    else
        echo -e "${RED}❌ 失败 ($http_code)${NC}"
        return 1
    fi
}

# 开始测试页面
echo -e "${YELLOW}📄 测试前端页面...${NC}"
page_success=0
page_total=0

for page_name in "${!pages[@]}"; do
    page_path="${pages[$page_name]}"
    if test_page "$page_name" "$page_path"; then
        ((page_success++))
    fi
    ((page_total++))
done

echo ""
echo -e "${BLUE}📄 页面测试结果: $page_success/$page_total 通过${NC}"

# 开始测试API
echo -e "${YELLOW}🔌 测试后端API...${NC}"
api_success=0
api_total=0

for api_name in "${!apis[@]}"; do
    api_path="${apis[$api_name]}"
    if test_api "$api_name" "$api_path"; then
        ((api_success++))
    fi
    ((api_total++))
done

echo ""
echo -e "${BLUE}🔌 API测试结果: $api_success/$api_total 通过${NC}"

# 测试特定功能
echo -e "${YELLOW}⚙️  测试特定功能...${NC}"

# 测试WebSocket连接
echo -n "测试 WebSocket 连接 ... "
if timeout 5 bash -c "echo 'test' | websocat ws://localhost:8080/api/v1/logs/stream" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 通过${NC}"
    ws_success=1
else
    echo -e "${RED}❌ 失败${NC}"
    ws_success=0
fi

# 测试静态资源
echo -n "测试 静态资源 ... "
if curl -s -f "http://localhost:$FRONTEND_PORT/favicon.png" > /dev/null 2>&1; then
    echo -e "${GREEN}✅ 通过${NC}"
    static_success=1
else
    echo -e "${RED}❌ 失败${NC}"
    static_success=0
fi

# 计算总体成功率
total_tests=$((page_total + api_total + 2))
total_success=$((page_success + api_success + ws_success + static_success))
success_rate=$((total_success * 100 / total_tests))

echo ""
echo -e "${BLUE}================================================${NC}"
echo -e "${BLUE}📊 总体测试结果${NC}"
echo -e "${BLUE}================================================${NC}"
echo -e "📄 前端页面: $page_success/$page_total 通过"
echo -e "🔌 后端API: $api_success/$api_total 通过"
echo -e "⚙️  特定功能: $((ws_success + static_success))/2 通过"
echo -e "${BLUE}总计: $total_success/$total_tests 通过 (成功率: $success_rate%)${NC}"

if [ $success_rate -ge 80 ]; then
    echo -e "${GREEN}🎉 测试结果良好！${NC}"
    exit 0
elif [ $success_rate -ge 60 ]; then
    echo -e "${YELLOW}⚠️  测试结果一般，需要改进${NC}"
    exit 1
else
    echo -e "${RED}❌ 测试结果较差，需要修复${NC}"
    exit 1
fi
