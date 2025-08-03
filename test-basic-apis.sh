#!/bin/bash

# 基础API测试脚本
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

# API基础URL
API_BASE="http://localhost:8080/api/v1"

# 打印测试结果
print_test_result() {
    local test_name="$1"
    local status_code="$2"
    local expected_code="$3"
    
    TOTAL_TESTS=$((TOTAL_TESTS + 1))
    
    if [ "$status_code" = "$expected_code" ]; then
        echo -e "${GREEN}✅ PASS${NC} $test_name (HTTP $status_code)"
        PASSED_TESTS=$((PASSED_TESTS + 1))
    else
        echo -e "${RED}❌ FAIL${NC} $test_name (Expected HTTP $expected_code, got $status_code)"
        FAILED_TESTS=$((FAILED_TESTS + 1))
    fi
}

# 测试GET API
test_get_api() {
    local endpoint="$1"
    local test_name="$2"
    local expected_code="${3:-200}"
    
    response=$(curl -s -w "\n%{http_code}" "$API_BASE$endpoint")
    status_code=$(echo "$response" | tail -n1)
    
    print_test_result "$test_name" "$status_code" "$expected_code"
}

echo -e "${BLUE}🧪 开始基础API测试...${NC}"
echo "========================================"

# 1. 健康检查
echo -e "\n${YELLOW}📊 健康检查${NC}"
test_get_api "/health" "健康检查"

# 2. 现有的API接口
echo -e "\n${YELLOW}📱 设备管理API${NC}"
test_get_api "/devices" "获取设备列表"

echo -e "\n${YELLOW}👥 用户管理API${NC}"
test_get_api "/users" "获取用户列表"

echo -e "\n${YELLOW}🛣️ 路由管理API${NC}"
test_get_api "/routes" "获取路由列表"

echo -e "\n${YELLOW}🌐 DNS管理API${NC}"
test_get_api "/dns/config" "获取DNS配置"
test_get_api "/dns/records" "获取DNS记录"

echo -e "\n${YELLOW}🚨 告警管理API${NC}"
test_get_api "/alerts/rules" "获取告警规则"
test_get_api "/alerts/history" "获取告警历史"
test_get_api "/alerts/notifications" "获取通知设置"

echo -e "\n${YELLOW}📋 日志管理API${NC}"
test_get_api "/logs" "获取日志列表"
test_get_api "/logs/export?format=json" "导出日志"

echo -e "\n${YELLOW}📈 报表管理API${NC}"
test_get_api "/reports/usage" "使用情况报表"
test_get_api "/reports/performance" "性能报表"
test_get_api "/reports/devices" "设备报表"
test_get_api "/reports/network" "网络报表"

echo "========================================"
echo -e "${BLUE}📊 测试结果统计${NC}"
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有基础API测试通过！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 有 $FAILED_TESTS 个测试失败${NC}"
    exit 1
fi
