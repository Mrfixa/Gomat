package reserves

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Service handles provable reserves
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new reserves service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// GenerateProof generates a new reserve proof
func (s *Service) GenerateProof(ctx context.Context, viewKey string, totalBalance decimal.Decimal) (*ReserveProof, error) {
	proofID := uuid.New()
	now := time.Now()

	// Calculate total user balances
	var userBalances string
	err := s.db.QueryRow(ctx, `
		SELECT COALESCE(SUM(balance_pico), 0)::TEXT FROM wallets
	`).Scan(&userBalances)
	if err != nil {
		return nil, fmt.Errorf("failed to calculate user balances: %w", err)
	}
	userBalancesDecimal, _ := decimal.NewFromString(userBalances)

	// Get insurance balance
	var insuranceBalance string
	s.db.QueryRow(ctx, `SELECT COALESCE(balance_pico, 0)::TEXT FROM insurance_fund WHERE id = 1`).Scan(&insuranceBalance)
	insuranceDecimal, _ := decimal.NewFromString(insuranceBalance)

	// Get operator bond
	var bondAmount string
	s.db.QueryRow(ctx, `SELECT COALESCE(amount_pico, 0)::TEXT FROM operator_bonds WHERE status = 'active' LIMIT 1`).Scan(&bondAmount)
	bondDecimal, _ := decimal.NewFromString(bondAmount)

	// Generate signature (placeholder - in production use proper cryptographic signing)
	signature := fmt.Sprintf("SIG-%s-%d", proofID.String(), now.Unix())

	// Generate Merkle root (placeholder)
	merkleRoot := fmt.Sprintf("MR-%s-%d", proofID.String()[:8], now.Unix())

	// Store proof
	_, err = s.db.Exec(ctx, `
		INSERT INTO reserve_proofs (
			id, view_key, total_balance_pico, user_balances_pico,
			insurance_balance_pico, operator_bond_pico,
			timestamp, signature, block_hash, block_height, merkle_root
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11)
	`, proofID, viewKey, totalBalance.BigInt(), userBalancesDecimal.BigInt(),
		insuranceDecimal.BigInt(), bondDecimal.BigInt(),
		now, signature, "BLOCKHASH", 1234567, merkleRoot)
	if err != nil {
		return nil, fmt.Errorf("failed to store proof: %w", err)
	}

	return &ReserveProof{
		ID:                  proofID,
		TotalBalancePico:    totalBalance,
		UserBalancesPico:    userBalancesDecimal,
		InsuranceBalancePico: insuranceDecimal,
		OperatorBondPico:    bondDecimal,
		Timestamp:           now,
		Signature:           signature,
		MerkleRoot:          merkleRoot,
	}, nil
}

// GetLatestProof returns the most recent reserve proof
func (s *Service) GetLatestProof(ctx context.Context) (*ReserveProof, error) {
	var proof ReserveProof
	err := s.db.QueryRow(ctx, `
		SELECT id, total_balance_pico, user_balances_pico,
			insurance_balance_pico, operator_bond_pico,
			timestamp, signature, merkle_root
		FROM reserve_proofs
		ORDER BY timestamp DESC
		LIMIT 1
	`).Scan(
		&proof.ID, &proof.TotalBalancePico, &proof.UserBalancesPico,
		&proof.InsuranceBalancePico, &proof.OperatorBondPico,
		&proof.Timestamp, &proof.Signature, &proof.MerkleRoot,
	)
	if err != nil {
		return nil, fmt.Errorf("no reserve proof found: %w", err)
	}
	return &proof, nil
}

// GetProofHistory returns reserve proof history
func (s *Service) GetProofHistory(ctx context.Context, limit int) ([]*ReserveProof, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, total_balance_pico, user_balances_pico,
			insurance_balance_pico, operator_bond_pico,
			timestamp, signature, merkle_root
		FROM reserve_proofs
		ORDER BY timestamp DESC
		LIMIT $1
	`, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to query proofs: %w", err)
	}
	defer rows.Close()

	var proofs []*ReserveProof
	for rows.Next() {
		var proof ReserveProof
		err := rows.Scan(
			&proof.ID, &proof.TotalBalancePico, &proof.UserBalancesPico,
			&proof.InsuranceBalancePico, &proof.OperatorBondPico,
			&proof.Timestamp, &proof.Signature, &proof.MerkleRoot,
		)
		if err != nil {
			continue
		}
		proofs = append(proofs, &proof)
	}
	return proofs, nil
}

// VerifyProof verifies a reserve proof (placeholder)
func (s *Service) VerifyProof(ctx context.Context, proofID uuid.UUID, userID uuid.UUID) error {
	// Record verification
	_, err := s.db.Exec(ctx, `
		INSERT INTO reserve_verifications (id, proof_id, user_id)
		VALUES ($1, $2, $3)
	`, uuid.New(), proofID, userID)
	if err != nil {
		return fmt.Errorf("failed to record verification: %w", err)
	}
	return nil
}

// ReserveProof represents a reserve proof
type ReserveProof struct {
	ID                  uuid.UUID
	TotalBalancePico    decimal.Decimal
	UserBalancesPico    decimal.Decimal
	InsuranceBalancePico decimal.Decimal
	OperatorBondPico    decimal.Decimal
	Timestamp           time.Time
	Signature           string
	MerkleRoot          string
}

// UserProofData holds data for individual user proof verification
type UserProofData struct {
	UserID       uuid.UUID
	BalancePico  decimal.Decimal
	MerkleProof  string
}
