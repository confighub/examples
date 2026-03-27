# Copyable Prompts

## 1. Orient Me First

Read this example and do not mutate anything yet.

Explain:
- what this example is for
- what it reads
- what it writes
- which steps mutate ConfigHub
- which steps mutate live infrastructure
- what success looks like

Then run only read-only preview commands.

## 2. Safe Walkthrough

Guide me through `gitops-import-argo` step by step.

Before each command:
- explain what it does
- say whether it mutates ConfigHub
- say whether it mutates live infrastructure
- tell me what success looks like
- tell me what to inspect next

Start with `./setup.sh --explain` and `./setup.sh --explain-json | jq`.

If the local ArgoCD port is already in use, notice the chosen host port and verify it from `var/argocd-host-port.txt` before you continue.

## 3. Verify The Import

After the example is running, verify:
- the cluster is reachable
- ArgoCD applications exist
- the healthy reference applications (`helm-guestbook`, `kustomize-guestbook`) are present and healthy
- the worker targets exist if configured
- `cub gitops discover` found resources
- `cub gitops import` created `-dry` and `-wet` units
- the renderer stage completed without overclaiming live reconciliation
- any failing contrast Applications are reported as live evidence, not hidden
- `cub-scout` status and ownership views if `cub-scout` is installed

Separate cluster evidence, ConfigHub evidence, and `cub-scout` evidence in the final summary.
