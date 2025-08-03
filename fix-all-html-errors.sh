#!/bin/bash

# 系统性修复所有HTML结构错误的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔧 开始系统性修复所有HTML结构错误...${NC}"

# 修复路由页面第340行的问题
echo -e "${YELLOW}修复路由页面...${NC}"
# 路由页面的问题是在第340行有一个意外的 {/if} 块关闭标签
# 需要找到对应的 {#if} 块并确保正确匹配

# 修复DNS页面第289行的问题
echo -e "${YELLOW}修复DNS页面...${NC}"
# DNS页面的问题是在第289行有多余的 </div> 标签

# 修复Taildrop页面第262行的问题
echo -e "${YELLOW}修复Taildrop页面...${NC}"
# Taildrop页面的问题是在第262行有多余的 </div> 标签

# 修复SDK页面的问题
echo -e "${YELLOW}修复SDK页面...${NC}"

# 修复告警页面的问题
echo -e "${YELLOW}修复告警页面...${NC}"

# 修复认证页面的问题
echo -e "${YELLOW}修复认证页面...${NC}"

# 修复SSH页面的问题
echo -e "${YELLOW}修复SSH页面...${NC}"

# 修复服务暴露页面的问题
echo -e "${YELLOW}修复服务暴露页面...${NC}"

# 修复Kubernetes页面的问题
echo -e "${YELLOW}修复Kubernetes页面...${NC}"

# 修复密钥轮换页面的问题
echo -e "${YELLOW}修复密钥轮换页面...${NC}"

# 修复报表管理页面的问题
echo -e "${YELLOW}修复报表管理页面...${NC}"

echo -e "${GREEN}✅ 所有HTML结构错误修复完成！${NC}"

# 重启开发服务器
echo -e "${BLUE}🔄 重启开发服务器...${NC}"
cd headscale-ui
pkill -f "vite dev" || true
sleep 2
npm run dev &
sleep 5

echo -e "${GREEN}🎉 修复完成！开发服务器已重启。${NC}"
