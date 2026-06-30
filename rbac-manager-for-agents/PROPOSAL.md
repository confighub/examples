# Proposal: RBAC Manager for Agents

**Status:** Draft for review
**Date:** 2026-06-22
**Location:** `examples/rbac-manager-for-agents`

## 1. Goal

Deliver the capabilities of the [RBAC Manager web app](../rbac-manager) in a form that an **AI
agent operating in a terminal** can use effectively and safely. The web app gives a human a
console for managing **Kubernetes RBAC config as data** across a ConfigHub fleet. We want an agent
to do the same work — inventory, "who can", hygiene findings, guardrailed edits, and fleet
propagation — through terminal-native tooling.

**Scope is Kubernetes RBAC only:** Role / ClusterRole / RoleBinding / ClusterRoleBinding /
ServiceAccount stored in Units across cluster-Spaces. ConfigHub's own per-entity permissions are
explicitly out of scope.

## 2. What the web app provides (and what to port)

The web app is a static SPA with **no app-specific backend** — all state lives in ConfigHub, and
it talks to ConfigHub through the published HTTP API/SDK. Its substance is in three layers:

1. **A fleet-snapshot orchestration layer** — runs two parallel server-side `get-resources`
   function invocations (RBAC kinds + ServiceAccounts), joins the results with Unit/Space/Target
   metadata, applies a scope rule, and decodes the resource lists.
2. **A pure analysis engine** (React-free, unit-tested) — the real value-add over raw `cub`:
   - **model** — parse K8s RBAC resources from Unit bodies into typed entities with provenance.
   - **semantics** — full K8s authorization matching (verb/resource/apiGroup wildcards,
     subresources, `resourceNames`, `nonResourceURLs`, RoleBinding→ClusterRole resolution,
     namespace scoping, **ClusterRole aggregation to a fixed point**, builtin ClusterRoles).
   - **whocan** — effective-access engine: `whoCan(verb, resource, ns?, name?)`, the inverse
     `subjectAccess()`, `allSubjects()`.
   - **findings** — six hygiene analyzers (wildcards, privilege-escalation verbs, risky grants,
     cluster-admin bindings, orphaned bindings, unbound service accounts).
3. **A governance / write layer** — structured edits compiled to server-side `yq-i` functions
   (dry-run → commit with change description), fleet bulk edits, override-preserving variant
   propagation, apply/approve/rollback, and a guardrail pack of Triggers + ApplyGates
   (`vet-schemas`, `vet-celexpr`, `vet-approvedby`).

**Key takeaway for the agent tool:** raw `cub` already does the CRUD, triggers, apply, and bulk
function invocation. What it does _not_ do is the **analysis engine** and the **fleet-snapshot
join**. An agent should not re-derive Kubernetes authorization semantics (especially ClusterRole
aggregation and wildcard matching) inside its context window on every task — that is exactly the
deterministic, testable logic we should ship as a binary.

## 3. Best-practice framing: CLI + Skills

Current Anthropic guidance points the same direction the ConfigHub tooling already leans:

- **Skills are the "brain"; tools are the "plumbing."** Skills are discoverable folders of
  instructions that teach an agent _when and how_ to use tools; they load on demand and keep
  context lean. ([Agent Skills](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills))
