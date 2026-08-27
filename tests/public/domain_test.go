package public_test

import (
	"testing"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/validation"
)

func TestOrderStateMachine_ValidTransitions(t *testing.T) {
	cases := []struct {
		from domain.OrderStatus
		to   domain.OrderStatus
		want bool
	}{
		{domain.OrderPending, domain.OrderApproved, true},
		{domain.OrderPending, domain.OrderFulfilled, false},
		{domain.OrderApproved, domain.OrderReserved, true},
		{domain.OrderReserved, domain.OrderFulfilling, true},
		{domain.OrderFulfilling, domain.OrderFulfilled, true},
		{domain.OrderFulfilled, domain.OrderCancelled, false},
		{domain.OrderFailed, domain.OrderPending, true},
	}
	for _, c := range cases {
		got := domain.IsValidTransition(c.from, c.to)
		if got != c.want {
			t.Errorf("IsValidTransition(%s, %s) = %v, want %v", c.from, c.to, got, c.want)
		}
	}
}

func TestValidation_BoundedRequestSize(t *testing.T) {
	if err := validation.RequireBounded("items", validation.MaxItemsPerOrder+1, validation.MaxItemsPerOrder); err == nil {
		t.Fatal("expected an over-limit item count to be rejected")
	}
	if err := validation.RequireBounded("items", 5, validation.MaxItemsPerOrder); err != nil {
		t.Fatalf("expected a normal item count to pass, got %v", err)
	}
}
