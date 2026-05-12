# Hermes Browser v5 计划审计报告

审计模型：glm-5.1
审计版本：v5（继承自 v4 审计后的修复）
审计日期：2026-05-12
审计方法：7 份 v5 计划文件全文审读 + mcp-go (commit f901ca7c) 源码验证 + coder/websocket v1.8.14 验证 + Hermes mcp_tool.py headers 传递链路验证

---

## v4→v5 修复确认

v4 审计报告中的问题修复状态：

| v4编号 | 问题 | v5状态 |
|--------|------|--------|
| P0#1 | hub.go 缺 fmt import | ✅ 已修 |
| P0#2 | client.go 缺 encoding/json | ✅ 已修 |
| P0#3 | protocol.go 缺 fmt | ✅ 已修 |
| P0#4 | max_content_length 默认值 | ✅ 已修（500000） |
| P1#5 | HTTPConfig/WSConfig 缺 Token 字段 | ❌ 未修 |
| P1#6 | startStdioServer() 未定义 | ❌ 未修 |
| P1#7 | 15 个 Tool 变量声明缺失 | ❌ 未修 |
| P1#8 | client.go 未使用 import | ✅ 已修 |
| P2#9 | 07 curl 引号不闭合 | ❌ 未修 |
| P2#10 | ScreenshotQuality 未被引用 | ❌ 未修 |
| P2#11 | waitForTabComplete 超时 resolve 而非 reject | ❌ 未修 |

---

## 新发现

### P0 - 编译/运行时阻断

（无新增 P0）

### P1 - 功能缺陷

#### 1. [继承 P1#5] HTTPConfig/WSConfig 缺 Token 字段
06 Config 结构体中 HTTPConfig 和 WSConfig 缺少 `Token string` 字段。
02 配置文档定义了 `server.http.token` 和 `websocket.token`，但结构体无法接收。
GetHTTPToken()/GetWebSocketToken() 方法体为 `// ...` 空实现，无法工作。

#### 2. [继承 P1#6] startStdioServer() 未定义
06 main.go 第 60 行调用 `startStdioServer(mcpServer)` 但无函数定义。
需补充 stdio transport 启动逻辑。

#### 3. [继承 P1#7] 15 个 Tool 变量声明缺失
06 registerTools() 引用 `navigateTool`, `screenshotTool` 等变量。
03 中定义了 `mcp.NewTool(...)` 调用但未赋值给变量。
需在 tools.go 顶部声明所有 15 个 Tool 变量。

### P2 - 文档细节

#### 4. [继承 P2#9] 07 curl 示例引号不闭合
07 第 123 行 `-H "Authorization: Bearer *** \` 缺闭合引号。
应为 `-H "Authorization: Bearer ***" \`

#### 5. [继承 P2#10] ScreenshotQuality 未被代码引用
06 Config 定义了 `ScreenshotQuality int`，但 handleScreenshot 未读取该默认值。
目前 handler 中 `quality` 参数仅在用户显式传入时使用。

#### 6. [继承 P2#11] waitForTabComplete 超时 resolve 而非 reject
05 第 304-308 行超时后调用 `resolve()` 而非 `reject()`。
调用方无法区分正常完成和超时。应改为 `reject(new Error('timeout'))`
或在 resolve 中标记超时状态让调用方判断。

#### 7. [新增] 所有文件头部仍标记为 v4
01-07 的标题行都写 "v4"，实际已是 v5。

#### 8. [新增] client.go readLoop 中 Read 返回值处理
06 第 374 行 `_, data, err := c.conn.Read(ctx)`，
但 coder/websocket v1.8.14 的 `Read(ctx)` 返回 `(MessageType, []byte, error)` 三值。
当前代码正确忽略了 MessageType，无问题。

#### 9. [新增] ws/server.go import 中包别名与 hub.go/client.go 一致
三处都使用 `ws "github.com/coder/websocket"`，一致。无问题。

---

## API 验证总结

所有关键 API 已通过源码验证，计划中的用法正确：

| API | 验证结果 | 计划中用法 |
|-----|----------|-----------|
| `AddTool(tool, handler)` | ✅ 签名匹配 | registerTools() 正确 |
| `GetArguments() map[string]any` | ✅ 返回 map[string]any | handler 中正确 |
| `NewToolResultError(text)` | ✅ 单参数 | 正确 |
| `NewToolResultImage(text, imageData, mimeType)` | ✅ 三参数 | screenshot handler 正确 |
| `NewMCPServer(name, version)` | ✅ 无 WithToolHandler | 正确 |
| `StreamableHTTPServer.ServeHTTP` | ✅ 实现 http.Handler | mux.Handle 正确 |
| `coder/websocket Read/Write/Accept` | ✅ 签名匹配 | client.go / server.go 正确 |
| Hermes headers 传递 | ✅ httpx client 级别 | config.yaml 格式正确 |

---

## 结论

v5 继承了 v4 的 7 个未修复问题（3 P1 + 4 P2），无新增 P0。
核心架构和 API 用法已全部验证正确。
修复剩余 3 个 P1 后，计划可直接进入编码实现。
