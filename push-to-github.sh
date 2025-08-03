#!/bin/bash

# GitHub推送脚本
# 作者: proregmao
# 功能: 自动推送项目到GitHub，支持新建和更新

set -e  # 遇到错误立即退出

# 默认配置
DEFAULT_EMAIL="proreg@163.com"
DEFAULT_USERNAME="proregmao"
GITHUB_BASE_URL="https://github.com"

# 颜色输出
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 输出函数
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

# 检查命令是否存在
check_command() {
    if ! command -v $1 &> /dev/null; then
        log_error "$1 命令未找到，请先安装"
        exit 1
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
        log_error "不支持的系统，请手动安装GitHub CLI"
        return 1
    fi

    if command -v gh &> /dev/null; then
        log_success "GitHub CLI安装成功"
        log_info "请运行 'gh auth login' 进行认证"
        return 0
    else
        log_error "GitHub CLI安装失败"
        return 1
    fi
}

# 检查必要工具
check_dependencies() {
    log_info "检查必要工具..."
    check_command "git"
    check_command "curl"

    # 检查GitHub CLI（可选）
    if ! command -v gh &> /dev/null; then
        log_warning "GitHub CLI未安装，将无法自动创建仓库"
        echo -n "是否现在安装GitHub CLI？[y/N]: "
        read -r install_gh
        if [[ "$install_gh" =~ ^[Yy]$ ]]; then
            if install_github_cli; then
                echo -n "是否现在进行GitHub认证？[y/N]: "
                read -r auth_gh
                if [[ "$auth_gh" =~ ^[Yy]$ ]]; then
                    gh auth login
                fi
            fi
        fi
    else
        log_success "GitHub CLI已安装"
    fi

    log_success "工具检查完成"
}

# 检查SSH连接
check_ssh_connection() {
    log_info "检查GitHub SSH连接..."

    # 添加GitHub到known_hosts
    if ! ssh-keygen -F github.com &>/dev/null; then
        log_info "添加GitHub SSH密钥到known_hosts..."
        ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
    fi

    # 简化的SSH连接测试
    if timeout 5 ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
        log_success "SSH连接正常"
        return 0
    else
        log_warning "SSH连接测试超时或失败，继续执行"
        log_info "如果推送失败，请检查SSH密钥配置"
        return 1
    fi
}

# 获取当前目录名作为项目名
get_project_name() {
    local current_path=$(pwd)
    basename "$current_path"
}

# 配置Git用户信息
setup_git_config() {
    log_info "配置Git用户信息..."
    
    # 检查是否已配置
    current_email=$(git config --global user.email 2>/dev/null || echo "")
    current_name=$(git config --global user.name 2>/dev/null || echo "")
    
    if [[ "$current_email" != "$DEFAULT_EMAIL" ]]; then
        git config --global user.email "$DEFAULT_EMAIL"
        log_info "设置邮箱: $DEFAULT_EMAIL"
    fi
    
    if [[ "$current_name" != "$DEFAULT_USERNAME" ]]; then
        git config --global user.name "$DEFAULT_USERNAME"
        log_info "设置用户名: $DEFAULT_USERNAME"
    fi
    
    log_success "Git配置完成"
}

# 初始化Git仓库
init_git_repo() {
    if [[ ! -d ".git" ]]; then
        log_info "初始化Git仓库..."
        git init
        log_success "Git仓库初始化完成"
    else
        log_info "Git仓库已存在"
    fi
}

# 检查GitHub仓库是否存在
check_github_repo() {
    local repo_name=$1
    local url="${GITHUB_BASE_URL}/${DEFAULT_USERNAME}/${repo_name}"

    # 使用GitHub API检查仓库（静默检查）
    local api_url="https://api.github.com/repos/${DEFAULT_USERNAME}/${repo_name}"
    local response=$(curl -s -o /dev/null -w "%{http_code}" "$api_url")

    if [[ "$response" == "200" ]]; then
        return 0  # 仓库存在
    else
        return 1  # 仓库不存在
    fi
}

