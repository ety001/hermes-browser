# Hermes 集成配置（v2）

> 修订记录：P0#1 确认 URL 路径与自定义 mux 一致

## Hermes config.yaml 配置

在 Hermes 的 `~/.hermes/config.yaml` 中添加 MCP server 配置：

```yaml
mcp_servers:
  hermes-browser:
    transport: http
    url: "http://192.168.199.54:19875/mcp"
    # Token 认证 - 通过 http_client 配置传递 Authorization header
    # 方式一：环境变量
    # HTTP_MCP_BROWSER_TOKEN=your-token-here
    # 方式二：直接配置（不推荐，明文存储）
```

## Token 配置说明

MCP Server 使用 Bearer token 认证。Hermes 的 MCP HTTP client 会通过
`http_client` 传递 Authorization header。

### 首次启动获取 Token

1. 启动 MCP Server（首次运行，token 为空时自动生成）：
   ```bash
   ./hermes-browser -c config.yaml
   ```
2. 查看 stderr 输出或 `~/.hermes-browser/.token` 文件获取生成的 token
3. 将 token 配置到 Hermes config.yaml 对应位置

### 手动指定 Token

在 MCP Server 的 `config.yaml` 中设置固定 token：
```yaml
token: "my-secret-token-here"
```

然后在 Hermes 端对应配置。

## URL 路径说明

MCP Server 的 HTTP endpoint 路径为 `/mcp`：
- mcp-go `StreamableHTTPServer` 默认 endpoint 是 `/mcp`
- 自定义 mux 中 `mux.Handle("/mcp", ...)` 保持一致
- Hermes 的 `mcp_tool.py` 使用 `streamable_http_client(url, ...)` 原样传递 URL
- **必须包含 `/mcp` 路径后缀**

完整 URL 示例：
```
http://192.168.199.54:19875/mcp
```

## WebSocket 配置（Chrome Extension 端）

Chrome Extension 的 Popup 面板中配置：
- WebSocket 地址：`ws://192.168.199.54:19876`
- Token：与 MCP Server 配置中的 token 一致

如果 MCP Server 和 Chrome 在同一台机器上，可以用：
- WebSocket 地址：`ws://127.0.0.1:19876`

## 网络拓扑

```
Hermes Agent (tai-worker: 192.168.44.2)
    |
    | HTTP POST http://192.168.199.54:19875/mcp
    | Authorization: Bearer <token>
    |
MCP Server (192.168.199.54:19875 + 19876)
    |
    | WebSocket ws://192.168.199.54:19876?token=<token>
    |
Chrome Extension (Background Service Worker)
    |
Content Script (按需注入目标页面)
```

## 防火墙配置

确保以下端口可达：
- 19875/TCP — MCP HTTP transport（Hermes → MCP Server）
- 19876/TCP — WebSocket（Chrome Extension → MCP Server）

如果使用默认配置（WebSocket 仅监听 127.0.0.1），
Chrome 和 MCP Server 必须在同一台机器上。

如果需要跨机器 WebSocket 连接（不推荐，增加暴露面）：
```yaml
websocket:
  bind: "0.0.0.0:19876"
```

## 验证步骤

1. 启动 MCP Server：
   ```bash
   ./hermes-browser -c config.yaml
   ```

2. 加载 Chrome Extension：
   - 打开 `chrome://extensions/`
   - 开启 "开发者模式"
   - 点击 "加载已解压的扩展程序"
   - 选择 `extension/` 目录

3. 在 Extension Popup 中配置 WebSocket 地址和 Token，点击连接

4. 验证 MCP 连接：
   ```bash
   # 从 Hermes 机器测试 HTTP 连通性
   curl -X POST http://192.168.199.54:19875/mcp \
     -H "Authorization: Bearer <token>" \
     -H "Content-Type: application/json" \
     -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{...}}'
   ```

5. 在 Hermes 中验证工具发现：
   - 工具名以 `mcp_browser_` 前缀
   - 应该看到 15 个工具

## stdio 模式（备选）

如果 Hermes 和 MCP Server 在同一台机器上，可以使用 stdio transport：

```yaml
mcp_servers:
  hermes-browser:
    transport: stdio
    command: "/path/to/hermes-browser"
    args: ["-c", "/path/to/config.yaml"]
    # server.http.bind 留空，程序自动使用 stdio 模式
    # WebSocket 仍然需要配置 bind，供 Chrome Extension 连接
```

stdio 模式下：
- MCP 通信通过 stdin/stdout
- WebSocket 仍然独立运行
- 程序会检测 stdin 是否为 pipe，非 pipe 时打印警告
