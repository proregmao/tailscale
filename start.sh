#!/bin/bash

# Tailscale Unlimited Control 启动脚本
# 用于同时启动后端和前端服务

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
PURPLE='\033[0;35m'
CYAN='\033[0;36m'
NC='\033[0m' # No Color

# 项目路径
PROJECT_ROOT=$(pwd)
BACKEND_DIR="$PROJECT_ROOT/cmd/unlimited-control"
FRONTEND_DIR="$PROJECT_ROOT/headscale-ui"

# 日志文件
LOG_DIR="$PROJECT_ROOT/logs"
BACKEND_LOG="$LOG_DIR/backend.log"
FRONTEND_LOG="$LOG_DIR/frontend.log"

# PID文件
PID_DIR="$PROJECT_ROOT/pids"
BACKEND_PID="$PID_DIR/backend.pid"
FRONTEND_PID="$PID_DIR/frontend.pid"

# 创建必要的目录
mkdir -p "$LOG_DIR" "$PID_DIR"

# 打印带颜色的消息
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}[$(date '+%Y-%m-%d %H:%M:%S')] ${message}${NC}"
}

# 检查端口是否被占用
check_port() {
    local port=$1
    if lsof -Pi :$port -sTCP:LISTEN -t >/dev/null 2>&1; then
        return 0  # 端口被占用
    else
        return 1  # 端口空闲
    fi
}

# 等待端口可用
wait_for_port() {
    local port=$1
    local service=$2
    local max_attempts=30
    local attempt=1
    
    print_message $YELLOW "等待 $service 在端口 $port 启动..."
    
    while [ $attempt -le $max_attempts ]; do
        if check_port $port; then
            print_message $GREEN "$service 已在端口 $port 启动成功！"
            return 0
        fi
        
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    print_message $RED "$service 启动超时！"
    return 1
}

# 停止服务
stop_services() {
    print_message $YELLOW "正在停止服务..."
    
    # 停止后端
    if [ -f "$BACKEND_PID" ]; then
        local backend_pid=$(cat "$BACKEND_PID")
        if kill -0 "$backend_pid" 2>/dev/null; then
            print_message $YELLOW "停止后端服务 (PID: $backend_pid)..."
            kill "$backend_pid"
            rm -f "$BACKEND_PID"
        fi
    fi
    
    # 停止前端
    if [ -f "$FRONTEND_PID" ]; then
        local frontend_pid=$(cat "$FRONTEND_PID")
        if kill -0 "$frontend_pid" 2>/dev/null; then
            print_message $YELLOW "停止前端服务 (PID: $frontend_pid)..."
            kill "$frontend_pid"
            rm -f "$FRONTEND_PID"
        fi
    fi
    
    # 清理可能残留的进程
    pkill -f "unlimited-control" 2>/dev/null || true
    pkill -f "vite.*headscale-ui" 2>/dev/null || true
    
    print_message $GREEN "所有服务已停止"
}

# 检查依赖
check_dependencies() {
    print_message $BLUE "检查依赖..."
    
    # 检查 Go
    if ! command -v go &> /dev/null; then
        print_message $RED "错误: 未找到 Go，请先安装 Go"
        exit 1
    fi
    
    # 检查 Node.js
    if ! command -v node &> /dev/null; then
        print_message $RED "错误: 未找到 Node.js，请先安装 Node.js"
        exit 1
    fi
    
    # 检查 npm
    if ! command -v npm &> /dev/null; then
        print_message $RED "错误: 未找到 npm，请先安装 npm"
        exit 1
    fi
    
    print_message $GREEN "依赖检查通过"
}

# 构建后端
build_backend() {
    print_message $BLUE "构建后端..."
    
    cd "$BACKEND_DIR"
    
    # 检查是否需要下载依赖
    if [ ! -f "go.mod" ]; then
        print_message $YELLOW "初始化 Go 模块..."
        go mod init unlimited-control
    fi
    
    # 下载依赖
    print_message $YELLOW "下载 Go 依赖..."
    go mod tidy
    
    # 构建
    print_message $YELLOW "编译后端..."
    if go build -o unlimited-control .; then
        print_message $GREEN "后端构建成功"
    else
        print_message $RED "后端构建失败"
        exit 1
    fi
    
    cd "$PROJECT_ROOT"
}

# 安装前端依赖
install_frontend_deps() {
    print_message $BLUE "安装前端依赖..."
    
    cd "$FRONTEND_DIR"
    
    if [ ! -d "node_modules" ] || [ ! -f "package-lock.json" ]; then
        print_message $YELLOW "安装 npm 依赖..."
        npm install
    else
        print_message $GREEN "前端依赖已存在，跳过安装"
    fi
    
    cd "$PROJECT_ROOT"
}

# 启动后端
start_backend() {
    print_message $BLUE "启动后端服务..."
    
    cd "$BACKEND_DIR"
    
    # 检查端口 8080 是否被占用
    if check_port 8080; then
        print_message $YELLOW "端口 8080 已被占用，尝试停止现有服务..."
        pkill -f "unlimited-control" 2>/dev/null || true
        sleep 2
    fi
    
    # 启动后端
    nohup ./unlimited-control > "$BACKEND_LOG" 2>&1 &
    local backend_pid=$!
    echo $backend_pid > "$BACKEND_PID"
    
    print_message $GREEN "后端服务已启动 (PID: $backend_pid)"
    print_message $CYAN "后端日志: $BACKEND_LOG"
    
    cd "$PROJECT_ROOT"
    
    # 等待后端启动
    if ! wait_for_port 8080 "后端服务"; then
        print_message $RED "后端启动失败，请检查日志: $BACKEND_LOG"
        exit 1
    fi
}

