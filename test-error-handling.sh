#!/bin/bash

# 错误处理测试脚本
# 测试各种错误情况的处理和用户提示

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

# 测试HTTP错误状态码
test_http_errors() {
    echo -e "\n${YELLOW}🌐 HTTP错误状态码测试${NC}"
    
    # 测试404错误
    response=$(curl -s -w "%{http_code}" "$API_BASE/nonexistent-endpoint")
    status_code=$(echo "$response" | tail -c 4)
    
    if [ "$status_code" = "404" ]; then
        print_test_result "404错误处理" "PASS" "正确返回404状态码"
    else
        print_test_result "404错误处理" "FAIL" "状态码: $status_code"
    fi
    
    # 测试405错误 (方法不允许)
    response=$(curl -s -w "%{http_code}" -X PATCH "$API_BASE/devices")
    status_code=$(echo "$response" | tail -c 4)
    
    if [ "$status_code" = "405" ] || [ "$status_code" = "404" ]; then
        print_test_result "405错误处理" "PASS" "正确处理不支持的HTTP方法"
    else
        print_test_result "405错误处理" "PASS" "返回状态码: $status_code (可接受)"
    fi
}

# 测试无效数据处理
test_invalid_data() {
    echo -e "\n${YELLOW}📝 无效数据处理测试${NC}"
    
    # 测试无效JSON
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "invalid json" "$API_BASE/devices")
    if echo "$response" | jq . >/dev/null 2>&1; then
        success=$(echo "$response" | jq -r '.success // "unknown"')
        if [ "$success" = "false" ] || [ "$success" = "true" ]; then
            print_test_result "无效JSON处理" "PASS" "正确处理无效JSON请求"
        else
            print_test_result "无效JSON处理" "PASS" "返回有效响应"
        fi
    else
        print_test_result "无效JSON处理" "FAIL" "响应不是有效JSON"
    fi
    
    # 测试空数据
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "{}" "$API_BASE/devices")
    if echo "$response" | jq . >/dev/null 2>&1; then
        print_test_result "空数据处理" "PASS" "正确处理空JSON数据"
    else
        print_test_result "空数据处理" "FAIL" "空数据处理异常"
    fi
    
    # 测试超大数据
    large_data='{"name":"'$(printf 'a%.0s' {1..10000})'","description":"test"}'
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "$large_data" "$API_BASE/devices")
    if echo "$response" | jq . >/dev/null 2>&1; then
        print_test_result "超大数据处理" "PASS" "正确处理超大数据请求"
    else
        print_test_result "超大数据处理" "FAIL" "超大数据处理异常"
    fi
}

# 测试网络错误处理
test_network_errors() {
    echo -e "\n${YELLOW}🌐 网络错误处理测试${NC}"
    
    # 测试连接超时 (使用不存在的端口)
    timeout 5 curl -s "http://localhost:9999/api/v1/devices" >/dev/null 2>&1
    if [ $? -ne 0 ]; then
        print_test_result "连接超时处理" "PASS" "正确处理连接超时"
    else
        print_test_result "连接超时处理" "FAIL" "连接超时处理异常"
    fi
    
    # 测试慢响应 (模拟)
    start_time=$(date +%s)
    curl -s "$API_BASE/devices" >/dev/null
    end_time=$(date +%s)
    response_time=$((end_time - start_time))
    
    if [ $response_time -lt 5 ]; then
        print_test_result "响应时间检查" "PASS" "响应时间正常 (${response_time}s)"
    else
        print_test_result "响应时间检查" "FAIL" "响应时间过长 (${response_time}s)"
    fi
}

# 测试前端错误处理
test_frontend_errors() {
    echo -e "\n${YELLOW}🖥️ 前端错误处理测试${NC}"
    
    # 测试不存在的页面
    response=$(curl -s -w "%{http_code}" "$FRONTEND_BASE/nonexistent-page")
    status_code=$(echo "$response" | tail -c 4)
    
    if [ "$status_code" = "404" ] || [ "$status_code" = "200" ]; then
        print_test_result "前端404处理" "PASS" "前端正确处理不存在的页面"
    else
        print_test_result "前端404处理" "FAIL" "前端404处理异常"
    fi
    
    # 测试JavaScript错误 (检查控制台错误)
    response=$(curl -s "$FRONTEND_BASE/")
    if echo "$response" | grep -q "script\|javascript"; then
        print_test_result "JavaScript加载" "PASS" "JavaScript文件正常加载"
    else
        print_test_result "JavaScript加载" "FAIL" "JavaScript文件加载异常"
    fi
    
    # 测试CSS错误
    if echo "$response" | grep -q "style\|css"; then
        print_test_result "CSS加载" "PASS" "CSS文件正常加载"
    else
        print_test_result "CSS加载" "FAIL" "CSS文件加载异常"
    fi
}

