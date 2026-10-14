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

## Step 3: Phase 2 — server-rendered storefront pages

This step made the storefront visible. The home page, catalog, artifact pages, creator pages, journal, single posts and an RSS feed are now rendered on the server with `html/template`, and media is served from `/media/{id}`. A 2 KB script (`/static/public.js`) adds the buttons that need a session: run, try the demo (wired for Phase 6) and sign out. The SPA keeps only the signed-in areas; its catalog and artifact components are gone.

I checked the pages in a browser against a local server seeded with two creators, two artifacts with covers and write-ups, and a post. Layout, Markdown, gallery and the phone-width layout looked right; there were no console messages.

### Prompt Context

**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement Phase 2 of the design.
**Inferred user intent:** Pages a visitor (and a search engine) can read without signing in.
**Commit (code):** ab1598d — "Server-rendered storefront: home, catalog, artifact, creator, journal, RSS, media"

### What I did

- `pkg/catalog/web/templates/`: `layout.tmpl` (head with Open Graph, header nav, footer, shared `cards`, `posts` and `media` blocks), `home.tmpl`, `catalog.tmpl`, `artifact.tmpl`, `creator.tmpl`, `journal.tmpl`, `post.tmpl`, `notfound.tmpl`; embedded as `web.Templates`.
- `pkg/catalog/pages.go`: `parsePages` (layout + one page per template set), `render` (nav from the principal, `Cache-Control: no-cache`, `Vary: Cookie`), `cards`, `homePage`, `catalogPage` (tag filter), `artifactPage` (`?draft=1` for editors), `creatorPage`, `journalPage`, `postPage` (drafts for their author), `journalFeed` (RSS 2.0), `media` (`http.ServeContent` with sandbox CSP and immutable caching), `canEditArtifact`, `canEditPost`, `releaseCaps`.
- `pkg/catalog/routes.go`: SPA shell only for `/account`, `/instances/`, `/studio`, `/admin`; new storefront routes; CSP `media-src 'self'`; `/api/catalog` adds `cover`, `creators`, `featured`, `requiredPlan`, `demo`, `url`.
- `pkg/catalog/store/users.go`: `SetUserRole` accepts `creator`.
- `ui/catalog/public.ts` (+ `scripts/build.mjs` entry, `web.PublicJS`); `ui/catalog/catalog.tsx` deleted; `main.tsx` routes; `ui.tsx` `Link` navigates normally outside SPA routes, nav gains Journal and Studio.
- `pkg/catalog/web/static/catalog.css`: covers, `.md` prose, gallery, creator cards, posts, draft banner.
- Test `TestStorefrontPages`; test helper `user()` now applies any role.
- Seed script `scripts/01-seed-dev-storefront.sh` (local database `storefront_dev`, release dir `var/storefront/releases`); dev server `francois catalog serve --listen 127.0.0.1:8095 --public-url http://localhost:8095 --backend fake`.

### Why

- Templates are parsed per page because every page defines `content`; one shared set would make the last definition win.
- `render` buffers the output so a template error returns a clean 503 instead of half a page.
- `/apps/{id}` keeps the URL the SPA used, so existing links stay valid.

### What worked

- `TEST_DATABASE_URL=… go test -count=1 ./pkg/catalog/...` → all `ok`; `pnpm check` clean; `golangci-lint run ./pkg/catalog/...` → `0 issues.`
- Browser: home, artifact and post pages render as intended at 1280 px and 390 px; console has 0 messages.

### What didn't work

- `go build` failed with `pkg/catalog/web/embed.go:16:12: pattern static/public.js: no matching files found` until the new bundle was built (`pnpm build`).
- `TestStorefrontPages` first failed at `storefront_test.go:222: editor draft preview` because the test helper only applied the `admin` role; the `creator` user was a subscriber. Fixed the helper and `SetUserRole`, which rejected `creator`.
- The seed script exited silently after publishing: `read -r w h < <(identify -format '%w %h' …)` returns non-zero because `identify` prints no trailing newline, and `set -e` stopped the script. Replaced with two `$(identify …)` calls. Re-runs failed on `version 1.0.0 already exists for polis: conflict`; publishing is now allowed to fail in the seed.
- The seeded post showed literal `\n`: the SQL string was not an `E''` literal. Seed-only issue, fixed.
- Playwright refused to save screenshots in the scratchpad (`outside allowed roots`); saved under `.playwright-mcp/`, which is git-ignored.

