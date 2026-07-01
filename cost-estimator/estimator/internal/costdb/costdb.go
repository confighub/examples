// Package costdb opens the cost database — a single SQLite file accessed through
// a pure-Go driver (modernc.org/sqlite), so there is no sqlite3 binary and no
// cgo. The schema is embedded and applied idempotently on open. It holds
// KubeCost-style per-(provider, region, resource) rates plus per-environment
// budgets; `costest import` loads it from costdb/fixtures/pricing.json.
package costdb

import (
	"database/sql"
	_ "embed"
	"fmt"
	"os"
	"strconv"
	"time"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schema string

// DefaultPath is the database file. COST_ESTIMATOR_DB overrides it; the default
// works when commands are run from the example root (as the scripts do).
func DefaultPath() string {
	if v := os.Getenv("COST_ESTIMATOR_DB"); v != "" {
		return v
	}
	return "costdb/cost.db"
}

// Open connects to the database (creating it if needed) and applies the schema.
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

// Status reports the database version — the timestamp of the most recent import
// (advances every time `costest import` runs) — and the price-row count.
func Status(path string) (version string, prices int, err error) {
	db, err := Open(path)
	if err != nil {
		return "", 0, err
	}
	defer db.Close()
	var ts sql.NullString
	if err := db.QueryRow("SELECT max(finished_at) FROM import_log").Scan(&ts); err != nil {
		return "", 0, err
	}
	_ = db.QueryRow("SELECT count(*) FROM price").Scan(&prices)
	return formatVersion(ts.String), prices, nil
}

// formatVersion turns SQLite's "2006-01-02 15:04:05" (UTC) into a compact,
// token-safe version string with no spaces or colons — "20060102T150405Z" —
// that is still lexicographically ordered by time. Empty/unparseable returned as-is.
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

// ─── Fixtures (import payload) ────────────────────────────────────────────────

// Fixtures is the JSON shape of costdb/fixtures/pricing.json (and any --source).
type Fixtures struct {
	Meta    map[string]string `json:"meta"`
	Prices  []PriceRow        `json:"prices"`
	Budgets []BudgetRow       `json:"budgets"`
}

type PriceRow struct {
	Provider string  `json:"provider"`
	Region   string  `json:"region"`
	Resource string  `json:"resource"`
	Unit     string  `json:"unit"`
	USD      float64 `json:"usd"`
}

type BudgetRow struct {
	Environment string  `json:"environment"`
	MonthlyUSD  float64 `json:"monthly_usd"`
}

// Import upserts the fixtures' prices/budgets/meta into the database and records
// an import_log row. Idempotent: existing rows are replaced.
func Import(path, source string, f *Fixtures) (int, error) {
	db, err := Open(path)
	if err != nil {
		return 0, err
	}
	defer db.Close()
	tx, err := db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck

	for _, p := range f.Prices {
		if _, err := tx.Exec(
			`INSERT INTO price(provider,region,resource,unit,usd) VALUES(?,?,?,?,?)
			 ON CONFLICT(provider,region,resource) DO UPDATE SET unit=excluded.unit, usd=excluded.usd`,
			p.Provider, p.Region, p.Resource, p.Unit, p.USD); err != nil {
			return 0, fmt.Errorf("insert price: %w", err)
		}
	}
	for _, b := range f.Budgets {
		if _, err := tx.Exec(
			`INSERT INTO budget(environment,monthly_usd) VALUES(?,?)
			 ON CONFLICT(environment) DO UPDATE SET monthly_usd=excluded.monthly_usd`,
			b.Environment, b.MonthlyUSD); err != nil {
			return 0, fmt.Errorf("insert budget: %w", err)
		}
	}
	for k, v := range f.Meta {
		if _, err := tx.Exec(
			`INSERT INTO meta(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
			k, v); err != nil {
			return 0, fmt.Errorf("insert meta: %w", err)
		}
	}
	if _, err := tx.Exec(
		`INSERT INTO import_log(source, detail, rows) VALUES(?,?,?)`,
		source, fmt.Sprintf("%d prices, %d budgets", len(f.Prices), len(f.Budgets)), len(f.Prices)); err != nil {
		return 0, fmt.Errorf("insert import_log: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}
	return len(f.Prices), nil
}

// ─── Book (in-memory price model the cost engine queries) ─────────────────────

// Book is the loaded cost model: a price lookup keyed by provider/region/resource,
// budgets by environment, and the meta defaults.
type Book struct {
	Version         string
	DefaultProvider string
	DefaultRegion   string
	Currency        string
	HoursPerMonth   float64
	prices          map[string]float64 // "provider|region|resource" -> usd
	budgets         map[string]float64 // environment -> monthly usd
}

func key(provider, region, resource string) string { return provider + "|" + region + "|" + resource }

// Load reads the whole cost DB into memory for fast, repeated lookups.
func Load(path string) (*Book, error) {
	db, err := Open(path)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	b := &Book{
		DefaultProvider: "aws",
		DefaultRegion:   "us-east-1",
		Currency:        "USD",
		HoursPerMonth:   730,
		prices:          map[string]float64{},
		budgets:         map[string]float64{},
	}

	rows, err := db.Query("SELECT key, value FROM meta")
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			rows.Close()
			return nil, err
		}
		switch k {
		case "default_provider":
			b.DefaultProvider = v
		case "default_region":
			b.DefaultRegion = v
		case "currency":
			b.Currency = v
		case "hours_per_month":
			if n, err := strconv.ParseFloat(v, 64); err == nil && n > 0 {
				b.HoursPerMonth = n
			}
		}
	}
	rows.Close()

	pr, err := db.Query("SELECT provider, region, resource, usd FROM price")
	if err != nil {
		return nil, err
	}
	for pr.Next() {
		var prov, reg, res string
		var usd float64
		if err := pr.Scan(&prov, &reg, &res, &usd); err != nil {
			pr.Close()
			return nil, err
		}
		b.prices[key(prov, reg, res)] = usd
	}
	pr.Close()

	br, err := db.Query("SELECT environment, monthly_usd FROM budget")
	if err != nil {
		return nil, err
	}
	for br.Next() {
		var env string
		var usd float64
		if err := br.Scan(&env, &usd); err != nil {
			br.Close()
			return nil, err
		}
		b.budgets[env] = usd
	}
	br.Close()

	ver, _, err := Status(path)
	if err != nil {
		return nil, err
	}
	b.Version = ver
	return b, nil
}

// Rate returns the price for a resource, falling back from the exact
// (provider, region) to the provider's default region, then the default
// provider's default region.
func (b *Book) Rate(provider, region, resource string) (float64, bool) {
	if provider == "" {
		provider = b.DefaultProvider
	}
	if region == "" {
		region = b.DefaultRegion
	}
	for _, k := range []string{
		key(provider, region, resource),
		key(provider, b.DefaultRegion, resource),
		key(b.DefaultProvider, b.DefaultRegion, resource),
	} {
		if v, ok := b.prices[k]; ok {
			return v, true
		}
	}
	return 0, false
}

// Budget returns the monthly budget for an environment (falling back to
// "default"), and whether one is configured.
func (b *Book) Budget(env string) (float64, bool) {
	if v, ok := b.budgets[env]; ok {
		return v, true
	}
	if v, ok := b.budgets["default"]; ok {
		return v, true
	}
	return 0, false
}
