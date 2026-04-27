# Contracts

## Read-Only Contracts

### `./setup.sh --explain-json`

- mutates: no
- output shape: stable JSON plan for the example
- proves:
  - which spaces will be created
  - which components participate
  - which layered dimensions are used
  - which STACK is selected
  - whether targets were provided
- expected anchors:
  - `.example == "global-app-layer-enterprise-rag-blueprint"`
  - `.mutates == false`
  - `.spaces | length == 8`
  - `.components | length == 4`
  - `.components | map(.component) == ["rag-server","nim-llm","nim-embedding","vector-db"]`
  - `.recipeManifest.unit == "recipe-enterprise-rag-stack"`
  - `.stack | IN("stub","ollama","nim")`

### `./verify.sh --json`

- mutates: no
- output shape: JSON object with `ok`, `example`, and either verification detail or `error`
- proves:
  - whether the example's ConfigHub state passes verification
  - which spaces and units were checked on success
  - that failures are surfaced as structured JSON
- expected anchors on success:
  - `.ok == true`
  - `.example == "global-app-layer-enterprise-rag-blueprint"`
  - `.spacesChecked | length == 8`
  - `.unitsChecked | length == 33`  # 4 components * 5 chain stages + 4 components * 3 deploy variants + 1 recipe manifest = 33
  - `.stack` matches the STACK env when setup ran

## ConfigHub State Contracts

### `cub space get <prefix>-recipe-enterprise-rag --json`

- mutates: no
- output shape: JSON object containing `Space` plus summary counters
- proves: the recipe space exists; `SpaceID` and labels are inspectable
- jq anchor:
  - `cub space get <prefix>-recipe-enterprise-rag --json | jq '.Space | {slug: .Slug, id: .SpaceID, labels: .Labels}'`

### `cub unit get --space <prefix>-recipe-enterprise-rag --json recipe-enterprise-rag-stack`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, and `UnitStatus`
- proves: the stack-level recipe receipt exists with rendered provenance
- jq anchor:
  - `cub unit get --space <prefix>-recipe-enterprise-rag --json recipe-enterprise-rag-stack | jq '.Unit | {slug: .Slug, revision: .HeadRevisionNum, labels: .Labels}'`

### `cub unit get --space <prefix>-deploy-tenant-acme --json rag-server-tenant-acme`

- mutates: no
- output shape: JSON object containing `Space`, `Unit`, `UnitStatus`, and often `UpstreamUnit`
- proves: the direct deployment-layer rag-server unit exists; for STACK=ollama, the rendered unit data contains `host.docker.internal`
- jq anchor:
  - `cub unit get --space <prefix>-deploy-tenant-acme --json rag-server-tenant-acme | jq '.Unit | {slug: .Slug, upstreamUnitID: .UpstreamUnitID, targetID: .TargetID, revision: .HeadRevisionNum}'`

### `cub view list --space <prefix>-recipe-enterprise-rag --json`

- mutates: no
- output shape: JSON array of Views
- proves: after `./seed-initiatives.sh`, the five initiative Views exist with the expected priority labels
- jq anchor:
  - `cub view list --space <prefix>-recipe-enterprise-rag --json | jq '[.[] | {slug: .View.Slug, priority: .View.Labels."initiative-priority", status: .View.Labels."initiative-status"}]'`

### `cub unit tree --edge clone --where "Labels.ExampleName = 'global-app-layer-enterprise-rag-blueprint'"`

- mutates: no
- output shape: text tree
- proves: the layered Blueprint ancestry exists in a human-readable view spanning all four components

### `./.logs/setup.latest.log`, `./.logs/verify.latest.log`, `./.logs/set-target.latest.log`

- mutates: no
- output shape: plain text
- proves: each command completed and its summary line + GUI URLs are durable for later inspection

## Expected Output Signals

When `./verify.sh` succeeds, expect:
- final line `All enterprise-rag-blueprint checks passed (stack=<stack>).`
- no clone-chain error output
- no missing-space or missing-unit errors
- no "value: ..." mismatches in any layer
- if `STACK=ollama`, the direct deployment unit for rag-server contains `host.docker.internal`

When `./query.sh` (STACK=ollama path) succeeds, expect:
- `/health` JSON has `ok: true`, `stack: "ollama"`, and the model/host fields populated
- `/answer` JSON has a non-empty `answer` field with no `(LLM call ... failed:` prefix
