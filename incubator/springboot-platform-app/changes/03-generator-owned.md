# 03. Generator-Owned

Request:

"Change `spring.datasource.*` or bypass the managed datasource boundary."

Why this must be blocked or escalated:

- datasource connectivity is platform-owned
- the field is not safe for app-local divergence
- direct mutation would bypass the runtime policy contract

Relevant files:

- upstream platform policy: [`../upstream/platform/runtime-policy.yaml`](../upstream/platform/runtime-policy.yaml)
- operational config: [`../operational/configmap.yaml`](../operational/configmap.yaml)
- route rule: [`../operational/field-routes.yaml`](../operational/field-routes.yaml)
- boundary bundle: [`../block-escalate.sh`](../block-escalate.sh)

What the route means:

- the app team should not mutate this field directly
- ConfigHub should block or escalate the request
- a platform engineer may later approve or make the change through the platform
  path
- in this example, `./block-escalate.sh --render-attempt` shows the exact
  dry-run datasource override that should eventually be blocked or escalated
