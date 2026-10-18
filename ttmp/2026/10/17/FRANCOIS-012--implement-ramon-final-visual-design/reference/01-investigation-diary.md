---
Title: Investigation diary
Ticket: FRANCOIS-012
Status: active
Topics:
    - frontend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Guide authorship, source investigation, design choices and delivery evidence."
LastUpdated: 2026-10-17T14:27:27-04:00
WhatFor: Preserve evidence and handoff decisions.
WhenToUse: Resuming or reviewing the guide.
---

# Investigation diary

## Step 1: Split the comparison into a concrete implementation guide

I created this ticket from the Final comparison and wrote a standalone intern guide. The explanation is detailed while the selected implementation is deliberately small. Runtime implementation and production configuration were not changed. This entry records the investigation and drafting already performed in this turn.

### Prompt Context

**User prompt (verbatim):**

> Ok, create two tickets: the visual part, and the signin email part. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
>
> (for each ticket)

**Additional user steering (verbatim):**

> the design doc is detailed, the design itself is simple, elegant, streamlined, concrete and pragmatic, no overengineering or future oriented, it is in the here and now.

**Assistant interpretation:** Create two independently usable intern guides, store each in its own docmgr ticket, and upload each to reMarkable.

**Inferred user intent:** Give a new contributor enough technical context to ship the present feature without introducing unnecessary architecture or speculative scope.

**Starting code revision:** `dbef98e`.

### What I did

- Created the ticket and design/diary documents through docmgr.
- Read the original comparison and existing Go/TypeScript source, following page, session, identity, release, runner and build contracts.
- Read the textbook-authoring, docmgr, diary and reMarkable delivery skills.
- Wrote foundational prose before the implementation details, then added concrete data flows, API tables, pseudocode, file references and acceptance tests.
- Generated two PNG diagrams with Graphviz using the tracked scripts/01-render-diagrams.py and visually reviewed them.
- Kept implementation tasks open and planned delivery bookkeeping separately.
- Validation completed: docmgr doctor passed; all relative Markdown image/document links resolve; fences are balanced; two diagram PNGs per guide were rendered and visually reviewed. sources/validation.json records the bounded check results. No application test run is claimed for documentation-only work.

### Why

The user explicitly separates detailed explanation from a small implementation. A new intern needs to understand current ownership boundaries, actual request flows and exact contracts; that does not require adding services or expanding scope.

### What worked

- Existing source provides concrete reuse points for the proposed feature.
- Graphviz generated both diagrams successfully; the guide references real PNG files rather than Mermaid source that might print as code.
- Both guides use the same measured Final reference and explicitly distinguish current behavior from proposed routes/data.

### What did not work

- Exploratory source reads initially guessed nonexistent pkg/catalog/config.go and pkg/catalog/store/identity.go; diagnostics were No such file or directory. Located Config in server.go and Identity in store/users.go instead. No source files were changed by those failed reads.
- One broad source read exceeded its output budget. Re-read relevant symbols in bounded searches and corrected the session constructor reference to store/sessions.go:49.

- The first staged whitespace check rejected blank quoted lines as trailing whitespace (`> ` at diary lines 29–30). The failure-gated sequence stopped before committing. Removed only those trailing spaces and reran the staged check.

### What I learned

The visual design reuses Go templates, public TypeScript and the separate React shell. Archive display ordinals are computed before filtering, existing Markdown holds editorial sections, and the runner starts explicitly. This refines preliminary FRANCOIS-011 suggestions: no new permanent-number schema, editorial content model or billing redesign is required. The existing public script caches its session promise, so inline guest creation must refresh it before the CSRF-protected launch handshake.

### What was tricky to build

The hard part was scoping the guide to an implementable present-day change without falsely claiming that all controls in a mockup already have backend support. Proposed choices and existing APIs are labeled separately, with concrete acceptance conditions.

### What warrants a second pair of eyes

Review the selected ticket boundary and the observable tests, then follow the cited entry points. In particular, verify that identity proof never changes account ownership/role and that visual changes preserve the existing instance isolation and resource limits.

### What should be done in the future

Implement this ticket’s listed tasks; no speculative roadmap is prescribed. Guide delivery does not mark implementation complete.

### Code review instructions

