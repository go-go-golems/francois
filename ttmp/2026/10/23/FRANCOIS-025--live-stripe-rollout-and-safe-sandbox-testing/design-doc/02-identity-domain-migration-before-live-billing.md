---
Title: Identity domain migration before live billing
Ticket: FRANCOIS-025
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-23T19:53:04.764280343-04:00
WhatFor: ""
WhenToUse: ""
---

# Pre-launch identity-domain migration and live billing sequence

## Scope and operator decisions

Move the shared ZITADEL public identity domain to `auth.slop.boutique`, then prepare live Stripe after validating identity and sandbox payments. The operator confirms there are no real customers and requests a pragmatic pre-launch cutover, not a zero-downtime enterprise migration. Preserve operator accounts, artifact/content data, shared-service credentials and the ZITADEL master key. No charge to a real payment method is implied by provisioning live configuration.

Use one bounded maintenance window per subsystem and a fresh recoverable checkpoint. Do not build dual-issuer compatibility layers or a global billing-mode toggle. Existing sessions may require reauthentication. Keep old-domain resources temporarily only as a documented recovery aid, not as an unsupported promise of token compatibility.

## Facts and risks

- ZITADEL currently advertises https://zitadel.yolo.scapegoat.dev. Helm values own ExternalDomain and Login URLs. The Git-managed branding reconciler also calls the old address.
- François stores unique (issuer, subject) identities. Both production and sandbox rows must move to the new issuer without changing local user IDs. Check for target-issuer collisions first; do not merge accounts by email.
- ZITADEL is shared. Inventory active consumers and distinguish current workloads from retired manifests before changing configuration. Preserve machine identities and service tokens.
- Google/GitHub OAuth redirect allowlists must admit https://auth.slop.boutique/idps/callback before public cutover. Internal OIDC application callback URLs remain on their own app hosts.
- DNS wildcard resolution alone does not provision TLS or register a ZITADEL instance domain. Follow current upstream self-hosted domain/setup behavior and verify persisted instance-domain state.
- Startup issuer checks, JWT iss claims, discovery, JWKS and provider metadata must agree. A redirect from the old hostname is not an issuer migration.
- Stripe production remains test-mode until the independent live billing transition. No dual-mode application data architecture is required for this pre-launch change.

## Execution sequence and gates

### A. Baseline and inventory

Capture current version/image, configured hostnames, enabled consumer workloads, callback allowlists and nonsecret provider/client identifiers. Inventory production billing counts/modes and sandbox state without dumping personal records. Run one controlled sandbox checkout and check webhook-driven entitlement plus portal/cancellation; use Stripe test cards only. Record any real blockers rather than demanding exhaustive regression before pre-launch progress.

### B. Prepare new identity hostname

Verify auth.slop.boutique DNS points to the ingress. Provision a certificate and route without replacing existing production routes prematurely. Determine the supported ZITADEL domain addition/primary-domain/setup steps from official docs and deployed-version implementation. Pre-register social callbacks in the correct Google/GitHub apps. Update shared Terraform provider endpoints and automation references deliberately; do not global-replace historical diaries or retired code.

### C. Short identity cutover

Take a fresh ZITADEL/François recovery checkpoint and record object keys plus master-key custody. Pause writers/reconciliation only where needed. Update canonical ZITADEL external domain/Login settings, instance domain, ingress and consumers together. Apply exact old-to-new issuer updates in both François databases inside transactions, preserving subjects/user IDs and checking collisions/counts. Roll consumers to load new configuration; restore normal reconciliation promptly.

### D. Focused identity verification

Verify new TLS, discovery issuer, JWKS, readiness and authenticated management access. Check Google/GitHub callback routing and email login with an existing operator. Verify the same local user ID, roles, artifact ownership and billing mappings before/after. Re-run one sandbox checkout/entitlement check with the new issuer. Record old-domain/session behavior honestly. No production customers means planned reauthentication is acceptable.

### E. Live billing preparation and cutover

Inventory test billing dependencies; archive scoped test mappings/subscriptions/attempts/events as necessary without touching users/content. Present the actual affected counts before destructive changes. Create live price equivalents with unchanged amounts/currency/cadence, portal and signed webhook; store keys only in Vault. Deploy live flag, keys, webhook verification and live plan mappings together. Existing test keys/records are not valid live identities. Preserve live event processing if any real transaction occurs. Do not flip production back to sandbox as a test mechanism.

### F. Handoff

Report active hostname/mode, completed checks, backups, commits and remaining operator actions. Real-money smoke test requires a separately approved payment/amount; sandbox regressions need no real charge. Update diary/tasks at meaningful boundaries.

## Recovery

Before new writes/charges, restore the recorded configuration and issuer mappings if qualification fails. Use database restoration only if required by actual state changes; do not blindly restore shared PostgreSQL. Preserve new evidence and inspect current state before repeating mutations after ambiguous responses. Once real charges occur, prefer disabling new purchases while continuing live webhook processing over database rollback.

## Acceptance

Identity migration completes when the new canonical issuer works, actual active consumers have been updated, existing operator identity/content links are preserved, callbacks work, and reconciliation is restored. Billing completion is separate. A prepared manifest or healthy Argo object alone is not end-to-end verification.

## Sources to verify during execution

- https://zitadel.com/docs/self-hosting/manage/custom-domain
- Platform gitops/applications/zitadel.yaml and gitops/kustomize/zitadel/branding/reconcile.py
- François cmd/francois/cmds/catalog.go and pkg/catalog/store/migrations/001_core.sql
- Existing sandbox design and deployment receipts in this ticket
