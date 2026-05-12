package ws

import (
	"context"
	"fmt"
	"net/http"
	"os"

	"github.com/ety001/hermes-browser/internal/auth"
	"github.com/ety001/hermes-browser/internal/config"
	wslib "github.com/coder/websocket"
)

// StartServer starts the WebSocket HTTP server for Chrome Extension
// connections. It returns the *http.Server so the caller can shut it down
// gracefully.
func StartServer(ctx context.Context, cfg *config.Config, hub *Hub) *http.Server {
	mux := http.NewServeMux()

	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		// Token authentication via query parameter
		token := r.URL.Query().Get("token")
		expected := cfg.GetWebSocketToken()
		if !auth.Validate(token, expected) {
			http.Error(w, "unauthorized", http.StatusUnauthorized)
			return
		}

		// Origin check (only when allowed_extensions is configured)
		if len(cfg.WebSocket.AllowedExtensions) > 0 {
			origin := r.Header.Get("Origin")
			if !auth.IsAllowedOrigin(origin, cfg.WebSocket.AllowedExtensions) {
				http.Error(w, "origin not allowed", http.StatusForbidden)
				return
			}
		}

		// WebSocket upgrade
		conn, err := wslib.Accept(w, r, &wslib.AcceptOptions{
			InsecureSkipVerify: true, // Allow any origin during handshake;
			                        // app-level origin check above handles auth.
		})
		if err != nil {
			fmt.Fprintf(os.Stderr, "WebSocket accept error: %v\n", err)
			return
		}

		// Create client and register with hub
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