Start with the guide introduction and its architecture diagram; a new reader should be able to explain the main entities and request sequence from those alone. Check file references against the starting revision, run docmgr doctor, validate local links/images, and use the upload receipt to distinguish cloud delivery from physical device synchronization.

### Technical details

- Scope: documentation, diagrams and ticket bookkeeping only.
- Diagram generator: Python standard library calling installed dot; no dependency installation.
- reMarkable destination: /ai/2026/10/17/FRANCOIS-012/.
- Final command/output evidence belongs in sources/upload-receipt.txt and the delivery diary checkpoint.

## Step 2: Validate the guide and deliver it to reMarkable

The completed guide was uploaded as its own PDF with a depth-two table of contents. The successful CLI receipt confirms cloud delivery. Physical device synchronization was not independently checked, and no routine cloud listing or extra PDF render was added.

### Prompt Context

**User prompt:** See Step 1, including the explicit request for one upload per ticket.
**Assistant interpretation:** Deliver the finished, validated guide and retain a reviewable receipt.
**Inferred user intent:** Make the technical guide available for reading and annotation on reMarkable.
**Commit (documentation):** `84da5b6` — Split Final redesign into visual and email sign in intern guides.

### What I did

- Confirmed docmgr metadata, local image/document links, balanced code fences and the rendered diagrams before delivery.
- Used the remarkable-upload skill’s bounded finalization handoff for the frozen guide; no content changes were made during upload.
- Ran the normal bundle upload with a unique document name, ticket-specific destination, toc-depth 2 and non-interactive mode, without force.
- Retained command output in sources/upload-receipt.txt and the compact result in sources/delivery-summary.json.
- Marked only the design/validation/delivery tasks complete; runtime implementation remains open.

### Why

The requested deliverable includes both a repository guide and reMarkable delivery. A clear successful upload is sufficient cloud evidence under the delivery skill; it is not a claim about the physical device.

### What worked

`OK: uploaded FRANCOIS 012 Final Visual Intern Guide.pdf -> /ai/2026/10/17/FRANCOIS-012`

### What did not work

No upload failure was observed.

### What I learned

The guide’s local PNG diagrams and Markdown references were accepted by the normal bundle pipeline. No extra renderer or upload mechanism was necessary.

### What was tricky to build

Delivery needed to preserve the final guide and its evidence while avoiding overwriting an annotated document. A unique ticket folder and document name allowed an ordinary upload with no replacement.

### What warrants a second pair of eyes

Implementation should follow the current scoped guide rather than broader preliminary suggestions in FRANCOIS-011. The successful receipt does not qualify any unimplemented feature.

### What should be done in the future

Implement the existing open tasks. There is no additional design roadmap in this delivery.

### Code review instructions

Read the guide, sources/validation.json and the successful receipt. Check that the remaining tasks still describe implementation rather than delivery. No application test result is claimed.

### Technical details

- Remote destination: /ai/2026/10/17/FRANCOIS-012/.
- Source guide revision: 84da5b6.
- Evidence level: successful cloud upload; no physical device synchronization verification.

## Step 3: Implement the Final shell and compact experiment archive

I started FRANCOIS-012 on a focused branch and replaced both public rendering shells. The archive now uses numbered compact rows, ordinary GET search/category/sort controls and first-publication ordering. Support and account use the same visual language while exposing their existing real actions. Runtime embedding is the next implementation slice.

### Prompt Context

**User prompt (verbatim):** "Implement the first ticket, ccommit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
**Assistant interpretation:** Implement FRANCOIS-012 and qualify its behavior with local integration/browser checks, committing coherent milestones.
**Inferred user intent:** Ship the concrete visual design already documented without expanding into email authentication or speculative architecture.
**Starting revision:** `fc84c85`; branch `feat/francois-012-final-visual-design`.

### What I did

- Replaced the Go and React wordmark/navigation/footer and added Final-scoped public layout classes.
- Implemented compact archive rows and retained numbering before filtering. Collected earliest publication dates while keeping latest-release metadata.
- Separated the journal route from the experiment archive and preserved working RSS/legal destinations.
- Restyled support and account around actual current subscription/account data; retained working checkout, portal and staff links.
- Updated obsolete layout/download-label expectations and built embedded browser assets.

### Why

The two existing shells must change together. A normal GET form and computed ordinals satisfy the archive without a new database field, API, service or client framework.

### What worked

