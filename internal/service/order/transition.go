package order

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/gobugger/gomarket/internal/repo"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"slices"
)

// SECURE: Idempotency key for order operations to prevent double-processing
type IdempotencyKey struct {
	Operation string
	OrderID   uuid.UUID
	Timestamp time.Time
}

func (k *IdempotencyKey) IsExpired() bool {
	return time.Since(k.Timestamp) > 24*time.Hour
}

// SECURE: Validate order state transition is allowed
func validTransition(current repo.OrderStatus, next repo.OrderStatus) bool {
	return slices.Contains(getValidStatuses(next), current)
}

// Returns all valid statuses where transition to next is valid
func getValidStatuses(next repo.OrderStatus) []repo.OrderStatus {
	switch next {
	case repo.OrderStatusPending:
		return []repo.OrderStatus{}
	case repo.OrderStatusPaid, repo.OrderStatusCancelled:
		return []repo.OrderStatus{repo.OrderStatusPending}
	case repo.OrderStatusAccepted:
		return []repo.OrderStatus{repo.OrderStatusPaid}
	case repo.OrderStatusDeclined:
		return []repo.OrderStatus{repo.OrderStatusPaid, repo.OrderStatusAccepted} // vendor declined or forgot to deliver on time
	case repo.OrderStatusDispatched:
		return []repo.OrderStatus{repo.OrderStatusAccepted}
	case repo.OrderStatusFinalized, repo.OrderStatusDisputed:
		return []repo.OrderStatus{repo.OrderStatusDispatched}
	case repo.OrderStatusSettled:
		return []repo.OrderStatus{repo.OrderStatusDisputed} // Settled by admin or vendor didn't counter
	default:
		return []repo.OrderStatus{}
	}
}

// SECURE: Update order status with validation - returns error if transition is invalid
func UpdateStatus(ctx context.Context, q *repo.Queries, orderID uuid.UUID, status repo.OrderStatus) (repo.Order, error) {
	// First, get the current order to validate the transition
	order, err := q.GetOrder(ctx, orderID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return repo.Order{}, fmt.Errorf("order not found: %w", ErrInvalidStatus)
		}
		return repo.Order{}, err
	}

	// SECURE: Validate the state transition is allowed
	if !validTransition(order.Status, status) {
		return repo.Order{}, fmt.Errorf("invalid status transition from %s to %s: %w", order.Status, status, ErrInvalidStatus)
	}

	// Update the status
	updatedOrder, err := q.UpdateOrderStatus(
		ctx,
		repo.UpdateOrderStatusParams{
			ID:            orderID,
			Status:        status,
			ValidStatuses: getValidStatuses(status),
		})

	if errors.Is(err, pgx.ErrNoRows) {
		return updatedOrder, fmt.Errorf("can not update order status to %s: %w", status, ErrInvalidStatus)
	}

	return updatedOrder, err
}

// SECURE: Atomic status update with balance operations to prevent race conditions
func UpdateStatusWithBalance(ctx context.Context, q *repo.Queries, orderID uuid.UUID, status repo.OrderStatus, balanceChange func(ctx context.Context, q *repo.Queries, order repo.Order) error) (repo.Order, error) {
	// Get current order state
	order, err := q.GetOrder(ctx, orderID)
	if err != nil {
		return repo.Order{}, err
	}

	// SECURE: Validate transition is allowed
	if !validTransition(order.Status, status) {
		return repo.Order{}, fmt.Errorf("invalid status transition from %s to %s: %w", order.Status, status, ErrInvalidStatus)
	}

	// SECURE: Perform balance operation if provided
	if balanceChange != nil {
		if err := balanceChange(ctx, q, order); err != nil {
			return repo.Order{}, err
		}
	}

	// Update status - will fail if status already changed (optimistic locking)
	updatedOrder, err := q.UpdateOrderStatus(
		ctx,
		repo.UpdateOrderStatusParams{
			ID:            orderID,
			Status:        status,
			ValidStatuses: getValidStatuses(status),
		})

	if errors.Is(err, pgx.ErrNoRows) {
		return updatedOrder, fmt.Errorf("order status was modified by another process: %w", ErrInvalidStatus)
	}

	return updatedOrder, err
}

// SECURE: Check if order should auto-dispute due to timeout
func ShouldAutoDispute(ctx context.Context, q *repo.Queries, orderID uuid.UUID) (bool, error) {
	order, err := q.GetOrder(ctx, orderID)
	if err != nil {
		return false, err
	}

	// Only check orders that are in dispatched state
	if order.Status != repo.OrderStatusDispatched {
		return false, nil
	}

	// Check if AF timer has expired (7 days + any extensions)
	dispatchTime := order.DispatchedAt
	timeout := 7 * 24 * time.Hour // Base 7 days
	if order.NumExtends > 0 {
		// Extended orders get more time based on config
		timeout += time.Duration(order.NumExtends) * 7 * 24 * time.Hour
	}

	return time.Since(dispatchTime) > timeout, nil
}
