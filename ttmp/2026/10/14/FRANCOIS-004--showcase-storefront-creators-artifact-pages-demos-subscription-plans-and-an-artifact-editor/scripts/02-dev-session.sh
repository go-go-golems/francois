#!/usr/bin/env bash
# Create a portal session for a local dev user and print the cookie value
# for francois_session_dev. Local databases only (no OIDC in the dev server).
# Usage: 02-dev-session.sh <database name> <user id>
set -euo pipefail
token=$(openssl rand -hex 32)
hash=$(printf '%s' "$token" | sha256sum | cut -d' ' -f1)
docker exec -i francois-catalog-pg psql -q -U francois -d "$1" -v ON_ERROR_STOP=1 -c \
  "INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at) VALUES ('\\x$hash','$2','$(openssl rand -hex 16)',now()+interval '12 hours',now()+interval '1 day')"
echo "$token"
