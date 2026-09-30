#!/usr/bin/env bash
# The whole "already running Argo CD" journey on kind, with cub argo, for the
# estates nothing generates: a live export -> plan -> apply.sh -> handover.sh
# -> status -> the way back -> cleanup.sh, with every UID compared at each
# move. It stops at the first thing that is not as it should be.
#
#   CONFIGHUB_OCI=<gateway host:port as the kind cluster reaches it> \
#     [CONFIGHUB_OCI_PLAIN_HTTP=1] bash cub-argo/e2e/run.sh [app-of-apps|by-hand]
#
#   app-of-apps  gitops/argo/beginner-app-of-apps: a root whose children are
#                plain Applications syncing plain directories
#   by-hand      gitops/argo/intermediate-ci-to-gitops: Applications applied by
#                hand, no parent. Its image is built by CI and does not pull, so
#                Argo holds it at Progressing, and status has to say so.
#
# Needs kind, kubectl, kustomize, go, and cub logged in to the organization to
# use. It makes its own kind cluster with its own kubeconfig, so kubectl's
# current context is never touched, and deletes it at the end unless
# E2E_KEEP=1. It creates Spaces prefixed e2e- and cleanup.sh removes them.
# The plugin under test is built from this checkout and reached as `cub argo`
# through a shim, so an installed plugin is not replaced.
set -uo pipefail
R=$(cd "$(dirname "$0")/../.." && pwd)
SHAPE=${1:-app-of-apps}
case $SHAPE in
  app-of-apps) EXAMPLE=gitops/argo/beginner-app-of-apps; BOOT=$EXAMPLE/root/root-app.yaml; HEALTHY=yes ;;
  by-hand) EXAMPLE=gitops/argo/intermediate-ci-to-gitops; BOOT=$EXAMPLE/gitops-repo/applications; HEALTHY=no ;;
  *) echo "usage: run.sh [app-of-apps|by-hand]"; exit 2 ;;
esac
NAME=${E2E_CLUSTER:-cub-argo-e2e}
PREFIX=${E2E_PREFIX:-e2e}
ARGO_VERSION=${ARGO_VERSION:-v3.5.3}
W=${E2E_DIR:-${TMPDIR:-/tmp}/cub-argo-e2e-$SHAPE}; W=${W%/}
say() { printf '\n##### %s  [%s]\n' "$*" "$(date -u +%H:%M:%S)"; }
fail() { say "FAILED: $*"; exit 1; }
: "${CONFIGHUB_OCI:?set CONFIGHUB_OCI to the gateway host:port the kind cluster reaches}"
export CONFIGHUB_OCI
for t in kind kubectl kustomize go cub; do command -v $t >/dev/null || fail "$t is not on PATH"; done
cub auth status >/dev/null 2>&1 || fail "cub is not logged in"

left=$(cub space list --where "Slug LIKE '$PREFIX-%'" -o json 2>/dev/null | grep -c '"Slug"' || true)
[ "${left:-0}" = 0 ] || fail "$left Spaces prefixed $PREFIX- exist already, from an earlier run: remove them, or set E2E_PREFIX"
rm -rf "$W"; mkdir -p "$W/bin"
export KUBECONFIG="$W/kubeconfig"
CTX=kind-$NAME
k() { kubectl --context "$CTX" "$@"; }
(cd "$R/cub-argo" && go build -o "$W/bin/cub-argo" .) || fail "building the plugin"
real_cub=$(command -v cub)
cat > "$W/bin/cub" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = argo ]; then shift; exec "$W/bin/cub-argo" "\$@"; fi
exec "$real_cub" "\$@"
EOF
chmod +x "$W/bin/cub"
export PATH="$W/bin:$PATH"

