// Package events implements the transactional-outbox publication path
// and the inbound consumer that applies business effects for events
// delivered by the message/event dependency simulator.
package events

import (
	"fmt"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

type Store interface {
	AppendOutbox(ev domain.OutboxEvent)
}

// Publisher records a business-relevant fact and the outbox event
// that must eventually be delivered for it.
type Publisher struct {
	store Store
}

func NewPublisher(store Store) *Publisher {
	return &Publisher{store: store}
}

// InjectedFailure lets private tests simulate a crash at the exact
// boundary between committing business state and staging its outbox
// event, without needing to kill the process.
type InjectedFailure func(stage string) error

// RecordStateChangeThenPublish applies a business-state mutation and
// then stages its outbox event as two independent steps.
//
// SOURCE CONDITION (STAB-09): the state mutation (applyState) and the
// outbox append are not part of one atomic unit - there is no
// transactional outbox / durable-recovery guarantee tying them
// together. If the process fails between the two steps (modelled here
// by fail returning a non-nil error for stage "after-state-before-outbox"),
// the business state has changed but no event was ever staged for
// delivery, and nothing will later notice or replay it: the change is
// silently invisible to every downstream consumer (inventory,
// shipping notification, audit).
func RecordStateChangeThenPublish(store Store, tenantID, aggregateID, eventType string, payload []byte, applyState func() error, fail InjectedFailure) error {
	if err := applyState(); err != nil {
		return fmt.Errorf("apply state: %w", err)
	}
	if fail != nil {
		if err := fail("after-state-before-outbox"); err != nil {
			// Business state has already changed; the outbox event
			// below is never written. This is the inconsistency.
			return fmt.Errorf("failure after state commit, before outbox publish: %w", err)
		}
	}
	store.AppendOutbox(domain.OutboxEvent{
		ID:          fmt.Sprintf("evt-%s-%d", aggregateID, time.Now().UnixNano()),
		TenantID:    tenantID,
		AggregateID: aggregateID,
		Type:        eventType,
		Payload:     payload,
		CreatedAt:   time.Now().UTC(),
	})
	return nil
}

// RecordStateChangeAtomic is the corrected primitive: state and
// outbox event are prepared together and only made visible as a unit.
// A repair is expected to route business mutations through something
// with this shape (or an equivalent single-transaction write) instead
// of RecordStateChangeThenPublish.
func RecordStateChangeAtomic(store Store, tenantID, aggregateID, eventType string, payload []byte, applyState func() error) error {
	// In the PostgreSQL storage implementation this wraps applyState
	// and the outbox INSERT in one database transaction. The
	// in-memory model keeps the same call shape so a candidate repair
	// can be exercised identically in the private evaluator.
	ev := domain.OutboxEvent{
		ID: fmt.Sprintf("evt-%s-%d", aggregateID, time.Now().UnixNano()), TenantID: tenantID,
		AggregateID: aggregateID, Type: eventType, Payload: payload, CreatedAt: time.Now().UTC(),
	}
	if err := applyState(); err != nil {
		return err
	}
	store.AppendOutbox(ev)
	return nil
}
