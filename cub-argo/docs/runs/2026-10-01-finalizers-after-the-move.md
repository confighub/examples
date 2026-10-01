# Run log: finalizers after the move, 2026-10-01

A check that moving generated Applications onto Units leaves their finalizers
as they were, and that the finalizer still does its job afterwards. Asked
because another route to the same end, new Applications adopting running
workloads, needs finalizers taken off, and their absence caused trouble later.

- **Argo CD:** v3.5.3 on kind, management cluster `rh-fargo`, installed from the
  release manifest, with `kustomize.buildOptions: --enable-helm` and
  `kustomize.path.v5` in `argocd-cm`.
- **Cluster:** `rh-fdev1` as `dev-1` (canary), registered with the example's
  labels.
- **ConfigHub:** v0.6.8, self-hosted, gateway `192.168.97.2:32181` over plain
  HTTP. `cub` v0.6.8, `cub argo` 0.2.0 as released.
- **Estate:** this repository's `gitops/argo/expert-app-of-apps`: `root`,
  `storefront`, three ApplicationSets, three generated Applications.
- **Output:** trimmed to the lines that matter; the UIDs are the real ones.

## 1. Generated Applications carry the finalizer already

None of the three templates names a finalizer. The ApplicationSet controller
adds it all the same:

```text
NAME                     SRC                                     FIN                                        OWNER
dev-1-apptique           https://github.com/confighub/examples   [resources-finalizer.argocd.argoproj.io]   ApplicationSet
dev-1-checkout-cache     https://github.com/confighub/examples   [resources-finalizer.argocd.argoproj.io]   ApplicationSet
dev-1-cluster-baseline   https://github.com/confighub/examples   [resources-finalizer.argocd.argoproj.io]   ApplicationSet
root                     oci://192.168.97.2:32181/space/rh-fin-root-children         [resources-finalizer.argocd.argoproj.io]   <none>
storefront               oci://192.168.97.2:32181/space/rh-fin-storefront-children   [resources-finalizer.argocd.argoproj.io]   <none>
```

`plan` read only the template, so its "never delete" line named `root` and
`storefront` and left these out. It now names every Application an
ApplicationSet generates, unless the ApplicationSet sets
`preserveResourcesOnDeletion`, and knows the finalizer's `/foreground` and
`/background` forms.

## 2. The move keeps finalizer and UID

`apply.sh`, `handover.sh`, the three ApplicationSets retired through their
Units, then `move-applications.sh canary`. Name, UID, finalizers, owner, sync
options, before and after:

```text
before
dev-1-apptique          69354083-5f33-4b49-a06e-5218ec055042  ["resources-finalizer.argocd.argoproj.io"]  ApplicationSet  -
dev-1-checkout-cache    308e52cf-7b10-4fb5-b335-f47525db755c  ["resources-finalizer.argocd.argoproj.io"]  ApplicationSet  -
dev-1-cluster-baseline  cbeb5161-7d93-4e08-b710-f1ed8ad8cad6  ["resources-finalizer.argocd.argoproj.io"]  ApplicationSet  -
after
dev-1-apptique          69354083-5f33-4b49-a06e-5218ec055042  ["resources-finalizer.argocd.argoproj.io"]  none  Prune=false  oci://…/space/rh-fin-apptique-dev-1  Synced/Healthy
dev-1-checkout-cache    308e52cf-7b10-4fb5-b335-f47525db755c  ["resources-finalizer.argocd.argoproj.io"]  none  Prune=false  oci://…/space/rh-fin-checkout-cache-dev-1  Synced/Healthy
dev-1-cluster-baseline  cbeb5161-7d93-4e08-b710-f1ed8ad8cad6  ["resources-finalizer.argocd.argoproj.io"]  none  Prune=false  oci://…/space/rh-fin-platform-addons-cluster-baseline-dev-1  Synced/Healthy
```

The two Deployments and two Services in `storefront-dev` had the UIDs they had
before. `root` and `storefront` kept their finalizers too.

## 3. The finalizer still works

`dev-1-checkout-cache` was deleted on purpose, to see whether Argo CD still
treats its workloads as its own:

```text
owned by dev-1-checkout-cache: ["Service/storefront-dev/checkout-cache","Deployment/storefront-dev/checkout-cache"]
application.argoproj.io "dev-1-checkout-cache" deleted
controller: Deleting application's resources with Foreground propagation policy
controller: 2 objects remaining for deletion
```

Within two seconds the Deployment and Service were gone, the Application was
gone, `storefront` had applied its Unit again, and the new Application had
synced both back from its Space:

```text
dev-1-checkout-cache  f3a72d7b-7190-41fd-8b9c-4ab0b7c44fcb  ["resources-finalizer.argocd.argoproj.io"]  oci://…/space/rh-fin-checkout-cache-dev-1  Synced/Healthy
Deployment checkout-cache  77b448e2-b52a-4106-8455-d0ecc1042b15  (was 1e2e464d-7cbb-4855-b279-c2027a791535)
Service    checkout-cache  b9376f57-dc31-404b-a088-a635ab0ea4af  (was 7c66972b-5afc-4907-b1a2-f0070bd86ea9)
```

So a delete does what it did before the move, which is why no script here
deletes an Application, and the estate puts back one deleted by hand.

## Not checked

An Application whose finalizers had been taken off before the handover. The
plugin would leave it without them, as it leaves everything else.

## Afterwards

Both kind clusters and every `rh-fin-*` Space were deleted.
