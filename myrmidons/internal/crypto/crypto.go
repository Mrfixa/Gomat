package crypto

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
)

// GenerateRandomBytes generates cryptographically secure random bytes
func GenerateRandomBytes(n int) ([]byte, error) {
	b := make([]byte, n)
	_, err := rand.Read(b)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// GenerateRandomHex generates a random hex string
func GenerateRandomHex(n int) (string, error) {
	bytes, err := GenerateRandomBytes(n)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(bytes), nil
}

// EncryptAES encrypts data using AES-GCM
func EncryptAES(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// DecryptAES decrypts data using AES-GCM
func DecryptAES(key, nonce, ciphertext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	return gcm.Open(nil, nonce, ciphertext, nil)
}

// HashPassword hashes a password with SHA-256 (use bcrypt in production)
func HashPassword(password string) string {
	hash := sha256.Sum256([]byte(password))
	return hex.EncodeToString(hash[:])
}

// ConstantTimeCompare performs constant-time comparison of two strings
func ConstantTimeCompare(a, b string) bool {
	if len(a) != len(b) {
		return false
	}

	var result byte
	for i := 0; i < len(a); i++ {
		result |= a[i] ^ b[i]
	}
	return result == 0
}

// XMRAddress represents a Monero address
type XMRAddress struct {
	Public  string
	ViewKey string
	SpendKey string
}

// ValidateXMRAddress validates a Monero address
func ValidateXMRAddress(addr string) bool {
	// Standard Monero address is 95 characters
	if len(addr) != 95 {
		return false
	}

	// Must start with 4
	if addr[0] != '4' {
		return false
	}

	// Second character must be 0-9 or A-B
	c := addr[1]
	if !((c >= '0' && c <= '9') || (c >= 'A' && c <= 'B')) {
		return false
	}

	// Rest should be valid base58 characters
	validChars := "123456789ABCDEFGHJKLMNPQRSTUVWXYZabcdefghijkmnopqrstuvwxyz"
	for _, c := range addr[2:] {
		if !contains(validChars, c) {
			return false
		}
	}

	return true
}

func contains(s string, c rune) bool {
	for _, ch := range s {
		if ch == c {
			return true
		}
	}
	return false
}

// ParseXMRAddress parses an XMR address
func ParseXMRAddress(addr string) (*XMRAddress, error) {
	if !ValidateXMRAddress(addr) {
		return nil, fmt.Errorf("invalid Monero address")
	}
	return &XMRAddress{Public: addr}, nil
}

// AmountFromPiconero converts piconero to XMR
func AmountFromPiconero(piconero int64) float64 {
	return float64(piconero) / 1e12
}

// AmountToPiconero converts XMR to piconero
func AmountToPiconero(xmr float64) int64 {
	return int64(xmr * 1e12)
}

// FormatXMR formats XMR amount for display
func FormatXMR(piconero int64) string {
	xmr := float64(piconero) / 1e12
	return fmt.Sprintf("%.12f", xmr)
}
