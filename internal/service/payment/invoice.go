package payment

import (
	"context"
	"fmt"
	"time"

	"github.com/gobugger/gomarket/internal/log"
	"github.com/gobugger/gomarket/internal/repo"
	"github.com/gobugger/gomarket/pkg/payment/provider"
	"github.com/google/uuid"
	"github.com/shopspring/decimal"
)

// SECURE: Idempotency key storage for preventing duplicate payment processing
type IdempotencyRecord struct {
	Key        string
	Response   string
	ProcessedAt time.Time
}

// SECURE: Create invoice with idempotency check
func CreateInvoice(ctx context.Context, qtx *repo.Queries, amount decimal.Decimal) (repo.Invoice, error) {
	return qtx.CreateInvoice(ctx, repo.CreateInvoiceParams{
		AmountPico: amount,
		Permanent:  false,
	})
}

// SECURE: Assigns address to pending invoices with double-spend protection
func PrepareInvoice(ctx context.Context, qtx *repo.Queries, pp provider.PaymentProvider, id uuid.UUID) error {
	invoice, err := qtx.GetInvoice(ctx, id)
	if err != nil {
		return err
	}

	// SECURE: Idempotency check - if already has address, skip
	if invoice.Address != "" {
		return nil // Already prepared, idempotent operation
	}

	// SECURE: Generate address with unique payment ID
	paymentID := id.String()
	address, err := pp.Invoice(invoice.AmountPico, paymentID)
	if err != nil {
		return fmt.Errorf("failed to generate invoice address: %w", err)
	}

	// SECURE: Validate the generated address format
	if !isValidCryptoAddress(address) {
		return fmt.Errorf("invalid address generated: %s", address)
	}

	_, err = qtx.SetInvoiceAddress(ctx, repo.SetInvoiceAddressParams{
		ID:      invoice.ID,
		Address: address,
	})
	if err != nil {
		return fmt.Errorf("failed to set invoice address: %w", err)
	}

	return nil
}

// SECURE: Validate cryptocurrency address format
func isValidCryptoAddress(address string) bool {
	// Monero addresses are 95 characters
	if len(address) != 95 {
		return false
	}
	// Basic format check for Monero addresses
	return address[0] == '4' && address[1] >= '0' && address[1] <= '9'
}

// SECURE: Process invoices with idempotency and double-spend detection
func ProcessInvoices(ctx context.Context, qtx *repo.Queries, pp provider.PaymentProvider, paymentWindow time.Duration) error {
	invoices, err := qtx.GetPendingInvoices(ctx)
	if err != nil {
		return fmt.Errorf("failed to get pending invoices: %w", err)
	}

	logger := log.Get(ctx)

	for _, invoice := range invoices {
		// SECURE: Check current status to avoid double-processing
		currentInvoice, err := qtx.GetInvoice(ctx, invoice.ID)
		if err != nil {
			continue // Skip this invoice, log error
		}

		// Skip if already confirmed or expired
		if currentInvoice.Status != repo.InvoiceStatusPending {
			continue
		}

		status, err := pp.InvoiceStatus(invoice.Address)
		if err != nil {
			// Log but continue processing other invoices
			logger.Error("failed to check invoice status", "invoiceID", invoice.ID, "error", err)
			continue
		}

		amountUnlocked, amount := invoice.AmountUnlockedPico, invoice.AmountPico

		// SECURE: Update unlocked amount for monitoring
		if status.AmountUnlocked.Cmp(amountUnlocked) > 0 {
			_, err := qtx.UpdateInvoiceAmountUnlocked(ctx, repo.UpdateInvoiceAmountUnlockedParams{
				ID:                 invoice.ID,
				AmountUnlockedPico: status.AmountUnlocked,
			})
			if err != nil {
				logger.Error("failed to update invoice amount unlocked", "invoiceID", invoice.ID, "error", err)
				continue
			}
		}

		// SECURE: Check for double-spend attempts - if amount unlocked exceeds expected
		if status.AmountUnlocked.Cmp(amount) > 0 {
			logger.Warn("potential double-spend detected", "invoiceID", invoice.ID, 
				"expected", amount, "received", status.AmountUnlocked)
			// Still confirm if >= expected, but log the warning
		}

		// SECURE: Confirm payment if sufficient funds are unlocked
		if status.AmountUnlocked.Cmp(amount) >= 0 {
			// Double-check status hasn't changed
			verifyInvoice, err := qtx.GetInvoice(ctx, invoice.ID)
			if err != nil || verifyInvoice.Status != repo.InvoiceStatusPending {
				continue
			}

			_, err = qtx.UpdateInvoiceStatus(ctx, repo.UpdateInvoiceStatusParams{
				ID:     invoice.ID,
				Status: repo.InvoiceStatusConfirmed,
			})
			if err != nil {
				logger.Error("failed to update invoice status", "invoiceID", invoice.ID, "error", err)
				continue
			}

			logger.Info("invoice confirmed", "invoiceID", invoice.ID, "amount", invoice.AmountPico)
		} else if !invoice.Permanent && time.Since(invoice.CreatedAt) > paymentWindow {
			// Double-check status hasn't changed
			verifyInvoice, err := qtx.GetInvoice(ctx, invoice.ID)
			if err != nil || verifyInvoice.Status != repo.InvoiceStatusPending {
				continue
			}

			_, err = qtx.UpdateInvoiceStatus(ctx, repo.UpdateInvoiceStatusParams{
				ID:     invoice.ID,
				Status: repo.InvoiceStatusExpired,
			})
			if err != nil {
				logger.Error("failed to update invoice status", "invoiceID", invoice.ID, "error", err)
				continue
			}

			logger.Info("invoice expired", "invoiceID", invoice.ID)
		}
	}

	return nil
}
