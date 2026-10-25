# Testing

## Public suite (model-visible, safe to run against the seeded export)

```
make test
# or: go test ./tests/public/...
```

Normal unit/integration/API tests. They exercise ordinary, documented
behaviour and do not reveal the location or nature of any seeded STAB
condition.

## Private suite (QA-controlled evaluator)

```
make test-private
# or: go test ./private/... -v
```

Twelve condition tests (`TestSTAB01_...` .. `TestSTAB12_...`), each with
a `seeded_...` subtest that reproduces the condition against the shipped
(buggy) code path and a `corrected_...` subtest that exercises the
corrected primitive the same package exposes, proving both
detectability and solvability without prescribing the only acceptable
repair. This suite is intentionally **not** part of `make test` / the
public CI entry point.

## Race detector

```
make race
```

STAB-01 and STAB-08 are concurrency conditions; both are written to be
robust under `-race` (verified with `-race -count=8` during
construction — see VERIFICATION.md).

## Determinism notes

- `storage.LockAcquireGap` and `storage.TransitionRaceWindow` are
  test-only hooks (zero by default in production) that widen
  concurrency windows so STAB-01 and STAB-08 reproduce reliably inside
  a bounded `go test` run instead of only intermittently under real
  load.
- `cmd/seed` uses a fixed seed and records a SHA-256 hash per dataset.