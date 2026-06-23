#!/usr/bin/env bash
#
# Nothing to clean up: the default mapper path is READ-ONLY and creates no
# ConfigHub or live state. If you later run the generated `cub` commands by
# hand to create real spaces/units, delete them with:
#
#   cub space delete <app>-staging
#   cub space delete <app>-production
#
# (Review with `cub space list` first.)
set -euo pipefail
echo "ctrlplane-on-confighub default path is read-only; nothing to clean up."
echo "If you created spaces by hand from --cub-commands, delete them with 'cub space delete <slug>'."
