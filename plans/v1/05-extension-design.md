# Chrome Extension 详细设计

## Manifest V3 配置

```json
{
  "manifest_version": 3,
  "name": "Hermes Browser",
  "version": "1.0.0",
  "description": "MCP bridge for Hermes Agent to control the browser",
  "permissions": [
    "activeTab",
    "scripting",
    "tabs",
    "storage"
  ],
  "host_permissions": [
    "<all_urls>"
  ],
  "background": {
    "service_worker": "background.js"
  },
  "action": {
    "default_popup": "popup/popup.html",
    "default_icon": {
      "16": "icons/icon16.png",
      "48": "icons/icon48.png",
      "128": "icons/icon128.png"
    }
  },
  "icons": {
    "16": "icons/icon16.png",
    "48": "icons/icon48.png",
    "128": "icons/icon128.png"
  },
  "content_scripts": []
}
```

注意：content_scripts 为空数组。Content Script 采用按需注入（chrome.scripting.executeScript），不在 manifest 中声明。这样做的好处：
- 不在所有页面自动注入，减少干扰
- 避免某些网站的 CSP 限制
- 更隐蔽，不容易被网站检测

## Background Service Worker

### 核心职责

1. **WebSocket 连接管理**
   - 维护与 MCP Server 的 WebSocket 长连接
   - 断线自动重连（指数退避：1s, 2s, 4s, 8s, 最大 30s）
   - 心跳检测（每 30s 发送 ping）

2. **命令转发**
   - 接收 MCP Server 的 JSON 命令
   - 转发给对应 tab 的 Content Script
   - 收集 Content Script 的响应
   - 返回给 MCP Server

3. **Tab 管理**
   - 跟踪哪些 tab 已注入 Content Script
   - Tab 创建/关闭/更新时清理状态
   - 支持指定 tab 操作

### 消息流程

```
MCP Server
    | WebSocket message: { id, method, params, tabId? }
    v
Background Service Worker
    | 1. 检查目标 tab 是否已注入 content script
    | 2. 如果未注入，先注入 content.js
    | 3. chrome.tabs.sendMessage(tabId, { id, method, params })
    v
Content Script (目标页面)
    | 执行操作，返回结果
    v
Background Service Worker
    | 收到响应，通过 WebSocket 发回
    v
MCP Server
```

### 关键代码结构

```javascript
// background.js

// WebSocket 连接状态
let ws = null;
let wsUrl = 'ws://127.0.0.1:19876';
let token = '';
let reconnectAttempts = 0;
const MAX_RECONNECT_DELAY = 30000;

// 已注入 content script 的 tab 集合
const injectedTabs = new Set();

// 待响应的请求映射（用于匹配响应）
const pendingRequests = new Map();

// 默认活动 tab
let activeTabId = null;

// WebSocket 连接
function connect() { ... }

// 断线重连
function scheduleReconnect() { ... }

// WebSocket 消息处理
function handleMessage(message) { ... }

// 注入 content script 到指定 tab
async function ensureContentScript(tabId) { ... }

// 转发命令到 content script
async function forwardToContentScript(tabId, command) { ... }

// Tab 事件监听
chrome.tabs.onCreated.addListener(...);
chrome.tabs.onRemoved.addListener(...);
chrome.tabs.onUpdated.addListener(...);

// Popup 消息处理
chrome.runtime.onMessage.addListener(...);
```

## Content Script

### 核心功能

Content Script 注入到目标页面后，作为一个"隐形代理"执行所有 DOM 操作。

### 操作函数

```javascript
// content.js

// 接收来自 background 的命令
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
  const { id, method, params } = message;

  switch (method) {
    case 'navigate':
      window.location.href = params.url;
      sendResponse({ status: 'navigating' });
      break;

    case 'get_content':
      sendResponse(extractContent(params));
      break;

    case 'click':
      sendResponse(clickElement(params));
      break;

    case 'type':
      sendResponse(typeText(params));
      break;

    case 'scroll':
      sendResponse(scrollPage(params));
      break;

    case 'execute_js':
      sendResponse(executeJavaScript(params));
      break;

    case 'wait_for':
      waitForElement(params).then(sendResponse);
      return true; // 保持 sendResponse 通道开放（异步响应）

    case 'screenshot':
      // 截图需要通过 background 调用 chrome.tabs.captureVisibleTab
      // content script 自身无法截图
      chrome.runtime.sendMessage({ type: 'capture_screenshot', params });
      break;

    case 'hover':
      sendResponse(hoverElement(params));
      break;

    case 'select_option':
      sendResponse(selectOption(params));
      break;

    default:
      sendResponse({ status: 'error', error: `Unknown method: ${method}` });
  }
});
```

