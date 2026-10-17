---
Title: Implementation diary
Ticket: FRANCOIS-009
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T19:07:30.511587596-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation diary

## Step 1: Inventory exports, package HTML and test actual hosted startup

Created FRANCOIS-009 for real artifact proofs of concept. The ZIPs contain ordinary HTML and classic JavaScript rather than a package-manager project, so packaging consists of inlining local scripts and embedding RAG resources. Original downloads are unchanged; a private RAG dataset is deliberately excluded from the public source tree and replaced with public sample documentation.

### Prompt Context

**User prompt (verbatim):**

> ok, can we publish some real html artifacts, potentially adapting them first (locally) to make sure they work, and as Proof of Concept?  Apparently they're zip now so we probably have to build them first or so. Create a new docmgr ticket and then import (and write up how you did it) Lattice Rift/Phantasmagoria.zip, Tabula Workbench.html Lattice Deck.html and RAG Workbench.zip from ~/Downloads.

**Assistant interpretation:** Create a ticket, preserve/download-inventory the originals, prepare reproducible single-HTML releases, test and adapt them against the real local runtime, then publish verified releases and explain the procedure.

**Inferred user intent:** Real usable catalog applications demonstrating hosting, interactive rendering, storage and model capabilities.

### What I did

- Applied docmgr and diary skills, created the ticket/guide/diary and five implementation tasks, and branched as feat/francois-009-real-artifact-imports from clean main.
- Found separate Lattice Rift.zip and Lattice Phantasmagoria.zip exports. Asked an optional clarification and prepared both as separate PoCs while working on the three unambiguous workbenches.
- Added ordered copy/build/browser-check scripts under this ticket. Recorded source SHA256/size/file inventory, extracted bounded safe ZIP entries and preserved original source bytes under examples/imports.
- RAG's corpus.json contains customer/project notes with publish:false metadata. Removed the working copy, excluded it from the copy script and Git, and authored a five-document public Francois/RAG corpus. The original ZIP remains untouched.
- Added one release.html and explicit capability manifest per application. Rift/Phantasmagoria need no hosted capability; Tabula uses sample/downloads; Deck uses sample/db/user/room; RAG uses sample/db/user/downloads.
- Embedded RAG's 29 presets, public corpus and library source into JSON rather than adding runtime file endpoints or overriding fetch globally. Replaced its four local resource fetches explicitly, selected minimal lexical search initially, and lowered the existing default asynchronous map concurrency to one for the hosted per-user model limit.
- Uploaded/published initial local versions through the existing studio validation/upload/publish APIs using the existing admin browser session. The validator accepted all five; only the original pinned CDN scripts produced informational findings.
- Opened every application on its actual isolated local workspace origin and captured screenshots. Rift, Phantasmagoria, Tabula and RAG started without page errors or failed network requests. RAG built five documents into 15 chunks with all declared capabilities available.
- Deck revealed a real integration error: its collaboration code calls .catch on room.presence(), while the runtime's presence API is fire-and-forget. Added Promise.resolve around the two application call sites in the packaged output. The source export remains byte-preserved; the new release is awaiting re-validation.

### Why

The service deliberately hosts one HTML release. Inlining the exported scripts and embedding static resources is sufficient; there is no need for another asset server or a framework build pipeline. Testing inside the hosted iframe exercises capability declaration, grants, browser origin behavior and the real runtime rather than only static rendering.

### What worked

- Safe copying/building completes with source/build inventories. Embedded RAG JSON round-trips its library source exactly.
- Initial release sizes: Rift 113,625 bytes, Phantasmagoria 152,468, Tabula 1,067,766, Deck about 426 KiB, RAG about 502 KiB; all comfortably fit the existing HTML limit.
- All five local drafts validated and published as version 1.0.0. Baseline browser evidence is in scripts/03-local-browser-check.js and various/screenshots.
- Tabula's real initial computation reports 151 nodes; Deck builds 11 nodes/two slides before its collaboration error. RAG's five safe sample documents load without any companion file requests.

### What didn't work

- The user's slash-style first name is not a literal Downloads path; discovery found two independent ZIPs.
- The copy script initially assumed a fixed parent depth, placing copies under ttmp/2026/examples. Corrected repository discovery by locating go.mod, moved only the generated copies and regenerated the inventory. Originals were never changed.
- Several guessed source filenames/globs did not exist; followed actual repository file listings instead. A zsh unmatched glob interrupted one bounded read; concrete filenames continued the inspection.
- Dynamic import in the browser tool failed with ERR_VM_DYNAMIC_IMPORT_CALLBACK_MISSING, and require is unavailable. Browser file inputs provide supported access to the requested local HTML files instead.
- Automatic approval review rejected exporting authenticated browser session state to /tmp because local testing did not specifically authorize copying cookies. Did not retry/bypass that export; local imports use the browser-held session directly. Normal sign-in will be used for production, with no cookie export.
- Ordinary sandbox shell reads stalled; scoped authorized reads with login disabled completed. The live service was not modified to work around shell/tool issues.

### What I learned

The downloaded artifacts already implement the supported claude.use contract. Most work is packaging; real startup exposed a narrow application assumption about the presence API. The RAG ZIP mixes reusable program code with a private corpus, so importing the app must not implicitly publish that dataset.

### What was tricky to build

Inline scripts must escape closing script sequences; embedded JSON must escape less-than signs and preserve library-source text. Keeping modifications in a deterministic builder preserves both original exports and exact deployed output. Published local workspaces retain release pins, so revised test releases require an explicit upgrade rather than simply refreshing Open.

### What warrants a second pair of eyes

