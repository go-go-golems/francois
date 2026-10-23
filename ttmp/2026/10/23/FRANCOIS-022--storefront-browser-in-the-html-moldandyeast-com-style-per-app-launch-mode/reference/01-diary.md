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

## Step 1: Site copy foundation (Phase 1)

Built the CMS-like mechanism first because the landing page, the early-access message and the runner labels all read from it. Site text is a registry of keys in Go with defaults; staff overrides live in the versioned settings document under `storefront.copy`. The header statement and the support callout now come from it.

### Prompt Context
**User prompt (verbatim):** "implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
**Earlier decisions (verbatim):** "hero: different sentence, the current header stays at the top. Use the html.moldandyeast copy for now, but make it customizable in the backend (I don't know if we have a CMS or CMS like page, but we should make one, for differnet other more hardcoded stuff too)" — later: "also, the big bold text should actually say (not the html.moldandyeast copy): These artifacts are the results of our research. Play with them, download the HTML, and drop them into your coding agent." — and: "when something is in early access and not accessible by the user, tell them they can subscribe. Also make the launch button blue to make it more prominent (using design token)."
**Assistant interpretation:** Implement the FRANCOIS-022 guide phase by phase with commits and a diary.
**Inferred user intent:** Ship the new storefront browser and launch modes with an editable text layer, reviewable step by step.
**Commit (code):** d8855d3 — "feat(catalog): editable site copy in the settings document"

### What I did
- `pkg/catalog/settings/copy.go`: `CopyKey`, `CopyKeys` (13 keys: header.statement, home.hero, home.search, callout.*, runner.*, earlyaccess.*), `CopyKeyByName`, `Document.CopyText`, `Document.SetCopy`, `validateCopy`.
- `settings.go`: `Storefront.Copy map[string]string` (`omitempty`), validated in `Document.Validate`.
- `pages.go`: `pageData.Copy siteCopy` with `Text`, `HTML` (Markdown via `s.md.Render`) and `Date` (fills `{date}`); set in `render` and, with defaults, in `unavailable` (which renders without the settings). Landing page description now falls back to the statement instead of a hard-coded string.
- `layout.tmpl`: statement and callout read copy keys; the callout body is a `div.callout-body` holding rendered Markdown.
- Admin: `GET /api/admin/settings` returns `copyKeys`; `admin.tsx` `SiteCopy` panel (one field per key, default as placeholder, counter, clearing restores default); `.site-copy` CSS.
- CLI: `francois catalog settings copy` (list with values) and `settings set-copy <key> --text|--file|--reset` via `Admin.EditSettings` (retries on settings conflicts).
- MCP: `set_storefront_text` accepts `copy` (empty value restores the default).
- Tests: `settings.TestCopy`, `catalog.TestSiteCopy`.

### Why
- Settings already have revisions, If-Match concurrency, an admin screen, CLI and MCP; a separate CMS table would duplicate all of that for a handful of strings.
- A registry instead of free-form keys: typos fail validation, the editor can list every key with its default, and templates cannot reference text that has no default.
- I returned the registry in the existing settings response instead of the separate `GET /api/admin/copy-keys` sketched in the guide: one request, same data.

### What worked
- `go test ./pkg/catalog/settings` ok; `TEST_DATABASE_URL=… go test ./pkg/catalog -count=1` ok (34 s); `go test ./cmd/... ./pkg/catalog/mcpserver` ok; `pnpm check`, `pnpm test` (18 pass); `golangci-lint run ./...` 0 issues.

### What didn't work
- First `TestSiteCopy` used `/pricing`, which has no support callout: `layout_test.go:47: default copy missing "<h2 id=\"support-heading\">Keep the lab running.</h2>"`. Switched to `/`.
- `settings_test.go:96:12: undefined: json` — missing import in the new test.
- golangci-lint: `predeclared` — the parameter `copy` shadowed the builtin; renamed to `overrides`.

### What I learned
- `pageFailed`/`unavailable` render the layout without loading settings, so anything the layout calls must work with a zero-value or defaults-backed `siteCopy` (a nil Markdown renderer would panic).
- Production settings have empty title/tagline/intro; `storefront.intro` is not rendered anywhere.

### What was tricky to build
- Template scope: inside `{{with .Body}}` the copy is reached through `$.Copy`; partials like `support-callout` receive the page data, so `.Copy` works there.

