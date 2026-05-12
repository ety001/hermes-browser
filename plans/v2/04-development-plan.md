# 开发计划（v2）

> 修订记录：根据审计意见修正 P2#9(coder/websocket)、P2#14(icons)、任务拆分对齐 v2 架构变更

## 阶段一：项目骨架和 MCP Server 核心（预计 2-3 天）

### Task 1.1：初始化 Go 项目
- [ ] 创建 `go.mod`，module 名 `github.com/ety001/hermes-browser`
- [ ] 添加依赖：
  - `github.com/mark3labs/mcp-go` — MCP SDK
  - `github.com/coder/websocket` — WebSocket（替代已归档的 gorilla/websocket）
  - `gopkg.in/yaml.v3` — YAML 配置解析
- [ ] 创建目录结构（含 `extension/icons/`）
- [ ] 编写 `Makefile`（build, run, clean, lint）
- [ ] 编写 `.gitignore`
- [ ] 准备 Extension 占位图标（icon16/48/128.png，可用简单 SVG 导出）

### Task 1.2：配置模块
- [ ] 实现 `internal/config/config.go`
- [ ] 支持 YAML 配置文件加载
- [ ] 支持环境变量覆盖（`HB_` 前缀）
- [ ] 支持命令行参数 `-c` 指定配置文件
- [ ] 统一 token 机制：顶层 token + 各 transport 可选覆盖
- [ ] Token 自动生成和持久化（`~/.hermes-browser/.token`）
- [ ] stdio 模式检测：stdin 非 pipe 时打印警告
- [ ] 单元测试

### Task 1.3：认证模块
- [ ] 实现 `internal/auth/auth.go`
- [ ] Token 验证函数（支持统一 token 和独立 token）
- [ ] WebSocket Origin 白名单检查
- [ ] HTTP transport auth 中间件
- [ ] 单元测试

### Task 1.4：WebSocket 协议定义
- [ ] 定义 `internal/ws/protocol.go`
  - Request/Response 消息结构体
  - 所有 method 名称常量
  - 错误码常量和错误码列表
- [ ] 实现 JSON 序列化/反序列化
- [ ] 单元测试

### Task 1.5：WebSocket Hub（单 client 模型）
- [ ] 实现 `internal/ws/hub.go`
  - 单 client 管理（`client *Client`，非 map）
  - requestID -> response channel 映射
  - 超时管理
  - 心跳检测（ping/pong）
- [ ] 实现 `internal/ws/client.go`
  - WebSocket 读写循环（使用 github.com/coder/websocket）
  - Token 认证握手
- [ ] 单元测试

### Task 1.6：MCP Server 框架
- [ ] 实现 `internal/mcp/server.go`
  - 创建 MCPServer 实例
  - 注册所有工具（空实现先占位）
  - **HTTP transport 使用中间件包装**（不调用 httpServer.Start，自己建 mux）
  - WebSocket server 启动（在 MCP server 生命周期内）
- [ ] 实现 `cmd/server/main.go`
  - 配置加载
  - stdio 模式检测和警告
  - 信号处理（优雅关闭）
  - 启动 MCP Server（HTTP 或 stdio）+ WebSocket Server
- [ ] 验证：`go run ./cmd/server/` 能启动，Hermes 能发现工具

---

## 阶段二：Chrome Extension 开发（预计 2-3 天）

### Task 2.1：Extension 骨架
- [ ] 编写 `manifest.json`（Manifest V3）
  - permissions: `activeTab`, `scripting`, `tabs`, `storage`
  - host_permissions: `<all_urls>`
  - background service worker
  - popup action
  - 不声明 `content_scripts` 字段（按需注入）
- [ ] 实现 `background.js` 基础框架
  - WebSocket 连接管理
  - 自动重连逻辑（指数退避：1s, 2s, 4s, 8s, 最大 30s）
  - 心跳检测（每 30s ping）
- [ ] 实现 `popup/popup.html` + `popup.js`
  - 显示连接状态（绿/红/黄圆点）
  - Token 配置输入
  - WebSocket 地址配置
  - 操作日志显示（最近 50 条）

### Task 2.2：Background 命令分发器（v2 核心）
- [ ] 实现命令路由表：
  | 类别 | 命令 |
  |------|------|
  | Background 直接处理 | navigate, screenshot, list_tabs, switch_tab, new_tab, close_tab, get_cookies |
  | 转发给 Content Script | click, type, hover, scroll, select_option, get_content, execute_js, wait_for |
- [ ] 实现 ensureTabActive(tabId)：操作前切换到目标 tab
- [ ] 实现 Background 直接处理的命令 handler：
  - `navigate`：chrome.tabs.update + waitForTabComplete
  - `screenshot`：chrome.tabs.captureVisibleTab（+ 可选元素裁剪）
  - `list_tabs`：chrome.tabs.query
  - `switch_tab`：chrome.tabs.update({ active: true })
  - `new_tab`：chrome.tabs.create
  - `close_tab`：chrome.tabs.remove
  - `get_cookies`：chrome.cookies.getAll
- [ ] 实现 Content Script 按需注入（chrome.scripting.executeScript）
- [ ] 实现 Content Script 消息转发和响应收集

