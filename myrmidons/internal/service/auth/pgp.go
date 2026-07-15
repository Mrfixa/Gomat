package auth

import (
	"context"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// PGPService handles PGP encryption/decryption for 2FA
type PGPService struct {
	privateKey *rsa.PrivateKey
}

// NewPGPService creates a new PGP service
func NewPGPService(privateKeyPEM string) (*PGPService, error) {
	block, _ := pem.Decode([]byte(privateKeyPEM))
	if block == nil {
		return nil, fmt.Errorf("failed to decode PEM block")
	}

	key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("failed to parse private key: %w", err)
	}

	return &PGPService{privateKey: key}, nil
}

// GenerateToken generates a random 2FA token
func (s *PGPService) GenerateToken() (string, error) {
	bytes := make([]byte, 13) // 26 characters when base32 encoded
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return base64.StdEncoding.EncodeToString(bytes)[:26], nil
}

// Generate2FAMessage generates an encrypted 2FA message
func (s *PGPService) Generate2FAMessage(token, publicKeyPEM string) (string, error) {
	// Parse public key
	block, _ := pem.Decode([]byte(publicKeyPEM))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block")
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		// Try parsing as RSA public key
		rsaPub, err := x509.ParsePKCS1PublicKey(block.Bytes)
		if err != nil {
			return "", fmt.Errorf("failed to parse public key: %w", err)
		}
		pub = rsaPub
	}

	rsaPub, ok := pub.(*rsa.PublicKey)
	if !ok {
		return "", fmt.Errorf("not an RSA public key")
	}

	// Encrypt token with RSA-OAEP
	ciphertext, err := rsa.EncryptOAEP(
		sha256.New(),
		rand.Reader,
		rsaPub,
		[]byte(token),
		[]byte("myrmidons-2fa"),
	)
	if err != nil {
		return "", fmt.Errorf("encryption failed: %w", err)
	}

	return base64.StdEncoding.EncodeToString(ciphertext), nil
}

// Decrypt2FAToken decrypts a 2FA token
func (s *PGPService) Decrypt2FAToken(encryptedMessage string) (string, error) {
	ciphertext, err := base64.StdEncoding.DecodeString(encryptedMessage)
	if err != nil {
		return "", fmt.Errorf("failed to decode base64: %w", err)
	}

	plaintext, err := rsa.DecryptOAEP(
		sha256.New(),
		rand.Reader,
		s.privateKey,
		ciphertext,
		[]byte("myrmidons-2fa"),
	)
	if err != nil {
		return "", fmt.Errorf("decryption failed: %w", err)
	}

	return string(plaintext), nil
}

// VerifyToken verifies a 2FA token with constant-time comparison
func (s *PGPService) VerifyToken(expected, provided string) bool {
	if len(expected) != len(provided) {
		return false
	}

	var result byte
	for i := 0; i < len(expected); i++ {
		result |= expected[i] ^ provided[i]
	}

	return result == 0
}

// Session represents a 2FA session
type Session struct {
	ID        uuid.UUID
	UserID    uuid.UUID
	Token     string
	ExpiresAt time.Time
	Used      bool
}

