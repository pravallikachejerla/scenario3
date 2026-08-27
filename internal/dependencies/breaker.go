package dependencies

import (
	"sync"

	"github.com/soma-genesis/scenario-3-order-stability/internal/domain"
)

// Breaker is a minimal failure-count circuit breaker. It trips
// "open" (blocking calls) once consecutive failures for a scope key
// reach Threshold, and resets after a successful call recorded via
// RecordSuccess for that same scope key.
type Breaker struct {
	Threshold int

	mu       sync.Mutex
	failures map[string]int
	open     map[string]bool
}

func NewBreaker(threshold int) *Breaker {
	return &Breaker{Threshold: threshold, failures: map[string]int{}, open: map[string]bool{}}
}

// GlobalScope is the scope key used by the seeded (buggy) call sites:
// it ignores tenant identity entirely.
func GlobalScope(dep domain.DependencyName) string { return string(dep) }

// TenantScope is the corrected scope key: a repair is expected to
// switch call sites from GlobalScope to TenantScope so that breaker
// state cannot leak across tenants.
func TenantScope(tenantID string, dep domain.DependencyName) string { return tenantID + "/" + string(dep) }

func (b *Breaker) Allow(scope string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return !b.open[scope]
}

func (b *Breaker) RecordFailure(scope string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures[scope]++
	if b.failures[scope] >= b.Threshold {
		b.open[scope] = true
	}
}

func (b *Breaker) RecordSuccess(scope string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.failures[scope] = 0
	b.open[scope] = false
}

func (b *Breaker) IsOpen(scope string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.open[scope]
}
