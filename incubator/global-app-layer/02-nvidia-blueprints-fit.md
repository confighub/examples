# NVIDIA NIM Blueprints and ConfigHub Fit

## Purpose

This note answers a narrow question: can current ConfigHub model the kind of workflow NVIDIA describes in its [Blueprints catalog](https://build.nvidia.com/blueprints), where AI applications (RAG, agents, voice, vertical workflows) are packaged as layered, validated, reproducible reference apps that run on top of a GPU cluster?

Short answer: yes, and more naturally than AICR. The recipe-chain model fits, the variant-fleet story is sharper at the app layer than at the cluster layer, and pairing it with ConfigHub initiatives turns the example into a real combined story rather than a parallel demo.

## What NVIDIA NIM Blueprints Are

NVIDIA NIM Blueprints are **application-level** reference packages, sister to AICR but at a different rung of the stack:

| | AICR | NIM Blueprints |
|---|---|---|
| Layer | GPU cluster substrate | AI application running on top |
| Inputs | platform · accelerator · OS · intent | use-case · publisher · components (NIMs/NeMo/partner) |
| Output | validated K8s + drivers + operators | deployable app reference (Helm/Launchable) |
| Example | `eks + h100 + ubuntu + training` | `Build an Enterprise RAG Pipeline`, `Nemotron Voice Agent`, `Cyborg Enterprise RAG` |

Same packaging philosophy at both layers — layered, version-locked, validated, reproducible, with deployable bundles.

The catalog has roughly 36 NIM Blueprints. NVIDIA publishes most; partners (Weights & Biases, Iguazio, Cyborg, H2O.ai, LangChain, CrewAI, Pipecat, Viavi) publish nine. The catalog already accepts third-party recipes — there is a precedent slot for ConfigHub-flavoured publications if positioning ever calls for it.

Relevant source material:

- NVIDIA NIM Blueprints catalog: https://build.nvidia.com/blueprints
- Build an Enterprise RAG Pipeline (the chosen example): one of the most central NIM Blueprints, listed as the upstream `relatedBlueprint` for AI-Q and Biomedical AI-Q Research Agent — i.e. NVIDIA already publishes Enterprise RAG as a base with vertical variants on top, which is structurally our variant-chain pattern.

## Why This Fits ConfigHub

ConfigHub already has the primitives needed to model a NIM Blueprint as a recipe chain:

- units hold component manifests
- clones and upstream links model layered specialization
- workers and targets give an execution path to real clusters
- functions mutate at each layer (model versions, GPU resources, retrieval params, tenant overrides)
- revisions and hashes give provenance and reproducibility

A NIM Blueprint is a recipe chain at the app rung. The same pattern that `gpu-eks-h100-training/` proves at the substrate rung extends one layer up.

## The Crossover With Initiatives

This is the part that AICR fit does not cover and NIM Blueprints do.

ConfigHub initiatives (see [`../../initiatives-demo/`](../../initiatives-demo/)) are Views with a filter, initiative metadata (priority / status / deadline), and an optional `vet-kyverno` Trigger that runs a Kyverno CEL policy against matched units. They are the governance layer over a set of related units.

NIM Blueprints have natural compliance concerns that map almost one-to-one onto initiatives:

| Initiative | Priority | Filter | What it checks |
|---|---|---|---|
| Pin Model Versions | HIGH | `Component in (nim-llm, nim-embedding, vector-db)` | no `:latest` image tags; pinned model refs |
| Embed/Index Dimension Match | HIGH | `Layer = profile or recipe` | `EMBED_DIM` on `nim-embedding` equals `EMBED_DIM` on `vector-db` |
| GPU Resource Limits | HIGH | `GPUUser = true` | `nvidia.com/gpu` resource request and memory limit are set on every GPU-using pod |
| Guardrail Policy Required | MEDIUM | `Component = rag-server` | `GUARDRAIL_POLICY` env is set and not `off` on every deployment variant |
| Resource Limits Enforcement | MEDIUM | `Layer = deployment` | every deployment pod declares CPU/memory requests and limits |

These are exactly the kind of policy gaps the Story 2 import wedge (in [`confighub-aicr-value-add.md`](./confighub-aicr-value-add.md)) is designed to find. Layering them on top of the recipe chain turns the example from "a recipe materializes in ConfigHub" into "a recipe materializes, the initiatives find the gaps, the upgrade flow respects both shared and tenant-local values, and the Views update as units change."

That is the full story we have been describing across the AICR fit, the value-add doc, and the existing examples — finally combined into one example.

## Runtime Story (M5 Max-Friendly)

The example supports three runtime paths so it is honest about what actually executes where. They share the same recipe chain; only the deployment-variant layer differs.

| Path | Selector | Cluster | LLM/Embedding | Vector DB | Use when |
|---|---|---|---|---|---|
| Stub | `STACK=stub` | k3d / kind / any | nginx/busybox stubs | stub | No inference. Structural proof only. Same approach as `gpu-eks-h100-training/`. |
| Ollama | `STACK=ollama` (default on macOS) | k3d on Docker Desktop | Ollama running natively on host (Metal GPU); rag-server reaches it via `host.docker.internal` | Qdrant or Milvus in-cluster (CPU is fine) | Real runtime on Apple Silicon. Real query/response. Metal GPU acceleration. |
| NIM | `STACK=nim` | x86_64 + NVIDIA GPU + NGC creds | real NIM containers in-cluster | Milvus in-cluster | Faithful NIM Blueprint deploy. Requires the right hardware. |

The Ollama path is the one that makes this example useful on the M5 Max. The LLM endpoint becomes a **deployment-layer config value** (`LLM_HOST=host.docker.internal:11434` for Ollama; `LLM_HOST=nim-llm.<ns>.svc.cluster.local:8000` for NIM), which is exactly the kind of tenant- or environment-local override the variant chain is supposed to prove.

The structural recipe — base, platform, accelerator, profile, recipe, deployment — is identical across all three paths.

## Worked Example: `enterprise-rag-blueprint/`

The recommended worked example mirrors `gpu-eks-h100-training/` one rung up.

Components (4):

- `rag-server` — orchestration pod, no GPU
- `nim-llm` — answer model
- `nim-embedding` — embedding model
- `vector-db` — Milvus or Qdrant stand-in

Layers (5, mirroring the AICR shape but with `profile` replacing `os`):

```
base -> platform(kgpu) -> accelerator(h100) -> profile(medium) -> recipe(enterprise-rag) -> deployment(tenant-acme)
                                                                                          -> deployment(tenant-globex)
```

Layer mutations per component (sketched):

- **platform** (`kgpu`): set `STORAGE_CLASS`, `INGRESS_CLASS`, GPU node selectors at the app level
- **accelerator** (`h100`): set `nvidia.com/gpu` resource requests on GPU-using pods, no-op on others
- **profile** (`medium`): set `MODEL_NAME`, `MODEL_TAG`, `EMBED_MODEL_NAME`, `EMBED_DIM`. The `STACK` selector lives here: `STACK=stub` uses stubs, `STACK=ollama` uses `ollama/ollama` and `nomic-embed-text`, `STACK=nim` uses `nvcr.io/nim/...`.
- **recipe** (`enterprise-rag`): wire the components together — `LLM_HOST`, `EMBEDDING_HOST`, `VECTOR_DB_HOST`, `RAG_TOP_K`, `PROMPT_TEMPLATE`, `GUARDRAIL_POLICY`
- **deployment** (`tenant-acme`): per-tenant namespace, vector-store endpoint override, region, optional LLM override

Same `setup.sh / verify.sh / cleanup.sh / upgrade-chain.sh / set-target.sh` contract as the existing examples, plus:

- `seed-initiatives.sh` — create the five Views + Triggers over the deployment-layer units
- `query.sh` (Ollama path only) — fire a sample RAG query through the deployed rag-server and print the answer

Crossover hits:

1. **Safe upgrades without flattening (Story 1)** — base bump = embedding model `nv-embedqa-e5-v5` to `v6`; tenant `top_k` and vector-store endpoint stay local.
2. **GitOps import wedge (Story 2)** — RAG Helm charts ship with policy issues. Import → scan → initiative finds it.
3. **Fleet variants (Story 3)** — `tenant-acme` and `tenant-globex` share the recipe; differ at the deployment leaf. Catalog already validates this shape (Enterprise RAG → AI-Q → Biomedical AI-Q).
4. **Bundle/attestation** — applies identically to NIM and Ollama image refs.
5. **Initiatives** — five Views with filters and triggers covering the deployment-layer units.

Crossover miss: partner-publisher precedent. Acceptable — that is a distribution-channel point, not an example-shape point.

## Where This Pushes ConfigHub

The example exercises three things the package has not had a single demo of together before:

1. **App-layer recipe chain** — proves the AICR-shaped pattern works one rung up, with components that have very different GPU profiles (rag-server: none; nim-llm: lots; vector-db: storage-heavy).
2. **Initiatives over a recipe chain** — proves Views with filters and Kyverno triggers compose with variant-chain provenance. Today the initiatives demo and the global-app-layer examples are siblings; this is the first example where they cohabit.
3. **Honest runtime story on heterogeneous hardware** — three `STACK` paths covering structural-only, Apple Silicon Metal, and CUDA + NIM. Same recipe, three deployment-variant flavours.

## What's Missing (Same Gaps As AICR)

The gaps called out in [`01-nvidia-aicr-fit.md`](./01-nvidia-aicr-fit.md) apply identically here:

1. First-class snapshot flow
2. First-class recipe catalog UX
3. First-class phased validation
4. Bundle, integrity, SBOM, attestation evidence (see [`04-bundles-attestation-and-todo.md`](./04-bundles-attestation-and-todo.md))

Adding a NIM-Blueprint-shaped example does not widen any of these. It does sharpen the case for them, because the app-layer recipe is where users will most want a recipe-browser UX and a phased validation lifecycle.

## Recommended Position

Same as AICR: do not copy the catalog, adopt the useful pattern.

- recipe chain = ordered variant chain of units (same as substrate)
- deployment = final tenant- or environment-specific clone
- bundle = published deployable output of the deployment target
- initiatives = Views over the deployment layer that turn compliance gaps into kanban work
- runtime = `STACK` selector at the deployment layer; structural / Ollama / NIM are three valid endpoints of the same chain

## Bottom Line

NIM Blueprints are useful for ConfigHub because they validate four ideas the package already pushes:

- layered recipes are the right shape at the app rung as well as the cluster rung
- compliance over a recipe chain is a first-class concern, not an afterthought
- variant fleets at the deployment leaf are how real customers run AI apps
- runtime honesty matters: the same recipe should support stubs, laptop hardware, and production GPUs without changing the chain

`enterprise-rag-blueprint/` is the worked example that exercises all four. It pairs naturally with `gpu-eks-h100-training/` (the substrate rung) and with `initiatives-demo/` (the governance primitive), and it is the first example where the AICR-shape, the NIM-Blueprint-shape, and the initiatives-shape cohabit.
