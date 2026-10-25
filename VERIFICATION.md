# Verification Record — Scenario 3 (soma-validation-scenario-3-stability)

This record documents what was actually built and actually run for this
construction pass, and states honestly what was not verified and why —
per SOMA-GEN-QA-REPO-001 §3.1 ("no fabricated ... test result ... trace").

## Build environment

This repository was constructed in a container with **no
package-registry network access** (`proxy.golang.org`, `codeload.github.com`
and `gopkg.in` were all unreachable). The module was therefore
deliberately designed to have **zero required third-party build-time
dependencies** (see `docs/architecture.md`, "Why no third-party router or
metrics library") so that every command below could be run and its
output captured directly, rather than asserted.

## Commands run and results (this pass)

```
$ go build ./...
(no output — success)

$ go vet ./...
(no output — success)

$ go test ./tests/public/... -v
ok  	github.com/soma-genesis/scenario-3-order-stability/tests/public	1.202s
(16/16 tests passed: health/ready/version/metrics, order CRUD and
workflow, admin bulk, inventory, configuration, worker status,
redaction, domain state-machine table, validation bounds)

$ go test ./private/... -v
ok  	github.com/soma-genesis/scenario-3-order-stability/private	4.969s
(TestSTAB01_LockOrdering .. TestSTAB12_EmergencyCustomisationBypassesInvariants,
24 sub-tests total, all passed — each condition's "seeded_..." subtest
reproduces the defect against the shipped code path, and its
"corrected_..." subtest exercises the corrected primitive the same
package exposes)

$ go test ./... -race -count=5
ok  	github.com/soma-genesis/scenario-3-order-stability/private	4.969s
ok  	github.com/soma-genesis/scenario-3-order-stability/tests/public	1.202s
(the full public + private suite, repeated 5x under the race detector —
relevant because STAB-01 and STAB-08 are concurrency conditions; the
STAB-01 reproduction was additionally hand-verified across 8 repeated
`-race` runs during construction before being included here)

$ go run ./cmd/seed -out testdata/fixtures -profile small
wrote testdata/fixtures/small.json (200 orders, sha256=672ae11f...)
$ go run ./cmd/seed -out testdata/fixtures -profile medium
wrote testdata/fixtures/medium.json (5000 orders, sha256=9ab919b8...)
$ go run ./cmd/seed -out testdata/fixtures -profile large
wrote testdata/fixtures/large.json (20000 orders, sha256=7bb1e638...)
```

## What this verification covers

- The full Go module builds and vets clean with the standard toolchain,
  offline.
- All twelve STAB conditions are independently reproducible and
  independently correctable inside a deterministic, repeatable `go test`
  run (no live infrastructure required) — this is the private evaluator
  referenced by `docs/private-issue-manifest.yaml`.
- The public suite exercises the HTTP API, domain state machine and
  validation bounds without touching any STAB condition.
- Deterministic dataset generation with a recorded SHA-256 hash.

## What this verification does NOT cover (explicit scope limitation)

- **No live PostgreSQL deployment or query-plan/lock evidence was
  captured against real PostgreSQL.** `migrations/0001_init.sql` was
  authored and reviewed but not applied against a running instance in
  this pass, because this build environment had no way to install/run a
  PostgreSQL server nor fetch a Go PostgreSQL driver. The STAB-01
  (lock-ordering) and STAB-08 (state-race) conditions are instead
  reproduced against `internal/storage.MemoryStore`'s own real
  `sync`-based locking (not a simulated/scripted deadlock) — a
  structurally equivalent circular-wait, verified under `-race`, but
  not the literal PostgreSQL deadlock detector.
- **`docker compose up` was not executed end-to-end** (no verified
  container-orchestration URLs, sandbox preview links, or a deployed
  baseline vs. candidate comparison). The Dockerfiles and compose file
  are written and internally consistent with the rest of the repository
  but unbuilt in this pass. Under SOMA-GEN-QA-REPO-001 §12.1 this
  repository is therefore **not yet at the "Baseline sandbox deployment
  works" gate** — that step requires the SOMA Genesis Sandbox Platform's
  actual build/deploy pipeline and cannot be honestly claimed from this
  container.
- **The frontend was not `npm install`/built or browser-tested** in this
  pass (no verified component-test or browser-smoke-test run); see
  `frontend/` and `docs/testing.md` for what exists.
- **No isolated-seed vs. combined-seed interference matrix run was
  recorded** in the SOMA-GEN-QA-REPO-001 §7.12 sense (there is only one
  seeded source here, not a separate clean/seeded pair with a
  private-provenance Git history) — see the scope note at the top of
  `docs/private-issue-manifest.yaml`.
- **No independent dry run by a second Test Engineer** (§7.16) has been
  recorded.

## Recommended next steps before a QA Manager sign-off

1. In a networked environment, run `go mod tidy -tags postgres`, build
   `-tags postgres`, and capture real `pg_stat`/deadlock evidence for
   STAB-01 against live PostgreSQL alongside the existing in-memory
   evidence.
2. Run `make up` and capture baseline/candidate preview URLs, health
   checks and displayed build identity per §12.2.
3. `npm install && npm run build` the frontend and add the browser-level
   smoke tests referenced in `docs/testing.md`.
4. Assign an independent Test Engineer to execute
   `soma-test-prompts/s3/*.txt` against the deployed baseline without
   Golden Findings access, per §7.16.

## Acceptance control summary (SOMA-GEN-QA-REPO-001 Appendix C, Scenario 3 checklist)

| Item | Status |
|---|---|
| Normal workflows remain usable | ✅ verified (`tests/public`) |
| Twelve instability conditions map to private triggers and evidence | ✅ verified (`private/condition_test.go`, `docs/private-issue-manifest.yaml`) |
| Incident pack contains no root-cause disclosure | ✅ `docs/operational-evidence/incident-register.json` (12 synthetic incidents, symptoms only, no stated root cause) |
| Dependency simulators are deterministic and isolated | ✅ verified (`internal/dependencies`, no real network) |
| At least one first-wave repair set is privately verified | ✅ each STAB condition has a verified corrected primitive; a QA-approved first-wave *combination* still needs to be selected (§10.7) |
| Baseline and candidate deployment comparison works | ❌ not run in this pass — see "What this verification does NOT cover" |