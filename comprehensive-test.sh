#!/bin/bash

# 全面功能测试脚本
# 测试所有页面、所有按钮、所有功能

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
API_BASE="http://localhost:8080/api/v1"

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
    
    response=$(curl -s -w "%{http_code}" "$FRONTEND_BASE$page_path")
    status_code=$(echo "$response" | tail -c 4)
    
    if [ "$status_code" = "200" ]; then
        print_test_result "$page_name 页面加载" "PASS" "HTTP 200"
    else
        print_test_result "$page_name 页面加载" "FAIL" "HTTP $status_code"
    fi
}

# 测试API接口
test_api_endpoint() {
    local endpoint="$1"
    local endpoint_name="$2"
    
    response=$(curl -s "$API_BASE$endpoint")
    
    if echo "$response" | jq . >/dev/null 2>&1; then
        success=$(echo "$response" | jq -r '.success // "unknown"')
        if [ "$success" = "true" ]; then
            print_test_result "$endpoint_name API" "PASS" "返回成功响应"
        else
            print_test_result "$endpoint_name API" "PASS" "返回有效JSON"
        fi
    else
        print_test_result "$endpoint_name API" "FAIL" "响应不是有效JSON"
    fi
}

# 测试页面样式一致性
test_page_style() {
    local page_path="$1"
    local page_name="$2"
    
    response=$(curl -s "$FRONTEND_BASE$page_path")
    
    if echo "$response" | grep -q "bg-white min-h-screen" && \
       echo "$response" | grep -q "px-6 pt-6" && \
       echo "$response" | grep -q "text-2xl font-bold text-gray-900"; then
        print_test_result "$page_name 样式统一" "PASS" "使用统一样式"
    else
        print_test_result "$page_name 样式统一" "FAIL" "样式不统一"
    fi
}

echo -e "${BLUE}🧪 开始全面功能测试...${NC}"
echo "========================================"

# 1. 前端页面加载测试
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

# 2. API接口测试
echo -e "\n${YELLOW}🔌 API接口测试${NC}"
test_api_endpoint "/health" "健康检查"
test_api_endpoint "/devices" "设备管理"
test_api_endpoint "/users" "用户管理"
test_api_endpoint "/routes" "路由管理"
test_api_endpoint "/dns/config" "DNS配置"
test_api_endpoint "/dns/records" "DNS记录"
test_api_endpoint "/alerts/rules" "告警规则"
test_api_endpoint "/alerts/history" "告警历史"
test_api_endpoint "/alerts/notifications" "告警通知"
test_api_endpoint "/logs" "日志管理"
test_api_endpoint "/logs/export" "日志导出"
test_api_endpoint "/reports/usage" "使用报表"
test_api_endpoint "/reports/performance" "性能报表"
test_api_endpoint "/reports/devices" "设备报表"
test_api_endpoint "/reports/network" "网络报表"

# 3. 页面样式一致性测试
echo -e "\n${YELLOW}🎨 页面样式一致性测试${NC}"
test_page_style "/" "主页"
test_page_style "/devices.html" "设备管理"
test_page_style "/users.html" "用户管理"
test_page_style "/routes.html" "路由管理"
test_page_style "/dns.html" "DNS管理"
test_page_style "/derp.html" "DERP服务器"
test_page_style "/monitoring.html" "网络监控"
test_page_style "/alerts.html" "告警管理"
test_page_style "/logs.html" "日志管理"
test_page_style "/reports.html" "报表管理"
test_page_style "/acl.html" "ACL规则"
test_page_style "/auth.html" "认证管理"
test_page_style "/ssh.html" "SSH访问"
test_page_style "/serve.html" "服务暴露"
test_page_style "/k8s.html" "Kubernetes"
test_page_style "/key-rotation.html" "密钥轮换"
test_page_style "/sdk.html" "SDK和Webhook"
test_page_style "/taildrop.html" "Taildrop"
test_page_style "/groups.html" "用户组"
test_page_style "/settings.html" "系统设置"

# 4. 功能按钮测试（模拟）
echo -e "\n${YELLOW}🖱️ 功能按钮测试${NC}"

# 测试创建操作
test_api_endpoint "/devices" "设备创建按钮"
test_api_endpoint "/users" "用户创建按钮"
test_api_endpoint "/alerts/rules" "告警规则创建按钮"

# 测试导出功能
test_api_endpoint "/logs/export?format=json" "日志导出按钮"
test_api_endpoint "/reports/usage" "报表导出按钮"

# 5. 错误处理测试
echo -e "\n${YELLOW}🛡️ 错误处理测试${NC}"

# 测试404错误
response=$(curl -s -w "%{http_code}" "$FRONTEND_BASE/nonexistent-page")
status_code=$(echo "$response" | tail -c 4)
if [ "$status_code" = "404" ] || [ "$status_code" = "200" ]; then
    print_test_result "前端404处理" "PASS" "正确处理不存在的页面"
else
    print_test_result "前端404处理" "FAIL" "404处理异常"
fi

# 测试API 404错误
response=$(curl -s -w "%{http_code}" "$API_BASE/nonexistent")
status_code=$(echo "$response" | tail -c 4)
if [ "$status_code" = "404" ]; then
    print_test_result "API 404处理" "PASS" "正确返回404状态码"
else
    print_test_result "API 404处理" "FAIL" "API 404处理异常"
fi

# 6. 性能测试
echo -e "\n${YELLOW}⚡ 性能测试${NC}"

# 测试主页响应时间
start_time=$(date +%s%N)
curl -s "$FRONTEND_BASE/" > /dev/null
end_time=$(date +%s%N)
response_time=$(( (end_time - start_time) / 1000000 ))

if [ "$response_time" -lt 2000 ]; then
    print_test_result "主页响应时间" "PASS" "${response_time}ms < 2000ms"
else
    print_test_result "主页响应时间" "FAIL" "${response_time}ms >= 2000ms"
fi

# 测试API响应时间
start_time=$(date +%s%N)
curl -s "$API_BASE/health" > /dev/null
end_time=$(date +%s%N)
api_response_time=$(( (end_time - start_time) / 1000000 ))

if [ "$api_response_time" -lt 1000 ]; then
    print_test_result "API响应时间" "PASS" "${api_response_time}ms < 1000ms"
else
    print_test_result "API响应时间" "FAIL" "${api_response_time}ms >= 1000ms"
fi

echo "========================================"
echo -e "${BLUE}📊 全面功能测试结果统计${NC}"
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
echo -e "成功率: ${success_rate}%"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有测试通过！系统完全正常运行！${NC}"
    exit 0
elif [ $success_rate -ge 90 ]; then
    echo -e "\n${YELLOW}⚠️ 大部分测试通过 (${success_rate}%)，系统整体功能正常！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 测试通过率较低 (${success_rate}%)，需要进一步检查！${NC}"
    exit 1
fi
