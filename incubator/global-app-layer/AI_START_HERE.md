# AI Start Here

Use this page when you want to drive the `global-app-layer` package safely with Codex, Claude, Cursor, or another AI assistant.

## What This Package Is For

This package demonstrates how ConfigHub can represent layered recipes as real versioned config objects.

It is for:
- multi-component app stacks
- NVIDIA AICR-style layered recipes
- safe updates and downstream propagation
- optional direct or delegated delivery after the recipe is materialized

It is **not** only about live deployment. A large part of the value is visible in the ConfigHub database before anything is applied to a cluster.

## Choose The Smallest Matching Demo

Before you run this package, pick the smallest demo that matches the user's actual question:

| User goal | Better first stop |
|---|---|
| “Show me Argo import from GitHub” | [cub-scout argo-import-confighub-demo](https://github.com/confighub/cub-scout/tree/main/examples/argo-import-confighub-demo) |
| “Show me Flux import from GitHub” | [cub-scout flux-import-confighub-demo](https://github.com/confighub/cub-scout/tree/main/examples/flux-import-confighub-demo) |
| “Show me Helm first” | [helm-platform-components](../../helm-platform-components/README.md) and [cub-scout Helm quickstart](https://github.com/confighub/cub-scout/blob/main/docs/reference/cub-track-quickstart-helm.md) |
| “Show me the smallest direct apply example” | [single-component](./single-component/README.md) |
| “Show me App-Deployment-Target” | [promotion-demo-data](../../promotion-demo-data/README.md) |
| “Show me microservices / app-of-apps / monorepo styles” | [global-app](../../global-app/README.md) and [cub-scout apptique examples](https://github.com/confighub/cub-scout/tree/main/examples/apptique-examples) |

Use `global-app-layer` when the question is specifically about:
- layered recipes
- deployment units at the leaf
- upstream propagation with preserved downstream specialization
- NVIDIA-shaped configuration chains

## WET-First, Not Live-First

This package is intended-state first.

The normal path is:
1. preview the layered recipe
2. materialize it in ConfigHub as WET objects
3. verify it in ConfigHub
4. optionally bind a target
5. optionally apply live

So `setup.sh` is ConfigHub-first, not cluster-first.
Do not treat these examples as "inspect the cluster first" walkthroughs.

## Safe First Steps

Start with read-only commands only:

```bash
git rev-parse --show-toplevel
which cub
kubectl version --client 2>/dev/null || true
cub version
cub context list --json | jq

cd incubator/global-app-layer
./find-runs.sh --json | jq

cd realistic-app
./setup.sh --explain
./setup.sh --explain-json | jq
```

What these do not mutate:
- they do not create spaces
- they do not create units
- they do not bind targets
- they do not apply to a cluster

Once you run `./setup.sh`, use the printed artifacts instead of relying on terminal scrollback:
- clickable GUI URLs for the recipe space, deploy space, recipe manifest, and one deployment unit
- durable logs in `.logs/setup.latest.log`, `.logs/set-target.latest.log`, `.logs/verify.latest.log`, and `.logs/cleanup.latest.log`

For live delivery, use the package-level preflight before you claim anything is ready:

```bash
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

## Ready For A Fresh Run

Use the same short path across the layered examples:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

If you start ConfigHub-only and later want the live path:

```bash
./set-target.sh <space/target>
```

If the human wants the whole lifecycle instead of only setup + verify, use:

- [whole-journey.md](./whole-journey.md)

That walkthrough covers:
- live target binding and apply
- shared upstream updates
- custom downstream deployment variants

## Capability Check

Before you choose a path, check what is actually available:

```bash
cub version
cub context list --json | jq
cub target list --space "*" --json | jq
```

Interpret the result like this:
- if `cub` is missing or auth is unavailable, stay in preview mode
- if auth works but target listing is empty or irrelevant, use ConfigHub-only mode
- if a real target is visible, run `./preflight-live.sh <space/target>` before you offer the live path

Important:
- `cub target list` proves visibility, not readiness
- `set-target.sh` proves binding, not readiness
- only call the live path ready if `preflight-live.sh` reports `applyReady: true`
- when exploring Argo integration, distinguish Argo rendering from real Argo-managed sync; the current `ArgoCDRenderer` target is a renderer/hydration path, not the final sync proof

**ArgoCDRenderer payload compatibility:**
- `ArgoCDRenderer` targets expect units containing ArgoCD `Application` CRDs (`apiVersion: argoproj.io/v1alpha1`)
- The raw-manifest examples (`realistic-app`, `single-component`, etc.) are **incompatible** with `ArgoCDRenderer`
- If you try, you'll get: `failed to parse Application: expected apiVersion argoproj.io/v1alpha1, got v1`
- For ArgoCDRenderer proof, use brownfield-imported Application units (e.g., `argocd-cubbychat-Application-dry`)
- See [contracts.md](./contracts.md) for the full compatibility matrix

## Capability Branching

### A. Preview only

Use this if the human wants explanation only, or if `cub` auth is unavailable.

Safe path:

```bash
cd incubator/global-app-layer
./find-runs.sh --json | jq
cd realistic-app
./setup.sh --explain-json | jq
```

### B. ConfigHub database only

Use this if auth works but there is no worker or target available.

Safe path:

```bash
cd incubator/global-app-layer/realistic-app
./setup.sh
./verify.sh
```

This writes ConfigHub spaces and units, but does not deploy anything live.

### C. Live target available

Use this only when `./preflight-live.sh <space/target>` reports `applyReady: true`.

Safe path:

```bash
cd incubator/global-app-layer
./preflight-live.sh <space/target>
cd incubator/global-app-layer/realistic-app
./setup.sh <prefix> <space/target>
./verify.sh
```

Then approve and apply the deployment units explicitly.

## Exact Commands To Run

Recommended first walkthrough:

```bash
cd incubator/global-app-layer/realistic-app

# Preview the full plan first
./setup.sh --explain
./setup.sh --explain-json | jq

# Ready for a fresh run
./setup.sh                              # ConfigHub-only
./setup.sh <prefix> <space/target>     # with live target
./verify.sh
```

If the user wants the smallest example instead:

```bash
cd incubator/global-app-layer/single-component
./setup.sh --explain
./setup.sh
./verify.sh
```

If the user specifically wants the NVIDIA-shaped example:

```bash
cd incubator/global-app-layer/gpu-eks-h100-training
./setup.sh --explain
./setup.sh
./verify.sh
```

If the user wants deployment, updates, and custom live variants instead of only ConfigHub-only walkthroughs, switch to [whole-journey.md](./whole-journey.md).

## What Mutates What

| Command | Reads | Writes |
|---|---|---|
| `./find-runs.sh --json` | ConfigHub labels on spaces/units | nothing |
| `./preflight-live.sh <space/target>` | ConfigHub target and worker state | nothing |
| `./setup.sh --explain-json` | local scripts and source manifests | nothing |
| `./setup.sh` | local source manifests + current ConfigHub org | ConfigHub spaces, units, links, recipe manifest, local `.state/`, local `.logs/setup.latest.log` |
| `./verify.sh` | ConfigHub objects created by the example | local `.logs/verify.latest.log` |
| `./set-target.sh <space/target>` | ConfigHub deployment units, target ref | ConfigHub target bindings, local `.logs/set-target.latest.log` |
| `cub unit apply ...` | ConfigHub deployment units + target + worker | live target state |

## What Success Looks Like

After a successful ConfigHub-only run you should see:
- five, six, or seven new spaces with a shared prefix
- units for each layer
- one recipe manifest in the recipe space
- `verify.sh` passing

After a successful live path you should also see:
- `./preflight-live.sh <space/target>` returning `applyReady: true`
- targets bound on deployment units
- successful `cub unit apply`
- for direct targets: worker-mediated apply evidence plus live resources
- for delegated targets: agent-side objects/sync evidence plus live resources

## GUI Checkpoints

Use the GUI while you go:

1. open the new spaces created by the example prefix
2. open the recipe space and inspect the recipe manifest unit
3. open one deployment unit and inspect its upstream chain and current intended state
4. if a target is set, inspect the deployment unit again and confirm the target binding is visible
5. if using the live path, inspect the deployment space again after apply and compare intended state vs live result

The easiest way to get there is to use the clickable URLs printed by `./setup.sh`.

## CLI Footguns To Avoid

- use `cub version`, not `cub --version`
- use `cub context list`, not `cub context current`
- for machine-readable unit inspection, prefer the exact jq examples in `contracts.md`
- do not treat a visible target as a live-ready target until `./preflight-live.sh` says so

## Cleanup

Use the example-specific cleanup script:

```bash
./cleanup.sh
```

This removes the ConfigHub objects created by the example.
It does not magically clean unrelated cluster resources if you applied and then changed things outside the example flow.
The cleanup output is also captured in `.logs/cleanup.latest.log`.
