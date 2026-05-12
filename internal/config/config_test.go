package config

import (
	"os"
	"path/filepath"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()
	if cfg.Browser.DefaultTimeout != 30000 {
		t.Errorf("expected DefaultTimeout 30000, got %d", cfg.Browser.DefaultTimeout)
	}
	if cfg.Browser.DefaultWaitUntil != "networkidle" {
		t.Errorf("expected DefaultWaitUntil 'networkidle', got %s", cfg.Browser.DefaultWaitUntil)
	}
	if cfg.Browser.ScreenshotFormat != "jpeg" {
		t.Errorf("expected ScreenshotFormat 'jpeg', got %s", cfg.Browser.ScreenshotFormat)
	}
	if cfg.Browser.ScreenshotQuality != 80 {
		t.Errorf("expected ScreenshotQuality 80, got %d", cfg.Browser.ScreenshotQuality)
	}
	if cfg.Browser.MaxContentLength != 500000 {
		t.Errorf("expected MaxContentLength 500000, got %d", cfg.Browser.MaxContentLength)
	}
	if cfg.Logging.Level != "info" {
		t.Errorf("expected Logging Level 'info', got %s", cfg.Logging.Level)
	}
}

func TestConfigYAMLLoading(t *testing.T) {
	// Create a temp config file
	tmpDir := t.TempDir()
	configPath := filepath.Join(tmpDir, "config.yaml")
	content := []byte(`
token: "test-token"
server:
  http:
    bind: "127.0.0.1:19875"
websocket:
  bind: "127.0.0.1:19876"
browser:
  default_wait_until: "load"
  default_timeout: 10000
logging:
  level: "debug"
`)
	if err := os.WriteFile(configPath, content, 0644); err != nil {
		t.Fatal(err)
	}

	cfg := DefaultConfig()
	if err := cfg.loadFile(configPath); err != nil {
		t.Fatal(err)
	}

	if cfg.Token != "test-token" {
		t.Errorf("expected token 'test-token', got '%s'", cfg.Token)
	}
	if cfg.Server.HTTP.Bind != "127.0.0.1:19875" {
		t.Errorf("expected bind '127.0.0.1:19875', got '%s'", cfg.Server.HTTP.Bind)
	}
	if cfg.WebSocket.Bind != "127.0.0.1:19876" {
		t.Errorf("expected ws bind '127.0.0.1:19876', got '%s'", cfg.WebSocket.Bind)
	}
	if cfg.Browser.DefaultWaitUntil != "load" {
		t.Errorf("expected wait_until 'load', got '%s'", cfg.Browser.DefaultWaitUntil)
	}
	if cfg.Browser.DefaultTimeout != 10000 {
		t.Errorf("expected timeout 10000, got %d", cfg.Browser.DefaultTimeout)
	}
	if cfg.Logging.Level != "debug" {
		t.Errorf("expected log level 'debug', got '%s'", cfg.Logging.Level)
	}
}

func TestEnvOverrides(t *testing.T) {
	os.Setenv("HB_TOKEN", "env-token")
	os.Setenv("HB_SERVER_HTTP_BIND", "0.0.0.0:9999")
	os.Setenv("HB_BROWSER_DEFAULT_TIMEOUT", "5000")
	defer os.Unsetenv("HB_TOKEN")
	defer os.Unsetenv("HB_SERVER_HTTP_BIND")
	defer os.Unsetenv("HB_BROWSER_DEFAULT_TIMEOUT")

	cfg := DefaultConfig()
	cfg.applyEnvOverrides()

	if cfg.Token != "env-token" {
		t.Errorf("expected token 'env-token', got '%s'", cfg.Token)
	}
	if cfg.Server.HTTP.Bind != "0.0.0.0:9999" {
		t.Errorf("expected http bind '0.0.0.0:9999', got '%s'", cfg.Server.HTTP.Bind)
	}
	if cfg.Browser.DefaultTimeout != 5000 {
		t.Errorf("expected timeout 5000, got %d", cfg.Browser.DefaultTimeout)
	}
}

func TestTokenResolution(t *testing.T) {
	// Test: HTTP token falls back to top-level token
	cfg := DefaultConfig()
	cfg.Token = "top-token"
	cfg.resolveTokens()

	if cfg.GetHTTPToken() != "top-token" {
		t.Errorf("expected HTTP token 'top-token', got '%s'", cfg.GetHTTPToken())
	}
	if cfg.GetWebSocketToken() != "top-token" {
		t.Errorf("expected WS token 'top-token', got '%s'", cfg.GetWebSocketToken())
	}

	// Test: per-transport token overrides top-level
	cfg.Server.HTTP.Token = "http-specific"
	cfg.WebSocket.Token = "ws-specific"
	cfg.resolveTokens()

	if cfg.GetHTTPToken() != "http-specific" {
		t.Errorf("expected HTTP token 'http-specific', got '%s'", cfg.GetHTTPToken())
	}
	if cfg.GetWebSocketToken() != "ws-specific" {
		t.Errorf("expected WS token 'ws-specific', got '%s'", cfg.GetWebSocketToken())
	}
}

func TestIsStdinPipe(t *testing.T) {
	// In test environment, stdin is typically a pipe or not connected
	// Just verify the function doesn't panic
	_ = IsStdinPipe()
}

// Helper: loadFile loads config from a specific path for testing.
func (c *Config) loadFile(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, c)
}
