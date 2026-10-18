---
Title: Email link sign in intern implementation guide
Ticket: FRANCOIS-013
Status: complete
Topics:
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/email_signin.go
      Note: Browser form flow and proof orchestration
    - Path: repo://pkg/catalog/identity/email.go
      Note: ZITADEL email challenge client
    - Path: repo://pkg/catalog/store/email_links.go
      Note: Unified pending links and role-preserving local sessions
ExternalSources: []
Summary: Unified mailbox verification deployed on production k3s, preserving Google and GitHub SSO.
LastUpdated: 2026-10-17T16:44:52.803167379-04:00
WhatFor: Implement the scoped feature.
WhenToUse: Planning and reviewing this implementation.
---

# Email link sign in intern implementation guide

Unified email onboarding is deployed on production k3s as `sha-8f5fa4a` (source PR #14, GitOps PR #370). Email verification supports new and existing provider identities without OTP enrollment; Google and GitHub SSO remain available. See the [active implementation guide](design-doc/02-email-only-signup-implementation-guide.md), [production receipt](sources/implementation/unified-email-production.json), and diary Steps 24–25. Earlier fixture and deployment evidence below is historical.

Local preview: http://localhost:18793/signin. The isolated fixture mailbox is at http://localhost:18025. Both use test-only data.

- [Browser validation and screenshot inventory](sources/implementation/browser-validation.json).
- [Desktop form](sources/implementation/signin-form-1440.png) and [mobile form](sources/implementation/signin-form-390.png).

- [ZITADEL feasibility and source trace](analysis/01-zitadel-email-only-magic-link-feasibility-and-source-trace.md).
- [Saved primary sources](sources/zitadel-email-login/README.md).

- [Intern implementation guide](design-doc/01-email-link-sign-in-intern-implementation-guide.md).
- [Investigation diary](reference/01-investigation-diary.md).
- [Tasks](tasks.md) and [changelog](changelog.md).
- [Related ticket](../FRANCOIS-012--implement-ramon-final-visual-design/index.md).
- [Original measured comparison](../FRANCOIS-011--align-slop-boutique-with-ramon-final-design/index.md).

The current guide includes system orientation, file/API references, pseudocode, a Mermaid sequence diagram and acceptance checks. Requested reMarkable delivery is recorded separately in sources/upload-receipt.txt after upload.

## Delivery

The original first-party guide was uploaded to `/ai/2026/10/17/FRANCOIS-013/`. [Upload receipt](sources/upload-receipt.txt) and [delivery summary](sources/delivery-summary.json) confirm cloud delivery; device sync was not verified. The guides were committed in `84da5b6`. Local implementation is complete. This delivery predates the ZITADEL-backed revision; the PDF has not been refreshed.
