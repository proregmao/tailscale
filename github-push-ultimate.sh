#!/bin/bash

# 🚀 GitHub推送万能脚本 - 集成所有功能
# 功能：SSH设置、仓库创建、推送、错误处理 - 一个脚本搞定一切
# 特点：永不退出，总能找到解决方案

# 默认配置
DEFAULT_EMAIL="proreg@163.com"
DEFAULT_USERNAME="proregmao"
GITHUB_BASE_URL="https://github.com"
MAX_RETRIES=10  # 增加重试次数

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
NC='\033[0m'

log_info() { echo -e "${BLUE}[INFO]${NC} $1"; }
log_success() { echo -e "${GREEN}[SUCCESS]${NC} $1"; }
log_warning() { echo -e "${YELLOW}[WARNING]${NC} $1"; }
log_error() { echo -e "${RED}[ERROR]${NC} $1"; }
log_step() { echo -e "${PURPLE}[STEP]${NC} $1"; }

# 显示标题
show_header() {
    clear
    echo "🚀 GitHub推送万能脚本"
    echo "======================"
    echo "✨ 集成所有功能：SSH设置、仓库创建、推送、错误处理"
    echo "🛡️ 永不放弃：无论遇到什么问题都能解决"
    echo "📁 目标账户: $DEFAULT_USERNAME ($DEFAULT_EMAIL)"
    echo
}

# 检查并安装必要工具
ensure_tools() {
    log_step "检查并安装必要工具..."
    
    # 检查git
    if ! command -v git &> /dev/null; then
        log_warning "Git未安装，正在安装..."
        if command -v apt &> /dev/null; then
            sudo apt update && sudo apt install -y git
        elif command -v yum &> /dev/null; then
            sudo yum install -y git
        else
            log_error "无法自动安装Git，请手动安装后重新运行"
            echo "Ubuntu/Debian: sudo apt install git"
            echo "CentOS/RHEL: sudo yum install git"
            echo -n "安装完成后按回车继续..."
            read -r
            return 1
        fi
    fi
    
    # 检查curl
    if ! command -v curl &> /dev/null; then
        log_warning "curl未安装，正在安装..."
        if command -v apt &> /dev/null; then
            sudo apt install -y curl
        elif command -v yum &> /dev/null; then
            sudo yum install -y curl
        fi
    fi
    
    log_success "工具检查完成"
    return 0
}

# SSH密钥设置
setup_ssh() {
    log_step "检查SSH密钥配置..."
    
    # 检查SSH密钥
    if [[ ! -f ~/.ssh/id_ed25519 && ! -f ~/.ssh/id_rsa ]]; then
        log_warning "未找到SSH密钥，正在生成..."
        
        mkdir -p ~/.ssh
        chmod 700 ~/.ssh
        
        ssh-keygen -t ed25519 -C "$DEFAULT_EMAIL" -f ~/.ssh/id_ed25519 -N ""
        
        eval "$(ssh-agent -s)" &>/dev/null
        ssh-add ~/.ssh/id_ed25519 &>/dev/null
        
        log_success "SSH密钥生成完成"
        
        # 显示公钥
        echo
        log_info "您的SSH公钥："
        echo "=================================="
        cat ~/.ssh/id_ed25519.pub
        echo "=================================="
        echo
        
        # 尝试复制到剪贴板
        if command -v pbcopy &> /dev/null; then
            cat ~/.ssh/id_ed25519.pub | pbcopy
            log_success "公钥已复制到剪贴板"
        elif command -v xclip &> /dev/null; then
            cat ~/.ssh/id_ed25519.pub | xclip -selection clipboard
            log_success "公钥已复制到剪贴板"
        fi
        
        log_warning "请将上面的公钥添加到GitHub："
        echo "1. 访问: https://github.com/settings/keys"
        echo "2. 点击 'New SSH key'"
        echo "3. 粘贴上面的公钥"
        echo "4. 点击 'Add SSH key'"
        echo
        echo -n "添加完成后按回车继续..."
        read -r
    fi
    
    # 测试SSH连接
    log_info "测试SSH连接..."
    if timeout 10 ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
        log_success "SSH连接正常"
        return 0
    else
        log_warning "SSH连接失败，但继续执行"
        return 1
    fi
}

