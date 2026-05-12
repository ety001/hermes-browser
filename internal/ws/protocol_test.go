package ws

import (
	"encoding/json"
	"testing"
)

func TestRequestMarshal(t *testing.T) {
	req := Request{
		ID:     "test-id-1",
		Method: "navigate",
		Params: map[string]interface{}{
			"url": "https://example.com",
		},
		TabID: 42,
	}

	data, err := json.Marshal(req)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Request
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ID != "test-id-1" {
		t.Errorf("expected ID 'test-id-1', got '%s'", decoded.ID)
	}
	if decoded.Method != "navigate" {
		t.Errorf("expected Method 'navigate', got '%s'", decoded.Method)
	}
	if decoded.TabID != 42 {
		t.Errorf("expected TabID 42, got %d", decoded.TabID)
	}
}

func TestResponseMarshal(t *testing.T) {
	resp := Response{
		ID:     "test-id-1",
		Status: "success",
		Data: map[string]interface{}{
			"url":   "https://example.com",
			"title": "Example",
		},
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.ID != "test-id-1" {
		t.Errorf("expected ID 'test-id-1', got '%s'", decoded.ID)
	}
	if decoded.Status != "success" {
		t.Errorf("expected Status 'success', got '%s'", decoded.Status)
	}
	if decoded.Code != "" {
		t.Errorf("expected empty Code, got '%s'", decoded.Code)
	}
}

func TestErrorResponse(t *testing.T) {
	resp := Response{
		ID:     "test-id-2",
		Status: "error",
		Code:   "ELEMENT_NOT_FOUND",
		Error:  "Element matching selector '#btn' not found",
	}

	data, err := json.Marshal(resp)
	if err != nil {
		t.Fatal(err)
	}

	var decoded Response
	if err := json.Unmarshal(data, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded.Status != "error" {
		t.Errorf("expected Status 'error', got '%s'", decoded.Status)
	}
	if decoded.Code != "ELEMENT_NOT_FOUND" {
		t.Errorf("expected Code 'ELEMENT_NOT_FOUND', got '%s'", decoded.Code)
	}
}

func TestMethodConstants(t *testing.T) {
	methods := []string{
		MethodNavigate, MethodScreenshot, MethodGetContent, MethodClick,
		MethodType, MethodScroll, MethodExecuteJS, MethodWaitFor,
		MethodGetCookies, MethodListTabs, MethodSwitchTab, MethodCloseTab,
		MethodNewTab, MethodHover, MethodSelectOption,
	}
	for _, m := range methods {
		if m == "" {
			t.Error("empty method constant found")
		}
	}
}

func TestErrorCodeConstants(t *testing.T) {
	codes := []string{
		ErrCodeElementNotFound, ErrCodeTimeout, ErrCodeNavigationError,
		ErrCodeJSExecutionError, ErrCodeTabNotFound, ErrCodeTabClosed,
		ErrCodeNoExtensionConnected, ErrCodePermissionDenied, ErrCodeUnknownMethod,
	}
	for _, c := range codes {
		if c == "" {
			t.Error("empty error code constant found")
		}
		if _, ok := ErrorCodes[c]; !ok {
			t.Errorf("error code %s missing from ErrorCodes map", c)
		}
	}
}

func TestErrorCodesNonEmpty(t *testing.T) {
	for code, desc := range ErrorCodes {
		if desc == "" {
			t.Errorf("empty description for error code %s", code)
		}
	}
}
