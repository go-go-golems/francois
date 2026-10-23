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

## Step 2: Build enterprise support

Built enterprise monthly support end to end, as designed in Step 1, using the design's proposed answers to its open questions. On the server, `kind: "enterprise"` is validated as a power of two between 8 and 4096 USD. The Stripe Checkout is named "Enterprise support", carries a tier tag and collects the company's name, address and tax ID; the tier is projected and stored (migration 021) and reported on the account. The pricing page has Individual / Enterprise tabs, and the enterprise amount moves with `>>1` and `<<1`. Both are tested, and the page was checked in a browser.

### Prompt Context
**User prompt (verbatim):** "build enterprise support"
**Assistant interpretation:** Implement the FRANCOIS-024 design; the open questions were not answered, so take the proposed defaults.
**Inferred user intent:** Ship the enterprise option on the pricing page.
**Commit (code):** a2f17f0 — "feat(billing): enterprise monthly support tier"; e3156f2 — "feat(catalog): Individual / Enterprise tabs on the pricing page"

### What I did
- `billing/policy.go`: `TierEnterprise`, `EnterpriseMinCents`/`MaxCents`/`DefaultCents`, `EnterpriseAmount(cents)`; `Subscription.SupportTier`.
- `payments/gateway.go`: `CheckoutRequest.SupportTier`. For enterprise, `CreateCheckout` sets the product name "Enterprise support", `subscription_data.metadata.francois_support_tier=enterprise`, `tax_id_collection.enabled`, `billing_address_collection=required` and `customer_update{name,address}=auto`. `Project` sets `SupportTier` only on a valid support subscription.
- `payments/service.go`: `SupportEnterprise` (attempt key `support:enterprise:usd:<cents>`); `checkout` takes the tier.
- `store/billing.go` + `migrations/021_support_tier.sql`: `support_tier text NOT NULL DEFAULT '' CHECK IN ('','enterprise')` is read and written.
- `support.go`: `validSupport(kind, cents)`; `supportCheckout` routes `enterprise` to `SupportEnterprise` with its own 400 message; `supportView.Tier`.
- `settings/copy.go`: keys `support.tab.individual`, `support.tab.enterprise`, `support.enterprise.label`, `support.enterprise.help`, `support.enterprise.button`.
- `pricing.tmpl`: `div[data-support-tiers]` with a `hidden` tablist and two tab panels; the enterprise form has an `<output>` for the amount and `>>1` / `<<1` buttons.
- `ui/catalog/enterprise-amount.ts` (`isEnterpriseAmount`, `shift`) + `tests/enterprise-amount.test.ts`. In `public.ts`: the shared `startSupport` (sign in / checkout / portal), `bindEnterprise` and `bindSupportTiers` (arrow keys, `?tier=` via `replaceState`). `bindSupport` ignores `?amount=` on enterprise links.
- `account.tsx`, `recurring-support.tsx`, `api.ts`: "Enterprise support, N USD a month."
- CSS: `.support-tiers`, `.support-tabs`, `.support-panel[hidden]`, `.support-shift` (monospace), disabled buttons.
- Tests: `payments/enterprise_test.go` (amount table, Stripe form payload, individual payload has no tier/tax fields, projection, service request and attempt key), `support_test.go` `TestEnterpriseSupport` (pricing markup, account summary with tier, entitlement, `validSupport` table).

### Why
- Reusing `francois_monthly_support=true` keeps the entitlement path untouched: an enterprise subscription is projected as support, and the tier is extra information.
- The tier is a column, not derived from the amount, because 8–4096 overlaps the individual range (an individual can give 256 too).
- The amount is an `<output>`, not an input, so only valid amounts can be chosen; the server checks again.

### What worked
- `go test ./... -count=1` all ok; `pnpm check` ok; `pnpm test` 22 pass; prettier clean; golangci-lint only reports the uncommitted preview file.
- Playwright on `/pricing`:
  - The tabs appear and the Enterprise panel is hidden at first. Clicking Enterprise sets `?tier=enterprise` and hides Individual.
  - `>>1` goes 256 → 128 → 64 → 32 → 16 → 8 and is then disabled. `<<1` goes 8 → … → 4096 and is then disabled.
  - The left and right arrow keys switch tabs.
  - A guest's submit at 1024 goes to `/signin?return=/pricing?tier=enterprise&amount=1024`.
  - `?tier=enterprise&amount=1024` restores the tab and 1024; `amount=300` falls back to 256.
  - The free user's submit reaches the server and shows "Billing is not configured" (no Stripe locally).
  - At 390 px nothing overflows (`scrollWidth` 375).

### What didn't work
- My first endpoint test table expected 400 for invalid amounts, but got `503 billing_unavailable` for all of them. `supportCheckout` returns 503 before reading the body when `s.Payments` is nil, which is always the case in the catalog harness. I extracted `validSupport` and tested it directly; the Stripe side is tested in `payments`.
- The first rewrite of the body check returned without a response on a JSON error (`httpx.Read` does not write one). I restored the 400 `invalid_amount` for unreadable bodies.

### What I learned
- The `payments` package already had a Stripe form-payload test pattern (`httptest` server + `ParseForm`). Reusing it checks the real `stripe-go` encoding of `tax_id_collection`, `billing_address_collection` and `customer_update`.

### What was tricky to build
- Query parameters for both forms: `?amount=` used to belong to the individual form. An enterprise link `?tier=enterprise&amount=1024` must not set the individual input to 1024, and switching tabs must drop a stale `amount`. Each form now reads `amount` only for its own tier, and a tab click deletes `amount` from the URL.

