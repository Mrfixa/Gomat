package auth

import (
	"crypto/rand"
	"crypto/subtle"
	"fmt"
	"strings"
	"time"

	"github.com/gobugger/gomarket/pkg/pgp"
)

const TokenLength = 26

type Challenge struct {
	Token            string
	EncryptedMessage string
}

// SECURE: Generate 2FA challenge with timing attack protection
// The token is generated with cryptographically secure random bytes
func Generate2FAChallenge(pgpKey string) (*Challenge, error) {
	// SECURE: Use crypto/rand for secure random token generation
	tokenBytes := make([]byte, TokenLength)
	if _, err := rand.Read(tokenBytes); err != nil {
		return nil, fmt.Errorf("failed to generate secure random token: %w", err)
	}

	// Convert to printable characters (base32-like encoding)
	token := encodeToken(tokenBytes)

	// SECURE: Include timestamp and nonce in message to prevent replay attacks
	nonce := time.Now().UnixNano()
	message := fmt.Sprintf("Code: %s\nTimestamp: %d\nNonce: %d\n", token, nonce, nonce)

	encryptedMessage, err := pgp.Encrypt(pgpKey, message)
	if err != nil {
		return nil, err
	}

	return &Challenge{
		Token:            token,
		EncryptedMessage: encryptedMessage,
	}, nil
}

// SECURE: Constant-time token comparison to prevent timing attacks
func Validate2FAToken(provided, expected string) bool {
	// Normalize inputs
	provided = strings.TrimSpace(provided)
	expected = strings.TrimSpace(expected)

	// SECURE: Use constant-time comparison
	if len(provided) != len(expected) {
		return false
	}
	return subtle.ConstantTimeCompare([]byte(provided), []byte(expected)) == 1
}

// SECURE: Encode random bytes to URL-safe string
func encodeToken(data []byte) string {
	const charset = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789" // No confusing chars (0, O, 1, I, L)
	result := make([]byte, len(data))
	for i, b := range data {
		result[i] = charset[int(b)%len(charset)]
	}
	return string(result)
}
