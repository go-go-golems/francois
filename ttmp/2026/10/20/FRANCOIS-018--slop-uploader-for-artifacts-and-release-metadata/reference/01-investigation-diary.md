---
Title: Investigation diary
Ticket: FRANCOIS-018
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
LastUpdated: 2026-10-20T21:24:12.108800621-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigation diary

## Step 1: Inspect the supplied uploader and specify a focused implementation

Inspected the supplied JPEG and standalone uploader ZIP, then traced every substantive form action into the existing Studio frontend, Go handlers, release publisher, and persistence model. The prototype provides a useful visual and interaction reference. The intended implementation is a focused Studio screen with existing upload APIs and a small limits-response addition.

The detailed guide explains artifact versus release versus page ownership before describing the implementation. It identifies concrete mismatches in the prototype: live metadata changes during draft save, outdated free-sample wording, lossy manifest conversion, retained files when switching artifacts, and ambiguous retries after partial success. Application source was not changed.

### Prompt Context
**User prompt (verbatim):**

> Ok, let's create a new docmgr ticket to implement a nice slop uploader where we can upload new artifacts and set their data. See ~/Downloads/slop-uploader.jpeg and ~/Downloads/slop-uploader.zip
>
> Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
>
> The document is detailed, the design is elegant, no bullshit, to the point.

**Assistant interpretation:** Create a new implementation ticket, investigate the supplied reference and current system, write an intern-ready technical guide, and deliver it to reMarkable. This is a design/documentation task, not authorization to implement or deploy the uploader now.
**Inferred user intent:** Give an implementer enough precise context to ship an attractive, straightforward upload workflow without adding unnecessary infrastructure.

### What I did
- Applied docmgr, diary, ticket-research-docmgr-remarkable and remarkable-upload skill instructions.
- Created FRANCOIS-018 and its design/diary documents through docmgr.
- Copied both original files into sources, recorded hashes, inspected archive paths, and extracted the source for stable reference.
- Visually inspected the JPEG and read the complete HTML, CSS, client, server and tests from the ZIP.
- Read Studio route dispatch, API hooks, auth guards, multipart decoder, release validation, media and attachment handling, artifact/page persistence, and release publication.
- Asked an optional delivery preference: integrated Studio versus standalone local uploader. With no answer received while researching, documented Studio as the explicit design assumption.
- Ran the supplied pure helper test and a bounded manifest round-trip experiment. Captured the result in sources/analysis-evidence.json.
- Wrote the implementation guide with prose orientation, data/sequence diagrams, API tables, pseudocode, file references, phases and acceptance criteria.
- Preserved the unrelated FRANCOIS-017 local diary edits from blocked deployment attempts.

### Why
- Existing Studio APIs already perform the required mutations and enforce creator ownership. The common task needs a better form, not another service.
- Release draft state and page draft state do not cover live artifact title/description/tags. The guide makes this boundary explicit rather than promising all-or-nothing draft behavior.
- Existing authored manifest options should survive checkbox editing, while recovery should not blindly recreate a draft after an uncertain response.

### What worked
- docmgr created the ticket and documents successfully.
- The helper test command exited 0; its runner reported one passing file and zero failures. This is not evidence of proxy or browser correctness.
- The manifest experiment reproduced option loss: a profile/email scope list was reduced to profile, and an additional sample option disappeared.
- Source inspection confirmed that all required mutation routes already exist. Only the current limits response needs a small extension, with matching Go/TypeScript client fields.

### What didn't work
- Initial source discovery guessed pkg/catalog/media.go, pkg/catalog/store/storefront.go and pkg/catalog/web.go; those files do not exist. Located the real implementations in studio.go, content/media.go, store/releases.go and web/embed.go.
- One broad search included generated minified JavaScript and produced truncated output. Narrowed subsequent inspection to authored TypeScript and Go source; no conclusions depend on the truncated output.
- No live workflow or production upload was tested. The supplied server tests were read, not run; no personal token was loaded.

