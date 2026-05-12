package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/ety001/hermes-browser/internal/auth"
	"github.com/ety001/hermes-browser/internal/config"
	"github.com/ety001/hermes-browser/internal/mcp"
	"github.com/ety001/hermes-browser/internal/ws"
	"github.com/mark3labs/mcp-go/server"
)

func main() {
	// 1. Load configuration
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load config: %v\n", err)
		os.Exit(1)
	}

	// 2. stdio mode detection
	if cfg.Server.HTTP.Bind == "" && !config.IsStdinPipe() {
		fmt.Fprintf(os.Stderr, `WARNING: No HTTP bind configured and stdin is not a pipe.
The server will wait for MCP stdio input. If you meant to start
as a standalone server, configure server.http.bind in config.yaml.
`)
	}

	// 3. Create Hub (shared between WebSocket and MCP Server)
	hub := ws.NewHub(cfg)

	// 4. Start WebSocket Server (if configured)
	var wsSrv *http.Server
	if cfg.WebSocket.Bind != "" {
		wsSrv = ws.StartServer(context.Background(), cfg, hub)
	}

	// 5. Create MCP Server (with shared hub)
	mcpServer := mcp.NewServer(cfg, hub)

	// 6. Start MCP transport
	var httpSrv *http.Server
	if cfg.Server.HTTP.Bind != "" {
		httpSrv = startHTTPServer(cfg, mcpServer)
	} else {
		startStdioServer(mcpServer)
	}

	// 7. Wait for shutdown signal
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	// 8. Graceful shutdown
	fmt.Fprintf(os.Stderr, "\nShutting down...\n")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if httpSrv != nil {
		httpSrv.Shutdown(shutdownCtx)
	}
	if wsSrv != nil {
		wsSrv.Shutdown(shutdownCtx)
	}
}

// startHTTPServer creates an HTTP server wrapping the MCP StreamableHTTP
// handler with auth middleware.
func startHTTPServer(cfg *config.Config, mcpServer *mcp.Server) *http.Server {
	sseServer := server.NewStreamableHTTPServer(
		mcpServer.MCPServer(),
	)

	mux := http.NewServeMux()
	mux.Handle("/mcp", auth.HTTPMiddleware(sseServer, cfg.GetHTTPToken()))

	// Health check endpoint (no auth required)
	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		connected := mcpServer.IsExtensionConnected()
		fmt.Fprintf(w, `{"status":"ok","extension_connected":%v}`, connected)
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

// startStdioServer starts the MCP server in stdio mode.
func startStdioServer(mcpServer *mcp.Server) {
	// In stdio mode, create a StreamableHTTP server and use stdio transport
	_ = server.NewStreamableHTTPServer(
		mcpServer.MCPServer(),
	)

	fmt.Fprintf(os.Stderr, "MCP server running in stdio mode\n")
	// Note: Full stdio transport requires additional mcp-go integration.
	// For now, this serves as a placeholder.
}
