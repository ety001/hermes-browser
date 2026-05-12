# hermes-browser plans/v2 审计报告

- **审计模型**: kimi-for-coding
- **审计计划版本**: v2
- **审计日期**: 2026-05-11
- **审计文件清单**:
  - 01-architecture.md
  - 02-config-design.md
  - 03-mcp-tools.md
  - 04-development-plan.md
  - 05-extension-design.md
  - 06-server-design.md
  - 07-hermes-integration.md

---

## 一、总体评价

v2 计划相比 v1 有了显著改进，主要架构决策（MCP Server 用 Go + Chrome Extension MV3 + WebSocket 连接）合理且可行。v1 审计中提出的多项意见（Hub 单 client 模型、navigate/screenshot 归 Background、统一 token、错误码体系、命令路由表等）已在 v2 中得到修正。整体计划结构清晰，但仍有若干细节需要补充或修正。

---

## 二、按文件审计

### 01-architecture.md — 架构设计

#### 已修正（✓）
- P0#3: Hub 简化为单 client 模型
- P0#2: navigate 归 Background 处理
- P1#4: screenshot 归 Background 处理
- P1#5: 统一 token 设计
- P2#10: 错误码体系
- P2#17: 命令路由表

#### 待修正 / 待补充

**[A1] WebSocket 与 HTTP 端口分离的安全隐患**

当前设计中 WebSocket（19876）和 HTTP MCP（19875）是两个独立端口。WebSocket 通过 query param 传 token（`ws://host:19876?token=***`），这在以下场景存在风险：
- 浏览器历史记录可能保存 WebSocket URL（含 token）
- 代理服务器日志可能记录完整 URL
- 开发工具 Network 面板可见 token

**建议**: 在 WebSocket 连接建立后，通过第一条消息进行 token 认证（而非 query param），或至少支持两种方式。如果坚持用 query param，应在文档中明确标注安全风险。

**[A2] `activeTab` 权限的局限未充分说明**

`activeTab` 权限只在用户与 Extension 交互时（点击 popup、使用快捷键）临时授予当前 tab。当 Hermes 通过 MCP 远程操作非当前激活 tab 时，`activeTab` 权限可能不生效。

计划中提到 `ensureTabActive` 自动切换 tab，但切换后 `activeTab` 是否自动授予新 tab 存在不确定性（Chrome 文档表述模糊）。

**建议**: 明确测试场景 —— 当用户未与 Extension 交互时，Background 通过 `chrome.tabs.update({active: true})` 切换 tab 后，`chrome.scripting.executeScript` 是否能在新 tab 上执行。如果不行，可能需要 `"tabs"` + `"scripting"` + `"<all_urls>"` 的组合权限，而非依赖 `activeTab`。

实际上 manifest 中已经声明了 `"tabs"` 和 `"scripting"` 权限，但 `activeTab` 的局限性应在文档中说明。

**[A3] Content Script 在 `chrome://` 和 `file://` 页面的限制**

`chrome.scripting.executeScript` 无法在 `chrome://` 页面（如 chrome://extensions/）和某些 `file://` 页面注入 Content Script。这会导致操作这些页面时失败。

**建议**: 在错误码中增加 `UNSUPPORTED_PAGE` 或 `RESTRICTED_URL`，并在文档中说明限制。

**[A4] Extension 重载后的 Content Script 残留问题**

文档提到"页面中残留的旧 Content Script 的 listener 会失效"，但实际上旧 Content Script 的 listener 不会自动失效 —— 它仍然存在于页面上下文中，只是与旧的 Background 连接断开。当新 Background 向该 tab 发送消息时，旧 listener 可能仍然响应（如果它还在运行）。

更准确的描述：旧 Content Script 的 `chrome.runtime.onMessage` listener 仍然有效，但 `chrome.runtime.sendMessage` 回传时会失败（因为旧的 port 已断开）。如果页面中有多个 Content Script 实例（旧+新），可能导致消息被多个 listener 处理。

