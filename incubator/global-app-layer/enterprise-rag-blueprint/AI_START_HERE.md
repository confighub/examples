# AI Start Here: enterprise-rag-blueprint

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen
3. Explain what the output means in plain English
4. If there is a GUI URL, print it
5. STOP and ask "Ready to continue?"
6. Only proceed when the human says to continue

## Suggested Prompt

```text
Read incubator/global-app-layer/enterprise-rag-blueprint/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Teaches

This is the app-layer counterpart to `gpu-eks-h100-training/`: NVIDIA's Enterprise RAG Blueprint expressed as a layered ConfigHub recipe with three deployment variants and five compliance initiatives. After the demo, the human will understand:

- The layered Blueprint shape: base → platform → accelerator → profile → recipe → deployment
- Four components moving together through the chain (rag-server, nim-llm, nim-embedding, vector-db)
- Three deployment variants (direct, Flux, Argo) — same as the substrate rung
- Five initiatives that turn blueprint compliance into kanban work
- A real runtime path on Apple Silicon (Ollama + Metal GPU on the host, in-cluster pods talk to it)
- The STACK selector that makes the same recipe work for stub, Ollama, or real NIM

## Prerequisites

- `cub` in PATH and authenticated
- `jq` for JSON preview and parsing
- `kind` and `docker` for the runtime path
- `kubectl` for the runtime path
- `ollama` running on the host with `llama3.2:3b` and `nomic-embed-text` pulled (STACK=ollama only)

---

## Stage 1: "Check Capabilities" (read-only)

Run:

```bash
cd incubator/global-app-layer/enterprise-rag-blueprint
which cub jq kind kubectl ollama
cub version
cub context list --json | jq
ollama list
curl -fsS http://localhost:11434/api/tags >/dev/null && echo "Ollama reachable"
```

What to explain:

- If `cub` auth fails, stay in preview mode (only `--explain` will work).
- If `ollama` is missing or has no models, the runtime path won't work — fall back to STACK=stub for the rest of the demo.
- If `kind` or `kubectl` is missing, the demo can still materialize the chain in ConfigHub; only the live-apply and query stages need them.

GUI now: No GUI checkpoint for this stage.

**PAUSE.** Wait for the human.

---

## Stage 2: "Preview The Recipe" (read-only)

Run:

```bash
STACK=ollama ./setup.sh --explain
STACK=ollama ./setup.sh --explain-json | jq '{example, stack, prefix, namespace, spaces: (.spaces | length), components: ([.components[].component])}'
```

What to explain:

- Eight spaces will be created (base → platform → accelerator → profile → recipe → 3 deploy spaces).
- Four component chains (rag-server, nim-llm, nim-embedding, vector-db) — same shape as `realistic-app/` plus one more component.
- Three deployment variants at the leaf: direct, flux, argo.
- The STACK selector chooses image refs at the profile layer.

**PAUSE.** Wait for the human.

---

## Stage 3: "Materialize In ConfigHub" (mutates ConfigHub)

Ask: "This will create 8 spaces, ~60 units, and three deployment variants. Ready to proceed?"

Run:

```bash
STACK=ollama ./setup.sh
```

What to explain:

- Spaces and units are now in ConfigHub.
- Each layer's mutations are visible in the unit data.
- The recipe-manifest unit (`recipe-enterprise-rag-stack`) records the full chain with revisions.
- Three deployment variants exist; binding to a target is a separate explicit step.

GUI now: Open the printed Recipe space URL and the Direct deploy space URL.

**PAUSE.** Wait for the human.

---

## Stage 4: "Verify The Structure" (read-only)

Run:

```bash
./verify.sh
./verify.sh --json | jq '{ok, prefix, stack, spacesChecked: (.spacesChecked|length), unitsChecked: (.unitsChecked|length)}'
```

What to explain:

- Verifies all 8 spaces, all 61 units, all clone links.
- Verifies stack-specific assertions (e.g. for STACK=ollama, the deployment-layer rag-server unit must contain `host.docker.internal`).
- Verifies the recipe manifest contains all 4 components and all 3 variants.

**PAUSE.** Wait for the human.

---

## Stage 5: "Layer Initiatives Over The Chain" (mutates ConfigHub)

Ask: "This creates 5 Views in the recipe space, each filtering units of this run. Ready?"

Run:

```bash
./seed-initiatives.sh
```

What to explain:

- Five initiatives now exist as ConfigHub Views with Filters and metadata.
- They span pinned-model-versions, embed/index-dim match, GPU resource limits, guardrail policy, and resource limits.
- They use the same View+Filter+Trigger shape as `../../../initiatives-demo/`. No `vet-kyverno` triggers attached yet — connect a worker if live policy enforcement is needed.
- The script prints a View Explorer URL per initiative (`/x/view-explorer?view=<ViewID>`). The filters span all 8 spaces, so a single initiative covers direct + Flux + Argo variants.

GUI now: Open one of the printed View Explorer URLs (e.g. GPU Resource Limits) — you should see 6 units (2 GPU components × 3 deploy variants).

**PAUSE.** Wait for the human.

---

## Stage 6: "Optional: Bring Up A Live Cluster" (mutates local Docker)

Only proceed if the human wants the runtime path on a kind cluster.

Run:

```bash
kind create cluster --name rag
kubectl cluster-info --context kind-rag
kubectl --context kind-rag create namespace tenant-acme
```

What to explain:

- One single-node kind cluster is enough for the demo.
- The rag-server pod will reach the host's Ollama via `host.docker.internal:11434`.
- The deploy namespace must exist before applying — the worker applies into the namespace but does not create it.

**PAUSE.** Wait for the human.

---

## Stage 7: "Optional: Create Worker And Bind Target" (mutates ConfigHub)

Run:

```bash
source .state/state.env
deploy="${PREFIX}-deploy-tenant-acme"
cub worker create --space "${deploy}" rag-worker
cub worker run --space "${deploy}" rag-worker -t kubernetes -d
sleep 5

