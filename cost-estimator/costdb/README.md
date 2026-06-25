# costdb — the cost database

`cost-estimator` prices each workload against a local **SQLite** cost database
(`cost.db`, a single file — no server, no container), the same way KubeCost
prices from cloud rate data: **per-resource rates that vary by provider and
region**, plus the per-environment budgets the `within-budget` guardrail gates
on.

The importer and the estimator are the one `costest` Go binary
(`../estimator/`), which reads and writes this file through a **pure-Go SQLite
driver** (`modernc.org/sqlite`) — no `sqlite3` binary, no cgo, no Python.

## Schema

```
price(provider, region, resource, unit, usd)   -- PK (provider, region, resource)
    resource ∈ cpu (core-hour) | memory (gb-hour) | storage (gb-month) | gpu (gpu-hour)
budget(environment, monthly_usd)               -- dev | staging | prod | default
meta(key, value)                               -- default_provider, default_region, currency, hours_per_month
import_log(source, detail, rows, finished_at)  -- provenance; max(finished_at) is the cost-DB version
```

A workload's monthly cost:

```
monthly = ( cpu_cores·cpu_rate + mem_gb·memory_rate + gpu·gpu_rate )·hours_per_month
        + storage_gb·storage_rate
```

where the rates are looked up by the unit's `(Provider, Region)` labels, falling
back to the provider's default region and then the default provider. `cpu_cores`
etc. are `requests × replicas` (see [`../README.md`](../README.md)). `budget-status`
compares `monthly` against the environment's budget (`OVER` > 100%, `WARN` ≥ 80%).

## Usage

Build the binary once (`cd ../estimator && go build -o costest .`), then:

```bash
export COST_ESTIMATOR_DB="$PWD/cost.db"        # optional; defaults to costdb/cost.db

# load the curated rates + budgets (offline, deterministic)
costest import --fixtures --fixtures-dir ./fixtures

# or import a custom pricing export (same JSON shape as fixtures/pricing.json)
costest import --source /path/to/pricing.json

# or just: ./build.sh   (builds costest + imports the fixtures; COST_SOURCE=<file> to override)
```

Re-running is idempotent: an import upserts the rows it touches (or pass
`--if-empty` to skip when the database is already populated). Edit
`fixtures/pricing.json` (or supply `--source`) and re-import to re-price; then
`costest estimate-fleet --write-back` re-stamps the fleet, and every Unit records
the new cost-DB version in its `cost-estimator.confighub.com/pricing-version`
annotation.

The rates here are rough public on-demand list prices (≈ AWS / GCP), enough to
make the model concrete. They are **not** a live billing feed — the point is a
deterministic, offline, auditable estimate.

## Inspect

The database is an ordinary SQLite file; if you have the `sqlite3` CLI:

```bash
sqlite3 cost.db "SELECT source, rows, finished_at FROM import_log ORDER BY finished_at;"
sqlite3 cost.db "SELECT provider, region, resource, usd FROM price ORDER BY 1,2,3;"
sqlite3 cost.db "SELECT * FROM budget;"
```
