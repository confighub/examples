# AI Start Here

Use this page when you want to drive the stable `graph-export` example safely with an AI assistant.

## CRITICAL: Demo Pacing

Pause after every stage.

For each stage:

1. run only that stage's commands
2. print the full output
3. explain what it means in plain English
4. print the GUI checkpoint when applicable
5. say what the GUI shows today
6. say what the GUI does not show yet
7. name the GUI feature ask and cite the issue number if one exists; if not, say that explicitly
8. tell the human to open the GUI and give them time to inspect it
9. ask `Ready to continue?`
10. do not move on until the human says to continue

## Suggested Prompt

```text
Read graph-export/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
For each stage, tell me what the GUI shows today, what it does not show yet, and the feature ask.
Give me time to click through the GUI before continuing.
Do not continue until I say continue.
```

## What This Example Is For

This example is for exporting live cluster topology into deterministic graph artifacts.

It creates a small local `kind` cluster, applies one native Deployment, and exports the graph as JSON, DOT, SVG, and HTML.

It does not mutate ConfigHub.

## Stage 1: Preview The Plan (read-only)

```bash
cd graph-export
./setup.sh --explain
./setup.sh --explain-json | jq
```

These commands do not mutate ConfigHub and do not mutate live infrastructure.

GUI checkpoint:

- GUI now: none; this stage is CLI-only preview
- GUI gap: there is no GUI preview for the graph export plan before the local cluster exists
- GUI feature ask: no issue filed yet for a pre-run graph export preview

Pause after this stage.

## Stage 2: Create The Fixture Cluster And Export Artifacts (mutates live infrastructure and local files)

```bash
./setup.sh
```

Important boundaries:

- `./setup.sh` mutates live infrastructure by creating a local cluster and applying one Deployment
- `./setup.sh` also writes local graph artifacts under `sample-output/`
- it does not mutate ConfigHub

What you should see after:

- one `Deployment`, `ReplicaSet`, and `Pod` in namespace `graph-demo`
- `graph.json`, `graph.dot`, `graph.svg`, and `graph.html` under `sample-output/`

GUI checkpoint:

- GUI now: open `sample-output/graph.html` in a browser to see the rendered topology artifact
- GUI gap: there is no richer in-product graph explorer or side-by-side evidence panel in this example
- GUI feature ask: no issue filed yet for a stronger artifact viewer or CLI-to-GUI graph handoff

Pause after this stage.

## Stage 3: Verify The Evidence (read-only)

```bash
./verify.sh
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

GUI checkpoint:

- GUI now: `sample-output/graph.html` shows the same topology as a browser-friendly artifact
- GUI gap: there is no linked inspector that connects nodes in the HTML export back to the raw JSON or the live cluster objects
- GUI feature ask: no issue filed yet for a linked graph artifact inspector

Pause after this stage.

## Stage 4: Cleanup

```bash
./cleanup.sh
```

This deletes the local `kind` cluster and clears the local sample output.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
