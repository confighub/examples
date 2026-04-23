# Case 02: Large CRDs

## Goal

Productize the external-secrets lesson without dragging the full Kubara stack
behind it:

> ConfigHub and the agent detect large-CRD delivery risk before a workload
> install, choose an explicit apply mode, and prove the CRDs plus dependent
> controller are operational.

This case should make the CRD decision visible instead of discovering a 256 KiB
annotation failure halfway through a sync.

2026-04-23 live-run harvest: preflight correctly predicted the large-CRD risk,
but Argo CD v3.3.8 still rejected the 306980-byte CRD when the fixture relied
only on Application-level `ServerSideApply=true`. Argo's documented safe
surface includes Application-level sync options and per-resource sync-option
annotations, and client-side apply migration is enabled by default unless
disabled. The hardened Case 02 fixture therefore uses all three signals before
the next run:

- `ServerSideApply=true` on the generated Application;
- `ClientSideApplyMigration=false` on the generated Application;
- `argocd.argoproj.io/sync-options: ServerSideApply=true` on
  `stores-large.yaml` itself.

Fresh-Claude handoff prompt: [`PROMPT.md`](./PROMPT.md).

## Scenario

One kind cluster. One CRD provider. One dependent resource.

The fixture includes:

- a small controller-style app with two CRDs;
- one ordinary CRD that applies normally;
- one intentionally large CRD that would fail if a client-side
  `last-applied-configuration` annotation is stamped;
- one dependent custom resource that proves the CRD exists and is usable.

The names are fake and harmless:

```text
ApplicationSet/mini-large-crds
  -> Application/controlplane-mini-large-crds
     -> CustomResourceDefinition/widgets.platform.example.com
     -> CustomResourceDefinition/stores.platform.example.com
     -> Deployment/mini-crd-controller
     -> Widget/example   (held back for Gate B)
```

## What This Trains

- CRD preflight before sync.
- Distinguishing "retry later" from "delivery mode is wrong."
- Routing large CRDs to server-side apply or a CRD-specific sync option.
- Proving a dependent custom resource only after the CRD exists.
- Avoiding hidden `kubectl` workarounds.

## Route

```text
CONFIGHUB SAYS: ROUTE
Lane: CH-WRITE for ApplicationSet, then RECOVERY/SETUP lane only if CRD apply
mode requires a special path
Scope: CRD provider app only
Wrong move: retrying a known annotation-size failure without changing apply mode
```

When the helper scripts are available, render the route with
`./scripts/confighub-banner route --force-color` and render preflight/gate proof
with `./scripts/confighub-proof-rows --force-color --color-by both`. Color is
part of the trust surface. If the terminal flattens ANSI color, say that
plainly instead of silently falling back to uncolored bullets.

## GUI Moment

The ConfigHub GUI must appear before Gate A approval, not only at closeout.
After the preflight runs and before asking for the first ConfigHub write, run:

```bash
./scripts/confighub-gui-urls --space mini-large-crds
```

Label the output:

```text
VIEW IN CONFIGHUB — OPEN NOW
```

Tell the user what the page should prove at that moment: worker Ready, target
bound, and unit panel empty. After Gate A lands, run the helper again with
`--unit mini-large-crds-appset` and point the user at the governed unit/revisions.

## Preflight Helper

Use the repo-local preflight helper before Gate A. It is read-only and does not
create clusters, install Argo, or apply CRDs.

```bash
incubator/mini-kubara/case-02-large-crds/preflight.sh --explain
incubator/mini-kubara/case-02-large-crds/preflight.sh
```

The helper:

- counts fixture CRDs;
- measures each CRD's serialized body size;
- flags any CRD that exceeds the Kubernetes 262144-byte annotation limit;
- checks that the large-CRD hardening is present:
  Application-level `ServerSideApply=true`, Application-level
  `ClientSideApplyMigration=false`, and the per-resource Argo sync-options
  annotation on the oversized CRD;
- states the intended apply mode before any mutation;
- explains why client-side apply is unsafe for the large fixture;
- recommends the explicit delivery path for the future live run.

