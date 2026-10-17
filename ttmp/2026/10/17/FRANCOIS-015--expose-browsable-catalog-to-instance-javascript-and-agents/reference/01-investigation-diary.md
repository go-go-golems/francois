---
Title: Investigation diary
Ticket: FRANCOIS-015
Status: active
Topics:
    - backend
    - frontend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T16:44:13.333225919-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigation diary

## Step 1: Map the actual instance, JavaScript and catalog boundaries

I traced catalog pages, source downloads, runtime grants, RPC dispatch and agent tool execution before selecting a design. The important correction was that this repository does not contain a server JavaScript interpreter: the representative RAG agent executes browser JavaScript, and hosted model calls forward application tools to the browser. A global API alone would therefore fail the request that every instance agent can discover site data.

The resulting design uses three read operations, one shared authorization/projection service and two callers: the hosted JavaScript facade and automatically attached server-owned model tools. It deliberately preserves the distinction between public editorial metadata and entitled downloadable source. No application implementation or production mutation was performed.

### Prompt Context
**User prompt (verbatim):**
> Also create another docmgr ticket, which is about exposing the db of the site itself (by that i mean the uploaded artifacts and blog posts and all that a user session can normally access through browsing itself) to the JS sandboxes (both agent and if the html has a JS functionality, that too), and expose it to any agent within an instance, maybe as slop_boutique.* JS API or something like that. That way, every coding agent can access the actual website functionality itself, and thus create "recursive apps".
>
> In a subagent: Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.

**Assistant interpretation:** Produce an implementation-ready read API design and upload it; avoid raw SQL or privileged data access.
**Inferred user intent:** Let HTML applications and coding agents use browsable site content to create applications based on existing experiments and writing.
**Commit (code):** No implementation commit; parent coordinates documentation commits in the shared checkout.

### What I did
- Read docmgr, diary, textbook-authoring and remarkable-upload skills, including the full investigation-entry format.
- Created FRANCOIS-015 with docmgr, reserving FRANCOIS-014 for the parent's downloaded-HTML animation work.
- Inspected pages.go, routes.go, entitlement.go, store release/content records, hosted principal/RPC/sample code, runtime.ts and the RAG agent's runEval.
- Wrote the intern guide with proposed DTOs, chunk semantics, permission rules, automatic model tools, implementation phases and a concrete validation matrix.
- Captured line-numbered primary evidence using the ticket's numbered script.

### Why
- Shared application policy prevents an instance API becoming a bypass for paid source downloads.
- A model callback can perform these reads directly without a new JS interpreter, queue, worker service or portal CORS integration.
- Explicit public projection avoids accidentally serializing creator-linked user IDs, uploader metadata, drafts, settings or billing state.

### What worked
- Repository inspection established actual end-to-end tool execution, including the 16-tool limit and browser eval scope.
- The same three service operations can serve both HTML callers and model-owned tools.
- Existing grant revocation, view ownership and entitlement code provide the needed authentication foundation.

### What didn't work
- `rg` over all pkg files included generated minified static JS and produced excessively large output. Restricted subsequent searches to Go and authored TypeScript files.
- `cat pkg/catalog/releases/manifest.go` failed with `No such file or directory`; releases are implemented in releases.go.
- `rg ... pkg/catalog/releases.go` failed because the download handlers are in routes.go, not a top-level releases.go.
- A shell glob `pkg/artifact/*.go` failed with `zsh:1: no matches found`; subsequent inventory used rg --files.
- `cat .docmgr.yaml` reported no file; docmgr reports the actual configuration as .ttmp.yaml. These were read-only discovery misses, with no mutations or lost work.

### What I learned
- The requested recursive capability is catalog/content access, not exposing instance-private document storage as the site database.
- A hosted page may declare no tools; automatic server tools are necessary for uniform model discovery.
- Published source visibility depends on release download rules, not simply whether the artifact description is public.

