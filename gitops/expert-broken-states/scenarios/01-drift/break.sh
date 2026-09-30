#!/usr/bin/env bash
set -euo pipefail

# This script documents the drift scenario. It does not run anything: it
# only prints the one command a person would run, in their own terminal,
# against their own cluster, to cause the drift described in README.md.
#
# Nothing in this repository, and no AI assistant following this example's
# AI_START_HERE.md, runs this command on your behalf.

cat <<'EOF_STEPS'
Scenario 1: drift

This is documentation only. Nothing below has been run.

To see live drift, in YOUR OWN terminal, against a cluster you control,
with apptique-broken-states already deployed there by Argo CD or Flux:

  kubectl -n apptique-broken-states scale deployment/frontend --replicas=5

Then watch:
  - Argo CD: the Application goes OutOfSync, then (with selfHeal) reverts.
  - Flux: kustomize-controller applies the desired state again on its next
    reconcile interval, which sets replicas back to 2.

Only fields the manifest sets are corrected. A hand edit to a field it
does not set, such as an added annotation, is left alone.

See ./README.md for what each controller reports and what ConfigHub shows.
EOF_STEPS