- `pnpm check` and `pnpm build` completed; the installed Node 20 emits the repository's Node >=24 engine warning but these commands succeeded.
- Database-backed `TestFinalLayout`, `TestStorefrontPages` and matching public-page tests passed against local PostgreSQL on 5435 after the signed-in footer fix.
- Confirmed showcase data exists on local 5435, whereas 5434's catalogue is empty. Local release blobs are in var/storefront/releases.
- Browser-verified the new archive at localhost:18792: eight real public rows, the new shell, and no horizontal overflow at 1440px. Saved sources/implementation/01-archive-desktop.png.

### What didn't work

- A zsh read using unquoted doc/local* failed with `no matches found`; switched to explicit directories.
- The first targeted compile used a TestFinal pattern before tests existed and reported no tests to run; it is compile evidence only.
- The first populated integration run failed `subscriber page misses download or sign-out`; restored the functioning public signed-in sign-out control in the footer and the affected tests passed.
- Initial tmux inspection inside the sandbox failed with Operation not permitted; repeated through the authorized host execution path.
- lsof found no user-owned listener for the existing container-forwarded 8090 port, so I did not rely on its process arguments. The known local blob was found in var/storefront/releases, not var/catalog/releases.

- The initial preview port 8092 was already occupied by an unrelated Agentforum app. The browser check identified its title and zero archive rows; no changes were made to that app. Started an isolated persistent tmux session on 18792 and verified the correct page.

### What I learned

The inline runner must discard public.ts's cached pre-guest session and fetch the new guest CSRF token. The existing standalone navigation implicitly did that; embedding must do it explicitly.

### What was tricky to build

Archive dates and current-release dates are different. The view model now retains both, and numbering is assigned on the complete publication-ordered set before the visible query is applied.

### What warrants a second pair of eyes

Check no draft/unlisted entries enter the archive, query state survives category changes, and staff navigation remains available without crowding the public header. Review scoped CSS against account, Studio and Admin.

### What should be done in the future

Complete the existing detail/inline-runner task and the ticket's browser and affected-behavior verification. Email sign-in remains FRANCOIS-013.

### Code review instructions

Start with pages.go feedPage/cards, home.tmpl/layout.tmpl, ui.tsx, account.tsx and the Final CSS block. Read the updated tests to distinguish changed presentation assertions from preserved visibility/download checks.

### Technical details

- Archive uses q/tag/sort GET parameters; unknown sort defaults to newest.
- Numbers are presentation ordinals; no migration was added.
- Local preview target is http://localhost:8092 with fake model backend and existing development data. No production deployment occurred.

## Step 4: Implement editorial detail pages and reuse the isolated runner inline

The detail page now follows Final’s title, download, runner and editorial layout. The runner starts explicitly and reuses the existing instance endpoints and isolation boundary. The same launch handler is shared with React; no new hosting API, content schema or database migration was added.

### Prompt Context

**User prompt:** See Step 3.
**Assistant interpretation:** Complete the detail/runner milestone, then qualify the complete visual ticket locally.
**Inferred user intent:** Run an experiment within its write-up while keeping the site minimal and the existing resource controls intact.
**Previous commit:** `06a1219` — Implement Final public shell and compact searchable archive.

### What I did

- Moved the latest authorized HTML download and actual filename/size above the runner, preserving attachment access rules and the current-release-only guarantee.
- Added a Goldmark AST section renderer for the four conventional labels, preserving unknown headings, video media and sanitized Markdown.
- Wrote five tracked imported-experiment page.md files and published them to the local preview through existing Studio APIs.
- Added shared exact-window/exact-origin launch handling; refreshed the cached guest session before requesting a CSRF-protected launch code.
- Added non-secret started/error notifications from the existing bootstrap, so Live follows the fetched application HTML rather than merely minting a code.
- Implemented same-instance Reload, browser fullscreen and a direct instance-page link. Page load, filtering and hover allocate no visit.
- Added meaningful archive, sanitizer and launch-boundary tests; updated the old combined-home expectations while retaining the combined RSS feed.

### Why

Reusing the established session, demo, instance and launch contracts preserves limits and security without a second execution system. Markdown sections provide the editorial layout through the existing editor and draft workflow.

### What worked

