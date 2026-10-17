---
Title: Hide the public blog by default behind an admin storefront setting
Ticket: FRANCOIS-016
Status: complete
Topics:
    - frontend
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/feed.go
      Note: Artifact-only RSS when blog is hidden
    - Path: repo://pkg/catalog/journal_visibility_test.go
      Note: Admin-toggle and visibility regression
    - Path: repo://pkg/catalog/pages.go
      Note: Public visibility and private editor previews
    - Path: repo://pkg/catalog/settings/settings.go
      Note: Default-off public journal setting
    - Path: repo://pkg/catalog/studio.go
      Note: Upload reauthorization race found during qualification
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T17:23:53.024441501-04:00
WhatFor: ""
WhenToUse: ""
---
# Hidden-by-default public blog

Add `storefront.journalEnabled`, false by default, to the existing admin settings document. The control is **Admin → Settings → Storefront → Show the blog publicly**. Existing saved documents without the property also default to hidden; no migration is needed.

The public site keeps its artifact archive, detail pages and release RSS. While the setting is off, journal index/feed/post pages are hidden from visitors, creator pages omit posts, and both public and React footers omit Journal. Authorized editors retain Studio access and private individual previews. No posts are deleted or unpublished.

- [Detailed implementation diary](reference/01-implementation-diary.md).
- [Browser validation](sources/browser-validation.json) and the admin screenshots.
- [Tasks](tasks.md) and [changelog](changelog.md).

The setting is implemented and verified in the admin browser. Full Go tests, frontend checks/build, Go build and lint pass. Full-suite validation also exposed an existing first-artifact upload authorization race; it was fixed separately using the existing reauthorization path and a deterministic regression. No production deployment is included.
