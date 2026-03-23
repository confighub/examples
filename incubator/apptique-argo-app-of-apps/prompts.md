# Prompts

## Fresh AI Prompt

```text
You are in the apptique-argo-app-of-apps example in confighub/examples.

Goal: verify the Argo app-of-apps pattern safely and explain what it proves.

Requirements:
- Start read-only.
- Use ./setup.sh --explain and ./setup.sh --explain-json first.
- Be explicit about what mutates live infrastructure.
- Do not claim this example mutates ConfigHub by itself.
- Use the dedicated kubeconfig under var/ for cluster checks.
- If cub-scout is installed, use it for ownership and provenance.

Flow:
1. inspect the example structure and explain the root app plus child app layout
2. run the read-only preview commands
3. run the setup path
4. verify the root Application, child Applications, namespaces, deployment, and service
5. if cub-scout is available, trace the deployment back through the child app to the root app
6. summarize what the example proves and what it does not prove

At the end, clearly separate:
- proven cluster facts
- proven Argo facts
- optional cub-scout ownership facts
- any remaining uncertainty
```
