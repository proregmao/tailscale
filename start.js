#!/usr/bin/env node

/**
 * Tailscale Unlimited Control 跨平台启动脚本
 * 支持 Windows、macOS、Linux
 */

const { spawn, exec } = require('child_process');
const fs = require('fs');
const path = require('path');
const os = require('os');

// 颜色定义
const colors = {
    reset: '\x1b[0m',
    red: '\x1b[31m',
    green: '\x1b[32m',
    yellow: '\x1b[33m',
    blue: '\x1b[34m',
    purple: '\x1b[35m',
    cyan: '\x1b[36m',
    white: '\x1b[37m'
};

// 项目路径
const PROJECT_ROOT = process.cwd();
const BACKEND_DIR = path.join(PROJECT_ROOT, 'cmd', 'unlimited-control');
const FRONTEND_DIR = path.join(PROJECT_ROOT, 'headscale-ui');

// 日志和PID目录
const LOG_DIR = path.join(PROJECT_ROOT, 'logs');
const PID_DIR = path.join(PROJECT_ROOT, 'pids');

// 确保目录存在
[LOG_DIR, PID_DIR].forEach(dir => {
    if (!fs.existsSync(dir)) {
        fs.mkdirSync(dir, { recursive: true });
    }
});

// 日志和PID文件路径
const BACKEND_LOG = path.join(LOG_DIR, 'backend.log');
const FRONTEND_LOG = path.join(LOG_DIR, 'frontend.log');
const BACKEND_PID = path.join(PID_DIR, 'backend.pid');
const FRONTEND_PID = path.join(PID_DIR, 'frontend.pid');

// 工具函数
class Logger {
    static log(level, message, color = colors.white) {
        const timestamp = new Date().toLocaleString();
        console.log(`${color}[${timestamp}] ${message}${colors.reset}`);
    }

    static info(message) {
        this.log('INFO', message, colors.blue);
    }

    static success(message) {
        this.log('SUCCESS', message, colors.green);
    }

    static warning(message) {
        this.log('WARNING', message, colors.yellow);
    }

    static error(message) {
        this.log('ERROR', message, colors.red);
    }

    static debug(message) {
        this.log('DEBUG', message, colors.purple);
    }
}

// 端口检查
function checkPort(port) {
    return new Promise((resolve) => {
        const net = require('net');
        const server = net.createServer();
        
        server.listen(port, () => {
            server.once('close', () => {
                resolve(false); // 端口空闲
            });
            server.close();
        });
        
        server.on('error', () => {
            resolve(true); // 端口被占用
        });
    });
}

// 等待端口可用
async function waitForPort(port, serviceName, maxAttempts = 30) {
    Logger.info(`等待 ${serviceName} 在端口 ${port} 启动...`);
    
    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        if (await checkPort(port)) {
            Logger.success(`${serviceName} 已在端口 ${port} 启动成功！`);
            return true;
        }
        
        process.stdout.write('.');
        await new Promise(resolve => setTimeout(resolve, 1000));
    }
    
    Logger.error(`${serviceName} 启动超时！`);
    return false;
}

// 执行命令
function execCommand(command, options = {}) {
    return new Promise((resolve, reject) => {
        exec(command, options, (error, stdout, stderr) => {
            if (error) {
                reject(error);
            } else {
                resolve({ stdout, stderr });
            }
        });
    });
}

// 启动进程
function spawnProcess(command, args, options = {}) {
    return new Promise((resolve, reject) => {
        const child = spawn(command, args, {
            stdio: 'pipe',
            ...options
        });
        
        child.on('error', reject);
        child.on('exit', (code) => {
            if (code === 0) {
                resolve(child);
            } else {
                reject(new Error(`Process exited with code ${code}`));
            }
        });
        
        // 返回子进程以便后续操作
        resolve(child);
    });
}

// 停止服务
async function stopServices() {
    Logger.info('正在停止服务...');
    
    // 停止后端
    if (fs.existsSync(BACKEND_PID)) {
        try {
            const pid = fs.readFileSync(BACKEND_PID, 'utf8').trim();
            process.kill(parseInt(pid), 'SIGTERM');
            fs.unlinkSync(BACKEND_PID);
            Logger.info(`停止后端服务 (PID: ${pid})`);
        } catch (error) {
            Logger.debug(`停止后端服务时出错: ${error.message}`);
        }
    }
    
    // 停止前端
    if (fs.existsSync(FRONTEND_PID)) {
        try {
            const pid = fs.readFileSync(FRONTEND_PID, 'utf8').trim();
            process.kill(parseInt(pid), 'SIGTERM');
            fs.unlinkSync(FRONTEND_PID);
            Logger.info(`停止前端服务 (PID: ${pid})`);
        } catch (error) {
            Logger.debug(`停止前端服务时出错: ${error.message}`);
        }
    }
    
    // 清理可能残留的进程
    try {
        if (os.platform() === 'win32') {
            await execCommand('taskkill /f /im unlimited-control.exe', { stdio: 'ignore' });
            await execCommand('taskkill /f /im node.exe', { stdio: 'ignore' });
        } else {
            await execCommand('pkill -f unlimited-control', { stdio: 'ignore' });
            await execCommand('pkill -f "vite.*headscale-ui"', { stdio: 'ignore' });
        }
    } catch (error) {
        // 忽略清理错误
    }
    
    Logger.success('所有服务已停止');
}