Review the excluded RAG corpus, public demo corpus, capability manifests, exact resource replacements and Deck presence correction. Initial startup screenshots are not proof of every primary interaction or successful model calls; those remain separate validation tasks.

### What should be done in the future

Publish/upgrade revised local Deck/RAG versions; test primary application interactions, persistence and small representative real model calls; then publish to the production studio API and verify those origins. Finish the guide with exact versions/IDs and limitations.

### Code review instructions

Start with scripts/01-copy-sources.py and 02-build-imports.py, sources/inventory.json/builds.json, each manifest and the public demo corpus. The original Downloads files are the byte reference. No private corpus or browser session state is committed.

### Technical details

Initial local instances: Rift ins_a8fbd875f986beb842b6; Phantasmagoria ins_d6c846b4fb77486c45ad; Tabula ins_e0f18c608c25a8a083bd; Deck ins_9b1a6c16280179a82fc3; RAG ins_fb70d88f0d7d6a2a659f. Production publication is not yet performed.


## Step 2: Validate interactions, persistence and real sampling; publish initial releases

The local workbenches now exercise real hosted capabilities rather than only rendering their initial screen. Three small sequential uncached sampling calls returned pong through LunaRoute. Production publication succeeded for all five applications, and production interaction testing found a viewer control overlap in Rift that required one further application adaptation.

### Prompt Context

See Step 1 for the verbatim request. This step implements local qualification and real publication. **Commit (code):** 05a327a — Import real artifact sources and package local hosted proofs of concept.

### What I did

- Published the corrected Deck and RAG local builds as 1.0.1 and explicitly upgraded their workspaces. Deck starts without its previous presence error.
- Tested Rift Freeze/Thaw and Phantasmagoria A advancing Boot to Cartridges. Changed Tabula Inputs!B2 from 420 to 421, reloaded and observed 421, then restored 420. Its workbook has 151 nodes.
- Ran Deck's scripted “Spin faster and make it pulse” action: successful speed 2.4/core mesh#core result and no agent errors.
- Verified RAG's model-concurrency query ranks Model Limits / Concurrency first. Uploaded the public poc-note.md fixture and verified it remains after reload through hosted document storage.
- Made one sequential uncached quick-tier sampling call from each model-capable app: Tabula 623 ms, Deck 613 ms, RAG 1,076 ms; all returned pong using lunaroute/glm-5.3-flash. These are capability-path checks, not claims about every agent workflow.
- Ran TruffleHog filesystem scanning with verification disabled on examples/imports: zero findings. No private corpus or session state is included.
- Added production operator scripts using normal Vault-backed ZITADEL login with credentials/session held in memory. No cookie/state export. Published 0.1.0 releases and catalog descriptions/write-ups for all five on slop.boutique.
- Production Rift clicking failed because the host Close link intercepted Freeze. Added only 96px right padding on Rift's header in the builder, rebuilt, published locally as 1.0.1 and verified a real pointer click at 1440×1000. Published the correction as production 0.1.1; existing workspaces require explicit release upgrade.

### Why

Preserving original sources and expressing adaptations in one deterministic builder keeps each hosting edit reviewable. A real browser click catches layout interference that dispatching synthetic DOM events would conceal. Existing hosted publishing, document and sampling APIs suffice; no platform changes, service deployment or package dependencies are needed.

### What worked

Local functional evidence is recorded in various/local-functional-receipt.json. Initial production release IDs and validation checks are in various/production-receipt.json; that file is updated after the final clean verification run. Source and build inventories retain exact hashes. The original RAG dataset is excluded before extraction into the public tree.

### What didn't work

- An initial retry waited for an exact Freeze button after an earlier interaction had left it labelled Thaw: `TimeoutError: locator.evaluate: Timeout 30000ms exceeded`. Changed the test to accept either toggle state and restore it.
- Source searches guessed nonexistent pkg/hosted/internal paths; used actual pkg/catalog paths after repository discovery.
- Production verification reported `locator.click: Timeout 30000ms exceeded` and `<a href="/" class="viewer-close u" aria-label="Close application">Close</a> intercepts pointer events`. Fixed Rift header spacing rather than forcing the click.
- One multi-file patch had a stale expected line and was rejected atomically; corrected its context and applied it successfully.
- The first publication script assumed a release list for a nonexistent artifact would be empty; source inspection showed 404, so explicit 404 handling was added before execution. Existing versions must match the local HTML digest before reuse.

### What I learned

Production uses a minimal floating Close control over the app frame. Imported top-right controls need enough space for it. Releases are immutable and instances stay pinned, so uploading a correction and testing an old instance are separate operations. Empty manifests are normalized by the platform to its default sampling declaration; the graphics apps themselves make no model calls.

### What was tricky to build

The normal admin login can include a second-factor setup screen; the operator follows the documented Skip path and never prints credentials. File upload uses a browser-held session locally and in-memory Playwright request authentication in production. Operator reruns reuse a published version only after checking its HTML SHA256, and must inspect/upgrade the test workspace before exercising it.

### What warrants a second pair of eyes

Review the public corpus boundary, deterministic builder assertions, capability manifests and the release SHA256 check. Production access settings are deliberately unchanged while the optional limited-tour preference is pending. Full multi-user collaboration and every advanced preset are outside these representative PoC checks.

### What should be done in the future

Finish the clean production verification, collect screenshots and usage evidence, finalize the guide and tasks, then commit/push the completed ticket.

### Code review instructions

Read scripts/02-build-imports.py, 04-publish-production.mjs and 05-verify-production.mjs. Rebuild with Python, compare sources/builds.json and release bytes, and inspect local/production receipts. Production publishing mutates the real catalog and requires operator credentials; do not run it as an ordinary unit test.

### Technical details