- `pnpm check`, `pnpm build`, and all nine frontend tests passed, including the new frame-ownership and pending-reply cleanup test.
- Final host validation used Node v24.18.0: pnpm check, pnpm test (nine tests), pnpm build and go build ./... all passed.
- The complete `go test ./... -count=1` passed with TEST_DATABASE_URL pointing to the local disposable-database harness, including catalogue, hosted admission/cache, payment and client packages.
- Browser verification at 1440, 900 and 390px passed layout/search/sort/ordinal checks with no document overflow and no implicit visit creation.
- A real local guest start obtained the new guest CSRF token; Reload and fullscreen preserved the same single instance. Expiring that test visit caused a visible server denial without allocating another visit.
- Signed-in account, Studio and Admin shells were captured and reviewed. No production data or deployment was changed.

### What didn't work

- The new archive fixture first used visibility='unlisted'; PostgreSQL rejected it with catalog_artifacts_visibility_check (SQLSTATE 23514). Corrected the fixture to the actual 'hidden' value.
- Initial full catalogue checks found TestAttachments missing its plan-specific access message and TestPlans missing its current-support marker. Restored the message, current-support semantics and data-plan metadata; targeted checks and the full suite then passed.
- The first browser detail check found zero anonymous downloads because the old local imports were private. Published new free-sample local preview releases through Studio, without changing download authorization code.
- A later hover check timed out after 30000ms because local imported pages had no covers. Seeded their previously captured WebP media through Studio, preserving existing page media where present.
- Corrected the verification helper to use the real admin settings contract: active.document plus PATCH with If-Match, rather than a speculative body revision field. The helper restores the previous local demo-enabled setting after its run and revokes its short-lived admin session.

### What I learned

A fetched app and a completed model call are unrelated states. The runner’s Live indicator comes from bootstrap delivery, while backend model admission stays unchanged. Browser evidence also needs to wait for actual authored DOM before capturing a live scene; a screenshot taken immediately on the delivery message can be blank while the app initializes.

### What was tricky to build

The formerly separate guest navigation refreshed session state implicitly. The inline controller must explicitly discard the anonymous cached promise, fetch /api/session again, and only then submit launch requests. Reload reuses that instance and handshake rather than calling the start endpoint again.

### What warrants a second pair of eyes

Review the shared launch handler and bootstrap messages, Markdown AST grouping, conditional download access, and the pending-reply cleanup test. Check that helper-only local fixture setup is not confused with production publication.

### What should be done in the future

Complete final visual review, build checks, evidence and ticket bookkeeping. No email sign-in work is included; FRANCOIS-013 remains separate.

### Code review instructions

Read instance-launch.ts, public.ts, instances.tsx and launch.ts together, followed by content/markdown.go and its sanitizer test. The local browser helper and browser-validation.json demonstrate both positive startup and expiration denial.

### Technical details

- Local-only browser helper: scripts/02-verify-local.mjs; repeatable tmux launcher: scripts/03-run-preview.sh.
- Existing release files are unchanged by the page write-ups; preview releases use the tracked import HTML and existing free-sample publication path.
- Browser credentials stay in memory; the temporary admin session is revoked in the helper’s finally block. SMTP/OIDC/email registration was not implemented.

## Step 5: Complete responsive, runtime and build verification

The final browser run qualified the implementation at desktop, tablet and phone widths, including an actual rendered application after its launch handshake. I reviewed the images, retained the receipts and prepared the ticket for completion. The preview remains available locally with the fake backend; no production deployment or email-login implementation is included.

### Prompt Context

**User prompt:** See Step 3.
**Assistant interpretation:** Finish the visual implementation and retain concrete verification and commit evidence.
**Inferred user intent:** Have a working, reviewable implementation of the first ticket with an honest record of checks and limitations.
**Implementation commits:** `06a1219` and `576764b`.

### What I did

- Verified twelve public page/viewport combinations at 1440, 900 and 390px plus authenticated account, Studio and Admin.
- Verified search without JavaScript; ordinary search/sort preserved archive ordinals and caused zero instance allocation.
- Verified hover preview with real images, same-instance guest reload, browser fullscreen, actual expired-visit denial, and a mocked paused-start 503 UI response. Existing Go operation tests separately cover the actual backend switches.
- Waited for authored iframe DOM before the live screenshot; corrected missing-media thumbnails to show a restrained monogram instead of a broken-image label.
- Kept unlabelled supplemental Markdown from creating empty accessibility headings, and labeled staff account entitlement Access rather than Support.
- Simplified detail metadata retrieval to one Store.ArchivePosition query rather than hydrating the entire archive’s covers, creators and release files for one number. The integration test compares that query against archive numbering/dates.
- Ran the final checks and recorded the exact bounded results in sources/implementation/checks.json.

