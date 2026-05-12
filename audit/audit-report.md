# Hermes Browser 开发计划审计报告

审计日期：2026-05-11
审计范围：plans/ 下全部 7 个文档
交叉验证源：mcp-go 源码（mark3labs/mcp-go）、Hermes mcp_tool.py

---

## P0 - 需要修改，否则无法工作

### 1. mcp-go 没有 WithHTTPMiddleware，HTTP transport 认证方案不可行

**文件：** 06-server-design.md（末段）

计划中展示的代码：
```go
httpServer := server.NewStreamableHTTPServer(mcpServer,
    server.WithHTTPMiddleware(func(next http.Handler) http.Handler { ... }),
)
```

**问题：** mcp-go 的 `StreamableHTTPServer` 不提供 `WithHTTPMiddleware` option。
实际可用的 option 包括 `WithEndpointPath`、`WithStateLess`、`WithStateful`、
`WithSessionIdManager`、`WithHeartbeatInterval`、`WithCORSAllowedOrigins` 等。
唯一能注入逻辑的是 `HTTPContextFunc`（http_transport_options.go），
但它只能往 context 注入值，不能拦截请求返回 401。

**修复方案：** `StreamableHTTPServer` 实现了 `http.Handler` 接口（ServeHTTP 方法），
因此可以用标准中间件模式包装：

```go
httpServer := server.NewStreamableHTTPServer(mcpServer,
    server.WithEndpointPath("/mcp"),
)

// 不调用 httpServer.Start()，自己建 http.Server + mux
mux := http.NewServeMux()
mux.Handle("/mcp", authMiddleware(httpServer))

srv := &http.Server{Addr: cfg.Server.HTTP.Bind, Handler: mux}
go srv.ListenAndServe()
```

其中 authMiddleware 是标准 net/http 中间件：
```go
func authMiddleware(next http.Handler) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" || !auth.Validate(strings.TrimPrefix(token, "Bearer ")) {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }
        next.ServeHTTP(w, r)
    })
}
```

注意：不能调用 `httpServer.Start(addr)`，因为它内部自己建 mux 和 http.Server。
必须自己管理 http.Server 生命周期。`Shutdown` 可以调用 `httpServer.Shutdown(ctx)`。

---

### 2. navigate 的响应链路断裂

**文件：** 05-extension-design.md（Content Script 操作函数）

计划中 navigate 的实现：
```javascript
case 'navigate':
    window.location.href = params.url;
    sendResponse({ status: 'navigating' });
    break;
```

**问题：** `window.location.href = ...` 触发页面导航后，Content Script 立即被销毁。
`sendResponse` 可能来不及发出，即使发出了也只返回 "navigating" 而非加载完成。
MCP Server 收到 "navigating" 后无法知道页面是否真正加载完成，
也无法在加载完成后返回 title 等信息。

**修复方案：** navigate 不应在 Content Script 中执行。
应由 Background Service Worker 接管：

```javascript
// background.js 中拦截 navigate 命令
async function handleNavigate(command) {
    const { tabId, id, params } = command;
    const { url, wait_until } = params;

    // 由 Background 调用 chrome.tabs.update 触发导航
    await chrome.tabs.update(tabId, { url });

    // 监听页面加载完成
    if (wait_until === 'networkidle' || wait_until === 'load') {
        await waitForTabComplete(tabId);
    }

    // 获取加载后的 tab 信息
    const tab = await chrome.tabs.get(tabId);
    sendToServer({ id, status: 'success', data: {
        url: tab.url, title: tab.title
    }});
}

function waitForTabComplete(tabId) {
    return new Promise((resolve) => {
        const listener = (updatedTabId, changeInfo) => {
            if (updatedTabId === tabId && changeInfo.status === 'complete') {
                chrome.tabs.onUpdated.removeListener(listener);
                resolve();
            }
        };
        chrome.tabs.onUpdated.addListener(listener);
    });
}
```

同时，MCP Server 的 navigate handler 需要特殊处理：
- 命令通过 WebSocket 发给 Background（而非 Content Script）
- Background 完成导航后回复

**影响：** Hub 的消息路由需要区分 "tab 级操作"（list_tabs, switch_tab, navigate 等，
由 Background 直接处理）和 "页面级操作"（click, type, get_content 等，
由 Content Script 处理）。

---

### 3. WebSocket Hub 的 tabClients 映射设计过度

**文件：** 06-server-design.md（Hub 设计）

计划中 Hub 维护 `tabClients map[int]*Client`，按 tabID 路由到不同 client。

**问题：** 一个 Chrome Extension 实例只有 1 个 Background Service Worker，
因此永远只有 1 个 WebSocket 连接。所有 tab 的操作都通过这 1 个连接传输，
命令中带 tabId 参数，由 Extension 内部路由到对应 tab。

tabClients 映射实际上是 1:1（一个 client 管所有 tab），增加了不必要的复杂度。

**建议简化为：**
```go
type Hub struct {
    mu           sync.RWMutex
    client       *Client          // 单一 Extension 客户端
    responses    map[string]chan Response
    // ...
}
```

命令格式中已包含 tabId 字段，Extension Background 收到后自行路由到对应 tab。
MCP Server 端不需要按 tab 维护 client 映射。

如果未来需要支持多个 Extension 实例（多台浏览器），再升级为多 client 模型。
