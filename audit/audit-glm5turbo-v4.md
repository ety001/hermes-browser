# Hermes Browser v4 计划审计报告

审计模型：glm-5-turbo
审计版本：v4
审计日期：2026-05-12
审计方法：7 份 v4 计划文件交叉检查 + mcp-go/coder/websocket 源码验证

---

## 审计总结

v3 审计发现的所有 P0 问题已修复。本轮主要发现代码模板中的 Go import 缺失和配置值不一致。

---

## P0 - 编译阻塞

### 1. hub.go 缺少 fmt import
06 第208行 hub.go import 只有 `sync, time, config, uuid`，但第280行使用 `fmt.Errorf`。

### 2. client.go 缺少 encoding/json import
06 第338行 client.go import 只有 `context, net/http, time, ws`，但使用了 `json.Unmarshal` 和 `json.Marshal`。

### 3. protocol.go 缺少 fmt import
06 第505行 `fmt.Errorf` 但无 import。

### 4. max_content_length 默认值 10 倍差异
02 定义 `max_content_length: 500000`（50万），06 Config 结构体默认 `MaxContentLength: 50000`（5万）。

## P1 - 功能缺陷

### 5. HTTPConfig/WSConfig 缺少 Token 字段
02 定义了 `server.http.token` 和 `websocket.token`，06 GetHTTPToken() 注释也说优先读 transport 级 token，但结构体没有 Token 字段。

### 6. startStdioServer() 未定义
06 main.go 第60行调用但整个文档无定义。

### 7. 15 个 Tool 变量声明缺失
06 registerTools 引用 navigateTool 等变量但未声明（03 中的 NewTool 定义就是这些变量）。

### 8. client.go 包含未使用 import
`net/http` 和 `time` 未使用。

## P2 - 文档细节

### 9. 07 curl 引号仍未闭合
v4 修订记录说已修正但实际未修。

### 10. ScreenshotQuality 未被代码引用
Config 定义了但 handler 没读取。

### 11. waitForTabComplete 超时 resolve 而非 reject
无法区分正常完成和超时。
