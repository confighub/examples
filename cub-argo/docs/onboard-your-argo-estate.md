# Onboard your Argo CD estate

You already describe your estate in Argo's own terms: a root Application, an
app of apps or two, ApplicationSets that generate one Application per cluster.
This guide turns that into a governed estate in ConfigHub, and the first two
commands change nothing at all.

It is deliberately two phases, because they carry very different risk:

```mermaid
flowchart LR
  p["cub argo plan<br/>offline, no account"] --> a["apply.sh<br/>fills ConfigHub<br/>no cluster touched"]
  a --> h["handover.sh<br/>moves each layer's source<br/>the only risky step"]
  a -.->|"cleanup.sh<br/>takes it back out"| p
```

**Onboarding** fills ConfigHub while Argo carries on syncing Git. Afterwards
ConfigHub holds a complete parallel copy that nothing reads, and `cleanup.sh`
— written beside `apply.sh` — takes it all back out. It is a script rather than
a line in this guide because the order is not guessable: a variant's Release
points at a Tag in its base Space, so the variants have to go before the bases
they were promoted from. Each Space then goes in one
`cub space delete --recursive --detach`.

**Handover** is the step that changes which source feeds your clusters, and it
is not undone by deleting Spaces — put each Application's source back to Git
first, which `cleanup.sh` checks before it does anything.

The `plan` command needs the `cub` CLI and plugin, but no login, `kustomize`,
or cluster access. To run the generated `apply.sh`, log into your organization
(`cub auth login`) and have `kustomize` on your PATH. To run `handover.sh`, you
also need `kubectl` access to the cluster Argo CD runs on and to each destination
cluster. The plugin lives in this repository rather than in one of its own, so
build it from a checkout (it needs Go):

```bash
git clone https://github.com/confighub/examples
cd examples/cub-argo
go build -o bin/cub-argo . && cub plugin install ./bin/cub-argo
cub plugin list   # argo should be listed, status ok
```

## The words you will meet

- **Layer**: one thing in your Argo tree that syncs a source — the root
  Application, an app of apps, an ApplicationSet. A handover repoints layers.
- **Base**: the shared render an app's overlays build on, stored once in
  ConfigHub. A change to the app is made here.
- **Variant**: a copy of the base for one cluster, holding only what that
  cluster's overlay renders differently.
- **Departure**: a field in which a variant differs from its base.
- **Space**: ConfigHub's folder for configuration. The base and each variant
  get one; your organization has a quota of them.
- **Component**: the group of one base and its variants. There is one per app.
- **Target**: a named destination, one per cluster, plus one for the cluster
  Argo CD itself runs on. A variant's releases go to its cluster's Target.
- **Change order**: one change moving through the stages, with an approval
  recorded in each. **Workflow**: the stages and what each waits for.

## 1. See the plan

Point it at your checkout:

```bash
cub argo plan . --stage-label rollout-phase --stages canary,secondary,primary
```

The plan runs offline, with no account and no cluster, and shows what ConfigHub
would hold. For the [expert example](../../gitops/argo/expert-app-of-apps/) in
this repository:

```text
Argo CD estate: 3 clusters, 3 components, 9 variants

Control tree (stays as it is: this is the management record)
  Application root                           wave   0  root, applied by hand
    AppProject platform                      wave -10  project
    AppProject storefront                    wave -10  project, 1 sync window
    ApplicationSet platform-addons           wave   0  generates 3 Applications
    Application storefront                   wave  10  app of apps
      ApplicationSet checkout-cache          wave   0  generates 3 Applications
      ApplicationSet apptique                wave   5  generates 3 Applications

Stages by rollout-phase: canary, secondary, primary

apptique  (ApplicationSet, owned by storefront, project storefront, wave 5)
  selects  clusters where storefront=true
  base     argo-apptique-base  reaches no cluster: its destination is empty
  stage canary
    dev-1       variant argo-apptique-dev-1  ->  Target argo-targets/dev-1
                Application dev-1-apptique, namespace storefront-dev
  ...
  in flight  nginx: 1.27-alpine on dev-1, staging-1; 1.26-alpine on prod-1
```

`plan` expands your generators the way Argo does, so those Application names
are the ones you already have, not a guess. It also exits non-zero when it
finds something to fix first — a cluster label that renders an overlay path
which does not exist, for instance, which Argo would only tell you about as a
failed sync.

The committed output for the example is
[`testdata/expert-app-of-apps.plan.txt`](../testdata/expert-app-of-apps.plan.txt),
so you can see the whole shape before running anything.

Leave out the stage options and every cluster is in one stage, `fleet`.

## 2. Write the steps, read them, run them

```bash
cub argo apply . --stage-label rollout-phase --stages canary,secondary,primary --out onboard
bash onboard/apply.sh
```

`apply` writes files and two scripts, and runs nothing. It checks `cub` is
logged in and `kustomize` is present, then:

1. Creates one Target per cluster, named for it, plus `argocd` for the cluster
   Argo runs on, all on a server-hosted worker.
