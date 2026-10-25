package events

import (
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// InboundEvent is a message delivered by the queue dependency
// simulator. EventID is the broker-assigned message ID; a redelivery
// (at-least-once semantics) carries the same EventID again.
type InboundEvent struct {
	EventID string
	OrderID string
	Type    string
	Payload []byte
}

// ConsumerStore is the subset of persistence the consumer needs.
type ConsumerStore interface {
	HasConsumed(idempotencyKey string) bool
	MarkConsumed(rec domain.ConsumedEvent)
}

// EffectFn applies the business effect of one event (for example,
// decrementing on-hand inventory once).
type EffectFn func(ev InboundEvent) error

// Consumer applies inbound events to business state.
//
// SOURCE CONDITION (STAB-02): Process applies the business effect
// unconditionally and only records the event as consumed afterwards,
// without ever checking whether that EventID was already consumed.
// A broker redelivery - which is expected under at-least-once
// delivery, for example after a consumer restart between processing
// and acknowledgement - therefore applies the same effect twice (for
// example, the same shipment or inventory decrement is recorded a
// second time).
type Consumer struct {
	store  ConsumerStore
	effect EffectFn
}

func NewConsumer(store ConsumerStore, effect EffectFn) *Consumer {
	return &Consumer{store: store, effect: effect}
}

func (c *Consumer) Process(ev InboundEvent) error {
	if err := c.effect(ev); err != nil {
		return err
	}
	c.store.MarkConsumed(domain.ConsumedEvent{
		EventID:        ev.EventID,
		IdempotencyKey: ev.EventID,
		ConsumedAt:     time.Now().UTC(),
	})
	return nil
}

// ProcessIdempotent is the corrected primitive: it checks
// HasConsumed before applying any effect. A repair is expected to
// route inbound delivery through something with this shape (or an
// equivalent durable idempotency check performed atomically with the
// effect) instead of Process.
func (c *Consumer) ProcessIdempotent(ev InboundEvent) error {
	if c.store.HasConsumed(ev.EventID) {
		return nil
	}
	if err := c.effect(ev); err != nil {
		return err
	}
	c.store.MarkConsumed(domain.ConsumedEvent{
		EventID:        ev.EventID,
		IdempotencyKey: ev.EventID,
		ConsumedAt:     time.Now().UTC(),
	})
	return nil
}
