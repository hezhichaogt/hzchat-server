package jwt

import "github.com/golang-jwt/jwt"

// Payload defines the structure of the JSON Web Token (JWT) claims for HZ Chat.
// It includes standard claims required by the JWT specification and custom claims
// necessary for identifying and authorizing users within the chat system.
type Payload struct {
	// StandardClaims embeds the necessary JWT standard fields such as Exp (Expiration),
	// Iat (Issued At), and Iss (Issuer). These are crucial for token validity checks.
	jwt.StandardClaims `json:"standard_claims"`

	// ID is the unified identifier for the participant, which can be a system-generated
	// Guest ID or a registered User ID, depending on the UserType.
	ID string `json:"id"`

	// Code specifies the chat room the token holder is currently authorized to access.
	// This is vital for security and context-specific authorization within REST API calls.
	Code string `json:"code"`

	// UserType defines the role of the participant, allowing the server to apply
	// different logic and permissions (e.g., "guest", "registered", or "subscriber").
	UserType string `json:"user_type"`
}
