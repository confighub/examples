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

## The Four Examples

| Example | Components | What it proves |
|---------|------------|----------------|
| [single-component](./single-component/) | backend + postgres stub | Smallest layered recipe |
| [frontend-postgres](./frontend-postgres/) | frontend + postgres + backend stub | Two-component recipe |
| [realistic-app](./realistic-app/) | backend + frontend + postgres | Three-component app, no stubs |
| [gpu-eks-h100-training](./gpu-eks-h100-training/) | gpu-operator + nvidia-device-plugin | NVIDIA AICR-shaped stack |

All examples use the same script interface:
- `./setup.sh --explain` — preview the plan
- `./setup.sh --explain-json` — machine-readable plan
- `./setup.sh` — materialize in ConfigHub
- `./verify.sh` — check the structure
- `./verify.sh --json` — machine-readable verification output
- `./cleanup.sh` — remove everything

## Delivery Matrix

| Mode | Status | Use case |
|------|--------|----------|
| **Direct Kubernetes** | Fully working | Simplest real proof |
| **Flux OCI** | Current standard | Controller-oriented delivery |
| **Argo OCI** | Implemented | Use with controller + live evidence |
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

## NVIDIA AICR Context

This package can express NVIDIA's [AICR](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/) layering model: base → platform → accelerator → OS → recipe → deploy.

The GPU example (`gpu-eks-h100-training`) is a **structural proof** using stub images. Swap in real NVIDIA images for functional GPU deployment.

For AICR details:
- [confighub-aicr-value-add.md](./confighub-aicr-value-add.md)
- [01-nvidia-aicr-fit.md](./01-nvidia-aicr-fit.md)

## Bundle Status

The bundle publication side is explained but not fully proven:
- Recipe and layering: real and working
- Bundle evidence: fixture-backed sample available
- Flux OCI: working controller path
- Argo OCI: implemented in selected examples
- Full in-product bundle inspection: not yet proven

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
