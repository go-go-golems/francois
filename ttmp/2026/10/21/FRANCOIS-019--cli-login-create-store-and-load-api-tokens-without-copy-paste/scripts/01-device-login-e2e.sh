#!/usr/bin/env bash
# End-to-end check of `francois catalog auth login` against a real catalog
# server. A creator and a browser session are inserted with SQL (no identity
# provider needed); the approval is posted like the /device page does.
# Needs the francois-019-pg container (see the diary, Step 3).
set -euo pipefail
cd "$(git rev-parse --show-toplevel)"
PG="docker exec -i francois-019-pg psql -q -v ON_ERROR_STOP=1 -U francois"
PORT=18093
ORIGIN="http://localhost:$PORT"
WORK=$(mktemp -d)
export FRANCOIS_CONFIG_DIR="$WORK/config"
unset FRANCOIS_SERVER FRANCOIS_TOKEN

$PG -d postgres -c "DROP DATABASE IF EXISTS francois_e2e WITH (FORCE)" -c "CREATE DATABASE francois_e2e"
GOWORK=off go build -o "$WORK/francois" ./cmd/francois
"$WORK/francois" catalog serve --database-url "postgres://francois:francois@127.0.0.1:55439/francois_e2e?sslmode=disable" \
  --migrate --backend fake --listen 127.0.0.1:$PORT --public-url "$ORIGIN" --app-domain apps.localhost \
  --release-dir "$WORK/releases" >"$WORK/serve.log" 2>&1 &
SERVER=$!
trap 'kill $SERVER 2>/dev/null || true' EXIT
for _ in $(seq 100); do curl -fs "$ORIGIN/healthz" >/dev/null && break; sleep 0.2; done

$PG -d francois_e2e <<'SQL'
INSERT INTO users(id,issuer,subject,email,email_verified,display_name,role)
  VALUES ('usr_e2e','https://issuer.test','e2e','e2e@example.com',true,'Eve Tester','creator');
INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,absolute_expires_at,id_token)
  VALUES (sha256('e2e-session'::bytea),'usr_e2e','e2e-csrf',now()+interval '1 day',now()+interval '2 days','');
SQL

echo "== /device serves the portal app"
curl -fs "$ORIGIN/device?code=X" | grep -q 'id="root"' && echo ok

echo "== auth login (background)"
"$WORK/francois" catalog auth login --server "$ORIGIN" --no-browser --name "e2e laptop" --format json >"$WORK/login.json" 2>"$WORK/login.err" &
LOGIN=$!
for _ in $(seq 50); do grep -q "code=" "$WORK/login.err" 2>/dev/null && break; sleep 0.2; done
cat "$WORK/login.err"
CODE=$(grep -o 'code=[A-Z-]*' "$WORK/login.err" | head -1 | cut -d= -f2)

echo "== approval page data for $CODE"
COOKIE="Cookie: francois_session_dev=e2e-session"
curl -fs -H "$COOKIE" "$ORIGIN/api/device/requests/$CODE"; echo
echo "== approve (studio only, 7 days)"
curl -fs -H "$COOKIE" -H "Origin: $ORIGIN" -H "X-CSRF-Token: e2e-csrf" -H "Content-Type: application/json" \
  -d '{"scopes":["studio"],"days":7}' "$ORIGIN/api/device/requests/$CODE/approve"; echo
wait $LOGIN
echo "== login result"; cat "$WORK/login.json"; tail -1 "$WORK/login.err"
ls -l "$FRANCOIS_CONFIG_DIR/credentials.json"

echo "== whoami with the stored login (no flags, no env)"
"$WORK/francois" catalog whoami --format json
echo "== auth status --check"
"$WORK/francois" catalog auth status --check --format table
echo "== auth token to a pipe (first 8 characters)"
"$WORK/francois" catalog auth token | cut -c1-8
echo "== logout revokes"
"$WORK/francois" catalog auth logout --format table
TOKEN=$(python3 -c 'import json,sys;print(json.load(open(sys.argv[1]))[0]["token_id"])' "$WORK/login.json")
$PG -d francois_e2e -At -c "SELECT id, revoked_at IS NOT NULL AS revoked FROM api_tokens WHERE id='$TOKEN'"
$PG -d francois_e2e -At -c "SELECT action FROM admin_audit WHERE actor_id='usr_e2e' ORDER BY created_at"
