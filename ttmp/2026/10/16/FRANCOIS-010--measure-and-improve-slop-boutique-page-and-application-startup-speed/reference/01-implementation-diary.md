---
Title: Implementation diary
Ticket: FRANCOIS-010
Status: active
Topics:
    - frontend
    - performance
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T20:36:24.634707905-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation diary

## Step 1: Backfill the measured baseline and establish the dedicated speed ticket

This ticket continues the performance investigation begun in FRANCOIS-009 after the public import rollout. That ticket's diary Step 6 retains the original cache question and early implementation chronology. This backfill establishes the current facts, the exact new request and the new report/evidence location without rewriting the earlier history.

### Prompt Context

**User prompts (verbatim):**

> what's the portal JS?

> any other speed improvements?

> Create a new docmgr ticket to optimize speed of this page, and  write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).
> Commit and push the bsidian vault when done (go-go-parc vault).
>
> The document is detailed, your improvements are striaght on effective and shipping oritended.
>
> Then implement, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)

**Assistant interpretation:** Create a dedicated ticket and a durable, intern-readable technical report in go-go-parc, then finish the measured changes through review/deployment while preserving a detailed implementation diary. **Inferred user intent:** Substantial explanation with small effective code changes and completed delivery. **Existing code commits:** c2aacec and 57edd23 on the import branch; cherry-picked as 1ec0d89 and b52d97b onto the isolated fix/francois-asset-cache branch for PR #9.

### What I did

- Created FRANCOIS-010 with docmgr, added its analysis report and this diary, and added explicit measurement, report/vault, implementation and deployment tasks.
- Used the diary, docmgr, obsidian-vault-writing and textbook-authoring instructions. Read the vault's ZK Tool example and the earlier Francois analytics note. The vault was clean before changes. Selected a new dated project note rather than editing historical reports.
- Moved the not-yet-committed cache-measurement script and initial receipt from FRANCOIS-009 to this ticket as scripts/01-measure-cache.mjs and various/cache-before.json. Import media manifests, publication scripts and image originals remain with their owning import ticket.
- Wrote analysis/01-page-speed-engineering-measured-costs-cache-design-and-implementation-guide.md. Its conceptual order is page kinds and origins, measured quantities, cache semantics, asset identity, emitted references, media delivery, protected launch, implementation gates and conditional follow-ups. It includes prose, code, pseudocode, diagrams, route/file/API references and explicit evidence limits, with no analogies.
- Recorded separate baseline costs: portal bundle 413644 bytes with no validator or compression; original Rift cover 3213622 bytes, correctly immutable but expensive cold; repeated warm media bodies zero bytes. Headless samples are identified as excluding analytics recording.
- The existing implementation prepares a fixed number of embedded assets at startup, sets distinct gzip/identity ETags, emits hash-versioned URLs and preserves private/authorized response policies. Added eager/high-priority main cover loading. The source work contains no extra service, queue, migration or dependency.
- Used the Studio-token CLI to publish seventeen quality-85 WebP copies. Exact total bytes change from 10605879 to 1983916; PNG originals and application release hashes remain intact. scripts/14-verify-web-media.mjs passed public page/image/caption/hash/cache/304 checks for all five applications.

### Why

The large bundle's repeat transfer and the cold cover transfer are independently observed problems. An isolated public-asset handler and immutable media replacement address them without touching authorization, model execution or state scheduling. The dedicated ticket separates the new speed work from the original application-import milestone; the report explains those boundaries for a new contributor.

### What worked

Full PostgreSQL-backed go test ./... -count=1 passed after the main cache change. After the final cover/injection edit, the actual database-backed catalog suite and hosted/web suites passed. The focused source PR is https://github.com/go-go-golems/francois/pull/9. Initial automated checks passed tests, lint, frontend and secret/dependency/vulnerability scans; the later revised head is checked again before merge.

### What didn't work

