# Executable Rewrite Plan: Spring Platform Examples

This plan exists to make the Spring platform examples readable, honest, and
directly comparable for a human reviewer, while preserving the core `cub-gen`
generator story.

This is not a brainstorming note.
It is an execution contract.

If an AI assistant works this plan, it must satisfy the checks in this file and
must not paper over missing functionality with nicer prose.

---

## Goal

After reading the plain-English introduction, a human should be able to open
any one of the three Spring examples and immediately think:

1. this is the same underlying model
2. this example is showing me one specific view of that model
3. I can tell what is real, what is simulated, and what is not implemented
4. I can compare this example cleanly against the other two
5. I can see the `cub-gen` generator story without needing an AI to narrate it

The bar is:

> "Yes, it all makes perfect sense."

---

## Non-Negotiable Rules

### Rule 1: Keep the core `cub-gen` story

All three examples must keep the same underlying story:

- app inputs exist
- platform inputs exist
- a generator transforms them into operational config
- ConfigHub stores and governs the operational config
- field lineage explains ownership and mutation routes

Do not rewrite the examples into generic platform demos that hide the generator.

### Rule 2: One model, three views

These are not three unrelated demos.
They are one underlying system shown through three views:

- `springboot-platform-app`
  Plain ConfigHub / core `cub-gen` view
  app + platform -> generator -> operational config -> ConfigHub
- `springboot-platform-app-centric`
  `ADT` view
  App -> Deployments -> Targets
- `springboot-platform-platform-centric`
  experimental `ADTP` view
  Platform -> Apps -> Deployments -> Targets

This exact framing must appear in all three READMEs.

### Rule 3: No wrapper language in public docs

Public-facing docs must not describe the second and third examples as wrappers,
delegates, shells, or thin views over the first example.

Internal code reuse is allowed.
Reader-facing language must present each example as a complete, runnable
example in its own right.

Forbidden phrases in public READMEs and AI guides:

- `wrapper`
- `delegates`
- `all implementation lives in`
- `use example #1 when you need the full implementation`

### Rule 4: Human-first docs, AI-second docs

AI guidance is important, but a human reader must be able to understand the
example from the README and code layout alone.

The README is the canonical entry point.
`AI_START_HERE.md` is secondary and must not carry core model explanation that
is missing from the README.

AI-first support must be available but not in the way.

That means:

- the README explains the example without requiring prompt choreography
- `AI_START_HERE.md` accelerates exploration after the human already
  understands the model
- the first screen of the example must not feel like a demo script before it
  feels like an example
- AI prompts must not displace the core model, truth matrix, or command surface

### Rule 5: The repo tree itself must teach the model

The file layout must make the example understandable before a reader opens a
single shell script.

At a glance, a human should be able to infer:

- where the inputs live
- where the generator lives
- where the operational outputs live
- where ConfigHub-facing material lives
- where route-specific change proofs live

Avoid a cluttered top level with many peer files that all look equally
important.

The top-level directory should feel tidy, intentional, and legible.
If the tree looks messy, the model will feel messy.

### Rule 6: No bluffing about implementation

Every example must state near the top:

- what is real today
- what is simulated today
- what is not implemented yet

Do not claim capabilities beyond what the scripts and product actually prove.

Examples:

- do not imply `lift upstream` creates a real GitHub PR unless the example
  actually performs that action
- do not imply `block/escalate` is enforced unless the example actually proves
  server-side blocking or escalation
- do not describe Noop target mode as real Kubernetes delivery

### Rule 7: The three examples must be directly comparable

All three examples must use the same section order, the same status vocabulary,
and the same comparison table.

The reader should not have to decode three different narrative structures.

---

## Canonical Model To Preserve

Every example must show the same artifact chain:

```text
App inputs + Platform inputs
-> Generator
-> Operational config
-> ConfigHub storage and governance
-> Mutation routing
-> Delivery / projection
```

Every example must also show the same mutation routes:

- `apply here`
- `lift upstream`
- `block/escalate`

For future work, the docs may mention:

- `project downstream`

But do not make that part of the main story unless the example actually proves
it.

---

## Required Positioning Per Example

### 1. `springboot-platform-app`

This must be presented as:

- the core `cub-gen` generator example
- the plain ConfigHub view
- the best place to understand generator inputs, outputs, lineage, and
  operational config

It must answer:

- what is the generator
- what does it read
- what does it produce
- how does ConfigHub store that output
- how do field routes come from lineage

### 2. `springboot-platform-app-centric`

This must be presented as:

- the same underlying system viewed as `ADT`
- the best place to understand one app across multiple deployments and targets

It must answer:

- what is the app
- what are the deployments
- what are the targets
- how does the same generator story appear in `ADT` terms

It must not read like a shell around `springboot-platform-app`.

### 3. `springboot-platform-platform-centric`

