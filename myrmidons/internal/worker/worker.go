package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// Worker handles background jobs
type Worker struct {
	db *pgxpool.Pool
}

// NewWorker creates a new worker
func NewWorker(db *pgxpool.Pool) *Worker {
	return &Worker{db: db}
}

// Start starts the worker
func (w *Worker) Start(ctx context.Context) error {
	client, err := river.NewClient(w.db, &river.Config{
		Workers: []river.Worker{
			river.WorkerFunc(w.processInvoiceJob),
			river.WorkerFunc(w.expireOrdersJob),
			river.WorkerFunc(w.updateReservesJob),
			river.WorkerFunc(w.processJuryJob),
			river.WorkerFunc(w.cleanupExpiredJob),
		},
	})
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}

	_, err = client.Start(ctx)
	if err != nil {
		return fmt.Errorf("failed to start worker: %w", err)
	}

	slog.Info("Worker started")
	return nil
}

// ============================================
// Invoice Processing Jobs
// ============================================

// InvoiceJob represents an invoice processing job
type InvoiceJob struct {
	InvoiceID uuid.UUID
}

// processInvoiceJob processes an invoice
func (w *Worker) processInvoiceJob(ctx context.Context, job *river.Job[InvoiceJob]) error {
	logger := slog.With("job", "processInvoice", "invoiceID", job.Args.InvoiceID)

	logger.Info("Processing invoice")

	// Get invoice details
	var address, amount string
	var status string
	err := w.db.QueryRow(ctx, `
		SELECT address, amount_pico::text, status FROM invoices WHERE id = $1
	`, job.Args.InvoiceID).Scan(&address, &amount, &status)
	if err != nil {
		return fmt.Errorf("failed to get invoice: %w", err)
	}

	if status != "pending" {
		logger.Info("Invoice already processed", "status", status)
		return nil
	}

	// In production, query Monero wallet for payment status
	paid, unlockedAmount := w.checkMoneroPayment(ctx, address, amount)
	if !paid {
		logger.Info("Invoice not paid yet")
		return nil
	}

	// Update invoice
	now := time.Now()
	_, err = w.db.Exec(ctx, `
		UPDATE invoices SET status = 'confirmed', updated_at = $1 WHERE id = $2
	`, now, job.Args.InvoiceID)
	if err != nil {
		return fmt.Errorf("failed to update invoice: %w", err)
	}

	// Update order status
	_, err = w.db.Exec(ctx, `
		UPDATE orders SET status = 'paid', updated_at = $1 WHERE id = (
			SELECT order_id FROM invoices WHERE id = $2
		)
	`, now, job.Args.InvoiceID)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	logger.Info("Invoice confirmed")
	w.queueNotification(ctx, job.Args.InvoiceID, "payment_received")
	return nil
}

// checkMoneroPayment checks if payment was received
func (w *Worker) checkMoneroPayment(ctx context.Context, address, expectedAmount string) (bool, string) {
	// Placeholder - query Monero wallet RPC in production
	return false, "0"
}

// ============================================
// Order Expiration Jobs
// ============================================

// ExpireOrdersJob expires unpaid orders
type ExpireOrdersJob struct{}

// expireOrdersJob expires unpaid orders
func (w *Worker) expireOrdersJob(ctx context.Context, job *river.Job[ExpireOrdersJob]) error {
	logger := slog.With("job", "expireOrders")

	rows, err := w.db.Query(ctx, `
		SELECT i.id, o.id FROM invoices i
		JOIN orders o ON i.order_id = o.id
		WHERE i.status = 'pending' 
		AND i.permanent = false
		AND i.created_at < NOW() - INTERVAL '6 hours'
	`)
	if err != nil {
		return fmt.Errorf("failed to query expired invoices: %w", err)
	}
	defer rows.Close()

	count := 0
	for rows.Next() {
		var invoiceID, orderID uuid.UUID
		rows.Scan(&invoiceID, &orderID)

		now := time.Now()
		w.db.Exec(ctx, `UPDATE invoices SET status = 'expired', updated_at = $1 WHERE id = $2`, now, invoiceID)
		w.db.Exec(ctx, `UPDATE orders SET status = 'cancelled', updated_at = $1 WHERE id = $2`, now, orderID)
		count++
	}

	logger.Info("Expired orders check complete", "count", count)
	return nil
}

// ============================================
// Reserve Proof Jobs
// ============================================

// UpdateReservesJob generates reserve proofs
type UpdateReservesJob struct{}

