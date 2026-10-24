---
Title: Live billing operations and recovery
Ticket: FRANCOIS-025
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://ttmp/2026/10/23/FRANCOIS-025--live-stripe-rollout-and-safe-sandbox-testing/scripts/10-stage-approved-live-cutover.py
      Note: Historical one-shot cutover contract; do not replay
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-24T04:44:56.387942475-04:00
WhatFor: ""
WhenToUse: ""
---

# Live billing operations and recovery

## Current contract

Production https://slop.boutique is permanently bound to live Stripe account `acct_1TxEMNLpxiqBMHHi`. Sandbox remains test-only, in its own database. Never flip production to test for development.

- Production flag: `--stripe-live-mode=true`; single live webhook signing secret, no test-key fallback.
- Runtime custody: `kv/apps/francois/prod/runtime`; live provisioning metadata: `kv/apps/francois/prod/stripe-live`. Never print either secret document.
- Default fixed plan: `price_1UU0JILpxiqBMHHixeXUFilX`, USD 1/month, settings revision 12. Custom monthly/enterprise/year amounts unchanged.
- Live portal: `bpc_1UU0JNLpxiqBMHHiMnAdY50K`; period-end cancellation, payment-method updates and invoice history. Return URL `/account`.
- Enabled webhook: `we_1UU0JLLpxiqBMHHi0cOkQRDv`, https://slop.boutique/webhooks/stripe, API `2026-06-24.dahlia`, 14 configured event types.
- Both deployments run qualified image sha-1317326 at cutover. Check current images before later promotion.

## Qualification and monitoring

Live Checkout creation succeeded for an authenticated operator at USD 1/month. No card was entered and no payment was submitted. The session was explicitly expired; Stripe delivered a real signed `checkout.session.expired` with livemode=true, and production processed it. Live portal session creation also succeeded. Charges API returned zero live charges at completion. This is **not** paid live entitlement qualification.

Check Argo `francois` Synced/Healthy, deployment readiness, storefront HTTP status, Stripe endpoint delivery errors and production `stripe_events` pending/error rows. Query only bounded metadata, never webhook payloads or stored session URLs. Test-mode events must not enter active production billing tables.

The runtime restricted key has no refund-write permission; an authorized refund is an operator dashboard action. Do not add payout/refund rights merely for diagnostics. A real-money smoke test, its amount/payment method, and any later refund require explicit authorization.

## Archived test state and recovery

Approved test state is retained in production schema `billing_test_archive_20261024083807`: 4 customer mappings, 2 subscriptions, 7 checkout attempts, 229 events and all 11 previous settings revisions. These are no longer active billing records. Users, content, model-call history, roles and complimentary grants were not deleted. An operator may still see Paying from preserved independent access; that does not imply a live paid subscription.

Quiesced pre-cutover checkpoint: `postgres/postgres-20261024T083817Z.sql.gz`. Previous runtime credentials: `kv/apps/francois/prod/pre-live-20261024083807`. The database archive contains sensitive customer/session data: keep it inside the database/backups, not git. Old test webhook IDs `we_1UT9dvLeHmcfmvuC8kc1FN7a` and `we_1UQ9zfLeHmcfmvuC639e6BPZ` are disabled. Old remote Stripe test subscriptions/customers were retained, not mass-deleted.

Prefer roll-forward recovery while retaining live webhook processing. Do not blindly restore the whole pre-cutover database or re-enable old test credentials: new live customers/events already exist, and future live charges cannot be discarded. Before any rollback, stop new checkout admission, inspect current Stripe charges/subscriptions/events, and decide how to preserve/reconcile every live record. There is no implemented application-level checkout-pause switch; coordinate a targeted ingress restriction if required rather than claiming one exists.

Scripts 09 and 10 are historical preparation/cutover tools, **not reconcilers**. Script 09 disables its endpoint and expects a test runtime. Script 10 archives approved exact counts and stops production. Never rerun them against launched production.

## Deployment caveat

Production still uses Recreate. Zero-downtime serving handoff is task wqph, not completed by this cutover. During cutover Argo resumed an in-flight old-revision sync, causing `Stripe key mode does not match --stripe-live-mode` and a readiness timeout. Applying the already merged desired Deployment resolved it; Argo then converged Synced/Healthy. Future coordinated secret/mode transitions must terminate or explicitly pin the old operation before resuming, not merely assume a hard refresh replaces it.
