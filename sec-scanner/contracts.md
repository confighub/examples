# Contracts: sec-scanner

Stable, machine-checkable behavior for this example. See
[EXAMPLE_CONTRACT_STANDARD.md](../EXAMPLE_CONTRACT_STANDARD.md) for the format.

### `./setup.sh` (real use)

- mutates: yes (ConfigHub only)
- creates: 3 Warn=true image guardrail Triggers (label `Pack=sec-guardrails`)
  plus a `Trigger` Filter selecting them, ONCE in a policy Space (default
  `policy-guardrails`, override with `--policy-space SLUG`)
- wires: points each in-scope Space's `TriggerFilterID` at that Filter —
  Spaces with Kubernetes/YAML Units, optionally narrowed with `--where-space EXPR`
- skips: Spaces with a custom `WhereTrigger`, a different `TriggerFilterID`, or
  Triggers of their own (reported, not modified); already-wired Spaces and
  existing objects (idempotent)
- supports: `--policy-space SLUG`, `--where-space EXPR`, `--explain` /
  `--explain-json` (the latter two mutate nothing)
- proves: image guardrails can be installed on a real organization, defined
  once and enforced fleet-wide, without blocking anyone (ApplyWarnings)

### `./verify.sh` (real use)

- mutates: no
- supports: `--policy-space SLUG`, `--where-space EXPR`
- output shape: plain text, one `ok`/`FAIL` line per check
- stable success text: `All checks passed.`
- proves: the policy Space holds the three guardrail Triggers (warn or promoted
  to blocking) and the Filter, and every in-scope Space points its
  `TriggerFilterID` at that Filter

### `./demo-setup.sh --explain`

- mutates: no
- output shape: plain text plan with ASCII diagram
- stable text anchors: `sec-scanner setup plan`, `Mutates: ConfigHub`
- proves: the example plan before any mutation

### `./demo-setup.sh --explain-json`

- mutates: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`,
  `mutates_live_infra`, `mutates_local_cvedb`, `pulls_images_locally`,
  `spaces`, `units`, `cve_sources`, `osv_ecosystems_imported`,
  `notes.expected_apply_gates`, `evaluation_modes`
- proves: the plan in machine-readable form, including which planted violations
  must end up gated and by which guardrail

### `./demo-setup.sh`

- mutates: yes (ConfigHub Spaces/Units + a local SQLite file at cvedb/cve.db;
  pulls the demo images locally to scan them; no Targets, Workers, or live infrastructure)
- creates: 5 spaces, 4 triggers, 2 filters, 3 base units, 9 cloned units,
  3 violation units; then loads the cvedb, writes the gate-signal annotations
  back onto every workload (incl. `scanned-at` + `cvedb-version`), publishes one
  `AppConfig/YAML` `sec-scan-record` Unit per Space (a multi-document YAML with
  the full findings, one document per workload), and a `cvedb-status` Unit
  (current CVE DB version) in the policy Space
- supports: `--no-scan` (seed ConfigHub only; skip cvedb + scan),
  `SEC_SCANNER_OFFLINE=1` (curated fixtures instead of OSV downloads)
- idempotent: re-running skips existing entities and re-scans
- cleanup: none by design (demo data persists); manual teardown in AI_START_HERE.md

### `./demo-verify.sh`

- mutates: no
- output shape: plain text, one `ok`/`FAIL` line per check
- stable success text: `All checks passed.`
- proves: the Space/Trigger/Filter/Unit layout exists; the scanner wrote
  `max-severity=CRITICAL` onto the vulnerable Units; each planted violation
  carries exactly its intended Apply Gate (`legacy-frontend`/`legacy-api` →
  `no-critical-cves`, `unpinned-web` → `no-latest-tag`); clean workloads are
  ungated; prod requires approval; each Space's `AppConfig/YAML`
  `sec-scan-record` Unit exists and holds per-workload findings documents; units
  record the `cvedb-version` they were scanned against and the `cvedb-status`
  Unit is present; the cvedb holds advisories

### `./scanner/secscan stale --space <glob>`

- mutates: no
- reads: the local cvedb version (latest import) + each in-scope Unit's
  `cvedb-version` annotation, via the REST API
- output: the Units scanned against an older CVE DB (or never scanned) and a
  re-scan hint; JSON with `--json`
- proves: re-importing the CVE database is detectable — you know exactly which
  Units to re-scan, without re-scanning everything

### `./scanner/secscan import`

- mutates: the cvedb SQLite file (default `cvedb/cve.db`), not ConfigHub
- sources: `--osv-zip ECO|URL|FILE` (repeatable), `--ghsa DIR`,
  `--cvelist DIR`, `--fixtures`; `--limit N` caps records per source;
  `--if-empty` skips when the database already has advisories
- behavior: normalizes every source to one OSV-flavored schema, dedupes by
  shared alias, loads through a pure-Go SQLite driver (no `sqlite3` binary, no
  Python); idempotent (replaces touched advisories)
- proves: three heterogeneous CVE sources unify into one queryable schema, in a
  single self-contained Go binary

### `./scanner/secscan scan <image>`

- mutates: no (pulls the image locally if absent)
- output shape: plain text finding table, or JSON (`--json`) — each finding is
  `{advisory, severity, cvss_score, package, version, fixed_version}` plus a
  per-image `max_severity` and C/H/M/L `counts`
- proves: an image's OS packages (apk/dpkg) can be extracted and matched against
  the cvedb with ecosystem-aware version comparison, no external scanner

### `./scanner/secscan inventory --space <glob>`

- mutates: no
- reads: the ConfigHub REST API directly (`CONFIGHUB_URL` + `CONFIGHUB_TOKEN`),
  the same surface the web app uses — no `cub` CLI
- output shape: plain text `SPACE UNIT IMAGE` rows, or JSON (`--json`)
- proves: the fleet's image inventory is a single query over ConfigHub data

### `./scanner/secscan scan-fleet --space <glob> --write-back`

- mutates: yes (ConfigHub) — on each workload Unit, sets the gate-signal +
  provenance annotations `sec-scanner.confighub.com/max-severity`, `.../cve-count`,
  `.../scanned-at`, `.../cvedb-version` via a server-side `yq-i` invocation; and
  upserts one **`AppConfig/YAML`** `sec-scan-record` Unit per Space (labeled
  `role=scan-record`) — a multi-document YAML holding the full, uncapped findings,
  one stable key-ordered document per workload (keyed by `unit:`)
- supports: `--status-space SLUG` (upsert a `cvedb-status` `AppConfig/YAML` Unit
  recording the current CVE DB version), `--where EXPR`, `--fail-on SEV`, `--json`
- all via the REST API; requires `CONFIGHUB_URL` + `CONFIGHUB_TOKEN`
- supports: `--where EXPR`, `--fail-on SEV` (exit 3 if any image ≥ SEV), `--json`
- proves: scan verdicts become ConfigHub data, which the no-critical-cves
  guardrail then gates on — config and verdict are the same versioned object