// 检查依赖
async function checkDependencies() {
    Logger.info('检查依赖...');
    
    // 检查 Go
    try {
        await execCommand('go version');
    } catch (error) {
        Logger.error('错误: 未找到 Go，请先安装 Go');
        process.exit(1);
    }
    
    // 检查 Node.js
    try {
        await execCommand('node --version');
    } catch (error) {
        Logger.error('错误: 未找到 Node.js，请先安装 Node.js');
        process.exit(1);
    }
    
    // 检查 npm
    try {
        await execCommand('npm --version');
    } catch (error) {
        Logger.error('错误: 未找到 npm，请先安装 npm');
        process.exit(1);
    }
    
    Logger.success('依赖检查通过');
}

// 构建后端
async function buildBackend() {
    Logger.info('构建后端...');
    
    process.chdir(BACKEND_DIR);
    
    // 检查是否需要初始化模块
    if (!fs.existsSync('go.mod')) {
        Logger.info('初始化 Go 模块...');
        await execCommand('go mod init unlimited-control');
    }
    
    // 下载依赖
    Logger.info('下载 Go 依赖...');
    await execCommand('go mod tidy');
    
    // 构建
    Logger.info('编译后端...');
    const binaryName = os.platform() === 'win32' ? 'unlimited-control.exe' : 'unlimited-control';
    await execCommand(`go build -o ${binaryName} .`);
    
    Logger.success('后端构建成功');
    process.chdir(PROJECT_ROOT);
}

// 安装前端依赖
async function installFrontendDeps() {
    Logger.info('安装前端依赖...');
    
    process.chdir(FRONTEND_DIR);
    
    if (!fs.existsSync('node_modules') || !fs.existsSync('package-lock.json')) {
        Logger.info('安装 npm 依赖...');
        await execCommand('npm install');
    } else {
        Logger.success('前端依赖已存在，跳过安装');
    }
    
    process.chdir(PROJECT_ROOT);
}

// 启动后端
async function startBackend() {
    Logger.info('启动后端服务...');
    
    process.chdir(BACKEND_DIR);
    
    // 检查端口 8080 是否被占用
    if (await checkPort(8080)) {
        Logger.warning('端口 8080 已被占用，尝试停止现有服务...');
        await stopServices();
        await new Promise(resolve => setTimeout(resolve, 2000));
    }
    
    // 启动后端
    const binaryName = os.platform() === 'win32' ? 'unlimited-control.exe' : './unlimited-control';
    const backendProcess = spawn(binaryName, [], {
        stdio: ['ignore', 'pipe', 'pipe'],
        detached: true
    });
    
    // 保存PID
    fs.writeFileSync(BACKEND_PID, backendProcess.pid.toString());
    
    // 重定向日志
    const backendLogStream = fs.createWriteStream(BACKEND_LOG, { flags: 'a' });
    backendProcess.stdout.pipe(backendLogStream);
    backendProcess.stderr.pipe(backendLogStream);
    
    Logger.success(`后端服务已启动 (PID: ${backendProcess.pid})`);
    Logger.info(`后端日志: ${BACKEND_LOG}`);
    
    process.chdir(PROJECT_ROOT);
    
    // 等待后端启动
    if (!await waitForPort(8080, '后端服务')) {
        Logger.error(`后端启动失败，请检查日志: ${BACKEND_LOG}`);
        process.exit(1);
    }
}

