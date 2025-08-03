#!/bin/bash

# GitHub配置清理脚本
# 清除所有GitHub相关配置，确保只连接到指定账户

set -e

# 默认配置
DEFAULT_EMAIL="proreg@163.com"
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

# 显示当前Git配置
show_current_config() {
    echo
    log_info "当前Git配置："
    echo "=================================="
    
    # 显示用户信息
    local current_email=$(git config --global user.email 2>/dev/null || echo "未设置")
    local current_name=$(git config --global user.name 2>/dev/null || echo "未设置")
    echo "全局用户邮箱: $current_email"
    echo "全局用户名称: $current_name"
    
    # 显示本地仓库信息
    if [[ -d ".git" ]]; then
        echo
        echo "本地仓库信息："
        local local_email=$(git config user.email 2>/dev/null || echo "未设置")
        local local_name=$(git config user.name 2>/dev/null || echo "未设置")
        echo "本地用户邮箱: $local_email"
        echo "本地用户名称: $local_name"
        
        # 显示远程仓库
        echo
        echo "远程仓库："
        if git remote -v &>/dev/null; then
            git remote -v
        else
            echo "无远程仓库"
        fi
        
        # 显示分支信息
        echo
        echo "分支信息："
        git branch -a 2>/dev/null || echo "无分支信息"
    else
        echo
        log_warning "当前目录不是Git仓库"
    fi
    
    echo "=================================="
    echo
}

# 清除全局Git配置中的其他GitHub账户信息
clean_global_config() {
    log_info "清理全局Git配置..."
    
    # 设置正确的用户信息
    git config --global user.email "$DEFAULT_EMAIL"
    git config --global user.name "$DEFAULT_USERNAME"
    
    # 清除可能的其他GitHub相关配置
    git config --global --unset-all credential.helper 2>/dev/null || true
    git config --global --unset-all github.user 2>/dev/null || true
    git config --global --unset-all github.token 2>/dev/null || true
    
    log_success "全局配置已清理并设置为目标账户"
}

# 清除本地仓库的GitHub配置
clean_local_config() {
    if [[ ! -d ".git" ]]; then
        log_warning "当前目录不是Git仓库，跳过本地配置清理"
        return 0
    fi
    
    log_info "清理本地仓库配置..."
    
    # 设置本地用户信息
    git config user.email "$DEFAULT_EMAIL"
    git config user.name "$DEFAULT_USERNAME"
    
    # 清除所有远程仓库
    local remotes=$(git remote 2>/dev/null || echo "")
    if [[ -n "$remotes" ]]; then
        for remote in $remotes; do
            log_info "移除远程仓库: $remote"
            git remote remove "$remote" 2>/dev/null || true
        done
    fi
    
    # 清除分支的远程跟踪配置
    git config --unset-all branch.main.remote 2>/dev/null || true
    git config --unset-all branch.main.merge 2>/dev/null || true
    git config --unset-all branch.master.remote 2>/dev/null || true
    git config --unset-all branch.master.merge 2>/dev/null || true
    
    # 清除其他可能的GitHub相关配置
    git config --unset-all remote.origin.url 2>/dev/null || true
    git config --unset-all credential.helper 2>/dev/null || true
    
    log_success "本地仓库配置已清理"
}

# 清除SSH known_hosts中的其他GitHub条目
clean_ssh_config() {
    log_info "清理SSH配置..."
    
    if [[ -f ~/.ssh/known_hosts ]]; then
        # 备份原文件
        cp ~/.ssh/known_hosts ~/.ssh/known_hosts.backup.$(date +%Y%m%d_%H%M%S)
        
        # 移除所有GitHub相关条目
        sed -i '/github\.com/d' ~/.ssh/known_hosts 2>/dev/null || true
        
        # 重新添加官方GitHub SSH密钥
        ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
        
        log_success "SSH配置已清理并重新添加官方GitHub密钥"
    else
        log_info "未找到SSH known_hosts文件"
    fi
}

