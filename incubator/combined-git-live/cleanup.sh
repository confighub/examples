#!/usr/bin/env bash
set -euo pipefail

kubectl delete namespace payment-dev payment-prod --ignore-not-found
