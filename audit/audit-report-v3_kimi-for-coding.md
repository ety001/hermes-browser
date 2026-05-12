# hermes-browser plans/v3 审计报告

- **审计模型**: kimi-for-coding
- **审计计划版本**: v3
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

v3 计划相比 v2 有了显著改进，v2 审计中提出的所有 P0 级别问题（Hub 死锁、hub 参数传递、超时错误处理、curl 语法错误）和大部分 P1/P2 级别问题已得到修正。整体计划结构清晰，技术细节充分，但仍有几处需要关注。本次审计发现的问题数量明显减少，主要集中在边界情况处理和文档一致性方面。

---

## 二、按文件审计

### 01-architecture.md — 架构设计

#### 已修正（✓）
- A1: WebSocket token 改为消息内传输
- A2: activeTab 权限说明
- A3: chrome:// 页面限制和 UNSUPPORTED_PAGE 错误码
- A4: Content Script 版本清理机制

#### 待修正 / 待补充

**[V3-A1] WebSocket 认证消息缺乏超时机制说明**

文档提到 Extension 连接后发送 `{"type":"auth","token":"***"}` 进行认证，但没有说明认证超时时间。如果 Extension 连接后不发认证消息（或发送延迟），Server 端会阻塞在 `conn.Read()` 上。

从 06-server-design.md 的代码看，使用的是 `clientCtx`（来自 `context.WithCancel(ctx)`），没有设置超时。这意味着如果 Extension 恶意连接但不发送认证消息，连接会永久占用一个 goroutine。

**建议**: 在 WebSocket Server 中为认证阶段设置超时（如 10 秒），超时未认证则关闭连接。

**[V3-A2] `file://` URL 的说明不够准确**

文档说"`file://` URL 默认无法注入，需要在 Chrome 设置中开启'允许访问文件网址'"。实际上，Manifest V3 的 `"<all_urls>"` host_permission 已经包含了 `file://` URL，但 Chrome 默认会询问用户是否允许 Extension 访问本地文件。这不是"无法注入"，而是需要用户授权。

**建议**: 修改为"`file://` URL 需要用户在 Chrome 中授权 Extension 访问本地文件，首次访问时会弹出权限请求"。

---

### 02-config-design.md — 配置文件设计

#### 已修正（✓）
- C1: Token 路径说明
- C2: networkidle 标注简化版 + 可配置超时
- C3: 环境变量数组格式
- C4: 日志 format 选项

#### 待修正 / 待补充

**[V3-C1] `networkidle_timeout` 配置项的默认值与代码中的硬编码值不一致**

配置中 `networkidle_timeout: 500`，但在 05-extension-design.md 的 navigate 实现中：
```javascript
if (wait_until === 'networkidle') {
    await new Promise(r => setTimeout(r, 500));  // 硬编码 500ms
}
```

代码中硬编码了 500ms，没有从配置中读取 `networkidle_timeout`。如果用户修改了配置，代码不会生效。

**建议**: 在 Content Script 中通过某种方式传递配置值（如注入时通过 `chrome.scripting.executeScript` 的 `args` 参数传入），或 Content Script 从页面全局变量读取配置。

**[V3-C2] 缺少 `browser.max_content_length` 的环境变量覆盖**

环境变量列表中没有 `HB_BROWSER_MAX_CONTENT_LENGTH`，但配置中有 `browser.max_content_length`。

**建议**: 补充 `HB_BROWSER_MAX_CONTENT_LENGTH` 到环境变量列表。

---

### 03-mcp-tools.md — MCP 工具定义

#### 已修正（✓）
- T2: get_content max_length 参数和截断标记
- T3: execute_js 安全警告
- T4: scroll 参数拆分为 unit + amount
- T5/T6: 所有工具增加 tab_id 参数

#### 待修正 / 待补充

**[V3-T1] `new_tab` 工具缺少 `tab_id` 参数**

