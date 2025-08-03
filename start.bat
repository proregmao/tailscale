@echo off
setlocal enabledelayedexpansion

REM Tailscale Unlimited Control 启动脚本 (Windows版本)
REM 用于同时启动后端和前端服务

title Tailscale Unlimited Control

REM 项目路径
set PROJECT_ROOT=%~dp0
set BACKEND_DIR=%PROJECT_ROOT%cmd\unlimited-control
set FRONTEND_DIR=%PROJECT_ROOT%headscale-ui

REM 日志目录
set LOG_DIR=%PROJECT_ROOT%logs
set BACKEND_LOG=%LOG_DIR%\backend.log
set FRONTEND_LOG=%LOG_DIR%\frontend.log

REM PID文件目录
set PID_DIR=%PROJECT_ROOT%pids

REM 创建必要的目录
if not exist "%LOG_DIR%" mkdir "%LOG_DIR%"
if not exist "%PID_DIR%" mkdir "%PID_DIR%"

REM 颜色定义 (Windows CMD颜色代码)
REM 0=黑色 1=蓝色 2=绿色 3=青色 4=红色 5=紫色 6=黄色 7=白色 8=灰色 9=亮蓝 A=亮绿 B=亮青 C=亮红 D=亮紫 E=亮黄 F=亮白

REM 打印带时间戳的消息
:print_message
echo [%date% %time%] %~2
goto :eof

REM 检查端口是否被占用
:check_port
netstat -an | findstr ":%~1 " | findstr "LISTENING" >nul
if %errorlevel% equ 0 (
    exit /b 0
) else (
    exit /b 1
)

REM 等待端口可用
:wait_for_port
set port=%~1
set service=%~2
set max_attempts=30
set attempt=1

call :print_message INFO "等待 %service% 在端口 %port% 启动..."

:wait_loop
call :check_port %port%
if %errorlevel% equ 0 (
    call :print_message SUCCESS "%service% 已在端口 %port% 启动成功！"
    exit /b 0
)

echo|set /p="."
timeout /t 1 /nobreak >nul
set /a attempt+=1
if %attempt% leq %max_attempts% goto wait_loop

call :print_message ERROR "%service% 启动超时！"
exit /b 1

REM 停止服务
:stop_services
call :print_message INFO "正在停止服务..."

REM 停止后端进程
taskkill /f /im unlimited-control.exe >nul 2>&1

REM 停止前端进程 (Node.js)
for /f "tokens=2" %%i in ('tasklist /fi "imagename eq node.exe" /fo csv ^| findstr "vite"') do (
    taskkill /f /pid %%i >nul 2>&1
)

REM 停止可能的npm进程
taskkill /f /im npm.cmd >nul 2>&1
taskkill /f /im node.exe >nul 2>&1

call :print_message SUCCESS "所有服务已停止"
goto :eof

REM 检查依赖
:check_dependencies
call :print_message INFO "检查依赖..."

REM 检查 Go
go version >nul 2>&1
if %errorlevel% neq 0 (
    call :print_message ERROR "错误: 未找到 Go，请先安装 Go"
    exit /b 1
)

REM 检查 Node.js
node --version >nul 2>&1
if %errorlevel% neq 0 (
    call :print_message ERROR "错误: 未找到 Node.js，请先安装 Node.js"
    exit /b 1
)

REM 检查 npm
npm --version >nul 2>&1
if %errorlevel% neq 0 (
    call :print_message ERROR "错误: 未找到 npm，请先安装 npm"
    exit /b 1
)

call :print_message SUCCESS "依赖检查通过"
goto :eof

REM 构建后端
:build_backend
call :print_message INFO "构建后端..."

cd /d "%BACKEND_DIR%"

REM 检查是否需要初始化模块
if not exist "go.mod" (
    call :print_message INFO "初始化 Go 模块..."
    go mod init unlimited-control
)

REM 下载依赖
call :print_message INFO "下载 Go 依赖..."
go mod tidy

REM 构建
call :print_message INFO "编译后端..."
go build -o unlimited-control.exe .
if %errorlevel% neq 0 (
    call :print_message ERROR "后端构建失败"
    exit /b 1
)

call :print_message SUCCESS "后端构建成功"
cd /d "%PROJECT_ROOT%"
goto :eof

REM 安装前端依赖
:install_frontend_deps
call :print_message INFO "安装前端依赖..."

cd /d "%FRONTEND_DIR%"

if not exist "node_modules" (
    call :print_message INFO "安装 npm 依赖..."
    npm install
    if %errorlevel% neq 0 (
        call :print_message ERROR "前端依赖安装失败"
        exit /b 1
    )
) else (
    call :print_message SUCCESS "前端依赖已存在，跳过安装"
)

cd /d "%PROJECT_ROOT%"
goto :eof

REM 启动后端
:start_backend
call :print_message INFO "启动后端服务..."

cd /d "%BACKEND_DIR%"

REM 检查端口 8080 是否被占用
call :check_port 8080
if %errorlevel% equ 0 (
    call :print_message INFO "端口 8080 已被占用，尝试停止现有服务..."
    taskkill /f /im unlimited-control.exe >nul 2>&1
    timeout /t 2 /nobreak >nul
)

REM 启动后端
start /b "" unlimited-control.exe > "%BACKEND_LOG%" 2>&1

