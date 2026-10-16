---
Title: Local Compose deployment and verification
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://Dockerfile
      Note: Rebuilt local image with explicit source revision
    - Path: repo://deploy/compose/bin/catalog-serve
      Note: Preserved runtime provider identity and font configuration
    - Path: repo://deploy/compose/compose.yaml
      Note: Existing local service and persistent volumes
ExternalSources: []
Summary: Real local rollout, migrations, health and authenticated browser validation.
LastUpdated: 2026-10-16T12:35:00-04:00
WhatFor: Verify the locally running analytics deployment and reproduce rollout.
WhenToUse: Reviewing the local deployment after implementation.
---

# Local deployment receipt

The existing Compose service is deployed at [local Analytics](http://localhost:8090/admin/analytics). This is the actual catalog and PostgreSQL database, not the synthetic preview. Production was not accessed or deployed.

## Deployed application

- Revision label: `1d8a137` (includes backend `2f53554` and frontend `f072af7`).
- Image: `sha256:9bba2f8f7ae5916c2b59ae74d803e346257442a81b8215fa142d6e7ec255702a`.
- Container: `francois-catalog-francois-1`.
- Preserved configuration: Pi backend; portal `http://localhost:8090`; application domain `apps.localhost`; 30-minute grants.
- Existing bootstrap/release/database and provider/font mounts were retained.
- Only the Francois service was recreated. PostgreSQL, ZITADEL and the existing Stripe forwarder were retained.

## Migration and data preservation

Before rollout the database had migrations 001–008. The rebuilt image applied `009_analytics.sql` and `010_model_call_artifacts.sql` through `catalog migrate`, without re-running identity/user provisioning.

Before and after: 7 users, 13 instances, 5 releases, 5 model calls. All five historical calls received provable artifact IDs. Verification did not invoke the model provider.

A private custom-format database backup (PGDMP header verified, 96,343 bytes, mode 0600) is retained at `/tmp/francois-008-local-predeploy.dump`. It contains application data and is deliberately not committed. The previous image is retained locally as `francois-catalog:pre-analytics-1d8a137`.

## Health and authenticated verification

- `/healthz`: HTTP 200, `{"status":"ok"}`.
- `/readyz`: HTTP 200, `{"models":true,"status":"ready"}`.
- The existing browser admin session survived recreation.
- Live feeds returned valid RSS XML; anonymous analytics access returned 401.
- Real page/referrer stacks reconcile to the collected traffic count.
- Existing model calls are grouped as Polis IDE (4) and Capability playground (1), without new provider calls.

### Result
{"checks":["Public document / responds","Public document /apps responds","Public document /journal responds","Public document /pricing responds","Existing admin session survives service recreation","Authenticated analytics reads real collected traffic","Exact page and direct-referrer filters work against real database","Real pages stack reconciles with total","Real referrers stack reconciles with total","Unauthenticated report access is denied","Live RSS /apps/feed.xml is valid","Live RSS /journal/feed.xml is valid","Stack-by-page mode selects","Stack-by-referrer mode selects","Date presets and Cancel work in deployed UI","Live model token and cost graphs render","All existing calls appear in per-artifact reporting without provider calls","No browser page errors"],"report":{"trafficViews":6,"modelCalls":5,"artifactCalls":5,"artifacts":[{"id":"polis","label":"Polis IDE","calls":4},{"id":"capabilities","label":"Capability playground","calls":1}]},"errors":[]}


## Rollout commands

Run from the repository root. Compose reads the existing local .env settings; do not print expanded credentials.

```sh
docker compose -f deploy/compose/compose.yaml build \
  --build-arg REVISION=1d8a137 --build-arg VERSION=local-analytics francois
docker compose -f deploy/compose/compose.yaml run --rm --no-deps \
  --entrypoint /bin/sh francois \
  -c 'exec francois catalog migrate --database-url "$DATABASE_URL"'
docker compose -f deploy/compose/compose.yaml stop francois
lsof-who -p 8090 -k
docker compose -f deploy/compose/compose.yaml up -d --no-deps --no-build \
  --force-recreate francois
```

The build ran in tmux and its temporary output was kept in /tmp. The completed build session was closed; Compose owns the running service. The database backup and previous image are retained for operator recovery, not automatically restored. Any recovery that replaces data should be explicitly reviewed.

The browser check source is `scripts/03-local-deployment-check.js`. It records this deployment's five-call baseline; update that expectation when reusing it after additional real usage.
