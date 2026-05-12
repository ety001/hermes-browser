// Hermes Browser — Background Service Worker
// Manages WebSocket connection to MCP Server and dispatches commands.

// ─── State ────────────────────────────────────────────────────────────────

let ws = null;
let wsUrl = 'ws://127.0.0.1:19876';
let token = '';
let reconnectAttempts = 0;
const MAX_RECONNECT_DELAY = 30000;
const HEARTBEAT_INTERVAL = 30000;
let heartbeatTimer = null;
let reconnectTimer = null;

// Tabs that have been injected with content script
const injectedTabs = new Set();

// Commands that are handled directly in background (not forwarded to content script)
const BACKGROUND_METHODS = new Set([
    'navigate', 'screenshot', 'list_tabs', 'switch_tab',
    'new_tab', 'close_tab', 'get_cookies',
    'execute_js',  // handled via MAIN world injection to bypass CSP
]);

// ─── WebSocket Connection ─────────────────────────────────────────────────

function connect() {
    if (ws && ws.readyState === WebSocket.OPEN) return;

    const url = token ? `${wsUrl}?token=${token}` : wsUrl;

    try {
        ws = new WebSocket(url);
    } catch (err) {
        console.error('[Hermes] WebSocket creation failed:', err);
        scheduleReconnect();
        return;
    }

    ws.onopen = () => {
        console.log('[Hermes] WebSocket connected');
        reconnectAttempts = 0;
        updatePopupState('connected');
        startHeartbeat();
    };

    ws.onclose = (event) => {
        console.log('[Hermes] WebSocket disconnected:', event.code, event.reason);
        ws = null;
        stopHeartbeat();
        updatePopupState('disconnected');
        scheduleReconnect();
    };

    ws.onerror = (err) => {
        console.error('[Hermes] WebSocket error:', err);
    };

    ws.onmessage = (event) => {
        try {
            const message = JSON.parse(event.data);
            handleMessage(message);
        } catch (err) {
            console.error('[Hermes] Failed to parse message:', err);
        }
    };
}

function disconnect() {
    stopHeartbeat();
    stopReconnect();
    if (ws) {
        ws.onclose = null; // prevent auto-reconnect on intentional close
        ws.close(1000, 'client disconnect');
        ws = null;
    }
    updatePopupState('disconnected');
}

function scheduleReconnect() {
    if (reconnectTimer) return;

    const delay = Math.min(
        Math.pow(2, reconnectAttempts) * 1000,
        MAX_RECONNECT_DELAY
    );
    reconnectAttempts++;

    console.log(`[Hermes] Reconnecting in ${delay}ms (attempt ${reconnectAttempts})`);
    updatePopupState('reconnecting');

    reconnectTimer = setTimeout(() => {
        reconnectTimer = null;
        connect();
    }, delay);
}

function stopReconnect() {
    if (reconnectTimer) {
        clearTimeout(reconnectTimer);
        reconnectTimer = null;
    }
}

// ─── Heartbeat ────────────────────────────────────────────────────────────

function startHeartbeat() {
    stopHeartbeat();
    heartbeatTimer = setInterval(() => {
        if (ws && ws.readyState === WebSocket.OPEN) {
            ws.send(JSON.stringify({ type: 'ping' }));
        }
    }, HEARTBEAT_INTERVAL);
}

function stopHeartbeat() {
    if (heartbeatTimer) {
        clearInterval(heartbeatTimer);
        heartbeatTimer = null;
    }
}

// ─── Popup Communication ──────────────────────────────────────────────────

function updatePopupState(state) {
    chrome.runtime.sendMessage({
        type: 'connection_state',
        state: state,
        reconnectAttempts: reconnectAttempts,
    }).catch(() => {
        // Popup may not be open; ignore
    });
}

// Listen for messages from popup
chrome.runtime.onMessage.addListener((message, sender, sendResponse) => {
    switch (message.type) {
        case 'get_state':
            sendResponse({
                connected: ws !== null && ws.readyState === WebSocket.OPEN,
                reconnecting: reconnectTimer !== null,
                reconnectAttempts: reconnectAttempts,
                wsUrl: wsUrl,
                hasToken: token !== '',
            });
            break;

        case 'connect':
            if (message.wsUrl) wsUrl = message.wsUrl;
            if (message.token !== undefined) token = message.token;
            saveSettings(wsUrl, token);
            connect();
            sendResponse({ success: true });
            break;

        case 'disconnect':
            disconnect();
            sendResponse({ success: true });
            break;

        case 'save_settings':
            saveSettings(message.wsUrl, message.token);
            sendResponse({ success: true });
            break;

        case 'command':
            // Popup quick-action commands (screenshot, get_content)
            handlePopupCommand(message.method, message.params)
                .then((result) => {
                    // Notify popup of result
                    chrome.runtime.sendMessage({
                        type: 'command_result',
                        method: message.method,
                        status: result.status,
                        data: result.data,
                    }).catch(() => {});
                    sendResponse(result);
                });
            return true; // async
    }
    return true;
});