### Why

The code must be tested through the real guest session transition and isolated frame, rather than only against mocked APIs. Reference-width captures and an intermediate width qualify responsive composition; existing backend tests qualify limits and access behavior.

### What worked

- Full Go suite passed with local database integration enabled; frontend check/build and all nine tests passed under Node v24.18.0; go build ./... passed.
- Final-query targeted integration checks and the build passed after the detail query simplification.
- Browser receipt records all checks, twelve public captures, real startup, authored DOM rendering, reload/fullscreen identity and expiry denial. Temporary browser admin sessions were revoked.
- The final local run deliberately used --keep-demo-enabled so the review preview can start bounded visits. This affects only the local development database; the default helper mode restores the previous flag.

### What didn't work

- Earlier old-fixture failures are recorded in Step 4. The initial live capture was taken before authored DOM rendering, producing a blank scene. The helper now waits for actual application DOM before that capture; the final image shows the working editor and scene.
- Three legacy local fixture media records point to missing blobs. The five real imported experiments were seeded with their existing WebP media; the archive now renders a monogram for missing legacy thumbnails. No production media was altered.

### What I learned

A catalogue detail does not need all archive media metadata to show a display ordinal. One aggregate query retains the exact publication-date/ID ordering without a new persisted number or extra schema.

### What was tricky to build

The screenshot needed to distinguish bootstrap delivery from the application rendering its own DOM. The successful launch notification validates delivery; the browser evidence explicitly waits for the authored page before photographing it.

### What warrants a second pair of eyes

Review current-release authorization and private cache tests, exact frame origin/source checks, AST sanitization, and the equivalence of detail/archive ordering. The local preview’s OIDC client is a display/test placeholder; real existing OIDC login configuration and future email login remain separate from this visual task.

### What should be done in the future

Review and deploy the committed change when requested. FRANCOIS-013 remains unimplemented. No additional architecture is prescribed.

### Code review instructions

Read sources/implementation/checks.json and browser-validation.json, then the mobile archive and live experiment screenshots. Reproduce with scripts/03-run-preview.sh and scripts/02-verify-local.mjs using the local development services. No credentials are retained in committed evidence.

### Technical details

- Local server: tmux session francois-final-012, http://localhost:18792.
- Private development database URL file: /tmp/francois-012/database-url; no secret file is committed.
- Data changes: local Studio publication of tracked page.md write-ups and existing showcase media, plus public sample preview releases through the standard release workflow.
- No database migration, new hosting endpoint, custom payment flow or production release was added.

## Step 6: Restore the missing support amount and connect the complete contribution workflow

Reopened the visual implementation after the user identified the missing 42 USD design. Direct Paper inspection confirmed that the current template omitted the complete editable contribution block, changed the lead/principle copy and replaced the year-funding amount. Implemented the missing layout and a distinct monthly contribution path through the existing billing components. This checkpoint is implemented but not yet qualified.

### Prompt Context
**User prompt (verbatim):** "cool, that works. We need to properly pass the $42 dollars design (which I don't see on slop.boutique/pricing), they're the pages from ramon's paper project. https://app.paper.design/file/01M4XVT9C83GGYK57CMAAAEJJT/p-6-0"
**Follow-up (verbatim):** "complete checkout flow. Also, when signed in as a free user, show the banner about keeping the lab running on the user page. Also implement the thank you page when supporter"
**Follow-up (verbatim):** "and make sure that if logged in as a support, you don't see the support monthly banners"
**Assistant interpretation:** Implement the real amount selector and complete checkout, free account callout, thank-you/supporter state and suppression of support solicitation for supporters.
**Inferred user intent:** Ship Ramon’s minimal presentation with working behavior and truthful billing state.

### What I did
- Read the Paper guide, Final metadata and exact desktop/mobile support and account JSX; inspected the support screenshot and confirmed ABC Areal availability.
- Checked live `/api/plans`: one configured Subscriber monthly price with an empty label; no custom amount workflow exists.
- Added the active contribution implementation guide and task 8yxo.
- Implemented monthly inline Stripe price data, reuse of existing admission/ownership/idempotency, two projection columns, and a support summary separate from access entitlements.
- Added the support form, thank-you route/account presentation, free-account callout and server/React supporter visibility rules.