# 清除Git凭据存储
clean_credentials() {
    log_info "清理Git凭据..."
    
    # 清除不同系统的凭据存储
    if command -v git-credential-manager &> /dev/null; then
        # Windows Git Credential Manager
        git-credential-manager delete --url=https://github.com 2>/dev/null || true
    fi
    
    if command -v git-credential-osxkeychain &> /dev/null; then
        # macOS Keychain
        git-credential-osxkeychain erase <<< "protocol=https
host=github.com" 2>/dev/null || true
    fi
    
    # 清除可能的凭据文件
    rm -f ~/.git-credentials 2>/dev/null || true
    
    log_success "Git凭据已清理"
}

# 验证清理结果
verify_cleanup() {
    log_info "验证清理结果..."
    
    local issues=0
    
    # 检查全局配置
    local global_email=$(git config --global user.email 2>/dev/null || echo "")
    local global_name=$(git config --global user.name 2>/dev/null || echo "")
    
    if [[ "$global_email" != "$DEFAULT_EMAIL" ]]; then
        log_error "全局邮箱配置不正确: $global_email"
        issues=$((issues + 1))
    fi
    
    if [[ "$global_name" != "$DEFAULT_USERNAME" ]]; then
        log_error "全局用户名配置不正确: $global_name"
        issues=$((issues + 1))
    fi
    
    # 检查本地配置（如果是Git仓库）
    if [[ -d ".git" ]]; then
        local local_email=$(git config user.email 2>/dev/null || echo "")
        local local_name=$(git config user.name 2>/dev/null || echo "")
        
        if [[ "$local_email" != "$DEFAULT_EMAIL" ]]; then
            log_error "本地邮箱配置不正确: $local_email"
            issues=$((issues + 1))
        fi
        
        if [[ "$local_name" != "$DEFAULT_USERNAME" ]]; then
            log_error "本地用户名配置不正确: $local_name"
            issues=$((issues + 1))
        fi
        
        # 检查是否还有远程仓库
        if git remote &>/dev/null && [[ -n "$(git remote)" ]]; then
            log_warning "仍有远程仓库存在："
            git remote -v
            issues=$((issues + 1))
        fi
    fi
    
    if [[ $issues -eq 0 ]]; then
        log_success "✅ 清理验证通过！"
        return 0
    else
        log_error "❌ 发现 $issues 个问题，请检查"
        return 1
    fi
}

# 主函数
main() {
    echo "🧹 GitHub配置清理工具"
    echo "======================"
    echo "目标账户: $DEFAULT_USERNAME ($DEFAULT_EMAIL)"
    echo
    
    # 显示当前配置
    show_current_config
    
    # 确认清理
    echo -n "是否要清理所有GitHub配置并设置为目标账户？[y/N]: "
    read -r confirm
    
    if [[ ! "$confirm" =~ ^[Yy]$ ]]; then
        log_info "操作已取消"
        exit 0
    fi
    
    echo
    log_info "开始清理GitHub配置..."
    
    # 执行清理
    clean_global_config
    clean_local_config
    clean_ssh_config
    clean_credentials
    
    echo
    log_info "清理完成，正在验证..."
    
    # 验证结果
    if verify_cleanup; then
        echo
        log_success "🎉 GitHub配置清理完成！"
        echo
        log_info "现在您可以："
        echo "1. 运行 ./setup-github-ssh.sh 设置SSH密钥"
        echo "2. 运行 ./push-to-github.sh 推送项目"
        echo
        log_info "清理后的配置："
        show_current_config
    else
        echo
        log_error "清理过程中发现问题，请手动检查"
    fi
}

# 使用说明
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "GitHub配置清理工具"
    echo "=================="
    echo
    echo "此脚本会："
    echo "1. 清除全局Git配置中的其他GitHub账户信息"
    echo "2. 清除本地仓库的所有远程仓库配置"
    echo "3. 清理SSH known_hosts中的GitHub条目"
    echo "4. 清除Git凭据存储"
    echo "5. 设置为目标GitHub账户配置"
    echo
    echo "目标账户: $DEFAULT_USERNAME ($DEFAULT_EMAIL)"
    echo
    echo "用法："
    echo "  $0              # 运行清理工具"
    echo "  $0 --help       # 显示帮助"
    echo
    echo "注意：此操作会清除所有GitHub相关配置！"
    exit 0
fi

# 执行主函数
main "$@"
