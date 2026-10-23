---
Title: Billing environments and live cutover design
Ticket: FRANCOIS-025
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/catalog.go
      Note: Startup Stripe mode and key validation
    - Path: repo://doc/production.md
      Note: Existing sandbox deployment contract
    - Path: repo://pkg/catalog/payments/service.go
      Note: Webhook mode rejection and reconciliation
    - Path: repo://pkg/catalog/store/billing.go
      Note: Customer and subscription queries
    - Path: repo://pkg/catalog/store/migrations/002_billing_usage.sql
      Note: Customer and checkout isolation constraints
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-23T18:15:39.69571272-04:00
WhatFor: ""
WhenToUse: ""
---

# Billing environments and live cutover design

## Confirmed operator decisions

- An activated live Stripe account exists (operator-reported; capability/account identity still needs read-only verification).
- Keep current prices. Inventory deployed amounts, currency, cadence, tiers and support options before creating live equivalents; do not infer current prices from historical docs.
- This ticket is planning only. No live keys, payment configuration, databases or deployments changed by this investigation.
- The operator asked whether sandbox/live switching can be a backend checkbox and how to balance testing with real billing.

## Recommendation: two environments, not one global checkbox

Keep production permanently bound to the live Stripe account, and provide a separate sandbox deployment built from the same code. Give sandbox its own hostname, database, Vault path, webhook endpoint/signing secret, Stripe test credentials and test price mapping. Gate its access and display a conspicuous SANDBOX banner. Use synthetic users/data rather than copying production customer/payment data. Account or sandbox identity must be verified, not inferred solely from a key prefix.

The backend should expose a **read-only billing environment indicator** and operational health: Live/Sandbox, configured account identity, last successful webhook, queue failures and links to the appropriate Stripe dashboard. Show no secret values. A link to the sandbox admin is useful. Do not make a generic settings save silently change credentials or billing mode.

A deployment-level transition can eventually be presented as an explicit, privileged operation with a preflight report, confirmation, audit trail and recovery steps. It is not a Boolean preference. Prefer a separate control to pause **new checkout admission** while leaving live webhooks, renewal processing and reconciliation running; confirm whether such a control exists before promising it.

Why not flip production to test for a few minutes? Stripe keeps delivering live renewals, cancellations, refunds and payment failures. The current service rejects events whose mode does not match its configured mode. A global switch would interrupt processing for existing paying users, not merely change the next checkout.

## Concrete current-code evidence

- `cmd/francois/cmds/catalog.go`: StripeLiveMode is a startup field; key mode is checked against it, live mode requires HTTPS, and the service is constructed with a single gateway/mode/deployment identity. The documented CLI switch is `--stripe-live-mode`.
- `pkg/catalog/payments/service.go`, Ingest: verifies webhook signature, then rejects `e.Livemode != s.Config.Livemode` before inserting the event. A mode checkbox would affect webhook handling and cannot safely be treated as UI-only state.
- `pkg/catalog/store/migrations/002_billing_usage.sql`: billing_customers is keyed only by user_id, with a unique stripe_customer_id. checkout_attempts has no mode/account namespace and one open attempt per user. Subscription/event rows have livemode, but that alone does not isolate all billing state.
- `pkg/catalog/store/billing.go`: CustomerForUser queries by user only; SaveCustomer inserts by user; subscription selection is user-scoped. All entitlement/reconciliation queries require review before any dual-mode design.
- `doc/production.md` section 3.5: plans own price mappings; obsolete runtime stripe-price-id is not read; bootstrap helper rejects live keys; webhook event list is maintained in `pkg/catalog/payments/webhook-events.txt`.
- The platform manifest inspected in the previous turn has key/signing-secret mounts but no live-mode flag. This is repository evidence, not a new live-cluster inspection.
- Historical docs reference a Terraform sandbox directory that was absent from the inspected Terraform checkout. Resolve actual ownership/location before creating a new environment or claiming existing state is reusable.

## Environment contract

| Concern | Production | Sandbox |
|---|---|---|
| Customer charges | Real money | Stripe test payments only |
| Hostname | slop.boutique | Dedicated hostname, to be chosen/provisioned |
| Database | Production users/content + live billing | Separate database and synthetic fixtures |
| Stripe API credentials | Live, Vault-owned | Sandbox/test, separate Vault path |
| Webhook signing secret | Live endpoint only | Test endpoint only |
| Products/prices | New live objects, same commercial pricing | Existing or recreated test equivalents |
| Identity | Existing production issuer/client | Separate client/callback registration; shared issuer is possible |
| Workers | Live events/reconciliation always running | Test events/reconciliation only |
| Emails/jobs | Approved production delivery | Restricted recipients or a sink; no unintended notifications |

