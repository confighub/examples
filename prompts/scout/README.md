# scout

The reference implementation of the questions in
[`config-repo-ten-questions.md`](../config-repo-ten-questions.md). One Python
file. Read-only: no network, no credentials, no telemetry, no writes.

```bash
pip install pyyaml
python3 scout.py /path/to/your/config-repo
python3 scout.py /path/to/your/config-repo --json
python3 scout.py /path/to/your/config-repo --resolve   # list every unresolved layer
```

The prompt asks an AI agent the questions and explains the answers in your
repo's own terms. scout computes them the same way every time. Where the two
disagree, trust scout.

## What it reports

| Section | Question |
|---|---|
| Q1 | Inputs were approved, but outputs are deployed |
| Q2 + sprawl ledger | Your primary key is a filename; values in use vs values declared |
| Q3 | Promotion by find-and-replace |
| Q4 | Is my change in progress, or abandoned? |
| Q5 | One value was edited. How many values changed? |
| Q6 | Safer, or just rarer? |
| Q7 | Allowed to differ, or just differing? |
| Q7b | Where observed data lands, and what stops hand edits |
| Q7c | Dependency pinning |

It works best on Argo CD ApplicationSets with Helm values. Flux and Kustomize
are covered where the questions apply.

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
Kustomize bases, quoted version ranges) and every planted answer in the
example fleet.
