# MCP 工具定义

## 工具列表

所有工具注册到 MCP Server，前缀为 `mcp_browser_`（由 Hermes 自动添加）。

### 1. navigate - 页面导航

导航到指定 URL，等待页面加载完成。

```go
mcp.NewTool("navigate",
    mcp.WithDescription("Navigate to a URL and wait for the page to load"),
    mcp.WithString("url",
        mcp.Description("The URL to navigate to"),
        mcp.Required(),
    ),
    mcp.WithString("wait_until",
        mcp.Description("Wait condition: domcontentloaded, load, networkidle"),
        mcp.DefaultString("networkidle"),
    ),
    mcp.WithNumber("timeout",
        mcp.Description("Timeout in milliseconds"),
        mcp.DefaultNumber(30000),
    ),
)
```

**响应：** `{ "url": "...", "title": "...", "status": 200 }`

### 2. screenshot - 页面截图

截取当前页面或指定元素。

```go
mcp.NewTool("screenshot",
    mcp.WithDescription("Take a screenshot of the current page or a specific element"),
    mcp.WithString("selector",
        mcp.Description("CSS selector of element to capture (optional, captures full page if omitted)"),
    ),
    mcp.WithString("format",
        mcp.Description("Image format: jpeg or png"),
        mcp.DefaultString("jpeg"),
    ),
    mcp.WithNumber("quality",
        mcp.Description("Image quality 0-100 (jpeg only)"),
        mcp.DefaultNumber(80),
    ),
    mcp.WithBoolean("full_page",
        mcp.Description("Capture full scrollable page"),
        mcp.DefaultBoolean(false),
    ),
)
```

**响应：** base64 编码的图片数据（通过 MCP ImageContent 返回）

### 3. get_content - 获取页面文本内容

提取页面的可读文本内容。

```go
mcp.NewTool("get_content",
    mcp.WithDescription("Get text content of the page or a specific element"),
    mcp.WithString("selector",
        mcp.Description("CSS selector to extract content from (optional, extracts full page if omitted)"),
    ),
    mcp.WithString("type",
        mcp.Description("Content type: text (default), markdown, or html"),
        mcp.DefaultString("text"),
    ),
)
```

**响应：** 页面文本内容

### 4. click - 点击元素

点击指定 CSS 选择器匹配的元素。

```go
mcp.NewTool("click",
    mcp.WithDescription("Click on an element matching the CSS selector"),
    mcp.WithString("selector",
        mcp.Description("CSS selector of the element to click"),
        mcp.Required(),
    ),
    mcp.WithNumber("timeout",
        mcp.Description("Timeout in milliseconds to wait for element"),
        mcp.DefaultNumber(10000),
    ),
)
```

**响应：** `{ "clicked": true, "tag": "button", "text": "Submit" }`

### 5. type - 输入文本

向指定输入框输入文本。

```go
mcp.NewTool("type",
    mcp.WithDescription("Type text into an input field"),
    mcp.WithString("selector",
        mcp.Description("CSS selector of the input field"),
        mcp.Required(),
    ),
    mcp.WithString("text",
        mcp.Description("Text to type"),
        mcp.Required(),
    ),
    mcp.WithBoolean("clear_first",
        mcp.Description("Clear existing content before typing"),
        mcp.DefaultBoolean(true),
    ),
    mcp.WithBoolean("press_enter",
        mcp.Description("Press Enter after typing"),
        mcp.DefaultBoolean(false),
    ),
)
```

**响应：** `{ "typed": true, "length": 42 }`

### 6. scroll - 滚动页面

```go
mcp.NewTool("scroll",
    mcp.WithDescription("Scroll the page up or down"),
    mcp.WithString("direction",
        mcp.Description("Scroll direction: up or down"),
        mcp.DefaultString("down"),
    ),
    mcp.WithString("amount",
        mcp.Description("Scroll amount: one_page, half_page, or a CSS value like 500px"),
        mcp.DefaultString("one_page"),
    ),
)
```

