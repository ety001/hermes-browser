---
name: mcp-browser
description: 使用 MCP 浏览器服务访问网页。当需要浏览器能力时优先使用 MCP 浏览器服务。
version: 1.0.0
author: Hermes Agent
metadata:
  hermes:
    tags: [browser, mcp, web]
---

# MCP 浏览器 (hermes-browser)

## 何时使用

当需要访问网页、截图、抓取内容时，优先使用 MCP 浏览器服务。

内置 `browser_*` 工具依赖本地 Playwright Chromium，在某些环境中可能因缺少系统库而不可用。

MCP 浏览器通过 MCP 协议远程调用，需要先在 Hermes `config.yaml` 中配置对应的 MCP 服务。

## 查找你的 MCP 浏览器服务地址

查看 Hermes 配置文件 `~/.hermes/config.yaml` 中的 `mcp.servers` 段，找到浏览器服务的配置：

```yaml
mcp:
  servers:
    hermes-browser:
      url: "http://your-host:port/mcp"
      headers:
        Authorization: "Bearer your-token"
```

将下面代码示例中的 `<MCP_URL>` 和 `<AUTH_TOKEN>` 替换为你的实际值。

## 调用方法

使用 `execute_code` 工具，Python 异步代码：

```python
import asyncio

async def main():
    from mcp.client.streamable_http import streamablehttp_client
    from mcp.client.session import ClientSession
    
    async with streamablehttp_client(
        "<MCP_URL>",  # 例如 "http://your-host:port/mcp"
        headers={"Authorization": "Bearer <AUTH_TOKEN>"}  # 按需填写认证信息
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

## 故障排查

### 截图报 `NO_EXTENSION_CONNECTED`（2026-09-30 已修复）

根因：服务端 `internal/ws/server.go` 未调用 `conn.SetReadLimit()`，coder/websocket 默认单条消息上限 32KB。截图响应是整屏 base64 图片（数 MB），扩展一发送服务端就超限拆连接，所有 pending 请求报 NO_EXTENSION_CONNECTED，几秒后扩展自动重连——表现为"截图必断连、其他命令正常"。

修复（服务端，已部署）：
- `internal/config/config.go`：WSConfig 增加 `read_limit`（yaml 可配），默认 64MB（`DefaultWSReadLimit`）
- `internal/ws/server.go`：`wslib.Accept()` 后调用 `conn.SetReadLimit(cfg.WebSocket.ReadLimit)`

若将来又出现"大响应必断连"，优先检查：是否重置了 read limit、响应体是否超过所设上限。

### Hub 死锁隐患（2026-09-30 已修复）

`internal/ws/hub.go` 的 `UnregisterClient` 原先持写锁做阻塞式 `ch <-`（channel 容量 1）。若断连瞬间 pending 请求已收到真实响应，会永久死锁整个 Hub（后续所有命令挂起）。已改为非阻塞 `select` + `default`。

### `switch_tab` 报 `tab_id is required`（2026-09-30 已修复）

`list_tabs` 返回 Go 格式化文本，tab_id 形如 `1.975625249e+09`，agent 原样回传时是字符串，`args["tab_id"].(float64)` 断言失败。现在 `handleSwitchTab` 同时接受 number 和字符串（含科学计数法，`strconv.ParseFloat` 解析）。

### 扩展端无需重载的边界

上述修复全部在服务端（Go 二进制），`extension/` 未改动，更新服务不需要在 `chrome://extensions/` 刷新扩展。只有改 `background.js`/`content.js`/`manifest.json` 才需要用户手动刷新扩展。

### 何时仍会出现 `Extension disconnected`

- 扩展端**主动重连**时的固有窗口：MCP 服务重启后，扩展指数退避重连（1s/2s/4s...，上限 30s），期间命令报 NO_EXTENSION_CONNECTED，等 5-10 秒自动恢复，可用 `ss -tnp | grep 19876` 看 ESTAB 确认。
- 瞬时断连：某条命令报断连、紧接着下一条正常——直接重试原命令即可，不必升级排障。

## 注意事项

1. **安装 mcp 包**：如果 import 失败，先安装 MCP SDK：`pip install mcp`
2. **结果解析**：工具返回的是 `CallToolResult` 对象，需通过 `result.content[0].text` 提取文本
3. **异步调用**：必须用 `asyncio.run(main())` 包装
4. **内置浏览器 fallback**：如果内置 `browser_*` 工具可用，也可以直接使用它们作为备选方案
5. **⚠️ 必须模仿人类操作速度**：在滚屏、点击、输入等操作时，**务必加入适当的等待时间**（`asyncio.sleep()`），模拟真实用户的操作节奏。例如：
   - 页面加载后等待 2-3 秒再操作
   - 滚屏时每次滚动后等待 1-2 秒
   - 点击后等待 1-2 秒等页面响应
   - 输入文本时使用 `type` 工具（自带逐字输入效果），不要用 `execute_js` 直接填充
   - 避免连续快速操作，两次操作之间至少间隔 0.5-1 秒
   - 如果不注意这一点，很容易被网站的反爬策略（如 Cloudflare、人机验证）拦截，导致访问失败
