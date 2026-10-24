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

## Step 2: Provision and qualify the isolated sandbox runtime

Provisioned sandbox.slop.boutique with independent application state, a separate PKCE client, operator ingress gate and dedicated Stripe test credentials. Runtime and TLS are healthy; the operator confirmed normal account sign-in works. Production billing was not switched to live, and the broader payment regression/cutover tasks remain open.

### Prompt Context
**User prompts (verbatim):** "ok, update the docmgr ticket design doc if needed, then go"; "i logged in to stripe"; "try again"; "continue"; "do you need me to do anything?"; "what should I log in with?"; "ok, worked. no need to rotate."
**Assistant interpretation:** Provision the isolated sandbox, use the authenticated Stripe test dashboard, and accept operator-confirmed login without rotating the operator gate password.
**Inferred user intent:** Obtain a usable test environment without altering production billing.
**Commits:** Terraform 04fd555 (sandbox OIDC); platform 2621d11 (sandbox), aa1bd44 (route priority), 45506f1 (VSO projection), merged via PRs #386–388. Initial ticket recovered as 4f9b8a9 in an isolated worktree.

### What I did
- Updated design: portal sandbox.slop.boutique and instance origins i-<id>.sandbox.yolo.scapegoat.dev. Both resolve via existing DNS. Separate portal HTTP-01 and wildcard DigitalOcean DNS-01 certificates; no Ramon DNS change needed.
- Created Terraform-owned project/client in the existing operator identity organization, with sandbox-only redirect/logout URLs and public PKCE. Plan exactly two additions; apply succeeded. No production client/policy changed.
- Created a named restricted test API key in the Slop Boutique account and moved it directly into Vault without outputting its value. Provisioned test product, $1 USD/month price matching the existing production test price, 14-event webhook and default test customer portal. Verified key authentication and livemode=false responses. No live Stripe requests or charges.
- Created standalone francois-sandbox package: separate DB/user/PVC, scoped VSO policies/roles, explicit test-mode flag, fake model backend, disabled email-link/signup flags and no production model credentials. Fonts and identity issuer/organization are intentionally shared; application data is not.
- Deployed observed production image sha-dff295d as the initial sandbox baseline. Production subsequently evolves independently; no automatic sandbox image promotion is claimed.
- Seeded validated sandbox settings through the running CLI: visible SANDBOX brand/header, no sampling, matching price/limits. No production users, subscriptions or content were copied.
- Gated portal and instance routes with Basic Auth; only exact /webhooks/stripe bypasses that gate and still requires Stripe signatures. Added noindex headers.
- Verified VSO healthy, database bootstrap complete, certificates Ready, deployment available, Argo Healthy/Synced. Unauthenticated portal returns 401, authorized portal 200 with SANDBOX/noindex, unsigned webhook 400. Browser screenshot visually reviewed. OIDC request reached hosted login; operator later confirms completed sign-in.

### Why
Separate persistence and credential custody protect real billing from test operations. Retaining a single-mode server avoids rewriting customer mappings and event processing. Existing storefront copy settings provide the visible test banner without a special application build.

### What worked
55-package GitOps validation and PR CI passed. Both certificates Ready. Dedicated test product prod_VUrdIdgaEDJm8G, price price_1UTrtzLpxiqBMHHiHvtBwWyX, webhook we_1UTrunLpxiqBMHHiGNMfI40a; portal configuration bpc_1UTs62LpxiqBMHHiO97ul0ZF is active, default and test-only. $1/month parity checked via the old test Price before creating the new equivalent. Operator confirmed sandbox login. Final production and sandbox Argo apps both Healthy/Synced.

