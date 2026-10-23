---
Title: Investigation diary
Ticket: FRANCOIS-025
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-23T18:15:40.673741703-04:00
WhatFor: ""
WhenToUse: ""
---

## Step 1: Establish live rollout ticket and recommend isolated billing environments

Created a dedicated planning ticket and investigated whether a backend checkbox can safely switch Stripe modes. Current startup-mode validation and unscoped customer/checkout mappings support recommending separate deployments rather than a runtime toggle. No implementation or live billing changes were made.

### Prompt Context
**User prompt (verbatim):**
> Ok, create a new docmgr ticket for that.
>
> Prices stay the same, I do have an activated live stripe account. Can we make switching from sandbox to live a checkbox in the backend? Or how would you balance testng with real?

**Assistant interpretation:** Create a ticket with the confirmed commercial/account decisions and explain a safe testing/live deployment strategy.
**Inferred user intent:** Keep testing convenient without accidentally charging users or corrupting real billing state.

### What I did
- Read AGENT.md, current billing migration, webhook mode gate and production runbook; reused the prior turn's CLI/manifest inspection as explicitly identified evidence.
- Created FRANCOIS-025, design and diary, with phased implementation tasks.
- Distinguished read-only environment indicator, privileged deployment transition and fully isolated dual-mode implementation.

### Why
A checkbox would select far more than a key: price IDs, customer mappings, webhook acceptance, queues, portal and entitlements. Live events continue even when staff want to test.

### What worked
Concrete code confirms startup key/mode checks, single-mode ingestion and customer/checkout state without mode/account isolation. User supplied activated-account readiness and unchanged pricing; recorded as operator decisions rather than independently verified Stripe facts.

### What didn't work
No errors in this ticket creation. The previous turn found the historical Terraform path stripe/apps/francois/envs absent; this discrepancy is recorded as an inventory task, not silently treated as deployed truth.

### What I learned
livemode columns on some rows do not establish environment isolation across billing. Sandbox validation must cover the same release while keeping its database and external effects separate.

### What was tricky to build
Separating a useful backend environment display from a dangerous global behavior toggle, and describing rollback honestly after real charges begin.

### What warrants a second pair of eyes
Existing sandbox-derived entitlements and the complete billing query surface before data cleanup; live price parity and scope of any real-money test.

### What should be done in the future
Execute the inventory and settle environment/data choices before implementation. No destructive cleanup, live credential changes or purchases are authorized by this ticket-creation step.

### Code review instructions
Read design-doc/01 and its source relations. Verify CLI and service mode guards against customer/checkout schema. Documentation-only work: no build/test suite was run or needed.

### Technical details
Repository has unrelated modified application files; stage only this ticket. Deployment hostname and account capability inspection are future work. No secret values were accessed.
