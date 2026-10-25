# Architecture — Scenario 3 Order Management and Fulfilment Platform

## Purpose

This repository is the model-visible fixture for **SOMA Genesis validation
Scenario 3 — Application Stabilisation and Reliability Improvement**. It is a
working order-management and fulfilment platform that behaves correctly
under ordinary conditions and exhibits twelve controlled instability
conditions under specific, documented triggers.

## Layers

```
frontend/        React + TypeScript UI (order entry/search, fulfilment
                  timeline, inventory view, batch dashboard, incident
                  panel, configuration view, queue/retry metrics)
internal/httpapi  Versioned REST API (Go standard library net/http,
                  method-and-pattern-aware ServeMux — no third-party
                  router dependency)
internal/orders,
internal/inventory,
internal/fulfilment,
internal/batch    Application/domain services
internal/events    Outbox publication + inbound consumer
internal/workers   Bounded worker pool
internal/dependencies
                  Configurable payment / shipping / inventory-provider /
                  queue dependency simulators, HTTP-client-layer retry,
                  and a circuit breaker
internal/configuration
                  Versioned configuration store + read-through cache +
                  per-environment profile loader
internal/storage  In-memory store (used by all tests and local dev) and
                  a PostgreSQL-schema-compatible store for the sandbox
                  deployment (migrations/0001_init.sql)
internal/observability
                  Structured JSON logging + a dependency-free
                  Prometheus-text-format metrics registry
internal/security  Log redaction
internal/validation Bounded, strict request validation
```

## Storage backends

`internal/storage.MemoryStore` is the default, fully-functional
implementation and is what the public test suite, the private evaluator
and `cmd/api` all exercise directly — every STAB-01..12 condition and its
private test is written and verified against it (see
`private/condition_test.go`). It intentionally uses real
`sync`-based locking and timing so that concurrency-shaped conditions
(STAB-01, STAB-08) are genuinely reproducible under `go test -race`
rather than only described in comments.

`migrations/0001_init.sql` defines the PostgreSQL schema this platform is
designed around, and `docker-compose.yml` deploys and migrates a real
PostgreSQL instance for sandbox demonstration. Wiring the API's
read/write path onto that schema (`internal/storage/postgres.go` under
`-tags postgres`) is a deliberate, documented follow-up rather than an
untested claim: this repository was built and verified (see
VERIFICATION.md) in an environment without package-registry network
access, so a PostgreSQL driver dependency could not be fetched and
exercised here. The default build (`go build ./...`, no tags) therefore
depends on the Go standard library only and is fully buildable, racy-safe
and tested offline; `-tags postgres` after `go mod tidy -tags postgres`
in a networked environment (the assumed SOMA Genesis Sandbox) compiles in
the PostgreSQL driver via `cmd/api/driver_postgres.go` and
`cmd/migrate/driver_postgres.go`.

## Why no third-party router or metrics library

Scenario 2 and Scenario 4 use `go-chi/chi` and
`prometheus/client_golang`. This scenario deliberately uses the Go 1.22+
standard library's method-and-pattern `http.ServeMux` and a small
hand-rolled Prometheus-text-format metrics registry
(`internal/observability/metrics.go`) instead, so the module has **zero
required third-party build-time dependencies** (`go.mod` declares none).
This was a build-environment necessity (see above) that turned out to be
a reasonable independent design choice: the platform still exposes the
same `/metrics` exposition format and the same REST conventions.

## Dependency simulators and failure injection

`internal/dependencies.Registry` holds one `Simulator` per external
dependency (payment, shipping, inventory-provider, queue/event broker).
Each simulator supports configurable delay, timeout, retryable failure,
permanent failure and scripted recovery (`FailFirstN`), entirely
in-process — no real network call is made. A sandbox-only admin endpoint
(`POST /api/v1/admin/simulators/{dependency}/{tenant}`, gated by
`App.SandboxMode`) lets the private evaluator and the Test Engineer dry
run script this behaviour deterministically without redeploying;
`APP_SANDBOX_MODE=false` disables it for any production-like deployment.

## Order state machine

See `internal/domain/types.go` (`validTransitions`). Every ordinary
transition goes through `orders.Service` → `storage.TransitionOrder{Unsafe,Safe}`,
which records a `StatusHistoryEntry` and (via `orders.Service`) an
`AuditEvent`. The administrative bulk-update path
(`storage.AdminBulkSetStatus`, `POST /api/v1/admin/orders/bulk-status`) is
a deliberately separate code path — see STAB-12 in
`docs/private-issue-manifest.yaml`.