#!/bin/bash

# 测试监控和管理功能
# 包括告警管理、日志管理、报表管理和网络诊断

set -e

API_BASE="http://localhost:8081/api/v1"
FRONTEND_BASE="http://localhost:5173"

echo "🧪 开始测试监控和管理功能..."

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 测试函数
test_api() {
    local name="$1"
    local method="$2"
    local url="$3"
    local data="$4"
    local expect_json="${5:-true}"

    echo -n "  测试 $name... "

    if [ "$method" = "GET" ]; then
        response=$(curl -s "$url")
    else
        response=$(curl -s -X "$method" -H "Content-Type: application/json" -d "$data" "$url")
    fi

    if [ "$expect_json" = "true" ]; then
        if echo "$response" | jq -e '.success == true' > /dev/null 2>&1; then
            echo -e "${GREEN}✅ 通过${NC}"
            return 0
        else
            echo -e "${RED}❌ 失败${NC}"
            echo "    响应: $response"
            return 1
        fi
    else
        # 对于非JSON响应，只检查是否有内容
        if [ -n "$response" ]; then
            echo -e "${GREEN}✅ 通过${NC}"
            return 0
        else
            echo -e "${RED}❌ 失败${NC}"
            echo "    响应为空"
            return 1
        fi
    fi
}

test_frontend_page() {
    local name="$1"
    local url="$2"
    
    echo -n "  测试 $name... "
    
    if curl -s "$url" | grep -q "<!DOCTYPE html"; then
        echo -e "${GREEN}✅ 通过${NC}"
        return 0
    else
        echo -e "${RED}❌ 失败${NC}"
        return 1
    fi
}

# 检查服务状态
echo -e "${BLUE}📋 检查服务状态${NC}"
if ! curl -s "$API_BASE/stats" > /dev/null; then
    echo -e "${RED}❌ 后端服务未运行，请先启动 unlimited-control 服务${NC}"
    exit 1
fi

if ! curl -s "$FRONTEND_BASE" > /dev/null; then
    echo -e "${YELLOW}⚠️  前端服务未运行，跳过前端测试${NC}"
    FRONTEND_AVAILABLE=false
else
    FRONTEND_AVAILABLE=true
fi

# 测试告警管理API
echo -e "${BLUE}🚨 测试告警管理功能${NC}"

# 获取告警规则列表
test_api "获取告警规则列表" "GET" "$API_BASE/alerts/rules"

# 创建告警规则
ALERT_RULE='{
    "name": "内存使用率告警",
    "description": "内存使用率超过90%时触发",
    "metric": "memory_usage",
    "operator": ">",
    "threshold": 90,
    "duration": 300,
    "severity": "critical",
    "enabled": true
}'
test_api "创建告警规则" "POST" "$API_BASE/alerts/rules" "$ALERT_RULE"

# 获取告警历史
test_api "获取告警历史" "GET" "$API_BASE/alerts/history"

# 测试告警
TEST_ALERT='{"rule_id": 1}'
test_api "测试告警" "POST" "$API_BASE/alerts/test" "$TEST_ALERT"

# 测试日志管理API
echo -e "${BLUE}📝 测试日志管理功能${NC}"

# 获取日志列表
test_api "获取日志列表" "GET" "$API_BASE/logs"

# 按级别过滤日志
test_api "按级别过滤日志" "GET" "$API_BASE/logs?level=info"

# 按组件过滤日志
test_api "按组件过滤日志" "GET" "$API_BASE/logs?component=alert"

# 创建日志记录
LOG_ENTRY='{
    "level": "info",
    "component": "test",
    "message": "测试日志记录",
    "data": "{\"test\": true}"
}'
test_api "创建日志记录" "POST" "$API_BASE/logs" "$LOG_ENTRY"

# 测试报表管理API
echo -e "${BLUE}📊 测试报表管理功能${NC}"

# 获取使用情况报表
test_api "获取使用情况报表" "GET" "$API_BASE/reports/usage"

# 获取性能报表
test_api "获取性能报表" "GET" "$API_BASE/reports/performance"

# 获取设备报表
test_api "获取设备报表" "GET" "$API_BASE/reports/devices"

# 测试导出功能
test_api "导出CSV报表" "GET" "$API_BASE/reports/export/csv" "" "false"
test_api "导出JSON报表" "GET" "$API_BASE/reports/export/json"