### Task 2.3：Content Script
- [ ] 实现 `content.js`
  - DOM 操作函数：click, type, hover, scroll, select_option
  - 页面内容提取（纯文本、HTML、类 Markdown）
  - execute_js：eval 执行并返回结果
  - wait_for：轮询等待元素出现
  - waitForPageLoad：domcontentloaded / load / networkidle
  - 智能文本清理（cleanText，正确的正则）
- [ ] 截图元素裁剪辅助（Background 发来截图后，Content Script 用 Canvas 裁剪）

### Task 2.4：Extension 设置持久化
- [ ] 使用 `chrome.storage.local` 存储 WebSocket 地址和 Token
- [ ] 首次安装引导流程

---

## 阶段三：工具实现和联调（预计 2-3 天）

### Task 3.1：Background 管理类工具
- [ ] `navigate` — MCP handler → WebSocket → Background
- [ ] `list_tabs` — MCP handler → WebSocket → Background
- [ ] `switch_tab` — MCP handler → WebSocket → Background
- [ ] `new_tab` — MCP handler → WebSocket → Background
- [ ] `close_tab` — MCP handler → WebSocket → Background
- [ ] `get_cookies` — MCP handler → WebSocket → Background
- [ ] 联调验证

### Task 3.2：页面交互工具
- [ ] `click` — MCP handler → WebSocket → Background → Content Script
- [ ] `type` — 同上
- [ ] `hover` — 同上
- [ ] `select_option` — 同上
- [ ] `scroll` — 同上
- [ ] 联调验证

### Task 3.3：信息获取工具
- [ ] `get_content` — MCP handler → WebSocket → Background → Content Script
- [ ] `screenshot` — MCP handler → WebSocket → Background
- [ ] `execute_js` — MCP handler → WebSocket → Background → Content Script
- [ ] `wait_for` — MCP handler → WebSocket → Background → Content Script
- [ ] 联调验证

### Task 3.4：端到端测试
- [ ] 完整流程测试：navigate → get_content → screenshot
- [ ] 多 tab 操作测试
- [ ] 错误场景测试（元素不存在、超时、Extension 未连接）
- [ ] 长时间运行稳定性测试
- [ ] Extension 重载后自动重连测试

---

## 阶段四：Hermes 集成和优化（预计 1-2 天）

### Task 4.1：Hermes 配置
- [ ] 在 `~/.hermes/config.yaml` 中添加 MCP server 配置
- [ ] 验证工具发现和调用
- [ ] 测试工具超时和重试

### Task 4.2：性能优化
- [ ] 大页面内容截断策略
- [ ] 截图压缩

### Task 4.3：日志和调试
- [ ] MCP Server 结构化日志
- [ ] Extension 调试日志
- [ ] 常见问题排查文档

### Task 4.4：文档
- [ ] README.md（安装、配置、使用）
- [ ] 开发文档

---

## 里程碑

| 里程碑 | 内容 | 预计时间 |
|--------|------|---------|
| M1 | MCP Server 启动，Hermes 能发现工具 | 第 1 周末 |
| M2 | Extension 能连接 MCP Server，执行 navigate | 第 2 周初 |
| M3 | 所有 15 个工具实现并可用 | 第 2 周末 |
| M4 | Hermes 端到端集成，完整流程跑通 | 第 3 周初 |

## v1 → v2 变更摘要

| 变更 | 来源 | 影响 |
|------|------|------|
| HTTP transport 认证改用标准 http.Handler 中间件 | P0#1 | 06-server-design 重写 |
| navigate/screenshot/get_cookies 归 Background 处理 | P0#2, P1#4 | 05-extension-design 重写 |
| Hub 简化为单 client 模型 | P0#3 | 06-server-design 简化 |
| WebSocket 库改用 github.com/coder/websocket | P2#9 | 04-development-plan 依赖更新 |
| 统一 token 设计 | P1#5 | 02-config-design 重写 |
| cleanText 正则修正 | P1#6 | 05-extension-design 代码修正 |
| networkidle 实现方案 | P1#7 | 02-config-design 新增说明 |
| activeTab ensureTabActive | P1#8 | 05-extension-design 新增 |
| 错误码体系 | P2#10 | 01-architecture + 03-mcp-tools |
| full_page 截图标记 TODO | P2#11 | 03-mcp-tools |
| 项目结构添加 icons/ | P2#14 | 01-architecture |
| stdio stdin 检测 | P2#15 | 02-config-design 新增 |

## 技术风险和缓解

| 风险 | 概率 | 缓解措施 |
|------|------|---------|
| Hermes 的 mcp_tool.py StreamableHTTP client 与 mcp-go server 不兼容 | 低 | 两者都遵循 MCP StreamableHTTP 规范，已确认 Hermes 用 streamable_http_client |
| Chrome Extension Content Script 注入被 CSP 阻止 | 低 | 使用 chrome.scripting.executeScript 而非 manifest 声明注入 |
| WebSocket 跨机器连接不稳定 | 中 | 实现自动重连 + 心跳检测 |
| 大页面截图 base64 过大 | 中 | 降低截图质量/分辨率，max_content_length 截断 |
| 部分网站 iframe 内操作受限 | 中 | 提供 execute_js 作为兜底方案 |
| activeTab 权限对后台 tab 限制 | 中 | ensureTabActive 自动切换到目标 tab |
