package mcp

import (
	"github.com/ety001/hermes-browser/internal/config"
	"github.com/ety001/hermes-browser/internal/ws"
	"github.com/mark3labs/mcp-go/server"
)

// Server wraps the MCP server and provides browser control tools.
type Server struct {
	cfg  *config.Config
	hub  *ws.Hub
	mcpS *server.MCPServer
}

// NewServer creates a new MCP Server instance, registering all tools.
func NewServer(cfg *config.Config, hub *ws.Hub) *Server {
	s := &Server{
		cfg: cfg,
		hub: hub,
	}

	s.mcpS = server.NewMCPServer(
		"hermes-browser",
		"1.0.0",
	)

	s.registerTools()
	return s
}

// MCPServer returns the underlying MCPServer for HTTP transport wrapping.
func (s *Server) MCPServer() *server.MCPServer {
	return s.mcpS
}

// IsExtensionConnected returns whether a Chrome Extension is currently
// connected via WebSocket.
func (s *Server) IsExtensionConnected() bool {
	return s.hub.HasClient()
}

// getActiveTabID resolves the target tab ID from request arguments.
// If tab_id is provided, it is used directly. Otherwise, the active tab
// is queried from the Extension.
func (s *Server) getActiveTabID(args map[string]any) (int, error) {
	if tabID, ok := args["tab_id"]; ok {
		if id, ok := tabID.(float64); ok {
			return int(id), nil
		}
	}

	// Query the active tab from the Extension
	resp, err := s.hub.SendCommand(ws.MethodListTabs, nil, 0)
	if err != nil {
		return 0, err
	}
	if resp.Status == "error" {
		return 0, nil
	}

	tabs, ok := resp.Data.([]any)
	if !ok || len(tabs) == 0 {
		return 0, nil
	}
	for _, t := range tabs {
		tab, ok := t.(map[string]any)
		if !ok {
			continue
		}
		if active, ok := tab["active"].(bool); ok && active {
			if id, ok := tab["id"].(float64); ok {
				return int(id), nil
			}
		}
	}
	return 0, nil
}

func (s *Server) registerTools() {
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
