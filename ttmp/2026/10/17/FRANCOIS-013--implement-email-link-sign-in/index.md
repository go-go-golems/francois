---
Title: Email link sign in intern implementation guide
Ticket: FRANCOIS-013
Status: active
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Detailed intern guide for a small concrete implementation; runtime work remains open."
LastUpdated: 2026-10-17T14:27:28-04:00
WhatFor: Implement the scoped feature.
WhenToUse: Planning and reviewing this implementation.
---

# Email link sign in intern implementation guide

The current guide selects ZITADEL Session API email-only magic links. Runtime implementation and the isolated real-mail qualification remain open. The superseded local-token design is archived.

- [ZITADEL feasibility and source trace](analysis/01-zitadel-email-only-magic-link-feasibility-and-source-trace.md).
- [Saved primary sources](sources/zitadel-email-login/README.md).

- [Intern implementation guide](design-doc/01-email-link-sign-in-intern-implementation-guide.md).
- [Investigation diary](reference/01-investigation-diary.md).
- [Tasks](tasks.md) and [changelog](changelog.md).
- [Related ticket](../FRANCOIS-012--implement-ramon-final-visual-design/index.md).
- [Original measured comparison](../FRANCOIS-011--align-slop-boutique-with-ramon-final-design/index.md).

The current guide includes system orientation, file/API references, pseudocode, a Mermaid sequence diagram and acceptance checks. Requested reMarkable delivery is recorded separately in sources/upload-receipt.txt after upload.

## Delivery

The original first-party guide was uploaded to `/ai/2026/10/17/FRANCOIS-013/`. [Upload receipt](sources/upload-receipt.txt) and [delivery summary](sources/delivery-summary.json) confirm cloud delivery; device sync was not verified. The guides were committed in `84da5b6`. Implementation remains open. This delivery predates the ZITADEL-backed revision; the PDF has not been refreshed.
