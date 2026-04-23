# Mini-Kubaras

Mini-Kubaras are smaller, repeatable variants of the Artem/Kubara journey. They
exist so we can build skill and confidence across the same kinds of platform
truths without re-running the full Kubara repo every time.

Use these before going back to a full customer-style run. This public
incubator copy is the runnable source for Argo and fresh AI sessions, so the
workload fixtures do not depend on private-repo credentials.

## Fresh Claude Runtime Note

If the first paste returns `Please run /login`, run `/login` in that Claude
session and paste the same case prompt again. That error is Claude runtime
authentication, not ConfigHub authentication and not a mini-Kubara failure.

The above-the-line welcome/restore is not case-specific prompt text. It is a
product-level Claude surface: SessionStart renders the generic
the `CONFIGHUB READY` status-line cockpit before the first prompt and
injects startup facts. UserPromptSubmit `surface_restore.sh` renders the same
marker after `/login`, resume, or any first prompt whose transcript has not yet
shown it. If a mini-Kubara prompt needs a bespoke welcome block to feel usable,
treat that as a startup-surface bug.

## Why Mini-Kubaras

The full Kubara run taught the right lessons, but it also stacked too many
variables at once:

- historical ConfigHub units;
- ApplicationSet adoption;
- cert-manager bootstrap timing;
- external-secrets large CRDs;
- Kyverno admission instability;
- dev/prod value variance;
- GUI/trace/trust-surface density.

The mini cases split those into four focused drills. Each case should be small
enough to run on one laptop kind cluster and rich enough to exercise the same
operator muscles.

## The Four Cases

| Case | Prompt | Training Goal | What It Should Prove | Graduation Target |
|---|---|---|---|---|
| [`case-01-clean-handoff/`](./case-01-clean-handoff/GUIDE.md) | [`PROMPT.md`](./case-01-clean-handoff/PROMPT.md) | Clean adoption and closeout | ConfigHub applies a bootstrap ApplicationSet, Argo adopts/generates a simple app, one-shot sync converges, closeout is boring | A simple public Argo ApplicationSet example |
| [`case-02-large-crds/`](./case-02-large-crds/GUIDE.md) | [`PROMPT.md`](./case-02-large-crds/PROMPT.md) | CRD-heavy delivery mechanics | The run detects large-CRD risk before apply, chooses the correct apply mode, then proves CRDs/controllers/dependent app | cert-manager/external-secrets-style platform repos |
| [`case-03-policy-admission/`](./case-03-policy-admission/GUIDE.md) | [`PROMPT.md`](./case-03-policy-admission/PROMPT.md) | Admission WATCH -> BLOCK discipline | Kyverno/OPA/admission risk is visible, fail-closed rejection stops mutation, recovery is explicit | Policy-heavy clusters and regulated customer demos |
| [`case-04-dev-prod-variants/`](./case-04-dev-prod-variants/GUIDE.md) | [`PROMPT.md`](./case-04-dev-prod-variants/PROMPT.md) | Dev/prod variants and promotion | ConfigHub represents dev/prod value differences explicitly; promotion does not leak dev-only config into prod | ConfigHub Promotions plus customer platform variants |

Case 01 also includes a setup helper:

```bash
incubator/mini-kubara/case-01-clean-handoff/setup.sh --explain
incubator/mini-kubara/case-01-clean-handoff/setup.sh
```

It prepares the dedicated kind cluster, Argo CD, ConfigHub worker, and target,
then stops before the first ApplicationSet unit write.

## Shared Shape

Each case should have:

- one plain-English goal;
- one intentionally small topology;
- one colored route card using `./scripts/confighub-banner ... --force-color`
  when available;
- one `cub-scout` opener with the exact kubeconfig/context;
- one ConfigHub GUI trust moment before the first write approval, using
  `./scripts/confighub-gui-urls --space <space>` when available;
- one colored proof-row block using `./scripts/confighub-proof-rows
  --force-color --color-by both` when available;
- one mutation gate;
- one closeout gate;
- one "what this does not prove" section.

## Build Order

1. Build `case-01-clean-handoff` first. It should be green and boring.
2. Build `case-03-policy-admission` next. It trains stop discipline.
3. Build `case-02-large-crds` third. It productizes the external-secrets
   lesson without full Kubara.
4. Build `case-04-dev-prod-variants` last. It connects the operational lessons
   to ConfigHub's dev/prod and promotion story.

## Definition Of Done For A Case

A mini case is done only when it has:

- a `GUIDE.md`;
- a copy/paste prompt for Claude/Codex;
- fixture manifests or a precise fixture source;
- expected command/proof surfaces;
- stop conditions;
- a final report template;
- a status in the public incubator catalog when it graduates beyond design-packet state.

Until then, the case is a design packet, not a runnable example.

## Rule Of Thumb

Do not make a mini-Kubara green by hiding the interesting problem. If the case
is about admission, let admission block. If the case is about large CRDs, make
the apply-mode decision visible. If the case is about variants, keep dev and
prod visibly different.
