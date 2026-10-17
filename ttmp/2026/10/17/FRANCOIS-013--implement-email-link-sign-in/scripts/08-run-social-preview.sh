#!/usr/bin/env bash
# Local visual check only: social provider IDs are fixture placeholders.
# Backend routing is covered separately by TestSharedSocialSignin.
set -euo pipefail
fixture_dir=/tmp/francois-013-private
task_org=$(python3 -c 'import json; print(json.load(open("/tmp/francois-013-private/fixture.json"))["organization"])')
exec go run ./cmd/francois catalog serve \
 --database-url-file "$fixture_dir/database-url" \
 --listen 127.0.0.1:18794 --public-url http://localhost:18794 --app-domain apps.localhost \
 --release-dir "$fixture_dir/releases" --font-dir /home/manuel/.config/francois/fonts \
 --backend fake --oidc-issuer http://localhost:18083 --oidc-client-id=local-preview \
 --email-signin-enabled --zitadel-organization "$task_org" \
 --zitadel-login-token-file "$fixture_dir/login.pat" \
 --zitadel-google-idp-id=google-local --zitadel-github-idp-id=github-local --signup-allowed
