# MCP工具集

这是一个完整的MCP（Model Context Protocol）工具集，包含Context7、Playwright和Sequential三个主要工具。

## 工具概述

### 1. Context7 - 文档检索工具
- **功能**: 库信息查询、文档检索
- **主要工具**:
  - `resolve-library-id`: 解析库名称到Context7兼容的库ID
  - `get-library-docs`: 获取库的最新文档

### 2. Playwright - 浏览器自动化工具  
- **功能**: 浏览器操作、网页自动化
- **主要工具**:
  - `browser_navigate`: 导航到指定URL
  - `browser_click`: 点击页面元素
  - `browser_type`: 在元素中输入文本
  - `browser_take_screenshot`: 截取页面截图
  - `browser_snapshot`: 获取页面快照

### 3. Sequential - 顺序思考工具
- **功能**: 结构化思考、问题分析
- **主要工具**:
  - `sequentialthinking`: 执行结构化顺序思考

## 安装和配置

### 1. 安装依赖
```bash
cd mcp
pip install -r requirements.txt
```

### 2. 安装Playwright浏览器（可选）
```bash
playwright install
```

## 使用方法

### 启动MCP服务器
```bash
cd mcp
python start_mcp.py
```

### 运行测试
```bash
# 运行基本测试
python test_mcp.py

# 运行交互式测试
python test_mcp.py --interactive
```

### 单独启动工具服务器
```bash
# 启动Context7服务器
python tools/context7_server.py

# 启动Playwright服务器  
python tools/playwright_server.py

# 启动Sequential服务器
python tools/sequential_server.py
```

## 配置文件

### mcp.json
主配置文件，定义了所有MCP服务器的配置：

```json
{
  "mcpServers": {
    "context7": {
      "command": "python",
      "args": ["tools/context7_server.py"]
    },
    "playwright": {
      "command": "python", 
      "args": ["tools/playwright_server.py"]
    },
    "sequential": {
      "command": "python",
      "args": ["tools/sequential_server.py"]
    }
  }
}
```

## 工具使用示例

### Context7示例
```python
# 解析库ID
request = {
    "method": "tools/call",
    "params": {
        "name": "resolve-library-id",
        "arguments": {
            "libraryName": "react"
        }
    }
}

# 获取文档
request = {
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
```

### Playwright示例
```python
# 导航到网页
request = {
    "method": "tools/call",
    "params": {
        "name": "browser_navigate",
        "arguments": {
            "url": "https://example.com"
        }
    }
}

# 截取截图
request = {
    "method": "tools/call",
    "params": {
        "name": "browser_take_screenshot",
        "arguments": {
            "filename": "screenshot.png",
            "fullPage": True
        }
    }
}
```

### Sequential示例
```python
# 顺序思考
request = {
    "method": "tools/call",
    "params": {
        "name": "sequentialthinking",
        "arguments": {
            "thought": "分析问题的核心要素",
            "nextThoughtNeeded": True,
            "thoughtNumber": 1,
            "totalThoughts": 3
        }
    }
}
```

## 故障排除

### 常见问题

1. **服务器启动失败**
   - 检查Python环境和依赖是否正确安装
   - 确认端口没有被占用
   - 查看日志输出获取详细错误信息

2. **Playwright工具无法使用**
   - 确保已安装Playwright浏览器: `playwright install`
   - 检查系统是否支持图形界面

3. **工具响应超时**
   - 检查网络连接
   - 增加配置文件中的timeout值

### 日志查看
所有服务器都会输出详细的日志信息，可以通过以下方式查看：

```bash
# 启动时查看日志
python start_mcp.py

# 单独启动服务器查看日志
python tools/context7_server.py
```

## 扩展开发

### 添加新工具
1. 在`tools/`目录下创建新的服务器脚本
2. 在`mcp.json`中添加服务器配置
3. 实现MCP协议的标准接口

### 自定义配置
可以修改`mcp.json`文件来：
- 启用/禁用特定工具
- 调整超时和重试设置
- 添加环境变量

## 许可证

本项目遵循MIT许可证。
