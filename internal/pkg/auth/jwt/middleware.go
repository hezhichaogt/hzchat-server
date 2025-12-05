package jwt

import (
	"context"
	"net/http"
	"strings"

	"hzchat/internal/pkg/logx"
)

// Define Context Key for storing the Payload struct, preventing key collisions with other packages.
type contextKey string

const (
	// ContextAuthPayloadKey is the key used to store the parsed jwt.Payload (user identity) in the request Context.
	ContextAuthPayloadKey contextKey = "auth_payload"
)

// IdentityExtractorMiddleware attempts to extract and validate a JWT from the request header.
// It injects the Payload into the Context upon success. It does NOT interrupt the request
// (no 401 response) on failure or missing token, treating the user as anonymous instead.
func IdentityExtractorMiddleware(secretKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {

			// Extract Token from the Authorization header
			authHeader := r.Header.Get("Authorization")
			if authHeader == "" {
				// Token is missing. Treat as anonymous user and continue.
				next.ServeHTTP(w, r)
				return
			}

			// Expected format: "Bearer <token>"
			parts := strings.SplitN(authHeader, " ", 2)
			if len(parts) != 2 || parts[0] != "Bearer" {
				// Invalid format. Treat as anonymous user and continue.
				next.ServeHTTP(w, r)
				return
			}
			tokenString := parts[1]

			// Call ParseToken for validation
			payload, err := ParseToken(tokenString, secretKey)

			if err != nil {
				// Token exists but is invalid (e.g., expired, wrong signature).
				// We log the warning but treat the user as anonymous and continue.
				logx.Warn("Invalid or expired JWT provided, treating as anonymous", "error", err)
				next.ServeHTTP(w, r)
				return
			}

			// Inject Payload into Context
			ctx := context.WithValue(r.Context(), ContextAuthPayloadKey, payload)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetPayloadFromContext safely extracts the authenticated Payload from the request Context.
// In contexts where IdentityExtractorMiddleware is used, a nil return means the user is anonymous.
func GetPayloadFromContext(r *http.Request) *Payload {
	payload, ok := r.Context().Value(ContextAuthPayloadKey).(*Payload)

	if !ok {
		return nil
	}

	return payload
}
