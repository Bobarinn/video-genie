package api

import (
	"crypto/subtle"
	"net/http"
	"strings"
)

// APIKeyAuth is middleware that validates requests against a backend API key.
// It checks the X-API-Key header first, then falls back to Authorization: Bearer <key>.
//
// This design makes it trivial to swap to user bearer tokens later:
// just replace the key comparison with JWT/session validation logic.
// The handler signature and middleware chain stay identical.
func APIKeyAuth(apiKey string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Try X-API-Key header first (preferred for backend-to-backend calls)
			key := r.Header.Get("X-API-Key")

			// Fall back to Authorization: Bearer <key>
			if key == "" {
				authHeader := r.Header.Get("Authorization")
				if strings.HasPrefix(authHeader, "Bearer ") {
					key = strings.TrimPrefix(authHeader, "Bearer ")
				}
			}

			if key == "" {
				respondJSON(w, http.StatusUnauthorized, map[string]string{
					"error": "Missing API key. Provide X-API-Key header or Authorization: Bearer <key>",
				})
				return
			}

			// Constant-time comparison to prevent timing attacks
			if subtle.ConstantTimeCompare([]byte(key), []byte(apiKey)) != 1 {
				respondJSON(w, http.StatusForbidden, map[string]string{
					"error": "Invalid API key",
				})
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