# Every Application, and every object in the apps' namespaces, by UID.
uids() {
  { k -n argocd get applications -o jsonpath='{range .items[*]}Application/{.metadata.name} {.metadata.uid}{"\n"}{end}'
    for ns in apptique-dev apptique-prod; do
      k -n $ns get deploy,svc,sa,cm,pods -o jsonpath='{range .items[*]}{.kind}/{.metadata.namespace}/{.metadata.name} {.metadata.uid}{"\n"}{end}' 2>/dev/null
    done; } | sort
}
apps() { k -n argocd get applications -o jsonpath='{range .items[*]}{.metadata.name}={.spec.source.repoURL}|{.status.sync.status}/{.status.health.status}@{.status.sync.revision}{"\n"}{end}'; }
# wait_all <pattern>: until every Application's line matches.
wait_all() {
  # Read once, then match without a pipe: under pipefail, grep -q leaving
  # early kills kubectl with SIGPIPE, and the failed pipeline reads as a match.
  local i out
  for i in $(seq 1 72); do
    out=$(apps)
    if [ -n "$out" ] && ! grep -v -E "$1" <<< "$out" >/dev/null; then return 0; fi
    sleep 5
  done
  printf '%s\n' "$out"; return 1
}
refresh_all() { for a in $(k -n argocd get applications -o jsonpath='{.items[*].metadata.name}'); do k -n argocd annotate application "$a" argocd.argoproj.io/refresh=hard --overwrite >/dev/null; done; }
healthy='Synced/Healthy'; [ "$HEALTHY" = yes ] || healthy='Synced/(Healthy|Progressing|Degraded)'

say "1. kind cluster $NAME, Argo CD $ARGO_VERSION, its own kubeconfig"
kind get clusters 2>/dev/null | grep -qx "$NAME" && fail "kind cluster $NAME exists: delete it, or set E2E_CLUSTER"
kind create cluster --name "$NAME" --kubeconfig "$KUBECONFIG" >/dev/null 2>&1 || fail "kind create cluster"
# At the end, or at any stop: the cluster goes (unless E2E_KEEP), and after a
# stop, so do the Spaces this run made. With the cluster gone nothing reads
# them, so cleanup.sh may say so.
passed=no
finish() {
  [ -n "${E2E_KEEP:-}" ] && return
  kind delete cluster --name "$NAME" --kubeconfig "$KUBECONFIG" >/dev/null 2>&1
  if [ "$passed" != yes ] && [ -f "$W/out/cleanup.sh" ]; then
    (cd "$W/out" && I_HAVE_PUT_THE_SOURCES_BACK=yes bash cleanup.sh </dev/null > "$W/cleanup-after-stop.log" 2>&1)
    echo "  the Spaces this run made were removed; see $W/cleanup-after-stop.log"
  fi
}
trap finish EXIT
k create namespace argocd >/dev/null
k apply -n argocd --server-side -f "https://raw.githubusercontent.com/argoproj/argo-cd/$ARGO_VERSION/manifests/install.yaml" >/dev/null || fail "installing Argo CD"
for d in deploy/argocd-repo-server deploy/argocd-applicationset-controller statefulset/argocd-application-controller; do
  k -n argocd rollout status $d --timeout=300s >/dev/null || fail "$d did not come up"
done

say "2. The estate, $EXAMPLE, syncing from GitHub"
k apply -f "$R/$BOOT" >/dev/null || fail "applying $BOOT"
wait_all "=https://github.com/confighub/examples.git[|]$healthy@" || fail "the estate did not sync from Git"
apps

say "3. Plan from a live export, then apply.sh"
{ k get applications,applicationsets,appprojects -n argocd -o yaml; echo '---'
  k get secrets -n argocd -l argocd.argoproj.io/secret-type=cluster -o yaml; } > "$W/export.yaml"
cub argo plan "$W/export.yaml" --prefix "$PREFIX" --repo-root "$R" > "$W/plan.txt" || { cat "$W/plan.txt"; fail "plan"; }
head -1 "$W/plan.txt"
cub argo apply "$W/export.yaml" --prefix "$PREFIX" --repo-root "$R" --out "$W/out" >/dev/null || fail "apply"
(cd "$W/out" && bash apply.sh > "$W/apply.log" 2>&1) || { tail -20 "$W/apply.log"; fail "apply.sh"; }
uids > "$W/uids-before.txt"
echo "  $(wc -l < "$W/uids-before.txt" | tr -d ' ') UIDs recorded"

