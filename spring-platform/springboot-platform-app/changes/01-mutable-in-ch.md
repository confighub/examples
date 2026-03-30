# 01. Mutable in ConfigHub

Request:

"Change `feature.inventory.reservationMode` in prod from `strict` to
`optimistic` for a rollout."

Why this is a direct ConfigHub mutation:

- the field is app-owned
- the change is per-deployment operational tuning
- the change should survive normal refreshes

Relevant files:

- upstream input: [`../upstream/app/src/main/resources/application-prod.yaml`](../upstream/app/src/main/resources/application-prod.yaml)
- operational config: [`../operational/configmap.yaml`](../operational/configmap.yaml)
- route rule: [`../operational/field-routes.yaml`](../operational/field-routes.yaml)

What the mutation means:

- ConfigHub stores the changed operational value for `inventory-api-prod`
- provenance still points back to the upstream Spring profile file
- future refreshes preserve the direct mutation unless policy says otherwise