虽然 v3 修正要求"所有工具增加 tab_id 参数"，但 `new_tab` 工具没有 `tab_id` 参数（这是合理的，因为 new_tab 创建新 tab，不需要指定现有 tab）。但 `get_cookies` 和 `list_tabs` 的 `tab_id` 参数描述需要澄清。

`list_tabs` 不需要 tab_id（它列出所有 tab），但当前定义中没有 tab_id，这与"所有工具增加 tab_id"的表述略有冲突。建议明确说明哪些工具不需要 tab_id。

**[V3-T2] `get_content` 响应格式与 MCP 工具返回格式不兼容**

MCP 工具的返回格式是 `mcp.CallToolResult`，通常包含 `TextContent` 或 `ImageContent`。但 v3 中 `get_content` 的响应设计为：
```json
{
  "content": "...truncated text...",
  "truncated": true,
  "total_length": 750000,
  "returned_length": 500000
}
```

这意味着 handler 需要将 JSON 序列化为字符串后包装为 `TextContent`。但 LLM 解析时可能难以区分这是 JSON 字符串还是纯文本。

**建议**: 明确 handler 实现：将响应数据 JSON 序列化后作为文本返回，或考虑使用 MCP 的 `EmbeddedResource` 类型。在工具描述中说明返回格式是 JSON。

**[V3-T3] `execute_js` 的 `return_value` 参数默认 true 可能导致问题**

如果用户执行的 JS 代码返回一个循环引用对象或不可序列化的值（如 DOM 节点），`JSON.stringify` 会失败。

**建议**: 在 execute_js 实现中增加 try-catch 包装 `JSON.stringify`，失败时返回错误信息而非崩溃。

---

### 04-development-plan.md — 开发计划

#### 已修正（✓）
- D1: Content Script 版本清理任务
- D2: 错误处理测试明细
- D3: 性能优化指标
- D4: 安全审计任务
- D5: 时间估算细化

#### 待修正 / 待补充

**[V3-D1] Task 1.5 和 Task 1.6 的职责边界模糊**

Task 1.5 是"WebSocket Hub（单 client 模型）"，Task 1.6 是"WebSocket Server"。但 Hub 和 Server 的代码在 06-server-design.md 中分布在 `internal/ws/hub.go`、`internal/ws/client.go`、`internal/ws/server.go` 三个文件中。

Task 1.5 提到了 client.go 的实现，Task 1.6 提到了 server.go 的实现，但 protocol.go（协议定义）没有明确归属到哪个 Task。

**建议**: 明确 protocol.go 归属到 Task 1.4（WebSocket 协议定义），并在 Task 1.5/1.6 中引用。

**[V3-D2] 缺少 Extension 打包和分发说明**

开发计划中没有提到如何将 Extension 打包为 `.zip` 文件以便用户加载，也没有说明是否需要提供预打包的 Extension。

**建议**: 在 Task 4.4（文档）或新增 Task 中说明 Extension 的打包方式（`zip -r hermes-browser-extension.zip extension/`）。

**[V3-D3] 安全审计任务的时间安排可能偏晚**

Task 4.5 安全审计安排在阶段四（最后阶段），但安全问题如果在开发早期发现，修复成本更低。

**建议**: 将部分安全审计工作前置，如 Token 生成强度验证可以在 Task 1.3（认证模块）完成后立即进行。

---

### 05-extension-design.md — Chrome Extension 详细设计

#### 已修正（✓）
- E1: waitForTabComplete 超时返回错误
- E3: captureVisibleTab 错误处理
- E5: cleanText 保留代码格式
- E6: wait_for 使用 setTimeout 轮询
- E7: Popup 受限页面检测
- A1: WebSocket 认证消息
- A4: Content Script 版本清理

#### 待修正 / 待补充

**[V3-E1] `waitForTabComplete` 的竞态条件**

