// Package domain contains the core entities for the enterprise Order
// Management and Fulfilment platform. Types are shared by storage,
// application and HTTP layers.
package domain

import "time"

type Tenant struct {
	ID   string
	Name string
}

type Role string

const (
	RoleAdmin       Role = "admin"
	RoleOpsManager  Role = "ops_manager"
	RoleFulfilment  Role = "fulfilment_agent"
	RoleViewer      Role = "viewer"
)

type User struct {
	ID       string
	TenantID string
	Name     string
	Role     Role
}

type OrderStatus string

const (
	OrderPending    OrderStatus = "pending"
	OrderApproved   OrderStatus = "approved"
	OrderReserved   OrderStatus = "reserved"
	OrderFulfilling OrderStatus = "fulfilling"
	OrderFulfilled  OrderStatus = "fulfilled"
	OrderCancelled  OrderStatus = "cancelled"
	OrderFailed     OrderStatus = "failed"
)

// validTransitions documents the authorised order state machine. It is
// used by the transition service (internal/orders) so that every
// status change - including administrative ones - can be checked
// against the same table.
var validTransitions = map[OrderStatus][]OrderStatus{
	OrderPending:    {OrderApproved, OrderCancelled},
	OrderApproved:   {OrderReserved, OrderCancelled},
	OrderReserved:   {OrderFulfilling, OrderCancelled},
	OrderFulfilling: {OrderFulfilled, OrderFailed, OrderCancelled},
	OrderFulfilled:  {},
	OrderCancelled:  {},
	OrderFailed:     {OrderPending},
}

func IsValidTransition(from, to OrderStatus) bool {
	for _, allowed := range validTransitions[from] {
		if allowed == to {
			return true
		}
	}
	return false
}

type Order struct {
	ID         string
	TenantID   string
	CustomerID string
	Status     OrderStatus
	Items      []OrderItem
	// Version supports optimistic concurrency control for state
	// transitions. Every accepted transition must increment it.
	Version   int64
	CreatedAt time.Time
	UpdatedAt time.Time
}

type OrderItem struct {
	SKU      string
	Quantity int
	UnitJPY  int64
}

type StatusHistoryEntry struct {
	ID        string
	OrderID   string
	From      OrderStatus
	To        OrderStatus
	ActorID   string
	Reason    string
	Source    string // "workflow" or "admin-bulk"
	Timestamp time.Time
}

type InventoryItem struct {
	SKU       string
	TenantID  string
	OnHand    int
	Reserved  int
	UpdatedAt time.Time
}

type InventoryReservation struct {
	ID        string
	OrderID   string
	SKU       string
	Quantity  int
	Status    string // "held", "released", "committed"
	CreatedAt time.Time
}

type FulfilmentRequest struct {
	ID        string
	OrderID   string
	Status    string // "queued", "in_progress", "shipped", "failed"
	CreatedAt time.Time
	UpdatedAt time.Time
}

// OutboxEvent is a durable record of a domain event awaiting
// publication. The clean design commits the outbox row in the same
// database transaction as the business-state change, then a worker
// publishes it and marks it dispatched.
type OutboxEvent struct {
	ID          string
	TenantID    string
	AggregateID string
	Type        string
	Payload     []byte
	CreatedAt   time.Time
	DispatchedAt *time.Time
}

// ConsumedEvent records an inbound event that has already produced
// its business effect, keyed by the event's own idempotency key so a
// redelivered copy can be detected and skipped.
type ConsumedEvent struct {
	EventID     string
	IdempotencyKey string
	ConsumedAt  time.Time
}

type ConfigVersion struct {
	ID        string
	TenantID  string
	Key       string
	Value     string
	Version   int64
	Active    bool
	CreatedAt time.Time
}

type DependencyName string

const (
	DependencyPayment  DependencyName = "payment"
	DependencyShipping DependencyName = "shipping"
	DependencyInventory DependencyName = "inventory-provider"
	DependencyQueue    DependencyName = "queue"
)

type DependencyCallRecord struct {
	ID         string
	TenantID   string
	Dependency DependencyName
	OrderID    string
	Attempt    int
	Layer      string // which layer issued the retry: "http-client", "service", "worker"
	Outcome    string // "ok", "timeout", "error"
	DurationMS int64
	Timestamp  time.Time
}

type AuditEvent struct {
	ID        string
	TenantID  string
	ActorID   string
	Action    string
	EntityID  string
	Details   string
	Timestamp time.Time
}

type BatchJob struct {
	ID        string
	TenantID  string
	Kind      string
	Status    string
	ItemCount int
	CreatedAt time.Time
	UpdatedAt time.Time
}

type IdempotencyRecord struct {
	Key       string
	RequestHash string
	ResponseSnapshot string
	CreatedAt time.Time
}
