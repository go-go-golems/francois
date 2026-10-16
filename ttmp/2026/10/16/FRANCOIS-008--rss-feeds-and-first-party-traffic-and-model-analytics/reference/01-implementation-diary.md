---
Title: Implementation diary
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T11:26:00.60295595-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation diary

## Step 1: Establish existing contracts and create the ticket

The request spans an implemented feature and an intern-facing research handoff. I inspected the hosted catalog before choosing the change: journal RSS and provider token settlement already existed, while durable traffic measurement did not. The new implementation therefore adds release syndication and portal traffic reporting while reading the existing model ledger.

### Prompt Context

**User prompt (verbatim):**

> Investigate, add new docmgr ticket, add RSS feed. Also add traffic analytics (tracking users , referrers, all the standard stuff, as well as token usage,...)
>
> Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
>
> The analysis detailed, the implementation is simple, elegant, straightforward

**Assistant interpretation:** Implement RSS/analytics, create a new docmgr ticket with a detailed intern guide, and deliver the guide to reMarkable.

**Inferred user intent:** A small maintainable feature with a thorough technical handoff.

### What I did

- Read ticket-research-docmgr-remarkable, docmgr, diary, and remarkable-upload skill instructions.
- Inspected portal Host routing, routes, publication models, sessions, the existing RSS handler, accounting SQL, runtime settlement, React admin UI, and migration runner.
- Created FRANCOIS-008 and its design and diary documents through docmgr.
- Consulted primary RSS, Go HTTP, and PostgreSQL aggregation references; linked them in the guide.

### Why

The local owner service and hosted platform have different persistence and identity models. Existing publication and accounting rules are safer boundaries than a parallel event/accounting implementation.

### What worked

- `docmgr status --summary-only` confirmed seven existing tickets before creation.
- Source inspection established that journal RSS already has both discovery and publication filtering.
- `model_calls` already records token/cache/cost usage; timestamp-based analysis includes demo period keys.

### What didn't work

- Initial guesses `web/`, `pkg/catalog/public.go`, and `pkg/catalog/feed.go` did not exist. Used repository file search to locate `pages.go` and `ui/catalog` instead; `feed.go` is now a new file.

### What I learned

`period` in model accounting is an allowance key, not a universal reporting date. Demo periods use `demo:<instance>`.

### What was tricky to build

The process serves several domains over one database. Traffic can record the configured portal site, but the historical model ledger has no site attribute. Reporting must state that model data stays global when traffic is filtered.

### What warrants a second pair of eyes

Review product expectations around document-load analytics versus SPA route events. The implementation and guide explicitly describe the former.

### What should be done in the future

Evaluate event ingestion only if client-side engagement or conversion reporting becomes a requirement.

### Code review instructions

Start at `pkg/catalog/server.go`, `auth.go`, `routes.go`, `pages.go`, and `store/usage.go` before reviewing the new files.

### Technical details

- New ticket: `ttmp/2026/10/16/FRANCOIS-008--rss-feeds-and-first-party-traffic-and-model-analytics`.
- No pre-existing working-tree changes were observed by initial `git status --short`.

## Step 2: Implement feeds, traffic records, and reporting

Added one traffic migration, one store aggregation module, one portal middleware/API module, and one release-feed module. The UI uses the existing RTK Query and admin navigation patterns. Accounting execution and credit settlement remain untouched.

### Prompt Context

See Step 1. No additional user scope changes occurred.

### What I did

- Added `/apps/feed.xml`, a listed/published query capped at 50, stable opaque GUIDs, shared XML serialization, and storefront feed links.
- Added host-only visitor/visit cookies, successful document-load recording, normalized referrers, UTM dimensions, browser/device categories, and optional authenticated user IDs.
- Added DNT/GPC and common-bot exclusions, a 250 ms insertion budget, and private/no-store tracked HTML responses.
- Added `/api/admin/analytics` with a five-second query budget, 90-day maximum date range, traffic site filter, top-100 grouping tables, and catalog-wide ledger usage.
- Added 90-day traffic cleanup at startup/hourly under the existing errgroup and updated privacy text/date.
- Added admin Analytics controls, traffic tiles/tables, and model token/cache/cost tables; rebuilt embedded JavaScript.

### Why

One database and ordinary handlers meet the current scale without an SDK, queue, worker pool, or external analytics deployment. Keeping provider usage in its existing ledger avoids double-counting and billing coupling.

### What worked

The integration tests exercise cookie reuse, user correlation, portal filtering, privacy exclusions, retention, aggregate model cache/credit data including demo periods, and RSS publication/visibility boundaries.

### What didn't work

- A new feed test initially called the nonexistent admin publishing path and returned `404 map[error:map[code:not_found message:Unknown API route]]`. Corrected it to `/api/studio/releases/{id}/publish`.
- Its visibility fixture initially used `unlisted`; PostgreSQL returned `violates check constraint "catalog_artifacts_visibility_check" (SQLSTATE 23514)`. Corrected the fixture to the existing `hidden` state.
- The existing storefront cache assertion expected `no-cache`. Updated it to the intentional tracked-response contract, `private, no-store`.
- Catalog lint found a named return (`nonamedreturns`) and numeric HTTP 503 literals (`ST1013`). Replaced the named return and used `http.StatusServiceUnavailable`.

### What I learned

RSS opaque GUIDs must explicitly declare `isPermaLink="false"`; a version-specific ID remains stable even when several versions link to the same artifact page.

### What was tricky to build

Cookies must be set before headers commit, while only successful HTML responses should become events. A small ResponseWriter wrapper captures status and changes HTML cache headers before delegating. Eligible failed routes may therefore still issue cookies; this is documented, while opted-out/excluded requests issue none.

### What warrants a second pair of eyes

Review the retention and cookie lifetimes, the limited UA classifier, and the interpretation of range-local single-page sessions. The reporting query deliberately does not claim completed-session bounce semantics.

### What should be done in the future

Measure report query cost at real traffic volume before introducing rollups or additional dimension indexes. Account erasure procedures must handle analytics user IDs explicitly.

### Code review instructions

Read new `analytics.go`, `store/analytics.go`, migration 009, `feed.go`, and `analytics_test.go`; then inspect the admin UI integration and generated bundle diff.

### Technical details

- No new dependencies or environment-variable reads were introduced.
- No production database migration, deployment, or Git commit was performed.
- Date ranges are inclusive `from`, exclusive `to`, at midnight UTC.
- Token/cost reports include every model-call period and use creation timestamps.

## Step 3: Validate and prepare the intern handoff

Validated the implementation with the actual local PostgreSQL service, not only skipped database tests. Wrote the detailed guide with a system introduction, source map, architecture diagram, request pseudocode, metric definitions, API examples, decisions, rollout instructions, tests, and explicit extension boundaries.

### Prompt Context

See Step 1. The user was notified before running the existing harness that reads `TEST_DATABASE_URL`; the URL was set explicitly for local integration tests.

### What I did

- Used a writable `/tmp/francois-go-cache` for Go builds/tests and a writable lint cache.
- Inspected running local containers with authorized execution and located PostgreSQL on port 5435.
- Ran the full Go suite with isolated per-test databases, TypeScript check, browser contracts, frontend build, Go build, and catalog lint.
- Kept the new React component formatted without reformatting unrelated historical UI code.

### Why

Aggregation, migrations, authorization, and publication filtering require a real PostgreSQL integration gate. Pure serialization checks would miss SQL/fixture mistakes.

### What worked

- `go test ./...` with explicit local `TEST_DATABASE_URL`: passed after fixture corrections.
- `pnpm check`: passed.
- `pnpm test`: five tests passed.
- `pnpm build`: passed and regenerated the embedded catalog bundle.
- `go build ./...`: passed with writable cache.
- `golangci-lint run ./pkg/catalog/...`: zero issues after cleanup.
- Final focused feature suite and docmgr checks are recorded below.

### What didn't work

- Initial `go test ./pkg/catalog/...` hit the sandbox's cache restriction: `open /home/manuel/.cache/go-build/...: read-only file system`. Switched to `GOCACHE=/tmp/francois-go-cache`.
- Sandbox execution then prevented httptest listeners (`httptest: failed to listen on a port`); Docker inspection also initially returned `connect: operation not permitted`. Authorized execution allowed local integration validation.
- One full-suite attempt saw the pre-existing `TestConcurrentNewArtifactUpload` return `409 ... code:version_exists ... version 1.0.0 already exists for race-1: conflict`. The next full suite passed. No unrelated upload-race code/test was changed.
- pnpm warned that Node v20.17.0 is below the required v24. Commands passed, but supported-version CI remains the release baseline.

### What I learned

A passing unit-only run would have skipped the important database contracts. Explicitly locating the running development PostgreSQL service made the validation meaningful without starting another server.

### What was tricky to build

Testing a new cache policy affected an existing storefront expectation. Preserving the private-cookie response contract was necessary; the test was updated to the intentional new value.

### What warrants a second pair of eyes

No browser visual inspection was performed. The guide clearly separates runtime-tested behavior from UI presentation review and production rollout.

### What should be done in the future

Run supported Node-version CI and the normal production rollout procedure before enabling these changes on the live service.

### Code review instructions

Read the design guide first, then run the commands in its validation section. The uploaded document describes cloud delivery rather than asserting physical tablet synchronization.

### Technical details

The final change is uncommitted in the shared working tree. Validation receipts and reMarkable upload result are appended as milestones below.

### Final validation and delivery receipt

