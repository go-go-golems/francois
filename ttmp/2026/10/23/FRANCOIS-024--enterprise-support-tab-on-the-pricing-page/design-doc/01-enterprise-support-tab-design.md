---
Title: Enterprise support tab design
Ticket: FRANCOIS-024
Status: active
Topics:
    - billing
    - frontend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/payments/gateway.go
      Note: CreateCheckout price_data and Project metadata
    - Path: repo://pkg/catalog/store/migrations/021_support_tier.sql
      Note: Stored tier
    - Path: repo://pkg/catalog/support.go
      Note: supportCheckout validation and supportView
    - Path: repo://pkg/catalog/web/templates/pricing.tmpl
      Note: Monthly support form gets the tabs
    - Path: repo://ui/catalog/enterprise-amount.ts
      Note: Amount rule (client)
    - Path: repo://ui/catalog/public.ts
      Note: bindSupport, model for the enterprise form
ExternalSources: []
Summary: 'A second tab on /pricing for enterprise monthly support: default 256 USD, halved or doubled with >>1 and <<1 between 8 and 4096 USD.'
LastUpdated: 2026-10-23T16:54:19.393243907-04:00
WhatFor: ""
WhenToUse: ""
---

# Enterprise support tab design

## 1. What we want

The pricing page (`/pricing`) has one way to support the lab every month: a number, `−` and `+` buttons, 42 USD by default, any whole amount from 1 to 9999 USD. Under it is "Or fund a whole year" (999,999.99 USD).

We add a second option next to the monthly one, chosen with two tabs:

```
 Individual   Enterprise                      ← tabs; Individual is selected by default
 ──────────
 Every month
 [ − ]  42 USD  [ + ]                         ← unchanged
 We suggest 42 USD. Choose any amount ...
 [ Support monthly → ]
```

```
 Individual   Enterprise
              ──────────
 Every month, for your company
 [ >>1 ]  256 USD  [ <<1 ]
 Halve or double. From 8 to 4096 USD a month.
 [ Support as a company → ]
```

- **Default:** 256 USD a month.
- **`>>1`** (shift right by one bit) halves the amount: 256 → 128 → 64 → 32 → 16 → 8.
- **`<<1`** (shift left by one bit) doubles it: 256 → 512 → 1024 → 2048 → 4096.
- The amount is always a power of two between 8 and 4096. It cannot be typed; the buttons are the only control. At 8 the `>>1` button is disabled, at 4096 the `<<1` button is disabled.
- `>>1` sits on the left and `<<1` on the right, in the places of `−` and `+`, so both tabs read "smaller on the left, larger on the right".

The year funding block below stays as it is, under both tabs.

## 2. Current state (what the code does today)

| Piece | File | Today |
|---|---|---|
| Form markup | `pkg/catalog/web/templates/pricing.tmpl` | One `form.support-monthly[data-support-monthly]` with `input[name=amount]` (1–9999, value 42) and two `button[data-amount-step]` (−1, +1). Hidden for visitors who already support (`{{if .Supporter}}`). |
| Client | `ui/catalog/public.ts` `bindSupport` | Steps the input by ±1, clamps 1–9999, reads `?amount=` on load, sends `POST /api/billing/support {amountCents}`. Guests are sent to `/signin?return=/pricing?amount=N`. `subscription_open` opens the Stripe portal instead. |
| Endpoint | `pkg/catalog/support.go` `supportCheckout` | Accepts `kind: "" or "month"` with 100–999900 cents in whole dollars, or `kind: "year"`. Requires a verified, non-guest user. |
| Payments | `pkg/catalog/payments/service.go` `Support` → `checkout` | One open support subscription per customer (otherwise `ErrSubscriptionOpen`). Admits a checkout attempt keyed `support:usd:<cents>`. |
| Stripe | `pkg/catalog/payments/gateway.go` `CreateCheckout` | Subscription Checkout with inline `price_data`: USD, monthly, product name "Monthly support", subscription metadata `francois_monthly_support=true`. |
| Projection | `gateway.go` `Project` | A subscription with that metadata, one USD monthly item, quantity 1, becomes `SupportAmountCents`. That grants the Paying entitlement (early access, higher limits). |
| Display | `support.go` `supportStatus`, `ui/catalog/account.tsx`, `recurring-support.tsx` | "N USD a month." |

Enterprise fits inside this path: it is monthly support with a different amount rule and a different name on the invoice.

## 3. Design

### 3.1 Page

- `pricing.tmpl`: wrap the monthly form in a tab list with two tabs, **Individual** and **Enterprise** (`role="tablist"`, `role="tab"`, `aria-selected`, `aria-controls`; arrow keys move between tabs). Each tab has its own panel and form.
- The Enterprise panel is a second form, `form.support-monthly[data-support-enterprise]`, with the same layout and classes as the individual one so both look the same. The value is a read-only output (`<output name="amount">256</output> USD`) instead of an input, because only powers of two are allowed.
- The selected tab is in the URL as `?tier=enterprise` (absent = individual), so links can point straight at it and sign-in returns to the same tab: `/signin?return=/pricing?tier=enterprise&amount=1024`.
- The tab labels, the enterprise label, help line and button text come from site copy (FRANCOIS-022 `settings.CopyKeys`), so they can be edited in the admin panel: `support.tab.individual`, `support.tab.enterprise`, `support.enterprise.label`, `support.enterprise.help`, `support.enterprise.button`.
- Without JavaScript both panels are shown one under the other (the tabs are progressive enhancement), and the enterprise form posts the default 256.

