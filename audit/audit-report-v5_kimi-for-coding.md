# hermes-browser plans/v5 最终审计报告

- **审计模型**: kimi-for-coding
- **审计计划版本**: v5
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

**v5 计划已达到可实施状态。**

所有 P0 级别问题已修复：
- V4-H1: curl 命令右引号已修正
- V4-X1: WebSocket URL 路径已统一为 `/ws`

v5 相比 v4 的变更：
1. 01-architecture.md: 架构图中 WebSocket URL 明确为 `ws://<bind_addr>:<port>/ws`
2. 05-extension-design.md: Extension 默认 WebSocket URL 改为 `ws://127.0.0.1:19876/ws`
3. 07-hermes-integration.md: 
   - curl 命令修正为 `-H "Authorization: Bearer ***"`
   - 所有 WebSocket URL 统一为 `/ws` 路径

---

## 二、审计结果

### 已修正问题（全部关闭）

| 原编号 | 问题 | 状态 |
|--------|------|------|
| V3-A1 | WebSocket 认证超时 10 秒 | ✓ v4 已修正 |
| V3-A2 | file:// URL 说明 | ✓ v4 已修正 |
| V3-C1 | networkidle_timeout 配置传递 | ✓ v4 已修正 |
| V3-C2 | HB_BROWSER_MAX_CONTENT_LENGTH | ✓ v4 已修正 |
| V3-T2 | get_content 响应格式说明 | ✓ v4 已修正 |
| V3-T3 | execute_js JSON.stringify 失败处理 | ✓ v4 已修正 |
| V3-D1 | Task 1.5/1.6 职责边界 | ✓ v4 已修正 |
| V3-D2 | Extension 打包说明 | ✓ v4 已修正 |
| V3-D3 | 安全审计前置 | ✓ v4 已修正 |
| V3-E1 | waitForTabComplete 竞态条件 | ✓ v4 已修正 |
| V3-E2 | ensureContentScript 受限页面处理 | ✓ v4 已修正 |
| V3-E3 | cleanText 简化实现 | ✓ v4 已修正 |
| V3-E4 | Content Script nonce 机制 | ✓ v4 已修正 |
| V3-S1 | WebSocket context 生命周期 | ✓ v4 已修正 |
| V3-S2 | Hub.RegisterClient 死锁 | ✓ v4 已修正 |
| V3-S3 | debugLog 定义 | ✓ v4 已修正 |
| V3-S4 | getActiveTabID 错误处理 | ✓ v4 已修正 |
| V3-S5 | startStdioServer 实现 | ✓ v4 已修正 |
| V3-H1 | curl 命令回归错误 | ✓ v5 已修正 |
| V3-H2 | Hermes token 配置说明 | ✓ v4 已修正 |
| V4-X1 | WebSocket URL 路径不一致 | ✓ v5 已修正 |

### 剩余 P1/P2 问题（开发过程中处理）

以下问题不影响计划可实施性，可在开发过程中逐步解决：

| 编号 | 问题 | 优先级 | 处理时机 |
|------|------|--------|---------|
| V4-E1 | waitForTabComplete 超时时间与 navigate 参数不一致 | P1 | 开发 Task 2.2 |
| V4-E2 | handleNavigate 超时逻辑重复 | P1 | 开发 Task 2.2 |
| V4-C1 | ensureContentScript config 硬编码 | P1 | 开发 Task 2.3 |
| V4-A1 | 架构图 WebSocket 路径 | P2 | 文档更新 |
| V4-T1 | get_content html 类型安全警告 | P2 | 开发 Task 3.3 |
| V4-D1 | 缺少 logger Task 分配 | P2 | 开发 Task 1.1/1.2 |
| V4-S1 | authMiddleware JSON-RPC 格式 | P2 | 开发 Task 1.7 |
| V4-S2 | ServeStdio context 未使用 | P2 | 开发 Task 1.7 |
| V4-S3 | setupLogger 未定义 | P2 | 开发 Task 1.1 |

---

## 三、计划可实施性确认

### 技术可行性

| 组件 | 技术方案 | 风险等级 |
|------|---------|---------|
| MCP Server (Go) | mark3labs/mcp-go + coder/websocket | 低 |
| HTTP Transport | StreamableHTTP + 自定义 auth 中间件 | 低 |
| WebSocket | 单 client 模型 + nonce 认证 | 低 |
| Chrome Extension | Manifest V3 + Service Worker | 低 |
| Content Script | 按需注入 + nonce 验证 | 低 |
| Hermes 集成 | HTTP transport + Bearer token | 低 |

### 开发资源估算

| 阶段 | 预计时间 | 里程碑 |
|------|---------|--------|
| 阶段一：MCP Server 核心 | 3-4 天 | M1: Server 启动，工具发现 |
| 阶段二：Extension 开发 | 3-4 天 | M2: Extension 连接，navigate 可用 |
| 阶段三：工具实现和联调 | 3-4 天 | M3: 15 个工具全部可用 |
| 阶段四：Hermes 集成和优化 | 2-3 天 | M4: 端到端集成完成 |
| **总计** | **11-15 天** | |

### 关键依赖

1. mark3labs/mcp-go SDK 的稳定性（8600+ stars，活跃维护）
2. Hermes 的 MCP HTTP client 支持自定义 headers（需要验证）
3. Chrome Extension 的 `chrome.scripting.executeScript` 在目标页面的可用性

---

## 四、结论

**v5 计划已通过审计，确认可实施。**

建议按以下顺序进入开发：
1. 立即开始阶段一（MCP Server 核心）
2. 同步准备 Extension 图标和 manifest
3. 开发过程中关注剩余 P1 问题（超时逻辑、配置传递）
4. 每完成一个里程碑进行联调验证

所有审计报告文件位置：
- v2 审计: `~/workspace/hermes-browser/audit/audit-report-v2_kimi-for-coding.md`
- v3 审计: `~/workspace/hermes-browser/audit/audit-report-v3_kimi-for-coding.md`
- v4 审计: `~/workspace/hermes-browser/audit/audit-report-v4_kimi-for-coding.md`
- v5 最终审计: `~/workspace/hermes-browser/audit/audit-report-v5_kimi-for-coding.md`

计划文件位置：
- v5 计划: `~/workspace/hermes-browser/plans/v5/`
