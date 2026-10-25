# Data Dictionary

See `migrations/0001_init.sql` for the authoritative PostgreSQL schema.
`internal/domain/types.go` defines the equivalent Go types used by the
in-memory store.

| Entity | Key fields | Notes |
|---|---|---|
| tenants | id, name | |
| users | id, tenant_id, role | admin / ops_manager / fulfilment_agent / viewer |
| orders | id, tenant_id, status, version | `version` supports optimistic concurrency (see STAB-08) |
| order_items | order_id, sku, quantity, unit_jpy | |
| order_status_history | order_id, from_status, to_status, source | `source` is `workflow` or `admin-bulk` |
| inventory_items | tenant_id, sku, on_hand, reserved | |
| inventory_reservations | order_id, sku, status | held / released / committed |
| fulfilment_requests | order_id, status | queued / in_progress / shipped / failed |
| outbox_events | id, aggregate_id, type, dispatched_at | transactional outbox (STAB-09) |
| consumed_events | event_id, idempotency_key | inbound idempotency ledger (STAB-02) |
| configuration_versions | tenant_id, key, value, version, active | (STAB-03, STAB-07) |
| dependency_call_records | dependency, order_id, attempt, layer, outcome | evidence for STAB-06 retry-amplification |
| audit_events | tenant_id, actor_id, action, entity_id | not written by the admin bulk path (STAB-12) |
| batch_jobs | id, kind, item_count | STAB-05 |
| dataset_meta | dataset_id, seed, sha256 | deterministic-dataset provenance |

## Deterministic datasets

`cmd/seed` generates `testdata/fixtures/{small,medium,large}.json` with a
fixed seed (`20260812`) and records a SHA-256 hash inside each file.
Regenerate with `make seed`.