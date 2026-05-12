# Go MCP Server 详细设计（v4）

> 修订记录：v4 修正 glm5turbo-v3 审计 P0#1(删除 WithEndpointPath)、P0#3(execute_js 参数名统一)、
> P1#4(navigate timeout 传递)、P2#7(错误码汇总补全)、P2#8(Config 结构体定义)、P2#9(screenshot 默认值读配置)；
> v3 修正 P0#2(去掉 WithToolHandler)、P0#4(参数访问用 GetString)、
> P0#5(ws 引用 auth)、P1#6(超时从 config 读取)、P1#7(Hub 共享、main.go 桥接)、P2#13(确认 NewToolResultImage 签名)；

## 入口 main.go

```go
package main

import (
    "context"
    "fmt"
    "net/http"
    "os"
    "os/signal"
    "syscall"
    "time"

    "github.com/ety001/hermes-browser/internal/config"
    "github.com/ety001/hermes-browser/internal/mcp"
    "github.com/ety001/hermes-browser/internal/ws"
)

func main() {
    // 1. 加载配置
    cfg, err := config.Load()
    if err != nil {
        fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
        os.Exit(1)
    }

    // 2. stdio 模式检测
    if cfg.Server.HTTP.Bind == "" && !isStdinPipe() {
        fmt.Fprintf(os.Stderr, `WARNING: No HTTP bind configured and stdin is not a pipe.
The server will wait for MCP stdio input. If you meant to start
as a standalone server, configure server.http.bind in config.yaml.
`)
    }

    // 3. 创建 Hub（WebSocket 和 MCP Server 共享）
    hub := ws.NewHub(cfg)

    // 4. 启动 WebSocket Server（如果配置了）
    var wsSrv *http.Server
    if cfg.WebSocket.Bind != "" {
        wsSrv = ws.StartServer(context.Background(), cfg, hub)
    }

    // 5. 创建 MCP Server（传入共享的 hub）
    mcpServer := mcp.NewServer(cfg, hub)

    // 6. 启动 MCP transport
    var httpSrv *http.Server
    if cfg.Server.HTTP.Bind != "" {
        httpSrv = startHTTPServer(cfg, mcpServer)
    } else {
        startStdioServer(mcpServer)
    }

    // 7. 等待信号
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
    <-sigCh

    // 8. 优雅关闭
    fmt.Fprintf(os.Stderr, "Shutting down...\n")
    shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()

    if httpSrv != nil {
        httpSrv.Shutdown(shutdownCtx)
    }
    if wsSrv != nil {
        wsSrv.Shutdown(shutdownCtx)
    }
}

func isStdinPipe() bool {
    fi, _ := os.Stdin.Stat()
    return fi.Mode()&os.ModeCharDevice == 0
}
```

## HTTP Server 启动（v2 重写）

核心变更：不调用 `StreamableHTTPServer.Start()`，自己建 `http.ServeMux` 包装。

```go
func startHTTPServer(cfg *config.Config, mcpServer *mcp.Server) *http.Server {
    // 创建 StreamableHTTPServer（P0#1: 不使用 WithEndpointPath，
    // 因为它仅在 Start() 中生效；作为 http.Handler 时路径由 mux 控制）
    sseServer := server.NewStreamableHTTPServer(
        mcpServer.MCPServer(),
    )

    // 创建自定义 mux，包装 auth 中间件
    mux := http.NewServeMux()
    mux.Handle("/mcp", authMiddleware(sseServer, cfg))

    // 健康检查端点（不需要认证）
    mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Content-Type", "application/json")
        w.WriteHeader(http.StatusOK)
        fmt.Fprintf(w, `{"status":"ok","extension_connected":%v}`, mcpServer.IsExtensionConnected())
    })

    httpSrv := &http.Server{
        Addr:    cfg.Server.HTTP.Bind,
        Handler: mux,
    }

    go func() {
        fmt.Fprintf(os.Stderr, "MCP HTTP server listening on %s\n", cfg.Server.HTTP.Bind)
        if err := httpSrv.ListenAndServe(); err != http.ErrServerClosed {
            fmt.Fprintf(os.Stderr, "HTTP server error: %v\n", err)
        }
    }()

    return httpSrv
}

// authMiddleware 校验 Bearer token（位于 internal/auth/auth.go）
// 此处展示调用方式，实际实现在 auth 包
func authMiddleware(next http.Handler, cfg *config.Config) http.Handler {
    return auth.HTTPMiddleware(next, cfg.GetHTTPToken())
}
```

