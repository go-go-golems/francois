---
Title: Investigation diary
Ticket: FRANCOIS-017
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-18T21:15:21.562550932-04:00
WhatFor: ""
WhenToUse: ""
---

# Investigation diary

## Step 1: Trace customer policy and identify why administration is unpredictable

Studied the hosted request path from identity and payment standing through workspace authorization, concurrency admission, reservation, provider rounds and settlement. Compared those rules with the public account and admin controls. The central finding is a mismatch: a generic multi-plan business model is exposed to administrators, while actual token measurements do not enforce the requested customer ceilings.

### Prompt Context
**User prompt (verbatim):** “Now study the entire project, under the guise of having only free or paynig customers, and wanting to have a fairly simple system of managing token ceilings . What do you think is overengineered? Study well”
**Additional user prompt (verbatim):** “The user management UI is quite complex / hard to understand, as an admin, for example. I don't know how things will behave.”
**Assistant interpretation:** Perform a broad source-backed assessment, make effective admin behavior central, and propose a small concrete model. Implement only the separately authorized PR review corrections.
**Inferred user intent:** Keep the minimal public site and six-lane service manageable with Free/Paying access and understandable internal controls.

### What I did
- Inventoried catalog, local host/store, shared browser runtime, provider worker, payment projection, publishing, CLI/MCP and admin interfaces using `rg --files` and focused source reads.
- Traced `hosted.sample` through model selection, caller budget, in-memory lane admission, atomic credit reservation, tool/provider rounds and settlement.
- Inspected current role/plan/source/payment/grant semantics and caller-versus-owner workspace decisions.
- Read the admin detail/settings UI and its API aggregation. Identified ignored core query errors and mismatches between raw subscriptions and year funding.
- Wrote the assessment with current-source examples, diagrams, implementation sketches, routes, a concrete admin layout and a three-part implementation order.
- Investigated and corrected PR 16 separately in ticket 012, Step 12; this ticket does not claim those corrections implement the broad proposal.

### Why
- A useful simplicity review must distinguish unnecessary business concepts from necessary tenant isolation, payment correctness and concurrent accounting.
- The administrator should see the effective answer and reason rather than manually combining role, ranked plans, credits and payment projections.

### What worked
- Source inspection showed actual charge authority: model.Credits at reserve/settle, with token usage written as measurements.
- The worker has a per-round output cap of 8,192 and up to ten rounds, rather than a per-customer daily token ceiling.
- The current single-process concurrency gate, database transaction boundary and durable Stripe projection can be retained; no new scheduling infrastructure is needed.

### What didn't work
- The first attempted nondeprecated code-quality skill path was absent. Located and read the catalogued deprecated skill path instead.
- PR correction test/fixture/lint failures are recorded where they occurred in ticket 012, Step 12. No production traffic or load experiment was performed for this study.

### What I learned
- Account role Subscriber is not paid status. Paid() includes staff and complimentary access. Neither label directly describes a charge or a token ceiling.
- A workspace is persistent state and membership rather than a per-instance pod. Active views, storage and admitted calls consume different resources.
- A strict token cap requires a specified unit and bounded in-flight reservation; post-call usage charts alone cannot enforce it.

### What was tricky to build
- Avoid replacing a confusing credit abstraction with an equally ambiguous token abstraction. The report recommends a hard generated-token daily cap and separately records total provider categories, explicitly distinguishing that from a hard all-token ceiling.
- Keep customer eligibility distinct from transient global capacity and instance-owner restrictions; one static allowed boolean cannot predict every later request.

### What warrants a second pair of eyes
- The recommended output-token unit is a design choice requiring product agreement before broad implementation. Existing input/cache/reasoning semantics must remain explicit.
- Removing arbitrary plans requires a direct schema/settings/artifact migration and matching CLI/MCP changes, not a UI-only removal.

### What should be done in the future
- Review the proposal, then implement the customer summary/admin page, token accounting and fixed two-class policy as focused changes. No broad refactor is claimed complete.

