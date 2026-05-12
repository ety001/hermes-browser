# Hermes Browser v3 计划审计报告

审计模型：glm-5-turbo
审计版本：v3
审计日期：2026-05-12
审计方法：交叉验证 mcp-go 源码(f901ca7c)、coder/websocket v1.8.14、7 份 v3 计划文件交叉检查

---

## P0 - 编译/运行时阻断

### 1. WithEndpointPath 作为 http.Handler 无效

mcp-go `WithEndpointPath` 的注释明确说明：
> 仅在 `Start` 方法中生效；作为 `http.Handler` 使用时无效

v3/06-server-design.md 中使用自定义 mux 包装 `StreamableHTTPServer` 作为 `http.Handler`，
此时 `WithEndpointPath("/mcp")` 完全无效。路径匹配由自定义 mux 的 `mux.Handle("/mcp", ...)`
控制，所以路由能工作，但 `WithEndpointPath` 是误导性代码。

**修复：** 删除 `WithEndpointPath` 调用，路径由 mux 完全控制。

### 2. coder/websocket Read/Write 签名与 Client 代码不一致

coder/websocket v1.8.14 的实际签名：
- `Accept(w, r, *AcceptOptions)` — opts 是指针类型，v3 传 nil 正确
- `Conn.Read(ctx)` 返回 `(MessageType, []byte, error)` — 不是 Read(ctx, msg)
- `Conn.Write(ctx, typ MessageType, p []byte)` 返回 error — 不是 Write(ctx, msg)

需要检查 v3/06-server-design.md 中 Client 的 Read/Write 代码是否正确使用了这个签名。

### 3. execute_js 参数名不一致

| 文件 | 参数名 |
|------|--------|
| 03-mcp-tools.md | `mcp.WithString("code", ...)` |
| 05-extension-design.md | `const { expression, return_value = true } = params` |

Extension 端会取到 `undefined`，JS 代码永远不会执行。

**修复：** 统一为 `expression`（与 Extension 和 MCP tool handler 一致）。

## P1 - 功能缺陷

### 4. navigate timeout 被 Extension 忽略

03-mcp-tools.md 定义了 navigate 的 `timeout` 参数（默认 30000ms）。
06-server-design.md Go handler 正确将 timeout 传入 params。
但 05-extension-design.md `handleNavigate` 只解构 `url` 和 `wait_until`，
**没有使用 timeout**，`waitForTabComplete(tabId)` 始终使用硬编码 60000ms。

**修复：** `handleNavigate` 解构 `timeout` 并传给 `waitForTabComplete`。

### 5. 04-development-plan.md 缺少 cookies 权限

05-extension-design.md manifest 有 `cookies` 权限，04 开发计划的 permissions 列表缺少。

**修复：** 04 中添加 `cookies`。

### 6. 项目结构缺少 internal/ws/server.go

01-architecture.md 项目结构只列了 `hub.go, client.go, protocol.go`，
但 06-server-design.md 有 `StartServer()` 在 `server.go` 中。

**修复：** 01 项目结构添加 `server.go`。

## P2 - 文档/完整性

### 7. 错误码汇总表不完整

03-mcp-tools.md 顶部汇总只列 6 个错误码，缺少 TAB_CLOSED、PERMISSION_DENIED、UNKNOWN_METHOD。

### 8. Config 辅助方法未在 02 中定义

06 调用了 `cfg.GetHTTPToken()`、`cfg.GetWebSocketToken()`、`cfg.Browser.DefaultTimeout` 等，
但 02-config-design.md 没有定义 Go 结构体和方法签名。

### 9. screenshot 配置值未被代码引用

02 定义了 `screenshot_format`/`screenshot_quality`，但 06 handler 中硬编码默认值。

### 10. 07 curl 示例引号未闭合

```bash
-H "Authorization: Bearer *** \
# 缺少闭合引号
```
