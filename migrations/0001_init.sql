-- Scenario 3 order-management platform: initial schema.
-- Deployed against PostgreSQL in the SOMA Genesis Sandbox. The
-- in-memory store (internal/storage/memory.go) used for local
-- development and the public/private test suites mirrors this shape.

CREATE TABLE tenants (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    name       TEXT NOT NULL,
    role       TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE orders (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    customer_id TEXT NOT NULL,
    status      TEXT NOT NULL,
    version     BIGINT NOT NULL DEFAULT 0,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_orders_tenant_status ON orders(tenant_id, status);

CREATE TABLE order_items (
    order_id TEXT NOT NULL REFERENCES orders(id),
    sku      TEXT NOT NULL,
    quantity INTEGER NOT NULL,
    unit_jpy BIGINT NOT NULL,
    PRIMARY KEY (order_id, sku)
);

CREATE TABLE order_status_history (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL REFERENCES orders(id),
    from_status TEXT NOT NULL,
    to_status   TEXT NOT NULL,
    actor_id    TEXT NOT NULL,
    reason      TEXT,
    source      TEXT NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_status_history_order ON order_status_history(order_id);

CREATE TABLE inventory_items (
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    sku        TEXT NOT NULL,
    on_hand    INTEGER NOT NULL,
    reserved   INTEGER NOT NULL DEFAULT 0,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (tenant_id, sku)
);

CREATE TABLE inventory_reservations (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL REFERENCES orders(id),
    sku        TEXT NOT NULL,
    quantity   INTEGER NOT NULL,
    status     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE fulfilment_requests (
    id         TEXT PRIMARY KEY,
    order_id   TEXT NOT NULL REFERENCES orders(id),
    status     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Transactional outbox: a repair for STAB-09 is expected to write a
-- row here in the same database transaction as the business-state
-- change it accompanies.
CREATE TABLE outbox_events (
    id            TEXT PRIMARY KEY,
    tenant_id     TEXT NOT NULL REFERENCES tenants(id),
    aggregate_id  TEXT NOT NULL,
    type          TEXT NOT NULL,
    payload       JSONB,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    dispatched_at TIMESTAMPTZ
);
CREATE INDEX idx_outbox_undispatched ON outbox_events(dispatched_at) WHERE dispatched_at IS NULL;

-- Idempotency ledger for inbound event consumption (STAB-02).
CREATE TABLE consumed_events (
    event_id        TEXT NOT NULL,
    idempotency_key TEXT PRIMARY KEY,
    consumed_at     TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE configuration_versions (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    key        TEXT NOT NULL,
    value      TEXT NOT NULL,
    version    BIGINT NOT NULL,
    active     BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_configuration_tenant_key ON configuration_versions(tenant_id, key);

CREATE TABLE dependency_call_records (
    id          TEXT PRIMARY KEY,
    tenant_id   TEXT NOT NULL REFERENCES tenants(id),
    dependency  TEXT NOT NULL,
    order_id    TEXT,
    attempt     INTEGER NOT NULL,
    layer       TEXT NOT NULL,
    outcome     TEXT NOT NULL,
    duration_ms BIGINT NOT NULL,
    "timestamp" TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_dependency_calls_order ON dependency_call_records(order_id);

CREATE TABLE audit_events (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    actor_id   TEXT NOT NULL,
    action     TEXT NOT NULL,
    entity_id  TEXT NOT NULL,
    details    TEXT,
    "timestamp" TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX idx_audit_tenant ON audit_events(tenant_id);

CREATE TABLE batch_jobs (
    id         TEXT PRIMARY KEY,
    tenant_id  TEXT NOT NULL REFERENCES tenants(id),
    kind       TEXT NOT NULL,
    status     TEXT NOT NULL,
    item_count INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE dataset_meta (
    dataset_id TEXT PRIMARY KEY,
    seed       BIGINT NOT NULL,
    profile    TEXT NOT NULL,
    sha256     TEXT NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);