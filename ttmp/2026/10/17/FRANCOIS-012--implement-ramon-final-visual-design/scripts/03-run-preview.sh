#!/usr/bin/env bash
set -euo pipefail
# Run from the repository root, in tmux. Credentials stay in a private file.
exec go run ./cmd/francois catalog serve \
  --listen 127.0.0.1:18792 --public-url http://localhost:18792 \
  --app-domain localhost --database-url-file "${1:?database URL file required}" \
  --release-dir ./var/storefront/releases \
  --font-dir "${2:?private licensed font directory required}" \
  --backend fake --oidc-issuer http://localhost:18081 \
  --oidc-client-id final-preview
