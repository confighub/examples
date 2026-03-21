# Why `cub run`: Evidence from `promotion-demo-data`

This document captures observations from running the [promotion-demo-data](../../promotion-demo-data/README.md) example and how they validate the `cub run` design.

## What the Example Does

The `promotion-demo-data` example creates a realistic multi-app, multi-environment dataset in ConfigHub. It runs a shell script with clear phases:

| Phase | Action | Count |
|---|---|---|
| 1 | Create shared worker space and target spaces | 1 worker plus 7 targets |
| 2 | Create deployment spaces for each app and target combination | 42 spaces |
| 3 | Create units from YAML templates in `us-dev-1` only | 22 units |
| 4 | Clone and promote units across lower and prod environments | 36 clone phases |
| 5 | Apply labels across spaces and units | 42 spaces |
| 6 | Customize deployments per environment | 42 spaces |
| 7 | Add prod scaling and intentional version skew | selected prod spaces |

Totals:

- 49 spaces
- about 154 units
- many repeated CLI invocations with ordering constraints

## Why This Validates `cub run`

This example is a near-perfect `cub run` candidate.

### 1. It already has a bounded multi-phase shape

The script naturally maps to steps such as:

```text
Step 1/7: infrastructure       DONE
Step 2/7: app-spaces           DONE
Step 3/7: units-in-dev         DONE
Step 4/7: clone-and-promote    DONE
Step 5/7: labeling             DONE
Step 6/7: customize            DONE
Step 7/7: scale-and-skew       DONE
```

These phases have real ordering constraints. You cannot label units before they exist. You cannot create prod clones before staging exists.

### 2. It still lacks explicit assertions

The script prints progress, but the strongest proof points are still implicit.

A `cub run` profile would make the important checks explicit:

```text
Assertion: expected spaces created     PASS
Assertion: expected units created      PASS
Assertion: all units labeled           PASS
Assertion: prod scaling present        PASS
Assertion: version skew exists         PASS
```

### 3. It lacks a stable operational record

If the procedure fails part way through, the operator has to infer what already completed by querying ConfigHub manually.

That is exactly the gap an `Operation` record would fill.

### 4. It is a good first profile because it avoids live-cluster complexity

This example uses ConfigHub objects and a noop-style target story. That makes it a good low-risk place to prove the `cub run` model before moving into Argo, Flux, Kubernetes, or delegated waiting behavior.

## Suggested Profile

If `cub run` existed with hardcoded profiles today, this example would be a strong candidate for:

```bash
cub run demo-data/install --record summary --assert
```

## Why It Still Matters Now

The current examples do not need `cub run` to be useful.

That is not a weakness. It is exactly why this example is valuable as design evidence. It shows a real bounded procedure that is already useful today and can later become a cleaner operational record without changing the underlying example.

## Related Pages

- [README.md](./README.md)
- [procedure-candidates.md](./procedure-candidates.md)
- [03-cub-run-prd.md](./03-cub-run-prd.md)
- [03-cub-run-rfc.md](./03-cub-run-rfc.md)