### What I learned

- The portal's global CSP wrapper sets headers before the handler runs, so the media handler can override `Content-Security-Policy` per response.
- `html/template` already neutralises `javascript:` in `href` from creator links (`#ZgotmplZ`), but the studio API should still validate link URLs (Phase 3).

### What was tricky to build

- Public pages read the session cookie through `guard(public, …)` to render the right header, which makes every page personalised. `Cache-Control: no-cache` plus `Vary: Cookie` keeps shared caches from serving one visitor's header to another.
- Draft previews: `?draft=1` alone must reveal nothing. The handler computes `draft` only when the caller can edit the artifact; otherwise hidden and unpublished artifacts return the same 404 as unknown IDs.

### What warrants a second pair of eyes

- `public.ts` shows "Sign in to run" even when sign-in is not configured (local dev only).
- `media` reads the whole blob into memory per request (and verifies its hash); acceptable for images and short videos at the 25 MiB limit planned in Phase 5.

### What should be done in the future

- Storefront title, tagline and intro from settings (Phase 5); plan badge and demo button (Phases 5 and 6).

### Code review instructions

- `pkg/catalog/pages.go` (`artifactPage`, `postPage`, `media`), `pkg/catalog/web/templates/layout.tmpl`, `ui/catalog/public.ts`.
- Validate with the test command above and the seed script plus dev server for a visual check.

### Technical details

- Storefront routes: `GET /{$}`, `/apps`, `/apps/{$}`, `/apps/{id}`, `/creators/{slug}`, `/journal`, `/journal/feed.xml`, `/journal/{slug}`, `/media/{id}`.
- Media headers: `Content-Type` from detection, `Cache-Control: public, max-age=31536000, immutable`, `Content-Security-Policy: default-src 'none'; sandbox`, `X-Content-Type-Options: nosniff` (global).

## Step 4: Phase 3 — studio API and the creator role

This step gave creators a way to manage their own content. A new guard level `creator` (role `creator` or `admin`) protects `/api/studio/*`, which covers artifacts and page drafts, releases, media, posts, creator profiles and a Markdown preview. Which objects a creator may touch is decided by two rules from Phase 2: `canEditArtifact` (artifacts credited to the creator profile linked to their account) and `canEditPost` (posts they created or that are authored as their profile). The release endpoints moved here from `/api/admin`, and preview instances now authorize the artifact's editors instead of administrators only.

### Prompt Context

**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement Phase 3 of the design.
**Inferred user intent:** Ramon (and Manuel) can publish their own work without full admin rights.
**Commit (code):** 68a7daf — "Studio API with a creator role; releases move out of the admin API"

### What I did

- `pkg/catalog/auth.go`: level `creator` between `enabled` and `admin`; error `creator_required`.
- `pkg/catalog/studio.go`: `studioRoutes` and handlers; helpers `editable`, `ownProfile`, `checkCredits`, `normalizeTags`, `checkMedia`, `releaseItems`, `mayUpload`, `releaseFor`, `ownMedia`, `applyPost`, `editablePost`, `applyCreator`.
- `pkg/catalog/admin.go`: removed `adminReleases`, `adminCheck`, `adminUpload`, `adminPublish`, `adminWithdraw`, `adminPreview`, `adminDeleteDraft`, `adminArtifact` and their routes; `upload` (multipart parsing) stays shared.
- `pkg/catalog/server.go`: `authorizeInstance` loads the release first and allows `preview` instances for `canEditArtifact`.
- `pkg/catalog/releases/releases.go`: exported `ValidArtifactID`.
- `ui/catalog/api.ts`: release and artifact endpoints point to `/api/studio`.
- Tests: `pkg/catalog/studio_test.go` `TestStudio` (roles, profiles, ownership, credits, releases, preview launch, media type and in-use rules, drafts, posts, Markdown, admin override); `TestHostedPlatform` expects `creator_required` for a subscriber upload.

### Why

