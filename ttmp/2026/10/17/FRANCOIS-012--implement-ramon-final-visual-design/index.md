---
Title: Final visual redesign intern implementation guide
Ticket: FRANCOIS-012
Status: complete
Topics:
    - frontend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/payments/gateway.go
      Note: Stripe recurring checkout boundary
    - Path: repo://pkg/catalog/web/templates/pricing.tmpl
      Note: Final support page and contribution form
    - Path: repo://ui/catalog/public.ts
      Note: Contribution controls and hosted checkout binding
ExternalSources: []
Summary: Detailed intern guide for a small concrete implementation; runtime work remains open.
LastUpdated: 2026-10-17T15:30:25.528444728-04:00
WhatFor: Implement the scoped feature.
WhenToUse: Planning and reviewing this implementation.
---

# Final visual redesign intern implementation guide

Implemented the scoped Final visual design: shared Go/React shell, compact searchable archive, editorial experiment pages, top current-release download and explicit-start isolated runner. Local preview: http://localhost:18792 (fake backend). Email-link sign-in remains FRANCOIS-013.

- [Intern implementation guide](design-doc/01-final-visual-redesign-intern-implementation-guide.md).
- [Investigation diary](reference/01-investigation-diary.md).
- [Tasks](tasks.md) and [changelog](changelog.md).
- [Related ticket](../FRANCOIS-013--implement-email-link-sign-in/index.md).
- [Original measured comparison](../FRANCOIS-011--align-slop-boutique-with-ramon-final-design/index.md).

The guide includes system orientation, prose explanations, file/API references, pseudocode, rendered diagrams and observable acceptance checks. Requested reMarkable delivery is recorded separately in sources/upload-receipt.txt after upload.

## Delivery

Uploaded to `/ai/2026/10/17/FRANCOIS-012/`. [Upload receipt](sources/upload-receipt.txt) and [delivery summary](sources/delivery-summary.json) confirm cloud delivery; device sync was not verified. The guides were committed in `84da5b6`. Implementation is complete; production deployment was not part of this task.

## Implementation evidence

- Commits: `06a1219` (shell/archive), `576764b` (detail/runner/editorial), plus the final evidence/query checkpoint.
- [Browser verification](sources/implementation/browser-validation.json): 1440/900/390 layouts, no-JavaScript search, no implicit visits, real guest startup, same-instance reload/fullscreen, paused-response UI and actual expiry denial.
- [Checks](sources/implementation/checks.json): complete Go suite, frontend checks/build and Go build passed.
- [Mobile archive](sources/implementation/archive-390.png), [desktop preview](sources/implementation/archive-hover-1440.png), [live experiment](sources/implementation/experiment-live-1440.png).
- Detailed rationale and failure/recovery evidence are in diary Steps 3–5.
