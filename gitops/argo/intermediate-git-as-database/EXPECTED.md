# Expected answers

The answers for `repo/`: a valid Argo CD fleet of 24 clusters (registered as
cluster Secrets), two ApplicationSets and one in-repo chart. Every answer below was
planted, so scout or an AI agent running
[`config-repo-seven-questions.md`](../../../prompts/config-repo-seven-questions.md)
can be checked against it. `./verify.sh` checks them with scout.

Order and wording follow the seven questions.

| # | Question | Planted | Correct answer |
|---|---|---|---|
| 1 | Am I using filenames as primary keys? | Cluster Secrets named `<org>-<env>-<region>`, with the same fields repeated as labels and in `stringData`, and a per-cluster values file with the same key | 3-part key, 24 rows. Renaming a cluster touches 2 files; `acme-prod-use1` touches 4 (its Secret, its values file, `appsets/node-tuning.yaml`, `generated/state.yaml`) |
| 1 | (silent lookups) | `ignoreMissingValueFiles: true`. `mon` still asks for `org/<org>.yaml` after the layer moved to `org/<org>/<env>.yaml`. Only `aws` has a provider file | 121 value-file lookups across targets; 36 resolve to nothing: 24 for the org layer (every cluster), 12 for `providers/azure.yaml`. `values/org/acme/prod.yaml` is reached by nothing. Declared vs in use: org 1 of 3, provider 1 of 2, env 2 of 2 |
| 2 | Am I approving the query, but deploying the result? | Two templated sources, no rendered output, no hydration, no render in CI | 2 templated sources, 0 rendered manifests. Nothing shows a reviewer the rendered output before merge |
| 3 | Am I using find-and-replace as my transaction? | `version: 1.4.0` written once per production wave in `mon` | The same version 4 times in one definition. No promotion record anywhere |
| 4 | Am I using someone's memory as my status column? | `mon` runs 1.4.0 on acme and beta production, 1.5.0 on dev, 1.3.2 pinned on gamma production | 1 definition, 3 versions live (8, 12 and 4 clusters). Nothing records a start, target, completion condition or owner |
| 5 | Am I using grep as my query engine? | `values/global.yaml` (5 values) is loaded by every `mon` Application; `values/clusters/acme-prod-use1.yaml` (9 values) by the two Applications on one cluster | Fan-out 24 vs 2. Whole-file blast radius 120 vs 18: the denser file has the smaller blast radius. `mon`'s `manifest-generate-paths` names only `global.yaml`. Reach is not shown at review time |
| 6 | Am I using rate limits as my constraints? | CI caps pull requests at 5 files under `clusters/`; CI blocks edits to `generated/`; Renovate is capped at 3 open pull requests | The bot cap limits how often. The file cap limits what one change does, but only for cluster files, the ones with the smallest reach. CI runs only for `clusters/**` and `generated/**`, so a change to `values/global.yaml` (24 targets) passes no check |
| 7 | Am I using defaults as my drift policy? | `mon` self-heals but does not prune, with nothing declared; `node-tuning` self-heals and prunes | 1 of 2 definitions leaves prune at the default. No ignore rules or sync options declared anywhere |
| also | Observed data | `generated/state.yaml`, written by a sync job, marked do-not-edit, blocked by CI on pull requests | Present, guarded on pull requests. Whether direct pushes are blocked (branch protection) is NOT OBSERVED |
| also | Dependency pinning | `charts/node-tuning` depends on `prometheus-node-exporter` `< 5.0.0`; `Chart.lock` is in `.gitignore` | Unpinned, no lock committed. The dependency resolves at render time |

## Not planted: correct answer is NOT OBSERVED

- Actual drift. There is no running system to compare with.
- Whether any rollout is stuck rather than in progress. That needs history and live status.
- Whether the in-repo cluster Secrets match what Argo CD has registered.
- What the charts render. Chart templates are not in the repo.
