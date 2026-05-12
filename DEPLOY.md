# 部署指南

## 网络拓扑

```
Hermes Agent (192.168.44.2)
    │
    │ HTTP POST http://192.168.199.11:19875/mcp
    │ Authorization: Bearer <token>
    │
MCP Server (192.168.199.11:19875)
    │
    │ WebSocket ws://127.0.0.1:19876?token=<token>
    │
Chrome Extension (192.168.199.11 本机)
```

两个端口：
| 端口 | 监听 | 用途 |
|------|------|------|
| **19875/TCP** | `0.0.0.0` → 远程可访问 | Hermes → MCP (StreamableHTTP) |
| **19876/TCP** | `127.0.0.1` → 仅本机 | MCP Server → Chrome Extension (WebSocket) |

---

## 当前部署状态

### ✅ 已完成

| 步骤 | 状态 |
|------|------|
| Go 二进制编译 (`hermes-browser`, 12MB) | ✅ |
| 部署配置 `configs/deploy.yaml` (固定 token) | ✅ |
| SCP 到 `192.168.199.11:/home/ety001/hermes-browser/` | ✅ |
| 扩展文件 (manifest.json, background.js, content.js, popup, icons) | ✅ |
| MCP Server 已在远端启动 | ✅ |
| Hermes 本地配置已添加 MCP Server | ✅ |

### ❌ 需要你手动操作

**只有以下 2 步需要人工介入：**

#### 1. 加载 Chrome 扩展

打开 Chrome → `chrome://extensions/` → 开启"开发者模式" → "加载已解压的扩展程序" → 选择 `/home/ety001/hermes-browser/extension/` 目录

验证方式：扩展图标出现在 Chrome 工具栏。

#### 2. 在扩展 Popup 中输入 Token 并连接

1. 点击 Chrome 工具栏的 Hermes Browser 图标
2. WebSocket 地址：`ws://127.0.0.1:19876`（默认已填好）
3. Token：`hermes-browser-deploy-token-2025`
4. 点击 **Connect**
5. 状态灯变绿 → 连接成功

---

## 验证方法

### A. Hermes 自动发现工具

MCP Server 运行时，Hermes 会自动调用 `tools/list` 发现所有 15 个工具：
- `mcp_browser_navigate`
- `mcp_browser_screenshot`
- `mcp_browser_click`
- `mcp_browser_type`
- ...（共 15 个）

在 Hermes 对话中输入 `/tools` 即可看到。

### B. 手动验证

```bash
# 1. 健康检查（不需要 token）
curl http://192.168.199.11:19875/health

# 2. MCP Initialize
curl -s -X POST http://192.168.199.11:19875/mcp \
  -H "Authorization: Bearer hermes-browser-deploy-token-2025" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'

# 3. ListTools（需要从 Initialize 响应获取 Mcp-Session-Id）
SESSION_ID=$(curl -s -i -X POST http://192.168.199.11:19875/mcp \
  -H "Authorization: Bearer hermes-browser-deploy-token-2025" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  | grep -i "Mcp-Session-Id" | awk '{print $2}' | tr -d '\r')

curl -s -X POST http://192.168.199.11:19875/mcp \
  -H "Authorization: Bearer hermes-browser-deploy-token-2025" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'
```

### C. Hermes 端到端测试

启动 Hermes 后说：
> "使用 hermes-browser 的 mcp 工具，导航到 https://example.com，获取页面内容，然后截图"

---

## 重启与日志

```bash
# 在 192.168.199.11 上重启 MCP Server
kill $(pgrep hermes-browser)
nohup /home/ety001/hermes-browser/hermes-browser \
  -c /home/ety001/hermes-browser/configs/deploy.yaml \
  > /tmp/hermes-browser.log 2>&1 &

# 查看日志
tail -f /tmp/hermes-browser.log
```

## 更新部署

```bash
# 在开发机编译后重新发送
cd ~/workspace/hermes-browser
go build -o hermes-browser ./cmd/server/
scp hermes-browser 192.168.199.11:/home/ety001/hermes-browser/
scp extension/*.js 192.168.199.11:/home/ety001/hermes-browser/extension/
# 在远端重启
ssh 192.168.199.11 "kill \$(pgrep hermes-browser) && \
  nohup /home/ety001/hermes-browser/hermes-browser \
  -c /home/ety001/hermes-browser/configs/deploy.yaml \
  > /tmp/hermes-browser.log 2>&1 &"
```