- The first full suite failed inside the sandbox because the Go cache was read-only and local sockets were prohibited. Rerunning the same command with the required permission passed. Exact relevant diagnostics are preserved in the preceding FRANCOIS-009 Step 6.
- CodeQL flagged hosted/routes.go line 189 with “If this JSON value contains a double quote, it could break out of the enclosing quotes.” Separated JSON-byte and script-markup assembly, escaped the generated URL for its attribute and corrected the explicit less-than replacement to the intended literal Unicode escape. This is revision 57edd23/b52d97b; no scanner finding was suppressed.
- The initial scripts/02-measure-startup.mjs before attempt timed out at page.waitForURL after 30000ms. It did not preserve a failure receipt. Added response-status handling and a finally-written diagnostic receipt; one diagnostic attempt returned HTTP 429, rate_limited: “Too many demos from your network; try again later”. Retained various/startup-before.json. No limit was relaxed and no further baseline tour attempts were made.
- Python's default environment has no matplotlib (ModuleNotFoundError). Charts, if added, must use an isolated plotting dependency rather than changing the repository's application dependencies; tables and Mermaid already preserve the essential comparisons.

### What I learned

The portal JS question reveals a necessary explanation boundary: public posts use a 3.3 KB action script, while account/instance routes load the React portal. The imported application has another dependency set. A first-byte timing, load event and visual application-ready condition are different measurements. A declined tour start supplies no valid startup latency sample.

### What was tricky to build

The hosted default no-store must remain on /launch and /page while public static files override it. Origin isolation means an instance's runtime cache does not automatically apply to another hostname. The source emits new references in four separate HTML-producing paths; updating only portal routes would leave some assets on their old cache behavior.

### What warrants a second pair of eyes

Review the allowlists, selected representation validators and hash-query freshness condition. Confirm that no protected release bytes or grant-bearing HTML enter the public cache handler. Read the report's measurement limits before interpreting any elapsed-time change. Compare captions and image readability after lossy delivery encoding.

### What should be done in the future

Copy the report/evidence into a new go-go-parc note, commit and push only those intended vault files. Finish source checks and the existing image/GitOps release path, repeat the identical cache measurement and append actual final evidence. Smaller thumbnails and route splitting remain conditional follow-ups rather than silently expanding the shipping scope.

### Code review instructions

Start at pkg/catalog/web/assets.go and assets_test.go, then the portal/hosted routes, shared template and cache boundary test. The report's file map explains each caller. Run TEST_DATABASE_URL=the-existing-local-connection go test ./... -count=1 with permission for local sockets, then scripts/01-measure-cache.mjs after the rollout. Publishing scripts mutate the live catalog and must not be run as tests.

### Technical details

Source baseline for the focused worktree is 0440798 from PR #8. The active PR's revised head is b52d97b. The deployment before this speed change is ghcr.io/go-go-golems/francois:sha-0440798. The WebP assets are already live using immutable media IDs; final source deployment and browser receipts are recorded by the next checkpoint. No new administrative token or settings change was required for this ticket.

#### Report publication and source merge checkpoint

The new vault report and six colocated evidence assets were committed as e08db6e and pushed to go-go-parc main. The note is Projects/2026/10/16/PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive.md. It contains the complete ticket report, preserved source references, native Mermaid diagrams, actual catalog screenshot, a Matplotlib byte-comparison figure and self-contained measurement JSON. Matplotlib ran through uv's isolated /tmp cache; no repository runtime dependency changed. The publisher refuses to overwrite an existing note or evidence asset.

All final source-head checks passed, including CodeQL, GoSec, tests, lint, frontend, image build and dependency/secret checks. PR #9 merged with an exact-head guard on b52d97ba14ee7a182d5935a3b1ad5c665b6123d7, producing 393bf35d51ab467081d7a09460937df66ba446b9. The main image workflow is 38987577959. The production release remains pending; final evidence will be appended after its image-only GitOps change and live verification. The report explicitly marks its initial publication boundary so later results can be added without rewriting historical prose.
