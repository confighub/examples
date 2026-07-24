# k8s-to-score

Convert the Kubernetes resources held in a ConfigHub Space into
[Score](https://score.dev) workload specifications.

ConfigHub stores configuration as data — fully materialized Kubernetes YAML in
versioned Units. That makes the Space a good source for a Score file: the
Deployment, its Service, its ConfigMap and its Ingress are all right there as
literal values, so the converter can resolve a `configMapKeyRef` to the actual
string rather than leaving a dangling reference.

The output is exactly what [`score-k8s`](https://github.com/score-spec/score-k8s)
consumes: one `score.dev/v1b1` Workload file per Deployment or StatefulSet.

```bash
k8s-to-score --space my-app-prod --out-dir score/
score-k8s init --no-sample && score-k8s generate score/*.yaml
```

This tool only ever **reads** from ConfigHub. It does not create, update, apply,
or delete anything.

## Build

```bash
make build     # -> bin/k8s-to-score
make test
```

## Try it without ConfigHub

`testdata/sample` holds a small two-workload app that exercises every mapping
path. Converting it needs no session:

```bash
make sample
# or:
bin/k8s-to-score --from-dir testdata/sample --stdout
```

## Convert a Space

Authenticate first, then point the tool at a Space:

```bash
cub auth login
bin/k8s-to-score --space my-app-prod --out-dir score/
```

Narrow the Units with a ConfigHub filter when a Space holds more than one app:

```bash
bin/k8s-to-score --space my-app-prod --where "Labels.Component = 'checkout'" --out-dir score/
```

### Flags

| Flag | Purpose |
|------|---------|
| `--space` | Space slug to read Units from (required unless `--from-dir`) |
| `--where` | ConfigHub filter over Units, e.g. `"Labels.Tier = 'web'"` |
| `--out-dir`, `-o` | Directory for the per-workload files (default `score`) |
| `--stdout` | Write a multi-document stream instead of files |
| `--from-dir` | Convert local YAML files instead of a Space |
| `--report-json` | Print the warning and skip report as JSON |
| `--explain` / `--explain-json` | Describe the conversion without reading anything |

## What maps to what

| Kubernetes | Score |
|---|---|
| Deployment | a workload |
| StatefulSet | a workload, plus the `k8s.score.dev/kind` annotation so `score-k8s` renders it back as a StatefulSet |
| container image / command / args | `containers[*].image`, `.command`, `.args` |
| `env` with a literal value | `containers[*].variables` |
| `env` from a `configMapKeyRef` | `containers[*].variables`, resolved to the ConfigMap Unit's literal value |
| `envFrom` a ConfigMap | one variable per key |
| container `resources` | `containers[*].resources.limits` / `.requests` (cpu and memory) |
| `livenessProbe` / `readinessProbe` (httpGet, exec) | the same, with named ports resolved to integers |
| Service whose selector matches the pods | `service.ports` |
| Ingress rule routing to that Service | a `route` resource (plus a `dns` resource when the rule has no host) |
| PVC, `volumeClaimTemplates`, `emptyDir` mount | a `volume` resource + `containers[*].volumes` |
| ConfigMap volume mount | `containers[*].files` with literal content |

### What does not map

Score is a smaller language than Kubernetes, deliberately. Everything below is
**reported**, never silently dropped — as a `warning:` or `skipped:` line on
stderr, or as JSON under `--report-json`:

- **`replicas`** — not part of the Score spec; scale belongs to the platform's
  provisioning layer.
- **Init containers** — Score expresses start-up ordering with a container
  `before` dependency instead.
- **Secrets** — a Secret's value is not config data. Secret-sourced environment
  variables and file mounts become `confighubplaceholder`, so the shape of the
  spec is complete and the gaps are visible. Supply real values, or model the
  Secret as a Score resource with a provisioner.
- **`tcpSocket` and `grpc` probes** — Score supports only `httpGet` and `exec`.
- **Pod-runtime `fieldRef`s** — `metadata.name` maps to `${metadata.name}`; the
  rest (pod IP, node name) are runtime state with no spec equivalent.
- **Everything that is not a workload** — CRDs, Namespaces, ServiceAccounts,
  RBAC, cert-manager Certificates, and so on. These are cluster scaffolding; in
  Score they are the platform's job, not the workload's.

`envFrom` keys that are not valid environment variable names (a ConfigMap
holding a `settings.yaml` file, say) are skipped, because the kubelet skips them
too — the converted spec matches what the container actually sees.

## How it decides what belongs together

A Space is a flat set of Units, so the converter reconstructs the relationships
the way Kubernetes does:

- A **Service** belongs to a workload when its `spec.selector` is a subset of
  the workload's pod template labels, in the same namespace. A Service with an
  empty selector never matches — it fronts something other than these pods.
- An **Ingress** belongs to a workload when its backend Service resolves to that
  workload. Its port must be one the workload's `service.ports` declares, which
  is what `score-k8s`'s route provisioner requires; a route that would not
  generate is reported instead of emitted.
- A **ConfigMap** or **Secret** is resolved by name and namespace against the
  other Units in the Space. One that lives outside the Space produces a
  placeholder and a warning rather than a silent omission.

Units normally hold one resource each, per ConfigHub's one-resource-per-Unit
doctrine, but multi-document Units convert too — which is what you get from a
Helm or Kustomize import.

## Round-tripping

The conversion is checked against the reference implementation, not just against
itself:

```bash
bin/k8s-to-score --from-dir testdata/sample --out-dir /tmp/score
cd /tmp/score && score-k8s init --no-sample && score-k8s generate *.yaml
```

That regenerates a Deployment, a StatefulSet, an HTTPRoute and the Services,
with images, variables, probes and resource requests intact.

## See also

- [Score specification](https://github.com/score-spec/spec)
- [`score-k8s`](https://github.com/score-spec/score-k8s) — the reference Kubernetes implementation
- [ConfigHub documentation](https://docs.confighub.com)
- [`contracts.md`](./contracts.md) — stable command outputs for automation