- **Prefer code/CLI execution over a wide tool surface.** Loading many tool definitions up front
  bloats context; having the agent call a CLI (and load only relevant instructions) is more
  token-efficient and composes better.
  ([Code execution with MCP](https://www.anthropic.com/engineering/code-execution-with-mcp),
  [Advanced tool use](https://www.anthropic.com/engineering/advanced-tool-use))
- **Design tools _for agents_, not for developers** — high-signal, token-efficient outputs;
  composable verbs. ([Writing tools for agents](https://www.anthropic.com/engineering/writing-tools-for-agents))

The `cub` CLI is already agent-friendly: agent-tuned help (`CONFIGHUB_AGENT=1 cub … --help`),
`-o json`/`jq`/`yq`/`--quiet` machine output, and the published ConfigHub Skills set with a strict
read/write `allowed-tools` permission discipline. The lowest-friction, most consistent choice is
to **extend that ecosystem**.

**Recommendation:** Build a Go CLI **`cub-rbac`** (dual-mode: standalone `cub-rbac …` and as a
`cub` plugin `cub rbac …`) that adds the missing higher-order RBAC operations, plus a set of
**agent Skills** that teach an agent to drive both `cub` and `cub-rbac`.

**MCP is deferred.** The target user is an agent in a terminal that already has Bash, so a CLI is
native and an MCP tool surface would only cost context. If a non-terminal host (desktop/web) needs
this later, we can add a thin MCP adapter over the same commands without forking logic. Not now.

## 4. Architecture

```
                 ┌─────────────────────────────────────────────┐
   AI agent ───► │  Agent Skills (ConfigHub skills style)       │
   (terminal)    │  rbac-audit · rbac-whocan · rbac-findings ·  │
                 │  rbac-edit · rbac-fleet · rbac-guardrails    │
                 └───────────────┬─────────────────────────────┘
                                 │ Bash
                 ┌───────────────▼─────────────┐     ┌──────────────────┐
                 │  cub-rbac (Go CLI, dual-mode)│     │  cub (existing)   │
                 │  • fleet snapshot orchestr.  │     │  • CRUD, triggers │
                 │  • analysis engine (Go port) │ ──► │  • apply/approve  │
                 │    model/semantics/whocan/   │exec │  • function invoke│
                 │    findings                  │ cub │  • -o json        │
                 │  • JSON-first output         │     └────────┬─────────┘
                 └──────────────────────────────┘              │ HTTP (cub session)
                                                       ┌────────▼─────────┐
                                                       │  ConfigHub server │
                                                       └───────────────────┘
```

### 4.1 How `cub-rbac` talks to ConfigHub: exec `cub` (not the SDK)

Important: This document is out of date. The RBAC manager now uses the SDK to call
ConfigHub APIs.

Shell out to `cub` for all ConfigHub I/O.

- **Auth for free.** The subprocess inherits the user's `cub` session — no token handling and no
  interactive device-auth flow to reimplement (an agent can't complete interactive login anyway).
  Preflight with `cub auth status` (server-verified) and `exec.LookPath("cub")`.
- **Lighter dependency tree.** Importing the SDK's function executor pulls in a heavy transitive
  tree (client-go, CEL, starlark, yq, kustomize). We avoid that by driving `cub` and parsing
  `-o json` / `-o jq=`.
- **Single source of truth.** `cub` already implements scoping, `--where`, filters, change
  descriptions, gates, and the function-invoke plumbing the snapshot needs. We don't duplicate it.

Concretely, the snapshot calls `cub function do --where … get-resources` (org-scoped invoke) with
a `WhereResource` filter for RBAC kinds, requests `-o json`, and parses. Mutations go through
`cub unit update` / `cub function do … yq-i` / `cub run` with `--change-desc`.

### 4.2 The one thing we keep in Go: the analysis engine

Port model / semantics / whocan / findings to Go as a self-contained package, parsing with
`k8s.io/api/rbac/v1` (lightweight; no function executor). This is pure, deterministic logic with
excellent unit-test coverage potential, and it is precisely what an agent should not recompute
in-context. Port the existing web-app test fixtures to Go table tests to lock behavior.

_(Alternative considered: call the ConfigHub Go SDK/HTTP API directly. Rejected as the default —
it means reimplementing auth/session, scoping, and change-description plumbing. Revisit only if a
needed operation has no `cub` surface, which appears rare.)_

### 4.3 Output: JSON-first, agent-shaped

- Default to compact, high-signal **JSON** on stdout (stable schemas), with `--output table` for
  humans. Findings / who-can / snapshot all emit structured results an agent can filter without
  re-reading raw YAML.
- Keep outputs token-lean: summaries + provenance (`space/unit/target/resourceName`), not full
  manifest dumps. Offer `--explain` for the reasoning trail and `--web` handoff links via `cub`.
- Errors: wrap with the failing `cub` command + stderr; embed the exact remediation command.

## 5. Command surface (web feature → `cub-rbac` subcommand)

Read (no mutation, safe to auto-approve):

| Web page    | `cub-rbac` command                                          | Notes                                                             |
| ----------- | ----------------------------------------------------------- | ----------------------------------------------------------------- |
| Dashboard   | `cub-rbac snapshot`                                         | Fleet inventory: per-cluster Units, gated/unapplied counts. JSON. |
| Explorer    | `cub-rbac list [--kind] [--cluster] [--persona]`            | Browse RBAC resources across the fleet.                           |
| Who can     | `cub-rbac who-can <verb> <resource> [--namespace] [--name]` | Effective-access query.                                           |
| (inverse)   | `cub-rbac access <subject>`                                 | What a subject can do (inverse query).                            |
| Findings    | `cub-rbac findings [--severity] [--analyzer]`               | Six hygiene analyzers.                                            |
| Unit detail | `cub-rbac show <space>/<unit>`                              | Friendly + raw, provenance, gate state.                           |

Write (mutating — dry-run by default, require `--change-desc`, never bypass gates):

| Web action                 | `cub-rbac` command                                              | Implementation                                                                  |
| -------------------------- | --------------------------------------------------------------- | ------------------------------------------------------------------------------- |
| Quick/structured edit      | `cub-rbac edit <unit> add-verb/remove-verb/add-subject/...`     | Compiles to server-side `yq-i` via `cub function do`; dry-run diff then commit. |
| Fleet bulk edit            | `cub-rbac fleet-edit --where … <op>`                            | Org-scoped `yq-i`; one server request, multi-diff preview.                      |
| Variant propagation        | `cub-rbac promote --where …`                                    | Override-preserving `--patch --upgrade`; reports "behind upstream".             |
| Apply / Approve / Rollback | defer to `cub unit apply/approve` + `cub unit update --restore` | Routed via skills.                                                              |
| Guardrail pack             | `cub-rbac guardrails install --policy-space … --where-space …`  | Triggers + Filter + `TriggerFilterID` wiring.                                   |

## 6. Agent Skills to ship

Authored in the ConfigHub skills format (SKILL.md frontmatter with a phrase-dense `description`,
`phase`, scoped `allowed-tools`, preflight gates starting with `cub auth status`, do/don't
routing, evals). Split read vs write so read-only skills physically cannot mutate.

- **`rbac-audit`** (read) — inventory + explorer; "what RBAC do we have across the fleet?"
- **`rbac-whocan`** (read) — "who can delete secrets in prod?", "what can the ci SA do?"
- **`rbac-findings`** (read) — "audit our RBAC hygiene", "any cluster-admin bindings?"
- **`rbac-edit`** (write) — guardrailed single/structured edits with change descriptions.
- **`rbac-fleet`** (write) — fleet bulk edits + variant propagation.
- **`rbac-guardrails`** (write) — install/inspect the Trigger+ApplyGate policy pack.

Each routes to the others and to the existing ConfigHub skills (`cub-apply`, `rollback-revision`,
`triggers-and-applygates`, `promote-release`).

## 7. Safety model (RBAC changes are high-stakes)

- **Read tools are read-only**, enforced by `allowed-tools` scoping (no `cub *`, no `cub * delete`).
- **Dry-run by default** for every mutation; show the diff, commit only on confirmation.
- **Every Unit-data mutation carries `--change-desc`** (summary + verbatim user prompt +
  clarifications) for reviewable provenance.
- **Never bypass ApplyGates/approvals.** Surface gate state; route approval to a human.
- **Preflight gates:** `cub auth status` (server-verified), caller capability checks, subject
  resolution, valid analysis scope.
- **Lock-out guard:** warn loudly on edits that touch cluster-admin / privilege-escalation verbs
  or remove the operator's own access.

## 8. Repo layout, build, testing

```
examples/rbac-manager-for-agents/
  go.mod                       # separate module; depends on cub as a subprocess
  cmd/cub-rbac/main.go         # thin: plugin hook then cli.NewRoot().Execute()
  internal/cli/                # one cobra file per command, root.go wires + SilenceUsage
  internal/cub/                # exec helpers (capture stderr, wrap errors with remediation)
  internal/snapshot/           # fleet snapshot orchestration (the get-resources join)
  internal/rbac/               # PORTED analysis engine: model, semantics, whocan, findings
  internal/rbac/testdata/      # fixtures ported from the web app's engine tests
  skills/<slug>/SKILL.md       # agent skills + evals/
  Makefile                     # build + release-build (ldflags version), test, vet, fmt
  README.md
```

- **Dual-mode binary:** standalone `cub-rbac …` and, when invoked by `cub`, `cub rbac …`
  (plugin hook in `main.go`; help text reflects how it was invoked).
- **Stack:** Go, cobra (+ pflag), `exec.CommandContext` for `cub`, `k8s.io/api/rbac/v1` for
  parsing. Separate Go module; depend on `cub` as a subprocess, not the heavy function executor.
- **Build:** self-contained Makefile; binary named `cub-rbac`. Add the binary to `.gitignore`.
- **Tests:** Go table tests for the analysis engine (port the web-app fixtures — this is the
  correctness core); skill `evals/evals.json` for agent behavior. A demo/seed dataset stays a
  shell script (not a `cub-rbac` subcommand) for e2e and demos.

## 9. Milestones

- **M0 — Skeleton:** module, dual-mode cobra root, `cub` exec helper, `cub auth status` preflight.
- **M1 — Read engine:** port model/semantics + fixtures to Go; `snapshot` and `list`. JSON out.
- **M2 — Queries & findings:** `who-can`, `access`, `findings`. Ship `rbac-audit` / `rbac-whocan` /
  `rbac-findings` skills + evals.
- **M3 — Guardrailed writes:** `edit` (yq-i dry-run→commit, `--change-desc`), `guardrails install`.
  Ship `rbac-edit` / `rbac-guardrails` skills.
- **M4 — Fleet ops:** `fleet-edit`, `promote`. Ship `rbac-fleet` skill.

## 10. Open questions

1. **Confirm the `get-resources` invoke is fully expressible via `cub`** (the app uses two
   parallel org-scoped invokes). If not, that is an SDK/CLI gap to file, not work around.
2. **Persona/cluster modeling** — confirm the label conventions (`Variant=base`, standard Space
   labels) the snapshot and scope rule should rely on.
3. **Findings parity** — port all six analyzers verbatim, or trim to the highest-signal set for v1?

## Sources

- [Code execution with MCP — Anthropic](https://www.anthropic.com/engineering/code-execution-with-mcp)
- [Writing effective tools for AI agents — Anthropic](https://www.anthropic.com/engineering/writing-tools-for-agents)
- [Equipping agents for the real world with Agent Skills — Anthropic](https://www.anthropic.com/engineering/equipping-agents-for-the-real-world-with-agent-skills)
- [Introducing advanced tool use — Anthropic](https://www.anthropic.com/engineering/advanced-tool-use)
