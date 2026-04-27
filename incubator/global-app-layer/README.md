# Global App Layer — Layered Recipes for ConfigHub

ConfigHub models layered software stacks as real versioned config objects. This package demonstrates that with four working examples.

## Quick Start

**Start with the smallest example:**

```bash
cd incubator/global-app-layer/single-component

# Preview without mutating anything
./setup.sh --explain
./setup.sh --explain-json | jq

# Materialize in ConfigHub
./setup.sh
./verify.sh
./verify.sh --json

# Cleanup
./cleanup.sh
```

For AI-driven walkthroughs: [single-component/AI_START_HERE.md](./single-component/AI_START_HERE.md)

## The Five Examples

| Example | Components | What it proves |
|---------|------------|----------------|
| [single-component](./single-component/) | backend + postgres stub | Smallest layered recipe |
| [frontend-postgres](./frontend-postgres/) | frontend + postgres + backend stub | Two-component recipe |
| [realistic-app](./realistic-app/) | backend + frontend + postgres | Three-component app, no stubs |
| [gpu-eks-h100-training](./gpu-eks-h100-training/) | gpu-operator + nvidia-device-plugin | NVIDIA AICR-shaped substrate stack |
| [enterprise-rag-blueprint](./enterprise-rag-blueprint/) | rag-server + nim-llm + nim-embedding + vector-db | NVIDIA Blueprint-shaped app stack with initiatives + Ollama runtime path on Apple Silicon |

All examples use the same script interface:
- `./setup.sh --explain` — preview the plan
- `./setup.sh --explain-json` — machine-readable plan
- `./setup.sh` — materialize in ConfigHub
- `./verify.sh` — check the structure
- `./verify.sh --json` — machine-readable verification output
- `./cleanup.sh` — remove everything

## Key Terms

This package uses a few words very deliberately:

- `recipe` = the layered app intent plus the provenance record that explains how
  the app was assembled
- `deployment variant` = the final ConfigHub unit you can bind to a target and
  deploy
- `bundle` = a deployable artifact produced for a controller-oriented delivery
  path, usually an OCI artifact
- `bundle evidence` = receipts about that artifact such as digests, checksums,
  SBOMs, attestations, or GUI views

Not every example publishes a bundle. Some examples stop at ConfigHub
materialization or use direct Kubernetes delivery, where the live proof is the
applied cluster state rather than a separate exported artifact.

## Delivery Matrix

| Mode | Status | Use case |
|------|--------|----------|
| **Direct Kubernetes** | Fully working | Simplest real proof |
| **Flux OCI** | Current standard | Controller-oriented delivery with OCI proof |
| **Argo OCI** | Implemented in selected examples | Use only when controller + live evidence are both shown |
| **ArgoCDRenderer** | Working | Renderer path only, not OCI delivery |

## WET-First, Not Live-First

These examples start by materializing intended state in ConfigHub.

1. Preview with `./setup.sh --explain`
2. Materialize with `./setup.sh`
3. Verify with `./verify.sh`
4. Optional: bind a target with `./set-target.sh`
5. Optional: apply live with `cub unit apply`

Live delivery is a later explicit step, not the starting point.

## Read-Only Discovery

To discover active runs without knowing the prefix:

```bash
./find-runs.sh
./find-runs.sh realistic-app --json | jq
```

For AI-safe or CI-safe dry inspection, use the read-only setup preview first:

```bash
./setup.sh --explain
./setup.sh --explain-json | jq .
```

To check live readiness before binding:

```bash
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

---

## NVIDIA Context

This package expresses both NVIDIA layering models — AICR (cluster substrate) and Blueprints (apps).

| Rung | NVIDIA artifact | Layering | Worked example |
|---|---|---|---|
| Substrate | [AICR](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/) | base → platform → accelerator → OS → recipe → deploy | `gpu-eks-h100-training/` |
| App | [Blueprints](https://build.nvidia.com/blueprints) | base → platform → accelerator → profile → recipe → deploy | `enterprise-rag-blueprint/` |

Both examples use stub-friendly images by default (structural proof). The Blueprint example also ships a `STACK=ollama` runtime path that gets a real Metal-accelerated answer on Apple Silicon, with no NVIDIA hardware required.

For NVIDIA fit details:
- [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md) — substrate-rung fit analysis
- [02-nvidia-blueprints-fit.md](./02-nvidia-blueprints-fit.md) — app-rung fit analysis (the spec for `enterprise-rag-blueprint/`)
- [confighub-aicr-value-add.md](./confighub-aicr-value-add.md) — the three stories (safe upgrades, GitOps wedge, fleet variants) common to both rungs

For an NVIDIA-oriented reading path, start with:

- [02-nvidia-blueprints-fit.md](./02-nvidia-blueprints-fit.md)
- [enterprise-rag-blueprint/README.md](./enterprise-rag-blueprint/README.md)
- [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md)
- [gpu-eks-h100-training/README.md](./gpu-eks-h100-training/README.md)
- [how-it-works.md](./how-it-works.md)

## Bundle Status

The bundle story is real, but it is split into separate layers of proof:
- Recipe and layering: real and working
- Deployment variants: real and working
- OCI bundle publication: real on the controller-oriented paths that say so
- Bundle evidence and GUI shape: sample/spec present, not fully proven end to end
- Supply-chain evidence such as SBOMs and attestations: design and fixture work,
  not yet full live proof in this package

See [05-bundle-publication-walkthrough.md](./05-bundle-publication-walkthrough.md) and [bundle-evidence-sample/](./bundle-evidence-sample/).

## How It Works

Each layer becomes a ConfigHub space with units connected by clone links. The variant chain preserves provenance so shared updates can flow downstream safely.

See [how-it-works.md](./how-it-works.md) for the full explanation.

## Prerequisites

| Requirement | Purpose |
|-------------|---------|
| `cub` CLI | ConfigHub commands |
| `cub auth login` | Authenticated session |
| `jq` | JSON processing |
| Kubernetes cluster + worker | For live delivery (optional) |

## AI-Safe Path

- [AI_START_HERE.md](./AI_START_HERE.md)
- [../ai-machine-seams-first.md](../ai-machine-seams-first.md)
- [../ai-cold-eval-prompt-pack.md](../ai-cold-eval-prompt-pack.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
- [whole-journey.md](./whole-journey.md)

## Supporting Documents

- [00-config-hub-hello-world.md](./00-config-hub-hello-world.md) — intro for new users
- [02-recipes-and-layers-spec.md](./02-recipes-and-layers-spec.md) — spec
- [04-bundles-attestation-and-todo.md](./04-bundles-attestation-and-todo.md) — bundle gaps
- [06-bundle-evidence-gui-spec.md](./06-bundle-evidence-gui-spec.md) — GUI spec
- [07-argo-oci-spec.md](./07-argo-oci-spec.md) — Argo OCI details
- [e2e/](./e2e/) — end-to-end tests