# 创建GitHub仓库
create_github_repo() {
    local repo_name=$1
    local description="$2"

    log_info "尝试创建GitHub仓库: $repo_name"

    # 检查是否有GitHub CLI
    if command -v gh &> /dev/null; then
        log_info "使用GitHub CLI创建仓库..."
        if [[ -n "$description" ]]; then
            gh repo create "$repo_name" --public --description "$description"
        else
            gh repo create "$repo_name" --public
        fi
        log_success "仓库创建成功"
        return 0
    else
        log_warning "GitHub CLI未安装，请手动在GitHub上创建仓库: $repo_name"
        log_info "或者安装GitHub CLI: https://cli.github.com/"
        echo -n "创建完成后按回车继续..."
        read -r
        return 0
    fi
}

# 获取项目名称（重写版本）
get_repo_name() {
    local default_name=$(get_project_name)
    echo "$default_name"
}

# 添加文件到Git
add_files() {
    log_info "添加文件到Git..."
    
    # 创建.gitignore如果不存在
    if [[ ! -f ".gitignore" ]]; then
        cat > .gitignore << EOF
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
frr/
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

# 生成默认提交信息
generate_default_commit_message() {
    local is_update=$1
    local repo_name=$2

    if [[ "$is_update" == "true" ]]; then
        # 更新时使用当前日期时间
        echo "Update $(date '+%Y-%m-%d %H:%M:%S')"
    else
        # 首次提交
        echo "Initial commit for $repo_name"
    fi
}

# 提交更改
commit_changes() {
    local repo_name=$1
    local is_update=$2

    # 检查是否有更改
    if git diff --staged --quiet; then
        log_warning "没有检测到更改，无需提交"
        return 1
    fi

    local commit_message=""
    local default_message=$(generate_default_commit_message "$is_update" "$repo_name")

    if [[ "$is_update" == "true" ]]; then
        echo -n "请输入提交信息 (回车使用默认: $default_message): "
        read -r commit_message

        if [[ -z "$commit_message" ]]; then
            commit_message="$default_message"
        fi
    else
        commit_message="$default_message"
    fi

    log_info "提交更改: $commit_message"
    git commit -m "$commit_message"
    log_success "提交完成"
    return 0
}

# 清除其他GitHub相关配置
clean_other_github_configs() {
    log_info "清除其他GitHub相关配置..."

    # 清除所有远程仓库
    local remotes=$(git remote)
    if [[ -n "$remotes" ]]; then
        for remote in $remotes; do
            log_info "移除远程仓库: $remote"
            git remote remove "$remote" 2>/dev/null || true
        done
    fi

    # 清除GitHub相关的配置
    git config --unset-all remote.origin.url 2>/dev/null || true
    git config --unset-all branch.main.remote 2>/dev/null || true
    git config --unset-all branch.main.merge 2>/dev/null || true
    git config --unset-all branch.master.remote 2>/dev/null || true
    git config --unset-all branch.master.merge 2>/dev/null || true

    log_success "清除完成，确保只连接到您的GitHub账户"
}

# 添加远程仓库
add_remote() {
    local repo_name=$1
    local remote_url="git@github.com:${DEFAULT_USERNAME}/${repo_name}.git"

    log_info "配置远程仓库: $repo_name"
    log_info "目标URL: $remote_url"

    # 检查是否已有远程仓库
    if git remote get-url origin &>/dev/null; then
        # 更新现有远程仓库URL
        git remote set-url origin "$remote_url"
        log_info "更新远程仓库URL"
    else
        # 添加新的远程仓库
        git remote add origin "$remote_url"
        log_info "添加远程仓库"
    fi

    log_success "远程仓库配置完成: $remote_url"
}

# 处理推送错误
handle_push_error() {
    local repo_name=$1
    local error_output="$2"

    echo
    log_error "推送失败！"
    echo "错误信息: $error_output"
    echo

    if [[ "$error_output" =~ "Repository not found" ]]; then
        log_warning "GitHub仓库不存在，请选择解决方案："
        echo "1) 创建新的GitHub仓库"
        echo "2) 使用不同的项目名称"
        echo "3) 手动创建仓库后重试"
        echo "4) 退出脚本"
        echo -n "请选择 [1-4]: "
        read -r choice

        case $choice in
            1)
                create_github_repo_interactive "$repo_name"
                return $?
                ;;
            2)
                echo -n "请输入新的项目名称: "
                read -r new_name
                if [[ -n "$new_name" ]]; then
                    # 更新远程仓库URL
                    git remote set-url origin "git@github.com:${DEFAULT_USERNAME}/${new_name}.git"
                    log_info "已更新远程仓库为: $new_name"
                    return 0
                else
                    log_error "项目名称不能为空"
                    return 1
                fi
                ;;
            3)
                echo
                log_info "请手动创建GitHub仓库："
                echo "1. 访问: https://github.com/new"
                echo "2. 仓库名: $repo_name"
                echo "3. 设置为公开仓库"
                echo "4. 不要初始化README、.gitignore或许可证"
                echo
                echo -n "创建完成后按回车继续..."
                read -r
                return 0
                ;;
            4)
                log_info "用户选择退出"
                exit 0
                ;;
            *)
                log_error "无效选择"
                return 1
                ;;
        esac
    elif [[ "$error_output" =~ "Permission denied" ]]; then
        log_warning "权限被拒绝，可能的解决方案："
        echo "1) 检查SSH密钥配置"
        echo "2) 重新设置SSH密钥"
        echo "3) 退出脚本"
        echo -n "请选择 [1-3]: "
        read -r choice

        case $choice in
            1)
                echo
                log_info "请检查SSH密钥配置："
                echo "1. 测试连接: ssh -T git@github.com"
                echo "2. 检查密钥: ls -la ~/.ssh/"
                echo "3. 查看公钥: cat ~/.ssh/id_ed25519.pub"
                echo
                echo -n "检查完成后按回车继续..."
                read -r
                return 0
                ;;
            2)
                log_info "运行SSH设置脚本..."
                if [[ -f "./setup-github-ssh.sh" ]]; then
                    ./setup-github-ssh.sh
                    return $?
                else
                    log_error "SSH设置脚本不存在"
                    return 1
                fi
                ;;
            3)
                log_info "用户选择退出"
                exit 0
                ;;
            *)
                log_error "无效选择"
                return 1
                ;;
        esac
    else
        log_warning "未知错误，可能的解决方案："
        echo "1) 重试推送"
        echo "2) 检查网络连接"
        echo "3) 退出脚本"
        echo -n "请选择 [1-3]: "
        read -r choice

        case $choice in
            1)
                return 0
                ;;
            2)
                echo
                log_info "请检查网络连接："
                echo "1. 测试网络: ping github.com"
                echo "2. 测试SSH: ssh -T git@github.com"
                echo
                echo -n "检查完成后按回车继续..."
                read -r
                return 0
                ;;
            3)
                log_info "用户选择退出"
                exit 0
                ;;
            *)
                log_error "无效选择"
                return 1
                ;;
        esac
    fi
}