### Why
- Showing 42 while charging a configured unrelated fixed price would be incorrect. Contributions must preserve the chosen amount end-to-end.
- Support does not unlock content; its state must be separate from access-plan privileges.

### What worked
- Paper MCP provided exact current exports and a reference screenshot. The existing billing service already handles retries, changed-choice supersession and owner-scoped refresh.

### What didn't work
- Initial branch creation failed with a read-only Git ref lock; the authorized escalated command created `feat/ramon-support-pricing` successfully.
- Early path guesses for the frontend package and billing handler were wrong; actual paths are root `package.json`, `ui/catalog/public.ts` and `pkg/catalog/routes.go`.

### What I learned
- The omitted design requires a billing amount contract, not only CSS. The supporter account artboard includes real history and impact data that must come from authoritative records.

### What was tricky to build
- Separate donation state from privileges while retaining the existing plan APIs. Preserve real Stripe price IDs and project contribution metadata/amount instead of adding price aliases.

### What warrants a second pair of eyes
- Chosen-amount idempotency, duplicate subscription prevention, projection trust and visibility of calls to action after subscription confirmation.

### What should be done in the future
- Complete qualification and real local desktop/mobile captures; no deployment is claimed at this checkpoint.

### Code review instructions
- Start with the active contribution guide, `support.go`, gateway/service changes and the existing account refresh flow. Inspect migration 015 and confirm existing users/sessions are unaffected.

### Technical details
- Default 42 USD/month, editable whole-dollar range 1–9999. Inline monthly prices carry server-authored metadata; the existing webhook worker remains the only reconciliation service.
- Source reference: https://docs.stripe.com/api/checkout/sessions/create

## Step 7: Qualify the complete monthly checkout and supporter presentation

Qualified the implemented contribution flow against PostgreSQL, real Chromium and Stripe sandbox. The actual 43 USD selection reached hosted Checkout, completed with the standard test card and returned to a confirmed supporter thank-you page. Local account data recorded the same amount, while access stayed free. No production deployment was performed.

### Prompt Context
**User prompts:** See Step 6, including complete checkout, free-account banner, thank-you page and no monthly prompts for supporters.
**Assistant interpretation:** Finish the implementation with meaningful integrated evidence and preserve a simple code path.
**Inferred user intent:** Launch a coherent support experience rather than a visual amount control detached from payment behavior.
**Commit (code):** 539a447 — Implement Ramon support contribution page and monthly checkout workflow; this checkpoint completes qualification and cleanup.

### What I did
- Added service regressions for amount bounds, same-amount reuse, changed-amount session expiry and duplicate subscription prevention.
- Added an actual stripe-go HTTP payload test and projection tests for server metadata/currency recognition.
- Added catalog integration checks for amount markup, real support summary, suppressed Go prompts and unchanged free entitlements.
- Fixed the support endpoint to use the existing enabled-account guard, retained honest bounded confirmation feedback and routed demo guests through normal sign-in.
- Removed unused plan-view assembly and the superseded plan-link JavaScript from the contribution page. Existing plan APIs remain functional for their existing consumers.
- Built assets, started the isolated preview in tmux `francois-support-012` on localhost:18796 and recorded desktop/mobile screenshots.
- Read the configured Stripe credential privately, required its test-mode prefix, and used only sandbox Checkout. Private files contain the credential and transient browser state; none are committed.
- Completed hosted Checkout with the standard 4242 test card, verified owner-scoped refresh and actual Stripe projection, tested Customer Portal creation and scheduled cancellation of the test subscription.
- Saved current exact Paper JSX exports for desktop/mobile support and desktop account beside the implementation evidence.

### Why
- Amount/idempotency/projection tests exercise payment correctness rather than just matching the implementation. Real hosted Checkout proves that the selected amount survives the gateway and authoritative reconciliation boundary.
- Supporter presentation uses active paid-through subscription state, not the checkout URL or account role. Contributions do not unlock a higher access plan.

