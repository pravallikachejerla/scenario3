package storage

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// TransitionRaceWindow is a test-only hook. Production code leaves it
// at zero; private concurrency tests enlarge the window between an
// order's status read and its write so that a race that would
// otherwise only appear intermittently under real load becomes
// reliably observable in a short-running test.
var TransitionRaceWindow time.Duration

// LockAcquireGap is a test-only hook (default zero) that widens the
// window between CancelOrder/FulfilOrder acquiring their first lock
// and attempting their second lock, so the STAB-01 lock-ordering
// condition reproduces deterministically under a bounded test
// timeout instead of only intermittently under real load.
var LockAcquireGap time.Duration

// MemoryStore is the default, fully-functional in-memory
// implementation of the platform's repositories. It is used by the
// public test suite, the private evaluator and local development.
type MemoryStore struct {
	mu sync.RWMutex

	tenants   map[string]domain.Tenant
	users     map[string]domain.User
	orders    map[string]*domain.Order
	history   []domain.StatusHistoryEntry
	inventory map[string]*domain.InventoryItem // key: tenant/sku
	reservations map[string]*domain.InventoryReservation
	fulfilments map[string]*domain.FulfilmentRequest
	outbox    []*domain.OutboxEvent
	consumed  map[string]domain.ConsumedEvent // key: idempotency key
	configs   map[string]*domain.ConfigVersion // key: tenant/key
	audits    []domain.AuditEvent
	depCalls  []domain.DependencyCallRecord
	batchJobs map[string]*domain.BatchJob

	locker *KeyedLocker
}

func NewMemoryStore() *MemoryStore {
	return &MemoryStore{
		tenants:      map[string]domain.Tenant{},
		users:        map[string]domain.User{},
		orders:       map[string]*domain.Order{},
		inventory:    map[string]*domain.InventoryItem{},
		reservations: map[string]*domain.InventoryReservation{},
		fulfilments:  map[string]*domain.FulfilmentRequest{},
		consumed:     map[string]domain.ConsumedEvent{},
		configs:      map[string]*domain.ConfigVersion{},
		batchJobs:    map[string]*domain.BatchJob{},
		locker:       NewKeyedLocker(),
	}
}

// --- reference / seed data -------------------------------------------------

func (m *MemoryStore) PutTenant(t domain.Tenant) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.tenants[t.ID] = t
}

func (m *MemoryStore) PutUser(u domain.User) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.users[u.ID] = u
}

func invKey(tenantID, sku string) string { return tenantID + "/" + sku }

func (m *MemoryStore) PutInventory(i domain.InventoryItem) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := i
	m.inventory[invKey(i.TenantID, i.SKU)] = &cp
}

func (m *MemoryStore) GetInventory(ctx context.Context, tenantID, sku string) (domain.InventoryItem, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	it, ok := m.inventory[invKey(tenantID, sku)]
	if !ok {
		return domain.InventoryItem{}, domain.ErrNotFound
	}
	return *it, nil
}

// --- orders ------------------------------------------------------------

func (m *MemoryStore) PutOrder(o domain.Order) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := o
	m.orders[o.ID] = &cp
}

func (m *MemoryStore) GetOrder(ctx context.Context, id string) (domain.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	o, ok := m.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	return *o, nil
}

func (m *MemoryStore) ListOrders(ctx context.Context, tenantID string) ([]domain.Order, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.Order
	for _, o := range m.orders {
		if o.TenantID == tenantID {
			out = append(out, *o)
		}
	}
	return out, nil
}

// TransitionOrderUnsafe applies a status transition.
//
// SOURCE CONDITION (STAB-08): the read of the current order and the
// write of its new status are two separate critical sections rather
// than one atomic read-modify-write, and no version/compare-and-swap
// guard is used. Under concurrent transitions on the same order this
// allows a lost update: two callers can both read the same starting
// status, both decide their transition is valid, and both write,
// with the second write silently discarding the effect of the first
// (for example, a cancellation and a fulfilment can both "succeed"
// against the same order).
func (m *MemoryStore) TransitionOrderUnsafe(ctx context.Context, id string, to domain.OrderStatus, actorID, reason, source string) (domain.Order, error) {
	m.mu.RLock()
	o, ok := m.orders[id]
	if !ok {
		m.mu.RUnlock()
		return domain.Order{}, domain.ErrNotFound
	}
	current := *o
	m.mu.RUnlock()

	if !domain.IsValidTransition(current.Status, to) {
		return domain.Order{}, fmt.Errorf("%w: %s -> %s", domain.ErrInvalidTransition, current.Status, to)
	}

	// Deliberate race window: in the clean design this whole
	// read-decide-write sequence is one atomic operation (a single
	// UPDATE ... WHERE status = $current AND version = $version, or
	// an equivalent CAS in memory). Here it is not, so another
	// transition can interleave right here.
	if TransitionRaceWindow > 0 {
		time.Sleep(TransitionRaceWindow)
	}

	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok = m.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	from := o.Status
	o.Status = to
	o.Version++
	o.UpdatedAt = time.Now().UTC()
	snapshot := *o
	m.history = append(m.history, domain.StatusHistoryEntry{
		ID: fmt.Sprintf("hist-%d", len(m.history)+1), OrderID: id,
		From: from, To: to, ActorID: actorID, Reason: reason, Source: source,
		Timestamp: snapshot.UpdatedAt,
	})
	return snapshot, nil
}

