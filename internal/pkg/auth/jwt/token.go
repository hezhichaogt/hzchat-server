package jwt

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt"
)

const (
	// RoomAccessExpiration defines the duration for room-specific access tokens (short-term).
	RoomAccessExpiration = 15 * time.Minute

	// UserIdentityExpiration defines the duration for general user identity tokens (long-term).
	UserIdentityExpiration = 24 * time.Hour

	// TokenIssuer identifies the issuer of the token.
	TokenIssuer = "HZChat-Server"
)

// GenerateToken creates and signs a new JWT Token string based on the provided Payload struct.
func GenerateToken(payload *Payload, secretKey string, duration time.Duration) (string, error) {
	now := time.Now()

	payload.StandardClaims = jwt.StandardClaims{
		ExpiresAt: now.Add(duration).Unix(),
		IssuedAt:  now.Unix(),
		Issuer:    TokenIssuer,
	}

	token := jwt.NewWithClaims(jwt.SigningMethodHS256, payload)

	return token.SignedString([]byte(secretKey))
}

// ParseToken parses and validates the JWT Token string using the provided secretKey.
func ParseToken(tokenString string, secretKey string) (*Payload, error) {
	claims := &Payload{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("unexpected signing method")
		}
		return []byte(secretKey), nil
	})

	if err != nil {
		return nil, err
	}

	if !token.Valid {
		return nil, errors.New("invalid or expired token")
	}

	return claims, nil
}
