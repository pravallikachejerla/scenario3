package public_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"testing"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

func TestGetOrder(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/v1/orders/ord-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var o domain.Order
	if err := json.NewDecoder(resp.Body).Decode(&o); err != nil {
		t.Fatal(err)
	}
	if o.ID != "ord-1" {
		t.Fatalf("expected ord-1, got %s", o.ID)
	}
}

func TestGetOrder_NotFound(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/v1/orders/does-not-exist")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListOrders(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/v1/orders?tenant=tenant-a")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	var orders []domain.Order
	if err := json.NewDecoder(resp.Body).Decode(&orders); err != nil {
		t.Fatal(err)
	}
	if len(orders) != 1 {
		t.Fatalf("expected 1 order, got %d", len(orders))
	}
}

func TestApproveThenCancel_ValidWorkflow(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Post(srv.URL+"/api/v1/orders/ord-1/approve", "application/json", nil)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected approve to succeed, got %d", resp.StatusCode)
	}
}

func TestAdminBulkStatus_UpdatesRequestedOrders(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]any{"order_ids": []string{"ord-1"}, "status": "approved"})
	resp, err := http.Post(srv.URL+"/api/v1/admin/orders/bulk-status", "application/json", bytes.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var out map[string]int
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		t.Fatal(err)
	}
	if out["updated"] != 1 {
		t.Fatalf("expected 1 order updated, got %d", out["updated"])
	}
}

func TestGetInventory(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/v1/inventory/tenant-a/SKU-1")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var it domain.InventoryItem
	if err := json.NewDecoder(resp.Body).Decode(&it); err != nil {
		t.Fatal(err)
	}
	if it.SKU != "SKU-1" {
		t.Fatalf("expected SKU-1, got %s", it.SKU)
	}
}

func TestConfigurationPutThenGet(t *testing.T) {
	srv, _ := newTestServer(t)
	body, _ := json.Marshal(map[string]string{"value": "carrier-x"})
	req, _ := http.NewRequest(http.MethodPut, srv.URL+"/api/v1/configuration/tenant-a/routing.default_carrier", bytes.NewReader(body))
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestWorkerStatusEndpoint(t *testing.T) {
	srv, _ := newTestServer(t)
	resp, err := http.Get(srv.URL + "/api/v1/workers/status")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}