Production 0.1.0 IDs: Rift rel_cdc4426a42fefb2c512d77f6 (superseded by 0.1.1 rel_1e376f5c236cec83a1ee6f53); Phantasmagoria rel_7666279d21ac37718f31e4e7; Tabula rel_a9d88eeef8c8cc334341e117; Deck rel_08a7a6168371c71f285397d6; RAG rel_48bc7b49c104ed89526e6249. No global access or quota settings were changed by publication.


## Step 3: Prepare report evidence and finish production qualification

The user requested a diary and screenshots specifically for a later vault report. Added a dedicated evidence index with caption guidance, exact receipt links and explicit boundaries between visible rendering, scripted execution, persistence and real model sampling. Preserved the export sources and extended the same narrow Close-link clearance to the three workbench toolbars after inspecting the production screenshots.

### Prompt Context

**User prompt (verbatim):** “write a diary and take screenshots and all so we can use them for our vault report when we are done”

**Assistant interpretation:** Preserve detailed chronological implementation evidence and actual local/production screenshots, organized so a later technical vault report can be written from verified facts.

**Inferred user intent:** A report-ready record of implementation decisions, real application behavior and visual results, including problems and their resolution.

**Commit (code):** a63e9ab — Qualify imported apps and publish production proofs of concept.

### What I did

- Added reference/02-screenshot-and-report-evidence-index.md with figure captions, local/production screenshot links, receipts, source/build inventory references and interpretation limits.
- Captured unauthenticated production catalog and RAG detail-page screenshots with scripts/06-public-evidence.mjs. Verified all five public pages return HTTP 200 and all five artifacts are present in /api/catalog and /apps/feed.xml.
- Visually inspected production Rift, Phantasmagoria, Tabula and Deck captures. Rift's Freeze control is separate from Close after the fix. Tabula/Deck retained controls close to that overlay, so applied the same minimal 96px top-toolbar clearance to Tabula, Deck and RAG.
- Published those toolbar builds locally as 1.0.2, explicitly upgraded the existing instances and reran all five local startup/screenshot checks: zero page errors and zero failed requests. RAG shows five bundled guides plus the persisted public upload.
- Published final production 0.1.1 releases for Tabula, Deck and RAG, retaining Rift 0.1.1 and Phantasmagoria 0.1.0. The operator script checks stored HTML SHA256 on version reuse.
- Replaced fixed reload sleeps with explicit workbook/index readiness assertions. Added incremental per-app receipt saving, a --verify-only mode and bounded model-ledger projection without user emails or prompts.
- Rewrote the import guide around the final system: immutable release/pinned workspace model, safe source boundary, exact builder adaptations, Mermaid flow diagram, API contract table, operator steps, actual URLs and qualification limits.
- Rebuilt twice and compared every release hash plus sources/builds.json: identical output. Python AST parsing and node --check pass for the authored scripts.

### Why

Screenshots should support a specific technical claim rather than stand in for functional testing. The evidence index explains which claims come from images and which come from browser/API assertions. Per-app result saving avoids losing successful evidence when a later workflow exposes a problem. Application toolbar edits are small enough to remain explicit in the deterministic builder; no platform styling changes or new abstraction is necessary.

### What worked

The final local source-to-release build is deterministic. Public discovery and release RSS work without authentication. Production browser checks exercise physical controls, a reactive workbook, executable scripted slide changes and real model calls on isolated app origins. Final results, including the remaining RAG storage/model checks, are recorded in the checkpoint below after the run completes.

### What didn't work

- The first production reload test for Tabula evaluated too early: `frame.evaluate: TypeError: Cannot read properties of undefined (reading 'S')`. Waiting for window.__tabula.S.session replaces the timing assumption.
- The first RAG result read ran before a ranked result was available: `TypeError: Cannot read properties of undefined (reading 'doc')`. The test now enters the known public query and waits for matching query text plus a nonempty result list.
- One publication retry still exercised the old pinned Rift workspace and reproduced the Close-link timeout. Added explicit current-release inspection and upgrade before reloading the frame; the corrected pointer toggle subsequently passed.
- A local Tabula Download book probe timed out waiting for the browser download event (`Timeout 15000ms exceeded`). A follow-up exact-menu-button probe also timed out after toggling menu state. No application export success is claimed; all export formats remain outside qualification.
- docmgr doctor rejected the public Markdown upload fixture because it initially lacked frontmatter. Added normal ticket metadata. The remaining missing_numeric_prefix warning is intentional: poc-note.md is a user-facing upload fixture rather than a numbered authored document.
- Some multi-file patch contexts were stale and were rejected without partial edits; corrected them against the actual text. A few guessed source paths/globs failed; searches then used concrete repository paths. No platform behavior was changed to work around tool issues.

### What I learned

Document readyState does not mean asynchronous capability setup or reactive query computation has completed. Correct browser checks wait for the actual application state they need. A release upload does not upgrade a pinned instance. Browser aborts caused by deliberate reload/upgrade and closing long-lived event streams are expected and remain visible in the raw receipt.

### What was tricky to build

The first successful stage-agent result is scripted executable code, while the separate sampling probe is a real provider call. The report must keep those claims distinct. Likewise, Tabula uses browser-local draft persistence, while RAG uses hosted document storage; a single generic “persistence works” claim would hide a meaningful difference.

### What warrants a second pair of eyes

Review the excluded corpus, explicit capabilities, byte-preserving sources and deterministic embedding. Inspect final screenshots for toolbar usability. Confirm final receipt HTML digests match sources/builds.json, and verify model ledger rows correspond to the tested artifact instances. Advanced presets, multi-user room collaboration and all export formats are explicitly not certified.

### What should be done in the future