### What was tricky to build
- The package graph already imports hosted from catalog. The guide places a small shared read package at the dependency boundary instead of introducing an import cycle.
- Authorization must be refreshed between model rounds, not copied from the principal saved when a long-running model request started.
- Byte-based UTF-8 chunking and response byte limits must agree with existing model tool result limits. The guide fixes concrete bounds and continuation behavior.

### What warrants a second pair of eyes
- Review automatic tool injection against provider-specific tool support and the worker's existing validation order.
- Check that HTTP and RPC download decisions remain identical and no credentials appear in returned DTOs.
- Confirm staff browsing projection excludes studio previews intentionally, and custom isolated eval realms require explicit proxy injection.

### What should be done in the future
- Implement the feature after authentication work, in the staged commits listed in the guide.
- Obtain real database, browser and model round-trip evidence during implementation. This design is not a completed runtime feature.

### Code review instructions
- Start with design sections 4, 6 and 7, then compare sources/code-evidence.md against the named repository symbols.
- Review proposed files and acceptance matrix before coding; implement policy parity and revocation tests first.

### Technical details
- Three JavaScript methods: list, read and readFile; corresponding exact reserved RPC methods and underscore-separated provider tool names.
- Maximum 20 list entries, 24 KiB serialized result, 12 KiB source chunks, 16 total model tools including three server tools.
- No schema migration, no raw SQL endpoint, no automatic publishing and no binary archive extraction.

## Step 2: Validate the guide and deliver it to reMarkable

The completed guide contains approximately 3,907 words, two diagrams, proposed JavaScript/RPC/model tool contracts, explicit file references and a permission-focused implementation and test sequence. I related the six principal implementation files with docmgr and validated the ticket before delivery. The guide was frozen during the upload.

The uploader completed successfully and returned a specific cloud destination. This confirms delivery to reMarkable cloud; it does not prove the physical device has synchronized. Runtime implementation and application tests remain pending because this assignment was explicitly documentation only.

### Prompt Context
**User prompt:** See Step 1.
**Assistant interpretation:** Validate and upload the requested complete implementation guide.
**Inferred user intent:** Make the guide available for implementation review and reading on reMarkable.
**Commit (code):** None assigned to this subagent; parent stages and commits the ticket.

### What I did
- Ran `docmgr doctor --ticket FRANCOIS-015 --stale-after 30`: all checks passed.
- Ran `remarquee upload bundle` with the guide, document name `FRANCOIS 015 Recursive Catalog Intern Guide`, remote directory `/ai/2026/10/17/FRANCOIS-015`, `--toc-depth 2`, and `--non-interactive`.
- Preserved the upload output in `sources/remarkable-upload.txt` and checked the exit status was zero.

### Why
- Ticket validation checks document metadata and relations before handing the work back.
- The remarkable-upload skill requires the normal upload receipt as completion evidence, without unnecessary extra renders or overwrite operations.

### What worked
- `OK: uploaded FRANCOIS 015 Recursive Catalog Intern Guide.pdf -> /ai/2026/10/17/FRANCOIS-015`.
- No reauthentication, overwrite, retry or upload failure occurred.

### What didn't work
- No failures occurred in this validation and delivery step.

### What I learned
- The existing uploader handled the guide's code blocks and Mermaid source without a reported conversion error. No independent visual rendering inspection was requested or performed.

### What was tricky to build
- Keeping implementation claims separate from design claims: the ticket remains active, and only research/documentation/delivery tasks are checked.

### What warrants a second pair of eyes
- The model-tool injection contract and shared download policy extraction are the most important implementation review boundaries.

### What should be done in the future
- Implement and verify the remaining application tasks after the login feature, following the guide's commit sequence.

### Code review instructions
- Review the intern guide, saved source excerpts and delivery receipt. No application code was changed by this ticket investigation.

### Technical details
- Remote document: `/ai/2026/10/17/FRANCOIS-015/FRANCOIS 015 Recursive Catalog Intern Guide.pdf`.
- Source guide: `design-doc/01-recursive-catalog-api-intern-implementation-guide.md`.
- Primary source capture: `scripts/01-capture-code-evidence.py` and `sources/code-evidence.md`.