### Code review instructions
- Start with the assessment conclusion and findings 2.3–2.5, then inspect hosted/sample.go, store/usage.go, worker/src/main.ts and adminUser/UserDetail.
- Check the preserved-boundaries table before proposing removal of payment recovery, scoped runtime grants or accounting transactions.

### Technical details
- Starting source revision: d847ba4 on feat/year-funding-single-subscriber.
- This assessment is local source analysis. Example quota values are illustrative. No production deployment, provider benchmark or payment was made.

## Step 2: Publish the completed assessment separately from the review correction

The analysis and diary are complete and docmgr validation passes. Kept the broad design proposal separate from implementation commit `0746b36`, which fixes the two concrete PR 16 findings and passes catalog/frontend/lint checks.

- **Prompt Context:** See Step 1; PR review requests are recorded in ticket 012, Step 12.
- **What changed:** Added the 3,400-word assessment, focused source relations, completed investigation task and an explicit pending design-review task.
- **Validation:** `docmgr doctor --ticket FRANCOIS-017` passed; `git diff --check` passed. The proposal is not an implemented policy migration.
- **Review:** Start with the conclusion, token-unit recommendation and admin summary example. No production deployment is included.

## Step 3: Address the new payment review and fix the implementation scope

The new review identifies repeat year funding during existing standing and outdated support copy. Added an admission-time check under the existing per-user checkout transaction and clarified the purchase description. Wrote a short design for two fixed classes, generated-token ceilings and an admin summary before changing that behavior.

### Prompt Context
**User prompt (verbatim, JSON-encoded to preserve whitespace):** "let's address the new code review comments, and then go through the simplication (write a short design doc), then implement  write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).      \n Commit and push the bsidian vault when done (go-go-parc vault)."
**Correction (verbatim):** “sorry, no project reort, but commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)”
**Assistant interpretation:** Implement the review fixes and the simplification with focused commits and a detailed diary. Do not write a vault report.
**Inferred user intent:** Ship an understandable Free/Paying administration and actual token ceilings without expanding infrastructure.

### What I did
- Read PR 16's latest comments: 4309640585 (year overlap) and 4309640592 (support promise).
- Added `ErrYearFunded` and checked current year standing while admitting checkout under a user-row transaction lock. Exposed a clear HTTP 409 error.
- Changed support copy to describe additional saved workspaces and model allowance.
- Authored the short design in this ticket before implementing policy changes.

### Why
- Rejecting overlap is simpler than stacking payments and preserves the existing expiry semantics. Monthly subscribers may still fund their first year.

### What worked
- `TestYearFundingConfirmationAndRefund` passed against isolated PostgreSQL, including the new overlap rejection and existing monthly-customer path.
- Prior PR head 1029788 has green CI; GitOps opening is skipped on pull requests as expected.

### What didn't work
- No new validation failure at this checkpoint.

### What I learned
- Checkout admission is the correct place to reject repeat confirmed year standing while preserving idempotent unpaid attempts.

### What was tricky to build
- Keep ordinary checkout retry behavior while adding the year-only guard; do not accidentally reject monthly subscribers' first year contribution.

### What warrants a second pair of eyes
- Admission and year projection remain separate transactions. Existing admitted attempts prevent a second open checkout; confirmed standing prevents a new one.

### What should be done in the future
- Implement the short design, then run the integration gate. No vault write or deployment.

### Code review instructions
- Review store/billing.go AdmitCheckout, routes.go billingFailure and the year regression test, then the short design.

### Technical details
- Source comments are tracked by GitHub numeric review IDs. This checkpoint does not claim broad simplification is complete.

## Step 4: Implement fixed classes, atomic daily tokens and early-access dates

Implemented the requested simplification across the backend, worker and admin interface. The customer page now explains effective access before exposing administrative details. Replaced per-call credit charging with a bounded generated-token budget for each logical run and a daily PostgreSQL reservation ledger.

