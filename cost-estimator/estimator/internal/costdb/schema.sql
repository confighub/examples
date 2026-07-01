-- cost-estimator cost database schema (SQLite).
--
-- KubeCost-style pricing: per-resource hourly/monthly rates that vary by cloud
-- provider and region, plus the per-environment budgets the within-budget
-- guardrail gates on. Seeded from costdb/fixtures/pricing.json by
-- `costest import`; the estimator reads it to cost each workload.
--
-- Applied idempotently (IF NOT EXISTS) every time the database is opened.

CREATE TABLE IF NOT EXISTS price (
    provider   TEXT NOT NULL,                 -- aws | gcp | azure | ...
    region     TEXT NOT NULL,                 -- us-east-1, us-west-2, ...
    resource   TEXT NOT NULL,                 -- cpu | memory | storage | gpu
    unit       TEXT NOT NULL,                 -- core-hour | gb-hour | gb-month | gpu-hour
    usd        REAL NOT NULL,                 -- price per unit, in USD
    PRIMARY KEY (provider, region, resource)
);

CREATE TABLE IF NOT EXISTS budget (
    environment TEXT PRIMARY KEY,             -- dev | staging | prod | default
    monthly_usd REAL NOT NULL
);

-- Small key/value table: default_provider, default_region, currency, hours_per_month.
CREATE TABLE IF NOT EXISTS meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

-- Provenance of each import; max(finished_at) is the cost-DB version.
CREATE TABLE IF NOT EXISTS import_log (
    source      TEXT NOT NULL,
    detail      TEXT,
    rows        INTEGER NOT NULL DEFAULT 0,
    finished_at TEXT NOT NULL DEFAULT (datetime('now'))
);
