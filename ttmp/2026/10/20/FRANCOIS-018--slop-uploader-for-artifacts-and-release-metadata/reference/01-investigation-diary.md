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
