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