- Final focused PostgreSQL feature suite: `go test ./pkg/catalog -run 'TestTraffic|TestReleaseFeed|TestRSS|TestStorefrontPages' -count=1` passed (`ok .../pkg/catalog 0.392s`).
- Full PostgreSQL suite passed; final focused tests passed after lint cleanup. Evidence is retained under `various/validation/`.
- Final `pnpm check`, `pnpm build`, `go build ./...`, and `git diff --check` passed. The build required authorized module metadata cache access after a sandbox-only attempt logged `go: writing stat cache ... read-only file system`.
- `docmgr doctor --ticket FRANCOIS-008 --stale-after 30`: all checks passed.
- Initial reMarkable upload in the sandbox failed with `dial tcp: lookup webapp-prod.cloud.remarkable.engineering: no such host`. The authorized network execution succeeded; no reauthentication or overwrite was needed.
- Exact delivery result: `OK: uploaded FRANCOIS-008 RSS and Analytics Intern Guide.pdf -> /ai/2026/10/16/FRANCOIS-008`.
- Source uploaded: `design-doc/01-rss-and-analytics-intern-architecture-and-implementation-guide.md` (5,305 words including frontmatter).
- This is a successful reMarkable cloud upload receipt, not verification of physical tablet synchronization.
- Added README discovery notes for feeds, analytics, limitations, and migration requirement.
- Changes remain uncommitted and undeployed.

## Step 4: Commit the feature and design analytics in Paper

The user requested ongoing commits and the detailed diary format, then asked for an Analytics group in Paper, graphs, and page/referrer filtering with an easier date dialog. This entry backfills the design work from the actual tool results. The feature was committed before the new graph/filter implementation began.

### Prompt Context

**User prompts (verbatim, in order):**

> commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill) Backfill diary if necessary. Did you add any pages / designs?

> Ok, add a new grorup in paper for the analytics pages.

> add graphs as well, following that aesthetic. Design in paper first.

> add ways to filter / stackchart for pages / referrers, and an easy way to select dates (a date dialog), maybe there's stuff on the other pages that can be reused at this point.

**Assistant interpretation:** Preserve implementation progress in Git, keep the investigation-format diary current, create a separate Paper Analytics group, design the requested graphs and controls before implementing them, and reuse existing product patterns.

**Inferred user intent:** A coherent Areal analytics experience with clear visual design and reviewable implementation history.

**Commit (code):** `0a661142ad0b6a4311b7d47423beaab18b513872` — Add release RSS and first-party catalog analytics.

### What I did

- Reviewed the entire implementation diff and existing PostgreSQL/lint receipts; staged only the 17 feature paths and committed the completed implementation.
- Loaded Paper's guide, inspected the active unified design file, existing Studio and catalog structures, design tokens, and ABC Areal/Areal Mono fonts.
- Created a divider-separated Analytics group after Studio and before Archive, with Traffic and Model usage pages. Existing pages remain in their original order.
- Built traffic and model-report desktop artboards, then added daily traffic bars, paired input/output bars, and a provider-cost line graph.
- Added traffic page/referrer filter controls, Total/Stack by page/Stack by referrer controls, a stacked page chart, and an additional date-dialog artboard with presets and start/end fields.
- Inspected existing catalog filters, admin ModelPicker, and Studio MediaPicker. Reused their visual language rather than inventing a different control system; no existing date-range dialog was found.
- Exported clean inline-style JSX through Paper into JSON source snapshots under `sources/paper/` for later implementation; measurements came from Paper styles/exports rather than screenshot estimation.

### Why

A separate group makes the new work discoverable without scattering analytics across existing storefront/account pages. Daily charts need chronological data, and page/referrer stacks need server grouping that preserves an Other bucket. Native date controls and quick presets are simpler than a custom calendar dependency.

### What worked

- Paper pages: Analytics — Traffic and Analytics — Model usage.
- Artboards: A1 Traffic overview, A2 Model usage, A3 Date range dialog.
- Screenshot checkpoints verified headers, metric strips, fixed-width table columns, charts, and the date dialog.
- Final desktop layouts use fit-content heights so the graph additions do not clip lower tables or footnotes.
- The first implementation commit completed successfully; no production deployment or push occurred.

### What didn't work

- An initial empty `get_guide` call returned `Invalid arguments ... path: ["topic"]`; corrected it by requesting `paper-mcp-instructions`.
- A text-based F mark did not render as intended; replaced it with the existing-style SVG mark.
- An SVG hatch fill did not appear in Paper, hiding the Other segment. Replaced it with the tint token plus a muted outline and corrected the matching legend.

### What I learned

The Paper file already carries the product's typography/color tokens. Reusing them produces a consistent visual result and avoids introducing a chart color palette that clashes with the storefront.

### What was tricky to build

Monochrome stacks must remain distinguishable and total-preserving. A light Other bucket needs an outline against the white ground; omitting it makes the plotted totals visually wrong. Date UI must also translate an included end day into the API's exclusive UTC boundary.

### What warrants a second pair of eyes

The new Paper designs are design references with sample data, not live statistics. Page/referrer filters must affect all traffic aggregates consistently, while model reporting remains globally scoped. No historical attribution should be inferred from portal filters.

### What should be done in the future

Implement the designed graphs, server-backed traffic filters/stacks, and shared date-range dialog; validate zero filling, UTC boundaries, aggregation totals, and keyboard dialog behavior.

### Code review instructions

Read `design-doc/02-analytics-paper-designs-and-implementation-mapping.md`, inspect the linked Paper pages, then compare `sources/paper/` against the UI implementation. The initial implementation is independently reviewable at commit `0a66114`.

### Technical details

- Paper file: `01M506DAH3CXP5QS28T2VQ4W3N`, slop boutique — unified design.
- Traffic page: `p-A-0`; model usage page: `p-B-0`; new divider: `p-C-0`.
- Artboards: `2VH-0`, `2VI-0`, `35O-0`.
- Existing tokens: paper/ink/muted/underline/hairline/tint and ABC Areal/Areal Mono.
- Sample daily views total 12,480; sample model input/output totals 8.42M/1.16M; provider-cost series totals $34.82. These are illustrative values.


### Step 4 addendum: implementation-ready design and artifact attribution

**Additional user prompt (verbatim):** "Update the design doc to properly implement them. In moel usage, also break down per "artifact""

Expanded the Paper mapping document into a detailed implementation contract: exact predicates, nullable-referrer semantics, repeatable-read snapshot, complete top-three-plus-Other stacks, chronological zero filling, SVG rendering, draft/applied dates, native modal behavior, phased tasks, and regression cases. Added the Usage by artifact table to Paper and documented its data contract.

Investigation found a material retention boundary: `DeleteInstance` physically deletes instances, while model calls survive. The design therefore requires migration 010 to snapshot artifact IDs at admission and backfill only surviving provable relationships. Unknown historical attribution remains visible; an inner join is explicitly rejected because it would undercount.

A docmgr check initially rejected the new validation Markdown with `frontmatter delimiters '---' not found`. Added full ticket/reference frontmatter to the evidence document and retained the diagnostic here. Existing validation logs were consolidated into that Markdown receipt to keep incidental .log files out of Git.

The Paper artifact-table screenshot passed spacing/type/contrast/alignment checks, and its sample totals reconcile with the model summary. No graph/filter/date/artifact runtime changes have yet been made at this design checkpoint.


## Step 5: Implement and validate filtered reports and artifact accounting

Implemented the backend portion of the Paper-first design. Traffic filters now define one SQL population for totals, browsing tables and complete daily stacks, all read in one repeatable-read transaction. Model calls snapshot artifact identity at admission, preserving usage when an instance disappears.

### Prompt Context

**User prompt (verbatim):** "Ok, implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

**Assistant interpretation:** Finish the runtime implementation of graphs, filters/stacks, the date dialog, and artifact breakdown; commit validated milestones and maintain this investigation diary.

**Inferred user intent:** Deliver the working feature rather than stopping at the design handoff.

**Prior design commit:** `6909bee` — Document analytics design, Paper graphs, filters and artifact attribution.

### What I did

- Added migration 010 with artifact-ID snapshots and evidence-based historical backfill.
- Extended ReserveRequest and the hosted runtime to copy the authorized instance's artifact ID into each model-call row.
- Replaced the positional site argument with TrafficFilter; exact path/referrer filters and nil-versus-empty semantics apply consistently.
- Added bounded top-three-plus-Other daily series, artifact usage groups, and one-to-one artifact display-name lookup.
- Used a read-only repeatable-read transaction to make chart/table totals consistent within a report.
- Added PostgreSQL tests for more than 100 categories, daily reconciliation, combined filters, direct referrers, range boundaries, actual migration replay, and attribution after deletion.

### Why

A chart constructed from truncated browsing tables loses categories. Joining calls to live instances loses historical spend after deletion. The implementation addresses both at their authoritative database boundaries.

### What worked

- Focused PostgreSQL feature tests passed (`ok .../pkg/catalog 0.335s`).
- `go test ./pkg/catalog/...` with the explicitly supplied local TEST_DATABASE_URL passed.
- `golangci-lint run ./pkg/catalog/...` returned `0 issues.`
- Existing model reservation/settlement behavior remains covered by the catalog suite.

### What didn't work

No validation failures at this milestone. During local review, corrected the model aggregation GROUP BY position after adding a label field, before executing tests.

### What I learned

An empty referrer is a real category. Both SQL and chart identities need an explicit remainder marker rather than reusing an empty key for Other.

### What was tricky to build

Backfill cannot recover already-deleted instance relationships. The migration leaves those records unknown instead of fabricating attribution; new snapshots remain intact after deletion. Artifact titles are resolved after aggregation to avoid multiplying call counts through unrelated joins.

### What warrants a second pair of eyes

Review transaction scope, all predicate call sites, and how old unknown attribution is represented. Model usage intentionally remains catalog-wide under traffic filters.

### What should be done in the future

Complete the frontend graph/date/filter milestone and validate the browser interactions before marking the follow-up task complete.

### Code review instructions

Start with store/analytics.go, migration 010, store/usage.go, hosted/sample.go, and analytics_reporting_test.go. Re-run the catalog suite against isolated PostgreSQL test databases.

### Technical details

The report is bounded to 90 days, four daily categorical series per dimension, and the existing five-second request budget. No new dependency, provider request or environment-variable reader was added. Test execution uses the existing harness and an explicit local database URL, following the session's prior notification.


