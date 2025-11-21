/*
Package pow implements the Proof-of-Work (PoW) mechanism, intended for rate limiting
or anti-abuse measures on client requests.

It manages the generation and validation of nonces and the issuance of temporary
Proof Tokens upon successful validation.
*/
package pow

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

const (
	// TokenHeaderKey is the HTTP header key used by the client to send the Proof Token.
	TokenHeaderKey = "X-PoW-Token"

	// ProofTokenDuration is the validity period for the Proof Token issued after successful PoW validation.
	ProofTokenDuration = 30 * time.Second

	// NonceExpiryDuration is the validity period for the challenge Nonce.
	NonceExpiryDuration = 5 * time.Minute
)

// PoWManager is responsible for managing the lifecycle of PoW challenges and Proof Tokens.
// It is concurrent-safe, using internal maps to store active nonces and tokens.
type PoWManager struct {
	// difficulty is the required number of leading zeros for the PoW challenge hash.
	difficulty int

	// nonceStore stores active nonces and their expiration times.
	nonceStore map[string]time.Time

	// tokenStore stores issued Proof Tokens and their expiration times.
	tokenStore map[string]time.Time

	// mu protects concurrent access to nonceStore and tokenStore.
	mu sync.RWMutex
}

// NewPoWManager creates and initializes a new PoWManager instance.
// It accepts the challenge difficulty and starts a background goroutine to clean up expired entries.
func NewPoWManager(difficulty int) *PoWManager {
	mgr := &PoWManager{
		difficulty: difficulty,
		nonceStore: make(map[string]time.Time),
		tokenStore: make(map[string]time.Time),
	}

	go mgr.cleanupExpiredEntries()

	return mgr
}

// GenerateNonce generates a unique Nonce string for the PoW challenge and stores it for validation.
// Returns the newly generated Nonce.
func (m *PoWManager) GenerateNonce() string {
	m.mu.Lock()
	defer m.mu.Unlock()

	nonce := uuid.New().String()
	m.nonceStore[nonce] = time.Now().Add(NonceExpiryDuration)
	return nonce
}

// ValidateProof validates the PoW proof provided by the client.
// It checks if the Nonce is valid and unexpired, and verifies if the SHA256 hash of the
// Nonce + Counter combination meets the difficulty requirement (number of leading zeros).
// If validation succeeds, it issues and returns a temporary Proof Token.
func (m *PoWManager) ValidateProof(nonce, counter string) (string, error) {
	m.mu.RLock()
	expiryTime, ok := m.nonceStore[nonce]
	m.mu.RUnlock()

	if !ok || time.Now().After(expiryTime) {
		return "", fmt.Errorf("nonce expired or invalid")
	}

	input := fmt.Sprintf("%s%s", nonce, counter)
	hash := sha256.Sum256([]byte(input))
	hashStr := hex.EncodeToString(hash[:])

	requiredPrefix := strings.Repeat("0", m.difficulty)
	if !strings.HasPrefix(hashStr, requiredPrefix) {
		return "", fmt.Errorf("proof does not meet difficulty requirement")
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	if _, stillExists := m.nonceStore[nonce]; !stillExists {
		return "", fmt.Errorf("nonce consumed by concurrent request")
	}

	delete(m.nonceStore, nonce)

	token := uuid.New().String()
	m.tokenStore[token] = time.Now().Add(ProofTokenDuration)
	return token, nil
}

// CheckProofToken checks if the request carries a valid Proof Token.
// The Proof Token can be located in the HTTP header (X-PoW-Token) or the URL query parameter (pow_token).
func (m *PoWManager) CheckProofToken(r *http.Request) bool {
	token := r.Header.Get(TokenHeaderKey)
	if token == "" {
		token = r.URL.Query().Get("pow_token")
	}

	if token == "" {
		return false
	}

	m.mu.RLock()
	expiryTime, ok := m.tokenStore[token]
	m.mu.RUnlock()

	if !ok || time.Now().After(expiryTime) {
		return false
	}

	return true
}

// cleanupExpiredEntries periodically cleans up expired entries in both nonceStore and tokenStore.
// This method is started as a background goroutine in NewPoWManager.
func (m *PoWManager) cleanupExpiredEntries() {
	ticker := time.NewTicker(time.Minute)
	defer ticker.Stop()

	for range ticker.C {
		m.mu.Lock()
		now := time.Now()

		for nonce, expiry := range m.nonceStore {
			if now.After(expiry) {
				delete(m.nonceStore, nonce)
			}
		}

		for token, expiry := range m.tokenStore {
			if now.After(expiry) {
				delete(m.tokenStore, token)
			}
		}
		m.mu.Unlock()
	}
}