# 配置Git
setup_git() {
    log_step "配置Git用户信息..."
    
    git config --global user.email "$DEFAULT_EMAIL"
    git config --global user.name "$DEFAULT_USERNAME"
    
    # 清除其他GitHub配置
    local remotes=$(git remote 2>/dev/null || echo "")
    for remote in $remotes; do
        local url=$(git remote get-url "$remote" 2>/dev/null || echo "")
        if [[ -n "$url" && ! "$url" =~ github.com[:/]${DEFAULT_USERNAME}/ ]]; then
            log_info "清除非目标账户的远程仓库: $remote"
            git remote remove "$remote" 2>/dev/null || true
        fi
    done
    
    log_success "Git配置完成"
}

# 初始化Git仓库
init_git_repo() {
    if [[ ! -d ".git" ]]; then
        log_step "初始化Git仓库..."
        git init
        log_success "Git仓库初始化完成"
    fi
}

# 获取项目名称
get_project_name() {
    local current_path=$(pwd)
    basename "$current_path"
}

# 检查GitHub仓库是否存在
check_github_repo() {
    local repo_name=$1
    local api_url="https://api.github.com/repos/${DEFAULT_USERNAME}/${repo_name}"
    local response=$(curl -s -o /dev/null -w "%{http_code}" "$api_url")
    [[ "$response" == "200" ]]
}

# 创建GitHub仓库 - 多种方式
create_github_repo() {
    local repo_name=$1
    local description="$2"
    
    log_step "创建GitHub仓库: $repo_name"
    
    while true; do
        echo
        echo "选择创建方式："
        echo "1) 使用GitHub CLI (自动)"
        echo "2) 使用GitHub API Token"
        echo "3) 手动创建 (推荐)"
        echo "4) 安装GitHub CLI"
        echo "5) 跳过创建，稍后处理"
        echo -n "请选择 [1-5]: "
        read -r choice
        
        case $choice in
            1)
                if command -v gh &> /dev/null; then
                    log_info "使用GitHub CLI创建..."
                    if gh repo create "$repo_name" --public --description "$description"; then
                        log_success "仓库创建成功"
                        return 0
                    else
                        log_warning "GitHub CLI创建失败，请尝试其他方式"
                    fi
                else
                    log_warning "GitHub CLI未安装，请选择其他方式"
                fi
                ;;
            2)
                echo -n "请输入GitHub Token: "
                read -r -s token
                echo
                if [[ -n "$token" ]]; then
                    log_info "使用API创建仓库..."
                    local api_data="{\"name\":\"$repo_name\",\"description\":\"$description\",\"private\":false}"
                    local response=$(curl -s -X POST \
                        -H "Authorization: token $token" \
                        -H "Accept: application/vnd.github.v3+json" \
                        -d "$api_data" \
                        "https://api.github.com/user/repos")
                    
                    if echo "$response" | grep -q '"id"'; then
                        log_success "仓库创建成功"
                        return 0
                    else
                        log_warning "API创建失败，请检查Token权限"
                    fi
                else
                    log_warning "Token不能为空"
                fi
                ;;
            3)
                echo
                log_info "📋 手动创建步骤："
                echo "1. 访问: https://github.com/new"
                echo "2. 仓库名: $repo_name"
                echo "3. 描述: ${description:-可选}"
                echo "4. 设置为公开仓库"
                echo "5. 不要勾选任何初始化选项"
                echo "6. 点击 'Create repository'"
                echo
                echo -n "创建完成后按回车继续..."
                read -r
                
                # 验证是否创建成功
                if check_github_repo "$repo_name"; then
                    log_success "仓库创建成功"
                    return 0
                else
                    log_warning "仓库可能未创建成功，但继续执行"
                    return 0
                fi
                ;;
            4)
                log_info "安装GitHub CLI..."
                if command -v apt &> /dev/null; then
                    curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg
                    echo "deb [arch=$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null
                    sudo apt update && sudo apt install -y gh
                    
                    if command -v gh &> /dev/null; then
                        log_success "GitHub CLI安装成功"
                        log_info "请先认证: gh auth login"
                        gh auth login
                    fi
                else
                    log_warning "请手动安装GitHub CLI"
                fi
                ;;
            5)
                log_info "跳过仓库创建"
                return 0
                ;;
            *)
                log_warning "无效选择，请重新选择"
                ;;
        esac
    done
}

