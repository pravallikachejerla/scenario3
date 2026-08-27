// Package storage implements the persistence layer for the Order
// Management and Fulfilment platform: an in-memory store used by
// public/private tests and local development, and a PostgreSQL store
// used by the sandbox deployment.
package storage

import (
	"context"
	"sync"
)

// KeyedLocker hands out per-key mutual exclusion with context-bounded
// waiting, so a caller that cannot obtain a lock within its deadline
// gets an error back instead of hanging forever - similar to a
// database session that hits lock_timeout. This lets concurrency
// conditions (including deadlock-shaped ones) be reproduced inside a
// normal `go test` run without ever hanging the suite.
type KeyedLocker struct {
	mu    sync.Mutex
	locks map[string]chan struct{}
}

func NewKeyedLocker() *KeyedLocker {
	return &KeyedLocker{locks: make(map[string]chan struct{})}
}

func (l *KeyedLocker) chanFor(key string) chan struct{} {
	l.mu.Lock()
	defer l.mu.Unlock()
	ch, ok := l.locks[key]
	if !ok {
		ch = make(chan struct{}, 1)
		l.locks[key] = ch
	}
	return ch
}

// Acquire blocks until the named lock is free or ctx is done. The
// returned release function must be called exactly once.
func (l *KeyedLocker) Acquire(ctx context.Context, key string) (func(), error) {
	ch := l.chanFor(key)
	select {
	case ch <- struct{}{}:
		return func() { <-ch }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}