# 使用GitHub API创建仓库
create_repo_with_api() {
    local repo_name=$1
    local repo_description="$2"

    log_info "使用GitHub API创建仓库..."

    # 检查是否有GitHub token
    if [[ -z "$GITHUB_TOKEN" ]]; then
        log_warning "未设置GITHUB_TOKEN环境变量"
        return 1
    fi

    local api_data="{\"name\":\"$repo_name\",\"description\":\"$repo_description\",\"private\":false}"

    local response=$(curl -s -X POST \
        -H "Authorization: token $GITHUB_TOKEN" \
        -H "Accept: application/vnd.github.v3+json" \
        -d "$api_data" \
        "https://api.github.com/user/repos")

    if echo "$response" | grep -q '"id"'; then
        log_success "仓库创建成功"
        return 0
    else
        log_error "API创建失败: $(echo "$response" | grep -o '"message":"[^"]*"' | cut -d'"' -f4)"
        return 1
    fi
}

# 使用curl创建仓库（无token）
create_repo_with_curl() {
    local repo_name=$1

    log_info "尝试使用SSH方式创建仓库..."

    # 创建一个临时的空仓库推送
    if git push origin main 2>&1 | grep -q "create a pull request"; then
        log_success "仓库已自动创建"
        return 0
    fi

    return 1
}

# 交互式创建GitHub仓库
create_github_repo_interactive() {
    local repo_name=$1

    log_info "尝试创建GitHub仓库: $repo_name"

    # 方法1: 尝试GitHub CLI
    if command -v gh &> /dev/null; then
        log_info "使用GitHub CLI创建仓库..."
        echo -n "请输入仓库描述 (可选): "
        read -r repo_description

        local gh_cmd="gh repo create $repo_name --public"
        if [[ -n "$repo_description" ]]; then
            gh_cmd="$gh_cmd --description \"$repo_description\""
        fi

        if eval "$gh_cmd"; then
            log_success "GitHub CLI创建成功"
            return 0
        else
            log_warning "GitHub CLI创建失败，尝试其他方法..."
        fi
    fi

    # 方法2: 尝试GitHub API
    if [[ -n "$GITHUB_TOKEN" ]]; then
        echo -n "请输入仓库描述 (可选): "
        read -r repo_description
        if create_repo_with_api "$repo_name" "$repo_description"; then
            return 0
        else
            log_warning "GitHub API创建失败，尝试其他方法..."
        fi
    fi

    # 方法3: 提供多种创建选项
    echo
    log_warning "自动创建失败，请选择创建方式："
    echo "1) 手动在浏览器中创建"
    echo "2) 安装GitHub CLI后自动创建"
    echo "3) 设置GitHub Token后自动创建"
    echo "4) 跳过创建，稍后手动处理"
    echo -n "请选择 [1-4]: "
    read -r create_choice

    case $create_choice in
        1)
            log_info "手动创建步骤："
            echo "1. 访问: https://github.com/new"
            echo "2. 仓库名: $repo_name"
            echo "3. 设置为公开仓库"
            echo "4. 不要初始化README、.gitignore或许可证"
            echo "5. 点击 'Create repository'"
            echo
            echo -n "创建完成后按回车继续..."
            read -r
            return 0
            ;;
        2)
            log_info "安装GitHub CLI："
            echo "curl -fsSL https://cli.github.com/packages/githubcli-archive-keyring.gpg | sudo dd of=/usr/share/keyrings/githubcli-archive-keyring.gpg"
            echo "echo \"deb [arch=\$(dpkg --print-architecture) signed-by=/usr/share/keyrings/githubcli-archive-keyring.gpg] https://cli.github.com/packages stable main\" | sudo tee /etc/apt/sources.list.d/github-cli.list > /dev/null"
            echo "sudo apt update && sudo apt install gh"
            echo "gh auth login"
            echo
            echo -n "安装完成后按回车重新运行脚本..."
            read -r
            return 1
            ;;
        3)
            log_info "设置GitHub Token："
            echo "1. 访问: https://github.com/settings/tokens"
            echo "2. 点击 'Generate new token (classic)'"
            echo "3. 勾选 'repo' 权限"
            echo "4. 复制生成的token"
            echo "5. 运行: export GITHUB_TOKEN=your_token_here"
            echo
            echo -n "设置完成后按回车重新运行脚本..."
            read -r
            return 1
            ;;
        4)
            log_info "跳过仓库创建，请稍后手动创建"
            return 1
            ;;
        *)
            log_error "无效选择"
            return 1
            ;;
    esac
}

