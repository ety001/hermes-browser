package auth

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// Validate compares two tokens using constant-time comparison to prevent
// timing side-channel attacks.
func Validate(provided, expected string) bool {
	if provided == "" || expected == "" {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// HTTPMiddleware returns a standard net/http middleware that validates
// Bearer token in the Authorization header.
func HTTPMiddleware(next http.Handler, expectedToken string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader := r.Header.Get("Authorization")
		if authHeader == "" {
			http.Error(w, `{"error":"missing authorization header"}`, http.StatusUnauthorized)
			return
		}

		if !strings.HasPrefix(authHeader, "Bearer ") {
			http.Error(w, `{"error":"invalid authorization format, expected Bearer <token>"}`, http.StatusUnauthorized)
			return
		}

		provided := strings.TrimPrefix(authHeader, "Bearer ")
		if !Validate(provided, expectedToken) {
			http.Error(w, `{"error":"invalid token"}`, http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// IsAllowedOrigin checks if the WebSocket Origin header matches one of the
// allowed Chrome Extension IDs.
// Origin format: "chrome-extension://<extension-id>"
func IsAllowedOrigin(origin string, allowedExtensions []string) bool {
	if len(allowedExtensions) == 0 {
		return true
	}
	for _, extID := range allowedExtensions {
		expected := "chrome-extension://" + extID
		if subtle.ConstantTimeCompare([]byte(origin), []byte(expected)) == 1 {
			return true
		}
	}
	return false
}
