#!/usr/bin/env python3
"""
MCP工具测试脚本
用于测试各个MCP服务器的功能
"""

import asyncio
import json
import sys
import os
from pathlib import Path

# 添加当前目录到Python路径
sys.path.insert(0, str(Path(__file__).parent))

from start_mcp import MCPManager

async def test_context7():
    """测试Context7工具"""
    print("\n=== 测试Context7工具 ===")
    
    # 测试库ID解析
    request = {
        "method": "tools/call",
        "params": {
            "name": "resolve-library-id",
            "arguments": {
                "libraryName": "react"
            }
        }
    }
    
    print("测试库ID解析...")
    print(f"请求: {json.dumps(request, indent=2)}")
    
    # 测试文档获取
    request2 = {
        "method": "tools/call", 
        "params": {
            "name": "get-library-docs",
            "arguments": {
                "context7CompatibleLibraryID": "/facebook/react",
                "tokens": 5000,
                "topic": "hooks"
            }
        }
    }
    
    print("\n测试文档获取...")
    print(f"请求: {json.dumps(request2, indent=2)}")

async def test_playwright():
    """测试Playwright工具"""
    print("\n=== 测试Playwright工具 ===")
    
    # 测试导航
    request = {
        "method": "tools/call",
        "params": {
            "name": "browser_navigate", 
            "arguments": {
                "url": "https://example.com"
            }
        }
    }
    
    print("测试浏览器导航...")
    print(f"请求: {json.dumps(request, indent=2)}")
    
    # 测试截图
    request2 = {
        "method": "tools/call",
        "params": {
            "name": "browser_take_screenshot",
            "arguments": {
                "filename": "test_screenshot.png",
                "fullPage": True
            }
        }
    }
    
    print("\n测试截图功能...")
    print(f"请求: {json.dumps(request2, indent=2)}")

async def test_sequential():
    """测试Sequential工具"""
    print("\n=== 测试Sequential工具 ===")
    
    # 测试顺序思考
    thoughts = [
        {
            "thought": "首先，我需要分析问题的核心要素",
            "nextThoughtNeeded": True,
            "thoughtNumber": 1,
            "totalThoughts": 3
        },
        {
            "thought": "接下来，我要考虑可能的解决方案",
            "nextThoughtNeeded": True, 
            "thoughtNumber": 2,
            "totalThoughts": 3
        },
        {
            "thought": "最后，我需要评估最佳方案并得出结论",
            "nextThoughtNeeded": False,
            "thoughtNumber": 3,
            "totalThoughts": 3
        }
    ]
    
    for i, thought_data in enumerate(thoughts, 1):
        request = {
            "method": "tools/call",
            "params": {
                "name": "sequentialthinking",
                "arguments": thought_data
            }
        }
        
        print(f"\n测试思考步骤 {i}...")
        print(f"请求: {json.dumps(request, indent=2)}")

async def run_interactive_test():
    """运行交互式测试"""
    print("MCP工具交互式测试")
    print("=" * 50)
    
    manager = MCPManager()
    manager.load_config()
    
    # 启动服务器
    await manager.start_all_servers()
    await asyncio.sleep(2)
    
    try:
        while True:
            print("\n可用测试:")
            print("1. 测试Context7")
            print("2. 测试Playwright") 
            print("3. 测试Sequential")
            print("4. 测试所有工具")
            print("5. 列出所有工具")
            print("0. 退出")
            
            choice = input("\n请选择测试 (0-5): ").strip()
            
            if choice == "0":
                break
            elif choice == "1":
                await test_context7_interactive(manager)
            elif choice == "2":
                await test_playwright_interactive(manager)
            elif choice == "3":
                await test_sequential_interactive(manager)
            elif choice == "4":
                await test_all_tools(manager)
            elif choice == "5":
                await list_all_tools(manager)
            else:
                print("无效选择，请重试")
                
    finally:
        await manager.stop_all_servers()

async def test_context7_interactive(manager):
    """交互式测试Context7"""
    print("\n=== Context7交互式测试 ===")
    
    # 测试库ID解析
    library_name = input("请输入要查询的库名称 (如: react): ").strip() or "react"
    
    request = {
        "method": "tools/call",
        "params": {
            "name": "resolve-library-id",
            "arguments": {
                "libraryName": library_name
            }
        }
    }
    
    response = await manager.send_request("context7", request)
    if response:
        print(f"响应: {json.dumps(response, indent=2, ensure_ascii=False)}")

async def test_playwright_interactive(manager):
    """交互式测试Playwright"""
    print("\n=== Playwright交互式测试 ===")
    
    url = input("请输入要访问的URL (如: https://example.com): ").strip() or "https://example.com"
    
    request = {
        "method": "tools/call",
        "params": {
            "name": "browser_navigate",
            "arguments": {
                "url": url
            }
        }
    }
    
    response = await manager.send_request("playwright", request)
    if response:
        print(f"响应: {json.dumps(response, indent=2, ensure_ascii=False)}")

async def test_sequential_interactive(manager):
    """交互式测试Sequential"""
    print("\n=== Sequential交互式测试 ===")
    
    thought = input("请输入您的思考内容: ").strip() or "这是一个测试思考"
    
    request = {
        "method": "tools/call",
        "params": {
            "name": "sequentialthinking",
            "arguments": {
                "thought": thought,
                "nextThoughtNeeded": False,
                "thoughtNumber": 1,
                "totalThoughts": 1
            }
        }
    }
    
    response = await manager.send_request("sequential", request)
    if response:
        print(f"响应: {json.dumps(response, indent=2, ensure_ascii=False)}")

async def test_all_tools(manager):
    """测试所有工具"""
    print("\n=== 测试所有工具 ===")
    await manager.test_servers()

async def list_all_tools(manager):
    """列出所有工具"""
    print("\n=== 所有可用工具 ===")
    
    for server_name in manager.processes:
        request = {
            "method": "tools/list",
            "params": {}
        }
        
        response = await manager.send_request(server_name, request)
        if response:
            tools = response.get("tools", [])
            print(f"\n{server_name} 服务器工具:")
            for tool in tools:
                print(f"  - {tool.get('name')}: {tool.get('description')}")

async def main():
    """主函数"""
    if len(sys.argv) > 1 and sys.argv[1] == "--interactive":
        await run_interactive_test()
    else:
        print("MCP工具测试示例")
        print("=" * 50)
        await test_context7()
        await test_playwright()
        await test_sequential()
        print("\n测试完成！")
        print("\n要运行交互式测试，请使用: python test_mcp.py --interactive")

if __name__ == "__main__":
    asyncio.run(main())
