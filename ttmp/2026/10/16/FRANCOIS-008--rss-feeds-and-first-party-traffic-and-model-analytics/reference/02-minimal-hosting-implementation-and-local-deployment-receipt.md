---
Title: Minimal hosting implementation and local deployment receipt
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/operations_test.go
      Note: Policy, concurrency, storage, reuse and public feed integration checks
    - Path: repo://pkg/catalog/store/migrations/011_operations.sql
      Note: Explicit settings migration applied locally
    - Path: repo://ttmp/2026/10/16/FRANCOIS-008--rss-feeds-and-first-party-traffic-and-model-analytics/scripts/04-local-controls-browser-check.js
      Note: 21 real-service checks with settings restoration
    - Path: repo://ui/catalog/service-notice.ts
      Note: Shared plain notice treatment
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T13:35:14.628830805-04:00
WhatFor: ""
WhenToUse: ""
---

# Minimal hosting implementation and local deployment receipt

## Delivered behavior

The implementation uses the existing Go application, PostgreSQL, React administration UI and Pi worker. Backend commit: `ad5f791`; public UI/feed commit: `42a1f3f`.

- Admin → Settings → Service controls manages new visits, AI access and read-write/read-only/off storage, with an optional plain-text notice.
- Six calls globally and one per user; existing private allowance checks and model accounting remain in force. The worker acknowledges call termination before admission is released.
- Tours resume with their original expiry. The existing network limiter also checks signed-in user identity, independent of address changes.
- Open resumes the oldest active workspace for that artifact atomically. Existing additional instances, release pins and URLs remain intact.
- Aggregate logical storage is limited across documents, assets, comments/anchors and leases. Concurrent writes share the same instance lock; shrinking/deleting remains possible above a lowered quota.
- A shared plain notice reports service limits/pauses. After a rejected save, subsequent autosave writes stop at the runtime and the page says edits cannot be saved until reload/retry.
- Account has no public model usage meter. The normal launcher has only Close. The home/catalog/journal listings share a chronological application/writing stream and `/feed.xml`; ordinary navigation omits instance management/pricing. The existing subscription destination is reduced to Support actions.
- Administrator analytics, authored content, entitled downloads, editor previews, subscription management and sign out remain available.

Paper: [Minimal site — service notices](https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-D-0). Current implementation guide: [minimal hosting design](../design-doc/03-minimal-public-experience-and-operational-capacity-controls.md).

## Automated validation

- Full `go test ./...` passed with `TEST_DATABASE_URL` pointing at disposable PostgreSQL 17, separate from the site database.
- `go build ./...` passed; `golangci-lint run -v` reported zero issues.
- TypeScript check, all eight frontend contract/chart tests and web build passed. The shell's Node 20 emitted an engine warning; the deployment image independently rebuilt assets and worker with Node 24.
- New integration checks cover operation changes on existing grants/views; read-only mutation coverage; signed blob reads; no reservation for paused sampling; view bounds; cross-address per-user tour limits; fixed tour expiry; atomic default opening; parallel byte/document-count limits; quota reduction cleanup; aggregate comment/lease storage; combined RSS/HTML publication and privacy rules.
- The cancellation helper delays its completion frame, proving Go waits before releasing admission.

## Actual local rollout

- Portal: http://localhost:8090/
- Controls: http://localhost:8090/admin/settings
- Analytics: http://localhost:8090/admin/analytics
- Primary RSS: http://localhost:8090/feed.xml
- Deployed revision: `42a1f3f`.
- Image: `sha256:b1ee8e4a1040f2413f42379e51c957799fb229af77b12188f260e6ec61fb36ab`.
- Migration result: `011_operations.sql | applied`.
- `/healthz`: HTTP 200, status ok. `/readyz`: HTTP 200, status ready, models true.
- Final process state: running, zero restarts.
- Settings restored after browser checks: acceptNewVisits true, samplingEnabled true, storageMode read-write, empty notice. Concurrency is six global/one per user; aggregate storage cap is 16 MiB.

| Preserved data | Before | After |
| --- | ---: | ---: |
| Users | 7 | 7 |
| Instances | 13 | 13 |
| Releases | 5 | 5 |
| Model calls | 5 | 5 |
| Documents | 7 | 7 |

Local custom-format backup: `/tmp/francois-008-pre-controls.dump`, mode 0600, 99,232 bytes. Previous image retained as `francois-catalog:pre-controls-42a1f3f`. These are private local recovery artifacts, not repository attachments. Migration 011 rewrites settings into the new strict schema: rollback to the old image requires its matching database backup, rather than simply changing the image tag. No restore was performed.

Only the app was recreated; identity, database, release, font and provider-configuration mounts were preserved. Production was not accessed. The build ran in tmux; the completed build session was closed. The disposable test PostgreSQL container is removed after final checks.

## Real-browser receipt

The tracked [browser check](../scripts/04-local-controls-browser-check.js) passed all 21 assertions against the running application using the preserved local admin session:

1. Existing settings editor contains service controls.
2. AI enabled on migrated deployment.
3. Storage initially read-write.
4. Controls save through the existing revision-checked editor.
5. Six global lanes and one per person.
6. Launcher hides instance management.
7. Launcher keeps simple Close action.
8. Real hosted AI request is rejected before provider use.
9. Real hosted write obeys read-only mode.
10. Subsequent autosave attempt is refused locally.
11. Unsaved state is explicit.
12. Operator notice is rendered as plain text.
13. Rejected write was not persisted and reads still work.
14. Open resumes the same workspace while new visits are paused.
15. Account has no public usage meter.
16. Sign out remains available.
17. Home omits hero and creator presentation.
18. Normal navigation omits instances and pricing.
19. Combined RSS is valid XML (eight items in this local dataset).
20. Temporary operation changes restored.
21. No browser page errors.

The check made zero provider calls and persisted no application documents. It deliberately saved temporary settings revisions and their normal audit history, then restored the original migrated policy. Aggregate database counts above independently confirm model-call/document preservation.

Screenshots were reviewed for layout and readability: [service controls](../various/validation/05-service-controls.png), [minimal home](../various/validation/05-minimal-home.png), [staff Account](../various/validation/05-minimal-account.png). The staff screenshot includes API-token management; ordinary accounts do not show it. No token secrets appear in these screenshots.