// 启动前端
async function startFrontend() {
    Logger.info('启动前端服务...');

    process.chdir(FRONTEND_DIR);

    // 检查端口 5173 是否被占用
    if (await checkPort(5173)) {
        Logger.warning('端口 5173 已被占用，尝试停止现有服务...');
        await new Promise(resolve => setTimeout(resolve, 2000));
    }

    // 启动前端
    const frontendProcess = spawn('npm', ['run', 'dev'], {
        stdio: ['ignore', 'pipe', 'pipe'],
        detached: true,
        shell: true
    });

    // 保存PID
    fs.writeFileSync(FRONTEND_PID, frontendProcess.pid.toString());

    // 重定向日志
    const frontendLogStream = fs.createWriteStream(FRONTEND_LOG, { flags: 'a' });
    frontendProcess.stdout.pipe(frontendLogStream);
    frontendProcess.stderr.pipe(frontendLogStream);

    Logger.success(`前端服务已启动 (PID: ${frontendProcess.pid})`);
    Logger.info(`前端日志: ${FRONTEND_LOG}`);

    process.chdir(PROJECT_ROOT);

    // 等待前端启动 - 检查多个可能的端口
    Logger.info('等待前端服务启动...');
    let frontendStarted = false;
    const maxAttempts = 30;

    for (let attempt = 1; attempt <= maxAttempts; attempt++) {
        // 检查常见的前端端口
        const ports = [5173, 8082, 8083, 8084];
        for (const port of ports) {
            if (await checkPort(port)) {
                // 验证日志中是否包含此端口
                try {
                    const logContent = fs.readFileSync(FRONTEND_LOG, 'utf8');
                    if (logContent.includes(`localhost:${port}`)) {
                        Logger.success(`前端服务已在端口 ${port} 启动成功！`);
                        frontendStarted = true;
                        break;
                    }
                } catch (error) {
                    // 忽略读取日志错误
                }
            }
        }

        if (frontendStarted) break;

        process.stdout.write('.');
        await new Promise(resolve => setTimeout(resolve, 1000));
    }

    if (!frontendStarted) {
        Logger.error(`前端启动失败，请检查日志: ${FRONTEND_LOG}`);
        process.exit(1);
    }
}

// 显示状态
async function showStatus() {
    console.log('\n==================== 服务状态 ====================');
    
    // 后端状态
    const backendRunning = await checkPort(8080);
    if (backendRunning) {
        Logger.success('✅ 后端服务: 运行中 (http://localhost:8080)');
    } else {
        Logger.error('❌ 后端服务: 未运行');
    }
    
    // 前端状态
    const frontendRunning = await checkPort(5173);
    if (frontendRunning) {
        Logger.success('✅ 前端服务: 运行中 (http://localhost:5173)');
    } else {
        Logger.error('❌ 前端服务: 未运行');
    }
    
    console.log('==================================================');
    Logger.info('🌐 访问地址:');
    Logger.info('   前端界面: http://localhost:5173');
    Logger.info('   后端API:  http://localhost:8080');
    console.log('');
    Logger.info('📋 管理命令:');
    Logger.info(`   查看后端日志: tail -f ${BACKEND_LOG}`);
    Logger.info(`   查看前端日志: tail -f ${FRONTEND_LOG}`);
    Logger.info('   停止服务:     node start.js stop');
    Logger.info('   重启服务:     node start.js restart');
    Logger.info('   查看状态:     node start.js status');
    console.log('');
}

// 显示帮助
function showHelp() {
    console.log('Tailscale Unlimited Control 跨平台启动脚本');
    console.log('');
    console.log('用法: node start.js [命令]');
    console.log('');
    console.log('命令:');
    console.log('  start     启动所有服务 (默认)');
    console.log('  stop      停止所有服务');
    console.log('  restart   重启所有服务');
    console.log('  status    查看服务状态');
    console.log('  logs      查看日志');
    console.log('  help      显示此帮助信息');
    console.log('');
}

// 主函数
async function main() {
    const command = process.argv[2] || 'start';
    
    // 处理Ctrl+C信号
    process.on('SIGINT', async () => {
        Logger.warning('收到中断信号，正在停止服务...');
        await stopServices();
        process.exit(0);
    });
    
    try {
        switch (command) {
            case 'start':
                console.log('\n🚀 启动 Tailscale Unlimited Control');
                console.log('==================================================');
                
                await checkDependencies();
                await stopServices();
                await buildBackend();
                await installFrontendDeps();
                await startBackend();
                await startFrontend();
                await showStatus();
                
                Logger.success('🎉 所有服务启动完成！');
                break;
                
            case 'stop':
                await stopServices();
                break;
                
            case 'restart':
                Logger.info('🔄 重启服务...');
                await stopServices();
                await new Promise(resolve => setTimeout(resolve, 2000));
                await main(); // 递归调用启动
                break;
                
            case 'status':
                await showStatus();
                break;
                
            case 'logs':
                Logger.info('📋 日志文件位置:');
                Logger.info(`后端日志: ${BACKEND_LOG}`);
                Logger.info(`前端日志: ${FRONTEND_LOG}`);
                break;
                
            case 'help':
            case '-h':
            case '--help':
                showHelp();
                break;
                
            default:
                Logger.error(`未知命令: ${command}`);
                Logger.info("使用 'node start.js help' 查看可用命令");
                process.exit(1);
        }
    } catch (error) {
        Logger.error(`执行失败: ${error.message}`);
        process.exit(1);
    }
}

// 执行主函数
if (require.main === module) {
    main();
}