## Step 6: Deliver Paper-designed graphs, filters, dates and artifact tables

Implemented the complete frontend using the existing RTK Query transport and monochrome styling tokens. Traffic and Model usage now have distinct views, daily SVG plots, a shared date-range dialog, and exact tables. The backend milestone is `2f53554`; this step completes its operator-facing surface.

### Prompt Context

See Step 5 for the implementation request.

**Additional user prompt (verbatim):** "why is the top box in boxcharts wider thatn th ebottom?"

**Assistant interpretation:** Correct the visual width mismatch in the stacked bars while completing the already-authorized implementation.

**Inferred user intent:** Keep Paper and the working application visually aligned, with equal visible edges across stack segments.

### What I did

- Extracted the analytics screen from admin.tsx into analytics.tsx; added pure UTC/chart preparation, SVG chart rendering and shared native date-dialog modules.
- Wired page/referrer filters to the guarded server report, with exact row shortcuts, Apply/Clear controls and distinct direct/all-referrer states.
- Added local Total/Page/Referrer stack switches, input/output token bars, cost line, exact daily data tables, empty states and single-day points.
- Added the per-artifact usage table with current title labels and explicit Unknown artifact history, plus model/user/instance/day grouping controls.
- Rebuilt the embedded frontend. Added focused date/chart tests and a test runner that explicitly executes both existing and new suites.
- Added an actual hosted-call artifact-snapshot assertion to the existing runtime integration test.
- Ran Chromium checks against built assets, including date drafts, invalid ranges, Apply, Cancel/Escape focus return, referrer query encoding, local stack switches, model graphs, unknown artifacts, equal stack edges and mobile overflow.
- Fixed the outlined Other segment in Paper and React, then refreshed the Paper source snapshot and screenshot-reviewed the results.

### Why

Native SVG and native date controls meet the design without a chart/calendar dependency. Keeping draft and applied filters/ranges distinct prevents partial requests. Exact daily tables make values available without relying on color or hover interaction.

### What worked

- Catalog backend checkpoint: PostgreSQL suite passed and catalog lint returned zero issues before commit `2f53554`.
- Final `go test ./...` with explicit local PostgreSQL passed (`pkg/catalog 4.317s`).
- Final `golangci-lint run ./...`: `0 issues.`
- TypeScript check, eight frontend tests, and embedded build passed, including direct execution with Node v24.18.0.
- Chromium: 18 interaction/layout checks passed with no page errors. See various/validation/02-browser-validation.md for the exact receipt and screenshots.
- Paper screenshot review confirms matching visible stack edges, preserved totals, adequate contrast and no clipping.

### What didn't work

- Python's default interpreter lacked Playwright (`ModuleNotFoundError: No module named 'playwright'`). Used the available Playwright MCP tools instead, without installing a dependency.
- React static rendering warned that SVG title children were arrays. Converted them to single template strings and reran tests without warnings.
- A generic Node --test invocation reported only file-level results in this environment; replaced it with tests/run.ts so all eight named contracts are visibly executed.
- Restarting the preview with send-keys failed with `can't find pane: francois-008-preview` because stopping its only process ended the session. Recreated the tmux session.
- Formatting the browser-tool function added a trailing semicolon, producing `SyntaxError: Unexpected token ';'` in that tool's expression wrapper. Removed it and preserved the expression format; all checks then passed.
- The user's width observation exposed a stroke issue: white side strokes reduced visible dark widths while the Other outline extended outward. Removed side strokes and inset the Other outline; a browser assertion now compares actual visible boundaries.
- Initial visual review found long token summaries crowded on narrower screens. Changed summary tokens to compact notation while retaining exact table/tooltip values.

### What I learned

Geometric SVG widths alone do not prove equal visible widths when strokes differ. The regression check includes half-stroke extents. Native dialog behavior also provided focus restoration and Escape handling without a modal framework.

### What was tricky to build

Direct referrers and the remainder both have empty raw keys but different identities. The preparation code keys on (other,key), and the UI sends explicit empty referrer only for Direct. RTK Query currentData avoids displaying old rows under newly applied filter labels.

### What warrants a second pair of eyes

Browser tests use synthetic API data; real SQL, migrations, and runtime accounting are covered separately by PostgreSQL integration tests. Production deployment and licensed-font rendering were not exercised in this session.

### What should be done in the future

Deploy through the existing migration/rollout procedure, applying both pending migrations before starting the new binary. Historical calls whose instances were already deleted remain unavoidably unknown.

### Code review instructions

Start with analytics-data.ts and tests/analytics.test.ts, then analytics-charts.tsx, date-range-dialog.tsx, analytics.tsx, and the scoped catalog.css additions. Use the ticket's preview and browser-check scripts for reproduction. Review the updated Paper traffic export for the stroke correction.

### Technical details

No new runtime or test dependency was added. Top-three-plus-Other stacks preserve every filtered view. All calendar arithmetic is UTC, the date UI includes the selected end day, and transport remains half-open. Model reporting stays catalog-wide under traffic filters. Changes are not deployed.


### Step 6 checkpoint receipt

- Backend: `2f53554` — Add filtered analytics series and durable artifact usage.
- Frontend: `f072af7` — Implement analytics graphs, traffic filters and date range dialog.
- After the final width/layout correction, the three focused frontend contracts and all 18 Chromium checks passed again; the refreshed embedded binary build passed.
- The staged whitespace check caught a trailing blank line in the newly added browser evidence Markdown. The shell continued to the frontend commit; the final documentation checkpoint removes that whitespace. No runtime code was affected.
- The updated guide will be uploaded under a new v2 name to preserve the annotations on the original reMarkable upload.


### Final delivery receipt

- Updated intern/system guide and the Paper implementation guide were bundled without overwriting the original upload.
- Exact result: `OK: uploaded FRANCOIS-008 RSS and Analytics Implementation Guide v2.pdf -> /ai/2026/10/16/FRANCOIS-008`.
- This proves reMarkable cloud delivery, not physical-tablet synchronization.
- Temporary preview stopped through `lsof-who -p 18908 -k`; its tmux session ended with the process.
- Final documentation hygiene: docmgr doctor passed and trailing evidence whitespace was removed.
- All requested runtime features are implemented. Applying migrations and deploying production remain operator rollout actions, not work performed in this session.


## Step 7: Deploy the completed feature to the existing local Compose stack

The user explicitly requested local deployment after implementation and clarified the distinction from production. Inspection identified the existing francois-catalog Compose deployment at localhost:8090, running with Pi, persistent catalog/release/bootstrap volumes, and migrations 001–008. The configured local settings match the running container, so deployment can preserve the existing behavior and data.

### Prompt Context

**User prompt (verbatim):** "commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill) and deploy locally"

**Assistant interpretation:** Rebuild and update the existing local deployment, apply migrations 009/010, verify the actual service, and commit deployment documentation or fixes at meaningful checkpoints.

**Inferred user intent:** Make the implemented analytics available in the running local application rather than leaving it only in Git or a synthetic preview.

**Application revision:** `1d8a137`, including backend `2f53554` and frontend `f072af7`.

### What I did

- Read the Compose configuration, Dockerfile and startup/setup scripts, then inspected local containers and the migration ledger.
- Notified the user before Compose/runtime environment settings were read; printed only the nonsecret BACKEND/PUBLIC_URL/APP_DOMAIN/GRANT_TTL values and mount targets.
- Confirmed that running and desired settings match: Pi backend, localhost:8090 portal, apps.localhost instances, 30-minute grants.
- Prepared a private custom-format database backup at `/tmp/francois-008-local-predeploy.dump` and retained the old image as `francois-catalog:pre-analytics-1d8a137`.
- Started a Compose image rebuild in tmux with explicit local image revision/version labels.

### Why

Using the existing deployment preserves local accounts, instances, releases, provider configuration and licensed-font mounts. Running only the migration command avoids re-running unrelated identity/user provisioning.

### What worked

Inspection and preflight succeeded. Migrations 009 and 010 were identified as pending. Build and deployment results are appended below when verified.

### What didn't work

No failure observed during preflight.

### What I learned

The prior image did not record its source revision (label `unknown`); the new build will carry the application revision for a reviewable deployment receipt.

### What was tricky to build

The service uses host networking and reads its local Compose .env plus startup environment. Narrow comparison confirms the intended configuration without exposing secrets or changing the running Pi backend.

### What warrants a second pair of eyes

Production is outside this deployment request. Validation should exercise actual local session/analytics behavior without issuing billable model calls.

### What should be done in the future

Finish migration/recreation, confirm health and authenticated analytics, and append exact receipts. Production rollout remains separate.

### Code review instructions

Review deploy/compose/compose.yaml, Dockerfile, catalog-serve and the deployment receipt before reproducing the local rollout. Keep the database backup private; it contains application data and is not a repository artifact.

### Technical details

Current image: `sha256:afdf1c8cfe901d9c69edd1ca218274eda7fc42480ec63ba489c3491156990f4e`. Local PostgreSQL container: francois-catalog-postgres-1, catalog database francois_catalog. Build runs in tmux session francois-008-local-build; transient build output is in /tmp, not Git.


### Step 7 final rollout and validation receipt

The local rollout completed successfully. Compose rebuilt the application image, applied the two pending migrations, and recreated only the Francois service. The existing authenticated browser session continued working after recreation, and the real deployment passed the targeted browser checks.

