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

## Step 2: Phase 1 — content model

This step added the storage for everything the storefront shows: a published and a draft page document per artifact, creators and their credits, a media library, journal posts, and the columns later phases need (roles, grant plans, demo expiry, API tokens), all in one migration. It also added `pkg/catalog/content` for Markdown rendering and media type detection.

One design change came out of reading the existing code: the `description` column stays the short summary and is not moved into the page document. It already appears on cards and in the release form, and it applies immediately like the title. I updated the design doc (§5.2, §5.7, §5.11, §7.4) to match.

### Prompt Context

**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement Phase 1 of the design.
**Inferred user intent:** A tested data layer the public pages and studio can build on.
**Commit (code):** 19ba56b — "Storefront content model: page drafts, creators, media, posts"

### What I did

- Moved the work to branch `feat/storefront` (`git checkout -b feat/storefront && git branch -f main origin/main`), because pushes to `main` build an image and open a deploy PR in the k3s repo.
- `pkg/catalog/store/migrations/004_storefront.sql`: role and instance-kind CHECK constraints replaced; `catalog_artifacts.page/page_draft/featured/sort_order/required_plan/demo_enabled`; tables `catalog_media`, `creators`, `artifact_creators`, `posts`, `api_tokens`; `access_grants.plan`; `instances.expires_at`.
- `pkg/catalog/store/releases.go`: `Artifact` gains `Page`, `PageDraft`, `Featured`, `SortOrder`, `RequiredPlan`, `DemoEnabled`; `UpsertArtifact` replaced by `CreateArtifact` (conflict on existing ID) and `UpdateArtifact` (immediate settings only); `SavePageDraft`, `PublishPageDraft`, `DiscardPageDraft`; `Artifacts` ordered featured, sort order, update time.
- `pkg/catalog/store/content.go`: media (`CreateMedia`, `Media`, `MediaByIDs`, `MediaList`, `SetMediaAlt`, `MediaReferenced`, `DeleteMedia`), creators (`CreateCreator`, `UpdateCreator`, `DeleteCreator`, `Creator`, `CreatorBySlug`, `CreatorForUser`, `Creators`, `SetArtifactCreators`, `ArtifactCreators`, `CreatorArtifacts`, `IsArtifactCreator`), posts (`CreatePost`, `UpdatePost`, `SetPostState`, `DeletePost`, `Post`, `PostBySlug`, `Posts`).
- `pkg/catalog/releases/releases.go`: `Publisher.prepare` fills the title from an existing artifact; `Publisher.Check` (used by `adminCheck`); `CreateDraft` creates the artifact only when missing.
- `pkg/catalog/content/markdown.go`: goldmark with GFM and heading IDs, no raw HTML, custom image renderer (images only under `/media/<id>`, others become links, dangerous URLs dropped), `Excerpt`.
- `pkg/catalog/content/media.go`: `DetectMedia` (PNG, JPEG, GIF, WebP, MP4, WebM; dimensions for PNG/JPEG/GIF), `IsVideo`.
- Tests: `pkg/catalog/content/content_test.go` (Markdown cases including script, HTML blocks, `javascript:` links and images, alt escaping, nested `/media/a/../..` paths; media detection including SVG and broken PNG); `pkg/catalog/storefront_test.go` `TestContentStore`.
- `go mod tidy` made goldmark a direct dependency.

### Why

- One migration for all phases keeps the schema change reviewable in one place and lets later phases focus on behaviour.
- `CreateDraft` used to upsert title, description and tags on every upload, which would silently undo edits made in the studio editor.
- `MediaReferenced` also searches Markdown bodies for `/media/<id>`, because images inserted into text are references too.

### What worked

- `go test ./pkg/catalog/content/` → `ok`.
- `TEST_DATABASE_URL=… go test -count=1 ./pkg/catalog/` → `ok` (all existing tests plus `TestContentStore`).
- `golangci-lint run ./pkg/catalog/...` → `0 issues.`

### What didn't work

- `go build ./...` after the store change: `pkg/catalog/releases/releases.go:246:22: p.Store.UpsertArtifact undefined` and then `pkg/catalog/admin.go:292:22: s.Store.UpsertArtifact undefined`. Fixed by the `prepare`/`CreateArtifact` change and by switching `adminArtifact` to `UpdateArtifact` (that handler moves to the studio in Phase 3).
- My first version of the creator column list used a small tokenizer (`prefixed`) to add a table alias to a column list containing `COALESCE(...)`. It worked but was needlessly clever; replaced with `creatorCols(alias)`.
- The editor's diagnostics (gopls) reported many unrelated errors (`log already declared`, `undefined: Site`) during the edit; they were stale metadata. `go build` and `go vet` were clean.

### What I learned

- `http.DetectContentType` recognises WebP, MP4 and WebM signatures, so no extra library is needed for type detection; only WebP dimensions would need `golang.org/x/image`, which the design allows to skip.
- goldmark's `html.IsDangerousURL` is exported and reusable in a custom image renderer.

### What was tricky to build

- Image rendering: goldmark's default renderer would emit `<img src="https://…">`, which the portal CSP blocks and which leaks visitor IPs to third parties. A prioritized node renderer (`util.Prioritized(imageRenderer{}, 100)`) replaces the image renderer only; it accepts `/media/<id>` with no further `/`, `?`, `#` or `\` so a path such as `/media/a/../../admin` is not treated as media.

### What warrants a second pair of eyes

- `MediaReferenced` uses `LIKE '%/media/<id>%'` over bodies; IDs are random hex so false positives are negligible, but it is a sequential scan (fine at storefront sizes).
- The migration drops and recreates CHECK constraints by their default names; verified on the local database (`users_role_check`, `instances_kind_check`).

### What should be done in the future

- N/A for this phase.

### Code review instructions

- Start with `pkg/catalog/store/migrations/004_storefront.sql`, then `store/releases.go` (artifact section) and `store/content.go`.
- `pkg/catalog/content/markdown.go` `renderImage` is the security-relevant part.
- Validate: `go test ./pkg/catalog/content/` and `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable' go test -count=1 ./pkg/catalog/`.

### Technical details

- Page document JSON: `{"body": "...", "cover": "med_…", "gallery": ["med_…"]}`; `gallery` is always an array after scanning.
- Post state change keeps the first `published_at` when republishing (`COALESCE(published_at, now())`).
