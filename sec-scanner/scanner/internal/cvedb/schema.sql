-- sec-scanner unified CVE/advisory schema (SQLite).
--
-- Three upstream sources (GitHub Advisory Database, the official CVE List V5,
-- and OSV.dev ecosystem exports) are normalized by `secscan import` into ONE
-- shape before they land here. The shape is deliberately OSV-flavored: an
-- advisory has a canonical id, cross-source aliases, a severity, and a set of
-- affected packages, each with version ranges and/or an enumerated version list.
--
-- The scanner does version-range matching in code (ecosystem-specific compare),
-- so this schema just stores the normalized facts.
--
-- Applied idempotently (IF NOT EXISTS) every time the database is opened.
-- Connections enable foreign keys so ON DELETE CASCADE clears the child rows of
-- an advisory that gets re-imported.

CREATE TABLE IF NOT EXISTS advisory (
    id            TEXT PRIMARY KEY,
    source        TEXT NOT NULL,                 -- canonical row's origin: ghsa | cvelist | osv
    sources       TEXT NOT NULL DEFAULT '',      -- comma-joined list of every source that contributed
    summary       TEXT,
    details       TEXT,
    severity      TEXT NOT NULL DEFAULT 'UNKNOWN', -- CRITICAL | HIGH | MEDIUM | LOW | UNKNOWN
    cvss_score    REAL,
    cvss_vector   TEXT,
    published     TEXT,
    modified      TEXT,
    withdrawn     TEXT,
    raw           TEXT
);

CREATE TABLE IF NOT EXISTS advisory_alias (
    advisory_id   TEXT NOT NULL REFERENCES advisory(id) ON DELETE CASCADE,
    alias         TEXT NOT NULL,
    PRIMARY KEY (advisory_id, alias)
);

CREATE TABLE IF NOT EXISTS affected (
    id            TEXT PRIMARY KEY,
    advisory_id   TEXT NOT NULL REFERENCES advisory(id) ON DELETE CASCADE,
    ecosystem     TEXT NOT NULL,
    package       TEXT NOT NULL,
    purl          TEXT
);

CREATE TABLE IF NOT EXISTS affected_range (
    affected_id   TEXT NOT NULL REFERENCES affected(id) ON DELETE CASCADE,
    range_type    TEXT NOT NULL DEFAULT 'ECOSYSTEM',
    introduced    TEXT,
    fixed         TEXT,
    last_affected TEXT
);

CREATE TABLE IF NOT EXISTS affected_version (
    affected_id   TEXT NOT NULL REFERENCES affected(id) ON DELETE CASCADE,
    version       TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS idx_alias_alias       ON advisory_alias (alias);
CREATE INDEX IF NOT EXISTS idx_affected_advisory ON affected (advisory_id);
CREATE INDEX IF NOT EXISTS idx_affected_lookup   ON affected (ecosystem, package);
CREATE INDEX IF NOT EXISTS idx_range_affected    ON affected_range (affected_id);
CREATE INDEX IF NOT EXISTS idx_version_affected  ON affected_version (affected_id);
CREATE INDEX IF NOT EXISTS idx_advisory_severity ON advisory (severity);

CREATE TABLE IF NOT EXISTS import_log (
    id            INTEGER PRIMARY KEY,
    source        TEXT NOT NULL,
    detail        TEXT,
    advisories    INTEGER NOT NULL DEFAULT 0,
    finished_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
