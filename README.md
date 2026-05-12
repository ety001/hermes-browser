# Hermes Browser

MCP bridge for Hermes Agent to control a real Chrome browser via Chrome Extension.

## Architecture

```
Hermes Agent ─── MCP StreamableHTTP ─── MCP Server (Go) ─── WebSocket ─── Chrome Extension
                                                                              │
                                                                        ┌─────┴──────┐
                                                      Background direct     Content Script
                                                   (navigate, screenshot,   (click, type,
                                                    list_tabs, cookies)     get_content, JS)
```

## Quick Start

### 1. Start MCP Server

```bash
# Build
make build

# Run with default config (listens on 0.0.0.0:19875 for MCP, 127.0.0.1:19876 for WS)
./hermes-browser -c configs/config.yaml
```

On first run, a token is auto-generated and saved to `~/.hermes-browser/.token`.

### 2. Load Chrome Extension

1. Open `chrome://extensions/`
2. Enable "Developer mode"
3. Click "Load unpacked"
4. Select the `extension/` directory

### 3. Connect

1. Click the Hermes Browser icon in Chrome toolbar
2. Enter WebSocket address (`ws://127.0.0.1:19876`)
3. Enter Token (from ~/.hermes-browser/.token)
4. Click "Connect" — status dot turns green

### 4. Verify with Hermes

```yaml
# ~/.hermes/config.yaml
mcp_servers:
  hermes-browser:
    url: "http://<mcp-server-host>:19875/mcp"
    headers:
      Authorization: "Bearer ${MCP_BROWSER_TOKEN}"
    timeout: 180
```

```bash
export MCP_BROWSER_TOKEN="$(cat ~/.hermes-browser/.token)"
```

### 5. Direct Verification

```bash
# Health check
curl http://127.0.0.1:19875/health

# Tool listing (MCP initialize)
curl -X POST http://127.0.0.1:19875/mcp \
  -H "Authorization: Bearer $(cat ~/.hermes-browser/.token)" \
  -H "Content-Type: application/json" \
  -d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","capabilities":{},"clientInfo":{"name":"test","version":"1.0"}}}'
```

## MCP Tools (15)

| Tool | Category | Handler |
|------|----------|---------|
| navigate | Navigation | Background (chrome.tabs.update + onUpdated) |
| screenshot | Screenshot | Background (captureVisibleTab) |
| list_tabs | Tab Management | Background (chrome.tabs.query) |
| switch_tab | Tab Management | Background |
| new_tab | Tab Management | Background |
| close_tab | Tab Management | Background |
| get_cookies | Cookies | Background (chrome.cookies.getAll) |
| click | Page Interaction | Content Script |
| type | Page Interaction | Content Script |
| hover | Page Interaction | Content Script |
| scroll | Page Interaction | Content Script |
| select_option | Page Interaction | Content Script |
| get_content | Content Extraction | Content Script |
| execute_js | JavaScript | Content Script (async IIFE) |
| wait_for | Waiting | Content Script (requestAnimationFrame poll) |

## Project Structure

```
├── cmd/server/main.go         # Entry point
├── internal/
│   ├── config/config.go        # YAML + env + CLI config
│   ├── auth/auth.go            # Token validation, HTTP middleware
│   ├── ws/                     # WebSocket protocol, hub, client, server
│   └── mcp/                    # MCP server, 15 tool handlers
├── extension/
│   ├── manifest.json           # Manifest V3
│   ├── background.js           # Service Worker: WS + command dispatch
│   ├── content.js              # DOM operations
│   └── popup/                  # Connection UI
└── configs/config.yaml         # Default configuration
```
