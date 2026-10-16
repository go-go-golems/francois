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