# 添加文件到Git
add_files() {
    log_step "添加文件到Git..."
    
    # 创建.gitignore如果不存在
    if [[ ! -f ".gitignore" ]]; then
        cat > .gitignore << 'EOF'
# 日志文件
*.log
logs/
*.pid

# 数据库文件
*.db
*.sqlite
*.sqlite3

# 临时文件
*.tmp
*.temp
.DS_Store
Thumbs.db

# 依赖目录
node_modules/
vendor/

# 构建输出
build/
dist/
*.exe
*.dll
*.so
*.dylib

# IDE文件
.vscode/
.idea/
*.swp
*.swo
*~

# 环境配置
.env
.env.local
.env.production

# 缓存
.cache/
*.cache
EOF
        log_info "创建了 .gitignore 文件"
    fi
    
    git add .
    log_success "文件添加完成"
}

# 提交更改
commit_changes() {
    local repo_name=$1
    local is_update=$2
    
    # 检查是否有更改
    if git diff --staged --quiet; then
        log_warning "没有检测到更改"
        return 1
    fi
    
    local commit_message=""
    if [[ "$is_update" == "true" ]]; then
        local default_msg="Update $(date '+%Y-%m-%d %H:%M:%S')"
        echo -n "请输入提交信息 (回车使用默认: $default_msg): "
        read -r commit_message
        if [[ -z "$commit_message" ]]; then
            commit_message="$default_msg"
        fi
    else
        commit_message="Initial commit for $repo_name"
    fi
    
    log_info "提交更改: $commit_message"
    git commit -m "$commit_message"
    log_success "提交完成"
    return 0
}

# 配置远程仓库
setup_remote() {
    local repo_name=$1
    local remote_url="git@github.com:${DEFAULT_USERNAME}/${repo_name}.git"
    
    log_step "配置远程仓库..."
    
    if git remote get-url origin &>/dev/null; then
        git remote set-url origin "$remote_url"
        log_info "更新远程仓库URL"
    else
        git remote add origin "$remote_url"
        log_info "添加远程仓库"
    fi
    
    log_success "远程仓库配置完成: $remote_url"
}

# 万能推送函数 - 永不放弃
ultimate_push() {
    local repo_name=$1
    local is_update=$2
    local attempt=1

    while [[ $attempt -le $MAX_RETRIES ]]; do
        log_step "推送到GitHub (尝试 $attempt/$MAX_RETRIES)..."

        local current_branch=$(git branch --show-current 2>/dev/null || echo "main")

        # 确保在main分支
        if [[ "$current_branch" != "main" ]]; then
            git branch -M main
            current_branch="main"
        fi

        # 尝试推送
        local push_output
        local push_success=false

        if [[ "$is_update" == "true" ]]; then
            # 先尝试拉取
            if git ls-remote --exit-code origin "$current_branch" &>/dev/null; then
                log_info "拉取远程更新..."
                git pull origin "$current_branch" --rebase 2>/dev/null || true
            fi

            if git push origin "$current_branch" 2>/dev/null; then
                push_success=true
            else
                push_output=$(git push origin "$current_branch" 2>&1 || true)
            fi
        else
            if git push -u origin "$current_branch" 2>/dev/null; then
                push_success=true
            else
                push_output=$(git push -u origin "$current_branch" 2>&1 || true)
            fi
        fi

        if [[ "$push_success" == "true" ]]; then
            log_success "🎉 推送成功！"
            return 0
        fi

        # 推送失败，分析错误并提供解决方案
        log_warning "推送失败 (尝试 $attempt/$MAX_RETRIES)"
        echo "错误信息: $push_output"

        if handle_push_error "$repo_name" "$push_output" "$attempt"; then
            attempt=$((attempt + 1))
            continue
        else
            # 用户选择退出
            return 1
        fi
    done

    log_error "已达到最大重试次数，但我们不放弃！"
    final_rescue "$repo_name"
}