**建议**: 在 Content Script 中增加版本标识，收到消息时检查 sender 是否匹配当前 Extension ID，不匹配则忽略。或在注入前通过 `chrome.scripting.executeScript` 的 `func` 参数先清理旧 listener。

---

### 02-config-design.md — 配置文件设计

#### 已修正（✓）
- P1#5: 统一 token 设计
- P2#15: stdio 模式检测

#### 待修正 / 待补充

**[C1] Token 持久化路径与配置加载路径不一致**

Token 持久化到 `~/.hermes-browser/.token`，但配置文件搜索路径包含 `~/.hermes-browser/config.yaml`。如果用户通过 `-c` 指定了其他位置的配置文件，token 仍然写到 `~/.hermes-browser/.token`，这可能导致困惑。

**建议**: 明确 token 文件路径与配置文件路径的关系。或者让 token 文件与配置文件同目录（如 `filepath.Dir(configPath)/.token`），或始终使用固定路径但文档中说明。

**[C2] `networkidle` 实现过于简化**

当前实现：`load` 事件 + 固定 500ms 等待。这与真正的 network idle（在指定时间窗口内无网络活动）差距较大。对于重度动态加载的 SPA（如 React/Vue 应用），500ms 可能不够；对于简单页面，又可能过度等待。

**建议**:
1. 首版可用当前方案，但应在文档中标注为"简化版 networkidle"
2. 后续升级为 PerformanceObserver 方案（计划中提到但未排期）
3. 考虑增加 `networkidle_timeout` 配置项，让用户可以调整等待时间

**[C3] 环境变量覆盖缺少数组类型支持**

`allowed_extensions` 是数组类型，但环境变量覆盖方案 `HB_WEBSOCKET_ALLOWED_EXTENSIONS` 没有说明如何表示数组（逗号分隔？JSON？）。

**建议**: 明确数组类型环境变量的格式，如 `HB_WEBSOCKET_ALLOWED_EXTENSIONS="ext1,ext2,ext3"`。

**[C4] 日志配置缺少 `format` 选项**

当前日志配置只有 `level` 和 `file`，缺少结构化日志格式选项（如 JSON vs 文本）。作为服务端程序，结构化日志对后续排查问题很重要。

**建议**: 增加 `logging.format` 配置项，支持 `"text"` 和 `"json"`。

---

### 03-mcp-tools.md — MCP 工具定义

#### 已修正（✓）
- P2#10: 工具描述中增加错误码说明
- P2#11: full_page 标记 TODO

#### 待修正 / 待补充

**[T1] `screenshot` 工具缺少 `full_page` 参数声明**

虽然文档中标注了 TODO，但工具定义代码中没有任何关于 `full_page` 的注释或占位。建议在工具定义中保留注释说明，避免后续添加时忘记更新。

**[T2] `get_content` 的 `max_content_length` 截断未在工具参数中体现**

配置中有 `browser.max_content_length: 500000`，但工具定义中没有对应的参数让 LLM 可以控制返回长度。LLM 可能请求一个超大页面的内容，结果被静默截断，导致信息丢失。

**建议**: 在 `get_content` 工具中增加可选的 `max_length` 参数，覆盖全局配置。返回被截断时应在响应中明确告知（如增加 `truncated: true` 和 `total_length` 字段）。

**[T3] `execute_js` 的安全风险未充分说明**

`execute_js` 允许在页面上下文中执行任意 JavaScript，这可以：
- 读取页面上的敏感信息（密码、token）
- 修改页面状态（提交表单、删除数据）
- 发起跨域请求（利用页面的 CORS 权限）

虽然这是设计上的功能，但应在工具描述中明确标注安全提示。

**建议**: 在 `execute_js` 的工具描述中增加安全警告，提示 LLM 谨慎使用，避免执行不信任的代码。