// TransitionOrderSafe is the version-guarded (CAS) transition path.
// It is provided so that a corrected implementation has a natural,
// idiomatic place to live and so the private evaluator can verify
// that a candidate repair actually enforces the guard rather than
// merely adding a lock around the unsafe path.
func (m *MemoryStore) TransitionOrderSafe(ctx context.Context, id string, expectedVersion int64, to domain.OrderStatus, actorID, reason, source string) (domain.Order, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	o, ok := m.orders[id]
	if !ok {
		return domain.Order{}, domain.ErrNotFound
	}
	if o.Version != expectedVersion {
		return domain.Order{}, domain.ErrConcurrentUpdate
	}
	if !domain.IsValidTransition(o.Status, to) {
		return domain.Order{}, fmt.Errorf("%w: %s -> %s", domain.ErrInvalidTransition, o.Status, to)
	}
	from := o.Status
	o.Status = to
	o.Version++
	o.UpdatedAt = time.Now().UTC()
	snapshot := *o
	m.history = append(m.history, domain.StatusHistoryEntry{
		ID: fmt.Sprintf("hist-%d", len(m.history)+1), OrderID: id,
		From: from, To: to, ActorID: actorID, Reason: reason, Source: source,
		Timestamp: snapshot.UpdatedAt,
	})
	return snapshot, nil
}

// AdminBulkSetStatus is the emergency customisation path used by
// support tooling to force an order into a status without going
// through TransitionOrder{Unsafe,Safe}.
//
// SOURCE CONDITION (STAB-12): it writes the status field directly,
// bypassing domain.IsValidTransition, the audit trail and any
// inventory/fulfilment side effects that a normal transition would
// trigger. It is reachable only from the administrative bulk
// operation handler, never from the ordinary order workflow.
func (m *MemoryStore) AdminBulkSetStatus(ctx context.Context, ids []string, to domain.OrderStatus, actorID string) (int, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	n := 0
	for _, id := range ids {
		o, ok := m.orders[id]
		if !ok {
			continue
		}
		o.Status = to
		o.UpdatedAt = time.Now().UTC()
		// No audit event, no history entry, no version bump: the
		// bulk path is invisible to everything that depends on the
		// normal transition contract.
		n++
	}
	return n, nil
}

// --- locking-aware workflow operations (STAB-01) ------------------------

// CancelOrder implements the cancellation workflow. It locks the
// order first, then the inventory row for each item.
func (m *MemoryStore) CancelOrder(ctx context.Context, id string) error {
	relOrder, err := m.locker.Acquire(ctx, "order:"+id)
	if err != nil {
		return fmt.Errorf("cancel: acquire order lock: %w", err)
	}
	defer relOrder()
	if LockAcquireGap > 0 {
		time.Sleep(LockAcquireGap)
	}

	o, err := m.GetOrder(ctx, id)
	if err != nil {
		return err
	}
	for _, item := range o.Items {
		relInv, err := m.locker.Acquire(ctx, "inv:"+invKey(o.TenantID, item.SKU))
		if err != nil {
			return fmt.Errorf("cancel: acquire inventory lock for %s: %w", item.SKU, err)
		}
		m.releaseReservationLocked(o.TenantID, item.SKU, item.Quantity)
		relInv()
	}
	_, err = m.TransitionOrderSafe(ctx, id, o.Version, domain.OrderCancelled, "system", "cancel workflow", "workflow")
	return err
}