### What warrants a second pair of eyes
- `customer_update` with `name`/`address: auto` overwrites the Stripe customer's name and address with what the company enters. For a person who later supports individually, the invoice would carry the company details. That is acceptable for invoices, but worth knowing.
- Stripe requires automatic tax or a valid business configuration for `tax_id_collection` in some countries; test mode should be tried before deploy.
- The admin subscriptions list does not show the tier yet (the API row includes `supportTier`).

### What should be done in the future
- A Stripe test-mode checkout of an enterprise subscription before deploying (check the tax ID field, the invoice and the webhook projection).
- Optionally show the tier in the admin subscriptions table.

### Code review instructions
- Server: `billing/policy.go` (end), `payments/gateway.go` `CreateCheckout`/`Project`, `payments/service.go` `SupportEnterprise`, `support.go` `validSupport`/`supportCheckout`, `migrations/021_support_tier.sql`.
- Client: `ui/catalog/enterprise-amount.ts`, `public.ts` `startSupport`/`bindEnterprise`/`bindSupportTiers`, `pricing.tmpl`.
- Validate: `go test ./pkg/catalog/payments ./pkg/catalog -run 'Enterprise|Support'`; `pnpm test`.

### Technical details
- Stripe form keys sent for enterprise: `line_items[0][price_data][product_data][name]=Enterprise support`, `subscription_data[metadata][francois_support_tier]=enterprise`, `tax_id_collection[enabled]=true`, `billing_address_collection=required`, `customer_update[name]=auto`, `customer_update[address]=auto`.

## Step 3: Operator direction and help text

While Step 2 was being written up, the user corrected two things. The operators go the other way: `<<1` divides by two and `>>1` multiplies by two. And the help line should not mention invoicing. Both are changed; the left button is now `<<1` (halve) and the right one `>>1` (double).

### Prompt Context
**User prompt (verbatim):** "<< = divide by 2 (shift left)\n>> = multilpe by 2 (shift right).\n\nupdate" and "don't add the \"invoiced to your company\""
**Assistant interpretation:** Swap the meaning of the two labels and drop the invoice sentence from the help text.
**Inferred user intent:** The control should read as the user thinks of it, and the page should stay short.
**Commit (code):** 22a4c1d — "fix(catalog): <<1 halves and >>1 doubles the enterprise amount"

### What I did
- `pricing.tmpl`: the left button is `data-shift="half"` with the label `&lt;&lt;1` and the right one `data-shift="double"` with `&gt;&gt;1`; the tooltips are still "Halve" / "Double".
- `enterprise-amount.ts`: `Shift = "half" | "double"`; `shift` multiplies or divides by 2 instead of using `<<`/`>>`, so the code no longer encodes a direction.
- `public.ts`, `tests/enterprise-amount.test.ts`, `support_test.go`, comment in `billing/policy.go` updated.
- `settings/copy.go` `support.enterprise.help` default: "Halve or double, from 8 to 4096 USD a month."
- Design doc: added an "Update" note under section 6.

### Why
- Naming the steps "half"/"double" keeps the button labels free to change (they are plain text in the template) without touching the logic.

### What worked
- `pnpm test` 22 pass; `go test ./pkg/catalog ./pkg/catalog/payments ./pkg/catalog/settings -run 'Enterprise|Support|Copy'` ok.
- Playwright: the left label is `<<1` and goes 256 → 128; the right label is `>>1` and goes 128 → 256 → 512; the help text is the new sentence.

### What didn't work
- N/A.

### What I learned
- N/A.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- Checkout still collects the billing address and tax ID for enterprise; only the sentence on the page was removed.

### What should be done in the future
- Stripe test-mode checkout before deploy (task open).

### Code review instructions
- `pricing.tmpl` enterprise controls; `enterprise-amount.ts`.

### Technical details
- N/A.

## Step 4: Enterprise checkout made identical to individual support

The user asked what differs between the two checkouts. I listed the enterprise-only Stripe settings (product name, tax ID, required billing address, customer update), and the user decided: "no they should just be the same, just the price changes." Removed those settings from `CreateCheckout` and kept only the invisible tier tag.

### Prompt Context
**User prompt (verbatim):** "is there a difference between the enterprise checkout and the normal checkout, besides the amount?" then "no they should just be the same, just the price changes."
**Assistant interpretation:** The Stripe Checkout for enterprise must be the individual one with another amount.
**Inferred user intent:** No extra friction or different invoices for companies; enterprise is a price level.
**Commit (code):** 01e0617 — "fix(payments): enterprise checkout is the individual checkout at another price"

### What I did
- `payments/gateway.go`: the enterprise branch now only adds `subscription_data.metadata.francois_support_tier=enterprise`.
- `payments/enterprise_test.go`: the enterprise payload must have the product name "Monthly support" and the tier tag, and must not send `tax_id_collection`, `billing_address_collection` or `customer_update`.
- Design doc: added an update note under section 6. Branch `fix/enterprise-checkout-same` (it also carries the unpushed FRANCOIS-022 Step 12 diary commit).

### What worked
- `go test ./pkg/catalog/payments ./pkg/catalog -run 'Enterprise|Support|Stripe'` ok; golangci-lint on payments: 0 issues.

### What didn't work
- My first Python edit of the test asserted on the pre-gofmt text and stopped with `AssertionError` (gofmt had aligned the map keys). The gateway edit had already been written, so the payments test failed until I rewrote the test lines by line number.

### What I learned
- N/A.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- The account page and the recurring-support note still say "Enterprise support"; that is outside the checkout. Say if those should read like individual support too.

### What should be done in the future
- Deploy this change. The "Stripe test-mode checkout" task now only needs to confirm the enterprise amount.

### Code review instructions
- `payments/gateway.go` `CreateCheckout`; `payments/enterprise_test.go`.

### Technical details
- N/A.
