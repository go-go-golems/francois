---
Title: Subscriber hosted artifact platform with SSO Stripe and administration
Ticket: FRANCOIS-002
Status: active
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-12T21:04:40.997707645-04:00
WhatFor: ""
WhenToUse: ""
---

# Subscriber hosted artifact platform with SSO, Stripe, and administration

FRANCOIS-002 now designs a hosted platform, superseding its initial distribution-only assumption. Subscribers run release-pinned application instances with isolated database state, rooms, real signed-in user identity, and quota-controlled Pi-backed LLM calls. An administrator UI manages HTML upload/publication, users, instances, billing/usage inspection, and validated settings. Downloads and release notifications remain available.

## Documents

- [Hosted/admin intern analysis, design, and implementation guide](design-doc/01-subscription-service-analysis-design-and-intern-implementation-guide.md)
- [Investigation diary](working-note/01-subscription-service-investigation-diary.md)
- [Prior-source manifest](sources/manifest.json)
- [Implementation tasks](tasks.md)

## Current status

The 14,176-word revision 2 is locally validated, uploaded to reMarkable cloud, and supersedes the initial downloadable-files scope. The guide describes one Go service, PostgreSQL, a Node Pi worker, trusted account/admin origin, and per-instance application origins. Nine implementation phases remain open; this ticket contains design rather than implemented hosted account functionality.

The original guide was uploaded to /ai/2026/10/12/FRANCOIS-002. Revision 2 is uploaded as FRANCOIS-002 Hosted Platform and Admin Intern Guide v2.pdf, preserving the earlier PDF and its annotations. Receipts and hashes are recorded under sources.
