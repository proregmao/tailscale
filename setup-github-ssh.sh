#!/bin/bash

# GitHub SSH密钥设置脚本
# 帮助用户快速配置GitHub SSH访问

set -e

# 默认配置
DEFAULT_EMAIL="proreg@163.com"

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

# 检查SSH密钥是否存在
check_ssh_key() {
    if [[ -f ~/.ssh/id_ed25519 ]] || [[ -f ~/.ssh/id_rsa ]]; then
        return 0  # 密钥存在
    else
        return 1  # 密钥不存在
    fi
}

# 生成SSH密钥
generate_ssh_key() {
    local email="$1"
    
    log_info "生成SSH密钥..."
    
    # 创建.ssh目录
    mkdir -p ~/.ssh
    chmod 700 ~/.ssh
    
    # 生成ED25519密钥（推荐）
    ssh-keygen -t ed25519 -C "$email" -f ~/.ssh/id_ed25519 -N ""
    
    # 启动ssh-agent并添加密钥
    eval "$(ssh-agent -s)"
    ssh-add ~/.ssh/id_ed25519
    
    log_success "SSH密钥生成完成"
}

# 显示公钥
show_public_key() {
    local key_file=""
    
    if [[ -f ~/.ssh/id_ed25519.pub ]]; then
        key_file="~/.ssh/id_ed25519.pub"
    elif [[ -f ~/.ssh/id_rsa.pub ]]; then
        key_file="~/.ssh/id_rsa.pub"
    else
        log_error "未找到SSH公钥文件"
        return 1
    fi
    
    echo
    log_info "您的SSH公钥内容："
    echo "=================================="
    cat "$key_file"
    echo "=================================="
    echo
}

# 复制公钥到剪贴板
copy_to_clipboard() {
    local key_file=""
    
    if [[ -f ~/.ssh/id_ed25519.pub ]]; then
        key_file="~/.ssh/id_ed25519.pub"
    elif [[ -f ~/.ssh/id_rsa.pub ]]; then
        key_file="~/.ssh/id_rsa.pub"
    else
        return 1
    fi
    
    # 尝试不同的剪贴板工具
    if command -v pbcopy &> /dev/null; then
        # macOS
        cat "$key_file" | pbcopy
        log_success "公钥已复制到剪贴板 (macOS)"
    elif command -v xclip &> /dev/null; then
        # Linux with xclip
        cat "$key_file" | xclip -selection clipboard
        log_success "公钥已复制到剪贴板 (Linux)"
    elif command -v xsel &> /dev/null; then
        # Linux with xsel
        cat "$key_file" | xsel --clipboard --input
        log_success "公钥已复制到剪贴板 (Linux)"
    else
        log_warning "无法自动复制到剪贴板，请手动复制上面的公钥内容"
    fi
}

# 测试GitHub连接
test_github_connection() {
    log_info "测试GitHub SSH连接..."
    
    # 添加GitHub到known_hosts
    if ! ssh-keygen -F github.com &>/dev/null; then
        log_info "添加GitHub到known_hosts..."
        ssh-keyscan -H github.com >> ~/.ssh/known_hosts 2>/dev/null
    fi
    
    # 测试连接
    if ssh -T git@github.com 2>&1 | grep -q "successfully authenticated"; then
        log_success "🎉 GitHub SSH连接测试成功！"
        return 0
    else
        log_warning "SSH连接测试失败，请检查密钥是否已添加到GitHub"
        return 1
    fi
}

# 显示GitHub设置说明
show_github_instructions() {
    echo
    log_info "📋 GitHub SSH密钥设置步骤："
    echo
    echo "1. 复制上面显示的SSH公钥"
    echo "2. 打开GitHub网站: https://github.com/settings/keys"
    echo "3. 点击 'New SSH key' 按钮"
    echo "4. 输入标题（如：My Computer）"
    echo "5. 粘贴公钥内容到 'Key' 字段"
    echo "6. 点击 'Add SSH key' 保存"
    echo
    log_warning "完成后按回车键测试连接..."
    read -r
}

# 主函数
main() {
    echo "🔑 GitHub SSH密钥设置助手"
    echo "========================="
    echo
    
    # 检查是否已有SSH密钥
    if check_ssh_key; then
        log_info "检测到现有SSH密钥"
        echo -n "是否要生成新的SSH密钥？[y/N]: "
        read -r generate_new
        
        if [[ "$generate_new" =~ ^[Yy]$ ]]; then
            generate_ssh_key "$DEFAULT_EMAIL"
        fi
    else
        log_info "未检测到SSH密钥，将生成新密钥"
        generate_ssh_key "$DEFAULT_EMAIL"
    fi
    
    # 显示公钥
    show_public_key
    
    # 尝试复制到剪贴板
    copy_to_clipboard
    
    # 显示设置说明
    show_github_instructions
    
    # 测试连接
    if test_github_connection; then
        echo
        log_success "✅ SSH设置完成！现在可以使用推送脚本了"
        echo
        log_info "运行推送脚本："
        echo "  ./push-to-github.sh"
    else
        echo
        log_error "❌ SSH连接失败"
        echo
        log_info "请检查："
        echo "1. 是否正确添加了SSH密钥到GitHub"
        echo "2. 网络连接是否正常"
        echo "3. 重新运行此脚本: ./setup-github-ssh.sh"
    fi
}

# 使用说明
if [[ "$1" == "--help" || "$1" == "-h" ]]; then
    echo "GitHub SSH密钥设置助手"
    echo "===================="
    echo
    echo "此脚本帮助您："
    echo "1. 生成SSH密钥对"
    echo "2. 显示公钥内容"
    echo "3. 指导添加到GitHub"
    echo "4. 测试SSH连接"
    echo
    echo "用法："
    echo "  $0              # 运行设置助手"
    echo "  $0 --help       # 显示帮助"
    echo
    echo "设置完成后可以使用推送脚本："
    echo "  ./push-to-github.sh"
    exit 0
fi

# 执行主函数
main "$@"
