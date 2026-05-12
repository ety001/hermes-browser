package mcp

import (
	"context"
	"fmt"

	mcplib "github.com/mark3labs/mcp-go/mcp"
)

// Tool definitions -----------------------------------------------------------

var navigateTool = mcplib.NewTool("navigate",
	mcplib.WithDescription("Navigate to a URL and wait for the page to load"),
	mcplib.WithString("url",
		mcplib.Description("The URL to navigate to"),
		mcplib.Required(),
	),
	mcplib.WithString("wait_until",
		mcplib.Description("Wait condition: domcontentloaded, load, networkidle"),
		mcplib.DefaultString("networkidle"),
	),
	mcplib.WithNumber("timeout",
		mcplib.Description("Timeout in milliseconds"),
		mcplib.DefaultNumber(30000),
	),
)

var screenshotTool = mcplib.NewTool("screenshot",
	mcplib.WithDescription("Take a screenshot of the current page or a specific element"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector of element to capture (optional)"),
	),
	mcplib.WithString("format",
		mcplib.Description("Image format: jpeg or png"),
		mcplib.DefaultString("jpeg"),
	),
	mcplib.WithNumber("quality",
		mcplib.Description("Image quality 0-100 (jpeg only)"),
		mcplib.DefaultNumber(80),
	),
)

var getContentTool = mcplib.NewTool("get_content",
	mcplib.WithDescription("Get text content of the page or a specific element"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector to extract content from (optional)"),
	),
	mcplib.WithString("type",
		mcplib.Description("Content type: text, markdown, or html"),
		mcplib.DefaultString("text"),
	),
)

var clickTool = mcplib.NewTool("click",
	mcplib.WithDescription("Click on an element matching the CSS selector"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector of the element to click"),
		mcplib.Required(),
	),
	mcplib.WithNumber("timeout",
		mcplib.Description("Timeout in milliseconds to wait for element"),
		mcplib.DefaultNumber(10000),
	),
)

var typeTool = mcplib.NewTool("type",
	mcplib.WithDescription("Type text into an input field"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector of the input field"),
		mcplib.Required(),
	),
	mcplib.WithString("text",
		mcplib.Description("Text to type"),
		mcplib.Required(),
	),
	mcplib.WithBoolean("clear_first",
		mcplib.Description("Clear existing content before typing"),
		mcplib.DefaultBool(true),
	),
	mcplib.WithBoolean("press_enter",
		mcplib.Description("Press Enter after typing"),
		mcplib.DefaultBool(false),
	),
)

var scrollTool = mcplib.NewTool("scroll",
	mcplib.WithDescription("Scroll the page up or down"),
	mcplib.WithString("direction",
		mcplib.Description("Scroll direction: up or down"),
		mcplib.DefaultString("down"),
	),
	mcplib.WithString("amount",
		mcplib.Description("Scroll amount: one_page, half_page, or a CSS value like 500px"),
		mcplib.DefaultString("one_page"),
	),
)

var executeJsTool = mcplib.NewTool("execute_js",
	mcplib.WithDescription("Execute JavaScript code in the page context"),
	mcplib.WithString("expression",
		mcplib.Description("JavaScript code to execute"),
		mcplib.Required(),
	),
	mcplib.WithBoolean("return_value",
		mcplib.Description("Whether to return the result of the last expression"),
		mcplib.DefaultBool(true),
	),
)

var waitForTool = mcplib.NewTool("wait_for",
	mcplib.WithDescription("Wait for an element to appear on the page"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector to wait for"),
		mcplib.Required(),
	),
	mcplib.WithString("state",
		mcplib.Description("Element state: visible, hidden, attached, detached"),
		mcplib.DefaultString("visible"),
	),
	mcplib.WithNumber("timeout",
		mcplib.Description("Maximum wait time in milliseconds"),
		mcplib.DefaultNumber(30000),
	),
)

var getCookiesTool = mcplib.NewTool("get_cookies",
	mcplib.WithDescription("Get cookies for the current page"),
	mcplib.WithString("url",
		mcplib.Description("Filter cookies by URL (optional)"),
	),
	mcplib.WithString("name",
		mcplib.Description("Filter cookies by name (optional)"),
	),
)

var listTabsTool = mcplib.NewTool("list_tabs",
	mcplib.WithDescription("List all open browser tabs"),
)