**注意：**
- `StreamableHTTPServer` 实现了 `http.Handler` 接口，可以直接传给 `mux.Handle`
- 不能调用 `sseServer.Start(addr)`，因为它内部自己建 mux 和 http.Server
- `authMiddleware` 实际实现在 `internal/auth/auth.go`，返回标准 `http.Handler` 中间件

## MCP Server 定义

```go
// internal/mcp/server.go
package mcp

import (
    "context"
    "fmt"

    "github.com/ety001/hermes-browser/internal/config"
    "github.com/ety001/hermes-browser/internal/ws"
    mcplib "github.com/mark3labs/mcp-go/mcp"
    "github.com/mark3labs/mcp-go/server"
)

type Server struct {
    cfg  *config.Config
    hub  *ws.Hub
    mcpS *server.MCPServer
}

func NewServer(cfg *config.Config, hub *ws.Hub) *Server {
    s := &Server{
        cfg: cfg,
        hub: hub,
    }

    // 创建 MCP Server（v3: 不使用 WithToolHandler，逐个 AddTool 注册）
    s.mcpS = server.NewMCPServer(
        "hermes-browser",
        "1.0.0",
    )

    // 注册工具
    s.registerTools()

    return s
}

func (s *Server) MCPServer() *server.MCPServer {
    return s.mcpS
}

func (s *Server) IsExtensionConnected() bool {
    return s.hub.HasClient()
}

func (s *Server) registerTools() {
    // 逐个注册，每个工具单独指定 handler
    s.mcpS.AddTool(navigateTool, s.handleNavigate)
    s.mcpS.AddTool(screenshotTool, s.handleScreenshot)
    s.mcpS.AddTool(getContentTool, s.handleGetContent)
    s.mcpS.AddTool(clickTool, s.handleClick)
    s.mcpS.AddTool(typeTool, s.handleType)
    s.mcpS.AddTool(scrollTool, s.handleScroll)
    s.mcpS.AddTool(executeJsTool, s.handleExecuteJs)
    s.mcpS.AddTool(waitForTool, s.handleWaitFor)
    s.mcpS.AddTool(getCookiesTool, s.handleGetCookies)
    s.mcpS.AddTool(listTabsTool, s.handleListTabs)
    s.mcpS.AddTool(switchTabTool, s.handleSwitchTab)
    s.mcpS.AddTool(closeTabTool, s.handleCloseTab)
    s.mcpS.AddTool(newTabTool, s.handleNewTab)
    s.mcpS.AddTool(hoverTool, s.handleHover)
    s.mcpS.AddTool(selectOptionTool, s.handleSelectOption)
}
```

## WebSocket Hub（v2 简化为单 client）

