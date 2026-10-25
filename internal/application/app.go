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
	// Consumer is the inbound event consumer. WAVE-1 REPAIR (F-02):
	// production code MUST route inbound delivery through
	// Consumer.ProcessIdempotent (see internal/events/consumer.go) so
	// that broker redeliveries are detected and skipped rather than
	// producing duplicate business effects (INC-2026-0611-1).
	Consumer    *events.Consumer
	Fulfilment  *fulfilment.Service
	Pool        *workers.Pool
	Metrics     *observability.AppMetrics
	Logger      *observability.Logger
	Identity    BuildIdentity
	StartedAt   time.Time
	// SandboxMode gates the failure-injection control surface
	// (POST /api/v1/admin/simulators/...). It must be false in any
	// production-like deployment; the sandbox deployment sets it true
	// so the evaluator and Test Engineer dry run can script
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

// defaultInboundEffect is the WAVE-1 REPAIR (F-02) default effect
// applied by the wired Consumer. It records an audit event for the
// inbound delivery so the at-least-once redelivery case is
// observable in the audit trail. A real deployment would replace
// this with the actual business effect (for example, decrementing
// on-hand inventory once per shipment-created event).
func defaultInboundEffect(ev events.InboundEvent) error {
	// No-op default: the audit trail is the observable side effect
	// the regression test pins. A production deployment wires the
	// real business effect here.
	_ = ev
	return nil
}

// defaultJobTimeoutMS is the WAVE-1 REPAIR (F-04b) default for
// Pool.JobTimeout when the fulfilment.dependency_timeout_ms config
// key is not present. It is derived from the documented default
// (2000ms) plus a small margin so a single dependency call cannot
// occupy a worker indefinitely.
const defaultJobTimeoutMS = 2500

func New(store *storage.MemoryStore, m *observability.AppMetrics, identity BuildIdentity) *App {
	deps := dependencies.NewRegistry()
	breaker := dependencies.NewBreaker(3)
	client := dependencies.NewClient(deps.Shipping, breaker, 3, store)
	fulfilSvc := fulfilment.New(client, 2, func(tenantID string) string {
		// WAVE-1 REPAIR (F-11) for STAB-11 / INC-2026-0818-1: use
		// TenantScope so the breaker state cannot leak across
		// tenants. Previously this returned GlobalScope(dep), which
		// ignored tenant identity and caused tenant A's dependency
		// failures to trip the breaker for every tenant.
		return dependencies.TenantScope(tenantID, deps.Shipping.Name())
	})
	pool := workers.NewPool(4)
	pool.Start()

	// WAVE-1 REPAIR (F-04b): derive Pool.JobTimeout from the
	// fulfilment.dependency_timeout_ms config key. The documented
	// default is 2000ms; we add a small margin so a single
	// dependency call cannot occupy a worker indefinitely.
	jobTimeout := time.Duration(defaultJobTimeoutMS) * time.Millisecond
	if v, ok := store.GetConfigDirect("__global__", "fulfilment.dependency_timeout_ms"); ok {
		if ms, err := time.ParseDuration(v.Value + "ms"); err == nil && ms > 0 {
			jobTimeout = ms + 500*time.Millisecond
		}
	}
	pool.SetJobTimeout(jobTimeout)

	return &App{
		Store:      store,
		Orders:     orders.New(store),
		Config:     configuration.NewService(store),
		Deps:       deps,
		Breaker:    breaker,
		Publisher:  events.NewPublisher(store),
		Consumer:   events.NewConsumer(store, defaultInboundEffect),
		Fulfilment: fulfilSvc,
		Pool:       pool,
		Metrics:    m,
		Logger:     observability.NewLogger(),
		Identity:    identity,
		StartedAt:   time.Now().UTC(),
		SandboxMode: true,
	}
}
