# Chrome Extension 详细设计（v2）

> 修订记录：根据审计意见修正 P0#2(navigate 归 Background)、P1#4(screenshot 归 Background)、P1#6(cleanText)、P1#7(networkidle)、P1#8(activeTab)、P2#13(删除 content_scripts)、P2#14(icons)、P2#17(命令路由表)

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
    "storage",
    "cookies"
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
  }
}
```

变更说明（v2）：
- 移除 `"content_scripts": []`（不需要就不声明）
- permissions 新增 `"cookies"`（get_cookies 工具需要）
- 图标文件由 `extension/icons/` 目录提供

## 命令路由表

Background Service Worker 作为命令分发器，收到 WebSocket 消息后根据 method 决定处理方式：

### Background 直接处理

| 命令 | Chrome API | 说明 |
|------|-----------|------|
| `navigate` | chrome.tabs.update + onUpdated | Background 触发导航，监听加载完成 |
| `screenshot` | chrome.tabs.captureVisibleTab | Background 截图，可选元素裁剪 |
| `list_tabs` | chrome.tabs.query | 列出所有 tab |
| `switch_tab` | chrome.tabs.update({ active: true }) | 切换 tab |
| `new_tab` | chrome.tabs.create | 新建 tab |
| `close_tab` | chrome.tabs.remove | 关闭 tab |
| `get_cookies` | chrome.cookies.getAll | 读取 cookies |

### 转发给 Content Script

| 命令 | 说明 |
|------|------|
| `click` | 查找元素 + 模拟点击 |
| `type` | 聚焦输入框 + 输入文本 |
| `hover` | 模拟鼠标悬停 |
| `scroll` | 滚动页面 |
| `select_option` | 选择下拉选项 |
| `get_content` | 提取页面内容 |
| `execute_js` | 执行 JS 代码 |
| `wait_for` | 等待元素出现 |

## Background Service Worker

### 核心职责

1. **WebSocket 连接管理**
   - 维护与 MCP Server 的 WebSocket 长连接
   - 断线自动重连（指数退避：1s, 2s, 4s, 8s, 最大 30s）
   - 心跳检测（每 30s 发送 ping）

2. **命令分发**（v2 新增）
   - 收到 WebSocket 消息后根据 method 路由
   - Background 直接处理的命令：调用 Chrome API，直接回复
   - 需要转发给 Content Script 的命令：注入 → 发送 → 等待响应 → 回复

3. **Tab 管理**
   - 跟踪哪些 tab 已注入 Content Script
   - Tab 创建/关闭/更新时清理状态
   - ensureTabActive：操作前确保目标 tab 激活

### ensureTabActive

每次需要操作页面（注入 Content Script、截图等）前调用：

```javascript
async function ensureTabActive(tabId) {
    const tab = await chrome.tabs.get(tabId);
    if (!tab.active) {
        await chrome.tabs.update(tabId, { active: true });
        // 等待 activeTab 权限生效
        await new Promise(r => setTimeout(r, 100));
    }
}
```

### 消息流程

**Background 直接处理的命令（如 navigate）：**
```
MCP Server
    | WebSocket: { id, method: "navigate", params, tab_id }
    v
Background Service Worker
    | ensureTabActive(tabId)
    | chrome.tabs.update(tabId, { url: params.url })
    | waitForTabComplete(tabId)
    | const tab = await chrome.tabs.get(tabId)
    v
Background → WebSocket: { id, status: "success", data: { url, title } }
    v
MCP Server
```

**转发给 Content Script 的命令（如 click）：**
```
MCP Server
    | WebSocket: { id, method: "click", params, tab_id }
    v
Background Service Worker
    | ensureTabActive(tabId)
    | await ensureContentScript(tabId)
    | chrome.tabs.sendMessage(tabId, { id, method: "click", params })
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

// 命令路由表：Background 直接处理的方法
const BACKGROUND_METHODS = new Set([
    'navigate', 'screenshot', 'list_tabs', 'switch_tab',
    'new_tab', 'close_tab', 'get_cookies',
]);

// WebSocket 消息处理
async function handleMessage(message) {
    const { id, method, params, tab_id } = message;

    if (BACKGROUND_METHODS.has(method)) {
        // Background 直接处理
        await handleBackgroundCommand(id, method, params, tab_id);
    } else {
        // 转发给 Content Script
        await forwardToContentScript(id, method, params, tab_id);
    }
}

