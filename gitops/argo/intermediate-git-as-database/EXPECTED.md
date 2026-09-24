# Expected answers

The answers for `repo/`: 24 clusters, two ApplicationSets, one chart. Every answer below was planted, so a scanner or an AI prompt run against it can be checked against the correct result. `./verify.sh` checks them with scout.

| Question | Planted | Correct answer |
|---|---|---|
| 1. Inputs were approved, but outputs are deployed | A Helm source with layered values; no rendered manifests | 1 templated source, 0 rendered |
| 2. Your primary key is a filename | Cluster files named `<org>-<env>-<region>`, the fields repeated inside, and a per-cluster override file with the same key | 3-field key, 24 rows; renaming one cluster touches 2 files |
| 2. (adding a value does nothing) | `ignoreMissingValueFiles: true`; a reference to `org/acme.yaml` after it was restructured to `org/acme/prod.yaml`; org declared for 1 of 3; provider for 1 of 2 | 1 literal ref resolves to nothing; org has no entry for beta and gamma (16 clusters); provider has no entry for azure (12) |
| 3. Promotion by find-and-replace | `version: 1.4.0` written once per prod wave | same version 4 times in one definition |
| 4. In progress, or abandoned? | 1.4.0 on prod, 1.5.0 on dev, 1.3.2 pinned on gamma prod, with nothing recording why | 1 definition, 3 versions live; the repo can't say which is a rollout and which is stuck |
| 5. One value edited, how many changed? | global values vs one cluster's values file | 24 vs 1: same one-line diff, 24× apart in reach |
| 6. Safer, or just rarer? | CI caps PRs at 5 cluster files, applied to `clusters/**`; bot capped at 3 concurrent PRs | 2 controls, both quotas. The cap guards the smallest-reach files; `values/global.yaml` is ungated |
| 7. Allowed to differ, or just differing? | `mon` has self-heal without prune, not declared; `canary` has both | 1 of 2 definitions: deletions linger and nobody decided |
| extra: observed data | `generated/` with a do-not-edit header and a CI check that fails on edits | present, guarded |
| extra: dependency pinning | Chart dependency `< 1.0.0`; `Chart.lock` excluded by .gitignore; chart source `>=1.0.0 <2.0.0` | unpinned, no lock, range at source |

## Not planted
- Real drift: there is no running system to compare against. The correct answer for actual divergence is NOT OBSERVED.
- The link between the CI cap and the reach of the files it guards: the answer to Q6 holds, but scout reports only that the quotas exist. Tying the gate to reach needs a person or the prompt.
