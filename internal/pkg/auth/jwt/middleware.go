package jwt

import (
	"context"
	"net/http"
	"strings"
)

// Define Context Key for storing the Payload struct, preventing key collisions with other packages.
type contextKey string

const (
	// ContextAuthPayloadKey is the key used to store the parsed jwt.Payload (user identity) in the request Context.
	ContextAuthPayloadKey contextKey = "auth_payload"
)

// IdentityExtractorMiddleware is an HTTP middleware that extracts and validates a JWT from the request.
// If a valid token is found, the corresponding Payload is injected into the request Context.
// If no token is found or if the token is invalid, the request proceeds as anonymous (no Payload in Context).
func IdentityExtractorMiddleware(secretKey string) func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			authHeader := r.Header.Get("Authorization")

			var tokenString string

			if authHeader == "" {
				if r.URL.Path != "/api/file/presign-download" {
					next.ServeHTTP(w, r)
					return
				}

				token := r.URL.Query().Get("t")
				if token == "" {
					next.ServeHTTP(w, r)
					return
				}
				tokenString = token

			} else {
				parts := strings.SplitN(authHeader, " ", 2)
				if len(parts) != 2 || parts[0] != "Bearer" {
					next.ServeHTTP(w, r)
					return
				}
				tokenString = parts[1]
			}

			payload, err := ParseToken(tokenString, secretKey)

			if err != nil {
				next.ServeHTTP(w, r)
				return
			}

			ctx := context.WithValue(r.Context(), ContextAuthPayloadKey, payload)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// GetPayloadFromContext safely extracts the authenticated Payload from the request Context.
func GetPayloadFromContext(r *http.Request) *Payload {
	payload, ok := r.Context().Value(ContextAuthPayloadKey).(*Payload)

	if !ok {
		return nil
	}

	return payload
}
