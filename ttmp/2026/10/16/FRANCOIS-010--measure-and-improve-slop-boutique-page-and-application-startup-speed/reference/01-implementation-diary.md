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

## Step 2: Deploy the focused change and publish measured outcomes

Completed the normal image/GitOps rollout and repeated the same fresh-context browser measurement. The verified improvement is a substantial transfer reduction and correct reuse of public code, with protected responses retaining their uncached policy. Added the live outcomes to the ticket report and vault as an append-only checkpoint, including cold timing results that did not improve.

### Prompt Context

**User prompt:** See Step 1 for the verbatim ticket, vault and implementation request. **Assistant interpretation:** Finish shipping and retain evidence that distinguishes implemented behavior from measured performance. **Inferred user intent:** A completed deploy and a technically accurate report. **Commits:** 393bf35 source merge; 4eda420 report/media/diary checkpoint; vault e08db6e initial report and 49b0d45 verified outcome.

### What I did

- Confirmed workflow 38987577959 published ghcr.io/go-go-golems/francois:sha-393bf35 successfully. Reviewed GitOps PR #361: its sole diff changes the image from sha-0440798 to sha-393bf35. Validation passed in 15 seconds.
- Merged GitOps PR #361 with exact-head guard 44e096b1fcc4655a729ba5a890f3362f6b74557e. The merge is e3ccf7b09784bb6fadef6dd847216badee7e2f2c. Requested the normal Argo refresh and observed Synced/Healthy/Succeeded, deployment generation 13, and one updated/ready/available replica. kubectl rollout status reports success.
- Ran scripts/01-measure-cache.mjs https://slop.boutique after. All three cold/warm route pairs completed. Wrote various/cache-after.json using the same instrumentation as the baseline.
- Ran scripts/05-verify-cache-contract.mjs successfully. Verified immutable headers and ETags for emitted static references; gzip on cold JavaScript; zero static wire bytes on warm visits; exact gzip-decoded bundle identity; distinct representation validators; conditional 304; HEAD; unversioned revalidation; session no-store; and eager/high-priority cover markup.
- Extended that verifier to a known pre-existing workspace origin, without creating a tour or requesting a grant. Its /launch remains no-store and emits a versioned script; runtime.js is immutable/gzipped; unauthenticated /page returns 401/no-store. These live checks and the earlier integration suite cover the intended cache boundary.
- Generated the after/before transfer plot from actual receipts. Appended production-outcome.txt to the ticket/vault report, copied four new self-contained evidence assets and committed the vault addition as 49b0d45. The initial report and its evidence remain intact.

### Why

A lower compressed file size alone does not demonstrate warm-cache behavior. The browser check proves reuse under ordinary navigation, while explicit HTTP checks cover revalidation and protected routes. Publishing raw timing and byte tables prevents a lower byte requirement from being misrepresented as an across-the-board elapsed-time guarantee.

### What worked

The portal bundle's identical 413644 source bytes transfer as 124245 gzip bytes, approximately 70% smaller. Warm static response transfer is zero on every measured route. Overall warm /account response wire bytes fall from 451089 to 1048; cold catalog response wire bytes fall from 4493872 to 967742. All seventeen optimized image copies were already verified live. The report's tables and receipts preserve exact values and units.

### What didn't work

No rollout or cache-contract assertion failed. Cold timing samples were mixed: catalog load 1141→1266 ms, Deck detail 1221→1131 ms, account 1490→1933 ms. Warm samples improved to 167/147/140 ms from 251/262/627 ms respectively. These are single samples, so no latency percentile or end-to-end startup improvement is claimed. The earlier public tour probe remains rate-limited; no limit was changed or additional anonymous launch attempted for final verification.

Final documentation validation initially reported frontmatter delimiters missing from the generated production-outcome.md excerpt, and git diff --check found a new blank line at EOF in the report. Retained the generated append fragment as production-outcome.txt rather than registering a duplicate analysis document, and normalized the source report's final newline. Ticket close's completed recovery journal is archived to /tmp after verifying its committed state; index/changelog remain the durable completion record.

The first journal archival guard looked for a top-level state and stopped with AssertionError without moving anything. Inspected the journal keys, verified receipt.state is committed and then archived it. Final docmgr doctor and git diff --check pass; every copied vault asset link resolves.

### What I learned

The expected transfer improvement is deterministic for these bytes, but elapsed time also contains network setup, scheduling and execution. The strongest completion evidence combines byte identity, browser reuse, explicit HTTP semantics and rollout identity. Existing workspace origins allow a read-only check of public runtime assets and unauthorized-page policy without consuming a public tour start.

### What was tricky to build

The production change has two independent paths: image publication/GitOps for embedded code and catalog metadata publication for delivery images. The receipts tie both to their actual content identities. The final vault update adds a dated result rather than replacing the initial draft's review/deployment boundary.

### What warrants a second pair of eyes

Inspect various/cache-contract-verification.json and production-deployment.json together, then compare the raw before/after receipts with the report tables. Ensure a future startup benchmark uses a permitted existing workspace or a naturally available tour allowance and does not equate canvas presence with full application qualification.

### What should be done in the future

The accepted shipping scope is complete. Smaller card thumbnails, route-specific bundles and selected dependency packaging remain measurement-driven follow-ups documented in report section 10. Full hosted application startup latency is still unmeasured; this is an explicit evidence limitation, not an undeployed part of the cache change.

### Code review instructions

Review source PR #9 and GitOps PR #361 for the production change. The complete local PostgreSQL-backed suite passed before release, the affected suites passed after the final edit, every required source/GitOps check passed, and the final browser/HTTP assertions passed live. Documentation uses Matplotlib only through an isolated temporary uv environment; the service acquired no new dependency.

### Technical details

Authoritative final receipts: various/cache-after.json, various/cache-contract-verification.json, various/production-deployment.json and various/production-outcome.txt. The final report includes native Mermaid diagrams, source/API references, both performance figures and copied JSON evidence. Vault path: Projects/2026/10/16/PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive.md. No private cookie, runtime grant, token value, model prompt or user payload is stored in the evidence.
