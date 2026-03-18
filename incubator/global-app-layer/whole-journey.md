# Whole Journey

Use this guide when you want the **full** layered-example story, not just `setup.sh` + `verify.sh`.

This walkthrough uses `realistic-app` as the main live-app example because it has:
- a real app shape
- a clear live deployment story
- a clear shared-update story
- a clear custom downstream variant story

The flow is:
1. preview the recipe
2. materialize it in ConfigHub
3. verify it in ConfigHub
4. bind a live target and apply
5. update the shared upstream chain
6. create a custom downstream deployment variant

## Before You Start

You need:
- `cub` in `PATH`
- `jq`
- an authenticated ConfigHub context
- a real target if you want the live path

Check capability first:

```bash
which cub
cub version
cub context list --json | jq
cub target list --space "*" --json | jq
```

If no target is available, stop after the ConfigHub-only path.

Important:
- `cub target list` only proves a target is visible in ConfigHub
- it does **not** prove the target's worker is online
- do not call the live path ready until `./preflight-live.sh <space/target>` reports `applyReady: true`

## 1. Preview The Recipe

```bash
cd incubator/global-app-layer/realistic-app

./setup.sh --explain
./setup.sh --explain-json | jq
```

This is read-only.

## 2. Materialize In ConfigHub

```bash
./setup.sh
./verify.sh
```

After setup, prefer the durable artifacts over scrollback:
- `.logs/setup.latest.log`
- `.logs/verify.latest.log`
- the clickable GUI URLs printed by `./setup.sh`

At this point you have:
- five ConfigHub spaces
- three deployment chains
- one recipe manifest
- no live deployment yet

## 3. Bind A Live Target And Apply

Before binding or applying, preflight the target:

```bash
cd incubator/global-app-layer
./preflight-live.sh <space/target>
./preflight-live.sh <space/target> --json | jq
```

If preflight fails, stop the live branch there.
You can still continue with ConfigHub-only upgrades and custom downstream variants.

If you already know the target:

```bash
../preflight-live.sh <space/target>
./setup.sh <prefix> <space/target>
./verify.sh
```

If you started ConfigHub-only, continue like this:

```bash
../preflight-live.sh <space/target>
./set-target.sh <space/target>
./verify.sh
```

Then approve and apply the deployment units:

```bash
./apply-live.sh
```

`./apply-live.sh` is the preferred live path because it:
- re-checks that the target is actually apply-ready
- refreshes the deployment clones from upstream before delivery
- applies the namespace bootstrap unit first
- waits for completion instead of treating "apply started" as success

What to inspect next:
- the deploy space URL printed by `./setup.sh` or `./set-target.sh`
- one deployment unit, for example `backend-cluster-a`
- `.logs/set-target.latest.log`
- `.logs/apply-live.latest.log`

## 4. Update The Shared Upstream Chain

This proves the value of clone-linked variants: update upstream once, then move it downstream safely.

```bash
./upgrade-chain.sh 1.1.8 1.1.8 16.1
./verify.sh
```

If you already have a target and want to push the updated deployment live:

```bash
./apply-live.sh
```

## 5. Create A Custom Downstream Deployment Variant

This is the step that proves the examples are not only about one fixed deploy space.

The commands below create a **new deployment-only downstream variant** for `cluster-b`.
They reuse the same helper functions the example scripts use.

