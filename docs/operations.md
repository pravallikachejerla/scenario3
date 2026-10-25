# Operations

## Running locally

```
make build
go run ./cmd/api            # listens on :8081, in-memory store
```

## Running the full stack (sandbox-shaped)

```
make up      # docker compose up --build -d (PostgreSQL, migrate, api, worker, frontend, seed)
make down    # tear down
```

## Health and diagnostics

- `GET /healthz` — liveness.
- `GET /readyz` — readiness + uptime.
- `GET /version` — build commit, build time, dataset id (source/deployment identity mapping).
- `GET /metrics` — Prometheus-text-format counters and gauges (order transitions, dependency calls by layer/outcome, worker queue depth/active, undispatched outbox count).
- `GET /api/v1/workers/status` — worker pool active/queued counts.

## Failure injection (sandbox only)

`POST /api/v1/admin/simulators/{payment|shipping|inventory-provider|queue}/{tenant}`
with body `{"delay_ms": 0, "mode": "none|timeout|retryable|permanent", "fail_first_n": 0}`.
Gated by `App.SandboxMode` (`APP_SANDBOX_MODE` env var); must be `false`
in any production-like deployment.

## Rollback

- Database: `make migrate` applies `migrations/*.sql` forward files only;
  `0001_init.down.sql` reverses the initial schema for local
  development. The sandbox deployment retains forward and reverse diffs
  per candidate per the governance controls in the source-creation
  guidelines.
- Application: the previous container image/tag remains deployable
  alongside the candidate (baseline vs. candidate comparison — see
  VERIFICATION.md).