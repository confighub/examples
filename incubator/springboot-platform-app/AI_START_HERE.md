# AI Start Here

Walk the user through a real ConfigHub → Kubernetes deployment.

## The Demo

```bash
cd incubator/springboot-platform-app

# 1. Setup (takes ~2 min)
./bin/create-cluster
./bin/build-image
CUB_SPACE=springboot-infra ./bin/install-worker

# 2. Deploy
export WORKER_SPACE=springboot-infra
./confighub-setup.sh --with-targets

# 3. Verify
./verify-e2e.sh
# Shows: reservationMode = strict

# 4. Mutate
cub function do --space inventory-api-prod \
  --change-desc "change reservation mode" \
  -- set-env inventory-api "FEATURE_INVENTORY_RESERVATIONMODE=optimistic"
cub unit apply --space inventory-api-prod inventory-api

# 5. Verify mutation
./verify-e2e.sh
# Shows: reservationMode = optimistic

# 6. Cleanup
./confighub-cleanup.sh
./bin/teardown
```

## What to Explain

After step 3: "The app is deployed. The HTTP response comes from a real pod in the Kind cluster."

After step 5: "The mutation went through ConfigHub, got applied via kubectl, and the running app now returns the new value. This is real end-to-end."

## What NOT to Claim

This example does NOT prove:
- Lift-upstream (routing changes to PRs) - just a design doc
- Block/escalate (policy enforcement) - just a design doc

Only claim what's actually running.

## Pacing

Pause after each numbered step. Show full output. Let the user inspect before continuing.