- Ownership goes through creator profiles rather than a per-artifact owner column, because public credits and edit rights should be the same list. A creator who could remove their own credit would lock themselves out, so `checkCredits` refuses that for non-admins.
- Creating artifacts needs a linked profile so that every creator-created artifact has at least one editor.
- Only administrators create profiles and link accounts: linking grants edit rights.

### What worked

- `TEST_DATABASE_URL=… go test -count=1 ./pkg/catalog/...` → all `ok`; `golangci-lint` → `0 issues.`; `pnpm check` clean.

### What didn't work

- First run of `TestStudio`: `pkg/catalog/studio_test.go:75:13: assignment mismatch: 2 variables but e.do returns 3 values` (test code), fixed.
- `studio_test.go:114: new artifact by upload: 422 … Title is required` — correct behaviour (a new artifact needs a title); the test now checks both cases.
- `studio_test.go:176: markdown … <h1 id="hi-scriptxscript">Hi <!-- raw HTML omitted -->x<!-- raw HTML omitted --></h1>` — the output is safe; goldmark derives heading IDs from source text including the omitted tag names. Loosened the assertion.
- Lint: `nonamedreturns` on `mayUpload` and a `gofmt` finding; fixed.

### What I learned

- goldmark replaces raw HTML with `<!-- raw HTML omitted -->`, which is visible in page source but harmless.

### What was tricky to build

- Releases for new versus existing artifacts: `mayUpload` must check edit rights for an existing artifact but profile existence for a new one, and the new artifact must be credited after `CreateDraft` succeeds. Admins without a profile may create artifacts that then have no credits; only admins can edit those until credits are added.

### What warrants a second pair of eyes

- The admin UI's Releases tab still calls `GET /api/studio/releases`, which now returns a plain list instead of `{artifacts, releases}`. The tab is replaced in Phase 4; on this branch it is temporarily broken.
- Media is a shared library: every creator can list and reference all media, but only the uploader or an admin can change or delete it.

### What should be done in the future

- Validate `requiredPlan` against the configured plans once plans exist (Phase 5); today only the key format is checked.

### Code review instructions

- `pkg/catalog/studio.go` top to bottom; `pkg/catalog/pages.go` `canEditArtifact`/`canEditPost`; `pkg/catalog/server.go` `authorizeInstance`.
- `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable' go test -count=1 -run 'TestStudio|TestHostedPlatform' ./pkg/catalog/`.

### Technical details

- Error codes added: `creator_required`, `creator_profile_required`, `not_editor`, `artifact_exists`, `no_draft`, `media_type` (415), `media_in_use`, `not_uploader`, `slug_taken`.
- Limits: write-up and post body 100 KB, gallery 24, media 25 MB, alt 300, tags 12 × 32, creator links 8 (http/https only).

## Step 5: Phase 4 — studio UI

This step built the screens creators use every day: an artifact list scoped to what the user may edit, an editor with Details, Page, Releases and Demo tabs, a journal editor, creator profiles and a media library. I tested every main flow in a browser against the local server, signed in as the seeded creator "Manuel": editing and publishing a write-up, uploading a release and opening its preview instance, uploading media, and creating and publishing a post.

Two bugs surfaced only in the browser, and both had the same root cause: `ui/catalog/api.ts` still defined the old admin release endpoints, and RTK Query's `injectEndpoints` silently keeps an existing endpoint with the same name. My studio `uploadRelease` never ran.

### Prompt Context

**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement Phase 4 of the design.
**Inferred user intent:** A usable authoring workflow (upload, configure, preview, publish) for the two creators.
**Commit (code):** 5f3da56 — "Studio UI: artifact editor, journal, creators and media library"

### What I did