**[T4] `scroll` 工具的 `amount` 参数类型模糊**

`amount` 可以是 `"one_page"`、`"half_page"` 或 `"500px"` 这样的 CSS 值。但 MCP 工具参数类型系统中，这只能定义为 `string`，无法做类型校验。

**建议**: 考虑拆分为两个参数：`amount_type`（enum: "page", "pixel"）和 `amount_value`（number/string），或者保留当前设计但在描述中明确说明格式。

**[T5] 工具缺少 `tab_id` 参数**

所有工具都需要操作某个 tab，但工具定义中没有 `tab_id` 参数。从 06-server-design.md 的 handler 代码看，`tabID` 是通过 `getActiveTabID()` 获取的，这意味着只能操作当前激活 tab。

这限制了多 tab 操作的灵活性。例如，LLM 可能想在 tab A 获取内容的同时在 tab B 执行操作。

**建议**: 在所有需要 tab 的工具中增加可选的 `tab_id` 参数，未指定时使用当前激活 tab。

**[T6] `close_tab` 的 `tab_id` 参数描述与实现不一致**

工具定义说 `tab_id` 是"omit to close current tab"，但 MCP 工具参数中 `tab_id` 没有标记为 Required，这意味着 LLM 可以不传。但 `getActiveTabID()` 获取的是当前激活 tab，如果用户想关闭后台 tab，必须传 `tab_id`。

这与 `switch_tab` 的设计也有冲突 —— `switch_tab` 的 `tab_id` 是 Required 的。

**建议**: 统一设计：所有 tab 相关操作的 `tab_id` 均为可选，默认当前激活 tab。

---

### 04-development-plan.md — 开发计划

#### 已修正（✓）
- P2#9: WebSocket 库改用 coder/websocket
- P2#14: icons 目录
- 任务拆分对齐 v2 架构变更

#### 待修正 / 待补充

**[D1] 缺少 Content Script 版本冲突处理**

Task 2.3 中没有提到处理 Extension 重载后旧 Content Script 残留的问题（见 A4）。

**建议**: 在 Task 2.3 中增加子任务：实现 Content Script 版本检查/清理机制。

**[D2] 缺少错误处理测试**

Task 3.4 的端到端测试中提到了"错误场景测试"，但没有具体说明要覆盖哪些错误码。

**建议**: 明确列出需要测试的错误场景：
- ELEMENT_NOT_FOUND（选择器不存在）
- TIMEOUT（操作超时）
- NAVIGATION_ERROR（无效 URL、网络错误）
- NO_EXTENSION_CONNECTED（Extension 断开）
- TAB_NOT_FOUND（无效 tab ID）
- JS_EXECUTION_ERROR（语法错误、运行时异常）
- PERMISSION_DENIED（CSP 限制、跨域限制）

**[D3] 性能优化阶段过于笼统**

Task 4.2 "性能优化"只有两条：大页面内容截断策略、截图压缩。缺少具体的优化目标和验收标准。

**建议**: 增加具体的性能指标，如：
- 截图响应时间 < 2s（1080p 页面）
- get_content 返回 < 100KB 文本时 < 1s
- WebSocket 重连时间 < 5s

**[D4] 缺少安全审计任务**

作为一个允许远程控制浏览器的系统，安全至关重要。但计划中没有专门的安全审计任务。

**建议**: 增加 Task 4.5 安全审计：
- Token 生成强度验证（32 字节随机）
- WebSocket token 传输安全（query param vs 消息内传输）
- HTTP transport 的 TLS 支持（如果需要跨公网）
- Origin 检查的有效性验证
- execute_js 的权限边界测试

**[D5] 时间估算可能偏乐观**

阶段一（2-3 天）包含 6 个任务，其中 Task 1.6 "MCP Server 框架"涉及 HTTP transport 中间件包装、WebSocket server 集成、信号处理等，复杂度较高。阶段二（2-3 天）的 Extension 开发涉及 Background/Content Script 的复杂交互。

