package auth

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestValidate(t *testing.T) {
	tests := []struct {
		name      string
		provided  string
		expected  string
		wantValid bool
	}{
		{"matching tokens", "abc123", "abc123", true},
		{"mismatched tokens", "abc123", "def456", false},
		{"empty provided", "", "abc123", false},
		{"empty expected", "abc123", "", false},
		{"both empty", "", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := Validate(tt.provided, tt.expected); got != tt.wantValid {
				t.Errorf("Validate(%q, %q) = %v, want %v", tt.provided, tt.expected, got, tt.wantValid)
			}
		})
	}
}

func TestHTTPMiddleware(t *testing.T) {
	tests := []struct {
		name          string
		authHeader    string
		expectedToken string
		wantStatus    int
	}{
		{
			name:          "valid token",
			authHeader:    "Bearer valid-token",
			expectedToken: "valid-token",
			wantStatus:    http.StatusOK,
		},
		{
			name:          "missing header",
			authHeader:    "",
			expectedToken: "valid-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "wrong format",
			authHeader:    "Basic valid-token",
			expectedToken: "valid-token",
			wantStatus:    http.StatusUnauthorized,
		},
		{
			name:          "wrong token",
			authHeader:    "Bearer wrong-token",
			expectedToken: "valid-token",
			wantStatus:    http.StatusUnauthorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
			})
			middleware := HTTPMiddleware(handler, tt.expectedToken)

			req := httptest.NewRequest(http.MethodGet, "/", nil)
			if tt.authHeader != "" {
				req.Header.Set("Authorization", tt.authHeader)
			}
			rec := httptest.NewRecorder()
			middleware.ServeHTTP(rec, req)

			if rec.Code != tt.wantStatus {
				t.Errorf("expected status %d, got %d", tt.wantStatus, rec.Code)
			}
		})
	}
}

func TestIsAllowedOrigin(t *testing.T) {
	tests := []struct {
		name              string
		origin            string
		allowedExtensions []string
		want              bool
	}{
		{
			name:              "empty whitelist allows all",
			origin:            "chrome-extension://abc123",
			allowedExtensions: []string{},
			want:              true,
		},
		{
			name:              "matching extension",
			origin:            "chrome-extension://known-ext-id",
			allowedExtensions: []string{"known-ext-id"},
			want:              true,
		},
		{
			name:              "non-matching extension",
			origin:            "chrome-extension://unknown-ext-id",
			allowedExtensions: []string{"known-ext-id"},
			want:              false,
		},
		{
			name:              "empty origin",
			origin:            "",
			allowedExtensions: []string{"known-ext-id"},
			want:              false,
		},
		{
			name:              "non-chrome origin",
			origin:            "https://evil.com",
			allowedExtensions: []string{"known-ext-id"},
			want:              false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsAllowedOrigin(tt.origin, tt.allowedExtensions); got != tt.want {
				t.Errorf("IsAllowedOrigin(%q, %v) = %v, want %v", tt.origin, tt.allowedExtensions, got, tt.want)
			}
		})
	}
}
