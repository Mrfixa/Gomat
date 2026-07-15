package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/go-playground/bcrypt"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

// Service handles authentication
type Service struct {
	db              *pgxpool.Pool
	maxLoginAttempts int
	lockoutDuration  time.Duration
}

// NewService creates a new auth service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:              db,
		maxLoginAttempts: 5,
		lockoutDuration:  15 * time.Minute,
	}
}

// User represents a user for authentication
type User struct {
	ID           uuid.UUID
	Username     string
	PasswordHash string
	PGPKey       string
	TwoFAEnabled bool
	IsLocked     bool
	Locale       string
}

// Register creates a new user account
func (s *Service) Register(ctx context.Context, username, password, pgpKey string) (*User, error) {
	// Validate username (3-20 chars, alphanumeric + underscore)
	if len(username) < 3 || len(username) > 20 {
		return nil, fmt.Errorf("username must be 3-20 characters")
	}

	// Check if username exists
	var exists bool
	err := s.db.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE username = $1)`, username).Scan(&exists)
	if err != nil {
		return nil, fmt.Errorf("database error: %w", err)
	}
	if exists {
		return nil, fmt.Errorf("username already taken")
	}

	// Hash password
	hashedPassword, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %w", err)
	}

	// Generate salt
	salt := make([]byte, 16)
	rand.Read(salt)

	userID := uuid.New()
	now := time.Now()

	// Insert user
	_, err = s.db.Exec(ctx, `
		INSERT INTO users (id, username, password_hash, salt, pgp_key, twofa_enabled, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $7)
	`, userID, username, string(hashedPassword), hex.EncodeToString(salt), pgpKey, pgpKey != "", now)
	if err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Create wallet
	_, err = s.db.Exec(ctx, `
		INSERT INTO wallets (user_id, balance_pico, locked_balance_pico, updated_at)
		VALUES ($1, 0, 0, $2)
	`, userID, now)
	if err != nil {
		return nil, fmt.Errorf("failed to create wallet: %w", err)
	}

	return &User{
		ID:       userID,
		Username: username,
		PGPKey:   pgpKey,
		TwoFAEnabled: pgpKey != "",
	}, nil
}

// Login authenticates a user
func (s *Service) Login(ctx context.Context, username, password, ipAddress string) (*User, error) {
	// Get user
	var user User
	err := s.db.QueryRow(ctx, `
		SELECT id, username, password_hash, pgp_key, twofa_enabled
		FROM users WHERE username = $1
	`, username).Scan(&user.ID, &user.Username, &user.PasswordHash, &user.PGPKey, &user.TwoFAEnabled)
	if err != nil {
		// Log failed attempt anyway
		s.logLoginAttempt(ctx, nil, ipAddress, username, false)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Check if locked
	if user.IsLocked {
		return nil, fmt.Errorf("account is locked")
	}

	// Check password
	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(password)); err != nil {
		s.logLoginAttempt(ctx, &user.ID, ipAddress, username, false)
		s.checkAndLockAccount(ctx, &user)
		return nil, fmt.Errorf("invalid credentials")
	}

	// Log successful attempt
	s.logLoginAttempt(ctx, &user.ID, ipAddress, username, true)

	return &user, nil
}

// logLoginAttempt records a login attempt
func (s *Service) logLoginAttempt(ctx context.Context, userID *uuid.UUID, ipAddress, username string, success bool) {
	s.db.Exec(ctx, `
		INSERT INTO login_attempts (id, user_id, ip_address, username, success, attempted_at)
		VALUES ($1, $2, $3::inet, $4, $5, $6)
	`, uuid.New(), userID, ipAddress, username, success, time.Now())
}

// checkAndLockAccount checks failed attempts and locks if needed
func (s *Service) checkAndLockAccount(ctx context.Context, user *User) {
	var attempts int
	err := s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM login_attempts
		WHERE user_id = $1 AND success = false AND attempted_at > $2
	`, user.ID, time.Now().Add(-s.lockoutDuration)).Scan(&attempts)
	if err != nil {
		return
	}

	if attempts >= s.maxLoginAttempts {
		s.db.Exec(ctx, `UPDATE users SET is_locked = true WHERE id = $1`, user.ID)
	}
}

