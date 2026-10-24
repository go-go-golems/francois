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
