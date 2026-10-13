#!/usr/bin/env bash
# Browser end-to-end test of https://francois.yolo.scapegoat.dev with the
# E2E accounts in the Francois ZITADEL organization. Credentials are read from
# Vault (kv/apps/francois/prod/e2e-users) into this process only.
# Usage: 05-yolo-e2e.sh <step>   (steps: admin, admin-models, alice-subscribe, alice-run, alice-sample, control, logout)
set -euo pipefail
here=$(cd "$(dirname "$0")" && pwd)
export VAULT_ADDR=${VAULT_ADDR:-https://vault.yolo.scapegoat.dev}
creds=$(vault kv get -format=json kv/apps/francois/prod/e2e-users | jq -c .data.data)
export E2E_CREDS="$creds"
export E2E_OUT=${E2E_OUT:-$here/../various/e2e}
mkdir -p "$E2E_OUT"
export NODE_PATH=${PLAYWRIGHT_NODE_PATH:-$(ls -d ~/.npm/_npx/*/node_modules | while read -r d; do [[ -f $d/playwright-core/package.json && $(jq -r .version "$d/playwright-core/package.json") == 1.61.1 ]] && echo "$d" && break; done)}
export CHROME=${CHROME:-$HOME/.cache/ms-playwright/chromium-1223/chrome-linux64/chrome}
exec node "$here/05-yolo-e2e.mjs" "$@"