- Image build succeeded; deployed revision label is `1d8a137`, image `sha256:9bba2f8f7ae5916c2b59ae74d803e346257442a81b8215fa142d6e7ec255702a`.
- Migration command reported `009_analytics.sql | applied` and `010_model_call_artifacts.sql | applied`.
- `/healthz` and `/readyz` both returned HTTP 200; readiness reports models available.
- Preserved row counts: 7 users, 13 instances, 5 releases and 5 model calls. All five historical calls were attributed: Polis IDE 4, Capability playground 1.
- Real local Chromium validation passed 18 checks: public pages, preserved admin session, authenticated report, exact direct/path filters, both full-stack reconciliations, anonymous denial, both RSS feeds, stack selectors, date preset/cancel, token/cost graphs, artifact totals and no page errors.
- Validation created six ordinary document-view events and made zero model calls. It did not modify application content, accounts, subscriptions or model settings.
- Container remains running with zero restarts; the completed tmux image-build session was closed. Compose continues managing the app at http://localhost:8090.
- No deployment failure or code fix was needed. Backup file permissions/header and the retained previous image were checked before migration.
- Reproduction script and exact deployment evidence are in `scripts/03-local-deployment-check.js` and `various/validation/04-local-deployment.md`.
- Production was not accessed. This session deploys locally only.

## Step 8: Investigate minimal public UI and six-lane operating policy

The user clarified that the site should feel intentionally minimal even though operators need usage tracking, fairness limits and the ability to pause capabilities. I read the Paper feedback and traced instance creation, live views, sampling, persistence and demo lifecycle to separate saved-workspace cost from active execution. The result is an evidence-backed next-phase design, with runtime changes explicitly left unimplemented in this advisory turn.

### Prompt Context

**User prompt (verbatim):**

> Ok, so the idea for the site is that it is intentionally minimal, and I put a bunch of comments on the paper design of what could go.
>
> But the main idea is that we have a free LLM hosting with infinity tokens, but restricted lane count (6 lanes), and we don't want to turn this site into a "product" where people really see usage or so. So instead, we want to still limit and track and maybe rate limit for all users, especially for free ones (maybe limited tour length, limited sessions per blabla), and also be able to take storage + llm usage functionality down / disable it, when we realize we are growing too fast or there are issues, and in this case we need to show a little warning message or page. Also, under these circumstances, how would you address "instances", and how much burden do they put on the infrastructure?

**Assistant interpretation:** Investigate the existing implementation and recommend simple private controls, restrained notices and an instance model appropriate for a six-lane shared service.

**Inferred user intent:** Preserve the site as a small collection of applications and writing while retaining operational control over growth, misuse and incidents.

**Prior commit:** `d9fadae` — Document verified local analytics deployment. No runtime implementation changes in this step.

### What I did

- Applied the diary and docmgr skills, read the current ticket checkpoint and inspected relevant code rather than assuming instances were containers.
- Loaded the Paper guide, inspected the unified file, and read all 20 open one-message comment previews. They include removal of most account/instance/pricing UI, merged feeds and a small warning-page reference. No design nodes or comment statuses changed.
- Traced `CreateInstance`, `Runtime.Handler`, `createView`, `sample`, `piworker.Start`, document/asset writes and demo creation/expiry. Checked worker output limits and model ledger retention.
- Inspected local aggregate resource/storage usage without reading environment values or user document contents.
- Added `design-doc/03-minimal-public-experience-and-operational-capacity-controls.md`, related seven primary source files through docmgr, and linked it from the ticket and original guide.
- Consulted the PostgreSQL explicit-locking documentation and RFC 9110 Retry-After semantics for proposed concurrency/error handling.

### Why

Saved workspace count, live browser views and occupied model lanes have different costs. Protecting the six lanes requires request admission and fair duration limits; counting instances or exposing credits to visitors does not establish that protection. Reversible capability controls allow a small site to stay readable during model or storage incidents.

### What worked

- Confirmed instance creation inserts shared PostgreSQL metadata and membership, with no per-instance worker/container. The Node worker is shared and application JavaScript runs in browsers.
- Confirmed existing demo TTL/capacity/network limits, transactional credit reservations, model counters, timeout/tool/output bounds, grants, revocation and retention provide reusable foundations.
- Measured local aggregate state: 13 instances, seven documents/606 logical bytes, zero assets, database size 10,401,459 bytes. One docker-stats snapshot reported app 85.15 MiB/0.00% CPU and PostgreSQL 89.58 MiB/0.29% CPU. This is an idle whole-container observation, not a load test or per-instance estimate.
- Identified gaps: global concurrency is process-local; connected view count is unbounded in the inspected path; document count checks can race; aggregate document/comment/lease bytes need limits; existing demo start limiting is process-local; model slots are held across tool waits; local cancellation release does not prove upstream completion.

### What didn't work

- The first `rg` invocation included unmatched `pkg/model*` and `pkg/runtime*` globs; zsh returned `zsh:1: no matches found: pkg/model*`. Replaced guessed directory globs with inspected concrete paths.
- Paper `list_comment_threads` with `previewLength: 1500` returned `maximum: 1000` / `Invalid input`. Retried with 1000 and received all 20 complete messages.
- Reads of guessed files `pkg/piworker/pi.go` and `pkg/catalog/launch.go` did not locate source; the latter reported `rg: pkg/catalog/launch.go: No such file or directory (os error 2)`. Used `rg --files pkg/piworker` and actual `server.go`/`client.go` paths to continue.
- Initial `docker stats --no-stream` failed at the sandbox boundary: `permission denied while trying to connect to the Docker daemon socket at unix:///var/run/docker.sock` / `connect: operation not permitted`. The same narrowly scoped aggregate read succeeded with approved escalation. No automatic-review rejection occurred.

### What I learned

The user values private operational accounting and a nearly invisible public service layer. Token volume is no longer the primary scarce-resource proxy. Existing model-call admission occupies a slot during the entire agent call; provider lane semantics and cancellation acknowledgement must be understood before promising a hard physical six-generation cap. A saved instance alone consumes neither a provider lane nor its own process.

### What was tricky to build

No runtime code was built. The design boundary is separating identity/persistence lifetime from access lifetime: an expiring model visit must not erase signed-in saved work, and refreshing a grant must not reset fairness allowances. Similarly, read-only storage must cover comments/assets/leases as well as documents while allowing healthy reads; a model pause should not unnecessarily close those views.

### What warrants a second pair of eyes

- Establish whether provider lanes represent generation, conversation or upstream sessions and whether this site owns all six.
- Review cancellation/termination before slot reuse, consistent quota locking, and guest-account/session cleanup.
- Decide existing extra-workspace reachability before hiding management UI. Do not silently merge or discard stored work.
- Choose initial retention and rolling duration/rate limits using representative applications; local idle measurements do not establish capacity.

### What should be done in the future

Implement the proposed controls in small phases: policy gates and shared notice; six-lane/fairness accounting; storage/view/start limits; implicit default workspaces and approved public UI reduction. Task `byyf` tracks the proposed implementation and remains open. No policy values or services were changed by this analysis.

### Code review instructions

Read the new proposal first, then its related source files and `pkg/catalog/hosted/rpc.go`, `pkg/catalog/store/assets.go`, `pkg/catalog/entitlement.go`, and `pkg/piworker/client.go`. Distinguish observed behavior, code defaults and tentative policy values. Documentation checks are appropriate for this checkpoint; no runtime test suite is required because runtime source/configuration are unchanged.

### Technical details

The default 5,000 documents times 256 KiB permit approximately 1.22 GiB of logical document bodies, plus 256 MiB of assets and database overhead. Existing SSE keepalive/rechecks are 15/30 seconds, model-call timeout is five minutes, browser-tool timeout is 150 seconds and per-round output cap is 8,192 tokens. Proposed independent controls target new activity, sampling and storage. Private capacity metrics should add occupied seconds, active calls, latency and rejection reasons to existing token/artifact accounting.

## Step 9: Rewrite the proposal as direct changes to the current application

The user rejected the unnecessary architectural possibilities introduced in Step 8 and the subsequent explanation. I rewrote the current design around the existing settings, handlers, counters, demo limiter and usage ledger. The document now gives a concrete settings object, the places to enforce it, and a short implementation sequence; it removes the speculative mechanisms instead of describing them as future additions.

### Prompt Context

**User prompt (verbatim):** "what was the service / queue / scheduling syste abouted? yeah this should be fucking straightforward bro. rewrite"

**Assistant interpretation:** Rewrite the design immediately, eliminating unnecessary architecture while retaining a technically useful implementation guide.

**Inferred user intent:** Straightforward changes to the application that already exists, with private limits and small notices.

**Prior commit:** `a6265e0` — Document minimal site capacity and service control design.

### What I did

- Rewrote `design-doc/03-minimal-public-experience-and-operational-capacity-controls.md` in place and preserved the source investigation in the earlier diary entry.
- Defined three operations settings plus plain notice text, using the existing admin settings GET/PATCH API and revision handling.
- Reused existing global/per-user call admission, daily/monthly call allowances, tour TTL/start limits and cleanup. Removed duration-budget reservations, distributed coordination, optional queues, priority scheduling, per-audience control rules and new model-only cancellation orchestration.
- Specified atomic resume/open using the existing per-owner lock; retained existing additional workspace identities and URLs.
- Kept storage checks local to the existing transaction methods, with a shared instance lock and logical-byte cap. Added initial view/tour/storage values as proposed policy, not claims about deployed settings.
- Updated task `byyf` through docmgr to reflect the simplified implementation, and recorded the rewrite in the changelog.

### Why

The current application already contains the mechanisms needed for a small hosted site. Explaining hypothetical scale-out designs distracted from the requested implementation and made the proposal appear much larger than necessary.

### What worked

The rewritten document maps each requirement to existing files and behaviors. It includes settings JSON, model-call pseudocode, notice copy, a small diagram and acceptance checks without requiring another runtime component. Runtime configuration, source and the local deployment remain unchanged.

### What didn't work

The earlier design included speculative architecture beyond the user's scope; the following chat answer repeated those concepts and caused confusion. This rewrite removes them. No tool failure occurred during the rewrite.

### What I learned

Technical detail should explain direct edits and their correctness, rather than enumerate possible future systems. Existing private credit allowances are sufficient as call-count safeguards; a new lane-time budget is unnecessary for this version.

### What was tricky to build

No runtime code was built. Simplification still needs correct boundaries: an AI pause permits running calls to finish; a storage pause applies to every persistent capability; quota reductions preserve readable existing data; and simultaneous Open requests must reuse the same workspace.