### What I learned
- Upload response projections differ from full Studio release records despite the broad frontend Release type.
- The current free-sample flag controls public file/attachment downloads, not hosted early-access execution.
- A same-origin Studio form can reuse the existing CSRF-aware query layer and media picker without token proxy setup.

### What was tricky to build
- Keeping a detailed handoff concise at the implementation level: one form, pure helpers, narrow shared fields, sequential existing mutations, and Studio recovery after partial success.
- Preserving editorial and release publication as separate real operations, including honest UI messages when only one succeeds.

### What warrants a second pair of eyes
- Confirm the Studio delivery assumption before implementing if the desired product is specifically the local utility from the ZIP.
- Review the live-metadata draft notice, partial-publication messaging, manifest preservation, and response shape changes.

### What should be done in the future
- Implement the open application tasks, then run the documented integration and browser scenarios. Keep draft/release/policy semantics aligned with the guide.

### Code review instructions
- Start with Sections 2, 5, 8 and 10 of the guide, then compare the referenced Studio handlers and store methods. The source inventory records the exact supplied inputs.

### Technical details
- Repository baseline: fbb4f72ffd3917bf6e4f2229c5043a9b176b29ea.
- Pure helper command: node --test ttmp/2026/10/20/FRANCOIS-018--slop-uploader-for-artifacts-and-release-metadata/sources/prototype/slop-uploader/test/lib.test.mjs.
- Proposed route: /studio/upload; no schema migration or new runtime service.


## Step 2: Validate and deliver the guide

Completed the document and uploaded its PDF to reMarkable cloud. The guide contains the supplied visual reference, system orientation, two portable text diagrams, API and field tables, concrete pseudocode, implementation phases, file references, and validation scenarios. Application implementation tasks remain open.

### Prompt Context
**User prompt:** See Step 1. Delivery to reMarkable was explicitly requested.
**Assistant interpretation:** Validate the ticket and deliver the finished guide without adding implementation work.
**Inferred user intent:** A usable technical handoff available both in the repository and on reMarkable.

### What I did
- Related six primary source files through docmgr, completed the investigation/writing tasks, and recorded the changelog.
- Checked local Markdown/image links and Git whitespace.
- Ran `docmgr doctor --ticket FRANCOIS-018 --stale-after 30`.
- Ran `remarquee upload bundle` for the design guide, with document name `FRANCOIS-018 Slop Uploader Implementation Guide`, remote directory `/ai/2026/10/20/FRANCOIS-018`, TOC depth 2 and non-interactive delivery.

### Why
- The source and contract checks validate the handoff; the upload command provides direct delivery evidence without unnecessary extra rendering or cloud listing.

### What worked
- Docmgr: all checks passed. Local document/image links resolve and whitespace validation passed.
- Upload receipt: `OK: uploaded FRANCOIS-018 Slop Uploader Implementation Guide.pdf -> /ai/2026/10/20/FRANCOIS-018`.
- Delivery receipt saved in sources/remarkable-delivery.json.

### What didn't work
- No delivery failure occurred. Physical device synchronization was not checked; the result proves cloud upload.

### What I learned
- The normal uploader completed rendering and cloud delivery without a separate PDF build or authentication recovery.

### What was tricky to build
- No additional delivery repair was needed. The prose distinguishes observed backend behavior from proposed changes and the unresolved standalone-versus-Studio preference.

### What warrants a second pair of eyes
- Confirm the explicit Studio delivery assumption if a standalone local utility is preferred.

### What should be done in the future
- Implement the two open ticket tasks and validate the real browser workflow. The earlier deployment task belongs to FRANCOIS-017 and was not resumed by this documentation request.

### Code review instructions
- Read the guide and source inventory. The ticket includes original inputs, extracted reference code, helper experiment evidence, and the upload receipt.

### Technical details
- Cloud destination: /ai/2026/10/20/FRANCOIS-018.
- Main deliverable: design-doc/01-slop-uploader-analysis-design-and-intern-implementation-guide.md.

## Step 3: Implement the guided Studio uploader

