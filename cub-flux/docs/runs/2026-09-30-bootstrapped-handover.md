# Run log: bootstrapped handover, 2026-09-30

A fleet set up the way `flux bootstrap` sets it up, handed over to ConfigHub
through the root, and handed back. Kept as a record of what was run and seen.

- **Flux:** v2.8.6 on kind, cluster `rh-boot`.
- **ConfigHub:** v0.6.8, self-hosted, gateway over plain HTTP. `cub` v0.6.8.
- **Fleet:** this repository's `gitops/flux/beginner`, served by a lab Git server the cluster reads and the test pushes to.
- **Bootstrap:** `clusters/dev/flux-system/` holds `gotk-components.yaml` (from `flux install --export`) and a `gotk-sync.yaml`. So the `flux-system` Kustomization applies `clusters/dev/` from that server, and owns both layers.
- **Output:** trimmed to the lines that matter; the digests and SHAs are the real ones.

## Before

```text
Deployment frontend 19dbb11e-dd46-4a2a-9632-c4c16e3c0847
GitRepository apptique-examples 7e3910d0-60d9-46b7-b6ef-505f1fb3ead4
Kustomization apps c2fe3ae7-b20b-4755-bbdd-7b6b83b2684c
Kustomization infrastructure ae461df8-e876-4dd8-abd7-a6da83aeb32c
Namespace apptique-dev f003e129-ad8a-480f-8349-1754eb414e30
Pod frontend-d8bc775bd-9d6p5 a085867b-4980-4105-ac18-82836767d871
Service frontend f1006ad6-9999-4236-b6d5-9021adb0cc78
revision 1
```

## 1. handover.sh: suspends flux-system, the root takes the layers, then it pauses

```text
== 3/4 Hand the layers to the root, which reads them from ConfigHub
ocirepository.source.toolkit.fluxcd.io/confighub-root created
kustomization.kustomize.toolkit.fluxcd.io/confighub-root created
kustomization.kustomize.toolkit.fluxcd.io/confighub-root condition met
kustomization.kustomize.toolkit.fluxcd.io/infrastructure condition met
kustomization.kustomize.toolkit.fluxcd.io/apps condition met
PAUSED: every layer is the root's now, and flux-system is suspended. (main@sha1:31d96a9fa6b11218a5650dac18271ebd9a621edd still has gitops/flux/beginner/clusters/dev/infrastructure.yaml)
Make one commit in the repository flux-system reads, and push it:
  git rm gitops/flux/beginner/clusters/dev/infrastructure.yaml
  git rm gitops/flux/beginner/clusters/dev/apps.yaml
  cp bootstrap/dev/confighub-root.yaml <out>/fleet/gitops/flux/beginner/clusters/dev/confighub-root.yaml && git add gitops/flux/beginner/clusters/dev/confighub-root.yaml
Then run this script again. It resumes flux-system only once that commit is what flux-system reads.
Do NOT resume flux-system by hand before then: it would re-apply the layers from Git, and then delete them, and everything they run, when the commit lands.
To go back instead, in this order:
  # first, so that removing the root cannot delete the layers:
  kubectl --context 'kind-rh-boot' -n 'flux-system' patch kustomization confighub-root --type merge -p '{"spec":{"suspend":true,"prune":false}}'
  kubectl --context 'kind-rh-boot' -n 'flux-system' patch kustomization infrastructure --type merge -p '{"spec":{"sourceRef":{"kind":"GitRepository","name":"apptique-examples"},"path":"./gitops/flux/beginner/infrastructure/dev"}}'
  kubectl --context 'kind-rh-boot' -n 'flux-system' patch kustomization apps --type merge -p '{"spec":{"sourceRef":{"kind":"GitRepository","name":"apptique-examples"},"path":"./gitops/flux/beginner/apps/dev"}}'
  kubectl --context 'kind-rh-boot' -n 'flux-system' delete kustomization confighub-root
  kubectl --context 'kind-rh-boot' -n 'flux-system' delete ocirepository confighub-root
  kubectl --context 'kind-rh-boot' -n 'flux-system' patch kustomization flux-system --type merge -p '{"spec":{"suspend":false}}'   # its Git still defines the layers
exit 2
```

On the cluster while paused:
- `flux-system` was suspended.
- Both layers were owned by `confighub-root`.
- Every UID above was identical.

## 2. The commit, exactly as printed

```text
git rm gitops/flux/beginner/clusters/dev/infrastructure.yaml
git rm gitops/flux/beginner/clusters/dev/apps.yaml
cp .../bootstrap/dev/confighub-root.yaml gitops/flux/beginner/clusters/dev/confighub-root.yaml
2fe4c48 dev: layers come from ConfigHub through confighub-root
```

## 3. handover.sh again: resumes flux-system on that commit

```text
  continuing: flux-system was suspended by an earlier run
  the layers are applied by Kustomization flux-system from Git: it will be suspended while the root takes them over, and resumed once Git no longer defines them
  flux-system resumed at main@sha1:2fe4c48d58f86d6fd44d2625444a2daf61d676d7; every layer is still the root's
2026-09-30T09:47:08Z Ready apps at latest@sha256:a998f128abb06eb41f65140183ace5850e61702254753778a7a9ed18377cac48, from the root
2026-09-30T09:47:08Z flux-system reads main@sha1:2fe4c48d58f86d6fd44d2625444a2daf61d676d7: the layers are gone from its Git and the root is in it
2026-09-30T09:47:13Z resumed flux-system at main@sha1:2fe4c48d58f86d6fd44d2625444a2daf61d676d7
2026-09-30T09:47:14Z the root is now flux-system's, from Git
```

After forcing two more `flux-system` reconciles:

```text
infrastructure owner: confighub-root
apps owner: confighub-root
confighub-root owner: flux-system
flux-system inventory: confighub-root Kustomization, flux-system Kustomization, confighub-root OCIRepository
every UID and the rollout revision identical
```

## 4. The way back after the commit: revert, with the root's pruning off first

These were run in order:
1. Suspend `flux-system`.
2. Patch `confighub-root` to `suspend: true, prune: false`.
3. `git revert` the commit, then push. The revert is `d15be98`.
4. Resume `flux-system`.

```text
apps: owner=flux-system source=GitRepository/apptique-examples
infrastructure: owner=flux-system source=GitRepository/apptique-examples
confighub-root: NotFound (pruned by flux-system; its own pruning was off, so the layers stayed)
every UID and the rollout revision identical
```

## Found by this run, and fixed

**A rerun did not finish.** After the commit, the second run found the layers already the root's and took the plain path. It exited 0 with `flux-system` still suspended. Now:
- the first run records the owner it suspended, in `handover-state/<cluster>.owner`;
- a rerun carries on from that record.

A test covers it.

## Not covered

**`cub flux plan`, `check` and `status` after a bootstrapped handover.** They derive layers from the cluster directory. After the commit, that directory no longer defines this cluster's layers, so they need `--kustomization`, `--space` and `--unit` until they read the layers Space instead.