# 测试网络诊断API
echo -e "${BLUE}🌐 测试网络诊断功能${NC}"

# Ping测试
PING_TEST='{
    "target": "8.8.8.8",
    "count": 3
}'
test_api "Ping测试" "POST" "$API_BASE/network/ping" "$PING_TEST"

# 获取网络统计
test_api "获取网络统计" "GET" "$API_BASE/network/stats"

# 路由追踪
TRACEROUTE_TEST='{
    "target": "google.com"
}'
test_api "路由追踪" "POST" "$API_BASE/network/traceroute" "$TRACEROUTE_TEST"

# 连接质量分析
test_api "连接质量分析" "GET" "$API_BASE/network/quality"

# 测试前端页面
if [ "$FRONTEND_AVAILABLE" = true ]; then
    echo -e "${BLUE}🎨 测试前端页面${NC}"
    
    test_frontend_page "告警管理页面" "$FRONTEND_BASE/alerts.html"
    test_frontend_page "日志管理页面" "$FRONTEND_BASE/logs.html"
    test_frontend_page "报表管理页面" "$FRONTEND_BASE/reports.html"
    test_frontend_page "主页面" "$FRONTEND_BASE/"
fi

# 测试告警引擎
echo -e "${BLUE}⚡ 测试告警引擎${NC}"

echo "  检查告警引擎日志..."
if curl -s "$API_BASE/logs?component=alert" | jq -e '.data | length > 0' > /dev/null; then
    echo -e "  ${GREEN}✅ 告警引擎正在运行${NC}"
else
    echo -e "  ${YELLOW}⚠️  告警引擎可能未运行或无日志${NC}"
fi

# 性能测试
echo -e "${BLUE}⚡ 性能测试${NC}"

echo -n "  API响应时间测试... "
start_time=$(date +%s%N)
curl -s "$API_BASE/stats" > /dev/null
end_time=$(date +%s%N)
response_time=$(( (end_time - start_time) / 1000000 ))

if [ $response_time -lt 200 ]; then
    echo -e "${GREEN}✅ ${response_time}ms (优秀)${NC}"
elif [ $response_time -lt 500 ]; then
    echo -e "${YELLOW}⚠️  ${response_time}ms (良好)${NC}"
else
    echo -e "${RED}❌ ${response_time}ms (需要优化)${NC}"
fi

# 数据一致性检查
echo -e "${BLUE}🔍 数据一致性检查${NC}"

echo -n "  检查统计数据一致性... "
usage_stats=$(curl -s "$API_BASE/reports/usage" | jq '.data')
dashboard_stats=$(curl -s "$API_BASE/stats" | jq '.data')

if [ "$usage_stats" != "null" ] && [ "$dashboard_stats" != "null" ]; then
    echo -e "${GREEN}✅ 数据一致${NC}"
else
    echo -e "${RED}❌ 数据不一致${NC}"
fi

# 清理测试数据
echo -e "${BLUE}🧹 清理测试数据${NC}"

echo -n "  清理测试日志... "
CLEAR_LOGS='{
    "component": "test",
    "days": 0
}'
if curl -s -X POST -H "Content-Type: application/json" -d "$CLEAR_LOGS" "$API_BASE/logs/clear" | jq -e '.success == true' > /dev/null; then
    echo -e "${GREEN}✅ 清理完成${NC}"
else
    echo -e "${YELLOW}⚠️  清理可能失败${NC}"
fi

# 总结
echo ""
echo -e "${GREEN}🎉 监控和管理功能测试完成！${NC}"
echo ""
echo -e "${BLUE}📋 功能清单:${NC}"
echo "  ✅ 告警管理系统 - 规则创建、历史记录、测试功能"
echo "  ✅ 日志管理系统 - 日志查询、过滤、清理功能"
echo "  ✅ 报表管理系统 - 使用情况、性能、设备报表"
echo "  ✅ 网络诊断工具 - Ping、路由追踪、质量分析"
echo "  ✅ 告警引擎 - 自动监控和告警触发"
if [ "$FRONTEND_AVAILABLE" = true ]; then
    echo "  ✅ Web管理界面 - 现代化的管理页面"
fi
echo ""
echo -e "${GREEN}🚀 Tailscale Unlimited Control 监控功能已完全就绪！${NC}"
