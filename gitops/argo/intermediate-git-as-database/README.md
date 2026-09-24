# Argo intermediate: seven ways a config repo becomes a database

This example is a small Argo CD fleet repo with one planted instance of each of
seven problems that appear when a Git repository becomes the operational store
for a fleet. It is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full index.

Every problem here is deliberate and every answer is known in advance, so the
repo doubles as a test: run [`scout`](../../../prompts/scout/README.md) or the
[ten-question prompt](../../../prompts/config-repo-ten-questions.md) against
it and compare the output with [`EXPECTED.md`](./EXPECTED.md).

## Who this is for

You run Argo CD with Helm across more than a handful of clusters. Your
repository has a `clusters/` directory, layered values files, and an
ApplicationSet with several generators. It works. You suspect it is doing a
job it was not designed for.

## The first question this person asks

"Is my config repo a database?"

## Repo layout

```
repo/
  clusters/            24 cluster files, named <org>-<env>-<region>.yaml
  values/
    global.yaml        shared by every cluster
    environments/      dev, prod
    providers/         aws only (the fleet also runs azure)
    org/acme/prod.yaml one org of three, restructured from org/acme.yaml
    clusters/          one override file per cluster
  appsets/
    mon.yaml           the fleet definition: six generators, three versions
    canary.yaml        one cluster, for comparison
  charts/mon/Chart.yaml
  generated/state.yaml observed data written back by a sync job
  .github/workflows/ci.yaml
  renovate.json
```

The chart has no templates. Nothing here renders; the example is about what
the repository says, and what it cannot say.

## The seven, one at a time

### 1. Inputs were approved, but outputs are deployed

Open `appsets/mon.yaml`. A reviewer sees a chart name, a version range and five
values files. What each cluster actually receives is the render of all of
them, and no render is stored anywhere in `repo/`. Approval attaches to the
inputs; the risk sits in the output.

### 2. Your primary key is a filename

Open `clusters/`. Each filename is a three-part key, `acme-prod-use1`, and the
same three fields are repeated inside the file because nothing can read a
filename as data. The key appears again in `values/clusters/`. Renaming a
cluster is a delete plus an insert in two places, and nothing checks the
references.

The sharper half: adding a value succeeds and does nothing. `mon.yaml`
references `values/org/acme.yaml`, which was restructured to
`values/org/acme/prod.yaml`. The line stayed. `ignoreMissingValueFiles: true`
means the missing file is skipped without a signal. The fleet runs three orgs
and two providers; `values/org/` declares one and `values/providers/` declares
one.

### 3. Promotion by find-and-replace

In `mon.yaml`, `version: 1.4.0` appears once per production wave: four times.
Promoting the next version is four text edits. A partial replace is valid
YAML, passes lint, and leaves the fleet on two versions with no record that
anyone chose it.

### 4. Is my change in progress, or abandoned?

`mon.yaml` holds three versions at once: 1.4.0 on production, 1.5.0 on dev,
and 1.3.2 pinned on gamma production. The repository has no start date, no
target, and no completion criterion for any of them. There may be a good
reason for the gamma pin. The repository does not say.

### 5. One value was edited. How many values changed?

A one-line change to `values/global.yaml` re-renders all 24 clusters. A
one-line change to `values/clusters/acme-prod-use1.yaml` re-renders one. The
two diffs look identical in review.

### 6. Safer, or just rarer?

`.github/workflows/ci.yaml` caps a pull request at five files under
`clusters/`, and `renovate.json` caps the bot at three concurrent pull
requests. Both limit how often things change. Neither limits what one change
does, and the cap guards the files with the smallest reach. `values/global.yaml`
is ungated.

### 7. Allowed to differ, or just differing?

`mon.yaml` self-heals but does not prune, and says nothing about why.
`canary.yaml` does both. For `mon`, a resource removed from desired state stays
on the cluster. Nobody recorded whether that was the intent.

## Two more

- **Observed data in the store.** `generated/state.yaml` is written by a sync
  job and marked do-not-edit. CI blocks hand edits. The repository now holds
  two kinds of truth: intent and observation.
- **Unpinned dependencies.** `charts/mon/Chart.yaml` depends on
  `prometheus < 1.0.0`, and `Chart.lock` is excluded by `.gitignore`. The
  dependency resolves at render time, so a rollout that spans a new upstream
  release renders different clusters against different versions.

## Read-only first

```bash
cd gitops/argo/intermediate-git-as-database
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Running it

```bash
pip install pyyaml
./setup.sh      # runs scout over repo/ and prints the seven
./verify.sh     # checks every answer against EXPECTED.md
```

Neither command mutates ConfigHub, a cluster, or any file.

## Try your AI tool on it

Open this directory in Claude Code, Cursor, Codex or any agentic tool with
shell access, paste the
[ten-question prompt](../../../prompts/config-repo-ten-questions.md), and
compare its answers with [`EXPECTED.md`](./EXPECTED.md). A tool that reports a
number it did not compute, or guesses a selector it could not resolve, will
show up here before it shows up on your own repository.

## What this example does not do yet

It does not upload into ConfigHub. The next step for this example is the
same fleet held in ConfigHub, with each of the seven answered there: reach on
the change, promotion as one recorded operation, rollout state readable,
drift tolerance declared. Until then, this is the "before".

## Mutation boundaries

- `setup.sh --explain`, `setup.sh --explain-json`: read-only
- `setup.sh`: read-only; runs scout over `repo/`
- `verify.sh`: read-only
- `cleanup.sh`: no-op; nothing is created
