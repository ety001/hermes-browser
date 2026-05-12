# 架构设计

## 整体架构

```
Hermes Agent (tai-worker: 192.168.44.2)
    |
    | MCP Protocol (stdio, JSON-RPC 2.0)
    |
MCP Server (Go binary, 运行在有浏览器的机器上)
    |
    | WebSocket (ws://<bind_addr>:<port>/ws?token=<token>)
    |
Chrome Extension (Manifest V3)
    | chrome.scripting.executeScript() / chrome.tabs.sendMessage()
    |
Content Script (按需注入目标页面)
    | 直接操作 DOM, 读取内容等
    |
目标网页
```

## 为什么这样设计

1. **MCP Server 用 Go**
   - 单二进制部署，无需运行时依赖
   - stdio transport 给 Hermes 用（标准 MCP 通信方式）
   - 内嵌 WebSocket server 给 Chrome Extension 连接
   - 使用 [mark3labs/mcp-go](https://github.com/mark3labs/mcp-go) SDK

2. **WebSocket 而非 Native Messaging**
   - Native Messaging 占用 stdin/stdout，与 MCP 的 stdio transport 冲突
   - WebSocket 支持远程连接（Hermes 和浏览器不在同一台机器）
   - 更灵活：支持重连、多 tab 管理、调试方便

3. **Chrome Extension Manifest V3**
   - 自用不上架，不需要审核
   - 真实浏览器环境，继承用户所有登录态
   - 不像 Playwright/Puppeteer 那样容易被反爬检测

## 网络拓扑

Hermes 运行在 tai-worker (192.168.44.2)，MCP Server 运行在有浏览器的机器上。
MCP Server 需要监听局域网 IP，使 Hermes 能通过 stdio 启动并连接。

注意：MCP stdio transport 要求 Hermes 能在本地启动 MCP Server 进程。
但 Hermes 和浏览器不在同一台机器，所以实际有两种部署方式：

### 方案 A：Hermes 直接启动 MCP Server（同一台机器）
- 适用于 Hermes 所在机器也有浏览器的情况
- stdio transport，Hermes 自动拉起 MCP Server

### 方案 B：MCP Server 独立运行 + HTTP transport（跨机器）★ 推荐
- MCP Server 在有浏览器的机器上独立运行
- Hermes 通过 HTTP transport 连接 MCP Server
- mcp-go 支持 StreamableHTTP transport

### 方案 C：SSH 桥接 stdio（跨机器）
- Hermes 通过 SSH 在远程机器上启动 MCP Server
- stdio 通过 SSH 隧道转发
- 需要配置 SSH key

**选择方案 B**，原因：
- 最简洁，不需要 SSH 配置
- MCP Server 独立运行，生命周期独立于 Hermes
- 支持多个 Hermes 实例同时连接
- WebSocket 在同一进程内管理，无额外端口

## 安全设计

1. **Token 认证**
   - MCP Server 启动时生成或读取 token（也可配置固定 token）
   - Chrome Extension 连接 WebSocket 时必须携带 token
   - MCP HTTP transport 也需要 token（通过 Authorization header）

2. **监听范围控制**
   - 配置文件中指定 bind address（如 `0.0.0.0` 或 `192.168.199.0/24`）
   - 默认只监听 localhost，需要显式配置才监听局域网

3. **Origin 检查**
   - WebSocket 升级时检查 Origin header
   - 只允许 `chrome-extension://<extension-id>` 来源

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
│   │   ├── hub.go                  # WebSocket 连接管理
│   │   ├── client.go               # 单个 WebSocket 客户端
│   │   └── protocol.go             # 命令/响应协议定义
│   ├── config/
│   │   └── config.go               # 配置加载
│   └── auth/
│       └── auth.go                 # Token 认证
├── extension/
│   ├── manifest.json               # Manifest V3
│   ├── background.js               # Service Worker, WebSocket 管理
│   ├── content.js                  # 页面 DOM 操作
│   └── popup/
│       ├── popup.html              # 状态面板 UI
│       └── popup.js
├── configs/
│   └── config.yaml                 # 默认配置文件
├── plans/                          # 开发计划文档
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
    "waitUntil": "networkidle"
  },
  "timeout": 30000
}
```

**响应（Extension -> MCP Server）：**
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

**错误响应：**
```json
{
  "id": "uuid-v4",
  "status": "error",
  "error": "Element not found: #login-button"
}
```

### Chrome Extension 内部通信

**Background <-> Content Script：** chrome.tabs.sendMessage / chrome.runtime.onMessage
**Popup <-> Background：** chrome.runtime.sendMessage / chrome.runtime.onMessage
