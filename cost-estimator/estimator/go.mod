// costest — cost-estimator's workload cost estimator.
//
// A self-contained Go binary that:
//   - talks to the ConfigHub REST API directly (CONFIGHUB_URL + CONFIGHUB_TOKEN),
//     the same surface the web app uses, to read workload resource requests and
//     write cost estimates back as annotations — no `cub` shell-out
//   - costs each workload from a static, versioned price book (pricing/pricebook.json)
module costest

go 1.25.0

require gopkg.in/yaml.v3 v3.0.1