```go
// internal/ws/hub.go
package ws

import (
    "sync"
    "time"

    "github.com/ety001/hermes-browser/internal/config"
    "github.com/google/uuid"
)

type Hub struct {
    mu        sync.RWMutex
    client    *Client        // 单一 Extension 客户端
    responses map[string]chan Response  // requestID -> response channel
    timeout   time.Duration
}

func NewHub(cfg *config.Config) *Hub {
    // P1#6: 超时从配置读取，加上网络传输余量
    timeout := time.Duration(cfg.Browser.DefaultTimeout) * time.Millisecond
    if timeout < 10*time.Second {
        timeout = 60 * time.Second  // 安全下限
    }
    // 在默认超时基础上加 10s 作为网络传输余量
    timeout += 10 * time.Second

    return &Hub{
        responses: make(map[string]chan Response),
        timeout:   timeout,
    }
}

func (h *Hub) HasClient() bool {
    h.mu.RLock()
    defer h.mu.RUnlock()
    return h.client != nil
}

// SendCommand 发送命令到 Extension，等待响应
func (h *Hub) SendCommand(method string, params interface{}, tabID int) (*Response, error) {
    h.mu.RLock()
    client := h.client
    h.mu.RUnlock()

    if client == nil {
        return nil, ErrNoExtensionConnected
    }

    requestID := uuid.New().String()
    respCh := make(chan Response, 1)

    h.mu.Lock()
    h.responses[requestID] = respCh
    h.mu.Unlock()

    defer func() {
        h.mu.Lock()
        delete(h.responses, requestID)
        h.mu.Unlock()
    }()

    // 构造请求
    req := Request{
        ID:     requestID,
        Method: method,
        Params: params,
        TabID:  tabID,
    }

    // 发送
    if err := client.SendJSON(req); err != nil {
        return nil, fmt.Errorf("send command failed: %w", err)
    }

    // 等待响应（带超时）
    select {
    case resp := <-respCh:
        return &resp, nil
    case <-time.After(h.timeout):
        return nil, ErrTimeout
    }
}

// RegisterClient 注册新的 Extension 客户端连接
func (h *Hub) RegisterClient(client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    // 关闭旧连接（如果存在）
    if h.client != nil {
        h.client.Close()
    }

    h.client = client
}

// UnregisterClient 注销 Extension 客户端连接
func (h *Hub) UnregisterClient(client *Client) {
    h.mu.Lock()
    defer h.mu.Unlock()

    if h.client == client {
        h.client = nil
        // 通知所有等待中的请求
        for id, ch := range h.responses {
            ch <- Response{ID: id, Status: "error", Code: "NO_EXTENSION_CONNECTED",
                          Error: "Extension disconnected"}
        }
    }
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

## WebSocket Client

```go
// internal/ws/client.go
package ws

import (
    "context"
    "net/http"
    "time"

    ws "github.com/coder/websocket"
)

type Client struct {
    conn   *ws.Conn
    hub    *Hub
    cancel context.CancelFunc
}

func NewClient(ctx context.Context, conn *ws.Conn, hub *Hub) *Client {
    ctx, cancel := context.WithCancel(ctx)
    c := &Client{
        conn:   conn,
        hub:    hub,
        cancel: cancel,
    }

    hub.RegisterClient(c)
    go c.readLoop(ctx)

    return c
}

func (c *Client) readLoop(ctx context.Context) {
    defer func() {
        c.hub.UnregisterClient(c)
        c.conn.Close(ws.StatusNormalClosure, "")
        c.cancel()
    }()

    for {
        _, data, err := c.conn.Read(ctx)
        if err != nil {
            return
        }

        var resp Response
        if err := json.Unmarshal(data, &resp); err != nil {
            continue // 忽略格式错误的消息
        }

        c.hub.HandleResponse(resp)
    }
}

func (c *Client) SendJSON(v interface{}) error {
    data, err := json.Marshal(v)
    if err != nil {
        return err
    }
    return c.conn.Write(context.Background(), ws.MessageText, data)
}

func (c *Client) Close() {
    c.cancel()
    c.conn.Close(ws.StatusNormalClosure, "server closing")
}
```

## WebSocket Server

```go
// internal/ws/server.go
package ws

import (
    "context"
    "fmt"
    "net/http"
    "os"

    "github.com/ety001/hermes-browser/internal/auth"
    "github.com/ety001/hermes-browser/internal/config"
    ws "github.com/coder/websocket"
)

