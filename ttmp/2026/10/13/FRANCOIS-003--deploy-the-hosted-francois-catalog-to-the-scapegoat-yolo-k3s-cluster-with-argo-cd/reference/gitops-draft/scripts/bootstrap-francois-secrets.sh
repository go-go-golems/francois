#!/usr/bin/env bash
# Seed kv/apps/francois/prod/runtime (and optionally .../pi) for
# gitops/kustomize/francois. Values come from the environment or from the
# existing Vault record; generated values are never printed.
#
# Required: VAULT_ADDR VAULT_TOKEN FRANCOIS_OIDC_CLIENT_ID FRANCOIS_ZITADEL_ORG_ID
#           FRANCOIS_STRIPE_SECRET_KEY FRANCOIS_STRIPE_WEBHOOK_SECRET FRANCOIS_STRIPE_PRICE_ID
# Optional: FRANCOIS_STRIPE_WEBHOOK_SECRET_PREVIOUS (rotation), FORCE_ROTATE_DB=1,
#           FRANCOIS_PI_DIR=<dir with auth.json/models.json/models-store.json/settings.json>
set -euo pipefail

for c in vault jq openssl; do
  command -v "$c" >/dev/null || { echo "required command not found: $c" >&2; exit 1; }
done
require() { [[ -n "${!1:-}" ]] || { echo "missing required environment variable: $1" >&2; exit 1; }; }
for v in VAULT_ADDR VAULT_TOKEN FRANCOIS_OIDC_CLIENT_ID FRANCOIS_ZITADEL_ORG_ID \
         FRANCOIS_STRIPE_SECRET_KEY FRANCOIS_STRIPE_WEBHOOK_SECRET FRANCOIS_STRIPE_PRICE_ID; do
  require "$v"
done
case "$FRANCOIS_STRIPE_SECRET_KEY" in
  sk_test_*|rk_test_*) ;;
  *) echo "refusing: the yolo beta uses a Stripe test-mode key" >&2; exit 1 ;;
esac

path=kv/apps/francois/prod/runtime
# Distinguish "no record yet" from any other Vault error, so a permission or
# network failure cannot silently rotate the database password.
if out=$(vault kv get -format=json "$path" 2>&1); then
  existing=$(jq -c '.data.data' <<<"$out")
elif grep -q 'No value found' <<<"$out"; then
  existing='{}'
else
  echo "$out" >&2
  exit 1
fi
password=$(jq -r '.password // empty' <<<"$existing")
if [[ -z $password || ${FORCE_ROTATE_DB:-0} == 1 ]]; then
  password=$(openssl rand -hex 24)
fi
database=francois
username=francois_app
host=postgres.postgres.svc.cluster.local

vault kv put "$path" \
  database="$database" \
  username="$username" \
  password="$password" \
  database-url="postgres://${username}:${password}@${host}:5432/${database}?sslmode=disable" \
  oidc-client-id="$FRANCOIS_OIDC_CLIENT_ID" \
  oidc-org-id="$FRANCOIS_ZITADEL_ORG_ID" \
  stripe-secret-key="$FRANCOIS_STRIPE_SECRET_KEY" \
  stripe-webhook-secret="$FRANCOIS_STRIPE_WEBHOOK_SECRET" \
  stripe-webhook-secret-previous="${FRANCOIS_STRIPE_WEBHOOK_SECRET_PREVIOUS:-}" \
  stripe-price-id="$FRANCOIS_STRIPE_PRICE_ID" >/dev/null
echo "Seeded $path (database $database, role $username)."

if [[ -n ${FRANCOIS_PI_DIR:-} ]]; then
  args=()
  for f in auth.json models.json models-store.json settings.json; do
    [[ -f $FRANCOIS_PI_DIR/$f ]] && args+=("$f=@$FRANCOIS_PI_DIR/$f")
  done
  ((${#args[@]})) || { echo "no Pi provider files in $FRANCOIS_PI_DIR" >&2; exit 1; }
  vault kv put kv/apps/francois/prod/pi "${args[@]}" >/dev/null
  echo "Seeded kv/apps/francois/prod/pi (${#args[@]} files)."
fi
