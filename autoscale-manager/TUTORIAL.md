# Tutorial: managing autoscaling with `cub-autoscale`

This walks `cub-autoscale` end to end against a live ConfigHub fleet: see what
autoscales, find the anti-patterns, edit an HPA as data, convert it to KEDA, and
stand up enforcement. Every command and block of output below is real.

`cub-autoscale` reads and writes ConfigHub Units — it never touches a cluster. Its
writes create new Unit revisions; rolling those out is a separate `cub unit
apply`. Read commands default to JSON; add `-o table` for the human view used here.

## Prerequisites

- A ConfigHub session — `cub auth login`.
- The binary: `make build` produces `bin/cub-autoscale`. It also runs as a `cub`
  plugin (`cub autoscale …`); this tutorial uses the `cub-autoscale` form.

## 1. Verify the session

```console
$ cub-autoscale preflight
cub-autoscale: ready (ConfigHub session valid)
```

## 2. Inventory the fleet

`snapshot` gives per-cluster counts: HPAs, KEDA ScaledObjects, scalable workloads
(and how many are actually autoscaled), and PodDisruptionBudgets.

```console
$ cub-autoscale snapshot -o table
CLUSTER                                 HPAS  SCALEDOBJECTS  WORKLOADS  AUTOSCALED  PDBS  UNITS
cluster-worker-kubernetes-yaml-cluster  0     0              1          0           0     6
dev-cluster                             0     0              19         0           0     33
nsmgr-proto-apptique                    0     0              1          0           1     8
nsmgr-proto-m3                          1     1              1          1           0     9
prod-cluster                            0     0              19         0           0     38

5 clusters, 1 HPAs, 1 ScaledObjects, 41 workloads (1 autoscaled), 1 PDBs
```

