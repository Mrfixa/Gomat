package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// InsuranceFund manages the insurance fund
type InsuranceFund struct {
	db *pgxpool.Pool
}

// NewInsuranceFund creates a new insurance fund service
func NewInsuranceFund(db *pgxpool.Pool) *InsuranceFund {
	return &InsuranceFund{db: db}
}

// GetBalance returns the current insurance fund balance
func (s *InsuranceFund) GetBalance(ctx context.Context) (decimal.Decimal, error) {
	var balance string
	err := s.db.QueryRow(ctx, `SELECT balance_pico FROM insurance_fund WHERE id = 1`).Scan(&balance)
	if err != nil {
		return decimal.Zero, fmt.Errorf("failed to get balance: %w", err)
	}
	bd, _ := decimal.NewFromString(balance)
	return bd, nil
}

// Contribute adds to the insurance fund
func (s *InsuranceFund) Contribute(ctx context.Context, amount decimal.Decimal) error {
	_, err := s.db.Exec(ctx, `
		UPDATE insurance_fund 
		SET balance_pico = balance_pico + $1,
			total_contributions_pico = total_contributions_pico + $1,
			updated_at = $2
		WHERE id = 1
	`, amount.BigInt(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to contribute: %w", err)
	}
	return nil
}

// Withdraw removes from the insurance fund (for payouts)
func (s *InsuranceFund) Withdraw(ctx context.Context, amount decimal.Decimal) error {
	// Check sufficient balance
	current, err := s.GetBalance(ctx)
	if err != nil {
		return err
	}

	if current.LessThan(amount) {
		return fmt.Errorf("insufficient insurance fund balance")
	}

	_, err = s.db.Exec(ctx, `
		UPDATE insurance_fund 
		SET balance_pico = balance_pico - $1,
			total_claims_paid_pico = total_claims_paid_pico + $1,
			updated_at = $2
		WHERE id = 1
	`, amount.BigInt(), time.Now())
	if err != nil {
		return fmt.Errorf("failed to withdraw: %w", err)
	}
	return nil
}

// GetStats returns insurance fund statistics
func (s *InsuranceFund) GetStats(ctx context.Context) (*InsuranceStats, error) {
	var balance, contributions, claims string
	err := s.db.QueryRow(ctx, `
		SELECT balance_pico, total_contributions_pico, total_claims_paid_pico
		FROM insurance_fund WHERE id = 1
	`).Scan(&balance, &contributions, &claims)
	if err != nil {
		return nil, fmt.Errorf("failed to get stats: %w", err)
	}

	balanceDecimal, _ := decimal.NewFromString(balance)
	contributionsDecimal, _ := decimal.NewFromString(contributions)
	claimsDecimal, _ := decimal.NewFromString(claims)

	return &InsuranceStats{
		Balance:            balanceDecimal,
		TotalContributions: contributionsDecimal,
		TotalClaimsPaid:    claimsDecimal,
	}, nil
}

// InsuranceStats holds insurance fund statistics
type InsuranceStats struct {
	Balance            decimal.Decimal
	TotalContributions decimal.Decimal
	TotalClaimsPaid    decimal.Decimal
}
