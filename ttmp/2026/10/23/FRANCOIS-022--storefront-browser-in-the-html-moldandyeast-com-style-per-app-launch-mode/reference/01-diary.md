---
Title: Diary
Ticket: FRANCOIS-022
Status: active
Topics:
    - frontend
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological diary for FRANCOIS-022."
LastUpdated: 2026-10-23T15:29:29.54802733-04:00
WhatFor: ""
WhenToUse: ""
---
# Diary

## 2026-10-23 — Study and design

**Request.** Use the html.moldandyeast.com layout for the slop.boutique landing page and browser while keeping our header, support banner and footer; add a per-app setting to start the HTML directly or to show the cover and open full screen in a new tab.

**Study.**
- Looked at html.moldandyeast.com with Playwright at 1440 px: header with address-style search, display headline, sort and tag counts, month groups, `.ri` rows with Open · Download · path · size, and a full-viewport viewer dialog (`/?open=slug`) with device sizes. It is client-rendered with absolute positioning. Saved `index.html`, `styles.css`, `js/app.js` in `sources/`.
- Mapped our side with an exploration pass: `feedPage`/`cards` in `pages.go`, `home.tmpl`, `artifact.tmpl`, `layout.tmpl`, `catalog.css` sections, `public.ts` `renderActions`, launch handshake (`instance-launch.ts`, `launch.ts`, `hosted/routes.go`), `/instances/{id}` Launcher, entitlements, demos, settings revisions, artifact model and migrations (next is 020).
- Key constraint found: `launch.ts` refuses to run unframed, so "new tab" must open the portal page `/instances/{id}`.

**Decisions from Manuel.** Two modes (`inline`, `tab`); header untouched; hero is a different sentence ("These artifacts are the results of our research. Play with them, download the HTML, and drop them into your coding agent."), editable in the backend; build a CMS-like mechanism; download hidden only during early access for non-subscribers; in early access tell non-subscribers they can subscribe; launch button blue through a design token.

**Design choices.**
- Site copy lives in the settings document (`storefront.copy`) with a Go registry of keys, because settings already have revisions, If-Match concurrency, admin UI, CLI and MCP.
- Default launch mode `tab` so nothing changes until an editor switches an app.
- No auto-start for demos on page load (demo budget); explicit clicks may start demos.
- Server-rendered list with CSS grid; small live filter in `public.ts`.

**Next.** Wait for review of the guide, then Phase 1.