**响应：** `{ "scrolled": true, "scroll_y": 1500, "scroll_height": 5000 }`

### 7. execute_js - 执行 JavaScript

在页面上下文中执行 JavaScript 代码。

```go
mcp.NewTool("execute_js",
    mcp.WithDescription("Execute JavaScript code in the page context"),
    mcp.WithString("code",
        mcp.Description("JavaScript code to execute"),
        mcp.Required(),
    ),
    mcp.WithBoolean("return_value",
        mcp.Description("Whether to return the result of the last expression"),
        mcp.DefaultBoolean(true),
    ),
)
```

**响应：** 执行结果（JSON 序列化）

### 8. wait_for - 等待元素出现

```go
mcp.NewTool("wait_for",
    mcp.WithDescription("Wait for an element to appear on the page"),
    mcp.WithString("selector",
        mcp.Description("CSS selector to wait for"),
        mcp.Required(),
    ),
    mcp.WithString("state",
        mcp.Description("Element state: visible, hidden, attached, detached"),
        mcp.DefaultString("visible"),
    ),
    mcp.WithNumber("timeout",
        mcp.Description("Maximum wait time in milliseconds"),
        mcp.DefaultNumber(30000),
    ),
)
```

**响应：** `{ "found": true, "elapsed": 1234 }`

### 9. get_cookies - 获取 Cookies

```go
mcp.NewTool("get_cookies",
    mcp.WithDescription("Get cookies for the current page"),
    mcp.WithString("url",
        mcp.Description("Filter cookies by URL (optional)"),
    ),
    mcp.WithString("name",
        mcp.Description("Filter cookies by name (optional)"),
    ),
)
```

**响应：** cookies 数组

### 10. list_tabs - 列出标签页

```go
mcp.NewTool("list_tabs",
    mcp.WithDescription("List all open browser tabs"),
)
```

**响应：** `[{ "id": 1, "url": "...", "title": "..." }]`

### 11. switch_tab - 切换标签页

```go
mcp.NewTool("switch_tab",
    mcp.WithDescription("Switch to a specific browser tab"),
    mcp.WithNumber("tab_id",
        mcp.Description("Tab ID to switch to"),
        mcp.Required(),
    ),
)
```

**响应：** `{ "switched": true, "url": "...", "title": "..." }`

### 12. close_tab - 关闭标签页

```go
mcp.NewTool("close_tab",
    mcp.WithDescription("Close a specific browser tab"),
    mcp.WithNumber("tab_id",
        mcp.Description("Tab ID to close (omit to close current tab)"),
    ),
)
```

**响应：** `{ "closed": true }`

### 13. new_tab - 新建标签页

```go
mcp.NewTool("new_tab",
    mcp.WithDescription("Open a new browser tab"),
    mcp.WithString("url",
        mcp.Description("URL to open in the new tab (optional)"),
    ),
)
```

**响应：** `{ "tab_id": 123, "url": "...", "title": "..." }`

### 14. hover - 鼠标悬停

```go
mcp.NewTool("hover",
    mcp.WithDescription("Hover the mouse over an element"),
    mcp.WithString("selector",
        mcp.Description("CSS selector of the element to hover over"),
        mcp.Required(),
    ),
)
```

**响应：** `{ "hovered": true, "tag": "div", "text": "..." }`

### 15. select_option - 选择下拉选项

```go
mcp.NewTool("select_option",
    mcp.WithDescription("Select an option in a dropdown/select element"),
    mcp.WithString("selector",
        mcp.Description("CSS selector of the select element"),
        mcp.Required(),
    ),
    mcp.WithString("value",
        mcp.Description("Value of the option to select"),
        mcp.Required(),
    ),
)
```

**响应：** `{ "selected": true, "value": "..." }`

## 工具命名映射

MCP 工具注册时用简短名称（如 `navigate`），Hermes 会自动加上前缀：
- `mcp_browser_navigate`
- `mcp_browser_screenshot`
- `mcp_browser_get_content`
- `mcp_browser_click`
- `mcp_browser_type`
- ...以此类推
