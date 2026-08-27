// Package orders is the application-layer entry point for order
// workflow transitions and the administrative bulk-update path.
package orders

import (
	"context"
	"fmt"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

type Store interface {
	GetOrder(ctx context.Context, id string) (domain.Order, error)
	TransitionOrderUnsafe(ctx context.Context, id string, to domain.OrderStatus, actorID, reason, source string) (domain.Order, error)
	TransitionOrderSafe(ctx context.Context, id string, expectedVersion int64, to domain.OrderStatus, actorID, reason, source string) (domain.Order, error)
	AdminBulkSetStatus(ctx context.Context, ids []string, to domain.OrderStatus, actorID string) (int, error)
	AppendAudit(a domain.AuditEvent)
	CancelOrder(ctx context.Context, id string) error
	FulfilOrder(ctx context.Context, id string) error
}

type Service struct {
	store Store
}

func New(store Store) *Service {
	return &Service{store: store}
}

func (s *Service) Approve(ctx context.Context, orderID, actorID string) (domain.Order, error) {
	o, err := s.store.TransitionOrderUnsafe(ctx, orderID, domain.OrderApproved, actorID, "approved", "workflow")
	if err != nil {
		return domain.Order{}, err
	}
	s.store.AppendAudit(domain.AuditEvent{
		ID: "audit-" + fmt.Sprint(time.Now().UnixNano()), TenantID: o.TenantID, ActorID: actorID,
		Action: "order.approve", EntityID: orderID, Timestamp: time.Now().UTC(),
	})
	return o, nil
}

func (s *Service) Cancel(ctx context.Context, orderID, actorID string) error {
	if err := s.store.CancelOrder(ctx, orderID); err != nil {
		return err
	}
	o, _ := s.store.GetOrder(ctx, orderID)
	s.store.AppendAudit(domain.AuditEvent{
		ID: "audit-" + fmt.Sprint(time.Now().UnixNano()), TenantID: o.TenantID, ActorID: actorID,
		Action: "order.cancel", EntityID: orderID, Timestamp: time.Now().UTC(),
	})
	return nil
}

func (s *Service) Fulfil(ctx context.Context, orderID, actorID string) error {
	if err := s.store.FulfilOrder(ctx, orderID); err != nil {
		return err
	}
	o, _ := s.store.GetOrder(ctx, orderID)
	s.store.AppendAudit(domain.AuditEvent{
		ID: "audit-" + fmt.Sprint(time.Now().UnixNano()), TenantID: o.TenantID, ActorID: actorID,
		Action: "order.fulfil", EntityID: orderID, Timestamp: time.Now().UTC(),
	})
	return nil
}

// AdminBulkUpdate is the emergency customisation entry point (see
// domain.AuditEvent and storage.AdminBulkSetStatus for the seeded
// STAB-12 condition it exposes).
func (s *Service) AdminBulkUpdate(ctx context.Context, ids []string, to domain.OrderStatus, actorID string) (int, error) {
	return s.store.AdminBulkSetStatus(ctx, ids, to, actorID)
}