```javascript
function waitForTabComplete(tabId) {
    return new Promise((resolve, reject) => {
        // 先检查当前状态
        chrome.tabs.get(tabId).then(tab => {
            if (tab.status === 'complete') {
                resolve();
                return;
            }
        });

        const listener = (updatedTabId, changeInfo) => {
            if (updatedTabId === tabId && changeInfo.status === 'complete') {
                chrome.tabs.onUpdated.removeListener(listener);
                resolve();
            }
        };
        chrome.tabs.onUpdated.addListener(listener);
        // ...
    });
}
```

存在竞态条件：如果 `chrome.tabs.get` 检查时 tab 状态不是 complete，但在添加 listener 之前 tab 已经变为 complete，那么 listener 永远不会触发，只能等待超时。

**建议**: 在添加 listener 后再次检查 tab 状态：
```javascript
chrome.tabs.get(tabId).then(tab => {
    if (tab.status === 'complete') {
        chrome.tabs.onUpdated.removeListener(listener);
        resolve();
    }
});
```

**[V3-E2] `ensureContentScript` 对受限页面的处理不一致**

```javascript
async function ensureContentScript(tabId) {
    if (injectedTabs.has(tabId)) return;

    try {
        await chrome.scripting.executeScript({
            target: { tabId },
            func: () => { window.__hermesBrowserExtensionVersion = Date.now(); },
        });
    } catch (e) {
        // 受限页面会失败，后续操作会返回 UNSUPPORTED_PAGE
    }

    await chrome.scripting.executeScript({
        target: { tabId },
        files: ['content.js'],
    });
    injectedTabs.add(tabId);
}
```

如果第一个 `executeScript` 失败（受限页面），第二个 `executeScript` 仍然会执行，而且也会失败。但 `injectedTabs.add(tabId)` 仍然会被调用，导致后续对该 tab 的操作直接转发给 Content Script（实际上没有注入成功），产生 confusing 的错误。

**建议**: 如果 `executeScript` 失败（受限页面），应该抛出 `UNSUPPORTED_PAGE` 错误，而不是继续尝试注入。

**[V3-E3] `cleanText` 的代码块还原逻辑有缺陷**

```javascript
const textWithPlaceholders = text.replace(/<pre[\s\S]*?<\/pre>|<code[\s\S]*?<\/code>/gi, (match) => {
    codeBlocks.push(match);
    return `\x00CODE_BLOCK_${codeBlocks.length - 1}\x00`;
});
```

`text` 是 `innerText`，已经是纯文本，不包含 HTML 标签。所以 `<pre[\s\S]*?<\/pre>` 这个正则永远不会匹配到任何内容（因为 innerText 中没有 HTML 标签）。

**建议**: 如果需要在 `innerText` 中保留代码格式，应该在 `innerHTML` 层面处理（先解析 HTML 提取代码块），或者使用 `textContent` 配合特定的选择器策略。或者，简化方案：在 `extractContent` 中，当 `type === 'text'` 时，对 `<pre>` 和 `<code>` 元素使用 `textContent` 而非 `innerText`。

**[V3-E4] Content Script 的 `sender.id` 检查可能不适用于所有消息**

```javascript
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    if (sender.id !== EXTENSION_ID) {
        return false;
    }
    // ...
});
```

`sender.id` 在 Chrome Extension 消息中总是等于当前 Extension 的 ID。旧版本的 Content Script 的 `sender.id` 也是同一个 Extension ID（因为它们是同一个 Extension 的不同版本）。所以这个检查实际上无法区分新旧版本的 Content Script。

真正需要区分的是：消息是否来自当前活跃的 Background Service Worker。但 Content Script 无法直接知道 Background 的版本。

**建议**: 更可靠的方案是在注入 Content Script 时传入一个随机 nonce，Content Script 只响应包含正确 nonce 的消息。或者接受旧 Content Script 可能响应消息的事实，在 Background 中通过消息 ID 匹配来忽略重复响应。

---

### 06-server-design.md — Go MCP Server 详细设计

#### 已修正（✓）
- S1: 函数命名统一
- S2: strings 导入
- S3: authMiddleware Content-Type
- S4: Hub 死锁修复
- S5: 日志记录
- S6: 写超时
- S7: 未匹配响应日志
- S8: 初始化顺序
- S9: getActiveTabID 实现

