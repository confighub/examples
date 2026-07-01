# Tutorial: managing workload posture with `cub-workload`

This walks through `cub-workload` end to end against a live ConfigHub fleet: find
the workloads that aren't production-ready, fix them as data, apply a fix across a
whole environment, and stand up enforcement. Every command and every block of
output below is real.

`cub-workload` **reads and writes ConfigHub Units** — it never touches a cluster.
Its writes create new Unit revisions; rolling those out is a separate, deliberate
`cub unit apply`. Read commands default to JSON; add `-o table` for the human view
used here.

## Prerequisites

- A ConfigHub session — `cub auth login` (an interactive browser sign-in).
- The binary: `make build` produces `bin/cub-workload`. It also runs as a `cub`
  plugin (`cub workload …`); this tutorial uses the `cub-workload` form.
- Some Kubernetes/YAML workload Units in your org. (The output here is from a
  five-cluster demo fleet.)

## 1. Verify the session

Every command that talks to ConfigHub runs this gate first; you can run it alone:

```console
$ cub-workload preflight
cub-workload: ready (ConfigHub session valid)
```

If it fails, run `cub auth login` and retry.

## 2. See the fleet

`snapshot` is the per-cluster inventory — workloads, PodDisruptionBudgets, and how
many Units are gated or unapplied. Clusters are ConfigHub Targets (the Space slug
stands in for unbound Units); canonical base/policy Spaces are excluded.

```console
$ cub-workload snapshot -o table
CLUSTER                                 WORKLOADS  PDBS  UNITS  GATED  UNAPPLIED
cluster-worker-kubernetes-yaml-cluster  1          0     6      0      1
dev-cluster                             19         0     33     0      20
nsmgr-proto-apptique                    1          1     8      0      8
nsmgr-proto-m3                          1          0     7      0      7
prod-cluster                            19         0     38     0      24

5 clusters, 41 workloads, 1 pdbs, 92 units (0 gated, 60 unapplied)
```

## 3. Score readiness

`readiness` scores every workload across five dimensions — **security**,
**resources**, **probes**, **hygiene**, **availability** — as pass / warn / fail,
with the worst as the overall. In table mode, warn+fail workloads list their exact
issues underneath.

```console
$ cub-workload readiness -o table --cluster dev-cluster --namespace appvote
CLUSTER      NAMESPACE  KIND        NAME    SECURITY  RESOURCES  PROBES  HYGIENE  AVAILABILITY  OVERALL
dev-cluster  appvote    Deployment  db      fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  redis   fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  result  fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  vote    fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  worker  fail      pass       fail    warn     pass          fail

5 workloads (0 pass, 0 warn, 5 fail)
  dev-cluster/appvote Deployment/db:
    [hygiene] container "postgres" terminationMessagePolicy is not FallbackToLogsOnError
    [probes] container "postgres" no liveness probe
    [probes] container "postgres" no readiness probe
    [security] automountServiceAccountToken is not false
    [security] container "postgres" allowPrivilegeEscalation is not false
    [security] container "postgres" does not drop ALL capabilities
    [security] container "postgres" does not set runAsNonRoot: true
    [security] container "postgres" readOnlyRootFilesystem is not true
    [security] container "postgres" seccompProfile is not RuntimeDefault
  ...
```

Narrow to one dimension with `--dimension security|resources|probes|hygiene|availability`,
or show only problems with `--failing-only`.

## 4. Triage with findings

`findings` flattens the whole scorecard into one severity-ranked list — the "what
to fix first" view. A failing security / resources / availability check is `high`;
a failing probe is `medium`; warnings are one step down; hygiene is `low`.

```console
$ cub-workload findings -o table --cluster dev-cluster --namespace appvote --severity high
SEVERITY  ANALYZER  CLUSTER      NAMESPACE  KIND        NAME    MESSAGE
HIGH      security  dev-cluster  appvote    Deployment  db      container "postgres" allowPrivilegeEscalation is not false
HIGH      security  dev-cluster  appvote    Deployment  db      container "postgres" does not set runAsNonRoot: true
HIGH      security  dev-cluster  appvote    Deployment  redis   container "redis" allowPrivilegeEscalation is not false
HIGH      security  dev-cluster  appvote    Deployment  redis   container "redis" does not set runAsNonRoot: true
HIGH      security  dev-cluster  appvote    Deployment  result  container "result" allowPrivilegeEscalation is not false
HIGH      security  dev-cluster  appvote    Deployment  result  container "result" does not set runAsNonRoot: true
HIGH      security  dev-cluster  appvote    Deployment  vote    container "vote" allowPrivilegeEscalation is not false
HIGH      security  dev-cluster  appvote    Deployment  vote    container "vote" does not set runAsNonRoot: true
HIGH      security  dev-cluster  appvote    Deployment  worker  container "worker" allowPrivilegeEscalation is not false
HIGH      security  dev-cluster  appvote    Deployment  worker  container "worker" does not set runAsNonRoot: true

10 findings (10 high, 0 medium, 0 low)
```

