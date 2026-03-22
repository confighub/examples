# AI Start Here

Use this page when you want to drive `graph-export` safely with an AI assistant.

## What This Example Is For

This example is for exporting live cluster topology into deterministic graph artifacts.

It creates a small local `kind` cluster, applies one native Deployment, and exports the graph as JSON, DOT, SVG, and HTML.

It does not mutate ConfigHub.

## Read-Only First

Start here:

```bash
cd incubator/graph-export
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

## Recommended Path

```bash
./setup.sh
./verify.sh
```

## Important Boundaries

- `./setup.sh --explain` is read-only
- `./setup.sh` mutates live infrastructure and writes local graph artifacts
- `./verify.sh` is read-only with respect to ConfigHub and live infrastructure
- `./cleanup.sh` deletes the local `kind` cluster and local artifacts
- this example never writes ConfigHub state

## What To Verify

```bash
kubectl get deployment -n graph-demo
kubectl get replicaset -n graph-demo
kubectl get pod -n graph-demo
jq '.schema_version' sample-output/graph.json
jq '[.nodes[].kind] | unique' sample-output/graph.json
jq '[.edges[].type] | unique' sample-output/graph.json
```

Use the evidence like this:

- `kubectl` proves the fixture workload and generated children exist
- `graph.json` proves the stable `graph.v1` contract
- DOT, SVG, and HTML prove the same graph can be shared in different formats

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
