# AI Start Here: watch-webhook

## CRITICAL: Demo Pacing

When walking a human through this example, you MUST pause after every stage.

After each stage:
1. Run the command(s) for that stage
2. Show the output faithfully on screen
3. Explain what the output means in plain English
4. If there is a GUI URL, print it
5. STOP and ask "Ready to continue?"
6. Only proceed when the human says to continue

## Suggested Prompt

```text
Read incubator/watch-webhook/AI_START_HERE.md and walk me through the demo.
Pause after every stage. Show full output.
Do not continue until I say continue.
```

## What This Example Teaches

This example demonstrates event delivery from `cub-scout watch` into a local webhook receiver. After the demo, the human will understand:

- How `cub-scout watch --webhook` streams events
- The event JSON schema
- One-shot event capture with `--once`

This example does NOT mutate ConfigHub.

## Prerequisites

- `kind` for local cluster
- `kubectl` in PATH
- `cub-scout` in PATH
- Python 3 for the webhook receiver
- `jq` for JSON inspection

---

## Stage 1: "Preview The Plan" (read-only)

Run:

```bash
cd incubator/watch-webhook
./setup.sh --explain
./setup.sh --explain-json | jq
```

What to explain:

- Creates a local kind cluster
- Applies fixture resources
- Starts a webhook receiver
- Runs one watch cycle

GUI now: No GUI checkpoint for this stage — this is CLI-only.

GUI gap: No visual preview of what resources will be created.

GUI feature ask: Setup preview for local examples. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 2: "Build The Environment" (mutates live infrastructure)

Run:

```bash
./setup.sh
```

What to explain:

- Creates a local kind cluster named `watch-webhook`
- Applies fixture resources to `watch-demo` namespace
- Starts a Python webhook receiver on a local port
- Runs `cub-scout watch --webhook --once` to capture events
- Events are written to `sample-output/webhook-events.jsonl`

GUI now: No GUI checkpoint — this is a local cluster example.

GUI gap: No visual dashboard for webhook event streams.

GUI feature ask: Event stream viewer in ConfigHub GUI. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 3: "Verify The Results" (read-only)

Run:

```bash
export KUBECONFIG=$PWD/var/watch-webhook.kubeconfig
./verify.sh
kubectl get all -n watch-demo
jq -r '.event.type' sample-output/webhook-events.jsonl
jq '.event.resource.namespace' sample-output/webhook-events.jsonl
```

What to explain:

- `kubectl` proves fixture resources exist
- The JSONL file proves events were captured
- Event types and namespaces are visible in the output

GUI now: No GUI checkpoint — this is a local example.

GUI gap: No event history viewer.

GUI feature ask: Event browser showing captured webhook events. No issue filed yet.

**PAUSE.** Wait for the human.

---

## Stage 4: "Cleanup"

Run:

```bash
./cleanup.sh
```

This deletes the local kind cluster and sample output.

---

## What Mutates What

| Command | Writes |
|---------|--------|
| `./setup.sh --explain-json` | Nothing |
| `./setup.sh` | Local kind cluster, local kubeconfig, local webhook-events.jsonl |
| `./verify.sh` | Nothing |
| `./cleanup.sh` | Deletes local kind cluster and sample output |

This example never writes ConfigHub state.

## Related Files

- [README.md](./README.md)
- [prompts.md](./prompts.md)
- [contracts.md](./contracts.md)
