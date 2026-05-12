package ws

import (
	"testing"
	"time"

	"github.com/ety001/hermes-browser/internal/config"
)

func newTestHub() *Hub {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultTimeout = 30000
	return NewHub(cfg)
}

func TestNewHub(t *testing.T) {
	hub := newTestHub()
	if hub == nil {
		t.Fatal("expected non-nil hub")
	}
	if hub.HasClient() {
		t.Error("expected no client initially")
	}
}

func TestHasClient(t *testing.T) {
	hub := newTestHub()
	if hub.HasClient() {
		t.Error("expected HasClient to be false with no client")
	}
}

func TestSendCommandNoClient(t *testing.T) {
	hub := newTestHub()
	_, err := hub.SendCommand("navigate", nil, 1)
	if err == nil {
		t.Fatal("expected error when no client connected")
	}
	if err.Error() != ErrNoExtensionConnected.Error() {
		t.Errorf("expected ErrNoExtensionConnected, got: %v", err)
	}
}

func TestRegisterUnregisterClient(t *testing.T) {
	hub := newTestHub()

	// We can't create a real Client without a WebSocket connection,
	// but we can test the hub's state changes.
	if hub.HasClient() {
		t.Error("expected no client")
	}

	// Verify client tracking map is empty
	hub.mu.RLock()
	clientCount := 0
	if hub.client != nil {
		clientCount++
	}
	hub.mu.RUnlock()
	if clientCount != 0 {
		t.Errorf("expected 0 clients, got %d", clientCount)
	}
}

func TestTimeoutConfiguration(t *testing.T) {
	// Test: large timeout uses config value + 10s buffer
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultTimeout = 60000 // 60s

	hub := NewHub(cfg)
	expected := 70 * time.Second // 60s + 10s buffer
	if hub.timeout != expected {
		t.Errorf("expected timeout %v, got %v", expected, hub.timeout)
	}

	// Test: small timeout is floored to 60s min + 10s buffer
	cfg.Browser.DefaultTimeout = 1000
	hub = NewHub(cfg)
	if hub.timeout != 70*time.Second {
		t.Errorf("expected timeout 70s for small config, got %v", hub.timeout)
	}
}

func TestHandleResponseNoChannel(t *testing.T) {
	hub := newTestHub()

	// HandleResponse should not panic when there's no waiting channel
	resp := Response{
		ID:     "nonexistent-id",
		Status: "success",
	}
	hub.HandleResponse(resp)
}