### 内容提取策略

```javascript
function extractContent(params) {
  const { selector, type } = params;
  const root = selector ? document.querySelector(selector) : document.body;

  if (!root) {
    return { status: 'error', error: `Element not found: ${selector}` };
  }

  switch (type) {
    case 'text':
      return { status: 'success', data: cleanText(root.innerText) };
    case 'html':
      return { status: 'success', data: root.innerHTML };
    case 'markdown':
      return { status: 'success', data: htmlToMarkdown(root) };
    default:
      return { status: 'success', data: cleanText(root.innerText) };
  }
}
```

### 智能文本清理

提取页面文本时需要清理：
- 隐藏元素（display:none, visibility:hidden, opacity:0）
- Script/Style 标签内容
- 连续空白压缩
- 导航栏、页脚等低价值内容（可选）

```javascript
function cleanText(text) {
  return text
    .replace(/\s+/g, ' ')           // 压缩空白
    .replace(/\n{3,}/g, '\n\n')     // 最多两个连续换行
    .trim();
}
```

### HTML to Markdown 简易转换

不需要引入外部库，实现一个轻量的转换器：
- h1-h6 -> # 标题
- p -> 段落
- a -> [text](href)
- img -> ![alt](src)
- ul/ol -> 列表
- table -> Markdown 表格
- pre/code -> 代码块

## Popup

### 功能

1. **连接状态显示**
   - 绿色圆点 = 已连接
   - 红色圆点 = 未连接
   - 黄色圆点 = 重连中

2. **配置面板**
   - WebSocket 地址输入框
   - Token 输入框
   - 连接/断开按钮

3. **操作日志**
   - 最近的命令和结果（最近 50 条）
   - 可清空

4. **快捷操作**
   - 截图当前页面
   - 获取当前页面内容
   - 复制当前页面 URL

### UI 布局

```
+----------------------------------+
| Hermes Browser          [状态灯] |
+----------------------------------+
| WebSocket: [ws://...:19876    ] |
| Token:     [••••••••••••••    ] |
| [连接]  [断开]                    |
+----------------------------------+
| 操作日志:                         |
| > navigate https://example.com   |
|   ✓ OK - Example Domain          |
| > get_content                    |
|   ✓ OK - (1234 chars)            |
| > click #btn                     |
|   ✗ Error: Element not found     |
+----------------------------------+
| [截图] [获取内容] [复制URL]       |
+----------------------------------+
```

## 截图方案

Content Script 自身无法调用 `chrome.tabs.captureVisibleTab`，需要通过 Background 中转：

1. Content Script 收到 `screenshot` 命令
2. Content Script 通过 `chrome.runtime.sendMessage` 转发给 Background
3. Background 调用 `chrome.tabs.captureVisibleTab` 获取截图
4. Background 将截图 base64 数据通过 WebSocket 发回 MCP Server

如果需要截取特定元素：
1. Content Script 先用 `element.getBoundingClientRect()` 获取元素位置
2. Background 裁剪截图到指定区域
3. 或者用 Canvas 在 Content Script 中裁剪（但需要获取完整页面截图）

更优方案：Content Script 中用 Canvas API 实现元素级截图
1. Background 调用 `chrome.tabs.captureVisibleTab` 获取可见区域截图
2. 将截图数据发送给 Content Script
3. Content Script 用 Canvas 裁剪指定元素区域
4. 返回裁剪后的 base64

对于 full_page 截图：
1. Content Script 逐屏截图并拼接
2. 或者 Background 通过 chrome.debugger API（需要额外权限，暂不使用）
