---
Title: Implementation diary
Ticket: FRANCOIS-004
Status: active
Topics:
    - backend
    - frontend
    - billing
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological record of designing and implementing the Francois storefront: content model, public pages, studio, plans, demos, tokens."
LastUpdated: 2026-10-14T19:40:00-04:00
WhatFor: "Resume work on FRANCOIS-004 and review how each phase was built and validated."
WhenToUse: "Before continuing or reviewing FRANCOIS-004 implementation."
---

# Implementation diary

## Step 1: Analysis and design document

The owners asked for a storefront where two creators showcase and sell artifact work as subscriptions to live artifacts, with a write-up per artifact, multiple tiers and a real authoring workflow. Before writing any code I read the hosted catalog end to end and wrote an intern-oriented analysis, design and implementation guide, then uploaded it to reMarkable.

The design splits the work into eight phases: content model, public server-rendered pages, a studio API with a creator role, the studio UI, plans and entitlements, visitor demos, API tokens with a push CLI, and documentation with rollout notes. Multi-file artifacts are described only as a direction.

### Prompt Context

**User prompt (verbatim):** "Yes as full design doc. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

Then implement, step by step, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

**Assistant interpretation:** Create ticket FRANCOIS-004 with a design document covering the storefront features discussed earlier (showcase pages, plans, editor, creator role, tokens/CLI, demos), upload it to reMarkable, then implement it phase by phase with commits and an investigation-format diary.

**Inferred user intent:** Turn the closed-beta catalog into something the two creators can actually use to show and sell their work, with documentation good enough for a newcomer to follow.

Earlier context from the same conversation that shaped the scope (the user's words): the product is "Ongoing access to live artifacts (subscriptions)"; the features to cover were listed in my previous answers (showcase pages, artifact editor with tabs, creator role, publishing from laptops, multi-file artifacts later, plans).

### What I did

- Read the hosted catalog: `pkg/catalog/server.go`, `routes.go`, `auth.go`, `admin.go`, `instances.go`, `releases/releases.go`, `billing/policy.go`, `settings/settings.go`, `hosted/runtime.go`, `hosted/routes.go`, `hosted/sample.go`, `store/*.go`, `store/migrations/001_core.sql`, `ui/catalog/main.tsx`, `ui/catalog/admin.tsx`, `ui/catalog/ui.tsx`, `pkg/catalog/web/embed.go`, `scripts/build.mjs`, `cmd/francois/cmds/catalog.go`.
- Ran the existing integration tests against the local PostgreSQL to confirm the harness works: `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable' go test -count=1 -v ./pkg/catalog/` → `TestHostedPlatform`, `TestSecondSite`, `TestLegalPages` and three unit tests pass.
- Created the ticket and documents:
  - `docmgr vocab add --category topics --slug billing …` and `--slug frontend …`
  - `docmgr ticket create-ticket --ticket FRANCOIS-004 --title "Showcase storefront: creators, artifact pages, demos, subscription plans and an artifact editor" --topics backend,frontend,billing`
  - `docmgr doc add … --doc-type design-doc` and `--doc-type working-note`
  - nine tasks (design + eight phases).
- Wrote `design-doc/01-showcase-storefront-and-subscription-plans-analysis-design-and-implementation-guide.md`.
- Uploaded: `remarquee upload bundle <design doc> --name "FRANCOIS-004 Storefront and Plans Design" --remote-dir "/ai/2026/10/14/FRANCOIS-004" --toc-depth 2 --non-interactive` → `OK: uploaded FRANCOIS-004 Storefront and Plans Design.pdf -> /ai/2026/10/14/FRANCOIS-004`.

### Why

- Server-rendered public pages, because the SPA renders everything client-side and the storefront must be indexable and previewable as links.
- Plans in the settings document, because settings already have validation, history and an editor, and plans reference the approved models in the same document.
- A separate `/api/studio` namespace so the creator role and API tokens never touch user, billing or settings administration.
- Guest users for demos so the launch, grant and credit paths stay unchanged.

### What worked

- The existing test harness (`setup`, `user`, `do`, `upload` in `pkg/catalog/catalog_test.go`) creates users and sessions directly in the store, which makes role and plan tests cheap to write.
- `goldmark` is already in `go.sum` (`grep -c 'yuin/goldmark v1.8.2' go.sum` → 2), so Markdown needs no new module download.

### What didn't work

- No failures in this step.

### What I learned

- The inline CHECK constraints in `001_core.sql` have PostgreSQL's default names; verified in the local database with `select conname from pg_constraint where conname like '%check%'` → `users_role_check`, `instances_kind_check`, among others. Migration 004 can drop and recreate them by name.
- `releases.Validate` and `Blobs` are cleanly separated from HTTP, so media storage can reuse `Blobs` directly.
- `free_sample` only affects `download`, not running; worth keeping in mind when describing plan gating.
- The portal CSP (`routes.go:80`) forbids inline styles and scripts, so server-rendered templates must use only `/static/catalog.css` and external scripts.

### What was tricky to build

- Rollout ordering: the old binary parses settings strictly (`DisallowUnknownFields`), so it cannot save a revision with `plans`; the new binary drops `--stripe-price-id`. The design therefore prescribes deploying first, then saving plans immediately, and accepting a short window in which subscribers resolve to the free plan (only test users exist in production).

### What warrants a second pair of eyes

- The entitlement rule that staff (creator and admin) resolve to the highest plan, which changes today's behaviour where admins needed a grant to run workspaces.
- The downgrade rule (oldest workspaces up to the limit stay launchable).

### What should be done in the future

- Revenue split reporting; per-site storefronts; demo seed data; multi-file artifacts (§13 of the design).

### Code review instructions

- Start with sections 3 (orientation) and 5 (architecture) of the design doc.
- No code changed in this step.

### Technical details

- Ticket path: `ttmp/2026/10/14/FRANCOIS-004--showcase-storefront-creators-artifact-pages-demos-subscription-plans-and-an-artifact-editor/`.
- reMarkable: `/ai/2026/10/14/FRANCOIS-004/FRANCOIS-004 Storefront and Plans Design.pdf`.