### What worked
- Full database-backed `go test ./... -count=1`: all packages passed; catalog 15.439s, payments 1.352s.
- Focused support/payment regression run passed; final simplified presentation checks passed (0.779s).
- `pnpm check`, nine frontend/worker tests, `pnpm build`, `go build ./...`, lint and logging checks passed.
- Browser checks passed at 1440/390px: amount stepping, no document overflow, free-account callout, supporter thank-you and absence of monthly solicitation.
- Actual sandbox receipt: chosen 4300 cents = Stripe 4300 cents; returned to `/thank-you`; confirmed support; free access retained; Customer Portal available; test cancellation scheduled.
- Reviewed pricing mobile and actual supporter desktop/mobile screenshots visually. Clean recapture removed the transient success-toast overlay.

### What didn't work
- Initial full Go run failed only `TestPlans` with `plans_test.go:139: pricing 200`: it expected Studio/Current support plan-card markup. Updated the assertion to verify the requested supporter page and no contribution form; focused and final full tests then passed.
- Initial lint reported `variable copy has same name as predeclared identifier (predeclared)` at support_test.go:54. Renamed it to `candidate`; final lint passed.
- First hosted form inspection found no input fields because Stripe initially displays payment-method accordions. Used the existing documented Card selection sequence, then filled the standard sandbox fields and completed checkout successfully.
- Formatting the shared API type file introduced unrelated churn. Restored its already-committed contents while retaining the new support contracts.

### What I learned
- Inline recurring prices work with the configured sandbox credential. Stripe’s existing subscription metadata and current-period fields are sufficient to distinguish contributions without price aliases or a new billing service.

### What was tricky to build
- The account return must wait for authoritative projection. The real flow used the existing owner-bound refresh operation and worker; a URL alone never created supporter state.
- Support cancellation keeps prompts hidden through the paid-through period. The contribution itself leaves access entitlements unchanged.

### What warrants a second pair of eyes
- Review server-authored contribution metadata, supported monthly USD recognition, amount-bound checkout keys and changed-amount supersession. Review enabled-account/CSRF handling and supporter prompt suppression in both Go and React shells.

### What should be done in the future
- Review and deploy this branch when requested. Current production still runs the previously deployed onboarding image.

### Code review instructions
- Start with support-sandbox-checkout.json and the actual sandbox thank-you screenshots, then support.go, payments/service.go, payments/gateway.go, migration 015 and ui/catalog/account.tsx.
- Reproduce the isolated preview with script 04 after supplying sandbox-only private credential files. Scripts 05–07 never save keys, cookies, payment URLs or authentication links to the ticket.

### Technical details
- Preview: http://localhost:18796/pricing. Private directory: /tmp/francois-support-private, mode 0700; credential files mode 0600.
- Migration 015 adds support_amount_cents and started_at to the existing subscription projection. No new queue, background service or identity-provider changes.
- Structured receipts and eight new screenshots are under sources/implementation/support-*; original visual-ticket evidence is preserved.

## Step 8: Publish the qualified implementation for review

Pushed the implementation and qualification evidence, then opened PR #15. Production remains on the previous onboarding release; this checkpoint records publication for review only.

### Prompt Context
**User prompts:** See Step 6.
**Assistant interpretation:** Make the completed, locally qualified work concrete and reviewable.
**Inferred user intent:** Preserve commits, evidence and a straightforward route to deployment.
**Commit (code/evidence):** 539a447 and d8634b2.

### What I did
- Pushed `feat/ramon-support-pricing` and opened https://github.com/go-go-golems/francois/pull/15 with the exact feature, migration and sandbox results.
- Normalized the trailing blank line in the routine TypeScript success receipt after Git’s whitespace check reported it.

### Why
- A source PR keeps review and the official image/GitOps pipeline attached to the tested implementation.

### What worked
- Branch push and PR creation succeeded. Required CI/review starts from the PR; it is not described as complete here.

### What didn't work
- `git diff --cached --check` reported `support-types.txt:4: new blank line at EOF`; the preceding qualification commit included this harmless receipt whitespace. Removed it in this publication checkpoint.

### What I learned
- Routine command receipts can be bounded and normalized; they need not retain decorative terminal blank lines.

### What was tricky to build
- N/A: this checkpoint publishes already qualified code and records review status.

### What warrants a second pair of eyes
- PR #15 billing trust, chosen-amount handling and supporter state boundaries.

### What should be done in the future
- Review, merge and deploy when requested; no production rollout is claimed.

