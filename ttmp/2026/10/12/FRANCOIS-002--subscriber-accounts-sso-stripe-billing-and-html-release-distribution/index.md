---
Title: Subscriber accounts SSO Stripe billing and HTML release distribution
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

# Subscriber accounts, SSO, Stripe billing, and HTML release distribution

This ticket contains the analysis and proposed implementation guide for a subscriber website that distributes curated HTML releases. The recommended first release uses ZITADEL OIDC, Stripe Checkout/Portal, PostgreSQL billing/account projections, private downloads, and opted-in release notifications. The existing localhost/Pi runtime remains a separate product operation.

## Documents

- [Intern analysis, design, and implementation guide](design-doc/01-subscription-service-analysis-design-and-intern-implementation-guide.md)
- [Investigation diary](working-note/01-subscription-service-investigation-diary.md)
- [Preserved research source manifest](sources/manifest.json)
- [Implementation tasks](tasks.md)

## Current status

The 20-section intern guide is complete, locally validated, and uploaded to reMarkable cloud at `/ai/2026/10/12/FRANCOIS-002`. Account/billing/distribution implementation remains open in eight phases. Pricing, trial/grace, deployment domain, sender, and license are visible owner decisions. The optional scope clarification was not required to complete the distribution-first guide.

Prior vault reports are preserved in sources; current API links are annotated in sources/06-external-api-references.md. The exact guide hash and upload receipt are recorded in sources/guide-delivery.json and the diary.
