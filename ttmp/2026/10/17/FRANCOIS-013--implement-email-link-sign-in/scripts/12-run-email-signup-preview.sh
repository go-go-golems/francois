#!/usr/bin/env bash
# Fixed local preview backed by the isolated ticket stack; no production mode.
set -euo pipefail
cd /home/manuel/code/wesen/go-go-golems/francois
fixture_dir=/tmp/francois-013-private
task_org=$(python3 -c 'import json; print(json.load(open("/tmp/francois-013-private/fixture.json"))["organization"])')
if ! psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -Atc "SELECT 1 FROM pg_database WHERE datname='francois_email_signup_preview'" | rg -q 1; then
  psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -v ON_ERROR_STOP=1 -c 'CREATE DATABASE francois_email_signup_preview'
fi
printf '%s' 'postgres://emailtest:emailtest@127.0.0.1:5437/francois_email_signup_preview?sslmode=disable' > "$fixture_dir/signup-database-url"
chmod 600 "$fixture_dir/signup-database-url"
go run ./cmd/francois catalog migrate --database-url-file "$fixture_dir/signup-database-url" >/dev/null
exec go run ./cmd/francois catalog serve \
 --database-url-file "$fixture_dir/signup-database-url" \
 --listen 127.0.0.1:18795 --public-url http://localhost:18795 --app-domain apps.localhost \
 --release-dir "$fixture_dir/releases" --font-dir /home/manuel/.config/francois/fonts \
 --backend fake --oidc-issuer http://localhost:18083 \
 --signup-allowed --email-signin-enabled --zitadel-organization "$task_org" \
 --zitadel-login-token-file "$fixture_dir/login.pat"