var switchTabTool = mcplib.NewTool("switch_tab",
	mcplib.WithDescription("Switch to a specific browser tab"),
	mcplib.WithNumber("tab_id",
		mcplib.Description("Tab ID to switch to"),
		mcplib.Required(),
	),
)

var closeTabTool = mcplib.NewTool("close_tab",
	mcplib.WithDescription("Close a specific browser tab"),
	mcplib.WithNumber("tab_id",
		mcplib.Description("Tab ID to close (omit to close current tab)"),
	),
)

var newTabTool = mcplib.NewTool("new_tab",
	mcplib.WithDescription("Open a new browser tab"),
	mcplib.WithString("url",
		mcplib.Description("URL to open in the new tab (optional)"),
	),
)

var hoverTool = mcplib.NewTool("hover",
	mcplib.WithDescription("Hover the mouse over an element"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector of the element to hover over"),
		mcplib.Required(),
	),
)

var selectOptionTool = mcplib.NewTool("select_option",
	mcplib.WithDescription("Select an option in a dropdown/select element"),
	mcplib.WithString("selector",
		mcplib.Description("CSS selector of the select element"),
		mcplib.Required(),
	),
	mcplib.WithString("value",
		mcplib.Description("Value of the option to select"),
		mcplib.Required(),
	),
)

// Handler implementations ----------------------------------------------------

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
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"url":        url,
		"wait_until": waitUntil,
	}
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		params["timeout"] = timeout
	}

	resp, err := s.hub.SendCommand("navigate", params, tabID)
	if err != nil {
		if isNoExtensionError(err) {
			return mcplib.NewToolResultText("Error: Chrome Extension is not connected. Please check if the Hermes Browser extension is running."), nil
		}
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	data, _ := resp.Data.(map[string]any)
	urlStr, _ := data["url"].(string)
	title, _ := data["title"].(string)
	return mcplib.NewToolResultText(fmt.Sprintf("Navigated to %s\nTitle: %s", urlStr, title)), nil
}

func (s *Server) handleScreenshot(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	format, _ := args["format"].(string)
	if format == "" {
		format = s.cfg.Browser.ScreenshotFormat
		if format == "" {
			format = "jpeg"
		}
	}

	selector, _ := args["selector"].(string)

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"format": format,
	}
	if quality, ok := args["quality"].(float64); ok && quality > 0 {
		params["quality"] = int(quality)
	} else {
		params["quality"] = s.cfg.Browser.ScreenshotQuality
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

	data, _ := resp.Data.(map[string]any)
	imageBase64, _ := data["image"].(string)

	mimeType := "image/jpeg"
	if format == "png" {
		mimeType = "image/png"
	}
	return mcplib.NewToolResultImage("Screenshot captured", imageBase64, mimeType), nil
}