### What warrants a second pair of eyes

Check cancellation termination before slot reuse and complete coverage of storage mutations/signed asset reads. Initial 20-minute tours, three starts/hour, three views, 500 documents and 16 MiB per-instance storage are proposed configurable starting values.

### What should be done in the future

Implement the simplified design through the existing handlers and settings. Create the shared warning variants in Paper before UI implementation. Task `byyf` remains open; this turn is the requested documentation rewrite.

### Code review instructions

Read the rewritten design and task text. Verify that requirements are explained through existing mechanisms and that the old speculative options do not remain in the current design. Run `docmgr doctor --ticket FRANCOIS-008 --stale-after 30` and `git diff --check`; runtime tests are unnecessary for this documentation-only change.

### Technical details

The proposed operations object contains `acceptNewVisits`, `samplingEnabled`, `storageMode` and `notice`. Model concurrency uses existing `Allowance` fields; limits use existing plan/demo allowances plus `Demo.PerUserPerHour`, `Limits.ViewsPerUser` and `Limits.StorageBytesPerInstance`. Existing model-call timestamps support private duration inspection without a new accounting subsystem.

## Step 10: Open the design in md-view and verify the desktop window

The requested viewer detached successfully but then crashed during GTK initialization. My first response incorrectly treated process launch as a visible window. After the user questioned it, I checked the log, identified the missing display setting, relaunched against the active X session and verified the actual window before bringing it forward.

### Prompt Context

**User prompts (verbatim):** "open the doc with md-view view"; "did it not work?"

**Assistant interpretation:** Open the rewritten design document and diagnose the missing viewer window.

**Inferred user intent:** Read the current design in the desktop Markdown viewer.

**Prior commit:** `fed4b07` — Simplify capacity design to existing settings and handlers.

### What I did

- Ran `md-view view` with the current design document path.
- Checked detached PID 964880 and its launch log after the user reported the missing window.
- Notified the user before reading the display-related environment allowlist. DISPLAY and XAUTHORITY were unset; the active local Xorg session uses display `:0` and `/run/user/1000/gdm/Xauthority`.
- Relaunched with `DISPLAY=:0 XAUTHORITY=/run/user/1000/gdm/Xauthority md-view view ...` using the same document path.
- Verified PID 978852 was running and `wmctrl -lp` listed `md-view: Minimal public experience and operational capacity controls` for that PID, then used `wmctrl -i -R` to bring that window forward.

### Why

A detached-launch success only proves the child was started. A GUI command also needs access to the graphical session and verification after initialization.

### What worked

The retry remained running and had an actual desktop window. Existing X authentication was used without printing its contents. No application source, runtime policy or design content changed.

### What didn't work

- The initial sandboxed launch reported `create background log: open /home/manuel/.cache/md-view/launch-4287680827.log: read-only file system`. Escalation permitted the requested viewer's cache write.
- Detached PID 964880 exited. Its log reported `panic: failed to init GTK`. The calling environment lacked DISPLAY; setting the verified display/auth paths fixed startup.
- Unescalated `loginctl list-sessions --no-legend` reported `Failed to connect to bus: Operation not permitted`; the scoped escalated session inspection succeeded.
- A first window-list check omitted the display override and reported `Cannot open display.` Repeating it with the same display/auth settings as the viewer succeeded.

### What I learned

This command environment does not inherit the desktop DISPLAY even though a local X session is active. Check both process survival and window presence before reporting that a GUI document has opened.

### What was tricky to build

No code was built. Detached execution hid the GTK startup failure from the parent command; the launch log supplied the missing evidence.

### What warrants a second pair of eyes

N/A. Window title and PID were verified on the active desktop session.

### What should be done in the future

Use the verified desktop display for subsequent viewer launches and confirm window creation.

### Code review instructions

This is a diary-only checkpoint. Review the launch-versus-window distinction and failure evidence; no runtime tests are required.

### Technical details

Successful launch log: `/home/manuel/.cache/md-view/launch-1245874169.log`. Its WebKit signal-handler notice did not prevent the process/window from running. The design remains the document committed in `fed4b07`.

## Step 11: Implement settings, runtime controls and bounded saved work

The user authorized implementation of the simplified design. I kept the work in the existing Go service, PostgreSQL store and Pi worker. This checkpoint adds operation settings, six-call/one-user admission, tour/workspace reuse and transactional storage limits. Public UI wiring follows in the next checkpoint.

### Prompt Context

**User prompt (verbatim):** "Ok, implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

**Assistant interpretation:** Implement the rewritten design completely, maintain this detailed diary and commit at meaningful integration boundaries.

**Inferred user intent:** Working private operational controls and a minimal public experience, using the current application mechanisms.

**Prior commit:** `4609126` records the viewer recovery; design checkpoint is `fed4b07`.

### What I did

- Added operations settings and validation, reduced defaults to six global calls/one per user, 20-minute tours, three views, 500 documents and 16 MiB aggregate instance payloads.
- Added migration 011 to explicitly transform stored settings into the new strict shape, following migration 005's existing schema-transition convention. It preserves plans, user data and model accounting; configured concurrency/document/TTL caps become no greater than the new defaults.
- Added runtime sampling/storage gates, including signed blob reads, and a view cap under the runtime mutex. Store mutation checks also enforce storage mode directly.
- Added `POST /api/instances/open` for atomic default-workspace resolution under the existing owner lock. Explicit creation remains its existing operation and obeys the new-activity pause. Existing workspace identities, release pins and URLs remain intact.
- Added live-tour lookup/reuse without expiry extension, with a repeated lookup under demo admission locking and a per-user key in the existing in-memory start limiter.
- Added shared per-instance transaction locking and aggregate logical-byte accounting across documents, assets, thread anchors/comments and leases. Cleanup/shrinking remains possible after a quota reduction.
- Made the worker emit an `ended` frame from its finalizer. The Go client holds admission until that frame, including after cancellation; an unresponsive worker is stopped after five seconds. Worker call IDs are now private random IDs so caller-supplied IDs cannot collide across users.
- Created the Paper page “Minimal site — service notices,” with a full-page busy notice and inline variants, before UI coding. Reviewed screenshots and exported exact typography/spacing. Kept the existing white/black ABC Areal aesthetic.

### Why

The existing settings, mutexes, transactions and model ledger supply the necessary mechanisms. Correct lifetime boundaries matter more than additional infrastructure: do not refresh a tour's expiry, duplicate a default workspace, acknowledge rejected storage writes or release a model slot before termination.

### What worked

- `go test ./pkg/catalog/settings ./pkg/piworker` passed, including cancellation waiting for worker completion.
- Existing catalog integration tests passed against a disposable PostgreSQL 17 container on port 15438, with isolated databases created by the test harness. This includes the real migration runner.
- Targeted new integration tests passed: pauses affect existing views, reads survive read-only mode, signed URLs obey storage-off, paused calls create no ledger row, direct store writes obey pause, concurrent storage growth admits only fitting writes, quota reductions permit shrinking/deletion, and concurrent workspace/tour reuse preserves identity/expiry.
- Paper screenshot review: consistent spacing, readable type, restrained contrast, aligned text, no clipping; a single warning treatment rather than additional product UI.

### What didn't work

The first broad `go test ./pkg/catalog/... ./pkg/piworker` could not write `/home/manuel/.cache/go-build/...`: `read-only file system`. Scoped escalation to use the existing Go cache resolved it. The first successful broad run without `TEST_DATABASE_URL` established compilation/unit behavior only; a subsequent run explicitly set the disposable test URL and executed the database integration tests.

### What I learned

The worker protocol needed termination acknowledgement, rather than returning on the UI result or sending cancellation and immediately releasing a slot. Also, preserving explicit instance creation while giving the normal Open action an idempotent endpoint avoids changing the semantics of existing management operations.

### What was tricky to build

All persistent writers must take the same instance lock for the aggregate quota to be meaningful. Quota checks occur in the mutation transaction and include replacement deltas; failed growth rolls back the inserted content. The existing credit race can now reject at either the concurrency gate or the allowance gate, so its assertion accepts either rejection while requiring exactly one admitted call.

### What warrants a second pair of eyes

Review cancellation acknowledgement/fail-stop behavior, all storage mutation paths, migration 011's explicit policy defaults and signed asset policy checks. The provider's remote execution contract remains external; local acknowledgement proves worker termination, not an independently measured provider-side session count.

### What should be done in the future

Finish the existing settings editor, shared runtime notices/autosave behavior, automatic Open action and public simplification. Run affected frontend and full integration validation, then update local deployment and documentation.

### Code review instructions

Start with settings, migration 011, `store/storage.go`, `operations_test.go`, `hosted/sample.go` and `piworker/client.go`. Tests use disposable database URL `postgres://francois:francois-test@127.0.0.1:15438/francois_test?sslmode=disable`; this is test-only data and credentials. No live site settings have been changed at this checkpoint.

### Technical details

Paper page: https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-D-0. Global controls are `acceptNewVisits`, `samplingEnabled`, `storageMode` and optional notice. Sampling pause drains existing calls. Tour start counters remain process-local. The model-call ledger remains private and authoritative for call/token accounting.

## Step 12: Wire controls and simplify the public experience

The backend checkpoint is committed as `ad5f791`. This checkpoint wires the existing settings editor, shared notice treatment and automatic Open action, and removes the public usage/account management presentation. Applications and writing now share one chronological page stream and one primary RSS subscription.

### Prompt Context

**User prompt:** See Step 11.

**Assistant interpretation:** Finish the approved minimal UI and operational controls without adding infrastructure.

**Inferred user intent:** Ordinary visitors see applications and writing, while operators retain limits and incident controls.

**Commit (backend):** `ad5f791` — Add simple operation controls and bounded hosted workspaces.

### What I did

