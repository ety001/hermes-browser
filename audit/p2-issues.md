# P2 - 遗漏和优化建议

### 9. gorilla/websocket 已停止维护

**文件：** 04-development-plan.md（Task 1.1 依赖列表）

计划使用 `github.com/gorilla/websocket`，但该库已于 2022 年归档（archived），不再维护。

**建议替代：**
- `github.com/coder/websocket`（原 nhooyr.io/websocket）：纯 Go 实现，支持 context，API 更现代
- `github.com/gobwas/ws`：零拷贝，性能最好，但 API 较底层

自用场景哪个都行，建议用 `github.com/coder/websocket`，API 更友好。

---

### 10. 缺少错误码体系

**文件：** 01-architecture.md（通信协议）、06-server-design.md（protocol.go）

当前错误响应只有 `status: "error"` + `error` 字符串。LLM 调用时靠字符串匹配
判断错误类型不靠谱，也不利于自动重试。

**建议添加错误码：**
```json
{
  "id": "uuid-v4",
  "status": "error",
  "code": "ELEMENT_NOT_FOUND",
  "error": "Element matching selector '#login-btn' not found"
}
```

预定义错误码：
| 错误码 | 说明 | 建议 LLM 行为 |
|--------|------|--------------|
| `ELEMENT_NOT_FOUND` | CSS 选择器未匹配到元素 | 检查选择器，等待后重试 |
| `TIMEOUT` | 操作超时 | 增大 timeout 参数后重试 |
| `NAVIGATION_ERROR` | 页面导航失败 | 检查 URL，可能需要登录 |
| `JS_EXECUTION_ERROR` | JavaScript 执行异常 | 修正代码后重试 |
| `TAB_NOT_FOUND` | Tab ID 不存在 | 重新 list_tabs 获取正确 ID |
| `TAB_CLOSED` | Tab 已被关闭 | 使用其他 tab |
| `NO_EXTENSION_CONNECTED` | Chrome Extension 未连接 | 提示用户检查 Extension |
| `PERMISSION_DENIED` | 权限不足 | 提示用户检查 tab 是否激活 |

---

### 11. full_page 截图方案空缺

**文件：** 05-extension-design.md（截图方案）

末尾提到 "Content Script 逐屏截图并拼接" 但没有详细方案。

**实际困难：**
- `captureVisibleTab` 只能截可见区域
- 需要多次 `window.scrollTo` + 截图 + Canvas 拼接
- 固定定位元素（导航栏、浮动按钮）会在每帧重复出现
- 懒加载内容需要滚动触发加载
- 完整实现约 100+ 行 JS 代码

**建议：** 首版不做 full_page，screenshot 工具的 `full_page` 参数标记为 TODO。
只支持可见区域截图和指定元素截图。后续作为增强功能添加。

---

### 12. Extension 重载场景未考虑

**文件：** 06-server-design.md（Hub 设计）

开发阶段会频繁修改 Extension 代码并点击 Chrome 的 "重新加载" 按钮。
每次重载会：
1. 旧 Background Service Worker 销毁（WebSocket 断开）
2. 新 Background Service Worker 启动（WebSocket 重连）

**当前设计的处理：**
- Hub 有 unregisterCh 处理断开，有 registerCh 处理重连
- 理论上能工作

**但需要注意：**
- 注入到页面中的旧 Content Script 实例不会自动销毁，
  但其 `chrome.runtime.onMessage` listener 会失效（Extension context invalidated）
- 新 Background 需要重新注入 Content Script
- injectedTabs 集合（Background 内存中）在重载后清空，所以重新注入没问题
- 但页面中残留的旧 content script 可能导致 `Receiving end does not exist` 错误

**建议：** 在计划中记录这个已知行为，不影响功能但调试时可能看到 console 警告。

---

### 13. content_scripts: [] 多余

**文件：** 05-extension-design.md（Manifest V3 配置）

manifest.json 中写了 `"content_scripts": []`。

**问题：** 如果不需要 content scripts，直接不写这个字段。空数组虽然 Chrome 不报错，
但语义上多余。

**修复：** 删除 `"content_scripts": []` 这一行。

---

### 14. 缺少 Chrome Extension 的图标文件

**文件：** 05-extension-design.md（Manifest V3 配置）、01-architecture.md（项目结构）

manifest.json 引用了 `icons/icon16.png`、`icons/icon48.png`、`icons/icon128.png`，
但项目结构中没有 `extension/icons/` 目录。

**影响：** Extension 加载时 Chrome 会报 warning，popup 和 toolbar 上显示默认图标。

**建议：** 在项目结构中添加 `extension/icons/`，可以用简单的 SVG 导出或占位图标。
首次加载时可以用 Chrome 默认图标，后续再补。

---

### 15. stdio 模式下空启动的用户体验

**文件：** 06-server-design.md（main.go）

当 `server.http.bind` 为空时，程序使用 stdio transport，会阻塞在 stdin 读取上。
如果用户直接在终端运行 `./hermes-browser`（不在 Hermes 环境下），
程序会静默挂起，没有提示。

**建议：** 检测 stdin 是否是 pipe/terminal：
```go
func isStdinPipe() bool {
    fi, _ := os.Stdin.Stat()
    return fi.Mode()&os.ModeCharDevice == 0
}
```

如果 stdin 是 terminal 且没有配置 HTTP bind，打印提示：
```
WARNING: No HTTP bind configured and stdin is not a pipe.
The server will wait for MCP stdio input. If you meant to start
as a standalone server, configure server.http.bind in config.yaml.
```

---

### 16. Hermes HTTP transport URL 路径确认

**文件：** 07-hermes-integration.md

Hermes 配置示例：
```yaml
url: "http://192.168.199.54:19875/mcp"
```

**确认：** mcp-go 的 `StreamableHTTPServer` 默认 endpoint path 是 `/mcp`
（源码 streamable_http.go line 278: `endpointPath: "/mcp"`）。
Hermes 的 mcp_tool.py 使用 `streamable_http_client(url, ...)` 将 URL 原样传递给 SDK。
路径匹配正确，无需修改。

但需注意：如果 MCP Server 使用自定义中间件包装（P0 #1 的修复方案），
自己建 mux 时 Handle 的 path 也必须是 `/mcp`。

---

### 17. 命令分发架构需要明确

**文件：** 05-extension-design.md、06-server-design.md

当前计划中 Background Service Worker 的角色描述不够清晰。
根据 P0 #2 和 P1 #4 的修复，Background 需要区分两类命令：

| 类别 | 命令 | 处理位置 |
|------|------|---------|
| Tab 管理 | list_tabs, switch_tab, new_tab, close_tab | Background（chrome.tabs API） |
| 页面导航 | navigate | Background（chrome.tabs.update） |
| 截图 | screenshot | Background（chrome.tabs.captureVisibleTab） |
| Cookie | get_cookies | Background（chrome.cookies API） |
| 页面交互 | click, type, hover, scroll, select_option | Content Script |
| 内容获取 | get_content, execute_js, wait_for | Content Script |

**建议：** 在 Extension 设计文档中增加一个命令路由表，
明确每个命令由 Background 直接处理还是转发给 Content Script。
这会影响 Hub 的设计和 MCP Server handler 的实现。