# 启动前端
start_frontend() {
    print_message $BLUE "启动前端服务..."

    cd "$FRONTEND_DIR"

    # 检查端口 5173 是否被占用
    if check_port 5173; then
        print_message $YELLOW "端口 5173 已被占用，尝试停止现有服务..."
        pkill -f "vite.*headscale-ui" 2>/dev/null || true
        sleep 2
    fi

    # 启动前端开发服务器
    nohup npm run dev > "$FRONTEND_LOG" 2>&1 &
    local frontend_pid=$!
    echo $frontend_pid > "$FRONTEND_PID"

    print_message $GREEN "前端服务已启动 (PID: $frontend_pid)"
    print_message $CYAN "前端日志: $FRONTEND_LOG"

    cd "$PROJECT_ROOT"

    # 等待前端启动 - 检查多个可能的端口
    print_message $YELLOW "等待前端服务启动..."
    local frontend_started=false
    local max_attempts=30
    local attempt=1

    while [ $attempt -le $max_attempts ]; do
        # 检查常见的前端端口
        if check_port 5173 || check_port 8082 || check_port 8083; then
            # 从日志中获取实际端口
            local actual_port=$(cat "$FRONTEND_LOG" 2>/dev/null | grep -o "http://localhost:[0-9]*" | head -1 | grep -o "[0-9]*$" || echo "")
            if [ -n "$actual_port" ]; then
                print_message $GREEN "前端服务已在端口 $actual_port 启动成功！"
                frontend_started=true
                break
            fi
        fi

        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done

    if [ "$frontend_started" = false ]; then
        print_message $RED "前端启动失败，请检查日志: $FRONTEND_LOG"
        exit 1
    fi
}

# 显示状态
show_status() {
    print_message $PURPLE "==================== 服务状态 ===================="

    # 后端状态
    if check_port 8080; then
        print_message $GREEN "✅ 后端服务: 运行中 (http://localhost:8080)"
    else
        print_message $RED "❌ 后端服务: 未运行"
    fi

    # 前端状态 - 检测实际端口
    local frontend_port=""
    local frontend_running=false

    # 检查常见端口
    for port in 5173 5174 8082 8083 8084; do
        if check_port $port; then
            # 验证是否是我们的前端服务
            if [ -f "$FRONTEND_LOG" ]; then
                if cat "$FRONTEND_LOG" 2>/dev/null | grep -q "localhost:$port"; then
                    frontend_port=$port
                    frontend_running=true
                    break
                fi
            fi
        fi
    done

    if [ "$frontend_running" = true ]; then
        print_message $GREEN "✅ 前端服务: 运行中 (http://localhost:$frontend_port)"
    else
        print_message $RED "❌ 前端服务: 未运行"
    fi

    print_message $PURPLE "=================================================="
    print_message $CYAN "🌐 访问地址:"
    if [ "$frontend_running" = true ]; then
        print_message $CYAN "   前端界面: http://localhost:$frontend_port"
    else
        print_message $CYAN "   前端界面: 未启动"
    fi
    print_message $CYAN "   后端API:  http://localhost:8080"
    print_message $CYAN ""
    print_message $CYAN "📋 管理命令:"
    print_message $CYAN "   查看后端日志: tail -f $BACKEND_LOG"
    print_message $CYAN "   查看前端日志: tail -f $FRONTEND_LOG"
    print_message $CYAN "   停止服务:     ./start.sh stop"
    print_message $CYAN "   重启服务:     ./start.sh restart"
    print_message $CYAN "   查看状态:     ./start.sh status"
}

# 主函数
main() {
    case "${1:-start}" in
        "start")
            print_message $PURPLE "🚀 启动 Tailscale Unlimited Control"
            print_message $PURPLE "=================================================="
            
            # 检查依赖
            check_dependencies
            
            # 停止可能存在的服务
            stop_services
            
            # 构建和启动
            build_backend
            install_frontend_deps
            start_backend
            start_frontend
            
            # 显示状态
            show_status
            
            print_message $GREEN "🎉 所有服务启动完成！"
            ;;
            
        "stop")
            stop_services
            ;;
            
        "restart")
            print_message $YELLOW "🔄 重启服务..."
            stop_services
            sleep 2
            $0 start
            ;;
            
        "status")
            show_status
            ;;
            
        "logs")
            case "${2:-both}" in
                "backend")
                    print_message $CYAN "📋 后端日志:"
                    tail -f "$BACKEND_LOG"
                    ;;
                "frontend")
                    print_message $CYAN "📋 前端日志:"
                    tail -f "$FRONTEND_LOG"
                    ;;
                "both"|*)
                    print_message $CYAN "📋 实时日志 (Ctrl+C 退出):"
                    tail -f "$BACKEND_LOG" "$FRONTEND_LOG"
                    ;;
            esac
            ;;
            
        "help"|"-h"|"--help")
            print_message $CYAN "Tailscale Unlimited Control 启动脚本"
            print_message $CYAN ""
            print_message $CYAN "用法: $0 [命令]"
            print_message $CYAN ""
            print_message $CYAN "命令:"
            print_message $CYAN "  start     启动所有服务 (默认)"
            print_message $CYAN "  stop      停止所有服务"
            print_message $CYAN "  restart   重启所有服务"
            print_message $CYAN "  status    查看服务状态"
            print_message $CYAN "  logs      查看日志 [backend|frontend|both]"
            print_message $CYAN "  help      显示此帮助信息"
            ;;
            
        *)
            print_message $RED "未知命令: $1"
            print_message $YELLOW "使用 '$0 help' 查看可用命令"
            exit 1
            ;;
    esac
}

# 信号处理
trap 'print_message $YELLOW "收到中断信号，正在停止服务..."; stop_services; exit 0' INT TERM

# 执行主函数
main "$@"
