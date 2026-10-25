package public_test

import (
	"context"
	"sync"
	"time"

	"github.com/soma-genesis/scenario-3-order-stability/internal/dependencies"
	"github.com/soma-genesis/scenario-3-order-stability/internal/workers"
)

// JobResult is the per-job timing/error record reported by testJob.
type JobResult struct {
	Started  time.Time
	Finished time.Time
	Err      error
}

// newBreakerForTest returns a fresh Breaker with the given threshold.
// It is a thin wrapper so the wave-1 tests do not need to import the
// internal/dependencies package directly in their hot path.
func newBreakerForTest(threshold int) *dependencies.Breaker {
	return dependencies.NewBreaker(threshold)
}

// newPoolForTest returns a fresh Pool with the given size. The pool
// is not started; the caller must call Start.
func newPoolForTest(size int) *workers.Pool {
	return workers.NewPool(size)
}

// testJob builds a workers.Job that runs for the given duration and
// reports its start/finish times and final error on the results
// channel. It is used by the W1-5 tests to exercise the
// config-derived JobTimeout contract.
func testJob(id string, runFor time.Duration, results chan<- JobResult) workers.Job {
	return workers.Job{
		ID: id,
		Run: func(ctx context.Context) error {
			started := time.Now()
			select {
			case <-time.After(runFor):
				results <- JobResult{Started: started, Finished: time.Now(), Err: nil}
				return nil
			case <-ctx.Done():
				results <- JobResult{Started: started, Finished: time.Now(), Err: ctx.Err()}
				return ctx.Err()
			}
		},
	}
}

// drainResults reads n results from the channel with a timeout. It
// is used by tests that submit multiple jobs and need to assert on
// each one without blocking forever if a job misbehaves.
func drainResults(n int, ch <-chan JobResult, timeout time.Duration) ([]JobResult, bool) {
	out := make([]JobResult, 0, n)
	deadline := time.After(timeout)
	for i := 0; i < n; i++ {
		select {
		case r := <-ch:
			out = append(out, r)
		case <-deadline:
			return out, false
		}
	}
	return out, true
}

// once is a tiny sync helper used by tests that need to coordinate a
// single signal between goroutines.
type once struct {
	mu sync.Mutex
	ch chan struct{}
}

func newOnce() *once { return &once{ch: make(chan struct{})} }

func (o *once) signal() {
	o.mu.Lock()
	defer o.mu.Unlock()
	if o.ch != nil {
		close(o.ch)
		o.ch = nil
	}
}

func (o *once) wait() <-chan struct{} {
	o.mu.Lock()
	defer o.mu.Unlock()
	return o.ch
}
