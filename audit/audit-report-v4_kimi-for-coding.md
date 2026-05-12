# hermes-browser plans/v4 审计报告

- **审计模型**: kimi-for-coding
- **审计计划版本**: v4
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

v4 计划相比 v3 有了显著改进。v3 审计中的 4 个 P0 级别问题中，3 个已得到修正（Hub 死锁、WebSocket context 生命周期、ensureContentScript 受限页面处理）。但 1 个 P0 问题（curl 命令回归错误）仍然存在，这是一个需要立即修复的文档错误。

整体计划的技术深度和完整性已达到可实施水平。剩余问题主要是文档层面的细节，不影响开发实施。

---

## 二、按文件审计

### 01-architecture.md — 架构设计

#### 已修正（✓）
- V3-A1: WebSocket 认证超时 10 秒
- V3-A2: file:// URL 说明修正
- V3-E4: Content Script nonce 机制

#### 待修正 / 待补充

**[V4-A1] 架构图中 WebSocket 路径缺少 `/ws` 后缀**

架构图中的 WebSocket 连接写的是 `ws://<bind_addr>:<port>`，但实际 WebSocket Server 代码中注册的路径是 `/ws`（`mux.HandleFunc("/ws", ...)`）。如果 Extension 连接时不带 `/ws` 路径，会 404。

**建议**: 在架构图和文档中明确 WebSocket URL 格式为 `ws://<bind_addr>:<port>/ws`。

---

### 02-config-design.md — 配置文件设计

#### 已修正（✓）
- V3-C1: networkidle_timeout 配置传递机制（通过 executeScript args 注入）
- V3-C2: 补充 HB_BROWSER_MAX_CONTENT_LENGTH

#### 待修正 / 待补充

**[V4-C1] `ensureContentScript` 中 config 值硬编码**

```javascript
const config = {
    networkidleTimeout: 500, // 从 chrome.storage 或全局配置读取
    maxContentLength: 500000,
};
```

注释说"从 chrome.storage 或全局配置读取"，但实际代码中仍然是硬编码的 500 和 500000。虽然计划文档中说明了配置传递机制，但代码示例中未展示如何从 MCP Server 配置传递到 Extension。

**建议**: 在 05-extension-design.md 中补充说明：Extension 启动时通过 WebSocket 从 MCP Server 获取配置，或 Extension 直接从 chrome.storage 读取用户配置。当前简化方案是硬编码默认值，后续版本实现配置同步。

---

### 03-mcp-tools.md — MCP 工具定义

#### 已修正（✓）
- V3-T2: get_content 响应格式说明
- V3-T3: execute_js JSON.stringify 失败处理

#### 待修正 / 待补充

**[V4-T1] `get_content` 的 `type` 参数描述中 "html" 类型的安全风险未说明**

`get_content` 支持 `type: "html"`，返回原始 HTML。这可能包含 `<script>` 标签等潜在危险内容。虽然这是用户请求的行为，但工具描述中应提醒 LLM 注意。

**建议**: 在工具描述中增加警告："html 类型返回原始 HTML，可能包含脚本标签，请谨慎处理。"

---

### 04-development-plan.md — 开发计划

#### 已修正（✓）
- V3-D1: Task 1.5/1.6 职责边界明确
- V3-D2: Extension 打包说明
- V3-D3: 安全审计前置到 Task 1.3

#### 待修正 / 待补充

**[V4-D1] 缺少 `internal/logger` 包的 Task 分配**

04-development-plan.md 中没有明确提到 `internal/logger` 包的开发任务。虽然 06-server-design.md 中提供了 logger 的实现，但开发计划中没有对应的 Task。

**建议**: 在 Task 1.1（初始化）或 Task 1.2（配置模块）中增加 logger 初始化。

---

### 05-extension-design.md — Chrome Extension 详细设计

