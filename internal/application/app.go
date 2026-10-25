// Package application wires the platform's storage, domain services,
// dependency simulators and observability into the object the HTTP
// layer and worker process both depend on.
package application

import (
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/configuration"
	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/events"
	"github.com/soma-genesis/scenario-3-order-stability/internal/fulfilment"
	"github.com/soma-genesis/scenario-3-order-stability/internal/observability"
	"github.com/soma-genesis/scenario-3-order-stability/internal/orders"
	"github.com/soma-genesis/scenario-3-order-stability/internal/storage"
	"github.com/soma-genesis/scenario-3-order-stability/internal/workers"
)

type BuildIdentity struct {
	Commit    string
	BuiltAt   string
	DatasetID string
}

type App struct {
	Store      *storage.MemoryStore
	Orders     *orders.Service
	Config     *configuration.Service
	Deps       *dependencies.Registry
	Breaker    *dependencies.Breaker
	Publisher  *events.Publisher
	Fulfilment *fulfilment.Service
	Pool       *workers.Pool
	Metrics    *observability.AppMetrics
	Logger     *observability.Logger
	Identity   BuildIdentity
	StartedAt  time.Time
	// SandboxMode gates the failure-injection control surface
	// (POST /api/v1/admin/simulators/...). It must be false in any
	// production-like deployment; the sandbox deployment sets it true
	// so the private evaluator and Test Engineer dry run can script
	// dependency behaviour deterministically.
	SandboxMode bool
}

// SimulatorByName resolves a dependency simulator by its API path
// segment, or nil if unknown.
func (a *App) SimulatorByName(name string) *dependencies.Simulator {
	switch name {
	case "payment":
		return a.Deps.Payment
	case "shipping":
		return a.Deps.Shipping
	case "inventory-provider":
		return a.Deps.InventoryProvider
	case "queue":
		return a.Deps.Queue
	default:
		return nil
	}
}

func New(store *storage.MemoryStore, m *observability.AppMetrics, identity BuildIdentity) *App {
	deps := dependencies.NewRegistry()
	breaker := dependencies.NewBreaker(3)
	client := dependencies.NewClient(deps.Shipping, breaker, 3, store)
	fulfilSvc := fulfilment.New(client, 2, func(tenantID string) string {
		// WAVE-1 REPAIR (F-10 / C-07 / STAB-11): the breaker scope is
		// now tenant-scoped rather than global, so a failure burst
		// from one tenant cannot trip the breaker for every other
		// tenant that calls Dispatch. The corrected primitive
		// TenantScope already exists in internal/dependencies/breaker.go.
		return dependencies.TenantScope(tenantID, deps.Shipping.Name())
	})
	pool := workers.NewPool(4)
	pool.Start()

	return &App{
		Store:       store,
		Orders:      orders.New(store),
		Config:      configuration.NewService(store),
		Deps:        deps,
		Breaker:     breaker,
		Publisher:   events.NewPublisher(store),
		Fulfilment:  fulfilSvc,
		Pool:        pool,
		Metrics:     m,
		Logger:      observability.NewLogger(),
		Identity:    identity,
		StartedAt:   time.Now().UTC(),
		SandboxMode: true,
	}
}
