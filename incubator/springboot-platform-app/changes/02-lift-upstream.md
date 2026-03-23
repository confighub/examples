# 02. Lift Upstream

Request:

"This service now needs Redis-backed caching."

Why this should be lifted upstream:

- the app code must gain a Redis dependency
- the Spring app inputs must grow cache configuration
- the platform-rendered operational shape changes as a consequence

Relevant files:

- upstream build input: [`../upstream/app/pom.xml`](../upstream/app/pom.xml)
- upstream app config: [`../upstream/app/src/main/resources/application.yaml`](../upstream/app/src/main/resources/application.yaml)
- operational deployment shape: [`../operational/deployment.yaml`](../operational/deployment.yaml)
- route rule: [`../operational/field-routes.yaml`](../operational/field-routes.yaml)
- read-only Redis bundle: [`../lift-upstream.sh`](../lift-upstream.sh)
- refreshed ConfigHub YAMLs: [`../lift-upstream/redis-cache/confighub`](../lift-upstream/redis-cache/confighub/)

What the route means:

- ConfigHub can capture the intent
- the durable change should be turned into an upstream app change
- the platform-rendered operational shape should then be refreshed from that
  new upstream state
- in this example, `./lift-upstream.sh --render-diff` shows the exact patch
  bundle for that upstream change
