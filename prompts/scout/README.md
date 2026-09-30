# scout

The seven questions in [`config-repo-seven-questions.md`](../config-repo-seven-questions.md)
as fixed checks, in one Python file. Read-only: no network, no credentials, no
telemetry, no writes.

```bash
pip install pyyaml
python3 scout.py /path/to/your/config-repo
python3 scout.py /path/to/your/config-repo --json
python3 scout.py /path/to/your/config-repo --resolve   # list every unresolved layer
```

The prompt asks an AI agent the questions and explains the answers in your repo's
own terms. scout computes them the same way every run. Running both is a useful
cross-check.

## What it reports

| Section | Question | What scout measures |
|---|---|---|
| 1 | Am I using filenames as primary keys? | key schemes in filenames, files that refer to one entity, value-file lookups per target that resolve to nothing, values in use vs values declared |
| 2 | Am I approving the query, but deploying the result? | templated sources, rendered manifests stored, Source Hydrator use |
| 3 | Am I using find-and-replace as my transaction? | version strings repeated within one definition |
| 4 | Am I using someone's memory as my status column? | definitions holding two or more versions |
| 5 | Am I using grep as my query engine? | fan-out per changed path, values per file, fan-out × values |
| 6 | Am I using rate limits as my constraints? | CI, bot and freeze controls, and whether each limits frequency or content |
| 7 | Am I using defaults as my drift policy? | self-heal, prune and declared tolerance per definition |
| also | Observed data and dependency pinning | written-back directories and their guards; chart ranges and lock files |

Targets come from Argo CD cluster Secrets in the repo when there are any, and
otherwise from a directory of per-target files. Reach is resolved for
ApplicationSet cluster selectors. For Flux and plain Applications, scout says
reach is NOT OBSERVED rather than reporting zero.

## What it will not do

- Guess. A selector it cannot interpret is reported as unresolved, never as
  "matches everything". Counts it cannot complete are marked as lower bounds.
- Score, rank or price anything.
- See the running system. Drift tolerance is in the repo; drift is not.

## Try it on a repo with known answers

[`gitops/argo/intermediate-git-as-database`](../../gitops/argo/intermediate-git-as-database/README.md)
is a small fleet with one planted instance of each question and an answer
sheet. Run scout there first to see what the output looks like.

## Tests

```bash
python3 test_scout.py
```

Covers the parsing edge cases (inline YAML, boolean selectors, `automated: {}`,
Kustomize bases, quoted version ranges, Argo template parameters, cluster
Secrets, references inside files, Flux-only repos) and every planted answer in
the example fleet.