Created the integrated upload route and reused the existing Studio mutations. The form contains HTML selection, artifact details, capability controls, full manifest JSON, public-download policy, staged media, a Markdown write-up and attachments. A sequential submission module records completed steps and reconciles uncertain release mutations without replaying creation.

### Prompt Context
**User prompt (verbatim):** "Implement it, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill). Take screenshots for the diary / a report at the end.

When done,  write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).
 Commit and push the bsidian vault when done (go-go-parc vault)."
**Assistant interpretation:** Implement FRANCOIS-018, verify the integrated UI, capture evidence, and deliver a technical report to the vault.
**Inferred user intent:** A usable uploader with understandable behavior and a durable explanation of its implementation.

### What I did
- Created `feat/slop-uploader` and rebased onto origin/main (e457daf), preserving the unrelated FRANCOIS-017 diary with autostash.
- Extracted Markdown and media fields into `studio-fields.tsx`; ignored stale Markdown preview responses and associated its label with its textarea.
- Extended StudioMe's Go, TypeScript and strict CLI contracts with HTML, gallery and attachment limits.
- Added `uploader-model.ts`, `uploader-submit.ts`, and the `/studio/upload` form, plus navigation links and scoped responsive CSS.
- Added six meaningful frontend tests covering manifest preservation, byte limits, publication order, partial failure, lost replies and media reuse.

### Why
- Existing handlers already enforce creator permissions, CSRF and release validation. A new backend workflow service would duplicate these contracts.
- Local File references belong to the form; persisted records remain in RTK Query. Successful writes are durable, not a fictional transaction.

### What worked
- `pnpm check`, `pnpm test` (18 passing), and `pnpm build` passed.
- Check mode performs no mutations. Publication saves page content before publishing the release. An attachment failure returns the actual draft and stops remaining writes.
- Unknown capability namespaces and scoped options survive checkbox changes.

### What didn't work
- Initial TypeScript check reported TS7034/TS7005 for the inferred manifest variable. Added an explicit parsed-manifest-or-undefined type.
- The shell uses Node 20.17.0 while package.json requires >=24. Commands passed with the engine warning; browser and Go integration remain to be verified.
- Initially wrote the uploader CSS under ui/catalog; corrected it to the project's actual static CSS source before committing.

### What I learned
- The repository serves `pkg/catalog/web/static/catalog.css` directly; the JavaScript build does not copy a UI CSS source.
- Upload replies are public release projections with checks, not full Studio release records. The response type now describes that shape.

### What was tricky to build
- A dropped HTTP reply can hide a successful database write. After creation is attempted, the form locks against replay and fetches the artifact. A lost final publish response is considered successful only when the reloaded release is published.
- Cached media IDs prevent duplicate media on retries before release creation. File object identity is sufficient within one mounted form.

### What warrants a second pair of eyes
- Review the independent page/release publication boundary and the live-metadata notice for existing artifacts.
- Browser checks should exercise artifact switching and mobile layout, not only submission helpers.

### What should be done in the future
- Complete database-backed tests, screenshot checks and the requested vault report in the next steps.

### Code review instructions
- Begin with uploader-submit.ts, then uploader-model.ts and uploader.tsx. Run pnpm check/test/build and the PostgreSQL catalog suite.

### Technical details
- No migration, new HTTP endpoint, service or dependency was introduced.
- Gallery limit 24 and attachment limit 20 are returned by StudioMe alongside configured byte limits.
- Metadata updates affect the live artifact; page PUT and release upload create separate drafts. Public download retains the existing freeSample wire field.

## Step 4: Verify browser behavior and capture the completed interface

Exercised the built React bundle in Chromium with deterministic API fixtures, then checked the actual Go handlers against PostgreSQL. The browser run exposed an attachment-selection lifetime bug that helper tests could not observe. Corrected it and verified publication, partial failure, artifact switching and draft saving.

