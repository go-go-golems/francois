---
Title: Hide the public blog by default behind an admin storefront setting
Ticket: FRANCOIS-016
Status: active
Topics:
    - frontend
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T17:15:03.336693646-04:00
WhatFor: ""
WhenToUse: ""
---
# Hidden-by-default public blog

Add `storefront.journalEnabled`, false by default, to the existing admin settings document. The control is **Admin → Settings → Storefront → Show the blog publicly**. Existing saved documents without the property also default to hidden; no migration is needed.

The public site keeps its artifact archive, detail pages and release RSS. While the setting is off, journal index/feed/post pages are hidden from visitors, creator pages omit posts, and both public and React footers omit Journal. Authorized editors retain Studio access and private individual previews. No posts are deleted or unpublished.

- [Detailed implementation diary](reference/01-implementation-diary.md).
- [Browser validation](sources/browser-validation.json) and the admin screenshots.
- [Tasks](tasks.md) and [changelog](changelog.md).

Full-suite validation also exposed an existing first-artifact upload authorization race, being fixed in a separate commit before final qualification. No production deployment is included.