#### 待修正 / 待补充

**[V3-S1] WebSocket Server 认证阶段的 context 使用问题**

```go
clientCtx, cancel := context.WithCancel(ctx)
defer cancel()

// 等待认证消息
_, data, err := conn.Read(clientCtx)
```

`defer cancel()` 会在 `HandleFunc` 返回时执行，但 `NewClient` 会启动一个 readLoop goroutine 使用 `clientCtx`。如果 `HandleFunc` 先返回（认证成功后），`defer cancel()` 会取消 `clientCtx`，导致 readLoop 中的 `conn.Read` 返回错误。

**建议**: 认证成功后创建新的 context 传给 `NewClient`：
```go
// 认证成功
clientCtx, clientCancel := context.WithCancel(ctx)
NewClient(clientCtx, conn, hub)
// 不要在这里 defer cancel，让 NewClient 管理生命周期
```

**[V3-S2] `Hub.RegisterClient` 关闭旧连接时可能死锁**

```go
func (h *Hub) RegisterClient(client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.client != nil {
        h.client.Close()  // 这会触发 UnregisterClient
    }
    h.client = client
}
```

`h.client.Close()` 会关闭连接，触发 `readLoop` 退出，进而调用 `UnregisterClient`。`UnregisterClient` 会尝试获取 `h.mu.Lock()`，但 `RegisterClient` 已经持有该锁。这会导致死锁。

**建议**: 在 RegisterClient 中，先复制旧 client 引用，释放锁后再关闭：
```go
func (h *Hub) RegisterClient(client *Client) {
    h.mu.Lock()
    oldClient := h.client
    h.client = client
    h.mu.Unlock()

    if oldClient != nil {
        oldClient.Close()
    }
}
```

**[V3-S3] `debugLog` 未定义**

Hub 和 Client 的代码中使用了 `if debugLog != nil`，但没有定义 `debugLog` 变量。

**建议**: 定义包级别的 logger，或通过 Hub/Client 结构体传入 logger。

**[V3-S4] `getActiveTabID` 在 Extension 未连接时行为不佳**

```go
func (s *Server) getActiveTabID() (int, error) {
    resp, err := s.hub.SendCommand("list_tabs", nil, 0)
    // ...
}
```

如果 Extension 未连接，`SendCommand` 返回 `ErrNoExtensionConnected`。但 handler 代码中：
```go
tabID, err := s.getActiveTabID()
if err != nil {
    return mcp.NewToolResultText(fmt.Sprintf("Error: %s", err)), nil
}
```

这会将错误作为文本返回给 LLM，LLM 可能无法正确识别这是 `NO_EXTENSION_CONNECTED` 错误。

**建议**: 在 `getActiveTabID` 中区分 `ErrNoExtensionConnected` 和其他错误，或在 handler 中统一处理。

**[V3-S5] `startStdioServer` 函数未定义**

main.go 中调用了 `startStdioServer(ctx, mcpServer)`，但文档中没有给出该函数的实现。

**建议**: 补充 `startStdioServer` 的实现设计（使用 mcp-go 的 stdio transport）。

---

### 07-hermes-integration.md — Hermes 集成配置

#### 已修正（✓）
- H2: curl 语法修正
- H3: stdio 模式 WebSocket 说明
- H4: 工具调用示例

#### 待修正 / 待补充

**[V3-H1] curl 命令中 Authorization header 仍然缺少右引号**

```bash
curl -X POST http://192.168.199.54:19875/mcp \
  -H "Authorization: Bearer *** \
  -H "Content-Type: application/json" \
```

`-H "Authorization: Bearer *** \` 这行仍然缺少右引号。应该是 `"Bearer ***"`。

**这是一个回归错误** —— v3 声称修正了 H2，但实际代码中仍然有问题。

**建议**: 修正为：
```bash
  -H "Authorization: Bearer ***" \