### Prompt Context
**User prompt:** See Step 3.
**Assistant interpretation:** Verify the shipped workflow and produce screenshots suitable for the diary and technical report.
**Inferred user intent:** Evidence that the form works, with clear records of implementation problems and their resolutions.
**Commit (code):** 0bd8a10 — "feat(studio): add guided artifact uploader with recoverable publication".

### What I did
- Added `scripts/01-check-uploader-browser.mjs` using the real built catalog bundle and explicit mocked API responses.
- Captured filled desktop/mobile forms, successful publication, partial failure, and focused overview/storefront screenshots in `sources/implementation/`.
- Fixed attachment selection by copying FileList synchronously before clearing the input. Added HTML drag-and-drop and navigation protection during uploads.
- Replaced unnecessary cryptographic IDs for local thumbnail keys with a form-local counter.
- Checked strict StudioMe decoding and all returned limit fields in TestClientContract.
- Ran the frontend check/build, frontend tests, PostgreSQL-backed catalog/client tests, full Go suite, lint and logcopter checks.

### Why
- Pure orchestration tests cover ordering and failure boundaries; browser interactions catch DOM and React state-lifetime mistakes. Real backend tests independently verify permissions, persistence and wire contracts.
- A thumbnail key only needs uniqueness within its mounted form. A counter avoids an unnecessary secure-context dependency.

### What worked
- All 18 frontend tests passed. TypeScript and production bundling passed.
- `TEST_DATABASE_URL=… go test ./pkg/catalog ./pkg/catalog/client -count=1` passed, followed by `TEST_DATABASE_URL=… go test ./...` with all packages passing.
- `make lint logcopter-check` passed; golangci-lint reported zero issues.
- Browser receipt records zero page errors, no horizontal overflow at 1440/390 px, successful publish, durable draft recovery, disabled replay, selection reset, full manifest preservation and draft save.
- Screenshot inspection confirmed readable responsive sections and a factual failure message identifying the saved draft.

### What didn't work
- Chromium initially failed inside the restricted sandbox: `FATAL:content/browser/sandbox_host_linux.cc:41 ... Operation not permitted (1)`. Running the browser check through approved escalation resolved this environment limitation.
- The first failure-state check timed out waiting for `Upload stopped`; recorded HTTP calls showed no attachment mutation. The event handler cleared the input before the React updater read FileList. Snapshotting FileList before setState fixed it.
- Staging cover media on the HTTP fixture origin exposed `crypto.randomUUID is not a function`. Local counter keys removed that dependency.

### What I learned
- A deferred state updater must receive durable values, not read mutable DOM input state after the event handler clears it.
- Screenshot preparation itself exercises useful paths: staging a cover caught a requirement that the text-only run had not reached.

### What was tricky to build
- Screenshots of a sticky sidebar depend on the current scroll position. Resetting scroll before full-page captures keeps the summary aligned with the first form section.
- The browser fixture now verifies the attachment filename before submitting, so a regression cannot silently bypass the intended error scenario.

### What warrants a second pair of eyes
- Browser screenshots use mocked APIs. They demonstrate actual rendered UI and client orchestration, not a production deployment or a single browser-to-PostgreSQL end-to-end run.
- Publication remains two operations. If the page publishes and release publication fails, the completed page mutation remains public.

### What should be done in the future
- Complete and push the requested vault report. Production deployment is outside this implementation request.

### Code review instructions
- Inspect the attachment handler in uploader.tsx and recovery branches in uploader-submit.ts.
- Reproduce screenshots with `node ttmp/2026/10/20/FRANCOIS-018--slop-uploader-for-artifacts-and-release-metadata/scripts/01-check-uploader-browser.mjs` from the repository root after `pnpm build`.
- The browser script uses the installed Playwright path documented in its first lines.

### Technical details
- [Filled desktop](../sources/implementation/uploader-filled-1440.png)
- [Filled mobile](../sources/implementation/uploader-filled-390.png)
- [Published release](../sources/implementation/uploader-published-1440.png)
- [Interrupted upload](../sources/implementation/uploader-recovery-1440.png)
- [Machine-readable browser receipt](../sources/implementation/browser.json)

