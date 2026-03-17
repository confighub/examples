# Decision: Deterministic Label Mapping Between Cluster/GitOps and ConfigHub

**Date:** 2026-03-17
**Status:** Proposed
**Scope:** cub-gen (gitops import), cub-scout (cluster discovery), cub CLI
**Origin:** Discovered while building the 4-recipe demo in `incubator/global-app-layer/`

## Problem

When `cub gitops discover` or `cub-scout map` observes an existing cluster, it finds Kubernetes resources and GitOps objects (ArgoCD Applications, Flux Kustomizations, Helm releases) that carry labels and annotations. These labels encode organizational meaning — team ownership, environment, tier, region — but there is no convention for how they map to ConfigHub's organizational model (spaces, unit labels, clone chain layers).

Today the mapping is ad-hoc:
- `cub gitops discover --space X` drops everything into space X — the space is chosen by the operator, not derived from cluster metadata.
- ArgoCD Application labels (`team=payments`, `env=prod`) are preserved in the imported YAML but carry no structural meaning in ConfigHub.
- cub-scout collects labels into its graph but doesn't suggest ConfigHub placement.
- The cub-scout `rm-demos-argocd` example's `hub.yaml` uses `environment` and `region` labels on targets, but nothing connects these to the labels found on cluster resources.

This means brownfield import is lossy — the organizational structure encoded in cluster labels is not carried into ConfigHub's space/unit/chain model.

## Context

### What exists today

**cub-gen (importer)** has field-origin mappings that track DRY->WET provenance:
```
spring.application.name  ->  Deployment/metadata/labels[app.kubernetes.io/name]
spec.lifecycle           ->  Application/metadata/labels[lifecycle]
```
These are config-field lineage, not organizational mapping.

**cub-gen (flow)** routes resources to providers by API group:
```go
case strings.Contains(s, "argoproj.io/"):
    return providerArgoCDRenderer
```
This is resource-type classification, not label-based routing.

**cub-scout** collects all labels into its resource graph (`collector_gitops.go`) and surfaces them in `map list --json`, but doesn't interpret them as ConfigHub placement hints.

**cub-scout (import_argocd.go)** imports ArgoCD Applications into ConfigHub units. The target space is passed as `--space` flag. No labels from the Application influence the choice of space or unit labels.

### What the rm-demos-argocd example implies

The `hub.yaml` in cub-scout's examples defines targets with labels:
```yaml
targets:
  - name: prod-us-east-1
    labels: { environment: production, region: us-east }
```

And AppSpaces are organized by team (`platform-team`, `payments-team`). But there's no declared mapping from "ArgoCD Application with label `team=payments`" to "import into space `payments-team`".

### How the 4-recipe demo exposed this

The `incubator/global-app-layer/` recipes proved that swapping between direct and ArgoCD targets is a one-line change (`worker-kubernetes-yaml-cluster` vs `worker-argocdrenderer-kubernetes-yaml-cluster`). But this raised the question: when ArgoCD is the delivery target, how do the ArgoCD Application's labels and the ConfigHub unit's labels stay in sync? Today they don't — they're independent taxonomies that happen to describe the same workload.

## Proposal

### A label-mapping spec with sensible defaults

Introduce a `.confighub/label-map.yaml` (or inline in `hub.yaml`) that declares how source labels (from Kubernetes resources, ArgoCD Applications, Flux objects) map to ConfigHub concepts:

```yaml
# .confighub/label-map.yaml
apiVersion: confighub.com/v1
kind: LabelMap
metadata:
  name: default

spec:
  # Where to find source labels (in priority order)
  sources:
    - kind: Application           # ArgoCD Application
      apiGroup: argoproj.io
    - kind: Kustomization         # Flux Kustomization
      apiGroup: kustomize.toolkit.fluxcd.io
    - kind: HelmRelease           # Flux HelmRelease
      apiGroup: helm.toolkit.fluxcd.io
    - kind: Deployment            # Fallback: workload labels
    - kind: StatefulSet

  # Mapping rules (evaluated in order, first match wins)
  rules:
    # Source label -> ConfigHub space
    - source:
        label: team
        # aliases: [owner, app.kubernetes.io/part-of]  # optional
      target: space
      transform: identity         # "payments" -> space "payments"
      # transform: template       # "payments" -> space "payments-team"
      # template: "{{ .Value }}-team"

    # Source label -> clone chain layer
    - source:
        label: environment
        aliases: [env, app.kubernetes.io/environment]
      target: chain-layer
      # Maps to position in the clone chain
      # e.g., "production" -> the prod layer in base->dev->staging->prod chain

    # Source label -> unit label (passthrough)
    - source:
        label: region
        aliases: [topology.kubernetes.io/region]
      target: unit-label
      key: Region                 # ConfigHub label key (can differ from source)

    # Source label -> unit label (passthrough)
    - source:
        label: tier
        aliases: [app.kubernetes.io/component]
      target: unit-label
      key: Component

    # Source annotation -> unit label
    - source:
        annotation: confighub.com/space
      target: space
      transform: identity         # Explicit override always wins

    # Catch-all: preserve unmatched labels as unit labels
    - source:
        label: "*"
        exclude: [kubectl.kubernetes.io/*, app.kubernetes.io/managed-by]
      target: unit-label
      transform: identity
```

