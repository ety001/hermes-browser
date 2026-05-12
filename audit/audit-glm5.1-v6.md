# Hermes Browser v6 计划审计报告

审计模型：glm-5.1
审计版本：v6
审计日期：2026-05-12
审计方法：7 份 v6 计划文件全文交叉检查 + mcp-go/coder/websocket 源码验证

---

## 审计总结

v5 审计发现的所有问题已全部修复。v6 审计仅发现 1 个 P0（main.go 缺 import）、1 个 P1（注释不一致）、1 个 P2（JS fallback 值），
审计过程中已直接修复。**计划已可直接用于编码实现。**

---

## 发现的问题（已修复）

### P0#1: main.go 缺少 auth 和 server import
main.go 中 startHTTPServer 使用 `server.NewStreamableHTTPServer` 和 `auth.HTTPMiddleware`，
但 import 中缺少 `github.com/ety001/hermes-browser/internal/auth` 和 `github.com/mark3labs/mcp-go/server`。
已修复。

### P1#1: MaxContentLength 注释写 50000
BrowserConfig 结构体注释 `// chars, default 50000`，实际默认值 500000。已修正注释。

### P2#1: handleNavigate JS fallback timeout 60000 vs MCP 默认 30000
JS 代码 fallback 为 60000ms，而 03-mcp-tools.md 中 navigate timeout 默认 30000ms。
运行时无影响（MCP handler 会传递 30000），但文档不一致。已修正为 30000。

---

## 验证通过的检查项

1. 版本号一致性：7 个文件均标记为 v6 ✓
2. 工具名一致性：03/05/06 三文件 15 个工具名完全对应 ✓
3. 参数名一致性：expression, return_value, selector, url 等全部一致 ✓
4. Config 字段完整性：Token/HTTP/WS/Browser/Logging 与 02 配置文档一一对应 ✓
5. Go import 完整性：所有 Go 代码块 import 覆盖完整 ✓
6. 函数定义完整性：main.go 调用的函数均有定义 ✓
7. API 签名正确性：AddTool, GetArguments, NewToolResultError, NewToolResultImage, Read, Write, Accept 全部匹配 ✓
8. JavaScript 正确性：异步 sendResponse + return true, Chrome API 用法正确 ✓
9. max_content_length 一致性：02 yaml 500000 = 06 默认值 500000 ✓
10. curl 示例引号闭合 ✓

---

## 结论

**v6 计划审计通过，可进入编码实现阶段。**

无剩余 P0/P1 问题。所有核心架构决策（Go MCP Server + WebSocket + Chrome Extension Manifest V3）
和 API 调用方式均已通过源码验证确认正确。