Filter with `--severity`, `--analyzer`, and the scope flags below.

## 5. Availability — PodDisruptionBudget coverage

`availability` is the disruption-survival view for **multi-replica** workloads: is
there a PodDisruptionBudget whose selector matches the workload's pods (a *cross-Unit*
join — the PDB and the workload are separate Units), does that PDB block all
evictions, and is pod anti-affinity / topology spread present?

```console
$ cub-workload availability -o table --cluster nsmgr-proto-apptique
CLUSTER               NAMESPACE  KIND        NAME  REPLICAS  PDB      BLOCKS-EVICT  SPREAD
nsmgr-proto-apptique  apptique   Deployment  web   2         web-pdb  no            yes

1 multi-replica workloads: 0 uncovered (no PDB), 0 block all evictions, 0 without spread
```

This is the check a per-object validator can't do inside ConfigHub: wired as a
per-Unit Trigger under one-resource-per-Unit, it would see the workload Unit alone
and couldn't tell whether a matching PDB exists elsewhere.

## 6. Fix one workload

Every write command is **dry-run by default**. Run it, read the plan, then re-run
with `--commit --change-desc` to write the revision.

Preview hardening the `vote` deployment (security-context + automount defaults):

```console
$ cub-workload harden appvote-dev/vote -o table
UNIT  MUTATED  ERROR
vote  yes      -

harden: 1 of 1 Unit(s) would change (dry-run — pass --commit --change-desc to write)
```

Commit it. The change description should carry a summary and the verbatim user
request:

```console
$ cub-workload harden appvote-dev/vote --commit \
    --change-desc "Apply security-context + automount defaults to vote

Tutorial walkthrough." -o table
UNIT  MUTATED  ERROR
vote  yes      -

harden: 1 of 1 Unit(s) changed
```

Verify — `vote`'s **security** dimension is now `pass`, while its unhardened
siblings still fail:

```console
$ cub-workload readiness -o table --cluster dev-cluster --namespace appvote
CLUSTER      NAMESPACE  KIND        NAME    SECURITY  RESOURCES  PROBES  HYGIENE  AVAILABILITY  OVERALL
dev-cluster  appvote    Deployment  db      fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  redis   fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  result  fail      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  vote    pass      pass       fail    warn     pass          fail
dev-cluster  appvote    Deployment  worker  fail      pass       fail    warn     pass          fail
```

`vote` is still `fail` overall because it has no probes — keep going with the other
per-workload fixes, each dry-run then commit:

```console
$ cub-workload set-resources appvote-dev/vote --tier medium      # cpu/memory requests + limits
$ cub-workload set-probes    appvote-dev/vote                    # liveness/readiness/startup
$ cub-workload ensure-pdb    appvote-dev/vote                    # author a matching PDB Unit
$ cub-workload ensure-spread appvote-dev/vote                    # soft pod anti-affinity
```

`ensure-pdb` reads the workload's pod labels and prints the PDB it would author:

```console
$ cub-workload ensure-pdb nsmgr-proto-apptique/web
{
  "action": "create-pdb",
  "dryRun": true,
  "space": "nsmgr-proto-apptique",
  "namespace": "apptique",
  "unit": "web-pdb",
  "manifest": "apiVersion: policy/v1\nkind: PodDisruptionBudget\nmetadata:\n  name: web-pdb\n  namespace: apptique\nspec:\n  maxUnavailable: 1\n  selector:\n    matchLabels:\n      app.kubernetes.io/name: web\n"
}
```

Re-runs are idempotent: a command that changes nothing reports `0 of 1 Unit(s)
would change`.

## 7. Profiles and fleet-wide remediation

Hardening one workload at a time doesn't scale. **Profiles** are named,
parameterized edits stored as ConfigHub Invocations; `fleet-edit` applies one
across a whole selector in a single server-side operation.

Seed the library once per org, then list it:

```console
$ cub-workload profile install
Space workload-profiles ready
Profile workload-profiles/resources-small ready
...
Profile workload-profiles/termination-message-policy ready

$ cub-workload profile list -o table
PROFILE                     FUNCTION                                     PARAMS     DESCRIPTION
anti-affinity-soft          set-yq                                       -          set-yq: preferred pod anti-affinity across nodes (selector from pod-template labels)
harden-restricted           set-pod-container-security-context-defaults  -          set-pod-container-security-context-defaults (runAsNonRoot, seccomp, drop ALL, readOnlyRootFilesystem)
probes-http                 set-container-probe-defaults                 -          set-container-probe-defaults (HTTP liveness/readiness/startup on the first port)
resources-large             set-container-resources                      container  set-container-resources requests 500m/512Mi, limits ×2
resources-medium            set-container-resources                      container  set-container-resources requests 250m/256Mi, limits ×2
resources-small             set-container-resources                      container  set-container-resources requests 100m/128Mi, limits ×2
termination-message-policy  set-yq                                       -          set-yq: terminationMessagePolicy: FallbackToLogsOnError on all containers
```

Harden every workload in `dev` in one operation — dry-run first to see the blast
radius (here, `vote` is already hardened, so it's 17 of 19, not 19):

```console
$ cub-workload fleet-edit --profile harden-restricted --environment dev -o table
...
fleet-edit harden-restricted: 17 of 19 Unit(s) would change (dry-run — pass --commit --change-desc to write)
```

Commit with `--commit --change-desc "…"`. A profile that takes a parameter (the
resource tiers take `container`) is supplied with `--param`:

```console
$ cub-workload fleet-edit --profile resources-medium --environment dev --param container='*' \
    --commit --change-desc "Set medium resource tier across dev"
```

## 8. Promote a fix downstream

When a fix is authored in a base Space and cloned into environment/region variant
Spaces, `promote` carries it forward with an override-preserving upgrade (keeping
each Space's local customizations). Scope it with the same flags:

```console
$ cub-workload promote --component checkout            # dry-run
$ cub-workload promote --component checkout --commit --change-desc "Promote checkout readiness fixes"
```

## 9. Enforce with guardrails

`guardrails install` stands up a policy pack — `Warn=true` `vet-cel` Triggers
(memory limits, run-as-non-root, terminationMessagePolicy) plus an
annotate-then-validate Trigger for PDB coverage — and wires in-scope Spaces to a
shared Trigger Filter. It's dry-run by default and **conservative**: it skips
Spaces that already select Triggers their own way rather than clobbering them.

```console
$ cub-workload guardrails install -o table
Plan (dry-run) — policy pack "workload-policy", filter "workload-policy/workload-guardrails"
  triggers: workload-has-limits, workload-runs-nonroot, workload-termination-message-policy, workload-pdb-coverage
  spaces to wire (0):
  already wired (1): platform-dev
```

Apply with `--commit`. The three per-resource rules fire on their own; the
cross-Unit PDB-coverage rule is fed by `guardrails annotate`, which stamps the
finding onto each uncovered multi-replica workload:

```console
$ cub-workload guardrails install --commit
$ cub-workload guardrails annotate --commit --change-desc "Annotate uncovered workloads"
$ cub-workload guardrails status -o table       # Units now carrying ApplyWarnings
```

Triggers install advisory (`Warn=true`); promote one to blocking later with
`cub trigger update <slug> --space workload-policy --unwarn`.

## Reference

### Scoping any command

- `--where "<expr>"` — a single ConfigHub Unit filter that can reference `Slug`,
  `Labels.*`, `Space.*`, and `Target.*`. `where` is flat `AND`-only (no
  parentheses, no `OR`).
- Label shorthands (compile to `Space.Labels.<Key>`): `--component`,
  `--environment`, `--region`, `--owner`, `--layer`, `--variant`.
- `--cluster` / `--namespace` — client-side display filters over the fetched
  results.

### Discipline

- **Reads** default to JSON; add `-o table`.
- **Writes** are dry-run until `--commit`, which requires `--change-desc`.
- Nothing is applied to a cluster — writes create Unit revisions; roll out with
  `cub unit apply` (or your ArgoCD/Flux pipeline) separately.
- Guardrail ApplyGates are never bypassed — fix the data (or the rule).

### Agent skills

The `skills/` directory ships six ConfigHub agent skills so an agent can drive all
of the above: `workload-audit`, `workload-availability`, `workload-findings`
(read) and `workload-harden`, `workload-fleet`, `workload-guardrails` (write).
