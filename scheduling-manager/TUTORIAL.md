# Tutorial: managing workload placement with `cub-scheduling`

This walks `cub-scheduling` end to end against a live ConfigHub fleet: see where
workloads are allowed to land, find the placement anti-pattern, fix it as data,
and stand up enforcement. Every command and block of output below is real.

`cub-scheduling` reads and writes ConfigHub Units — it never touches a cluster.
Its writes create new Unit revisions; rolling those out is a separate `cub unit
apply`. Read commands default to JSON; add `-o table` for the human view used
here.

## Prerequisites

- A ConfigHub session — `cub auth login`.
- The binary: `make build` produces `bin/cub-scheduling`. It also runs as a `cub`
  plugin (`cub scheduling …`); this tutorial uses the `cub-scheduling` form.

## 1. Verify the session

```console
$ cub-scheduling preflight
cub-scheduling: ready (ConfigHub session valid)
```

## 2. See where a workload lands

`placement` reports each workload's `nodeSelector`, `tolerations`, and node
affinity, and whether it is **constrained** — i.e. whether it actually restricts
which nodes it lands on. Here `web` pins nothing:

```console
$ cub-scheduling placement -o table --cluster nsmgr-proto-m3
CLUSTER         NAMESPACE  KIND        NAME  NODESELECTOR  TOLERATIONS  NODEAFFINITY  CONSTRAINED
nsmgr-proto-m3  m3app      Deployment  web   -             -            none          no

1 workloads (0 constrained, 1 unconstrained)
```

(`snapshot` gives per-cluster counts; `list` is the workload explorer.)

## 3. Create — and detect — the anti-pattern

Suppose we let `web` tolerate the GPU taint but forget to pin it to the GPU pool.
Writes are dry-run by default; `--commit --change-desc` writes the revision:

```console
$ cub-scheduling set-tolerations nsmgr-proto-m3/web --toleration nvidia.com/gpu:NoSchedule \
    --commit --change-desc "Tolerate gpu taint"
set-tolerations: 1 of 1 Unit(s) changed
```

Now `findings` flags it — a toleration only *permits* scheduling onto a tainted
node; without a matching `nodeSelector` or node affinity the pod may still land on
general nodes:

```console
$ cub-scheduling findings -o table --cluster nsmgr-proto-m3
SEVERITY  ANALYZER   CLUSTER         NAMESPACE  KIND        NAME  MESSAGE
MEDIUM    placement  nsmgr-proto-m3  m3app      Deployment  web   tolerates taint(s) [nvidia.com/gpu] but has no nodeSelector or required node affinity — may schedule onto general nodes

1 findings (0 high, 1 medium, 0 low)
```

## 4. Fix it as data — with a profile

Profiles are named placement edits stored as ConfigHub Invocations. Seed the
library once, then list it:

```console
$ cub-scheduling profile install
Space scheduling-profiles ready
Profile scheduling-profiles/placement-gpu ready
Profile scheduling-profiles/placement-spot ready
Profile scheduling-profiles/node-pool ready

$ cub-scheduling profile list -o table
PROFILE         FUNCTION  PARAMS  DESCRIPTION
node-pool       set-yq    pool    set-yq: pin nodeSelector.pool to the given pool (param: pool)
placement-gpu   set-yq    -       set-yq: pin to the gpu node pool + tolerate the nvidia.com/gpu taint
placement-spot  set-yq    -       set-yq: pin to the spot node pool + tolerate the spot taint
```

Pin `web` to the gpu pool with the parameterized `node-pool` profile:

```console
$ cub-scheduling profile apply node-pool nsmgr-proto-m3/web --param pool=gpu \
    --commit --change-desc "Pin to gpu pool"
profile apply node-pool: 1 of 1 Unit(s) changed
```

(For one workload you can also use `set-node-selector nsmgr-proto-m3/web
--selector pool=gpu` directly, or `set-node-affinity … --required
"topology.kubernetes.io/zone=us-east-1a,us-east-1b"`.)

## 5. Verify

`web` is now constrained, and the finding is gone:

```console
$ cub-scheduling placement -o table --cluster nsmgr-proto-m3
CLUSTER         NAMESPACE  KIND        NAME  NODESELECTOR  TOLERATIONS     NODEAFFINITY  CONSTRAINED
nsmgr-proto-m3  m3app      Deployment  web   pool=gpu      nvidia.com/gpu  none          yes

1 workloads (1 constrained, 0 unconstrained)

$ cub-scheduling findings -o table --cluster nsmgr-proto-m3

0 findings (0 high, 0 medium, 0 low)
```

## 6. Fleet-wide

To place many workloads at once, `fleet-edit` applies a profile across a
`--where` selector in one operation (dry-run first to see the blast radius):

```console
$ cub-scheduling fleet-edit --profile placement-gpu --environment prod --component ml
fleet-edit placement-gpu: N of N Unit(s) would change (dry-run — pass --commit --change-desc to write)
```

`promote --component ml` then carries a placement change from a base Space to its
downstream environment/region variants (override-preserving).

## 7. Enforce with guardrails

`guardrails install` stands up a `Warn=true` `vet-cel` rule
(`workload-toleration-needs-placement`) and wires in-scope Spaces to it. It's
dry-run by default and conservative — it skips Spaces that already select Triggers
their own way:

```console
$ cub-scheduling guardrails install -o table        # preview
$ cub-scheduling guardrails install --commit        # apply
$ cub-scheduling guardrails status -o table         # Units now carrying warnings
```

Promote the rule to blocking later with
`cub trigger update workload-toleration-needs-placement --space scheduling-policy --unwarn`.

## Reference

- **Scoping** any command: `--where "<expr>"` (Unit / `Space.*` / `Target.*`,
  flat `AND`-only) plus shorthands `--component` / `--environment` / `--region` /
  `--owner` / `--layer` / `--variant`. `--cluster` / `--namespace` are client-side
  display filters.
- **Discipline**: reads default to JSON (`-o table` for humans); writes are
  dry-run until `--commit --change-desc`; nothing is applied to a cluster (roll
  out with `cub unit apply` separately); ApplyGates are never bypassed.
- **Placement vs availability**: pod anti-affinity and topology spread live in
  [`workload-manager`](../workload-manager), not here.