The user then specified that payment grants early access as well as higher limits. Implemented a concrete artifact end date, applied once on first publication and enforced consistently for workspaces, downloads, attachments and demos. Later releases cannot restart the window. Existing published artifacts stay available to Free accounts.

### Prompt Context
**User request:** See Step 3 for the implementation request and cancellation of the vault report.
**Additional prompts (verbatim):** “there is no need for backwards compatibility btw”; “no need to be overly cautious while working, we care about the final result, not be perfect at each step”; “free accounts can download all normally published artifacts, but we can set a period for new artifacts to only be downloadable / runnable to be only used by paying subscribers”; “like, you pay for early access, and higher limits”.
**Assistant interpretation:** Ship the simple policy, directly update current callers/schema, preserve evidence and run checks at the integration boundary.
**Inferred user intent:** Understandable operations for a minimal site: early access and higher limits for Paying customers, ordinary access for Free customers, no product-catalog machinery.
**Review correction commit:** 8f52e02; diary whitespace correction 7098eed. These were pushed to PR 16 separately from this implementation branch.

### What I did
- Replaced arbitrary plans with exactly Free and Paying resource entries. Removed rank, visibility/marketing fields, plan model lists, named-plan grants, per-model credit pricing and the retired artifact required-plan field from active contracts.
- Added daily generated-token reservations, absolute per-user ceiling overrides/reset, UTC resets and conservative recovery. Historical credits and provider input/output/cache measurements remain historical data, not converted quotas.
- Passed a server-only call-wide output budget to the worker. Every provider round consumes it; missing/invalid usage fails closed. The fake backend also respects the admitted budget.
- Added a backend customer summary with class/source/expiry, enabled state, AI eligibility/reason, daily usage/reservation/ceiling and workspace limits. Missing core data produces an explicit error. Missing billing-customer records are normal for Free users.
- Kept disabled state separate from payment standing in admin presentation: a disabled paying customer remains visibly Paying but is blocked from AI. Runtime authorization still checks the original disabled user.
- Rebuilt Users/detail/settings UI around those facts. Moved payment contracts, identity, staff permissions and session controls under Details. Added direct ceiling preview/reset and complimentary Paying access, with independent form state.
- Added default early-access days and editable per-artifact end date to Studio/CLI/MCP. Locked first publication, preserved the date across versions and checked the same window for running/download/demo paths, including free-sample releases.
- Removed automatic oldest-N workspace freezing on downgrade. Creation quotas remain; existing saved workspaces do not get frozen solely by creation order.
- Added concurrency/settlement/migration/admin/early-access tests and the real worker output-budget unit tests. Captured desktop/mobile browser fixtures and exercised primary customer controls.

### Why
- Two resource classes and one named daily accounting unit directly express the product. A concrete early-access date expresses the only artifact-specific commercial rule.
- Transactions and recovery remain necessary for concurrent limits; no queue, service, distributed lock or new scheduling layer was added.
- A provider error or cancellation does not prove zero usage. Conservatively charging the reserved budget keeps repeated uncertain calls from bypassing the limit.

### What worked
- Full database-backed `go test ./... -count=1` exercised the implementation; two obsolete expectations remained (removed CLI --plan flag and time-zone spelling). After correction, targeted CLI/catalog integration passed, including the real client contract and all changed policy paths.
- New tests passed for simultaneous admission, measured settlement/replay, a smaller final run, override/reset, crash recovery and UTC day boundaries.
- Early-access tests passed for paid/free running and downloads, demo rejection, free-sample enforcement, later releases preserving the clock, expiry and explicit clearing.
- `pnpm check`, twelve frontend/worker tests, embedded frontend build and Go build passed. Final lint/logcopter checks passed with zero issues.
- Browser fixtures passed at 1440px and 390px with no horizontal overflow or page errors; set/reset ceiling, disable/enable and complimentary grant all updated the actual bundled UI. These are explicitly mocked API fixtures, not production screenshots.

