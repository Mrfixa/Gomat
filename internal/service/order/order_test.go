package order

import (
	"context"
	"testing"
	"time"

	"github.com/gobugger/gomarket/internal/repo"
	"github.com/gobugger/gomarket/internal/service/currency"
	"github.com/gobugger/gomarket/internal/service/servicetest"
	"github.com/gobugger/gomarket/internal/testutil"
	"github.com/shopspring/decimal"

	"github.com/stretchr/testify/require"
)

var infra *testutil.Infra

func TestMain(m *testing.M) {
	infra = testutil.NewInfra(context.Background())

	m.Run()

	infra.Close()
}

func TestFinalize(t *testing.T) {
	ctx := t.Context()
	qtx := repo.New(infra.Db)
	currency.DebugStart(ctx, qtx)

	vendor := servicetest.SetupVendor(t, infra, decimal.NewFromInt(0))
	customer := servicetest.SetupCustomer(t, infra, decimal.NewFromInt(0))
	product := servicetest.SetupProduct(t, infra, vendor.ID)

	dms, err := qtx.GetDeliveryMethodsForVendor(ctx, vendor.ID)
	require.NoError(t, err)
	dm := dms[0]
	price := product.Pricing[0]

	_, err = qtx.CreateCartItem(ctx, repo.CreateCartItemParams{
		CustomerID: customer.ID,
		PriceID:    price.ID,
	})
	require.NoError(t, err)

	order, err := Create(ctx, qtx, CreateParams{
		DeliveryMethodID: dm.ID,
		CustomerID:       customer.ID,
		Details:          "Order details here",
	})
	require.NoError(t, err)

	err = Accept(ctx, qtx, order.ID)
	require.Error(t, err)

	_, err = UpdateStatus(ctx, qtx, order.ID, repo.OrderStatusPaid)
	require.NoError(t, err)

	err = Accept(ctx, qtx, order.ID)
	require.NoError(t, err)
	servicetest.RequireOrderStatus(t, qtx, order.ID, repo.OrderStatusAccepted)

	p, err := qtx.GetProduct(ctx, product.Product.ID)
	require.NoError(t, err)
	require.Equal(t, product.Product.Inventory-price.Quantity, p.Inventory)

	err = Dispatch(ctx, qtx, order.ID)
	require.NoError(t, err)
	servicetest.RequireOrderStatus(t, qtx, order.ID, repo.OrderStatusDispatched)

	err = Finalize(ctx, qtx, order.ID)
	require.NoError(t, err)
	servicetest.RequireBalanceForUser(t, qtx, vendor.ID, order.TotalPricePico)
}

func TestCancel(t *testing.T) {
	ctx := t.Context()
	qtx := repo.New(infra.Db)
	currency.DebugStart(ctx, qtx)

	vendor := servicetest.SetupVendor(t, infra, decimal.NewFromInt(0))
	customer := servicetest.SetupCustomer(t, infra, decimal.NewFromInt(0))
	product := servicetest.SetupProduct(t, infra, vendor.ID)

	dms, err := qtx.GetDeliveryMethodsForVendor(ctx, vendor.ID)
	require.NoError(t, err)
	dm := dms[0]
	price := product.Pricing[0]

	_, err = qtx.CreateCartItem(ctx, repo.CreateCartItemParams{
		CustomerID: customer.ID,
		PriceID:    price.ID,
	})
	require.NoError(t, err)

	order, err := Create(ctx, qtx, CreateParams{
		DeliveryMethodID: dm.ID,
		CustomerID:       customer.ID,
		Details:          "Order details here",
	})
	require.NoError(t, err)
	require.Equal(t, repo.OrderStatusPending, order.Status)

	err = Cancel(ctx, qtx, order.ID)
	require.NoError(t, err)

	servicetest.RequireOrderStatus(t, qtx, order.ID, repo.OrderStatusCancelled)
	servicetest.RequireBalanceForUser(t, qtx, vendor.ID, decimal.NewFromInt(0))
}