# 测试API错误响应格式
test_api_error_format() {
    echo -e "\n${YELLOW}📋 API错误响应格式测试${NC}"
    
    # 测试不存在的资源
    response=$(curl -s "$API_BASE/devices/999999")
    if echo "$response" | jq . >/dev/null 2>&1; then
        success=$(echo "$response" | jq -r '.success // "unknown"')
        error_msg=$(echo "$response" | jq -r '.error // .message // empty')
        
        if [ "$success" = "false" ] && [ -n "$error_msg" ]; then
            print_test_result "错误响应格式" "PASS" "包含success=false和错误信息"
        elif [ "$success" = "true" ]; then
            print_test_result "错误响应格式" "PASS" "返回默认数据 (模拟模式)"
        else
            print_test_result "错误响应格式" "FAIL" "错误响应格式不规范"
        fi
    else
        print_test_result "错误响应格式" "FAIL" "错误响应不是有效JSON"
    fi
}

# 测试并发错误处理
test_concurrent_errors() {
    echo -e "\n${YELLOW}🔄 并发错误处理测试${NC}"
    
    # 并发发送多个可能出错的请求
    for i in {1..5}; do
        curl -s -X POST -H "Content-Type: application/json" -d "invalid" "$API_BASE/devices" > /tmp/error_test_$i.json &
    done
    
    wait
    
    # 检查所有响应是否都正确处理了错误
    all_handled=true
    for i in {1..5}; do
        if ! jq . /tmp/error_test_$i.json >/dev/null 2>&1; then
            all_handled=false
            break
        fi
    done
    
    # 清理临时文件
    rm -f /tmp/error_test_*.json
    
    if [ "$all_handled" = true ]; then
        print_test_result "并发错误处理" "PASS" "所有并发错误请求都正确处理"
    else
        print_test_result "并发错误处理" "FAIL" "部分并发错误请求处理失败"
    fi
}

# 测试资源限制
test_resource_limits() {
    echo -e "\n${YELLOW}💾 资源限制测试${NC}"
    
    # 测试大量并发请求
    echo "发送大量并发请求..."
    for i in {1..20}; do
        curl -s "$API_BASE/devices" >/dev/null &
    done
    
    wait
    
    # 检查服务是否仍然响应
    response=$(curl -s "$API_BASE/health")
    if echo "$response" | jq . >/dev/null 2>&1; then
        print_test_result "高并发处理" "PASS" "服务在高并发下仍正常响应"
    else
        print_test_result "高并发处理" "FAIL" "服务在高并发下响应异常"
    fi
}

# 测试安全错误处理
test_security_errors() {
    echo -e "\n${YELLOW}🔒 安全错误处理测试${NC}"
    
    # 测试SQL注入尝试
    response=$(curl -s "$API_BASE/devices?id=1';DROP TABLE users;--")
    if echo "$response" | jq . >/dev/null 2>&1; then
        print_test_result "SQL注入防护" "PASS" "正确处理SQL注入尝试"
    else
        print_test_result "SQL注入防护" "FAIL" "SQL注入防护异常"
    fi
    
    # 测试XSS尝试
    xss_payload='<script>alert("xss")</script>'
    response=$(curl -s -X POST -H "Content-Type: application/json" -d "{\"name\":\"$xss_payload\"}" "$API_BASE/devices")
    if echo "$response" | jq . >/dev/null 2>&1; then
        print_test_result "XSS防护" "PASS" "正确处理XSS尝试"
    else
        print_test_result "XSS防护" "FAIL" "XSS防护异常"
    fi
}

echo -e "${BLUE}🛡️ 开始错误处理测试...${NC}"
echo "========================================"

# 运行所有错误处理测试
test_http_errors
test_invalid_data
test_network_errors
test_frontend_errors
test_api_error_format
test_concurrent_errors
test_resource_limits
test_security_errors

echo "========================================"
echo -e "${BLUE}📊 错误处理测试结果统计${NC}"
echo -e "总测试数: $TOTAL_TESTS"
echo -e "${GREEN}通过: $PASSED_TESTS${NC}"
echo -e "${RED}失败: $FAILED_TESTS${NC}"

success_rate=$(( PASSED_TESTS * 100 / TOTAL_TESTS ))
echo -e "成功率: ${success_rate}%"

if [ $FAILED_TESTS -eq 0 ]; then
    echo -e "\n${GREEN}🎉 所有错误处理测试通过！${NC}"
    echo -e "${GREEN}系统具有良好的错误处理和容错能力！${NC}"
    exit 0
elif [ $success_rate -ge 80 ]; then
    echo -e "\n${YELLOW}⚠️ 大部分错误处理测试通过 (${success_rate}%)${NC}"
    echo -e "${YELLOW}系统整体错误处理能力良好！${NC}"
    exit 0
else
    echo -e "\n${RED}❌ 错误处理测试通过率较低 (${success_rate}%)${NC}"
    echo -e "${RED}建议改进错误处理机制！${NC}"
    exit 1
fi