func StartServer(ctx context.Context, cfg *config.Config, hub *Hub) *http.Server {
    mux := http.NewServeMux()

    mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
        // Token 认证
        token := r.URL.Query().Get("token")
        expected := cfg.GetWebSocketToken()
        if !auth.Validate(token, expected) {
            http.Error(w, "unauthorized", http.StatusUnauthorized)
            return
        }

        // Origin 检查（仅当配置了 allowed_extensions 时）
        if len(cfg.WebSocket.AllowedExtensions) > 0 {
            origin := r.Header.Get("Origin")
            if !auth.IsAllowedOrigin(origin, cfg.WebSocket.AllowedExtensions) {
                http.Error(w, "origin not allowed", http.StatusForbidden)
                return
            }
        }

        // WebSocket 升级
        conn, err := ws.Accept(w, r, nil)
        if err != nil {
            fmt.Fprintf(os.Stderr, "WebSocket accept error: %v\n", err)
            return
        }

        // 创建 client 并注册到 hub
        NewClient(ctx, conn, hub)
    })

    srv := &http.Server{
        Addr:    cfg.WebSocket.Bind,
        Handler: mux,
    }

    go func() {
        fmt.Fprintf(os.Stderr, "WebSocket server listening on %s\n", cfg.WebSocket.Bind)
        if err := srv.ListenAndServe(); err != http.ErrServerClosed {
            fmt.Fprintf(os.Stderr, "WebSocket server error: %v\n", err)
        }
    }()

    return srv
}
```

## 协议定义

```go
// internal/ws/protocol.go
package ws

// Request 是 MCP Server 发给 Extension 的命令
type Request struct {
    ID     string      `json:"id"`
    Method string      `json:"method"`
    Params interface{} `json:"params,omitempty"`
    TabID  int         `json:"tab_id"`
}

// Response 是 Extension 返回的结果
type Response struct {
    ID     string      `json:"id"`
    Status string      `json:"status"`  // "success" or "error"
    Data   interface{} `json:"data,omitempty"`
    Code   string      `json:"code,omitempty"`   // 错误码（v2 新增）
    Error  string      `json:"error,omitempty"`  // 错误信息
}

// 错误码常量（v2 新增）
const (
    ErrCodeElementNotFound      = "ELEMENT_NOT_FOUND"
    ErrCodeTimeout              = "TIMEOUT"
    ErrCodeNavigationError      = "NAVIGATION_ERROR"
    ErrCodeJSExecutionError     = "JS_EXECUTION_ERROR"
    ErrCodeTabNotFound          = "TAB_NOT_FOUND"
    ErrCodeTabClosed            = "TAB_CLOSED"
    ErrCodeNoExtensionConnected = "NO_EXTENSION_CONNECTED"
    ErrCodePermissionDenied     = "PERMISSION_DENIED"
    ErrCodeUnknownMethod        = "UNKNOWN_METHOD"
)

// 错误变量
var (
    ErrNoExtensionConnected = fmt.Errorf("%s: Chrome Extension not connected", ErrCodeNoExtensionConnected)
    ErrTimeout              = fmt.Errorf("%s: operation timed out", ErrCodeTimeout)
)

// 错误码列表（供工具描述使用）
var ErrorCodes = map[string]string{
    ErrCodeElementNotFound:      "CSS selector did not match any element. Check selector or wait and retry.",
    ErrCodeTimeout:              "Operation timed out. Increase timeout parameter and retry.",
    ErrCodeNavigationError:      "Page navigation failed. Check URL, may need login.",
    ErrCodeJSExecutionError:     "JavaScript execution error. Fix the code and retry.",
    ErrCodeTabNotFound:          "Tab ID does not exist. Re-run list_tabs to get correct IDs.",
    ErrCodeTabClosed:            "Tab has been closed. Use a different tab.",
    ErrCodeNoExtensionConnected: "Chrome Extension is not connected. Ask user to check Extension.",
    ErrCodePermissionDenied:     "Permission denied. Check if tab is active.",
    ErrCodeUnknownMethod:        "Unknown command. Check method name.",
}
```

## 工具 Handler 示例

```go
// internal/mcp/tools.go
package mcp

import (
    "context"
    "fmt"

    mcplib "github.com/mark3labs/mcp-go/mcp"
    "github.com/ety001/hermes-browser/internal/ws"
)

