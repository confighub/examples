# cvedb — the unified CVE/advisory database

`sec-scanner` matches the packages it finds inside an image against a local
**SQLite** database of known vulnerabilities (`cve.db`, a single file — no
server, no container). Three upstream sources feed it, and they are
**normalized to one shape before import** so the scanner only ever sees a single
model:

| Source | What it is | How it's read |
|---|---|---|
| **OSV.dev ecosystem exports** | Per-ecosystem zips (`Alpine:v3.9`, `Debian:12`, …) aggregating Alpine secdb, Debian, and language advisories. Best OS-package coverage. | `secscan import --osv-zip Alpine:v3.9` |
| **GitHub Advisory Database** | A clone of [`github/advisory-database`](https://github.com/github/advisory-database) — the "GitHub CVE database". Files under `advisories/` are OSV format. | `secscan import --ghsa <clone>` |
| **Official CVE List V5** | A clone of [`CVEProject/cvelistV5`](https://github.com/CVEProject/cvelistV5) — the canonical CVE records (JSON 5.0). | `secscan import --cvelist <clone>` |

All of them collapse into the OSV-flavored record the schema stores: a canonical
id (CVE-… when available), cross-source **aliases**, a computed **severity**,
and a set of **affected packages** each with version **ranges** and/or an
enumerated **versions** list. Records sharing an alias are merged, keeping the
richest field from each source.

Both the importer and the scanner are the one `secscan` Go binary
(`../scanner/`), which reads and writes this file through a **pure-Go SQLite
driver** (`modernc.org/sqlite`) — no `sqlite3` binary, no cgo, no Python.

## Schema

```
advisory(id, source, sources, summary, details, severity, cvss_score, cvss_vector, published, modified, withdrawn, raw)
advisory_alias(advisory_id, alias)                    -- CVE <-> GHSA cross refs
affected(id, advisory_id, ecosystem, package, purl)   -- ecosystem+package is the scanner's lookup key
affected_range(affected_id, range_type, introduced, fixed, last_affected)
affected_version(affected_id, version)                -- explicitly enumerated affected versions
import_log(source, detail, advisories, finished_at)   -- provenance of each run
```

Severity is computed from the CVSS v3.x **vector** (the OSV exports carry the
vector, not a number) using the published v3.1 base-score formula, then bucketed
CRITICAL ≥ 9.0, HIGH ≥ 7.0, MEDIUM ≥ 4.0, LOW > 0.

## Usage

Build the binary once (`cd ../scanner && go build -o secscan .`), then:

```bash
# the database is just a file; the schema is applied automatically on first open.
export SEC_SCANNER_DB="$PWD/cve.db"        # optional; defaults to cvedb/cve.db

# fastest / offline: curated fixtures that cover the demo images
secscan import --fixtures --fixtures-dir ./fixtures

# real data: OSV ecosystem exports for the OSes you scan (repeatable)
secscan import --osv-zip Alpine:v3.9 --osv-zip Debian:12

# from local clones of the GitHub sources (use --limit while exploring)
secscan import --ghsa    ~/src/advisory-database --limit 5000
secscan import --cvelist ~/src/cvelistV5/cves     --limit 5000

# or just: ./build.sh   (builds secscan + imports OSV; SEC_SCANNER_OFFLINE=1 for fixtures)
```

Re-running is idempotent: an import replaces the advisories it touches (or pass
`--if-empty` to skip entirely when the database is already populated). The full
GitHub clones are large (tens of thousands of files); for an example, the
fixtures or a couple of `--osv-zip` ecosystems are enough and load in seconds.

`secscan gen-fixtures` regenerates `fixtures/alpine-demo.json` from a populated
database (the advisories the demo's vulnerable images match).

## Inspect

The database is an ordinary SQLite file; if you have the `sqlite3` CLI handy:

```bash
sqlite3 cve.db "SELECT source, advisories, finished_at FROM import_log ORDER BY finished_at;"
sqlite3 cve.db "SELECT severity, count(*) FROM advisory GROUP BY severity ORDER BY 2 DESC;"
```