# 推送到GitHub（带错误处理）
push_to_github() {
    local is_update=$1
    local repo_name=$2
    local max_retries=3
    local retry_count=0

    while [[ $retry_count -lt $max_retries ]]; do
        log_info "推送到GitHub... (尝试 $((retry_count + 1))/$max_retries)"

        # 获取当前分支
        current_branch=$(git branch --show-current 2>/dev/null || echo "main")

        # 执行推送
        local push_output
        local push_result

        if [[ "$is_update" == "true" ]]; then
            # 更新推送
            log_info "推送分支: $current_branch"

            # 先拉取最新更改避免冲突
            if git ls-remote --exit-code origin "$current_branch" &>/dev/null; then
                log_info "拉取远程更新..."
                if ! git pull origin "$current_branch" --rebase; then
                    log_warning "自动合并失败，请手动解决冲突后重新运行脚本"
                    return 1
                fi
            fi

            if git push origin "$current_branch"; then
                push_result=0
                push_output="推送成功"
            else
                push_result=$?
                push_output=$(git push origin "$current_branch" 2>&1 || true)
            fi
        else
            # 首次推送
            if [[ "$current_branch" != "main" ]]; then
                log_info "切换到main分支"
                git branch -M main
                current_branch="main"
            fi

            log_info "首次推送分支: $current_branch"
            if git push -u origin "$current_branch"; then
                push_result=0
                push_output="首次推送成功"
            else
                push_result=$?
                push_output=$(git push -u origin "$current_branch" 2>&1 || true)
            fi
        fi

        # 检查推送结果
        if [[ $push_result -eq 0 ]]; then
            log_success "推送完成！"
            return 0
        else
            log_warning "推送失败 (尝试 $((retry_count + 1))/$max_retries)"

            # 如果不是最后一次尝试，提供解决方案
            if [[ $retry_count -lt $((max_retries - 1)) ]]; then
                if handle_push_error "$repo_name" "$push_output"; then
                    retry_count=$((retry_count + 1))
                    continue
                else
                    return 1
                fi
            else
                # 最后一次尝试失败
                log_error "推送失败，已达到最大重试次数"
                handle_push_error "$repo_name" "$push_output"
                return 1
            fi
        fi

        retry_count=$((retry_count + 1))
    done

    return 1
}

