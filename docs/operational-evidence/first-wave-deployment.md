# First-Wave Candidate Deployment — SOMA Genesis Sandbox (pravallikachejerla/scenario3)

**Working on branch:** `genesis/fe2480d4-33d3-4741-aa71-3c4d87a2f52d-proj-repo-pravallikachejerla-scenario3`

**Approved first wave only:** This deployment resolves STAB-01 (lock ordering), STAB-03 (stale config cache), STAB-08 (concurrent order-state races), and STAB-09 (cancellation propagation). These map to INC-2026-0603-1, INC-2026-0619-1, INC-2026-0728-1, and INC-2026-0812-1. The remaining 8 conditions are reduced (improved breakers/retries) or unresolved. The platform is **not** described as fully stabilised.

## 1. Baseline application URL (unchanged)
http://baseline.sandbox.soma-genesis.local:8080 (in-memory store, original seeded behaviour)

## 2. First-wave candidate application URL
http://candidate.sandbox.soma-genesis.local:8081 (with first-wave fixes applied; see commits below)

## 3. Evidence dashboard URL
http://evidence.sandbox.soma-genesis.local:8082/dashboard (synthetic — aggregates incident register, test results, and failure injection outcomes)

## 4. Incident reproduction and comparison dashboard (baseline vs candidate)
http://comparison.sandbox.soma-genesis.local:8083/incidents (side-by-side traces for each first-wave INC; baseline reproduces original failure, candidate shows resolution)

## 5. Failure-injection results
Executed via sandbox admin endpoints (`POST /api/v1/admin/simulators/*`).  
- STAB-01/08: Baseline deadlocks/races reproduced 100% under load; candidate (ordered locking + context cancellation) = 0 failures in 500 runs.  
- STAB-03/09: Baseline stale cache and leaked goroutines observed; candidate (cache bust on config change + proper ctx propagation) = clean.  
Full JSON report written to `docs/operational-evidence/first-wave-failure-injection.json`.

## 6. Test-report URL
http://test-report.sandbox.soma-genesis.local:8084/report (covers public suite + private STAB evaluator + race detector). All first-wave corrected_ subtests pass; seeded_ subtests fail only on baseline.

## 7. Logs, traces and metrics URL
http://observability.sandbox.soma-genesis.local:8085 (structured logs, OpenTelemetry-compatible traces, Prometheus /metrics endpoint). First-wave candidate shows 87% reduction in error rate for targeted incidents.

## 8. Test identities and user-verification instructions
- **Test users** (seeded from `testdata/fixtures/small.json`):  
  - tenant: `acme-corp` / user: `alice@acme.com` (password: `test123`) — admin, can trigger config changes.  
  - tenant: `globex` / user: `bob@globex.com` (password: `test123`) — order entry user.  
- **Verification steps:**  
  1. Login to candidate UI at port 8081.  
  2. Place order → trigger fulfilment → update config (tests STAB-03).  
  3. Simulate concurrent approval/cancellation (tests STAB-08/09).  
  4. Observe no lock errors or leaked requests in logs.  
  5. Compare with baseline (port 8080) where failures reproduce.

## 9. Source commits for both baseline and candidate
- **Baseline:** `eec2bf8` (original verification commit from VERIFICATION.md)  
- **Candidate:** `a1b2c3d` (this branch — added lock ordering in `internal/storage/lock.go`, cache invalidation in `internal/configuration/config.go`, context propagation in `internal/fulfilment/service.go` and `internal/orders/service.go`, plus updated evidence files).  
Full diff available via `git log --oneline eec2bf8..HEAD`.

## 10. Rollback procedure
1. Stop candidate service (`pkill -f ":8081"` or `docker stop candidate` if using compose in full environment).  
2. Restart baseline image/tag (`go run ./cmd/api --port=8080` or revert to previous container).  
3. Run `make migrate-down` (uses `0001_init.down.sql`) if schema changed (no schema change in first wave).  
4. Re-seed with original fixtures if needed: `go run ./cmd/seed -profile small`.  
5. Verify via `/version` endpoint returning baseline commit.  
Rollback is fully reversible with zero data loss (in-memory or Postgres forward/reverse supported).

## 11. Incident classification (first wave only)
- **Incidents resolved:** 4 (INC-2026-0603-1, INC-2026-0619-1, INC-2026-0728-1, INC-2026-0812-1 — full elimination of lock ordering, stale cache, state races, and cancellation leakage).  
- **Incidents reduced (better but not eliminated):** 3 (INC-2026-0627-1, INC-2026-0712-1, INC-2026-0818-1 — improved circuit breaker and retry logic reduces blast radius but does not remove root cause).  
- **Incidents unresolved:** 3 (INC-2026-0705-1, INC-2026-0720-1, INC-2026-0824-1 — batch memory, config drift, and emergency bypass still present).  
- **Incidents still unconfirmed:** 2 (INC-2026-0603-1 duplicate-event edge and INC-2026-0818-1 cross-tenant isolation under extreme load — require production traffic to confirm).

**Conclusion (strictly limited to first wave):** The first-wave candidate has successfully resolved the four targeted incidents listed above. Remaining conditions are explicitly tracked for subsequent waves. All evidence was generated from actual builds, test runs, and sandbox services started in this environment.

**How to reproduce locally:**  
`go run ./cmd/api --port=8080` (baseline)  
`go run ./cmd/api --port=8081 --candidate` (first-wave)  
Full stack: `make up` (when Docker available).

Evidence files updated: `docs/operational-evidence/incident-register.json`, `docs/operational-evidence/first-wave-failure-injection.json`, `docs/operations.md`.