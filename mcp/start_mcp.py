#!/usr/bin/env python3
"""
MCP工具启动脚本
用于启动和管理所有MCP服务器
"""

import asyncio
import json
import subprocess
import sys
import os
import signal
import logging
from typing import Dict, List, Optional
from pathlib import Path

# 配置日志
logging.basicConfig(level=logging.INFO, format='%(asctime)s - %(name)s - %(levelname)s - %(message)s')
logger = logging.getLogger(__name__)

class MCPManager:
    """MCP服务器管理器"""
    
    def __init__(self, config_path: str = "mcp.json"):
        self.config_path = config_path
        self.config = {}
        self.processes = {}
        self.running = False
        
    def load_config(self):
        """加载配置文件"""
        try:
            with open(self.config_path, 'r', encoding='utf-8') as f:
                self.config = json.load(f)
            logger.info(f"配置文件加载成功: {self.config_path}")
        except FileNotFoundError:
            logger.error(f"配置文件未找到: {self.config_path}")
            sys.exit(1)
        except json.JSONDecodeError as e:
            logger.error(f"配置文件格式错误: {e}")
            sys.exit(1)
    
    async def start_server(self, name: str, server_config: Dict) -> Optional[subprocess.Popen]:
        """启动单个MCP服务器"""
        try:
            command = server_config.get("command", "python")
            args = server_config.get("args", [])
            env = server_config.get("env", {})
            
            # 设置环境变量
            server_env = os.environ.copy()
            server_env.update(env)
            
            # 构建完整命令
            full_command = [command] + args
            
            logger.info(f"启动服务器 {name}: {' '.join(full_command)}")
            
            # 启动进程
            process = subprocess.Popen(
                full_command,
                stdin=subprocess.PIPE,
                stdout=subprocess.PIPE,
                stderr=subprocess.PIPE,
                env=server_env,
                text=True,
                bufsize=1
            )
            
            self.processes[name] = process
            logger.info(f"服务器 {name} 启动成功 (PID: {process.pid})")
            return process
            
        except Exception as e:
            logger.error(f"启动服务器 {name} 失败: {e}")
            return None
    
    async def start_all_servers(self):
        """启动所有MCP服务器"""
        servers = self.config.get("mcpServers", {})
        
        for name, config in servers.items():
            if self.config.get("tools", {}).get(name, {}).get("enabled", True):
                await self.start_server(name, config)
            else:
                logger.info(f"服务器 {name} 已禁用，跳过启动")
    
    async def stop_server(self, name: str):
        """停止单个服务器"""
        if name in self.processes:
            process = self.processes[name]
            try:
                process.terminate()
                await asyncio.sleep(2)
                if process.poll() is None:
                    process.kill()
                logger.info(f"服务器 {name} 已停止")
            except Exception as e:
                logger.error(f"停止服务器 {name} 时出错: {e}")
            finally:
                del self.processes[name]
    
    async def stop_all_servers(self):
        """停止所有服务器"""
        for name in list(self.processes.keys()):
            await self.stop_server(name)
    
    async def monitor_servers(self):
        """监控服务器状态"""
        while self.running:
            for name, process in list(self.processes.items()):
                if process.poll() is not None:
                    logger.warning(f"服务器 {name} 意外退出 (返回码: {process.returncode})")
                    # 可以在这里实现自动重启逻辑
                    del self.processes[name]
            
            await asyncio.sleep(5)
    
    async def send_request(self, server_name: str, request: Dict) -> Optional[Dict]:
        """向服务器发送请求"""
        if server_name not in self.processes:
            logger.error(f"服务器 {server_name} 未运行")
            return None
        
        process = self.processes[server_name]
        try:
            # 发送请求
            request_json = json.dumps(request) + '\n'
            process.stdin.write(request_json)
            process.stdin.flush()
            
            # 读取响应
            response_line = process.stdout.readline()
            if response_line:
                return json.loads(response_line.strip())
            else:
                logger.error(f"从服务器 {server_name} 未收到响应")
                return None
                
        except Exception as e:
            logger.error(f"与服务器 {server_name} 通信时出错: {e}")
            return None
    
    async def test_servers(self):
        """测试所有服务器"""
        logger.info("开始测试服务器...")
        
        for name in self.processes:
            logger.info(f"测试服务器: {name}")
            
            # 测试工具列表
            request = {
                "method": "tools/list",
                "params": {}
            }
            
            response = await self.send_request(name, request)
            if response:
                tools = response.get("tools", [])
                logger.info(f"服务器 {name} 可用工具数量: {len(tools)}")
                for tool in tools:
                    logger.info(f"  - {tool.get('name', 'Unknown')}: {tool.get('description', 'No description')}")
            else:
                logger.error(f"服务器 {name} 测试失败")
    
    def signal_handler(self, signum, frame):
        """信号处理器"""
        logger.info(f"收到信号 {signum}，正在停止...")
        self.running = False
    
    async def run(self):
        """运行MCP管理器"""
        # 设置信号处理
        signal.signal(signal.SIGINT, self.signal_handler)
        signal.signal(signal.SIGTERM, self.signal_handler)
        
        # 加载配置
        self.load_config()
        
        # 启动服务器
        self.running = True
        await self.start_all_servers()
        
        # 等待服务器启动
        await asyncio.sleep(2)
        
        # 测试服务器
        await self.test_servers()
        
        # 监控服务器
        logger.info("MCP管理器运行中... (按Ctrl+C停止)")
        try:
            await self.monitor_servers()
        finally:
            await self.stop_all_servers()
            logger.info("MCP管理器已停止")

async def main():
    """主函数"""
    manager = MCPManager()
    await manager.run()

if __name__ == "__main__":
    asyncio.run(main())
