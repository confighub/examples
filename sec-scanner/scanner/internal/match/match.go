// Package match loads candidate advisories for an ecosystem from the cvedb and
// decides which installed packages are affected, doing version-range comparison
// in Go (ecosystem-aware) rather than in SQL.
package match

import (
	"fmt"

	"secscan/internal/cvedb"
	"secscan/internal/pkgdb"
)

// Finding is one vulnerable (package, advisory) pair.
type Finding struct {
	Advisory     string  `json:"advisory"`
	Severity     string  `json:"severity"`
	Score        float64 `json:"cvss_score"`
	Package      string  `json:"package"`
	Version      string  `json:"version"`
	FixedVersion string  `json:"fixed_version,omitempty"`
}

type rng struct {
	introduced, fixed, lastAffected string
}

type entry struct {
	adv      string
	severity string
	score    float64
	ranges   []rng
	versions []string
}

// DB is an in-memory index of one ecosystem's advisories, keyed by package.
type DB struct {
	Ecosystem string
	byPkg     map[string][]entry
}

const query = `
SELECT a.package, v.id, v.severity, COALESCE(v.cvss_score,0), 'R',
       COALESCE(r.introduced,''), COALESCE(r.fixed,''), COALESCE(r.last_affected,'')
  FROM affected a
  JOIN advisory v ON v.id = a.advisory_id
  JOIN affected_range r ON r.affected_id = a.id
 WHERE a.ecosystem = ? AND v.withdrawn IS NULL
UNION ALL
SELECT a.package, v.id, v.severity, COALESCE(v.cvss_score,0), 'V',
       av.version, '', ''
  FROM affected a
  JOIN advisory v ON v.id = a.advisory_id
  JOIN affected_version av ON av.affected_id = a.id
 WHERE a.ecosystem = ? AND v.withdrawn IS NULL`

// Load reads every advisory for the given ecosystem into memory, via the
// pure-Go SQLite driver (no external sqlite3 binary).
func Load(ecosystem string) (*DB, error) {
	if ecosystem == "" {
		return nil, fmt.Errorf("empty ecosystem (unrecognized base image OS)")
	}
	sqldb, err := cvedb.Open("")
	if err != nil {
		return nil, err
	}
	defer sqldb.Close()

	rows, err := sqldb.Query(query, ecosystem, ecosystem)
	if err != nil {
		return nil, fmt.Errorf("query cvedb: %w", err)
	}
	defer rows.Close()

	db := &DB{Ecosystem: ecosystem, byPkg: map[string][]entry{}}
	// Accumulate ranges/versions per (package, advisory).
	type key struct{ pkg, adv string }
	acc := map[key]*entry{}
	var order []key
	for rows.Next() {
		var pkg, adv, sev, kind, c6, c7, c8 string
		var score float64
		if err := rows.Scan(&pkg, &adv, &sev, &score, &kind, &c6, &c7, &c8); err != nil {
			return nil, err
		}
		k := key{pkg, adv}
		e := acc[k]
		if e == nil {
			e = &entry{adv: adv, severity: sev, score: score}
			acc[k] = e
			order = append(order, k)
		}
		switch kind {
		case "R":
			e.ranges = append(e.ranges, rng{introduced: c6, fixed: c7, lastAffected: c8})
		case "V":
			if c6 != "" {
				e.versions = append(e.versions, c6)
			}
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	for _, k := range order {
		db.byPkg[k.pkg] = append(db.byPkg[k.pkg], *acc[k])
	}
	return db, nil
}

// Scan returns the findings for a set of installed packages.
func (db *DB) Scan(pkgs []pkgdb.Package) []Finding {
	var out []Finding
	for _, p := range pkgs {
		for _, e := range db.byPkg[p.Name] {
			if fixed, ok := vulnerable(p, e); ok {
				out = append(out, Finding{
					Advisory: e.adv, Severity: e.severity, Score: e.score,
					Package: p.Name, Version: p.Version, FixedVersion: fixed,
				})
			}
		}
	}
	return out
}

// vulnerable decides whether installed package p is affected by advisory entry
// e, returning the fixed version to report when known.
func vulnerable(p pkgdb.Package, e entry) (string, bool) {
	for _, r := range e.ranges {
		// lower bound: empty or "0" means unbounded below
		if r.introduced != "" && r.introduced != "0" {
			if pkgdb.Compare(p.Type, p.Version, r.introduced) < 0 {
				continue
			}
		}
		switch {
		case r.fixed != "":
			if pkgdb.Compare(p.Type, p.Version, r.fixed) < 0 {
				return r.fixed, true
			}
		case r.lastAffected != "":
			if pkgdb.Compare(p.Type, p.Version, r.lastAffected) <= 0 {
				return "", true
			}
		default:
			// introduced with no upper bound: still vulnerable
			return "", true
		}
	}
	for _, v := range e.versions {
		if pkgdb.Compare(p.Type, p.Version, v) == 0 {
			return "", true
		}
	}
	return "", false
}
