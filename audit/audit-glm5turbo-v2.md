# Hermes Browser v2 计划审计报告

审计模型：glm-5-turbo
审计版本：v2
审计日期：2026-05-12
审计方法：交叉验证 mcp-go 源码(f901ca7c)、coder/websocket v1.8.14、Hermes mcp_tool.py

---

## P0 - 编译/运行时阻断

### 1. mcp.DefaultBoolean 不存在，应为 DefaultBool

**文件：** 03-mcp-tools.md（type、click 等多处）

mcp-go SDK 中属性选项函数名称为 `DefaultBool`（mcp/tools.go:1097），不是 `DefaultBoolean`。

受影响工具：type、execute_js 等所有使用布尔参数的工具。

```
mcp.WithBoolean("clear_first", mcp.DefaultBoolean(true), ...)  // 编译失败
mcp.WithBoolean("clear_first", mcp.DefaultBool(true), ...)     // 正确
```

### 2. server.WithToolHandler 不存在

**文件：** 06-server-design.md（server.go NewServer 函数）

```go
s.mcpS = server.NewMCPServer(
    "hermes-browser", "1.0.0",
    server.WithToolHandler(s.handleToolCall),  // 不存在
)
```

mcp-go 没有 `WithToolHandler` 选项。实际存在的是 `WithToolHandlerMiddleware`（middleware 模式），但这个场景根本不需要。计划中的 `registerTools()` 已经用 `AddTool(tool, handler)` 逐个注册了 handler，不需要全局 handler。直接删掉 `server.WithToolHandler(s.handleToolCall)` 即可。

### 3. Hermes config.yaml 的 mcp_servers 格式错误

**文件：** 07-hermes-integration.md

计划中：
```yaml
mcp_servers:
  hermes-browser:
    transport: http                        # 无效字段，被忽略
    url: "http://192.168.199.54:19875/mcp"
    # Token 认证注释不清晰
```

Hermes 不使用 `transport` 字段判断类型，通过 `url` 是否存在来判断。认证 header 通过 `headers` 字段传递，支持 `${ENV_VAR}` 环境变量插值。正确格式：

```yaml
mcp_servers:
  hermes-browser:
    url: "http://192.168.199.54:19875/mcp"
    headers:
      Authorization: "Bearer ${MCP_BROWSER_TOKEN}"
    timeout: 180
    connect_timeout: 60
```

### 4. mcp-go CallToolRequest 参数访问方式未确认

**文件：** 06-server-design.md（tools.go）

计划中用 `req.Params.Arguments["url"].(string)` 访问参数，但 mcp-go 的 CallToolRequest 结构体：
- `Params` 类型是 `CallToolParams`
- `CallToolParams.Arguments` 类型是 `any`（不是 map[string]any）
- SDK 提供了辅助方法 `req.GetArguments()` 返回 `map[string]any`

需要验证 `req.Params.Arguments` 是否可以直接类型断言为 `map[string]any`，或者必须用 `req.GetArguments()`。

### 5. WebSocket Server handler 中引用了 auth 包但未 import

**文件：** 06-server-design.md（ws/server.go）

WebSocket Server 的 handler 函数中调用了 `auth.Validate()` 和 `isAllowedOrigin()`，但这些函数在 `internal/auth` 包中，`internal/ws` 包需要 import 它。同时 `isAllowedOrigin` 函数未在任何文档中定义实现。

---

## P1 - 设计缺陷

### 6. Hub.SendCommand 超时硬编码为 30s

**文件：** 06-server-design.md（hub.go）

Hub 的 timeout 硬编码为 30s，但 navigate 工具的默认超时是 30s，加上网络传输开销，会超时。应在 NewHub 时从 config.Browser.DefaultTimeout 读取，并且允许每个命令设置独立超时。

### 7. main.go 中 WebSocket Server 启动逻辑未实现

**文件：** 06-server-design.md（main.go）

main.go 调用了 `startWebSocketServer(ctx, cfg)` 但这个函数未定义。实际的 WebSocket 启动逻辑在 `internal/ws/server.go` 的 `StartServer` 中，需要一个桥接函数把 Hub 传进去。

同时 Hub 在 main.go 中没有创建——mcp.NewServer(cfg) 内部创建了 Hub，但 WebSocket server 也需要这个 Hub。需要确保 MCP Server 和 WebSocket Server 共享同一个 Hub 实例。

### 8. Content Script onMessage 异步处理不完整

**文件：** 05-extension-design.md（content.js）

只有 `wait_for` 返回了 `true`（保持通道开放），但实际上 `execute_js` 如果执行异步代码也需要异步响应。当前设计 `execute_js` 用 `eval()` 是同步的，但如果用户代码中有 Promise，会丢失结果。

### 9. navigate 流程中 tab_id 来源不明

**文件：** 05-extension-design.md、06-server-design.md

MCP handler 调用 `s.getActiveTabID()` 但这个函数未定义。在 MCP 工具定义中，只有 switch_tab/close_tab 有 `tab_id` 参数，navigate、screenshot 等工具没有 `tab_id` 参数。

需要在工具定义中为所有需要操作页面的工具添加 `tab_id` 参数（可选，默认当前活跃 tab），或者在 MCP handler 中通过 Extension 的 list_tabs 获取当前活跃 tab。

---

## P2 - 优化建议

### 10. captureVisibleTab 的 quality 参数类型

Chrome API `captureVisibleTab` 的 quality 参数需要整数（0-100），但 MCP 工具定义中用 `mcp.WithNumber`，从 Arguments 取出时是 float64。需要在 Extension 端做类型转换。

### 11. cleanText 会将代码块中的缩进清除

`cleanText` 的 `replace(/[ \t]+/g, ' ')` 会把代码块中的缩进全部清除。get_content 返回 HTML 时没问题（innerHTML 不经过 cleanText），但 text 模式下代码内容会被破坏。建议在 get_content 的 text 模式下，先识别 `<pre>` 和 `<code>` 块，保留其原始格式。

### 12.waitForTabComplete 超时后仍 resolve 而非 reject

**文件：** 05-extension-design.md

waitForTabComplete 超时后调用 `resolve()`，但调用方无法区分"真正加载完成"和"超时"。建议 reject 或返回超时标记。

### 13. NewToolResultImage 函数需确认

**文件：** 06-server-design.md（tools.go）

代码中使用了 `mcp.NewToolResultImage(imageBase64, "image/jpeg")`，需确认 mcp-go 是否有这个函数。mcp-go 返回图片可能需要用 `mcp.NewToolResult` + `mcp.ImageContent` 的组合方式。

### 14. missing icon16.png 等文件

**文件：** 项目结构

extension/icons/ 目录在 v2 计划中有声明，但 Task 1.1 只说了"准备占位图标"。需要确认是放在项目仓库中还是首次运行时生成。

---

## 审计结论

P0 有 5 个编译级阻断（3 个 API 不存在、1 个配置格式错误、1 个参数访问方式不确定）。P1 有 4 个设计缺陷需在编码前解决。建议修正后生成 v3 版本。
