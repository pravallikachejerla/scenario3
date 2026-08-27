// Package fulfilment coordinates dispatch to the shipping dependency
// once an order has entered the fulfilling state.
package fulfilment

import (
	"context"

	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
)

// Service is the application-layer entry point for dispatching a
// fulfilment request to the shipping dependency.
//
// SOURCE CONDITION (STAB-06, layer 2 of 3): Dispatch retries a
// retryable failure itself, in addition to the retry already
// performed inside dependencies.Client (layer 1) and the retry
// performed by the worker that invokes Dispatch (layer 3, see
// internal/workers callers in cmd/worker). None of the three layers
// knows about the others' retry budget.
type Service struct {
	Client       *dependencies.Client
	ServiceRetryMax int
	ScopeFn      func(tenantID string) string
}

func New(client *dependencies.Client, serviceRetryMax int, scopeFn func(tenantID string) string) *Service {
	return &Service{Client: client, ServiceRetryMax: serviceRetryMax, ScopeFn: scopeFn}
}

func (s *Service) Dispatch(ctx context.Context, tenantID, orderID string) error {
	var lastErr error
	for attempt := 1; attempt <= s.ServiceRetryMax; attempt++ {
		err := s.Client.Call(ctx, tenantID, orderID, s.ScopeFn(tenantID))
		if err == nil {
			return nil
		}
		lastErr = err
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return lastErr
}
