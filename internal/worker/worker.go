package worker

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/gobugger/gomarket/internal/repo"
	"github.com/gobugger/gomarket/internal/service/payment"
	"github.com/gobugger/gomarket/pkg/payment/provider"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/riverqueue/river"
)

// SECURE: Job retry configuration
const (
	MaxRetries    = 3
	RetryBackoff  = 5 * time.Second
)

type PrepareInvoiceArgs struct {
	ID            uuid.UUID
	RetryCount    int `json:"retry_count"`
}

func (PrepareInvoiceArgs) Kind() string { return "prepare_invoice" }

// SECURE: Configure retry behavior for failed jobs
func (args PrepareInvoiceArgs) UniqueFields() []string {
	return []string{"ID"}
}

type PrepareInvoiceWorker struct {
	Db *pgxpool.Pool
	Pp provider.PaymentProvider
	river.WorkerDefaults[PrepareInvoiceArgs]
}

func (w *PrepareInvoiceWorker) Work(ctx context.Context, job *river.Job[PrepareInvoiceArgs]) error {
	logger := slog.Default()

	// SECURE: Check for duplicate job execution (idempotency)
	tx, err := w.Db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	qtx := repo.New(tx)

	// SECURE: Verify invoice hasn't already been processed
	invoice, err := qtx.GetInvoice(ctx, job.Args.ID)
	if err != nil {
		logger.Error("failed to get invoice", "invoiceID", job.Args.ID, "error", err)
		return fmt.Errorf("failed to get invoice: %w", err)
	}

	// If invoice already has an address, skip (idempotent)
	if invoice.Address != "" {
		logger.Info("invoice already prepared, skipping", "invoiceID", job.Args.ID)
		return nil
	}

	if err = payment.PrepareInvoice(ctx, qtx, w.Pp, job.Args.ID); err != nil {
		logger.Error("failed to prepare invoice", "invoiceID", job.Args.ID, "attempt", job.Attempt, "error", err)

		// SECURE: Return error to trigger retry
		return fmt.Errorf("failed to prepare invoice: %w", err)
	}

	if err = tx.Commit(ctx); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	logger.Info("invoice prepared successfully", "invoiceID", job.Args.ID)
	return nil
}

// SECURE: Error handler for custom retry logic
func (w *PrepareInvoiceWorker) ErrorHandler(ctx context.Context, job *river.Job[PrepareInvoiceArgs], err error) *river.ErrRetryJob {
	// Don't retry if we've exceeded max attempts
	if job.Attempt >= MaxRetries {
		slog.Error("job failed after max retries", "jobID", job.ID, "attempt", job.Attempt, "error", err)
		return nil // Don't retry, let it fail
	}

	// Calculate backoff with exponential increase
	backoff := RetryBackoff * time.Duration(1<<job.Attempt)
	slog.Info("scheduling job retry", "jobID", job.ID, "attempt", job.Attempt+1, "backoff", backoff)

	return river.ErrRetryJobAck().WithRetryAt(job, time.Now().Add(backoff))
}
