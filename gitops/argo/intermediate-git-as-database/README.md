# Argo intermediate: is your config repo doing a database's job?

This example is a small, valid Argo CD fleet repo with one planted instance of
each of seven habits that appear when a Git repository becomes the operational
store for a fleet. It is part of the `gitops/` canonical example set. See
[`../../README.md`](../../README.md) for the full index.

Every problem here is deliberate and every answer is known in advance, so the
repo doubles as a test: run [`scout`](../../../prompts/scout/README.md) or the
[seven-question prompt](../../../prompts/config-repo-seven-questions.md) against
it and compare the output with [`EXPECTED.md`](./EXPECTED.md).

## Who this is for

You run Argo CD with Helm across more than a handful of clusters. Your
repository registers clusters, layers values files, and has an ApplicationSet
with several generators. It works. You suspect it is doing a job it was not
designed for.

## The first question this person asks

"Is my config repo a database?"

## Repo layout

```
repo/
  clusters/            24 Argo CD cluster Secrets, named <org>-<env>-<region>.yaml
  values/
    global.yaml        shared by every mon Application (5 values)
    environments/      dev, prod
    providers/         aws only (the fleet also runs azure)
    org/acme/prod.yaml one org of three, restructured from org/<org>.yaml
    clusters/          one override file per cluster
  appsets/
    mon.yaml           the fleet definition: six generators, three versions,
                       chart from a Helm repo, values from this repo (ref: values)
    node-tuning.yaml   one cluster, chart kept in this repo
  charts/node-tuning/  in-repo chart with a ranged dependency
  generated/state.yaml observed data written back by a sync job
  .github/workflows/ci.yaml
  renovate.json
```

The ApplicationSets use Go templates and validate against Argo CD's published
CRD schema. The cluster Secrets carry labels but no connection credentials.
Nothing here renders: the charts' templates are not in the repo. The example is
about what the repository says, and what it cannot say.

## The seven, one at a time

### 1. Am I using filenames as primary keys?

Open `clusters/`. Each filename is a three-part key, `acme-prod-use1`, and the
same three fields are repeated inside the file as labels, because nothing reads
a filename as data. The key appears again in `values/clusters/`, in
`appsets/node-tuning.yaml` and in `generated/state.yaml`. Renaming that cluster
means editing four files, and nothing checks you found them all.

The sharper half: adding a value succeeds and does nothing. `mon.yaml` asks for
`values/org/<org>.yaml`, but the org layer was restructured to
`values/org/<org>/<env>.yaml` and the old line stayed. With
`ignoreMissingValueFiles: true`, every one of the 24 lookups misses without a
signal. The fleet runs two providers but only `aws` has a file, so 12 more
lookups miss. 36 of 121 value-file lookups resolve to nothing.

### 2. Am I approving the query, but deploying the result?

Open `appsets/mon.yaml`. A reviewer sees a chart name, a version per wave and
five values files. What each cluster receives is the render of all of them, and
no render is stored or shown anywhere. The approval covers the inputs; the risk
is in the output.

### 3. Am I using find-and-replace as my transaction?

In `mon.yaml`, `version: 1.4.0` appears once per production wave: four times.
Promoting the next version is four text edits. A partial replace is valid YAML,
passes lint, and leaves the fleet on two versions with no record that anyone
chose that.

### 4. Am I using someone's memory as my status column?

`mon.yaml` holds three versions at once: 1.4.0 on acme and beta production,
1.5.0 on dev, and 1.3.2 pinned on gamma production. Nothing records a start
date, a target, a completion condition or an owner. There may be a good reason
for the gamma pin. The repository does not say, so the only record is whoever
remembers.

### 5. Am I using grep as my query engine?

A one-line change to `values/global.yaml` re-renders all 24 `mon`
Applications. A one-line change to `values/clusters/acme-prod-use1.yaml`
re-renders two, the `mon` and `node-tuning` Applications on one cluster. The
two diffs look identical in review, and the only way to tell them apart is to
search the repo and resolve the selectors and templates by hand.

Blast radius is fan-out times density: Applications reached, times values
changed on each. The global file holds 5 values and reaches 24 Applications, so
rewriting it changes 120 live values. The cluster file is denser, 9 values, but
reaches 2, so rewriting it changes 18. Density is in the file where a reviewer
can see it. Fan-out is not.

### 6. Am I using rate limits as my constraints?

`.github/workflows/ci.yaml` caps a pull request at five files under
`clusters/`, and `renovate.json` caps the bot at three open pull requests. The
bot cap limits how often. The file cap limits what one change does, but only
for cluster files, the files with the smallest reach. CI runs only when
`clusters/**` or `generated/**` change, so an edit to `values/global.yaml`,
which reaches every cluster, passes no check at all.

### 7. Am I using defaults as my drift policy?

`mon.yaml` self-heals but does not prune, and says nothing about why.
`node-tuning.yaml` does both. For `mon`, a resource removed from desired state
stays on the cluster. Prune was never decided; it was left at the default.

## Two more

- **Observed data in the store.** `generated/state.yaml` is written by a sync
  job and marked do-not-edit. CI blocks edits in pull requests. The repository
  now holds two kinds of truth: intent and observation.
- **Unpinned dependencies.** `charts/node-tuning/Chart.yaml` depends on
  `prometheus-node-exporter < 5.0.0`, and `Chart.lock` is excluded by
  `.gitignore`. The dependency resolves at render time, so a rollout that spans
  a new upstream release renders different clusters against different versions.

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

Copy `repo/` somewhere on its own, so the agent cannot read `EXPECTED.md`:

```bash
cp -r repo /tmp/fleet && cd /tmp/fleet
```

Open that directory in Claude Code, Cursor, Codex or any agentic tool with shell
access, paste the
[seven-question prompt](../../../prompts/config-repo-seven-questions.md), and
compare its answers with [`EXPECTED.md`](./EXPECTED.md). A tool that reports a
number it did not compute, or guesses a selector it could not resolve, will show
up here before it shows up on your own repository.

## What this example does not do yet

It does not upload into ConfigHub. The next step for this example is the same
fleet held in ConfigHub, with each of the seven answered there: reach on the
change, promotion as one recorded operation, rollout state readable, drift
tolerance declared. Until then, this is the "before".

## Mutation boundaries

- `setup.sh --explain`, `setup.sh --explain-json`: read-only
- `setup.sh`: read-only; runs scout over `repo/`
- `verify.sh`: read-only
- `cleanup.sh`: no-op; nothing is created