// getActiveTabID 通过 Extension 获取当前激活 tab 的 ID
// 如果命令参数中指定了 tab_id 则使用该值，否则通过 list_tabs 获取活跃 tab
func (s *Server) getActiveTabID(args map[string]any) (int, error) {
    if tabID, ok := args["tab_id"]; ok {
        if id, ok := tabID.(float64); ok {
            return int(id), nil
        }
    }
    // 未指定 tab_id，通过 Extension 获取当前活跃 tab
    resp, err := s.hub.SendCommand("list_tabs", nil, 0)
    if err != nil {
        return 0, err
    }
    tabs, ok := resp.Data.([]interface{})
    if !ok || len(tabs) == 0 {
        return 0, fmt.Errorf("no tabs found")
    }
    for _, t := range tabs {
        tab, _ := t.(map[string]interface{})
        if active, _ := tab["active"].(bool); active {
            id, _ := tab["id"].(float64)
            return int(id), nil
        }
    }
    return 0, fmt.Errorf("no active tab found")
}

// P0#4: 使用 mcp-go 提供的 GetString/RequireString 等便捷方法访问参数
// CallToolRequest.Arguments 实际类型是 any（运行时 map[string]any）
// SDK 提供 GetArguments() map[string]any 和 GetString/RequireString 等便捷方法

func (s *Server) handleNavigate(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
    args := req.GetArguments()

    url, ok := args["url"].(string)
    if !ok || url == "" {
        return mcplib.NewToolResultError("url is required"), nil
    }
    waitUntil, _ := args["wait_until"].(string)
    if waitUntil == "" {
        waitUntil = s.cfg.Browser.DefaultWaitUntil
    }

    tabID, err := s.getActiveTabID(args)
    if err != nil {
        return mcplib.NewToolResultText(fmt.Sprintf("Error: %s", err)), nil
    }

    params := map[string]interface{}{
        "url":        url,
        "wait_until": waitUntil,
    }
    if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
        params["timeout"] = timeout
    }

    resp, err := s.hub.SendCommand("navigate", params, tabID)
    if err != nil {
        if err == ws.ErrNoExtensionConnected {
            return mcplib.NewToolResultText("Error: Chrome Extension is not connected. Please check if the Hermes Browser extension is running."), nil
        }
        return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
    }

    if resp.Status == "error" {
        return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
    }

    data, _ := resp.Data.(map[string]interface{})
    return mcplib.NewToolResultText(fmt.Sprintf("Navigated to %s\nTitle: %s", data["url"], data["title"])), nil
}

func (s *Server) handleScreenshot(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
    args := req.GetArguments()
    format, _ := args["format"].(string)
    if format == "" {
        format = s.cfg.Browser.ScreenshotFormat  // P2#9: 从配置读取默认值
        if format == "" {
            format = "jpeg"
        }
    }
    selector, _ := args["selector"].(string)

    tabID, err := s.getActiveTabID(args)
    if err != nil {
        return mcplib.NewToolResultText(fmt.Sprintf("Error: %s", err)), nil
    }

    params := map[string]interface{}{
        "format": format,
    }
    if quality, ok := args["quality"].(float64); ok && quality > 0 {
        params["quality"] = int(quality)  // P2#10: float64 -> int 转换
    }
    if selector != "" {
        params["selector"] = selector
    }

    resp, err := s.hub.SendCommand("screenshot", params, tabID)
    if err != nil {
        return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
    }

    if resp.Status == "error" {
        return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
    }

    data, _ := resp.Data.(map[string]interface{})
    imageBase64, _ := data["image"].(string)

    // P2#13: NewToolResultImage 签名为 (text, imageData, mimeType string)
    mimeType := "image/jpeg"
    if format == "png" {
        mimeType = "image/png"
    }
    return mcplib.NewToolResultImage("Screenshot captured", imageBase64, mimeType), nil
}
```

## auth 模块设计

```go
// internal/auth/auth.go
package auth

import (
    "crypto/subtle"
    "net/http"
    "strings"
)

