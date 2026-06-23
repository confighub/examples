# secscan — the custom scanner + CVE importer

One self-contained Go binary that owns the whole CVE toolchain — importing the
database, scanning images, and reading/writing it all through a **pure-Go SQLite
driver** (`modernc.org/sqlite`, no cgo and no `sqlite3` binary). It does what an
image scanner does, in the open, so the mechanism is legible:

1. **Dig into the image.** go-containerregistry (crane) pulls the image's layers
   straight from the registry and flattens them in-process (no Docker daemon);
   the scanner reads just the OS package database out of the resulting tar
   stream — `/lib/apk/db/installed` (Alpine), `/var/lib/dpkg/status`
   (Debian/Ubuntu) — plus `/etc/os-release` to learn the ecosystem.
2. **Build the SBOM.** Parse that database into `(package, version)` pairs.
3. **Match against the cvedb.** Load every advisory for the image's ecosystem
   from the SQLite cvedb and decide, with **ecosystem-aware version comparison**
   (apk vercmp and the dpkg algorithm, implemented here), which installed
   versions fall inside an unfixed affected range.

Fleet reads and the findings write-back go through the **ConfigHub REST API
directly** — the same API surface the web app uses (`/unit`, `/space/{id}/unit/
{id}/data`, `/space/{id}/function/invoke`), authenticated with a bearer token.
Image layers are pulled from the registry in-process via go-containerregistry,
and the CVE database is accessed in-process via the embedded SQLite driver. So
the scanner needs no `cub` CLI, no Docker daemon, and no `sqlite3` binary.

Set the ConfigHub endpoint and token (the app's bearer-token mode) for the
`inventory` and `scan-fleet` commands:

```bash
export CONFIGHUB_URL="https://hub.confighub.com"   # or your server
export CONFIGHUB_TOKEN="$(cub auth get-token)"      # bearer token
```

## Build

```bash
go build -o secscan .
```

## Use

```bash
export SEC_SCANNER_DB="$PWD/../cvedb/cve.db"   # optional; defaults to cvedb/cve.db

# scan one image
./secscan scan nginx:1.16-alpine
./secscan scan nginx:1.16-alpine --json

# list the images referenced across a ConfigHub fleet
./secscan inventory --space 'sec-demo-*'

# scan every fleet image; --write-back records the gate signal (max-severity +
# cve-count annotations, which no-critical-cves gates on) plus scan provenance
# (scanned-at + cvedb-version), and publishes the full findings as one
# AppConfig/YAML "sec-scan-record" Unit per Space (a multi-doc YAML, one stable
# document per workload). --status-space publishes a cvedb-status Unit recording
# the current CVE DB version.
./secscan scan-fleet --space 'sec-demo-*' --write-back --status-space sec-demo-policy
./secscan scan-fleet --space 'sec-demo-*' --fail-on CRITICAL   # exit 3 if any image is critical

# after re-importing the CVE DB, see which Units were scanned against an older
# snapshot and need re-scanning
./secscan stale --space 'sec-demo-*'

# load the CVE database (see ../cvedb/README.md for all sources)
./secscan import --osv-zip Alpine:v3.9 --osv-zip Alpine:v3.10 --osv-zip Alpine:v3.12
./secscan import --fixtures --fixtures-dir ../cvedb/fixtures      # offline
./secscan gen-fixtures                                           # regenerate the fixtures file
```

## Output

`scan` prints a per-finding table (or JSON with `--json`); each finding is
`{advisory, severity, cvss_score, package, version, fixed_version}`. `scan-fleet`
prints one row per Unit with its max severity and a C/H/M/L count.

## Scope

OS packages only (apk, dpkg). Language-dependency manifests (npm, PyPI, Go
modules) and RPM are out of scope for this example — the same three-step shape
extends to them, but they are not implemented here. This scanner is the
example's teaching artifact, not a replacement for Trivy/Grype in production.
