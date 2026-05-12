package config

import (
	"crypto/rand"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

// Config is the top-level configuration structure.
type Config struct {
	Token     string        `yaml:"token"`
	Server    ServerConfig  `yaml:"server"`
	WebSocket WSConfig      `yaml:"websocket"`
	Browser   BrowserConfig `yaml:"browser"`
	Logging   LoggingConfig `yaml:"logging"`
}

// ServerConfig holds MCP HTTP transport settings.
type ServerConfig struct {
	HTTP HTTPConfig `yaml:"http"`
}

// HTTPConfig holds HTTP transport bind address and optional token override.
type HTTPConfig struct {
	Bind  string `yaml:"bind"`
	Token string `yaml:"token"`
}

// WSConfig holds WebSocket server settings.
type WSConfig struct {
	Bind              string   `yaml:"bind"`
	Token             string   `yaml:"token"`
	AllowedExtensions []string `yaml:"allowed_extensions"`
}

// BrowserConfig holds default browser operation settings.
type BrowserConfig struct {
	DefaultTimeout     int    `yaml:"default_timeout"`
	DefaultWaitUntil   string `yaml:"default_wait_until"`
	ScreenshotFormat   string `yaml:"screenshot_format"`
	ScreenshotQuality  int    `yaml:"screenshot_quality"`
	MaxContentLength   int    `yaml:"max_content_length"`
}

// LoggingConfig holds logging settings.
type LoggingConfig struct {
	Level string `yaml:"level"`
	File  string `yaml:"file"`
}

// tokenDir is the directory for token persistence.
const tokenDir = "~/.hermes-browser"

// tokenFile is the token file name.
const tokenFile = ".token"

// DefaultConfig returns a Config with sensible defaults.
func DefaultConfig() *Config {
	return &Config{
		Browser: BrowserConfig{
			DefaultTimeout:    30000,
			DefaultWaitUntil:  "networkidle",
			ScreenshotFormat:  "jpeg",
			ScreenshotQuality: 80,
			MaxContentLength:  500000,
		},
		Logging: LoggingConfig{
			Level: "info",
		},
	}
}

// Load loads configuration with the following priority (high to low):
// 1. Command line flag -c
// 2. Environment variable HERMES_BROWSER_CONFIG
// 3. ./config.yaml
// 4. ~/.hermes-browser/config.yaml
// 5. Executable directory config.yaml (not implemented for simplicity)
func Load() (*Config, error) {
	cfg := DefaultConfig()

	configPath := findConfigPath()
	if configPath != "" {
		data, err := os.ReadFile(configPath)
		if err != nil {
			return nil, fmt.Errorf("reading config file %s: %w", configPath, err)
		}
		if err := yaml.Unmarshal(data, cfg); err != nil {
			return nil, fmt.Errorf("parsing config file %s: %w", configPath, err)
		}
	}

	// Apply environment variable overrides
	cfg.applyEnvOverrides()

	// Resolve tokens
	if err := cfg.resolveTokens(); err != nil {
		return nil, fmt.Errorf("resolving tokens: %w", err)
	}

	return cfg, nil
}

// findConfigPath searches for a config file with priority.
func findConfigPath() string {
	// 1. Check -c flag
	var configFlag string
	flag.StringVar(&configFlag, "c", "", "path to config file")
	flag.Parse()

	if configFlag != "" {
		if _, err := os.Stat(configFlag); err == nil {
			return configFlag
		}
		return ""
	}

	// 2. Environment variable
	envPath := os.Getenv("HERMES_BROWSER_CONFIG")
	if envPath != "" {
		if _, err := os.Stat(envPath); err == nil {
			return envPath
		}
	}

	// 3. Current directory
	if _, err := os.Stat("config.yaml"); err == nil {
		return "config.yaml"
	}

	// 4. ~/.hermes-browser/config.yaml
	homeDir, err := os.UserHomeDir()
	if err == nil {
		path := filepath.Join(homeDir, ".hermes-browser", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return path
		}
	}

	return ""
}

// applyEnvOverrides overrides config fields from environment variables (HB_ prefix).
func (c *Config) applyEnvOverrides() {
	if v := os.Getenv("HB_TOKEN"); v != "" {
		c.Token = v
	}
	if v := os.Getenv("HB_SERVER_HTTP_BIND"); v != "" {
		c.Server.HTTP.Bind = v
	}
	if v := os.Getenv("HB_SERVER_HTTP_TOKEN"); v != "" {
		c.Server.HTTP.Token = v
	}
	if v := os.Getenv("HB_WEBSOCKET_BIND"); v != "" {
		c.WebSocket.Bind = v
	}
	if v := os.Getenv("HB_WEBSOCKET_TOKEN"); v != "" {
		c.WebSocket.Token = v
	}
	if v := os.Getenv("HB_BROWSER_DEFAULT_WAIT_UNTIL"); v != "" {
		c.Browser.DefaultWaitUntil = v
	}
	if v := os.Getenv("HB_BROWSER_DEFAULT_TIMEOUT"); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			c.Browser.DefaultTimeout = i
		}
	}
	if v := os.Getenv("HB_LOGGING_LEVEL"); v != "" {
		c.Logging.Level = v
	}
}

// resolveTokens ensures tokens are properly set, auto-generating if needed.
func (c *Config) resolveTokens() error {
	// Resolve HTTP token
	if c.Server.HTTP.Token == "" {
		c.Server.HTTP.Token = c.Token
	}
	if c.Server.HTTP.Token == "" {
		token, err := loadOrGenerateToken()
		if err != nil {
			return err
		}
		c.Server.HTTP.Token = token
	}

	// Resolve WebSocket token
	if c.WebSocket.Token == "" {
		c.WebSocket.Token = c.Token
	}
	if c.WebSocket.Token == "" {
		token, err := loadOrGenerateToken()
		if err != nil {
			return err
		}
		c.WebSocket.Token = token
	}

	return nil
}

// loadOrGenerateToken loads the token from the persistent file,
// or generates a new one if the file doesn't exist.
func loadOrGenerateToken() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("getting home directory: %w", err)
	}

	dir := filepath.Join(homeDir, ".hermes-browser")
	tokenPath := filepath.Join(dir, tokenFile)

	// Try to read existing token
	data, err := os.ReadFile(tokenPath)
	if err == nil {
		return strings.TrimSpace(string(data)), nil
	}

	// Generate new token
	if !os.IsNotExist(err) {
		return "", fmt.Errorf("reading token file: %w", err)
	}

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return "", fmt.Errorf("generating token: %w", err)
	}
	token := hex.EncodeToString(tokenBytes)

	// Persist token
	if err := os.MkdirAll(dir, 0700); err != nil {
		return "", fmt.Errorf("creating token directory: %w", err)
	}
	if err := os.WriteFile(tokenPath, []byte(token+"\n"), 0600); err != nil {
		return "", fmt.Errorf("writing token file: %w", err)
	}

	fmt.Fprintf(os.Stderr, "Token auto-generated and saved to %s\n", tokenPath)
	return token, nil
}

// GetHTTPToken returns the effective token for HTTP transport.
func (c *Config) GetHTTPToken() string {
	return c.Server.HTTP.Token
}

// GetWebSocketToken returns the effective token for WebSocket transport.
func (c *Config) GetWebSocketToken() string {
	return c.WebSocket.Token
}

// IsStdinPipe checks if stdin is a pipe (not a terminal).
func IsStdinPipe() bool {
	fi, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return fi.Mode()&os.ModeCharDevice == 0
}
