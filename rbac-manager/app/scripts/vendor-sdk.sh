#!/usr/bin/env bash
# Refresh the vendored ConfigHub SDK client files from the public SDK repo.
#
# Usage:
#   scripts/vendor-sdk.sh [<git-ref>]    # default: main
set -euo pipefail

REF="${1:-main}"
BASE="https://raw.githubusercontent.com/confighub/sdk/${REF}/core/openapi/rtkqueryclient"
DEST="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)/src/sdk"

for f in confighubapi.gen.ts validation.gen.ts; do
  echo "Fetching ${f} @ ${REF}"
  curl -fsSL "${BASE}/${f}" -o "${DEST}/${f}"
done

sed -i.bak "s/^- Vendored: .*/- Vendored: $(date +%Y-%m-%d) (ref: ${REF})/" "${DEST}/VENDORED_FROM.md" \
  && rm -f "${DEST}/VENDORED_FROM.md.bak"

cat <<'EOF'
Done. Note: confighubapi.ts is a fork, not vendored — diff it against
upstream's rtkqueryclient/confighubapi.ts and port relevant changes manually.
EOF
