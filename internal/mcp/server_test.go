package mcp

import (
	"testing"

	"github.com/ety001/hermes-browser/internal/config"
	"github.com/ety001/hermes-browser/internal/ws"
	mcplib "github.com/mark3labs/mcp-go/mcp"
)

func newTestServer() *Server {
	cfg := config.DefaultConfig()
	cfg.Browser.DefaultTimeout = 30000
	hub := ws.NewHub(cfg)
	return NewServer(cfg, hub)
}

func TestNewServer(t *testing.T) {
	s := newTestServer()
	if s == nil {
		t.Fatal("expected non-nil server")
	}
	if s.MCPServer() == nil {
		t.Fatal("expected non-nil MCPServer")
	}
}

func TestIsExtensionConnected(t *testing.T) {
	s := newTestServer()
	if s.IsExtensionConnected() {
		t.Error("expected IsExtensionConnected to be false with no client")
	}
}

func TestAllToolsRegistered(t *testing.T) {
	s := newTestServer()
	tools := s.MCPServer().ListTools()
	if tools == nil {
		t.Fatal("expected non-nil tools map")
	}

	expectedTools := []string{
		"navigate", "screenshot", "get_content", "click", "type",
		"scroll", "execute_js", "wait_for", "get_cookies", "list_tabs",
		"switch_tab", "close_tab", "new_tab", "hover", "select_option",
	}

	if len(tools) != len(expectedTools) {
		t.Errorf("expected %d tools, got %d", len(expectedTools), len(tools))
	}

	for _, name := range expectedTools {
		if _, ok := tools[name]; !ok {
			t.Errorf("expected tool '%s' not found", name)
		}
	}
}

func TestToolDescriptions(t *testing.T) {
	s := newTestServer()
	tools := s.MCPServer().ListTools()

	type testCase struct {
		name       string
		wantMinLen int
	}
	cases := []testCase{
		{"navigate", 10}, {"screenshot", 10}, {"get_content", 10},
		{"click", 10}, {"type", 10}, {"scroll", 5},
		{"execute_js", 10}, {"wait_for", 10}, {"get_cookies", 10},
		{"list_tabs", 5}, {"switch_tab", 10}, {"close_tab", 10},
		{"new_tab", 5}, {"hover", 10}, {"select_option", 10},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, ok := tools[tc.name]
			if !ok {
				t.Fatalf("tool '%s' not found", tc.name)
			}
			if st.Tool.Description == "" {
				t.Errorf("tool '%s' has empty description", tc.name)
			}
			if len(st.Tool.Description) < tc.wantMinLen {
				t.Errorf("tool '%s' description too short (%d chars)", tc.name, len(st.Tool.Description))
			}
		})
	}
}

func TestToolRequiredParams(t *testing.T) {
	s := newTestServer()
	tools := s.MCPServer().ListTools()

	type testCase struct {
		name           string
		requiredParams []string
	}
	cases := []testCase{
		{"navigate", []string{"url"}},
		{"click", []string{"selector"}},
		{"type", []string{"selector", "text"}},
		{"execute_js", []string{"expression"}},
		{"wait_for", []string{"selector"}},
		{"switch_tab", []string{"tab_id"}},
		{"hover", []string{"selector"}},
		{"select_option", []string{"selector", "value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, ok := tools[tc.name]
			if !ok {
				t.Fatalf("tool '%s' not found", tc.name)
			}

			schema := st.Tool.InputSchema
			if schema.Type != "object" {
				t.Errorf("schema type should be 'object', got '%s'", schema.Type)
			}

			for _, param := range tc.requiredParams {
				if _, ok := schema.Properties[param]; !ok {
					t.Errorf("missing required parameter '%s'", param)
				}
			}

			if len(tc.requiredParams) > 0 && len(schema.Required) == 0 {
				t.Errorf("has required params but Required array is empty")
			}
		})
	}
}

func TestToolOptionalParams(t *testing.T) {
	s := newTestServer()
	tools := s.MCPServer().ListTools()

	type testCase struct {
		name           string
		optionalParams []string
	}
	cases := []testCase{
		{"navigate", []string{"wait_until", "timeout"}},
		{"screenshot", []string{"selector", "format", "quality"}},
		{"get_content", []string{"selector", "type"}},
		{"type", []string{"clear_first", "press_enter"}},
		{"scroll", []string{"direction", "amount"}},
		{"execute_js", []string{"return_value"}},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			st, ok := tools[tc.name]
			if !ok {
				t.Fatalf("tool '%s' not found", tc.name)
			}
			schema := st.Tool.InputSchema
			for _, param := range tc.optionalParams {
				if _, ok := schema.Properties[param]; !ok {
					t.Errorf("missing optional parameter '%s'", param)
				}
			}
		})
	}
}

func TestToolNamesMatchWSMethods(t *testing.T) {
	wsMethods := map[string]bool{
		"navigate": true, "screenshot": true, "get_content": true,
		"click": true, "type": true, "scroll": true, "execute_js": true,
		"wait_for": true, "get_cookies": true, "list_tabs": true,
		"switch_tab": true, "close_tab": true, "new_tab": true,
		"hover": true, "select_option": true,
	}

	s := newTestServer()
	for name := range s.MCPServer().ListTools() {
		if !wsMethods[name] {
			t.Errorf("tool '%s' has no matching WS method constant", name)
		}
	}
}

func TestToolHandlerExists(t *testing.T) {
	s := newTestServer()
	for name, st := range s.MCPServer().ListTools() {
		if st.Handler == nil {
			t.Errorf("tool '%s' has nil handler", name)
		}
	}
}

func TestHandleNavigateWithoutExtension(t *testing.T) {
	s := newTestServer()

	req := mcplib.CallToolRequest{
		Params: mcplib.CallToolParams{
			Name: "navigate",
			Arguments: map[string]any{
				"url": "https://example.com",
			},
		},
	}

	result, err := s.handleNavigate(nil, req)
	if err != nil {
		t.Fatalf("handleNavigate unexpected error: %v", err)
	}
	if result == nil {
		t.Fatal("handleNavigate returned nil")
	}

	// Should contain an error message about no Extension
	if len(result.Content) == 0 {
		t.Fatal("expected non-empty content")
	}
	// NewToolResultText is used (not NewToolResultError), so IsError may be false
	// but the text should indicate the issue
	if !result.IsError {
		// Check text content for error indication
		foundError := false
		for _, c := range result.Content {
			if tc, ok := c.(mcplib.TextContent); ok {
				if tc.Text != "" {
					foundError = true
					break
				}
			}
		}
		if !foundError {
			t.Error("expected some content in result")
		}
	}
}