Use the evidence index and guide as the source package for the requested later vault report. The report itself is not authored in this ticket step. Decide separately whether to enable the existing bounded anonymous tours; global policy remains unchanged while that preference is pending.

### Code review instructions

Start with the guide and evidence index, then inspect scripts/02-build-imports.py and 04–06 operator/browser scripts. Compare each generated release digest to the build/production receipts. Images live under various/screenshots. Do not commit credentials or authenticated browser state; none is needed to consume these evidence files.

### Technical details

Final workbench local release IDs: Tabula rel_a7a9b9710959392d6ab3596a, Deck rel_4bfc816142e8dc13bb77dac4, RAG rel_b3360f57a4727f524f10439e. Final production workbench IDs: Tabula rel_d8227318d8645a1409d1a73c, Deck rel_7eb142a73b88a690d042c952, RAG rel_87f08be07833f2a4c43ac4e1. Production remains the existing platform deployment; these changes publish content and require no k3s rollout.


#### RAG test diagnosis correction

The RAG failure above was initially interpreted as a readiness race. Source inspection of source/rag.js:decorateResult showed the actual cause: W.lastResult is a runtime result object with signals.q and a non-enumerable top(n) method. Its toJSON() exposes q and a top array. The earlier local inspection used JSON.stringify, so reusing that serialized shape for direct object inspection was incorrect. The production test now waits for lastResult.signals.q and calls lastResult.top(1)[0]. This corrects the test without changing the application, corpus, runtime or release. Only RAG is rerun using --verify-only --artifact=rag-workbench; the four completed checks are preserved. This is a meaningful test-authoring mistake, not evidence of an application retrieval failure.


#### Final production checkpoint

All five production browser checks pass. Generated HTML SHA256 values match the five published release receipts. There are no page errors; recorded failed requests are only expected ERR_ABORTED launch/event-stream requests from explicit reload/upgrade. RAG ranks Model Limits / Concurrency first and retains poc-note.md after reload. Three model-capable apps returned pong through lunaroute/glm-5.3-flash, and the bounded production model ledger contains all three artifact attributions with token counts.

- tabula-workbench: real quick-tier call 2299 ms; response pong.
- lattice-deck: real quick-tier call 1585 ms; response pong.
- rag-workbench: real quick-tier call 1010 ms; response pong.

Final release versions: Rift, Tabula, Deck and RAG 0.1.1; Phantasmagoria 0.1.0. The complete screenshot/evidence index is reference/02-screenshot-and-report-evidence-index.md. Expected browser-abort observations are retained; no failed feature assertion is concealed. No global access settings were changed. Anonymous tours remain disabled and authenticated workspace access is available. The production ledger omits user emails, prompts and session data.


Ticket tasks are complete and docmgr closed FRANCOIS-009 successfully (close-cd083c793c7c7fde5538b8ddbd4b9b5a, committed). Its completed recovery journal was verified and archived under /tmp/francois-009-completed-docmgr-operations; permanent status/changelog remain in the ticket. Final doctor reports only the intentional upload-fixture filename warning. Authored diffs pass whitespace checks; scripts pass syntax checks; no Go/platform code changed, so a backend test-suite rerun is not relevant to these content imports. Thirteen PNG screenshots are preserved, including a diagnostic capture. Milestone commits and the final evidence commit are on feat/francois-009-real-artifact-imports.


## Step 4: Curate multiple real screenshots and simplify public downloads

The user asked for screenshots that present the applications well, multiple images per application, publication through the token/CLI and public access. Composed sixteen real screenshots using built-in scenes, presenter mode, workbook-generated slides and retrieval views. Also investigated the duplicate Download HTML links: the public page rendered every published release, so 0.1.0 and 0.1.1 appeared with the same label.

### Prompt Context

**User prompts (verbatim):**

- “can you take screenshots to make things look good?”
- “Also why are there two download links? https://slop.boutique/apps/lattice-deck Maybe we only need the latest version”
- “take multiple screenshots per thing”
- “we can upload multiple right?”
- “can you use the token and the CLI?”
- “make them all public as well”
- “yes, create a token”

The last answer explicitly approves the one-day administrative token requested for the single global tour-setting change. **Assistant interpretation:** Publish attractive covers and galleries, expose all five through bounded anonymous tours/public HTML downloads and reduce the public download UI to the current HTML release. **Inferred user intent:** A minimal usable public catalog with good visual examples, while retaining operator history and hosting limits.

### What I did

- Added scripts/07-capture-showcase.js and sources/showcase.json. Captures use 1800×1125 workspaces and direct slide-element crops for clean presentation figures. Recorded PNG dimensions, SHA256, captions and corresponding published HTML hashes.
- Captured three Rift sections, three Phantasmagoria scenes, three Deck examples, four Tabula views/slides and three RAG views. Used actual app controls; no generated mockups or image retouching. Selected a cover plus the remaining images for each gallery.
- Found two published Deck releases, 0.1.0 and 0.1.1, rendered as repeated Download HTML/Manifest rows. Changed pkg/catalog/pages.go to offer only the current release's HTML, with current attachments. Studio history, manifests, historical file APIs and workspace pins remain available.
- Added TestArtifactPageLatestDownload: two releases, exactly one current HTML link for entitled users, no manifest link, no stale free-sample fallback to anonymous visitors, and historical files/manifests still downloadable through the authorized API.
- Targeted tests TestArtifactPageLatestDownload, TestStorefrontPages and TestAttachments pass. Full TEST_DATABASE_URL=local PostgreSQL go test ./... -count=1 passes with actual catalog integration tests enabled.
- Standard token file and FRANCOIS_TOKEN environment fallback were absent. Added a normal-login bootstrap for one-day API tokens, storing only the requested tokens in private temporary files with mode 0600. Session cookies and Vault values are never exported.
- Added the CLI-only gallery/publication workflow: media upload, artifact page-set/page-publish, metadata update, new immutable free-sample releases of unchanged HTML, and settings show/apply for the single approved demo.enabled change. Public access retains the existing duration/concurrency/start/budget limits.