# The worker auto-discovers kubectl contexts and registers a target per cluster
cub target list --space "${deploy}" -o json \
  | jq -r '.[] | select(.Target.Slug | endswith("kind-rag")) | "\(.Space.Slug)/\(.Target.Slug)"'

./set-target.sh <space>/<kubernetes-target>
```

What to explain:

- The worker connects out to ConfigHub, registers itself, and exposes one target per kubectl context.
- Targets are routed by provider type. `Kubernetes` → direct variant.
- Setting a target re-renders the recipe-manifest unit with the bundle hint.

**PAUSE.** Wait for the human.

---

## Stage 8: "Optional: Apply Live" (mutates the kind cluster)

Run (in order — the rag-server pod waits on the others to be reachable):

```bash
source .state/state.env
deploy_space="${PREFIX}-deploy-tenant-acme"

for unit in vector-db-tenant-acme nim-embedding-tenant-acme nim-llm-tenant-acme rag-server-tenant-acme; do
  cub unit approve --space "${deploy_space}" "${unit}"
  cub unit apply   --space "${deploy_space}" "${unit}"
done

kubectl --context kind-rag -n tenant-acme rollout status deploy/rag-server
```

What to explain:

- The worker takes each unit's rendered YAML and applies it to the kind cluster.
- All four components run concurrently. rag-server is the only one that actually serves traffic in STACK=ollama; the others exist in-chain but do nothing useful (the LLM and embedding are bypassed to host Ollama).
- If apply fails with `namespaces "tenant-acme" not found`, run `kubectl --context kind-rag create namespace tenant-acme` then retry the apply.

**PAUSE.** Wait for the human.

---

## Stage 9: "Optional: Real Query End-To-End" (live)

Run:

```bash
./query.sh "What is the capital of France? Answer in one sentence."
```

What to explain:

- The script port-forwards to rag-server, hits `/health` (dumps the env vars ConfigHub fed in), then `/answer`.
- The answer comes from `llama3.2:3b` running on the host's Metal GPU, called by the in-cluster pod via `host.docker.internal`.
- Every value in the `/health` response (model name, top-k, prompt template, guardrail policy, vector-db host) traces back through the recipe chain to a layer mutation in ConfigHub.

**PAUSE.** Wait for the human.

---

## Stage 10: "Optional: Show Upgrade Propagation" (mutates ConfigHub)

Run:

```bash
./upgrade-chain.sh llama3.2:1b nomic-embed-text
./verify.sh
```

What to explain:

- The bump happened at the profile layer.
- `cub unit push-upgrade` propagates the change down to recipe and deployment variants.
- Tenant-local values (namespace, region, the `LLM_HOST` override for STACK=ollama) survive the propagation. That's Story 1 (safe upgrades) made concrete.

**PAUSE.** Wait for the human.

---

## Stage 11: "Cleanup"

Run:

```bash
./cleanup.sh
kind delete cluster --name rag
```

This removes every space and unit, plus the kind cluster.

---

## What Mutates What

| Command | Writes |
|---|---|
| `./setup.sh --explain` / `--explain-json` | Nothing |
| `./setup.sh` | ConfigHub spaces + units + clone links + recipe-manifest unit + local `.state/` + `.logs/` |
| `./verify.sh` | local `.logs/verify.latest.log` |
| `./seed-initiatives.sh` | ConfigHub Filters + Views in the recipe space |
| `./set-target.sh` | ConfigHub target bindings + re-rendered recipe manifest |
| `./upgrade-chain.sh` | ConfigHub unit data at profile/recipe/deployment layers |
| `cub unit apply` | Live cluster state |
| `./query.sh` | Nothing in ConfigHub; port-forwards to a pod and prints a query result |

## Related

- [README.md](./README.md)
- [contracts.md](./contracts.md)
- [prompts.md](./prompts.md)
- [../02-nvidia-blueprints-fit.md](../02-nvidia-blueprints-fit.md)
- [../whole-journey.md](../whole-journey.md)
