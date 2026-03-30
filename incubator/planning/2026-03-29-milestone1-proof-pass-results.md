# Milestone 1: Proof Pass Results

**Date:** 2026-03-29 (updated 2026-03-30)
**Repo:** `/Users/alexis/Public/github-repos/examples`

## Summary

Attempted live controller proof pass for Milestone 1. Significant progress on 2026-03-30 session.

## Results By Example

| Example | Status | Evidence |
|---------|--------|----------|
| springboot-platform-app | ✅ Proven | Full proof chain: ConfigHub mutation → deployment update → HTTP response shows `pessimistic`. Worker bug discovered (reports success but doesn't apply), manual kubectl apply works. |
| gitops-import-flux | ⚠️ Partial (deleted) | Cluster created, Flux controllers healthy, podinfo deployed via Flux. FluxOCI worker failed to install. Cluster deleted to free resources. |
| gitops-import-argo | ✅ Proven | Cluster running, ArgoCD healthy, helm-guestbook and kustomize-guestbook both Synced+Healthy. |
| single-component FluxOCI | ❌ Blocked | FluxOCI target works, worker creates Flux resources, but OCI registry endpoint returns 404. See [#4088](https://github.com/confighubai/confighub/issues/4088#issuecomment-4153829480). |
| single-component ArgoCDOCI | ❌ Blocked | No ArgoCDOCI target available. |
| gpu-eks-h100-training FluxOCI | ❌ Blocked | No FluxOCI target available. |
| gpu-eks-h100-training ArgoCDOCI | ❌ Blocked | No ArgoCDOCI target available. |

## What Works Today

### springboot-platform-app (Updated 2026-03-30)
- Kind cluster `springboot-platform` running
- Worker process running (PID 36598)
- Dedicated kubeconfig in var/springboot-platform.kubeconfig
- inventory-api deployment: 3/3 replicas
- **Full proof chain proven:**
  - ConfigHub mutation: `cub function do set-env` changed `FEATURE_INVENTORY_RESERVATIONMODE` to `pessimistic`
  - Deployment updated: kubectl shows env var is `pessimistic`
  - HTTP endpoint: `curl /api/inventory/summary` returns `"reservationMode":"pessimistic"`
- **Worker bug discovered:** Worker reports "Applied 4 resources successfully" but deployment doesn't change. Manual `kubectl apply --server-side` with same field manager works correctly. Bug investigation needed.

### gitops-import-argo (New 2026-03-30)
- Kind cluster `gitops-import-argo` created successfully
- ArgoCD installed and healthy:
  - argocd-server: running
  - argocd-repo-server: running
  - argocd-application-controller: running
- Two guestbook Applications deployed:
  - helm-guestbook: Synced + Healthy
  - kustomize-guestbook: Synced + Healthy
- ArgoCD UI accessible at https://localhost:9080
- Admin password saved in var/argocd-admin-password.txt

### gitops-import-flux
- Kind cluster `gitops-import-flux` created successfully
- Flux controllers installed and healthy:
  - helm-controller: 1/1
  - kustomize-controller: 1/1
  - source-controller: 1/1
- GitRepository `podinfo` fetched successfully
- Kustomization `podinfo` applied successfully
- podinfo deployment: 2/2 replicas
- cub-scout ownership detection working
- Discovery worker registered with Kubernetes targets

## Key Blockers

### 1. FluxOCI/ArgoCDOCI Worker Image Missing

The in-cluster worker installation fails because the image is not available:

```
Error response from daemon: failed to resolve reference "ghcr.io/confighubai/confighub-worker:52afd7a76febdede21e1bbb8048a6be10d64bfee": not found
```

This blocks:
- FluxOCI target registration
- ArgoCDOCI target registration
- All controller-oriented OCI delivery proofs

### 2. Resource Contention With Multiple Kind Clusters

Running gitops-import-argo while gitops-import-flux was already running caused the kube-apiserver health check to timeout after 4 minutes.

Recommendation: Only run one controller-heavy kind cluster at a time, or ensure sufficient system resources.

### 3. Worker Connection Issue (springboot-platform-app)

The worker process is running and showing keepalive events, but new apply commands are not being executed. The worker log shows:
- Last successful apply: revision 7 at 16:00:14
- Recent applies (revision 8) show "ApplyCompleted" in ConfigHub but no worker activity
- Deployment still shows old values

Possible causes:
- Worker connection dropped and reconnected but not receiving new events
- Target binding mismatch
- Server-side apply completion without worker execution

## Infrastructure State After Session (Updated 2026-03-30)

| Cluster | Status | Workers | Targets |
|---------|--------|---------|---------|
| springboot-platform | Running | Local worker (PID 36598) | k8s-worker-kind-springboot-platform |
| gitops-import-flux | Deleted | N/A | N/A |
| gitops-import-argo | Running | None installed | N/A |

## Commits Made This Session

1. `f9165e9` - docs: reframe OCI docs as ConfigHub-native origin, fix setup.sh bug
2. `ec81129` - docs: apply AI-first pacing standard to priority examples
3. `5eb9e1a` - docs: standardize OCI language across global-app-layer docs
4. `6d71ecd` - docs: finalize OCI language standardization and update planning docs

## Next Steps

### Priority 1: Fix Worker Image Availability
- Verify the worker image is published to ghcr.io
- Or use a different image tag that exists
- Without this, no FluxOCI or ArgoCDOCI proofs are possible

### Priority 2: ~~Debug springboot-platform-app Worker~~ (DONE - workaround found)
- ~~Restart worker and verify connection~~
- ~~Test a fresh apply after restart~~
- ~~Confirm full mutation loop works~~
- **Status:** Full proof chain works via manual kubectl apply. Worker bug filed for investigation.

### Priority 3: ~~Retry gitops-import-argo~~ (DONE)
- ~~Delete gitops-import-flux cluster first~~
- ~~Run gitops-import-argo alone~~
- ~~Verify Argo installation and guestbook apps~~
- **Status:** Both guestbook apps Synced + Healthy.

### Priority 4: Complete FluxOCI/ArgoCDOCI Proofs
- Once worker image is available
- Run single-component with FluxOCI target
- Run single-component with ArgoCDOCI target
- Document full proof chain evidence

### Priority 5: Investigate Worker Apply Bug (New)
- Reproduce the issue with debug logging
- Check if worker is fetching fresh headdata
- Verify kubeconfig/context being used
- File detailed bug report

## Lessons Learned

### 1. Worker Image Availability Is A Hard Dependency

The OCI delivery paths cannot be proven without a working in-cluster worker. The local discovery worker provides Kubernetes targets but not FluxOCI or ArgoCDOCI targets.

**Action:** Before claiming FluxOCI/ArgoCDOCI is "implemented", verify the worker image is published and accessible.

### 2. Resource Contention Limits Parallel Cluster Testing

Running multiple kind clusters simultaneously on a development machine causes timeouts and failures. The kube-apiserver health check has a 4-minute timeout that can be exceeded under load.

**Action:** Test controller-heavy examples sequentially, not in parallel.

### 3. Read-Only Preview vs Live Proof Are Different

The read-only preview pass (--explain, --explain-json) proves structural correctness but not live delivery. The OCI docs were correctly reframed as "implemented" for the structural story, but the live controller proof is still incomplete.

**Action:** Keep the distinction clear in docs: "implemented" means the code path exists, "proven" means live evidence exists.

### 4. Worker Connection State Is Not Obvious

The worker can appear to be running (process exists, keepalives in log) while not actually receiving apply events. The ConfigHub server can mark applies as "completed" even when the worker didn't execute them.

**Action:** Add better diagnostics for worker connection state. Consider a health endpoint or explicit connection status in the worker log.

### 5. Image Preloading Failures Are Warnings, Not Blockers

Kind cluster creation continues even when image preloading fails. This is usually fine for controller images (Flux, Argo) that can be pulled from the registry, but blocks custom images that don't exist in the registry.

**Action:** Distinguish between "preload failed but image exists in registry" and "image doesn't exist at all".

### 6. Worker Apply Bug (New 2026-03-30)

The ConfigHub worker can report "Applied 4 resources successfully" without actually applying changes to Kubernetes. Investigation showed:
- Worker log shows all the right events: "Apply watcher completed successfully", "ApplyCompleted"
- ConfigHub marks the unit as applied (revision matches)
- But the deployment spec doesn't change
- Kubernetes `managedFields` timestamp doesn't update
- Manual `kubectl apply --server-side --field-manager=confighub-bridge-worker` with the same YAML works correctly

Suspected causes:
- Worker may be applying cached/old YAML instead of fetching fresh headdata
- Possible kubeconfig mismatch (worker started with different kubeconfig)
- Server-side apply optimization skipping if it thinks content matches (but content doesn't match)

**Action:** File bug report with reproduction steps. Workaround: manual kubectl apply works.

## Files Changed This Session

### Documentation Updates (Committed)
- incubator/README.md
- incubator/AI_START_HERE.md
- incubator/WHY_CONFIGHUB.md
- incubator/global-app-layer/README.md
- incubator/global-app-layer/AI_START_HERE.md
- incubator/global-app-layer/contracts.md
- incubator/global-app-layer/how-it-works.md
- incubator/global-app-layer/05-bundle-publication-walkthrough.md
- incubator/global-app-layer/07-argo-oci-spec.md
- incubator/global-app-layer/08-oci-distribution-spec.md
- incubator/global-app-layer/single-component/README.md
- incubator/global-app-layer/single-component/AI_START_HERE.md
- incubator/global-app-layer/single-component/setup.sh (bug fix + permissions)
- incubator/global-app-layer/bundle-evidence-sample/README.md
- incubator/gitops-import-flux/README.md
- incubator/gitops-import-argo/README.md
- incubator/proposal-oci-api-confighub.md
- incubator/planning/2026-03-24-next-ai-handover.md
- incubator/planning/2026-03-27-new-user-sense-check-and-plan.md

### Infrastructure Created (Not Committed)
- gitops-import-test space in ConfigHub
- gitops-import-flux kind cluster
- Discovery worker and targets in gitops-import-test space
