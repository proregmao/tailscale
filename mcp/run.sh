#!/bin/bash
# MCP工具快速启动脚本

echo "=== MCP工具启动脚本 ==="
echo "当前目录: $(pwd)"

# 检查Python3是否可用
if ! command -v python3 &> /dev/null; then
    echo "错误: 未找到python3，请先安装Python3"
    exit 1
fi

# 显示菜单
show_menu() {
    echo ""
    echo "请选择操作:"
    echo "1. 启动所有MCP服务器"
    echo "2. 运行基本测试"
    echo "3. 运行交互式测试"
    echo "4. 安装依赖"
    echo "5. 查看帮助"
    echo "0. 退出"
    echo ""
}

# 启动MCP服务器
start_servers() {
    echo "启动MCP服务器..."
    python3 start_mcp.py
}

# 运行基本测试
run_basic_test() {
    echo "运行基本测试..."
    python3 test_mcp.py
}

# 运行交互式测试
run_interactive_test() {
    echo "运行交互式测试..."
    python3 test_mcp.py --interactive
}

# 安装依赖
install_deps() {
    echo "安装Python依赖..."
    if [ -f "requirements.txt" ]; then
        pip3 install -r requirements.txt
        echo "依赖安装完成"
    else
        echo "未找到requirements.txt文件"
    fi
}

# 显示帮助
show_help() {
    echo ""
    echo "=== MCP工具帮助 ==="
    echo ""
    echo "可用工具:"
    echo "1. Context7 - 文档检索工具"
    echo "   - resolve-library-id: 解析库名称到库ID"
    echo "   - get-library-docs: 获取库文档"
    echo ""
    echo "2. Playwright - 浏览器自动化工具"
    echo "   - browser_navigate: 导航到URL"
    echo "   - browser_click: 点击元素"
    echo "   - browser_type: 输入文本"
    echo "   - browser_take_screenshot: 截图"
    echo "   - browser_snapshot: 获取页面快照"
    echo ""
    echo "3. Sequential - 顺序思考工具"
    echo "   - sequentialthinking: 结构化思考"
    echo ""
    echo "手动启动命令:"
    echo "  启动服务器: python3 start_mcp.py"
    echo "  运行测试: python3 test_mcp.py"
    echo "  交互测试: python3 test_mcp.py --interactive"
    echo ""
}

# 主循环
main() {
    while true; do
        show_menu
        read -p "请输入选择 (0-5): " choice
        
        case $choice in
            1)
                start_servers
                ;;
            2)
                run_basic_test
                ;;
            3)
                run_interactive_test
                ;;
            4)
                install_deps
                ;;
            5)
                show_help
                ;;
            0)
                echo "退出"
                exit 0
                ;;
            *)
                echo "无效选择，请重试"
                ;;
        esac
        
        echo ""
        read -p "按Enter键继续..."
    done
}

# 如果有命令行参数，直接执行对应操作
if [ $# -gt 0 ]; then
    case $1 in
        "start")
            start_servers
            ;;
        "test")
            run_basic_test
            ;;
        "interactive")
            run_interactive_test
            ;;
        "install")
            install_deps
            ;;
        "help")
            show_help
            ;;
        *)
            echo "用法: $0 [start|test|interactive|install|help]"
            exit 1
            ;;
    esac
else
    main
fi