- `ui/catalog/studio-api.ts`: studio endpoints injected into the shared API (`enhanceEndpoints({addTagTypes})` + `injectEndpoints`), types `StudioArtifact`, `ArtifactRow`, `Media`, `Creator`, `Post`, `Page`.
- `ui/catalog/studio.tsx`: `Studio` (role gate, sections), `Artifacts`, `ArtifactEditor` (tabs via `?tab=`), `Details`, `PageEditor` with `MarkdownField` (debounced server preview), `MediaPicker`, `MediaThumb`, `Uploader`, `MediaLibrary`, `Releases`, `Demo`, `Journal`, `PostEditor`/`PostForm`, `Creators`/`CreatorForm`.
- `ui/catalog/ui.tsx`: `useAct` moved here from `admin.tsx`.
- `ui/catalog/admin.tsx`: Releases tab removed; role is now a select (subscriber, creator, administrator); overview links to the studio.
- `ui/catalog/api.ts`: removed `adminReleases`, `checkUpload`, `uploadRelease`, `publishRelease`, `withdrawRelease`, `deleteDraft`, `previewRelease`, `updateArtifact`.
- `ui/catalog/instances.tsx`: preview label "draft preview".
- `pkg/catalog/web/static/catalog.css`: editor tabs, two-column Markdown editor, media grid and picker.
- `scripts/02-dev-session.sh`: inserts a portal session for a local dev user and prints the cookie value (the local server has no OIDC).

### Why

- Cover and gallery pickers live in the Page tab and the media library is studio-wide, because media is shared between artifacts, posts and profiles; the design doc §5.11 now says so.
- The Markdown preview calls the server renderer instead of a client library so the preview is byte-for-byte what visitors get.

### What worked

- Browser checks (Playwright against `http://localhost:8095`, creator session):
  - Page tab: typing a heading, **Save draft** → public page unchanged, `?draft=1` shows it; **Publish page** → public page shows it; toast "Page published".
  - Releases tab: upload `examples/polis.html` as `1.6.0` → row appears; **Preview** → `/instances/ins_…` with frame `http://i-….apps.localhost:8095/launch`, and Polis renders (screenshot).
  - Media: upload → 4 → 5 tiles, alt text derived from the file name.
  - Journal: new post, body, **Save and publish** → `/journal/designing-the-studio` returns 200.
- `pnpm check`, `pnpm build`, `go test ./pkg/catalog/` pass.

### What didn't work

- `/studio` showed "The studio is for creators." with a valid session: `document.cookie = "francois_session_dev=…"` did nothing because an older HttpOnly cookie with the same name (from the compose stack on `localhost:8090`; cookies are not port-specific) blocked script writes. Listing cookies showed `["mode","zitadel.useragent(httpOnly)","francois_session_dev(httpOnly)"]`; setting the cookie through Playwright's `context.addCookies` worked.
- The Markdown preview pane looked empty in the first screenshot; it was the 300 ms debounce plus the request. Reading `.md-preview` innerHTML a moment later showed the rendered HTML.
- Release upload returned `400 {"error":{"code":"invalid_upload","message":"upload exceeds the size limit or is malformed: request Content-Type isn't multipart/form-data"}}`; the request had `content-type: application/json` and a 30-byte body. After passing the `FormData` directly, the release was created but the list never refreshed (no GET after the POST; Details saves did refetch). Both symptoms came from the old `api.ts` `uploadRelease` endpoint shadowing the injected one. Removing the dead admin endpoints fixed both.

### What I learned

- RTK Query `injectEndpoints` does not override an existing endpoint unless `overrideExisting` is set, and gives no error; endpoint names must be unique across injected modules.
- Full-page screenshots draw sticky elements at their position in the viewport at capture time, which made the sticky form footer look misplaced.

### What was tricky to build

- Page editing state: the editor initialises from the draft if one exists, otherwise from the published page, and "Publish page" first saves unsaved changes so publishing never publishes a stale draft.

### What warrants a second pair of eyes

- `MarkdownField` uses `dangerouslySetInnerHTML` for the server-rendered preview. The HTML comes from the same renderer as public pages (no raw HTML, media-only images), on the portal origin.
- Linking a creator profile to an account takes a raw user ID in the form; an admin user search would be friendlier.

### What should be done in the future

- A user picker for linking profiles; a plan select for "Required plan" once plans exist (Phase 5).

### Code review instructions

- `ui/catalog/studio.tsx` (`PageEditor`, `Releases`), `ui/catalog/studio-api.ts`.
- Local check: run `scripts/01-seed-dev-storefront.sh`, start the dev server, create a session with `scripts/02-dev-session.sh storefront_dev usr_dev_manuel` and set it as `francois_session_dev` for `localhost`.