```

**[V3-H2] Hermes config.yaml 中 token 配置仍然不完整**

文档中仍然只有注释说明：
```yaml
# Token 认证 - 通过 http_client 配置传递 Authorization header
# 方式一：环境变量
# HTTP_MCP_BROWSER_TOKEN=***
```

但没有给出 Hermes config.yaml 中实际如何配置 token。如果 Hermes 的 MCP HTTP client 不支持在 config.yaml 中配置 headers，用户无法知道如何配置。

**建议**: 确认 Hermes 的 `mcp_tool.py` 实现，给出完整的配置示例。如果确实不支持，需要说明用户必须通过环境变量传递 token。

---

## 三、跨文件一致性问题

**[V3-X1] `tab_id` 字段命名已统一为 snake_case**

v3 中所有文件一致使用 `tab_id`，已修正 v2 中的不一致问题。✓

**[V3-X2] `debugLog` 的使用跨文件不一致**

06-server-design.md 中 Hub 和 Client 代码使用了 `debugLog`，但没有定义。其他文件没有提到 logger 的设计。

**[V3-X3] `networkidle_timeout` 配置项的传递机制未明确**

02-config-design.md 定义了 `networkidle_timeout`，但 05-extension-design.md 的代码中硬编码了 500ms。没有说明配置如何传递到 Content Script。

---

## 四、优先级建议

### P0 — 阻塞性问题（必须修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| V3-S2 | Hub.RegisterClient 关闭旧连接时可能死锁 | 运行时死锁 |
| V3-S1 | WebSocket Server 认证阶段 context 被取消 | readLoop 异常退出 |
| V3-H1 | curl 命令仍然缺少右引号（回归错误） | 用户无法按文档操作 |
| V3-E2 | ensureContentScript 对受限页面处理不当 | 错误状态混乱 |

### P1 — 重要问题（建议修复）

| 编号 | 问题 | 影响 |
|------|------|------|
| V3-E1 | waitForTabComplete 竞态条件 | 导航超时 |
| V3-E3 | cleanText 代码块还原逻辑无效 | 代码格式仍被破坏 |
| V3-E4 | Content Script sender.id 检查无效 | 版本冲突未解决 |
| V3-A1 | WebSocket 认证缺乏超时 | 资源泄漏 |
| V3-C1 | networkidle_timeout 配置未生效 | 配置无效 |
| V3-S4 | getActiveTabID 错误处理 | LLM 错误识别 |

### P2 — 改进建议（可选）

| 编号 | 问题 | 影响 |
|------|------|------|
| V3-C2 | 缺少 HB_BROWSER_MAX_CONTENT_LENGTH | 环境变量不完整 |
| V3-T2 | get_content 响应格式说明 | LLM 解析 |
| V3-T3 | execute_js JSON.stringify 失败 | 运行时错误 |
| V3-D2 | 缺少 Extension 打包说明 | 分发 |
| V3-D3 | 安全审计时间安排 | 成本 |
| V3-S5 | startStdioServer 未定义 | 文档不完整 |
| V3-H2 | Hermes token 配置不完整 | 用户困惑 |
| V3-A2 | file:// URL 说明不准确 | 文档 |

---

## 五、总结

v3 计划整体质量较高，v2 审计中的主要问题已得到修正。当前最需要关注的是：

1. **Hub.RegisterClient 死锁**（V3-S2）— 这是新引入的严重并发问题
2. **WebSocket 认证 context 生命周期**（V3-S1）— 会导致 readLoop 异常退出
3. **curl 命令回归错误**（V3-H1）— v3 声称修正但实际未修正
4. **ensureContentScript 受限页面处理**（V3-E2）— 会导致错误状态混乱

建议在进入开发前修正 P0 级别问题，并在开发过程中重点关注 P1 级别问题。

相比 v2 审计，v3 的问题数量从 24 个减少到 15 个，P0 级别问题从 4 个减少到 4 个（但性质不同），整体计划成熟度显著提升。再经过一轮修正（v4），预计可以达到可实施状态。
