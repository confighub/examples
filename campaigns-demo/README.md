# Campaigns Demo

Scripts to populate a ConfigHub space with 10 compliance campaigns, each backed by a Kyverno CEL policy, plus ~47 sample Kubernetes units for the campaigns to evaluate.

Use this to explore the Campaigns feature in ConfigHub: filtering units, tracking remediation progress, setting priorities and deadlines, and (optionally) running automated policy checks via vet-kyverno.

## Prerequisites

- [`cub` CLI](https://docs.confighub.com/get-started/setup/#install-the-cli) installed and on PATH.
- Run `cub upgrade` to ensure you have the latest build.
- Authenticated to ConfigHub: `cub auth login`

## Quick Start

```bash
./setup.sh      # Create the demo space, units, and campaigns
./cleanup.sh    # Delete everything when you're done
```

By default the scripts target `https://app.confighub.com` and create a space named `campaigns-demo`. Override with environment variables:

```bash
CONFIGHUB_URL=https://my-server.com SPACE=my-space ./setup.sh
```

## What Gets Created

### Space

One space: `campaigns-demo`

### Units (~47)

Sample Kubernetes manifests with intentionally varied compliance characteristics, grouped by team label so the campaign filters can select them. Unit types include:

| Kind | Description |
|------|-------------|
| Deployment (compliant) | Fully configured — probes, limits, nonroot, approved registry |
| Deployment (no probes) | Missing liveness and readiness probes |
| Deployment (no limits) | Missing CPU and memory limits |
| Deployment (root) | No `runAsNonRoot` — runs as UID 0 by default |
| Deployment (Docker Hub) | Image from unapproved public registry |
| Deployment (Node 18) | Legacy Node.js 18 base image |
| Deployment (no labels) | Missing standard `app.kubernetes.io/*` labels |
| StatefulSet | Large-memory database workload |
| CronJob | Batch/scheduled job |
| TLS Secret | Certificate for rotation campaign |

### Campaigns (10)

Each campaign is a **View** with a **Filter** (selects units by team label) and campaign metadata stored in Labels and Annotations. An optional **Trigger** runs `vet-kyverno` against matched units when a Ready worker is connected.

| Campaign | Priority | Status | Policy |
|----------|----------|--------|--------|
| Require App Label | HIGH | in_progress | ValidatingPolicy |
| Resource Limits Enforcement | HIGH | in_progress | ValidatingPolicy |
| High Availability — Minimum Replicas | MEDIUM | draft | ValidatingPolicy |
| Liveness and Readiness Probe Compliance | HIGH | in_progress | ValidatingPolicy |
| Run-As-NonRoot Enforcement | MEDIUM | draft | ValidatingPolicy |
| Disallow Latest Image Tag | HIGH | in_progress | ValidatingAdmissionPolicy |
| Disallow Privileged Containers | HIGH | in_progress | ValidatingAdmissionPolicy |
| Read-Only Root Filesystem | MEDIUM | draft | ValidatingAdmissionPolicy |
| Image Registry Restriction | HIGH | in_progress | ValidatingAdmissionPolicy |
| Disallow Host Ports | LOW | **completed** | ValidatingAdmissionPolicy |

## Campaign Metadata

Campaigns use ConfigHub Labels and Annotations on Views to store their state:

### Labels

| Label | Values |
|-------|--------|
| `campaign` | `"true"` — marks the View as a campaign |
| `campaign-priority` | `HIGH`, `MEDIUM`, `LOW` |
| `campaign-status` | `draft`, `in_progress`, `completed` |

### Annotations

| Annotation | Description |
|------------|-------------|
| `campaign-description` | Human-readable goal |
| `campaign-deadline` | Target completion date (YYYY-MM-DD) |
| `campaign-completed-at` | ISO 8601 timestamp when completed |
| `campaign-trigger-id` | ID of the associated vet-kyverno Trigger |
| `campaign-check-summary` | JSON: `{passing, failing, total, checkedAt}` |

## Using vet-kyverno Triggers

If a Ready bridge worker with `vet-kyverno` support is detected when `setup.sh` runs, a Trigger is created (disabled by default) for each campaign. The Trigger runs the embedded Kyverno CEL policy against every unit matched by the campaign filter.

To set up a worker:

1. Create a worker in your space:
   ```bash
   cub worker create --space campaigns-demo kyverno-worker
   ```

2. Start `cub-worker` with the worker credentials:
   ```bash
   cub-worker
   ```

3. Once the worker is Ready, re-run `./setup.sh` to attach triggers to all campaigns.

4. Enable a trigger in the ConfigHub UI (campaign settings → Trigger → Enable) and mutate a unit to fire it.

## Exploring the Data

```bash
# List all units in the demo space
cub unit list --space campaigns-demo

# Filter by team
cub unit list --space campaigns-demo --where "Labels.team = 'Security'"

# List all campaign views
cub view list --space campaigns-demo --where "Labels.campaign = 'true'"

# Filter by priority
cub view list --space campaigns-demo --where "Labels.campaign-priority = 'HIGH'"

# Filter by status
cub view list --space campaigns-demo --where "Labels.campaign-status = 'in_progress'"
```
