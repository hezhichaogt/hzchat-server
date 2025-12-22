/*
Package randx provides functions for generating cryptographically secure random numbers and unique identifiers.

It is primarily used to generate fixed-length Base62 encoded room codes and standard UUID message IDs.
*/
package randx

import (
	"crypto/rand"
	"fmt"
	"math/big"
	"strings"

	"github.com/google/uuid"
)

const (
	// Base62Chars defines the character set used for Base62 encoding (0-9, A-Z, a-z).
	Base62Chars = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZabcdefghijklmnopqrstuvwxyz"

	// Base62Len is the total number of characters in the Base62 character set (62).
	Base62Len = int64(len(Base62Chars))

	// RoomCodeLength is the fixed length required for the generated room code.
	RoomCodeLength = 6

	// GuestIDPrefix is the required prefix for client-generated guest IDs.
	GuestIDPrefix = "guest_"

	// GuestIDRawLength is the fixed length of the Base62 part of the GuestID.
	GuestIDRawLength = 6
)

// RoomCode generates a Base62 encoded room code using a cryptographically secure random number generator (crypto/rand).
// It returns a string of length RoomCodeLength and any error encountered.
func RoomCode() (string, error) {
	length := RoomCodeLength

	result := make([]byte, length)

	for i := range length {
		num, err := rand.Int(rand.Reader, big.NewInt(Base62Len))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number for room code: %v", err)
		}

		result[i] = Base62Chars[num.Int64()]
	}

	return string(result), nil
}

// MessageID generates a standard UUID v4 string to serve as a unique identifier for a message.
func MessageID() string {
	return uuid.New().String()
}

// IsValidRoomCode checks if the given string is a valid room code.
// Validity criteria include: length equals RoomCodeLength and all characters belong to the Base62Chars set.
func IsValidRoomCode(code string) bool {
	if len(code) != RoomCodeLength {
		return false
	}

	for _, char := range code {
		if !strings.ContainsRune(Base62Chars, char) {
			return false
		}
	}

	return true
}

// IsValidGuestID checks if the given string is a valid Guest ID.
func IsValidGuestID(id string) bool {
	if !strings.HasPrefix(id, GuestIDPrefix) {
		return false
	}

	rawID := id[len(GuestIDPrefix):]

	if len(rawID) != GuestIDRawLength {
		return false
	}

	for _, char := range rawID {
		if !strings.ContainsRune(Base62Chars, char) {
			return false
		}
	}

	return true
}

// UserNickname generates a random nickname with a "User_" prefix and 6 random Base62 characters.
func UserNickname() (string, error) {
	const nicknameRandomLength = 6
	result := make([]byte, nicknameRandomLength)

	for i := range nicknameRandomLength {
		num, err := rand.Int(rand.Reader, big.NewInt(Base62Len))
		if err != nil {
			return "", fmt.Errorf("failed to generate random number for nickname: %v", err)
		}
		result[i] = Base62Chars[num.Int64()]
	}

	return "User_" + string(result), nil
}