### What warrants a second pair of eyes
- `CopyText` trims and treats whitespace-only overrides as "use default"; `SetCopy("")` deletes. Confirm that is the desired editor behaviour.
- The landing page meta description is now `tagline` if set, else the statement (before: always the hard-coded statement).

### What should be done in the future
- Move pricing and sign-in page copy into keys when someone wants to edit them.

### Code review instructions
- Start at `pkg/catalog/settings/copy.go`, then `pages.go` (`siteCopy`, `render`, `unavailable`), `layout.tmpl`, `admin.tsx` (`SiteCopy`), `api_commands.go` (`settings copy|set-copy`).
- Validate: `go test ./pkg/catalog/settings`, `TEST_DATABASE_URL=… go test ./pkg/catalog -run 'TestSiteCopy|TestFinalLayout'`.

### Technical details
- Stored shape: `{"storefront": {"copy": {"home.hero": "…"}}}`; omitted when empty, so existing documents round-trip unchanged.
- `{date}` placeholder is formatted `2 January 2006` in UTC.

## Step 2: Launch mode field end to end (Phase 2)

Added the per-artifact `launchMode` setting through every layer, with `tab` as the default so the live site does not change until someone switches an app. This step is data only: nothing reads the mode on the public pages yet (Phase 3/4).

### Prompt Context
**User prompt (verbatim):** see Step 1. Mode decision (verbatim): "1. two modes".
**Assistant interpretation:** Store and edit `inline | tab` per artifact.
**Inferred user intent:** Choose per app whether it starts in the page or full screen in a tab.
**Commit (code):** 1094837 — "feat(catalog): per-artifact launch mode (inline or tab)"

### What I did
- `store/migrations/020_launch_mode.sql`: `launch_mode text NOT NULL DEFAULT 'tab' CHECK (launch_mode IN ('inline','tab'))`.
- `store/releases.go`: `Artifact.LaunchMode`, constants `LaunchInline`/`LaunchTab`, `CreateArtifact` default, `UpdateArtifact`, `artifactColumns`, `scanArtifact`.
- `studio.go`: PATCH accepts `launchMode` (400 `invalid_request` otherwise), audit includes it, `artifactSummary` returns it. `routes.go` `/api/catalog` returns it.
- `client/types.go` (`Artifact`, `ArtifactRow`, `ArtifactPatch`), CLI `artifacts update --launch-mode`, MCP `update_artifact.launchMode`.
- `studio-api.ts` `LaunchMode` type; `studio.tsx` Details "Launch" select with help text.
- Tests: new `launch_mode_test.go` `TestLaunchMode`; `TestContentStore` round-trips the field.

### Why
- Default `tab` = today's behaviour (cover, then start); flipping per app is an explicit editorial act.
- A text column with a CHECK constraint rather than a boolean: readable in SQL and the audit log, and room for a third mode later without another migration of meaning.

### What worked
- `go test ./pkg/catalog/... -count=1` (with `TEST_DATABASE_URL`) ok; `golangci-lint` 0 issues; `pnpm check`; `pnpm test` 18 pass.

### What didn't work
- Full suite after Phase 1: `--- FAIL: TestClientContract … client_contract_test.go:186: settings: json: unknown field "copyKeys"`. The Go client decodes the settings response strictly; Phase 1 (d8855d3) added `copyKeys` to the API without the client type, so that commit fails `TestClientContract` on its own. Fixed in 1094837 by adding `CopyKeys []settings.CopyKey` to `client.Settings`.
- Prettier flagged `ui/catalog/studio.tsx`; ran `prettier --write`.

### What I learned
- Run the whole `./pkg/catalog/...` suite (not only the touched tests) after changing any API response shape: the contract test decodes with `DisallowUnknownFields`.

### What was tricky to build
- Nothing beyond keeping the ten positional SQL parameters aligned in `CreateArtifact` and `UpdateArtifact`.

### What warrants a second pair of eyes
- Migration on production: `ADD COLUMN … DEFAULT 'tab' NOT NULL` is a metadata-only change in PostgreSQL 11+, so it is instant on the small table.

### What should be done in the future
- After deploy, set modes per app with `francois catalog artifacts update <id> --launch-mode inline|tab`.

### Code review instructions
- `store/releases.go` (columns and scan order), `studio.go` `studioPatchArtifact`, `launch_mode_test.go`.

