# Hermes Browser

MCP bridge for Hermes Agent to control a real Chrome browser through a Chrome Extension.

## Architecture

```
┌─ Hermes Agent ───────────────────────────────────────┐
│  ~/.hermes/config.yaml:                              │
│    mcp_servers.hermes-browser.url → ...:19875/mcp    │
│    headers: Authorization: Bearer <token>            │
└──────────────────────┬───────────────────────────────┘
                       │ MCP StreamableHTTP (JSON-RPC 2.0)
                       ▼
┌─ MCP Server (Go binary) ────────────────────────────┐
│  Port 19875: HTTP transport (for Hermes)            │
│  Port 19876: WebSocket transport (for Extension)    │
│  Single token authenticates both transports         │
└──────────────────────┬──────────────────────────────┘
                       │ WebSocket (ws://...:19876?token=...)
                       ▼
┌─ Chrome Extension (Manifest V3) ────────────────────┐
│  background.js: command dispatcher                  │
│    ├─ Direct: navigate, screenshot, list_tabs,      │
│    │  switch_tab, new_tab, close_tab,               │
│    │  get_cookies, execute_js                       │
│    └─ Content Script: click, type, hover, scroll,   │
│       select_option, get_content, wait_for          │
└─────────────────────────────────────────────────────┘
```

## Deployment Topologies

### Scenario A — Same machine

Hermes Agent and Chrome browser run on the same computer.
Both ports bind to `127.0.0.1`.

```
localhost
├── hermes
├── MCP Server (127.0.0.1:19875, 127.0.0.1:19876)
└── Chrome + Extension
```

### Scenario B — Cross-machine (Hermes remote, browser local)

```
Hermes machine                    Browser machine
┌──────────────┐                  ┌──────────────────────┐
│ hermes       │── HTTP :19875──→ │ MCP Server (0.0.0.0) │
│              │                  │ Chrome + Extension   │
└──────────────┘                  │ WS :19876 (127.0.0.1)│
                                  └──────────────────────┘
```

| Port | Default bind | Purpose |
|------|-------------|---------|
| `19875` | `0.0.0.0` (cross‑machine) / `127.0.0.1` (same‑machine) | MCP StreamableHTTP — Hermes connects here |
| `19876` | `127.0.0.1` always | WebSocket — Extension connects here |

---

## Quick Start

### Choose your binary source

**Option 1 — Download pre-built binary (recommended)**