### What didn't work
- Original ticket path disappeared after the other worktree switched/rebased to main; appending failed with FileNotFoundError. Recovered original doc-only commit into a separate worktree using cherry-pick; did not reset unrelated work. An overly broad discovery find timed out and was abandoned for git worktree/commit inspection.
- Browser initially lacked Stripe authentication; operator signed in and access succeeded on retry. Dialog snapshot selector matched two dialogs; scoped find identified the active workflow. Permission selection could not find the Webhook Endpoints cell by exact label because it included a docs link; used the observed Write control. No unrestricted/live key was used.
- Browser VM dynamic import of node:child_process failed with ERR_VM_DYNAMIC_IMPORT_CALLBACK_MISSING. Used a single-request loopback-only bridge to store the credential through Vault, with no body logging or credential files. The bridge closed after custody succeeded.
- First webhook provisioning treated the comma-separated event file as one event; Stripe HTTP 400: Invalid enabled_events[0]. Corrected comma parsing and reused stable idempotency keys; provisioning succeeded without duplicate products/prices.
- Initial portal probe returned 404 instead of 401. Added explicit route priority in PR #387, but the same probe still returned 404. Traefik then showed the actual cause: `failed to load basic auth credentials: found 2 elements for secret 'francois-sandbox/sandbox-ingress-auth', must be single element exactly`. VSO had included _raw alongside users despite excludes. PR #388 set excludeRaw=true; confirmed only users remains, then 401/200 behavior passed. The earlier priority hypothesis alone was insufficient. No further speculative fixes were attempted.
- Running the settings CLI with --output=json failed: unknown flag --output. Installed help specified --format=json; this worked. The initial database had no settings row, so seeded defaults plus sandbox-only overrides through the validated CLI rather than SQL-copying production settings.

### What I learned
The exact runtime CLI contract differs from older Glazed output conventions. VSO excludes does not suppress its raw payload unless excludeRaw is enabled. Explicit app routing and secret-shape validation matter even when Argo reports healthy. A shared identity organization does not imply shared application data.

### What was tricky to build
Secure browser-to-Vault credential custody; separate DNS/TLS boundaries without changing Cloudflare; preventing sandbox model/email side effects; and safely distinguishing restricted portal traffic from signed webhook access.

### What warrants a second pair of eyes
Full sandbox Checkout/entitlement/portal cancellation/refund regression remains unverified. The operator gate password was pasted into chat; the operator explicitly declined rotation, so no rotation was performed and the value is not repeated in these documents. Credential permissions include test provisioning operations; narrow those separately if a stricter runtime-only policy is desired. Do not claim a shared Stripe test account provides object-level isolation: it is a separate test-mode boundary with a dedicated key and tagged objects, not a Stripe account-per-deployment guarantee.

### What should be done in the future
Exercise actual sandbox Checkout and webhook-driven entitlement changes, portal/cancellation/refund cases and permission coverage; publish an explicit image promotion process. Before live cutover, review all remaining ticket gates and obtain real-money test approval. Shared pg_dumpall covers the database; sandbox blob data is disposable without a dedicated object backup.

### Code review instructions
Review platform PRs #386–388, Terraform sandbox client, scripts 01–04, sources/runtime-provisioning.json, sandbox-settings.json and sandbox-home.png. Confirm no font binary, password or Stripe key is committed. The historical provisioning scripts are operator tools, not a daemon; review partial state before replaying.

### Technical details
Namespace/database: francois-sandbox / francois_sandbox. Vault runtime apps/francois/sandbox/runtime; key custody apps/francois/sandbox/stripe; gate apps/francois/sandbox/ingress. Stripe account acct_1TxEMNLpxiqBMHHi in test mode; API version 2026-06-24.dahlia. Readiness and TLS checked after deploy. Application-level payment completion is not yet claimed.


## Step 3: Execute the auth-domain cutover and restore serving availability

Migrated the shared identity hostname to auth.slop.boutique, preserving the ZITADEL instance and local user IDs. Stopped the active consumers for a quiesced PostgreSQL/Vault checkpoint and transactional issuer mapping updates. This deliberately caused an outage; the operator then requested deployments keep the old version serving. Slop was restored and returned HTTP 200 before pursuing deployment-policy changes.

The intended sandbox payment baseline has not been completed. This entry records identity infrastructure progress, not completion of login continuity or billing qualification.

