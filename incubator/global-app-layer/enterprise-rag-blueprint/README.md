# `enterprise-rag-blueprint`

A working ConfigHub example that fuses three things — NVIDIA AICR, NVIDIA Blueprints, and ConfigHub — into one demo on a single Apple Silicon laptop.

## What this is and why

NVIDIA publishes two distinct, complementary packaging models:

### NVIDIA AICR — the cluster-substrate recipe
[AICR](https://developer.nvidia.com/blog/validate-kubernetes-for-gpu-infrastructure-with-layered-reproducible-recipes/) (AI Cluster Runtime) packages **validated GPU Kubernetes substrates** as layered, version-locked recipes. The layers are infrastructure-shaped: `base → platform (eks/gke/aks) → accelerator (h100/a100) → OS (ubuntu/rocky) → workload intent (training/inference) → deploy`. A recipe is a tested combination of cloud, GPU, OS, drivers, and operators — known-good for a given workload. The output is a deployable bundle of operators (gpu-operator, device-plugin) plus the manifests they need. **AICR's job is making the GPU cluster correct.**

### NVIDIA Blueprints — the AI-application reference
[Blueprints](https://build.nvidia.com/blueprints) (~36 today) are the **application-rung counterpart**. Each is a deployable reference for a specific use case — Enterprise RAG, voice agent, agentic commerce, multi-agent warehouse, biomedical research agent — built from NIM microservices, NeMo, and partner components. Blueprints assume a working GPU cluster *underneath* (the AICR layer or equivalent) and focus on app composition: which NIMs to wire together, what model versions, what retrieval params, what guardrails. **Blueprints' job is making the AI app correct.**

The two compose: AICR validates the substrate, Blueprints validate the workload that runs on top. NVIDIA already publishes them with the same layered packaging philosophy at different rungs of the stack.

### ConfigHub + AI — the operational layer NVIDIA doesn't ship
NVIDIA stops at "here is a validated recipe." Real customers then need three things NVIDIA's catalog doesn't address:

1. **Safe upgrades over time.** Recipes age. Model versions move. Guardrails tighten. Customers want shared base updates to flow through their fleet without flattening per-tenant overrides (vector-store endpoints, regional configs, custom prompt templates). NVIDIA gives you the recipe; ConfigHub keeps it current.
2. **Governance and compliance over the recipe.** Recipes ship with policy gaps — pinned-model-version drift, embed/index dimension mismatches, missing GPU resource limits, off-by-default guardrails. ConfigHub initiatives turn those into kanban work that's tracked, owned, and (with `vet-kyverno`) automatically enforced.
3. **Variant fleets.** One Blueprint becomes many real deployments — per-tenant, per-region, per-customer. Each shares the recipe but has local overrides that must survive upstream upgrades. The catalog itself validates this shape: NVIDIA already publishes Enterprise RAG as the upstream `relatedBlueprint` of AI-Q and Biomedical AI-Q (vertical agent variants on top of the same base).

**ConfigHub's role:** turn a static NVIDIA recipe (substrate or app) into a managed, governed, fleet-aware deployment with provenance you can audit and updates you can trust. AI agents drive the workflow through the same CLI surface humans use.

### What this example does
This example materializes NVIDIA's **Build an Enterprise RAG Pipeline** Blueprint — the catalog's canonical RAG reference and upstream parent of AI-Q and Biomedical AI-Q — as a layered ConfigHub recipe. It exercises all three of ConfigHub's value-adds against it (upgrades, governance, fleet variants), and it backs the structural proof with a *real* runtime path that returns a Metal-accelerated answer on an Apple Silicon laptop with no NVIDIA hardware required.

This is the **app rung** of the [`global-app-layer`](../) package. The substrate-rung counterpart is [`gpu-eks-h100-training`](../gpu-eks-h100-training/) (which expresses AICR). Together they cover both layers of the NVIDIA stack.

## The four components

```
rag-server      orchestration pod (Python service, ConfigMap-mounted)   no GPU
nim-llm         answer model    (placeholder for NIM container)         GPU
nim-embedding   embedding model (placeholder for NIM container)         GPU
vector-db       Qdrant or Milvus stand-in                               no GPU
```

Each moves through a 5-stage variant chain — `base → platform=kgpu → accelerator=h100 → profile=medium → recipe=enterprise-rag` — then forks into three deployment variants (`direct` / `flux` / `argo`). 4 components × 5 chain stages + 4 components × 3 deploy variants + 1 recipe-manifest unit = **33 units in 8 spaces**.

## Why this matters — five user-visible benefits

These are the ConfigHub crossover points the example exercises against the NVIDIA Blueprint shape.

### 1. Safe upgrades without flattening ✓✓
Run `./upgrade-chain.sh llama3.2:1b nomic-embed-text-v2`. The bump propagates from the profile layer down through the recipe and into all three deployment variants. Tenant-local values (namespace, region, LLM_HOST override pointing at host Ollama, RAG_TOP_K) survive the propagation. This is the cleanest demo of the upgrade story we have anywhere in the package — RAG specifically is the right surface because model versions, prompt templates, and guardrail policies churn weekly while infrastructure churns yearly.

### 2. GitOps import wedge ✓✓
RAG Helm charts in the wild ship with policy issues on first import — resource limits unset, TLS to the vector DB missing, demo-grade secrets, broad egress. The five initiatives layered over this example (`./seed-initiatives.sh`) are exactly the policy gaps you'd find on import. Run scan, find one concrete issue, decide. The classic "Import → scan → one finding" wedge from `confighub-aicr-value-add.md` lands hard on AI Blueprints because the issues are real.

### 3. Fleet variants ✓✓
The natural variant axes for an AI app are **tenant, region, default LLM, vector-store endpoint, guardrail policy**. The catalog itself validates this is the real shape — NVIDIA already publishes Enterprise RAG as the upstream `relatedBlueprint` of AI-Q and Biomedical AI-Q (vertical agent variants on top of the same base). That is literally the variant-chain pattern, validated by NVIDIA's own catalog. The README's "Adding a second tenant" section walks through it concretely.

### 4. Bundle / attestation ✓
NIM containers are signed; NVIDIA pushes attestation hard; the recipe-manifest unit records every component × every layer × every variant with revision number, dataHash, and `bundleHint`. The supply-chain machinery in [`../04-bundles-attestation-and-todo.md`](../04-bundles-attestation-and-todo.md) applies directly.

### 5. Initiatives over the recipe chain ✓✓
Five Views with Filters cover blueprint-specific compliance (Pin Model Versions, Embed/Index Dim Match, GPU Resource Limits, Guardrail Policy Required, Resource Limits Enforcement). Filters span all 8 spaces of the run, so each initiative covers direct + Flux + Argo variants of every relevant component. This is the first example in the package where the [`initiatives-demo`](../../../initiatives-demo/) primitive cohabits with a layered recipe chain.

(Crossover miss: partner-publisher precedent — Enterprise RAG is NVIDIA-published, not partner. Acceptable; that crossover is a distribution-channel point, not an example-shape one.)

## Quick start (Ollama path on Apple Silicon)

```bash
# Prerequisites
brew install ollama jq kind kubectl
ollama serve >/tmp/ollama.log 2>&1 &
ollama pull llama3.2:3b
ollama pull nomic-embed-text
cub auth login

cd incubator/global-app-layer/enterprise-rag-blueprint

# 1. Plan-only preview (no mutation)
STACK=ollama ./setup.sh --explain
STACK=ollama ./setup.sh --explain-json | jq

# 2. Materialize the chain in ConfigHub
STACK=ollama ./setup.sh
./verify.sh

# 3. Layer five initiatives over the chain
./seed-initiatives.sh    # prints View Explorer URLs

# 4. Bring up cluster + worker + bind direct variant
kind create cluster --name rag
source .state/state.env
deploy="${PREFIX}-deploy-tenant-acme"

cub worker create --space "${deploy}" rag-worker
cub worker run    --space "${deploy}" rag-worker -t kubernetes -d
sleep 5
target_ref=$(cub target list --space "${deploy}" -o json \
  | jq -r '.[] | select(.Target.Slug | endswith("kind-rag")) | "\(.Space.Slug)/\(.Target.Slug)"')
./set-target.sh "${target_ref}"

# 5. Apply + run a real RAG query
kubectl --context kind-rag create namespace tenant-acme
for u in vector-db-tenant-acme nim-embedding-tenant-acme nim-llm-tenant-acme rag-server-tenant-acme; do
  cub unit approve --space "${deploy}" "${u}"
  cub unit apply   --space "${deploy}" "${u}"
done
kubectl --context kind-rag -n tenant-acme rollout status deploy/rag-server

./query.sh "What is the capital of France?"
```

Expected `query.sh` output:

```json
{
  "query": "What is the capital of France?",
  "answer": "The capital of France is Paris.",
  "tenant": "tenant-acme",
  "model": "llama3.2:3b",
  "stack": "ollama"
}
```

That answer was generated by `llama3.2:3b` on the host's Metal GPU, called from the in-cluster `rag-server` pod via `host.docker.internal:11434`. Every value in the response (model name, tenant, stack) traces back through the recipe chain to a layer mutation in ConfigHub.

## What to look for once it's running

Use this checklist after `setup.sh + seed-initiatives.sh + apply` to verify each part of the demo in both GUI and CLI. Replace `<prefix>` with the value in `.state/state.env`'s `PREFIX`.

### Point of interest 1: the variant chain (substrate of the demo)

**See it in CLI:**
```bash
cub unit tree --edge clone --space "*" \
  --where "Labels.ExampleChain = '$(source .state/state.env && echo $PREFIX)'"
```
Shows each of the 4 components walking from `base → kgpu → h100 → medium → enterprise-rag` and forking into 3 deploy variants. The `direct` leaf for `tenant-acme` shows `Ready`; Flux and Argo show `NotLive` (no target bound).

**See it in GUI:** open the Recipe space — table view shows all 4 chain leaves side by side with their revisions and labels.

### Point of interest 2: the recipe-manifest unit (the receipt)

**CLI:**
```bash
cub unit data --space "$(source .state/state.env && echo ${PREFIX}-recipe-enterprise-rag)" \
  recipe-enterprise-rag-stack | head -50
```
Shows the rendered Recipe YAML with every component × every chain stage × every deployment variant captured with revision number, dataHash, and bundleHint. This is the full provenance receipt for the chain.

**GUI:** the recipe-space link printed by `setup.sh` (and `set-target.sh` after binding) takes you to the unit page where you can see the rendered manifest, its labels, and its history.

### Point of interest 3: the five initiatives (governance)

`seed-initiatives.sh` prints these. Each link opens the View Explorer with the units that match.

| Initiative | Priority | Filter | Matches |
|---|---|---|---|
| Pin Model Versions | HIGH | `GPUUser=true` | 10 (2 GPU components × 5 chain stages) |
| Embedding/Index Dim Match | HIGH | `Layer=profile` | 4 (one per component at the profile layer) |
| GPU Resource Limits | HIGH | `GPUUser=true AND Layer=deployment` | 6 (2 GPU components × 3 deploy variants) |
| Guardrail Policy Required | MEDIUM | `Component=rag-server AND Layer=deployment` | 3 (rag-server × 3 deploy variants) |
| Resource Limits Enforcement | MEDIUM | `Layer=deployment` | 12 (4 components × 3 deploy variants) |

The View Explorer URL pattern is `https://<server>/x/view-explorer?view=<ViewID>`. Filters span all 8 spaces of the run, so a single initiative covers direct + Flux + Argo variants.

**CLI to fetch URLs after the fact:**
```bash
source .state/state.env
for slug in pin-model-versions embed-index-dim-match gpu-resource-limits guardrail-policy-required resource-limits-enforcement; do
  vid=$(cub view get --space "${PREFIX}-recipe-enterprise-rag" -o json "${slug}" | jq -r '.View.ViewID')
  echo "${slug}: $(cub context get --quiet 2>/dev/null)/x/view-explorer?view=${vid}"
done
```

### Point of interest 4: the live cluster (real runtime)

**Cluster overview** (cub-scout):
```bash
cub-scout doctor
cub-scout map list --namespace tenant-acme
cub-scout tree ownership --namespace tenant-acme
```
Confirms 9 of 10 resources in `tenant-acme` are owned by ConfigHub (only `kube-root-ca.crt` is Native), and each running deployment traces back to a ConfigHub unit by name.

**Deep trace of one running pod:**
```bash
cub-scout explain deployment/rag-server --namespace tenant-acme
kubectl --context kind-rag -n tenant-acme get pods -o wide
kubectl --context kind-rag -n tenant-acme logs deploy/rag-server | tail -20
```

### Point of interest 5: the runtime path (config-driven)

```bash
./query.sh                                    # default question
./query.sh "Summarise the Hawaiian language."
```
Health endpoint dumps the env vars (`MODEL_NAME`, `LLM_HOST`, `EMBED_MODEL_NAME`, `RAG_TOP_K`, `PROMPT_TEMPLATE`, `GUARDRAIL_POLICY`) — every one set by a layer mutation in ConfigHub, then propagated to the deployment unit, then mounted as pod env. Answer endpoint calls host Ollama and returns a real Metal-accelerated answer.

### Point of interest 6: upgrade propagation (Story 1 in motion)

```bash
./upgrade-chain.sh llama3.2:1b nomic-embed-text
./verify.sh
cub unit data --space "$(source .state/state.env && echo ${PREFIX}-deploy-tenant-acme)" \
  rag-server-tenant-acme | grep -E "MODEL_NAME|LLM_HOST|TENANT|REGION"
```
You should see `MODEL_NAME` updated to `llama3.2:1b` (propagated) but `LLM_HOST=host.docker.internal:11434`, `TENANT=tenant-acme`, `REGION=us-east` unchanged (preserved local values).

Re-apply to push the change to the running pod:
```bash
cub unit apply --space "$(source .state/state.env && echo ${PREFIX}-deploy-tenant-acme)" rag-server-tenant-acme
./query.sh; # /health now shows the new model
```

## STACK selector (the runtime story)

The recipe chain is identical across stacks; only profile-layer image refs and deployment-layer endpoint overrides differ.

| `STACK` | Cluster | LLM/embedding | Vector DB | Use when |
|---|---|---|---|---|
| `stub` (default) | any | nginx/busybox stubs | busybox | structural proof, no inference |
| `ollama` | kind on Docker Desktop | host Ollama (Metal GPU) via `host.docker.internal` | qdrant in-cluster | real runtime on Apple Silicon |
| `nim` | x86_64 + NVIDIA + NGC creds | real NIM containers in-cluster | milvus in-cluster | faithful Blueprint deploy |

### Why these models on the Ollama path

[Ollama](https://ollama.com) is a local model runtime that uses Apple's Metal GPU on M-series Macs (and CUDA elsewhere). It exposes an HTTP API on `localhost:11434` and is the simplest way to run real LLM inference on a laptop without NVIDIA hardware.

For the Ollama path this example uses:

- **`llama3.2:3b`** as the answer LLM. Llama 3.2 is Meta's open-weight family — same lineage as the `llama-3.1-70b-instruct` NIM that the canonical NVIDIA Enterprise RAG Blueprint specifies, just at 3 billion parameters instead of 70 billion. That keeps the model under 2 GB on disk, loads in seconds, and runs at conversational latency on M-series Metal. It's a faithful stand-in: same model family, same Llama instruction-tuning, smaller hardware footprint. The full 70B variant is what STACK=nim points to.
- **`nomic-embed-text`** as the embedding model. 768-dimensional, 274 MB, fast on Metal. Stand-in for `nv-embedqa-e5-v5` (which is what the production NIM blueprint uses). The `EMBED_DIM` env var ConfigHub sets at the profile layer is what keeps the embedder and the vector store consistent — swap `nomic-embed-text` for any 768-dim embedder and the recipe still works; swap to a 1024-dim embedder and the **Embedding/Index Dim Match** initiative starts flagging.

You can swap the model with one line at the profile layer:

```bash
./upgrade-chain.sh llama3.2:1b      # smaller, faster
./upgrade-chain.sh qwen2.5:7b       # different family
```

The point isn't which exact model — it's that the model name is a config value flowing through the chain, and changing it propagates correctly to running pods without flattening tenant-local overrides.

### NIM stack notes

For STACK=nim, manually edit profile-layer units to point at `nvcr.io/nim/...` images before applying — `cub function set-image-reference` only accepts `:tag` or `@digest`, not full image refs.

## Files

| File | Purpose |
|---|---|
| `lib.sh` | Shared functions, constants, layer mutations, STACK selector |
| `setup.sh` | Materialize the chain. `--explain` / `--explain-json` for plan-only |
| `verify.sh` | Structural checks. `--json` for machine-readable |
| `cleanup.sh` | Delete all spaces for this prefix |
| `upgrade-chain.sh` | Bump model versions at the profile layer; propagate through the chain |
| `set-target.sh` | Bind targets after setup; routed by provider type |
| `seed-initiatives.sh` | Create 5 Views with Filters covering blueprint compliance |
| `query.sh` | Live runtime test (STACK=ollama only). Real query → real Metal answer |
| `recipe.base.yaml` | Templated recipe manifest, rendered by `lib.sh` with revision/hash provenance |
| `*.base.yaml` | Per-component K8s manifests with stub-friendly defaults |

## Per-stage layer mutations

| Stage | What gets set |
|---|---|
| `platform=kgpu` | `STORAGE_CLASS=gp3`, `INGRESS_CLASS=alb` (rag-server), `STORAGE_SIZE=200Gi` (vector-db) |
| `accelerator=h100` | `nvidia.com/gpu` resource request + `NODE_SELECTOR=nvidia-h100` + `GPU_MEMORY` on nim-llm/nim-embedding; label-only on rag-server/vector-db |
| `profile=medium` | `MODEL_NAME`, `MODEL_TAG`, `EMBED_MODEL_NAME`, `EMBED_DIM`, `MAX_BATCH_SIZE`, `STACK` env |
| `recipe=enterprise-rag` | Wire `LLM_HOST`, `EMBEDDING_HOST`, `VECTOR_DB_HOST`, `RAG_TOP_K=5`, `PROMPT_TEMPLATE=enterprise-default`, `GUARDRAIL_POLICY=enterprise-default`, `RAG_USE_CASE=enterprise-rag` |
| `deployment` | `namespace`, `TENANT`, `REGION`, `CLUSTER`. STACK=ollama: re-points rag-server's `LLM_HOST` to `host.docker.internal:11434` (the per-tenant override that proves Story 3) |

## Adding a second tenant (Story 3 made concrete)

```bash
source .state/state.env
deploy_b="${PREFIX}-deploy-tenant-globex"

cub space create "${deploy_b}" \
  --label "ExampleName=global-app-layer-enterprise-rag-blueprint" \
  --label "ExampleChain=${PREFIX}" \
  --label "Tenant=tenant-globex"

for c in rag-server nim-llm nim-embedding vector-db; do
  cub unit create --space "${deploy_b}" "${c}-tenant-globex" \
    --upstream-unit "${c}-kgpu-h100-medium-enterprise-rag" \
    --upstream-space "${PREFIX}-recipe-enterprise-rag"
  cub function do set-namespace tenant-globex --space "${deploy_b}" --unit "${c}-tenant-globex"
done

# Per-tenant overrides that should survive shared upgrades:
cub function do set-env rag-server "RAG_TOP_K=10" --space "${deploy_b}" --unit "rag-server-tenant-globex"
cub function do set-env rag-server "REGION=eu-west" --space "${deploy_b}" --unit "rag-server-tenant-globex"

# Now bump the shared base — both tenants pick it up but globex's overrides survive.
./upgrade-chain.sh llama3.2:1b
```

## Functional vs structural

The **structure** (recipe chain, initiatives, upgrade propagation, target routing) is real on any cluster.

The **runtime** is real on three different hardware substrates:

- `STACK=stub` → real cluster, stub pods that just exist (proves apply path works)
- `STACK=ollama` → real cluster, real Python rag-server, real Metal-accelerated LLM via host bridge
- `STACK=nim` → real cluster, real NIM containers (requires NVIDIA + NGC creds)

Same recipe; the deployment-layer override (`LLM_HOST`) is what differs.

## Cleanup

```bash
./cleanup.sh                       # removes all 8 spaces, ~33 units, 5 Views, 1 Filter
kind delete cluster --name rag     # removes local cluster
cub worker stop rag-worker         # stops daemon worker (if still running)
```

## Related reading

- [`../02-nvidia-blueprints-fit.md`](../02-nvidia-blueprints-fit.md) — fit analysis (this example is what it specifies)
- [`../01-nvidia-aicr-fit.md`](../01-nvidia-aicr-fit.md) — substrate-rung counterpart fit
- [`../gpu-eks-h100-training/`](../gpu-eks-h100-training/) — substrate-rung worked example
- [`../../../initiatives-demo/`](../../../initiatives-demo/) — initiatives primitive (with `vet-kyverno` triggers)
- [`../confighub-aicr-value-add.md`](../confighub-aicr-value-add.md) — the three stories this example exercises
- [`../04-bundles-attestation-and-todo.md`](../04-bundles-attestation-and-todo.md) — bundle/attestation gaps that apply identically here