2. Stores each app of apps' children as Units — your `AppProject`,
   `ApplicationSet` and child `Application` manifests, copied byte for byte, so
   reviewing what ConfigHub holds is reviewing what Argo reads today.
3. Renders each app's base with `kustomize build` and stores it, with the
   workflow its changes follow.
4. Creates each cluster's variant, bound to its Target, holding what that
   cluster's overlay renders.
5. Releases the first version stage by stage: promote, approve, publish. One
   operator does all three, as the workflow is generated (see "Making a change
   afterwards").

Nothing above touches a cluster. The whole script is safe to re-run: it picks
up where ConfigHub says each step stands, and never writes over a change made
in ConfigHub since.

## 3. Hand the estate over

This is the step that moves your clusters, so read it first:

```bash
ARGOCD_CONTEXT=<kubectl context of the cluster Argo CD runs on> \
  DEST_CONTEXT_prod_1=<kubectl context of prod-1> ... \
  bash onboard/handover.sh
```

One `DEST_CONTEXT_<cluster>` for each cluster Argo deploys to other than its
own; step 0 names any that are missing.

Each layer is **repointed**, never orphaned or deleted. That matters because an
app of apps does not own its children through `ownerReferences` — it owns them
by syncing a directory and pruning whatever is not in it. There is nothing to
sever, so repointing the parent is both the gentler move and the only one that
survives the parent's next reconcile.

Before, every layer reads Git:

```mermaid
flowchart LR
  r1["root"] -->|"creates"| s1["storefront"]
  s1 -->|"creates"| a1["ApplicationSet apptique"]
  a1 -->|"generates"| g1["dev-1-apptique<br/>staging-1-apptique<br/>prod-1-apptique"]
  r1 -.->|"syncs bootstrap/children"| git["Git repo"]
  s1 -.->|"syncs apps-of-apps/storefront"| git
  a1 -.->|"template reads apps/apptique/overlays/*"| git
```

After, the same layers read ConfigHub. Same objects, same names, three
sources moved, in this order:

```mermaid
flowchart LR
  r2["root"] -->|"creates"| s2["storefront"]
  s2 -->|"creates"| a2["ApplicationSet apptique"]
  a2 -->|"generates"| g2["dev-1-apptique<br/>staging-1-apptique<br/>prod-1-apptique"]
  r2 ==>|"1 · argo-root-children"| ch["ConfigHub"]
  s2 ==>|"2 · argo-storefront-children"| ch
  a2 ==>|"3 · one Target per cluster"| ch
```

The tree is the same on both sides. Only the three sources move, and in that
order, because a parent pointed at a Space that holds nothing is a parent
syncing an empty source — and with `prune: true` it deletes the children it
applied. `apply.sh` publishes each Space before `handover.sh` repoints the layer
that syncs it.

`root` is patched in the cluster, because nothing above it can repoint it under
review. `storefront` is a Unit by then, so its repoint is promoted and approved
like any other change. Each ApplicationSet is retired last, and
`move-applications.sh` then makes every Application it generated a Unit reading
its own Space, stage by stage, under the same name, so Argo's tracking does not
change and no workload is recreated.

**What the script checks before it changes anything.** That Argo CD is v3.1 or
newer, which is where an `oci://` source is read natively, and that your
AppProjects allow the gateway under `sourceRepos` — until they do, every repoint
is refused. Then two comparisons, and only the second can see your cluster:

| Question | What it compares | What it can catch |
|---|---|---|
| Does ConfigHub hold what the overlay renders? | `kustomize build` against `cub unit data` | Git moving since `apply.sh` ran. Both sides come from Git, so nothing more |
| Does ConfigHub hold what Argo actually owns? | `Application.status.resources` against the release | An object on the cluster that Git has never described. Where Argo prunes, that object is **deleted** when the source moves |

The second is `cub argo check`, and it runs for every variant before anything
is repointed. It reads Argo's own record — not an inference from labels — and
`requiresPruning` is Argo's own answer to what it would delete. Objects the
release holds but Argo does not own are named too, as additions. Sync hooks are
a note, since Argo runs rather than holds them.

Afterwards the script asks `cub scout map list -q "owner=Native ..."` for what
no controller claims in those namespaces: things applied by hand, which neither
record mentions and which a handover leaves behind. That one is a cross-check,
not a gate, and cub-scout's absence is not a failure.

**The repository Secret, verified against Argo CD v3.5.3.** The Application's
`repoURL` and the Secret's `url` both need the `oci://` scheme — without it Argo
treats the address as a git repository and fails with `list refs: invalid auth
method`. The Secret's `type` is `oci`, and the worker is the username and
password:

```yaml
stringData:
  type: oci
  url: oci://<gateway>/space/<space>
  username: <the Targets' server worker id>
  password: <its secret>
  # only for a gateway served over plain HTTP:
  insecureOCIForceHttp: "true"
```

**Nothing is deleted.** `root` and `storefront` carry
`resources-finalizer.argocd.argoproj.io`, which deletes everything they
deployed. Every step is a patch for exactly that reason.

**The way back is printed, leaves first.** Before `handover.sh` patches `root`,
it records `root`'s source as it was, in `handover-state/argo.txt`. After the
repoint it checks that the digest `root` synced is the control Space's newest
release, and stops if not. If the run stops anywhere after `root` moved, it
prints the way back, in this order, and rolls nothing back by itself. It prints
the same at the end of a clean run.

1. Restore any retired ApplicationSet's Unit to the revision before
   `create-only`, and publish. The controller then puts its Applications back
   on the template's Git source.
2. Restore the Unit of any app of apps repointed through it, and publish.
3. Patch `root` back to its recorded source, on the context the run used.

Moving `root` back alone also puts everything back, with every UID intact. It
was run live on 2026-09-30. But for a moment it restores the Git AppProjects
while a generated Application still reads the gateway, and that Application is
refused until it moves too. Undoing from the bottom avoids that.

## What the plugin checks for you, and what it cannot

Three of these run on your behalf. The fourth is yours, and the plan says so
rather than pretending otherwise.

```mermaid
flowchart TB
  a["Does ConfigHub hold what the overlay renders?<br/>kustomize build vs cub unit data"] --> b["Does ConfigHub hold what Argo owns?<br/>Application.status.resources vs the release"]
  b --> c["Has the Workshop Catalog decided about each chart?<br/>a verdict per chart, version and values base"]
  c --> d["Do your values leave that verdict's scope?<br/>partly checkable, the rest is yours"]
```

**Would the swap change the cluster?** `cub argo check` reads
`Application.status.resources` — Argo's own record of what it reconciles, not
an inference from labels — and compares it with what the release holds. It names
any object Argo owns that the release lacks, and says what would happen to it:

```text
prod-1-apptique: 3 objects match what Argo owns
  ServiceAccount storefront-prod/frontend: Argo owns it and the release does
  not hold it, and this Application prunes, so it would be DELETED from the cluster
```

Whether that reads DELETED or "left on the cluster, managed by nothing" comes
from the Application's own `spec.syncPolicy.automated.prune`. `handover.sh`
runs this for every variant before anything moves, passing the cluster's
context so it reads the estate being handed over rather than whichever context
happens to be current.

**You run it the way you ran `plan`.** `check` takes the same repository
directory and works the estate out for itself — there is no per-Application
flag to get right, and no list to keep in step with the repository:

```bash
cub argo check ./my-estate --stage-label rollout-phase --stages canary,secondary,primary
```

```text
prod-1-apptique: 3 objects match what Argo owns
staging-apptique: 3 objects match what Argo owns
...
9 of 9 clean
```

It exits non-zero if any Application is not clean, so it drops into CI as it
is. `--kube-context` names the cluster to read; without it kubectl uses
whatever context is current, which during a handover is very likely the wrong
one.

**Has the chart been audited?** Where an overlay inflates a Helm chart, the
plan reports what the Workshop Catalog has already decided about it — one
verdict per chart, version **and values base**, because a verdict is not a
property of a chart. The Catalog's own data makes the point: `cloudpirates/redis`
0.34.11 is `unsafe-to-flatten` on its default base and `safe-to-flatten` on
`reuse-existing-secret`, because the default leaves `auth.existingSecret` unset,
so the Secret template renders, mints a password and fires a lookup.

A chart the Catalog has not audited is reported as undecided, and a verdict for
another version is reported as not carrying. Neither reads as safe.

**Do your own values leave that verdict's scope?** Each verdict names the values
changes that take a variant out of it. Where the scope names a path and you set
it, the plan says the verdict does not carry. Much of the scope is written for a
reader — "authentication or TLS enabled", "certificates supplied from outside
the render" — and that part is handed back to you explicitly rather than passed
over in silence.

## Three questions, not one: objects, identities, fields

A handover is safe when swapping the source changes nothing. "Nothing" breaks
into three questions, and each needs a different thing to be read.

```mermaid
flowchart TB
  q1["**Same objects?**<br/>status.resources vs the release"] --> a1["catches an object the release<br/>would add, or that Argo would prune"]
  q2["**Same identities?**<br/>metadata.uid, before and after"] --> a2["catches a delete-and-recreate<br/>wearing the same name"]
  q3["**Same values?**<br/>every field the release sets,<br/>vs the live object"] --> a3["catches a hand edit the object<br/>set cannot see"]
```

The first two are the object-set check and the UID comparison the handover
rehearsal runs. The third is `--fields`, and it is the one that sees a person.

```bash
cub argo check ./my-estate --fields
```

For every object the release holds, it reads the live object and compares
**only the fields the release sets**. That restriction is the whole design:
Kubernetes fills in defaults a release never mentions and controllers own
others, so comparing everything the cluster holds reports noise as drift. It is
the same rule a reconciler applies.

Each difference is reported with Kubernetes' own record of who has written that
object:

```text
Deployment apptique-dev/frontend .spec.replicas: cluster has 4, the release
holds 2 (written on this object by kubectl-scale, argocd-controller)
```

**It checks the release, not the head.** A handover delivers a published
release, and a unit changed since the last publish holds something the cluster
will not get. So `check` reads the newest published release, or the one
`--release sha256:...` names, finds the revision of the unit it bundled, and
compares that. It prints the release number and manifest digest, and says so
when the unit's head has moved past it. `handover.sh` records each digest
before checking it, and prints it beside each repoint, so the digest Argo then reports in
`status.sync.revision` can be compared with the one that was checked.

**`--record` keeps the verdict in ConfigHub.** `cub argo check --fields
--record` writes each Application's verdict as a `LiveCheck` attestation on the Unit
revision the checked release bundled. A clean check is a Pass. Anything that
differs is a rejection that names it: an object the release would add or prune,
a field, or an object it could not read. The claims name the Application and the
release digest. `LiveCheck` is the type `cub kubara check --record` uses for
the same claim, so a workflow can require one type whichever plugin checked. It
needs `--fields`: a claim that the cluster runs this release rests on every
field the release sets. The recording is the same code as `cub flux`,
which was run live on 2026-09-30.

**It reads each object where it runs.** The Application is read on the cluster
Argo CD runs on, and the objects it deploys on the cluster it deploys them to.
Those are the same cluster only when the destination is Argo's own
(`in-cluster`). For any other, pass that cluster's context:

```bash
cub argo check ./my-estate --cluster prod-1 --fields \
  --kube-context <Argo CD's cluster> --destination-context <prod-1's context>
```

The check accepts that context only if kubectl reaches it at the address the
Application deploys to. A kind cluster, say, is `127.0.0.1:<port>` from your
laptop and something else from inside Argo, and then you say which destination
it is with `--destination <server>`. Without either, it refuses rather than read
the management cluster, where an object of the same name would be compared and
could pass. `handover.sh` asks for one `DEST_CONTEXT_<cluster>` per cluster up
front and passes the destination the plan found.

An object the check cannot read, because it is forbidden, timed out or the
cluster is unreachable, is listed and counted: the check says how many of the
release's objects it compared, and reading fewer than all of them is never
clean.

`managedFields` is that record, and `kubectl get -o json` **strips it** unless
asked — the plugin passes `--show-managed-fields`, without which attribution
comes back silently empty. The managers are not ordered by time: kubectl leaves
the timestamp off some writes, so naming a last writer would be a guess where a
fact belongs.

**Measured on a live cluster**, Argo CD v3.5.3, with a `kubectl scale` against
an Application whose `selfHeal` was off:

- The object sets matched exactly. The inventory check passed, correctly.
- `--fields` named `.spec.replicas`, both values, and `kubectl-scale` among the
  managers.
- `kubectl scale` took **sole** ownership of `f:replicas` via the scale
  subresource — `argocd-controller` no longer held that field at all. That
  subresource records no timestamp, which is why the ordering claim was dropped.
- `argocd-controller` writes with `Update`, not `Apply`: client-side apply, not
  server-side.
- Argo CD itself reported `OutOfSync`/`Healthy` once its refresh had run. Argo
  does notice a hand edit — the point of `--fields` is not that Argo is blind,
  but that Argo compares the app to **Git** and reports one status, while this
  compares it to **the release you are about to hand it**, per field, with a
  name attached.

`cub scout compare` remains the broader tool: it works across an estate without
a plan, and infers ownership where nothing declares it. `handover.sh` asks
cub-scout what no controller claims in your namespaces, as a cross-check rather
than a gate.

## ApplicationSets are retired, not repointed

An ApplicationSet generates one Application per cluster from **one shared
template**, so it cannot give each cluster its own Space by editing that
template with a literal address. But ConfigHub already holds your fleet
enumerated — one variant Space per cluster, staged — so by handover time the
generator has nothing left to generate.

```mermaid
flowchart TB
  subgraph before["Before: generated"]
    as["ApplicationSet<br/>one template"] -->|"generates"| a1["dev-1-apptique"]
    as --> a2["staging-1-apptique"]
    as --> a3["prod-1-apptique"]
  end
  subgraph after["After: enumerated, one Space each"]
    b1["dev-1-apptique"] --> s1["argo-apptique-dev-1"]
    b2["staging-1-apptique"] --> s2["argo-apptique-staging-1"]
    b3["prod-1-apptique"] --> s3["argo-apptique-prod-1"]
  end
  before -->|"retire the generator,<br/>keep every Application"| after
```

This is the same move `cub sveltos` makes when it drops a profile's
`clusterSelector` for a `clusterRefs` naming one cluster. A new cluster stops
producing an Application on its own and becomes a variant you add in ConfigHub —
a reviewed change rather than an automatic one, which is the point.

**Measured on Argo CD v3.5.3**, and each of these changes what the script does.
The first measurements used one cluster and a path swap. The sequence has since
been run end to end against real variant Spaces on two registered clusters; see
"Run against a live estate on three clusters" below.

- **Patching a generated Application while its ApplicationSet is live is
  reverted in under a second.** The generator stands down first, with
  `spec.syncPolicy.applicationsSync: create-only`; the controller then creates
  but never updates or deletes what it made.
- **With `create-only`, the patch holds**, the Application genuinely re-syncs to
  the new source, and its **UID does not change**.
- **Applications then move one cluster at a time.** Verified: one Application
  repointed while its sibling stayed where it was — the staged rollout a shared
  template cannot express.
- **`create-only` is not enough once an Application stands alone.** Measured on
  2026-09-30: `create-only` stops the controller updating the Applications it
  owns, but one that no longer carries its ownerReference is, to the
  controller, one it has yet to create, and it "creates" it back to the
  template, source and owner included, within seconds. So a retired
  ApplicationSet also generates nothing: its generators become
  `[{list: {elements: []}}]`. `create-only` never deletes, so every
  Application it made stays where it is. `handover.sh` step 5 prints both edits,
  to its Unit, and `move-applications.sh` refuses to start until both are live.

### Each Application becomes a Unit

Once the ApplicationSets are retired, `move-applications.sh` gives each variant
its delivery object in ConfigHub — the shape `cub variant create` gives a
variant on a `cub cluster up` cluster: one Application Unit, named after the
variant's Space, in the Space the parent reads.

```bash
ARGOCD_CONTEXT=<context> CONFIGHUB_OCI=<gateway> bash onboard/move-applications.sh canary
# check it, then
ARGOCD_CONTEXT=<context> CONFIGHUB_OCI=<gateway> bash onboard/move-applications.sh secondary
```

For each Application in the stage it reads the Application from the cluster and
stores it (`cub argo application-unit`) as a Unit in the control Space that holds
its ApplicationSet: the same name, project, destination, labels, finalizer and
sync policy, the source pointed at the variant's Space and nothing else, and two
sync options:

- `Prune=false`, so no parent ever deletes it — not when its Unit is removed on
  the way back, and not when the parent is pointed back at Git, which does not
  hold it. Deleting an Application with the resources finalizer deletes its
  workloads.
- `Replace=true`, so the parent replaces the Application with its Unit rather
  than merging into it. Measured: `checkout-cache`'s template sets
  `source.kustomize.version: v5`, which a merge would have kept, and Argo would
  then have tried to build the rendered bundle as a kustomization. A replace is
  an update, so the UID stays; it also takes off the ApplicationSet's
  ownerReference, so the Application stands on its own.

Then it publishes that control Space, waits for the parent to apply it, and
checks each Application reads its Space, has synced that Space's newest release,
and is the same object it was (by UID). A stage whose Space has no release yet
is refused before anything is made. The way back is printed at the end and if it
stops: take the Unit out and publish — the Application stays, as `Prune=false`
says — then put its recorded source back: all of it, from
`handover-state/source-<application>.json`, since a template can set more than
the repository, path and revision.

**Run live on 2026-09-30,** Argo CD v3.5.3, ConfigHub v0.6.8: canary, then
secondary, six Applications across `dev-1` and `staging-1`, including the
Helm-inflated `checkout-cache`. Every Application and every Deployment, Service
and ConfigMap on both clusters kept its UID. The way back was run for one
Application: with its Unit removed `root` reported it `requiresPruning` and left
it; patched back it synced Git again, and the retired generator left it alone.
Moving it again picked up where it was.

### Removing a retired ApplicationSet later

The retired ApplicationSets stay in place, inert. Removing one is a separate,
later decision, and **the obvious way to do it destroys the estate**:

> Measured: a generated Application carries an `ownerReference` to its
> ApplicationSet with `blockOwnerDeletion: true`. Deleting the ApplicationSet
> let Kubernetes garbage-collect every Application it made, and Argo then
> removed their workloads and their namespaces.
> **`preserveResourcesOnDeletion: true` did not prevent this.** It was set, and
> everything went anyway.

Strip the `ownerReferences` first and the Applications stand on their own —
measured, both Applications and the Deployment kept their UIDs:

```bash
kubectl -n argocd patch application dev-1-apptique --type json \
  -p '[{"op":"remove","path":"/metadata/ownerReferences"}]'   # every generated Application
kubectl -n argocd delete applicationset apptique             # only then
```

`cleanup.sh` prints this for each retired ApplicationSet, naming the exact
Applications, in the order that does not delete them. An Application
`move-applications.sh` has moved carries no ownerReference already (its Unit
replaced it without one), so only those not yet moved need the patch.

## What the Argo handover has been through

Rehearsed on Argo CD v3.5.3 against a self-hosted ConfigHub v0.6.2. Both
parents — `root` and `storefront` — now read ConfigHub, and **every UID is
unchanged**: the AppProjects, the ApplicationSet and the child Application are
the same objects they were before.

Eleven bugs came out of that, and none of them would have been found by reading
the code. The ones worth knowing about as an operator:

**`sourceRepos` cannot be widened with `kubectl`.** The AppProjects are
themselves synced by the root Application, so a patch is reverted and the
repoint is refused again with no sign of why. Commit it where Argo reads it.
The allow-entry also needs `/**`, not `/*` — Argo's glob does not cross `/`,
and the gateway address has two path segments.

**The credential is a `repo-creds` Secret, not a `repository` one.** Argo
matches a `repository` Secret to an Application by url, and these repoURLs carry
a `/space/<space>` path that `oci://<host>` does not match. The credential was
silently ignored, Argo fell back to anonymous **https**, and it failed with
`cannot get digest for revision latest` over a scheme nobody had asked for.
`repo-creds` is the prefix form: one credential for every Space under the
gateway.

**A control Space needs its own release Target and a published Release.**
Without one, the parent repointed at it reads tag `latest` from a repository
that has none, and fails with `<space>:latest: not found` — which reads as a
wrong address rather than as an empty Space.

**A child app of apps is repointed in ConfigHub, not on the cluster.** Once its
parent reads ConfigHub, the parent owns it: a `kubectl patch` of the child is
applied and then reverted on the next reconcile, and it looks like it worked for
about a minute. `handover.sh` now prints the `cub unit update` and
`cub release publish` to run instead — which is the reviewed path, and is the
point of the child being a Unit.

```mermaid
flowchart TB
  a["root repointed<br/>with kubectl"] --> b["root now syncs<br/>argo-root-children"]
  b --> c["storefront is a Unit there"]
  c --> d["patch storefront with kubectl<br/>= reverted on next reconcile"]
  c --> e["cub unit update + release publish<br/>= flows down, and is reviewed"]
```

**Waiting for `Synced` on a parent is the wrong gate.** A parent reports
`OutOfSync` while any child still differs, and during a handover its children
are exactly what is being moved. The script now waits for Argo to have *read*
the new source — `sync.status` leaving `Unknown` — and says so.

**A file that will not parse is now a refused plan, not a footnote.** This one
cost two live AppProjects. A broken indent in `projects.yaml` meant the plugin
skipped it, reported it only as a count under a banner reading "not Kubernetes
YAML (such as Helm templates)", and the control Space was built without it. The
repointed parent, which prunes, then deleted both AppProjects from the cluster.
`plan` now names every skipped file, and refuses outright when one sits in a
directory an app of apps syncs:

```text
Problems to fix first
  - bootstrap/children/projects.yaml sits beside objects an app of apps syncs, and
    could not be read as Kubernetes YAML. It would be missing from the Space that
    replaces that directory, and the parent prunes, so whatever it defines would be
    DELETED from the cluster at handover.
```

**Not covered by this first rehearsal:** the ApplicationSet-generated
Applications, since it had one cluster. They are covered by the three-cluster
run below. A TLS gateway is still untested: every run here used a plain-HTTP
gateway, with `CONFIGHUB_OCI_PLAIN_HTTP`.

**Run against a live estate on three clusters.** On 2026-09-30: Argo CD v3.5.3
on a management kind cluster, two workload clusters registered as `dev-1`
(canary) and `staging-1` (secondary) with the example's labels, the example's
`root` synced from GitHub, and a self-hosted ConfigHub v0.6.8. The plan was made
from a live `kubectl get` export, with the cluster Secrets' credentials removed.

| What | Result |
| --- | --- |
| `apply.sh` from the live export | 2 clusters, 3 components, 6 variants, control Spaces for `root` and `storefront` |
| Step 4, all six generated Applications | each read on its own cluster, clean, against its release's digest |
| `root`, then `storefront`, repointed | both read their control Spaces; `storefront` through a reviewed Unit change |
| `apptique` retired through its Unit, then both Applications repointed | the stock controller did not revert either through repeated reconciles |
| Each Application's `status.sync.revision` | equal to the release digest step 4 checked |
| Every UID on both clusters, every Application's UID, every rollout revision | identical before, after, and after the rollback |

That run found four things, now fixed or written down:

- **A live export planned `root` and `storefront` as ordinary components**,
  aimed at an `in-cluster` Target nothing creates, and `apply.sh` failed. The
  plan found an app of apps' children by file, and an exported object has none.
  It now reads the directory the parent syncs and matches its children by kind
  and name, so a live export gives the same control tree as the repository.
- **`handover.sh` re-rendered a Helm-in-Kustomize overlay without
  `--enable-helm`**, which `apply.sh` passed. Both use the same rule now.
- **Roll back from the bottom.** Repointing `root` back to Git put everything
  back, as the tree is owned top down, and every UID survived. But it also put
  back the Git copy of the AppProjects, without the gateway in `sourceRepos`,
  while a generated Application still read the gateway, which was briefly
  refused. Leaves first, then parents, avoids that.
- **The example needs two Argo CD settings its README does not name:**
  `kustomize.buildOptions: --enable-helm` and a registered `kustomize.path.v5`
  in `argocd-cm`, for `checkout-cache`.

One step was stood in for. `sourceRepos` has to allow the gateway before any
repoint, and the AppProjects came from GitHub `main`, which a rehearsal cannot
commit to. The change was made in the `projects` Unit of the root control Space,
which is what `apply.sh` would have captured from that commit, and on the cluster
with `root`'s `selfHeal` briefly off.

## A published release does not arrive on its own

This one is measured, and it surprised us. After `handover.sh`, an Application
reads `oci://<gateway>` at `targetRevision: latest` — and Argo **caches the
digest it resolved for that tag**. On Argo CD v3.5.3 a newly published release
was still unread ninety seconds later: the Application sat at the previous
digest, and the cluster ran the previous replica count.

A hard refresh re-resolves the tag to a digest, and the release lands at once:

```bash
kubectl -n argocd annotate application <name> argocd.argoproj.io/refresh=hard --overwrite
```

```mermaid
flowchart LR
  p["cub release publish"] --> g["ConfigHub gateway<br/>new digest under :latest"]
  g -.->|"Argo does not notice:<br/>the old digest is cached"| a["Application"]
  g ==>|"hard refresh<br/>re-resolves tag to digest"| a
  a --> c["the cluster"]
```

**[argobot](https://github.com/confighub/argobot) does this for you, and reports
back.** `argobot.sh` runs it beside Argo CD with the Targets' server worker as
its identity, the one `cub cluster up` gives it, so it acts for exactly these
Targets:

- on each `release.published` it hard-refreshes the Applications reading that
  Space, so an approved release reaches the cluster at once;
- it writes each Application's live state — sync, health, operation, the digest
  it synced — back to the Space it reads, as `confighub.com/live-status`, which
  the ConfigHub UI and the Healthy gate read.

Both find an Application by the Space its source reads, since a moved estate
keeps Argo's names (`dev-1-apptique` reads `argo-apptique-dev-1`). The status
does that in every argobot. The refresh needs confighub/argobot#14, which no
argobot release has yet: one without it looks only for an Application named
after the Space, so `argobot.sh` installs a release that reports status and
says, when it finishes, that releases still wait for Argo's poll or the
annotation above. Set `ARGOBOT_VERSION` to a release with #14 once there is one.

**Run live on 2026-09-30,** argobot built with #14, against the estate above:
it wrote live status for all eight Applications; a change released to canary
reached `dev-1` in 2 seconds, and to `staging-1` in 2 more once promoted and
approved, against the 90 seconds and more above. With three Targets it also
stopped twice on a `409` from ConfigHub, at its first start and after half an
hour, when its polls collided; that is confighub/argobot#15. In a cluster its
Deployment restarts it, and a missed refresh costs only immediacy.

Without argobot, or without that annotation in whatever promotes your releases,
an approved release sits unread on the gateway and the approval gate you built
governs nothing.

## Making a change afterwards

ConfigHub now holds each app's base, so a change starts there. It is made once
and moves through the stages with an approval in each:

```bash
cub unit data --space argo-apptique-base apptique > apptique.yaml
# edit it, then store it on the base
cub unit update --space argo-apptique-base apptique apptique.yaml \
  --change-desc "Raise the frontend to 4 replicas"
cub changeorder create --space argo-apptique-base more-replicas \
  --change-workflow argo-apptique-base/rollout --description "Raise replicas"

cub variant promote --change-order argo-apptique-base/more-replicas --target-stage canary
cub variant approve --change-order argo-apptique-base/more-replicas --stage canary
cub release publish argo-apptique-dev-1 --revision ChangeOrder:argo-apptique-base/more-replicas
```

ConfigHub refuses to promote into the next stage until this one has released
the change, and refuses each release until the change is approved in its stage.
Both refusals come from the server, in its own words. As generated, though, one
person can give that approval; the paragraph below the diagram says what that
does and does not show.

```mermaid
flowchart LR
  b["base<br/>one reviewed edit"] --> c1["canary<br/>dev-1"]
  c1 -->|"released, then approved"| c2["secondary<br/>staging-1"]
  c2 -->|"released, then approved"| c3["primary<br/>prod-1"]
```

The generated workflow is a single-operator one. It declares the `approval`
attestation with `AllowAuthors: true`, and `apply.sh` promotes, approves and
publishes as the same actor. That is a reviewed workflow for one person trying
this: every step is recorded and the server still refuses to skip a stage. But
an approval recorded that way is not evidence that a separate reviewer looked at
the change.

To require one, use what `cub changeworkflow create --help` describes for an
attestation prerequisite in a workflow file. `AllowAuthors: false` (by default
an author of the change does not count) stops the person who wrote the change
approving it. `Count: 2` asks for that many distinct users recording a pass.
`FromUserIDs: [<user id>]` names who may approve, and `MaxAge: 72h` lets an
approval lapse. Reviewers record theirs with `cub variant approve
--change-order <space>/<order> --stage <stage>`, and `--reject --note "<why>"`
records a refusal, which blocks. Edit the `<component>/change-workflow.yaml`
that `cub argo apply` wrote and put it on the live workflow with `cub
changeworkflow update --space <base Space> rollout --filename
<component>/change-workflow.yaml`; it applies to change orders created afterwards, not to
one already under way. This has not been run here. The help shows no other
enforcement and does not say what stops one person holding two logins, so the
separation still comes from who holds which credentials: the person running
`apply.sh` should not be a user who can approve.

## When a cluster joins

Register the cluster with Argo and label it as you always have. Nothing is
generated for it: its ApplicationSets are retired. Plan and apply again with a
fresh export of the cluster Secrets, and re-run `apply.sh`:

```bash
kubectl get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o json \
  | jq '.items[] |= (del(.data.config) | del(.metadata.annotations."kubectl.kubernetes.io/last-applied-configuration"))' \
  > clusters.json
cub argo plan . clusters.json --stage-label rollout-phase --stages canary,secondary,primary
```

Strip both. The plan reads names, servers and labels only, but a Secret made
with `kubectl apply` keeps its plain `stringData`, bearer token included, in the
last-applied annotation, so removing `.data.config` alone leaves the token in
the file. Measured.

The plan shows the new cluster's variants. `apply.sh` leaves every existing
variant as it is, clones the new one from the base as the base stands today,
including every change made since, and releases it through its stages with an
approval in each. Then `move-applications.sh <its stage>` makes its
Applications: there is none on the cluster to read, so each is made like its
sibling's Unit (the same component on another cluster), with the name,
destination and stage label the plan works out from the template. Joining is a
reviewed change rather than a side effect of a label.

**Run live on 2026-09-30:** `prod-1` registered after the retirement got no
Application from Git; `apply.sh` released its three variants; `move-applications.sh
primary` made its three Applications, and `prod-1-cluster-baseline` synced
Healthy. The two in the `storefront` project waited, correctly, for the
example's own deny window on `prod-1-*` to close; the script says so.

A change still in flight when a cluster joins stops `apply.sh`: a change order
across variants at different revisions of a Unit is refused ("the targets are
at different revisions of it"). Finish or abandon the change first.

## What this leaves alone

- The `argocd` install itself, and anything Argo does not manage.
- Cluster Secret credentials. The plan reads names, servers and labels only.
- AppProject RBAC, and sync windows: those stay runtime settings in Argo. The
  plan reports which of your Applications a window covers, and `handover.sh`
  says when a repoint's first sync will wait for one to close.
- Sync waves. ConfigHub's stages own the rollout order between clusters; waves
  still order objects within a sync.
- Generators it does not read offline: git, SCM provider, pull request, merge
  and plugin. The plan names them rather than guessing.
- Multi-source Applications' paths, and anything needing Sprig functions.

## What has and has not been checked

Nothing in this guide is claimed without something having run.

**Checked here, offline:** every render in the example's script runs against
kustomize 5.8.1 and gives the same bytes twice over. Both generated scripts are
valid bash. `plan` reads every Argo example in this repository. One command,
`scripts/verify-gitops-plugins.sh`, runs all of that and has twice caught
defects nobody was looking for.

**Checked on a live cluster:** `cub argo check` was run against Argo CD v3.5.3
on kind, syncing this repository's `beginner-applicationset` example, with the
render stored in ConfigHub. It reported four matching objects; with one object
removed from the release it caught the difference and exited non-zero.

That rehearsal found two bugs no test had:

- The message said an object would be "left on the cluster, managed by nothing"
  when the Application prunes and Argo would have **deleted** it. The code read
  the per-resource `requiresPruning` flag, which describes what is out of sync
  *now* — and with everything in sync it is absent on every entry, which is
  exactly the state a handover is checked in. It reads
  `spec.syncPolicy.automated.prune` instead.
- The check read whichever kubectl context happened to be current. On a fleet
  handed over cluster by cluster, it could have reported on one cluster while
  another was being changed, and passed. It now takes `--kube-context`, and
  `handover.sh` passes it.

Every unit test passed throughout both bugs, because the fakes were written
from the same wrong beliefs as the code.

**The delivery path has now been run end to end.** A ConfigHub release was
published to the OCI gateway, an Argo CD v3.5.3 Application was pointed at it,
and it reported `Synced` and `Healthy` at the exact manifest digest
`cub release list` showed. The workloads came up. Two things were learned that
way and are now in the script and this guide: the repository Secret's shape,
and that a second release does not arrive without a hard refresh.

**Since run:** a handover of an estate *already live under Argo*, repointing
Applications that were syncing from Git, including ones an ApplicationSet
generated. See "Run against a live estate on three clusters" above. And on
2026-09-30, `move-applications.sh` through three stages, its way back, a
cluster joining after the retirement, and argobot refreshing and reporting
status: see "Each Application becomes a Unit", "A published release does not
arrive on its own" and "When a cluster joins".

**Not claimed at all:** that a plain directory of manifests can be onboarded
(`kustomize build` will not read one, though Argo will — the plan says so),
that git, SCM-provider, pull-request, merge or plugin generators are resolved,
or that an Application whose source is a Helm chart, from a chart repository or
a chart kept in the repository, can be governed yet (the plan says so).