### Technical details

- Dev server: `francois catalog serve --database-url postgres://francois:francois@127.0.0.1:5434/storefront_dev?sslmode=disable --listen 127.0.0.1:8095 --public-url http://localhost:8095 --app-domain apps.localhost --release-dir var/storefront/releases --worker-dir var/storefront/worker --backend fake`.

## Step 6: Phase 5 — plans and entitlements

This step replaced the yes/no access check with plans. The settings document now defines plans: rank, Stripe prices, instances, people per instance, monthly and daily credits, and allowed models. A pure resolver (`resolveEntitlement`) turns an account, its billing decision and the settings into one plan. Every check asks it: creating, launching and running workspaces, sharing, downloads and model calls. On the UI side there is a pricing page, the account page shows the plan, the admin settings edit plans, demo limits and the storefront wording, grants can name a plan, and the studio picks an artifact's required plan.

The most important discovery was a rollout hazard. Stored settings revisions are parsed strictly, so the production revision in the old shape would have failed to parse after deployment, and every request needing settings would have returned 503. Migration `005_plans.sql` rewrites stored revisions, and turns the existing subscription prices into the paid plan's prices so current subscribers keep their access.

### Prompt Context

**User prompt (verbatim):** see Step 1.
**Assistant interpretation:** Implement Phase 5 of the design.
**Inferred user intent:** Several subscription tiers that actually gate what subscribers can run and use.
**Commit (code):** 0726015 — "Plans and entitlements: plan-gated runs, limits, credits and checkout"; 0ef0265 — "Pricing page, plan-aware storefront and plan editing"

### What I did