### Technical details
- API: `PATCH /api/studio/artifacts/{id} {"launchMode": "inline"}` → artifact JSON with `"launchMode": "inline"`; `GET /api/catalog` rows carry `launchMode`.

## Step 3: Landing page browser, server side (Phase 3)

Rebuilt `/` as the html.moldandyeast.com-style browser while leaving the header, support callout and footer alone. Everything works without JavaScript: search, sort and tags are a GET form and links; **Open** goes to the app page until Phase 4 adds the viewer and tab launch.

### Prompt Context
**User prompt (verbatim):** see Step 1. Row decisions (verbatim): "2. header untouched", "3. download is hidden only when under the early acces period and non subscriber", and "when something is in early access and not accessible by the user, tell them they can subscribe. Also make the launch button blue to make it more prominent (using design token)."
**Assistant interpretation:** Server-render the new list with the decided download/early-access rules and the accent token.
**Inferred user intent:** A browsable, attractive index where each app can be tried or downloaded, and non-subscribers learn how to get early-access apps.
**Commit (code):** cbbe3df — "feat(catalog): landing page browser in the html.moldandyeast.com style"

### What I did
- `pages.go`: `card` gains `LaunchMode`, `HTMLFile`, `FreeSample`, `EarlyAccess()`; `cardOf` picks the release's `role:"html"` file.
- `feedPage`: loads settings, resolves the visitor (`member` = signed in and not guest/disabled, `paying` = `Entitlement.Paid()`), sort orders `newest|oldest|az|small|large` (unknown → newest), tag counts over all cards, `archiveRowOf` (download rule), `groupArchive` (month labels, initials for A–Z, one "By size" group). Body: `Groups, Count, Total, Tags{Name,Href,Count,Current}, Orders, Host, …`.
- `home.tmpl`: hero (`home.hero`), controls (search, sort menu from `Orders`, live count, tags with counts), groups of `article.ri` rows with `data-open`/`data-launch`/`data-demo`/`data-early-access-until` for Phase 4, early-access line with Subscribe →, empty state, a `dialog.browser-viewer` placeholder.
- `layout.tmpl`: `thumb` sizes `(max-width: 900px) 100vw, 402px`; video posters use the new `variantSet` srcset; `mini` partial removed.
- `catalog.css`: `--color-accent`/`--color-accent-hover` on `.final-site`, the three `#cee5ff` literals now use it, `.site-button.is-primary`; new `.browser*`, `.tags/.tag*`, `.group-label`, `.browser .ri*`, `.ri-early`, mobile rules; all `.archive-*` list rules removed (only `.archive` for the journal remains).
- Removed dead `catalog.tmpl` and `journal.tmpl` (never rendered).
- Tests: new `browser_test.go` `TestBrowser` (hero, tag counts, month groups and counts, every sort order, A–Z initials, unknown sort, download/early-access matrix for guest, free, paying, admin, subscribe invitation); updated `final_archive_test`, `TestMediaVariants`, `TestVideos` for the new markup.
- Local preview: a throwaway, uncommitted `pkg/catalog/zz_preview_test.go` seeds the six imported apps with their real covers (from the import's `gallery/`) and serves on `127.0.0.1:18555`; screenshots at 1440 px and 390 px.

### Why
- Server-side groups and sorting keep the page cacheable, indexable and usable without JavaScript; the reference's client-side layout engine is unnecessary for tens of rows.
- The download rule mirrors `mayDownload` exactly, so the list never offers a link the API would refuse except for guests, who get a sign-in link.

### What worked
- `go test ./pkg/catalog/... -count=1` all ok; `golangci-lint` clean apart from the uncommitted preview file.
- Visual check: hero, controls, tag counts, month group label, rows with covers, Open/Download/path·size, early-access Subscribe line (Hypercore, seeded in early access) all render as intended on desktop and mobile.

### What didn't work
- `TestBrowser` first run: `browser_test.go:135: fred@example.com sees alpha download as signin, want direct`. My first rule only gave direct links to paying visitors; `mayDownload` lets any signed-in member download non-early-access releases. Fixed with the `visitor{member, paying}` struct.
- Compile error in the test: `v.who.user.Name undefined` (store.User has no Name); used Email.
- `SourceStaff` is defined in package `catalog`, not `billing`; then simplified to `e.Paid()`, which already includes staff.
- Preview screenshot showed a small empty box under the headline: the closed `<dialog class="viewer">` was displayed because `.viewer` sets `display`. Added `.browser-viewer:not([open]) { display:none; }`.
- A `find / -iname 'ABCAreal*woff2'` to locate fonts ran past the 2-minute limit and was stopped; the fonts are installed system-wide as TTF, which the browser used.

### What I learned
- The early-access rule is stricter than `mayDownload` for free samples on purpose (user decision: hide during early access for non-subscribers even if the release is a free sample).
- `Store.Releases` already loads files, so sizes cost no extra query.

### What was tricky to build
- Sorting newest-first by reversing a sort with `SliceStable` and ties on ID: implemented as `published(b, a)` so equal timestamps still order deterministically.

### What warrants a second pair of eyes
- Early access + free sample + non-paying visitor: no download link (by decision), although the API would allow it.
- Tag counts are computed over all listed cards, not over the current search, like the reference.

### What should be done in the future
- Phase 4: client behaviour (live filter, Open viewer, tab launch, app page inline auto-start and subscribe button), remove the old hover-preview script.

### Code review instructions
- `pages.go` `feedPage`, `archiveRowOf`, `groupArchive`; `home.tmpl`; `catalog.css` "Landing page browser" block; `browser_test.go`.
- Validate: `TEST_DATABASE_URL=… go test ./pkg/catalog -run 'TestBrowser|TestFinalArchive|TestMediaVariants|TestVideos'`.

### Technical details
- Download href: `/api/releases/{rel}/files/{file}/download` with `download="{name}"`; sign-in: `/signin?return=%2Fapps%2F{id}`.
- Group label formats: `January 2006` (UTC), initial letter or `#`, `By size`.

## Step 4: Client behaviour (Phase 4)

Made the browser and the app page behave as designed: the list filters as you type, **Open** starts inline apps in a full-viewport viewer and tab apps full screen in a new tab, the app page auto-starts inline apps for visitors who may run them, and visitors without access to an early-access app are told they can subscribe, with a blue (accent token) button. The access rules live in one pure module with unit tests.

The whole flow was checked end to end with Playwright against the local preview (six seeded apps, guest/free/paying tokens), on top of the Go, TypeScript and build gates.

### Prompt Context
**User prompt (verbatim):** see Step 1; the subscribe/blue-button decision is quoted in Step 3.
**Assistant interpretation:** Implement Phase 4 of the guide (client) and validate it in a browser.
**Inferred user intent:** Apps should be one click away, in the mode that suits each app, and the early-access wall should sell the subscription instead of looking like a dead end.
**Commit (code):** 1055630 — "feat(catalog): open apps inline in a viewer or full screen in a new tab"

### What I did
- New `ui/catalog/launch-decision.ts`: `Plan`, `LaunchData`, `Decision` (`subscribe | signin | start{demo}`), `earlyAccess`, `canRun` (mirrors `Entitlement.CanRun` plus instance allowance), `decide`, `autoStart` (inline, may start, not a demo), `inNewTab`. Tests in `tests/launch-decision.test.ts` (10-case table + auto-start/new-tab), registered in `tests/run.ts`.
- `ui/catalog/public.ts` rewritten around it: helpers `label`, `openInstance`, `mountFrame`, `openInNewTab`; `renderActions` (Subscribe / Sign in / Start with `.site-button.is-primary`, labels from `data-label-*` = site copy); a `Viewer` class for `dialog[data-viewer]` (title, Details, Close · Esc, Phone/Tablet/Desktop, Reload, Open in new tab, `?open=` in history, Back closes); `bindBrowser` (live filter on `data-search`, hides empty groups, updates count and `?q=`, sort submits, Open interception, `?open=` on load only when `autoStart` allows). The old `bindArchive` and hover preview are gone.
- `artifact.tmpl` runner: `data-launch` and label attributes; server fallback for non-JS / before hydration: early-access blocked → `p.runner-early` + Subscribe button, else Sign in. `artifactPage` computes `paying` and `EarlyBlocked`.
- `ui/catalog/instances.tsx`: the full-screen page's Close returns to `/apps/{artifactId}` (Studio for previews).
- `catalog.css`: viewer dialog reset, `.v-bottom`, `.v-sizes`, `.v-state`, `.runner-early`.
- `browser_test.go`: app-page assertions (`data-launch`, labels, subscribe invite for a free user, none for a paying one).
- `make web-build` regenerated `pkg/catalog/web/static/{catalog.css,catalog.js,public.js}`.

### Why
- One pure decision module keeps the list and the app page consistent and lets the rules be tested without a DOM.
- The new tab is opened as `about:blank` inside the click handler and navigated after the instance is created; opening it after the `await` would be blocked as a popup.
- Demos never auto-start: every start spends the demo budget, and a crawler or a stray `?open=` link would burn it.

### What worked
- `TEST_DATABASE_URL='postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable' go test ./... -count=1`: all packages ok.
- `pnpm check` ok; `pnpm test` 20 pass; `make web-build` ok.
- Playwright on `http://localhost:18555`:
  - typing "comic" → "2 experiments", URL `?q=comic`;
  - guest, Open on inline Living Comic Studio → viewer opens, demo state "Live", Esc closes and restores the URL;
  - paying user, Open on tab-mode Mimeo → new tab at `/instances/ins_…`;
  - inline app page auto-starts and shows Live;
  - tab-mode Kiln page shows a blue `rgb(206, 229, 255)` "Start experiment →" button and does not auto-start;
  - free user on early-access Hypercore Studio: "In early access for subscribers until 12 November 2026. Subscribe to run it now." + "Subscribe →".

### What didn't work
- A Python splice script that rewrote `public.ts` in place stopped with `ValueError: substring not found` at the step removing the old inline `Plan` type (the anchor text had already changed). I rewrote the file as a whole instead of splicing.
- First Playwright attempt to read the token file from inside `browser_run_code_unsafe` failed with `require is not defined` (the code runs in a sandbox without Node modules); tokens were passed in as literals instead.
- `golangci-lint`: one gofmt finding, only in the uncommitted `zz_preview_test.go`.
- Prettier warns about `ui/catalog/device.tsx`, `launch-protocol.ts`, `service-notice.ts`, `tokens.tsx`; a stash comparison showed the same warnings before my changes, so they were left alone.

### What I learned
- `canRun` must special-case staff before the instance count (staff run with no plan allowance).
- A guest with a demo on an early-access app must get "subscribe", not "start demo": early access wins over demos.

### What was tricky to build
- History handling for the viewer: Open pushes `?open={id}`; Close/Esc pushes the URL without `open` (so Back reopens nothing stale), and `popstate` to a URL without `open` closes the dialog without touching history. `?open=` on load reopens only when `autoStart` allows, so shared links never start demos.

### What warrants a second pair of eyes
- `canRun` duplicates server rules; the server still decides when the instance is opened, so a mismatch shows an error rather than leaking access.
- The early-access + free-sample + non-paying case hides Download by decision although the API would allow it.

### What should be done in the future
- Phase 5: deploy, set launch modes per app (`francois catalog artifacts update <id> --launch-mode inline|tab`), smoke test in production.

### Code review instructions
- Start with `ui/catalog/launch-decision.ts` and its test, then `ui/catalog/public.ts` (`renderActions`, `Viewer`, `bindBrowser`), `artifact.tmpl` runner block, `pages.go` `artifactPage`.
- Validate: `pnpm check && pnpm test`; `go test ./pkg/catalog -run TestBrowser`.

### Technical details
- Row data attributes: `data-open data-artifact data-release data-title data-description data-launch data-demo data-early-access-until`.
- Runner label attributes: `data-label-start data-label-demo data-label-signin data-label-subscribe`, from copy keys `runner.*` and `earlyaccess.button`.

## Step 5: Real apps in the viewer and in a new tab (pre-deploy check)

Ran the six imported apps, with their real HTML and manifests, through the new viewer at Phone, Tablet and Desktop sizes, and two of them through the new-tab path. Until now the local preview had seeded every app with the test placeholder page (it just says "board"), so the user saw a blank instance and asked why it didn't load. With the real builds, every app reaches "Live" in about a second and renders at all three sizes. The only defect found in our code was a redundant `allowfullscreen` attribute that made Chrome warn on every viewer open; it is fixed.

### Prompt Context
**User prompt (verbatim):** "why does http://localhost:18555/instances/ins_30daff07ede8499e4208 not load ?" (with a pasted browser console), then "ok, so when we deploy it it would be good?", then "ok, do it. commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
**Assistant interpretation:** Before deploying, check that the real apps (several MB, 3D) work in the inline viewer at each size and in the new-tab path, fix anything found, and record it.
**Inferred user intent:** Confidence that deploying FRANCOIS-022 won't break the six live apps.
**Commit (code):** f0d389e — "fix(catalog): drop the redundant allowfullscreen on the viewer frame"

### What I did
- Diagnosed the "does not load" report: `GET /api/instances/ins_30daff07ede8499e4208` as the admin test user returned the instance (owner `ada`, Hypercore Studio). In a browser the full launch chain succeeded (`/launch` 200, `/api/instances/{id}/launch` 200, `/api/launch/exchange` 200, `/page` 200, `runtime.js` 200, `/api/views` 200), and the frame showed the placeholder text "board". The pasted CSP errors come from `content.js` / `index.js` (a browser extension) and "tooltipContainer does not exist" from the same source; none are from our bundles.
- Changed the throwaway preview seed (`pkg/catalog/zz_preview_test.go`, uncommitted; a copy is saved as `scripts/01-local-preview-seed.go.txt`) to load `scratchpad/build/<id>.html` and `<id>.json` (the manifests published to production) when present.
- Flipped all six apps to `inline` directly in the throwaway test DB (`UPDATE catalog_artifacts SET launch_mode='inline' WHERE id <> 'polis'`), then, as the paying user, opened `/?open=<id>` for each app, waited for "Live", and took screenshots at Desktop, Tablet (834 px) and Phone (390 px) in a 1440×900 window.
- Put four apps back to `tab` and clicked Open on Kiln Workbench and Form Skate Workbench from the landing page: each opened a new tab at `/instances/ins_…` with the app frame at `i-….localhost:18555/launch`; the full-screen page's Close points to `/apps/{id}`.
- Removed `allowfullscreen: ""` from `mountFrame` in `ui/catalog/public.ts`, ran `pnpm check` and `make web-build`, restarted the preview and confirmed the frame has only `title, sandbox, allow, src` and the console has no warnings.

### Why
- The placeholder made the preview useless for judging the viewer with real content; the risk in the deploy is size, 3D and layout of real apps inside a resizable dialog, not the launch protocol (unchanged).
- Changing the test DB directly instead of the Studio API: the API needs the CSRF token from the session, which `/api/me` does not expose to a script (two attempts returned `403 csrf_failed`); the DB is a throwaway created by the test.

### What worked
- All six apps reached "Live" in the viewer in 1.08–1.21 s (Living Comic 1078 ms, Hypercore 1185, Form Skate 1146, Mimeo 1164, Kiln 1209, Octavo 1139).
- All rendered at Desktop and Tablet; Phone renders but see below.
- No console errors from our code or the apps; the only warning was the `allowfullscreen` one, now gone.
- New-tab path works for real apps (Kiln, Form Skate).

### What didn't work
- My first browser check of the user's instance returned `401 login_required` and "Instance not found": I had hardcoded the admin token from before the preview restart (each restart creates a new DB and new tokens). With the current token it loaded. No bug.
- `PATCH /api/studio/artifacts/{id}` from the page with `x-csrf-token` taken from `/api/me` → `403 {"error":{"code":"csrf_failed","message":"Missing or invalid CSRF token"}}` (both `me.csrf` and `me.token` are wrong sources). Used the test DB instead.
- A first `psql` attempt stored `docker exec … psql` in a shell variable and failed with `command not found` (zsh does not word-split); used a function.
- Frame lookup in the screenshot script (`f.url().includes('/page')`) found no frame (the frame keeps its `/launch` URL), so per-size measurements of the app's inner width are missing; screenshots were checked by eye instead.

### What I learned
- The app frame's URL stays `/launch` in Playwright's frame list even after it shows `/page`; match on the instance origin instead.
- Living Comic Studio (and Hypercore, same engine) lays out its comic page wider than 390 px, so at Phone size the panels and chat input are cut off on the right inside the frame. That is the app's own layout, not the viewer; a real phone would show the same.

### What was tricky to build
- Nothing new in the code; the challenge was making the preview faithful (real HTML + manifest, real covers, the right mix of launch modes, a fresh token per restart).

### What warrants a second pair of eyes
- Phone size for the comic apps (Living Comic, Hypercore): decide whether to keep Phone in the viewer for them or accept the app's own overflow.
- Production differs from the preview in one way that matters: the storefront and app domain are different hosts there (here both are `localhost`). The CSP `frame-src` and the app domain's `frame-ancestors` are built from config and were only exercised locally.

### What should be done in the future
- Phase 5: deploy, then set launch modes per app and repeat the viewer check on production.

### Code review instructions
- `ui/catalog/public.ts` `mountFrame`.
- Reproduce: copy `scripts/01-local-preview-seed.go.txt` to `pkg/catalog/zz_preview_test.go`, `touch $S/preview.run`, run `TEST_DATABASE_URL=… PREVIEW_STOP_FILE=$S/preview.run go test ./pkg/catalog -run TestZZPreview -v -timeout 8h`, open `http://localhost:18555/`, set the cookie `francois_session_dev` to a token from `$S/preview-tokens.txt`.

### Technical details
- Viewer sizes: Phone `max-width: 390px`, Tablet `834px`, Desktop `100%`.
- Frame attributes after the fix: `sandbox="allow-scripts allow-same-origin allow-forms allow-popups allow-modals allow-downloads"`, `allow="clipboard-write; fullscreen"`.
- Screenshots (not committed): `$S/screens/real/<id>-{Desktop,Tablet,Phone}.png`, `sheet.png`, `<id>-newtab.png`, where `$S` is the session scratchpad.

## Step 6: Preview sign-in links and the early-access baseline

Two small things came out of the user trying the preview. Setting the session cookie from the console did nothing for them, because a guest session from an earlier demo had left an HttpOnly cookie with the same name, which scripts cannot overwrite. I added `/preview-login?as=…` links to the throwaway preview. They also pointed out that the early-access sentence sat higher than the Subscribe button label; that is fixed.

### Prompt Context
**User prompt (verbatim):** "How do I set the cookie? I'm not sure it work?", "no somehow, the api/me still shows guest afer setting the subscriber", "oh i had to manually remove the cookie?", "How can I align the baseline here" (with a screenshot of the early-access line and Subscribe button)
**Assistant interpretation:** Make signing in to the preview reliable; align the early-access sentence with the button.
**Inferred user intent:** Try the preview as each kind of visitor without fiddling, and polish the early-access invitation before deploy.
**Commit (code):** 4fbed45 — "fix(catalog): align the early-access line with the Subscribe button"

### What I did
- Preview only (uncommitted `zz_preview_test.go`; copy refreshed in `scripts/01-local-preview-seed.go.txt`): wrapped the server handler in a mux with `GET /preview-login?as=paying|free|admin|guest`, which sets `francois_session_dev` server-side (HttpOnly, 1 day; `guest` or unknown clears it) and redirects to `/`.
- `catalog.css`: `.runner-start` `align-items:center` → `baseline`; `.runner-early` `margin:0 0 14px` → `margin:0`.

### Why
- `setCookie` in `auth.go` always sets `HttpOnly: true`, so a guest session from a demo blocks `document.cookie` writes with the same name; the browser ignores them silently. Only a server response (or deleting the cookie in devtools) can replace it.
- Baseline alignment keeps the 17 px sentence and the 15 px button label on one line, whatever the button padding.

### What worked
- `curl` with a stale `francois_session_dev=stale-guest` cookie, then each login link, then `/api/me`: `pat@example.com`, `fred@example.com`, `ada@example.com`.
- Screenshot of `.runner-start` on `/apps/hypercore-studio` as the free user shows the sentence and the button label on one baseline.

### What didn't work
- First curl loop: zsh `no matches found: http://localhost:18555/preview-login?as=paying` (the unquoted `?` is a glob); quoted the URL.
- A numeric baseline probe (zero-size inline-block appended to each element) reported text 462.6 px vs button 457.6 px; inside the `inline-flex` button the probe becomes a flex item, so the number is not the label's baseline. Judged by the screenshot instead.

### What I learned
- Any page that tells people to set the session cookie from the console will fail after a demo; the server-side link is the reliable way, also for future previews.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- On narrow screens `.runner-start .site-button` is `width:100%` and wraps below the sentence; baseline alignment does not affect that layout.

### What should be done in the future
- Phase 5 as before (deploy, launch modes per app, production viewer check).

### Code review instructions
- `pkg/catalog/web/static/catalog.css` lines for `.runner-start` and `.runner-early`.

### Technical details
- Preview links: `http://localhost:18555/preview-login?as=paying|free|admin|guest`.

## Step 7: Gallery screenshots open full size in a modal

Gallery screenshots on an app page now open full size in a modal. Before, clicking one did nothing, on production too (slop.boutique's gallery is plain `<figure><img>`). Each image is now wrapped in a link to the full-size file, so it still works without JavaScript. With JavaScript, the link opens a full-viewport lightbox with caption, position, previous/next and keyboard support.

### Prompt Context
**User prompt (verbatim):** "also when clicking on screenshots they should show up full size as modal (like https://slop.boutique/apps/hypercore-studio)"
**Assistant interpretation:** On app pages, clicking a gallery screenshot should open it full size in a modal (the URL names the page with the screenshots; it has no modal today).
**Inferred user intent:** Let visitors look at the screenshots properly without leaving the page.
**Commit (code):** 54ddbd2 — "feat(catalog): open gallery screenshots full size in a modal"

### What I did
- `artifact.tmpl` gallery: images are wrapped in `<a class="gallery-zoom" href="/media/{id}" data-zoom aria-label="View full size: {alt}">`; videos are not wrapped (they keep their controls).
- `ui/catalog/public.ts` `bindLightbox(gallery)`: one `dialog.lightbox` per gallery with the image, caption (alt), `n / N`, ← Previous / Next → (only with more than one image), Close · Esc. Arrow keys step, wrapping around. A click on the dialog padding closes it. Focus returns to the thumbnail link on close. Modified clicks (Cmd/Ctrl/Shift, middle button) keep the browser's default and open the file.
- `catalog.css`: `.gallery-zoom` (zoom-in cursor), `.lightbox*`. The image uses `object-fit: contain` in the remaining height. `html:has(.lightbox[open]) { overflow:hidden }` stops the page scrolling behind the modal. The bar wraps on narrow screens: caption on its own line, nav and Close on the next.
- `videos_test.go` `TestVideos`: the page gallery holds an image and a video. The test asserts the image is wrapped in the zoom link and the video is not.
- Preview seed: uploads up to six screenshots from each app's `gallery/` folder as the page gallery.

### Why
- A link to `/media/{id}` (the original, up to 2400 px wide) gives full size without JavaScript and in a new tab, and the modal is progressive enhancement, like the rest of `public.ts`.
- Videos already have controls; zooming them would conflict with play/pause clicks.

### What worked
- Playwright on `/apps/hypercore-studio` (1440×900): click the 2nd screenshot → dialog open, "2 / 6", `src` is the original (natural 2080×1704); ArrowRight → "3 / 6"; ArrowLeft twice → "1 / 6"; Esc closes and focus is back on the link; a click outside the image closes; Close · Esc closes; no console errors.
- 390×844: image fits, bar wraps onto two lines, `html` overflow is `hidden` while open and `visible` after.
- `go test ./pkg/catalog -run 'TestVideos|TestBrowser|TestMediaVariants'` ok; `pnpm check`, `pnpm test`, prettier clean.

### What didn't work
- First phone screenshot: the bar was squeezed into one row ("Hypercore / Studio, / grow-a-monster", "3 / 6" broken over two lines), and the page scrollbar showed at the right edge. Fixed with `flex-wrap`, `white-space:nowrap` on the nav, a full-width caption under 640 px, and the `:has()` scroll lock.
- `golangci-lint` still reports one gofmt issue, in the uncommitted `zz_preview_test.go` only.

### What I learned
- There was no existing lightbox anywhere in the storefront, so nothing to reuse; the landing-page viewer dialog was the model for the full-viewport `<dialog>` reset.

### What was tricky to build
- Closing on a click outside the image: the image box fills the remaining height with `object-fit: contain`, so a click on the letterbox area beside the picture hits the `img`, not the dialog, and does not close. Only the dialog's padding and bar gaps close. I accepted this; Esc and Close are always there.

### What warrants a second pair of eyes
- `:has()` is needed for the scroll lock. Browsers without it (old Firefox before 121) just let the page scroll behind the modal.

### What should be done in the future
- Possibly preload the next image, and support swipe on touch screens.

### Code review instructions
- `ui/catalog/public.ts` `bindLightbox`; `artifact.tmpl` gallery line; `catalog.css` `.lightbox*`; `videos_test.go`.

### Technical details
- Full-size URL: `/media/{id}` without `?w=`, i.e. the original upload.