### Why

Public users should see one current download, while release history remains operational data. A free-sample flag belongs to an immutable release, so public download access requires a new release of the same HTML bytes rather than editing the stored release. Cover and gallery images are page metadata, so they can publish immediately without rebuilding the application service.

### What worked

The apps already support rich visual examples: Deck's presenter mode gives clean graphics/chart frames; Tabula generates a dark slide deck from its live workbook; RAG exposes ranked sections and execution traces. Narrow and full PostgreSQL-backed tests pass for the download-page simplification. The CLI authenticates successfully using the Studio token and supports all required image/page/artifact operations.

### What didn't work

- A Deck preset selector initially assumed accessible-name spacing from textContent and timed out; selecting the actual preset-menu button text worked.
- Tabula Reset layout activates Grid; the capture script initially waited for take buttons while Stage was hidden. It now selects the Stage tab explicitly.
- Stage/Trace/Corpus are button elements with tab roles, so getByRole(button) did not match. The capture script uses their actual button/tab text selectors.
- Collapsing RAG's agent exposed an existing narrow-rail overflow in the export. Kept the complete agent panel for the final captures rather than masking content with screenshot-only CSS.
- The image relay rejected detail low; used the supported default image detail for visual review.
- CLI --output json was rejected with unknown flag. Installed command help specifies --format json; the publication script uses it.
- Automatic approval review rejected the initial admin-scoped token as broader than gallery publishing required. Used a Studio-only token first, explained the separate global setting, requested explicit approval for a one-day admin:read/admin:write token, and received “yes, create a token”. The approved administrative token is separate and used only for the existing tour-setting operation.

### What I learned

The gallery supports up to 24 media entries plus a cover. All published image bytes can be reused for the vault report, with captions and the source-release hash recorded. Public listing, anonymous running and public downloading are separate policies: visibility, artifact/global demo controls and immutable release free-sample access must all be handled deliberately.

### What was tricky to build

The display regression was caused by looping over release history, not duplicate media uploads. Selecting the current release in the page model fixes both duplicate HTML links and stale attachments without touching the underlying releases. The gallery publisher preserves each existing Markdown write-up and checks for an existing draft before modifying page metadata.

### What warrants a second pair of eyes

Review latest-only page rendering, access checks and the regression test. Confirm the CLI publication receipt keeps HTML hashes identical and changes only demo.enabled globally. The short-lived tokens are in /tmp/francois-009-private, excluded from Git and tool logs.

### What should be done in the future

Verify the public covers/galleries, anonymous downloads and a bounded tour after publication. Release the small download-page renderer fix through the existing image/GitOps workflow. Update the report evidence index with all final media IDs and live figures.

### Code review instructions

Start with pkg/catalog/pages.go and TestArtifactPageLatestDownload in pkg/catalog/storefront_test.go. Then inspect scripts/07–09, sources/showcase.json and the final gallery receipt. Do not run the publishing scripts as unit tests: they mutate the real catalog and require the scoped temporary tokens.

### Technical details

Current CLI structured output is --format json. The content token has only studio; the approved administrative token has admin:read/admin:write. Each expires after one day. Media upload is catalog media upload; page images use artifacts page-set --cover/--gallery then page-publish. Tour settings use an explicit base revision and preserve every field except demo.enabled. Final publication/deployment evidence is appended below after execution.


#### Public gallery/access checkpoint

All sixteen curated images were uploaded with the Francois CLI, then published as five covers plus eleven gallery entries. Five new immutable 0.1.2 releases expose public HTML downloads while preserving each generated HTML SHA256. All artifact metadata is listed/free-plan eligible/demo-enabled. Settings revision 2 changes only demo.enabled to true; 20-minute tours, three starts per user/address/hour, five credits, twenty concurrent tours and shared daily budget 200 are unchanged. Global model/storage settings are identical to revision 1.

`scripts/10-verify-public-galleries.mjs` uses a fresh unauthenticated browser context: all five public pages have the expected loaded image counts, a Try button and a successful anonymous HTML download whose SHA256 matches the original build inventory. A fresh anonymous RAG tour has five bundled guides, zero uploaded files from the administrator's workspace, and a real quick-tier GLM sampling reply of pong. Final public catalog/gallery/tour captures are retained under various/showcase.

The focused rendering fix is commit 5133753 on the import branch, cherry-picked as 83e7312 on the isolated fix/francois-latest-download worktree/branch. Source PR https://github.com/go-go-golems/francois/pull/8 changes only pages.go and its regression tests, so the service rollout is independent of the larger imported-source/screenshot archive. Full PostgreSQL-backed go test ./... -count=1 passes; GitHub tests, lint, dependency/secret scans and CodeQL have passed while the image build and GoSec finish. Production rollout evidence follows after CI.


## Step 5: Address publication-order review and match instance browser titles

Source PR #8 received an automated P1 review pointing out that Store.Releases orders by creation time while the catalog considers publication time current. Corrected the selection before rollout, and expanded the regression to publish drafts out of creation order. The user then requested matching instance document titles, which had remained the generic static portal title.

### Prompt Context

**User prompt (verbatim):** “The HTML TITLE on https://slop.boutique/instances/ins_9c5a9d0c6055a2bf3168 and probably other instance pages should match the relevant post page title, btw”