Use the same reviewed release artifact in both deployments, with configuration differences explicitly recorded. Independent rollout can still introduce drift; run the sandbox gate against the exact release being promoted.

## Production cutover phases

### 1. Read-only inventory and design gate

Confirm Stripe account identity/capabilities, deployed flags, mounted secret contract, all billing entry points (plans, one-time support, recurring support, tier/enterprise variants), webhook endpoints and customer portal settings. Snapshot current settings revisions and price mappings. Verify secret values only in memory; report IDs/modes and configuration status, not keys.

Inventory sandbox customer mappings, subscriptions, checkout attempts, queued events, entitlements and reconciliation work in the production database. Determine whether any real financial records already exist; stop if assumptions are wrong. Decide how test-derived benefits should be treated. Preserve users, content and unrelated usage data.

### 2. Establish repeatable sandbox validation

Provision isolated runtime and credentials, synthetic fixtures, callback/webhook URLs and explicit sandbox UI. Test checkout completion/expiry/failure, duplicate or delayed events, subscription renewal/cancellation, refunds, portal access and entitlement changes. Check cross-mode events fail safely and never grant production access.

### 3. Prepare live configuration

Create live product/price equivalents with unchanged commercial terms; IDs necessarily differ. Validate actual amount/currency/interval and supported payment flow rather than copying IDs. Configure live customer portal products, plan changes, cancellation and return URLs. Create live webhook at https://slop.boutique/webhooks/stripe with the version-appropriate full event list from the repository; confirm API-version compatibility with the SDK. Keep API and endpoint signing secrets in Vault, not Terraform state, commits or logs. Replace the sandbox-only bootstrap contract with an explicit live provisioning path and mode validation.

### 4. Handle existing test state deliberately

Take a recoverable database/settings checkpoint. Pause new checkout admission and coordinate in-flight attempts and worker processing. Archive or migrate sandbox billing records out of production's active billing namespace using a reviewed scoped procedure. Obtain approval for any destructive cleanup. Do not delete users/content or mass-delete Stripe objects. If sandbox data must remain queryable, implement an explicit archival/read-only boundary rather than letting test subscriptions grant real entitlements.

### 5. Coordinated deployment

Deploy live credentials, signing secret, live flag and plan mappings as one controlled change. Account for mixed replicas during rollout and any pending events. Verify readiness, webhook processing and account/price mode checks before reopening checkout. Do not retain a sandbox signing secret as though it were merely a previous live key; rotation history and environment identity are different concerns.

### 6. Controlled real-money qualification

Only with explicit operator approval, execute a small real transaction using an appropriate real payment method and verify checkout, webhook processing, entitlements, portal and cancellation. A refund is a separate financial action requiring authorization; do not assume fees are refundable. Confirm taxes, receipts and statement descriptor before launch. A generic successful redirect does not prove webhook-driven entitlement activation.

### 7. Operate without mode switching

Use sandbox/test cards/fixtures for routine development and failure scenarios. Use passive production health checks and transaction reconciliation for live monitoring. Real-money smoke tests should be rare, bounded and expressly approved. Test alerting and have an operational response for webhook errors and payment/account capability changes.

## If same-backend dual-mode testing is required later

This is a separate feature, not this ticket's default approach. It requires `(Stripe account, mode, object ID)` ownership throughout customer mappings, subscriptions, checkout attempts, idempotency, event inboxes, reconciliation and entitlement queries. Keep both webhook receivers and workers active, isolate key selection per authorized operation, and prevent test entitlements from leaking into real authorization. Historical checkout URLs must retain their original context. Only staff may initiate sandbox flows; a user-supplied mode parameter must not select credentials. Switching the displayed admin environment must never switch production processing globally.

A separate database/deployment provides most of these boundaries with less custom billing logic. Therefore prioritize it unless the operator identifies a concrete workflow requiring dual-mode storage.

## Recovery and acceptance

Before the first live transaction, reversal to the previous application configuration may be possible if no live objects/events were created; prove that boundary. After real transactions begin, restoring a stale database or flipping back to sandbox can lose financial state. Prefer pausing new checkouts, retaining live event ingestion and reconciling against Stripe. A backup is not authorization to discard subsequent charges.

Completion evidence must include: inventory, approved price parity, mode isolation, sandbox regression coverage, scoped data-transition approval/results, live account/portal/webhook verification, credential custody, coordinated deployment, explicitly approved real-money qualification, operational checks and recovery instructions. Creating this design does not complete any implementation gate.

## Open choices

- Sandbox hostname and access policy.
- Sandbox hosting cost/resource limits and which external integrations are disabled.
- Treatment of existing test subscriptions/benefits and billing history.
- Exact live account identity and tax configuration.
- Approval and amount for the eventual real-money test.
