# soma-validation-scenario-3-stability

Model-visible fixture for **SOMA Genesis validation Scenario 3 —
Application Stabilisation and Reliability Improvement**, built per
`SOMA Genesis Source Repository Creation Guidelines — Scenarios 2, 3 and
4` (SOMA-GEN-QA-REPO-001) §10.

An Enterprise Order Management and Fulfilment platform (React +
TypeScript frontend, Go backend, PostgreSQL-shaped schema) that is
usable under ordinary conditions and exhibits twelve controlled
instability conditions (STAB-01 .. STAB-12) under specific, documented
triggers — database lock ordering, duplicate event processing, stale
configuration cache, worker-pool exhaustion, batch memory retention,
retry amplification, environment configuration drift, concurrent
order-state races, partial transaction/event inconsistency, missing
cancellation propagation, shared cross-tenant failure control, and an
emergency-customisation path that bypasses workflow invariants.

## Quick start

```
make build          # go build ./...
make test           # public suite (safe, model-visible)
make test-private    # private STAB-01..12 evaluator (QA-controlled)
make race            # concurrency conditions under -race
go run ./cmd/api      # in-memory dev server on :8081
make up               # full docker-compose stack (PostgreSQL + api + worker + frontend)
```

## Repository layout

See `docs/architecture.md` for the full layer breakdown. Top level:

```
cmd/            api, migrate, seed, worker entry points
internal/       domain, orders, inventory, fulfilment, batch, events,
                workers, dependencies, configuration, storage, httpapi,
                observability, security, validation
frontend/       React + TypeScript UI
migrations/     PostgreSQL schema (forward + one reverse migration)
testdata/       deterministic seed fixtures (small/medium/large)
tests/public/   model-visible test suite
private/        QA-controlled condition evaluator (STAB-01..12)
docs/           architecture, API, data dictionary, operations, testing
deploy/         Dockerfiles for the sandbox deployment
```

## No required third-party build-time dependency

`go.mod` declares zero third-party requirements. The HTTP layer uses Go
1.22+'s standard-library `net/http.ServeMux` and metrics use a small
hand-rolled Prometheus-text-format registry
(`internal/observability/metrics.go`). See "Why no third-party router or
metrics library" in `docs/architecture.md` for the reasoning — this
repository was built and independently verified (`go build ./...`,
`go vet ./...`, `go test ./... -race`) in an environment without
package-registry network access, so it was designed to need none rather
than to silently fail offline. PostgreSQL connectivity
(`internal/storage/postgres.go`) is a documented, explicitly separate
`-tags postgres` build path for the networked sandbox environment; see
`docs/architecture.md`, "Storage backends".

## Status

See `VERIFICATION.md` for the exact commands run, their output, and the
honest scope/limitations statement for this build pass.