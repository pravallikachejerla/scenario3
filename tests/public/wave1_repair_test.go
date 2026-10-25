package public_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/application"
	"github.com/soma-genesis/scenario-3-order-stability/internal/configuration"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/httpapi"
	"github.com/soma-genesis/scenario-3-order-stability/internal/observability"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
	"github.com/soma-genesis/scenario-3-order-stability/internal/workers"
)

// --- F-01: Lock-order inversion -----------------------------------------

func TestWave1_F01_ConcurrentCancelFulfilNoDeadlock(t *testing.T) {
	store := storage.NewMemoryStore()
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "Test"})
	store.PutInventory(domain.InventoryItem{SKU: "SKU-1", TenantID: "tenant-a", OnHand: 100})
	store.PutOrder(domain.Order{
		ID: "ord-1", TenantID: "tenant-a", CustomerID: "cust-1",
		Status: domain.OrderApproved,
		Items:  []domain.OrderItem{{SKU: "SKU-1", Quantity: 1, UnitJPY: 1000}},
	})

	origGap := storage.LockAcquireGap
	storage.LockAcquireGap = 50 * time.Millisecond
	defer func() { storage.LockAcquireGap = origGap }()

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()

	var wg sync.WaitGroup
	wg.Add(2)
	go func() { defer wg.Done(); _ = store.CancelOrder(ctx, "ord-1") }()
	go func() { defer wg.Done(); _ = store.FulfilOrder(ctx, "ord-1") }()

	done := make(chan struct{})
	go func() { wg.Wait(); close(done) }()

	select {
	case <-done:
	case <-ctx.Done():
		t.Fatalf("F-01: concurrent cancel+fulfil deadlocked (ctx deadline exceeded)")
	}
}

// --- F-03: Configuration cache invalidation ----------------------------

func TestWave1_F03_ConfigUpdateInvalidatesCache(t *testing.T) {
	store := storage.NewMemoryStore()
	svc := configuration.NewService(store)

	svc.Update(domain.ConfigVersion{
		ID: "cfg-1", TenantID: "tenant-a", Key: "routing.default_carrier",
		Value: "carrier-X", Version: 1, Active: true, CreatedAt: time.Now().UTC(),
	})

	v1, ok := svc.Get("tenant-a", "routing.default_carrier")
	if !ok || v1.Value != "carrier-X" {
		t.Fatalf("expected initial value carrier-X, got %q (ok=%v)", v1.Value, ok)
	}

	svc.Update(domain.ConfigVersion{
		ID: "cfg-2", TenantID: "tenant-a", Key: "routing.default_carrier",
		Value: "carrier-Y", Version: 2, Active: true, CreatedAt: time.Now().UTC(),
	})

	v2, ok := svc.Get("tenant-a", "routing.default_carrier")
	if !ok {
		t.Fatal("expected to find the updated config")
	}
	if v2.Value != "carrier-Y" {
		t.Fatalf("F-03: expected carrier-Y after update, got %q (stale cache)", v2.Value)
	}
}

func TestWave1_F03_ConfigUpdateInvalidatesCache_HTTP(t *testing.T) {
	store := storage.NewMemoryStore()
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "Test"})
	m := observability.NewAppMetrics()
	app := application.New(store, m, application.BuildIdentity{Commit: "test", BuiltAt: time.Now().UTC().Format(time.RFC3339)})
	app.SandboxMode = false
	defer app.Pool.Close()
	srv := httptest.NewServer(httpapi.New(app))
	defer srv.Close()

	put := func(value string) {
		body := strings.NewReader(`{"value":"` + value + `"}`)
		req, _ := http.NewRequest("PUT", srv.URL+"/api/v1/configuration/tenant-a/routing.default_carrier", body)
		req.Header.Set("Content-Type", "application/json")
		resp, err := http.DefaultClient.Do(req)
		if err != nil {
			t.Fatalf("PUT failed: %v", err)
		}
		resp.Body.Close()
		if resp.StatusCode != 200 {
			t.Fatalf("PUT returned %d", resp.StatusCode)
		}
	}

	get := func() string {
		resp, err := http.Get(srv.URL + "/api/v1/configuration/tenant-a/routing.default_carrier")
		if err != nil {
			t.Fatalf("GET failed: %v", err)
		}
		defer resp.Body.Close()
		var c domain.ConfigVersion
		if err := json.NewDecoder(resp.Body).Decode(&c); err != nil {
			t.Fatalf("decode failed: %v", err)
		}
		return c.Value
	}

	put("carrier-X")
	if got := get(); got != "carrier-X" {
		t.Fatalf("expected carrier-X after first PUT, got %q", got)
	}
	put("carrier-Y")
	if got := get(); got != "carrier-Y" {
		t.Fatalf("F-03: expected carrier-Y after second PUT, got %q (stale cache via HTTP)", got)
	}
}

// --- F-04: Bounded per-job timeout --------------------------------------

func TestWave1_F04_BoundedJobTimeout(t *testing.T) {
	pool := workers.NewPool(2)
	pool.SetJobTimeout(100 * time.Millisecond)
	pool.Start()
	defer pool.Close()

	jobDuration := 2 * time.Second
	type result struct {
		id       int
		started  time.Time
		finished time.Time
	}
	results := make(chan result, 4)

	for i := 0; i < 4; i++ {
		id := i
		pool.Submit(workers.Job{
			ID: fmt.Sprintf("job-%d", id),
			Run: func(ctx context.Context) error {
				started := time.Now()
				select {
				case <-time.After(jobDuration):
					results <- result{id: id, started: started, finished: time.Now()}
					return nil
				case <-ctx.Done():
					results <- result{id: id, started: started, finished: time.Now()}
					return ctx.Err()
				}
			},
		})
	}

	for i := 0; i < 4; i++ {
		select {
		case r := <-results:
			elapsed := r.finished.Sub(r.started)
			if elapsed > 500*time.Millisecond {
				t.Fatalf("F-04: job %d ran for %v, expected ~%v (bounded by JobTimeout)", r.id, elapsed, 100*time.Millisecond)
			}
		case <-time.After(5 * time.Second):
			t.Fatal("F-04: jobs did not complete within 5s")
		}
	}
}

// --- F-10: Caller context propagation -----------------------------------

func TestWave1_F10_CallerContextCancellation(t *testing.T) {
	pool := workers.NewPool(1)
	pool.SetJobTimeout(0)
	pool.Start()
	defer pool.Close()

	jobStarted := make(chan struct{}, 1)
	jobReturned := make(chan time.Time, 1)

	callerCtx, cancel := context.WithCancel(context.Background())
	pool.Submit(workers.Job{
		ID:  "job-1",
		Ctx: callerCtx,
		Run: func(ctx context.Context) error {
			close(jobStarted)
			select {
			case <-time.After(10 * time.Second):
				return nil
			case <-ctx.Done():
				jobReturned <- time.Now()
				return ctx.Err()
			}
		},
	})

	<-jobStarted
	cancel()

	select {
	case returned := <-jobReturned:
		elapsed := time.Since(returned)
		if elapsed > 500*time.Millisecond {
			t.Fatalf("F-10: job did not return promptly after caller cancellation (took %v)", elapsed)
		}
	case <-time.After(3 * time.Second):
		t.Fatal("F-10: job did not return within 3s of caller cancellation")
	}
}
