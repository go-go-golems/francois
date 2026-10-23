---
Title: Diary
Ticket: FRANCOIS-024
Status: active
Topics:
    - billing
    - frontend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-23T16:54:19.744219259-04:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

<!-- What is the purpose of this reference document? -->

## Context

<!-- Provide background context needed to use this reference -->

## Quick Reference

<!-- Provide copy/paste-ready content, API contracts, or quick-look tables -->

## Usage Examples

<!-- Show how to use this reference in practice -->

## Related

<!-- Link to related documents or resources -->

## Step 1: Design for the enterprise support tab

Wrote a short design for a second tab on `/pricing`: enterprise monthly support, 256 USD by default, halved with `>>1` and doubled with `<<1` between 8 and 4096 USD. It reuses the monthly support checkout end to end and adds a tier on the Stripe product name and metadata, plus tax ID and billing address collection for company invoices. No code yet.

### Prompt Context
**User prompt (verbatim):** "Let's add one more feature, make a small design doc: Besides the $42 +/- subscription thing, I want a second tab next to it (like you can toggle between the subs) and make a \"enterprise subscription\" with default $256 and then you can <<1 or >>1 to go 128 64 32 16 8 or conversely 256 512 1024 2048 4096"
**Assistant interpretation:** Design (not yet build) a tabbed pricing form with an enterprise option whose amount moves by powers of two.
**Inferred user intent:** Let companies support the lab at larger, round amounts, with a playful bit-shift control.
**Commit (code):** N/A (docs only)

### What I did
- Read `pricing.tmpl`, `public.ts` `bindSupport`, `support.go` `supportCheckout`/`supportStatus`, `payments/service.go` `Support`/`checkout`, `payments/gateway.go` `CreateCheckout`/`Project`.
- Wrote `design-doc/01-enterprise-support-tab-design.md` (goal with sketches, current state table, page/client/server design, plan, open questions) and added the tasks.

### Why
- Enterprise as a tier of the existing monthly support keeps one entitlement path (`francois_monthly_support=true` → `SupportAmountCents` → Paying) and the one-subscription-per-customer rule.

### What worked
- The existing inline `price_data` checkout takes any amount, so no Stripe prices need to be created.

### What didn't work
- N/A.

### What I learned
- Today's monthly support Checkout does not collect a tax ID or billing address; companies need both for invoices.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- Direction of the operators: `>>1` halves and sits where `−` is, `<<1` doubles and sits where `+` is (the request listed them as "<<1 or >>1 to go 128 … or conversely 256 512 …"; the design follows the real bit-shift meaning).

### What should be done in the future
- Answer the open questions in section 5, then implement in the order of section 4.

### Code review instructions
- Read the design doc; the current-state table cites the files above.

### Technical details
- Valid enterprise amounts: 8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096 USD; `amountCents = amount * 100`, within today's 100–999900 limit.
