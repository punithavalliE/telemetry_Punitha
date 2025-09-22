package security

import (
	"crypto/subtle"
	"net/http"
	"os"
	"strings"
)

// APIKeyMiddleware validates API keys for incoming requests
func APIKeyMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health checks, metrics, and Swagger documentation
		if r.URL.Path == "/health" ||
			r.URL.Path == "/metrics" ||
			r.URL.Path == "/topics" ||
			strings.HasPrefix(r.URL.Path, "/swagger/") ||
			r.URL.Path == "/swagger" ||
			r.URL.Path == "/docs" ||
			strings.HasPrefix(r.URL.Path, "/docs/") ||
			r.URL.Path == "/swagger-ui/" ||
			r.URL.Path == "/swagger/index.html" ||
			r.URL.Path == "/swagger.json" ||
			r.URL.Path == "/swagger.yaml" {
			next.ServeHTTP(w, r)
			return
		}

		apiKey := r.Header.Get("X-API-Key")
		if apiKey == "" {
			// Try Authorization header
			auth := r.Header.Get("Authorization")
			if strings.HasPrefix(auth, "Bearer ") {
				apiKey = strings.TrimPrefix(auth, "Bearer ")
			}
		}

		if !validateAPIKey(apiKey) {
			http.Error(w, "Unauthorized: Invalid API key", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// validateAPIKey checks if the provided API key is valid
func validateAPIKey(apiKey string) bool {
	validKey := os.Getenv("API_KEY")
	if validKey == "" {
		validKey = "telemetry-api-secret-2025"
	}

	// Use constant-time comparison to prevent timing attacks
	return subtle.ConstantTimeCompare([]byte(apiKey), []byte(validKey)) == 1
}

// ServiceAuthMiddleware validates service-to-service communication
func ServiceAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Skip auth for health checks
		if r.URL.Path == "/health" || r.URL.Path == "/topics" {
			next.ServeHTTP(w, r)
			return
		}

		serviceToken := r.Header.Get("X-Service-Token")
		if !validateServiceToken(serviceToken) {
			http.Error(w, "Unauthorized: Invalid service token", http.StatusUnauthorized)
			return
		}

		next.ServeHTTP(w, r)
	})
}

func validateServiceToken(token string) bool {
	validToken := os.Getenv("SERVICE_TOKEN")
	if validToken == "" {
		validToken = "service-internal-token-change-in-production"
	}

	return subtle.ConstantTimeCompare([]byte(token), []byte(validToken)) == 1
}