**Assistant interpretation:** Use the current corresponding artifact page title and site brand for the outer instance page's document.title. **Inferred user intent:** Browser tabs/history should identify the application consistently with its public page, including after artifact renames. **Commit (code):** 4fa1b57 on the import branch; cherry-picked as 0ac2190 onto the isolated source PR branch.

### What I did

- Replaced creation-order truncation with selection of the maximum PublishedAt release. The page card and single HTML download now use the same selected release, matching the catalog/upgrade definition of newer.
- Changed TestArtifactPageLatestDownload to create two drafts then publish the earlier-created one last. The older-published release is a free sample, so the test also proves an anonymous visitor cannot receive a stale free download while the current release is paid.
- Added artifactTitle to the authorized instance-detail response and optional Go/TypeScript client fields. It is read after membership authorization, from the artifact's current metadata; no instance names are exposed to anonymous readers.
- Launcher sets document.title to artifactTitle plus the existing session brand and restores the previous title on unmount. The instance's custom name and iframe label remain distinct.
- Added TestInstanceUsesCurrentArtifactTitle: create a workspace with a custom private title, rename its artifact, then assert detail returns both the unchanged custom title and the current public artifact title.
- pnpm check and pnpm build pass; the generated catalog.js is committed. The complete PostgreSQL-backed catalog/client integration suites pass after both changes. The earlier full Go suite passed before these targeted review/title changes; GitHub reruns its complete checks on the final head.

### Why

The published page title is artifact metadata, not a snapshot taken when a workspace was created. Returning it with the authorized detail avoids another frontend request and works for permitted pinned/hidden artifacts. The existing session already exposes the correct configured site brand, so the launcher uses the same suffix as the server-rendered page.

### What worked

The semantic regression covers artifact renaming while preserving private workspace naming. Publication-order coverage exercises the actual bug identified by review rather than asserting the implementation's loop shape. Generated assets and TypeScript types build successfully.

### What didn't work

The first download patch selected the first store result, which was creation-sorted. Review correctly identified its mismatch with publication-time semantics. The initial P1 finding was addressed before merging. The generic SPA HTML has no per-instance data, so document.title must be updated after the authorized detail/session load; no metadata is leaked from an unauthenticated server route.

### What I learned

An artifact's latest public release is a publication-time decision, not necessarily its highest version or newest uploaded draft. Application display names can change independently of saved-workspace names. Those distinctions belong in the test cases.

### What was tricky to build

The launcher must update only while it owns the page and restore the prior title when routing away, avoiding a stale app name on account/admin pages. It waits for both the current artifact title and site brand before composing the document title.

### What warrants a second pair of eyes

Review PublishedAt ordering and the authorized detail field, then verify live document titles against public artifact pages. Confirm no historical release data is deleted and both anonymous/free and entitled download policies remain correct.

### What should be done in the future

Wait for final CI, merge the isolated source PR and resulting image-only GitOps PR, then verify the rollout, single public download and instance titles in a real visitor session. Retain gallery/report evidence on the import branch.

### Code review instructions

Start with pkg/catalog/pages.go, pkg/catalog/instances.go, ui/catalog/instances.tsx and the two semantic integration tests. Compare the updated PR description with the final scope; frontend output is generated by pnpm build.

### Technical details

Source PR: https://github.com/go-go-golems/francois/pull/8. Final browser title format is artifact.Title + " · " + session.brand. No additional settings fields, migrations, queue or service were introduced. Current public HTML/media and tour policy were already verified independently of this service-code rollout.


#### Release and cover refinement checkpoint

All source checks passed on final head 0ac2190, including the image build, GoSec, CodeQL, tests and lint. The P1 review thread is resolved after the publication-time fix. PR #8 merged as 0440798cc622b214a74136f10344e058895eddd1; the normal main-branch image/GitOps release is in progress. gh pr edit encountered the installed client's deprecated Projects (classic) GraphQL field; a REST PATCH updated the PR title/description successfully. No required checks were bypassed.

The catalog's fixed-aspect thumbnail cropped the heading in Deck's full-slide cover. Added a direct canvas screenshot as its cover and retained the complete procedural slide in the gallery. There are now seventeen curated images: five covers and twelve gallery entries. The Studio-token CLI updated only Deck's images; all free-sample release HTML hashes and tour settings remain unchanged. The earlier sixteen-image checkpoint remains the historical first publication.

#### Production rollout and final verification checkpoint

The normal main-branch image workflow 38985147906 published ghcr.io/go-go-golems/francois:sha-0440798. GitOps PR https://github.com/wesen/2026-03-27--hetzner-k3s/pull/360 passed validation and merged as 083adeaccd3dc3247b7d5f7eef05151ebd5f154d. Its sole deployment change pins the image from sha-fda5382 to sha-0440798. The deployment reports one desired, updated, ready and available replica; Argo CD reports that GitOps revision Synced and Healthy. No settings or schema changes were made by this release.

Ran node scripts/11-verify-portal-rollout.mjs (from the ticket directory) against production in a fresh unauthenticated Chromium context; exit 0. All five pages return 200, each exposes exactly one Download HTML and no Manifest link, and all seventeen cover/gallery images load. New anonymous Deck and RAG tours have document titles exactly equal to their corresponding public page titles: Lattice Deck · slop.boutique and RAG Workbench · slop.boutique. Both retain the exact twenty-minute expiry, and Close restores a non-instance page title. These two checks make no model calls. Existing open instance tabs need a refresh to load the updated portal JavaScript; the user's existing tab was left untouched.