`list` is the autoscaler explorer — kind, scale target, bounds, and whether it's
**pinned** (`min == max`, so it can't scale):

```console
$ cub-autoscale list -o table --where "Slug LIKE '%hpa%'"
CLUSTER         NAMESPACE  KIND          NAME  TARGET          MIN  MAX  PINNED  UNIT
nsmgr-proto-m3  m3app      HPA           web   Deployment/web  5    15   no      autoscale-test-hpa
nsmgr-proto-m3  m3app      ScaledObject  web   web             2    10   no      web-hpa

2 autoscalers
```

## 3. Find the anti-patterns

`findings` ranks issues most-severe first, over ConfigHub Units only:

- **autoscaler-pinned** — an HPA/ScaledObject with `min == max`.
- **pdb-blocks-min-scale** — the cross-resource check: a PodDisruptionBudget whose
  `minAvailable` is `>=` the autoscaler's `minReplicas`, so at minimum scale no pod
  may be voluntarily evicted (node drains stall).
- **no-autoscaler** — a Deployment/StatefulSet nothing autoscales.

```console
$ cub-autoscale findings -o table --min-severity medium
No autoscaling findings.

$ cub-autoscale findings -o table --cluster dev-cluster | head -4
SEVERITY  ANALYZER       CLUSTER      NAMESPACE  KIND        NAME      MESSAGE
low       no-autoscaler  dev-cluster  appchat    Deployment  backend   no HorizontalPodAutoscaler or ScaledObject targets this workload
low       no-autoscaler  dev-cluster  appchat    Deployment  frontend  no HorizontalPodAutoscaler or ScaledObject targets this workload
```

## 4. Edit an HPA as data

`set-hpa` edits an HPA's replica bounds and cpu/memory targets. Writes are dry-run
by default; `--commit --change-desc` writes the revision:

```console
$ cub-autoscale set-hpa nsmgr-proto-m3/autoscale-test-hpa --min 3 --max 20 --cpu 65 -o table
UNIT                MUTATED  ERROR
autoscale-test-hpa  yes      -

set-hpa: 1 of 1 Unit(s) would change (dry-run — pass --commit --change-desc to write)

$ cub-autoscale set-hpa nsmgr-proto-m3/autoscale-test-hpa --min 3 --max 20 --cpu 65 \
    --commit --change-desc "raise bounds to 3-20, cpu 65%" -o table
UNIT                MUTATED  ERROR
autoscale-test-hpa  yes      -

set-hpa: 1 of 1 Unit(s) changed
```

For reusable edits, the profile library stores named, parameterized set-yq
Invocations:

```console
$ cub-autoscale profile install
Space autoscale-profiles ready
Profile autoscale-profiles/hpa-conservative ready
Profile autoscale-profiles/hpa-aggressive ready
Profile autoscale-profiles/hpa-range ready

$ cub-autoscale profile list -o table
PROFILE           FUNCTION  PARAMS   DESCRIPTION
hpa-aggressive    set-yq    -        set-yq: pack tighter — cpu target 85% average Utilization (fewer replicas)
hpa-conservative  set-yq    -        set-yq: scale out early — cpu target 60% average Utilization (more headroom)
hpa-range         set-yq    min,max  set-yq: set minReplicas/maxReplicas (params: min, max)

$ cub-autoscale profile apply hpa-range nsmgr-proto-m3/autoscale-test-hpa \
    --param min=5 --param max=15 --commit --change-desc "apply hpa-range 5-15" -o table
UNIT                MUTATED  ERROR
autoscale-test-hpa  yes      -

profile apply hpa-range: 1 of 1 Unit(s) changed
```

## 5. Convert HPA → KEDA

`convert-keda` rewrites an HPA as an equivalent KEDA ScaledObject — preserving
`min`/`max` and cpu/memory metrics — by running the `convert-hpa-to-keda` function
in an embedded executor **in-process** (no server-side function, no worker). Dry
run prints the result:

```console
$ cub-autoscale convert-keda nsmgr-proto-m3/autoscale-test-hpa -o table
Dry run — would convert the HPA in nsmgr-proto-m3/autoscale-test-hpa to a KEDA ScaledObject:

apiVersion: keda.sh/v1alpha1
kind: ScaledObject
metadata:
  name: web
  namespace: m3app
spec:
  scaleTargetRef:
    name: web
  minReplicaCount: 5
  maxReplicaCount: 15
  triggers:
  - type: cpu
    metricType: Utilization
    metadata:
      value: "65"

Re-run with --commit --change-desc "…" to write the revision. It is not applied until you apply it.
```

The Unit is edited, not applied; deploying a KEDA ScaledObject also requires the
KEDA operator installed in the target cluster. Only cpu/memory Resource metrics are
translated — Pods/Object/External metrics need a matching KEDA scaler and are left
for you to add.

## 6. Fleet-wide

To change many autoscalers at once, `fleet-edit` applies a profile across a
`--where` selector in one operation (dry-run first to see the blast radius):

```console
$ cub-autoscale fleet-edit --profile hpa-conservative --where "Slug = 'autoscale-test-hpa'" -o table
UNIT                MUTATED  ERROR
autoscale-test-hpa  yes      -

fleet-edit hpa-conservative: 1 of 1 Unit(s) would change (dry-run — pass --commit --change-desc to write)
```

`promote --component <c>` then carries an autoscaling change from a base Space to
its downstream environment/region variants (override-preserving).

## 7. Enforce with guardrails

`guardrails install` stands up two `Warn=true` Triggers in an `autoscale-policy`
Space and wires in-scope Spaces to them. It's dry-run by default and conservative —
it skips Spaces that already select Triggers their own way:

```console
$ cub-autoscale guardrails install --commit -o table
Policy pack ready in autoscale-policy.
Applied — policy pack "autoscale-policy", filter "autoscale-policy/autoscale-guardrails"
  triggers: autoscaler-not-pinned, schema-valid
  ...
```

- `autoscaler-not-pinned` (`vet-cel`) flags any HPA/ScaledObject with `min == max`.
- `schema-valid` (`vet-schemas`) validates every mutation against the Kubernetes/CRD
  schema catalog — this is the **post-convert check** for `convert-keda`, since
  `keda.sh` is in the catalog. A committed ScaledObject passes:

```console
$ cub function do --space nsmgr-proto-m3 --where "Slug = 'web-hpa'" vet-schemas
  Passed: true  Function: vet-schemas
```

`guardrails status -o table` lists Units now carrying warnings. Promote a rule to
blocking later with
`cub trigger update autoscaler-not-pinned --space autoscale-policy --unwarn`.

## Reference

- **Scoping** any command: `--where "<expr>"` (Unit / `Space.*` / `Target.*`,
  flat `AND`-only) plus shorthands `--component` / `--environment` / `--region` /
  `--owner` / `--layer` / `--variant`. `--cluster` / `--namespace` (on read
  commands) are client-side display filters.
- **Discipline**: reads default to JSON (`-o table` for humans); writes are
  dry-run until `--commit --change-desc`; nothing is applied to a cluster (roll
  out with `cub unit apply` separately); ApplyGates are never bypassed.
- **HPA → KEDA** runs client-side in an embedded executor; deploying ScaledObjects
  needs the KEDA operator in the target cluster.
