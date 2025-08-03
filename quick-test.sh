#!/bin/bash

# 快速测试所有页面是否正常加载

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# 基础URL
FRONTEND_BASE="http://localhost:5173"

# 打印测试结果
print_test_result() {
    local test_name="$1"
    local status="$2"
    local details="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status" = "PASS" ]; then
        echo -e "${GREEN}✅ PASS${NC} $test_name"
        [ -n "$details" ] && echo -e "   ${BLUE}详情:${NC} $details"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAIL${NC} $test_name"
        [ -n "$details" ] && echo -e "   ${RED}错误:${NC} $details"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# 测试页面加载
test_page_load() {
    local page_path="$1"
    local page_name="$2"
    
    response=$(curl -s -w "%{http_code}" "$FRONTEND_BASE$page_path" --max-time 10)
    status_code=$(echo "$response" | tail -c 4)
    
    if [ "$status_code" = "200" ]; then
        print_test_result "$page_name 页面加载" "PASS" "HTTP 200"
    else
        print_test_result "$page_name 页面加载" "FAIL" "HTTP $status_code"
    fi
}

echo -e "${BLUE}🧪 开始快速页面测试...${NC}"
echo "========================================"

# 测试所有页面
echo -e "\n${YELLOW}📱 前端页面加载测试${NC}"
test_page_load "/" "主页"
test_page_load "/devices.html" "设备管理"
test_page_load "/users.html" "用户管理"
test_page_load "/routes.html" "路由管理"
test_page_load "/dns.html" "DNS管理"
test_page_load "/derp.html" "DERP服务器"
test_page_load "/monitoring.html" "网络监控"
test_page_load "/alerts.html" "告警管理"
test_page_load "/logs.html" "日志管理"
test_page_load "/reports.html" "报表管理"
test_page_load "/acl.html" "ACL规则"
test_page_load "/auth.html" "认证管理"
test_page_load "/ssh.html" "SSH访问"
test_page_load "/serve.html" "服务暴露"
test_page_load "/k8s.html" "Kubernetes"
test_page_load "/key-rotation.html" "密钥轮换"
test_page_load "/sdk.html" "SDK和Webhook"
test_page_load "/taildrop.html" "Taildrop"
test_page_load "/groups.html" "用户组"
test_page_load "/settings.html" "系统设置"

echo "========================================"
echo -e "${BLUE}📊 快速测试结果统计${NC}"
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
echo -e "成功率: ${success_rate}%"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有页面测试通过！${NC}"
    exit 0
elif [ $success_rate -ge 90 ]; then
    echo -e "\n${YELLOW}⚠️ 大部分页面测试通过 (${success_rate}%)${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 页面测试通过率较低 (${success_rate}%)${NC}"
    exit 1
fi
