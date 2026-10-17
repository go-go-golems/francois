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
