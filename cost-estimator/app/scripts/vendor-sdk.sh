#!/usr/bin/env bash
# Regenerate the ConfigHub API client types from the public OpenAPI spec.
#
# fleet-ql is a portable, Redux-free engine: it talks to ConfigHub through a thin
# fetch transport (src/api/fqlTransport.ts) built on `openapi-fetch`, not the
# SDK's RTK Query runtime client. So instead of vendoring the RTK client, we
# generate a TYPES-ONLY `paths` map straight from the OpenAPI document with
# `openapi-typescript`. openapi-fetch then gives us a fully-typed `fetch` client
# with zero React/Redux dependencies.
#
# Usage:
#   npm run vendor-sdk            # ref: main
#   scripts/vendor-sdk.sh <ref>   # a specific git ref
set -euo pipefail

REF="${1:-main}"
URL="https://raw.githubusercontent.com/confighub/sdk/${REF}/core/openapi/openapi.json"
DEST="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT="${DEST}/src/sdk/confighub.openapi.ts"

tmp="$(mktemp -d)"
trap 'rm -rf "${tmp}"' EXIT

echo "Fetching openapi.json @ ${REF}"
curl -fsSL "${URL}" -o "${tmp}/openapi.json"

echo "Generating ${OUT}"
npx --yes openapi-typescript@7 "${tmp}/openapi.json" \
  --output "${tmp}/schema.ts" \
  --root-types

{
  cat <<EOF
// ConfigHub API — generated OpenAPI types (the \`paths\` map openapi-fetch is
// typed against). Generated from the public OpenAPI spec; see VENDORED_FROM.md.
//
// Do not edit by hand. Refresh with \`npm run vendor-sdk\` (scripts/vendor-sdk.sh).

EOF
  cat "${tmp}/schema.ts"
} > "${OUT}"

sed -i.bak "s/^- Generated: .*/- Generated: $(date +%Y-%m-%d) (ref: ${REF})/" "${DEST}/src/sdk/VENDORED_FROM.md" \
  && rm -f "${DEST}/src/sdk/VENDORED_FROM.md.bak"

echo "Wrote ${OUT} ($(wc -l < "${OUT}") lines)."
