# 开发计划

## 阶段一：项目骨架和 MCP Server 核心（预计 2-3 天）

### Task 1.1：初始化 Go 项目
- [ ] 创建 `go.mod`，module 名 `github.com/ety001/hermes-browser`
- [ ] 添加依赖：`github.com/mark3labs/mcp-go`、`github.com/gorilla/websocket`、`github.com/spf13/viper`
- [ ] 创建目录结构
- [ ] 编写 `Makefile`（build, run, clean, lint）
- [ ] 编写 `.gitignore`

### Task 1.2：配置模块
- [ ] 实现 `internal/config/config.go`
- [ ] 支持 YAML 配置文件加载
- [ ] 支持环境变量覆盖（`HB_` 前缀）
- [ ] 支持命令行参数 `-c` 指定配置文件
- [ ] Token 自动生成和持久化逻辑
- [ ] 单元测试

### Task 1.3：认证模块
- [ ] 实现 `internal/auth/auth.go`
- [ ] Token 验证函数
- [ ] WebSocket Origin 白名单检查
- [ ] 单元测试

### Task 1.4：WebSocket 协议定义
- [ ] 定义 `internal/ws/protocol.go`
  - Request/Response 消息结构体
  - 所有 method 名称常量
  - 错误码定义
- [ ] 实现 JSON 序列化/反序列化
- [ ] 单元测试

### Task 1.5：WebSocket Hub
- [ ] 实现 `internal/ws/hub.go`
  - 客户端注册/注销
  - 消息路由（按 tab_id 或默认 tab）
  - 超时管理
  - 心跳检测（ping/pong）
- [ ] 实现 `internal/ws/client.go`
  - WebSocket 读写循环
  - Token 认证握手
  - 断线重连支持
- [ ] 单元测试

### Task 1.6：MCP Server 框架
- [ ] 实现 `internal/mcp/server.go`
  - 创建 MCPServer 实例
  - 注册所有工具（空实现先占位）
  - HTTP transport 配置
  - WebSocket server 启动（在 MCP server 生命周期内）
- [ ] 实现 `cmd/server/main.go`
  - 配置加载
  - 信号处理（优雅关闭）
  - 启动 MCP Server + WebSocket Server
- [ ] 验证：`go run ./cmd/server/` 能启动，Hermes 能发现工具

---

## 阶段二：Chrome Extension 开发（预计 2-3 天）

### Task 2.1：Extension 骨架
- [ ] 编写 `manifest.json`（Manifest V3）
  - permissions: `activeTab`, `scripting`, `tabs`, `storage`
  - host_permissions: `<all_urls>`
  - background service worker
  - popup action
- [ ] 实现 `background.js` 基础框架
  - WebSocket 连接管理
  - 自动重连逻辑
  - 消息收发
- [ ] 实现 `popup/popup.html` + `popup.js`
  - 显示连接状态
  - Token 配置输入
  - WebSocket 地址配置
  - 操作日志显示

### Task 2.2：Content Script
- [ ] 实现 `content.js`
  - DOM 操作函数集合
  - 页面内容提取（纯文本、简化 HTML、类 Markdown）
  - 截图功能
  - 等待元素出现
  - 键盘事件模拟
  - 鼠标事件模拟
  - 滚动操作
  - 下拉框选择

### Task 2.3：Background <-> Content Script 通信
- [ ] 消息路由框架
- [ ] 按需注入 Content Script（不全局注入）
- [ ] 跨 tab 操作支持

### Task 2.4：Extension 设置持久化
- [ ] 使用 `chrome.storage.local` 存储 WebSocket 地址和 Token
- [ ] 首次安装引导流程

---

## 阶段三：工具实现和联调（预计 2-3 天）

### Task 3.1：基础导航工具
- [ ] `navigate` - MCP handler -> WebSocket -> Extension -> content script
- [ ] `list_tabs` - 通过 chrome.tabs API
- [ ] `switch_tab` - 通过 chrome.tabs API
- [ ] `new_tab` - 通过 chrome.tabs API
- [ ] `close_tab` - 通过 chrome.tabs API
- [ ] 联调验证

### Task 3.2：页面交互工具
- [ ] `click` - 查找元素 + 模拟点击
- [ ] `type` - 聚焦输入框 + 输入文本
- [ ] `hover` - 模拟鼠标悬停
- [ ] `select_option` - 选择下拉选项
- [ ] `scroll` - 滚动页面
- [ ] 联调验证

### Task 3.3：信息获取工具
- [ ] `get_content` - 提取页面文本
- [ ] `screenshot` - 截图并返回 base64
- [ ] `get_cookies` - 读取 cookies
- [ ] `execute_js` - 执行 JS 代码
- [ ] `wait_for` - 等待元素
- [ ] 联调验证

### Task 3.4：端到端测试
- [ ] 完整流程测试：导航 -> 读取内容 -> 截图
- [ ] 多 tab 操作测试
- [ ] 错误场景测试（元素不存在、超时等）
- [ ] 长时间运行稳定性测试

---

## 阶段四：Hermes 集成和优化（预计 1-2 天）

### Task 4.1：Hermes 配置
- [ ] 在 `~/.hermes/config.yaml` 中添加 MCP server 配置
- [ ] 验证工具发现和调用
- [ ] 测试工具超时和重试

### Task 4.2：性能优化
- [ ] 大页面内容截断策略
- [ ] 截图压缩
- [ ] WebSocket 消息批处理（如果需要）

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

## 技术风险和缓解

| 风险 | 概率 | 缓解措施 |
|------|------|---------|
| mcp-go 的 HTTP transport 在 Hermes 侧不兼容 | 低 | mcp-go 是官方推荐的 Go SDK，Hermes 支持标准 MCP HTTP transport |
| Chrome Extension Content Script 注入被 CSP 阻止 | 低 | 使用 chrome.scripting.executeScript 而非 manifest 声明注入 |
| WebSocket 跨机器连接不稳定 | 中 | 实现自动重连 + 心跳检测 |
| 大页面截图 base64 过大，超出 MCP 响应限制 | 中 | 降低截图质量/分辨率，实现截断策略 |
| 部分网站 iframe 内操作受限 | 中 | 提供 execute_js 作为兜底方案 |
