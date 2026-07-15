package order

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/shopspring/decimal"
)

// Status represents the order status
type Status string

const (
	StatusPending   Status = "pending"
	StatusPaid      Status = "paid"
	StatusAccepted  Status = "accepted"
	StatusDeclined  Status = "declined"
	StatusDispatched Status = "dispatched"
	StatusDelivered Status = "delivered"
	StatusDisputed  Status = "disputed"
	StatusFinalized Status = "finalized"
	StatusRefunded  Status = "refunded"
	StatusCancelled Status = "cancelled"
)

// ValidTransitions defines valid status transitions
var ValidTransitions = map[Status][]Status{
	StatusPending:    {StatusPaid, StatusCancelled},
	StatusPaid:       {StatusAccepted, StatusDeclined},
	StatusAccepted:   {StatusDispatched},
	StatusDispatched: {StatusDelivered, StatusDisputed},
	StatusDisputed:   {StatusFinalized, StatusRefunded},
}

// Order represents an order
type Order struct {
	ID              uuid.UUID
	CustomerID      uuid.UUID
	VendorID        uuid.UUID
	Status          Status
	TotalPricePico  decimal.Decimal
	ShippingPricePico decimal.Decimal
	FeePico         decimal.Decimal
	Details         string
	Version         int
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// OrderItem represents an item in an order
type OrderItem struct {
	ID         uuid.UUID
	OrderID    uuid.UUID
	ProductID  uuid.UUID
	PriceTierID uuid.UUID
	Quantity   int
	PricePico  decimal.Decimal
	Title      string
}

// Service provides order operations
type Service struct {
	db *pgxpool.Pool
}

// NewService creates a new order service
func NewService(db *pgxpool.Pool) *Service {
	return &Service{db: db}
}

// Create creates a new order
func (s *Service) Create(ctx context.Context, params CreateParams) (*Order, error) {
	tx, err := s.db.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	// Calculate totals
	totalPrice := decimal.Zero
	for _, item := range params.Items {
		totalPrice = totalPrice.Add(item.PricePico)
	}
	totalPrice = totalPrice.Add(params.ShippingPricePico)

	// Calculate fee (5%)
	fee := totalPrice.Mul(decimal.NewFromFloat(0.05))

	orderID := uuid.New()
	now := time.Now()

	// Insert order
	_, err = tx.Exec(ctx, `
		INSERT INTO orders (id, customer_id, vendor_id, status, total_price_pico, 
			shipping_price_pico, fee_pico, details, version, created_at, updated_at)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, 1, $9, $9)
	`, orderID, params.CustomerID, params.VendorID, StatusPending,
		totalPrice.BigInt(), params.ShippingPricePico.BigInt(), fee.BigInt(),
		params.Details, now)
	if err != nil {
		return nil, fmt.Errorf("failed to insert order: %w", err)
	}

	// Insert order items
	for _, item := range params.Items {
		_, err = tx.Exec(ctx, `
			INSERT INTO order_items (id, order_id, product_id, price_tier_id, quantity, price_pico, title)
			VALUES ($1, $2, $3, $4, $5, $6, $7)
		`, uuid.New(), orderID, item.ProductID, item.PriceTierID, item.Quantity, item.PricePico.BigInt(), item.Title)
		if err != nil {
			return nil, fmt.Errorf("failed to insert order item: %w", err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	return s.GetByID(ctx, orderID)
}

// CreateParams contains parameters for order creation
type CreateParams struct {
	CustomerID       uuid.UUID
	VendorID         uuid.UUID
	ShippingPricePico decimal.Decimal
	Details          string
	Items            []ItemParams
}

// ItemParams contains parameters for an order item
type ItemParams struct {
	ProductID   uuid.UUID
	PriceTierID uuid.UUID
	Quantity    int
	PricePico   decimal.Decimal
	Title       string
}

// GetByID retrieves an order by ID
func (s *Service) GetByID(ctx context.Context, id uuid.UUID) (*Order, error) {
	var order Order
	err := s.db.QueryRow(ctx, `
		SELECT id, customer_id, vendor_id, status, total_price_pico,
			shipping_price_pico, fee_pico, details, version, created_at, updated_at
		FROM orders WHERE id = $1
	`, id).Scan(
		&order.ID, &order.CustomerID, &order.VendorID, &order.Status,
		&order.TotalPricePico, &order.ShippingPricePico, &order.FeePico,
		&order.Details, &order.Version, &order.CreatedAt, &order.UpdatedAt,
	)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, fmt.Errorf("order not found")
		}
		return nil, fmt.Errorf("failed to get order: %w", err)
	}
	return &order, nil
}

// UpdateStatus updates the order status with validation
func (s *Service) UpdateStatus(ctx context.Context, id uuid.UUID, newStatus Status) error {
	order, err := s.GetByID(ctx, id)
	if err != nil {
		return err
	}

	// Validate transition
	if !isValidTransition(order.Status, newStatus) {
		return fmt.Errorf("invalid status transition from %s to %s", order.Status, newStatus)
	}

	// Use optimistic locking
	result, err := s.db.Exec(ctx, `
		UPDATE orders SET status = $1, version = version + 1, updated_at = $2
		WHERE id = $3 AND version = $4
	`, newStatus, time.Now(), id, order.Version)
	if err != nil {
		return fmt.Errorf("failed to update order: %w", err)
	}

	if result.RowsAffected() == 0 {
		return fmt.Errorf("order was modified by another transaction")
	}

	return nil
}

// isValidTransition checks if a status transition is valid
func isValidTransition(from, to Status) bool {
	validTargets, ok := ValidTransitions[from]
	if !ok {
		return false
	}
	for _, target := range validTargets {
		if target == to {
			return true
		}
	}
	return false
}

// GetByCustomer retrieves orders for a customer
func (s *Service) GetByCustomer(ctx context.Context, customerID uuid.UUID) ([]*Order, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, customer_id, vendor_id, status, total_price_pico,
			shipping_price_pico, fee_pico, details, version, created_at, updated_at
		FROM orders WHERE customer_id = $1
		ORDER BY created_at DESC
	`, customerID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID, &order.CustomerID, &order.VendorID, &order.Status,
			&order.TotalPricePico, &order.ShippingPricePico, &order.FeePico,
			&order.Details, &order.Version, &order.CreatedAt, &order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}
	return orders, nil
}

// GetByVendor retrieves orders for a vendor
func (s *Service) GetByVendor(ctx context.Context, vendorID uuid.UUID) ([]*Order, error) {
	rows, err := s.db.Query(ctx, `
		SELECT id, customer_id, vendor_id, status, total_price_pico,
			shipping_price_pico, fee_pico, details, version, created_at, updated_at
		FROM orders WHERE vendor_id = $1
		ORDER BY created_at DESC
	`, vendorID)
	if err != nil {
		return nil, fmt.Errorf("failed to query orders: %w", err)
	}
	defer rows.Close()

	var orders []*Order
	for rows.Next() {
		var order Order
		err := rows.Scan(
			&order.ID, &order.CustomerID, &order.VendorID, &order.Status,
			&order.TotalPricePico, &order.ShippingPricePico, &order.FeePico,
			&order.Details, &order.Version, &order.CreatedAt, &order.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan order: %w", err)
		}
		orders = append(orders, &order)
	}
	return orders, nil
}

// Accept marks an order as accepted by vendor
func (s *Service) Accept(ctx context.Context, id uuid.UUID) error {
	return s.UpdateStatus(ctx, id, StatusAccepted)
}

// Decline marks an order as declined by vendor
func (s *Service) Decline(ctx context.Context, id uuid.UUID) error {
	return s.UpdateStatus(ctx, id, StatusDeclined)
}

// Dispatch marks an order as dispatched by vendor
func (s *Service) Dispatch(ctx context.Context, id uuid.UUID) error {
	return s.UpdateStatus(ctx, id, StatusDispatched)
}

// Finalize marks an order as finalized by buyer
func (s *Service) Finalize(ctx context.Context, id uuid.UUID) error {
	return s.UpdateStatus(ctx, id, StatusFinalized)
}

// Cancel cancels an order
func (s *Service) Cancel(ctx context.Context, id uuid.UUID) error {
	return s.UpdateStatus(ctx, id, StatusCancelled)
}
