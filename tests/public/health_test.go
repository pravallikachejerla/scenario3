// Package public contains the normal-engineering tests that run on
// every build. They exercise ordinary, documented behaviour and do
// not reveal the location or nature of any seeded STAB condition.
package public_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/application"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
	"github.com/soma-genesis/scenario-3-order-stability/internal/httpapi"
	"github.com/soma-genesis/scenario-3-order-stability/internal/observability"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
)

func newTestServer(t *testing.T) (*httptest.Server, *storage.MemoryStore) {
	t.Helper()
	store := storage.NewMemoryStore()
	store.PutTenant(domain.Tenant{ID: "tenant-a", Name: "tenant-a"})
	store.PutInventory(domain.InventoryItem{SKU: "SKU-1", TenantID: "tenant-a", OnHand: 50})
	store.PutOrder(domain.Order{
		ID: "ord-1", TenantID: "tenant-a", CustomerID: "cust-1", Status: domain.OrderPending,
		Items:     []domain.OrderItem{{SKU: "SKU-1", Quantity: 1, UnitJPY: 1000}},
		CreatedAt: time.Now().UTC(), UpdatedAt: time.Now().UTC(),
	})
	m := observability.NewAppMetrics()
	app := application.New(store, m, application.BuildIdentity{Commit: "test", BuiltAt: "now"})
	t.Cleanup(app.Pool.Close)
	srv := httptest.NewServer(httpapi.New(app))
	t.Cleanup(srv.Close)
	return srv, store
}

func TestHealthz(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestReadyz(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/readyz")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestVersionEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/version")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var v map[string]string
	if err := json.NewDecoder(resp.Body).Decode(&v); err != nil {
		t.Fatal(err)
	}
	if v["commit"] != "test" {
		t.Fatalf("expected commit=test, got %v", v["commit"])
	}
}

func TestMetricsEndpointServesPrometheusFormat(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/metrics")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
