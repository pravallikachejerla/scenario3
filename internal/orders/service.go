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

// transitionWithCAS is the WAVE-1 REPAIR (F-08) helper for STAB-08 /
// INC-2026-0728-1: it reads the current order, then performs a
// version-guarded transition (CAS) so that two concurrent
// transitions on the same order cannot both succeed. If the CAS
// fails because the version moved, it retries up to maxAttempts
// times before giving up with ErrConcurrentUpdate.
func (s *Service) transitionWithCAS(ctx context.Context, orderID string, to domain.OrderStatus, actorID, reason, source string) (domain.Order, error) {
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		current, err := s.store.GetOrder(ctx, orderID)
		if err != nil {
			return domain.Order{}, err
		}
		o, err := s.store.TransitionOrderSafe(ctx, orderID, current.Version, to, actorID, reason, source)
		if err == nil {
			return o, nil
		}
		// Retry only on version conflict; surface other errors
		// (invalid transition, not found) immediately.
		if err != domain.ErrConcurrentUpdate {
			return domain.Order{}, err
		}
	}
	return domain.Order{}, domain.ErrConcurrentUpdate
}

func (s *Service) Approve(ctx context.Context, orderID, actorID string) (domain.Order, error) {
	o, err := s.transitionWithCAS(ctx, orderID, domain.OrderApproved, actorID, "approved", "workflow")
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