A fresh Claude session must run `--explain` and then the default form before
proposing Gate A. It must not create a kind cluster, install Argo CD, stand up
a worker/target, or `kubectl apply` any CRD on the strength of this helper.

## Fixture Layout

Public fixture in this repo:

```text
incubator/mini-kubara/case-02-large-crds/fixtures/
  README.md
  generate-stores-large.sh
  applicationsets/mini-large-crds.yaml
  workloads/
    controller.yaml
    crds/
      widgets.yaml
      stores-large.yaml     (intentionally > 262144 bytes)
  resources/
    widget-example.yaml     (held back for Gate B)
```

Runtime source for Argo:

```text
https://github.com/confighub/examples
path: incubator/mini-kubara/case-02-large-crds/fixtures/workloads
```

The Application reconciles `fixtures/workloads/` recursively with
`syncOptions: [CreateNamespace=true, ServerSideApply=true,
ClientSideApplyMigration=false]`. The oversized `stores-large.yaml` CRD also
carries `argocd.argoproj.io/sync-options: ServerSideApply=true` directly on the
resource. The dependent `Widget/example` is not under `workloads/` so Gate B
controls when the custom resource is created — only after both CRDs exist.

The generated Application deliberately has no `syncPolicy.automated` block.
That keeps the approval boundary sharp, but it also means a fresh run must
trigger a one-shot Argo sync after Gate A applies the ApplicationSet. Do not
poll for auto-sync; prove the Application exists, then ask for the one-shot sync
approval if it is in scope.

Worker setup gotcha from the 2026-04-23 live pass: `cub worker install --export`
emits the non-secret manifest and redacts Secret material. Use the current
`cub worker install --help` surface, export the Secret explicitly when required
(for example with `--export-secret-only` if present), and never redirect mixed
stderr/stdout logs into a Kubernetes YAML file.

The public source is mandatory, not cosmetic. A fresh Argo install in kind will
not have credentials for the private authoring repo. The generated
`stores-large.yaml` must be committed on the public side too because Argo
cannot run the generator.

Public graduation targets:

- external-secrets-style CRDs;
- cert-manager-style CRDs;
- any CRD-heavy platform app from a public example repo.

Do not start with the real public CRDs. The local large-CRD fixture keeps the
failure mode predictable.

## Prompt To Run

```text
Run Mini-Kubara Case 02: Large CRDs.

Start with the read-only preflight.
  incubator/mini-kubara/case-02-large-crds/preflight.sh --explain
  incubator/mini-kubara/case-02-large-crds/preflight.sh

Do not create a cluster, install Argo, or apply CRDs on the strength of the
preflight alone.

If the preflight reports any CRD over the 262144-byte annotation limit, stop
and propose the explicit delivery mode. For the default governed path, the
hardened fixture should show Application `ServerSideApply=true`,
`ClientSideApplyMigration=false`, and a per-resource
`argocd.argoproj.io/sync-options: ServerSideApply=true` annotation on the
oversized CRD. If any of those are missing, stop before Gate A and classify the
fixture as not hardened. Do not retry blindly.

After explicit Y/N approval, apply the provider path, prove both CRDs exist,
prove the controller is Running/Ready, then sync or apply the dependent
custom resource as a separate Gate B. Close with cub-scout, ConfigHub
receipts, Argo status, and exact CRD proof.
```

## Expected Proof

- Preflight names the large CRD risk before sync.
- The selected apply mode is visible in the route card.
- Both CRDs exist after the mutation.
- No `metadata.annotations: Too long` error appears.
- Controller pod is Running/Ready.
- Dependent custom resource is accepted by the API server.
- The final report says whether the large-CRD path was ConfigHub-governed or a
  separately approved dev setup write.

## Stop Conditions

- CRD preflight cannot determine apply mode.
- The selected path still stamps an oversized annotation.
- The dependent custom resource is attempted before the CRD exists.
- Admission denial appears and was not part of the case scope.
- The agent describes a direct live CRD workaround as governed workload proof.

## What This Does Not Prove

- Admission-controller reliability.
- Promotion.
- Values-file governance.

This case is only about making CRD delivery semantics first-class.