#### 已修正（✓）
- V3-E1: waitForTabComplete 竞态条件修复（添加 listener 后再次检查 + resolved 标志）
- V3-E2: ensureContentScript 受限页面处理（注入失败时抛出 UNSUPPORTED_PAGE）
- V3-E3: cleanText 简化实现
- V3-E4: Content Script nonce 机制

#### 待修正 / 待补充

**[V4-E1] `waitForTabComplete` 的超时时间硬编码为 60s**

```javascript
setTimeout(() => {
    // ...
    reject({ code: 'TIMEOUT', message: 'Navigation timed out after 60s' });
}, 60000);
```

超时时间硬编码为 60s，但 navigate 工具的 timeout 参数默认是 30000ms。这意味着即使 LLM 设置了 30s 超时，Background 仍然会等待 60s。

**建议**: 将 navigate 的 timeout 参数传递给 `waitForTabComplete`，或者使用 navigate 的 timeout 值。

**[V4-E2] `handleNavigate` 中 networkidle 等待与 `waitForTabComplete` 重复**

```javascript
async function handleNavigate(tabId, params) {
    // ...
    if (wait_until === 'load' || wait_until === 'networkidle') {
        await waitForTabComplete(tabId);  // 等待到 complete
    }
    if (wait_until === 'networkidle') {
        await new Promise(r => setTimeout(r, 500));  // 再额外等待 500ms
    }
}
```

当 `wait_until === 'networkidle'` 时，先调用 `waitForTabComplete`（等待到 complete 状态），然后再额外等待 500ms。这是正确的行为，但 `waitForTabComplete` 的超时（60s）与 navigate 的 timeout 参数（30s）不一致。

**建议**: 统一超时逻辑，将 navigate 的 timeout 参数传递给 `waitForTabComplete`。

---

### 06-server-design.md — Go MCP Server 详细设计

#### 已修正（✓）
- V3-S1: WebSocket Server 认证阶段 context 生命周期（认证成功后创建新 context）
- V3-S2: Hub.RegisterClient 死锁修复（先赋值再关闭旧连接）
- V3-S3: debugLog 定义（新增 internal/logger 包）
- V3-S4: getActiveTabID 错误处理（区分 ErrNoExtensionConnected）
- V3-S5: startStdioServer 实现设计

#### 待修正 / 待补充

**[V4-S1] `authMiddleware` 的 JSON 错误响应格式不符合 MCP 规范**

```go
fmt.Fprint(w, `{"jsonrpc":"2.0","error":{"code":-32001,"message":"Unauthorized"}}`)
```

MCP 的 JSON-RPC 错误响应需要包含 `id` 字段。如果认证失败，无法知道原始请求的 id。此外，StreamableHTTP 规范要求错误响应也是合法的 JSON-RPC 响应。

**建议**: 由于中间件在 MCP handler 之前执行，无法获取 JSON-RPC 请求的 id。可以：
1. 返回 HTTP 401 状态码 + 简单的 JSON 错误（当前做法，虽然不是标准 JSON-RPC，但 HTTP 层已足够表达错误）
2. 或者将认证逻辑集成到 MCP handler 内部

当前做法在实际中是可以工作的，因为 Hermes 的 HTTP client 会检查 HTTP 状态码。

**[V4-S2] `ServeStdio` 的 context 参数未使用**

```go
func (s *Server) ServeStdio(ctx context.Context) error {
    return server.ServeStdio(s.mcp)
}
```

context 参数传入但未使用。如果需要在 stdio 模式下支持优雅关闭，需要将 context 传递给 `server.ServeStdio` 或使用其他机制。

**建议**: 检查 mcp-go 的 `server.ServeStdio` 是否支持 context，如果不支持，在文档中说明 stdio 模式下不支持优雅关闭。

**[V4-S3] `setupLogger` 函数未定义**

main.go 中调用了 `setupLogger(cfg)`，但文档中没有给出该函数的实现。

