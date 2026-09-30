# Run log: an in-cluster estate onboarded, status written back, handed back, 2026-09-30

An Argo CD estate with no cluster Secrets at all, onboarded from a live export,
handed over, given a reviewed release, and handed back. Kept as a record of what
was run and seen.

- **Argo CD:** v3.5.3 on kind, cluster `cs-argo`, installed from the release manifest.
- **ConfigHub:** v0.6.8, self-hosted, gateway over plain HTTP. `cub` v0.6.8, kustomize v5.8.1.
- **Estate:** this repository's `gitops/argo/beginner-applicationset`, applied by hand and syncing from GitHub `main`: one ApplicationSet with a git directory generator, two Applications, both deploying to Argo CD's own cluster.
- **Plugin:** built from this branch, reached as `cub argo` through a shim on `PATH`, so the installed plugin was not touched.
- **Output:** trimmed to the lines that matter; digests and UIDs are the real ones.

## Why this estate

Before this run, `plan` refused it: "no Argo CD cluster Secrets in the input".
An estate that deploys only to Argo CD's own cluster has none, because
`in-cluster` needs no Secret. And had the plan passed, `apply.sh` would have
failed at `cub variant create --target cs-targets/in-cluster`, a Target it never
created. Both are fixed, and this run is the proof.

## 1. Plan and apply.sh, from a live export

```text
$ { kubectl get applications,applicationsets,appprojects -n argocd -o yaml; echo '---';
    kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o yaml; } > export.yaml
$ cub argo plan export.yaml --prefix cs --repo-root <checkout>
Argo CD estate: 1 cluster, Argo CD's own (in-cluster), 1 components, 2 variants
    dev         variant cs-apptique-dev  ->  Target cs-targets/in-cluster
    prod        variant cs-apptique-prod  ->  Target cs-targets/in-cluster
Live: 2 generated Applications in the input, so handover is needed

$ bash apply.sh
== 1/5 One named Target per cluster, in cs-targets
== 4/5 One variant per cluster, each holding what its overlay renders to
Created variant space cs-apptique-dev
Created variant space cs-apptique-prod
Done. ConfigHub holds this estate and nothing reads it yet.
```

## 2. handover.sh: every field already matches

```text
== 2/5 Prove nothing on the clusters would change
  cs-apptique-dev holds what gitops/argo/beginner-applicationset/apps/apptique/overlays/dev renders today
apptique-dev: release 1 (sha256:329bae81...) holds apptique at revision 3
apptique-dev: 4 objects match what Argo owns
  and every field the release sets, on all 4 objects, already has that value on the cluster
apptique-prod: 4 objects match what Argo owns
  and every field the release sets, on all 4 objects, already has that value on the cluster
```

The ApplicationSet was applied by hand, so the script printed the two steps for
a person: set it to `applicationsSync: create-only`, then patch each Application
to its Space. Both were run as printed.

```text
apptique-dev=Synced/Healthy@sha256:329bae81ede827bd273aba5c7356d13d0e12975915d14e1a7bff2d317c25feed
apptique-prod=Synced/Healthy@sha256:d10f83e957fd34243f908fc79a41b8d552d26f31d8febf9285662ea201296fe4
EVERY UID UNCHANGED (13)
```

Each digest is the one step 2 checked. The 13 UIDs are the ApplicationSet, both
Applications, and every Deployment, Service, Pod and ConfigMap in both
namespaces.

## 3. cub argo status: what ConfigHub hears

```text
$ cub argo status export.yaml --prefix cs --repo-root <checkout> --kube-context kind-cs-argo
apptique-dev -> cs-apptique-dev: Synced/Healthy/Succeeded at sha256:329bae81...: release 1 synced (written; the Healthy gate would pass)
apptique-prod -> cs-apptique-prod: Synced/Healthy/Succeeded at sha256:d10f83e9...: release 1 synced (written; the Healthy gate would pass)

$ cub argo status ...   # again, nothing changed
apptique-dev -> cs-apptique-dev: ... (unchanged; the Healthy gate would pass)

$ cub space get cs-apptique-dev -o json | jq -r '.Space.Annotations["confighub.com/live-status"]'
{"source":"cub-argo","app":"apptique-dev","syncStatus":"Synced","healthStatus":"Healthy","operationPhase":"Succeeded","revision":"sha256:329bae81ede827bd273aba5c7356d13d0e12975915d14e1a7bff2d317c25feed","message":"release 1 synced","observedAt":"2026-09-30T17:22:23Z"}
```

## 4. A reviewed release, and the hard refresh

One change on the base (`replicas: 1` to `2`), through a change order: promote,
approve, publish to `cs-apptique-dev` as release 2.

```text
# 20 seconds after the publish; Argo still holds the digest it cached
apptique-dev -> cs-apptique-dev: OutOfSync/Healthy/Succeeded at sha256:329bae81...: release 1 is synced; release 2 is published and Argo CD has not read it ... (written; the Healthy gate would not pass)

$ cub argo status --application apptique-dev --space cs-apptique-dev --kube-context kind-cs-argo --hard-refresh
apptique-dev: asked Argo CD for a hard refresh, so it reads the newest release
Synced/Progressing@sha256:b293aec0c1bc584cdfe8a53df2d7654767099b4dfcf76b3257ae5e4ab12a83cb
Synced/Healthy@sha256:b293aec0c1bc584cdfe8a53df2d7654767099b4dfcf76b3257ae5e4ab12a83cb

apptique-dev -> cs-apptique-dev: Synced/Healthy/Succeeded at sha256:b293aec0...: release 2 synced (written; the Healthy gate would pass)
NAME       REPLICAS
frontend   2
```

So the gate stayed shut for as long as Argo ran the release before, and opened
on the digest of the release that was approved. Without the refresh, the
release would have sat unread, as the guide measured before.

## 5. The way back, for an ApplicationSet applied by hand

`handover.sh` used to say "restore its Unit" for every retired ApplicationSet.
One applied by hand has no Unit. Its way back is to undo `create-only`, and the
controller puts each Application back on the template's Git source:

```text
$ kubectl -n argocd patch applicationset apptique --type json -p '[{"op":"remove","path":"/spec/syncPolicy/applicationsSync"}]'
apptique-dev=https://github.com/confighub/examples.git|Synced/Healthy apptique-prod=https://github.com/confighub/examples.git|Synced/Healthy
EVERY UID UNCHANGED (14)
```

Status then replaced its own reading, so the gate does not go on passing on a
Space nothing reads:

```text
apptique-dev -> cs-apptique-dev: Unknown/Unknown: the Application no longer reads this Space: it reads https://github.com/confighub/examples.git, ... (written; the Healthy gate would not pass)
```

## What this run found

- The plan refused an in-cluster estate, and `apply.sh` could not have created
  its Target. Fixed; `TestInClusterEstateNeedsNoClusterSecret` holds it.
- A cluster generator with an empty selector left out Argo CD's own cluster,
  which the ApplicationSet controller includes. Fixed;
  `TestClusterGeneratorAndTheLocalCluster`.
- The way back named "step 5" in a script where retiring is step 3, and had no
  instruction for an ApplicationSet applied by hand. Fixed, and step 5 above is
  that instruction run.
- The `cub scout` cross-check reported `kube-root-ca.crt`, which Kubernetes puts
  in every namespace. It is filtered out.
- The first status message said Argo reads the gateway again "every few
  minutes". It does not: it caches the digest until a hard refresh. The
  message now says so.
