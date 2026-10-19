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
**User prompt (verbatim):** “let's address the new code review comments, and then go through the simplication (write a short design doc), then implement  write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill).      
 Commit and push the bsidian vault when done (go-go-parc vault).”
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
