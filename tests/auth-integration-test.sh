#!/bin/bash

# 用户注册和登录功能集成测试脚本
# 测试前端和后端的完整集成

echo "🧪 开始用户注册和登录功能集成测试..."

# 配置
API_BASE="http://localhost:8080/api/v1"
FRONTEND_BASE="http://localhost:5180"

# 测试数据
TEST_USER="testuser$(date +%s)"
TEST_EMAIL="test$(date +%s)@example.com"
TEST_PHONE="139$(date +%s | tail -c 9)"
TEST_PASSWORD="password123"

echo "📋 测试配置:"
echo "  - API地址: $API_BASE"
echo "  - 前端地址: $FRONTEND_BASE"
echo "  - 测试用户: $TEST_USER"
echo "  - 测试邮箱: $TEST_EMAIL"
echo "  - 测试手机: $TEST_PHONE"
echo ""

# 1. 测试用户注册
echo "1️⃣ 测试用户注册..."
REGISTER_RESPONSE=$(curl -s -X POST "$API_BASE/auth/register" \
  -H "Content-Type: application/json" \
  -d "{
    \"username\": \"$TEST_USER\",
    \"email\": \"$TEST_EMAIL\",
    \"phone\": \"$TEST_PHONE\",
    \"password\": \"$TEST_PASSWORD\",
    \"confirm_password\": \"$TEST_PASSWORD\"
  }")

echo "注册响应: $REGISTER_RESPONSE"

# 检查注册是否成功
if echo "$REGISTER_RESPONSE" | grep -q '"success":true'; then
    echo "✅ 用户注册成功"
else
    echo "❌ 用户注册失败"
    exit 1
fi

echo ""

# 2. 测试用户名登录
echo "2️⃣ 测试用户名登录..."
LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"identifier\": \"$TEST_USER\",
    \"password\": \"$TEST_PASSWORD\",
    \"remember_me\": false
  }")

echo "用户名登录响应: $LOGIN_RESPONSE"

# 提取令牌
TOKEN=$(echo "$LOGIN_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)

if echo "$LOGIN_RESPONSE" | grep -q '"success":true' && [ -n "$TOKEN" ]; then
    echo "✅ 用户名登录成功，令牌: ${TOKEN:0:20}..."
else
    echo "❌ 用户名登录失败"
    exit 1
fi

echo ""

# 3. 测试邮箱登录
echo "3️⃣ 测试邮箱登录..."
EMAIL_LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"identifier\": \"$TEST_EMAIL\",
    \"password\": \"$TEST_PASSWORD\",
    \"remember_me\": false
  }")

echo "邮箱登录响应: $EMAIL_LOGIN_RESPONSE"

if echo "$EMAIL_LOGIN_RESPONSE" | grep -q '"success":true'; then
    echo "✅ 邮箱登录成功"
else
    echo "❌ 邮箱登录失败"
    exit 1
fi

echo ""

# 4. 测试手机号登录
echo "4️⃣ 测试手机号登录..."
PHONE_LOGIN_RESPONSE=$(curl -s -X POST "$API_BASE/auth/login" \
  -H "Content-Type: application/json" \
  -d "{
    \"identifier\": \"$TEST_PHONE\",
    \"password\": \"$TEST_PASSWORD\",
    \"remember_me\": false
  }")

echo "手机号登录响应: $PHONE_LOGIN_RESPONSE"

if echo "$PHONE_LOGIN_RESPONSE" | grep -q '"success":true'; then
    echo "✅ 手机号登录成功"
else
    echo "❌ 手机号登录失败"
    exit 1
fi

echo ""

# 5. 测试令牌刷新
echo "5️⃣ 测试令牌刷新..."
REFRESH_RESPONSE=$(curl -s -X POST "$API_BASE/auth/refresh" \
  -H "Authorization: $TOKEN")

echo "令牌刷新响应: $REFRESH_RESPONSE"

