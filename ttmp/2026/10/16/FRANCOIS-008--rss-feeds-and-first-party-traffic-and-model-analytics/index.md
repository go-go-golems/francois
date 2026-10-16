---
Title: RSS feeds and first-party traffic and model analytics
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T11:26:00.385927044-04:00
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

Initial implementation is committed as `0a66114`. The expanded graph/filter/date/artifact work is specified for the next implementation milestone.

## Feature entry points

- Release feed: `/apps/feed.xml`; existing journal feed: `/journal/feed.xml`.
- Admin dashboard: `/admin/analytics`.
- Admin JSON report: `/api/admin/analytics`.
- Migration: `009_analytics.sql`; apply through the normal catalog migration command before rollout.

The first implementation counts successful portal HTML document loads. SPA-only navigation, time-on-page, geolocation, and conversion funnels are documented extensions. Traffic can be filtered by site; model usage remains catalog-wide because its ledger has no site attribution.