# 显示结果
show_result() {
    local repo_name=$1
    local repo_url="${GITHUB_BASE_URL}/${DEFAULT_USERNAME}/${repo_name}"
    
    echo
    log_success "🎉 项目已成功推送到GitHub！"
    echo
    echo "📁 项目名称: $repo_name"
    echo "🔗 仓库地址: $repo_url"
    echo "👤 用户名: $DEFAULT_USERNAME"
    echo "📧 邮箱: $DEFAULT_EMAIL"
    echo
    log_info "您可以访问上述链接查看您的项目"
}

# 主函数
main() {
    echo "🚀 GitHub项目推送脚本"
    echo "========================"
    echo
    
    # 检查依赖
    check_dependencies

    # 检查SSH连接
    check_ssh_connection

    # 配置Git
    setup_git_config
    
    # 初始化仓库
    init_git_repo
    
    # 获取项目名称
    repo_name=$(get_repo_name)

    # 验证仓库名称
    if [[ -z "$repo_name" || "$repo_name" =~ [^a-zA-Z0-9._-] ]]; then
        log_error "无效的仓库名称: $repo_name"
        log_info "仓库名称只能包含字母、数字、点、下划线和连字符"
        exit 1
    fi

    log_info "使用项目名称: $repo_name"

    # 检查GitHub仓库是否存在
    log_info "检查GitHub仓库是否存在: $repo_name"
    if check_github_repo "$repo_name"; then
        log_warning "GitHub仓库 '$repo_name' 已存在"
        echo "选择操作:"
        echo "1) 使用现有仓库 (更新)"
        echo "2) 输入新的项目名称"
        echo -n "请选择 [1-2]: "
        read -r choice

        case $choice in
            1)
                log_info "使用现有仓库: $repo_name"
                ;;
            2)
                echo -n "请输入新的项目名称: "
                read -r new_name

                if [[ -z "$new_name" ]]; then
                    log_error "项目名称不能为空"
                    exit 1
                fi

                if check_github_repo "$new_name"; then
                    log_error "仓库 '$new_name' 也已存在，请选择其他名称"
                    exit 1
                fi
                repo_name="$new_name"
                log_info "使用新项目名称: $repo_name"
                ;;
            *)
                log_error "无效选择"
                exit 1
                ;;
        esac
    fi

    # 检查是否为更新
    is_update="false"
    repo_exists=$(check_github_repo "$repo_name" && echo "true" || echo "false")

    if git remote get-url origin &>/dev/null; then
        is_update="true"
        log_info "检测到现有本地仓库，执行更新操作"
    else
        if [[ "$repo_exists" == "false" ]]; then
            log_info "新建仓库操作"
            echo -n "请输入仓库描述 (可选): "
            read -r repo_description
            create_github_repo "$repo_name" "$repo_description"
        else
            log_info "连接到现有GitHub仓库"
        fi
    fi
    
    # 清除其他GitHub配置（确保只连接到您的账户）
    if [[ "$is_update" != "true" ]] || [[ ! $(git remote get-url origin 2>/dev/null) =~ github.com[:/]${DEFAULT_USERNAME}/ ]]; then
        clean_other_github_configs
    fi

    # 添加文件
    add_files

    # 提交更改
    if commit_changes "$repo_name" "$is_update"; then
        # 配置远程仓库
        add_remote "$repo_name"
        
        # 推送到GitHub
        push_to_github "$is_update" "$repo_name"
        
        # 显示结果
        show_result "$repo_name"
    else
        log_info "没有更改需要推送"
    fi
}

# 执行主函数
main "$@"
