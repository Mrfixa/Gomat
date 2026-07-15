package order

import (
	"testing"
)

// TestIsValidTransition tests order status transitions
func TestIsValidTransition(t *testing.T) {
	tests := []struct {
		name     string
		from     Status
		to       Status
		expected bool
	}{
		// Valid transitions from Pending
		{"Pending to Paid", StatusPending, StatusPaid, true},
		{"Pending to Cancelled", StatusPending, StatusCancelled, true},
		{"Pending to Accepted", StatusPending, StatusAccepted, false},
		{"Pending to Dispatched", StatusPending, StatusDispatched, false},

		// Valid transitions from Paid
		{"Paid to Accepted", StatusPaid, StatusAccepted, true},
		{"Paid to Declined", StatusPaid, StatusDeclined, true},
		{"Paid to Pending", StatusPaid, StatusPending, false},
		{"Paid to Dispatched", StatusPaid, StatusDispatched, false},

		// Valid transitions from Accepted
		{"Accepted to Dispatched", StatusAccepted, StatusDispatched, true},
		{"Accepted to Pending", StatusAccepted, StatusPending, false},
		{"Accepted to Declined", StatusAccepted, StatusDeclined, false},

		// Valid transitions from Dispatched
		{"Dispatched to Delivered", StatusDispatched, StatusDelivered, true},
		{"Dispatched to Disputed", StatusDispatched, StatusDisputed, true},
		{"Dispatched to Finalized", StatusDispatched, StatusFinalized, false}, // Must go through Delivered
		{"Dispatched to Pending", StatusDispatched, StatusPending, false},

		// Valid transitions from Disputed
		{"Disputed to Finalized", StatusDisputed, StatusFinalized, true},
		{"Disputed to Refunded", StatusDisputed, StatusRefunded, true},
		{"Disputed to Pending", StatusDisputed, StatusPending, false},

		// Invalid terminal states
		{"Finalized to Pending", StatusFinalized, StatusPending, false},
		{"Refunded to Pending", StatusRefunded, StatusPending, false},
		{"Cancelled to Pending", StatusCancelled, StatusPending, false},
		{"Declined to Pending", StatusDeclined, StatusPending, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := isValidTransition(tt.from, tt.to)
			if result != tt.expected {
				t.Errorf("isValidTransition(%s, %s) = %v, want %v",
					tt.from, tt.to, result, tt.expected)
			}
		})
	}
}

// TestStatusConstants verifies status values
func TestStatusConstants(t *testing.T) {
	// Ensure all statuses are unique
	statuses := []Status{
		StatusPending,
		StatusPaid,
		StatusAccepted,
		StatusDeclined,
		StatusDispatched,
		StatusDelivered,
		StatusDisputed,
		StatusFinalized,
		StatusRefunded,
		StatusCancelled,
	}

	seen := make(map[Status]bool)
	for _, s := range statuses {
		if seen[s] {
			t.Errorf("Duplicate status: %s", s)
		}
		seen[s] = true
	}
}

// TestValidTransitionsMap verifies the transition map is complete
func TestValidTransitionsMap(t *testing.T) {
	// Every status in the map should be a valid starting status
	for from := range ValidTransitions {
		if _, ok := statusIndex(from); !ok {
			t.Errorf("Invalid status in ValidTransitions map: %s", from)
		}
	}

	// Pending should always have transitions
	if len(ValidTransitions[StatusPending]) == 0 {
		t.Error("Pending should have at least one transition")
	}
}

// statusIndex converts status to array index (for validation)
func statusIndex(s Status) (int, bool) {
	switch s {
	case StatusPending:
		return 0, true
	case StatusPaid:
		return 1, true
	case StatusAccepted:
		return 2, true
	case StatusDeclined:
		return 3, true
	case StatusDispatched:
		return 4, true
	case StatusDelivered:
		return 5, true
	case StatusDisputed:
		return 6, true
	case StatusFinalized:
		return 7, true
	case StatusRefunded:
		return 8, true
	case StatusCancelled:
		return 9, true
	default:
		return -1, false
	}
}
