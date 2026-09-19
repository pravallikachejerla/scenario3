// Package wave1 contains the regression suite for the Wave-1 STAB set.
//
// This file is the STAB-08 ("concurrent order-state race") regression
// test at the storage boundary. It exercises the same FulfilOrder /
// CancelOrder call sites that the public HTTP API surfaces through
// internal/fulfilment/service.go and internal/orders/service.go, so
// a regression here is observable from the order-management surface,
// not just from the in-memory store.
//
// Why this test calls FulfilOrder (the real fixed call site) and not
// TransitionOrderSafe directly:
//
//   TransitionOrderSafe is the CAS primitive. A regression test that
//   calls TransitionOrderSafe directly would never fail against the
//   pre-fix code, because the pre-fix code routes FulfilOrder and
//   CancelOrder through TransitionOrderUnsafe - the regression is in
//   the call site, not in the CAS primitive. The whole point of this
//   test is to prove that the real FulfilOrder call site now uses the
//   CAS-guarded path under contention. Calling TransitionOrderSafe
//   would bypass the regression entirely.
//
// How the race window is widened:
//
//   storage.LockAcquireGap is the existing test-only hook (default
//   zero) that the STAB-01 suite already uses to widen the gap
//   between CancelOrder / FulfilOrder acquiring their first lock and
//   their second lock. In a deadlock test the gap forces the two
//   goroutines to overlap on their *first* lock; here the same gap
//   is re-purposed to widen the window between FulfilOrder's initial
//   (unlocked) GetOrder / Version read and its final (locked)
//   transition call, so two concurrent FulfilOrder callers reliably
//   race against the same starting version instead of serialising
//   against the order lock in some interleaving that happens to
//   hide the bug.
//
// Assertions:
//
//   - exactly one of the two concurrent FulfilOrder calls returns nil
//   - the other returns domain.ErrConcurrentUpdate (the CAS loser)
//   - the order's final Version advanced by exactly one (no lost
//     write, regardless of which goroutine happened to win)
//
// Cleanup:
//
//   t.Cleanup always resets storage.LockAcquireGap to zero, so this
//   test cannot poison a sibling test with a widened race window
//   even if it panics mid-flight.
package wave1_test

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
)

// seedFulfilableOrder returns a MemoryStore containing a single
// tenant, a single SKU with enough stock and reservation to satisfy
// a FulfilOrder, and one OrderReserved order that is a legal source
// state for OrderFulfilling. The order's Version is left at zero so
// the CAS guard comparison is deterministic.
func seedFulfilableOrder(t *testing.T) (*storage.MemoryStore, string) {
	t.Helper()
	store := storage.NewMemoryStore()
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	store.PutInventory(domain.InventoryItem{
		SKU:      "SKU-1",
		TenantID: "tenant-a",
		OnHand:   100,
		Reserved: 3,
	})
	const orderID = "ord-fight"
	store.PutOrder(domain.Order{
		ID:        orderID,
		TenantID:  "tenant-a",
		CustomerID: "cust-1",
		Status:    domain.OrderReserved,
		Items: []domain.OrderItem{
			{SKU: "SKU-1", Quantity: 3, UnitJPY: 1000},
		},
		Version:   0,
		CreatedAt: time.Now().UTC(),
		UpdatedAt: time.Now().UTC(),
	})
	return store, orderID
}

// TestSTAB08_Regression is the Wave-1 regression test for the
// concurrent-order-state race at the storage boundary.
//
// Two Goroutines:
//
//   G1: FulfilOrder(ctx, id)   // the real, fixed call site
//   G2: FulfilOrder(ctx, id)   // the real, fixed call site, contending
//
// Both call FulfilOrder on the same OrderReserved order from a fresh
// store. Because FulfilOrder's initial GetOrder is unlocked, and the
// race window between that read and the final TransitionOrderSafe has
// been widened via storage.LockAcquireGap, both goroutines read the
// same starting Version (0) and both attempt the final transition
// with expectedVersion=0. The version-guarded TransitionOrderSafe
// lets exactly one of them succeed and returns
// domain.ErrConcurrentUpdate to the other.
//
// Assertions:
//
//   - one and only one call returns nil
//   - the other call's error, after errors.Is, matches
//     domain.ErrConcurrentUpdate
//   - the order's final Version is 1 (not 2, which would indicate
//     both writes succeeded; not 0, which would indicate no writes)
func TestSTAB08_Regression(t *testing.T) {
	t.Cleanup(func() {
		storage.LockAcquireGap = 0
	})

	store, orderID := seedFulfilableOrder(t)

	// ~20ms is what the private TestSTAB08 suite uses for
	// TransitionRaceWindow; we re-use the same magnitude here so the
	// race window between FulfilOrder's GetOrder and its final
	// TransitionOrderSafe reliably contains the scheduler's
	// interleaving of two sibling goroutines on a 1-2 CPU sandbox.
	storage.LockAcquireGap = 20 * time.Millisecond

	var wg sync.WaitGroup
	wg.Add(2)
	var err1, err2 error

	gate := make(chan struct{})

	go func() {
		defer wg.Done()
		<-gate
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err1 = store.FulfilOrder(ctx, orderID)
	}()

	go func() {
		defer wg.Done()
		<-gate
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		err2 = store.FulfilOrder(ctx, orderID)
	}()

	// Open the starting gate together so both goroutines begin
	// within the same window of the scheduler.
	close(gate)
	wg.Wait()

	// Exactly one nil, exactly one ErrConcurrentUpdate.
	successes := 0
	var loser error
	if err1 == nil {
		successes++
	} else {
		loser = err1
	}
	if err2 == nil {
		successes++
	} else {
		loser = err2
	}
	if successes != 1 {
		t.Fatalf("expected exactly one of two concurrent FulfilOrder calls to succeed under the CAS guard, got %d successes (err1=%v err2=%v)", successes, err1, err2)
	}
	if !errors.Is(loser, domain.ErrConcurrentUpdate) {
		t.Fatalf("expected the losing FulfilOrder call to return domain.ErrConcurrentUpdate, got %v", loser)
	}

	// Final Version must advance by exactly one, regardless of
	// which goroutine won. Version 2 would mean both writes
	// succeeded (the bug we are guarding against); Version 0 would
	// mean none did.
	final, err := store.GetOrder(context.Background(), orderID)
	if err != nil {
		t.Fatalf("failed to read final order state: %v", err)
	}
	if final.Version != 1 {
		t.Fatalf("expected the order's final Version to advance by exactly one, got %d (status=%s)", final.Version, final.Status)
	}
	// And the winning workflow must have set the order to
	// OrderFulfilling (the transition the winning FulfilOrder
	// tried to apply).
	if final.Status != domain.OrderFulfilling {
		t.Fatalf("expected the winning FulfilOrder to leave status=OrderFulfilling, got %s", final.Status)
	}
}

