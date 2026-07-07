#!/usr/bin/env bash
# Seed a realistic multi-component dev/staging/prod fleet for FQL demos.
#
#   3 components (storefront, orders, payments) x 3 environments = 9 spaces,
#   each a Deployment + Service, bound to an env cluster, labeled with the
#   ConfigHub well-known fleet labels (Component/Environment/Region/Owner/Variant).
#
# Promotion topology is dev -> staging -> prod via per-unit upstream links
# (cub unit create --upstream-*). staging/prod carry env-specific replica
# overrides (local divergence preserved by upgrades). storefront-dev is bumped
# ahead of its downstreams to seed a "change awaiting promotion" gap.
#
# Idempotent (safe to re-run). ConfigHub only; the OCI targets use a server
# worker, so no real cluster is contacted. Tear down with fleet-teardown.sh.
set -euo pipefail
cub=cub

PREFIX=acme
# component | base image | bumped dev image (skew, optional) | owner team
COMPONENTS=(
  "storefront|nginx:1.26-alpine|nginx:1.27-alpine|web-team"
  "orders|python:3.12-slim||commerce-team"
  "payments|node:20-alpine||payments-team"
)
# env | Environment label | Region | Variant | replicas | cluster target
ENVS=(
  "dev|Dev|us-west-2|dev|1|dev-cluster"
  "staging|Staging|us-east-2|staging|2|staging-cluster"
  "prod|Prod|us-east-1|prod|3|prod-cluster"
)

tmp="$(mktemp -d)"; trap 'rm -rf "$tmp"' EXIT
note() { printf '  %s\n' "$*"; }

deployment_yaml() { # name image replicas component
  cat >"$tmp/$1-deploy.yaml" <<YAML
apiVersion: apps/v1
kind: Deployment
metadata:
  name: $1
  labels: { app: $1, component: $4 }
spec:
  replicas: $3
  selector: { matchLabels: { app: $1 } }
  template:
    metadata: { labels: { app: $1 } }
    spec:
      containers:
        - name: $1
          image: $2
          ports: [ { containerPort: 8080 } ]
YAML
  echo "$tmp/$1-deploy.yaml"
}
service_yaml() { # name
  cat >"$tmp/$1-svc.yaml" <<YAML
apiVersion: v1
kind: Service
metadata:
  name: $1
spec:
  selector: { app: $1 }
  ports: [ { port: 80, targetPort: 8080 } ]
YAML
  echo "$tmp/$1-svc.yaml"
}

unit_exists()  { $cub unit get  --space "$1" "$2" >/dev/null 2>&1; }

ensure_cluster() { # space cluster
  $cub worker create oci-worker --is-server-worker --space "$1" --allow-exists >/dev/null 2>&1 || true
  $cub target create "$2" '{}' oci-worker --space "$1" --provider OCI --toolchain Any \
    --allow-exists >/dev/null 2>&1 || true
}

for entry in "${COMPONENTS[@]}"; do
  IFS='|' read -r comp image bump owner <<<"$entry"
  dunit="$comp"; sunit="${comp}-svc"
  prev_space=""
  for env_entry in "${ENVS[@]}"; do
    IFS='|' read -r env envlabel region variant replicas cluster <<<"$env_entry"
    space="${PREFIX}-${comp}-${env}"
    note "Space ${space} (Component=${comp} Environment=${envlabel} Region=${region})"

    $cub space create "$space" --allow-exists \
      --label "Component=${comp}" --label "Environment=${envlabel}" \
      --label "Region=${region}" --label "Owner=${owner}" --label "Variant=${variant}" \
      >/dev/null 2>&1 || true
    ensure_cluster "$space" "$cluster"

    ul=(--label "Component=${comp}" --label "Environment=${envlabel}"
        --label "Region=${region}" --label "Owner=${owner}")

    if [[ -z "$prev_space" ]]; then
      # dev: source units from generated manifests
      if ! unit_exists "$space" "$dunit"; then
        $cub unit create --space "$space" "$dunit" "$(deployment_yaml "$comp" "$image" "$replicas" "$comp")" \
          "${ul[@]}" --change-desc "Seed ${comp} Deployment" >/dev/null
        note "  created Deployment ${dunit}"
      fi
      if ! unit_exists "$space" "$sunit"; then
        $cub unit create --space "$space" "$sunit" "$(service_yaml "$comp")" \
          "${ul[@]}" --change-desc "Seed ${comp} Service" >/dev/null
      fi
    else
      # staging/prod: clone from the previous env (upstream chain), then set
      # env-specific replicas as a local override.
      for u in "$dunit" "$sunit"; do
        unit_exists "$space" "$u" && continue
        $cub unit create --space "$space" "$u" --upstream-unit "$u" --upstream-space "$prev_space" \
          "${ul[@]}" --change-desc "Clone ${u} from ${prev_space}" >/dev/null
      done
      note "  cloned from ${prev_space}"
      $cub function do --space "$space" --unit "$dunit" --change-desc "Set ${env} replicas" \
        -- yq-i ".spec.replicas = ${replicas}" >/dev/null 2>&1 || true
    fi

    $cub unit set-target "$cluster" --space "$space" --unit "$dunit,$sunit" >/dev/null 2>&1 || true
    prev_space="$space"
  done

  # Seed a promotion gap: bump the dev image ahead of staging/prod.
  if [[ -n "$bump" ]]; then
    note "  bump ${comp}-dev image -> ${bump} (awaiting promotion)"
    $cub function do --space "${PREFIX}-${comp}-dev" --unit "$dunit" \
      --change-desc "Upgrade ${comp} image to ${bump}" \
      -- yq-i "(.spec.template.spec.containers[] | .image) = \"${bump}\"" >/dev/null
  fi
done

echo "Done. Fleet under ${PREFIX}-*. Try: npx vite-node scripts/live.ts"
