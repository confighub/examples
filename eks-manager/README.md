# cub-eks — EKS cluster manager for ConfigHub

`cub-eks` manages **AWS EKS clusters as data**: the control plane, node capacity,
addons, access entries, and the VPC and IAM resources a cluster needs — stored as
ConfigHub Units containing **Crossplane managed resources**, across a whole fleet
of cluster-Spaces. It is designed for use by an AI agent in a terminal, and is a
sibling of [`workload-manager`](../workload-manager),
[`namespace-manager`](../namespace-manager),
[`rbac-manager-for-agents`](../rbac-manager-for-agents), and
[`network-policy-manager`](../network-policy-manager).

**Why not just eksctl?** The command shape here is deliberately eksctl's — `create
cluster`, `upgrade cluster`, `scale nodegroup`. The substrate is not. eksctl is a
generator plus an imperative executor: its config file is an input to a one-shot
run, and the real state lives in CloudFormation stacks. A Crossplane managed
resource *is* the desired state, continuously reconciled. ConfigHub adds what
neither has — a versioned, fleet-queryable, promotable, gated record. Two things
fall out of that for free: **a control-plane upgrade becomes a promotion**
(dev → staging → every prod region, override-preserving), and **a nodegroup scale
becomes a Unit mutation** with a diff and a revision.

**Why a manager, not just `cub unit update`?** Because of one sharp edge.
Terraform destroys and recreates a resource when an immutable field changes.
Crossplane *refuses*: it returns `refuse to update the external resource because
the following update requires replacing it`, and retries forever. So editing
`instanceTypes` on a nodegroup leaves you with a committed revision, a clean
diff, a successful apply — and **nothing happening, permanently**. Every signal
reads as success while the record and reality silently diverge. `cub-eks plan`
grades each pending change as in-place, rolling, or a replacement, so that is
caught at the source of record instead of in a controller log nobody is reading.
For a Crossplane managed resource, immutable fields are *identity*, not
configuration — so changing one is handled as a blue/green replacement, which
eksctl has no command for at all.

## Status

This build ships **M0 (skeleton)**, **M1 (fleet snapshot and read commands)**,
**`create cluster`**, **M2 (`plan` and `findings`)**, and **M3b (`upgrade`,
`scale`, `create nodegroup|addon`)**. Commands marked *(planned)* are designed
but not yet implemented; see the milestone list.

| Command | Kind | What it does |
|---|---|---|
| `preflight` / `version` | diag | Verify the ConfigHub session; print version |
| `snapshot` | read | Per-cluster inventory of EKS, EC2, and IAM resources |
| `list` | read | Enumerate managed resources across the fleet |
| `get` | read | The assembled view of one cluster |
| `versions` | read | Fleet version matrix: control planes, node-group skew, addon versions |
| `status` | read | *(planned)* Intent vs reality: MR conditions and `status.atProvider` |
| `plan` | read | Grade pending changes: in-place / rolling / replacement |
| `consistency` | read | *(planned)* Cross-region and cross-environment drift |
| `findings` | read | Severity-ranked findings across all analyzers |
| `create cluster` | write | Generate the full envelope of Units for a new cluster |
| `upgrade cluster` | write | Staged control-plane upgrade with EKS ordering enforced |
| `upgrade nodegroup` | write | Roll a node group to a new version |
| `scale nodegroup` | write | Set min / max / desired size |
| `create nodegroup` / `create addon` | write | Add a node group or addon to an existing cluster |
| `replace-nodegroup` | write | Blue/green replacement for immutable-field changes |
| `fleet-edit` | write | One field change across every EKS resource matching a selector |
| `promote` | write | Carry upstream config downstream, preserving local overrides |
| `guardrails install` / `status` | write | Validating-Trigger pack; show gates and warnings |
| `attributes install` | write | *(planned)* Reference / disruption path registration |

Milestones: **M0** skeleton ✅ · **M1** snapshot + read ✅ · **M3a**
`create cluster` ✅ · **M2** change-impact classifier + findings ✅ · **M3b**
upgrade + scale + generation ✅ · **M4** blue/green replacement, fleet ops,
guardrails ✅ · **M5** adoption of existing clusters.

All read commands will default to JSON (`-o json`); pass `-o table` for a human
view. Write commands are **dry-run by default** and require `--commit
--change-desc`; they create and edit Units but never apply to a cluster (that is
a separate `cub unit apply`), and never bypass ApplyGates.

## Scoping the fleet

Read commands take one flat ConfigHub Unit filter plus label shorthands, all
`AND`-combined:

| Flag | Compiles to |
|---|---|
| `--where` | used verbatim |
| `--cluster` | `Space.Labels.Cluster = '<value>'` |
| `--component` | `Space.Labels.Component = '<value>'` |
| `--environment` | `Space.Labels.Environment = '<value>'` |
| `--region` | `Space.Labels.Region = '<value>'` |
| `--owner` / `--layer` / `--variant` | `Space.Labels.Owner` / `.Layer` / `.Variant` |

ConfigHub `where` is flat **`AND`-only — no parentheses, no `OR`**. Note that
`--cluster` is server-side here, unlike in the sibling managers where it filters
on the Target slug client-side: for `cub-eks` **a cluster is a Space**, because
its Units *describe* a cluster rather than deploy to one.

## Model

A cluster is a **Space** (`eks-<name>-<region>`), labelled `Cluster`, `Region`,
`Environment`, `Provider=aws`. Inside it, **one managed resource per Unit** — so
a nodegroup scale never shares a revision, a diff, or an ApplyGate with the
control plane.

The Space's Target is the **Crossplane management cluster**: ConfigHub publishes
the Units, ArgoCD syncs them there, and Crossplane reconciles them into AWS. The
resulting EKS cluster then becomes a Target for application Spaces — so the
fleet's own topology is config-as-data too.

Resources are addressed by ConfigHub ResourceType, which is `group/version/Kind`
— e.g. `eks.aws.upbound.io/v1beta2/Cluster`. Targeting is
[`crossplane-contrib/provider-upjet-aws`](https://github.com/crossplane-contrib/provider-upjet-aws)
v2.6.x, on the object-shaped `v1beta2` API (not the deprecated list-shaped
`v1beta1`). **EKS Auto Mode is the default** for node capacity — it removes the
`NodeGroup` resource entirely, and with it a class of autoscaler conflicts that
is currently unfixable upstream.

## Grading a pending change

`plan` is the command this tool exists for.

```bash
cub-eks plan -o table
cub-eks plan --blocking-only        # only what cannot be applied at all
```

Terraform destroys and recreates a resource when an immutable field changes.
**Crossplane refuses** — it returns `refuse to update the external resource
because the following update requires replacing it`, the managed resource goes
`Synced=False`, and it retries forever while AWS is never touched. So bumping a
node group's `instanceTypes` gives you a committed revision, a clean diff, and a
successful apply — and then nothing happens, permanently. Every signal reads as
success while the record and reality silently diverge.

`plan` compares each Unit's head revision against its **last-applied** revision
and grades every changed field:

| Grade | Meaning |
|---|---|
| `in-place` | an ordinary update |
| `in-place-disruptive` | in place, but service-affecting (control-plane upgrade, addon restart) |
| `rolling` | in place, but EKS drains and replaces every node in the group |
| `replace` | the resource must be destroyed and recreated — **will not apply** |
| `replace-cluster` | the EKS control plane must be replaced — **will not apply** |

Units that have never been applied are reported and not graded: with no baseline
there is nothing to disrupt, because creating a resource is not replacing one.

## Upgrading and scaling

Every write command grades its own change through the same classifier `plan`
uses, and **refuses** anything that cannot be reconciled in place — so a
node-group replacement is caught before it becomes a committed-but-inert
revision, not after.

```bash
cub-eks scale nodegroup <space>/<unit> --nodes-min 3 --nodes-max 12
cub-eks upgrade nodegroup <space>/<unit> --to 1.35
cub-eks upgrade cluster <cluster> --to 1.35
```

`upgrade cluster` produces the **ordered** plan EKS requires, and enforces two
constraints nothing downstream checks:

- **One minor at a time, never backwards.** The CRD marks `version` as an
  ordinary optional field with no validation and the provider passes it through,
  so an illegal transition becomes an `InvalidParameterException` and a
  permanently unsynced managed resource rather than an error you would see.
- **Node groups already behind the current control plane are caught up first.**
  Advancing the control plane would strand them further behind than EKS
  supports, and node groups also move one minor at a time — so the command
  refuses, and prints the catch-up commands to run first.

It writes only the control-plane stage; the node-group stages each drain and
replace every node in their group, so they are yours to run once the control
plane is healthy.

## Replacing a node group

When `plan` reports a change as `replace`, this is the way out:

```bash
cub-eks replace-nodegroup <space>/<unit> --instance-types m6i.2xlarge
```

A node group's name is its identity in AWS, so the replacement gets a new one
(`system` → `system-v2`) and both coexist for a controlled swap. The command
creates the replacement Unit and stops — it does not apply anything, drain
anything, or delete anything, and it prints the remaining steps for you to run.
Draining is a converging imperative loop over live pod state, and is out of scope
by design.

It refuses when nothing immutable actually changes, pointing you at `scale` or
`upgrade` instead. And if the original Unit already carries a committed
immutable edit — the usual reason you are here — it says so, because creating
the replacement does not un-wedge the original.

eksctl has no command for this; for self-managed node groups it is a documented
manual procedure.

## Fleet operations and guardrails

```bash
cub-eks fleet-edit --kind Cluster --path spec.forProvider.upgradePolicy.supportType --value EXTENDED
cub-eks promote --environment prod
cub-eks guardrails install
cub-eks guardrails status
```

`fleet-edit` grades the path before touching anything and **refuses** an
immutable one outright — a fleet-wide immutable change is not a bulk edit, it is
a bulk silent failure. `promote` is the override-preserving upgrade that makes a
fleet-wide version bump a promotion rather than N independent edits.

`guardrails install` creates validating Triggers in a policy Space and wires them
to every cluster Space, skipping any Space that already has its own Trigger
configuration rather than clobbering it. Rules ship **advisory** (`Warn=true`,
so failures attach an ApplyWarning); promote one to blocking with `cub trigger
update <slug> --space eks-policy --unwarn`. Gate-versus-warning lives on the
Trigger, not the rule, so the same pack can advise in dev and block in prod.

## Creating a cluster

```bash
# Dry run: see the 34 Units that would be generated.
cub-eks create cluster prod-use1 --region us-east-1 --version 1.34 --environment prod -o table

