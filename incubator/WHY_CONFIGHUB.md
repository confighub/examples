# Why ConfigHub

ConfigHub is a management layer for operational configuration.

It sits between your config sources (Git, Helm charts, existing clusters) and your deployment targets (Kubernetes, Flux, Argo CD). It stores versioned config, applies policy, tracks provenance, and executes controlled delivery.

## What ConfigHub Adds Beyond GitOps

GitOps controllers (Argo CD, Flux) are powerful reconcilers. They sync Git state to cluster state.

ConfigHub adds:

- **Unified view**: import and compare config from Git, live clusters, and multiple controllers in one place
- **Policy and scanning**: validate config against rules before delivery, not after
- **Mutation control**: make controlled changes through one write API with audit
- **Layered composition**: build deployment variants from shared recipes with tracked ancestry
- **Delivery options**: apply through direct workers, Flux OCI, or Argo OCI depending on your environment

ConfigHub does not replace your GitOps controller. It extends what you can see and what you can safely change.

## The Four Reasons

### Import

Ingest existing config from Git repos, live clusters, or GitOps controllers. Compare what Git says with what the cluster has. Surface drift, ownership gaps, and orphaned resources.

### Mutate

Use ConfigHub as a write API for operational config. Apply governed changes through explicit functions with audit and approval workflows.

### Apply

Deliver real changes to real targets. ConfigHub stores the intended state, renders it for the target, and applies through workers or controllers with verification.

### Model

Represent layered or governed configuration structures. Build recipes from shared components, track variant ancestry, preserve provenance across deployment stages.

## Quick Decision Tree

```
What do you want to do?
│
├─ See what I already have → Import
│   └─ gitops-import-argo or gitops-import-flux
│
├─ Make controlled changes → Mutate
│   └─ platform-write-api
│
├─ Deploy real workloads → Apply
│   └─ global-app-layer/single-component
│
└─ Model layered config → Model
    └─ global-app-layer
```

## Glossary

| Term | Meaning |
|------|---------|
| **Space** | A named workspace in ConfigHub |
| **Unit** | One versioned config object |
| **Variant** | A specialized unit derived from another |
| **Clone link** | How shared changes flow downstream with provenance |
| **Worker** | Agent that registers delivery endpoints |
| **Target** | Named delivery endpoint owned by a worker |
| **Bundle** | Deployable output produced for a target |
| **Recipe manifest** | Metadata unit explaining how a recipe was assembled |

## Next Steps

- For the standard GitOps import story: [gitops-import-argo](./gitops-import-argo/README.md) or [gitops-import-flux](./gitops-import-flux/README.md)
- For the smallest real apply proof: [global-app-layer/single-component](./global-app-layer/single-component/README.md)
- For the full incubator catalog: [README.md](./README.md)
- For AI-oriented guidance: [AI_START_HERE.md](./AI_START_HERE.md)
