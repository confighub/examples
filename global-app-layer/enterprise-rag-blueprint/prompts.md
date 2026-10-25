# Copyable Prompts

## Orient Me First

Read `enterprise-rag-blueprint` and do not mutate anything yet.

Explain:
- what blueprint it represents (NVIDIA's Enterprise RAG)
- what stack it is for
- what it reads
- what it writes
- the difference between stub, ollama, and nim STACK paths
- what counts as structural proof vs functional runtime proof
- what success looks like

Then run only the read-only preview commands:

```bash
STACK=ollama ./setup.sh --explain
STACK=ollama ./setup.sh --explain-json | jq
```

Tell me whether `cub` auth works.
Do not guess unsupported `cub` subcommands or JSON paths; check the docs and `--help` first.

## Safe Walkthrough

Guide me through `enterprise-rag-blueprint` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub or live infrastructure
- say what success looks like
- say what to inspect next in both CLI and GUI
- use the documented jq anchors for inspection commands
- after setup, surface the printed GUI URLs and `.logs/*.latest.log` files
- keep the distinction clear between the direct deployment variant and the Flux/Argo deployment variants
- for raw-manifest delivery, route live targets by provider type:
  - `Kubernetes` → direct deployment variant
  - `FluxOCI` or `FluxOCIWriter` → Flux deployment variant
  - `ArgoCDOCI` → Argo deployment variant

## Verify Everything

After running `enterprise-rag-blueprint`, verify:
- created spaces and units
- layered variant ancestry
- recipe manifest content (4 components, 3 variants)
- five initiative Views in the recipe space
- target binding per deployment variant if used
- summarize what definitely happened, what did not happen, and what still depends on missing infrastructure

## Demonstrate Story 1 — Safe Upgrades

Show me Story 1 from `confighub-aicr-value-add.md` using this example.

Do this in order:
1. Run setup with STACK=ollama
2. Capture the model name in the rag-server-tenant-acme deployment unit
3. Run upgrade-chain.sh with a smaller model (e.g. `llama3.2:1b`)
4. Verify
5. Show the deployment unit again: the model name changed but the per-tenant fields (REGION, TENANT, RAG_TOP_K) didn't

The point is: a shared upgrade propagates while local values are preserved.

## Demonstrate Story 3 — Variant Fleets

Show me Story 3 using this example.

1. Run setup with STACK=ollama (creates `tenant-acme`)
2. Hand-create a `tenant-globex` deployment space, clone all 4 deployment units from the recipe layer, override `REGION=eu-west` and `RAG_TOP_K=10` on rag-server-tenant-globex
3. Run upgrade-chain.sh
4. Inspect both deployment units — show that `tenant-globex`'s overrides survived and that the model bump propagated to both tenants
5. Use `cub unit tree --edge clone` to show the shared base and divergent leaves

## Run The Whole Lifecycle

Guide me through the full `enterprise-rag-blueprint` lifecycle.

Start read-only, then continue through:
- ConfigHub materialization (STACK=ollama)
- initiatives seeding
- shared upgrade-chain propagation
- second-tenant variant
- verify final state
- cleanup
