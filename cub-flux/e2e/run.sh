#!/usr/bin/env bash
# The whole "already running Flux" journey on kind, with cub flux, for one
# cluster of gitops/flux/beginner: its layers applied from GitHub, then plan ->
# apply.sh -> handover.sh -> status -> cleanup.sh refusing -> the way back
# handover.sh printed -> cleanup.sh, with every UID compared at each move. It
# stops at the first thing that is not as it should be.
#
#   CONFIGHUB_OCI=<gateway host:port as the kind cluster reaches it> \
#     [CONFIGHUB_OCI_PLAIN_HTTP=1] bash cub-flux/e2e/run.sh
#
# The layers are applied with kubectl, as `flux install` leaves a cluster, not
# by `flux bootstrap`: a bootstrapped fleet's handover pauses for a commit in
# the repository flux-system reads, which a rig reading GitHub cannot make.
# docs/runs/2026-09-30-bootstrapped-handover.md is that run.
#
# Needs kind, kubectl, flux, kustomize, go, and cub logged in to the
# organization to use. It makes its own kind cluster with its own kubeconfig,
# so kubectl's current context is never touched, and deletes it at the end
# unless E2E_KEEP=1. It creates Spaces prefixed e2f- and removes them. The
# plugin under test is built from this checkout and reached as `cub flux`
# through a shim, so an installed plugin is not replaced.
set -uo pipefail
R=$(cd "$(dirname "$0")/../.." && pwd)
FLEET=gitops/flux/beginner
CLUSTER=dev
NAME=${E2E_CLUSTER:-cub-flux-e2e}
PREFIX=${E2E_PREFIX:-e2f}
W=${E2E_DIR:-${TMPDIR:-/tmp}/cub-flux-e2e}; W=${W%/}
say() { printf '\n##### %s  [%s]\n' "$*" "$(date -u +%H:%M:%S)"; }
fail() { say "FAILED: $*"; exit 1; }
: "${CONFIGHUB_OCI:?set CONFIGHUB_OCI to the gateway host:port the kind cluster reaches}"
export CONFIGHUB_OCI
for t in kind kubectl flux kustomize go cub; do command -v $t >/dev/null || fail "$t is not on PATH"; done
cub auth status >/dev/null 2>&1 || fail "cub is not logged in"
left=$(cub space list --where "Slug LIKE '$PREFIX-%'" -o json 2>/dev/null | grep -c '"Slug"' || true)
[ "${left:-0}" = 0 ] || fail "$left Spaces prefixed $PREFIX- exist already, from an earlier run: remove them, or set E2E_PREFIX"

rm -rf "$W"; mkdir -p "$W/bin"
export KUBECONFIG="$W/kubeconfig"
CTX=kind-$NAME
k() { kubectl --context "$CTX" "$@"; }
(cd "$R/cub-flux" && go build -o "$W/bin/cub-flux" .) || fail "building the plugin"
real_cub=$(command -v cub)
cat > "$W/bin/cub" <<EOF
#!/usr/bin/env bash
if [ "\${1:-}" = flux ]; then shift; exec "$W/bin/cub-flux" "\$@"; fi
exec "$real_cub" "\$@"
EOF
chmod +x "$W/bin/cub"
export PATH="$W/bin:$PATH"

# Every Flux object and every object in the app's namespace, by UID.
uids() {
  { k -n flux-system get kustomizations,gitrepositories -o jsonpath='{range .items[*]}{.kind}/{.metadata.name} {.metadata.uid}{"\n"}{end}'
    k -n apptique-dev get deploy,svc,sa,cm,pods -o jsonpath='{range .items[*]}{.kind}/{.metadata.namespace}/{.metadata.name} {.metadata.uid}{"\n"}{end}' 2>/dev/null
  } | grep -v -E '^Kustomization/confighub-root ' | sort
}
layers() { k -n flux-system get kustomizations infrastructure apps -o jsonpath='{range .items[*]}{.metadata.name}={.spec.sourceRef.kind}|{.status.conditions[?(@.type=="Ready")].status}@{.status.lastAppliedRevision}{"\n"}{end}'; }
# wait_layers <pattern>: until both layers' lines match. Read once, then
# match without a pipe: under pipefail, grep -q leaving early fails kubectl.
wait_layers() {
  local i out
  for i in $(seq 1 72); do
    out=$(layers 2>/dev/null)
    if [ "$(grep -c -E "$1" <<< "$out")" = 2 ]; then return 0; fi
    sleep 5
  done
  printf '%s\n' "$out"
  k -n flux-system get gitrepositories,ocirepositories,kustomizations 2>&1 | head -20
  return 1
}

