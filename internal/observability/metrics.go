package observability

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
)

// Metrics is a small, dependency-free counter/gauge registry that
// renders itself in Prometheus text exposition format. It is
// implemented against the standard library only so the platform has
// no required third-party build-time dependency; the exposition
// format is unchanged, so any standard scraping/dashboard tool still
// works against /metrics.
type Metrics struct {
	mu       sync.Mutex
	counters map[string]*counterVec
	gauges   map[string]*gauge
}

type counterEntry struct {
	labelValues []string
	value       int64
}

type counterVec struct {
	mu     sync.Mutex
	help   string
	labels []string
	byKey  map[string]*counterEntry
}

type gauge struct {
	help  string
	value int64
}

func NewMetrics() *Metrics {
	return &Metrics{counters: map[string]*counterVec{}, gauges: map[string]*gauge{}}
}

// AppMetrics is the fixed set of named metrics the platform exposes.
type AppMetrics struct {
	registry *Metrics

	OrdersTransitioned *counterVec
	DependencyCalls    *counterVec
	WorkerQueueDepth   *gauge
	WorkerActive       *gauge
	OutboxUndispatched *gauge
}

func NewAppMetrics() *AppMetrics {
	reg := NewMetrics()
	return &AppMetrics{
		registry:           reg,
		OrdersTransitioned: reg.registerCounter("orders_transitioned_total", "Order status transitions.", "to", "source"),
		DependencyCalls:    reg.registerCounter("dependency_calls_total", "Dependency call attempts.", "dependency", "layer", "outcome"),
		WorkerQueueDepth:   reg.registerGauge("worker_queue_depth", "Jobs waiting in the worker pool queue."),
		WorkerActive:       reg.registerGauge("worker_active", "Workers currently executing a job."),
		OutboxUndispatched: reg.registerGauge("outbox_undispatched", "Outbox events not yet dispatched."),
	}
}

func (a *AppMetrics) RenderTo(w io.Writer) { a.registry.RenderTo(w) }

func (m *Metrics) registerCounter(name, help string, labels ...string) *counterVec {
	m.mu.Lock()
	defer m.mu.Unlock()
	c := &counterVec{help: help, labels: labels, byKey: map[string]*counterEntry{}}
	m.counters[name] = c
	return c
}

func (m *Metrics) registerGauge(name, help string) *gauge {
	m.mu.Lock()
	defer m.mu.Unlock()
	g := &gauge{help: help}
	m.gauges[name] = g
	return g
}

func labelKey(values ...string) string { return strings.Join(values, "\x1f") }

func (c *counterVec) Inc(labelValues ...string) {
	key := labelKey(labelValues...)
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.byKey[key]
	if !ok {
		e = &counterEntry{labelValues: labelValues}
		c.byKey[key] = e
	}
	atomic.AddInt64(&e.value, 1)
}

func (g *gauge) Set(v int64) { atomic.StoreInt64(&g.value, v) }
func (g *gauge) Get() int64  { return atomic.LoadInt64(&g.value) }

// WriteTo renders the registry in Prometheus text exposition format.
func (m *Metrics) RenderTo(w io.Writer) {
	m.mu.Lock()
	defer m.mu.Unlock()

	var names []string
	for n := range m.counters {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		c := m.counters[n]
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s counter\n", n, c.help, n)
		c.mu.Lock()
		var keys []string
		for k := range c.byKey {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			e := c.byKey[k]
			labels := ""
			if len(c.labels) > 0 {
				var parts []string
				for j, lv := range e.labelValues {
					if j < len(c.labels) {
						parts = append(parts, fmt.Sprintf(`%s="%s"`, c.labels[j], lv))
					}
				}
				labels = "{" + strings.Join(parts, ",") + "}"
			}
			fmt.Fprintf(w, "%s%s %d\n", n, labels, atomic.LoadInt64(&e.value))
		}
		c.mu.Unlock()
	}

	var gnames []string
	for n := range m.gauges {
		gnames = append(gnames, n)
	}
	sort.Strings(gnames)
	for _, n := range gnames {
		g := m.gauges[n]
		fmt.Fprintf(w, "# HELP %s %s\n# TYPE %s gauge\n%s %d\n", n, g.help, n, n, g.Get())
	}
}
