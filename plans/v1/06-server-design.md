# Go MCP Server 详细设计

## 程序入口

```go
// cmd/server/main.go
package main

import (
    "context"
    "flag"
    "fmt"
    "os"
    "os/signal"
    "syscall"

    "github.com/ety001/hermes-browser/internal/config"
    "github.com/ety001/hermes-browser/internal/mcp"
    "github.com/ety001/hermes-browser/internal/ws"
)

func main() {
    configPath := flag.String("c", "", "config file path")
    flag.Parse()

    // 1. 加载配置
    cfg, err := config.Load(*configPath)
    if err != nil {
        fmt.Fprintf(os.Stderr, "failed to load config: %v\n", err)
        os.Exit(1)
    }

    // 2. 创建 WebSocket Hub
    hub := ws.NewHub(cfg.WebSocket)

    // 3. 创建 MCP Server
    mcpServer := mcp.NewServer(cfg, hub)

    // 4. 启动 WebSocket Server
    wsServer := ws.NewServer(cfg.WebSocket, hub)

    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    // 启动 WebSocket 监听
    go func() {
        if err := wsServer.ListenAndServe(); err != nil {
            fmt.Fprintf(os.Stderr, "websocket server error: %v\n", err)
            cancel()
        }
    }()

    // 5. 启动 MCP Server (stdio 或 HTTP)
    if cfg.Server.HTTP.Bind != "" {
        go func() {
            // HTTP transport
            httpServer := server.NewStreamableHTTPServer(mcpServer)
            if err := httpServer.Start(cfg.Server.HTTP.Bind); err != nil {
                fmt.Fprintf(os.Stderr, "mcp http server error: %v\n", err)
                cancel()
            }
        }()
    } else {
        go func() {
            // stdio transport
            if err := server.ServeStdio(mcpServer); err != nil {
                fmt.Fprintf(os.Stderr, "mcp stdio error: %v\n", err)
                cancel()
            }
        }()
    }

    // 6. 打印启动信息到 stderr
    fmt.Fprintf(os.Stderr, "hermes-browser server started\n")
    fmt.Fprintf(os.Stderr, "  WebSocket: %s\n", cfg.WebSocket.Bind)
    fmt.Fprintf(os.Stderr, "  Token: %s\n", maskToken(cfg.WebSocket.Token))
    if cfg.Server.HTTP.Bind != "" {
        fmt.Fprintf(os.Stderr, "  MCP HTTP: %s\n", cfg.Server.HTTP.Bind)
    }

    // 7. 等待信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    fmt.Fprintf(os.Stderr, "shutting down...\n")
    cancel()
    hub.Shutdown()
}
```

## MCP Server 实现

```go
// internal/mcp/server.go
package mcp

import (
    "context"
    "time"

    "github.com/ety001/hermes-browser/internal/config"
    "github.com/ety001/hermes-browser/internal/ws"
    "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

type MCPServer struct {
    server  *server.MCPServer
    hub     *ws.Hub
    config  *config.Config
}

func NewServer(cfg *config.Config, hub *ws.Hub) *server.MCPServer {
    s := server.NewMCPServer(
        "hermes-browser",
        "1.0.0",
        server.WithToolCapabilities(true),
    )

    // 注册所有工具
    registerTools(s, hub, cfg)

    return s
}
```

## 工具 Handler 模式

所有工具 handler 遵循统一模式：
1. 从 MCP request 中提取参数
2. 构造 WebSocket 命令
3. 通过 Hub 发送到 Chrome Extension
4. 等待响应（带超时）
5. 将响应转换为 MCP CallToolResult

```go
// internal/mcp/tools.go

// 通用调用函数：发送命令到 Extension 并等待响应
func callExtension(
    ctx context.Context,
    hub *ws.Hub,
    method string,
    params map[string]any,
    tabID int,
    timeout time.Duration,
) (map[string]any, error) {

    requestID := generateUUID()
    command := ws.Request{
        ID:      requestID,
        Method:  method,
        Params:  params,
        TabID:   tabID,
    }

    // 注册响应等待器
    responseCh := make(chan ws.Response, 1)
    hub.RegisterResponseHandler(requestID, responseCh)
    defer hub.UnregisterResponseHandler(requestID)

    // 发送命令
    if err := hub.SendCommand(tabID, command); err != nil {
        return nil, fmt.Errorf("failed to send command: %w", err)
    }

    // 等待响应
    select {
    case resp := <-responseCh:
        if resp.Status == "error" {
            return nil, fmt.Errorf("%s", resp.Error)
        }
        return resp.Data, nil
    case <-time.After(timeout):
        return nil, fmt.Errorf("timeout after %v", timeout)
    case <-ctx.Done():
        return nil, ctx.Err()
    }
}

// navigate handler
func handleNavigate(ctx context.Context, request mcp.CallToolRequest, hub *ws.Hub, cfg *config.Config) (*mcp.CallToolResult, error) {
    url := request.GetString("url", "")
    waitUntil := request.GetString("wait_until", cfg.Browser.DefaultWaitUntil)
    timeout := time.Duration(request.GetNumber("timeout", float64(cfg.Browser.DefaultTimeout))) * time.Millisecond

    data, err := callExtension(ctx, hub, "navigate", map[string]any{
        "url":        url,
        "wait_until": waitUntil,
    }, 0, timeout)

    if err != nil {
        return mcp.NewToolResultError(err.Error()), nil
    }

    return &mcp.CallToolResult{
        Content: []mcp.Content{
            mcp.TextContent{
                Type: "text",
                Text: fmt.Sprintf("Navigated to %s: %s", data["url"], data["title"]),
            },
        },
    }, nil
}

// ... 其他工具 handler 类似结构
```