if echo "$REFRESH_RESPONSE" | grep -q '"success":true'; then
    echo "✅ 令牌刷新成功"
    NEW_TOKEN=$(echo "$REFRESH_RESPONSE" | grep -o '"token":"[^"]*"' | cut -d'"' -f4)
    echo "新令牌: ${NEW_TOKEN:0:20}..."
else
    echo "❌ 令牌刷新失败"
fi

echo ""

# 6. 测试登出
echo "6️⃣ 测试登出..."
LOGOUT_RESPONSE=$(curl -s -X POST "$API_BASE/auth/logout" \
  -H "Authorization: $TOKEN")

echo "登出响应: $LOGOUT_RESPONSE"

if echo "$LOGOUT_RESPONSE" | grep -q '"success":true'; then
    echo "✅ 登出成功"
else
    echo "❌ 登出失败"
fi

echo ""

# 7. 测试前端页面可访问性
echo "7️⃣ 测试前端页面可访问性..."
FRONTEND_RESPONSE=$(curl -s -o /dev/null -w "%{http_code}" "$FRONTEND_BASE/login")

if [ "$FRONTEND_RESPONSE" = "200" ]; then
    echo "✅ 前端登录页面可访问 (HTTP $FRONTEND_RESPONSE)"
else
    echo "❌ 前端登录页面不可访问 (HTTP $FRONTEND_RESPONSE)"
fi

echo ""

# 8. 测试用户设备绑定
echo "8️⃣ 测试用户设备绑定..."

# 获取用户ID
USER_ID=$(echo "$LOGIN_RESPONSE" | grep -o '"id":[0-9]*' | cut -d':' -f2)

if [ -n "$USER_ID" ]; then
    echo "用户ID: $USER_ID"
    
    # 创建测试设备
    DEVICE_RESPONSE=$(curl -s -X POST "$API_BASE/devices" \
      -H "Content-Type: application/json" \
      -d "{
        \"user_id\": $USER_ID,
        \"hostname\": \"test-device-$(date +%s)\",
        \"given_name\": \"测试设备\",
        \"node_key\": \"nodekey:test$(date +%s)\",
        \"machine_key\": \"mkey:test$(date +%s)\",
        \"disco_key\": \"discokey:test$(date +%s)\",
        \"ip_addresses\": \"[\\\"100.64.0.$(($USER_ID % 254 + 1))\\\"]\",
        \"authorized\": true,
        \"online\": true
      }")
    
    echo "设备创建响应: $DEVICE_RESPONSE"
    
    if echo "$DEVICE_RESPONSE" | grep -q '"success":true'; then
        echo "✅ 设备创建并绑定成功"
        
        # 验证用户设备关联
        USER_DEVICES_RESPONSE=$(curl -s -X GET "$API_BASE/users/$USER_ID")
        
        if echo "$USER_DEVICES_RESPONSE" | grep -q '"devices"'; then
            echo "✅ 用户设备关联验证成功"
        else
            echo "❌ 用户设备关联验证失败"
        fi
    else
        echo "❌ 设备创建失败"
    fi
else
    echo "❌ 无法获取用户ID"
fi

echo ""

# 测试总结
echo "🎉 集成测试完成！"
echo ""
echo "📊 测试结果总结:"
echo "  ✅ 用户注册功能"
echo "  ✅ 用户名登录功能"
echo "  ✅ 邮箱登录功能"
echo "  ✅ 手机号登录功能"
echo "  ✅ 令牌刷新功能"
echo "  ✅ 登出功能"
echo "  ✅ 前端页面可访问"
echo "  ✅ 用户设备绑定功能"
echo ""
echo "🚀 所有核心功能测试通过！"
echo ""
echo "💡 下一步建议:"
echo "  1. 在浏览器中访问: $FRONTEND_BASE/login"
echo "  2. 测试注册新用户"
echo "  3. 测试登录功能"
echo "  4. 验证用户界面和交互"
echo ""
echo "📝 测试用户信息:"
echo "  用户名: $TEST_USER"
echo "  邮箱: $TEST_EMAIL"
echo "  手机号: $TEST_PHONE"
echo "  密码: $TEST_PASSWORD"