```bash
cd incubator/global-app-layer/realistic-app
source ./lib.sh
load_state

CUSTOM_CLUSTER="cluster-b"
CUSTOM_SPACE="$(state_prefix)-deploy-${CUSTOM_CLUSTER}"

_mapfile custom_space_labels < <(
  space_label_args deployment \
    --label "Region=${REGION_VALUE}" \
    --label "Role=${ROLE_VALUE}" \
    --label "Cluster=${CUSTOM_CLUSTER}"
)
create_space_if_missing "${CUSTOM_SPACE}" "${custom_space_labels[@]}"

for component in "${COMPONENTS[@]}"; do
  custom_unit="${component}-${CUSTOM_CLUSTER}"

  _mapfile custom_unit_labels < <(
    label_args deployment "${component}" \
      --label "Region=${REGION_VALUE}" \
      --label "Role=${ROLE_VALUE}" \
      --label "Cluster=${CUSTOM_CLUSTER}"
  )

  create_clone_unit "${CUSTOM_SPACE}" "${custom_unit}" \
    "$(recipe_space)" "$(unit_name "${component}" recipe)" \
    "${custom_unit_labels[@]}"

  cub function do set-namespace "${CUSTOM_CLUSTER}" --space "${CUSTOM_SPACE}" --unit "${custom_unit}"
  cub function do set-env "${component}" "CLUSTER=${CUSTOM_CLUSTER}" --space "${CUSTOM_SPACE}" --unit "${custom_unit}"
done

cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host \
  "backend.${CUSTOM_CLUSTER}.demo.confighub.local" \
  --space "${CUSTOM_SPACE}" --unit "backend-${CUSTOM_CLUSTER}"

cub function do set-string-path networking.k8s.io/v1/Ingress spec.rules.0.host \
  "frontend.${CUSTOM_CLUSTER}.demo.confighub.local" \
  --space "${CUSTOM_SPACE}" --unit "frontend-${CUSTOM_CLUSTER}"
```

Verify the custom variant:

```bash
cub unit list --space "${CUSTOM_SPACE}" --quiet --json | jq '.[] | {slug: .Unit.Slug, upstream: (.UpstreamUnit.Slug // null), status: .UnitStatus.Status}'

cub unit get --space "${CUSTOM_SPACE}" --data-only backend-${CUSTOM_CLUSTER} | grep -E "(namespace:|CLUSTER|host:)"
```

If you have a second live target, you can bind and apply this custom variant too:

```bash
SECOND_TARGET="<space/target>"

cub unit set-target "${SECOND_TARGET}" --space "${CUSTOM_SPACE}" backend-${CUSTOM_CLUSTER}
cub unit set-target "${SECOND_TARGET}" --space "${CUSTOM_SPACE}" frontend-${CUSTOM_CLUSTER}
cub unit set-target "${SECOND_TARGET}" --space "${CUSTOM_SPACE}" postgres-${CUSTOM_CLUSTER}

cub unit approve --space "${CUSTOM_SPACE}" backend-${CUSTOM_CLUSTER}
cub unit approve --space "${CUSTOM_SPACE}" frontend-${CUSTOM_CLUSTER}
cub unit approve --space "${CUSTOM_SPACE}" postgres-${CUSTOM_CLUSTER}

cub unit apply --space "${CUSTOM_SPACE}" backend-${CUSTOM_CLUSTER}
cub unit apply --space "${CUSTOM_SPACE}" frontend-${CUSTOM_CLUSTER}
cub unit apply --space "${CUSTOM_SPACE}" postgres-${CUSTOM_CLUSTER}
```

This is the main customization proof:
- keep one shared recipe
- keep one shared upgrade path
- create an extra downstream deployment variant only where needed

## 6. Cluster-Proof End-To-End

If you want automated live cluster proof instead of a manual walkthrough, use the greenfield e2e flow:

```bash
./incubator/global-app-layer/e2e/02-greenfield.sh
```

Or deliver one already-materialized example directly:

```bash
./incubator/global-app-layer/e2e/deliver-direct.sh realistic-app
./incubator/global-app-layer/e2e/assert-cluster.sh realistic-app
```

See [e2e/README.md](./e2e/README.md) for prerequisites and flow details.

## GPU Note

For `gpu-eks-h100-training`, the same lifecycle applies:
- preview
- materialize
- verify
- bind/apply
- upgrade
- create extra downstream deployment variants

But functional NVIDIA proof also requires:
- real NVIDIA images instead of the stub images
- GPU-capable nodes
- correct NVIDIA drivers and node labeling

Without those, the GPU example remains structural proof, not full functional proof.
