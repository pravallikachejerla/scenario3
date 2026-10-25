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
	Store       *storage.MemoryStore
	Orders      *orders.Service
	Config      *configuration.Service
	Deps        *dependencies.Registry
	Breaker     *dependencies.Breaker
	Publisher   *events.Publisher
	// IdempotentConsumer is the first-wave (F-02) durable
	// idempotency primitive. It is wired only when the binary is
	// built with `-tags firstwave`; the baseline build leaves it
	// nil and uses the original Consumer path.
	IdempotentConsumer *events.IdempotentConsumer
	Fulfilment  *fulfilment.Service
	Pool        *workers.Pool
	Metrics     *observability.AppMetrics
	Logger      *observability.Logger
	Identity    BuildIdentity
	StartedAt   time.Time
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
		// SOURCE CONDITION (STAB-11 call site): uses the global scope,
		// not the tenant-scoped one, so the breaker above is shared
		// across every tenant that calls Dispatch.
		return dependencies.GlobalScope(deps.Shipping.Name())
	})
	pool := workers.NewPool(4)
	pool.Start()

	return &App{
		Store:      store,
		Orders:     orders.New(store),
		Config:     configuration.NewService(store),
		Deps:       deps,
		Breaker:    breaker,
		Publisher:  events.NewPublisher(store),
		Fulfilment: fulfilSvc,
		Pool:       pool,
		Metrics:    m,
		Logger:     observability.NewLogger(),
		Identity:    identity,
		StartedAt:   time.Now().UTC(),
		SandboxMode: true,
	}
}
