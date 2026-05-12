// Hermes Browser — Popup Script

const MAX_LOG_ENTRIES = 50;

// ─── DOM References ───────────────────────────────────────────────────────

const wsUrlInput = document.getElementById('wsUrlInput');
const tokenInput = document.getElementById('tokenInput');
const connectBtn = document.getElementById('connectBtn');
const disconnectBtn = document.getElementById('disconnectBtn');
const statusDot = document.getElementById('statusDot');
const statusLabel = document.getElementById('statusLabel');
const logContainer = document.getElementById('logContainer');
const clearLogBtn = document.getElementById('clearLogBtn');
const screenshotBtn = document.getElementById('screenshotBtn');
const getContentBtn = document.getElementById('getContentBtn');

// ─── Initialization ───────────────────────────────────────────────────────

// Load saved settings
chrome.storage.local.get(['wsUrl', 'token'], (result) => {
    if (result.wsUrl) wsUrlInput.value = result.wsUrl;
    if (result.token) tokenInput.value = result.token;
});

// Check current connection state
chrome.runtime.sendMessage({ type: 'get_state' }, (response) => {
    if (response) {
        updateUI(response);
        if (response.wsUrl) wsUrlInput.value = response.wsUrl;
        if (response.hasToken && !tokenInput.value) {
            // Token exists but isn't shown in the input for security
            tokenInput.placeholder = 'Token saved (enter to change)';
        }
    }
});

// ─── Event Handlers ───────────────────────────────────────────────────────

connectBtn.addEventListener('click', () => {
    const wsUrl = wsUrlInput.value.trim() || 'ws://127.0.0.1:19876';
    const token = tokenInput.value.trim();

    // Save settings
    chrome.storage.local.set({ wsUrl: wsUrl, token: token });

    // Send connect to background
    chrome.runtime.sendMessage({
        type: 'connect',
        wsUrl: wsUrl,
        token: token,
    }, () => {
        addLogEntry('info', `Connecting to ${wsUrl}...`);
    });
});

disconnectBtn.addEventListener('click', () => {
    chrome.runtime.sendMessage({ type: 'disconnect' }, () => {
        addLogEntry('info', 'Disconnected');
    });
});

clearLogBtn.addEventListener('click', () => {
    logContainer.innerHTML = '';
});

screenshotBtn.addEventListener('click', () => {
    // Send a screenshot command via the background
    chrome.runtime.sendMessage({
        type: 'command',
        method: 'screenshot',
        params: { format: 'jpeg', quality: 80 },
    }, (response) => {
        if (response && response.status === 'success') {
            addLogEntry('success', 'Screenshot captured');
        } else {
            addLogEntry('error', `Screenshot failed: ${response?.error || 'unknown'}`);
        }
    });
});

getContentBtn.addEventListener('click', () => {
    chrome.runtime.sendMessage({
        type: 'command',
        method: 'get_content',
        params: { type: 'text' },
    }, (response) => {
        if (response && response.status === 'success') {
            const length = response.data ? response.data.length : 0;
            addLogEntry('success', `Content extracted (${length} chars)`);
        } else {
            addLogEntry('error', `Get content failed: ${response?.error || 'unknown'}`);
        }
    });
});

// Listen for connection state updates from background
chrome.runtime.onMessage.addListener((message) => {
    if (message.type === 'connection_state') {
        updateUI(message);
        switch (message.state) {
            case 'connected':
                addLogEntry('success', 'Connected to server');
                break;
            case 'disconnected':
                addLogEntry('info', 'Disconnected from server');
                break;
            case 'reconnecting':
                addLogEntry('info', `Reconnecting... (attempt ${message.reconnectAttempts})`);
                break;
        }
    } else if (message.type === 'command_result') {
        const cls = message.status === 'success' ? 'success' : 'error';
        addLogEntry(cls, `${message.method}: ${message.status}`);
    }
});

// ─── UI Updates ───────────────────────────────────────────────────────────

function updateUI(state) {
    if (state.connected) {
        statusDot.className = 'status-dot connected';
        statusLabel.textContent = 'Connected';
        connectBtn.disabled = true;
        disconnectBtn.disabled = false;
    } else if (state.reconnecting) {
        statusDot.className = 'status-dot reconnecting';
        statusLabel.textContent = `Reconnecting... (#${state.reconnectAttempts})`;
        connectBtn.disabled = true;
        disconnectBtn.disabled = true;
    } else {
        statusDot.className = 'status-dot disconnected';
        statusLabel.textContent = 'Disconnected';
        connectBtn.disabled = false;
        disconnectBtn.disabled = true;
    }
}

// ─── Logging ──────────────────────────────────────────────────────────────

function addLogEntry(cls, text) {
    const entry = document.createElement('div');
    entry.className = `log-entry ${cls}`;
    const time = new Date().toLocaleTimeString();
    entry.textContent = `${time} ${text}`;
    logContainer.appendChild(entry);

    // Trim logs
    while (logContainer.children.length > MAX_LOG_ENTRIES) {
        logContainer.removeChild(logContainer.firstChild);
    }

    // Auto-scroll to bottom
    logContainer.scrollTop = logContainer.scrollHeight;
}