### 3.2 Client (`ui/catalog/public.ts`)

- `bindSupportTabs`: switches panels, updates `?tier=` with `history.replaceState`, keyboard support.
- `bindEnterprise(form)`: holds `amount` as a power of two. `>>1` → `amount >> 1` if `amount > 8`; `<<1` → `amount << 1` if `amount < 4096`. Disables the button at each end. Reads `?amount=` on load and accepts it only when it is a power of two in range.
- Submit: same flow as `bindSupport` (sign in if needed, `subscription_open` → portal), posting `{kind: "enterprise", amountCents: amount * 100}`.
- The power-of-two rule lives in a small pure helper (`enterpriseStep(amount, direction)`) with unit tests, like `launch-decision.ts`.

### 3.3 Server

- `supportCheckout`: accept `kind: "enterprise"` when `amountCents/100` is a power of two between 8 and 4096; otherwise `400 invalid_amount` "Choose 8, 16, 32 … 4096 US dollars per month".
- `payments.Service.SupportEnterprise(ctx, user, cents, …)`: same checks and `checkout` as `Support`, attempt price key `support:enterprise:usd:<cents>`.
- `CheckoutRequest` gets `Tier string` ("" or "enterprise"). For enterprise, `CreateCheckout`:
  - product name **"Enterprise support"** (shows on Stripe receipts and invoices);
  - subscription metadata `francois_monthly_support=true` (so `Project` and the entitlement keep working unchanged) plus `francois_support_tier=enterprise`;
  - `TaxIDCollection.Enabled = true`, `BillingAddressCollection = "required"` and `CustomerUpdate{Name: auto, Address: auto}` so companies get a proper invoice with their name, address and VAT/tax ID.
- `billing.Subscription` gets `SupportTier string` read from that metadata in `Project`; `supportView` gets `Tier`. The account page and the recurring-support panel say "Enterprise support, 1024 USD a month." Everything else (entitlement, limits, early access) is the same as individual support.
- One support subscription per customer stays the rule: an individual supporter who opens Enterprise gets the Stripe portal, as today for a second individual subscription.

### 3.4 What it does not change

- Entitlements: enterprise support grants the same Paying limits as individual support (see open question 1).
- Year funding, plans and the Stripe portal.
- Existing subscriptions: they have no tier metadata and stay individual.

## 4. Implementation plan

1. Site copy keys for the tab and enterprise texts (`settings/copy.go`), with defaults.
2. Server: `kind: "enterprise"` validation, `SupportEnterprise`, gateway tier with product name, metadata, tax ID and billing address; `Project` reads the tier; `supportView.Tier`. Tests: amount validation table (7, 8, 12, 256, 4096, 8192 USD), gateway params for enterprise (fake gateway records the request), projection of the tier.
3. Client: `enterpriseStep` helper with tests; `bindSupportTabs` and `bindEnterprise`; `?tier=`/`?amount=` round trip through sign-in.
4. Template and CSS: tabs (`.support-tabs`, reuse `.tag`-like underline style from the landing page), enterprise panel reusing `.support-amount-controls`; the `>>1`/`<<1` buttons use the monospace font so the operators read as code.
5. Account page and recurring-support panel: show the tier.
6. Browser check on the local preview (Stripe test mode or fake gateway), screenshots desktop and phone; diary; commit per step.

## 5. Open questions

1. **What does enterprise get?** Proposed: the same as individual support (it is a way for a company to pay more and get an invoice). Alternatives: more instances or model credits, scaled with the amount; several seats.
2. **Invoices:** collect tax ID and billing address in Checkout for enterprise only (proposed), or for individuals too?
3. **Labels:** "Individual / Enterprise" for the tabs, "Support as a company →" for the button? Should the `>>1`/`<<1` buttons have a plain-language tooltip ("Halve", "Double") for people who don't read bit shifts?
4. **Switching:** should an individual supporter be able to move to enterprise from the pricing page (Stripe would replace the subscription), or is going through the portal, as proposed, enough?

## 6. Decisions taken when building (2026-10-23)

The user asked to "build enterprise support" without answering section 5, so the proposed answers were used:
1. Same access as individual support.
2. Tax ID and billing address are collected for enterprise only.
3. Labels: "Individual" / "Enterprise", "Every month, for your company", "Support as a company →". The `>>1`/`<<1` buttons have "Halve" / "Double" tooltips. All of these are site copy and editable in the admin panel.
4. An individual supporter is sent to the Stripe portal, as for a second individual subscription.

One deviation from section 3.1: the help text is one sentence that also mentions the invoice ("Halve or double, from 8 to 4096 USD a month. Invoiced to your company, with its address and tax ID.").

### Update: operator direction and help text

The user decided the buttons the other way round from sections 1 and 3: **`<<1` divides by two** (left button, 256 → 128 → … → 8) and **`>>1` multiplies by two** (right button, 256 → 512 → … → 4096). The help line is "Halve or double, from 8 to 4096 USD a month.", without the invoice sentence. Checkout still collects the company's address and tax ID.