**建议**: 为每个 Task 增加更细粒度的时间估算，或预留 20-30% 的缓冲时间。

---

### 05-extension-design.md — Chrome Extension 详细设计

#### 已修正（✓）
- P0#2: navigate 归 Background
- P1#4: screenshot 归 Background
- P1#6: cleanText 正则修正
- P1#7: networkidle 实现
- P1#8: activeTab / ensureTabActive
- P2#13: 删除 content_scripts 声明
- P2#14: icons
- P2#17: 命令路由表

#### 待修正 / 待补充

**[E1] `waitForTabComplete` 的超时处理不合理**

```javascript
setTimeout(() => {
    chrome.tabs.onUpdated.removeListener(listener);
    resolve(); // 超时也返回，不阻塞
}, 60000);
```

超时时 `resolve()` 而不是 `reject()`，这会导致 navigate 返回成功但实际页面可能未加载完成。LLM 可能基于错误的"成功"响应继续操作，导致后续命令失败。

**建议**: 超时应返回错误（`reject` 或返回 `{status: 'error', code: 'TIMEOUT'}`），让 LLM 知道页面未加载完成。

**[E2] `screenshot` 的元素裁剪实现不完整**

当前实现返回完整截图 + 元素坐标，标注"Element cropping not implemented in v1"。但 v2 计划中仍然没有实现方案。

**建议**: 明确元素裁剪的实现方案（如使用 OffscreenCanvas 在 Service Worker 中裁剪，或注入 Content Script 用 Canvas 裁剪），并安排到开发计划中。

**[E3] `chrome.tabs.captureVisibleTab` 的 `windowId` 获取可能失败**

```javascript
const tab = await chrome.tabs.get(tabId);
const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {...});
```

如果 tab 所在的窗口被最小化或隐藏，`captureVisibleTab` 可能失败或返回空白截图。

**建议**: 增加错误处理，捕获 `captureVisibleTab` 的异常，返回 `PERMISSION_DENIED` 或新的错误码。

**[E4] Content Script 的 `execute_js` 使用 `eval` 存在安全问题**

```javascript
case 'execute_js':
    sendResponse(executeJavaScript(params));
```

如果页面本身有 CSP（Content Security Policy）限制 `unsafe-eval`，`eval` 会失败。

**建议**: 说明 `execute_js` 在 CSP 限制页面的行为，或考虑使用 `chrome.scripting.executeScript`（在 Background 中执行，不受页面 CSP 限制）作为备选方案。

**[E5] `cleanText` 正则可能过度清理**

```javascript
function cleanText(text) {
    return text
        .replace(/[ \t]+/g, ' ')        // 行内多个空格/tab 压缩为一个
        .replace(/ *\n */g, '\n')       // 行尾行首空白清理
        .replace(/\n{3,}/g, '\n\n')     // 多个空行压缩为两个换行
        .trim();
}
```

`/[ \t]+/g` 会压缩代码块中的缩进（如 Python 代码的 4 空格缩进变成 1 空格），破坏代码可读性。

**建议**: 保留 `<pre>`、`<code>` 标签内的原始格式，或增加 `preserve_formatting` 参数让 LLM 选择是否清理。

**[E6] `wait_for` 使用 `requestAnimationFrame` 轮询可能过于频繁**

```javascript
requestAnimationFrame(check);
```

`requestAnimationFrame` 在可见页面约 60fps（16ms 间隔），对于不可见页面可能更慢或被节流。但对于简单的 DOM 查询，16ms 间隔过于频繁，浪费 CPU。

**建议**: 改用 `setTimeout(check, 100)` 或指数退避轮询（100ms -> 200ms -> 500ms）。

**[E7] Popup 的快捷操作缺少权限检查**

Popup 中的"截图当前页面"、"获取当前页面内容"快捷操作，如果当前页面是 `chrome://` 或受限页面，会失败但没有错误提示。

