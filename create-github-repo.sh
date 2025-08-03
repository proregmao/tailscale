#!/bin/bash

# GitHub仓库创建脚本
# 提供多种方式创建GitHub仓库

set -e

# 默认配置
DEFAULT_USERNAME="proregmao"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# 使用GitHub CLI创建仓库
create_with_gh_cli() {
    local repo_name=$1
    local description="$2"
    
    if ! command -v gh &> /dev/null; then
        log_error "GitHub CLI未安装"
        return 1
    fi
    
    log_info "使用GitHub CLI创建仓库..."
    
    local cmd="gh repo create $repo_name --public"
    if [[ -n "$description" ]]; then
        cmd="$cmd --description \"$description\""
    fi
    
    if eval "$cmd"; then
        log_success "仓库创建成功: https://github.com/$DEFAULT_USERNAME/$repo_name"
        return 0
    else
        log_error "GitHub CLI创建失败"
        return 1
    fi
}

# 使用GitHub API创建仓库
create_with_api() {
    local repo_name=$1
    local description="$2"
    local token="$3"
    
    log_info "使用GitHub API创建仓库..."
    
    local api_data="{\"name\":\"$repo_name\",\"description\":\"$description\",\"private\":false}"
    
    local response=$(curl -s -X POST \
        -H "Authorization: token $token" \
        -H "Accept: application/vnd.github.v3+json" \
        -d "$api_data" \
        "https://api.github.com/user/repos")
    
    if echo "$response" | grep -q '"id"'; then
        local repo_url=$(echo "$response" | grep -o '"html_url":"[^"]*"' | cut -d'"' -f4)
        log_success "仓库创建成功: $repo_url"
        return 0
    else
        local error_msg=$(echo "$response" | grep -o '"message":"[^"]*"' | cut -d'"' -f4)
        log_error "API创建失败: $error_msg"
        return 1
    fi
}

# 安装GitHub CLI
install_github_cli() {
    log_info "安装GitHub CLI..."
    
    if command -v apt &> /dev/null; then
        # Ubuntu/Debian
        curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
        echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
        sudo apt update && sudo apt install -y gh
    elif command -v yum &> /dev/null; then
        # CentOS/RHEL
        sudo yum install -y gh
    elif command -v brew &> /dev/null; then
        # macOS
        brew install gh
    else
        log_error "不支持的系统，请手动安装"
        echo "访问: https://cli.github.com/manual/installation"
        return 1
    fi
    
    if command -v gh &> /dev/null; then
        log_success "GitHub CLI安装成功"
        return 0
    else
        log_error "安装失败"
        return 1
    fi
}

# 主函数
main() {
    local repo_name="$1"
    local description="$2"
    
    echo "🚀 GitHub仓库创建工具"
    echo "======================"
    echo
    
    # 获取仓库名称
    if [[ -z "$repo_name" ]]; then
        echo -n "请输入仓库名称: "
        read -r repo_name
    fi
    
    if [[ -z "$repo_name" ]]; then
        log_error "仓库名称不能为空"
        exit 1
    fi
    
    # 获取描述
    if [[ -z "$description" ]]; then
        echo -n "请输入仓库描述 (可选): "
        read -r description
    fi
    
    log_info "仓库名称: $repo_name"
    log_info "仓库描述: ${description:-无}"
    log_info "目标用户: $DEFAULT_USERNAME"
    echo
    
    # 选择创建方式
    echo "选择创建方式："
    echo "1) 使用GitHub CLI (推荐)"
    echo "2) 使用GitHub API Token"
    echo "3) 安装GitHub CLI后创建"
    echo "4) 手动创建指导"
    echo -n "请选择 [1-4]: "
    read -r choice
    
    case $choice in
        1)
            if create_with_gh_cli "$repo_name" "$description"; then
                echo
                log_success "✅ 仓库创建完成！"
                echo "现在可以推送代码："
                echo "  git remote add origin git@github.com:$DEFAULT_USERNAME/$repo_name.git"
                echo "  git push -u origin main"
            else
                log_error "创建失败，请尝试其他方式"
                exit 1
            fi
            ;;
        2)
            echo -n "请输入GitHub Token: "
            read -r -s token
            echo
            if [[ -z "$token" ]]; then
                log_error "Token不能为空"
                exit 1
            fi
            
            if create_with_api "$repo_name" "$description" "$token"; then
                echo
                log_success "✅ 仓库创建完成！"
                echo "现在可以推送代码："
                echo "  git remote add origin git@github.com:$DEFAULT_USERNAME/$repo_name.git"
                echo "  git push -u origin main"
            else
                log_error "创建失败，请检查Token权限"
                exit 1
            fi
            ;;
        3)
            if install_github_cli; then
                echo
                log_info "请先进行GitHub认证："
                gh auth login
                echo
                if create_with_gh_cli "$repo_name" "$description"; then
                    echo
                    log_success "✅ 仓库创建完成！"
                else
                    log_error "创建失败"
                    exit 1
                fi
            else
                log_error "GitHub CLI安装失败"
                exit 1
            fi
            ;;
        4)
            echo
            log_info "📋 手动创建步骤："
            echo "1. 访问: https://github.com/new"
            echo "2. 仓库名: $repo_name"
            echo "3. 描述: ${description:-可选}"
            echo "4. 设置为公开仓库"
            echo "5. 不要勾选 'Add a README file'"
            echo "6. 不要勾选 'Add .gitignore'"
            echo "7. 不要勾选 'Choose a license'"
            echo "8. 点击 'Create repository'"
            echo
            log_info "创建完成后运行："
            echo "  git remote add origin git@github.com:$DEFAULT_USERNAME/$repo_name.git"
            echo "  git push -u origin main"
            ;;
        *)
            log_error "无效选择"
            exit 1
            ;;
    esac
}

# 使用说明
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "GitHub仓库创建工具"
    echo "=================="
    echo
    echo "用法："
    echo "  $0 [仓库名] [描述]"
    echo "  $0 --help"
    echo
    echo "示例："
    echo "  $0 my-project \"我的项目描述\""
    echo "  $0 my-project"
    echo "  $0"
    echo
    echo "支持的创建方式："
    echo "1. GitHub CLI (需要安装和认证)"
    echo "2. GitHub API Token"
    echo "3. 自动安装GitHub CLI"
    echo "4. 手动创建指导"
    exit 0
fi

# 执行主函数
main "$@"
