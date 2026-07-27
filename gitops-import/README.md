# GitOps Import (Argo CD) — ARCHIVED

> **This example no longer works and is retained for reference only.**
>
> It is built on the bridge worker protocol, which has been removed from ConfigHub.
> `cub gitops discover`, `cub gitops import`, `cub unit apply`, and the
> `ArgoCDRenderer` / `ArgoCDOCI` bridges no longer exist. The accompanying
> documentation pages have been removed from docs.confighub.com, and their
> content is inlined below so it is preserved alongside the scripts.
>
> Configuration now reaches a cluster by being published as an immutable OCI
> Release that a GitOps operator pulls. See
> [Integrating with GitOps operators](https://docs.confighub.com/guide/gitops/).

This directory contains scripts and data to set up a test environment for GitOps import on ConfigHub with Argo CD. It sets up a local kind cluster with Argo CD and a representative gitops config repo ([jesperfj/gitops-argocd](https://github.com/jesperfj/gitops-argocd)) containing:

- **2 Argo CD Projects**: `app-cubbychat` (application team) and `platform` (infrastructure)
- **2 ApplicationSets**: one per project, using git directory generators
- **3 Applications** generated from the upstream repo: `cubbychat` (app), `alloy` and `grafana` (platform)

The upstream repo has this structure:

```
apps/
  cubbychat/
    backend/
    frontend/
    postgres/
platform/
  alloy/
  cert-manager/
  external-dns/
  external-secrets/
  grafana/
  traefik/
```

## How it worked

- **Argo CD** continuously synced a git repo to the cluster — the repo structure *was* the deployment config.
- **A ConfigHub worker** ran inside the cluster, read Argo CD application state, and reported it back to ConfigHub.
- **ConfigHub** used that data to let you discover, import, and manage those apps.

The result: manifests were rendered into ConfigHub and then deployed from ConfigHub via Argo CD through an OCI image fetched from ConfigHub. That gave you versioning, validation, policy enforcement, review, and approval on the fully rendered configuration — and modifications made in ConfigHub were preserved across re-renders, similar to patches.

## Setup

```bash
bin/create-cluster       # Create kind cluster (kubeconfig saved to var/)
bin/install-argocd       # Install Argo CD with direct localhost access
bin/setup-apps           # Create projects and apply ApplicationSets
CUB_SPACE=<space> bin/install-worker   # Create and deploy ConfigHub worker
```

### 1. Create a local cluster

Creates a kind (Kubernetes in Docker) cluster named `gitops-import`. The kubeconfig is saved to `var/gitops-import.kubeconfig` — isolated from your global `~/.kube/config`.

### 2. Install Argo CD

Installs Argo CD into the cluster and configures it for local access:

- Installs Argo CD from the official stable manifests
- Exposes the Argo CD UI at **http://localhost:9080** (no `port-forward` needed)
- Pre-provisions a `confighub-worker` service account in Argo CD with read-only access to applications — the account ConfigHub used to query Argo CD

The admin password is printed at the end and saved to `var/argocd-admin-password.txt`.

### 3. Set up applications

Creates two Argo CD **AppProjects** (logical tenants) and two **ApplicationSets** pointed at the example GitOps repo:

| AppProject      | ApplicationSet  | Watches            |
| --------------- | --------------- | ------------------ |
| `app-cubbychat` | `app-cubbychat` | `apps/cubbychat/*` |
| `platform`      | `platform`      | `platform/*`       |

Argo CD scans those directories and creates Applications. They are configured with `CreateNamespace=true` but **not** with automated sync — the `ArgoCDRenderer` bridge required autosync to be off on imported Applications so that it, not Argo CD, owned the rendered output.

### 4. Install the ConfigHub worker

```bash
CUB_SPACE=<your-space> bin/install-worker
```

This was the bridge between Argo CD and ConfigHub. The script:

1. Creates a ConfigHub space (if it doesn't exist) and registers a worker in it
2. Generates a non-expiring API token for the `confighub-worker` Argo CD account
3. Deploys the worker as a Kubernetes Deployment in the `confighub` namespace, with three capabilities:
   - `kubernetes` — read cluster resources and apply manifests
   - `argocdrenderer` — query Argo CD and render application configs into ConfigHub
   - `argocdoci` — create Argo CD `Application` resources that deploy rendered manifests from ConfigHub's OCI registry
4. The worker talked to Argo CD via internal cluster DNS (`argocd-server.argocd.svc.cluster.local`) and called back to ConfigHub

Because a single worker hosted all three bridges, one target — `worker-kubernetes-yaml-cluster` — was enough for discovery, rendering, and deployment. Those capabilities registered on that single target as additional `ConfigTypes`; a unit's `ProviderType` selected which bridge handled it, so dry units (`ArgoCDRenderer`) and wet units (`ArgoCDOCI`) shared the same target.

## Access

| Resource  | URL                                 |
| --------- | ----------------------------------- |
| Argo CD UI | http://localhost:9080              |
| Username  | `admin`                             |
| Password  | `cat var/argocd-admin-password.txt` |

To use `kubectl` directly:

```bash
export KUBECONFIG=$PWD/var/gitops-import.kubeconfig
kubectl get applications -n argocd
```

## GitOps discover / import

```bash
# Discover Argo CD Applications in the cluster
cub gitops discover --space <space> worker-kubernetes-yaml-cluster

# Import: creates dry units (renderer inputs) and wet units (rendered output),
# linked so applying a dry unit populates its wet unit's Data.
cub gitops import --space <space> worker-kubernetes-yaml-cluster
```

`cub gitops import` then:

- Created **dry units** containing the Application / HelmRelease / Kustomization resources. Each dry unit had a Target attached supporting the corresponding renderer bridge. Dry units were the renderer inputs.
- Created corresponding **wet units** to hold the rendered manifests, linked to their dry unit via a `MergeUnits` link with `UseLiveState: true`. CRDs were split into separate wet units.
- Set each wet and CRD unit's `ProviderType` to `ArgoCDOCI` or `FluxOCI` so that applying it deployed through ConfigHub's OCI registry via the GitOps tool.
- For Argo CD: disabled auto-sync on the imported `Application` (the existing one had to remain in the cluster but must not be manually synced or deleted — ConfigHub then owned the rendered output).
- For Flux: suspended the imported `HelmRelease` or `Kustomization` for the same reason.

### The dry → link → wet pipeline

ConfigHub used a dry unit → link → wet unit pipeline for all rendering workflows (Helm, Argo CD, Flux):

- A **dry unit** was attached to a renderer bridge target and contained an Argo CD `Application` resource referring to DRY configuration in git, such as a Helm chart or kustomization. When applied, its bridge worker rendered the template (Helm chart → Kubernetes manifests) and returned the output as LiveState. No infrastructure was directly affected — the unit existed solely to produce rendered configuration.
- A **link** connected a downstream wet unit (FromUnit) to an upstream dry unit (ToUnit). `MergeUnits` links were created with `UseLiveState: true`, so the wet unit received the dry unit's rendered LiveState rather than its Data.
- A **wet unit** received rendered configuration through the link and applied it to live infrastructure. Its Data was populated by link resolution, not written directly.

The merge behaviour matched unit upgrades, so changes made to a wet unit were not clobbered by subsequent re-renderings.

### The unit-level model, by hand

The bulk `cub gitops` flow was built on this primitive:

1. The GitOps tool (Flux or Argo CD) and the worker had to be running in the cluster. For Argo CD, the worker called Argo CD, so `ARGOCD_SERVER` had to be set to the cluster DNS name for Argo CD's server, `ARGOCD_AUTH_TOKEN` to a valid token, and `ARGOCD_INSECURE` to `true` if targeting the service directly. Flux was not called directly.
2. Create a unit with `ToolchainType` `Kubernetes/YAML` containing the KRM resource specifying what and how to render — a `HelmRelease` or `Kustomization` for Flux, an `Application` for Argo CD. Any source repository resources referenced had to already exist in the cluster.
3. Attach a Target supporting the corresponding renderer bridge `ProviderType`, `FluxRenderer` or `ArgoCDRenderer`.
4. Create a corresponding empty unit of `ToolchainType` `Kubernetes/YAML` to receive the rendered configuration.
5. Link the empty unit to the renderer unit with `UpdateType` `MergeUnits` and `UseLiveState` true:
   ```bash
   cub link create --space "$SPACE" - rendered-unit renderer-unit --use-live-state --auto-update --update-type MergeUnits
   ```
6. Apply the renderer unit. The rendered configuration was copied to the rendered unit automatically when `--auto-update` was specified, or on demand:
   ```bash
   cub unit update --space "$SPACE" --patch --resolve "Link:*" rendered-unit
   ```

### Rendering and deploying

- The `ArgoCDRenderer` bridge called Argo CD to render the manifests — similar in effect to [`argocd app manifests`](https://argo-cd.readthedocs.io/en/stable/user-guide/commands/argocd_app_manifests/) — and returned the result to ConfigHub as LiveState.
- The `FluxRenderer` bridge performed the equivalent rendering for `HelmRelease` / `Kustomization` resources.

Applying a wet (or CRD) unit deployed the rendered manifests via the `ArgoCDOCI` or `FluxOCI` bridge. ConfigHub's worker created a new Argo CD `Application` (or Flux `HelmRelease` / `Kustomization`) that fetched an OCI image from ConfigHub's OCI registry and deployed it to the cluster. The new GitOps resource took over the workload the original, now-suspended resource had been managing.

### Updating after a git change

After a change to DRY configuration in git (helm values, templates, or kustomizations):

1. Re-apply the corresponding dry unit(s) to re-render. The renderer bridge pulled the latest from git and produced updated manifests.
2. Review the diff on the linked wet units. Modifications previously made to a wet unit were preserved — the merge behaviour matched unit upgrades.
3. Approve and apply the updated wet units to roll the change out through Argo CD / Flux.

## Importing through the UI

The same flow was available in the ConfigHub UI via the GitOps Import link in the app navigation. You selected **ArgoCD Renderer** as the import type, chose a discovery target (the `kubernetes` worker) and a render target (the `argocdrenderer` worker), and picked a space for the temporary Argo CD discovery unit.

Behind the discovery step, ConfigHub created a scratch unit storing Argo CD Applications as resources, then used a ConfigHub function to extract and parse them and return the list to the UI, which displayed one row per Argo CD Application. You then selected which to import, ran the rendering process to create the rendered units linked to the dry application units, and reviewed a summary of all units created.

The original documentation page included screenshots of each wizard step and a step-by-step video walkthrough; those assets remain in the history of the [confighub/docs](https://github.com/confighubai/docs) repository.

## Applying ConfigHub units to this cluster

You could also use this cluster to apply ConfigHub-managed units directly. `cub unit apply` did not create namespaces automatically, but you could create the namespace as a ConfigHub unit and link your workload unit to it so ordering was handled for you:

```bash
export KUBECONFIG=$PWD/var/gitops-import.kubeconfig

# Create and apply a namespace unit
kubectl create namespace <namespace> -o yaml --dry-run=client \
  | egrep -v "creationTimestamp|status" \
  | cub unit create --space <deploy-space> <namespace-unit> -
cub unit apply --space <deploy-space> <namespace-unit>

# Link the workload unit to the namespace unit, then apply
cub link create --space <deploy-space> - <deploy-unit> <namespace-unit>
cub unit apply --space <deploy-space> <deploy-unit>
```

## Teardown

```bash
bin/teardown
```