## Step 5: Deliver the technical report to go-go-parc

Wrote a 3,613-word textbook-style article covering the implemented data model, component boundaries, API sequence, validation, manifest preservation, browser file lifetimes, partial-write recovery and verification. Copied all six screenshots into the vault's dated `_assets` directory and pushed the report to origin/main.

### Prompt Context
**User prompt:** See Step 3.
**Assistant interpretation:** Produce a self-contained technical deep dive, commit it and push the vault.
**Inferred user intent:** Preserve the implementation knowledge and visual evidence in the normal Obsidian project archive.
**Commit (code):** 16ad125 — "fix(studio): retain staged attachments and verify uploader workflows".
**Commit (vault):** 87fe8c0 — "docs: explain Slop Uploader implementation and recovery with screenshots".

### What I did
- Followed the obsidian-vault-writing and textbook-authoring skills: new dated article, prose-first explanation, concrete example, diagrams, pseudocode, API table and source map.
- Updated the clean vault from origin/main before adding the report.
- Verified embedded image paths and committed only the report plus its six prefixed screenshots.
- Pushed vault main; remote receipt was `23e13eb..87fe8c0 main -> main`.

### Why
- The report explains durable implementation choices rather than repeating a chronological change list. Screenshots are copied into the vault so the article remains self-contained.

### What worked
- Vault push succeeded and `git status --short --branch` reports `main...origin/main` with no local changes.
- All implementation tasks are checked. The source repository's unrelated FRANCOIS-017 diary remains untouched and unstaged by this task.

### What didn't work
- The first staging command combined git -C with an unquoted relative screenshot glob; zsh expanded the glob in the Francois directory and reported `no matches found`. No files were staged by that command. Running from the vault working directory resolved it.

### What I learned
- Shell glob expansion occurs before git applies its -C directory option. Set the tool's working directory for commands with repository-relative globs.

### What was tricky to build
- The article distinguishes real server tests from mocked-API browser evidence and explicitly records the lack of production deployment.

### What warrants a second pair of eyes
- Review the publication boundaries and the scope of the test evidence before merging the implementation branch.

### What should be done in the future
- Merge and deploy the implementation when requested. No remaining work is required for this implementation/report delivery.

### Code review instructions
- Read the article and diary alongside commits 0bd8a10 and 16ad125. Screenshot reproduction and its receipt are under scripts/ and sources/implementation/ in this ticket.

### Technical details
- Vault note: `Projects/2026/10/20/ARTICLE - Slop Uploader - A Small Publication Workflow with Explicit Recovery.md`.
- Vault root: `/home/manuel/code/wesen/go-go-golems/go-go-parc`.
- Assets: six `Projects/2026/10/20/_assets/francois-uploader-*.png` files.

## Step 6: Run the real local uploader and open it

Started the current Go application in tmux against an isolated local PostgreSQL database and opened the real uploader in the user's browser with a disposable local admin session. This is a functioning local server, not the screenshot fixture.

### Prompt Context
**User prompt (verbatim):** "run locally and open for me"
**Assistant interpretation:** Start the implemented app and open its uploader ready to use.
**Inferred user intent:** Try the new workflow interactively.

### What I did
- Reused the running local PostgreSQL container and created `francois_uploader_preview`.
- Ran `go run ./cmd/francois catalog serve` in tmux session `francois-uploader`, with localhost:8090, backend fake, migrations enabled and release files under `/tmp/francois-uploader-preview/releases`.
- Inserted a local admin and a one-day session into this isolated database. A one-time loopback HTTP helper set the HttpOnly development cookie and redirected the browser to `/studio/upload`, then stopped.

### Why
- The isolated database permits actual uploads without production changes. A disposable local session avoids needing external identity infrastructure for this preview.

### What worked
- All migrations applied; the server logged readiness.
- Authenticated `/api/studio/me` returned the admin role. Browser handoff completed and `/studio/upload` returned HTTP 200.

### What didn't work
- Initial tmux inspection required execution outside the restricted sandbox; approved escalation resolved it. No server failure occurred.