**建议**: 补充 `setupLogger` 的实现，或说明它在 `internal/logger` 包中。

---

### 07-hermes-integration.md — Hermes 集成配置

#### 已修正（✓）
- V3-H2: Hermes token 配置说明（增加了 headers 配置说明和备选方案）

#### 待修正 / 待补充

**[V4-H1] curl 命令仍然缺少右引号（P0 回归错误）**

```bash
curl -X POST http://192.168.199.54:19875/mcp \
  -H "Authorization: Bearer *** \
  -H "Content-Type: application/json" \
```

`-H "Authorization: Bearer *** \` 这行仍然缺少右引号。应该是 `"Bearer ***"`。

**这是 v3 审计中已标记的 P0 问题，v4 声称修正但实际未修正。**

**建议**: 立即修正为：
```bash
  -H "Authorization: Bearer ***" \
```

---

## 三、跨文件一致性问题

**[V4-X1] WebSocket URL 路径不一致**

- 01-architecture.md: `ws://<bind_addr>:<port>`（缺少 `/ws`）
- 02-config-design.md: WebSocket 配置中没有提到路径
- 05-extension-design.md: `wsUrl = 'ws://127.0.0.1:19876'`（缺少 `/ws`）
- 06-server-design.md: `mux.HandleFunc("/ws", ...)`（路径是 `/ws`）
- 07-hermes-integration.md: `ws://192.168.199.54:19876`（缺少 `/ws`）

Extension 代码中的 WebSocket URL 没有包含 `/ws` 路径，但 Server 注册在 `/ws` 路径上。这会导致连接 404。

**建议**: 统一所有文档和代码中的 WebSocket URL 为 `ws://<host>:<port>/ws`。

---

## 四、优先级建议

### P0 — 阻塞性问题（必须修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| V4-H1 | curl 命令仍然缺少右引号（第二次回归） | 用户无法按文档操作 |
| V4-X1 | WebSocket URL 路径不一致（缺少 `/ws`） | Extension 连接 404 |

### P1 — 重要问题（建议修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| V4-E1 | waitForTabComplete 超时时间与 navigate 参数不一致 | 超时行为不符合预期 |
| V4-E2 | handleNavigate 超时逻辑重复 | 代码冗余 |
| V4-C1 | ensureContentScript config 硬编码 | 配置不生效 |

### P2 — 改进建议（可选）

| 编号 | 问题 | 影响 |
|------|------|------|
| V4-A1 | 架构图 WebSocket 路径 | 文档 |
| V4-T1 | get_content html 类型安全警告 | 安全 |
| V4-D1 | 缺少 logger Task 分配 | 计划完整性 |
| V4-S1 | authMiddleware JSON-RPC 格式 | 规范合规 |
| V4-S2 | ServeStdio context 未使用 | 优雅关闭 |
| V4-S3 | setupLogger 未定义 | 文档完整性 |

---

## 五、总结

v4 计划整体质量很高，v3 审计中的主要技术问题已得到修正。当前最需要关注的是：

1. **curl 命令回归错误**（V4-H1）— 这是第二次出现，说明复制粘贴时容易遗漏
2. **WebSocket URL 路径不一致**（V4-X1）— 会导致 Extension 无法连接

这两个问题都是文档/配置层面的，修复成本低但影响大。

相比 v3 审计，v4 的问题数量从 15 个减少到 8 个，P0 级别从 4 个减少到 2 个。计划已达到**基本可实施**状态。

**建议**: 修复 V4-H1 和 V4-X1 后，即可进入开发阶段。开发过程中注意：
- 所有 WebSocket URL 统一使用 `/ws` 路径
- navigate 的超时参数需要传递给 waitForTabComplete
- config 值从硬编码改为从配置读取

如果用户要求生成 v5，只需修正 V4-H1 和 V4-X1 即可，其他 P1/P2 问题可以在开发过程中逐步解决。