// Background 命令处理
async function handleBackgroundCommand(id, method, params, tabId) {
    try {
        let data;
        switch (method) {
            case 'navigate':
                data = await handleNavigate(tabId, params);
                break;
            case 'screenshot':
                data = await handleScreenshot(tabId, params);
                break;
            case 'list_tabs':
                data = await handleListTabs();
                break;
            case 'switch_tab':
                data = await handleSwitchTab(tabId);
                break;
            case 'new_tab':
                data = await handleNewTab(params);
                break;
            case 'close_tab':
                data = await handleCloseTab(tabId);
                break;
            case 'get_cookies':
                data = await handleGetCookies(tabId, params);
                break;
        }
        sendToServer({ id, status: 'success', data });
    } catch (err) {
        sendToServer({ id, status: 'error', code: err.code || 'UNKNOWN', error: err.message });
    }
}

// 转发到 Content Script
async function forwardToContentScript(id, method, params, tabId) {
    try {
        await ensureTabActive(tabId);
        await ensureContentScript(tabId);

        const response = await chrome.tabs.sendMessage(tabId, {
            id, method, params
        });

        sendToServer({ id, status: response.status, data: response.data,
                        code: response.code, error: response.error });
    } catch (err) {
        sendToServer({ id, status: 'error', code: err.code || 'UNKNOWN', error: err.message });
    }
}

// 注入 content script 到指定 tab
async function ensureContentScript(tabId) {
    if (injectedTabs.has(tabId)) return;

    await chrome.scripting.executeScript({
        target: { tabId },
        files: ['content.js'],
    });
    injectedTabs.add(tabId);
}

// Tab 事件监听
chrome.tabs.onRemoved.addListener((tabId) => {
    injectedTabs.delete(tabId);
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
    // 页面导航后 content script 失效，需要重新注入
    if (changeInfo.status === 'loading') {
        injectedTabs.delete(tabId);
    }
});

// Popup 消息处理
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    // Popup 的配置更新等消息处理
});
```

### navigate 实现（Background 直接处理）

```javascript
async function handleNavigate(tabId, params) {
    const { url, wait_until = 'networkidle' } = params;

    // chrome.tabs.update 触发导航
    await chrome.tabs.update(tabId, { url });

    // 等待页面加载完成
    if (wait_until === 'load' || wait_until === 'networkidle') {
        await waitForTabComplete(tabId);
    }

    // 如果是 networkidle，额外等待
    if (wait_until === 'networkidle') {
        await new Promise(r => setTimeout(r, 500));
    }

    const tab = await chrome.tabs.get(tabId);
    return { url: tab.url, title: tab.title };
}

function waitForTabComplete(tabId) {
    return new Promise((resolve, reject) => {
        // 先检查当前状态
        chrome.tabs.get(tabId).then(tab => {
            if (tab.status === 'complete') {
                resolve();
                return;
            }
        });

        const listener = (updatedTabId, changeInfo) => {
            if (updatedTabId === tabId && changeInfo.status === 'complete') {
                chrome.tabs.onUpdated.removeListener(listener);
                resolve();
            }
        };
        chrome.tabs.onUpdated.addListener(listener);

        // 超时保护
        setTimeout(() => {
            chrome.tabs.onUpdated.removeListener(listener);
            resolve(); // 超时也返回，不阻塞
        }, 60000);
    });
}
```

### screenshot 实现（Background 直接处理）

```javascript
async function handleScreenshot(tabId, params) {
    const { format = 'jpeg', quality = 80, selector } = params;

    await ensureTabActive(tabId);

    const tab = await chrome.tabs.get(tabId);
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
        format,
        quality,
    });

    // 如果指定了 selector，需要裁剪
    if (selector) {
        return await cropElement(tabId, dataUrl, selector);
    }

    // 去掉 data:image/jpeg;base64, 前缀
    const base64 = dataUrl.replace(/^data:image\/\w+;base64,/, '');
    return { image: base64, format };
}