### What I learned
- Only PostgreSQL was running from the earlier local stack; the app needed a new tmux session.

### What was tricky to build
- Local session handoff uses a random single-use URL on loopback and stops after delivery; no authentication shortcut was added to application code.

### What warrants a second pair of eyes
- The model backend is deterministic fake; this preview does not exercise live LLM providers or billing.

### What should be done in the future
- Stop the preview when no longer needed; production remains unchanged.

### Code review instructions
- Inspect `tmux capture-pane -t francois-uploader -p`. The uploader is at http://localhost:8090/studio/upload.

### Technical details
- Listener: 127.0.0.1:8090. Database: francois_uploader_preview on local PostgreSQL port 5437.
- Temporary browser handoff helper: `/tmp/francois-uploader-open.py`; session secrets were generated in memory and not committed.

## Step 7: Match Studio styling across every uploader control

Corrected the uploader's remaining browser-default controls. Primary and secondary actions now use Studio's existing button variants; file picker buttons, the artifact selector, checkbox accents, focus rings and section labels follow the same visual system. Replaced incidental gray values with the site's color tokens.

### Prompt Context
**User prompt (verbatim):** "make sure the style matches, your buttons are not styled and stuff"
**Assistant interpretation:** Review the entire form's controls against the existing Studio styles and fix the inconsistencies.
**Inferred user intent:** A finished, consistent interface rather than a working form with browser-default widgets.

### What I did
- Applied `btn btn-soft` to secondary actions and `btn btn-ink` to Publish, including media, attachment and recovery actions.
- Styled native `::file-selector-button` controls without replacing accessible file inputs. Styled the artifact selector with the existing underline treatment and a dropdown indicator.
- Aligned checkbox color, field-label typography, section headings, borders and focus outlines with the existing design tokens.
- Captured fresh screenshots under `sources/style-review/`, preserving the original implementation evidence.
- Rebuilt embedded assets and restarted the real local server against the same preview database.

### Why
- Existing button classes provide consistent padding, hover states, disabled states and typography. A scoped CSS adjustment preserves native input behavior and avoids another component abstraction.

### What worked
- TypeScript check and build passed. Browser workflow checks passed at 1440/390 pixels with zero page errors or horizontal overflow.
- Visually inspected the desktop overview: file chooser and action buttons now have deliberate flat styling, with one dark primary action.

### What didn't work
- The earlier implementation left secondary buttons and native file controls unstyled. This review corrected that omission; no new verification failure occurred.

### What I learned
- Styling only the primary action was insufficient: media actions, file-selector pseudo-elements and recovery controls need the same review.

### What was tricky to build
- File inputs require styling their native button pseudo-element; applying a button class to the input alone does not style its internal chooser button.

### What warrants a second pair of eyes
- Compare `sources/style-review/` with the original evidence and the surrounding Studio controls.

### What should be done in the future
- No remaining work for this styling correction. The prior vault report remains a historical record of the initial implementation.

### Code review instructions
- Review uploader.tsx button classes and the scoped uploader section of catalog.css. The screenshot script accepts an optional output directory for later visual comparisons.

### Technical details
- Local server: http://localhost:8090/studio/upload, tmux session `francois-uploader-style`.
- Existing local admin session, database and uploaded files are preserved.

## Step 8: Align the whole page with Studio's layout and typography

Paused deployment as requested and replaced the uploader's independent hero/card layout with the existing Studio page structure. The title is now “Upload artifact” in a shared lib-head; the editor uses the shared editor-form width, aligns with the page's left edge and uses simple section dividers with regular-weight headings.

### Prompt Context
**User prompt (verbatim):** "hold on, stick to the design language of the other pages as well. Like the title, the font weights, the width, etc..."
**User continuation (verbatim):** "continue"
**Assistant interpretation:** Continue the visual alignment, keeping deployment paused.
**Inferred user intent:** Make the uploader part of the existing site rather than an independently styled landing page.

