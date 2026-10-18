-- 0016: ADR-0006 cross-module contract bus + M-FIN integer-kobo ledger engine.
-- These tables are embedded in internal/database/sqlite for SQLite; this
-- migration keeps Postgres on the identical contract. All monetary columns
-- are BIGINT kobo — floats never touch the stored ledger.

-- Transactional outbox: producers insert an event row in their own
-- transaction, the dispatcher delivers it afterwards (at-least-once).
CREATE TABLE IF NOT EXISTS domain_events (
    id            TEXT PRIMARY KEY,
    property_id   TEXT NOT NULL,
    type          TEXT NOT NULL,
    source        TEXT NOT NULL DEFAULT '',
    entity_id     TEXT DEFAULT '',
    payload       TEXT NOT NULL DEFAULT '{}',
    status        TEXT NOT NULL DEFAULT 'pending',
    attempts      INTEGER NOT NULL DEFAULT 0,
    error         TEXT NOT NULL DEFAULT '',
    emitted_by    TEXT,
    at            BIGINT NOT NULL DEFAULT 0,
    dispatched_at BIGINT NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_domain_events_property_at ON domain_events(property_id, at);
CREATE INDEX IF NOT EXISTS idx_domain_events_property_type ON domain_events(property_id, type);
CREATE INDEX IF NOT EXISTS idx_domain_events_status ON domain_events(status);

-- M-FIN chart of accounts (property-scoped, engine-neutral).
CREATE TABLE IF NOT EXISTS gl_accounts (
    id         TEXT PRIMARY KEY,
    property_id TEXT NOT NULL,
    code       TEXT NOT NULL,
    name       TEXT NOT NULL,
    type       TEXT NOT NULL,
    subtype    TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE (property_id, code)
);

-- Flat double-entry lines (mirror of Convex glEntries, kobo integers).
CREATE TABLE IF NOT EXISTS gl_entries (
    id            TEXT PRIMARY KEY,
    property_id   TEXT NOT NULL,
    business_date DATE NOT NULL,
    account_code  TEXT NOT NULL,
    debit_kobo    BIGINT NOT NULL DEFAULT 0,
    credit_kobo   BIGINT NOT NULL DEFAULT 0,
    description   TEXT NOT NULL DEFAULT '',
    ref           TEXT,
    source        TEXT NOT NULL DEFAULT '',
    emitted_by    TEXT,
    at            BIGINT NOT NULL DEFAULT 0,
    created_at    TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_gl_entries_property_date ON gl_entries(property_id, business_date);
CREATE INDEX IF NOT EXISTS idx_gl_entries_ref ON gl_entries(property_id, ref, source);

-- Period-lock rows for the ledger (SQLite has this table natively; PG
-- historically used system_configs — the ledger contract needs this shape).
CREATE TABLE IF NOT EXISTS system_settings (
    id          TEXT PRIMARY KEY,
    property_id TEXT NOT NULL,
    category    TEXT NOT NULL DEFAULT 'general',
    key         TEXT NOT NULL,
    value       TEXT,
    data_type   TEXT DEFAULT 'string',
    description TEXT,
    created_at  TIMESTAMP DEFAULT NOW(),
    updated_at  TIMESTAMP DEFAULT NOW(),
    UNIQUE (property_id, category, key)
);

-- Loyalty member/ledger tables shared by the CRM event subscribers
-- (SQLite-shaped; PG 0013 only carried tiers and points log).
CREATE TABLE IF NOT EXISTS loyalty_members (
    id                   TEXT PRIMARY KEY,
    property_id          TEXT NOT NULL,
    guest_id             TEXT NOT NULL,
    member_number        TEXT UNIQUE NOT NULL,
    tier_id              TEXT,
    total_points         INTEGER DEFAULT 0,
    lifetime_points      INTEGER DEFAULT 0,
    lifetime_value_kobo  BIGINT NOT NULL DEFAULT 0,
    enrolled_at          TIMESTAMP DEFAULT NOW(),
    last_activity        TIMESTAMP
);
CREATE TABLE IF NOT EXISTS loyalty_transactions (
    id           TEXT PRIMARY KEY,
    member_id    TEXT NOT NULL,
    type         TEXT NOT NULL,
    points       INTEGER DEFAULT 0,
    description  TEXT,
    reference_id TEXT,
    created_at   TIMESTAMP DEFAULT NOW()
);
CREATE INDEX IF NOT EXISTS idx_loyalty_tx_member ON loyalty_transactions(member_id, type, reference_id);

-- Backfill for tables created before the column existed.
ALTER TABLE loyalty_members ADD COLUMN IF NOT EXISTS lifetime_value_kobo BIGINT NOT NULL DEFAULT 0;
