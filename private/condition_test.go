// Package private contains the condition-level tests that exercise
// the twelve planted STAB source conditions. They are intentionally
// not run as part of the public CI (see tests/public); they are the
// private evaluator hooks referenced by the issue manifest
// (docs/private-issue-manifest.yaml) and VERIFICATION.md.
//
// Every test both reproduces its condition against the seeded
// (buggy) code path and exercises the corrected primitive the same
// package exposes, so the suite documents that each condition is
// both detectable and solvable without prescribing the only
// acceptable repair shape.
package private_test

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/batch"
	"github.com/soma-genesis/scenario-3-order-stability/internal/configuration"
	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/events"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
	"github.com/soma-genesis/scenario-3-order-stability/internal/workers"
)

func freshOrderStore(t *testing.T, status domain.OrderStatus, sku string, qty int) (*storage.MemoryStore, string) {
	t.Helper()
	s := storage.NewMemoryStore()
	s.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	s.PutInventory(domain.InventoryItem{SKU: sku, TenantID: "tenant-a", OnHand: 100, Reserved: qty})
	id := "ord-1"
	s.PutOrder(domain.Order{
		ID: id, TenantID: "tenant-a", CustomerID: "cust-1", Status: status,
		Items: []domain.OrderItem{{SKU: sku, Quantity: qty, UnitJPY: 1000}},
		Version: 0, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	return s, id
}

// --- STAB-01: inconsistent database lock ordering -----------------------

func TestSTAB01_LockOrdering(t *testing.T) {
	t.Cleanup(func() { storage.LockAcquireGap = 0 })

	t.Run("seeded_reproduces_deadlock_timeout", func(t *testing.T) {
		s, id := freshOrderStore(t, domain.OrderReserved, "SKU-1", 3)
		// The gap must be smaller than the per-call timeout so both
		// goroutines finish acquiring their *first* lock and start
		// waiting on their *second* lock before either context
		// expires - that is what turns this into a genuine circular
		// wait rather than two independent timeouts.
		storage.LockAcquireGap = 15 * time.Millisecond

		var cancelErr, fulfilErr error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			cancelErr = s.CancelOrder(ctx, id)
		}()
		go func() {
			defer wg.Done()
			ctx, cancel := context.WithTimeout(context.Background(), 150*time.Millisecond)
			defer cancel()
			fulfilErr = s.FulfilOrder(ctx, id)
		}()
		wg.Wait()

		// Clean behaviour: with consistent lock ordering, two
		// workflows on the same order simply serialise and both
		// complete - no lock-acquisition timeout is possible for a
		// single order. Seeded behaviour: because CancelOrder locks
		// order-then-inventory while FulfilOrder locks
		// inventory-then-order, the two sides can each hold what the
		// other needs next; like a real database deadlock detector,
		// at least one side is forced to abort with a lock-timeout
		// once the circular wait is detected (the other may then
		// proceed once the aborted side releases its lock).
		if cancelErr == nil && fulfilErr == nil {
			t.Fatalf("expected the reversed lock ordering to force at least one of the two concurrent operations to time out, got cancelErr=%v fulfilErr=%v", cancelErr, fulfilErr)
		}
	})

	t.Run("consistent_ordering_does_not_deadlock", func(t *testing.T) {
		// Two independent orders (disjoint lock keys) never contend,
		// which is the shape a corrected FulfilOrder must preserve:
		// locking order-then-inventory like CancelOrder removes the
		// circular wait for the *same* order+SKU pair.
		s := storage.NewMemoryStore()
		s.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
		s.PutInventory(domain.InventoryItem{SKU: "SKU-1", TenantID: "tenant-a", OnHand: 100, Reserved: 1})
		s.PutInventory(domain.InventoryItem{SKU: "SKU-2", TenantID: "tenant-a", OnHand: 100, Reserved: 1})
		s.PutOrder(domain.Order{ID: "ord-a", TenantID: "tenant-a", Status: domain.OrderReserved,
			Items: []domain.OrderItem{{SKU: "SKU-1", Quantity: 1}}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
		s.PutOrder(domain.Order{ID: "ord-b", TenantID: "tenant-a", Status: domain.OrderReserved,
			Items: []domain.OrderItem{{SKU: "SKU-2", Quantity: 1}}, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC()})
		ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
		defer cancel()
		var wg sync.WaitGroup
		var err1, err2 error
		wg.Add(2)
		go func() { defer wg.Done(); err1 = s.CancelOrder(ctx, "ord-a") }()
		go func() { defer wg.Done(); err2 = s.FulfilOrder(ctx, "ord-b") }()
		wg.Wait()
		if err1 != nil || err2 != nil {
			t.Fatalf("unexpected error for disjoint orders: %v / %v", err1, err2)
		}
	})
}

// --- STAB-02: duplicate event processing ---------------------------------

func TestSTAB02_DuplicateEventProcessing(t *testing.T) {
	ev := events.InboundEvent{EventID: "evt-100", OrderID: "ord-1", Type: "inventory.decrement"}

	t.Run("seeded_process_applies_effect_twice_on_redelivery", func(t *testing.T) {
		s := storage.NewMemoryStore()
		var applied int32
		c := events.NewConsumer(s, func(events.InboundEvent) error {
			atomic.AddInt32(&applied, 1)
			return nil
		})
		if err := c.Process(ev); err != nil {
			t.Fatal(err)
		}
		if err := c.Process(ev); err != nil { // redelivery of the same broker message
			t.Fatal(err)
		}
		if got := atomic.LoadInt32(&applied); got != 2 {
			t.Fatalf("expected Process (no idempotency guard) to apply the effect twice on redelivery, got %d", got)
		}
	})

	t.Run("corrected_processidempotent_applies_effect_once", func(t *testing.T) {
		s := storage.NewMemoryStore()
		var applied int32
		c := events.NewConsumer(s, func(events.InboundEvent) error {
			atomic.AddInt32(&applied, 1)
			return nil
		})
		if err := c.ProcessIdempotent(ev); err != nil {
			t.Fatal(err)
		}
		if err := c.ProcessIdempotent(ev); err != nil {
			t.Fatal(err)
		}
		if got := atomic.LoadInt32(&applied); got != 1 {
			t.Fatalf("expected ProcessIdempotent to apply the effect exactly once, got %d", got)
		}
	})
}

// --- STAB-03: stale configuration cache -----------------------------------

func TestSTAB03_StaleConfigurationCache(t *testing.T) {
	s := storage.NewMemoryStore()
	s.PutConfig(domain.ConfigVersion{ID: "cfg-1", TenantID: "tenant-a", Key: "routing.carrier", Value: "v1", Version: 1, Active: true})
	svc := configuration.NewService(s)

	v, ok := svc.Get("tenant-a", "routing.carrier")
	if !ok || v.Value != "v1" {
		t.Fatalf("expected initial value v1, got %+v ok=%v", v, ok)
	}

	svc.Update(domain.ConfigVersion{ID: "cfg-1", TenantID: "tenant-a", Key: "routing.carrier", Value: "v2", Version: 2, Active: true})

	t.Run("seeded_get_still_returns_stale_value_after_update", func(t *testing.T) {
		v2, _ := svc.Get("tenant-a", "routing.carrier")
		if v2.Value != "v1" {
			t.Fatalf("expected Get to still return stale cached value v1 (no invalidation on Update), got %q", v2.Value)
		}
	})

	t.Run("corrected_invalidate_then_get_returns_new_value", func(t *testing.T) {
		svc.Invalidate("tenant-a", "routing.carrier")
		v3, ok := svc.Get("tenant-a", "routing.carrier")
		if !ok || v3.Value != "v2" {
			t.Fatalf("expected refreshed value v2 after Invalidate, got %+v ok=%v", v3, ok)
		}
	})
}

// --- STAB-04: worker-pool exhaustion during dependency timeout -----------

func TestSTAB04_WorkerPoolExhaustion(t *testing.T) {
	const delay = 120 * time.Millisecond

	t.Run("seeded_no_bounded_timeout_exhausts_pool", func(t *testing.T) {
		sim := dependencies.NewSimulator(domain.DependencyShipping)
		sim.Configure("tenant-a", dependencies.Behaviour{Delay: delay, Mode: dependencies.FailureNone})
		pool := workers.NewPool(2)
		pool.Start()
		start := time.Now()
		for i := 0; i < 6; i++ {
			pool.Submit(workers.Job{ID: fmt.Sprintf("job-%d", i), Ctx: context.Background(), Run: func(ctx context.Context) error {
				return sim.Call(ctx, "tenant-a")
			}})
		}
		pool.Close()
		elapsed := time.Since(start)
		// 6 jobs / 2 workers, each occupying a worker for the full
		// delay because no bounded per-call timeout is enforced:
		// ~3 rounds x 120ms = ~360ms.
		if elapsed < 300*time.Millisecond {
			t.Fatalf("expected pool exhaustion to take at least ~300ms with no bounded timeout, took %s", elapsed)
		}
	})

	t.Run("bounded_timeout_fails_fast_and_drains_quickly", func(t *testing.T) {
		sim := dependencies.NewSimulator(domain.DependencyShipping)
		sim.Configure("tenant-a", dependencies.Behaviour{Delay: delay, Mode: dependencies.FailureNone})
		pool := workers.NewPool(2)
		pool.Start()
		start := time.Now()
		for i := 0; i < 6; i++ {
			pool.Submit(workers.Job{ID: fmt.Sprintf("job-%d", i), Run: func(ctx context.Context) error {
				bounded, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
				defer cancel()
				return sim.Call(bounded, "tenant-a")
			}})
		}
		pool.Close()
		elapsed := time.Since(start)
		if elapsed > 200*time.Millisecond {
			t.Fatalf("expected a bounded per-call timeout to keep total drain time low, took %s", elapsed)
		}
	})
}

// --- STAB-05: batch memory retention ---------------------------------------

func TestSTAB05_BatchMemoryRetention(t *testing.T) {
	const n = 200
	records := make([]batch.Record, n)
	for i := range records {
		records[i] = batch.Record{OrderID: fmt.Sprintf("ord-%d", i), Payload: make([]byte, 1024)}
	}

	t.Run("seeded_retains_entire_batch_simultaneously", func(t *testing.T) {
		var maxRetained int
		err := batch.RunRetainAll(records, func(batch.Record) error { return nil }, func(retained int) {
			if retained > maxRetained {
				maxRetained = retained
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if maxRetained != n {
			t.Fatalf("expected RunRetainAll to retain all %d records simultaneously, observed peak %d", n, maxRetained)
		}
	})

	t.Run("corrected_streaming_bounds_retention", func(t *testing.T) {
		const window = 8
		var maxRetained int
		err := batch.RunStreaming(records, window, func(batch.Record) error { return nil }, func(retained int) {
			if retained > maxRetained {
				maxRetained = retained
			}
		})
		if err != nil {
			t.Fatal(err)
		}
		if maxRetained > window {
			t.Fatalf("expected RunStreaming to bound peak retention to %d, observed %d", window, maxRetained)
		}
	})
}

// --- STAB-06: retry amplification -----------------------------------------

func TestSTAB06_RetryAmplification(t *testing.T) {
	sim := dependencies.NewSimulator(domain.DependencyShipping)
	sim.Configure("tenant-a", dependencies.Behaviour{Mode: dependencies.FailureRetryable}) // never recovers
	breaker := dependencies.NewBreaker(1000)                                              // effectively disabled for this test
	client := dependencies.NewClient(sim, breaker, 3, nil)                                  // layer 1: http-client retries x3

	const serviceRetryMax = 2 // layer 2: fulfilment service retries x2
	const workerRetryMax = 2  // layer 3: worker retries x2

	err := workers.RunWithRetry(func() error {
		var lastErr error
		for i := 0; i < serviceRetryMax; i++ {
			if err := client.Call(context.Background(), "tenant-a", "ord-1", dependencies.GlobalScope(domain.DependencyShipping)); err == nil {
				return nil
			} else {
				lastErr = err
			}
		}
		return lastErr
	}, workerRetryMax)
	if err == nil {
		t.Fatal("expected the permanently failing dependency call to still return an error")
	}

	got := sim.CallCount("tenant-a")
	want := 3 * serviceRetryMax * workerRetryMax // = 12
	if got != want {
		t.Fatalf("expected uncoordinated retries at three layers to multiply to %d simulator calls for one logical request, got %d", want, got)
	}
	const acceptableBound = 5
	if got <= acceptableBound {
		t.Fatalf("sanity check failed: %d calls should exceed the acceptable coordinated-retry bound of %d", got, acceptableBound)
	}
}

// --- STAB-07: environment-dependent configuration drift -------------------

func TestSTAB07_EnvironmentConfigurationDrift(t *testing.T) {
	const key = "fulfilment.dependency_timeout_ms"
	env1 := configuration.Load(configuration.EnvOne, key)
	env2 := configuration.Load(configuration.EnvTwo, key)
	documented, ok := configuration.DocumentedDefault(key)
	if !ok {
		t.Fatal("expected a documented default for the timeout key")
	}

	if env1 != documented {
		t.Fatalf("expected env-1 to match the documented default %q, got %q", documented, env1)
	}
	if env2 == documented {
		t.Fatalf("expected env-2's implicit fallback to differ from the documented default %q (that is the seeded drift), got the same value", documented)
	}
	if env1 == env2 {
		t.Fatalf("expected env-1 (%q) and env-2 (%q) to resolve the same documented key to different runtime values", env1, env2)
	}
}

// --- STAB-08: concurrent order-state race ----------------------------------

func TestSTAB08_ConcurrentOrderStateRace(t *testing.T) {
	t.Cleanup(func() { storage.TransitionRaceWindow = 0 })

	t.Run("seeded_unsafe_transition_allows_both_concurrent_writers_to_succeed", func(t *testing.T) {
		s, id := freshOrderStore(t, domain.OrderApproved, "SKU-1", 1)
		storage.TransitionRaceWindow = 20 * time.Millisecond

		var err1, err2 error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err1 = s.TransitionOrderUnsafe(context.Background(), id, domain.OrderReserved, "agent-1", "reserve", "workflow")
		}()
		go func() {
			defer wg.Done()
			_, err2 = s.TransitionOrderUnsafe(context.Background(), id, domain.OrderCancelled, "agent-2", "cancel", "workflow")
		}()
		wg.Wait()

		// Clean/CAS-protected behaviour: exactly one of two mutually
		// exclusive transitions from the same starting status can
		// succeed. Seeded behaviour: the read-decide-write sequence is
		// not atomic, so both callers observe the same starting status
		// as valid and both writes "succeed" - a lost update.
		if err1 != nil || err2 != nil {
			t.Fatalf("expected the unsafe path to let both concurrent transitions report success (that is the bug), got err1=%v err2=%v", err1, err2)
		}
	})

	t.Run("corrected_cas_transition_rejects_the_loser", func(t *testing.T) {
		s, id := freshOrderStore(t, domain.OrderApproved, "SKU-1", 1)
		storage.TransitionRaceWindow = 20 * time.Millisecond

		var err1, err2 error
		var wg sync.WaitGroup
		wg.Add(2)
		go func() {
			defer wg.Done()
			_, err1 = s.TransitionOrderSafe(context.Background(), id, 0, domain.OrderReserved, "agent-1", "reserve", "workflow")
		}()
		go func() {
			defer wg.Done()
			_, err2 = s.TransitionOrderSafe(context.Background(), id, 0, domain.OrderCancelled, "agent-2", "cancel", "workflow")
		}()
		wg.Wait()

		succeeded := 0
		if err1 == nil {
			succeeded++
		}
		if err2 == nil {
			succeeded++
		}
		if succeeded != 1 {
			t.Fatalf("expected the version-guarded path to let exactly one of two conflicting transitions succeed, got %d (err1=%v err2=%v)", succeeded, err1, err2)
		}
	})
}

// --- STAB-09: partial transaction and event inconsistency -----------------

func TestSTAB09_PartialTransactionEventInconsistency(t *testing.T) {
	t.Run("seeded_failure_between_state_and_outbox_loses_the_event", func(t *testing.T) {
		s := storage.NewMemoryStore()
		var stateChanged int32
		err := events.RecordStateChangeThenPublish(s, "tenant-a", "ord-1", "order.fulfilled", nil,
			func() error { atomic.AddInt32(&stateChanged, 1); return nil },
			func(stage string) error { return fmt.Errorf("simulated crash at %s", stage) },
		)
		if err == nil {
			t.Fatal("expected the injected failure to surface as an error")
		}
		if atomic.LoadInt32(&stateChanged) != 1 {
			t.Fatalf("expected business state to have already changed before the injected failure, got %d", stateChanged)
		}
		if got := len(s.UndispatchedOutbox()); got != 0 {
			t.Fatalf("expected no outbox event to exist after a failure between state commit and publish (state changed but event lost), got %d", got)
		}
	})

	t.Run("corrected_atomic_path_keeps_state_and_outbox_together", func(t *testing.T) {
		s := storage.NewMemoryStore()
		var stateChanged int32
		err := events.RecordStateChangeAtomic(s, "tenant-a", "ord-1", "order.fulfilled", nil,
			func() error { atomic.AddInt32(&stateChanged, 1); return nil },
		)
		if err != nil {
			t.Fatal(err)
		}
		if atomic.LoadInt32(&stateChanged) != 1 {
			t.Fatalf("expected state to change exactly once, got %d", stateChanged)
		}
		if got := len(s.UndispatchedOutbox()); got != 1 {
			t.Fatalf("expected exactly one outbox event staged together with the state change, got %d", got)
		}
	})
}

// --- STAB-10: missing cancellation propagation -----------------------------

func TestSTAB10_MissingCancellationPropagation(t *testing.T) {
	const delay = 150 * time.Millisecond

	t.Run("seeded_pool_ignores_caller_cancellation", func(t *testing.T) {
		sim := dependencies.NewSimulator(domain.DependencyShipping)
		sim.Configure("tenant-a", dependencies.Behaviour{Delay: delay, Mode: dependencies.FailureNone})
		pool := workers.NewPool(1)
		pool.Start()

		callerCtx, cancel := context.WithCancel(context.Background())
		done := make(chan error, 1)
		start := time.Now()
		pool.Submit(workers.Job{ID: "job-1", Ctx: callerCtx, Run: func(ctx context.Context) error {
			err := sim.Call(ctx, "tenant-a")
			done <- err
			return err
		}})
		time.Sleep(5 * time.Millisecond)
		cancel() // caller gives up almost immediately

		select {
		case <-done:
		case <-time.After(2 * time.Second):
			t.Fatal("job never completed")
		}
		elapsed := time.Since(start)
		pool.Close()

		// Seeded behaviour: Worker.run always executes job.Run with
		// context.Background(), so cancelling callerCtx has no effect
		// and the call runs to completion anyway.
		if elapsed < delay-20*time.Millisecond {
			t.Fatalf("expected the pool to ignore caller cancellation and run to completion (~%s), finished after only %s", delay, elapsed)
		}
	})

	t.Run("corrected_propagation_aborts_promptly_on_cancellation", func(t *testing.T) {
		sim := dependencies.NewSimulator(domain.DependencyShipping)
		sim.Configure("tenant-a", dependencies.Behaviour{Delay: delay, Mode: dependencies.FailureNone})

		callerCtx, cancel := context.WithCancel(context.Background())
		start := time.Now()
		go func() {
			time.Sleep(5 * time.Millisecond)
			cancel()
		}()
		err := workers.RunWithPropagation(workers.Job{ID: "job-1", Ctx: callerCtx, Run: func(ctx context.Context) error {
			return sim.Call(ctx, "tenant-a")
		}})
		elapsed := time.Since(start)

		if err == nil {
			t.Fatal("expected cancellation to surface as an error")
		}
		if elapsed > 100*time.Millisecond {
			t.Fatalf("expected propagated cancellation to abort well before the %s dependency delay, took %s", delay, elapsed)
		}
	})
}

// --- STAB-11: shared failure control across tenants -----------------------

func TestSTAB11_SharedFailureControlAcrossTenants(t *testing.T) {
	t.Run("seeded_global_scope_blocks_healthy_tenant", func(t *testing.T) {
		b := dependencies.NewBreaker(2)
		scope := dependencies.GlobalScope(domain.DependencyShipping)
		b.RecordFailure(scope) // tenant-a's failures...
		b.RecordFailure(scope)
		if b.Allow(scope) {
			t.Fatal("expected the breaker to be open after reaching the threshold")
		}
		// A request nominally from a completely healthy tenant-b uses
		// the same global scope key, so it is blocked too.
		if b.Allow(dependencies.GlobalScope(domain.DependencyShipping)) {
			t.Fatal("expected tenant-b's request to be blocked by tenant-a's failures under the global scope key (that is the bug)")
		}
	})

	t.Run("corrected_tenant_scope_isolates_the_breaker", func(t *testing.T) {
		b := dependencies.NewBreaker(2)
		scopeA := dependencies.TenantScope("tenant-a", domain.DependencyShipping)
		scopeB := dependencies.TenantScope("tenant-b", domain.DependencyShipping)
		b.RecordFailure(scopeA)
		b.RecordFailure(scopeA)
		if b.Allow(scopeA) {
			t.Fatal("expected tenant-a's scope to be open")
		}
		if !b.Allow(scopeB) {
			t.Fatal("expected tenant-b's scope to remain healthy and allowed")
		}
	})
}

// --- STAB-12: emergency customisation bypasses workflow invariants --------

func TestSTAB12_EmergencyCustomisationBypassesInvariants(t *testing.T) {
	t.Run("seeded_admin_bulk_path_skips_validation_and_audit", func(t *testing.T) {
		s, id := freshOrderStore(t, domain.OrderPending, "SKU-1", 1)
		// Pending -> Fulfilled is not a valid transition per
		// domain.IsValidTransition, but AdminBulkSetStatus never
		// checks it.
		n, err := s.AdminBulkSetStatus(context.Background(), []string{id}, domain.OrderFulfilled, "support-agent")
		if err != nil {
			t.Fatal(err)
		}
		if n != 1 {
			t.Fatalf("expected 1 order updated, got %d", n)
		}
		got, _ := s.GetOrder(context.Background(), id)
		if got.Status != domain.OrderFulfilled {
			t.Fatalf("expected the bulk path to force the status change despite it being an invalid transition, got %s", got.Status)
		}
		if audit := s.ListAudit("tenant-a"); len(audit) != 0 {
			t.Fatalf("expected the bulk path to leave no audit trail (that is the invariant violation), got %d entries", len(audit))
		}
	})

	t.Run("normal_workflow_path_rejects_the_same_invalid_jump", func(t *testing.T) {
		s, id := freshOrderStore(t, domain.OrderPending, "SKU-1", 1)
		_, err := s.TransitionOrderUnsafe(context.Background(), id, domain.OrderFulfilled, "agent-1", "attempt", "workflow")
		if err == nil {
			t.Fatal("expected the ordinary transition path to reject Pending -> Fulfilled as invalid")
		}
	})
}