call :print_message SUCCESS "后端服务已启动"
call :print_message INFO "后端日志: %BACKEND_LOG%"

cd /d "%PROJECT_ROOT%"

REM 等待后端启动
call :wait_for_port 8080 "后端服务"
if %errorlevel% neq 0 (
    call :print_message ERROR "后端启动失败，请检查日志: %BACKEND_LOG%"
    exit /b 1
)
goto :eof

REM 启动前端
:start_frontend
call :print_message INFO "启动前端服务..."

cd /d "%FRONTEND_DIR%"

REM 检查端口 5173 是否被占用
call :check_port 5173
if %errorlevel% equ 0 (
    call :print_message INFO "端口 5173 已被占用，尝试停止现有服务..."
    for /f "tokens=2" %%i in ('tasklist /fi "imagename eq node.exe" /fo csv ^| findstr "vite"') do (
        taskkill /f /pid %%i >nul 2>&1
    )
    timeout /t 2 /nobreak >nul
)

REM 启动前端开发服务器
start /b "" npm run dev > "%FRONTEND_LOG%" 2>&1

call :print_message SUCCESS "前端服务已启动"
call :print_message INFO "前端日志: %FRONTEND_LOG%"

cd /d "%PROJECT_ROOT%"

REM 等待前端启动
call :wait_for_port 5173 "前端服务"
if %errorlevel% neq 0 (
    call :print_message ERROR "前端启动失败，请检查日志: %FRONTEND_LOG%"
    exit /b 1
)
goto :eof

REM 显示状态
:show_status
echo.
echo ==================== 服务状态 ====================

REM 后端状态
call :check_port 8080
if %errorlevel% equ 0 (
    echo ✅ 后端服务: 运行中 ^(http://localhost:8080^)
) else (
    echo ❌ 后端服务: 未运行
)

REM 前端状态
call :check_port 5173
if %errorlevel% equ 0 (
    echo ✅ 前端服务: 运行中 ^(http://localhost:5173^)
) else (
    echo ❌ 前端服务: 未运行
)

echo ==================================================
echo 🌐 访问地址:
echo    前端界面: http://localhost:5173
echo    后端API:  http://localhost:8080
echo.
echo 📋 管理命令:
echo    查看后端日志: type "%BACKEND_LOG%"
echo    查看前端日志: type "%FRONTEND_LOG%"
echo    停止服务:     start.bat stop
echo    重启服务:     start.bat restart
echo    查看状态:     start.bat status
echo.
goto :eof

REM 显示帮助
:show_help
echo Tailscale Unlimited Control 启动脚本 ^(Windows版本^)
echo.
echo 用法: start.bat [命令]
echo.
echo 命令:
echo   start     启动所有服务 ^(默认^)
echo   stop      停止所有服务
echo   restart   重启所有服务
echo   status    查看服务状态
echo   logs      查看日志
echo   help      显示此帮助信息
echo.
goto :eof

REM 主函数
:main
set command=%~1
if "%command%"=="" set command=start

if "%command%"=="start" goto start_all
if "%command%"=="stop" goto stop_all
if "%command%"=="restart" goto restart_all
if "%command%"=="status" goto status_all
if "%command%"=="logs" goto logs_all
if "%command%"=="help" goto help_all
if "%command%"=="-h" goto help_all
if "%command%"=="--help" goto help_all

call :print_message ERROR "未知命令: %command%"
echo 使用 'start.bat help' 查看可用命令
exit /b 1

:start_all
echo.
echo 🚀 启动 Tailscale Unlimited Control
echo ==================================================

REM 检查依赖
call :check_dependencies
if %errorlevel% neq 0 exit /b 1

REM 停止可能存在的服务
call :stop_services

REM 构建和启动
call :build_backend
if %errorlevel% neq 0 exit /b 1

call :install_frontend_deps
if %errorlevel% neq 0 exit /b 1

call :start_backend
if %errorlevel% neq 0 exit /b 1

call :start_frontend
if %errorlevel% neq 0 exit /b 1

REM 显示状态
call :show_status

call :print_message SUCCESS "🎉 所有服务启动完成！"
echo.
echo 按任意键退出...
pause >nul
goto :eof

:stop_all
call :stop_services
goto :eof

:restart_all
call :print_message INFO "🔄 重启服务..."
call :stop_services
timeout /t 2 /nobreak >nul
call :main start
goto :eof

:status_all
call :show_status
goto :eof

:logs_all
echo 📋 查看日志文件:
echo.
echo 后端日志: %BACKEND_LOG%
echo 前端日志: %FRONTEND_LOG%
echo.
echo 选择要查看的日志:
echo 1. 后端日志
echo 2. 前端日志
echo 3. 退出
echo.
set /p choice="请输入选择 (1-3): "

if "%choice%"=="1" (
    echo.
    echo === 后端日志 ===
    type "%BACKEND_LOG%" 2>nul || echo 日志文件不存在或为空
) else if "%choice%"=="2" (
    echo.
    echo === 前端日志 ===
    type "%FRONTEND_LOG%" 2>nul || echo 日志文件不存在或为空
) else (
    goto :eof
)

echo.
pause
goto :eof

:help_all
call :show_help
goto :eof

REM 执行主函数
call :main %*
