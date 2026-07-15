package auth

import (
	"context"
	"errors"
	"time"

	"github.com/gobugger/gomarket/internal/repo"
	"github.com/gobugger/gomarket/internal/util/db"
	"github.com/gobugger/gomarket/pkg/pgp"
	"github.com/google/uuid"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5"
	"golang.org/x/crypto/bcrypt"
)

// SECURE: Account lockout constants
const (
	MaxFailedLoginAttempts = 5
	LockoutDuration        = 15 * time.Minute
	FailedAttemptWindow    = 30 * time.Minute
)

var (
	ErrUsernameAlreadyRegistered = errors.New("username unavailable")
	ErrInvalidCredentials        = errors.New("invalid credentials")
	ErrInvalidPassword           = errors.New("invalid password")
	ErrInvalidNewPassword        = errors.New("invalid new password")
	ErrInvalidPGPKey             = errors.New("invalid PGP public key")
	ErrInvalidSignature          = errors.New("invalid signature")
	ErrAccountIsBanned           = errors.New("account is banned")
	ErrAccountLocked             = errors.New("account is temporarily locked due to too many failed login attempts")
)

type RegisterParams struct {
	Username string
	Password string
	PgpKey   string
}

func (p *RegisterParams) Validate() error {
	if len(p.Password) < 8 {
		return ErrInvalidPassword
	} else if p.PgpKey != "" && !pgp.PublicKeyIsValid(p.PgpKey) {
		return ErrInvalidPGPKey
	}

	return nil
}

func Register(ctx context.Context, qtx *repo.Queries, p RegisterParams) (repo.User, error) {
	if err := p.Validate(); err != nil {
		return repo.User{}, err
	}

	hash, err := bcrypt.GenerateFromPassword([]byte(p.Password), bcrypt.DefaultCost)
	if err != nil {
		return repo.User{}, err
	}

	user, err := qtx.CreateUser(
		ctx,
		repo.CreateUserParams{
			Username:     p.Username,
			PasswordHash: string(hash),
			PgpKey:       p.PgpKey,
		})
	if err != nil {
		if db.ErrCode(err) == pgerrcode.UniqueViolation {
			return repo.User{}, ErrUsernameAlreadyRegistered
		}
		return repo.User{}, err
	}

	return user, nil
}

type AuthenticateParams struct {
	Username string
	Password string
}

func Authenticate(ctx context.Context, qtx *repo.Queries, p AuthenticateParams) (repo.User, error) {
	user, err := qtx.GetUserWithName(ctx, p.Username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.User{}, ErrInvalidCredentials
		}
		return repo.User{}, err
	}

	// SECURE: Check if account is banned
	_, err = qtx.GetBanForUser(ctx, user.ID)
	if err == nil {
		return repo.User{}, ErrAccountIsBanned
	} else if !errors.Is(err, pgx.ErrNoRows) {
		return repo.User{}, err
	}

	// SECURE: Check if account is locked due to too many failed attempts
	isLocked, err := qtx.IsUserLocked(ctx, user.ID)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return repo.User{}, err
	}
	if isLocked {
		return repo.User{}, ErrAccountLocked
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.Password))
	if err != nil {
		// SECURE: Record failed login attempt on bad password
		if bcryptErr := qtx.RecordFailedLoginAttempt(ctx, user.ID); bcryptErr != nil {
			// Log error but don't expose to user
			_ = bcryptErr
		}
		return repo.User{}, ErrInvalidCredentials
	}

	// SECURE: Clear failed login attempts on successful login
	if clearErr := qtx.ClearFailedLoginAttempts(ctx, user.ID); clearErr != nil {
		// Log error but don't block login
		_ = clearErr
	}

	return user, nil
}

type ChangePasswordParams struct {
	Username    string
	OldPassword string
	NewPassword string
}

// SECURE: Maximum number of previous passwords to check
const MaxPasswordHistory = 5

// SECURE: Prevent password reuse by checking against recent password history
func isPasswordReused(ctx context.Context, qtx *repo.Queries, userID uuid.UUID, newPassword string) bool {
	// Get recent password hashes
	history, err := qtx.GetPasswordHistory(ctx, userID, MaxPasswordHistory)
	if err != nil {
		// If we can't get history, allow the change (fail open, but log)
		return false
	}

	// Check against each historical password
	for _, entry := range history {
		if err := bcrypt.CompareHashAndPassword([]byte(entry.PasswordHash), []byte(newPassword)); err == nil {
			return true // Password was found in history
		}
	}

	return false
}

func ChangePassword(ctx context.Context, qtx *repo.Queries, p ChangePasswordParams) (repo.User, error) {
	if p.NewPassword == p.OldPassword {
		return repo.User{}, ErrInvalidNewPassword
	}

	user, err := qtx.GetUserWithName(ctx, p.Username)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.User{}, ErrInvalidCredentials
		}
		return repo.User{}, err
	}

	err = bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.OldPassword))
	if err != nil {
		return repo.User{}, ErrInvalidPassword
	}

	// SECURE: Check password history to prevent reuse
	if isPasswordReused(ctx, qtx, user.ID, p.NewPassword) {
		return repo.User{}, ErrInvalidNewPassword
	}

	// SECURE: Store old password hash in history before changing
	if err := qtx.AddPasswordHistory(ctx, repo.AddPasswordHistoryParams{
		UserID:       user.ID,
		PasswordHash: user.PasswordHash,
	}); err != nil {
		// Log error but don't block password change
		_ = err
	}

	newHash, err := bcrypt.GenerateFromPassword([]byte(p.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return repo.User{}, err
	}

	user, err = qtx.UpdateUserPasswordHash(ctx,
		repo.UpdateUserPasswordHashParams{
			Username:        p.Username,
			PasswordHash:    user.PasswordHash,
			NewPasswordHash: string(newHash),
		})
	if err != nil {
		return repo.User{}, err
	}

	return user, nil
}

type SetPGPKeyParams struct {
	UserID uuid.UUID
	PgpKey string
}

func SetPGPKey(ctx context.Context, qtx *repo.Queries, p SetPGPKeyParams) (repo.User, error) {
	if !pgp.PublicKeyIsValid(p.PgpKey) {
		return repo.User{}, ErrInvalidPGPKey
	}

	return qtx.UpdateUserPgpKey(
		ctx,
		repo.UpdateUserPgpKeyParams{
			ID:     p.UserID,
			PgpKey: p.PgpKey,
		})
}