### Prompt Context
**User prompt (verbatim):** "can slop.boutique be set up to no have "no available server"? always keep one running on the previous version"
**Assistant interpretation:** Restore availability immediately and make future deployments readiness-gated rather than stop-first.
**Inferred user intent:** Keep the public site serving through routine upgrades.
**Earlier execution authorization:** Compacted context records authorization to execute the pre-launch migration, commit at meaningful intervals, and keep a detailed diary; this is a summary, not a reconstructed quotation.
**Commit (code):** Platform cfe0ebb (PR #389) changes domain/active consumers; 7ab70f6 (PR #391) corrects ingress precedence. Both PRs merged.

### What I did
- Added the new /idps/callback URI to Google and GitHub OAuth clients, retaining the existing URI. GitHub persisted values independently reloaded; Google save returned to credentials listing, independent reload still pending.
- Added the new certificate SAN and changed ZITADEL ExternalDomain, login URLs, active consumer issuers and branding reconciler URL.
- Script 05 held reconciliation, stopped consumers/ZITADEL, ran successful PostgreSQL and Vault backup jobs, migrated only matching users. Receipt: sources/issuer-cutover.json.
- PostgreSQL preserved 10 production users (5 old-issuer rows updated), and 1 sandbox user (1 updated). Agentlogic had 0 users; Datadrop preserved all 3 IDs/subjects. SQLite snapshots reside on their respective PVCs and passed integrity checks; these are local rollback copies, not off-cluster backups.
- Resumed all four active consumers. Restored prereqs reconciliation and branding CronJob scheduling.
- Explicitly updated the persisted instance LoginV2 feature through PUT /v2/features/instance; independent GET confirmed the new baseUri. Helm DefaultInstance configuration alone did not update this persisted value.
- Added task wqph for a ready-gated serving handoff; no deployment strategy change claimed.

### Why
Local account continuity depends on updating exact (issuer, subject) mappings, not matching email addresses. The original migration accepted downtime, but the operator now wants deployments to preserve serving capacity.

### What worked
- GitOps invariant checks: all 55 packages, zero violations; both platform PR CI validations passed.
- Successful checkpoint objects: postgres/postgres-20261024T000201Z.sql.gz and vault/vault-20261024T000202Z.snap.gz. Backup jobs completed; this new checkpoint has not undergone an independent restore drill.
- New discovery returns issuer https://auth.slop.boutique with valid HTTPS; existing admin token can read/update the original instance.
- kubectl -n francois rollout status deployment/francois completed; curl https://slop.boutique/ returned HTTP 200 after resuming reconciliation.

### What didn't work
- Initial `curl -fsS https://auth.slop.boutique/.well-known/openid-configuration` returned `curl: (22) The requested URL returned error: 503`. Traefik access logs identified `websecure-francois-francois-slop-boutique-a5904d09de3219f21490@kubernetes`: the wildcard Slop router won over exact ZITADEL routes, hitting the intentionally stopped application. Explicit API priority 300 / Login priority 310 fixed it; discovery then succeeded.
- `kubectl -n agentlogic exec deployment/agentlogic -- sh ...` and the corresponding Datadrop command failed with `exec: "sh": executable file not found in $PATH`. Inspected upstream migration schemas instead and used short-lived Python Jobs mounting each PVC for backed-up transactions.
- A two-block edit was rejected because one oldText matched twice; no change was applied. Retried with unique newline/indentation contexts.
- Existing instance feature readback still showed the old LoginV2 baseUri after Helm rollout. Corrected through the instance feature API and verified readback.

### What I learned
A ready identity backend is insufficient if a wildcard ingress wins routing. Also, the serving application is explicitly configured Recreate because views, rooms, worker and asset-signing keys are process-local; rolling overlap is not safely solved by changing only one YAML field. The initial promise to change strategy must be qualified by this discovered constraint.

### What was tricky to build
Migration changes span Helm bootstrap/runtime values, persisted instance features, exact local account mappings, OAuth allowlists and ingress precedence. Public availability needed restoration before expanding deployment redesign. The cutover helper intentionally leaves reconciliation held until the separate GitOps transition; it is not a safe blindly replayable reconciler.

### What warrants a second pair of eyes
Review rolling/blue-green handoff against process-local state, per-process signing keys, shared writable worker/Pi paths, PVC scheduling and schema backward compatibility. Do not treat maxUnavailable=0 as proof of application correctness. Verify actual Google/GitHub/email login and stable local IDs after migration; HTTP 200/discovery are not end-to-end authentication evidence.

### What should be done in the future
Implement ready-gated handoff (wqph); complete login/account continuity checks, independent callback readbacks, Terraform issuer-reference updates, billing inventory and sandbox purchase/webhook/portal qualification before live Stripe. No real charges or billing cleanup were performed.

### Code review instructions
Review scripts/05-issuer-cutover.py and its receipt, platform PRs #389/#391, deployment strategy and pkg/catalog/hosted/runtime.go. Recheck application health and new-host login flows. Preserve the rollback snapshots until validation is complete.

### Technical details
Active consumers: francois, francois-sandbox, agentlogic and datadrop. Todo deployments remain retired at zero replicas and were not reactivated or migrated. Current production strategy remains Recreate with one replica. The new desired behavior is to keep the serving predecessor until its replacement is ready; implementation remains open.


## Step 4: Exercise sandbox payments end to end, including refund and cleanup

The operator confirmed the login flows work. Using the existing authenticated sandbox account and an origin-scoped Basic Auth browser context, exercised real hosted Stripe test Checkout and the customer portal. All tested business flows passed on deployed image sha-dff295d. No production credentials, real cards, live payments, production settings or application code were changed.

Finished with both test subscriptions canceled, the year payment fully refunded, the abandoned Checkout expired, and the account displaying Free. Test payment history remains intentionally available for audit; the two monthly test charges were not refunded. This qualifies the listed paths, not every possible payment method, failure or renewal scenario.

### Prompt Context
**User prompt (verbatim):** "login flows work. Test the sandbox checkout stuff and all."
**Assistant interpretation:** Accept operator login confirmation and test purchase, access, webhooks, portal and cancellation/refund behavior in the isolated sandbox.
**Inferred user intent:** Establish concrete billing confidence before live cutover without charging real money.

### What I did
- Confirmed every hosted payment page displayed Sandbox/Test mode; API mutations required an rk_test_ key and fetched objects with livemode=false before modification.
- Individual support: selected $1/month, submitted declined test card 4000000000000002, saw the decline and independently verified account remained Free. Retried with 4242424242424242; Checkout returned to the sandbox with $1/month support and the next-payment date.
- Verified processed checkout.session.completed, customer.subscription.created and invoice.paid rows plus an active test-only subscription.
- Customer portal opened correctly, showed the paid invoice and test payment method. Scheduled cancellation; account retained support until period end. Used Don't cancel subscription → Renew subscription; API and account confirmed cancellation removed.
- Verified active-support pricing replaces the monthly purchase buttons with Manage subscription. This is UI duplicate prevention, not a direct adversarial API test.
- Canceled the exact individual test subscription through Stripe's test API, processed deletion webhook, and verified Free access.
- Enterprise support: bought the default $256/month test offering; collected a fictitious billing address. Database projected support_tier=enterprise and support_amount_cents=25600.
- Year funding: paid the displayed $999,999.99 test-only one-time amount. Account showed one year through 24 October 2027, no automatic renewal, and correctly warned that enterprise monthly support remained active.
- Refunded that exact year payment in the Stripe test dashboard. charge.refunded processed, checkout payment_state became refunded, year access disappeared, and enterprise access remained.
- Canceled the exact enterprise test subscription and verified final Free access.
- Started another $1 test Checkout, returned without paying, verified Free, and started again: the same open Checkout was reused. Expired that session using Stripe's test API; checkout.session.expired processed.
- Script 06 replayed an actual signed test event (HTTP 200), rejected an invalid signature (400), and rejected a signed wrong-livemode event (400). Final DB checks showed one original event row, zero wrong-mode rows, and no pending/error events.
- Ran `go test ./pkg/catalog/payments ./pkg/catalog/billing ./pkg/catalog/store`: payments and billing PASS; store reports no test files, not an integration-test pass.

### Why
A redirect alone is insufficient: payment events must project access, failed/abandoned payments must not grant access, cancel-at-period-end must preserve the paid period, and refunds must remove only the affected entitlement. All transitions were observed through the deployed browser/API/database boundaries.

### What worked
All paths listed above passed. The durable sanitized receipt is sources/sandbox-billing-qualification.json, produced by script 07; webhook HTTP outcomes are sources/sandbox-webhook-checks.json. Final projections: two canceled test subscriptions, three completed purchases (one year payment refunded), one expired abandoned Checkout, and all 18 recorded event rows processed without errors (including four account-refresh events).

### What didn't work
- Stripe's card accordion button is overlaid: normal Playwright click timed out with `element is not visible`; clicking the Card text was intercepted by the accordion button. Dispatching click on that observed button expanded its normal UI. No CAPTCHA was solved or bypassed.
- A portal locator guessed Renew subscription on the overview and timed out: actual overview link is Don't cancel subscription, followed by a Renew subscription confirmation button.
- Enterprise is role=tab, not button; corrected the locator after inspecting the actual element.
- Enterprise Checkout collected a full address, so the simple ZIP accessible-name locator failed. Expanded Enter address manually and filled the observed address fields.
- A read-only GET /v1/charges using the restricted runtime key returned HTTP 403 / more_permissions_required. Did not broaden the key. Performed the test refund through the authenticated Stripe dashboard; the deployed refund webhook processed successfully with current runtime permissions.
- Initial SQL probes used historical names: `ERROR: column "subscription_id" does not exist` and `ERROR: column "plan" does not exist`. Current columns are stripe_subscription_id and checkout_attempts.price. Inspected the deployed information schema before producing the final receipt. Script 07 now preserves SQL stderr on failure.

### What I learned
Current sandbox checkout is custom-amount support, enterprise support and year funding—not merely the initially provisioned fixed $1 plan. Each offered mode needs qualification. Year funding intentionally coexists with an existing monthly subscription; its refund must not erase that subscription's access.

### What was tricky to build
Stripe Checkout has different billing fields for the enterprise flow, and some controls use hidden overlay buttons. Portal wording differs between the overview and confirmation screen. Keeping secrets in memory and writing only bounded receipts avoids archiving Basic Auth credentials, raw Checkout/portal session URLs or complete event payloads.

### What warrants a second pair of eyes
Still untested: 3DS challenges, time-advanced renewal/dunning, partial refunds, non-card methods, payment-method replacement and adversarial cross-account Checkout access. Existing unit tests passed but are not proof those paths work in the deployed environment. Operator reports login flows working; no claim of independent browser verification of every login provider is made here.

### What should be done in the future
Inventory production test billing and prepare the separately authorized live transition. Retain the explicit real-money approval gate. Ready-gated deployment handoff remains open and was not changed during payment testing.

### Code review instructions
Review scripts 06–07 and their sanitized JSON receipts. Re-run script 07 only while this qualification state is still current; its assertions intentionally expect these test subscriptions canceled. Script 06 signs a synthetic wrong-mode probe and replays a real test event, so it is not read-only. Browser observations are recorded above; no production mutation is part of this checkpoint.

### Technical details
Test account acct_1TxEMNLpxiqBMHHi, deployment francois-sandbox/francois, database francois_sandbox, image ghcr.io/go-go-golems/francois:sha-dff295d. Individual subscription sub_1UTzsoLpxiqBMHHiwGVXsupk and enterprise sub_1UTzyZLpxiqBMHHiSJo1SinC are canceled. Year payment pi_3UTzzRLpxiqBMHHi12FVCwSX is fully refunded. No live billing cutover was performed.


## Step 5: Prepare verified live Stripe objects without switching production

Inventoried current production test billing, captured an online database backup, and prepared live Stripe objects in the approved Slop Boutique account. The operator completed passkey verification; the new restricted live key was transferred directly from browser memory to Vault. Production runtime credentials, database rows, settings and mode remain unchanged. The new live webhook is disabled pending coordinated cutover.

### Prompt Context
**User prompts (verbatim):** "ok, go ahead."; "ok pass key is good."; "you can read the key now"; "ok, do it"
**Assistant interpretation:** Inventory production and prepare live prices, portal and webhook configuration; capture the key after operator verification, without a real charge.
**Inferred user intent:** Advance to live billing, minimizing additional planning while respecting the explicit destructive-cleanup and real-money approval gates.

### What I did
- Production inventory: 4 billing customer mappings, 2 active test subscriptions, 7 checkout attempts (2 completed / 5 expired), 229 test events (222 processed / 7 ignored). No live subscription/event rows observed.
- Existing test subscriptions: $45/month custom support and the legacy fixed-price plan. Legacy Stripe price independently verified as $1 USD/month. Settings revision 11 has free/subscriber plans with 1000 instances each, daily tokens 10,000 / 500,000 and members per instance 1 / 10. These current values supersede historical inventory assumptions; no limits changed.
- Confirmed production's old test account is acct_1TxEMcLeHmcfmvuC; prepared live objects in acct_1TxEMNLpxiqBMHHi, the Slop Boutique account already used in test mode by the sandbox.
- Created restricted key francois-live-runtime. Custody: kv/apps/francois/prod/stripe-live, not the mounted runtime path. Enabled Customers, Products, Prices, Subscriptions, Customer Portal, Checkout Sessions and Webhook Endpoints write; Events and Charges/Refunds read; added Accounts read for capability verification. No payout/refund-write permission granted.
- Verified live account ID, USD/US, charges_enabled=true, payouts_enabled=true and card_payments=active.
- Script 09 provisions a $1/month live price preserving the fixed plan's amount/currency/cadence; custom individual/enterprise/year amounts remain generated by the application unchanged.
- Prepared default live portal with invoice history, payment method updates and period-end cancellation, returning to https://slop.boutique/account.
- Prepared live webhook https://slop.boutique/webhooks/stripe, API 2026-06-24.dahlia, all 14 repository events; disabled it after creation and verified the final disabled state.
- Successful online checkpoint: postgres/postgres-20261024T082503Z.sql.gz. This is not a new restore drill or a quiesced final cutover checkpoint.

### Why
Test customer IDs and entitlements cannot remain in production's active single-mode billing tables when switching Stripe accounts/modes. Live configuration can be prepared without exposing new checkout admission or touching those records.

### What worked
Script 09 succeeded and read back the live price, default/active portal and disabled live webhook. Every created resource ID is checkpointed in Vault; the public receipt sources/live-stripe-preparation.json contains no credentials. Passkey verification was performed by the operator, not bypassed.

### What didn't work
- A cookie banner intercepted Continue; rejected optional cookies and retried normally.
- Stripe initially displayed `Verification required` and `We encountered some problems. You might be able to authenticate using a different option.` Operator completed the passkey step before custody proceeded.
- GET /v1/account returned 403 requiring `Accounts Read ('connected_account_read')`. Added precisely that read permission and repeated the request successfully. No broader account-write capability was added.

### What I learned
Production is now sha-1317326, whereas the completed sandbox qualification used sha-dff295d. Diff includes 01e0617, removing enterprise-specific address/tax collection and naming its Checkout product Monthly support, plus unrelated release validation/studio UI changes. The latest production image must be promoted to sandbox and its affected checkout behavior requalified before cutover; do not silently claim artifact parity.

### What was tricky to build
Live custody required operator passkey action and a short-lived loopback bridge. Configuration preparation must not update the mounted runtime or enable an endpoint against a still-test-mode receiver. First live portal configuration was confirmed is_default=true rather than assuming programmatic creation selects it.

### What warrants a second pair of eyes
Script 09 briefly creates the endpoint enabled before immediately disabling it (the create API does not use the update-only disabled switch); no live customers/payments were created. Replays must inspect saved resource IDs and current intent: this preparation script deliberately disables its endpoint and must not be run after launch. It also asserts the production key is still test-only. Do not commit Vault contents or run with shell tracing.

### What should be done in the future
Obtain scoped approval to archive the 4 mappings, 2 subscriptions, 7 attempts and 229 events out of active production billing tables and remove their test-derived benefits, preserving users/content/usage/history. Then qualify the exact release, take a final checkpoint and coordinate the actual live cutover. Real-money smoke test remains a separate explicit authorization. Deployment availability handoff remains open.

### Code review instructions
Review scripts 08–09 and sources/live-stripe-preparation.json; check mode/account assertions, Vault CAS checkpoints, price parity and final webhook disabled state. No production rollout or cleanup is included in this commit.

### Technical details
Live product prod_VV0JkispMzlf9y; price price_1UU0JILpxiqBMHHixeXUFilX; portal bpc_1UU0JNLpxiqBMHHiMnAdY50K; webhook we_1UU0JLLpxiqBMHHi0cOkQRDv. Runtime remains test-only and production remains available; no charge or customer was created by preparation.


## Step 6: Match the production image and qualify the changed enterprise Checkout

Updated only sandbox to sha-1317326 and reran enterprise Checkout with a Stripe test card. The simplified flow succeeded, the enterprise entitlement remained correct, and the test subscription was canceled afterward. Production was not restarted, switched live or cleaned up.

### Prompt Context
**User prompt (verbatim):** "ok"
**Assistant interpretation:** Proceed with the just-explained focused version-matched sandbox check; not a new authorization for destructive production cleanup.
**Inferred user intent:** Verify the small checkout change and advance toward cutover.
**Commit (code):** Platform 4db0f7a — sandbox image alignment; PR #393 merged after CI passed.

### What I did
- Updated sandbox image from sha-dff295d to sha-1317326 through GitOps; waited for readiness.
- Verified running production and sandbox imageIDs (resolved digests) match, not just image tags.
- Started enterprise support at $256/month. Stripe displayed Monthly support and only the ordinary card/name/country/ZIP fields; no enterprise-only full address or tax-ID field.
- Completed Checkout using test card 4242424242424242. Database showed active, livemode=false, amount 25600 cents, tier enterprise. checkout.session.completed, customer.subscription.created and invoice.paid were processed without errors.
- Account displayed Enterprise support, 256 USD a month.
- Canceled only test subscription sub_1UU0NoLpxiqBMHHiFzQy8sAP and verified the account returned to Free.

### Why
The first qualification used an older artifact. This focused check covers the enterprise Checkout behavior changed since then without repeating the entire earlier matrix.

### What worked
All 55 GitOps packages passed invariant validation, PR CI passed, rollout completed, deployed image digests matched, Checkout and entitlement projection succeeded, and cleanup restored Free access. Receipt: sources/current-image-enterprise-qualification.json.

### What didn't work
No new failures observed.

### What I learned
The customer-facing product name is now Monthly support for both tiers; the enterprise metadata still correctly drives application entitlement and account presentation.

### What was tricky to build
Distinguishing harmless product-name simplification from accidentally losing enterprise tier metadata. Checked both the hosted page and database projection.

### What warrants a second pair of eyes
The earlier broad matrix remains evidence for the older artifact; this is specifically the changed enterprise flow plus digest parity, not a claim that every scenario was rerun on the new image.

### What should be done in the future
Proceed with the separately scoped production test-data archival approval and coordinated live cutover; no real-money smoke test is authorized by this check.

### Code review instructions
Review platform PR #393 and the new image qualification receipt. Production deployment, credentials, settings and billing rows were untouched.

### Technical details
Both deployments now run sha-1317326 with matching resolved image digests. The live Stripe configuration remains staged; live webhook remains disabled pending cutover.
