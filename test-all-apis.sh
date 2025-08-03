#!/bin/bash

# 全面API测试脚本
# 测试所有后端API接口

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试计数器
TOTAL_TESTS=0
PASSED_TESTS=0
FAILED_TESTS=0

# API基础URL
API_BASE="http://localhost:8080/api/v1"

# 打印测试结果
print_test_result() {
    local test_name="$1"
    local status_code="$2"
    local expected_code="$3"
    local response="$4"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status_code" = "$expected_code" ]; then
        echo -e "${GREEN}✅ PASS${NC} $test_name (HTTP $status_code)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAIL${NC} $test_name (Expected HTTP $expected_code, got $status_code)"
        echo -e "${YELLOW}Response:${NC} $response"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# 测试API接口
test_api() {
    local method="$1"
    local endpoint="$2"
    local expected_code="$3"
    local test_name="$4"
    local data="$5"
    
    if [ "$method" = "GET" ]; then
        response=$(curl -s -w "\n%{http_code}" "$API_BASE$endpoint")
    elif [ "$method" = "POST" ]; then
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X POST -H "Content-Type: application/json" -d "$data" "$API_BASE$endpoint")
        else
            response=$(curl -s -w "\n%{http_code}" -X POST "$API_BASE$endpoint")
        fi
    elif [ "$method" = "PUT" ]; then
        if [ -n "$data" ]; then
            response=$(curl -s -w "\n%{http_code}" -X PUT -H "Content-Type: application/json" -d "$data" "$API_BASE$endpoint")
        else
            response=$(curl -s -w "\n%{http_code}" -X PUT "$API_BASE$endpoint")
        fi
    elif [ "$method" = "DELETE" ]; then
        response=$(curl -s -w "\n%{http_code}" -X DELETE "$API_BASE$endpoint")
    fi
    
    status_code=$(echo "$response" | tail -n1)
    response_body=$(echo "$response" | head -n -1)
    
    print_test_result "$test_name" "$status_code" "$expected_code" "$response_body"
}

echo -e "${BLUE}🧪 开始全面API测试...${NC}"
echo "========================================"

# 1. 健康检查
echo -e "\n${YELLOW}📊 健康检查${NC}"
test_api "GET" "/health" "200" "健康检查"

# 2. 设备管理API
echo -e "\n${YELLOW}📱 设备管理API${NC}"
test_api "GET" "/devices" "200" "获取设备列表"
test_api "POST" "/devices" "200" "创建设备" '{"name":"test-device","user":"test-user"}'
test_api "GET" "/devices/1" "200" "获取单个设备"
test_api "PUT" "/devices/1" "200" "更新设备" '{"name":"updated-device"}'
test_api "POST" "/devices/1/authorize" "200" "授权设备"
test_api "DELETE" "/devices/1" "200" "删除设备"

# 3. 用户管理API
echo -e "\n${YELLOW}👥 用户管理API${NC}"
test_api "GET" "/users" "200" "获取用户列表"
test_api "POST" "/users" "200" "创建用户" '{"name":"test-user","email":"test@example.com"}'
test_api "GET" "/users/1" "200" "获取单个用户"
test_api "PUT" "/users/1" "200" "更新用户" '{"name":"updated-user"}'
test_api "DELETE" "/users/1" "200" "删除用户"

# 4. 路由管理API
echo -e "\n${YELLOW}🛣️ 路由管理API${NC}"
test_api "GET" "/routes" "200" "获取路由列表"
test_api "POST" "/routes" "200" "创建路由" '{"prefix":"10.0.0.0/24","device_id":1}'
test_api "GET" "/routes/1" "200" "获取单个路由"
test_api "PUT" "/routes/1" "200" "更新路由" '{"enabled":true}'
test_api "DELETE" "/routes/1" "200" "删除路由"

