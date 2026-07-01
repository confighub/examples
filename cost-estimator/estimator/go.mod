// costest — cost-estimator's workload cost estimator.
//
// A self-contained Go binary that:
//   - talks to the ConfigHub REST API directly (CONFIGHUB_URL + CONFIGHUB_TOKEN),
//     the same surface the web app uses, to read workload resource requests and
//     write cost estimates back as annotations — no `cub` shell-out
//   - costs each workload from a SQLite cost database (costdb/cost.db) of
//     KubeCost-style per-(provider, region, resource) rates + per-env budgets
module costest

go 1.25.0

require (
	gopkg.in/yaml.v3 v3.0.1
	modernc.org/sqlite v1.53.0
)

require (
	github.com/dustin/go-humanize v1.0.1 // indirect
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-isatty v0.0.20 // indirect
	github.com/ncruces/go-strftime v1.0.0 // indirect
	github.com/remyoudompheng/bigfft v0.0.0-20230129092748-24d4a6f8daec // indirect
	golang.org/x/sys v0.44.0 // indirect
	modernc.org/libc v1.73.4 // indirect
	modernc.org/mathutil v1.7.1 // indirect
	modernc.org/memory v1.11.0 // indirect
)