**建议**: 在 Popup 中增加页面类型检测，受限页面时禁用快捷操作并显示提示。

---

### 06-server-design.md — Go MCP Server 详细设计

#### 已修正（✓）
- P0#1: HTTP transport 中间件包装
- P0#3: Hub 单 client 模型
- P2#9: coder/websocket
- P2#10: 错误码
- P2#15: stdin 检测

#### 待修正 / 待补充

**[S1] `startWebSocketServer` 的返回值类型错误**

```go
wsServer := startWebSocketServer(ctx, cfg)
defer wsServer.Shutdown(ctx)
```

但从后面的代码看，`StartServer` 返回的是 `*http.Server`（不是自定义类型），而 `http.Server` 有 `Shutdown` 方法。这本身没问题，但 `startWebSocketServer` 这个函数名在代码中没有定义 —— 实际定义的是 `ws.StartServer`（从 `internal/ws` 包导入）。

**建议**: 统一命名，或明确说明 `startWebSocketServer` 是对 `ws.StartServer` 的包装。

**[S2] `authMiddleware` 中 `strings` 包未导入**

```go
provided := strings.TrimPrefix(token, "Bearer ")
```

但 main.go 的导入列表中没有 `strings` 包。

**建议**: 补充 `"strings"` 导入。

**[S3] `authMiddleware` 的 JSON 错误响应格式**

```go
http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
```

`http.Error` 默认设置 `Content-Type: text/plain; charset=utf-8`，但响应体是 JSON。客户端可能按 text/plain 解析，导致 JSON 解析失败。

**建议**: 手动设置 `Content-Type: application/json`：
```go
w.Header().Set("Content-Type", "application/json")
w.WriteHeader(http.StatusUnauthorized)
w.Write([]byte(`{"error":"missing authorization header"}`))
```

**[S4] `Hub.UnregisterClient` 的并发安全问题**

```go
func (h *Hub) UnregisterClient(client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.client == client {
        h.client = nil
        for id, ch := range h.responses {
            ch <- Response{...}
        }
    }
}
```

在持有 `mu.Lock` 的情况下向 channel 发送消息，如果 channel 已满或没有接收者，会阻塞。更严重的是，`HandleResponse` 也使用 `mu.RLock()` 访问 `h.responses`，如果 `UnregisterClient` 在遍历 `h.responses` 时阻塞，其他 goroutine 调用 `HandleResponse` 获取读锁也会被阻塞（写锁等待时，读锁获取被阻塞）。

**建议**: 在发送通知前复制 channel 列表，释放锁后再发送：
```go
func (h *Hub) UnregisterClient(client *Client) {
    h.mu.Lock()
    if h.client != client {
        h.mu.Unlock()
        return
    }
    h.client = nil
    // 复制 channel 列表
    pending := make(map[string]chan Response)
    for id, ch := range h.responses {
        pending[id] = ch
    }
    h.mu.Unlock()

    // 释放锁后再发送
    for id, ch := range pending {
        select {
        case ch <- Response{ID: id, Status: "error", Code: "NO_EXTENSION_CONNECTED", Error: "Extension disconnected"}:
        default:
        }
    }
}
```

**[S5] `Client.readLoop` 中 `json.Unmarshal` 的错误处理**

```go
var resp Response
if err := json.Unmarshal(data, &resp); err != nil {
    continue // 忽略格式错误的消息
}
```

如果 Extension 发送了格式错误的消息（如开发调试时的 log 消息），会被静默忽略。这在排查问题时会很困难。

**建议**: 至少记录日志（如果配置了 debug 级别）。

**[S6] `Client.SendJSON` 使用 `context.Background()`**

```go
return c.conn.Write(context.Background(), ws.MessageText, data)
```

使用 `context.Background()` 意味着写操作永远不会因上下文取消而中断。如果 Extension 已断开但 Hub 还未清理，`SendJSON` 可能阻塞。