# Write them into Space eks-prod-use1.
cub-eks create cluster prod-use1 --region us-east-1 --version 1.34 --environment prod \
  --commit --change-desc "Create prod-use1 EKS cluster"

cub-eks get prod-use1 -o table
```

Auto Mode is the default; `--auto-mode=false` generates classic managed node
groups plus the baseline addons instead. Other knobs: `--zones` / `--zone-count`,
`--vpc-cidr`, `--nat single|per-az|none`, `--public-endpoint`, `--orphan`.

Two things worth knowing about the output:

- **Availability zones are pinned into the Units.** eksctl randomizes AZ
  selection per invocation, so the same config yields different infrastructure.
  Here the same input always produces the same output.
- **Auto Mode's `computeConfig.nodeRoleArn` is written as `confighubplaceholder`.**
  It is the one value that cannot be a reference — the provider gives it no
  Ref/Selector — so it must be supplied once the node Role exists, or up front
  via `--auto-node-role-arn`. `vet-placeholders` blocks apply until it is filled,
  which is the intended behavior rather than a gap. Classic mode has no
  placeholder, since node groups use `nodeRoleArnRef`.

Adding to an existing cluster later uses the same generator, so a node group
created now is byte-identical to one the cluster was created with:

```bash
cub-eks create nodegroup prod-use1 batch --capacity-type SPOT --nodes-min 0 --nodes-max 20
cub-eks create addon prod-use1 aws-efs-csi-driver
```

Both read the cluster's region, version, environment, and deletion policy from
its existing control plane, so the new resource matches. Both warn rather than
block when the cluster runs Auto Mode — a managed node group alongside Auto Mode
is legal but rarely intended, and Auto Mode already provides `vpc-cni`,
`coredns`, `kube-proxy`, and the EBS CSI driver.

Nothing is applied to a cluster. Rolling out is a separate `cub unit apply`.

## Build

```bash
make build      # -> bin/cub-eks
make test
```

Run standalone as `cub-eks ...`, or install it as a cub plugin and run it as
`cub eks ...` — the behavior is identical. ConfigHub I/O uses your existing cub
session; run `cub auth login` first if you are not signed in, then check with:

```bash
bin/cub-eks preflight
```

## Prior art

[`vm-fleet`](../vm-fleet) is the existing Crossplane-on-AWS example in this repo
— upbound `ec2` / `autoscaling` / `iam` managed resources stored as Units and
mutated with `cub run set-string-path --resource-type ...`. `cub-eks` follows its
authoring conventions, with one deliberate difference: `vm-fleet` packs two
managed resources into a single multi-document Unit, while `cub-eks` keeps one
resource per Unit.
