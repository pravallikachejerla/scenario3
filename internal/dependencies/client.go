package dependencies

import (
	"context"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// Recorder captures one attempted dependency call for evidence and
// for the retry-amplification private test.
type Recorder interface {
	RecordDependencyCall(r domain.DependencyCallRecord)
}

// Client is the HTTP-client-layer wrapper around a Simulator.
//
// SOURCE CONDITION (STAB-06, layer 1 of 3): Client retries a
// retryable failure internally up to RetryMax times. The
// application/service layer (internal/fulfilment) and the worker
// layer (internal/workers) each also retry independently on the same
// kind of failure, with no shared retry budget or coordination
// between the three layers. One dependency failure for one business
// request can therefore fan out to RetryMax(client) x
// RetryMax(service) x RetryMax(worker) simulator calls.
//
// SOURCE CONDITION (STAB-04): Call never derives its own bounded
// per-attempt deadline; it uses whatever ctx it is handed as-is. If
// the caller (see internal/workers) does not set a short deadline,
// one slow or hanging dependency response occupies the calling
// goroutine for the full simulated delay.
type Client struct {
	Sim      *Simulator
	Breaker  *Breaker
	RetryMax int
	Rec      Recorder
}

func NewClient(sim *Simulator, breaker *Breaker, retryMax int, rec Recorder) *Client {
	return &Client{Sim: sim, Breaker: breaker, RetryMax: retryMax, Rec: rec}
}

// Call issues the dependency call, retrying retryable failures up to
// RetryMax times, and consulting/updating the breaker using
// scopeFn - callers pass GlobalScope (seeded) or TenantScope
// (corrected).
func (c *Client) Call(ctx context.Context, tenantID, orderID string, scope string) error {
	if !c.Breaker.Allow(scope) {
		return domain.ErrDependencyFailed
	}
	var lastErr error
	for attempt := 1; attempt <= c.RetryMax; attempt++ {
		start := time.Now()
		err := c.Sim.Call(ctx, tenantID)
		dur := time.Since(start)
		outcome := "ok"
		if err != nil {
			outcome = "error"
		}
		if c.Rec != nil {
			c.Rec.RecordDependencyCall(domain.DependencyCallRecord{
				ID:       "dep-" + tenantID + "-" + time.Now().Format("150405.000000000"),
				TenantID: tenantID, Dependency: c.Sim.name, OrderID: orderID,
				Attempt: attempt, Layer: "http-client", Outcome: outcome,
				DurationMS: dur.Milliseconds(), Timestamp: time.Now().UTC(),
			})
		}
		if err == nil {
			c.Breaker.RecordSuccess(scope)
			return nil
		}
		lastErr = err
		c.Breaker.RecordFailure(scope)
		if ctx.Err() != nil {
			return ctx.Err()
		}
	}
	return lastErr
}
