package jwt

import "github.com/golang-jwt/jwt"

type Payload struct {
	jwt.StandardClaims `json:"standard_claims"`

	// ID is the unified identifier for the participant, which can be a system-generated
	// Guest ID or a registered User ID, depending on the UserType.
	ID string `json:"id"`

	// Code optionally specifies the chat room the token holder is authorized to access.
	// When present, it elevates the token from a general identity credential to a
	// room-specific access token. It is omitted in long-term identity tokens.
	Code string `json:"code,omitempty"`

	// UserType defines the role of the participant, allowing the server to apply
	// different logic and permissions (e.g., "guest", "registered", or "subscriber").
	UserType string `json:"userType"`

	Nickname string `json:"nickname,omitempty"`
	Avatar   string `json:"avatar,omitempty"`
}
