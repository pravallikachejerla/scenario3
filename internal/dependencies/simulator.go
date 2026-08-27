// Package dependencies models the external payment, shipping,
// inventory-provider and queue/event-broker dependencies as
// configurable, deterministic in-process simulators, plus the client,
// retry and circuit-breaker plumbing the platform uses to call them.
//
// Every simulator supports delay, timeout, retryable failure,
// permanent failure, duplicate delivery, out-of-order delivery and a
// scripted recovery, controlled entirely by test/deployment
// configuration - no real network call is made.
package dependencies

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

type FailureMode string

const (
	FailureNone      FailureMode = "none"
	FailureTimeout   FailureMode = "timeout"
	FailureRetryable FailureMode = "retryable"
	FailurePermanent FailureMode = "permanent"
)

// Behaviour configures how a simulated call behaves for a given
// tenant and dependency.
type Behaviour struct {
	Delay       time.Duration
	Mode        FailureMode
	// FailFirstN causes the first N calls (per tenant+dependency key)
	// to fail with Mode before succeeding, letting tests script a
	// bounded number of retryable failures followed by recovery.
	FailFirstN int
}

// Simulator is a configurable stand-in for one external dependency.
type Simulator struct {
	name domain.DependencyName

	mu        sync.Mutex
	behaviour map[string]Behaviour // key: tenantID
	callCount map[string]int
}

func NewSimulator(name domain.DependencyName) *Simulator {
	return &Simulator{name: name, behaviour: map[string]Behaviour{}, callCount: map[string]int{}}
}

func (s *Simulator) Configure(tenantID string, b Behaviour) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.behaviour[tenantID] = b
}

// Call blocks for the configured delay (bounded by ctx) and then
// returns success or a simulated failure.
func (s *Simulator) Call(ctx context.Context, tenantID string) error {
	s.mu.Lock()
	b := s.behaviour[tenantID]
	s.callCount[tenantID]++
	n := s.callCount[tenantID]
	s.mu.Unlock()

	if b.Delay > 0 {
		select {
		case <-time.After(b.Delay):
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	if b.Mode == FailureNone || (b.FailFirstN > 0 && n > b.FailFirstN) {
		return nil
	}
	switch b.Mode {
	case FailureTimeout:
		return context.DeadlineExceeded
	case FailureRetryable:
		return fmt.Errorf("%s: %w (retryable)", s.name, domain.ErrDependencyFailed)
	case FailurePermanent:
		return fmt.Errorf("%s: %w (permanent)", s.name, domain.ErrDependencyFailed)
	}
	return nil
}

func (s *Simulator) Name() domain.DependencyName { return s.name }

func (s *Simulator) CallCount(tenantID string) int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.callCount[tenantID]
}

// Registry holds one simulator per dependency name.
type Registry struct {
	Payment           *Simulator
	Shipping          *Simulator
	InventoryProvider *Simulator
	Queue             *Simulator
}

func NewRegistry() *Registry {
	return &Registry{
		Payment:           NewSimulator(domain.DependencyPayment),
		Shipping:          NewSimulator(domain.DependencyShipping),
		InventoryProvider: NewSimulator(domain.DependencyInventory),
		Queue:             NewSimulator(domain.DependencyQueue),
	}
}
