---
Title: RSS feeds and first-party traffic and model analytics
Ticket: FRANCOIS-008
Status: complete
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T18:22:24.67028727-04:00
WhatFor: ""
WhenToUse: ""
---

# RSS feeds and first-party traffic and model analytics

Implemented release RSS and first-party portal document analytics with an admin dashboard and timestamp-based model usage reporting. Existing journal RSS and model settlement remain the foundations. No production migration or deployment was performed.

## Deliverables

- [Intern architecture and implementation guide](design-doc/01-rss-and-analytics-intern-architecture-and-implementation-guide.md): system orientation, evidence, diagrams, APIs, pseudocode, decisions, implementation and rollout instructions, validation, and limits.
- [Implementation diary](reference/01-implementation-diary.md): original request, investigation, implementation decisions, failures, validation, and delivery receipts.
- [Tasks](tasks.md) and [changelog](changelog.md).

## Paper designs and follow-up implementation

- [Paper designs and implementation guide](design-doc/02-analytics-paper-designs-and-implementation-mapping.md): graph/filter/date contracts, SQL/API details, durable artifact attribution, implementation phases and tests.
- [Paper Traffic](https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-A-0) and [Paper Model usage](https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-B-0).

Initial implementation is committed as `0a66114`; the backend follow-up is `2f53554` and the frontend is `f072af7`. The expanded graph/filter/date/artifact work is now implemented and validated; see the diary and implementation guide for checkpoint evidence.

## Local deployment

The application is now running at [local Analytics](http://localhost:8090/admin/analytics). See the [local deployment receipt](various/validation/04-local-deployment.md) for the deployed image/revision, applied migrations, data preservation, recovery artifacts and real-service verification. Production remains unchanged.

## Feature entry points

The [minimal hosting implementation guide](design-doc/03-minimal-public-experience-and-operational-capacity-controls.md) is now implemented in backend `ad5f791` and UI `42a1f3f`, including service controls, six-call admission, private limits, workspace/tour reuse, bounded storage, shared notices and a merged public stream. See the [implementation/local rollout receipt](reference/02-minimal-hosting-implementation-and-local-deployment-receipt.md) for validation and preserved-data evidence.

- Release feed: `/apps/feed.xml`; existing journal feed: `/journal/feed.xml`.
- Admin dashboard: `/admin/analytics`.
- Admin JSON report: `/api/admin/analytics`.
- Migration: `009_analytics.sql`; apply through the normal catalog migration command before rollout.

The first implementation counts successful portal HTML document loads. SPA-only navigation, time-on-page, geolocation, and conversion funnels are documented extensions. Traffic can be filtered by site; model usage remains catalog-wide because its ledger has no site attribution.


## Production completion

PR [#7](https://github.com/go-go-golems/francois/pull/7) and GitOps PR [#358](https://github.com/wesen/2026-03-27--hetzner-k3s/pull/358) are merged. Production runs `ghcr.io/go-go-golems/francois:sha-fda5382` with migrations 009–011 and all three reviewed fixes. See the [production rollout receipt](reference/03-production-rollout-receipt.md), [smoke script](scripts/05-production-smoke.py) and diary Steps 16–18 for validation, backup/rollback requirements and exact evidence.
