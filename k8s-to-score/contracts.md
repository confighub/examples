# k8s-to-score contracts

Stable command outputs automation and AI assistants can rely on. See
[`EXAMPLE_CONTRACT_STANDARD.md`](../EXAMPLE_CONTRACT_STANDARD.md).

Every command in this file is **read-only**. `k8s-to-score` has no mutating
code path: it never creates, updates, applies, or deletes a ConfigHub entity,
and never touches live infrastructure. The only thing it writes is local files
under `--out-dir`.

---

### `k8s-to-score --explain`

- mutates: no
- reads ConfigHub: no
- output shape: plain text
- stable text anchors: `k8s-to-score`, `Mapping:`, `Not represented`, `Mutations: none.`
- proves: the mapping the tool implements, before any conversion

### `k8s-to-score --explain-json`

- mutates: no
- reads ConfigHub: no
- output shape: JSON object
- stable fields: `example_name`, `mutates`, `mutates_confighub`, `mutates_live_infra`,
  `score_api_version`, `mapping`, `not_represented`, `evaluation_modes`
- invariants: `mutates`, `mutates_confighub` and `mutates_live_infra` are always `false`
- proves: the conversion plan, machine-readably

### `k8s-to-score --from-dir testdata/sample --stdout`

- mutates: no
- reads ConfigHub: no
- output shape: multi-document YAML on stdout, one `score.dev/v1b1` Workload per document
- stable text anchors: `apiVersion: score.dev/v1b1`, `metadata:`, `containers:`
- stable content: two workloads named `checkout` and `ledger`, in that order
- proves: the output shape without needing a session

### `k8s-to-score --space <space> --out-dir <dir>`

- mutates ConfigHub: **no** (Unit reads only)
- mutates live infra: no
- writes: `<dir>/<workload>.yaml`, one file per Deployment or StatefulSet
- output shape: one `wrote <path> (<Kind> <name> from unit <slug>)` line per file on stderr,
  followed by `skipped:` and `warning:` lines
- stable text anchors: `wrote `, `skipped: `, `warning: `
- exit code: `0` on success; `1` when the Space has no Units with config data,
  or no Deployment or StatefulSet to convert
- proves: the Space's workloads as Score specs, plus everything Score cannot express

### `k8s-to-score --space <space> --report-json`

- mutates: no
- output shape: JSON object on stdout
- stable fields: `workloads` (array of names), `warnings` (array of
  `{workload, unit, message}`), `skipped` (array of `{unit, kind, name}`)
- invariant: every resource in the source Space appears exactly once — as a
  converted workload, folded into one, or listed in `skipped`
- proves: the full conversion report, machine-readably

---

## Round-trip contract

The output is verified against the reference implementation, not only against
itself:

```bash
k8s-to-score --from-dir testdata/sample --out-dir /tmp/score
cd /tmp/score && score-k8s init --no-sample && score-k8s generate *.yaml
```

- `score-k8s generate` exits `0`
- `manifests.yaml` contains a `Deployment` (`checkout`), a `StatefulSet`
  (`ledger`), an `HTTPRoute`, and the backing `Service` objects
- proves: the emitted Score files are valid input to `score-k8s`, not merely
  well-formed YAML
