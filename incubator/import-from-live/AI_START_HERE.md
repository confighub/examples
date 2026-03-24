# AI Start Here

Use this page when you want to drive `import-from-live` safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. ask `Ready to continue?`
6. wait before proceeding

## Suggested Prompt

```text
Read incubator/import-from-live/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output. Give GUI links where possible.
Do not continue until I say continue.
```

## What This Example Is For

This example is for brownfield discovery from a running cluster.

It creates a small local `kind` cluster with mixed Argo, Helm, and native ownership signals, then runs `cub-scout import --dry-run --json` to propose a ConfigHub structure.

It does not mutate ConfigHub by default.

## Stage 1: Preview The Plan (read-only)

```bash
cd incubator/import-from-live
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- none yet; this stage is CLI-only preview

Pause after this stage.

## Stage 2: Build The Brownfield Cluster (mutates live infrastructure)

```bash
./setup.sh
```

What this mutates:

- local `kind` cluster
- local sample output

What you should see after:

- Argo Application objects in `argocd`
- Helm and native fixtures in the cluster
- `sample-output/suggestion.json`

GUI checkpoint:

- none by default; this example is about cluster-side evidence and dry-run output rather than a GUI walkthrough

Pause after this stage.

## Stage 3: Verify The Cluster And Dry-Run Proposal (read-only)

```bash
./verify.sh
kubectl get application -n argocd
kubectl get deployment -n myapp-dev
kubectl get statefulset -n myapp-prod
jq '.proposal.appSpace' sample-output/suggestion.json
jq '.proposal.units[] | {slug, app, variant}' sample-output/suggestion.json
```

What this proves:

- `kubectl` proves the live cluster fixtures exist
- `cub-scout import --dry-run --json` proves the proposed ConfigHub structure
- the committed expected output proves the suggestion contract stayed stable for this example

What this does not prove:

- no ConfigHub mutation has happened yet

GUI checkpoint:

- if the human later chooses to import for real, the next GUI stop would be the ConfigHub space and unit list for the imported proposal

Pause after this stage.

## Stage 4: Optional Real Import (mutates ConfigHub)

Do this only if the human explicitly asks for the next step.

```bash
cub-scout import --yes ...
```

Say clearly before running it:

- this is the first ConfigHub-mutating step
- the dry-run proposal above is what the real import should create

## Stage 5: Cleanup

```bash
./cleanup.sh
```

This deletes the local `kind` cluster and local sample output.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
