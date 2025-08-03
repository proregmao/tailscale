#!/bin/bash

# 快速推送脚本 - 简化版
# 用于已配置好的项目快速推送

set -e

# 默认配置
DEFAULT_EMAIL="proreg@163.com"
DEFAULT_USERNAME="proregmao"

# 颜色输出
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
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

# 清除其他GitHub配置
clean_other_github_configs() {
    log_info "检查并清除其他GitHub配置..."

    # 检查当前远程仓库是否指向正确的账户
    local current_url=$(git remote get-url origin 2>/dev/null || echo "")

    if [[ -n "$current_url" ]] && [[ ! "$current_url" =~ github.com[:/]${DEFAULT_USERNAME}/ ]]; then
        log_warning "检测到非目标GitHub账户的远程仓库，正在清除..."

        # 清除所有远程仓库
        local remotes=$(git remote)
        for remote in $remotes; do
            git remote remove "$remote" 2>/dev/null || true
        done

        log_warning "已清除其他GitHub配置，请重新运行完整推送脚本"
        echo "请运行: ./push-to-github.sh"
        exit 1
    fi
}

# 快速推送函数
quick_push() {
    local commit_msg="$1"

    # 如果没有提供提交信息，使用默认时间戳
    if [[ -z "$commit_msg" ]]; then
        commit_msg="Update $(date '+%Y-%m-%d %H:%M:%S')"
    fi
    
    log_info "开始快速推送..."
    
    # 配置Git用户信息（如果需要）
    git config user.email "$DEFAULT_EMAIL" 2>/dev/null || true
    git config user.name "$DEFAULT_USERNAME" 2>/dev/null || true
    
    # 添加所有更改
    log_info "添加文件..."
    git add .
    
    # 检查是否有更改
    if git diff --staged --quiet; then
        log_warning "没有检测到更改"
        return 0
    fi
    
    # 提交更改
    log_info "提交更改: $commit_msg"
    git commit -m "$commit_msg"
    
    # 获取当前分支
    current_branch=$(git branch --show-current)
    
    # 推送
    log_info "推送到远程仓库 ($current_branch)..."
    
    # 如果远程分支存在，先拉取
    if git ls-remote --exit-code origin "$current_branch" &>/dev/null; then
        git pull origin "$current_branch" --rebase
    fi
    
    git push origin "$current_branch"
    
    log_success "推送完成！"
}

# 主函数
main() {
    echo "⚡ 快速推送到GitHub"
    echo "=================="
    
    # 检查是否在Git仓库中
    if [[ ! -d ".git" ]]; then
        echo "错误：当前目录不是Git仓库"
        echo "请先运行完整的推送脚本：./push-to-github.sh"
        exit 1
    fi
    
    # 检查是否有远程仓库
    if ! git remote get-url origin &>/dev/null; then
        echo "错误：没有配置远程仓库"
        echo "请先运行完整的推送脚本：./push-to-github.sh"
        exit 1
    fi

    # 清除其他GitHub配置检查
    clean_other_github_configs
    
    # 获取提交信息
    if [[ -n "$1" ]]; then
        commit_msg="$1"
    else
        echo -n "请输入提交信息 (回车使用默认): "
        read -r commit_msg
    fi
    
    # 执行快速推送
    quick_push "$commit_msg"
    
    # 显示仓库信息
    repo_url=$(git remote get-url origin | sed 's/git@github.com:/https:\/\/github.com\//' | sed 's/\.git$//')
    echo
    log_success "🎉 推送完成！"
    echo "🔗 仓库地址: $repo_url"
}

# 使用说明
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "快速推送脚本使用说明"
    echo "==================="
    echo
    echo "用法："
    echo "  $0                    # 交互式输入提交信息"
    echo "  $0 \"提交信息\"         # 直接指定提交信息"
    echo "  $0 --help            # 显示帮助"
    echo
    echo "示例："
    echo "  $0 \"修复登录bug\""
    echo "  $0 \"添加新功能\""
    echo "  $0 \"更新文档\""
    echo
    echo "注意：此脚本用于已配置好的项目快速推送"
    echo "首次使用请运行：./push-to-github.sh"
    exit 0
fi

# 执行主函数
main "$@"
