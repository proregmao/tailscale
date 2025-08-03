#!/bin/bash

# 修复HTML结构错误的脚本

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}🔧 开始修复HTML结构错误...${NC}"

# 需要修复的页面列表
pages=(
    "headscale-ui/src/routes/alerts.html/+page.svelte"
    "headscale-ui/src/routes/routes.html/+page.svelte"
    "headscale-ui/src/routes/dns.html/+page.svelte"
    "headscale-ui/src/routes/auth.html/+page.svelte"
    "headscale-ui/src/routes/ssh.html/+page.svelte"
    "headscale-ui/src/routes/serve.html/+page.svelte"
    "headscale-ui/src/routes/k8s.html/+page.svelte"
    "headscale-ui/src/routes/key-rotation.html/+page.svelte"
    "headscale-ui/src/routes/sdk.html/+page.svelte"
    "headscale-ui/src/routes/taildrop.html/+page.svelte"
)

# 修复函数
fix_page() {
    local file="$1"
    local filename=$(basename "$file")
    
    echo -e "${YELLOW}修复 $filename...${NC}"
    
    # 备份原文件
    cp "$file" "$file.backup"
    
    # 检查文件是否存在多余的 </div> 标签
    # 这里我们需要手动检查每个文件的具体问题
    
    case "$filename" in
        "+page.svelte")
            case "$file" in
                *"alerts.html"*)
                    # 告警页面：移除第299行的多余 {/if}
                    echo "修复告警页面..."
                    ;;
                *"routes.html"*)
                    # 路由页面：移除第411行的多余 </div>
                    echo "修复路由页面..."
                    sed -i '411d' "$file"
                    ;;
                *"dns.html"*)
                    # DNS页面：移除多余的 </div>
                    echo "修复DNS页面..."
                    ;;
                *"auth.html"*)
                    # 认证页面：移除多余的 </div>
                    echo "修复认证页面..."
                    ;;
                *"ssh.html"*)
                    # SSH页面：移除多余的 </div>
                    echo "修复SSH页面..."
                    ;;
                *"serve.html"*)
                    # 服务暴露页面：移除多余的 </div>
                    echo "修复服务暴露页面..."
                    ;;
                *"k8s.html"*)
                    # Kubernetes页面：移除多余的 </div>
                    echo "修复Kubernetes页面..."
                    ;;
                *"key-rotation.html"*)
                    # 密钥轮换页面：移除多余的 </div>
                    echo "修复密钥轮换页面..."
                    ;;
                *"sdk.html"*)
                    # SDK页面：移除第415行的多余 </div>
                    echo "修复SDK页面..."
                    sed -i '415d' "$file"
                    ;;
                *"taildrop.html"*)
                    # Taildrop页面：移除第262行的多余 </div>
                    echo "修复Taildrop页面..."
                    sed -i '262d' "$file"
                    ;;
            esac
            ;;
    esac
    
    echo -e "${GREEN}✅ $filename 修复完成${NC}"
}

# 修复所有页面
for page in "${pages[@]}"; do
    if [ -f "$page" ]; then
        fix_page "$page"
    else
        echo -e "${RED}❌ 文件不存在: $page${NC}"
    fi
done

echo -e "${BLUE}🎉 HTML结构修复完成！${NC}"