func TestDecline(t *testing.T) {
	ctx := t.Context()
	qtx := repo.New(infra.Db)
	currency.DebugStart(ctx, qtx)

	vendor := servicetest.SetupVendor(t, infra, decimal.NewFromInt(0))
	customer := servicetest.SetupCustomer(t, infra, decimal.NewFromInt(0))
	product := servicetest.SetupProduct(t, infra, vendor.ID)

	dms, err := qtx.GetDeliveryMethodsForVendor(ctx, vendor.ID)
	require.NoError(t, err)
	dm := dms[0]
	price := product.Pricing[0]

	_, err = qtx.CreateCartItem(ctx, repo.CreateCartItemParams{
		CustomerID: customer.ID,
		PriceID:    price.ID,
	})
	require.NoError(t, err)

	order, err := Create(ctx, qtx, CreateParams{
		DeliveryMethodID: dm.ID,
		CustomerID:       customer.ID,
		Details:          "Order details here",
	})
	require.NoError(t, err)

	err = Decline(ctx, qtx, order.ID)
	require.Error(t, err)

	_, err = UpdateStatus(ctx, qtx, order.ID, repo.OrderStatusPaid)
	require.NoError(t, err)

	err = Decline(ctx, qtx, order.ID)
	require.NoError(t, err)
	servicetest.RequireOrderStatus(t, qtx, order.ID, repo.OrderStatusDeclined)

	servicetest.RequireBalanceForUser(t, qtx, customer.ID, currency.AddFee(order.TotalPricePico))
	servicetest.RequireBalanceForUser(t, qtx, vendor.ID, decimal.NewFromInt(0))
}

// SECURE: Test order status transitions
func TestValidOrderTransitions(t *testing.T) {
	tests := []struct {
		name     string
		current  repo.OrderStatus
		next     repo.OrderStatus
		expected bool
	}{
		{"Pending to Paid", repo.OrderStatusPending, repo.OrderStatusPaid, true},
		{"Pending to Cancelled", repo.OrderStatusPending, repo.OrderStatusCancelled, true},
		{"Pending to Accepted", repo.OrderStatusPending, repo.OrderStatusAccepted, false},
		{"Paid to Accepted", repo.OrderStatusPaid, repo.OrderStatusAccepted, true},
		{"Paid to Declined", repo.OrderStatusPaid, repo.OrderStatusDeclined, true},
		{"Accepted to Dispatched", repo.OrderStatusAccepted, repo.OrderStatusDispatched, true},
		{"Dispatched to Finalized", repo.OrderStatusDispatched, repo.OrderStatusFinalized, true},
		{"Dispatched to Disputed", repo.OrderStatusDispatched, repo.OrderStatusDisputed, true},
		{"Disputed to Settled", repo.OrderStatusDisputed, repo.OrderStatusSettled, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := validTransition(tt.current, tt.next)
			if result != tt.expected {
				t.Errorf("validTransition(%s, %s) = %v, want %v", tt.current, tt.next, result, tt.expected)
			}
		})
	}
}

// SECURE: Test idempotency key expiration
func TestIdempotencyKeyExpiration(t *testing.T) {
	key := &IdempotencyKey{
		Operation: "test",
		Timestamp: time.Now().Add(-24 * time.Hour),
	}

	if !key.IsExpired() {
		t.Error("IdempotencyKey should be expired after 24 hours")
	}

	key.Timestamp = time.Now()
	if key.IsExpired() {
		t.Error("IdempotencyKey should not be expired immediately")
	}
}

// SECURE: Test refund factor validation
func TestValidateRefundFactor(t *testing.T) {
	tests := []struct {
		name    string
		factor  float64
		wantErr bool
	}{
		{"Valid 0", 0, false},
		{"Valid 1", 1, false},
		{"Valid 0.5", 0.5, false},
		{"Invalid negative", -0.1, true},
		{"Invalid over 1", 1.1, true},
		{"Invalid precision", 0.00001, true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateRefundFactor(tt.factor)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateRefundFactor(%f) error = %v, wantErr %v", tt.factor, err, tt.wantErr)
			}
		})
	}
}