# 5. DNS管理API
echo -e "\n${YELLOW}🌐 DNS管理API${NC}"
test_api "GET" "/dns/config" "200" "获取DNS配置"
test_api "PUT" "/dns/config" "200" "更新DNS配置" '{"magic_dns":true,"base_domain":"example.com"}'
test_api "GET" "/dns/records" "200" "获取DNS记录"
test_api "POST" "/dns/records" "200" "创建DNS记录" '{"name":"test","type":"A","value":"1.2.3.4"}'

# 6. 告警管理API
echo -e "\n${YELLOW}🚨 告警管理API${NC}"
test_api "GET" "/alerts/rules" "200" "获取告警规则"
test_api "POST" "/alerts/rules" "200" "创建告警规则" '{"name":"test-rule","condition":"cpu > 80"}'
test_api "GET" "/alerts/history" "200" "获取告警历史"
test_api "GET" "/alerts/notifications" "200" "获取通知设置"
test_api "PUT" "/alerts/notifications" "200" "更新通知设置" '{"email":{"enabled":true}}'

# 7. 日志管理API
echo -e "\n${YELLOW}📋 日志管理API${NC}"
test_api "GET" "/logs" "200" "获取日志列表"
test_api "POST" "/logs" "200" "创建日志" '{"level":"info","message":"test log"}'
test_api "DELETE" "/logs" "200" "清空日志"
test_api "GET" "/logs/export?format=json" "200" "导出日志"

# 8. 报表管理API
echo -e "\n${YELLOW}📈 报表管理API${NC}"
test_api "GET" "/reports/usage" "200" "使用情况报表"
test_api "GET" "/reports/performance" "200" "性能报表"
test_api "GET" "/reports/devices" "200" "设备报表"
test_api "GET" "/reports/network" "200" "网络报表"

# 9. SSH管理API
echo -e "\n${YELLOW}🔑 SSH管理API${NC}"
test_api "GET" "/ssh/keys" "200" "获取SSH密钥"
test_api "POST" "/ssh/keys" "200" "创建SSH密钥" '{"name":"test-key","public_key":"ssh-rsa AAAAB3..."}'
test_api "GET" "/ssh/sessions" "200" "获取SSH会话"

# 10. 服务暴露API
echo -e "\n${YELLOW}🌍 服务暴露API${NC}"
test_api "GET" "/serve/configs" "200" "获取服务配置"
test_api "POST" "/serve/configs" "200" "创建服务配置" '{"port":8080,"protocol":"http"}'
test_api "GET" "/funnel/configs" "200" "获取Funnel配置"

# 11. Kubernetes集成API
echo -e "\n${YELLOW}☸️ Kubernetes集成API${NC}"
test_api "GET" "/k8s/clusters" "200" "获取集群列表"
test_api "POST" "/k8s/clusters" "200" "创建集群" '{"name":"test-cluster","endpoint":"https://k8s.example.com"}'
test_api "GET" "/k8s/policies" "200" "获取网络策略"
test_api "POST" "/k8s/operator" "200" "创建Operator配置" '{"namespace":"tailscale"}'

# 12. Webhook系统API
echo -e "\n${YELLOW}🔗 Webhook系统API${NC}"
test_api "GET" "/webhooks" "200" "获取Webhook列表"
test_api "POST" "/webhooks" "200" "创建Webhook" '{"url":"https://example.com/webhook","events":["device.created"]}'
test_api "POST" "/webhooks/1/test" "200" "测试Webhook"

# 13. API密钥管理
echo -e "\n${YELLOW}🔐 API密钥管理${NC}"
test_api "GET" "/api-keys" "200" "获取API密钥"
test_api "POST" "/api-keys" "200" "创建API密钥" '{"name":"test-key","permissions":["read","write"]}'

# 14. LocalAPI
echo -e "\n${YELLOW}🏠 LocalAPI${NC}"
test_api "GET" "/localapi/v0/status" "200" "LocalAPI状态"
test_api "GET" "/localapi/v0/prefs" "200" "LocalAPI配置"

echo "========================================"
echo -e "${BLUE}📊 测试结果统计${NC}"
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有API测试通过！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 有 $FAILED_TESTS 个测试失败${NC}"
    exit 1
fi
