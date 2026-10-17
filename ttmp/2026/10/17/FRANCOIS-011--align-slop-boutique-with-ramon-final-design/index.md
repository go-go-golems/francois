---
Title: Align Slop Boutique with Ramon Final design
Ticket: FRANCOIS-011
Status: active
Topics:
    - frontend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources:
    - https://app.paper.design/file/01M4XVT9C83GGYK57CMAAAEJJT/p-6-0
Summary: "Ramon Final is authoritative; investigation complete, implementation planned."
LastUpdated: 2026-10-17T14:03:27-04:00
WhatFor: ""
WhenToUse: ""
---

# Align Slop Boutique with Ramon Final design

Ramon’s Final Paper page supersedes the older design reference. The comparison is complete; no UI implementation or deployment is included in this investigation.

- [Comparison and implementation plan](design-doc/01-final-design-comparison-and-implementation-plan.md): measured desktop/mobile differences, existing source and API contracts, three shipping slices and validation.
- [Investigation diary](reference/01-investigation-diary.md): prompt, evidence, failures and design decisions.
- [Tasks](tasks.md) and [changelog](changelog.md).
- [Final Paper page](https://app.paper.design/file/01M4XVT9C83GGYK57CMAAAEJJT/p-6-0).

The sources directory contains ten Final PNGs, ten exact JSX exports, eight fresh production screenshots and browser/style receipts. The capture script is in scripts/. Outstanding choices concern embedded-runner startup, archive content policy, annual funding and actual email-login delivery.

## Scoped implementation guides — 17 October 2026

The user requested separate tickets and a concrete, streamlined implementation. Use these current guides for implementation; this earlier comparison remains the measured evidence record.

- [FRANCOIS-012 — visual design](../FRANCOIS-012--implement-ramon-final-visual-design/index.md): computed display ordinals, existing Markdown and explicit-start reuse of the runner; no visual-ticket database migration or billing redesign.
- [FRANCOIS-013 — email sign-in](../FRANCOIS-013--implement-email-link-sign-in/index.md): one pending-link table and direct mail submission for existing subscriber accounts, reusing current sessions.
