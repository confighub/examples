#!/usr/bin/env bash
# Check both onboarding plugins against every GitOps example in this
# repository, offline: no account, no cluster, no network.
#
#   scripts/verify-gitops-plugins.sh
#
# What it checks, in order:
#   1. Each plugin builds, vets, is gofmt-clean, and its own tests pass.
#   2. `plan` reads every example under gitops/, and either produces a plan or
#      says in its own words what it cannot read. Producing nothing quietly is
#      the failure this catches: it is how five gaps hid until the plugins were
#      pointed at examples they were not written against.
#   3. `apply` writes scripts that are valid bash, and every path they would
#      render exists and holds a kustomization.
#   4. Every render those scripts perform actually succeeds, twice, with the
#      same bytes. A rendering that cannot be repeated cannot be compared with
#      what Git produces later, which is what the handover turns on.
set -euo pipefail

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$root"
fail=0
note() { printf '\n== %s\n' "$*"; }
ok() { printf '   ok    %s\n' "$*"; }
bad() { printf '   FAIL  %s\n' "$*"; fail=1; }

command -v go >/dev/null || { echo "go is needed"; exit 1; }
command -v kustomize >/dev/null || { echo "kustomize is needed"; exit 1; }

note "1. Each plugin builds and its own tests pass"
for p in cub-argo cub-flux; do
  ( cd "$p" && go build ./... && go vet ./... && [ -z "$(gofmt -l .)" ] && go test ./... >/dev/null ) \
    && ok "$p" || bad "$p: build, vet, gofmt or tests"
done

bin="$(mktemp -d)"
trap 'rm -rf "$bin"' EXIT
( cd cub-argo && go build -o "$bin/cub-argo" . )
( cd cub-flux && go build -o "$bin/cub-flux" . )

note "2. plan reads every example, or says what it cannot read"
for d in gitops/argo/*/; do
  out=$("$bin/cub-argo" plan "$d" 2>&1) || true
  # A plan with no components is only acceptable when it says why.
  if grep -q '0 components' <<<"$out" && ! grep -qE 'Problems to fix first|Left out' <<<"$out"; then
    bad "$(basename "$d"): no components and no reason given"
  else
    why=""
    grep -q 'Problems to fix first' <<<"$out" && why=" (reports problems)"
    ok "argo $(basename "$d"): $(head -1 <<<"$out")$why"
  fi
done
for d in gitops/flux/*/; do
  out=$("$bin/cub-flux" plan "$d" 2>&1) || true
  if grep -q '0 layers' <<<"$out" && ! grep -qE 'Problems to fix first|Left out' <<<"$out"; then
    bad "$(basename "$d"): no layers and no reason given"
  else
    why=""
    grep -q 'Problems to fix first' <<<"$out" && why=" (reports problems)"
    ok "flux $(basename "$d"): $(head -1 <<<"$out")$why"
  fi
done

note "3. apply writes valid scripts, and what they read exists"
work="$(mktemp -d)"
trap 'rm -rf "$bin" "$work"' EXIT
# apply refuses a plan with problems, which is correct. Report that rather
# than letting set -e end the run with no word about why.
if ! "$bin/cub-argo" apply gitops/argo/expert-app-of-apps --stage-label rollout-phase \
  --stages canary,secondary,primary --out "$work/argo" >"$work/argo.log" 2>&1; then
  bad "cub-argo apply refused the expert example: $(grep -A2 'Problems to fix first' "$work/argo.log" | tail -1)"
fi
if ! "$bin/cub-flux" apply gitops/flux/expert-fleet --out "$work/flux" >"$work/flux.log" 2>&1; then
  bad "cub-flux apply refused the expert example: $(grep -A2 'Problems to fix first' "$work/flux.log" | tail -1)"
fi
shopt -s nullglob
scripts=("$work"/*/apply.sh "$work"/*/handover.sh)
[ ${#scripts[@]} -gt 0 ] || bad "apply wrote no scripts to check"
for s in "${scripts[@]}"; do
  bash -n "$s" && ok "$(basename "$(dirname "$s")")/$(basename "$s") parses" \
    || bad "$(basename "$s") is not valid bash"
done
for s in "$work"/*/apply.sh; do
  while read -r p; do
    [ -f "$root/$p/kustomization.yaml" ] || bad "$p has no kustomization.yaml but would be rendered"
  done < <(grep -oE "^render '[^']+'" "$s" | sed "s/render //;s/'//g")
done
ok "every rendered path holds a kustomization"

note "4. every render succeeds, and twice over gives the same bytes"
n=0
for s in "$work"/*/apply.sh; do
  while read -r p; do
    a="$work/a.yaml"; b="$work/b.yaml"
    if ! kustomize build --enable-helm "$root/$p" >"$a" 2>/dev/null; then
      bad "$p does not render"; continue
    fi
    kustomize build --enable-helm "$root/$p" >"$b" 2>/dev/null
    if cmp -s "$a" "$b"; then n=$((n+1)); else bad "$p renders differently twice over"; fi
  done < <(grep -oE "^render '[^']+'" "$s" | sed "s/render //;s/'//g")
done
ok "$n renders, each repeatable, with $(kustomize version)"

printf '\n'
if [ "$fail" -eq 0 ]; then
  echo "All checks passed. Nothing here touched an account, a cluster or the network."
else
  echo "Some checks failed; see FAIL above." >&2
fi
exit "$fail"