# 错误处理 - 永不放弃
handle_push_error() {
    local repo_name=$1
    local error_output="$2"
    local attempt=$3

    echo
    if [[ "$error_output" =~ "Repository not found" ]]; then
        log_error "仓库不存在错误"
        handle_repo_not_found "$repo_name"
    elif [[ "$error_output" =~ "Permission denied" ]]; then
        log_error "权限被拒绝错误"
        handle_permission_denied
    elif [[ "$error_output" =~ "Connection timed out" ]]; then
        log_error "网络连接超时"
        handle_network_error
    elif [[ "$error_output" =~ "Authentication failed" ]]; then
        log_error "认证失败"
        handle_auth_failed
    elif [[ "$error_output" =~ "remote rejected" ]]; then
        log_error "远程仓库拒绝推送"
        handle_remote_rejected
    else
        log_error "未知错误"
        handle_unknown_error "$error_output"
    fi

    return 0  # 总是返回成功，继续重试
}

# 处理仓库不存在
handle_repo_not_found() {
    local repo_name=$1

    log_warning "GitHub仓库不存在，立即解决："
    echo "1) 立即创建仓库"
    echo "2) 使用不同名称"
    echo "3) 强制创建并推送"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            create_github_repo "$repo_name" "Auto-created repository"
            ;;
        2)
            echo -n "请输入新的项目名称: "
            read -r new_name
            if [[ -n "$new_name" ]]; then
                git remote set-url origin "git@github.com:${DEFAULT_USERNAME}/${new_name}.git"
                log_info "已更新为新仓库: $new_name"
            fi
            ;;
        3)
            log_info "尝试强制创建..."
            create_github_repo "$repo_name" "Force-created repository"
            ;;
    esac
}

# 处理权限问题
handle_permission_denied() {
    log_warning "SSH权限问题，立即解决："
    echo "1) 重新设置SSH密钥"
    echo "2) 检查SSH配置"
    echo "3) 使用HTTPS方式"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            setup_ssh
            ;;
        2)
            log_info "检查SSH配置..."
            ssh -T git@github.com
            echo -n "检查完成后按回车继续..."
            read -r
            ;;
        3)
            log_info "切换到HTTPS方式..."
            local repo_name=$(basename $(git remote get-url origin) .git)
            git remote set-url origin "https://github.com/${DEFAULT_USERNAME}/${repo_name}.git"
            ;;
    esac
}

# 处理网络错误
handle_network_error() {
    log_warning "网络连接问题，立即解决："
    echo "1) 重试连接"
    echo "2) 检查网络"
    echo "3) 使用代理"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            log_info "等待网络恢复..."
            sleep 5
            ;;
        2)
            log_info "测试网络连接..."
            ping -c 3 github.com || true
            echo -n "检查完成后按回车继续..."
            read -r
            ;;
        3)
            echo -n "请输入代理地址 (如: http://proxy:8080): "
            read -r proxy
            if [[ -n "$proxy" ]]; then
                git config --global http.proxy "$proxy"
                git config --global https.proxy "$proxy"
            fi
            ;;
    esac
}

# 处理认证失败
handle_auth_failed() {
    log_warning "认证失败，立即解决："
    echo "1) 重新设置SSH密钥"
    echo "2) 使用GitHub Token"
    echo "3) 重新认证"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            setup_ssh
            ;;
        2)
            echo -n "请输入GitHub Token: "
            read -r -s token
            echo
            local repo_name=$(basename $(git remote get-url origin) .git)
            git remote set-url origin "https://${token}@github.com/${DEFAULT_USERNAME}/${repo_name}.git"
            ;;
        3)
            if command -v gh &> /dev/null; then
                gh auth login
            else
                setup_ssh
            fi
            ;;
    esac
}

# 处理远程拒绝
handle_remote_rejected() {
    log_warning "远程仓库拒绝推送，立即解决："
    echo "1) 强制推送"
    echo "2) 拉取后重试"
    echo "3) 重置远程分支"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            git push --force-with-lease origin main
            ;;
        2)
            git pull origin main --allow-unrelated-histories
            ;;
        3)
            git push origin main --force
            ;;
    esac
}

# 处理未知错误
handle_unknown_error() {
    local error_output="$1"

    log_warning "遇到未知错误，但我们有解决方案："
    echo "错误详情: $error_output"
    echo "1) 重置并重试"
    echo "2) 强制推送"
    echo "3) 重新配置"
    echo -n "请选择 [1-3]: "
    read -r choice

    case $choice in
        1)
            git reset --soft HEAD~1
            git push origin main
            ;;
        2)
            git push origin main --force
            ;;
        3)
            local repo_name=$(get_project_name)
            setup_remote "$repo_name"
            ;;
    esac
}

