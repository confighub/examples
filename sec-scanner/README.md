# sec-scanner — Container Image CVEs as Data

Container image security where the image you ship, the scan that judged it, and
the gate that blocks it are **the same object** — because the image reference
and its CVE verdict live in ConfigHub as data, next to the workload, not in a
report that drifts away from the manifest.

This example stands up a small, real vulnerability pipeline and wires it into a
ConfigHub fleet:

- **A unified CVE database.** Three upstream sources — the [GitHub Advisory
  Database](https://github.com/github/advisory-database), the [official CVE List
  V5](https://github.com/CVEProject/cvelistV5), and [OSV.dev](https://osv.dev)
  ecosystem exports — are normalized to **one schema** and loaded into a single
  SQLite file (no database server).
- **A custom scanner.** A small Go binary pulls an image's layers from the
  registry (go-containerregistry/crane, no Docker daemon), flattens them,
  reads the OS package database (apk/dpkg)
  straight out of it, and matches each package against the CVE database with
  ecosystem-aware version comparison. No Trivy, no Grype — the mechanism is in
  the open.
- **The fleet inventory you didn't have.** "Which clusters run an image with a
  known critical CVE?" is one query across every Space, because image refs are
  data.
- **Enforced guardrails, not advisory linting.** Images on `:latest` and images
  the scanner flagged **CRITICAL** are blocked by Apply Gates before they reach
  a cluster. Prod changes additionally require approval.
- **Findings stored back as data.** The gate signal (`max-severity` +
  `cve-count`) is written onto the workload Unit so the gate decides on the same
  object you review and version; the **full findings** land in one
  `AppConfig/YAML` `sec-scan-record` Unit per Space — a multi-document YAML (one
  stable, key-ordered document per workload), uncapped and queryable with
  `where_data`. Re-scanning is byte-stable apart from the timestamp, so the next
  scan is a clean diff.
- **Scan freshness, not guesswork.** Every scan records the CVE DB version it
  ran against; after you re-import the database, `secscan stale` (and a **stale**
  badge in the console) tells you exactly which Units to re-scan — no blind
  full-fleet re-runs.

The layout mirrors [`rbac-manager`](../rbac-manager/): canonical base Units,
per-cluster clones, a central policy Space, and planted violations that make the
gate story visible immediately.

## What demo-setup creates

```
sec-demo-policy    Guardrail Triggers + Filters (no Units)
sec-demo-base      Workload Units on current images: frontend, api, cache
sec-demo-dev       Cluster Space (env=dev)     — clones + planted violations
sec-demo-staging   Cluster Space (env=staging) — clones
sec-demo-prod      Cluster Space (env=prod)    — clones, approval required
```

The ConfigHub Spaces are "paper clusters" — no Targets or Workers, nothing
deploys to live infrastructure. The scanner *does* pull the real demo images
locally in order to scan them; that is the only outside contact. The planted
violations in dev:

| Unit | Image | Why it's blocked |
|---|---|---|
| `legacy-frontend` | `nginx:1.16-alpine` | known **CRITICAL** CVEs → gated by `no-critical-cves` (after scan) |
| `legacy-api` | `python:3.7-alpine3.10` | known **CRITICAL** CVEs → gated by `no-critical-cves` (after scan) |
| `unpinned-web` | `nginx:latest` | floating tag → gated by `no-latest-tag` (static, no scan needed) |

## Layout

```
cvedb/      The unified CVE database — a SQLite file (schema, importer, fixtures) — see cvedb/README.md
scanner/    The custom Go scanner (registry pull → flatten → apk/dpkg → match)  — see scanner/README.md
app/        Security console SPA (React/Vite/MUI) over the ConfigHub API       — see app/README.md
deploy/     Reference container + k8s manifest for hosting the console         — see deploy/README.md
manifests/  Workload Units (current images) and planted violations
setup.sh    Real use: install the image guardrails on your own Spaces
demo-*.sh   Self-contained demo fleet + verification
```

## Console (UI)

A static React SPA in [`app/`](app/) renders the fleet's security posture
directly from the ConfigHub API — image inventory, severity rollup, every CVE,
and per-Unit gate state — with an in-app **Upgrade image** action. It reads the
same data the gates do (image refs + the scanner's verdict annotations); it
computes nothing itself.

```bash
cd app && npm install
CONFIGHUB_URL=http://localhost:9090 npm run dev   # http://localhost:5180
```

Seed and scan the demo fleet first (`./demo-setup.sh`) so the console has data.

## Prerequisites

- [cub CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed and authenticated (`cub auth login`)
- `go` (builds `secscan`, which is both the scanner and the CVE importer)
- network access — the scanner pulls image layers straight from their registries
  (no Docker daemon), and the importer fetches CVE data

The CVE database is a SQLite file accessed through a pure-Go driver, so there's
no Postgres, no Python, no `sqlite3` binary, and no Docker to install.

## Usage

**Real use** — install the guardrails on your own Spaces, then scan your fleet:

```bash
./setup.sh --explain                          # preview (no mutation)
./setup.sh                                     # install guardrails (Warn=true)
./setup.sh --where-space "Slug LIKE 'prod-%'"  # narrow with a filter expression

./cvedb/build.sh                               # create cvedb/cve.db + import CVE data
(cd scanner && go build -o secscan .)
# the scanner talks to the ConfigHub REST API directly (like the web app):
export CONFIGHUB_URL="https://hub.confighub.com" CONFIGHUB_TOKEN="$(cub auth get-token)"
./scanner/secscan scan-fleet --space "<your-spaces>" --write-back

./verify.sh                                    # confirm the guardrails are installed
```

Guardrails install as `Warn=true` (advisory ApplyWarnings, never blocking).
Promote one to a blocking ApplyGate once warnings are clean —
`cub trigger update no-latest-tag --space policy-guardrails --unwarn` — and that
one change enforces it fleet-wide.

**Demo** — a self-contained fleet with planted violations and real scans:

```bash
./demo-setup.sh --explain   # preview the plan (no mutation)
./demo-setup.sh             # seed the fleet, load CVEs, scan, gate (idempotent)
./demo-verify.sh            # assert the layout, gate matrix, and CVE DB
```

Use `SEC_SCANNER_OFFLINE=1 ./demo-setup.sh` to load curated CVE fixtures instead
of downloading OSV data, and `PREFIX=my-prefix` to change the `sec-demo-` prefix.

## Try it

```bash
# Scan one image directly — dig into its packages and known CVEs (no ConfigHub needed)
./scanner/secscan scan nginx:1.16-alpine

# The fleet image inventory: every image, every cluster, one query.
# inventory/scan-fleet read the ConfigHub REST API, so set a token first:
export CONFIGHUB_URL="https://hub.confighub.com" CONFIGHUB_TOKEN="$(cub auth get-token)"
./scanner/secscan inventory --space "sec-demo-*"

# A gated vulnerable image, with the gate that blocks it
cub unit get legacy-frontend --space sec-demo-dev -o jq=".Unit.ApplyGates"

# Which clusters were scanned CRITICAL? (data written back by the scanner)
cub unit list --space "*" --where "Annotations.'sec-scanner.confighub.com/max-severity' = 'CRITICAL'"
```

For a paced, stage-by-stage walkthrough, see [AI_START_HERE.md](AI_START_HERE.md).
Stable command contracts for automation are in [contracts.md](contracts.md).

## Boundaries

This example manages desired-state image configuration and its CVE verdict. It
is deliberately **not** a production scanner (use Trivy/Grype for that — the
scanner here covers OS packages only, apk/dpkg, to keep the mechanism legible),
**not** a runtime admission controller, and **not** a vulnerability-management
system of record. It shows the ConfigHub shape: scan results as data, policy as
gates on that data.