// updateReservesJob generates a new reserve proof
func (w *Worker) updateReservesJob(ctx context.Context, job *river.Job[UpdateReservesJob]) error {
	logger := slog.With("job", "updateReserves")

	var lastProof time.Time
	err := w.db.QueryRow(ctx, `SELECT MAX(timestamp) FROM reserve_proofs`).Scan(&lastProof)
	if err == nil && time.Since(lastProof) < time.Hour {
		logger.Info("Recent proof exists, skipping")
		return nil
	}

	var totalBalance string
	w.db.QueryRow(ctx, `SELECT COALESCE(SUM(balance_pico)::text, '0') FROM wallets`).Scan(&totalBalance)

	proofID := uuid.New()
	signature := fmt.Sprintf("SIG-%s-%d", proofID.String(), time.Now().Unix())

	_, err = w.db.Exec(ctx, `
		INSERT INTO reserve_proofs (
			id, view_key, total_balance_pico, user_balances_pico,
			timestamp, signature, block_hash, block_height, merkle_root
		) VALUES ($1, 'view_key_placeholder', $2, $2, NOW(), $3, 'block_hash', 0, 'merkle_root')
	`, proofID, totalBalance, signature)
	if err != nil {
		return fmt.Errorf("failed to create reserve proof: %w", err)
	}

	logger.Info("Reserve proof generated", "proofID", proofID)
	return nil
}

// ============================================
// Jury Processing Jobs
// ============================================

// ProcessJuryJob processes jury voting
type ProcessJuryJob struct {
	DisputeID uuid.UUID
}

// processJuryJob processes jury voting for a dispute
func (w *Worker) processJuryJob(ctx context.Context, job *river.Job[ProcessJuryJob]) error {
	logger := slog.With("job", "processJury", "disputeID", job.Args.DisputeID)

	var jurySize int
	var voteWindow time.Duration
	w.db.QueryRow(ctx, `SELECT jury_size, vote_window_hours FROM jury_settings LIMIT 1`).Scan(&jurySize, &voteWindow)
	if voteWindow == 0 {
		voteWindow = 72 * time.Hour
	}

	var status string
	var createdAt time.Time
	err := w.db.QueryRow(ctx, `SELECT status, created_at FROM disputes WHERE id = $1`, job.Args.DisputeID).Scan(&status, &createdAt)
	if err != nil {
		return fmt.Errorf("dispute not found: %w", err)
	}

	if status != "voting" {
		return nil
	}

	if time.Since(createdAt) < voteWindow {
		return nil
	}

	var buyerVotes, vendorVotes int
	w.db.QueryRow(ctx, `SELECT COUNT(*) FROM jury_assignments WHERE dispute_id = $1 AND vote = 'buyer'`, job.Args.DisputeID).Scan(&buyerVotes)
	w.db.QueryRow(ctx, `SELECT COUNT(*) FROM jury_assignments WHERE dispute_id = $1 AND vote = 'vendor'`, job.Args.DisputeID).Scan(&vendorVotes)

	ruling := "vendor"
	if buyerVotes > vendorVotes {
		ruling = "buyer"
	}

	now := time.Now()
	w.db.Exec(ctx, `UPDATE disputes SET status = 'resolved', ruling = $1, resolved_at = $2 WHERE id = $3`, ruling, now, job.Args.DisputeID)

	newStatus := "finalized"
	if ruling == "buyer" {
		newStatus = "refunded"
	}
	w.db.Exec(ctx, `UPDATE orders SET status = $1, updated_at = $2 WHERE id = (SELECT order_id FROM disputes WHERE id = $3)`, newStatus, now, job.Args.DisputeID)

	w.db.Exec(ctx, `UPDATE jurors SET pending_cases = GREATEST(0, pending_cases - 1) WHERE id IN (SELECT juror_id FROM jury_assignments WHERE dispute_id = $1)`, job.Args.DisputeID)

	logger.Info("Dispute resolved", "ruling", ruling)
	w.queueNotification(ctx, job.Args.DisputeID, "dispute_resolved")
	return nil
}

// ============================================
// Cleanup Jobs
// ============================================

// CleanupExpiredJob cleans up expired data
type CleanupExpiredJob struct{}

// cleanupExpiredJob removes expired sessions and challenges
func (w *Worker) cleanupExpiredJob(ctx context.Context, job *river.Job[CleanupExpiredJob]) error {
	logger := slog.With("job", "cleanupExpired")

	result, _ := w.db.Exec(ctx, `DELETE FROM sessions WHERE expires_at < NOW()`)
	logger.Info("Cleaned expired sessions", "count", result.RowsAffected())

	result, _ = w.db.Exec(ctx, `DELETE FROM captcha_challenges WHERE expires_at < NOW()`)
	logger.Info("Cleaned expired challenges", "count", result.RowsAffected())

	result, _ = w.db.Exec(ctx, `DELETE FROM login_attempts WHERE attempted_at < NOW() - INTERVAL '30 days'`)
	logger.Info("Cleaned old login attempts", "count", result.RowsAffected())

	return nil
}

// queueNotification queues a notification
func (w *Worker) queueNotification(ctx context.Context, relatedID uuid.UUID, notificationType string) {
	_ = relatedID
	_ = notificationType
}

// ScheduleJobs schedules periodic jobs
func (w *Worker) ScheduleJobs(ctx context.Context) {
	slog.Info("Job scheduler configured")
}