async function handlePopupCommand(method, params) {
    try {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        if (!tab) {
            return { status: 'error', error: 'No active tab found' };
        }
        const tabId = tab.id;

        if (BACKGROUND_METHODS.has(method)) {
            const handler = getBackgroundHandler(method);
            const data = await handler(tabId, params || {});
            return { status: 'success', data };
        } else {
            await ensureContentScript(tabId);
            const response = await chrome.tabs.sendMessage(tabId, {
                id: 'popup',
                method: method,
                params: params || {},
            });
            return response;
        }
    } catch (err) {
        return { status: 'error', error: err.message || String(err) };
    }
}

function getBackgroundHandler(method) {
    switch (method) {
        case 'navigate': return handleNavigate;
        case 'screenshot': return handleScreenshot;
        case 'list_tabs': return handleListTabs;
        case 'switch_tab': return handleSwitchTab;
        case 'new_tab': return handleNewTab;
        case 'close_tab': return handleCloseTab;
        case 'get_cookies': return handleGetCookies;
        case 'execute_js': return handleExecuteJs;
        default: return null;
    }
}

// ─── Settings Persistence ─────────────────────────────────────────────────

function loadSettings() {
    chrome.storage.local.get(['wsUrl', 'token'], (result) => {
        if (result.wsUrl) wsUrl = result.wsUrl;
        if (result.token) token = result.token;
    });
}

function saveSettings(newWsUrl, newToken) {
    wsUrl = newWsUrl || wsUrl;
    token = newToken !== undefined ? newToken : token;
    chrome.storage.local.set({ wsUrl: wsUrl, token: token });
}

// Load settings on startup
loadSettings();

// ─── Message Handling (placeholder — filled in Phase 2.2) ─────────────────

function handleMessage(message) {
    const { id, method, params, tab_id } = message;

    if (BACKGROUND_METHODS.has(method)) {
        handleBackgroundCommand(id, method, params, tab_id);
    } else {
        forwardToContentScript(id, method, params, tab_id);
    }
}

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
            case 'execute_js':
                data = await handleExecuteJs(tabId, params);
                break;
        }
        sendToServer({ id, status: 'success', data });
    } catch (err) {
        sendToServer({
            id, status: 'error',
            code: err.code || 'UNKNOWN',
            error: err.message,
        });
    }
}

async function forwardToContentScript(id, method, params, tabId) {
    try {
        await ensureTabActive(tabId);
        await ensureContentScript(tabId);

        const response = await chrome.tabs.sendMessage(tabId, {
            id, method, params,
        });

        sendToServer({
            id,
            status: response.status,
            data: response.data,
            code: response.code,
            error: response.error,
        });
    } catch (err) {
        sendToServer({
            id, status: 'error',
            code: err.code || 'UNKNOWN',
            error: err.message,
        });
    }
}

function sendToServer(response) {
    if (ws && ws.readyState === WebSocket.OPEN) {
        ws.send(JSON.stringify(response));
    }
}

// ─── Tab Management ───────────────────────────────────────────────────────

async function ensureTabActive(tabId) {
    if (!tabId) return;
    const tab = await chrome.tabs.get(tabId);
    if (!tab.active) {
        await chrome.tabs.update(tabId, { active: true });
        // Brief wait for activeTab permission to take effect
        await new Promise(r => setTimeout(r, 100));
    }
}

// ─── Content Script Injection ─────────────────────────────────────────────

async function ensureContentScript(tabId) {
    if (injectedTabs.has(tabId)) return;

    await chrome.scripting.executeScript({
        target: { tabId },
        files: ['content.js'],
    });
    injectedTabs.add(tabId);
}

// ─── Tab Event Listeners ──────────────────────────────────────────────────

chrome.tabs.onRemoved.addListener((tabId) => {
    injectedTabs.delete(tabId);
});

chrome.tabs.onUpdated.addListener((tabId, changeInfo) => {
    if (changeInfo.status === 'loading') {
        injectedTabs.delete(tabId);
    }
});

// ─── Background Command Handlers ───────────────────────────────────────────

async function handleNavigate(tabId, params) {
    const { url, wait_until = 'networkidle', timeout } = params;
    if (!url) throw { code: 'NAVIGATION_ERROR', message: 'url is required' };

    // Navigate
    await chrome.tabs.update(tabId, { url });

    // Wait for page to load
    const timeoutMs = timeout || 30000;
    if (wait_until === 'load' || wait_until === 'networkidle') {
        await waitForTabComplete(tabId, timeoutMs);
    }

    // For networkidle, add extra wait
    if (wait_until === 'networkidle') {
        await new Promise(r => setTimeout(r, 500));
    }

    const tab = await chrome.tabs.get(tabId);
    return { url: tab.url, title: tab.title };
}

function waitForTabComplete(tabId, timeoutMs = 60000) {
    return new Promise((resolve, reject) => {
        // Check current status
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

        setTimeout(() => {
            chrome.tabs.onUpdated.removeListener(listener);
            reject({ code: 'TIMEOUT', message: `Navigation timeout after ${timeoutMs}ms` });
        }, timeoutMs);
    });
}