Evidence is various/portal-rollout-verification.json and various/portal-deployment-receipt.json. Refreshed the illustrated catalog and Deck-gallery captures, and added the two instance captures. The evidence index links the final receipts and figures for the later vault report. No new failure occurred during this checkpoint. Source implementation is deployed from PR #8; the separate import branch retains the reproducible source archive and report evidence. No requested implementation or deployment work remains for this ticket's follow-ups.

## Step 6: Measure slow requests and correct public asset caching

Investigated live headers and cold/warm Chromium loads after the user reported slowness. Public uploaded images were already cached correctly, but embedded assets lacked validators and compression. Added a bounded asset handler and content-versioned references, and published smaller screenshot delivery copies without changing the original report PNGs or application HTML.

### Prompt Context

**User prompt (verbatim):** “do we properly handle cache headers and stuff? I saw quite a bit of slowness”

**Assistant interpretation:** Measure live caching and transfer costs, explain the observed gaps and fix straightforward causes. **Inferred user intent:** Make opening and revisiting the minimal public site faster without adding infrastructure. **Commit (code):** c2aacec on the import branch; cherry-picked as 1ec0d89 onto fix/francois-asset-cache for a focused service PR.

### What I did

- Read portal/hosted routes, embedded asset allowlists, storefront media templates, fonts, analytics wrapping and the production ingress. The ingress has no compression middleware; live responses confirm no Content-Encoding for text assets.
- Added scripts/12-measure-cache.mjs: read-only fresh browser contexts for /apps, /apps/lattice-deck and /account, each cold then warm. Captures only an allowlist of response headers, paths, timing and byte counts, never cookie/grant values. No tours or model calls.
- curl --compressed of /static/catalog.js transferred 413644 bytes in 3.869698 seconds. No ETag, Last-Modified or Content-Encoding was present, and Cache-Control was no-cache. Chromium independently downloaded the entire bundle on both cold and warm account visits; see various/cache-before.json.
- Added pkg/catalog/web/assets.go: compress/hash the fixed embedded allowlists once at startup, serve selected gzip/identity bytes with distinct strong ETags and ServeContent, and use immutable year-long caching only when the URL's content hash matches. Bare or mismatched asset URLs require revalidation.
- Updated storefront templates, the SPA shell, launch shell and injected runtime script to use those content-versioned URLs. The launch document, grant-bearing /page response and private APIs retain uncached policies. No generic response wrapper touches SSE, model calls, media ranges or authenticated HTML.
- Added tests for content changes/URL changes, conditional responses and variants, compressed byte identity, HEAD, compression negotiation, allowlists and the launch-document/static-asset boundary. Full PostgreSQL-backed go test ./... -count=1 passes.
- Added scripts/13-optimize-web-media.py and sources/showcase-web.json. cwebp -q 85 creates delivery copies at the original dimensions: 10605879 bytes of PNG become 1983916 bytes of WebP, an 81.3% reduction across all seventeen images. Originals and their hashes are untouched. The CLI publisher's --plan option selects this alternate manifest and preserves the published Markdown and release HTML.
- Published all seventeen WebP copies with the existing Studio-only token and CLI. No administrative settings changes, new token, new application version or model calls were needed.

### Why

no-cache permits storage but requires revalidation; without a validator the existing handler returned a full 200 body each time. Content-versioned references make a long freshness lifetime safe and avoid a validation round trip on normal repeat visits. Precompression costs a small bounded amount of process memory and startup work, with no per-request compression and no additional service. Public media IDs already identify immutable bytes; smaller delivery copies address first-visit costs rather than changing a correct image cache policy.

### What worked

The live measurements distinguish a warm transfer defect from an initial media-size cost. The cold catalog fetched roughly 4.49 MB including a 3.21 MB Rift cover; warm media responses transferred zero bytes. The account page's JavaScript was still 413644 bytes on its warm request. All local integration tests pass with actual PostgreSQL. The publisher reused the current free-sample releases and successfully verified each cover/gallery selection.

### What didn't work

The first un-escalated full test command failed because the sandbox made the Go build cache read-only (open /home/manuel/.cache/go-build/...: read-only file system) and disallowed httptest/PostgreSQL sockets (socket: operation not permitted). Reran the identical TEST_DATABASE_URL=local go test ./... -count=1 command with the required sandbox permission; the entire suite passed. Early guessed source/ingress paths did not exist; rg --files located the actual templates and gitops/kustomize/francois/ingress.yaml.

### What I learned

The catalog's original screenshot optimization gap dominated initial image bytes; cache headers alone cannot reduce that first download. Instance origins each have a separate browser cache, so immutable runtime caching primarily helps reloads within the same instance. The portal bundle benefits across account/instance/studio routes on the shared portal origin. Headless-browser probes skip traffic tracking, so these timing samples are not an end-user latency guarantee.

### What was tricky to build

The hosted router sets no-store by default. Only fixed public static files override it. /page includes a bearer grant in its bootstrap JSON and must retain its uncached policy; compression here was deliberately scoped to non-secret embedded assets. Strong validators differ for compressed and uncompressed representations, and Vary: Accept-Encoding preserves cache variant selection. Updated asset bytes change the URL emitted by fresh HTML rather than waiting for cache expiry.

### What warrants a second pair of eyes

Review asset allowlists, hash query matching and the grant-bearing page boundary. Verify that first-load responses negotiate gzip, warm browser requests transfer no asset bodies and a new binary emits new asset URLs. Screenshot compression is lossy WebP at quality 85; use original PNGs for the technical report and inspect actual public rendering for text legibility.

### What should be done in the future

Complete service PR checks and production rollout, then repeat the same browser measurement and retain the final receipt. Remaining app-specific startup work includes the authenticated launch handshake, intentionally uncached application HTML and any third-party scripts retained by the imported export. This patch does not claim to eliminate all of those costs.

### Code review instructions

