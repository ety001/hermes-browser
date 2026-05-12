package ws

import (
	"context"
	"encoding/json"
	"sync"
	"time"

	wslib "github.com/coder/websocket"
)

// Client represents a single WebSocket client connection from a
// Chrome Extension.
type Client struct {
	conn   *wslib.Conn
	hub    *Hub
	cancel context.CancelFunc

	mu      sync.Mutex
	closed  bool
}

// NewClient creates a new Client, registers it with the Hub, and starts
// the read loop.
func NewClient(ctx context.Context, conn *wslib.Conn, hub *Hub) *Client {
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

// readLoop reads incoming WebSocket messages and forwards them to the Hub
// as responses.
func (c *Client) readLoop(ctx context.Context) {
	defer func() {
		c.hub.UnregisterClient(c)
		c.close()
	}()

	for {
		_, data, err := c.conn.Read(ctx)
		if err != nil {
			return
		}

		var resp Response
		if err := json.Unmarshal(data, &resp); err != nil {
			continue // ignore malformed messages
		}

		c.hub.HandleResponse(resp)
	}
}

// SendJSON marshals v as JSON and sends it as a text message over the
// WebSocket connection.
func (c *Client) SendJSON(v interface{}) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return c.conn.Write(ctx, wslib.MessageText, data)
}

// Close terminates the WebSocket connection.
func (c *Client) Close() {
	c.cancel()
	c.close()
}

func (c *Client) close() {
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.closed {
		c.closed = true
		c.conn.Close(wslib.StatusNormalClosure, "server closing")
	}
}
