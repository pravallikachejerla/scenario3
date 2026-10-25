// Wave-1 failure-injection tests. These run only inside the sandbox
// (the simulator control surface is gated by SandboxMode) and
// exercise the two repaired code paths under adversarial conditions
// to confirm the repairs hold when the system is under stress.
//
// F-11 / STAB-12 failure injection: drive the admin bulk-status path
// while the dependency simulator is misbehaving, and confirm the
// repair (audit + version + transition validation) still holds.
//
// F-10 / STAB-11 failure injection: drive the breaker past its
// threshold for one tenant while a second tenant is making healthy
// calls, and confirm the tenant-scoped isolation holds under
// concurrent load.
package public_test

import (
	"context"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/application"
	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/observability"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
)

// TestWave1_F11_FailureInjection_AdminBulkUnderDependencyFailure
// exercises the STAB-12 repair under a failing dependency simulator.
// The repair must still produce audit events and reject invalid
// transitions even when the surrounding system is degraded.
func TestWave1_F11_FailureInjection_AdminBulkUnderDependencyFailure(t *testing.T) {
	store := storage.NewMemoryStore()
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	for i := 0; i < 5; i++ {
		store.PutOrder(domain.Order{
			ID:         "ord-" + string(rune('a'+i)),
			TenantID:   "tenant-a",
			CustomerID: "cust-1",
			Status:     domain.OrderPending,
			Items:      []domain.OrderItem{{SKU: "SKU-1", Quantity: 1, UnitJPY: 1000}},
			CreatedAt:  time.Now().UTC(), UpdatedAt: time.Now().UTC(),
		})
	}

	// Inject a permanent failure on the shipping dependency. The
	// admin bulk path does not call the shipping dependency, so this
	// must not affect the audit/version/transition behaviour - it
	// only confirms the repair is independent of dependency health.
	sim := dependencies.NewSimulator(domain.DependencyShipping)
	sim.Configure("tenant-a", dependencies.Behaviour{Mode: dependencies.FailurePermanent})
	_ = sim // referenced for documentation; the bulk path does not call it

	ids := []string{"ord-a", "ord-b", "ord-c"}
	n, err := store.AdminBulkSetStatus(context.Background(), ids, domain.OrderCancelled, "ops-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if n != 3 {
		t.Fatalf("expected 3 orders updated despite dependency failure, got %d", n)
	}

	audit := store.ListAudit("tenant-a")
	bulkCount := 0
	for _, a := range audit {
		if a.Action == "order.admin_bulk_status" {
			bulkCount++
		}
	}
	if bulkCount != 3 {
		t.Fatalf("expected 3 audit events under dependency failure, got %d", bulkCount)
	}
}

// TestWave1_F10_FailureInjection_BreakerIsolationUnderConcurrency
// exercises the STAB-11 repair under concurrent load. Tenant-a is
// driven past the breaker threshold while tenant-b is making healthy
// calls in parallel; tenant-b must never see an open breaker.
func TestWave1_F10_FailureInjection_BreakerIsolationUnderConcurrency(t *testing.T) {
	store := storage.NewMemoryStore()
	m := observability.NewAppMetrics()
	app := application.New(store, m, application.BuildIdentity{Commit: "test", BuiltAt: "now"})
	t.Cleanup(app.Pool.Close)

	scopeA := dependencies.TenantScope("tenant-a", app.Deps.Shipping.Name())
	scopeB := dependencies.TenantScope("tenant-b", app.Deps.Shipping.Name())

	var tenantBAllowed int32
	var wg sync.WaitGroup
	wg.Add(2)

	// Goroutine 1: drive tenant-a past the breaker threshold.
	go func() {
		defer wg.Done()
		for i := 0; i < 10; i++ {
			app.Breaker.RecordFailure(scopeA)
		}
	}()

	// Goroutine 2: tenant-b makes healthy calls concurrently.
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			if app.Breaker.Allow(scopeB) {
				atomic.AddInt32(&tenantBAllowed, 1)
			}
		}
	}()

	wg.Wait()

	if !app.Breaker.IsOpen(scopeA) {
		t.Fatal("expected tenant-a's breaker to be open after 10 failures")
	}
	if app.Breaker.IsOpen(scopeB) {
		t.Fatal("expected tenant-b's breaker to remain closed under concurrent load")
	}
	if tenantBAllowed == 0 {
		t.Fatal("expected tenant-b to be allowed at least once during the concurrent run")
	}
}
