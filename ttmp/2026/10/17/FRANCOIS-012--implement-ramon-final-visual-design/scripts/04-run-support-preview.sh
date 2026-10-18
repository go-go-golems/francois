#!/usr/bin/env bash
# Isolated local support preview. Stripe credential must be sandbox-mode.
set -euo pipefail
cd /home/manuel/code/wesen/go-go-golems/francois
fixture=/tmp/francois-support-private
if ! psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -Atc "SELECT 1 FROM pg_database WHERE datname='francois_support_preview'" | rg -q 1; then
 psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -c 'CREATE DATABASE francois_support_preview'
fi
printf '%s' 'postgres://emailtest:emailtest@127.0.0.1:5437/francois_support_preview?sslmode=disable' > "$fixture/database-url"
chmod 600 "$fixture/database-url"
exec go run ./cmd/francois catalog serve --migrate \
 --listen 127.0.0.1:18796 --public-url http://localhost:18796 --app-domain apps.localhost \
 --database-url-file "$fixture/database-url" --release-dir "$fixture/releases" \
 --font-dir /home/manuel/.config/francois/fonts --backend fake \
 --stripe-secret-key-file "$fixture/stripe-key" --stripe-webhook-secret-files "$fixture/stripe-webhook"