### Code review instructions
- Read the PR description and the active support guide’s verified result. Use the actual sandbox receipt rather than seeded screenshots as proof of payment completion.

### Technical details
- PR: https://github.com/go-go-golems/francois/pull/15
- Local preview: http://localhost:18796/pricing

## Step 9: Rebase the merged support release and deploy to production k3s

Rebased the completed feature onto the user-merged PR and deployed its official image through the existing GitOps pipeline. Production now serves Ramon’s 42 USD contribution controls; migration 015 is applied and both public origins pass read-only rollout checks. No production checkout or email was submitted during deployment verification.

### Prompt Context
**User prompt (verbatim):** "merged, rebase, deploy"
**Assistant interpretation:** Reconcile the local branch with merged main and deploy the merged support implementation to production.
**Inferred user intent:** Make the reviewed support experience available on slop.boutique.
**Commit (code):** 08332bb6649ebfba671a46f0a2138d1a54416ca7 — Merge pull request #15.

### What I did
- Confirmed PR #15 was merged, fetched main and ran `git rebase origin/main` successfully on the feature branch. Switched to main and fast-forwarded to the merge.
- Used official image release run 39085953535; it succeeded and published `ghcr.io/go-go-golems/francois:sha-08332bb`.
- Prepared the image-only manifest in the isolated GitOps checkout and ran `bash scripts/validate_gitops.sh`: 54 packages, zero violations.
- Verified the generated automation manifest byte-for-byte against the validated manifest. GitOps PR #371 had exactly one image-pin change and passed its validate check.
- Merged exact PR head a711ec84150611923034db7f317c5d402fe20ecd; GitOps merge de029e0dd79947aea9d36dcd631b09cfbe9c2902.
- Refreshed the Francois Argo application and waited for the exact image, deployment readiness and Healthy status.
- Added read-only production verification and screenshot scripts 08–09, preserving prior qualification evidence.

### Why
- The official source/image/GitOps path retains immutable release provenance. No application configuration, identity setting, credentials or additional service needed modification.

### What worked
- Rebase completed without conflicts; local feature head equals merged source 08332bb.
- GitOps validation passed all 54 packages. Automated deployment content matched the validated file exactly.
- Deployment rolled out successfully; Argo is Synced/Healthy/Succeeded at the GitOps merge revision.
- Migration `015_monthly_support.sql` applied at 2026-10-18 01:21:49.336381 UTC.
- Receipt at 01:22:12 UTC confirms one ready/updated replica, exact image and both public hostnames.
- Pricing includes the 42 USD form and year-funding section; `/thank-you`, health, readiness, archive, Lattice Deck and RSS return 200. Existing email/Google/GitHub sign-in and hidden-journal settings are retained.
- Live Chromium checks at 1440/390px verified plus/minus changes 42→43→42 and no horizontal overflow, with new production screenshots.

### What didn't work
- No deployment or verification failures observed. The reused GitOps checkout retained the prior deployment’s validated image edit; before restoring it, compared its bytes with current origin/main to prove that no unexpected local change would be discarded.
- The prior local-preview restart briefly reported `duplicate session: francois-support-012` while the old Go process exited. Once that session had exited, started a new tmux session and verified HTTP 200 with the 42 USD form. This affected the local preview only.

### What I learned
- The support release needs only its image update: startup migration handles the added projection fields, and the existing runtime Stripe configuration supports the new flow.

### What was tricky to build
- Waited for the desired image before checking rollout success, then verified database migration separately from public route availability. The existing one-replica Recreate deployment strategy was preserved.

### What warrants a second pair of eyes
- The production checks are read-only. Actual payment and confirmed supporter behavior were qualified through real Stripe sandbox Checkout in Step 7; deployment verification deliberately did not submit another payment.

### What should be done in the future
- No deployment step remains for this release.

### Code review instructions
- Read support-production.json, production pricing screenshots, source PR #15, release run 39085953535 and GitOps PR #371. Scripts 08–09 reproduce read-only runtime checks without exposing credentials or creating checkout sessions.

### Technical details
- Source: https://github.com/go-go-golems/francois/pull/15
- Release: https://github.com/go-go-golems/francois/actions/runs/39085953535
- GitOps: https://github.com/wesen/2026-03-27--hetzner-k3s/pull/371
- Public contribution page: https://slop.boutique/pricing
