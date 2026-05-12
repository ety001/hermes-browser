# Hermes Browser

MCP bridge for Hermes Agent to control a real Chrome browser through a Chrome Extension.

## Architecture

```
┌─ Hermes Agent ──────────────────────────────────────────────────────┐
│  ~/.hermes/config.yaml:                                             │
│    mcp_servers.hermes-browser.url → http://<host>:19875/mcp         │
│    headers: Authorization: Bearer <token>                           │
└────────────────────────┬────────────────────────────────────────────┘
                         │ MCP StreamableHTTP (JSON-RPC 2.0)
                         ▼
┌─ MCP Server (Go binary) ───────────────────────────────────────────┐
│  Listens on 0.0.0.0:19875 (HTTP, for Hermes)                      │
│  Listens on 127.0.0.1:19876 (WebSocket, for Chrome Extension)     │
│  Token: configurable — same token authenticates both transports   │
└────────────────────────┬────────────────────────────────────────────┘
                         │ WebSocket (ws://127.0.0.1:19876?token=...)
                         ▼
┌─ Chrome Extension (Manifest V3) ───────────────────────────────────┐
│                                                                     │
│  background.js ─── 命令路由 ───┬─ Background 直接处理               │
│                               │   navigate, screenshot, list_tabs, │
│                               │   switch_tab, new_tab, close_tab,  │
│                               │   get_cookies, execute_js          │
│                               │                                    │
│                               └─ Content Script 转发               │
│                                   click, type, hover, scroll,      │
│                                   select_option, get_content,      │
│                                   wait_for                         │
└─────────────────────────────────────────────────────────────────────┘
```

## Network Topology (Two-Machine Deployment)

```
192.168.44.2  (Hermes Agent)           192.168.199.11  (Browser Machine)
┌────────────────────────┐            ┌──────────────────────────────┐
│  hermes CLI            │            │  MCP Server (Go binary)      │
│  ~/.hermes/config.yaml │─ HTTP ────→│  0.0.0.0:19875              │
│                        │            │  127.0.0.1:19876 (WebSocket) │
└────────────────────────┘            │  Chrome + Extension          │
                                      └──────────────────────────────┘
```

**Ports:**

| Port | Bind | Purpose | Access |
|------|------|---------|--------|
| `19875` | `0.0.0.0` | MCP StreamableHTTP (Hermes → Server) | Remote |
| `19876` | `127.0.0.1` | WebSocket (Server → Chrome Extension) | Local only |

If Hermes and the browser are on the same machine, both ports can bind to `127.0.0.1`.

---

## Quick Start

### Prerequisites

- Go 1.23+
- Chrome or Chromium browser
- Hermes Agent (for MCP integration)

### 1. Build

```bash
git clone git@github.com:ety001/hermes-browser.git
cd hermes-browser
make build          # produces hermes-browser binary
```

### 2. Configure

Choose one token strategy:

**Option A — Fixed token (recommended for deployment):**
Edit `configs/deploy.yaml` and set `token` to a known value:
```yaml
token: "your-chosen-token"
```

**Option B — Auto-generated token:**
Leave `token: ""` in config. The server generates a 64-char hex token on first run,
saves it to `~/.hermes-browser/.token`, and prints it to stderr.

### 3. Start Server

```bash
./hermes-browser -c configs/deploy.yaml
```

Expected output:
```
WebSocket server listening on 127.0.0.1:19876
MCP HTTP server listening on 0.0.0.0:19875
```

### 4. Load Chrome Extension

1. Open `chrome://extensions/`
2. Enable **Developer mode** (top-right toggle)
3. Click **Load unpacked**
4. Select the `extension/` directory from this repo
5. The Hermes Browser icon appears in the toolbar

**After updating extension files:** always click the 🔄 reload button on `chrome://extensions/`.

### 5. Connect Extension to Server

1. Click the Hermes Browser icon in the Chrome toolbar
2. **WebSocket Address:** `ws://127.0.0.1:19876`
3. **Token:** (the token from step 2)
4. Click **Connect**
5. Status dot turns **green**

### 6. Configure Hermes

Add to `~/.hermes/config.yaml`:

```yaml
mcp_servers:
  hermes-browser:
    url: "http://<SERVER_IP>:19875/mcp"
    headers:
      Authorization: "Bearer <TOKEN>"
    timeout: 180
    connect_timeout: 60
```

Where:
- `<SERVER_IP>` — IP of the machine running MCP Server (`192.168.199.11` in the example topology)
- `<TOKEN>` — the token from step 2

**IP reference:**

| Machine | Role | IP |
|---------|------|----|
| Hermes Agent | AI agent | `192.168.44.2` |
| Browser machine | MCP Server + Chrome | `192.168.199.11` |

### 7. Verify

```bash
# Health check (no auth required)
curl http://<SERVER_IP>:19875/health
# → {"status":"ok","extension_connected":true}

# MCP initialize (get session ID)
SESSION_ID=$(curl -s -i -X POST http://<SERVER_IP>:19875/mcp \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}' \
  | grep -i "Mcp-Session-Id" | awk '{print $2}' | tr -d '\r')

# List all 15 tools
curl -s -X POST http://<SERVER_IP>:19875/mcp \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'

# Call a tool
curl -s -X POST http://<SERVER_IP>:19875/mcp \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"list_tabs","arguments":{}}}'

# Navigate to a page
curl -s -X POST http://<SERVER_IP>:19875/mcp \
  -H "Authorization: Bearer <TOKEN>" \
  -H "Content-Type: application/json" \
  -H "Mcp-Session-Id: $SESSION_ID" \
  -d '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"navigate","arguments":{"url":"https://example.com"}}}'
```

---

## MCP Tools (15)

All tools are registered as `<name>`. Hermes automatically prefixes them with `mcp_browser_`.

### Tab Management

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `list_tabs` | (none) | `[{id, url, title, active}]` | List all open tabs |
| `switch_tab` | `tab_id: number` (required) | `{switched, url, title}` | Switch to a tab by ID |
| `new_tab` | `url?: string` | `{tab_id, url, title}` | Open a new tab |
| `close_tab` | `tab_id?: number` | `{closed}` | Close a tab (omits = current) |

### Navigation & Screenshot

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `navigate` | `url` (req), `wait_until?`, `timeout?` | `{url, title}` | Navigate and wait for load |
| `screenshot` | `format?` (jpeg\|png), `quality?`, `selector?` | ImageContent | Capture visible viewport or element |

**wait_until options:** `"domcontentloaded"`, `"load"`, `"networkidle"` (default, adds 500ms after load)

### Page Interaction (Content Script)

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `click` | `selector` (req), `timeout?` | `{clicked, tag, text}` | Click element matching CSS selector |
| `type` | `selector` (req), `text` (req), `clear_first?`, `press_enter?` | `{typed, length}` | Type text into input/textarea/contenteditable |
| `hover` | `selector` (req) | `{hovered, tag, text}` | Hover over element |
| `scroll` | `direction?` (up\|down), `amount?` | `{scrolled, scroll_y, scroll_height}` | Scroll page |
| `select_option` | `selector` (req), `value` (req) | `{selected, value}` | Select dropdown option |

**`type` validation:** Only works on `<input>` (type: text/email/password/search/tel/url/number), `<textarea>`, and `contenteditable` elements. Non-input elements return `ELEMENT_NOT_FOUND`.

### Content & Scripting

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `get_content` | `selector?`, `type?` (text\|markdown\|html) | string | Extract page content |
| `execute_js` | `expression` (req), `return_value?` | `{value, type}` | Execute JS in MAIN world (bypasses CSP) |
| `wait_for` | `selector` (req), `state?`, `timeout?` | `{found, elapsed}` | Wait for element state |

**`execute_js` notes:**
- Injected into the page's MAIN world via `chrome.scripting.executeScript` — bypasses page CSP
- Supports `await` and Promise-based code
- With `return_value: true`, the expression's result is returned
- For multi-statement code, use `return_value: false` or ensure the last statement is an expression

**`wait_for` states:** `"visible"` (default, checks offsetParent), `"hidden"`, `"attached"`, `"detached"`

### Cookies

| Tool | Parameters | Returns | Description |
|------|-----------|---------|-------------|
| `get_cookies` | `url?`, `name?` | `[{name, value, domain, path, secure, httpOnly}]` | Get cookies for current page |

### Error Codes