## Step 3: Render the diagrams and deliver an illustrated guide

Parent review identified that Mermaid code blocks were not sufficient evidence of diagrams in the delivered PDF. I replaced the inline blocks with actual PNG image references and retained both their original Mermaid source and editable Graphviz DOT source. Both rendered PNGs were visually inspected and have legible labels and unclipped connections. The first delivery's successful exit had established cloud delivery only; it had not established diagram rendering.

The illustrated guide was uploaded successfully under a distinct name, preserving the original document and any annotations. A subsequent parent request added the precise source contract shared with FRANCOIS-014: readFile returns original immutable published source, not the proposed branded derived download. That small note is present locally. Parent then instructed stopping further upload churn, so the Source Edition was rendered locally but was not uploaded; the actual final cloud delivery is the Illustrated document.

### Prompt Context
**User prompt:** See Step 1. Parent review requested actual embedded diagrams and the original-source clarification.
**Assistant interpretation:** Complete the diagram requirement and keep delivered versus local revisions explicit.
**Inferred user intent:** Provide an illustrated technical guide suitable for reMarkable reading without replacing annotated documents.
**Commit (code):** Parent coordinates the documentation commit; no staging or application edits by this subagent.

### What I did
- Added two DOT diagrams and original Mermaid source files under sources/.
- Added scripts/02-render-diagrams.sh and generated two PNGs with installed dot.
- Replaced Mermaid blocks in the guide with relative PNG references.
- Inspected both PNGs and uploaded `FRANCOIS 015 Recursive Catalog Intern Guide Illustrated` without force or overwrite.
- Added the later original-source contract note locally, rendered a local Source Edition with the uploader's PDF-only mode, and verified embedded image dimensions using pdfimages.
- Inspected the local final PDF's diagram pages 4 and 8: both diagrams are readable and unclipped. Parent's stop instruction arrived after these checks; no further upload was started.

### Why
- An upload exit code does not establish that diagram source became rendered graphics.
- Read APIs must preserve original blob hashes and avoid recursive accumulation of watermark wrappers.
- A distinct illustrated title avoids deleting annotations from earlier cloud documents.

### What worked
- `OK: uploaded FRANCOIS 015 Recursive Catalog Intern Guide Illustrated.pdf -> /ai/2026/10/17/FRANCOIS-015`.
- Local PDF image inventory shows actual RGB images on pages 4 and 8 at 885x1118 and 996x1038, respectively; both were visually checked.

### What didn't work
- Trying pdfimages on the uploader's temporary PDF after upload failed because its temporary directory had already been removed: `I/O Error: Couldn't open file ... No such file or directory.` The PDF-only output supplied a persistent local artifact for the requested validation.
- Broad rg inventory of /tmp encountered unrelated system-private permission errors. Searches were narrowed to the known uploader output directory.

### What I learned
- Distinguish editable diagram source, rendered PNG, verified local PDF content and successful cloud delivery as separate evidence.

### What was tricky to build
- Two parent follow-ups arrived during delivery. The diary and index explicitly distinguish the uploaded Illustrated version from the locally amended Source Edition rather than claiming the latest small text amendment is already on the device.

### What warrants a second pair of eyes
- No new implementation decisions; review the original-source paragraph alongside the FRANCOIS-014 download transformation design.

### What should be done in the future
- No further delivery work is pending under the parent's stop instruction. Implement the open runtime tasks when scheduled.

### Code review instructions
- Review the two PNGs and DOT sources, the guide's relative image links, remarkable-upload-illustrated.txt and pdf-image-verification.txt.

### Technical details
- Actual latest cloud delivery: `/ai/2026/10/17/FRANCOIS-015/FRANCOIS 015 Recursive Catalog Intern Guide Illustrated.pdf`.
- Local amended PDF: `/tmp/francois-015-final-pdf/FRANCOIS 015 Recursive Catalog Intern Guide Illustrated Source Edition.pdf`; not uploaded.
- Both original and illustrated cloud documents were left intact.