// CheckPasswordHistory checks if password was recently used
func (s *Service) CheckPasswordHistory(ctx context.Context, userID uuid.UUID, password string) error {
	rows, err := s.db.Query(ctx, `
		SELECT password_hash FROM password_history
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT 5
	`, userID)
	if err != nil {
		return nil // No history is fine
	}
	defer rows.Close()

	for rows.Next() {
		var hash string
		rows.Scan(&hash)
		if bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil {
			return fmt.Errorf("password was recently used")
		}
	}

	return nil
}

// AddToPasswordHistory adds password to history
func (s *Service) AddToPasswordHistory(ctx context.Context, userID uuid.UUID, password string) error {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), 10)
	if err != nil {
		return err
	}

	_, err = s.db.Exec(ctx, `
		INSERT INTO password_history (user_id, password_hash, created_at)
		VALUES ($1, $2, $3)
	`, userID, string(hash), time.Now())

	// Keep only last 5
	s.db.Exec(ctx, `
		DELETE FROM password_history
		WHERE user_id = $1 AND id NOT IN (
			SELECT id FROM password_history
			WHERE user_id = $1
			ORDER BY created_at DESC
			LIMIT 5
		)
	`, userID)

	return err
}

// ChangePassword changes a user's password
func (s *Service) ChangePassword(ctx context.Context, userID uuid.UUID, oldPassword, newPassword string) error {
	// Get current hash
	var currentHash string
	err := s.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, userID).Scan(&currentHash)
	if err != nil {
		return fmt.Errorf("user not found")
	}

	// Verify old password
	if bcrypt.CompareHashAndPassword([]byte(currentHash), []byte(oldPassword)) != nil {
		return fmt.Errorf("current password is incorrect")
	}

	// Check history
	if err := s.CheckPasswordHistory(ctx, userID, newPassword); err != nil {
		return err
	}

	// Hash new password
	newHash, err := bcrypt.GenerateFromPassword([]byte(newPassword), 10)
	if err != nil {
		return err
	}

	// Update password
	_, err = s.db.Exec(ctx, `
		UPDATE users SET password_hash = $1, updated_at = $2 WHERE id = $3
	`, string(newHash), time.Now(), userID)
	if err != nil {
		return err
	}

	// Add to history
	return s.AddToPasswordHistory(ctx, userID, newPassword)
}

// GenerateSession creates a new session for a user
func (s *Service) GenerateSession(ctx context.Context, userID uuid.UUID, ipAddress, userAgent string) (string, error) {
	// Generate random token
	tokenBytes := make([]byte, 32)
	rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	tokenHash := fmt.Sprintf("%x", tokenBytes[:16]) // Store partial hash

	// Session expires in 6 hours
	expiresAt := time.Now().Add(6 * time.Hour)

	_, err := s.db.Exec(ctx, `
		INSERT INTO sessions (id, user_id, token_hash, ip_address, user_agent, expires_at, created_at)
		VALUES ($1, $2, $3, $4::inet, $5, $6, $7)
	`, uuid.New(), userID, tokenHash, ipAddress, userAgent, expiresAt, time.Now())
	if err != nil {
		return "", fmt.Errorf("failed to create session: %w", err)
	}

	return token, nil
}

// ValidateSession validates a session token
func (s *Service) ValidateSession(ctx context.Context, token string) (*User, error) {
	// For simplicity, we'll just check if token is valid format
	// In production, hash the token and look up
	if len(token) != 64 {
		return nil, fmt.Errorf("invalid session")
	}

	// This is a placeholder - implement proper session validation
	return nil, fmt.Errorf("session validation not implemented")
}

// Logout invalidates a session
func (s *Service) Logout(ctx context.Context, token string) error {
	// In production, delete session from database
	_ = token
	return nil
}