- Added the three operation controls and notice field to the existing admin settings editor, plus view/aggregate-storage/user-tour limits in their existing settings sections.
- Added a shared plain-text notice builder, reused for public action failures and hosted runtime notices. Paused storage rejects further autosave mutations locally after the first server rejection, rolls failed optimistic writes back through the existing database client, and clearly says changes cannot be saved.
- Changed normal Open to the atomic `/api/instances/open` endpoint. Removed the new-instance wording, duplicate demo action and public plan upsell from the application action.
- Reduced Account to subscription management/sign out, keeping operator API-token management available to staff. Removed the launcher's instance-management toolbar/panel while preserving saved instance URLs and a Close link.
- Removed hero/Made by sections from the front page, merged application and journal lists by publication time, and added `/feed.xml` with published listed releases and published posts. Existing specialized RSS endpoints remain ordinary feeds.
- Removed release-history/plan badges from application presentation while preserving authored content, entitled downloads, attachments and editor draft previews. Reduced the existing pricing destination to a small Support page without allowance/instance comparisons; removed it from normal navigation.
- Exported the Paper notice typography/spacing and corrected text nodes to explicitly use ABC Areal, then reviewed the inline variants again. No clipped or misaligned content was found.
- Rebuilt the repository's tracked embedded frontend bundles. Kept unrelated compact source formatting intact rather than including a broad reformat.

### Why

The site's routine flow should be Open/Try and read. Incident controls belong in the existing operator settings; a single short notice is enough for visitors. Pausing saving must be truthful even when an authored app keeps requesting autosaves.

### What worked

- `pnpm run check`, all eight existing frontend contract/chart tests and `pnpm run build` passed.
- Combined feed integration tests passed: published post/release ordering, site-specific links, hidden/draft exclusion and merged HTML content on the public listing routes.
- Public tests still verify entitled downloads, draft isolation and billing/access metadata while checking the reduced visible presentation.
- Full `go test ./...` passed with real isolated PostgreSQL after correcting the timing-sensitive demo test. `go build ./...` passed. `golangci-lint run -v` reports zero issues.

### What didn't work

- The broad suite initially failed `TestPlans` at `plans_test.go:145: artifact plan badge`: the old assertion required the visible “Studio plan” badge deliberately removed by this design. It now checks retained access metadata and absence of that badge.
- The full suite later exposed `demos_test.go:103: guest call 2: 429 ... code:rate_limited`. Sequential allowance assertions had assumed the preceding asynchronous call had already released its slot. Their helper now waits on that transient concurrency rejection; `TestDemos -count=3` passes and independent concurrency tests still assert the strict cap.
- Lint found three `ST1013` numeric HTTP-status literals in the combined feed and one `QF1008` redundant embedded-field selector in the worker client. Replaced the literals with `http.StatusServiceUnavailable` and `wire.Request.CallID` with `wire.CallID`; lint then passed.
- The local shell uses Node 20 while package metadata asks for Node 24. Checks/tests/build succeeded with an engine warning; the deployment Dockerfile builds and runs with Node 24 and will provide the production-shaped build verification.

### What I learned

Keeping explicit creation separate from the normal Open action preserves existing workspace data and management API semantics while simplifying the public flow. A shared RSS feed also removes the need to present two separate subscription choices on the minimal page.

### What was tricky to build

Authored HTML runs on an isolated origin, so runtime notices cannot assume portal CSS or navigate a portal-relative back link inside their iframe. The shared notice is styled from the exported design and the trusted launcher supplies Close outside the frame. Reads remain usable during read-only mode; future writes stop after a rejected save until the person reloads to retry.

### What warrants a second pair of eyes

Review the shared error/notice handling and preservation of access checks while reducing visible plan information. The storage byte cap counts logical payloads rather than PostgreSQL physical/index overhead.

### What should be done in the future

Deploy the completed code locally, apply migration 011 after a backup, verify the real settings UI and paused runtime behavior, and record the exact rollout/browser results. Production remains outside scope.

### Code review instructions

Start with `ui/catalog/admin.tsx`, `service-notice.ts`, `public.ts`, `account.tsx`, `instances.tsx`, `ui/src/runtime.ts`, `pkg/catalog/pages.go`, `feed.go` and `operations_test.go`. Generated embedded JavaScript is intentionally included because this repository tracks its browser assets.

### Technical details

Pause settings are saved through the existing versioned GET/PATCH administration API with If-Match. `/feed.xml` excludes hidden artifacts and draft posts/releases and uses stable GUIDs. No changes to provider credentials or live accounts/data were needed for tests.

## Step 13: Deploy locally, verify the actual runtime and deliver the guide

The completed application was rebuilt from `42a1f3f` using the existing Dockerfile's Node 24 build/runtime. After a private database backup, migration 011 applied and only the local application container was recreated. The real deployment passed 21 browser checks, including temporary service pauses and settings restoration, with no new provider call or application document.

### Prompt Context

**User prompt:** See Step 11; continuing the previously authorized local deployment and requested reMarkable delivery.

**Assistant interpretation:** Complete actual local rollout and user-visible verification, then record/deliver the implementation evidence.

**Inferred user intent:** Working code available locally, reviewable commits and a trustworthy implementation diary.

**Commits (code):** `ad5f791` — Add simple operation controls and bounded hosted workspaces; `42a1f3f` — Keep public hosting minimal with shared service notices.

### What I did

- Notified the user before Compose read its existing environment file; no provider credentials or auth-file contents were printed.
- Recorded baseline counts, made `/tmp/francois-008-pre-controls.dump` (custom-format PostgreSQL, mode 0600, 99,232 bytes), and tagged the previous image `francois-catalog:pre-controls-42a1f3f`.
- Built in tmux, ran the image's catalog migration command, and recreated only the app without rebuilding identity/database services or removing volumes.
- Verified health/readiness, image/revision and zero restarts, then used the preserved authenticated local browser session to exercise the real settings editor and isolated runtime.
- Added `scripts/04-local-controls-browser-check.js`, a detailed implementation/local rollout receipt, and reviewed screenshots of the actual controls/home/staff Account.
- Added focused integration coverage proving that changing IP addresses cannot bypass a signed-in user's tour-start limit and parallel inserts cannot overrun document count; both passed.
- Updated the production runbook and ticket guides/index/task status. Task `byyf` is complete.
- Used the remarkable-upload skill to upload the current guide plus rollout receipt under a new name, preserving earlier annotated uploads.
- Closed the build tmux session and removed the disposable integration-test PostgreSQL container; the real local site remains running.

### Why

Browser verification catches the boundary between portal settings, existing grants, authored-app capability access, runtime error handling and save behavior. Keeping a matching pre-migration dump matters because old images reject the explicit new settings fields.

### What worked

- Image build completed under Node 24. Deployed image is `sha256:b1ee8e4a1040f2413f42379e51c957799fb229af77b12188f260e6ec61fb36ab`, revision `42a1f3f`.
- Migration output: `011_operations.sql | applied`.
- `/healthz` and `/readyz` returned HTTP 200; readiness reports models true. Container is running with zero restarts.
- All 21 real-browser assertions passed: controls visible/saved through revision checks, six/one concurrency policy, minimal launcher, sampling refused before provider use, read-only writes refused, repeated autosaves stopped locally, explicit unsaved text, plain-text operator notice, healthy reads, no persisted rejected write, same-workspace resume while new activity is paused, minimal Account/navigation/home, valid combined RSS, restored controls and no browser page errors.
- Independent final counts equal baseline: seven users, 13 instances, five releases, five model calls and seven documents. Browser checks changed only temporary settings revisions/audit history and ordinary traffic/grant records.
- Final operations policy restored to acceptNewVisits true, samplingEnabled true, storageMode read-write and empty notice.
- Additional targeted tour/document concurrency tests passed against disposable PostgreSQL. Earlier full suite/build/lint/frontend receipts remain applicable because subsequent source additions were tests only.
- Exact delivery result: `OK: uploaded FRANCOIS-008 Minimal Hosting Implementation Guide.pdf -> /ai/2026/10/16/FRANCOIS-008`. This proves reMarkable cloud delivery, not physical-tablet synchronization.

### What didn't work

- The rollout command chained stop, required port cleanup and startup. After Compose stopped the app, `lsof-who -p 8090 -k` reported `no process listening/bound on port 8090` and exited nonzero, so startup had not run. Issued startup separately immediately; curl's first connection-refused attempt retried during normal startup and health/readiness then passed.
- The browser check first used Locator.contentFrame as a Frame, producing `TypeError: frame.waitForFunction is not a function`. Switched to ElementHandle.contentFrame.
- Its next attempt waited for `window.claude.db` and timed out after 30 seconds. The actual API is `claude.use("db")` and `claude.use("sample")`; corrected the check to use the existing capability contract. The check's finally block restored settings and closed its temporary tab on both failures. No application fix was needed, and model-call counts remained five.

### What I learned

The real browser verified the intended minimal flow without provider traffic. Operational controls are independent: pausing model access does not disable healthy reads, and pausing new activity does not prevent resuming saved work. The worker completion boundary and save acknowledgement remain the two important correctness boundaries.

### What was tricky to build

Existing browser tabs can keep valid grants across a settings change; policy enforcement therefore occurs at each affected operation and persistent mutation. The browser's second save attempt was rejected locally after the first 503, demonstrating that the common runtime prevents repeated autosaves while leaving reads available.

### What warrants a second pair of eyes

The remote provider's exact lane semantics are still its external contract; the implementation enforces six admitted application calls in this service and waits for worker completion. Backup plus matching image are needed for rollback across the strict settings migration. No production rollout was performed.

### What should be done in the future

Tune existing private allowance/start/resource values from normal local/operator observation. The requested implementation, local deployment, diary and guide delivery are complete.

### Code review instructions

Read the implementation/local rollout receipt and reproduction script, then the current design guide and code commits. Screenshots show real local data but contain no credentials/token secrets. The app is at http://localhost:8090/ and controls are at /admin/settings. The private database dump stays outside Git.

### Technical details