say "4. handover.sh"
(cd "$W/out" && ARGOCD_CONTEXT=$CTX bash handover.sh > "$W/handover.log" 2>&1) || { tail -30 "$W/handover.log"; fail "handover.sh"; }
grep -E "objects match|every field|reads .* at sha256" "$W/handover.log"
# A child its parent syncs is changed where handover.sh says: in its Unit.
if [ -d "$W/out/handover-state/repointed" ]; then
  for f in "$W/out"/handover-state/repointed/*/*; do
    space=$(basename "$(dirname "$f")"); unit=$(basename "$f" .yaml)
    cub unit update --space "$space" "$unit" "$f" --change-desc "e2e: point $unit at ConfigHub" --quiet || fail "updating $space/$unit"
  done
  for d in "$W/out"/handover-state/repointed/*/; do
    out=$(cub release publish "$(basename "$d")" --quiet 2>&1) || case "$out" in
      *"no changes were made since :latest bundle"*) ;;  # published already
      *) echo "$out"; fail "publishing $(basename "$d")" ;;
    esac
  done
  refresh_all
fi
wait_all "=oci://[^|]*[|]$healthy@sha256:" || fail "not every Application reads ConfigHub"
apps
uids > "$W/uids-after.txt"
diff "$W/uids-before.txt" "$W/uids-after.txt" || fail "a UID changed in the handover"
echo "  EVERY UID UNCHANGED ($(wc -l < "$W/uids-after.txt" | tr -d ' '))"

say "5. cub argo status"
cub argo status "$W/export.yaml" --prefix "$PREFIX" --repo-root "$R" --kube-context "$CTX" | tee "$W/status.log" || fail "status"
n=$(k -n argocd get applications -o name | wc -l | tr -d ' ')
if [ "$HEALTHY" = yes ]; then
  [ "$(grep -c 'written; the Healthy gate would pass' "$W/status.log")" = "$n" ] || fail "every Application should be reported, and pass the gate"
else
  [ "$(grep -c 'Progressing.*(written; the Healthy gate would not pass)' "$W/status.log")" = "$n" ] || fail "Progressing Applications must be reported, and not pass the gate"
fi

say "6. cleanup.sh refuses while anything reads ConfigHub"
(cd "$W/out" && ARGOCD_CONTEXT=$CTX bash cleanup.sh </dev/null > "$W/cleanup-refused.log" 2>&1) && fail "cleanup.sh ran while Applications read ConfigHub"
grep "still read a Space" "$W/cleanup-refused.log" || fail "cleanup.sh did not say why it refused"

say "7. The way back handover.sh printed, leaves first"
sed -n '/The way back, leaves first:/,/^One more thing/p' "$W/handover.log" \
  | sed -n -E 's/^ +((cub|kubectl) .*)$/\1/p' > "$W/way-back.sh"
cat "$W/way-back.sh"
(cd "$W/out" && bash "$W/way-back.sh" >/dev/null) || fail "the way back"
refresh_all
wait_all "=https://github.com/confighub/examples.git[|]$healthy@" || fail "not every Application is back on Git"
uids > "$W/uids-back.txt"
diff "$W/uids-before.txt" "$W/uids-back.txt" || fail "a UID changed on the way back"
echo "  EVERY UID UNCHANGED AFTER THE WAY BACK"

say "8. cleanup.sh"
(cd "$W/out" && ARGOCD_CONTEXT=$CTX bash cleanup.sh </dev/null > "$W/cleanup.log" 2>&1) || { cat "$W/cleanup.log"; fail "cleanup.sh"; }
grep -q "every Space apply.sh made is gone" "$W/cleanup.log" || fail "Spaces remain"
[ -z "$(k -n argocd get secrets -l argocd.argoproj.io/secret-type=repo-creds -o name)" ] || fail "the gateway credential was left behind"
echo "  every Space and the credential are gone"

passed=yes
say "PASSED: $SHAPE"
