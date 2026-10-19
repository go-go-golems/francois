---
Title: Two customer classes, daily generated-token ceilings and clear administration
Ticket: FRANCOIS-017
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-18T22:23:54.841274895-04:00
WhatFor: ""
WhenToUse: ""
---

# Two customer classes, daily generated-token ceilings and clear administration

## Executive Summary

<!-- Provide a high-level overview of the design proposal -->

## Problem Statement

<!-- Describe the problem this design addresses -->

## Proposed Solution

<!-- Describe the proposed solution in detail -->

## Design Decisions

<!-- Document key design decisions and rationale -->

## Alternatives Considered

<!-- List alternative approaches that were considered and why they were rejected -->

## Implementation Plan

<!-- Outline the steps to implement this design -->

## Open Questions

<!-- List any unresolved questions or concerns -->

## References

<!-- Link to related documents, RFCs, or external resources -->

# Two classes and predictable administration

The public site remains minimal. Free and Paying accounts use the same published artifacts. Payment changes hosted resource limits rather than selecting a catalog of ranked products. Staff editing permissions remain separate from customer access.

## Policy

- Resolve every ordinary customer to Free or Paying. Monthly, year and complimentary standing all grant the same Paying limits. Staff use Paying resource limits, explicitly labelled staff access; admin permission is not evidence of a charge.
- Keep two fixed settings entries, `free` and `subscriber` (displayed Paying). Remove arbitrary plan creation, rank-based artifact authorization, named-plan grants and per-model credit pricing from the operational UI and effective policy.
- Saved-workspace/member limits remain independent storage controls. Existing workspace ownership and scoped runtime security remain intact. Explain a workspace denial separately from an AI allowance denial.
- Enforce **generated/output tokens per UTC day**. Record input/cache/cost separately. One logical run receives a server-only output budget, capped across all rounds; one active call per user and six globally remain unchanged.
- Reserve up to the run cap from the remaining daily budget atomically. Settle measured output, releasing unused reservation. Uncertain failed/cancelled usage conservatively consumes the reservation and is marked for review. A restarted process recovers outstanding reservations using the existing mechanism.
- A per-user absolute daily ceiling overrides the class default until explicitly reset. It does not grant Paying access, enable a disabled account or alter Stripe contracts. No additive credit balances or monthly customer allowance.

## Admin

The first detail section answers class, enabled state, source/expiry, AI eligibility/reason, generated tokens used/reserved/ceiling and UTC reset. Show workspace counts/limits separately. Primary actions: enable/disable, set/reset daily ceiling, grant/end complimentary Paying access. Raw identity, payment contracts, staff permissions and session revocation belong under Details. Failed core queries must produce an error rather than a zero/Free-looking summary.

Both customer list and detail use the backend's effective classification. Account eligibility is a timestamped snapshot, not a guarantee that a global lane will remain available. Year standing and recurring contracts are represented independently.

## Implementation

Use the existing catalog/settings, entitlement, hosted call path, PostgreSQL ledger and worker protocol. Add no queue, service or policy language. The accounting migration preserves historical model-call measurements and starts the new daily token period without translating credits into fictional tokens. Explicitly migrate the active settings to the two fixed entries; retain old settings revisions as historical documents, not active policy. Update CLI/MCP control names alongside HTTP/UI.

`customer state → fixed limits → atomic daily reservation → call-wide worker budget → measured settlement`

Implement review corrections first, policy/accounting second, admin/settings presentation third. Validate once at the integration boundary, with focused coverage for year overlap, quota concurrency/settlement, worker multi-round bounds and admin explanations. No vault report or production deployment is part of this request.