async function cropElement(tabId, dataUrl, selector) {
    // 注入 content script 获取元素位置
    await ensureContentScript(tabId);
    const response = await chrome.tabs.sendMessage(tabId, {
        id: 'internal',
        method: 'get_element_rect',
        params: { selector },
    });

    if (response.status === 'error') {
        throw { code: 'ELEMENT_NOT_FOUND', message: response.error };
    }

    const rect = response.data; // { x, y, width, height }

    // 用 OffscreenCanvas 裁剪（Manifest V3 推荐）
    // 或者用 Background 中的 Canvas
    // 由于 Background 是 Service Worker，需要用 OffscreenCanvas
    // 简化方案：返回完整截图 + 元素坐标，让 MCP Server 或 LLM 处理
    // 完整方案：传给 content script 用 Canvas 裁剪后返回

    // 首版：返回完整截图 + 元素坐标信息
    const base64 = dataUrl.replace(/^data:image\/\w+;base64,/, '');
    return {
        image: base64,
        format: 'jpeg',
        element_rect: rect,
        note: 'Element cropping not implemented in v1, full screenshot returned with element bounds',
    };
}
```

## Content Script

### 操作函数

Content Script 只处理转发来的页面级命令（不处理 navigate、screenshot 等）：

```javascript
// content.js

chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    const { id, method, params } = message;

    switch (method) {
        case 'click':
            sendResponse(clickElement(params));
            break;

        case 'type':
            sendResponse(typeText(params));
            break;

        case 'hover':
            sendResponse(hoverElement(params));
            break;

        case 'scroll':
            sendResponse(scrollPage(params));
            break;

        case 'select_option':
            sendResponse(selectOption(params));
            break;

        case 'get_content':
            sendResponse(extractContent(params));
            break;

        case 'execute_js':
            sendResponse(executeJavaScript(params));
            break;

        case 'wait_for':
            waitForElement(params).then(sendResponse);
            return true; // 异步响应，保持通道开放

        case 'get_element_rect':
            // 内部命令，供 screenshot 裁剪使用
            sendResponse(getElementRect(params));
            break;

        default:
            sendResponse({
                status: 'error',
                code: 'UNKNOWN_METHOD',
                error: `Unknown method: ${method}`,
            });
    }
});
```

### 内容提取

```javascript
function extractContent(params) {
    const { selector, type = 'text' } = params;
    const root = selector ? document.querySelector(selector) : document.body;

    if (!root) {
        return { status: 'error', code: 'ELEMENT_NOT_FOUND',
                 error: `Element not found: ${selector}` };
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

### 智能文本清理（v2 修正）

```javascript
function cleanText(text) {
    return text
        .replace(/[ \t]+/g, ' ')        // 行内多个空格/tab 压缩为一个
        .replace(/ *\n */g, '\n')       // 行尾行首空白清理
        .replace(/\n{3,}/g, '\n\n')     // 多个空行压缩为两个换行
        .trim();
}
```

### 等待元素

```javascript
function waitForElement(params) {
    const { selector, state = 'visible', timeout = 30000 } = params;

    return new Promise((resolve) => {
        const start = Date.now();

        const check = () => {
            const el = document.querySelector(selector);

            if (state === 'visible' && el && el.offsetParent !== null) {
                resolve({ status: 'success', data: { found: true, elapsed: Date.now() - start } });
                return;
            }
            if (state === 'hidden' && (!el || el.offsetParent === null)) {
                resolve({ status: 'success', data: { found: true, elapsed: Date.now() - start } });
                return;
            }
            if (state === 'attached' && el) {
                resolve({ status: 'success', data: { found: true, elapsed: Date.now() - start } });
                return;
            }
            if (state === 'detached' && !el) {
                resolve({ status: 'success', data: { found: true, elapsed: Date.now() - start } });
                return;
            }

            if (Date.now() - start >= timeout) {
                resolve({ status: 'error', code: 'TIMEOUT',
                          error: `Timeout waiting for '${selector}' to be ${state}` });
                return;
            }

            requestAnimationFrame(check);
        };

        check();
    });
}
```

### HTML to Markdown 简易转换

不引入外部库，轻量实现：
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
|   ✗ ELEMENT_NOT_FOUND            |
+----------------------------------+
| [截图] [获取内容] [复制URL]       |
+----------------------------------+
```
