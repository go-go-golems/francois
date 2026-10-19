---
Title: Two customer classes, daily generated-token ceilings and clear administration
Ticket: FRANCOIS-017
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/customer_summary.go
      Note: Admin explanation and independent customer standing
    - Path: repo://pkg/catalog/early_access_test.go
      Note: Access-window integration evidence
    - Path: repo://pkg/catalog/store/migrations/018_early_access.sql
      Note: Direct two-class and early-access migration
    - Path: repo://pkg/catalog/store/usage.go
      Note: Atomic daily token accounting
    - Path: repo://ui/catalog/admin.tsx
      Note: Effective-access controls
    - Path: repo://worker/src/output-budget.ts
      Note: Shared budget across provider rounds
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

The public site remains minimal. Free accounts can run and download normally published artifacts after their early-access period ends. Payment grants early access and higher hosted resource limits rather than selecting a catalog of ranked products. Staff editing permissions remain separate from customer access.

## Policy

- Resolve every ordinary customer to Free or Paying. Monthly, year and complimentary standing all grant the same Paying limits. Staff use Paying resource limits, explicitly labelled staff access; admin permission is not evidence of a charge.
- Keep exactly two settings entries, `free` and `subscriber` (displayed Paying). Remove arbitrary plan creation, rank, plan visibility, marketing metadata, named-plan grants and per-model credit pricing from the current schema and controls. No compatibility adapter.
- Saved-workspace/member limits remain independent storage controls. Creation limits do not freeze an arbitrary oldest/newest subset of existing workspaces on downgrade. Existing workspace ownership and scoped runtime security remain intact. Explain a workspace denial separately from an AI allowance denial.
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

## Early access: confirmed implementation policy

The user clarified: “free accounts can download all normally published artifacts, but we can set a period for new artifacts to only be downloadable / runnable to be only used by paying subscribers” and “like, you pay for early access, and higher limits”.

`publishing.earlyAccessDays` is 0–365, default 0. First publication stamps `artifact.earlyAccessUntil` in the same transaction, locking the artifact so concurrent first releases cannot restart it. Later releases keep that date. Studio, CLI and MCP accept an RFC3339 end date; an empty string clears it. Before first publication, a blank date receives the configured default; a concrete past date opts out.

Until that date, running, downloads (including attachments/free-sample releases) and demos require Paying access. After it, Free accounts can run/download within their resource limits. Staff and complimentary Paying access qualify. Existing published artifacts receive no retroactive early-access window. The public page states the end date. Settings default to 10,000 generated tokens/day and one saved workspace for Free, and 100,000/day and ten workspaces for Paying. Existing Paying workspace/member limits are preserved by migration.

The migrations introduce a new token ledger without converting historic credits into invented token usage. They retain historical call measurements/settings JSON, replace the active policy with two fixed classes, preserve accepted Stripe price IDs, and drop the retired artifact `required_plan` column. Use one active application process during this migration and stop the old worker before starting the new version; the in-memory six-lane counter remains process-local.

## Review and verification

The PostgreSQL tests cover concurrent reservations, partial remaining budgets, idempotent settlement, conservative crash recovery, absolute overrides/reset, explicit admin failures, disabled Paying classification, settings migration and early-access expiry. Worker tests exercise the same output-budget helper used for provider rounds. Browser fixtures exercise the actual bundled admin at 1440px and 390px with explicit mocked API responses; the real API is verified separately by database-backed tests. Screenshots and a labelled browser receipt are under `sources/implementation/`.
