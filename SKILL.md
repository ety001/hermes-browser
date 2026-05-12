---
name: mcp-browser
description: 使用 hermes-browser MCP 服务访问网页。当需要浏览器能力时优先使用此 MCP 服务，而非内置 browser_* 工具。
version: 1.0.0
author: Hermes Agent
metadata:
  hermes:
    tags: [browser, mcp, web]
---

# MCP 浏览器 (hermes-browser)

## 何时使用

当需要访问网页、截图、抓取内容时，**优先使用 MCP 浏览器**而非内置 `browser_*` 工具。

内置 `browser_*` 依赖本地 Playwright Chromium，当前环境缺少 libnspr4 等依赖库，经常不可用。

MCP 浏览器运行在远程服务 `192.168.199.11:19875`，通过 MCP SDK 调用。

## 调用方法

使用 `execute_code` 工具，Python 异步代码：

```python
import asyncio

async def main():
    from mcp.client.streamable_http import streamablehttp_client
    from mcp.client.session import ClientSession
    
    async with streamablehttp_client(
        "http://192.168.199.11:19875/mcp",
        headers={"Authorization": "Bearer hermes-browser-deploy-token-2025"}
    ) as (read, write, _):
        async with ClientSession(read, write) as session:
            await session.initialize()
            
            # 导航到页面
            result = await session.call_tool("navigate", {"url": "https://example.com"})
            print([c.text for c in result.content if hasattr(c, 'text')])
            
            # 获取页面文本内容
            result = await session.call_tool("get_content", {})
            content = result.content[0].text if hasattr(result.content[0], 'text') else str(result)
            print(content[:3000])
            
            # 截图
            result = await session.call_tool("screenshot", {})
            # 截图返回图片路径

asyncio.run(main())
```

## 可用工具列表

| 工具名 | 参数 | 说明 |
|--------|------|------|
| `navigate` | `{"url": "..."}` | 导航到 URL |
| `get_content` | `{}` 或 `{"selector": "..."}` | 获取页面文本内容 |
| `click` | `{"selector": "..."}` | 点击元素 |
| `type` | `{"selector": "...", "text": "..."}` | 输入文本 |
| `screenshot` | `{}` 或 `{"selector": "..."}` | 截图 |
| `scroll` | `{"direction": "up"\|"down"}` | 滚动页面 |
| `execute_js` | `{"code": "..."}` | 执行 JS |
| `get_cookies` | `{}` | 获取 cookies |
| `new_tab` | `{"url": "..."}` | 新标签页 |
| `close_tab` | `{"tab_id": "..."}` | 关闭标签页 |
| `list_tabs` | `{}` | 列出标签页 |
| `switch_tab` | `{"tab_id": "..."}` | 切换标签页 |
| `wait_for` | `{"selector": "..."}` | 等待元素出现 |
| `hover` | `{"selector": "..."}` | 鼠标悬停 |
| `select_option` | `{"selector": "...", "value": "..."}` | 选择下拉选项 |

## 注意事项

1. **必须先安装 mcp 包**：如果 import 失败，先执行 `/home/ety001/.hermes/hermes-agent/venv/bin/pip install mcp`
2. **结果解析**：工具返回的是 `CallToolResult` 对象，需通过 `result.content[0].text` 提取文本
3. **异步调用**：必须用 `asyncio.run(main())` 包装
4. **内置浏览器不可用**：当前环境 `browser_navigate` 等内置工具因缺少系统库（libnspr4 等）而失败，不要反复尝试
5. **⚠️ 必须模仿人类操作速度**：这是最重要的原则。在滚屏、点击、输入等操作时，**务必加入适当的等待时间**（`asyncio.sleep()`），模拟真实用户的操作节奏。例如：
   - 页面加载后等待 2-3 秒再操作
   - 滚屏时每次滚动后等待 1-2 秒
   - 点击后等待 1-2 秒等页面响应
   - 输入文本时使用 `type` 工具（自带逐字输入效果），不要用 `execute_js` 直接填充
   - 避免连续快速操作，两次操作之间至少间隔 0.5-1 秒
   - 如果不注意这一点，很容易被网站的反爬策略（如 Cloudflare、人机验证）拦截，导致访问失败

## 内置 vs MCP 浏览器对比

| | 内置 browser_* | MCP mcp_hermes_browser_* |
|---|---|---|
| 工具名 | browser_navigate, browser_click | mcp_hermes_browser_navigate（需 SDK 调用） |
| 后端 | 本地 Playwright | 远程 MCP 服务 192.168.199.11:19875 |
| 状态 | ❌ 缺系统库不可用 | ✅ 可用 |
| 调用方式 | 直接调用 | execute_code + MCP SDK |
