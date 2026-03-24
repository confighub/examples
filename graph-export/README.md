# Graph Export

This stable example adapts the `graph-export` flow from `cub-scout` into the official `examples` repo.

It shows a small live topology path:

- one local `kind` cluster
- one native Deployment fixture
- one canonical `graph.v1` JSON export
- three shareable renderings from the same graph data:
  - DOT
  - SVG
  - HTML

## What This Example Is For

Use this example when you want to show how a live cluster can be turned into deterministic topology artifacts for automation, diagrams, and handoff.

This example does not write ConfigHub state. It focuses on graph export artifacts.

## Source

This example is adapted from:

- [cub-scout graph-export](https://github.com/confighub/cub-scout/tree/main/examples/graph-export)

## What It Reads

It reads:

- the copied fixture manifest under `fixtures/`
- the current Kubernetes context created by `kind`
- the `cub-scout` binary

## What It Writes

It writes live infrastructure during setup:

- one local `kind` cluster
- one namespace: `graph-demo`
- one native `Deployment`

It writes local artifacts under `sample-output/`:

- `graph.json`
- `graph.dot`
- `graph.svg`
- `graph.html`

It does not write ConfigHub state.

## Read-Only First

```bash
cd graph-export
./setup.sh --explain
./setup.sh --explain-json | jq
```

## Quick Start

```bash
./setup.sh
./verify.sh
```

## Mutation Boundaries

`./setup.sh --explain` and `./setup.sh --explain-json` are read-only.

`./setup.sh` mutates live infrastructure by creating a local `kind` cluster and applying the fixture Deployment. It also writes local graph export artifacts.

`./verify.sh` is read-only with respect to ConfigHub and live infrastructure. It checks the live cluster and verifies the exported artifact contracts.

`./cleanup.sh` mutates live infrastructure by deleting the local `kind` cluster and clears local artifacts.

## What Success Looks Like

At the cluster level you should see:

- one `Deployment` named `graph-app` in `graph-demo`
- one generated `ReplicaSet`
- one generated `Pod`

At the export level you should see:

- `graph.json` with `schema_version: graph.v1`
- nodes for `Deployment`, `ReplicaSet`, and `Pod`
- `owns` edges in the graph
- DOT, SVG, and HTML files generated from the same graph payload

## Evidence To Check

Direct cluster evidence:

```bash
kubectl get deployment -n graph-demo
kubectl get replicaset -n graph-demo
kubectl get pod -n graph-demo
```

Graph export evidence:

```bash
./verify.sh
jq '{schema_version, cluster, nodeCount: (.nodes | length), edgeCount: (.edges | length)}' sample-output/graph.json
jq '[.nodes[].kind] | unique' sample-output/graph.json
jq '[.edges[].type] | unique' sample-output/graph.json
head -n 5 sample-output/graph.dot
head -n 5 sample-output/graph.svg
head -n 5 sample-output/graph.html
```

## Why This Example Matters

This gives the stable example set a small shareability example that is not about import.

It answers a practical question quickly:

- how do we turn live cluster relationships into artifacts we can inspect, automate on, and share?

That makes it a good companion to `custom-ownership-detectors` and the broader evidence-first wedge.

## AI-Safe Path

If you are driving this with an assistant, start here:

- [AI_START_HERE.md](./AI_START_HERE.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)

## Cleanup

```bash
./cleanup.sh
```
