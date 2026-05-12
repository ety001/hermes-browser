# P1 - 设计缺陷，需要调整

### 4. screenshot 的响应链路没闭环

**文件：** 05-extension-design.md（Content Script 操作函数、截图方案）

计划中 content script 处理 screenshot：
```javascript
case 'screenshot':
    chrome.runtime.sendMessage({ type: 'capture_screenshot', params });
    break;  // 没有 sendResponse！
```

**问题：**
1. `chrome.runtime.sendMessage` 是异步的，`break` 后 sendResponse 通道关闭，
   Background 收到的消息里没有 MCP request ID，无法匹配响应。
2. `chrome.tabs.captureVisibleTab` 只能在 Background Service Worker 中调用，
   Content Script 无权调用。所以 screenshot 本来就不该走 Content Script。

**修复方案：** screenshot 应该在 Background 层直接处理，不经过 Content Script。

流程：
```
MCP Server 发 { id, method: "screenshot", params: { format, quality, selector }, tabId }
  → WebSocket → Background
  → Background 调用 chrome.tabs.captureVisibleTab(tab.windowId, { format, quality })
  → 如果有 selector：注入 content script 获取元素坐标，Canvas 裁剪
  → Background 通过 WebSocket 回复 { id, status: "success", data: { image: "base64..." } }
```

Background 中需要拦截 screenshot 命令，和 navigate 类似作为 "tab 级操作" 处理。

**结论：** 需要在 Background 中维护一个命令分发器，区分：
- **Background 直接处理：** navigate, screenshot, list_tabs, switch_tab, new_tab, close_tab, get_cookies
- **转发给 Content Script：** click, type, hover, scroll, select_option, get_content, execute_js, wait_for

---

### 5. Token 双配置增加负担

**文件：** 02-config-design.md

计划中 WebSocket token 和 HTTP token 分开配置：
```yaml
server:
  http:
    token: ""
websocket:
  token: ""
```

**问题：** 自用场景下，MCP Server 和 WebSocket 在同一进程内，
两个 token 分开管理没有安全收益，反而增加配置负担。
用户需要分别配两个 token，Hermes config.yaml 里也要对应配对。

**建议：** 默认共用一个 token，配置简化为：
```yaml
token: ""                    # 顶层 token，WebSocket 和 HTTP 共用
server:
  http:
    bind: "0.0.0.0:19875"
    # token: ""              # 可选覆盖，留空则用顶层 token
websocket:
  bind: "127.0.0.1:19876"
  # token: ""                # 可选覆盖，留空则用顶层 token
```

优先级：`server.http.token` > `token`（HTTP），`websocket.token` > `token`（WebSocket）。

---

### 6. cleanText 逻辑错误

**文件：** 05-extension-design.md（智能文本清理）

```javascript
function cleanText(text) {
    return text
        .replace(/\s+/g, ' ')           // 所有空白（含换行）变成空格
        .replace(/\n{3,}/g, '\n\n')     // 此时已经没有换行了，这行无效
        .trim();
}
```

**问题：** 第一个 replace 把 `\n` 也变成了空格，第二个 replace 匹配不到 `\n` 了。
结果是所有文本被压成一行。

**修复方案：**
```javascript
function cleanText(text) {
    return text
        .replace(/[ \t]+/g, ' ')        // 行内多个空格/tab 压缩为一个
        .replace(/ *\n */g, '\n')       // 行尾行首空白清理
        .replace(/\n{3,}/g, '\n\n')     // 多个空行压缩为两个换行
        .trim();
}
```

---

### 7. networkidle 等待策略未说明实现方式

**文件：** 02-config-design.md、03-mcp-tools.md

多处引用 `networkidle` 作为默认等待策略，但没有说明在 Manifest V3 环境下如何实现。

**问题：** Chrome Extension 没有 Playwright 的 `page.waitForNetworkIdle()` API。
需要自行实现。

**建议实现方案（写入计划）：**

方案 A：PerformanceObserver（推荐）
```javascript
function waitForNetworkIdle(timeout = 500) {
    return new Promise((resolve) => {
        let timer;
        const observer = new PerformanceObserver((list) => {
            // 有新网络活动，重置计时器
            clearTimeout(timer);
            timer = setTimeout(() => {
                observer.disconnect();
                resolve();
            }, timeout);
        });
        observer.observe({ entryTypes: ['resource'] });
        // 初始计时
        timer = setTimeout(() => {
            observer.disconnect();
            resolve();
        }, timeout);
    });
}
```

方案 B：简化版（首版够用）
```javascript
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

建议首版用方案 B，后续再升级为方案 A。

---

### 8. activeTab 权限对后台 tab 操作的限制

**文件：** 05-extension-design.md（Manifest V3 配置）

当前 permissions: `activeTab`, `scripting`, `tabs`, `storage`

**问题：** `activeTab` 权限只在用户主动与页面交互（点击 Extension 图标、快捷键等）
时临时授予当前 tab 的权限。如果 Agent 操作的是非激活 tab（后台 tab），
`chrome.tabs.captureVisibleTab`、`chrome.scripting.executeScript` 等操作会失败。

**建议：**
1. 所有需要操作页面的命令执行前，先 `chrome.tabs.update(tabId, { active: true })`
   切换到目标 tab（这会触发 activeTab 授权）
2. 或者在 manifest.json 中添加 `"permissions": ["activeTab", "scripting", "tabs", "storage", "tabCapture"]`
   但 tabCapture 主要用于音频，对截图无帮助
3. 最简方案：始终在当前激活 tab 操作，多 tab 切换时先 update active

```javascript
// Background 中，每次操作前确保 tab 激活
async function ensureTabActive(tabId) {
    const tab = await chrome.tabs.get(tabId);
    if (!tab.active) {
        await chrome.tabs.update(tabId, { active: true });
        // 等待一小段时间让 activeTab 权限生效
        await new Promise(r => setTimeout(r, 100));
    }
}
```
