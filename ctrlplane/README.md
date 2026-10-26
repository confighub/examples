# ctrlplane-on-confighub

Map a [Ctrlplane](https://ctrlplane.dev) **System** bundle onto a ConfigHub
governed-app plan — read-only.

## Stack And Scenario

Ctrlplane is a release-**orchestration** layer: it decides *when* / *where* a
release should go and *what gates* it must pass, then dispatches a Job to a Job
Agent (ArgoCD, Kubernetes, GitHub Actions, Terraform Cloud). It explicitly does
not store config or apply manifests — *"that's ArgoCD/Flux/kubectl."*

A Ctrlplane user already has a declarative **System** bundle
(`System` / `Deployment` / `Environment` / `Resource` / `JobAgent` / `Policy`).
This example reads that bundle and proposes the equivalent ConfigHub app:

```
Ctrlplane                 ConfigHub
---------                 ---------
System            ->      app (naming root)
Deployment        ->      base Unit (upstream, in <app>-base)
Environment       ->      Space + downstream Unit variant
Resource          ->      Target
JobAgent          ->      delivery strategy (argocd -> confighub-oci-argo)
Policy            ->      approval gate / verification note
(promotion)       ->      cub unit update <variant> --upgrade
```

The Deployment is modelled as a **base/upstream Unit**; each Environment gets a
**downstream variant** linked via `--upstream-unit`. Promoting a change is then a
governed `--upgrade` (pull from upstream) — the ConfigHub-native parallel to
Ctrlplane promoting a Version across environments. This preserves the promotion
relationship that independent per-environment units would throw away.

## What This Proves

You can turn a Ctrlplane "app" definition into a ConfigHub governed app —
spaces, units, targets, a delivery strategy, and approval gates — **without
touching ConfigHub or a cluster**. It also surfaces the real seams where the two
models do not line up (see "Mapping seams").

This is the read-only first slice of a larger thesis: **Ctrlplane orchestrates
*when/where*; ConfigHub governs *what* and *proves* each step.**

## Prerequisites

- `python3` with `PyYAML` (`pip install pyyaml`)
- `jq` for the JSON path
- `cub` in PATH only if you later run the generated commands by hand

## What This Reads And Writes

- Reads: `systems/*.yaml` (a sample Ctrlplane System bundle, hand-authored)
- Writes: nothing. The default path is read-only.

## Read-Only Preview

```bash
./setup.sh --explain
./setup.sh --explain-json | jq
./setup.sh --cub-commands
```

Point it at your own Ctrlplane bundle:

```bash
./setup.sh --explain --source /path/to/your/ctrlplane/system.yaml
```

## Run It

There is nothing to "run" beyond the preview — the mapper *is* the example.

```bash
./verify.sh   # expect: All checks passed.
```

## Expected Output

`--explain` prints the proposed spaces (3: one base + staging + production),
units (3: one base + two upstream-linked variants), targets (3), the
`confighub-oci-argo` delivery strategy, the approval + verification gates, and a
"Mapping seams" section. `--explain-json` emits the same as a View Packet with
`"mutates": false`.

## Mapping seams (read these)

The mapper prints these because they are structurally true, not bundle-specific:

1. **Base unit needs config data; variants inherit it.** A Ctrlplane Deployment
   carries an image *reference*, not a rendered manifest. Supply the manifest
   **once** on the base Unit (via `import-from-helm` / `import-from-kustomize` or
   a YAML file); the environment variants inherit it through the upstream link
   and carry only their environment-local overrides.
2. **Promotion is `--upgrade`, and you must diff it.** Promoting a change is
   `cub unit update --space <env> <unit> --upgrade` (pull from the upstream
   base). Caveat: `--upgrade` can silently under-propagate list/nested fields
   when leaf vs base list shapes differ — diff the variant after upgrading; do
   not assume it landed.
3. **Verification has no ConfigHub-core equivalent.** Ctrlplane Policy
   verification (Datadog/Prometheus/HTTP) maps to a post-apply external check
   that feeds the promotion gate, not a ConfigHub primitive.
4. **Timing stays in Ctrlplane.** Environment progression and gradual-rollout
   *timing* is orchestration ConfigHub does not schedule. Keep Ctrlplane (or a
   promotion driver) for sequencing; ConfigHub governs and proves each step.

## Inspect It In The GUI

This read-only slice creates no ConfigHub objects, so there is no object URL
yet. Once you run the generated `cub` commands against a real space, each space
and unit gets a ConfigHub trust URL — that is the next proof step.

## Status

The mapping (`--explain`, `--explain-json`, `--cub-commands`) is implemented and
verified. The live create/apply path (`--apply`) is **not yet proven
end-to-end** against a ConfigHub space; the POC deliberately refuses to
auto-create and asks you to review and run the generated commands by hand.

## Verify It

```bash
./verify.sh
```

## Troubleshooting

- `PyYAML is required` → `pip install pyyaml`
- Empty/odd plan → confirm your `--source` bundle uses Ctrlplane `type:` docs
  (`System`, `Deployment`, `Environment`, `Resource`, `JobAgent`, `Policy`)
- Resources show as "needs manual Target binding" → the mapper only evaluates
  `resource.metadata["k"] == "v"` selectors; richer CEL needs manual binding

## Cleanup

```bash
./cleanup.sh
```

Nothing to clean unless you ran the generated `cub` commands by hand.
