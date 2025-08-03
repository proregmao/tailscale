#!/usr/bin/env python3
"""
Context7 MCP服务器 - 文档检索和库信息工具
提供代码库文档检索、库信息查询等功能
"""

import asyncio
import json
import sys
import os
from typing import Any, Dict, List, Optional
import logging

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class Context7Server:
    """Context7 MCP服务器实现"""
    
    def __init__(self):
        self.tools = {
            "resolve-library-id": {
                "name": "resolve-library-id",
                "description": "解析库名称到Context7兼容的库ID",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "libraryName": {
                            "type": "string",
                            "description": "要搜索的库名称"
                        }
                    },
                    "required": ["libraryName"]
                }
            },
            "get-library-docs": {
                "name": "get-library-docs", 
                "description": "获取库的最新文档",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "context7CompatibleLibraryID": {
                            "type": "string",
                            "description": "Context7兼容的库ID"
                        },
                        "tokens": {
                            "type": "number",
                            "description": "要检索的最大token数量",
                            "default": 10000
                        },
                        "topic": {
                            "type": "string", 
                            "description": "要关注的主题"
                        }
                    },
                    "required": ["context7CompatibleLibraryID"]
                }
            }
        }
    
    async def handle_request(self, request: Dict[str, Any]) -> Dict[str, Any]:
        """处理MCP请求"""
        try:
            method = request.get("method")
            params = request.get("params", {})
            
            if method == "tools/list":
                return {
                    "tools": list(self.tools.values())
                }
            elif method == "tools/call":
                tool_name = params.get("name")
                arguments = params.get("arguments", {})
                return await self.call_tool(tool_name, arguments)
            else:
                return {
                    "error": {
                        "code": -32601,
                        "message": f"未知方法: {method}"
                    }
                }
        except Exception as e:
            logger.error(f"处理请求时出错: {e}")
            return {
                "error": {
                    "code": -32603,
                    "message": f"内部错误: {str(e)}"
                }
            }
    
    async def call_tool(self, tool_name: str, arguments: Dict[str, Any]) -> Dict[str, Any]:
        """调用指定的工具"""
        if tool_name == "resolve-library-id":
            return await self.resolve_library_id(arguments)
        elif tool_name == "get-library-docs":
            return await self.get_library_docs(arguments)
        else:
            return {
                "error": {
                    "code": -32602,
                    "message": f"未知工具: {tool_name}"
                }
            }
    
    async def resolve_library_id(self, args: Dict[str, Any]) -> Dict[str, Any]:
        """解析库名称到库ID"""
        library_name = args.get("libraryName", "")
        
        # 模拟库ID解析逻辑
        library_mappings = {
            "react": "/facebook/react",
            "vue": "/vuejs/vue",
            "angular": "/angular/angular", 
            "express": "/expressjs/express",
            "fastapi": "/tiangolo/fastapi",
            "django": "/django/django",
            "flask": "/pallets/flask",
            "tailscale": "/tailscale/tailscale",
            "go": "/golang/go",
            "python": "/python/cpython"
        }
        
        library_id = library_mappings.get(library_name.lower())
        if library_id:
            return {
                "content": [
                    {
                        "type": "text",
                        "text": f"找到库ID: {library_id}\n库名称: {library_name}\n描述: 已成功解析库名称到Context7兼容的库ID"
                    }
                ]
            }
        else:
            return {
                "content": [
                    {
                        "type": "text", 
                        "text": f"未找到库 '{library_name}' 的映射。请检查库名称是否正确。"
                    }
                ]
            }
    
    async def get_library_docs(self, args: Dict[str, Any]) -> Dict[str, Any]:
        """获取库文档"""
        library_id = args.get("context7CompatibleLibraryID", "")
        tokens = args.get("tokens", 10000)
        topic = args.get("topic", "")
        
        # 模拟文档检索
        docs_content = f"""
# {library_id} 文档

## 概述
这是 {library_id} 的文档内容。

## 主要功能
- 功能1: 基础功能描述
- 功能2: 高级功能描述  
- 功能3: 扩展功能描述

## 使用示例
```python
# 示例代码
import library
result = library.main_function()
```

## API参考
详细的API文档和参数说明...

Token限制: {tokens}
关注主题: {topic if topic else '全部'}
"""
        
        return {
            "content": [
                {
                    "type": "text",
                    "text": docs_content
                }
            ]
        }

async def main():
    """主函数"""
    server = Context7Server()
    logger.info("Context7 MCP服务器启动")
    
    try:
        while True:
            # 从stdin读取请求
            line = await asyncio.get_event_loop().run_in_executor(None, sys.stdin.readline)
            if not line:
                break
                
            try:
                request = json.loads(line.strip())
                response = await server.handle_request(request)
                print(json.dumps(response))
                sys.stdout.flush()
            except json.JSONDecodeError:
                logger.error(f"无效的JSON: {line}")
            except Exception as e:
                logger.error(f"处理请求时出错: {e}")
                
    except KeyboardInterrupt:
        logger.info("服务器停止")

if __name__ == "__main__":
    asyncio.run(main())
