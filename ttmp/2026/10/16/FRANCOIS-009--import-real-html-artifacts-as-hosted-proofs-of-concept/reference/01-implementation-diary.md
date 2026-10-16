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
