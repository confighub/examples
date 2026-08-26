#!/usr/bin/env bash
# Bump every @confighub/{api,react-auth,rtk-query} requirement in the repo to one
# js-sdk release. The three packages are published together from the same tag, so
# a single version applies to all of them; moving them together avoids the
# mixed-version state a per-package bump would leave behind.
#
# Usage: scripts/update-js-sdk.sh [version]   # default: latest @confighub/api on npm
#
# Env:
#   SUMMARY_FILE  write a markdown summary here (for the PR body)
#   SKIP_VERIFY=1 skip the per-package lint/build/test verification
#
# A package that fails to verify after the bump is restored to its previous
# package.json/package-lock.json and reported as skipped, so one stale console does
# not hold back the rest.

set -euo pipefail

script_dir="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
repo_root="$(cd "${script_dir}/.." && pwd)"
cd "${repo_root}"

version="${1:-}"
if [[ -z "${version}" ]]; then
  version="$(npm view @confighub/api version 2>/dev/null || true)"
fi
if [[ ! "${version}" =~ ^[0-9] ]]; then
  echo "Could not determine a js-sdk version (got: '${version}')" >&2
  exit 1
fi
echo "==> Target js-sdk version: ${version}"

# webkit is a `file:` dependency of the consoles, and its sources are typechecked by
# their builds, so it has to be on the new version before any of them is verified.
# bash 3.2 (macOS) has no mapfile, and this has to run on a laptop as well as CI.
manifests=()
while IFS= read -r m; do
  manifests+=("$m")
done < <(
  find . -name package.json -not -path '*/node_modules/*' -not -path '*/.vite/*' | sort |
    while read -r f; do
      grep -q '"@confighub/\(api\|react-auth\|rtk-query\)"' "$f" && echo "$f"
    done
)
ordered=()
for m in "${manifests[@]}"; do case "$m" in ./webkit/*) ordered+=("$m");; esac; done
for m in "${manifests[@]}"; do case "$m" in ./webkit/*) ;; *) ordered+=("$m");; esac; done

updated=(); skipped=(); unchanged=()

for manifest in "${ordered[@]}"; do
  dir="$(dirname "${manifest}")"
  name="${dir#./}"
  echo "==> ${name}"

  cp "${manifest}" "${manifest}.orig"
  # Some packages deliberately keep no lockfile; note that so `npm install` does not
  # leave one behind as an untracked file.
  had_lock=1
  if [[ -f "${dir}/package-lock.json" ]]; then
    cp "${dir}/package-lock.json" "${dir}/package-lock.json.orig"
  else
    had_lock=0
  fi

  # Rewrite every dependency block that names one of the three packages. peerDependencies
  # matter too: webkit declares the SDK there, and a stale peer range fails installs.
  VERSION="${version}" node -e '
    const fs = require("fs");
    const file = process.argv[1];
    const pkg = JSON.parse(fs.readFileSync(file, "utf8"));
    const names = ["@confighub/api", "@confighub/react-auth", "@confighub/rtk-query"];
    for (const block of ["dependencies", "devDependencies", "peerDependencies"]) {
      const deps = pkg[block];
      if (!deps) continue;
      for (const n of names) if (deps[n]) deps[n] = "^" + process.env.VERSION;
    }
    fs.writeFileSync(file, JSON.stringify(pkg, null, 2) + "\n");
  ' "${manifest}"

  failure=""
  ( cd "${dir}" && npm install --no-audit --no-fund ) >/dev/null 2>&1 || failure="npm install"

  if [[ -z "${failure}" && "${SKIP_VERIFY:-}" != "1" ]]; then
    for step in lint build test; do
      node -e 'process.exit(JSON.parse(require("fs").readFileSync(process.argv[1],"utf8")).scripts?.[process.argv[2]] ? 0 : 1)' \
        "${manifest}" "${step}" || continue
      ( cd "${dir}" && npm run "${step}" ) >/dev/null 2>&1 || { failure="npm run ${step}"; break; }
    done
  fi

  if [[ -n "${failure}" ]]; then
    echo "    skipped: ${failure} failed"
    mv "${manifest}.orig" "${manifest}"
    if [[ -f "${dir}/package-lock.json.orig" ]]; then
      mv "${dir}/package-lock.json.orig" "${dir}/package-lock.json"
    elif [[ "${had_lock}" -eq 0 ]]; then
      rm -f "${dir}/package-lock.json"
    fi
    ( cd "${dir}" && npm install --no-audit --no-fund ) >/dev/null 2>&1 || true
    skipped+=("${name} (${failure} failed)")
    continue
  fi

  if cmp -s "${manifest}" "${manifest}.orig"; then
    unchanged+=("${name}")
  else
    updated+=("${name}")
  fi
  if [[ "${had_lock}" -eq 0 ]]; then
    rm -f "${dir}/package-lock.json"
  fi
  rm -f "${manifest}.orig" "${dir}/package-lock.json.orig"
done

summary_lines=()
summary_lines+=("Bumped \`@confighub/api\`, \`@confighub/react-auth\`, and \`@confighub/rtk-query\` to \`${version}\`.")
summary_lines+=("")
if [[ ${#updated[@]} -gt 0 ]]; then
  summary_lines+=("Updated (${#updated[@]}):")
  for m in "${updated[@]}"; do summary_lines+=("- \`${m}\`"); done
  summary_lines+=("")
fi
if [[ ${#skipped[@]} -gt 0 ]]; then
  summary_lines+=("Left alone — the bump did not verify, so these still need a code change (${#skipped[@]}):")
  for m in "${skipped[@]}"; do summary_lines+=("- ${m}"); done
  summary_lines+=("")
fi
if [[ ${#unchanged[@]} -gt 0 ]]; then
  summary_lines+=("Already current (${#unchanged[@]}): ${unchanged[*]}")
fi

printf '%s\n' "" "${summary_lines[@]}"
if [[ -n "${SUMMARY_FILE:-}" ]]; then
  printf '%s\n' "${summary_lines[@]}" > "${SUMMARY_FILE}"
fi
