#!/usr/bin/env bash
# Fixed local preview backed by the isolated ticket stack; no production mode.
set -euo pipefail
cd /home/manuel/code/wesen/go-go-golems/francois
fixture_dir=/tmp/francois-013-private
task_org=$(python3 -c 'import json; print(json.load(open("/tmp/francois-013-private/fixture.json"))["organization"])')
task_subject=$(python3 -c 'import json; print(json.load(open("/tmp/francois-013-private/fixture.json"))["subject"])')
if ! psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -Atc "SELECT 1 FROM pg_database WHERE datname='francois_email_preview'" | rg -q 1; then
  psql 'postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' -v ON_ERROR_STOP=1 -c 'CREATE DATABASE francois_email_preview'
fi
printf '%s' 'postgres://emailtest:emailtest@127.0.0.1:5437/francois_email_preview?sslmode=disable' > "$fixture_dir/database-url"
chmod 600 "$fixture_dir/database-url"
go run ./cmd/francois catalog migrate --database-url-file "$fixture_dir/database-url" >/dev/null
psql 'postgres://emailtest:emailtest@127.0.0.1:5437/francois_email_preview?sslmode=disable' -v ON_ERROR_STOP=1 --set=subject="$task_subject" <<'SQL'
INSERT INTO users(id,issuer,subject,email,email_verified,display_name)
VALUES('usr_email_fixture','http://localhost:18083',:'subject','supporter@slop.test',true,'Local Supporter')
ON CONFLICT(issuer,subject) DO NOTHING;
SQL
exec go run ./cmd/francois catalog serve \
 --database-url-file "$fixture_dir/database-url" \
 --listen 127.0.0.1:18793 --public-url http://localhost:18793 --app-domain apps.localhost \
 --release-dir "$fixture_dir/releases" --font-dir /home/manuel/.config/francois/fonts \
 --backend fake --oidc-issuer http://localhost:18083 \
 --email-signin-enabled --zitadel-organization "$task_org" \
 --zitadel-login-token-file "$fixture_dir/login.pat"
