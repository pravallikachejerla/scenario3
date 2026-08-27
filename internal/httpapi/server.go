// Package httpapi exposes the platform's versioned REST API using the
// standard library's method-and-pattern-aware ServeMux (Go 1.22+),
// deliberately avoiding a third-party router so the module has no
// required external build-time dependency.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/application"
	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

func New(app *application.App) http.Handler {
	mux := http.NewServeMux()

	mux.HandleFunc("GET /healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("GET /readyz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"status": "ready", "uptime": time.Since(app.StartedAt).String(),
		})
	})
	mux.HandleFunc("GET /version", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{
			"commit": app.Identity.Commit, "built_at": app.Identity.BuiltAt, "dataset_id": app.Identity.DatasetID,
		})
	})
	mux.HandleFunc("GET /metrics", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/plain; version=0.0.4")
		app.Metrics.RenderTo(w)
	})

	mux.HandleFunc("GET /api/v1/orders", listOrders(app))
	mux.HandleFunc("GET /api/v1/orders/{id}", getOrder(app))
	mux.HandleFunc("POST /api/v1/orders/{id}/approve", approveOrder(app))
	mux.HandleFunc("POST /api/v1/orders/{id}/cancel", cancelOrder(app))
	mux.HandleFunc("POST /api/v1/orders/{id}/fulfil", fulfilOrder(app))
	mux.HandleFunc("POST /api/v1/admin/orders/bulk-status", adminBulkStatus(app))
	mux.HandleFunc("GET /api/v1/inventory/{tenant}/{sku}", getInventory(app))
	mux.HandleFunc("GET /api/v1/configuration/{tenant}/{key}", getConfig(app))
	mux.HandleFunc("PUT /api/v1/configuration/{tenant}/{key}", putConfig(app))
	mux.HandleFunc("GET /api/v1/audit/{tenant}", listAudit(app))
	mux.HandleFunc("GET /api/v1/workers/status", workerStatus(app))
	// Sandbox-only failure-injection control surface (see App.SandboxMode).
	mux.HandleFunc("POST /api/v1/admin/simulators/{dependency}/{tenant}", configureSimulator(app))

	return recoverer(requestSizeLimit(1<<20, mux))
}

func recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if rec := recover(); rec != nil {
				writeJSON(w, http.StatusInternalServerError, map[string]any{"error": "internal error"})
			}
		}()
		next.ServeHTTP(w, r)
	})
}

func requestSizeLimit(max int64, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r.Body = http.MaxBytesReader(w, r.Body, max)
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func listOrders(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.URL.Query().Get("tenant")
		out, err := app.Store.ListOrders(r.Context(), tenant)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		writeJSON(w, http.StatusOK, out)
	}
}

func getOrder(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		o, err := app.Store.GetOrder(r.Context(), id)
		if err != nil {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, o)
	}
}

func approveOrder(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		o, err := app.Orders.Approve(r.Context(), id, actor(r))
		if err != nil {
			writeErr(w, http.StatusConflict, err)
			return
		}
		app.Metrics.OrdersTransitioned.Inc(string(o.Status), "workflow")
		writeJSON(w, http.StatusOK, o)
	}
}

func cancelOrder(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := app.Orders.Cancel(ctx, id, actor(r)); err != nil {
			writeErr(w, http.StatusConflict, err)
			return
		}
		app.Metrics.OrdersTransitioned.Inc("cancelled", "workflow")
		writeJSON(w, http.StatusOK, map[string]string{"status": "cancelled"})
	}
}

func fulfilOrder(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
		defer cancel()
		if err := app.Orders.Fulfil(ctx, id, actor(r)); err != nil {
			writeErr(w, http.StatusConflict, err)
			return
		}
		app.Metrics.OrdersTransitioned.Inc("fulfilling", "workflow")
		writeJSON(w, http.StatusOK, map[string]string{"status": "fulfilling"})
	}
}

type bulkStatusRequest struct {
	OrderIDs []string           `json:"order_ids"`
	Status   domain.OrderStatus `json:"status"`
}

func adminBulkStatus(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req bulkStatusRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		n, err := app.Orders.AdminBulkUpdate(r.Context(), req.OrderIDs, req.Status, actor(r))
		if err != nil {
			writeErr(w, http.StatusInternalServerError, err)
			return
		}
		if n > 0 {
			app.Metrics.OrdersTransitioned.Inc(string(req.Status), "admin-bulk")
		}
		writeJSON(w, http.StatusOK, map[string]int{"updated": n})
	}
}

func getInventory(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, sku := r.PathValue("tenant"), r.PathValue("sku")
		it, err := app.Store.GetInventory(r.Context(), tenant, sku)
		if err != nil {
			writeErr(w, http.StatusNotFound, err)
			return
		}
		writeJSON(w, http.StatusOK, it)
	}
}

func getConfig(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, key := r.PathValue("tenant"), r.PathValue("key")
		c, ok := app.Config.Get(tenant, key)
		if !ok {
			writeErr(w, http.StatusNotFound, domain.ErrNotFound)
			return
		}
		writeJSON(w, http.StatusOK, c)
	}
}

type putConfigRequest struct {
	Value string `json:"value"`
}

func putConfig(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant, key := r.PathValue("tenant"), r.PathValue("key")
		var req putConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		app.Config.Update(domain.ConfigVersion{
			ID: "cfg-" + tenant + "-" + key, TenantID: tenant, Key: key, Value: req.Value,
			Version: time.Now().UnixNano(), Active: true, CreatedAt: time.Now().UTC(),
		})
		writeJSON(w, http.StatusOK, map[string]string{"status": "updated"})
	}
}

func listAudit(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		tenant := r.PathValue("tenant")
		writeJSON(w, http.StatusOK, app.Store.ListAudit(tenant))
	}
}

func workerStatus(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]int{
			"active": app.Pool.Active(), "queued": app.Pool.Queued(),
		})
	}
}

type simulatorConfigRequest struct {
	DelayMS    int64  `json:"delay_ms"`
	Mode       string `json:"mode"`
	FailFirstN int    `json:"fail_first_n"`
}

func configureSimulator(app *application.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if !app.SandboxMode {
			writeErr(w, http.StatusForbidden, domain.ErrDependencyFailed)
			return
		}
		dep := r.PathValue("dependency")
		tenant := r.PathValue("tenant")
		var req simulatorConfigRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeErr(w, http.StatusBadRequest, err)
			return
		}
		sim := app.SimulatorByName(dep)
		if sim == nil {
			writeErr(w, http.StatusNotFound, domain.ErrNotFound)
			return
		}
		sim.Configure(tenant, dependenciesBehaviour(req))
		writeJSON(w, http.StatusOK, map[string]string{"status": "configured"})
	}
}

func actor(r *http.Request) string {
	if v := r.Header.Get("X-Test-Actor"); v != "" {
		return v
	}
	return "unknown"
}

func dependenciesBehaviour(req simulatorConfigRequest) dependencies.Behaviour {
	mode := dependencies.FailureNone
	switch req.Mode {
	case "timeout":
		mode = dependencies.FailureTimeout
	case "retryable":
		mode = dependencies.FailureRetryable
	case "permanent":
		mode = dependencies.FailurePermanent
	}
	return dependencies.Behaviour{
		Delay: time.Duration(req.DelayMS) * time.Millisecond,
		Mode:  mode, FailFirstN: req.FailFirstN,
	}
}