## WebSocket Hub 设计

```go
// internal/ws/hub.go
package ws

import (
    "sync"
    "time"
)

type Hub struct {
    mu          sync.RWMutex
    clients     map[*Client]struct{}
    // tabID -> client 映射
    tabClients  map[int]*Client
    // requestID -> response channel 映射
    responses   map[string]chan Response
    config      WebSocketConfig
    broadcastCh chan []byte
    registerCh  chan *Client
    unregisterCh chan *Client
    done        chan struct{}
}

func NewHub(cfg WebSocketConfig) *Hub {
    return &Hub{
        clients:      make(map[*Client]struct{}),
        tabClients:   make(map[int]*Client),
        responses:    make(map[string]chan Response),
        config:       cfg,
        broadcastCh:  make(chan []byte, 256),
        registerCh:   make(chan *Client),
        unregisterCh: make(chan *Client),
        done:         make(chan struct{}),
    }
}

func (h *Hub) Run() {
    // 主循环，处理注册/注销/广播
    for {
        select {
        case client := <-h.registerCh:
            h.mu.Lock()
            h.clients[client] = struct{}{}
            h.mu.Unlock()
        case client := <-h.unregisterCh:
            h.mu.Lock()
            delete(h.clients, client)
            // 清理 tabClients 映射
            for tabID, c := range h.tabClients {
                if c == client {
                    delete(h.tabClients, tabID)
                }
            }
            h.mu.Unlock()
            client.Close()
        case msg := <-h.broadcastCh:
            h.mu.RLock()
            for client := range h.clients {
                client.Send(msg)
            }
            h.mu.RUnlock()
        case <-h.done:
            return
        }
    }
}

// SendCommand 发送命令到指定 tab 的 Extension
func (h *Hub) SendCommand(tabID int, req Request) error {
    h.mu.RLock()
    client := h.tabClients[tabID]
    h.mu.RUnlock()

    if client == nil {
        return fmt.Errorf("no client connected for tab %d", tabID)
    }

    data, _ := json.Marshal(req)
    return client.Send(data)
}

// HandleResponse 处理从 Extension 收到的响应
func (h *Hub) HandleResponse(resp Response) {
    h.mu.RLock()
    ch, ok := h.responses[resp.ID]
    h.mu.RUnlock()

    if ok {
        ch <- resp
    }
}
```

## WebSocket Server

```go
// internal/ws/server.go
package ws

import (
    "net/http"
    "strings"

    "github.com/gorilla/websocket"
)

type WSServer struct {
    addr     string
    hub      *Hub
    upgrader websocket.Upgrader
    config   WebSocketConfig
    auth     *auth.Authenticator
}

func NewServer(cfg WebSocketConfig, hub *Hub) *WSServer {
    return &WSServer{
        addr:   cfg.Bind,
        hub:    hub,
        config: cfg,
        upgrader: websocket.Upgrader{
            CheckOrigin: func(r *http.Request) bool {
                // 检查 Origin 是否在白名单
                origin := r.Header.Get("Origin")
                if len(cfg.AllowedExtensions) == 0 {
                    return true // 开发模式，不检查
                }
                for _, extID := range cfg.AllowedExtensions {
                    if strings.HasPrefix(origin, "chrome-extension://"+extID) {
                        return true
                    }
                }
                return false
            },
        },
    }
}

func (s *WSServer) ServeHTTP(w http.ResponseWriter, r *http.Request) {
    // 1. Token 认证（通过 query param）
    token := r.URL.Query().Get("token")
    if !s.auth.Validate(token) {
        http.Error(w, "unauthorized", http.StatusUnauthorized)
        return
    }

    // 2. WebSocket 升级
    conn, err := s.upgrader.Upgrade(w, r, nil)
    if err != nil {
        return
    }

    // 3. 创建 Client 并注册到 Hub
    client := NewClient(conn, s.hub)
    s.hub.Register(client)

    go client.ReadPump()
    go client.WritePump()
}

func (s *WSServer) ListenAndServe() error {
    return http.ListenAndServe(s.addr, s)
}
```

## HTTP Transport 认证

如果使用 HTTP transport（跨机器场景），需要在 MCP HTTP 层面也做认证。

mcp-go 的 StreamableHTTPServer 支持 middleware，可以注入 token 验证：

```go
httpServer := server.NewStreamableHTTPServer(mcpServer,
    server.WithHTTPMiddleware(func(next http.Handler) http.Handler {
        return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            token := r.Header.Get("Authorization")
            if token == "" || !auth.Validate(strings.TrimPrefix(token, "Bearer ")) {
                http.Error(w, "unauthorized", http.StatusUnauthorized)
                return
            }
            next.ServeHTTP(w, r)
        })
    }),
)
```