The combined live RSS contained eight items. Signed-in tour counters and network counters remain process-local. Storage quotas measure logical payload bytes, not physical PostgreSQL overhead. Existing instances/releases/account data and provider/font mounts were preserved.

## Step 14: Publish the pull request and wait for code review

The user requested a PR and review wait. The prior working branch belonged to the already-merged FRANCOIS-007 PR, so I refreshed origin/main and published the completed FRANCOIS-008 commits under their own branch. GitHub automatically started Codex review when the ready PR opened.

### Prompt Context

**User prompt (verbatim):** "ok,  let's open a PR and wait for code reviews"

**Assistant interpretation:** Publish the completed work, provide review context and monitor the repository's code reviews/CI.

**Inferred user intent:** Get an external review of the implementation before deciding the next action.

**Implementation head under review:** `aa9ca58b90969803d28588538947d328902a01d4`.

### What I did

- Checked working-tree cleanliness, remotes, branch and existing PR status; FRANCOIS-007 PR #6 was already merged.
- Ran `git fetch origin`, which refreshed main to `64c7ece`. Confirmed the remaining diff contains the FRANCOIS-008 work rather than the already-merged scoped-token feature.
- Created and pushed `feat/francois-008-minimal-hosting` without rewriting the existing commits.
- Wrote the exact PR body to `/tmp/francois-008-pr-body.md`, then used `gh pr create --base main --head feat/francois-008-minimal-hosting --body-file ...` with the title “FRANCOIS-008: analytics, RSS and minimal hosting controls.”
- Included final behavior, validation, migration/rollback requirements, local-only deployment and guide/diary starting points in the PR description.
- Checked the new PR's review and CI state. Monitoring results are appended below.

### Why

A branch named for the current ticket and an accurate base keep reviewers focused on the new changes. Existing implementation/validation evidence should accompany the review rather than requiring the reviewer to reconstruct this conversation.

### What worked

- PR opened: https://github.com/go-go-golems/francois/pull/7.
- GitHub's Codex review summary reports code review running on `aa9ca58`, automatically triggered by PR creation.
- Initial CI includes Analyze, Dependency Review, TruffleHog, frontend check, Go test/lint, release build and vulnerability/security scans.

### What didn't work

No publication failure observed. The first comparison used a stale local origin/main; refreshing it before publication correctly excluded the already-merged FRANCOIS-007 scope.

### What I learned

This repository automatically runs Codex review when a ready PR opens; no manual reviewer ping or comment was necessary.

### What was tricky to build

No code was built in this step. Preserve the reviewed source head while the review runs; collect the publication/review diary locally and commit the receipt afterward.

### What warrants a second pair of eyes

The PR description calls out migrations 009–011 and strict-settings rollback. Await the actual external findings rather than treating passing local checks as approval.

### What should be done in the future

Wait for review completion, inspect any inline findings and CI failures, and record their exact status. Do not merge without a user instruction.

### Code review instructions

Review PR #7; start with the current minimal-hosting design and implementation receipt in this ticket. Source and validation commits are preserved individually for review.

### Technical details

Base: main at `64c7ece`; head branch: `feat/francois-008-minimal-hosting`; original source head: `aa9ca58`. The existing local app remains on revision `42a1f3f` and is independent of this GitHub publication.

### Step 14 final review and CI receipt

- Codex review completed on `aa9ca58` at `2026-10-16T17:45:22Z` with two P2 findings. PR remains open and unmerged.
- Workspace selection can repeatedly return a workspace pinned to a withdrawn release that current policy forbids running: https://github.com/go-go-golems/francois/pull/7#discussion_r4297572224.
- Concurrent signed-in Try requests consume multiple start-limit hits before the locked store coalesces them into one tour: https://github.com/go-go-golems/francois/pull/7#discussion_r4297572241.
- Source inspection confirms the two boundaries: `OpenWorkspace` lacks pinned-release usability filtering, and the tour handler consumes limiter hits before `CreateDemo`'s atomic reuse decision. Findings remain unresolved; the subsequent user request is the vault report, not a claim that review fixes have landed.
- Frontend, Go tests, lint, vulnerability checks, GoSec, CodeQL and image publication succeeded. The GitOps PR step was skipped; no production deployment was performed.
- Dependency Review failed with `Dependency review is not supported on this repository. Please ensure that Dependency graph is enabled` (run `38941532816`). This is a repository capability/configuration failure, not a reported vulnerable dependency.
- TruffleHog failed with `Found unverified URI result with HTML encoding` and exit code 183 (run `38941532847`). Its summary reports zero verified secrets and one unverified secret. The available log did not identify its exact source. It remains an unresolved scan finding; zero verified secrets does not mean the check passed.
- The scanner's echoed shell contains a generic BASE/HEAD-equal error branch, but the actual scan used distinct revisions. That echoed diagnostic is not the cause of this failure.
- No runtime source, provider configuration or deployed policy was changed while waiting for review.

## Step 15: Write and publish the Obsidian technical project report

The user requested a detailed technical blog post in the go-go-parc vault, using the textbook skill and no analogies. I wrote a new dated report from the ticket's complete diary, current design, source code, tests, deployment receipts and completed external review. The report and five copied figures were committed and pushed to the vault's main branch without changing earlier Francois notes.

### Prompt Context

**User prompt (verbatim):**

> write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).
> Commit and push the bsidian vault when done (go-go-parc vault).

**Assistant interpretation:** Produce a substantial prose-first technical report with definitions, a concrete application trace, code/SQL, diagrams, implementation evidence and limits; save it in the current vault and commit/push only its note and assets.

**Inferred user intent:** A durable technical explanation of the completed work and its actual review/deployment state, suitable for future readers learning the system.

**Commit (vault):** `afc8140c310f1f96e71e0b0eeaf82329b9db0af0` — Document Francois private analytics and minimal hosting in technical deep dive.

### What I did

- Applied `pr-deep-dive-report`, `textbook-authoring` and `obsidian-vault-writing`; used the explicit go-go-parc destination over the older report skill's vault path.
- Read all FRANCOIS-008 diary entries in bounded chunks, the current architecture/implementation guides and deployment receipt, and actual analytics, storage, admission, worker, workspace/tour and browser-runtime code.
- Read the existing Francois storefront report and vault exemplars for continuity. Checked applicable ancestor instructions and confirmed go-go-parc was clean on main and synchronized with origin after fetching.
- Traced one illustrative Polis request through document-load measurement, Open, launch, model admission/accounting, and an existing-view read-only rejection. Defined artifact/release/instance/view/grant/call before relying on them.
- Wrote a 6,634-word, 489-line report with three Mermaid diagrams, real Go/TypeScript/SQL excerpts, an API example, source maps, test evidence, local rollout/rollback semantics and concrete follow-up work.
- Viewed all five source screenshots before copying them to the note's dated `_assets/` folder. Captions explicitly distinguish synthetic analytics fixtures, older navigation in those captures and the actual local deployment.
- Inspected completed Codex review and CI logs, confirmed the two findings against source, and represented them as unresolved rather than claiming full approval or green CI.
- Created only `Projects/2026/10/16/PROJ - Francois - Private Analytics and Minimal Hosting - A Technical Deep Dive.md` and five `francois-008-*.png` figures in the vault. Preserved previous historical notes and unrelated assets.
- Validated YAML, balanced code fences, all pinned GitHub source paths with `git cat-file -e`, reference-link definitions, both vault wikilinks and every relative image target. Checked for prohibited analogy/filler phrases.
- Staged exactly the six intended files, reviewed the staged stat/numstat and whitespace, committed, then pushed with `git push origin main`.
- Recorded review/CI follow-ups as docmgr tasks `t41l`, `c74u` and `nr27`, updated the ticket changelog and ran doctor successfully. These are new review tasks; completed implementation checkpoints remain historical facts.

### Why

The system's important decisions concern different data and execution lifetimes. A report organized around those boundaries teaches the implementation more effectively than a file inventory or chronological changelog. A pinned source revision and explicit review snapshot keep the report truthful as subsequent fixes arrive.

### What worked

- Vault report created at `/home/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/10/16/PROJ - Francois - Private Analytics and Minimal Hosting - A Technical Deep Dive.md`.
- All source/link/figure/metadata checks passed. The note exactly matches the inspected draft.
- Vault commit contains one new note and five PNGs, with no workspace state or historical edits.
- Push receipt: `090ac0f..afc8140 main -> main`; final vault status is clean and synchronized with `origin/main`.
- Earlier runtime validation remains accurately cited; no unnecessary application test rerun was performed for this report-only milestone.
- `docmgr doctor --ticket FRANCOIS-008 --stale-after 30` and `git diff --check` passed before the ticket documentation commit.

### What didn't work

No vault write, validation, commit or push failure occurred. Some batched reading output was truncated; narrower follow-up reads recovered the relevant source and diary sections before writing. The two failing CI jobs and two unresolved code-review findings are recorded in Step 14 and the report rather than characterized as publication failures.

### What I learned

The external review identifies limits of the existing tests: atomic tour reuse can pass while start accounting is wrong, and active-workspace reuse can pass while release usability is wrong. These are separate predicates and require separate regression cases. The current report keeps both limits visible.

### What was tricky to build

Historical analytics screenshots show older navigation and synthetic totals. Copying them without captions would imply the current deployed interface or real usage. The report labels that evidence explicitly and pairs it with actual local controls/home captures. Provider lane semantics likewise remain external; the article states the process-local admission and worker-completion guarantee precisely.

### What warrants a second pair of eyes

Review the two PR findings, the unidentified unverified URI scan result and the repository's Dependency Review configuration before merging. The report documents existing behavior and proposed corrections without claiming they have been implemented.

### What should be done in the future

Address the two P2 findings and CI follow-ups in a subsequent implementation checkpoint. The requested vault report is complete and pushed; production rollout and PR merge have not been performed.

### Code review instructions

Read vault commit `afc8140`, especially Sections 9–12 for lifetime/transaction behavior and Section 16 for external findings. The note links source at `aa9ca58`; its five relative figures resolve within the same dated vault directory. Verify continuation work against current source rather than treating this historical snapshot as a later fix receipt.

