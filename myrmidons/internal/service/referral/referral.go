package referral

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service handles referral operations
type Service struct {
	db                   *pgxpool.Pool
	commissionPercentage decimal.Decimal
}

// NewService creates a new referral service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{
		db:                   db,
		commissionPercentage: decimal.NewFromFloat(0.10), // 10%
	}
}

// Referral represents a referral relationship
type Referral struct {
	ID                   uuid.UUID
	ReferrerID           uuid.UUID
	ReferredID           uuid.UUID
	ReferralCode         string
	CommissionEarnedPico decimal.Decimal
	CommissionClaimedPico decimal.Decimal
	FirstPurchaseCompleted bool
	CreatedAt            time.Time
}

// GenerateReferralCode generates a unique referral code
func (s *Service) GenerateReferralCode() (string, error) {
	bytes := make([]byte, 5)
	if _, err := rand.Read(bytes); err != nil {
		return "", err
	}
	return strings.ToUpper(hex.EncodeToString(bytes)), nil
}

// CreateReferral creates a referral relationship
func (s *Service) CreateReferral(ctx context.Context, referrerID, referredID uuid.UUID) (*Referral, error) {
	// Get referrer's code or generate one
	var code string
	err := s.db.QueryRow(ctx, `
		SELECT referral_code FROM users WHERE id = $1
	`, referrerID).Scan(&code)
	if err != nil {
		return nil, fmt.Errorf("referrer not found")
	}
	if code == "" {
		code, err = s.GenerateReferralCode()
		if err != nil {
			return nil, err
		}
		s.db.Exec(ctx, `UPDATE users SET referral_code = $1 WHERE id = $2`, code, referrerID)
	}

	referralID := uuid.New()
	_, err = s.db.Exec(ctx, `
		INSERT INTO referrals (id, referrer_id, referred_id, referral_code, created_at)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (referrer_id, referred_id) DO NOTHING
	`, referralID, referrerID, referredID, code, time.Now())
	if err != nil {
		return nil, err
	}

	return s.GetReferral(ctx, referralID)
}

// GetReferral retrieves a referral by ID
func (s *Service) GetReferral(ctx context.Context, id uuid.UUID) (*Referral, error) {
	var r Referral
	err := s.db.QueryRow(ctx, `
		SELECT id, referrer_id, referred_id, referral_code,
			commission_earned_pico, commission_claimed_pico,
			first_purchase_completed, created_at
		FROM referrals WHERE id = $1
	`, id).Scan(
		&r.ID, &r.ReferrerID, &r.ReferredID, &r.ReferralCode,
		&r.CommissionEarnedPico, &r.CommissionClaimedPico,
		&r.FirstPurchaseCompleted, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetByReferralCode retrieves a referral by code
func (s *Service) GetByReferralCode(ctx context.Context, code string) (*Referral, error) {
	var r Referral
	err := s.db.QueryRow(ctx, `
		SELECT id, referrer_id, referred_id, referral_code,
			commission_earned_pico, commission_claimed_pico,
			first_purchase_completed, created_at
		FROM referrals WHERE referral_code = $1
	`, code).Scan(
		&r.ID, &r.ReferrerID, &r.ReferredID, &r.ReferralCode,
		&r.CommissionEarnedPico, &r.CommissionClaimedPico,
		&r.FirstPurchaseCompleted, &r.CreatedAt,
	)
	if err != nil {
		return nil, err
	}
	return &r, nil
}

// GetReferralsByReferrer returns all referrals for a referrer
func (s *Service) GetReferralsByReferrer(ctx context.Context, referrerID uuid.UUID) ([]*Referral, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, referrer_id, referred_id, referral_code,
			commission_earned_pico, commission_claimed_pico,
			first_purchase_completed, created_at
		FROM referrals WHERE referrer_id = $1
		ORDER BY created_at DESC
	`, referrerID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var referrals []*Referral
	for rows.Next() {
		var r Referral
		rows.Scan(
			&r.ID, &r.ReferrerID, &r.ReferredID, &r.ReferralCode,
			&r.CommissionEarnedPico, &r.CommissionClaimedPico,
			&r.FirstPurchaseCompleted, &r.CreatedAt,
		)
		referrals = append(referrals, &r)
	}

	return referrals, nil
}

// RecordPurchase records a purchase and calculates commission
func (s *Service) RecordPurchase(ctx context.Context, referralID uuid.UUID, orderTotal decimal.Decimal) error {
	// Get referral
	referral, err := s.GetReferral(ctx, referralID)
	if err != nil {
		return err
	}

	// Calculate commission
	commission := orderTotal.Mul(s.commissionPercentage)

	// Update referral
	_, err = s.db.Exec(ctx, `
		UPDATE referrals
		SET commission_earned_pico = commission_earned_pico + $1,
			first_purchase_completed = true
		WHERE id = $2
	`, commission.BigInt(), referralID)
	if err != nil {
		return err
	}

	// Create payment record
	_, err = s.db.Exec(ctx, `
		INSERT INTO referral_payments (id, referral_id, order_id, commission_pico, created_at)
		VALUES ($1, $2, $3, $4, $5)
	`, uuid.New(), referralID, uuid.Nil, commission.BigInt(), time.Now())

	return err
}

// ClaimCommission transfers available commission to referrer's wallet
func (s *Service) ClaimCommission(ctx context.Context, referralID uuid.UUID) error {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	// Get unclaimed commission
	var earned, claimed decimal.Decimal
	var referrerID uuid.UUID
	err = tx.QueryRow(ctx, `
		SELECT referrer_id, commission_earned_pico, commission_claimed_pico
		FROM referrals WHERE id = $1
	`, referralID).Scan(&referrerID, &earned, &claimed)
	if err != nil {
		return err
	}

	unclaimed := earned.Sub(claimed)
	if unclaimed.LessThanOrEqual(decimal.Zero) {
		return fmt.Errorf("no commission to claim")
	}

	// Transfer to wallet
	_, err = tx.Exec(ctx, `
		UPDATE wallets
		SET balance_pico = balance_pico + $1, updated_at = $2
		WHERE user_id = $3
	`, unclaimed.BigInt(), time.Now(), referrerID)
	if err != nil {
		return err
	}

	// Update claimed amount
	_, err = tx.Exec(ctx, `
		UPDATE referrals
		SET commission_claimed_pico = commission_claimed_pico + $1
		WHERE id = $2
	`, unclaimed.BigInt(), referralID)
	if err != nil {
		return err
	}

	return tx.Commit(ctx)
}

// GetCommissionStats returns referral statistics for a user
func (s *Service) GetCommissionStats(ctx context.Context, userID uuid.UUID) (*CommissionStats, error) {
	var stats CommissionStats

	// Get totals
	err := s.db.QueryRow(ctx, `
		SELECT 
			COALESCE(SUM(commission_earned_pico), 0),
			COALESCE(SUM(commission_claimed_pico), 0),
			COUNT(*),
			COUNT(CASE WHEN first_purchase_completed THEN 1 END)
		FROM referrals WHERE referrer_id = $1
	`, userID).Scan(
		&stats.TotalEarned,
		&stats.TotalClaimed,
		&stats.TotalReferrals,
		&stats.CompletedReferrals,
	)
	if err != nil {
		return nil, err
	}

	stats.Available = stats.TotalEarned.Sub(stats.TotalClaimed)

	return &stats, nil
}

// CommissionStats holds referral statistics
type CommissionStats struct {
	TotalEarned       decimal.Decimal
	TotalClaimed      decimal.Decimal
	Available         decimal.Decimal
	TotalReferrals    int
	CompletedReferrals int
}

import "strings"
