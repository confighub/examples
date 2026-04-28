# `enterprise-rag-blueprint`

## What this is

A working end-to-end demo of one of NVIDIA's standard NIM Blueprints for AI applications — *Build an Enterprise RAG Pipeline* — running under ConfigHub.

RAG ("retrieval-augmented generation") is the standard pattern for an enterprise chatbot: when you ask a question, the application first looks things up in your private data, and then asks a large language model to write the answer using what it found.

The demo runs in two places:

- on a Mac, using a small local language model through [Ollama](https://ollama.com)
- on a real GPU cluster, using NVIDIA's official NIM containers

It's the same recipe in both cases. Only the image references and one endpoint change between them.

A companion example in this same package, [`gpu-eks-h100-training`](../gpu-eks-h100-training/), does the equivalent thing one rung lower in NVIDIA's stack: it expresses an **AICR** ("AI Cluster Runtime") recipe — NVIDIA's recipe format for the GPU Kubernetes substrate (operators, drivers, OS, accelerator profile). AICR makes the cluster correct; a NIM Blueprint makes the application correct on top. ConfigHub manages both, with the same primitives.

## Why to use ConfigHub for your AI stacks

1. **Easy to create and manage many custom configs in fleet deployments.** Real customers run a copy per tenant, region, or team, each with small differences. NVIDIA's catalog already shows this (AI-Q is a variant of Enterprise RAG for research agents); ConfigHub handles it directly instead of forking Helm charts.

2. **Updates from NVIDIA don't wipe your customers' settings.** When NVIDIA ships a new NIM version, the customer's per-tenant settings (vector-store URL, region, prompt template, retrieval count) stay put. No re-merging changes every release.

3. **Standard compliance settings.** Five compliance checks come pre-built: model versions are pinned, the embedder and vector store agree on dimensions, GPU limits are set, the rag-server has a guardrail policy, every component has resource limits.

4. **A receipt of what's actually deployed.** ConfigHub records the applied revision and content hash of every component, plus the target it's bound to. Useful for audits and regulated-industry reviews.

5. **Developer and demo friendly footprint.** Everything works on a MacBook through Ollama: setup, compliance checks, even a live RAG query that returns a real answer. The same recipe targets real NIM containers when you have a GPU cluster.

6. **Direct deploy or use CNCF GitOps tools.** The same recipe produces outputs for direct apply, Flux, or Argo. NVIDIA doesn't have to recommend one tool over another, and customers keep their existing pipeline.

**When not to recommend it.** One Blueprint, one tenant, no governance to worry about, the published Helm chart works fine — ConfigHub is overkill.

## How this works

NVIDIA publishes Enterprise RAG as a [**NIM Blueprint**](https://build.nvidia.com/blueprints) — a public, versioned recipe that says which AI models to use, how to wire them together, and what default settings to apply. NIM ("NVIDIA Inference Microservices") is NVIDIA's container format for serving models behind a uniform API. There are about 36 NIM Blueprints in the catalog today; Enterprise RAG is one of the most central, and is the published parent of several others (AI-Q and Biomedical AI-Q both list it as their `relatedBlueprint`).

CONFIGHUB is a system of record for managing your configs and organising them into application models. Locating ConfigHub in between source code (in Git) and live production (eg K8s, GitOps) lets a user see live operational truth, verify it, and operate on it directly. This example takes the Blueprint and breaks it into its layers — base, platform, accelerator, model profile, use case, deployment — and stores each layer as a versioned ConfigHub config object. Each of the four components walks through the same chain of layers; three delivery variants (direct, Flux OCI, Argo OCI) come off the recipe layer.

This works out of the box with real models. In this example: A query through the deployed pipeline returns a real answer from your model. On a Mac that could be eg. `llama3.2:3b` running on the Metal GPU on the host, called from the in-cluster `rag-server` pod through `host.docker.internal`. On a CUDA cluster it's `llama-3.1-70b-instruct` served from a real NIM container.

## Components

ConfigHub manages dependencies and layers for you, so that any Blueprint component may be customised and varied independently. Each component moves through a five-stage chain — `base → platform=kgpu → accelerator=h100 → profile=medium → recipe=enterprise-rag` — and then forks into three delivery variants (`direct`, `flux`, `argo`). That works out to 4 × 5 chain units + 4 × 3 deployment units + 1 recipe-manifest unit = **33 units across 8 ConfigHub spaces**.

| Component | Role | Uses GPU? |
|---|---|---|
| `rag-server` | orchestration pod that calls the LLM and embedder | no |
| `nim-llm` | answer model — a NIM container in production, a stub in the demo | yes |
| `nim-embedding` | embedding model, same shape | yes |
| `vector-db` | Qdrant or Milvus stand-in | no |

## Quick start (Ollama path on Apple Silicon)

First, log in to ConfigHub by running `cub auth login` at your command line. Then follow the steps below — and keep [hub.confighub.com](https://hub.confighub.com) open in a browser tab so you can watch the spaces and units appear as each step runs.

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

## What to look at once it's running

After `setup.sh`, `seed-initiatives.sh`, and the apply step, here's what's worth checking and where to find it. Replace `<prefix>` below with the value in `.state/state.env`'s `PREFIX`.

### The variant chain

```bash
cub unit tree --edge clone --space "*" \
  --where "Labels.ExampleChain = '$(source .state/state.env && echo $PREFIX)'"
```
Each of the four components shows up walking from `base → kgpu → h100 → medium → enterprise-rag` and forking into the three deploy variants. The `direct` leaf for `tenant-acme` is `Ready`; Flux and Argo are `NotLive` (no target bound).

In the GUI, open the recipe space — the table shows all four chain leaves side by side with revisions and labels.

### The recipe-manifest unit

```bash
cub unit data --space "$(source .state/state.env && echo ${PREFIX}-recipe-enterprise-rag)" \
  recipe-enterprise-rag-stack | head -50
```
The rendered manifest captures every component × every chain stage × every deployment variant with revision number, dataHash, and bundleHint per delivery variant.

In the GUI, the recipe-space link `setup.sh` prints (and `set-target.sh` re-prints after binding) takes you to the unit page with the rendered YAML and its history.

### The five initiatives

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

### The live cluster

`cub-scout` reads the cluster and shows what owns what:

```bash
cub-scout doctor
cub-scout map list --namespace tenant-acme
cub-scout tree ownership --namespace tenant-acme
```
Of the 10 resources in `tenant-acme`, nine are owned by ConfigHub; `kube-root-ca.crt` is the only Native one (Kubernetes auto-creates it). Each running deployment traces back to a ConfigHub unit by name.

To trace one specific pod:
```bash
cub-scout explain deployment/rag-server --namespace tenant-acme
kubectl --context kind-rag -n tenant-acme get pods -o wide
kubectl --context kind-rag -n tenant-acme logs deploy/rag-server | tail -20
```

### The runtime path

```bash
./query.sh                                    # default question
./query.sh "Summarise the Hawaiian language."
```
The script port-forwards to `rag-server` and hits two endpoints. `/health` returns the env vars it's running with — `MODEL_NAME`, `LLM_HOST`, `EMBED_MODEL_NAME`, `RAG_TOP_K`, `PROMPT_TEMPLATE`, `GUARDRAIL_POLICY` — and every one of them was set by a layer mutation in ConfigHub. `/answer` calls host Ollama and returns the result.

### Upgrade propagation

```bash
./upgrade-chain.sh llama3.2:1b nomic-embed-text
./verify.sh
cub unit data --space "$(source .state/state.env && echo ${PREFIX}-deploy-tenant-acme)" \
  rag-server-tenant-acme | grep -E "MODEL_NAME|LLM_HOST|TENANT|REGION"
```
`MODEL_NAME` is now `llama3.2:1b` (propagated through the chain). `LLM_HOST=host.docker.internal:11434`, `TENANT=tenant-acme`, and `REGION=us-east` are unchanged — those are the deployment-layer overrides that should survive an upstream bump.

To push the new value to the running pod:
```bash
cub unit apply --space "$(source .state/state.env && echo ${PREFIX}-deploy-tenant-acme)" rag-server-tenant-acme
./query.sh    # /health now shows the new model
```

## STACK selector

The recipe chain is identical across the three stacks; only the profile-layer image refs and deployment-layer endpoint overrides differ.

| `STACK` | Cluster | LLM/embedding | Vector DB | Use when |
|---|---|---|---|---|
| `stub` (default) | any | nginx/busybox stubs | busybox | structural proof, no inference |
| `ollama` | kind on Docker Desktop | host Ollama (Metal GPU) via `host.docker.internal` | qdrant in-cluster | real runtime on Apple Silicon |
| `nim` | x86_64 + NVIDIA + NGC creds | real NIM containers in-cluster | milvus in-cluster | faithful NIM Blueprint deploy |

### Why these models on the Ollama path

[Ollama](https://ollama.com) is a local model runtime that uses Apple's Metal GPU on M-series Macs (and CUDA elsewhere). It exposes an HTTP API on `localhost:11434` and is the simplest way to run real LLM inference on a laptop without NVIDIA hardware.

For the Ollama path this example uses:

- **`llama3.2:3b`** as the answer LLM. Llama 3.2 is Meta's open-weight family — same lineage as the `llama-3.1-70b-instruct` NIM that the canonical NVIDIA Enterprise RAG NIM Blueprint specifies, just at 3 billion parameters instead of 70 billion. That keeps the model under 2 GB on disk, loads in seconds, and runs at conversational latency on M-series Metal. It's a faithful stand-in: same model family, same Llama instruction-tuning, smaller hardware footprint. The full 70B variant is what STACK=nim points to.
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
