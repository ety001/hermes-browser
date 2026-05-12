# 系统架构（v4）

> 修订记录：v4 修正 glm5turbo-v3 审计 P0#1(WithEndpointPath)、P0#3(execute_js 参数名)、P1#4(navigate timeout)、P1#6(项目结构)；
> v3 修正 glm5turbo-v2 审计 P0(P0#1~P0#5)、P1(P1#6~P1#9)；

## 整体架构

```
Hermes Agent (tai-worker: 192.168.44.2)
    |
    | MCP Protocol (Streamable HTTP, JSON-RPC 2.0)
    | Authorization: Bearer <token>
    |
MCP Server (Go binary, 独立运行在有浏览器的机器上)
    |
    | WebSocket (ws://<bind_addr>:<port>?token=<token>)
    |
Chrome Extension Background Service Worker (Manifest V3)
    |
    |--- Background 直接处理：navigate, screenshot, list_tabs,
    |    switch_tab, new_tab, close_tab, get_cookies
    |
    |--- 转发给 Content Script：click, type, hover, scroll,
    |    select_option, get_content, execute_js, wait_for
    |
Content Script (按需注入目标页面, chrome.scripting.executeScript)
    |
目标网页
```

## 为什么这样设计

1. **MCP Server 用 Go**
   - 单二进制部署，无需运行时依赖
   - 使用 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) SDK（8600+ stars）
   - 内嵌 WebSocket server 给 Chrome Extension 连接
   - 通过 StreamableHTTP transport 暴露 MCP 接口给 Hermes

2. **WebSocket 而非 Native Messaging**
   - Native Messaging 占用 stdin/stdout，与 MCP 的 stdio transport 冲突
   - WebSocket 支持远程连接（Hermes 和浏览器不在同一台机器）
   - 更灵活：支持重连、多 tab 管理、调试方便

3. **Chrome Extension Manifest V3**
   - 自用不上架，不需要审核
   - 真实浏览器环境，继承用户所有登录态
   - 不像 Playwright/Puppeteer 那样容易被反爬检测

## 部署方案

Hermes 运行在 tai-worker (192.168.44.2)，MCP Server 运行在有浏览器的机器上。
MCP Server 通过 HTTP transport 暴露 MCP 接口，Hermes 远程连接。

**选择方案 B：MCP Server 独立运行 + HTTP transport**
- MCP Server 在有浏览器的机器上独立运行（systemd 或手动）
- Hermes 通过 StreamableHTTP transport 连接 MCP Server
- 生命周期独立于 Hermes，支持多客户端
- WebSocket 在同一进程内管理，无额外端口

备选方案 A（同一台机器）：stdio transport，Hermes 自动拉起 MCP Server 进程。
备选方案 C（跨机器）：SSH 桥接 stdio，需要 SSH key 配置。

## 安全设计

1. **统一 Token 认证**
   - 顶层配置一个 token，WebSocket 和 HTTP transport 共用
   - 支持各 transport 独立覆盖（可选）
   - Chrome Extension 连接 WebSocket 时通过 query param 携带 token
   - Hermes 连接 MCP HTTP 时通过 Authorization: Bearer header 携带 token
   - Token 可配置固定值或自动生成（32 字节随机，hex 编码，持久化到 ~/.hermes-browser/.token）

2. **监听范围控制**
   - WebSocket 和 HTTP 分别配置 bind address
   - 默认只监听 localhost，需要显式配置才监听局域网

3. **Origin 检查**
   - WebSocket 升级时检查 Origin header
   - 只允许 `chrome-extension://<extension-id>` 来源

4. **HTTP transport 认证**
   - mcp-go 的 StreamableHTTPServer 没有内置 middleware 选项
   - 使用标准 net/http 中间件包装 StreamableHTTPServer 的 ServeHTTP 方法
   - 在中间件中校验 Authorization header

## 命令路由架构

Chrome Extension Background Service Worker 作为命令分发器，
区分两类命令：

| 类别 | 命令 | 处理位置 | 使用的 Chrome API |
|------|------|---------|------------------|
| Tab 管理 | list_tabs, switch_tab, new_tab, close_tab | Background | chrome.tabs |
| 页面导航 | navigate | Background | chrome.tabs.update + onUpdated |
| 截图 | screenshot | Background | chrome.tabs.captureVisibleTab |
| Cookie | get_cookies | Background | chrome.cookies |
| 页面交互 | click, type, hover, scroll, select_option | Content Script | DOM API |
| 内容获取 | get_content, execute_js, wait_for | Content Script | DOM API |

