# Proof Language Stop Point

Stop point after the documentation and standards phase.

## What This Phase Achieved

1. **AI guide standard documented and enforced**
   - Created `incubator/ai-guide-standard.md` explaining required files and markers
   - Extended verifier to enforce CRITICAL: Demo Pacing, Suggested Prompt, Stage headings, GUI gap, GUI feature ask

2. **Machine-readable contract standard documented**
   - Created `EXAMPLE_CONTRACT_STANDARD.md` at repo root
   - Documents `--explain-json`, `contracts.md`, and `verify.sh` requirements
   - Enforced `mutates:` and `proves:` markers in all contracts.md files

3. **Verifier coverage expanded to 30 examples**
   - 4 stable examples: spring-platform (2), campaigns-demo, promotion-demo-data
   - 26 incubator examples across global-app-layer, gitops-import, apptique, and others

4. **Proof language tightened for 3 primary examples**
   - springboot-platform-app: clarified that only apply-here is fully proven
   - single-component: clarified that verify.sh proves ConfigHub-only structure
   - platform-write-api: documented dependency on spring-platform sibling

## Key Commit Chain

```
7c22835 — docs: incubator usability + AI-first cleanup pass
3928d56 — fix: repair malformed fenced code blocks in ai-example-template
6909c9d — docs: expand AI guide verifier to all 26 runnable incubator examples
ee1a820 — docs: add AI guide standard reference doc
6686fe3 — docs: extend AI guide standard to stable examples
feb441a — docs: add machine-readable example contract standard
829db35 — fix: enforce mutates/proves markers in contracts.md
11a3250 — fix: correct single-component contracts.md spaces count
<pending> — docs: tighten proof language for 3 primary examples
```

## Current Verifier Baseline

`scripts/verify.sh` now enforces:
- Shell script syntax (bash -n)
- Required files: README.md, AI_START_HERE.md, contracts.md
- AI guide markers: CRITICAL: Demo Pacing, Suggested Prompt, Stage headings, GUI gap, GUI feature ask
- setup.sh --explain and --explain-json support
- contracts.md mutates: and proves: markers (case-insensitive)

Exemptions:
- `campaigns-demo` and `promotion-demo-data`: exempt from contracts.md and --explain
- `incubator/watch-webhook`: exempt from contracts.md

## Three Examples Reviewed

### springboot-platform-app

| Proof level | Status |
|-------------|--------|
| Read-only preview | Proven (--explain, verify.sh) |
| ConfigHub-only | Proven (confighub-setup.sh, confighub-verify.sh) |
| Direct live apply | Proven (--with-targets, verify-e2e.sh) |
| Apply-here mutation | Fully proven (cub function do, audit trail) |
| Lift-upstream | Documented only (bundle exists, no PR automation) |
| Block-escalate | Documented only (boundary exists, no server enforcement) |

### single-component

| Proof level | Status |
|-------------|--------|
| Read-only preview | Proven (--explain-json) |
| ConfigHub-only | Proven (setup.sh, verify.sh) |
| Direct Kubernetes | Code path working; manual verification with target |
| Flux OCI | Code path working; manual verification with target |
| Argo OCI | Code path working; manual verification with target |
| Controller reconciliation | Not automated in verify.sh |

### platform-write-api

| Proof level | Status |
|-------------|--------|
| Read-only preview | Proven (--explain-json) |
| ConfigHub-only | Proven (setup.sh, mutate.sh) |
| Apply-here mutation | Proven (mutate.sh with audit) |
| Lift-upstream | Documented (bundle exists, no PR automation) |
| Block-escalate | Documented (boundary exists, no server enforcement) |
| Field lineage | Depends on spring-platform sibling |

## Residual Work

1. **Lift-upstream automation**: cub-gen #208 (PR creation not implemented)
2. **Block-escalate server enforcement**: cub-gen #207 (server-side blocking not implemented)
3. **Controller proof automation**: verify.sh could be extended to check Flux/Argo reconciliation status for single-component
4. **platform-write-api self-containment**: could inline the delegated scripts from spring-platform

## Obvious Next Step

Add automated controller-proof checks to single-component verify.sh when targets are bound:
- If `FLUX_TARGET_REF` is set, verify `OCIRepository` exists and reconciled
- If `ARGO_TARGET_REF` is set, verify `Application` is Synced and Healthy

This would make verify.sh prove what the Delivery Matrix claims.
