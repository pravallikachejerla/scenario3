// Package main is a race-instrumented deployment entrypoint.
//
// It is the same wiring as cmd/api/main.go, plus a single test-only
// hook: the STAB-08 regression requires widening the gap between
// FulfilOrder's first lock and its second lock so two concurrent
// fulfil calls reliably overlap in the window between the unlocked
// GetOrder/Version read and the locked TransitionOrder call. The prod
// entrypoint does not expose that knob; this one reads
// STORAGE_LOCK_GAP_MS from the environment and applies it to
// storage.LockAcquireGap at process start, then runs the same
// application.New / httpapi.New pipeline as cmd/api.
//
// This binary is built and used ONLY for the Wave-1 STAB-08
// reproduction. It does not change cmd/api or any other production
// surface.
package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/application"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/httpapi"
	"github.com/soma-genesis/scenario-3-order-stability/internal/observability"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
)

func init() {
	if raw := os.Getenv("STORAGE_LOCK_GAP_MS"); raw != "" {
		if ms, err := strconv.Atoi(raw); err == nil && ms > 0 {
			storage.LockAcquireGap = time.Duration(ms) * time.Millisecond
			log.Printf("api-race: STORAGE_LOCK_GAP_MS=%d applied", ms)
		}
	}
}

func main() {
	addr := getenv("APP_ADDR", ":8081")
	m := observability.NewAppMetrics()
	store := storage.NewMemoryStore()
	seedDemoData(store)
	identity := application.BuildIdentity{
		Commit:    getenv("BUILD_COMMIT", "dev"),
		BuiltAt:   getenv("BUILD_BUILT_AT", time.Now().UTC().Format(time.RFC3339)),
		DatasetID: getenv("BUILD_DATASET_ID", ""),
	}
	app := application.New(store, m, identity)
	app.SandboxMode = getenv("APP_SANDBOX_MODE", "true") == "true"
	defer app.Pool.Close()

	s := httpapi.New(app)
	srv := &http.Server{Addr: addr, Handler: s, ReadHeaderTimeout: 5 * time.Second}
	go func() {
		log.Printf("scenario-3 order-management api-race listening on %s", addr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("listen: %v", err)
		}
	}()
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = srv.Shutdown(ctx)
}

func seedDemoData(store *storage.MemoryStore) {
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "Aurora Retail"})
	store.PutTenant(domain.Tenant{ID: "tenant-b", Name: "Boreal Wholesale"})
	store.PutInventory(domain.InventoryItem{SKU: "SKU-100", TenantID: "tenant-a", OnHand: 500})
	store.PutInventory(domain.InventoryItem{SKU: "SKU-200", TenantID: "tenant-a", OnHand: 200})
	store.PutInventory(domain.InventoryItem{SKU: "SKU-100", TenantID: "tenant-b", OnHand: 300})
	store.PutOrder(domain.Order{
		ID: "ord-demo-1", TenantID: "tenant-a", CustomerID: "cust-1", Status: domain.OrderPending,
		Items:   []domain.OrderItem{{SKU: "SKU-100", Quantity: 2, UnitJPY: 3500}},
		Version: 0, CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	store.PutConfig(domain.ConfigVersion{
		ID: "cfg-tenant-a-fulfilment.dependency_timeout_ms", TenantID: "tenant-a",
		Key: "fulfilment.dependency_timeout_ms", Value: "2000", Version: 1, Active: true, CreatedAt: time.Now().UTC(),
	})
}

func getenv(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}
