# 2026-03-29 Native OCI Doc Correction Pass

## Summary

Completed proof-first evidence pass and native-OCI doc correction pass across incubator examples.

## Script Fixes

1. **single-component/setup.sh**: Fixed unbound variable bug when calling `./setup.sh --explain` without target args. The script now uses `${#target_refs[@]} -gt 0` check before array expansion under `set -u`.

2. **gpu-eks-h100-training/contracts.md**: Fixed space count from 7 to 8 (includes three deployment variant spaces).

## Native OCI Doc Corrections

Reframed documents to reflect that ConfigHub-native OCI is an implemented core capability:

### proposal-oci-api-confighub.md
- Added status header noting the core surface is implemented
- Changed from proposal ("should expose") to present tense ("exposes")
- Documented remaining gaps: user-facing docs, full e2e proof, productization polish

### how-it-works.md
- Updated target table to mention ConfigHub-native OCI origin
- Updated Flux OCI flow diagram to show ConfigHub-native OCI as default
- Added note that external registries are optional

### 07-argo-oci-spec.md
- Updated publication contract to show ConfigHub-native OCI origin
- Updated Application source examples to use ConfigHub OCI URLs
- Marked "Registry Configuration" as answered (ConfigHub-native is default)
- Updated comparison table status from "Target-state" to "Implemented"

### 08-oci-distribution-spec.md
- Marked as "Historical Exploration (Superseded)"
- Added note that ConfigHub-native OCI is now the standard path
- Renamed "Proposed Architecture" to "Historical Proposed Architecture (Superseded)"

### single-component/README.md
- Updated verification contracts to require full proof chain
- Added explicit evidence chain requirement: `ConfigHub revision -> OCI ref/digest -> controller source -> live workload`
- Updated "What You Should Expect To See" for Argo OCI

### gitops-import-flux/README.md
- Added note clarifying that this example provisions `fluxoci` infrastructure but does not by itself prove native OCI delivery
- Referenced global-app-layer examples for full OCI proof

### gitops-import-argo/README.md
- Added important note distinguishing ArgoCDRenderer from Argo OCI delivery
- Referenced global-app-layer examples for native Argo OCI proof

### WHY_CONFIGHUB.md
- Updated delivery matrix to mention ConfigHub-native OCI origin
- Added note that external registries are optional

## Revised Evidence Matrix

| Example | Read-only Preview | Docs Updated | Live OCI Proof |
|---------|-------------------|--------------|----------------|
| springboot-platform-app | Proven | N/A (not OCI) | N/A |
| gitops-import-argo | Proven | Updated | Not this example's scope |
| gitops-import-flux | Proven | Updated | Provides infrastructure |
| single-component | Proven (after fix) | Updated | Unproven (requires live run) |
| gpu-eks-h100-training | Proven | Updated | Unproven (requires live run) |

## Key Distinctions Now Documented

1. **ConfigHub-native OCI origin** is the default path; external registries are optional
2. **ArgoCDRenderer** is not Argo OCI delivery (renderer vs controller)
3. **"Publishes an OCI artifact"** is not sufficient - require full proof chain
4. **gitops-import-* examples** provide infrastructure but don't prove OCI delivery by themselves

## Next Steps

1. Run live Flux OCI proof through single-component with a real `FluxOCI` target
2. Run live Argo OCI proof through single-component with a real `ArgoCDOCI` target
3. Capture full evidence chain for each delivery mode
4. Update global-app-layer README with live proof evidence when captured

## Files Changed

```
incubator/global-app-layer/single-component/setup.sh
incubator/global-app-layer/gpu-eks-h100-training/contracts.md
incubator/proposal-oci-api-confighub.md
incubator/global-app-layer/how-it-works.md
incubator/global-app-layer/07-argo-oci-spec.md
incubator/global-app-layer/08-oci-distribution-spec.md
incubator/global-app-layer/single-component/README.md
incubator/gitops-import-flux/README.md
incubator/gitops-import-argo/README.md
incubator/WHY_CONFIGHUB.md
```