### What I did
- Compared Artifacts, Media, API tokens and ArtifactEditor markup and CSS.
- Reused library/lib-head and editor-form instead of the centered 1180px wrapper and custom hero typography.
- Removed the eyebrow, promotional heading, numbered uppercase section labels and enclosing card borders.
- Used shared spacing/color tokens, body-size section headings, 500-weight capability labels and the editor's 1240px maximum width.
- Added a Media-page reference screenshot and compared computed title typography and positioning against the uploader. Saved all captures under sources/studio-alignment/.
- Rebuilt and restarted the local server, preserving its database and login.

### Why
- Reusing existing layout classes makes the title and width follow the same rules as adjacent Studio screens. Adjusting only button styles had left the larger visual hierarchy inconsistent.

### What worked
- TypeScript and build passed. Chromium's comparison verified identical title font size (40px), weight (500), line height (42px), letter spacing (-1.2px), and desktop position (40px left, 204px top).
- Workflow and no-overflow checks passed at 1440/390 pixels. Visually inspected the new overview.

### What didn't work
- The first position comparison ran after form filling had scrolled the uploader; its top coordinate was -1676px versus the reference's 204px. Resetting both pages to the top before measurement corrected the test setup.

### What I learned
- Page-level layout conventions matter as much as control styling. The shared Studio header already supplies the required type scale and spacing.

### What was tricky to build
- Visual comparisons must use the same scroll position and viewport. Browser automation scrolls focused fields into view automatically.

### What warrants a second pair of eyes
- Review the new Media reference and uploader overview together; production deployment is still on hold.

### What should be done in the future
- Resume deployment only after the user releases the hold.

### Code review instructions
- Review the uploader header and editor-form classes, then scoped CSS changes. Reproduce the browser script with output directory ../sources/studio-alignment/.

### Technical details
- Local URL: http://localhost:8090/studio/upload.
- Tmux session: francois-uploader-aligned. No production writes were made.

## Step 9: Protect unsaved uploads during browser history navigation

Addressed PR #18's P2 review about Back/Forward bypassing the uploader's click and beforeunload guards. Navigation approval now belongs to the SPA router and runs before its route state changes. Cancelling a traversal restores the previous history position without unmounting the uploader or replacing its staged File objects.

### Prompt Context
**User prompt (review excerpt):** "When the form is dirty, browser Back/Forward navigation bypasses both guards registered here ... register a route/history guard in addition to the anchor-click handler."
**Assistant interpretation:** Fix the reported data-loss path and verify it through real browser history traversal.
**Inferred user intent:** Preserve staged edits consistently regardless of how the user navigates away.

### What I did
- Added registerNavigationGuard to the existing router, used by both navigate() and popstate.
- Recorded positions on router-owned history entries. A rejected popstate returns to the previous position while keeping Redux route state and the form unchanged; restoration events do not prompt again.
- Replaced the uploader's document-click handler with a registered guard and cleanup. Kept beforeunload for document navigation/reload.
- Added browser scenarios with a history entry on each side of the uploader: cancel Back, cancel Forward, cancel a link, accept Back, follow Forward, and verify guard cleanup.
- Verified the exact staged filename and edited title survive both cancelled history moves.

### Why
- The router is the common boundary for SPA links, programmatic navigation and history traversal. Handling only clicks misses Back/Forward; dispatching before confirmation would already destroy the form.

### What worked
- pnpm check, pnpm build and all 18 frontend tests passed.
- Browser navigation scenarios and the complete existing uploader browser suite passed with zero page errors. Evidence is in sources/history-guard/browser.json.

### What didn't work
- The initial browser script awaited click completion before dismissing its synchronous confirmation dialog, leaving the test blocked. Starting the click promise, dismissing the dialog and then awaiting completion corrected the harness.

### What I learned
- popstate is delivered after the address changes. Cancelling must restore the browser history position as well as refrain from updating the application route.

### What was tricky to build
- Restoration itself emits popstate. A restoration flag prevents a second prompt and prevents new application navigation from racing the return traversal.

