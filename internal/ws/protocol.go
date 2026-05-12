package ws

import "fmt"

// Request is a command sent from the MCP Server to the Chrome Extension
// via WebSocket.
type Request struct {
	ID     string      `json:"id"`
	Method string      `json:"method"`
	Params interface{} `json:"params,omitempty"`
	TabID  int         `json:"tab_id"`
}

// Response is a result returned by the Chrome Extension to the MCP Server
// via WebSocket.
type Response struct {
	ID     string      `json:"id"`
	Status string      `json:"status"`            // "success" or "error"
	Data   interface{} `json:"data,omitempty"`    // response data
	Code   string      `json:"code,omitempty"`    // error code
	Error  string      `json:"error,omitempty"`   // error message
}

// Method constants — all supported WebSocket commands.
const (
	MethodNavigate       = "navigate"
	MethodScreenshot     = "screenshot"
	MethodGetContent     = "get_content"
	MethodClick          = "click"
	MethodType           = "type"
	MethodScroll         = "scroll"
	MethodExecuteJS      = "execute_js"
	MethodWaitFor        = "wait_for"
	MethodGetCookies     = "get_cookies"
	MethodListTabs       = "list_tabs"
	MethodSwitchTab      = "switch_tab"
	MethodCloseTab       = "close_tab"
	MethodNewTab         = "new_tab"
	MethodHover          = "hover"
	MethodSelectOption   = "select_option"
)

// Error codes for typed error responses.
const (
	ErrCodeElementNotFound      = "ELEMENT_NOT_FOUND"
	ErrCodeTimeout              = "TIMEOUT"
	ErrCodeNavigationError      = "NAVIGATION_ERROR"
	ErrCodeJSExecutionError     = "JS_EXECUTION_ERROR"
	ErrCodeTabNotFound          = "TAB_NOT_FOUND"
	ErrCodeTabClosed            = "TAB_CLOSED"
	ErrCodeNoExtensionConnected = "NO_EXTENSION_CONNECTED"
	ErrCodePermissionDenied     = "PERMISSION_DENIED"
	ErrCodeUnknownMethod        = "UNKNOWN_METHOD"
)

// Predefined error types for the Hub.
var (
	ErrNoExtensionConnected = fmt.Errorf("%s: Chrome Extension is not connected", ErrCodeNoExtensionConnected)
	ErrTimeout              = fmt.Errorf("%s: operation timed out", ErrCodeTimeout)
)

// ErrorCodes maps error codes to human-readable explanations.
var ErrorCodes = map[string]string{
	ErrCodeElementNotFound:      "CSS selector did not match any element. Check selector or wait and retry.",
	ErrCodeTimeout:              "Operation timed out. Increase timeout parameter and retry.",
	ErrCodeNavigationError:      "Page navigation failed. Check URL, may need login.",
	ErrCodeJSExecutionError:     "JavaScript execution error. Fix the code and retry.",
	ErrCodeTabNotFound:          "Tab ID does not exist. Re-run list_tabs to get correct IDs.",
	ErrCodeTabClosed:            "Tab has been closed. Use a different tab.",
	ErrCodeNoExtensionConnected: "Chrome Extension is not connected. Ask user to check Extension.",
	ErrCodePermissionDenied:     "Permission denied. Check if tab is active.",
	ErrCodeUnknownMethod:        "Unknown command. Check method name.",
}
