// Package workers implements the bounded worker pool that executes
// dependency-calling jobs (fulfilment dispatch, outbox delivery,
// batch steps) off the request path.
package workers

import (
	"context"
	"sync"
	"sync/atomic"
	"time"
)

// Job is one unit of work submitted to the pool.
type Job struct {
	ID  string
	Run func(ctx context.Context) error
	// Ctx is the caller's own context (for example, an HTTP request
	// context). It is retained on the Job so the corrected pool
	// implementation has something to derive a child context from.
	Ctx context.Context
}

// Pool is a fixed-size worker pool with an unbounded input queue.
//
// SOURCE CONDITION (STAB-04): workers call job.Run with whatever
// context the job carries, without imposing a bounded per-job
// timeout of its own. When the downstream dependency is slow (see
// internal/dependencies), a worker stays occupied for the full
// duration of that call. With QueueDepth() unbounded and PoolSize
// workers, N concurrent slow jobs beyond PoolSize simply queue up
// with no shedding or back-pressure, so recovery time after a slow
// dependency episode grows with queue depth rather than staying
// bounded.
//
// SOURCE CONDITION (STAB-10): Worker.run invokes job.Run with
// context.Background() rather than a context derived from job.Ctx.
// Cancelling the caller's context (client disconnect, request
// deadline, controlled shutdown) therefore has no effect on work
// already picked up by a worker - it keeps running to completion
// (or to the dependency's own delay) even though nothing is waiting
// for the result any more.
type Pool struct {
	PoolSize   int
	JobTimeout time.Duration

	queue   chan Job
	wg      sync.WaitGroup
	started int32

	active int32 // currently-occupied workers, for evidence
	queued int32 // jobs waiting to be picked up
}

func NewPool(size int) *Pool {
	return &Pool{PoolSize: size, JobTimeout: 30 * time.Second, queue: make(chan Job, 100000)}
}

func (p *Pool) SetJobTimeout(d time.Duration) {
	p.JobTimeout = d
}

func (p *Pool) Start() {
	if !atomic.CompareAndSwapInt32(&p.started, 0, 1) {
		return
	}
	for i := 0; i < p.PoolSize; i++ {
		p.wg.Add(1)
		go p.run()
	}
}

func (p *Pool) run() {
	defer p.wg.Done()
	for job := range p.queue {
		atomic.AddInt32(&p.queued, -1)
		atomic.AddInt32(&p.active, 1)
		ctx := job.Ctx
		if ctx == nil {
			ctx = context.Background()
		}
		if p.JobTimeout > 0 {
			var cancel context.CancelFunc
			ctx, cancel = context.WithTimeout(ctx, p.JobTimeout)
			defer cancel()
		}
		_ = job.Run(ctx)
		atomic.AddInt32(&p.active, -1)
	}
}

// RunWithRetry is the worker-layer retry wrapper (layer 3 of the
// STAB-06 condition; see internal/dependencies.Client for layer 1 and
// internal/fulfilment.Service for layer 2). It retries a failing run
// function up to retryMax times with no awareness that the function
// it calls may already retry internally.
func RunWithRetry(run func() error, retryMax int) error {
	var lastErr error
	for i := 0; i < retryMax; i++ {
		if err := run(); err == nil {
			return nil
		} else {
			lastErr = err
		}
	}
	return lastErr
}

// RunWithPropagation is the corrected job execution primitive: it
// derives its working context from job.Ctx so caller cancellation
// reaches job.Run. A repair is expected to make Worker.run (or an
// equivalent execution path) use this shape instead of the
// background-context call above.
func RunWithPropagation(job Job) error {
	ctx := job.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	return job.Run(ctx)
}

func (p *Pool) Submit(j Job) {
	atomic.AddInt32(&p.queued, 1)
	p.queue <- j
}

func (p *Pool) Active() int { return int(atomic.LoadInt32(&p.active)) }
func (p *Pool) Queued() int { return int(atomic.LoadInt32(&p.queued)) }

func (p *Pool) Close() {
	close(p.queue)
	p.wg.Wait()
}

// WaitUntilDrainedOrTimeout is a test/evidence helper.
func (p *Pool) WaitUntilDrainedOrTimeout(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if p.Queued() == 0 && p.Active() == 0 {
			return true
		}
		time.Sleep(2 * time.Millisecond)
	}
	return false
}