### What warrants a second pair of eyes
- Review route state dispatch timing and guard cleanup, especially that a cancelled traversal retains the mounted component.

### What should be done in the future
- Keep all same-document application navigation routed through navigate(). Production deployment remains on hold.

### Code review instructions
- Start with ui/catalog/store.ts, then the uploader effect. Reproduce the browser check with output directory ../sources/history-guard/.

### Technical details
- Existing history state is preserved on replaceState; newly pushed SPA entries carry francoisRoutePosition.
- No dependency or new router framework was introduced.

## Step 10: Rebase onto merged main and deploy to production

Rebased onto PR #18's merge commit, published its CI-built image through the normal GitOps workflow, and verified both production domains. The prior production image was sha-08332bb, so this rollout also applied four already-merged migrations for year funding, token ceilings, payment state and early access. Fresh database and artifact backups completed before rollout.

### Prompt Context
**User prompt (verbatim):** "ok, merged, rebase deploy"
**Assistant interpretation:** The prior deployment hold is released; deploy the merged implementation and verify it.
**Inferred user intent:** Make the completed uploader and review correction available on the live site.
**Source commit:** 21b8458cbe75cb34e2219d75bd6536a5f38df80d — merge of PR #18.

### What I did
- Ran git fetch and git rebase --autostash origin/main; preserved the unrelated FRANCOIS-017 diary edits.
- Verified main's publish-image run 39539386083 completed successfully and published sha-21b8458.
- Ran existing backup CronJobs as postgres-backup-francois-21b8458 and francois-backup-21b8458; both completed.
- Reviewed infrastructure PR #374: exactly one image-line change. Its validate-gitops check passed before merge.
- Merged the pinned head a76394cf18f17cb9cc648ef4080502d007e6394f. GitOps merge commit is 73f03d45212561022275e2b8257474c150a757a5.
- Refreshed Argo, waited for the exact image in the Deployment, then waited for rollout readiness.
- Added and ran scripts/02-verify-production.py. It performs read-only checks and writes sources/production-rollout.json.

### Why
- A previously healthy pod is not evidence that a new release has deployed. Checking the exact image before rollout status and comparing served assets against source proves the intended code reached both sites.
- The pending migrations change the active settings representation; the pre-deployment backups preserve a recovery point.

### What worked
- Production runs ghcr.io/go-go-golems/francois:sha-21b8458 with one updated, ready replica.
- Argo reports Synced/Healthy at infrastructure revision 73f03d4.
- Startup logs confirm successful application of 016_year_funding.sql, 017_daily_tokens.sql, 017_year_payment_state.sql and 018_early_access.sql.
- Health, readiness, home, pricing, sign-in, uploader shell and RSS checks passed on slop.boutique and francois.yolo.scapegoat.dev.
- Served catalog.js and catalog.css match the merged source byte-for-byte. JavaScript includes the history-navigation guard.

### What didn't work
- No deployment or verification failure occurred. The image build and GitOps validation were awaited before changing production.

### What I learned
- Earlier application merges were not yet running in production. Comparing against the live image identified the additional migrations before deployment.

### What was tricky to build
- Argo's old deployment initially remained healthy while the new desired revision was being processed. Waiting for the desired image avoided falsely reporting completion during that interval.

### What warrants a second pair of eyes
- Production smoke checks are read-only and anonymous; they verify the uploader shell and exact deployed assets, not a new authenticated production upload.

### What should be done in the future
- No further action is required for this rollout. Older unmerged image-pin PRs were left unchanged and should not be deployed over the current image.

### Code review instructions
- Read sources/production-rollout.json and scripts/02-verify-production.py. CI run: https://github.com/go-go-golems/francois/actions/runs/39539386083. Deployment PR: https://github.com/wesen/2026-03-27--hetzner-k3s/pull/374.

### Technical details
- Live uploader: https://slop.boutique/studio/upload.
- Verification time: 2026-10-21T03:40:53Z (20 October local time).
- No credentials, production data or session cookies are stored in the receipt.