**建议**: 使用带超时的 context，或检查 conn 的关闭状态。

**[S7] `HandleResponse` 中未找到 channel 的消息被丢弃**

```go
func (h *Hub) HandleResponse(resp Response) {
    h.mu.RLock()
    ch, ok := h.responses[resp.ID]
    h.mu.RUnlock()

    if ok {
        ch <- resp
    }
}
```

如果 `resp.ID` 不在 `h.responses` 中（如超时的请求已被清理），响应被静默丢弃。这可能是正常的（请求已超时），但也可能是 bug（ID 不匹配）。

**建议**: 增加 debug 日志记录未匹配的响应。

**[S8] WebSocket Server 的 `StartServer` 缺少 `hub` 参数传递**

```go
func StartServer(ctx context.Context, cfg *config.Config, hub *Hub) *http.Server {
```

但 main.go 中调用的是：
```go
wsServer := startWebSocketServer(ctx, cfg)
```

没有传递 `hub` 参数。实际上 `hub` 是在 `mcp.NewServer(cfg)` 中创建的，main.go 需要获取这个 hub 才能传给 WebSocket server。

**建议**: 调整初始化顺序：
```go
hub := ws.NewHub(cfg)
mcpServer := mcp.NewServer(cfg, hub) // 传入 hub
wsServer := ws.StartServer(ctx, cfg, hub)
```

**[S9] `getActiveTabID()` 的实现缺失**

06-server-design.md 的 handler 代码中使用了 `s.getActiveTabID()`，但没有给出实现。这个函数需要向 Extension 发送 `list_tabs` 命令并找到 active tab，或者 Extension 在连接时上报当前 active tab。

**建议**: 补充 `getActiveTabID` 的实现设计，或改为 Extension 连接时上报当前 tab 信息，Hub 缓存 active tab ID。

---

### 07-hermes-integration.md — Hermes 集成配置

#### 已修正（✓）
- P0#1: URL 路径与自定义 mux 一致

#### 待修正 / 待补充

**[H1] Hermes config.yaml 的 token 配置不完整**

文档中 token 配置只有注释说明：
```yaml
# Token 认证 - 通过 http_client 配置传递 Authorization header
# 方式一：环境变量
# HTTP_MCP_BROWSER_TOKEN=***
```

但没有给出 Hermes config.yaml 中实际如何配置 token。Hermes 的 MCP HTTP client 是否支持在 `config.yaml` 中配置 headers？

**建议**: 确认 Hermes 的 `mcp_tool.py` 中 `streamable_http_client` 的实现，给出完整的配置示例。如果不支持在 config.yaml 中配置 headers，需要修改 Hermes 端或提供其他方案（如环境变量）。

**[H2] curl 测试命令有语法错误**

```bash
curl -X POST http://192.168.199.54:19875/mcp \
  -H "Authorization: Bearer *** \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}'
```

`-H "Authorization: Bearer *** \` 这行缺少右引号。

**建议**: 修正为：
```bash
  -H "Authorization: Bearer ***" \
```

**[H3] stdio 模式的 WebSocket 配置矛盾**

```yaml
mcp_servers:
  hermes-browser:
    transport: stdio
    command: "/path/to/hermes-browser"
    args: ["-c", "/path/to/config.yaml"]
    # server.http.bind 留空，程序自动使用 stdio 模式
    # WebSocket 仍然需要配置 bind，供 Chrome Extension 连接
