package wallet

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
)

// AdvisoryLock provides database-level locking for wallet operations
type AdvisoryLock struct {
	db *pgxpool.Pool
}

// NewAdvisoryLock creates a new advisory lock manager
func NewAdvisoryLock(db *pgxpool.Pool) *AdvisoryLock {
	return &AdvisoryLock{db: db}
}

// LockWallet acquires an exclusive advisory lock for a wallet
// Returns a function to release the lock when done
func (l *AdvisoryLock) LockWallet(ctx context.Context, walletID uuid.UUID) (func(), error) {
	// Generate a consistent lock ID from the wallet UUID
	// PostgreSQL advisory locks use int64 keys
	lockID := int64FromUUID(walletID)

	// Acquire the lock
	_, err := l.db.Exec(ctx, "SELECT pg_advisory_lock($1)", lockID)
	if err != nil {
		return nil, fmt.Errorf("failed to acquire wallet lock: %w", err)
	}

	// Return unlock function
	return func() {
		l.db.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	}, nil
}

// TryLockWallet attempts to acquire a lock without blocking
func (l *AdvisoryLock) TryLockWallet(ctx context.Context, walletID uuid.UUID) (bool, func(), error) {
	lockID := int64FromUUID(walletID)

	var acquired bool
	err := l.db.QueryRow(ctx, "SELECT pg_try_advisory_lock($1)", lockID).Scan(&acquired)
	if err != nil {
		return false, nil, fmt.Errorf("failed to try lock: %w", err)
	}

	if !acquired {
		return false, nil, nil
	}

	return true, func() {
		l.db.Exec(ctx, "SELECT pg_advisory_unlock($1)", lockID)
	}, nil
}

// int64FromUUID converts a UUID to int64 for advisory lock key
// Uses the first 8 bytes of the UUID
func int64FromUUID(id uuid.UUID) int64 {
	var result int64
	for i := 0; i < 8; i++ {
		result = result<<8 | int64(id[i])
	}
	return result
}

// WithWalletLock executes a function while holding a wallet lock
func (l *AdvisoryLock) WithWalletLock(ctx context.Context, walletID uuid.UUID, fn func() error) error {
	unlock, err := l.LockWallet(ctx, walletID)
	if err != nil {
		return err
	}
	defer unlock()

	return fn()
}
