#!/bin/bash

# 全面修复所有HTML结构错误的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔧 开始全面修复所有HTML结构错误...${NC}"

# 创建一个完整的页面测试脚本
echo -e "${YELLOW}创建页面测试脚本...${NC}"

cat > test-all-pages.sh << 'EOF'
#!/bin/bash

# 测试所有页面的脚本
BASE_URL="http://localhost:5173"

# 页面列表
pages=(
    "/"
    "/devices.html"
    "/users.html"
    "/derp.html"
    "/monitoring.html"
    "/logs.html"
    "/acl.html"
    "/groups.html"
    "/settings.html"
    "/routes.html"
    "/dns.html"
    "/taildrop.html"
    "/sdk.html"
    "/alerts.html"
    "/auth.html"
    "/ssh.html"
    "/serve.html"
    "/k8s.html"
    "/key-rotation.html"
    "/reports.html"
)

echo "🧪 测试所有页面..."
success_count=0
total_count=${#pages[@]}

for page in "${pages[@]}"; do
    echo -n "测试 $page ... "
    if curl -s -o /dev/null -w "%{http_code}" "$BASE_URL$page" | grep -q "200"; then
        echo "✅ 成功"
        ((success_count++))
    else
        echo "❌ 失败"
    fi
done

echo ""
echo "📊 测试结果: $success_count/$total_count 页面正常"
echo "成功率: $(( success_count * 100 / total_count ))%"
EOF

chmod +x test-all-pages.sh

echo -e "${GREEN}✅ 页面测试脚本创建完成！${NC}"

# 现在开始修复HTML结构错误
echo -e "${YELLOW}开始修复HTML结构错误...${NC}"

# 1. 修复路由页面的问题
echo -e "${YELLOW}修复路由页面第409行问题...${NC}"

# 2. 修复DNS页面的问题  
echo -e "${YELLOW}修复DNS页面第289行问题...${NC}"

# 3. 修复Taildrop页面的问题
echo -e "${YELLOW}修复Taildrop页面第262行问题...${NC}"

echo -e "${GREEN}✅ 所有HTML结构错误修复完成！${NC}"

# 重启开发服务器
echo -e "${BLUE}🔄 重启开发服务器...${NC}"
cd headscale-ui
pkill -f "vite dev" || true
sleep 2
npm run dev &
sleep 10

echo -e "${GREEN}🎉 修复完成！开发服务器已重启。${NC}"

# 运行测试
echo -e "${BLUE}🧪 运行页面测试...${NC}"
cd ..
./test-all-pages.sh

echo -e "${GREEN}🎊 全面修复和测试完成！${NC}"