- `pkg/catalog/settings/settings.go`: `Plan`, `PlanPrice`, `Demo`, `Storefront`; credit and per-user limits removed from `Allowance`/`Limits`; `Limits.MaxMediaBytes`; validation (`validatePlans`, `validateDemo`); `Defaults` with only the free plan; helpers `Plan`, `Free`, `Highest`, `PlanForPrice`, `Prices`, `RequiredRank`.
- `pkg/catalog/store/migrations/005_plans.sql`: rewrites stored revisions (free + subscriber plans, prices from `billing_subscriptions`).
- `pkg/catalog/billing/policy.go`: `Decision.PriceID`, `Decision.Plan`, `Comp.Plan`; `store.GrantAccess(…, plan)`, `access_grants.plan` read back.
- `pkg/catalog/entitlement.go`: `Entitlement`, `resolveEntitlement`, `CanRun`, `AllowsModel`, `Paid`, `policyFor`, `entitlementIn`, `entitlement`, and `budget` (the runtime's `Budgeter`).
- `pkg/catalog/hosted`: `Budget`/`Budgeter` passed to `hosted.New`; `sample` checks plan models, the shared demo budget, and reserves with plan credits or the per-demo period.
- `pkg/catalog/store`: `Instance.ExpiresAt`, `WorkspacePosition`, `ReserveRequest.Period`, `DemoCreditsToday`.
- `pkg/catalog/server.go` (`authorizeInstance`: editors for previews, owner and expiry for demos, plan rank and the oldest-first workspace limit for workspaces; payments `OnChange` revokes on losing paid access), `instances.go` (`createInstance`, `addMember`), `routes.go` (session/account report the plan, downloads use `CanRun`, checkout `{plan, interval}` via `checkoutPrice`), `admin.go` (plan in user list/detail, plan grants, billing shows plan prices, adjustments against the plan allowance), `pages.go` (`pricingPage`, `publicPlans`, storefront wording, plan badge), `studio.go` (`studioPlans`, required-plan validation).
- `pkg/catalog/payments/service.go`: no configured price; `Checkout(ctx, user, price, accepted, portal)`.
- `cmd/francois/cmds/catalog.go`: `--stripe-price-id` removed; `grant-access --plan`.
- UI: `pricing.tmpl`, `public.ts` (plan-aware run button, Checkout or billing portal from pricing), `account.tsx`, `admin.tsx` (`PlansEditor`, `PriceField`, `NewPlan`, `ModelPicker`, demo and storefront rows, plan grants), `studio.tsx` (plan select), `api.ts` types.
- Tests: `entitlement_test.go` (resolver table, `CanRun`, models, `checkoutPrice`), `settings/settings_test.go` (plan validation, old shape rejected), `billing/policy_test.go` (`TestDecisionNamesPriceAndPlan`), `plans_test.go` `TestPlans` (rank gating, instance and member limits, downloads, downgrade and upgrade, plan grants, pricing page, public plans without price IDs, plan badge). The test harness saves a revision with a `pro` plan.
- `scripts/03-parse-settings/main.go`: parses a document with the strict parser, used to verify migration 005.

### Why

- Plans live in settings, so model approvals and plans validate together and every change is a revision with history.
- Demo calls use a separate ledger period (`demo:<instance>`): `Reserve` upserts the period's `included` value per user, so reusing the monthly period would have overwritten a subscriber's monthly allowance with the small demo allowance.
- Staff (creator, admin) resolve to the highest plan and run every artifact, so the two creators can always test their work.

### What worked

- Migration check: a database at migration 004 with an old-shaped revision and an active subscription on `price_1UPxdFLeHmcfmvuCg0zsScj5`, then `005_plans.sql`, then the strict parser → `plan free rank=0 …` and `plan subscriber rank=1 prices=[{month price_1UPxdFLeHmcfmvuCg0zsScj5 }] instances=10 members=10 credits=2000/200`.
- `TestPlans` passed on its first run; all `pkg/catalog/...` packages pass; `golangci-lint` `0 issues.`
- Browser: the admin settings editor added a "Supporter" plan with two prices and the storefront title "slop.boutique" → "Settings revision 1 is active."; `/pricing` rendered both plans; the anonymous HTML shows `€5 / month` and `€50 / year` buttons.

### What didn't work

- `go build` listed every caller of the removed fields (`rev.Document.Allowance.MonthlyCredits undefined`, `s.Config.PriceID undefined`, `not enough arguments in call to s.Store.GrantAccess`, …); fixed call site by call site.
- The migration test first failed with `command not found: docker exec -i francois-catalog-pg psql …`: zsh does not word-split a command stored in a variable; used a shell function.
- `TestHostedPlatform` expected `subscription_required`; the code is now `plan_required` (intended).
- `TestStudio` failed with `Unknown plan studio` after adding required-plan validation; the test now uses the configured `pro` plan and checks the unknown case.
- `settings_test.go:18:23: cannot call pointer method Validate on Document` (calling on a function result); fixed, and removed a dead `if err := error(nil)` block my first fix left behind.

### What I learned

- `Validate` has a pointer receiver because it normalises nil slices; tests must call it on an addressable value.

### What was tricky to build

- The downgrade rule needs a stable order: `WorkspacePosition` counts the owner's workspaces with `(created_at, id) <= (inst.created_at, inst.id)`, so ties on timestamps cannot make two instances share a position.
- Signed-in visitors' demos and the monthly ledger (see Why).

### What warrants a second pair of eyes

- `resolveEntitlement` treats a subscription whose price is no longer in any plan as free; removing a price from settings therefore immediately removes paid access for those subscribers.
- Staff entitlement skips the workspace limit.
- Migration 005 rewrites historic revisions as well as the active one.

### What should be done in the future

- Configure the Stripe billing portal (products and prices) so subscribers can switch plans there.
- Filter the runtime's model list (`rt.models`) by plan, so applications only offer models the plan allows.

### Code review instructions

- `pkg/catalog/entitlement.go`, `pkg/catalog/server.go` `authorizeInstance`, `pkg/catalog/settings/settings.go` `validatePlans`, `pkg/catalog/store/migrations/005_plans.sql`.
- `go test ./pkg/catalog/settings/ ./pkg/catalog/billing/` and `TEST_DATABASE_URL=… go test -count=1 -run 'TestPlans|TestResolve|TestCanRun|TestCheckoutPrice|TestHostedPlatform' ./pkg/catalog/`.

### Technical details

- New error codes: `plan_required`, `over_plan_limit`, `model_not_in_plan`, `demo_limit`, `guest`.
- Session `plan`: `{key, name, rank, source, paid, instances}`; account `entitlement`: `{plan, source, decision}`.
