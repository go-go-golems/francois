---
Title: Production rollout receipt
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://doc/production.md
      Note: Operational policy and strict-schema rollback
    - Path: repo://ttmp/2026/10/16/FRANCOIS-008--rss-feeds-and-first-party-traffic-and-model-analytics/scripts/05-production-smoke.py
      Note: Read-only checks on both production domains
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T18:21:31.67594065-04:00
WhatFor: ""
WhenToUse: ""
---

# Production rollout receipt

PR [#7](https://github.com/go-go-golems/francois/pull/7) merged as `fda53827a21abb0039c6c40cff6c4c72e072343d` at 2026-10-16T22:14:49Z. It includes fixes for all three P2 review findings: unusable withdrawn release pins, concurrent tour-start charging and expired lease quota retention. All three review threads are resolved.

## Release and deployment

- Source image workflow: [38974582680](https://github.com/go-go-golems/francois/actions/runs/38974582680), successful publish and GitOps PR creation.
- Main Secret Scanning: [38974582260](https://github.com/go-go-golems/francois/actions/runs/38974582260), passed. The earlier PR-only failure was the historical synthetic referrer fixture; the final tree uses a precise inline ignore, with no broad detector/path exclusion. Dependency Review passed after enabling repository alerts/graph access.
- Deployment PR: [wesen/2026-03-27--hetzner-k3s#358](https://github.com/wesen/2026-03-27--hetzner-k3s/pull/358). Exact diff: one image line; manifest validation passed.
- GitOps merge: `3c71ab40ead780b334ecd23ad1235374941c9f06` at 2026-10-16T22:20:05Z.
- Deployment image: `ghcr.io/go-go-golems/francois:sha-fda5382`.
- Observed pod image: `ghcr.io/go-go-golems/francois@sha256:275821a9cd83a15e1af1a29045cfafc554951114a2f97e0b19dac7680efb0c3b`.
- Pod `francois-78fbcf8976-s6992`: Running, Ready, zero restarts.
- Argo CD at the GitOps merge: Synced, Healthy, operation Succeeded, message “successfully synced (all tasks run)”.
- `kubectl rollout status deployment/francois`: successfully rolled out, checked after observing the expected image.

Production, including both https://slop.boutique/ and https://francois.yolo.scapegoat.dev/, now runs the new implementation. The separate local Compose stack was not redeployed during this production rollout.

## Backup and rollback

Before source merge/migration, the existing in-cluster PostgreSQL backup CronJob ran as Job `postgres-backup-francois-008-20261016`. It completed, its upload receipt is present, and its log contains no pg_dump/pg_dumpall/fatal upload error. Data stayed in the established cluster/object-storage backup workflow.

Automatic approval review rejected exporting the production database into local /tmp because the deployment request did not specifically authorize transferring sensitive data to that destination. That export was not retried or bypassed; the established backup job was the safer alternative. No production dump was downloaded or committed.

Migration 011 rewrites stored settings revisions into the new strict shape. Reverting only the image is insufficient; rollback requires matching pre-migration Francois database state and the old image. The generated GitOps PR's generic rollback body was corrected to say so. The old CLI's `gh pr edit` failed on deprecated classic Projects GraphQL fields; REST PATCH successfully applied and verified the correction.

## Policy and persistence

Migrations 001–011 are present; 009, 010 and 011 applied during this release. Active settings revision remains 1, transformed by the explicit migration:

| Setting | Verified production value |
| --- | --- |
| Accept new visits | true |
| Sampling enabled | true |
| Storage mode | read-write |
| Operator notice | empty |
| Global/per-user call concurrency | 6 / 1 |
| Per-user views | 3 |
| Aggregate logical storage per instance | 16,777,216 bytes |
| Document count per instance | 500 |

| Aggregate records | Before | After |
| --- | ---: | ---: |
| Users | 3 | 3 |
| Instances | 4 | 4 |
| Releases | 1 | 1 |
| Model calls | 1 | 1 |
| Documents | 0 | 0 |

The existing model call is attributed to an artifact after migration 010 (one total, one attributed). These checks read only counts, migration names and nonsecret policy fields; no saved user content or credential values were printed.

## Public smoke verification

The tracked [production smoke script](../scripts/05-production-smoke.py) passed 22 read-only checks across both domains:

- Home, application/journal listings, privacy and embedded catalog.js return HTTP 200.
- Health returns HTTP 200; readiness returns ready with models true.
- `/feed.xml`, `/apps/feed.xml` and `/journal/feed.xml` parse as RSS. Links use the request's portal origin; combined-feed GUIDs are opaque non-permalinks.
- Combined/release feeds each have one item; the journal feed has zero items in this production dataset.
- Anonymous `/api/admin/analytics` returns HTTP 401 on both domains.

The script uses a curl-prefixed agent excluded by traffic recording and makes no provider call or persistent application mutation. Authenticated browser operations and service-pause behavior were verified locally in the earlier 21-check receipt; this production smoke run does not claim to repeat those browser/account/provider tests.

## Validation of review fixes

- Full real-PostgreSQL `go test ./... -count=1` passed after the final storage change (`pkg/catalog 6.871s`).
- `go build ./...` passed.
- Repository lint passed after workspace/tour fixes; catalog lint passed after lease cleanup, both with zero issues. Main CI tests/lint/frontend/CodeQL/security checks passed.
- Regression tests cover preserving data/identity during withdrawn-pin recovery, preserving permitted pins, concurrent Try requests spending one start, failed capacity consuming no start, live lease quota enforcement and expired lease reclamation.
- The earlier intermittent new-artifact-upload race is recorded in the detailed diary; it was not hidden by changing its assertions.

The detailed investigation and release chronology are in [the diary](01-implementation-diary.md), Steps 16–18. The report in go-go-parc commit `afc8140` intentionally remains the earlier reviewed snapshot; this receipt records its subsequently completed review fixes and production release.


## Dependency alerts surfaced after enabling repository review

The documentation push reported 11 open Dependabot advisories (two high, six moderate, three low). The two high alerts concern undici in pnpm-lock.yaml: [unrequested WebSocket subprotocol denial of service](https://github.com/go-go-golems/francois/security/dependabot/9) and [BalancedPool TLS option propagation](https://github.com/go-go-golems/francois/security/dependabot/5); both report a fix in 8.10.2. They require separate dependency/exposure triage and were not fixed in this release.

Comparison of source base 64c7ece and deployed fda5382 confirms pnpm-lock.yaml, go.mod, go.sum and worker/package.json are unchanged. The package.json change only updates the frontend test command. These are pre-existing locked dependencies surfaced by enabling alerts, distinct from a newly introduced dependency change or a failing release check. The verified production image remains sha-fda5382 with one ready replica after the documentation push.
