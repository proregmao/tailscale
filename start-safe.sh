#!/bin/bash

# 安全启动脚本 - 专门解决 SvelteKit 竞争条件问题
# 用于启动 Tailscale Unlimited Control 系统

set -e

# 颜色定义
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# 项目路径
PROJECT_ROOT=$(pwd)
BACKEND_DIR="$PROJECT_ROOT/cmd/unlimited-control"
FRONTEND_DIR="$PROJECT_ROOT/headscale-ui"
LOG_DIR="$PROJECT_ROOT/logs"
PID_DIR="$PROJECT_ROOT/pids"

# 创建必要的目录
mkdir -p "$LOG_DIR" "$PID_DIR"

# 打印带颜色的消息
print_message() {
    local color=$1
    local message=$2
    echo -e "${color}[$(date '+%Y-%m-%d %H:%M:%S')] ${message}${NC}"
}

# 清理函数
cleanup() {
    print_message $YELLOW "清理环境..."
    
    # 停止现有进程
    if [ -f "$PID_DIR/backend.pid" ]; then
        local backend_pid=$(cat "$PID_DIR/backend.pid")
        if ps -p "$backend_pid" > /dev/null 2>&1; then
            print_message $YELLOW "停止后端进程 (PID: $backend_pid)"
            kill "$backend_pid" 2>/dev/null || true
        fi
        rm -f "$PID_DIR/backend.pid"
    fi
    
    if [ -f "$PID_DIR/frontend.pid" ]; then
        local frontend_pid=$(cat "$PID_DIR/frontend.pid")
        if ps -p "$frontend_pid" > /dev/null 2>&1; then
            print_message $YELLOW "停止前端进程 (PID: $frontend_pid)"
            kill "$frontend_pid" 2>/dev/null || true
        fi
        rm -f "$PID_DIR/frontend.pid"
    fi
    
    # 清理可能残留的进程
    pkill -f "unlimited-control" 2>/dev/null || true
    pkill -f "vite.*headscale-ui" 2>/dev/null || true
    
    # 清理前端缓存
    cd "$FRONTEND_DIR"
    print_message $YELLOW "清理 SvelteKit 缓存..."
    rm -rf .svelte-kit
    rm -rf node_modules/.vite
    rm -rf node_modules/.cache
    
    print_message $GREEN "环境清理完成"
}

# 启动后端服务
start_backend() {
    print_message $BLUE "启动后端服务..."
    
    cd "$BACKEND_DIR"
    
    # 启动后端
    nohup go run . > "$LOG_DIR/backend.log" 2>&1 &
    local backend_pid=$!
    echo $backend_pid > "$PID_DIR/backend.pid"
    
    print_message $GREEN "后端服务已启动 (PID: $backend_pid)"
    
    # 等待后端启动
    sleep 3
    
    # 检查后端是否正常运行
    if ! ps -p $backend_pid > /dev/null 2>&1; then
        print_message $RED "后端服务启动失败"
        return 1
    fi
    
    print_message $GREEN "后端服务运行正常"
    return 0
}

# 安全启动前端服务
start_frontend_safe() {
    print_message $BLUE "安全启动前端服务..."
    
    cd "$FRONTEND_DIR"
    
    # 检查 node_modules
    if [ ! -d "node_modules" ]; then
        print_message $YELLOW "安装前端依赖..."
        npm install
    fi
    
    # 第一步：同步 SvelteKit
    print_message $YELLOW "同步 SvelteKit 配置..."
    npx svelte-kit sync
    
    # 等待同步完成
    sleep 2
    
    # 第二步：预构建依赖
    print_message $YELLOW "预构建依赖..."
    npx vite optimize --force
    
    # 等待预构建完成
    sleep 2
    
    # 第三步：启动开发服务器
    print_message $YELLOW "启动前端开发服务器..."
    nohup npm run dev:safe > "$LOG_DIR/frontend.log" 2>&1 &
    local frontend_pid=$!
    echo $frontend_pid > "$PID_DIR/frontend.pid"
    
    print_message $GREEN "前端服务已启动 (PID: $frontend_pid)"
    
    # 等待前端启动
    local max_attempts=20
    local attempt=1
    
    while [ $attempt -le $max_attempts ]; do
        if ps -p $frontend_pid > /dev/null 2>&1; then
            # 检查日志中是否有错误
            if grep -q "Error:" "$LOG_DIR/frontend.log" 2>/dev/null; then
                print_message $YELLOW "检测到错误，尝试重启..."
                kill $frontend_pid 2>/dev/null || true
                sleep 2
                return 1
            fi
            
            # 检查是否成功启动
            if grep -q "ready in" "$LOG_DIR/frontend.log" 2>/dev/null; then
                print_message $GREEN "前端服务启动成功"
                return 0
            fi
        else
            print_message $RED "前端进程意外退出"
            return 1
        fi
        
        echo -n "."
        sleep 1
        attempt=$((attempt + 1))
    done
    
    print_message $RED "前端服务启动超时"
    return 1
}

# 主函数
main() {
    print_message $BLUE "🚀 启动 Tailscale Unlimited Control 系统 (安全模式)"
    print_message $BLUE "=================================================="
    
    # 清理环境
    cleanup
    
    # 启动后端
    if ! start_backend; then
        print_message $RED "后端启动失败，退出"
        exit 1
    fi
    
    # 尝试启动前端（最多3次）
    local frontend_attempts=3
    local frontend_success=false
    
    for i in $(seq 1 $frontend_attempts); do
        print_message $YELLOW "前端启动尝试 $i/$frontend_attempts"
        
        if start_frontend_safe; then
            frontend_success=true
            break
        else
            print_message $YELLOW "前端启动失败，清理并重试..."
            
            # 清理前端相关文件
            cd "$FRONTEND_DIR"
            rm -rf .svelte-kit
            pkill -f "vite.*headscale-ui" 2>/dev/null || true
            
            if [ $i -lt $frontend_attempts ]; then
                sleep 3
            fi
        fi
    done
    
    if [ "$frontend_success" = false ]; then
        print_message $RED "前端启动失败，请检查日志："
        tail -20 "$LOG_DIR/frontend.log"
        exit 1
    fi
    
    print_message $GREEN "🎉 系统启动完成！"
    print_message $BLUE "=================================================="
    print_message $GREEN "📊 后端服务: http://localhost:8080"
    print_message $GREEN "🎨 前端服务: http://localhost:5173"
    print_message $BLUE "=================================================="
    print_message $YELLOW "📝 查看日志:"
    print_message $YELLOW "   后端: tail -f $LOG_DIR/backend.log"
    print_message $YELLOW "   前端: tail -f $LOG_DIR/frontend.log"
    print_message $YELLOW "🛑 停止服务: ./stop.sh"
}

# 运行主函数
main "$@"
