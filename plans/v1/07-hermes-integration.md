# Hermes 集成配置

## config.yaml 配置

由于 MCP Server 运行在有浏览器的机器上（非 tai-worker），使用 HTTP transport：

```yaml
mcp_servers:
  browser:
    url: "http://192.168.199.54:19875/mcp"
    headers:
      Authorization: "Bearer <token>"
    timeout: 60
    connect_timeout: 30
```

如果 Hermes 和浏览器在同一台机器，也可以用 stdio transport：

```yaml
mcp_servers:
  browser:
    command: "/path/to/hermes-browser"
    args: ["-c", "/path/to/config.yaml"]
    timeout: 60
    connect_timeout: 30
```

## 工具使用示例

Agent 调用工具时，工具名称带 `mcp_browser_` 前缀：

### 示例 1：访问网页并获取内容

```
Agent 调用:
  mcp_browser_navigate(url="https://news.ycombinator.com")
  → 返回: "Navigated to https://news.ycombinator.com: Hacker News"

Agent 调用:
  mcp_browser_get_content(type="text")
  → 返回: 页面文本内容

Agent 调用:
  mcp_browser_screenshot(format="jpeg", quality=80)
  → 返回: base64 图片
```

### 示例 2：登录网站

```
Agent 调用:
  mcp_browser_navigate(url="https://example.com/login")

Agent 调用:
  mcp_browser_type(selector="#username", text="user@example.com")
  mcp_browser_type(selector="#password", text="password123")

Agent 调用:
  mcp_browser_click(selector="button[type=submit]")

Agent 调用:
  mcp_browser_wait_for(selector=".dashboard", state="visible")
```

### 示例 3：多 tab 操作

```
Agent 调用:
  mcp_browser_list_tabs()
  → 返回: [{id: 1, url: "...", title: "..."}, ...]

Agent 调用:
  mcp_browser_new_tab(url="https://example.com")

Agent 调用:
  mcp_browser_switch_tab(tab_id=1)
```

### 示例 4：执行 JavaScript

```
Agent 调用:
  mcp_browser_execute_js(code="document.querySelectorAll('.price').map(e => e.textContent)")
  → 返回: ["$29.99", "$49.99", "$19.99"]

Agent 调用:
  mcp_browser_execute_js(code="JSON.stringify(performance.getEntriesByType('navigation')[0])")
  → 返回: 导航性能数据
```

## 工具超时配置

| 工具类型 | 默认超时 | 说明 |
|---------|---------|------|
| navigate | 30s | 页面加载可能较慢 |
| click | 10s | 等待元素出现并点击 |
| type | 10s | 输入文本 |
| get_content | 15s | 大页面提取可能较慢 |
| screenshot | 15s | 截图+编码 |
| execute_js | 10s | JS 执行 |
| wait_for | 30s | 等待元素出现 |
| list_tabs | 5s | 列出标签页 |
| scroll | 5s | 滚动页面 |
