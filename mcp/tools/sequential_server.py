#!/usr/bin/env python3
"""
Sequential MCP服务器 - 顺序思考工具
提供结构化思考、问题分析、步骤规划等功能
"""

import asyncio
import json
import sys
import os
from typing import Any, Dict, List, Optional
import logging
import uuid
from datetime import datetime

# 配置日志
logging.basicConfig(level=logging.INFO)
logger = logging.getLogger(__name__)

class SequentialServer:
    """Sequential MCP服务器实现"""
    
    def __init__(self):
        self.thinking_sessions = {}
        self.tools = {
            "sequentialthinking": {
                "name": "sequentialthinking",
                "description": "结构化顺序思考工具，用于分析复杂问题",
                "inputSchema": {
                    "type": "object",
                    "properties": {
                        "thought": {
                            "type": "string",
                            "description": "当前思考步骤的内容"
                        },
                        "nextThoughtNeeded": {
                            "type": "boolean",
                            "description": "是否需要下一个思考步骤"
                        },
                        "thoughtNumber": {
                            "type": "integer",
                            "description": "当前思考步骤编号",
                            "minimum": 1
                        },
                        "totalThoughts": {
                            "type": "integer", 
                            "description": "预估总思考步骤数",
                            "minimum": 1
                        },
                        "isRevision": {
                            "type": "boolean",
                            "description": "是否为修订思考"
                        },
                        "revisesThought": {
                            "type": "integer",
                            "description": "修订的思考步骤编号",
                            "minimum": 1
                        },
                        "branchFromThought": {
                            "type": "integer",
                            "description": "分支起点思考步骤编号",
                            "minimum": 1
                        },
                        "branchId": {
                            "type": "string",
                            "description": "分支标识符"
                        },
                        "needsMoreThoughts": {
                            "type": "boolean",
                            "description": "是否需要更多思考步骤"
                        }
                    },
                    "required": ["thought", "nextThoughtNeeded", "thoughtNumber", "totalThoughts"]
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
        if tool_name == "sequentialthinking":
            return await self.sequential_thinking(arguments)
        else:
            return {
                "error": {
                    "code": -32602,
                    "message": f"未知工具: {tool_name}"
                }
            }
    
    async def sequential_thinking(self, args: Dict[str, Any]) -> Dict[str, Any]:
        """执行顺序思考"""
        thought = args.get("thought", "")
        next_thought_needed = args.get("nextThoughtNeeded", True)
        thought_number = args.get("thoughtNumber", 1)
        total_thoughts = args.get("totalThoughts", 1)
        is_revision = args.get("isRevision", False)
        revises_thought = args.get("revisesThought")
        branch_from_thought = args.get("branchFromThought")
        branch_id = args.get("branchId")
        needs_more_thoughts = args.get("needsMoreThoughts", False)
        
        # 创建或获取思考会话
        session_id = "default"
        if session_id not in self.thinking_sessions:
            self.thinking_sessions[session_id] = {
                "thoughts": [],
                "created_at": datetime.now(),
                "branches": {}
            }
        
        session = self.thinking_sessions[session_id]
        
        # 记录思考步骤
        thought_entry = {
            "number": thought_number,
            "content": thought,
            "timestamp": datetime.now().isoformat(),
            "is_revision": is_revision,
            "revises_thought": revises_thought,
            "branch_from_thought": branch_from_thought,
            "branch_id": branch_id
        }
        
        session["thoughts"].append(thought_entry)
        
        # 分析思考进度
        progress = (thought_number / total_thoughts) * 100
        
        # 生成响应
        response_text = f"""
思考步骤 {thought_number}/{total_thoughts} ({progress:.1f}%)

思考内容:
{thought}

状态分析:
- 当前步骤: {thought_number}
- 总预估步骤: {total_thoughts}
- 进度: {progress:.1f}%
- 是否修订: {'是' if is_revision else '否'}
- 需要更多思考: {'是' if needs_more_thoughts else '否'}
- 下一步需要: {'是' if next_thought_needed else '否'}

思考历史:
"""
        
        # 添加思考历史
        for i, t in enumerate(session["thoughts"][-5:], 1):  # 显示最近5个思考
            response_text += f"{i}. [{t['number']}] {t['content'][:100]}...\n"
        
        if not next_thought_needed:
            response_text += "\n✅ 思考过程完成！"
            
            # 生成思考总结
            summary = self.generate_thinking_summary(session["thoughts"])
            response_text += f"\n\n思考总结:\n{summary}"
        
        return {
            "content": [
                {
                    "type": "text",
                    "text": response_text
                }
            ]
        }
    
    def generate_thinking_summary(self, thoughts: List[Dict[str, Any]]) -> str:
        """生成思考总结"""
        if not thoughts:
            return "无思考记录"
        
        summary = f"""
总思考步骤数: {len(thoughts)}
开始时间: {thoughts[0]['timestamp']}
结束时间: {thoughts[-1]['timestamp']}

主要思考路径:
"""
        
        for i, thought in enumerate(thoughts, 1):
            summary += f"{i}. {thought['content'][:80]}...\n"
        
        summary += "\n关键洞察:\n"
        summary += "- 问题已通过结构化思考得到分析\n"
        summary += "- 思考过程遵循逻辑顺序\n"
        summary += "- 每个步骤都有明确的目标和结论\n"
        
        return summary

async def main():
    """主函数"""
    server = SequentialServer()
    logger.info("Sequential MCP服务器启动")
    
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