Background 在执行操作前会自动 ensureTabActive(tabId) 确保目标 tab 激活。

## 项目结构

```
~/workspace/hermes-browser/
├── cmd/
│   └── server/
│       └── main.go                 # MCP Server 入口
├── internal/
│   ├── mcp/
│   │   ├── server.go               # MCP Server 定义和工具注册
│   │   └── tools.go                # 各工具的 handler 实现
│   ├── ws/
│   │   ├── server.go              # WebSocket HTTP 服务器 (P1#6: 补充)
│   │   ├── hub.go                  # WebSocket 连接管理（单 client 模型）
│   │   ├── client.go               # 单个 WebSocket 客户端
│   │   └── protocol.go             # 命令/响应协议定义（含错误码）
│   ├── config/
│   │   └── config.go               # 配置加载
│   └── auth/
│       └── auth.go                 # Token 认证
├── extension/
│   ├── manifest.json               # Manifest V3
│   ├── background.js               # Service Worker: WebSocket + 命令分发
│   ├── content.js                  # 页面 DOM 操作
│   ├── icons/
│   │   ├── icon16.png
│   │   ├── icon48.png
│   │   └── icon128.png
│   └── popup/
│       ├── popup.html              # 状态面板 UI
│       └── popup.js
├── configs/
│   └── config.yaml                 # 默认配置文件
├── plans/                          # 开发计划文档
│   ├── v1/                         # 初版计划（已归档）
│   └── v2/                         # 修订版计划
├── go.mod
├── go.sum
├── Makefile
└── README.md
```

## 通信协议

### MCP Server <-> Chrome Extension (WebSocket)

消息格式均为 JSON：

**请求（MCP Server -> Extension）：**
```json
{
  "id": "uuid-v4",
  "method": "navigate",
  "params": {
    "url": "https://example.com",
    "wait_until": "networkidle"
  },
  "tab_id": 123
}
```

**成功响应（Extension -> MCP Server）：**
```json
{
  "id": "uuid-v4",
  "status": "success",
  "data": {
    "url": "https://example.com",
    "title": "Example Domain"
  }
}
```

**错误响应（Extension -> MCP Server）：**
```json
{
  "id": "uuid-v4",
  "status": "error",
  "code": "ELEMENT_NOT_FOUND",
  "error": "Element matching selector '#login-btn' not found"
}
```

### 错误码定义

| 错误码 | 说明 | 建议 LLM 行为 |
|--------|------|--------------|
| `ELEMENT_NOT_FOUND` | CSS 选择器未匹配到元素 | 检查选择器，等待后重试 |
| `TIMEOUT` | 操作超时 | 增大 timeout 参数后重试 |
| `NAVIGATION_ERROR` | 页面导航失败 | 检查 URL，可能需要登录 |
| `JS_EXECUTION_ERROR` | JavaScript 执行异常 | 修正代码后重试 |
| `TAB_NOT_FOUND` | Tab ID 不存在 | 重新 list_tabs 获取正确 ID |
| `TAB_CLOSED` | Tab 已被关闭 | 使用其他 tab |
| `NO_EXTENSION_CONNECTED` | Chrome Extension 未连接 | 提示用户检查 Extension |
| `PERMISSION_DENIED` | 权限不足 | 提示用户检查 tab 是否激活 |
| `UNKNOWN_METHOD` | 未知命令 | 检查方法名 |

### Chrome Extension 内部通信

**Background <-> Content Script：** chrome.tabs.sendMessage / chrome.runtime.onMessage
**Popup <-> Background：** chrome.runtime.sendMessage / chrome.runtime.onMessage

### 已知行为：Extension 重载

开发阶段频繁点击 Chrome "重新加载" 时：
- 旧 Background Service Worker 销毁 → WebSocket 断开 → Hub 清理 client
- 新 Background Service Worker 启动 → WebSocket 重连 → Hub 注册新 client
- 页面中残留的旧 Content Script 的 listener 会失效，
  可能产生 `Receiving end does not exist` console 警告，不影响功能
