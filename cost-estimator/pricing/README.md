# Price book

`pricebook.json` is the static, versioned pricing model the estimator uses. It
holds nothing live — the point of the example is a deterministic, offline,
auditable estimate, not real-time cloud billing.

| Field | Meaning |
|---|---|
| `version` | Stamped onto every estimate as `cost-estimator.confighub.com/pricing-version`. |
| `rates.cpu_core_hour` | USD per requested CPU core per hour. |
| `rates.memory_gb_hour` | USD per requested GB of memory per hour. |
| `rates.storage_gb_month` | USD per GB of persistent storage per month. |
| `hours_per_month` | Hours used to annualize hourly rates (730 = 365×24/12). |
| `region_multipliers` | Per-region multiplier applied to the whole estimate; region comes from the unit's `Region` label. |
| `default_region` | Region (and multiplier) used when a unit has no `Region` label. |
| `budgets_monthly_usd` | Per-environment monthly budget; the unit's `Environment` label (lower-cased) selects one, else `default`. |

The estimate for one workload:

```
monthly_usd = ( cpu_cores · cpu_core_hour + mem_gb · memory_gb_hour ) · hours_per_month
            + storage_gb · storage_gb_month
            ) · region_multiplier

cpu_cores  = Σ container.requests.cpu    × replicas
mem_gb     = Σ container.requests.memory × replicas
storage_gb = Σ volumeClaimTemplates / PVC storage
```

`budget-status` = `OVER` when `monthly_usd > budget`, `WARN` at ≥ 80%, else `OK`.

Rates are rough public-cloud on-demand figures (≈ AWS us-east-1) chosen so the
demo's `oversized-analytics` workload clears its dev budget. Edit them freely;
re-run `estimate-fleet --write-back` to re-stamp the fleet. Override budgets per
run with `--budgets <file.json>` (same `budgets_monthly_usd` shape).