| Code | Meaning | Suggested Action |
|------|---------|-----------------|
| `ELEMENT_NOT_FOUND` | CSS selector matched nothing | Check selector, use `wait_for` first |
| `TIMEOUT` | Operation timed out | Increase timeout, check page state |
| `NAVIGATION_ERROR` | Page failed to load | Check URL, network, login status |
| `JS_EXECUTION_ERROR` | JavaScript threw | Fix the expression syntax |
| `TAB_NOT_FOUND` | Tab ID doesn't exist | Run `list_tabs` to refresh |
| `TAB_CLOSED` | Tab was closed | Use a different tab |
| `NO_EXTENSION_CONNECTED` | Chrome Extension disconnected | Check popup connection status |
| `PERMISSION_DENIED` | Tab not active or permission missing | `ensureTabActive` runs automatically |

---

## Development

### Run Tests

```bash
make test          # all tests
go test ./internal/mcp/ -v    # MCP server tests
go test ./internal/ws/ -v     # WebSocket tests
```

### Build Verification

```bash
make verify
```

This starts the server, runs health check, MCP initialize, and tools/list — all without a browser.

### Project Structure

```
├── cmd/server/main.go         # Entry point: config → hub → WS → MCP
├── internal/
│   ├── config/config.go        # YAML + env vars + -c flag + token auto-gen
│   ├── auth/auth.go            # Constant-time token compare, HTTP middleware
│   ├── ws/
│   │   ├── protocol.go         # Request/Response structs, error codes
│   │   ├── hub.go              # Single-client hub, request/response dispatch
│   │   ├── client.go           # WebSocket read loop, JSON send
│   │   └── server.go           # WS upgrade, token auth, origin check
│   └── mcp/
│       ├── server.go           # MCPServer, 15 tool registration
│       └── tools.go            # Tool definitions + handlers
├── extension/
│   ├── manifest.json           # Manifest V3, permissions, host_permissions
│   ├── background.js           # Service Worker: WS connection, command dispatch
│   ├── content.js              # DOM operations, content extraction, markdown
│   ├── icons/                  # 16/48/128 PNG icons
│   └── popup/                  # Connection UI, log display
├── configs/
│   ├── config.yaml             # Development config (auto-generated token)
│   └── deploy.yaml             # Deployment config (fixed token)
├── Makefile                    # build, run, test, verify, clean
├── DEPLOY.md                   # Detailed deployment guide
└── README.md                   # This file
```

### Updating Extension Files

Extension files are served from disk by Chrome. After SCP'ing new files:

```bash
scp extension/*.js <target>:/home/ety001/hermes-browser/extension/
```

Then **reload the extension** on `chrome://extensions/` (click 🔄).

### Updating the Go Binary

```bash
make build
scp hermes-browser <target>:/home/ety001/hermes-browser/
ssh <target> "kill \$(pgrep hermes-browser) && \
  nohup /home/ety001/hermes-browser/hermes-browser \
  -c /home/ety001/hermes-browser/configs/deploy.yaml \
  > /tmp/hermes-browser.log 2>&1 &"
```

---

## Troubleshooting

### `extension_connected: false` in health check

1. Chrome Extension not loaded → open `chrome://extensions/`, check it's listed
2. Token mismatch → verify token in popup matches `deploy.yaml`
3. Extension needs reload → click 🔄 on `chrome://extensions/`
4. Wrong WebSocket address → popup should use `ws://127.0.0.1:19876`

### `execute_js` fails with CSP error

This means the code is still running in the content script's isolated world.
**Reload the extension** on `chrome://extensions/` — the fix uses `world: 'MAIN'`
injection which bypasses page CSP entirely.

### `type` returns success but nothing was typed

The element might not be a text input. Check:
- Is it an `<input>` with a valid type (text, email, password, search, tel, url, number)?
- Is it a `<textarea>`?
- Does it have `contenteditable` attribute?

### WebSocket connection fails

1. Check server is running: `curl http://127.0.0.1:19875/health`
2. Check WebSocket port: `ss -tlnp | grep 19876`
3. Verify token in popup matches config
4. Look for errors in Chrome extension console (right-click popup → Inspect → Console)

### Auto-generated token location

```bash
cat ~/.hermes-browser/.token
```

Used when `token:` is empty in config.