// Validate 安全比较两个 token（防时序攻击）
func Validate(provided, expected string) bool {
    if provided == "" || expected == "" {
        return false
    }
    return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// HTTPMiddleware 返回标准 net/http 中间件，校验 Bearer token
func HTTPMiddleware(next http.Handler, expectedToken string) http.Handler {
    return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        token := r.Header.Get("Authorization")
        if token == "" {
            http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
            return
        }

        provided := strings.TrimPrefix(token, "Bearer ")
        if !Validate(provided, expectedToken) {
            http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
            return
        }

        next.ServeHTTP(w, r)
    })
}

// IsAllowedOrigin 检查 WebSocket 请求的 Origin 是否在白名单中
// allowedExtensions 是 Chrome Extension ID 列表
// Origin 格式: chrome-extension://<extension-id>
func IsAllowedOrigin(origin string, allowedExtensions []string) bool {
    if len(allowedExtensions) == 0 {
        return true  // 未配置白名单则允许所有
    }
    for _, extID := range allowedExtensions {
        expected := "chrome-extension://" + extID
        if subtle.ConstantTimeCompare([]byte(origin), []byte(expected)) == 1 {
            return true
        }
    }
    return false
}
```

## Config 结构体定义（P2#8）

```go
// internal/config/config.go
package config

import (
    "os"
    "gopkg.in/yaml.v3"
)

type Config struct {
    Token    string        `yaml:"token"`
    Server   ServerConfig  `yaml:"server"`
    WebSocket WSConfig     `yaml:"websocket"`
    Browser  BrowserConfig `yaml:"browser"`
    Logging  LoggingConfig `yaml:"logging"`
}

type ServerConfig struct {
    HTTP HTTPConfig `yaml:"http"`
}

type HTTPConfig struct {
    Bind string `yaml:"bind"`  // e.g. "0.0.0.0:19875"
}

type WSConfig struct {
    Bind              string   `yaml:"bind"`  // e.g. "0.0.0.0:19876"
    AllowedExtensions []string `yaml:"allowed_extensions"`
}

type BrowserConfig struct {
    DefaultTimeout   int    `yaml:"default_timeout"`    // ms, default 30000
    DefaultWaitUntil string `yaml:"default_wait_until"` // "load" or "networkidle"
    ScreenshotFormat string `yaml:"screenshot_format"`  // "jpeg" or "png"
    ScreenshotQuality int   `yaml:"screenshot_quality"` // 1-100, default 80
    MaxContentLength  int    `yaml:"max_content_length"` // chars, default 50000
}

type LoggingConfig struct {
    Level string `yaml:"level"` // "debug", "info", "warn", "error"
    File  string `yaml:"file"`  // log file path, empty = stderr
}

func Load() (*Config, error) {
    // 查找配置文件：优先 -c 参数 > ~/.hermes-browser/config.yaml > ./config.yaml
    // ... (文件查找逻辑)
    data, err := os.ReadFile(path)
    if err != nil {
        return nil, err
    }
    cfg := &Config{
        Browser: BrowserConfig{
            DefaultTimeout:    30000,
            DefaultWaitUntil:  "networkidle",
            ScreenshotFormat:  "jpeg",
            ScreenshotQuality: 80,
            MaxContentLength:  50000,
        },
        Logging: LoggingConfig{
            Level: "info",
        },
    }
    if err := yaml.Unmarshal(data, cfg); err != nil {
        return nil, err
    }
    return cfg, nil
}

// GetHTTPToken 返回 HTTP 传输认证 token
// 优先级：transport.http.token > 顶层 token > 环境变量 MCP_BROWSER_TOKEN
func (c *Config) GetHTTPToken() string {
    // ...
}

// GetWebSocketToken 返回 WebSocket 认证 token
// 优先级：transport.websocket.token > 顶层 token > 环境变量 MCP_BROWSER_TOKEN
func (c *Config) GetWebSocketToken() string {
    // ...
}
```

## 项目依赖

```go
// go.mod
module github.com/ety001/hermes-browser

go 1.23

require (
    github.com/mark3labs/mcp-go v0.24.0
    github.com/coder/websocket v1.8.14
    gopkg.in/yaml.v3 v3.0.1
    github.com/google/uuid v1.6.0
)
```
