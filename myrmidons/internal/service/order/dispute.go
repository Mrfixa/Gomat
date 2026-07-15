package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
)

// DisputeService handles order disputes
type DisputeService struct {
	db *pgxpool.Pool
}

// NewDisputeService creates a new dispute service
func NewDisputeService(db *pgxpool.Pool) *DisputeService {
	return &DisputeService{db: db}
}

// CreateDispute opens a dispute for an order
func (s *DisputeService) CreateDispute(ctx context.Context, orderID uuid.UUID, buyerStatement string) error {
	// Verify order is in dispute-able state
	var status string
	err := s.db.QueryRow(ctx, `SELECT status FROM orders WHERE id = $1`, orderID).Scan(&status)
	if err != nil {
		return fmt.Errorf("order not found: %w", err)
	}

	if status != string(StatusDispatched) {
		return fmt.Errorf("order cannot be disputed in status: %s", status)
	}

	disputeID := uuid.New()
	now := time.Now()

	_, err = s.db.Exec(ctx, `
		INSERT INTO disputes (id, order_id, status, buyer_statement, created_at)
		VALUES ($1, $2, 'open', $3, $4)
	`, disputeID, orderID, buyerStatement, now)
	if err != nil {
		return fmt.Errorf("failed to create dispute: %w", err)
	}

	// Update order status
	_, err = s.db.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = $2 WHERE id = $3
	`, StatusDisputed, now, orderID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Assign jury members
	if err := s.assignJury(ctx, disputeID); err != nil {
		return fmt.Errorf("failed to assign jury: %w", err)
	}

	return nil
}

// assignJury assigns random jurors to a dispute
func (s *DisputeService) assignJury(ctx context.Context, disputeID uuid.UUID) error {
	// Get jury size from settings
	var jurySize int
	err := s.db.QueryRow(ctx, `SELECT jury_size FROM jury_settings LIMIT 1`).Scan(&jurySize)
	if err != nil {
		jurySize = 3 // Default
	}

	// Get available jurors (not locked, with bond)
	rows, err := s.db.Query(ctx, `
		SELECT user_id FROM jurors
		WHERE locked = false AND pending_cases < 3
		ORDER BY RANDOM()
		LIMIT $1
	`, jurySize)
	if err != nil {
		return fmt.Errorf("failed to query jurors: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var jurorID uuid.UUID
		if err := rows.Scan(&jurorID); err != nil {
			continue
		}

		seed := fmt.Sprintf("%s-%d", disputeID.String(), jurorID.String())

		_, err = s.db.Exec(ctx, `
			INSERT INTO jury_assignments (id, dispute_id, juror_id, seed)
			VALUES ($1, $2, $3, $4)
		`, uuid.New(), disputeID, jurorID, seed)
		if err != nil {
			continue
		}

		// Increment pending cases
		s.db.Exec(ctx, `UPDATE jurors SET pending_cases = pending_cases + 1 WHERE user_id = $1`, jurorID)
		count++
	}

	// Update dispute status to voting
	_, err = s.db.Exec(ctx, `
		UPDATE disputes SET status = 'voting' WHERE id = $1
	`, disputeID)
	if err != nil {
		return fmt.Errorf("failed to update dispute status: %w", err)
	}

	return nil
}

// RecordVote records a jury vote
func (s *DisputeService) RecordVote(ctx context.Context, disputeID, jurorID uuid.UUID, vote string, signature string) error {
	now := time.Now()

	result, err := s.db.Exec(ctx, `
		UPDATE jury_assignments 
		SET vote = $1, vote_signature = $2, voted_at = $3
		WHERE dispute_id = $4 AND juror_id = $5 AND vote IS NULL
	`, vote, signature, now, disputeID, jurorID)
	if err != nil {
		return fmt.Errorf("failed to record vote: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("vote already recorded or not assigned")
	}

	// Check if all votes are in
	if err := s.checkAndResolveDispute(ctx, disputeID); err != nil {
		return fmt.Errorf("failed to resolve dispute: %w", err)
	}

	return nil
}

// checkAndResolveDispute checks if voting is complete and resolves the dispute
func (s *DisputeService) checkAndResolveDispute(ctx context.Context, disputeID uuid.UUID) error {
	// Get required votes count
	var requiredVotes int
	err := s.db.QueryRow(ctx, `SELECT jury_size FROM jury_settings LIMIT 1`).Scan(&requiredVotes)
	if err != nil {
		requiredVotes = 3
	}

	// Count recorded votes
	var recordedVotes int
	err = s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM jury_assignments
		WHERE dispute_id = $1 AND vote IS NOT NULL
	`, disputeID).Scan(&recordedVotes)
	if err != nil {
		return err
	}

	if recordedVotes < requiredVotes {
		return nil // Still waiting for votes
	}

	// Count votes
	var buyerVotes, vendorVotes int
	s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM jury_assignments
		WHERE dispute_id = $1 AND vote = 'buyer'
	`, disputeID).Scan(&buyerVotes)
	s.db.QueryRow(ctx, `
		SELECT COUNT(*) FROM jury_assignments
		WHERE dispute_id = $1 AND vote = 'vendor'
	`, disputeID).Scan(&vendorVotes)

	// Determine outcome
	var ruling string
	var refundAmount *int64

	if buyerVotes > vendorVotes {
		ruling = "buyer"
		// Get order amount for refund
		var amount string
		s.db.QueryRow(ctx, `
			SELECT total_price_pico FROM orders WHERE id = (
				SELECT order_id FROM disputes WHERE id = $1
			)
		`, disputeID).Scan(&amount)
		if amount != "" {
			var amt int64 = 0
			fmt.Sscanf(amount, "%d", &amt)
			refundAmount = &amt
		}
	} else {
		ruling = "vendor"
	}

	// Update dispute
	_, err = s.db.Exec(ctx, `
		UPDATE disputes SET 
			status = 'resolved',
			ruling = $1,
			refund_amount_pico = $2,
			resolved_at = $3
		WHERE id = $4
	`, ruling, refundAmount, time.Now(), disputeID)
	if err != nil {
		return fmt.Errorf("failed to resolve dispute: %w", err)
	}

	// Update order status
	var newStatus Status
	if ruling == "buyer" {
		newStatus = StatusRefunded
	} else {
		newStatus = StatusFinalized
	}

	_, err = s.db.Exec(ctx, `
		UPDATE orders SET status = $1, updated_at = $2 WHERE id = (
			SELECT order_id FROM disputes WHERE id = $3
		)
	`, newStatus, time.Now(), disputeID)
	if err != nil {
		return fmt.Errorf("failed to update order status: %w", err)
	}

	// Unlock jurors
	s.db.Exec(ctx, `
		UPDATE jurors SET 
			pending_cases = GREATEST(0, pending_cases - 1),
			honest_votes = honest_votes + CASE WHEN vote = $1 THEN 1 ELSE 0 END,
			dishonest_votes = dishonest_votes + CASE WHEN vote != $1 THEN 1 ELSE 0 END
		WHERE user_id IN (
			SELECT juror_id FROM jury_assignments WHERE dispute_id = $2
		)
	`, ruling, disputeID)

	return nil
}

// GetDispute retrieves a dispute by ID
func (s *DisputeService) GetDispute(ctx context.Context, id uuid.UUID) (*Dispute, error) {
	var dispute Dispute
	err := s.db.QueryRow(ctx, `
		SELECT id, order_id, status, buyer_statement, vendor_statement,
			refund_amount_pico, ruling, created_at, resolved_at
		FROM disputes WHERE id = $1
	`, id).Scan(
		&dispute.ID, &dispute.OrderID, &dispute.Status,
		&dispute.BuyerStatement, &dispute.VendorStatement,
		&dispute.RefundAmountPico, &dispute.Ruling,
		&dispute.CreatedAt, &dispute.ResolvedAt,
	)
	if err != nil {
		return nil, fmt.Errorf("dispute not found: %w", err)
	}
	return &dispute, nil
}

// Dispute represents a dispute
type Dispute struct {
	ID              uuid.UUID
	OrderID         uuid.UUID
	Status          string
	BuyerStatement  string
	VendorStatement string
	RefundAmountPico *int64
	Ruling          string
	CreatedAt       time.Time
	ResolvedAt      *time.Time
}

// GetJuryAssignments retrieves jury assignments for a dispute
func (s *DisputeService) GetJuryAssignments(ctx context.Context, disputeID uuid.UUID) ([]JuryAssignment, error) {
	rows, err := s.db.Query(ctx, `
		SELECT ja.id, ja.dispute_id, ja.juror_id, ja.vote, ja.voted_at,
			u.username
		FROM jury_assignments ja
		JOIN users u ON ja.juror_id = u.id
		WHERE ja.dispute_id = $1
	`, disputeID)
	if err != nil {
		return nil, fmt.Errorf("failed to query assignments: %w", err)
	}
	defer rows.Close()

	var assignments []JuryAssignment
	for rows.Next() {
		var a JuryAssignment
		err := rows.Scan(&a.ID, &a.DisputeID, &a.JurorID, &a.Vote, &a.VotedAt, &a.JurorUsername)
		if err != nil {
			continue
		}
		assignments = append(assignments, a)
	}
	return assignments, nil
}

// JuryAssignment represents a jury assignment
type JuryAssignment struct {
	ID             uuid.UUID
	DisputeID      uuid.UUID
	JurorID        uuid.UUID
	Vote           *string
	VotedAt        *time.Time
	JurorUsername  string
}