```

如果 `server.http.bind` 留空，程序使用 stdio 模式。但 WebSocket 仍然需要配置 bind。这意味着 stdio 模式下 MCP Server 仍然监听一个 HTTP 端口（WebSocket），只是不监听 MCP HTTP 端口。

这与"stdio 模式"的直觉不符 —— 用户可能期望 stdio 模式下没有任何 HTTP 端口监听。

**建议**: 明确说明 stdio 模式下 WebSocket 端口仍然需要监听，或考虑让 WebSocket 也支持 stdio 桥接（如通过 stdin/stdout 与 Extension 通信，但这需要 Extension 支持 Native Messaging）。

**[H4] 缺少 Hermes 端工具调用示例**

文档说明了如何配置，但没有给出 Hermes 实际调用工具的示例。对于开发者来说，看到具体的调用方式有助于理解集成效果。

**建议**: 增加示例：
```yaml
# Hermes 中使用工具
- 工具名: mcp_browser_navigate
  参数: { "url": "https://example.com" }
- 工具名: mcp_browser_screenshot
  参数: {}
```

---

## 三、跨文件一致性问题

**[X1] `tab_id` 字段命名不一致**

- 01-architecture.md: `tab_id`（WebSocket 请求）
- 03-mcp-tools.md: 工具参数中没有 `tab_id`
- 05-extension-design.md: `tabId`（JavaScript 变量）
- 06-server-design.md: `TabID`（Go 结构体字段）

建议统一为 `tab_id`（snake_case，与 MCP 工具参数风格一致）。

**[X2] 错误码常量命名风格不一致**

- 01-architecture.md: `ELEMENT_NOT_FOUND`（文档中的字符串）
- 06-server-design.md: `ErrCodeElementNotFound`（Go 常量）和 `ErrNoExtensionConnected`（Go 变量）

Go 代码中的命名风格正确（Go 惯例），但文档中应明确映射关系。

**[X3] `getActiveTabID` 的实现位置不明确**

06-server-design.md 中使用了 `s.getActiveTabID()`，但没有说明这个方法的实现位置（`internal/mcp/server.go` 还是 `internal/mcp/tools.go`）。

**[X4] `auth.Validate` 函数签名未定义**

06-server-design.md 中使用了 `auth.Validate(token, expected)`，但 `internal/auth/auth.go` 的设计在 04-development-plan.md 中只有简要说明，没有给出函数签名。

---

## 四、优先级建议

### P0 — 阻塞性问题（必须修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| S4 | Hub.UnregisterClient 并发安全问题 | 死锁风险 |
| S8 | WebSocket Server 缺少 hub 参数传递 | 编译错误 |
| E1 | waitForTabComplete 超时返回成功 | 错误状态传递 |
| H2 | curl 命令语法错误 | 用户无法按文档操作 |

### P1 — 重要问题（建议修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| A1 | WebSocket token 通过 query param 传输 | 安全风险 |
| S3 | authMiddleware JSON 响应 Content-Type 错误 | 客户端解析失败 |
| T5 | 工具缺少 tab_id 参数 | 多 tab 操作受限 |
| E3 | captureVisibleTab 窗口隐藏时失败 | 截图可靠性 |
| C1 | Token 持久化路径与配置路径不一致 | 用户困惑 |

### P2 — 改进建议（可选）

| 编号 | 问题 | 影响 |
|------|------|------|
| C2 | networkidle 实现过于简化 | SPA 页面兼容性 |
| T2 | get_content 截断未告知 | 信息丢失 |
| E5 | cleanText 破坏代码缩进 | 代码可读性 |
| D4 | 缺少安全审计任务 | 安全隐患 |
| A3 | chrome:// 页面限制未说明 | 用户体验 |

---

## 五、总结

v2 计划在架构层面已经比较成熟，核心设计决策合理，v1 审计中的主要问题已得到修正。当前最需要关注的是：

1. **并发安全**（S4 Hub 死锁风险）— 这是运行时可能触发的严重 bug
2. **参数传递完整性**（S8 hub 参数、T5 tab_id）— 影响功能完整性
3. **错误处理正确性**（E1 超时返回成功）— 影响 LLM 决策
4. **安全细节**（A1 token 传输、A2 activeTab 权限）— 影响系统安全

建议在进入开发前，先修正 P0 级别问题，并在开发过程中重点关注 P1 级别问题。
