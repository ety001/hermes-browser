package ws

import (
	"fmt"
	"sync"
	"time"

	"github.com/ety001/hermes-browser/internal/config"
	"github.com/google/uuid"
)

// Hub manages a single Chrome Extension WebSocket client connection and
// provides request/response dispatching.
type Hub struct {
	mu        sync.RWMutex
	client    *Client               // single Extension client
	responses map[string]chan Response // requestID -> response channel
	timeout   time.Duration
}

// NewHub creates a new Hub with timeout derived from the configuration.
func NewHub(cfg *config.Config) *Hub {
	timeout := time.Duration(cfg.Browser.DefaultTimeout) * time.Millisecond
	if timeout < 10*time.Second {
		timeout = 60 * time.Second
	}
	timeout += 10 * time.Second // network buffer

	return &Hub{
		responses: make(map[string]chan Response),
		timeout:   timeout,
	}
}

// HasClient returns true if an Extension is currently connected.
func (h *Hub) HasClient() bool {
	h.mu.RLock()
	defer h.mu.RUnlock()
	return h.client != nil
}

// SendCommand sends a command to the Extension and waits for the response.
// It generates a new request ID, sends the command via the WebSocket client,
// and blocks until a response is received or the timeout expires.
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

	req := Request{
		ID:     requestID,
		Method: method,
		Params: params,
		TabID:  tabID,
	}

	if err := client.SendJSON(req); err != nil {
		return nil, fmt.Errorf("send command failed: %w", err)
	}

	select {
	case resp := <-respCh:
		return &resp, nil
	case <-time.After(h.timeout):
		return nil, ErrTimeout
	}
}

// RegisterClient registers a new Extension client. If a previous client
// exists, it is closed first.
func (h *Hub) RegisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.client != nil {
		h.client.Close()
	}
	h.client = client
}

// UnregisterClient removes a client from the hub. If it is the current
// client, all pending requests are notified of the disconnection.
func (h *Hub) UnregisterClient(client *Client) {
	h.mu.Lock()
	defer h.mu.Unlock()

	if h.client == client {
		h.client = nil
		for id, ch := range h.responses {
			ch <- Response{
				ID:     id,
				Status: "error",
				Code:   ErrCodeNoExtensionConnected,
				Error:  "Extension disconnected",
			}
		}
	}
}

// HandleResponse routes an incoming response to the waiting request channel.
func (h *Hub) HandleResponse(resp Response) {
	h.mu.RLock()
	ch, ok := h.responses[resp.ID]
	h.mu.RUnlock()

	if ok {
		select {
		case ch <- resp:
		default:
			// Channel already has a value or closed; discard.
		}
	}
}
