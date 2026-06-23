// Package cvedb opens the unified CVE database — a single SQLite file accessed
// through a pure-Go driver (modernc.org/sqlite), so there is no sqlite3 binary
// and no cgo. The schema is embedded and applied idempotently on open.
package cvedb

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DefaultPath is the database file. SEC_SCANNER_DB overrides it; the default
// works when commands are run from the example root (as the scripts do).
func DefaultPath() string {
	if v := os.Getenv("SEC_SCANNER_DB"); v != "" {
		return v
	}
	return "cvedb/cve.db"
}

// Status reports the database version — the timestamp of the most recent
// import (advances every time `secscan import` runs) — and the advisory count.
// updatedAt is "" if nothing has been imported yet.
func Status(path string) (updatedAt string, advisories int, err error) {
	db, err := Open(path)
	if err != nil {
		return "", 0, err
	}
	defer db.Close()
	var ts sql.NullString
	if err := db.QueryRow("SELECT max(finished_at) FROM import_log").Scan(&ts); err != nil {
		return "", 0, err
	}
	_ = db.QueryRow("SELECT count(*) FROM advisory").Scan(&advisories)
	return formatVersion(ts.String), advisories, nil
}

// formatVersion turns SQLite's "2006-01-02 15:04:05" (UTC) into a compact,
// token-safe version string with no spaces or colons — "20060102T150405Z" —
// that is still lexicographically ordered by time (so staleness is a string
// comparison). An unparseable or empty value is returned unchanged.
func formatVersion(s string) string {
	if s == "" {
		return ""
	}
	t, err := time.Parse("2006-01-02 15:04:05", s)
	if err != nil {
		return s
	}
	return t.UTC().Format("20060102T150405Z")
}

// Open connects to the database (creating it if needed), enables foreign keys
// so re-imports cascade away stale child rows, and applies the schema.
func Open(path string) (*sql.DB, error) {
	if path == "" {
		path = DefaultPath()
	}
	db, err := sql.Open("sqlite", path+"?_pragma=foreign_keys(1)")
	if err != nil {
		return nil, err
	}
	if _, err := db.Exec(schema); err != nil {
		db.Close()
		return nil, fmt.Errorf("apply schema: %w", err)
	}
	return db, nil
}