This must be presented as:

- the same underlying system viewed as experimental `ADTP`
- the best place to understand one platform organizing multiple apps

It must answer:

- what is platform-owned
- what is app-owned
- how the generator story appears when platform is made explicit

It must label `ADTP` as experimental near the top.

---

## Mandatory README Structure

Each example README must use this exact high-level structure, in this order:

1. Title and one-sentence thesis
2. "This View" section
3. "The Same Underlying Model" section
4. "What Is Real Today" section
5. "What Is Simulated Today" section
6. "What Is Not Implemented Yet" section
7. "Quick Start" section
8. "Artifact Chain" section
9. "Mutation Routes" section
10. "Compare This View To The Other Two" section
11. "Key Files" section

This is mandatory.
Do not invent different structures for different examples.

---

## Mandatory Truth Matrix

Each README must contain the same status matrix with the same row set.

Required rows:

- generator transformation
- field lineage / explain-field
- ConfigHub mutation storage
- mutation history / audit trail
- refresh preview
- real Kubernetes delivery
- Noop target simulation
- running app HTTP verification
- `lift upstream` automated PR
- `block/escalate` server-side enforcement
- Flux/Argo delivery path

Allowed status values:

- `Real`
- `Simulated`
- `Not Implemented Yet`
- `Experimental`

Do not invent extra status vocabularies.

---

## Required Comparison Table

Each README must include the same comparison table:

| View | Example | Core question answered |
|------|---------|------------------------|
| Plain ConfigHub | `springboot-platform-app` | How does `cub-gen` transform app + platform into governed operational config? |
| ADT | `springboot-platform-app-centric` | How do I understand one app across deployments and targets? |
| Experimental ADTP | `springboot-platform-platform-centric` | How do I make platform explicit above apps and deployments? |

This table must be materially the same in all three examples.

---

## Required Top-Level Repo Shape

Each example should converge toward a top-level layout that makes the model
obvious.

Preferred visible categories:

- `README.md`
- `AI_START_HERE.md`
- `inputs/` or clearly named upstream input folders
- `generator/`
- `operational/`
- `confighub/`
- `routes/` or `changes/`
- `scripts/` or another clearly named home for lower-level shell helpers

Exact names may vary if existing names are already strong, but the structure
must visually communicate:

1. inputs
2. transformation
3. outputs
4. governed operational view
5. route-specific change paths

Flat top-level piles of many `.sh` files should be treated as a design smell.

---

## Public Command Surface Limit

Public docs must present no more than five top-level user commands per example.

Allowed public command categories:

1. explain / preview
2. generator inspection
3. setup
4. verify
5. cleanup

Other scripts may exist internally, but they must not be presented as equal
entry points in the main README.

If additional scripts remain, they must be grouped under:

- internal support scripts
- route-specific proof helpers
- optional advanced commands

The human reader must never see a long flat list of shell scripts and have to
guess which ones matter.

---

## Script Rationalization Rules

For `springboot-platform-app`, classify every current `.sh` file into one of
these buckets:

- public command
- internal support
- optional advanced proof
- remove / merge

Current script inventory to classify:

- `setup.sh`
- `verify.sh`
- `confighub-setup.sh`
- `confighub-cleanup.sh`
- `confighub-verify.sh`
- `confighub-compare.sh`
- `confighub-field-routes.sh`
- `confighub-refresh-preview.sh`
- `lift-upstream.sh`
- `lift-upstream-verify.sh`
- `block-escalate.sh`
- `block-escalate-verify.sh`
- `demo-full-loop.sh`
- `verify-e2e.sh`

The rewrite must make it obvious which of these are user-facing and which are
not.

---

## Concrete File Work

### Folder-level files to rewrite

- [README.md](/Users/alexis/Public/github-repos/examples/spring-platform/README.md)
- [AI_START_HERE.md](/Users/alexis/Public/github-repos/examples/spring-platform/AI_START_HERE.md)

### Example 1 files to rewrite

- [README.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app/README.md)
- [AI_START_HERE.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app/AI_START_HERE.md)
- [contracts.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app/contracts.md)
- [prompts.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app/prompts.md)

### Example 2 files to rewrite

- [README.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app-centric/README.md)
- [AI_START_HERE.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app-centric/AI_START_HERE.md)
- [contracts.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app-centric/contracts.md)
- [prompts.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app-centric/prompts.md)

### Example 3 files to rewrite

- [README.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-platform-centric/README.md)
- [AI_START_HERE.md](/Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-platform-centric/AI_START_HERE.md)

### Optional support-file changes

Only if needed after the README rewrite:

- move low-level scripts under an internal folder
- add one machine-readable status file shared by all three examples
- add one shared comparison partial or generated table

---

## Execution Sequence

### Phase 0: Freeze truth

Before rewriting prose, produce one definitive capability table for the shared
underlying implementation:

- real
- simulated
- not implemented yet
- experimental

This table becomes the source for all three examples.

### Phase 1: Rewrite the folder-level entry point

Rewrite the folder README so it says clearly:

- one underlying model
- three views
- plain ConfigHub, `ADT`, experimental `ADTP`
- where to start based on the reader's question

Rewrite the folder AI guide so it points back to human docs instead of carrying
the model itself.

### Phase 2: Rewrite example 1 as the canonical generator story

Make Example 1 the cleanest explanation of:

- generator inputs
- generator transformation
- operational outputs
- ConfigHub authority
- route logic from lineage

Push all missing-functionality truth up near the top.

### Phase 3: Rewrite example 2 as true `ADT`

Make Example 2 stand on its own as the clearest `ADT` explanation.

Do not use public wrapper language.
Do not require the reader to read Example 1 to understand it.

### Phase 4: Rewrite example 3 as experimental `ADTP`

Make Example 3 stand on its own as the clearest platform-explicit view.

Label it experimental at the top and in the status matrix.

### Phase 5: Rationalize script visibility

Reduce the public command surface.
Keep supporting scripts if needed, but demote them in docs.

### Phase 6: Verify and compare

Confirm that all three READMEs:

- use the same section structure
- use the same truth matrix rows
- use the same comparison table
- present the same underlying model
- differ only in viewpoint

---

## Hard Verification Checks

These checks must pass before claiming the rewrite is done.

### Check 1: No public wrapper language

Run:

```bash
rg -n "wrapper|delegates|all implementation lives in|use .* when you need the full implementation" \
  /Users/alexis/Public/github-repos/examples/spring-platform \
  --glob 'README.md' --glob 'AI_START_HERE.md'
```

Expected result:

- no matches in the three example READMEs
- no matches in the three example AI guides

### Check 2: `ADTP` is always marked experimental

Run:

```bash
rg -n "ADTP|Platform -> Apps -> Deployments -> Targets" \
  /Users/alexis/Public/github-repos/examples/spring-platform \
  --glob 'README.md' --glob 'AI_START_HERE.md'
```

Expected result:

- every `ADTP` mention is explicitly labeled `experimental`

### Check 3: Missing features are top-level truth, not buried footnotes

Manual check:

- every README must expose `What Is Not Implemented Yet` near the top
- each README must say whether `lift upstream` PR automation exists
- each README must say whether `block/escalate` enforcement exists

### Check 4: Same canonical section structure

Run:

```bash
rg -n "^## " \
  /Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app/README.md \
  /Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-app-centric/README.md \
  /Users/alexis/Public/github-repos/examples/spring-platform/springboot-platform-platform-centric/README.md
```

Expected result:

- the same section headings, in the same order, across all three READMEs

### Check 5: Human-first docs

Manual check:

- a reader can understand the model from each README without opening
  `AI_START_HERE.md`
- AI docs add pacing and prompts, not core missing architecture
- AI-first material is available but visibly secondary to human understanding

### Check 6: Public command surface is small

Manual check:

- each README exposes no more than five primary commands
- low-level scripts are grouped as internals or advanced helpers

### Check 7: Repo tree teaches the model

Manual check:

- a human can glance at the example top-level tree and recognize inputs,
  generator, outputs, governed view, and route-specific material
- the top level does not look like an undifferentiated pile of shell scripts
- support scripts are grouped clearly enough that the model is visible from the
  filesystem layout

---

## What AI Must Not Do

This section exists specifically to prevent cheating.

An AI assistant must not:

- reword ambiguity instead of removing it
- keep wrapper/delegation architecture but merely hide the words
- claim a capability is implemented because a read-only preview exists
- claim a route is enforced because a bundle or dry-run command exists
- bury implementation gaps far below the fold
- preserve three different README structures for the sake of local convenience
- make the AI guides better while leaving the human READMEs confusing
- let AI-first prompt choreography dominate the reader-facing docs
- leave the repo tree messy while claiming the example is now easy to understand

---

## Definition Of Done

The rewrite is done only when all of the following are true:

1. The three examples read as peer examples, not wrappers.
2. The core `cub-gen` generator story is explicit in all three.
3. Plain ConfigHub, `ADT`, and experimental `ADTP` are obvious and comparable.
4. A human can open any example and immediately see the same model.
5. Real vs simulated vs not implemented is visible near the top.
6. Script sprawl is not exposed as the reader-facing interface.
7. AI-first support is present but not in the way of human understanding.
8. The repo tree itself helps teach the model.
9. The verification checks in this file pass.

---

## Final Reader Test

Use this as the last review question:

> If someone reads the plain-English introduction first and then opens any one
> of these three examples, do they instantly recognize the same underlying
> model and understand why this example exists?

If the answer is not an immediate yes, the rewrite is not done.