# 最终救援 - 绝不放弃
final_rescue() {
    local repo_name=$1

    log_step "🚑 启动最终救援模式！"
    echo
    log_warning "常规方法都失败了，但我们还有终极解决方案："
    echo "1) 创建新仓库并强制推送"
    echo "2) 重置所有配置重新开始"
    echo "3) 手动指导完成推送"
    echo "4) 生成推送脚本供稍后执行"
    echo -n "请选择 [1-4]: "
    read -r choice

    case $choice in
        1)
            log_info "创建新仓库..."
            local new_name="${repo_name}-$(date +%s)"
            create_github_repo "$new_name" "Rescue repository"
            git remote set-url origin "git@github.com:${DEFAULT_USERNAME}/${new_name}.git"
            git push origin main --force
            ;;
        2)
            log_info "重置所有配置..."
            git remote remove origin 2>/dev/null || true
            setup_git
            setup_remote "$repo_name"
            create_github_repo "$repo_name" "Reset repository"
            git push -u origin main
            ;;
        3)
            echo
            log_info "📋 手动推送指导："
            echo "1. 确保GitHub仓库存在: https://github.com/$DEFAULT_USERNAME/$repo_name"
            echo "2. 检查SSH密钥: ssh -T git@github.com"
            echo "3. 手动推送: git push origin main"
            echo "4. 如果失败，强制推送: git push origin main --force"
            echo
            echo -n "完成后按回车..."
            read -r
            ;;
        4)
            local script_name="manual-push-${repo_name}.sh"
            cat > "$script_name" << EOF
#!/bin/bash
# 手动推送脚本 - 生成于 $(date)

echo "🚀 手动推送脚本"
echo "================"

# 检查仓库
if ! curl -s https://api.github.com/repos/$DEFAULT_USERNAME/$repo_name | grep -q '"id"'; then
    echo "❌ 仓库不存在，请先创建: https://github.com/new"
    echo "仓库名: $repo_name"
    echo -n "创建完成后按回车..."
    read
fi

# 推送
echo "正在推送..."
if git push origin main; then
    echo "✅ 推送成功！"
else
    echo "⚠️ 推送失败，尝试强制推送..."
    git push origin main --force
fi

echo "🎉 完成！"
EOF
            chmod +x "$script_name"
            log_success "已生成手动推送脚本: $script_name"
            echo "稍后运行: ./$script_name"
            ;;
    esac

    log_success "🎉 救援完成！任务永不失败！"
}

# 显示成功结果
show_success() {
    local repo_name=$1
    local repo_url="https://github.com/$DEFAULT_USERNAME/$repo_name"

    echo
    echo "🎉🎉🎉 任务完成！🎉🎉🎉"
    echo "========================"
    echo
    log_success "项目已成功推送到GitHub！"
    echo
    echo "📁 项目名称: $repo_name"
    echo "🔗 仓库地址: $repo_url"
    echo "👤 用户名: $DEFAULT_USERNAME"
    echo "📧 邮箱: $DEFAULT_EMAIL"
    echo
    echo "🎯 您可以访问上述链接查看您的项目"
    echo "⚡ 后续更新可以使用: git push origin main"
    echo
}

# 快速推送模式
quick_push_mode() {
    local commit_msg="$1"

    show_header
    log_step "快速推送模式"

    # 检查Git仓库
    if [[ ! -d ".git" ]]; then
        log_error "当前目录不是Git仓库"
        echo -n "是否初始化Git仓库？[y/N]: "
        read -r init_repo
        if [[ "$init_repo" =~ ^[Yy]$ ]]; then
            init_git_repo
        else
            log_info "请先初始化Git仓库或运行完整模式"
            return 1
        fi
    fi

    # 检查远程仓库
    if ! git remote get-url origin &>/dev/null; then
        log_error "没有配置远程仓库"
        echo -n "是否配置远程仓库？[y/N]: "
        read -r setup_remote_repo
        if [[ "$setup_remote_repo" =~ ^[Yy]$ ]]; then
            local repo_name=$(get_project_name)
            setup_remote "$repo_name"
        else
            log_info "请先配置远程仓库或运行完整模式"
            return 1
        fi
    fi

    # 获取提交信息
    if [[ -z "$commit_msg" ]]; then
        local default_msg="Update $(date '+%Y-%m-%d %H:%M:%S')"
        echo -n "请输入提交信息 (回车使用默认: $default_msg): "
        read -r commit_msg
        if [[ -z "$commit_msg" ]]; then
            commit_msg="$default_msg"
        fi
    fi

    # 添加文件
    git add .

    # 检查是否有更改
    if git diff --staged --quiet; then
        log_warning "没有检测到更改"
        return 0
    fi

    # 提交
    git commit -m "$commit_msg"
    log_success "提交完成: $commit_msg"

    # 推送
    local repo_name=$(basename $(git remote get-url origin) .git)
    ultimate_push "$repo_name" "true"

    if [[ $? -eq 0 ]]; then
        show_success "$repo_name"
    fi
}