passed=no
finish() {
  [ -n "${E2E_KEEP:-}" ] && return
  kind delete cluster --name "$NAME" --kubeconfig "$KUBECONFIG" >/dev/null 2>&1
  if [ "$passed" != yes ] && [ -f "$W/out/cleanup.sh" ]; then
    (cd "$W/out" && I_HAVE_PUT_THE_SOURCES_BACK=yes bash cleanup.sh </dev/null > "$W/cleanup-after-stop.log" 2>&1)
    echo "  the Spaces this run made were removed; see $W/cleanup-after-stop.log"
  fi
}

say "1. kind cluster $NAME, Flux $(flux --version | awk '{print $3}'), its own kubeconfig"
kind get clusters 2>/dev/null | grep -qx "$NAME" && fail "kind cluster $NAME exists: delete it, or set E2E_CLUSTER"
kind create cluster --name "$NAME" --kubeconfig "$KUBECONFIG" >/dev/null 2>&1 || fail "kind create cluster"
trap finish EXIT
flux install --context "$CTX" >/dev/null 2>&1 || fail "flux install"

say "2. The fleet's $CLUSTER layers, reading GitHub"
k apply -f "$R/$FLEET/infrastructure/base/sources/apptique-examples.yaml" >/dev/null || fail "the GitRepository"
k apply -f "$R/$FLEET/clusters/$CLUSTER/infrastructure.yaml" -f "$R/$FLEET/clusters/$CLUSTER/apps.yaml" >/dev/null || fail "the layers"
wait_layers '=GitRepository[|]True@' || fail "the layers did not become Ready from Git"
layers

say "3. Plan, then apply.sh"
cub flux plan "$R/$FLEET" --prefix "$PREFIX" --repo-root "$R" > "$W/plan.txt" || { cat "$W/plan.txt"; fail "plan"; }
head -1 "$W/plan.txt"
cub flux apply "$R/$FLEET" --prefix "$PREFIX" --repo-root "$R" --out "$W/out" >/dev/null || fail "apply"
(cd "$W/out" && bash apply.sh > "$W/apply.log" 2>&1) || { tail -20 "$W/apply.log"; fail "apply.sh"; }
uids > "$W/uids-before.txt"
echo "  $(wc -l < "$W/uids-before.txt" | tr -d ' ') UIDs recorded"

say "4. handover.sh for $CLUSTER"
(cd "$W/out" && CLUSTER=$CLUSTER FLUX_CONTEXT=$CTX bash handover.sh > "$W/handover.log" 2>&1) || { tail -30 "$W/handover.log"; fail "handover.sh"; }
grep -E "objects match|every field|condition met" "$W/handover.log" | head -12
wait_layers '=OCIRepository[|]True@' || fail "the layers do not read ConfigHub"
layers
uids > "$W/uids-after.txt"
diff "$W/uids-before.txt" "$W/uids-after.txt" || fail "a UID changed in the handover"
echo "  EVERY UID UNCHANGED ($(wc -l < "$W/uids-after.txt" | tr -d ' '))"

say "5. cub flux status"
cub flux status "$R/$FLEET" --prefix "$PREFIX" --repo-root "$R" --cluster "$CLUSTER" --kube-context "$CTX" | tee "$W/status.log" || fail "status"
[ "$(grep -c 'written; the Healthy gate would pass' "$W/status.log")" = 2 ] || fail "both layers should be reported, and pass the gate"

say "6. cleanup.sh refuses while anything reads ConfigHub"
(cd "$W/out" && FLUX_CONTEXT=$CTX bash cleanup.sh </dev/null > "$W/cleanup-refused.log" 2>&1) && fail "cleanup.sh ran while layers read ConfigHub"
grep "still read a Space" "$W/cleanup-refused.log" || fail "cleanup.sh did not say why it refused"

say "7. The way back handover.sh printed, in its order"
sed -n '/To go back, in this order/,$p' "$W/handover.log" | sed -n -E 's/^ +(kubectl .*)$/\1/p' > "$W/way-back.sh"
cat "$W/way-back.sh"
[ -s "$W/way-back.sh" ] || fail "handover.sh printed no way back"
(cd "$W/out" && bash "$W/way-back.sh" >/dev/null) || fail "the way back"
wait_layers '=GitRepository[|]True@' || fail "the layers are not back on Git"
uids > "$W/uids-back.txt"
diff "$W/uids-before.txt" "$W/uids-back.txt" || fail "a UID changed on the way back"
echo "  EVERY UID UNCHANGED AFTER THE WAY BACK"

say "8. cleanup.sh"
(cd "$W/out" && FLUX_CONTEXT=$CTX bash cleanup.sh </dev/null > "$W/cleanup.log" 2>&1) || { cat "$W/cleanup.log"; fail "cleanup.sh"; }
grep -q "every Space apply.sh made is gone" "$W/cleanup.log" || fail "Spaces remain"
[ -z "$(k -n flux-system get secret "confighub-$PREFIX-targets" -o name 2>/dev/null)" ] || fail "the pull Secret was left behind"
echo "  every Space and the pull Secret are gone"

passed=yes
say "PASSED"