### Technical details

Vault remote: `ssh://git@github.com/go-go-golems/go-go-parc`; branch main. Report source snapshot: `aa9ca58`; local app remains `42a1f3f`. The new report has no database dumps, credentials, raw scanner payloads or cross-repository image dependencies. No messages were sent to reviewers or other users.


## Step 16: Address review findings and prepare the production release

The user authorized fixing the next review issue, merging and production deployment. Both outstanding P2 findings affect the normal Open/Try flow, so I fixed them before preparing the release. Production remains unchanged at this checkpoint; the established in-cluster backup completed before migration.

### Prompt Context

**User prompt (verbatim):** "address next code review issue, then merge, and deploy to prod."

**Assistant interpretation:** Correct the outstanding reviewed behavior, validate it, merge PR #7 and deploy through the existing production GitOps workflow.

**Inferred user intent:** Finish the review-to-production lifecycle, preserving saved work and keeping implementation straightforward.

### What I did

- Read the production runbook, image/GitOps workflow, settings authorization and the exact workspace/tour selection paths. Confirmed production currently uses `ghcr.io/go-go-golems/francois:sha-64c7ece`, one ready replica.
- Changed `OpenWorkspace` to preserve the workspace's identity/data and re-pin it to the requested published release only when its old pin cannot run under publishing policy. Allowed withdrawn pins remain unchanged. Recovery works at the instance-count cap and while new creation is paused.
- Serialized tour resume/check/create using the existing process-local limiter mutex. Limit checks no longer spend hits; successful creation records user/network starts afterward. Repeated requests resume without spending starts, and failed capacity admission spends none.
- Added `TestOpenWorkspaceRecoversWithdrawnPin`, `TestConcurrentTourRequestsSpendOneStart` and `TestFailedTourCreationDoesNotSpendStart`. The tests cover preserved data, permitted old pins, restrictive policy, real launch, concurrent requests and remaining start allowance.
- Reproduced the secret scan privately with the same scanner image. Its sole URI finding is the deliberate referrer fixture at analytics_test.go:27, not a production credential. Added an explanatory comment and a narrow `trufflehog:ignore` on that line, as supported by the upstream scanner documentation. Historical commits still contain the old fixture until scanned as the merged change; no broad detector/path exclusion was added.
- Enabled the repository's dependency vulnerability alerts; the dependency-graph SBOM API then became available. This addresses the missing repository capability without disabling the review job.
- Started disposable PostgreSQL 17 on loopback port 15438. Notified the user before using the existing TEST_DATABASE_URL harness, then ran the focused regressions and full suite.
- Used the existing production PostgreSQL backup CronJob through manual Job `postgres-backup-francois-008-20261016`, keeping data in the established cluster/object-storage backup destination. The job completed successfully before rollout.

### Why

A new workspace is unnecessary when only the old release pin is unusable: reusing its identity preserves data and works even when the account has no room for another instance. Tour start charging must follow successful creation; holding the existing mutex across resume/check/create makes that decision consistent in the documented single-process deployment.

### What worked

- Four focused existing/new workspace/tour checks passed against PostgreSQL (`pkg/catalog 0.938s`).
- The full `go test ./... -count=1` passed with the explicit isolated test URL after the known unrelated upload-race failure described below (`pkg/catalog 9.995s`).
- `golangci-lint run -v` passed with zero issues after the local expression correction.
- The final focused run including failed-creation accounting passed (`pkg/catalog 0.773s`). Later edits were tests and the fixture comment only.
- Production cluster access and existing backup workflow work. Manual backup Job reports Complete; final merge/build/rollout receipts follow in the next step.

### What didn't work

- First full suite failed the already-recorded `TestConcurrentNewArtifactUpload`: `studio_test.go:226: race-4 upload 1: 409 ... code:version_exists`. This predates the review fixes; the next complete run passed. No unrelated upload code/assertion was altered.
- Lint first reported `QF1001: could apply De Morgan's law` for the release usability predicate; rewrote it directly and lint passed.
- `gh pr view --json baseRefOid` is unsupported by the installed gh version; used REST `.base.sha` instead. zsh rejected an unquoted jsonpath-style custom-columns argument containing `[0]`; quoted it and verified the deployment.
- Automatic approval review rejected exporting the full production database to local `/tmp`, because it may contain sensitive user data and the deployment request does not specifically authorize that transfer/destination. Did not retry or bypass the export. Used the existing in-cluster backup CronJob as the safer alternative, which completed without transferring the dump locally.

### What I learned

The review exposed separate correctness properties: successful workspace selection must also yield a usable release, and atomic tour identity does not imply atomic start accounting. A narrow synthetic-fixture scanner exception is appropriate; hiding an entire test file or disabling URI scanning would be excessive.

### What was tricky to build

Tour requests must be serialized before their live lookup, not just inside store creation, so concurrent handlers cannot all spend a start before finding the same committed tour. Persistent workspace recovery stays under the existing per-owner transaction lock and keeps document rows/hostname unchanged.

### What warrants a second pair of eyes

Review automatic re-pinning only for unusable releases and preservation of allowed pinned releases. The tour mutex is process-local, matching the existing limiter and single-replica deployment. The legacy upload race remains a separate intermittent test failure.

### What should be done in the future

Push fixes, inspect the refreshed CI, merge PR #7, wait for its image/GitOps PR, validate and merge that deployment change, then verify production image, migrations, health, feeds and aggregate preservation. Do not treat workflow image publication alone as a successful rollout.

### Code review instructions

Start with store/instances.go, instances.go, demos.go and the three regression cases in operations_test.go. The scanner raw result stays private under /tmp; source verification identifies only the dummy test referrer. Read Step 17 for actual release results.

### Technical details

Test database credentials are disposable local fixtures. Production backup uses the pre-existing CronJob's own secret mounts/environment internally. No secret values, production dump or raw scanner results are included in Git. The vault report remains the historical snapshot published before these review fixes.


## Step 17: Clear the additional expired-lease review finding

A third P2 review finding arrived while the first two fixes ran through CI. Expired leases were included indefinitely in logical storage totals. I added cleanup to the existing locked write transaction, so live leases remain charged and expired payloads stop consuming capacity when an instance next writes.

### Prompt Context

**User prompt:** See Step 16; continuing review fixes before the authorized merge and production rollout.

**Assistant interpretation:** Address the newly arrived storage review issue before releasing.

**Inferred user intent:** Deploy reviewed, verified behavior rather than leave a known quota defect.

**Commit (prior fixes):** `4ac3e59` — Fix default workspace recovery and concurrent tour start accounting.

### What I did

- Read the new finding at https://github.com/go-go-golems/francois/pull/7#discussion_r4299193185 and traced lease acquisition and aggregate quota accounting.
- Added per-instance expired-lease deletion after storage-mode validation and under the existing storage advisory transaction lock. Every writable persistent mutation now clears stale leases before measuring usage; paused storage still performs no mutation.
- Added `TestExpiredLeasesReleaseStorageQuota`: a live lease blocks growth, forced expiry restores capacity for a document and a lease on another path, and the old row is physically removed.
- Resolved the first two GitHub review threads after their code/test commit; the lease thread is resolved after its fix is pushed.
- Confirmed the safe production backup uploaded successfully with no pg_dump/pg_dumpall/fatal upload error in its receipt. No dump was downloaded.
- Recorded aggregate production baseline: 3 users, 4 instances, 1 release, 1 model call, 0 documents. Migrations 001–008 are present; 009–011 remain pending before this rollout.

### Why

Lease payloads are temporary application state. Retaining expired rows in quota accounting eventually prevents unrelated writes; purging them during normal writes both restores logical capacity and prevents indefinite stale-row accumulation without another cleanup service.

### What worked

- Targeted PostgreSQL tests for expired leases, aggregate quota/cleanup and existing-view operations passed (`pkg/catalog 0.397s`).
- Catalog lint passed with zero issues.
- Refreshed PR CI passes Dependency Review, frontend checks, Go tests, lint and vulnerability checking so far. Enabling the repository capability corrected Dependency Review without disabling the check.
- Backup Job `postgres-backup-francois-008-20261016` completed and its upload receipt is present.

### What didn't work

- Aggregate inspection initially used the conventional PostgreSQL role `postgres`, but this cluster uses a custom role: `FATAL: role "postgres" does not exist`. The scoped read then used the pod's configured role internally and printed only counts/migration names.
- The PR's full-history secret scan still encounters the original dummy URI fixture in commit `0a66114`, before its new inline ignore. No real secret was identified. A squash merge preserves the reviewed final tree with the exact-line ignore; verify the main-branch scan before deploying that image. No broad detector suppression or history rewrite was introduced.

### What I learned

Accounting for temporary data requires an expiry boundary, not merely summing rows. The transaction already serializes all writers, so it is also the appropriate place to remove expired lease rows before computing quota.

### What was tricky to build

Cleanup must occur after policy checks, so read-only/off modes remain nonmutating, and before both quota measurements so cleanup and new growth are compared consistently. The regression test manipulates expiry in PostgreSQL rather than waiting for wall-clock expiration.

### What warrants a second pair of eyes

Check that live leases remain quota-accounted and that cleanup is limited to the instance locked by this transaction. Existing data with unexpired leases is preserved.

### What should be done in the future

Merge the final reviewed tree after CI completes, require the main secret scan to pass, then roll out through the GitOps image-pin PR and verify production. Subsequent rollout receipts are recorded in Step 18.

### Code review instructions

Review the small storage.go change and `TestExpiredLeasesReleaseStorageQuota`, then the already-committed fixes in Step 16. The primary production backup remains in the established backup destination.

### Technical details

The cleanup SQL is `DELETE FROM instance_leases WHERE instance_id=$1 AND expires_at<=now()` within `beginStorageWrite`. No schema migration, additional goroutine or new service is needed for this fix.