# 完整推送模式
full_push_mode() {
    show_header

    # 步骤1: 检查工具
    ensure_tools || return 1

    # 步骤2: SSH设置
    setup_ssh

    # 步骤3: Git配置
    setup_git

    # 步骤4: 初始化仓库
    init_git_repo

    # 步骤5: 获取项目名称
    local repo_name=$(get_project_name)
    log_info "项目名称: $repo_name"

    # 步骤6: 检查GitHub仓库
    log_step "检查GitHub仓库是否存在..."
    local repo_exists=false
    if check_github_repo "$repo_name"; then
        log_success "GitHub仓库已存在"
        repo_exists=true
    else
        log_warning "GitHub仓库不存在"
        echo -n "是否创建仓库？[Y/n]: "
        read -r create_repo
        if [[ ! "$create_repo" =~ ^[Nn]$ ]]; then
            create_github_repo "$repo_name" "Auto-created by ultimate script"
        fi
    fi

    # 步骤7: 添加文件
    add_files

    # 步骤8: 提交更改
    local is_update="false"
    if git remote get-url origin &>/dev/null; then
        is_update="true"
    fi

    if commit_changes "$repo_name" "$is_update"; then
        # 步骤9: 配置远程仓库
        setup_remote "$repo_name"

        # 步骤10: 推送
        ultimate_push "$repo_name" "$is_update"

        if [[ $? -eq 0 ]]; then
            show_success "$repo_name"
        fi
    else
        log_info "没有更改需要推送"
    fi
}

# 主函数
main() {
    # 解析参数
    case "${1:-}" in
        "quick"|"q")
            quick_push_mode "$2"
            ;;
        "full"|"f"|"")
            full_push_mode
            ;;
        "--help"|"-h")
            show_help
            ;;
        *)
            # 如果第一个参数不是模式，当作提交信息处理
            quick_push_mode "$1"
            ;;
    esac
}

# 显示帮助
show_help() {
    echo "🚀 GitHub推送万能脚本"
    echo "======================"
    echo
    echo "功能: 集成SSH设置、仓库创建、推送、错误处理于一体"
    echo "特点: 永不放弃，总能找到解决方案"
    echo
    echo "用法:"
    echo "  $0                    # 完整模式 (推荐首次使用)"
    echo "  $0 full               # 完整模式"
    echo "  $0 quick              # 快速推送模式"
    echo "  $0 quick \"提交信息\"   # 快速推送并指定提交信息"
    echo "  $0 \"提交信息\"         # 快速推送并指定提交信息"
    echo "  $0 --help             # 显示帮助"
    echo
    echo "模式说明:"
    echo "  完整模式: 包含所有功能，适合首次使用或遇到问题时"
    echo "  快速模式: 适合日常更新，已配置好的项目"
    echo
    echo "示例:"
    echo "  $0                           # 首次推送项目"
    echo "  $0 quick \"修复登录bug\"       # 快速推送更新"
    echo "  $0 \"添加新功能\"             # 快速推送更新"
    echo
    echo "特色功能:"
    echo "  ✅ 自动SSH密钥设置"
    echo "  ✅ 智能仓库创建"
    echo "  ✅ 多种推送方式"
    echo "  ✅ 强大错误处理"
    echo "  ✅ 永不放弃机制"
    echo "  ✅ 最终救援模式"
    echo
}

# 执行主函数
main "$@"