Go to the [Releases page](https://github.com/ety001/hermes-browser/releases) and download the correct binary for your browser machine.

> ⚠️ **AI Agent note:** Before downloading, check the target machine's platform:
> ```bash
> # On the machine that will run the browser:
> uname -s   # → Linux / Darwin
> uname -m   # → x86_64 / aarch64 / arm64
> ```
> Then pick the matching `hermes-browser-<os>-<arch>` file.
>
> Download table:
> | `uname -s` | `uname -m` | Download |
> |---|---|---|
> | Linux | x86_64 | `hermes-browser-linux-amd64` |
> | Linux | aarch64 | `hermes-browser-linux-arm64` |
> | Darwin | x86_64 | `hermes-browser-darwin-amd64` |
> | Darwin | arm64 | `hermes-browser-darwin-arm64` |

**Option 2 — Build from source**

```bash
git clone https://github.com/ety001/hermes-browser.git
cd hermes-browser
make build
```

### File placement on browser machine

Place the files on the machine that runs Chrome:

```
~/.hermes-browser/           # config and token directory
├── config.yaml              # server configuration
└── .token                   # auto-generated token (if token:"")

~/hermes-browser/             # or any directory you choose
├── hermes-browser            # the compiled binary
└── extension/                # Chrome Extension directory
    ├── manifest.json
    ├── background.js
    ├── content.js
    ├── icons/
    └── popup/
```

### 1. Configure

Create `~/.hermes-browser/config.yaml` (or use `configs/deploy.yaml` from the repo):

```yaml
token: "your-chosen-token"

server:
  http:
    bind: "0.0.0.0:19875"        # use "127.0.0.1" if Hermes is local

websocket:
  bind: "127.0.0.1:19876"

browser:
  default_wait_until: "networkidle"
  default_timeout: 30000
  screenshot_format: "jpeg"
  screenshot_quality: 80
  max_content_length: 500000

logging:
  level: "info"
```

**Token strategies:**
- **Fixed token:** set `token:` to a value you choose — same token works for both HTTP and WebSocket
- **Auto-generated:** leave `token: ""`. On first start, a 64-char hex token is generated and saved to `~/.hermes-browser/.token`

### 2. Start server (on browser machine)

```bash
# If downloaded from Release:
chmod +x hermes-browser-linux-amd64
./hermes-browser-linux-amd64 -c ~/.hermes-browser/config.yaml

# If built from source:
./hermes-browser -c configs/deploy.yaml
```

Expected output:
```
WebSocket server listening on 127.0.0.1:19876
MCP HTTP server listening on 0.0.0.0:19875
```

### 3. Load Chrome Extension (on browser machine)

1. Open `chrome://extensions/`
2. Enable **Developer mode** (top‑right toggle)
3. Click **Load unpacked**
4. Select the `extension/` directory
5. The Hermes Browser icon appears in the toolbar

> After updating extension files: always click the 🔄 reload button on `chrome://extensions/`.

### 4. Connect Extension to Server

1. Click the Hermes Browser icon in the Chrome toolbar
2. **WebSocket address:** `ws://127.0.0.1:19876`
3. **Token:** the same token from your config
4. Click **Connect**
5. Status dot turns **green**

### 5. Configure Hermes

Run this on the machine where Hermes is installed:

```bash
# Replace SERVER_IP with the browser machine's IP (or 127.0.0.1 if same machine)
# Replace TOKEN with the token from step 1
hermes mcp add hermes-browser \
  --url "http://<SERVER_IP>:19875/mcp" \
  --auth header
```

When prompted, enter:
- Header name: `Authorization`
- Header value: `Bearer <TOKEN>`

**Or**, manually add to `~/.hermes/config.yaml`:

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
- `<SERVER_IP>` — the IP address of the machine running the MCP Server and Chrome
- `<TOKEN>` — the token from the config file on the browser machine

### 6. Verify

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
```

---

## MCP Tools (15)

All registered as `<name>`. Hermes automatically prefixes with `mcp_browser_`.

### Tab Management

| Tool | Parameters | Returns |
|------|-----------|---------|
| `list_tabs` | (none) | `[{id, url, title, active}]` |
| `switch_tab` | `tab_id` (req) | `{switched, url, title}` |
| `new_tab` | `url?` | `{tab_id, url, title}` |
| `close_tab` | `tab_id?` | `{closed}` |

### Navigation & Screenshot

| Tool | Parameters | Returns |
|------|-----------|---------|
| `navigate` | `url` (req), `wait_until?` (`domcontentloaded`\|`load`\|`networkidle`), `timeout?` | `{url, title}` |
| `screenshot` | `format?` (jpeg\|png), `quality?`, `selector?` | ImageContent |

### Page Interaction (Content Script)

| Tool | Parameters | Returns |
|------|-----------|---------|
| `click` | `selector` (req), `timeout?` | `{clicked, tag, text}` |
| `type` | `selector` (req), `text` (req), `clear_first?`, `press_enter?` | `{typed, length}` |
| `hover` | `selector` (req) | `{hovered, tag, text}` |
| `scroll` | `direction?` (up\|down), `amount?` (one_page\|half_page\|`<N>px`) | `{scrolled, scroll_y, scroll_height}` |
| `select_option` | `selector` (req), `value` (req) | `{selected, value}` |

> **`type` validation:** Only works on `<input>` (type: text/email/password/search/tel/url/number), `<textarea>`, and `contenteditable` elements.

### Content & Scripting

| Tool | Parameters | Returns |
|------|-----------|---------|
| `get_content` | `selector?`, `type?` (text\|markdown\|html) | string |
| `execute_js` | `expression` (req), `return_value?` | `{value, type}` |
| `wait_for` | `selector` (req), `state?` (visible\|hidden\|attached\|detached), `timeout?` | `{found, elapsed}` |

> **`execute_js`:**
> - Runs in the page's MAIN world via `chrome.scripting.executeScript` — bypasses CSP
> - Supports `await` and Promise-based code
> - For multi-statement code, use `return_value: false`

### Cookies

| Tool | Parameters | Returns |
|------|-----------|---------|
| `get_cookies` | `url?`, `name?` | `[{name, value, domain, path, secure, httpOnly}]` |

### Error Codes

| Code | Meaning | Suggested Action |
|------|---------|-----------------|
| `ELEMENT_NOT_FOUND` | CSS selector not matched | Check selector, use `wait_for` first |
| `TIMEOUT` | Operation timed out | Increase timeout, check page state |
| `NAVIGATION_ERROR` | Page navigation failed | Check URL, network, login |
| `JS_EXECUTION_ERROR` | JavaScript threw | Fix expression syntax |
| `TAB_NOT_FOUND` | Tab ID doesn't exist | Run `list_tabs` to refresh |
| `TAB_CLOSED` | Tab was closed | Use a different tab |
| `NO_EXTENSION_CONNECTED` | Extension disconnected | Check popup connection status |
| `PERMISSION_DENIED` | Tab not active | `ensureTabActive` runs automatically |

---

## Development

### Prerequisites

- Go 1.23+
- Chrome / Chromium

### Commands

```bash
make build      # compile Go binary
make test       # run all tests (config, auth, ws, mcp)
make verify     # start server + health check + MCP initialize + listTools
make clean      # remove binary
```

### Project Structure

```
├── .github/workflows/ci.yml
├── cmd/server/main.go            # Entry point
├── internal/
│   ├── config/config.go           # YAML + env + CLI config, token auto-gen
│   ├── auth/auth.go               # Token validation, HTTP middleware
│   ├── ws/                        # WebSocket protocol, hub, client, server
│   └── mcp/                       # MCP server, 15 tool handlers
├── extension/                     # Chrome Extension (Manifest V3)
│   ├── manifest.json
│   ├── background.js              # Service Worker
│   ├── content.js                 # DOM operations
│   └── popup/                     # Connection UI
├── configs/
│   ├── config.yaml                # Dev config (auto-generated token)
│   └── deploy.yaml                # Deployment config (fixed token)
├── Makefile
└── README.md
```

---

## Troubleshooting

### `extension_connected: false`

1. Extension loaded? → check `chrome://extensions/`
2. Token mismatch? → popup token must match config token
3. Reload needed? → click 🔄 on `chrome://extensions/`
4. Wrong WS address? → popup should use `ws://127.0.0.1:19876`

### `execute_js` CSP error

Reload the extension on `chrome://extensions/` — the fix uses `world: 'MAIN'` injection.

### WebSocket connection fails

```bash
curl http://127.0.0.1:19875/health     # server running?
ss -tlnp | grep 19876                   # WebSocket port listening?
```

### Auto-generated token

```bash
cat ~/.hermes-browser/.token
```

---

## License

MIT