// FulfilOrder implements the fulfilment workflow.
//
// SOURCE CONDITION (STAB-01): it locks the inventory row first, then
// the order - the reverse of CancelOrder's order-then-inventory
// sequence. When a cancellation and a fulfilment run concurrently
// against the same order and the same SKU, each can hold the lock the
// other needs next, producing a deadlock-shaped wait (here bounded by
// ctx so the test observes a timeout rather than hanging forever,
// mirroring how PostgreSQL's deadlock detector aborts one
// transaction).
func (m *MemoryStore) FulfilOrder(ctx context.Context, id string) error {
	o, err := m.GetOrder(ctx, id)
	if err != nil {
		return err
	}
	for _, item := range o.Items {
		relInv, err := m.locker.Acquire(ctx, "inv:"+invKey(o.TenantID, item.SKU))
		if err != nil {
			return fmt.Errorf("fulfil: acquire inventory lock for %s: %w", item.SKU, err)
		}
		if LockAcquireGap > 0 {
			time.Sleep(LockAcquireGap)
		}
		relOrder, err := m.locker.Acquire(ctx, "order:"+id)
		if err != nil {
			relInv()
			return fmt.Errorf("fulfil: acquire order lock: %w", err)
		}
		m.commitReservationLocked(o.TenantID, item.SKU, item.Quantity)
		relOrder()
		relInv()
	}
	_, err = m.TransitionOrderSafe(ctx, id, o.Version, domain.OrderFulfilling, "system", "fulfilment workflow", "workflow")
	return err
}

func (m *MemoryStore) releaseReservationLocked(tenantID, sku string, qty int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.inventory[invKey(tenantID, sku)]
	if !ok {
		return
	}
	it.Reserved -= qty
	if it.Reserved < 0 {
		it.Reserved = 0
	}
	it.UpdatedAt = time.Now().UTC()
}

func (m *MemoryStore) commitReservationLocked(tenantID, sku string, qty int) {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.inventory[invKey(tenantID, sku)]
	if !ok {
		return
	}
	it.OnHand -= qty
	it.Reserved -= qty
	if it.Reserved < 0 {
		it.Reserved = 0
	}
	it.UpdatedAt = time.Now().UTC()
}

func (m *MemoryStore) ReserveStock(ctx context.Context, tenantID, sku string, qty int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	it, ok := m.inventory[invKey(tenantID, sku)]
	if !ok {
		return domain.ErrNotFound
	}
	if it.OnHand-it.Reserved < qty {
		return domain.ErrInsufficientStock
	}
	it.Reserved += qty
	it.UpdatedAt = time.Now().UTC()
	return nil
}

// --- outbox / events -----------------------------------------------------

func (m *MemoryStore) AppendOutbox(ev domain.OutboxEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := ev
	m.outbox = append(m.outbox, &cp)
}

func (m *MemoryStore) UndispatchedOutbox() []domain.OutboxEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.OutboxEvent
	for _, e := range m.outbox {
		if e.DispatchedAt == nil {
			out = append(out, *e)
		}
	}
	return out
}

func (m *MemoryStore) MarkDispatched(id string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	now := time.Now().UTC()
	for _, e := range m.outbox {
		if e.ID == id {
			e.DispatchedAt = &now
		}
	}
}

func (m *MemoryStore) HasConsumed(idempotencyKey string) bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	_, ok := m.consumed[idempotencyKey]
	return ok
}

func (m *MemoryStore) MarkConsumed(rec domain.ConsumedEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.consumed[rec.IdempotencyKey] = rec
}

// --- configuration ---------------------------------------------------------

func cfgKey(tenantID, key string) string { return tenantID + "/" + key }

func (m *MemoryStore) PutConfig(c domain.ConfigVersion) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := c
	m.configs[cfgKey(c.TenantID, c.Key)] = &cp
}

func (m *MemoryStore) GetConfigDirect(tenantID, key string) (domain.ConfigVersion, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	c, ok := m.configs[cfgKey(tenantID, key)]
	if !ok {
		return domain.ConfigVersion{}, false
	}
	return *c, true
}

// --- audit -----------------------------------------------------------------

func (m *MemoryStore) AppendAudit(a domain.AuditEvent) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.audits = append(m.audits, a)
}

func (m *MemoryStore) ListAudit(tenantID string) []domain.AuditEvent {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.AuditEvent
	for _, a := range m.audits {
		if a.TenantID == tenantID {
			out = append(out, a)
		}
	}
	return out
}

// --- dependency call records -------------------------------------------

func (m *MemoryStore) RecordDependencyCall(r domain.DependencyCallRecord) {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.depCalls = append(m.depCalls, r)
}

func (m *MemoryStore) DependencyCallsForOrder(orderID string) []domain.DependencyCallRecord {
	m.mu.RLock()
	defer m.mu.RUnlock()
	var out []domain.DependencyCallRecord
	for _, r := range m.depCalls {
		if r.OrderID == orderID {
			out = append(out, r)
		}
	}
	return out
}

// --- batch jobs --------------------------------------------------------

func (m *MemoryStore) PutBatchJob(b domain.BatchJob) {
	m.mu.Lock()
	defer m.mu.Unlock()
	cp := b
	m.batchJobs[b.ID] = &cp
}

func (m *MemoryStore) GetBatchJob(id string) (domain.BatchJob, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	b, ok := m.batchJobs[id]
	if !ok {
		return domain.BatchJob{}, false
	}
	return *b, true
}
