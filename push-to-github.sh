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

# 检查必要工具
check_dependencies() {
    log_info "检查必要工具..."
    check_command "git"
    check_command "curl"
    log_success "所有必要工具已安装"
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

# 推送到GitHub
push_to_github() {
    local is_update=$1

    log_info "推送到GitHub..."

    # 获取当前分支
    current_branch=$(git branch --show-current 2>/dev/null || echo "main")

    if [[ "$is_update" == "true" ]]; then
        # 更新推送
        log_info "推送分支: $current_branch"

        # 先拉取最新更改避免冲突
        if git ls-remote --exit-code origin "$current_branch" &>/dev/null; then
            log_info "拉取远程更新..."
            git pull origin "$current_branch" --rebase || {
                log_warning "自动合并失败，请手动解决冲突后重新运行脚本"
                exit 1
            }
        fi

        git push origin "$current_branch"
    else
        # 首次推送
        if [[ "$current_branch" != "main" ]]; then
            log_info "切换到main分支"
            git branch -M main
            current_branch="main"
        fi

        log_info "首次推送分支: $current_branch"
        git push -u origin "$current_branch"
    fi

    log_success "推送完成！"
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
        push_to_github "$is_update"
        
        # 显示结果
        show_result "$repo_name"
    else
        log_info "没有更改需要推送"
    fi
}

# 错误处理
trap 'log_error "脚本执行失败，请检查错误信息"; exit 1' ERR

# 执行主函数
main "$@"
