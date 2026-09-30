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
#   3. `apply` writes scripts that are valid bash, for the expert examples and
#      for the beginner ones (plain directories, Argo CD's own cluster), and
#      every path they would render exists.
#   4. Every render those scripts perform actually succeeds, twice, with the
#      same bytes, rendered by the script's own build function. A rendering
#      that cannot be repeated cannot be compared with what Git produces
#      later, which is what the handover turns on.
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
applies() {
  local plugin=$1 out=$2; shift 2
  if ! "$bin/cub-$plugin" apply "$@" --out "$work/$out" >"$work/$out.log" 2>&1; then
    bad "cub-$plugin apply refused $1: $(grep -A2 'Problems to fix first' "$work/$out.log" | tail -1)"
  fi
}
applies argo argo gitops/argo/expert-app-of-apps --stage-label rollout-phase --stages canary,secondary,primary
applies argo argo-beginner-app-of-apps gitops/argo/beginner-app-of-apps
applies argo argo-beginner-applicationset gitops/argo/beginner-applicationset
applies flux flux gitops/flux/expert-fleet
shopt -s nullglob
scripts=("$work"/*/apply.sh "$work"/*/handover.sh "$work"/*/cleanup.sh "$work"/*/join.sh)
# cub-argo writes three scripts per estate and cub-flux four (join.sh too).
# cub-flux once wrote no cleanup.sh at all, which is only visible if the count
# is checked rather than the parse.
[ ${#scripts[@]} -eq 13 ] || bad "expected 3 scripts for each of 3 Argo estates and 4 for Flux, found ${#scripts[@]}"
for s in "${scripts[@]}"; do
  bash -n "$s" && ok "$(basename "$(dirname "$s")")/$(basename "$s") parses" \
    || bad "$(basename "$s") is not valid bash"
done
for s in "$work"/*/apply.sh; do
  while read -r p; do
    [ -d "$root/$p" ] || bad "$p is not a directory but would be rendered"
  done < <(grep -oE "^render '[^']+'" "$s" | sed "s/render //;s/'//g")
done
ok "every rendered path exists"

note "4. every render succeeds, and twice over gives the same bytes"
n=0
for s in "$work"/*/apply.sh; do
  # Render as the script does: with its own build function where it has one.
  # A plain directory is listed in a kustomization of its own there, which a
  # bare kustomize build of the path would refuse.
  fn="$work/build.sh"
  if grep -q '^build() {' "$s"; then
    awk '/^build\(\) \{/,/^\}/' "$s" > "$fn"
  else
    printf 'build() { kustomize build ${KUSTOMIZE_FLAGS:-} "$1"; }\n' > "$fn"
  fi
  while read -r p mode; do
    a="$work/a.yaml"; b="$work/b.yaml"
    if ! KUSTOMIZE_FLAGS=--enable-helm bash -c "set -euo pipefail; source '$fn'; build '$root/$p' '$mode'" >"$a" 2>/dev/null || [ ! -s "$a" ]; then
      bad "$p does not render"; continue
    fi
    KUSTOMIZE_FLAGS=--enable-helm bash -c "set -euo pipefail; source '$fn'; build '$root/$p' '$mode'" >"$b" 2>/dev/null
    if cmp -s "$a" "$b"; then n=$((n+1)); else bad "$p renders differently twice over"; fi
  done < <(grep -E "^render '[^']+'" "$s" | sed -E "s/^render '([^']+)' '[^']+'( (recurse))?.*/\1 \3/")
done
ok "$n renders, each repeatable, with $(kustomize version)"

printf '\n'
if [ "$fail" -eq 0 ]; then
  echo "All checks passed. Nothing here touched an account, a cluster or the network."
else
  echo "Some checks failed; see FAIL above." >&2
fi
exit "$fail"