### What didn't work
- Initial Go check could not write the sandboxed Go cache (`read-only file system`); reran required checks with cache access.
- Migration 017 initially failed with `operator is not unique: unknown - unknown (SQLSTATE 42725)` because JSON extraction/subtraction needed parentheses. Corrected the expression and reran the database gate.
- New migration fixture initially used two parameterized statements in one pgx call: `cannot insert multiple commands into a prepared statement (SQLSTATE 42601)`. Split deletion and parameterized insertion.
- Strict client decoding rejected the added `customerClass` field until the Go client contract was updated. The newly strict admin aggregate treated an absent Stripe customer as an error; made only ErrNotFound normal while retaining real failures.
- Old tests expected plan-restricted Free downloads and named grants. Updated them to the requested two-class/time-window policy. A Studio test compared time-zone spellings; normalized stored early-access dates to UTC.
- Lint found `SA4006: this value of owner is never used` after removing position-based freezing. Removed the unused initialization while preserving shared-owner authorization.
- Automatic approval review rejected a broad automated rewrite and later a plan-gate removal because its policy scope was not explicit. The rejected rewrites did not run. Continued with bounded edits; the user's explicit early-access policy resolved the access question. No compatibility layer was added.

### What I learned
- Payment standing and enabled state must be independently represented; otherwise disabling an account misleadingly relabels a paying customer Free.
- Provider output budgets must span the complete tool loop, rather than restarting at 8,192 per round.
- First-publication dates must be established under the artifact lock so concurrent versions cannot independently extend early access.

### What was tricky to build
- Preserve real Stripe price acceptance when collapsing the old catalog, without preserving arbitrary ranks as active policy or translating credit balances into fictitious tokens.
- Charge uncertain calls conservatively while releasing unused output reservation for successful measured calls. Settlement and recovery are idempotent; an old process must be stopped before running migrations/new worker.
- Use the same early-access gate for free-sample files and demos, which previously had independent shortcuts.

### What warrants a second pair of eyes
- Review migrations 017–018, reservation/settlement/recovery, worker round budget and the download/demo early-access branches. The selected quota unit is generated/output tokens; total input/cache usage remains analytics.
- The global six-call counter is still process-local. This change does not make a multi-replica deployment share six lanes.

### What should be done in the future
- Review the implementation branch and deploy when requested. No deployment or vault write is included in this task.

### Code review instructions
- Read the short design, then store/usage.go and migrations, worker/output-budget.ts and main.ts, entitlement/customer_summary, early-access publishing/access paths, and finally the admin controls.
- Use token_ceiling_test.go and early_access_test.go as behavioral examples. Browser fixtures and screenshots are labelled separately from the database-backed API evidence.

### Technical details
- Free defaults: 10,000 generated tokens/UTC day, one saved workspace. Paying defaults: 100,000/day, ten workspaces. Default maximum output per logical run: 8,192; six global calls and one per user.
- New API: `PUT /api/admin/users/{id}/token-ceiling` with `{limit: number|null, reason}`. A null ceiling restores the class default.
- Artifact patch: `earlyAccessUntil` as RFC3339 or an empty string to clear. Publishing default: `publishing.earlyAccessDays`, 0–365. No scheduler is required for expiry; requests compare the stored date.

## Step 5: Publish the simplification separately from the payment fixes

Committed the implementation as `8708cc4` and pushed `feat/simple-customer-limits`. Opened draft PR 17, stacked on PR 16, so the original payment review fixes remain focused. PR 16 is green at `7098eed`; both new findings have explicit replies. PR 17 CI was running at publication.

- **Prompt context:** See Steps 3–4. No vault report was written and no deployment was performed.
- **Validation:** Ticket doctor and Git whitespace checks passed. Local implementation checks are recorded in Step 4; screenshots and the browser receipt explicitly identify mocked API fixtures.
- **Review:** [PR 17](https://github.com/go-go-golems/francois/pull/17), short design, migrations 017–018 and the provider output budget are the starting points. [PR 16](https://github.com/go-go-golems/francois/pull/16) contains the separate overlap/copy corrections.
- **Next:** Review and deploy when requested. No additional infrastructure or compatibility layer is needed.