// Create2FASession creates a pending 2FA session
func (s *Service) Create2FASession(ctx context.Context, userID uuid.UUID, pgpPublicKey string) (*Session, error) {
	// Generate token
	token, err := s.pgpService.GenerateToken()
	if err != nil {
		return nil, err
	}

	// Encrypt for user's PGP key
	encryptedMessage, err := s.pgpService.Generate2FAMessage(token, pgpPublicKey)
	if err != nil {
		return nil, err
	}

	session := &Session{
		ID:        uuid.New(),
		UserID:    userID,
		Token:     token,
		ExpiresAt: time.Now().Add(10 * time.Minute),
	}

	// Store session (encrypted message stored for verification)
	_, err = s.db.Exec(ctx, `
		INSERT INTO twofa_sessions (id, user_id, encrypted_token, expires_at)
		VALUES ($1, $2, $3, $4)
	`, session.ID, userID, encryptedMessage, session.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return session, nil
}

// Verify2FA verifies a 2FA code
func (s *Service) Verify2FA(ctx context.Context, sessionID uuid.UUID, code string) (bool, error) {
	var session Session
	err := s.db.QueryRow(ctx, `
		SELECT id, token, expires_at, used FROM twofa_sessions WHERE id = $1
	`, sessionID).Scan(&session.ID, &session.Token, &session.ExpiresAt, &session.Used)
	if err != nil {
		return false, fmt.Errorf("session not found")
	}

	if session.Used {
		return false, fmt.Errorf("session already used")
	}

	if time.Now().After(session.ExpiresAt) {
		return false, fmt.Errorf("session expired")
	}

	// Verify with constant-time comparison
	valid := s.pgpService.VerifyToken(session.Token, code)

	if valid {
		// Mark session as used
		s.db.Exec(ctx, `UPDATE twofa_sessions SET used = true WHERE id = $1`, sessionID)
	}

	return valid, nil
}

// pgpService reference (would be initialized in NewService)
var pgpService *PGPService

func init() {
	// Placeholder - would be initialized with actual key
}

// JailService handles bot protection via CAPTCHA
type JailService struct {
	db *pgxpool.Pool
}

// NewJailService creates a new jail service
func NewJailService(db *pgxpool.Pool) *JailService {
	return &JailService{db: db}
}

// Challenge represents a CAPTCHA challenge
type Challenge struct {
	ID           uuid.UUID
	OnionAddress string
	Solution     string
	HiddenChars  []int
	CreatedAt    time.Time
	ExpiresAt    time.Time
	Used        bool
}

// GenerateChallenge creates a new CAPTCHA challenge
func (s *JailService) GenerateChallenge(ctx context.Context, onionAddress string) (*Challenge, error) {
	// Generate random solution (4 characters from address)
	solution := make([]byte, 4)
	indices := make([]int, 4)
	
	// Get indices 5, 10, 15, 20 of the onion address
	positions := []int{5, 10, 15, 20}
	for i, pos := range positions {
		if pos < len(onionAddress) {
			solution[i] = onionAddress[pos]
			indices[i] = pos
		} else {
			solution[i] = 'X'
			indices[i] = -1
		}
	}

	challenge := &Challenge{
		ID:           uuid.New(),
		OnionAddress: onionAddress,
		Solution:     string(solution),
		HiddenChars:  indices,
		CreatedAt:    time.Now(),
		ExpiresAt:    time.Now().Add(5 * time.Minute),
	}

	// Store challenge
	_, err := s.db.Exec(ctx, `
		INSERT INTO captcha_challenges (id, onion_address, solution, hidden_indices, expires_at)
		VALUES ($1, $2, $3, $4, $5)
	`, challenge.ID, onionAddress, challenge.Solution, indices, challenge.ExpiresAt)
	if err != nil {
		return nil, err
	}

	return challenge, nil
}

// VerifyChallenge verifies a CAPTCHA solution
func (s *JailService) VerifyChallenge(ctx context.Context, challengeID uuid.UUID, solution string) (bool, error) {
	var challenge Challenge
	err := s.db.QueryRow(ctx, `
		SELECT id, solution, expires_at, used FROM captcha_challenges WHERE id = $1
	`, challengeID).Scan(&challenge.ID, &challenge.Solution, &challenge.ExpiresAt, &challenge.Used)
	if err != nil {
		return false, fmt.Errorf("challenge not found")
	}

	if challenge.Used {
		return false, fmt.Errorf("challenge already used")
	}

	if time.Now().After(challenge.ExpiresAt) {
		return false, fmt.Errorf("challenge expired")
	}

	// Normalize and compare
	solution = strings.ToUpper(strings.TrimSpace(solution))
	expected := strings.ToUpper(challenge.Solution)

	valid := solution == expected

	if valid {
		s.db.Exec(ctx, `UPDATE captcha_challenges SET used = true WHERE id = $1`, challengeID)
	}

	return valid, nil
}

// IsJailed checks if an IP is currently jailed
func (s *JailService) IsJailed(ctx context.Context, ipAddress string) (bool, error) {
	var count int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM jail_records
		WHERE ip_address = $1::inet AND released_at IS NULL AND expires_at > NOW()
	`, ipAddress).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

// ReleaseFromJail releases an IP from jail
func (s *JailService) ReleaseFromJail(ctx context.Context, ipAddress string) error {
	_, err := s.db.Exec(ctx, `
		UPDATE jail_records SET released_at = NOW()
		WHERE ip_address = $1::inet AND released_at IS NULL
	`, ipAddress)
	return err
}

// ProofOfWork represents a PoW challenge
type ProofOfWork struct {
	Prefix   string
	Difficulty int
	ExpiresAt time.Time
}

// GeneratePoW creates a new PoW challenge
func (s *JailService) GeneratePoW(ctx context.Context) (*ProofOfWork, error) {
	// Generate random prefix
	prefixBytes := make([]byte, 8)
	rand.Read(prefixBytes)
	prefix := hex.EncodeToString(prefixBytes)

	pow := &ProofOfWork{
		Prefix:    prefix,
		Difficulty: 16, // 16 leading zero bits
		ExpiresAt: time.Now().Add(5 * time.Minute),
	}

	return pow, nil
}

// VerifyPoW verifies a PoW solution
func (s *JailService) VerifyPoW(prefix string, solution string, difficulty int) bool {
	combined := prefix + solution
	hash := sha256.Sum256([]byte(combined))
	
	// Check leading zero bits
	required := big.NewInt(1)
	required.Lsh(required, 256-difficulty)
	
	hashInt := new(big.Int).SetBytes(hash[:])
	return hashInt.Cmp(required) <= 0
}
