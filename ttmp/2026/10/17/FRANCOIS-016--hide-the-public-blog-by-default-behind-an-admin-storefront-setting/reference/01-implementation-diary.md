---
Title: Implementation diary
Ticket: FRANCOIS-016
Status: active
Topics:
    - frontend
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T17:15:03.46672879-04:00
WhatFor: ""
WhenToUse: ""
---
# Implementation diary

## Step 1: Default the public blog off and wire the admin control

The user wants an artifact-focused public site before remote deployment, while retaining a way to enable the blog. I added one boolean to the existing versioned settings document, rather than a new configuration service or migration. The setting covers every existing public post surface and retains private editorial previews.

The focused database tests and a real admin browser round trip passed. The local preview was left with the setting false. A full-suite run exposed an existing concurrent first-artifact upload authorization race; that issue is separated from this feature and will be addressed before final qualification.

### Prompt Context

**User prompt (verbatim):** "ok, before deploying remotely, let's make the blog hidden per deafult (admin setting), and keep just the artifact"
**Assistant interpretation:** Implement a default-off public blog setting, expose it in Admin and apply it consistently before any production deployment.
**Inferred user intent:** Keep public presentation minimal and artifact-focused while preserving the editorial system.
**Starting revision:** 0cb9c35.

### What I did

- Added storefront.journalEnabled with a natural false default, including previously saved JSON documents where the field is absent.
- Added the Show the blog publicly checkbox under Admin Settings Storefront and the matching TypeScript contract.
- Gated /journal and /journal/feed.xml, public individual post access, creator-page posts and mixed RSS blog items. Artifact pages and release feeds remain available.
- Retained editor-authorized individual previews with a clear hidden-blog banner, private/no-store and noindex. Existing draft previews also receive private caching.
- Made the public footer conditional and returned the setting from /api/session for the React footer. Settings saves invalidate the Session query so navigation updates immediately.
- Made visibility-sensitive feeds revalidate rather than cache stale blog visibility for five minutes; artifact-only feed caching remains unchanged.
- Updated older journal tests to explicitly enable the feature they exercise. Added default-off/old-document/admin-toggle/preview/role/feed regression coverage.
- Added a local-only browser script and screenshots. It creates a temporary post/session, saves both checkbox states through the real UI, verifies anonymous post/feed access and cleans up afterward.
- Updated production documentation and added a cross-ticket visibility requirement to the proposed recursive catalog API ticket.

### Why

A hidden footer link alone would leave published posts reachable by URL or RSS. Enforcing the same flag at each public read surface gives the requested behavior. Editorial availability and public blog visibility are separate concerns; hiding the blog must not delete or unpublish content.

### What worked

- Focused database suite: TestJournal, TestFinal, TestStorefrontPages and TestCombinedFeed passed.
- pnpm check, pnpm test (9 tests) and pnpm build using Node v24.18.0/Corepack passed.
- go build ./... and make lint logcopter-check passed.
- Chromium confirmed the default-off checkbox/footer, enabling and disabling through Save & activate, anonymous post/feed visibility, and preserved artifact archive. Four browser checks passed; temporary post/session were removed and hidden state restored.

### What didn't work

- The initial full suite failed TestConcurrentNewArtifactUpload with race-7 upload 0 returning 409 version_exists instead of the expected 403 not_editor. Three isolated repeats passed, but the next full suite reproduced the failure at race-1. The upload code is outside the modified visibility paths; its existing check-to-publication race is recorded for a separate fix, not hidden by repeated retries.
- The first local browser run failed its readiness assertion because the previous tmux pane had returned to a shell after the old server stopped. A respawn attempt still did not produce a healthy preview. The local preview database also lacked the earlier email feature's 012_email_signins.sql migration; applying that existing local migration and starting a fresh pane produced a healthy server. No new migration is required by the blog flag itself.

### What I learned

Changing a setting used by the React footer also requires invalidating the Session query, not only Settings and Account. Existing saved JSON already has the correct false default without a compatibility adapter.

### What was tricky to build

Published posts must remain previewable by their authorized editors while being hidden from all ordinary visitors and other creators. The preview uses the existing canEditPost policy and private caching rather than a second public route or a user-supplied bypass parameter.

### What warrants a second pair of eyes

Review the post preview checks, RSS SQL predicate and creator-page query guard together. The setting is presentation control, not a promise to revoke cached copies of previously published media URLs.

### What should be done in the future

Resolve the upload authorization race exposed by the full gate, then record final verification. Production deployment remains a separate user decision.

### Code review instructions

Start with settings.Storefront, Admin Settings, pages.go post/journal/creator handlers and feed.go. Run TestJournalHiddenByDefaultAndAdminToggle against a disposable database. Browser evidence is under sources/.

### Technical details

- Existing settings endpoint: PATCH /api/admin/settings with normal admin, CSRF and If-Match requirements.
- Default and missing-property behavior: storefront.journalEnabled=false.
- Local preview: http://localhost:18792; no production settings were changed.