func (s *Server) handleGetContent(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, _ := args["selector"].(string)
	contentType, _ := args["type"].(string)
	if contentType == "" {
		contentType = "text"
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"type": contentType,
	}
	if selector != "" {
		params["selector"] = selector
	}

	resp, err := s.hub.SendCommand("get_content", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	data, _ := resp.Data.(string)
	if len(data) > s.cfg.Browser.MaxContentLength {
		data = data[:s.cfg.Browser.MaxContentLength] + "\n\n... [content truncated]"
	}
	return mcplib.NewToolResultText(data), nil
}

func (s *Server) handleClick(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, ok := args["selector"].(string)
	if !ok || selector == "" {
		return mcplib.NewToolResultError("selector is required"), nil
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"selector": selector,
	}
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		params["timeout"] = timeout
	}

	resp, err := s.hub.SendCommand("click", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Clicked element: %s", selector)), nil
}

func (s *Server) handleType(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, ok := args["selector"].(string)
	if !ok || selector == "" {
		return mcplib.NewToolResultError("selector is required"), nil
	}
	text, ok := args["text"].(string)
	if !ok {
		return mcplib.NewToolResultError("text is required"), nil
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"selector": selector,
		"text":     text,
	}
	if clearFirst, ok := args["clear_first"].(bool); ok {
		params["clear_first"] = clearFirst
	}
	if pressEnter, ok := args["press_enter"].(bool); ok {
		params["press_enter"] = pressEnter
	}

	resp, err := s.hub.SendCommand("type", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Typed %d characters into %s", len(text), selector)), nil
}

func (s *Server) handleScroll(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	direction, _ := args["direction"].(string)
	if direction == "" {
		direction = "down"
	}
	amount, _ := args["amount"].(string)
	if amount == "" {
		amount = "one_page"
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"direction": direction,
		"amount":    amount,
	}

	resp, err := s.hub.SendCommand("scroll", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Scrolled %s by %s", direction, amount)), nil
}

func (s *Server) handleExecuteJs(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	expression, ok := args["expression"].(string)
	if !ok || expression == "" {
		return mcplib.NewToolResultError("expression is required"), nil
	}

	returnValue := true
	if rv, ok := args["return_value"].(bool); ok {
		returnValue = rv
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"expression":   expression,
		"return_value": returnValue,
	}

	resp, err := s.hub.SendCommand("execute_js", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Execution result: %v", resp.Data)), nil
}

func (s *Server) handleWaitFor(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, ok := args["selector"].(string)
	if !ok || selector == "" {
		return mcplib.NewToolResultError("selector is required"), nil
	}

	state, _ := args["state"].(string)
	if state == "" {
		state = "visible"
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"selector": selector,
		"state":    state,
	}
	if timeout, ok := args["timeout"].(float64); ok && timeout > 0 {
		params["timeout"] = timeout
	}

	resp, err := s.hub.SendCommand("wait_for", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Element '%s' is now %s", selector, state)), nil
}

func (s *Server) handleGetCookies(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{}
	if url, ok := args["url"].(string); ok {
		params["url"] = url
	}
	if name, ok := args["name"].(string); ok {
		params["name"] = name
	}

	resp, err := s.hub.SendCommand("get_cookies", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Cookies: %v", resp.Data)), nil
}

func (s *Server) handleListTabs(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	resp, err := s.hub.SendCommand("list_tabs", nil, 0)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Tabs: %v", resp.Data)), nil
}

func (s *Server) handleSwitchTab(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	tabID, ok := args["tab_id"].(float64)
	if !ok || tabID == 0 {
		return mcplib.NewToolResultError("tab_id is required"), nil
	}

	params := map[string]any{}
	resp, err := s.hub.SendCommand("switch_tab", params, int(tabID))
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Switched to tab %d\nData: %v", int(tabID), resp.Data)), nil
}

func (s *Server) handleCloseTab(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	// tab_id is optional; if not provided, close the current tab
	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	resp, err := s.hub.SendCommand("close_tab", nil, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Closed tab %d", tabID)), nil
}

func (s *Server) handleNewTab(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	params := map[string]any{}
	if url, ok := args["url"].(string); ok {
		params["url"] = url
	}

	resp, err := s.hub.SendCommand("new_tab", params, 0)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("New tab: %v", resp.Data)), nil
}

func (s *Server) handleHover(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, ok := args["selector"].(string)
	if !ok || selector == "" {
		return mcplib.NewToolResultError("selector is required"), nil
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"selector": selector,
	}

	resp, err := s.hub.SendCommand("hover", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Hovered over: %s", selector)), nil
}

func (s *Server) handleSelectOption(ctx context.Context, req mcplib.CallToolRequest) (*mcplib.CallToolResult, error) {
	args := req.GetArguments()

	selector, ok := args["selector"].(string)
	if !ok || selector == "" {
		return mcplib.NewToolResultError("selector is required"), nil
	}
	value, ok := args["value"].(string)
	if !ok || value == "" {
		return mcplib.NewToolResultError("value is required"), nil
	}

	tabID, err := s.getActiveTabID(args)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	params := map[string]any{
		"selector": selector,
		"value":    value,
	}

	resp, err := s.hub.SendCommand("select_option", params, tabID)
	if err != nil {
		return mcplib.NewToolResultText(fmt.Sprintf("Error: %v", err)), nil
	}

	if resp.Status == "error" {
		return mcplib.NewToolResultText(fmt.Sprintf("Error [%s]: %s", resp.Code, resp.Error)), nil
	}

	return mcplib.NewToolResultText(fmt.Sprintf("Selected option in: %s", selector)), nil
}

// isNoExtensionError checks if an error is a "no extension connected" error.
func isNoExtensionError(err error) bool {
	return err != nil && err.Error() == "NO_EXTENSION_CONNECTED: Chrome Extension is not connected"
}