Start with pkg/catalog/web/assets.go and its tests, then the two static routes and the emitted URLs in web/templates/layout.tmpl, web.PortalIndex and hosted/routes.go. Run the full Go suite with local TEST_DATABASE_URL, and run scripts/12-measure-cache.mjs against the deployed site with an after label. Do not rerun publication scripts as tests; they mutate the live catalog.

### Technical details

The implementation uses standard net/http ServeContent and compress/gzip with no new dependency. Matching asset URL: /static/catalog.js?v=<SHA256> → public, max-age=31536000, immutable; unversioned: public, no-cache. ETag is the source hash for identity and source hash plus -gzip for gzip, with quoted syntax. Existing media URLs remain public, max-age=31536000, immutable with SHA256 ETags. The WebP publication receipt keeps the original media entries as history and points the current covers/galleries at the new media IDs.

## Step 7: Publish the final combined project report in go-go-parc

Wrote a new final report that synthesizes the completed imports and delivery-speed work. It preserves the earlier vault notes and explains the shipped system in one continuous technical narrative: source packaging, public access, application launch, evidence, cache correction and the production k3s revision.

### Prompt Context

**User prompt (verbatim):**

> write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).
> Commit and push the bsidian vault when done (go-go-parc vault).

**Assistant interpretation:** Produce a final standalone report covering FRANCOIS-009 and FRANCOIS-010, with preserved earlier notes and self-contained evidence, and commit/push the vault. **Inferred user intent:** A durable technical explanation of the now-shipped project. **Vault commit:** 5b0216c — Write final Francois HTML import and k3s delivery engineering report.

### What I did

- Read the vault/textbook-writing skills, the vault's ZK Tool exemplar, the import guide, packaging script, cache implementation/tests, public-policy and deployment receipts, and the evidence index. Both repositories were clean at the start.
- Added analysis/01-from-html-exports-to-public-applications-on-k3s-final-technical-project-report.md. The 6100-word vault version introduces artifact/release/instance/capability/origin concepts before the implementation and follows RAG through exact build identity, publication and launch.
- Covered all five imports, private-corpus exclusion, explicit script/resource substitutions, model concurrency, release/public-page semantics, title ownership, screenshot capture, WebP delivery, gzip/ETag policy, measured transfer/timing results, review failures, k3s rollout and remaining qualification limits. Added diagrams, representative code, pseudocode and file/API maps without analogies.
- Added scripts/15-publish-final-vault-report.py. It refuses to overwrite an existing note or asset directory, verifies original screenshot SHA256 values, copies seventeen PNGs/two performance figures/fifteen JSON receipts and records a source/hash inventory.
- Published Projects/2026/10/16/PROJ - Francois - From HTML Exports to Public Applications on k3s - A Technical Deep Dive.md with colocated _assets/francois-public-html. Five application figures and two performance plots are embedded; the full original seventeen-image set remains linked for reuse.
- Verified all 40 local vault links resolve and all 34 copied files match inventory sizes/hashes. Scanned copied JSON field names for credential-bearing keys; none found. Verified all five tracked release HTML hashes agree with their build and public-publication receipts.
- Staged only the new report and its evidence directory, checked the staged diff, committed as 5b0216c and pushed go-go-parc main. Application code, production settings and runtime data were not changed by this report-writing step.

### Why

The earlier speed report preserves its original design/review checkpoint and a later appended production result. A new final report can present the completed import and production-delivery work coherently while preserving that earlier history. Separating catalog publication from service-image deployment also explains why screenshots/releases can change without rebuilding the Go service.

### What worked

The new report contains its own figures and exact-byte receipts, so it remains readable and auditable outside the source checkout. It explains why a screenshot with six RAG files does not contradict the fresh tour's five bundled/zero uploaded files, why a scripted Deck action is distinct from a model completion, and why lower transfer is not a universal cold-latency claim.

### What didn't work

The initial apply_patch for the publisher script omitted the patch-line prefix on one literal YAML line and was rejected with invalid hunk at line 67. Corrected the patch and added the script successfully; the failed operation did not create a partial script. docmgr doctor has no new error; its existing warning is the intentional poc-note.md upload-fixture name without a numeric prefix.

### What I learned

The final report benefits from two explicit provenance identities: source archive snapshot 17645c6 and deployed service 393bf35. A complete source archive and a production runtime image have different contents and publication paths. The narrative can describe both precisely without implying that content publication alone deployed new service code.

### What was tricky to build

Keeping the report self-contained requires copying original figures rather than linking across repositories. Canonical ticket links remain repository-relative; the publisher rewrites those references to the colocated vault asset directory. The evidence table distinguishes the earlier sixteen-image anonymous-tour receipt from current seventeen-image media publication and byte verification.

### What warrants a second pair of eyes

Review the report's claims against the copied receipts, especially historical release versions, model-vs-scripted actions, runtime authorization, source/GitOps commits and mixed cold timings. The final rollout is described as the recorded 00:47 UTC observation, not a new capacity or startup benchmark.

### What should be done in the future

The requested report publication is complete. The existing implementation's conditional performance and qualification follow-ups remain in the report; no new product work is implied by this documentation step.

### Code review instructions

Read the new analysis and the vault report, inspect scripts/15 and various/final-project-report-assets.json, and verify the new note's local figure/evidence references. Existing source tests and live receipts are cited as prior validation; no application test rerun is needed for this report-only change.

### Technical details

The vault report is 6132 whitespace-delimited words, with 34 copied files totaling 10838972 bytes plus its inventory. It uses type: project metadata, tickets FRANCOIS-009/010, the source/archive and deployed service hashes, and the recorded production timestamp. No existing vault note, application release or screenshot original was overwritten.
