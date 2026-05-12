# 配置文件设计（v2）

> 修订记录：根据审计意见修正 P1#5(统一 token)、P2#15(stdin 检测)

## 配置文件路径

优先级（高到低）：
1. 命令行参数 `-c /path/to/config.yaml`
2. 环境变量 `HERMES_BROWSER_CONFIG`
3. 当前目录 `./config.yaml`
4. `~/.hermes-browser/config.yaml`
5. 可执行文件同目录 `config.yaml`

## 配置文件格式

```yaml
# 顶层 token，WebSocket 和 HTTP transport 共用
# 留空则自动生成（首次启动打印到 stderr，持久化到 ~/.hermes-browser/.token）
token: ""

# MCP Server 配置
server:
  # MCP HTTP transport 监听地址（供 Hermes 连接）
  # 如果为空则只使用 stdio transport
  http:
    bind: "0.0.0.0:19875"
    # 留空则使用顶层 token
    # token: ""

# WebSocket 服务配置（供 Chrome Extension 连接）
websocket:
  # 监听地址
  # "127.0.0.1:19876" - 仅本机（默认）
  # "0.0.0.0:19876"   - 所有网卡
  # "192.168.199.54:19876" - 指定网卡
  bind: "127.0.0.1:19876"
  # 留空则使用顶层 token
  # token: ""
  # 允许的 Chrome Extension ID 列表（Origin 白名单）
  # 留空则不检查 Origin（仅适用于开发环境）
  allowed_extensions: []

# 浏览器操作默认配置
browser:
  # 页面导航默认等待策略
  # "domcontentloaded" - DOM 加载完成即返回
  # "load"             - 所有资源加载完成
  # "networkidle"      - load 事件 + 额外等待 500ms（推荐，适合 SPA）
  default_wait_until: "networkidle"
  # 默认操作超时（毫秒）
  default_timeout: 30000
  # 截图默认格式（jpeg 或 png）
  screenshot_format: "jpeg"
  # 截图默认质量（0-100，仅 jpeg 有效）
  screenshot_quality: 80
  # 页面内容最大返回长度（字符），超过则截断
  max_content_length: 500000

# 日志配置
logging:
  # 日志级别: debug, info, warn, error
  level: "info"
  # 日志文件路径，留空则只输出到 stderr
  file: ""
```

## Token 设计

### 统一 Token（v2 新增）

WebSocket 和 HTTP transport 默认共用顶层 `token` 字段。

Token 解析优先级：
- HTTP transport：`server.http.token` > `token`
- WebSocket transport：`websocket.token` > `token`

### Token 生成策略

如果 token（含各 transport 的独立 token）配置为空：
1. 首次启动时自动生成 32 字节随机 token（hex 编码 = 64 字符）
2. 保存到 `~/.hermes-browser/.token` 文件
3. 打印到 stderr（仅在首次生成时）
4. 后续启动读取该文件

如果 token 配置了具体值：直接使用该值。

## networkidle 实现说明

Chrome Extension 没有 Playwright 的 `page.waitForNetworkIdle()` API。
采用简化实现方案：

```javascript
// content.js
function waitForPageLoad(waitUntil) {
    return new Promise((resolve) => {
        if (waitUntil === 'domcontentloaded') {
            if (document.readyState !== 'loading') return resolve();
            document.addEventListener('DOMContentLoaded', resolve, { once: true });
        } else if (waitUntil === 'load') {
            if (document.readyState === 'complete') return resolve();
            window.addEventListener('load', resolve, { once: true });
        } else if (waitUntil === 'networkidle') {
            // load 事件 + 额外等待 500ms
            const onLoad = () => setTimeout(resolve, 500);
            if (document.readyState === 'complete') {
                onLoad();
            } else {
                window.addEventListener('load', onLoad, { once: true });
            }
        }
    });
}
```

后续可升级为 PerformanceObserver 方案（监听 resource entry types，
在 500ms 窗口内无新网络请求时视为 idle）。

## 环境变量覆盖

所有配置项均可通过环境变量覆盖，使用 `HB_` 前缀，层级用 `_` 分隔：

| 环境变量 | 对应配置 |
|---------|---------|
| `HB_TOKEN` | token |
| `HB_SERVER_HTTP_BIND` | server.http.bind |
| `HB_SERVER_HTTP_TOKEN` | server.http.token |
| `HB_WEBSOCKET_BIND` | websocket.bind |
| `HB_WEBSOCKET_TOKEN` | websocket.token |
| `HB_BROWSER_DEFAULT_WAIT_UNTIL` | browser.default_wait_until |
| `HB_BROWSER_DEFAULT_TIMEOUT` | browser.default_timeout |
| `HB_LOGGING_LEVEL` | logging.level |

## stdio 模式检测（v2 新增）

当 `server.http.bind` 为空时，程序使用 stdio transport。
启动前检测 stdin 是否为 pipe，避免用户直接在终端运行时静默挂起：

```go
func isStdinPipe() bool {
    fi, _ := os.Stdin.Stat()
    return fi.Mode()&os.ModeCharDevice == 0
}
```

如果 stdin 是 terminal 且没有配置 HTTP bind，打印警告到 stderr：
```
WARNING: No HTTP bind configured and stdin is not a pipe.
The server will wait for MCP stdio input. If you meant to start
as a standalone server, configure server.http.bind in config.yaml.
```