async function handleScreenshot(tabId, params) {
    const { format = 'jpeg', quality = 80, selector } = params;

    await ensureTabActive(tabId);

    const tab = await chrome.tabs.get(tabId);
    const dataUrl = await chrome.tabs.captureVisibleTab(tab.windowId, {
        format,
        quality,
    });

    // If a specific element is requested, crop via content script
    if (selector) {
        return await cropElementScreenshot(tabId, dataUrl, selector);
    }

    // Strip data:image/...;base64, prefix
    const base64 = dataUrl.replace(/^data:image\/\w+;base64,/, '');
    return { image: base64, format };
}

async function cropElementScreenshot(tabId, dataUrl, selector) {
    // Inject content script and get element rect
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

    // Return full screenshot with element bounds
    // (Full Canvas cropping deferred to future version)
    const base64 = dataUrl.replace(/^data:image\/\w+;base64,/, '');
    return {
        image: base64,
        format: 'jpeg',
        element_rect: rect,
        note: 'Full screenshot returned with element bounds',
    };
}

async function handleListTabs() {
    const tabs = await chrome.tabs.query({});
    return tabs.map(t => ({
        id: t.id,
        url: t.url,
        title: t.title,
        active: t.active,
    }));
}

async function handleSwitchTab(tabId) {
    if (!tabId) throw { code: 'TAB_NOT_FOUND', message: 'tab_id is required' };
    const tab = await chrome.tabs.update(tabId, { active: true });
    return { switched: true, url: tab.url, title: tab.title };
}

async function handleNewTab(params) {
    const tab = await chrome.tabs.create({ url: params.url || 'about:blank' });
    return { tab_id: tab.id, url: tab.url, title: tab.title };
}

async function handleCloseTab(tabId) {
    if (tabId) {
        await chrome.tabs.remove(tabId);
    } else {
        const [tab] = await chrome.tabs.query({ active: true, currentWindow: true });
        if (tab) await chrome.tabs.remove(tab.id);
    }
    return { closed: true };
}

async function handleGetCookies(tabId, params) {
    // Need to get the URL from the tab if not provided
    let url = params.url;
    if (!url && tabId) {
        const tab = await chrome.tabs.get(tabId);
        url = tab.url;
    }
    const options = {};
    if (url) options.url = url;
    if (params.name) options.name = params.name;

    const cookies = await chrome.cookies.getAll(options);
    return cookies.map(c => ({
        name: c.name,
        value: c.value,
        domain: c.domain,
        path: c.path,
        secure: c.secure,
        httpOnly: c.httpOnly,
    }));
}

// ─── Execute JS (MAIN world injection to bypass CSP) ─────────────────────

async function handleExecuteJs(tabId, params) {
    const { expression, return_value = true } = params;
    if (!expression) throw { code: 'JS_EXECUTION_ERROR', message: 'expression is required' };

    await ensureTabActive(tabId);

    try {
        // Inject into MAIN world to bypass page CSP restrictions on eval.
        // chrome.scripting.executeScript automatically awaits the returned Promise.
        const results = await chrome.scripting.executeScript({
            target: { tabId },
            world: 'MAIN',
            func: (code, needResult) => {
                return (async () => {
                    try {
                        const wrappedCode = needResult
                            ? `(async () => { return (${code}); })()`
                            : `(async () => { ${code} })()`;
                        const rawResult = await eval(wrappedCode);
                        if (needResult) {
                            return {
                                value: rawResult !== undefined ? String(rawResult) : undefined,
                                type: typeof rawResult,
                            };
                        }
                        return { value: undefined, type: 'undefined' };
                    } catch (e) {
                        return { _error: e.message };
                    }
                })();
            },
            args: [expression, return_value],
        });

        const frame = results[0];
        if (!frame || frame.result === undefined) {
            throw { code: 'JS_EXECUTION_ERROR', message: 'No result from injected script' };
        }

        const result = frame.result;
        if (result._error) {
            throw { code: 'JS_EXECUTION_ERROR', message: result._error };
        }

        return result;
    } catch (err) {
        if (err.code) throw err;
        throw { code: 'JS_EXECUTION_ERROR', message: err.message || String(err) };
    }
}

// ─── Initialization ───────────────────────────────────────────────────────

// First-run onboarding
chrome.runtime.onInstalled.addListener((details) => {
    if (details.reason === 'install') {
        console.log('[Hermes] First install — set default settings');
        chrome.storage.local.set({
            wsUrl: 'ws://127.0.0.1:19876',
            onboardingComplete: false,
        });
    }
});

// Auto-connect if settings exist
chrome.storage.local.get(['wsUrl', 'token'], (result) => {
    if (result.wsUrl) {
        wsUrl = result.wsUrl;
        token = result.token || '';
        connect();
    }
});

console.log('[Hermes] Background service worker loaded');