### Default convention (zero-config)

When no `label-map.yaml` exists, apply these defaults:

| Source label | ConfigHub target | Default behavior |
|---|---|---|
| `app.kubernetes.io/name` | unit name | Already done (used as unit slug) |
| `argocd.argoproj.io/instance` | unit name (fallback) | Already done |
| `team` or `owner` | space suggestion | Log suggestion, don't auto-create |
| `environment` / `env` | clone chain hint | Attach as unit label `Environment=X` |
| `app.kubernetes.io/component` | unit label `Component` | Passthrough |
| `topology.kubernetes.io/region` | unit label `Region` | Passthrough |
| `confighub.com/space` | space (explicit override) | Highest priority |
| `confighub.com/chain-layer` | chain layer (explicit) | Highest priority |

The `confighub.com/*` annotations are escape hatches — users can annotate their existing resources to control ConfigHub placement without changing their label taxonomy.

### Interaction with existing tools

**cub-gen `gitops discover`** — When `--label-map` is provided (or `.confighub/label-map.yaml` exists in the repo), the discover step should:
1. Read labels from detected resources
2. Apply mapping rules to derive space and unit labels
3. Include the derived placement in the detection output (the `changes.yaml`)
4. `gitops import` then uses these placements instead of requiring `--space` for everything

**cub-scout `map`** — When connected to ConfigHub, `map list` could show a `suggested-space` column based on the label map. This helps the "explore -> understand -> import" journey.

**cub-scout `import argocd`** — Apply label map rules to derive `--space` automatically when not explicitly provided. Print the derived mapping for confirmation.

### Determinism requirements

1. Same labels + same label-map = same ConfigHub placement. Always.
2. Missing labels = explicit "unmapped" status, never a guess.
3. Conflicting rules = first match wins, with a warning logged.
4. The label-map file is versioned in Git alongside the config it maps.

### Escape hatches

1. `--space` flag always overrides label-derived space (backward compatible).
2. `confighub.com/space` annotation on the resource overrides label rules.
3. `--no-label-map` flag disables automatic mapping entirely.
4. Users can exclude specific labels from mapping via the `exclude` list.

## What this is NOT

- This is not a label *enforcement* policy (that's hub.yaml constraints).
- This is not a label *mutation* system (ConfigHub doesn't write back to cluster labels).
- This is not a replacement for `--space` — it's a *default derivation* when `--space` is omitted.
- This is not required — zero-config import continues to work exactly as today.

## Implementation phases

### Phase 1: Convention documentation + `confighub.com/*` annotations
- Document the default label convention in cub-scout and cub-gen docs.
- Implement `confighub.com/space` and `confighub.com/chain-layer` annotation support in `cub-scout import argocd` and `cub-gen gitops import`.
- No label-map file yet — just explicit annotations.

### Phase 2: Label-map spec + cub-gen integration
- Define the `LabelMap` schema.
- Implement rule evaluation in cub-gen's discover/import flow.
- Add `--label-map` flag to `gitops discover` and `gitops import`.
- Golden tests for each mapping rule type.

### Phase 3: cub-scout integration
- `cub-scout map list --suggest-placement` uses label map to show suggested ConfigHub spaces.
- `cub-scout import argocd` derives `--space` from labels when not explicitly provided.
- Connected mode: upload label map to ConfigHub for server-side use.

### Phase 4: Hub-level label policy
- Allow `hub.yaml` to embed label-map rules (so it's org-wide, not per-repo).
- Add validation: "imported unit has label `environment=production` but was placed in space `dev-team`" -> warning.

## Open questions

1. **Should the label map live in the repo (`.confighub/label-map.yaml`) or in ConfigHub (hub-level config)?** Probably both — repo-level for import-time, hub-level for ongoing governance.
2. **How do Flux labels differ from ArgoCD labels in practice?** Need real-world samples beyond the iits/d2 fixtures in cub-scout.
3. **Should `chain-layer` mapping auto-create clone chains?** Probably not in Phase 1 — just tag with the label and let the user build the chain manually.
4. **What about multi-cluster label conflicts?** Same app label might mean different things across clusters. Scope rules per cluster/context?

## Related

- cub-scout [`import_argocd.go`](https://github.com/confighub/cub-scout) — current ArgoCD import (no label mapping)
- cub-gen `internal/importer/importer.go` — field-origin mappings (config provenance, not organizational)
- cub-scout `internal/graph/collector_gitops.go` — label collection into graph
- cub-scout `examples/rm-demos-argocd/confighub/hub.yaml` — target labels convention
- cub-scout `examples/rm-demos-argocd/confighub/spaces/*.yaml` — team-based space convention
- examples `incubator/global-app-layer/` — 4-recipe demo that exposed this gap
