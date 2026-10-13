---
Title: Subscription service investigation diary
Ticket: FRANCOIS-002
Status: active
Topics:
    - backend
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://deploy/compose/compose.yaml
      Note: End-to-end acceptance stack (ZITADEL, Stripe CLI, Pi)
    - Path: repo://pkg/catalog/catalog_test.go
      Note: PostgreSQL integration test of the hosted platform
    - Path: repo://pkg/catalog/hosted/routes.go
      Note: Launch exchange, grant-gated page injection, assets
    - Path: repo://pkg/catalog/hosted/sample.go
      Note: Model allowlist, admission, reservation and settlement
    - Path: repo://pkg/catalog/payments/service.go
      Note: Checkout attempts, webhook ingest and reconciliation worker
    - Path: repo://pkg/catalog/server.go
      Note: Host dispatch and the single hosted-access rule (authorizeInstance)
    - Path: repo://pkg/catalog/store/usage.go
      Note: Allowance ledger (reserve/start/settle/recover)
    - Path: repo://ui/catalog/instances.tsx
      Note: Launcher frame and parent side of the launch handshake
ExternalSources: []
Summary: Evidence and decisions for the proposed subscriber HTML distribution service; implementation remains open.
LastUpdated: 2026-10-12T21:18:58.719369-04:00
WhatFor: Preserve source research decisions validation failures and delivery evidence.
WhenToUse: Resume the subscription portal design or begin its implementation.
---

# Subscription service investigation diary

## Step 1: Establish scope and inspect prior identity and billing work

The user requested a new subscription service design grounded in prior work in the go-go-parc vault. I created FRANCOIS-002, preserved five relevant vault reports, inspected the available ZITADEL TODO billing source, and wrote an intern guide with a separate distribution-server boundary. No account, Stripe catalog, production deployment, or subscriber mailing was created.

The first-release design sells authenticated downloads with opted-in release notifications. It leaves the local Pi runtime on each subscriber's machine. An optional scope question about hosted applications/LLM calls was asked early; this guide marks distribution as the working assumption rather than pretending an unanswered question establishes approval for a hosted model service.

### Prompt Context
**User prompt (verbatim):** "Add user management, SSO, stripe. Look at go-go-parc to see what we did in the past. We want to set up a subscription service where people can subscribe to receive and download such html files. Create a new docmgr ticket, Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."
**Assistant interpretation:** Deliver an evidence-backed design and phased implementation handoff for subscriber accounts, SSO, Stripe billing, HTML release publication/downloads, and receiving releases. Upload the finished guide to reMarkable.
**Inferred user intent:** Extend the project into a commercially usable HTML distribution service while reusing proven identity and subscription patterns.

### What I did
- Read ticket-research/docmgr/diary/textbook/reMarkable instructions and repository guidelines.
- Created ticket FRANCOIS-002 with design and diary documents via the installed docmgr CLI.
- Searched go-go-parc for Stripe, SSO/OIDC, subscriptions, and artifact distribution; selected the explicit Stripe report, subscription architecture playbook, hyperblog, Fortress, and serve-artifacts reports.
- Copied those five reports into sources and recorded original paths, bytes, SHA-256 hashes, Francois revision, and inspected predecessor revision in sources/manifest.json.
- Inspected actual prior code in /home/manuel/code/wesen/2026-07-25--zitadel-go-test, especially internal/billing/stripe.go, PostgreSQL event processing, and cmd/todo-demo OIDC startup/session persistence.
- Verified relevant current API references using primary Stripe, ZITADEL, OpenID, and AWS documentation. Stored annotated links rather than copying complete external pages.
- Authored a roughly 10,000-word guide with orientation, representative user journey, current-state evidence, product contract, user/SSO/session design, billing convergence, release/download/email contracts, schema sketches, package layout, HTTP/API examples, eight implementation phases, tests, and owner choices.
- Added a ticket validation script and related six focused implementation/source files to the guide using docmgr doc relate.

### Why
The current host has one durable owner, loopback listeners, private Pi settings, and browser tools. Relabeling its owner UUID as a subscriber identity would not create a safe or comprehensible subscription website. A separate catalog command and origin meet the distribution use case without public model execution or multi-user runtime migration.

The prior TODO project supplies real Checkout/Portal/webhook code and sandbox acceptance history. hyperblog supplies the server-side byte-withholding rule. Fortress supplies identity/permission separation, while serve-artifacts supplies release storage and operations lessons. They are references, not a reason to introduce their whole feature sets or import internal packages illegally.

### What worked
- docmgr created the ticket and both documents in the expected workspace.
- Actual predecessor source exists for Stripe billing, PostgreSQL projection, OIDC login, and session persistence.
- The guide validation script passed: 20 numbered sections, balanced code fences, all five source hashes, actual current/predecessor file references, and explicit proposed-design status.
- Six focused RelatedFiles were added through docmgr.
- Existing application code and prior repositories were not modified during research. The predecessor has an unrelated dirty ticket changelog; it was preserved.

### What didn't work
- A probe for `.docmgr.yaml` failed with `cat: .docmgr.yaml: No such file or directory`. `docmgr status --summary-only` identified the actual `.ttmp.yaml` configuration and ttmp root.
- The hyperblog report's source checkout was absent: `rg: /home/manuel/workspaces/2026-07-30/transcript-agent/hyperblog/pkg: IO error ... No such file or directory`. Used its preserved report and labeled the source-level verification limitation.
- A guessed prior `internal/auth/*.go` directory was absent. `rg --files` located OIDC startup in cmd/todo-demo/serve.go and persistent sessions in cmd/todo-demo/session_store.go and internal/store/postgres/sessions.go.
- The browsing tool could not open the direct Stripe portal-session API URL; the official Customer Portal integration guide supplied the endpoint contract.
- The first guide validator used the wrong repository ancestor and failed with `AssertionError: cmd/francois/cmds/commands.go`. Corrected the root from parents[3] to parents[4]; the same validation then passed.

### What I learned
The prior TODO policy grants past_due access and stores one projected subscription per user. Paid file distribution needs its own explicit policy and must avoid a delayed canceled subscription overwriting a replacement. The guide proposes strict new-download access and subscription-ID records with current Customer-level reconciliation.

Email delivery also has a different authority from billing and identity mail: release notifications use opted-in verified contacts, Stripe sends billing communications, and ZITADEL handles identity verification. A durable email job does not imply exactly-once delivery at the provider.

### What was tricky to build
The most important boundary is between receiving application bytes and executing them. Downloaded applications can require the local Claude API runtime and model credentials, but that does not imply they should execute on the subscriber site's authenticated origin. The design gives them private attachment delivery and safe catalog screenshots, with no same-origin arbitrary HTML preview.

Current-state Stripe retrieval, duplicate event IDs, and subscription-row identity solve different failure modes. All three are explained separately. Similarly, CLI release publication and local HTML update are distinct product operations; ordinary local edits must not automatically notify subscribers.

### What warrants a second pair of eyes
Review the product scope assumption, paid-file status policy, replacement-subscription convergence, disabled-account access to its own billing management, and publication/notification crash boundaries. Confirm pricing, trial/grace, download catalog policy, sender, domain, licensing, and deployment settings before live launch.

### What should be done in the future
Implement the eight phases tracked as open tasks. Resolve the earlier known toolchain/dependency vulnerability findings before production. No live launch, billing resource provisioning, or actual notification mailing is part of this design deliverable.

### Code review instructions
Read the guide's Sections 1–5 first for orientation and evidence. Review Section 8 with the predecessor's internal/billing/stripe.go and PostgreSQL billing.go; review Section 11 for paid-byte and origin boundaries. Run scripts/01-validate-guide.py and docmgr doctor before editing or delivering an updated guide.

### Technical details
- Baseline code: bd373eb; preserved prior report sources and inspected source revision are in sources/manifest.json.
- Proposed execution: francois catalog serve; no dependency on pkg/host, Node, or ~/.pi/agent.
- Storage: PostgreSQL account/catalog/work records plus a private immutable release directory, distinct from local SQLite state.
- Delivery: opted-in email release-page links, authenticated attachment downloads, customer-owned local runtime.
- Source and diagram/API contracts in the guide distinguish existing files from proposed package paths and commands.


## Step 2: Validate and deliver the intern guide

The finished guide contains 20 sections and roughly 10,500 words, including three ASCII relationship/sequence diagrams that remain readable in the PDF export. It covers all requested domains and leaves implementation phases open. A delegated upload finalizer followed the reMarkable specialist policy while the guide was frozen; the uploader returned a successful cloud-delivery result.

### Prompt Context
See Step 1 for the verbatim request. This step completes the requested ticket documentation and reMarkable upload.

### What I did
- Manually reviewed orientation, separation of existing/proposed behavior, policy assumptions, current-state Stripe convergence, and file/notification boundaries.
- Ran scripts/01-validate-guide.py, docmgr doctor, and git diff --check.
- Uploaded the guide as a PDF bundle with a two-level table of contents; no overwrite flag was used.
- Recorded the frozen guide SHA-256 and cloud upload receipt in sources/guide-delivery.json.
- Marked research/delivery tasks complete while leaving all eight implementation phases open.

### Why
A design handoff needs actual source evidence and a verifiable delivery boundary. Completing the guide does not establish subscriber functionality or a live launch.

### What worked
- Validator: PASS with 20 numbered sections, balanced fences, all five source hashes, and existing-code references.
- docmgr doctor: all checks passed.
- Exact delivery result: `OK: uploaded FRANCOIS-002 Subscription Service Intern Guide.pdf -> /ai/2026/10/12/FRANCOIS-002`.
- The uploaded input was unchanged after handoff; its hash is retained with the receipt.

### What didn't work
No upload failure or overwrite conflict was reported. Physical tablet synchronization was not checked; the receipt establishes cloud upload only.

### What I learned
The prior implementation references were sufficient to keep the new design small: one distribution service and database, private files, external identity/billing, and durable in-process workers.

### What was tricky to build
The guide distinguishes notification job deduplication from exactly-once email delivery, and subscription event deduplication from current-state convergence. Those distinctions remain explicit in the phased acceptance criteria.

### What warrants a second pair of eyes
Confirm commercial and deployment choices listed in Section 19 before implementation reaches live services. Review the distribution-only assumption if hosted application/LLM execution is desired.

### What should be done in the future
Start Phase 1 using this guide. No existing runtime or vault history was changed by the ticket research.

### Code review instructions
Open the design doc from the ticket index and run its validation script. Review sources/manifest.json for provenance and sources/guide-delivery.json for the exact delivered document identity.

### Technical details
Remote destination: /ai/2026/10/12/FRANCOIS-002. PDF name: FRANCOIS-002 Subscription Service Intern Guide.pdf. Delivery preserves the normal no-overwrite policy; no physical-device synchronization claim is made.

## Step 3: Revise the product for hosted capabilities and an administrator UI

The user clarified that file distribution alone is insufficient: the service must provide hosted db, room, user, and LLM behavior, followed by an explicit administrator interface for uploading HTML and managing users/settings. Revision 2 updates the existing guide and scope rather than pretending the original distribution assumption was accepted. The initial cloud PDF remains historical and will not be overwritten.

The new design uses release-pinned instances with private defaults, explicit subscriber membership, isolated browser origins, scoped launch/runtime grants, server-owned provider configuration, and per-caller model allowance. A trusted admin UI manages publishing, accounts, instances, billing/usage, and validated nonsecret settings. No hosting or administration code was implemented in this documentation revision.

### Prompt Context
**User prompt (verbatim):** "we should actually provide hosting, i.e. db / room / user / LLM functionality. How would that work?"
**User follow-up (verbatim):** "we also want an admin side of things, to upload new html, manage users, settings, etc..."
**Assistant interpretation:** Correct the product scope to include hosted runtime capabilities and a real management interface, and explain/design the ownership and lifecycle needed to implement them.
**Inferred user intent:** Subscribers should use the HTML applications directly on the service while administrators operate the product, rather than requiring every subscriber to install a local runner.

### What I did
- Re-read current host RPC, view bootstrapping, user/presence code, and model protocol to identify global-owner assumptions.
- Replaced distribution-only conclusions with hosted instances, while preserving local commands and optional downloads.
- Added instance/private-document/room/member ownership, per-instance virtual origins, browser-bound launch grants, quota-controlled Pi execution, usage/start events, and revocation behavior.
- Added required admin screens and APIs for upload/draft/publish/withdraw, user/session management, instance inspection, billing reconciliation/usage, and settings revisions/audit.
- Updated proposed schema, package layout, process graph, API table, deployment requirements, acceptance tests, and implementation plan to nine phases.
- Preserved the original reMarkable receipt as revision 1 tied to b294a00. The new upload uses a revision-2 document name and separate receipt.
- Updated the ticket index/tasks and strengthened its document validator with hosted/admin requirements and phase order.

### Why
A single local Host.Owner cannot identify several subscribers. Scoping by release/artifact alone also merges different users' data and rooms. Instance identity is needed independently of user identity, while a trusted principal establishes who may act within it. Arbitrary HTML must not execute under the admin/account origin.

Pi's provider runtime remains a useful implementation, but the service operator supplies its credentials and pays for hosted inference. Subscription admission therefore needs a model allowlist, transactionally reserved call allowance, bounded concurrency, and observable usage. The current worker discards per-round SDK usage; this is an explicit implementation gap.

### What worked
- Existing source confirms where view context, query ownership, private-user paths, user profiles, and peer identity must change.
- The guide retains the existing browser capability contract and browser-side tool execution rather than introducing a separate arbitrary-code server executor.
- The revised process remains one Go service plus one Node worker/database, with origin boundaries and scoped data rather than a container or microservice per application.
- The validator checks 20 sections, nine implementation phases, required hosted/admin concepts, reference paths, source hashes, and absence of selected obsolete distribution-only conclusions.

### What didn't work
No new runtime failure was encountered because this step changes documentation only. The initial distribution assumption was incorrect for the intended product and is now explicitly superseded.

### What I learned
Per-instance origin isolation and runtime authentication are separate requirements. A cross-site iframe must not depend on third-party cookies; the guide proposes a controlled parent/bootstrap exchange with a one-use browser-bound code and instance-scoped grant. postMessage alone is not identity proof.

### What was tricky to build
The application can act through its permitted instance capabilities, but must not inherit the portal's billing/admin authority. The guide separates the portal session, launch exchange, instance grant, view identity, and server provider credentials. Model allowance also requires a clear distinction between reservation, provider-start ambiguity, and terminal settlement.

### What warrants a second pair of eyes
Review launch protocol source/origin/verifier binding, paid instance-owner/member access, role and private-user-data policy, subscription revocation, model accounting under crash/cancellation, and settings changes. These are proposed contracts with acceptance tests, not verified deployed behaviors.

### What should be done in the future
Implement the revised nine phases. Set allowance amounts, allowed models, storage/instance limits, sharing/cancellation policy, domains/TLS, sender and commercial settings before launch. Use the admin UI and CLI through the same domain services.

### Code review instructions
Read revised Sections 1–5 for the hosted product, Sections 13–15 for schema/packages/APIs, and Phases 3 and 6 for runtime/admin/model work. Compare the guide against b294a00 to see which initial assumptions were removed.

### Technical details
- Instance documents: instance_id + path; private data additionally requires verified UserID.
- Presence: instance_id + room; sameTab compares views, isMe compares users.
- Runtime grant: scoped user/instance/origin, bounded lifetime, server subscription/ACL rechecks, no portal/admin authority.
- Hosted LLM: operator Pi auth, allowed model policy, caller allowance reservation, per-round usage, browser tools.
- Admin mutations: session role + CSRF + audit; uploaded HTML executes only on an application origin.


### Revision 2 delivery checkpoint

The 14,176-word hosted/admin guide passed its validator and docmgr doctor. The cloud upload result is `OK: uploaded FRANCOIS-002 Hosted Platform and Admin Intern Guide v2.pdf -> /ai/2026/10/12/FRANCOIS-002`. The original PDF was not overwritten. Its new hash and revision are in sources/guide-delivery-v2.json. Physical tablet synchronization was not verified. All nine implementation phases remain open.

## Step 4: Implement the hosted platform core, admin API and subscriber UI

This step turns the revision-2 design into running code. I added a separate `francois catalog` command group whose server owns one PostgreSQL database, a private release directory, a trusted portal origin and isolated `i-<id>.<app-domain>` instance origins dispatched by exact `Host`. The existing local host (`pkg/host`, SQLite, loopback ports) is unchanged apart from shared helper extraction.

The integration test drives the real handler graph against PostgreSQL through admin upload/publish, subscriber denial, launch exchange, instance isolation, sharing roles, private namespaces, room identity, member revocation, a last-credit race and logout revocation; it passes. OIDC, Stripe and the browser UI compile and smoke-test, but have not yet been exercised against a real ZITADEL, Stripe sandbox or browser — that is the next step.

### Prompt Context
**User prompt (verbatim):** "Implement FRANCOIS-002 as a hosted subscriber platform, including the React/TypeScript UI using Redux Toolkit and RTK Query. Subscribers should sign in through ZITADEL, subscribe through Stripe, and launch release-pinned application instances with persistent db, shared room, real user identity, and quota-controlled Pi-backed LLM calls. Instances are private by default, with explicit sharing permissions, and execute on separate origins from the trusted account/admin site. Build the subscriber catalog, account/billing pages, instance launcher, and admin screens for HTML upload → draft → preview → publish, user/access management, instances, billing/usage, and validated settings. Follow the existing minimalist styling. Keep provider credentials server-side and enforce permissions and allowances in backend operations.

Start with the intern guide (ttmp/2026/10/12/FRANCOIS-002--subscriber-accounts-sso-stripe-billing-and-html-release-distribution/design-doc/01-subscription-service-analysis-design-and-intern-implementation-guide.md), especially Sections 1–5 for architecture, 13–15 for schemas/packages/APIs, and 17–18 for implementation phases and acceptance tests. Follow the task list (ttmp/2026/10/12/FRANCOIS-002--subscriber-accounts-sso-stripe-billing-and-html-release-distribution/tasks.md) and read Step 3 of the diary (ttmp/2026/10/12/FRANCOIS-002--subscriber-accounts-sso-stripe-billing-and-html-release-distribution/working-note/01-subscription-service-investigation-diary.md): it supersedes the original distribution-only scope. The design is committed at 07e8677; hosting/admin implementation remains open. Reuse the existing browser runtime and Pi worker, replacing global-owner assumptions with authenticated instance context. Deliver working slices, validate the UI in a browser, commit at meaningful milestones, and maintain the detailed ticket diary."

"commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

"Use as a guideline for aesthetics and style http://html.moldandyeast.com/ (we're basically building the subscription version of it)."

**User follow-up (verbatim, mid-step):** "create a docker compose setup to test the whole thing, btw"

**Assistant interpretation:** Build phases 1–6 as working slices (catalog boundary, OIDC sessions, admin publishing + hosted instances, Stripe Checkout/Portal, durable webhooks, metered Pi sampling), with a React/RTK Query portal and admin UI styled after html.moldandyeast.com, plus a Compose stack for end-to-end testing.
**Inferred user intent:** A runnable subscription version of the moldandyeast HTML index where paying users run curated HTML apps with hosted capabilities, operated through a real admin UI.
**Commit (code):** 495066f — "Add hosted subscriber catalog: PostgreSQL store, instances, launch grants, admin API and UI"

### What I did
- Read guide Sections 1–20, tasks, diary Step 3; read `pkg/host/{server,routes,rpc,rooms,assets,comments}.go`, `pkg/store/*`, `pkg/piworker/*`, `ui/src/runtime*.ts`, `worker/src/main.ts`, and the predecessor's `internal/billing/stripe.go`, `internal/store/postgres/{migrations,billing}.go`, `cmd/todo-demo/serve.go`.
- Fetched the style reference (`html.moldandyeast.com` HTML + `styles.css`) into the scratchpad and adopted its tokens/patterns: type-only black on white, square ink/soft buttons, underline-as-selection, 300px label column rows, display-size hero sentence, grey "table" viewer for opened artefacts, "For people / For machines" footer.
- Started a disposable dev database: `docker run -d --name francois-catalog-pg ... -p 127.0.0.1:5434:5432 postgres:17`.
- `pkg/catalog/store`: embedded checksum-protected migrations under an advisory lock (`001_core.sql`, `002_billing_usage.sql`), plus users/sessions/login flows/audit/access grants, settings revisions, releases/files, instances/members, launch codes/grants, documents/leases, assets/comments, Stripe customers/subscriptions/checkout attempts/event inbox, and the usage ledger.
- Refactored `pkg/store/documents.go` to export `ValidateQuery`, `Apply` and `ValidateDepth`; moved sample validation to `piworker.ValidateRequest`. Hosted documents scope SQL by `(instance_id, collection)` and private owner *before* the shared filter/order/limit code.
- `pkg/catalog/billing/policy.go`: pure `Policy.Access` with stable reason codes and a 20-case table test.
- `pkg/catalog/releases`: validation (complete HTML, injected-bootstrap marker, credential-like text, embedded SQLite, unknown capabilities, external script hosts as info) and content-addressed atomic blob storage with hash verification on read.
- `pkg/catalog/identity`: zitadel/oidc v3 relying party, own login-flow storage (state hash, browser binding cookie, nonce, PKCE verifier), nonce checked by the library verifier via context.
- `pkg/catalog/payments`: stripe-go v86 gateway (Customer, Checkout, Portal, subscription listing with `status=all`, item-level period), event extraction, durable ingest, worker claim/complete/backoff, scheduled reconciliation through the same inbox.
- `pkg/catalog/hosted`: `/launch` bootstrap (CSP `frame-ancestors` portal), `/api/launch/exchange` (origin + S256 verifier + one-use code), `/page` injection, views/SSE with 30s re-authorization, scoped RPC, rooms keyed by instance, signed short-lived blob URLs, metered sampling.
- `pkg/catalog/{server,auth,routes,instances,admin}.go`: host dispatch, session/CSRF/Origin guard levels, all portal/admin endpoints from guide Section 15.
- `cmd/francois/cmds/catalog.go`: `catalog serve|migrate|publish|settings`, `catalog users list|promote|disable|enable|revoke-sessions|grant-access`.
- UI: `ui/catalog/{api,store,ui,catalog,account,instances,admin,main}.tsx`, `launch.ts`, `launch-protocol.ts`; runtime hosted mode in `ui/src/runtime.ts`; stylesheet `pkg/catalog/web/static/catalog.css`.
- Worker: private `usage` frame per round and server-set `maxRounds`; local host drops `usage` frames.

### Why
- Instance identity is separate from user identity, so every hosted operation carries `(user, instance, release, role)` re-derived from a grant on each request; the global `Host.Owner` cannot express that.
- Complimentary access grants (`access_grants`) give the "fixture entitlement" Phase 3 asks for while remaining a legitimate audited admin feature that never fabricates a Stripe row.
- The grant is held in page memory and sent as a bearer header because a cross-site iframe cannot rely on third-party cookies; asset `<img>` URLs use per-asset HMAC URLs that expire after one hour since they cannot carry headers.
- The bootstrap uses `document.open/write` to replace itself with the authored page so authored scripts run after the injected runtime on the same instance origin, without ever serving paid HTML to a request lacking a grant.

### What worked
- `go run ./cmd/francois catalog migrate --database-url ...` applied `001_core.sql` and `002_billing_usage.sql`; a second run applied nothing.
- `TEST_DATABASE_URL=... go test ./pkg/catalog/ -run TestHostedPlatform -count=1` → `ok` (creates and drops its own database).
- `go test ./pkg/catalog/billing/ ./pkg/catalog/releases/` → `ok`; existing `pkg/host`, `pkg/store`, `pkg/piworker`, `cmd` tests still pass; `pnpm test` passes; `pnpm check` clean; `go vet ./...` clean.
- Smoke run: `/healthz`, `/readyz`, `/api/session`, SPA shell with CSP headers, unknown instance host → JSON 404, unauthenticated admin → 401.

### What didn't work
- First test compile: `pkg/catalog/catalog_test.go:74:6: actor redeclared in this block` (collides with `admin.go`'s `actor(r)` helper). Renamed the test type to `tactor`.
- First test run: `viewer private write: 400 ... Invalid document or collection path`. My test used `data/users/<uid>` which has three segments and is therefore a collection; private documents are `data/users/<uid>/<doc>`. Fixed the test, not the code.
- Initial draft of `identity.AuthURL` contained a nonsense placeholder option (`func() []oauth2Opt`); rewritten with an explicit `withParam` helper before first compile.
- Playwright MCP refused to open the style reference: `Browser is already in use for /home/manuel/.cache/ms-playwright-mcp/mcp-chrome-...`. Used `curl` for the HTML/CSS instead.

### What I learned
- stripe-go v86 lists via `client.V1Subscriptions.List(ctx, params).All(ctx)` (Go iterator) and `stripe.ConstructEvent` rejects API-version mismatches by default.
- zitadel/oidc v3 lets the caller own PKCE (`rp.WithCodeChallenge` / `rp.WithCodeVerifier`) and nonce (`rp.WithNonce(func(ctx) string)`), so flow state can live in PostgreSQL rather than encrypted cookies.

### What was tricky to build
- Allowance race: the reservation `INSERT ... ON CONFLICT DO UPDATE ... RETURNING` both creates the period row and row-locks it for the rest of the transaction, so the availability check and `reserved += credits` serialize per user. The test fires two calls with one credit and asserts exactly one 202 and one 402.
- Launch-code consumption happens before the verifier check, so a wrong verifier also burns the code (replay after a failed guess fails too); the test asserts this ordering.
- Settlement is idempotent by state: `reserved` → released (no provider start), `started` → charged; restart recovery converts open `started` calls to `review` and charges them rather than refunding uncertain work.

### What warrants a second pair of eyes
- `document.open/write` replacement in `ui/catalog/launch.ts`: verify in real browsers that CDN scripts in POLIS execute in order and that `window.claude` is installed before authored code.
- Same-site dev domains (`localhost` vs `*.apps.localhost`): Lax cookies are host-only so the portal cookie never reaches app hosts, but app code can still send same-site requests to the portal; CSRF tokens + strict `Origin` checking are the control (tested). Production should use a separate registrable app domain.
- `authorizeInstance` loads release/settings/subscriptions on every hosted request; acceptable for one replica, but a cache would need explicit invalidation.

### What should be done in the future
- Compose stack with ZITADEL and real browser validation (next step); Stripe sandbox Checkout/Portal and signed CLI webhooks; Pi worker hosted sampling with the operator's LunaRoute models.

### Code review instructions
- Start at `pkg/catalog/server.go` (`Handler`, `authorizeInstance`), then `pkg/catalog/hosted/routes.go` (`exchange`, `page`), `hosted/views.go`, `hosted/rpc.go` (`writable`), `hosted/sample.go` (`resolveModel`, `run`), `store/usage.go` (`Reserve`, `SettleCall`, `RecoverCalls`), `billing/policy.go`.
- Validate: `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable' go test ./... && pnpm check && pnpm test`.

### Technical details
- Origins: portal `http://localhost:8090`, apps `http://i-<20 hex>.apps.localhost:8090` (Chrome resolves `*.localhost` to loopback and treats it as a secure context, so `crypto.subtle` works).
- Grant TTL 30 min, renewed through the parent at T−2 min or on a 401; launch code TTL 60 s; login flow TTL 10 min; session idle 7 d / absolute 30 d.
- Hosted capability set: `sample, user, db, room, comments, assets, downloads`; `files` and `artifact.publish` are not offered.

## Step 5: Compose acceptance stack and real browser validation

The user asked mid-step for a Docker Compose setup to test the whole thing. I added `deploy/compose` with PostgreSQL, ZITADEL v4 (built-in login), an idempotent provisioning job, the catalog image (Go server + built UI + Node Pi worker) and an optional Stripe CLI forwarder profile. With it I drove the complete journey in Playwright across three separate browser contexts (admin, Alice, Bob) against real ZITADEL, a Stripe sandbox and the operator's Pi/LunaRoute models.

Browser validation found four real bugs that unit/integration tests had missed (admin release list empty, CSRF race on direct launcher loads, an invalid `v`-flag input pattern, missing `max` thinking level) plus layout overflow; all were fixed and the admin-list case now has a regression assertion.

### Prompt Context
**User prompt (verbatim):** see Step 4 (including the mid-step follow-up "create a docker compose setup to test the whole thing, btw").
**Assistant interpretation:** Make the platform runnable end to end locally and validate the UI and runtime in a real browser.
**Inferred user intent:** Confidence that the subscription product actually works as a whole, reproducibly.
**Commit (code):** b59bbbd — "Add Docker Compose acceptance stack and fix browser-found catalog bugs"; 341262f — "Validate Stripe sandbox and hosted Pi sampling in the Compose stack"

### What I did
- `deploy/compose/compose.yaml`, `Dockerfile`, `postgres-init.sql`, `bin/catalog-setup`, `bin/catalog-serve`; `catalog serve --oidc-client-id-file` and `--grant-ttl`.
- Provisioning (`catalog-setup`): waits for the issuer, migrates, creates ZITADEL project `francois` and PKCE web app `francois-portal` (auth method NONE, dev mode, `idTokenUserinfoAssertion`), imports `admin@`, `alice@`, `bob@francois.test`, and promotes the admin by exact issuer/subject. Output: `client 394885842482566147`.
- Created one sandbox Product/Price in the existing test-mode account (`prod_VQpH0Crg2Ztp5I`, `price_1UPxdFLeHmcfmvuCg0zsScj5`, placeholder USD 1.00/month labelled as such). Key and price live only in ignored, 0600 `deploy/compose/.env`.
- Browser journey (screenshots under `.playwright-mcp/shot-*.png`, not committed): ZITADEL login → admin overview → upload `examples/capabilities.html` → Check (warnings for `files`/`artifact`) → Save draft → Preview on `i-….apps.localhost` → add a task → Publish → Alice gated to `/account?subscribe=1` → admin grants Alice/Bob complimentary access → admin approves `test/fake` → Alice runs an instance, shares with Bob (editor) → both see the shared task and "2 tabs here" → Bob samples (1 credit) and is refused `openai/gpt-x` → `user.search` lists co-members without emails → admin suspends the instance → Bob's live frame closes with "This instance is suspended".
- Stripe: admin revokes Bob's complimentary access → Bob subscribes via hosted Checkout (4242 card) → return page → "Active subscription, renews Nov 12" → Manage billing → Portal → period-end cancellation → "Ends Nov 12 · cancellation scheduled" with continued access.
- Pi: `FRANCOIS_BACKEND=pi` with `PI_AGENT_DIR=~/.pi/agent`; approved `lunaroute/glm-5.3-flash` (low/medium/high, 1 credit) and `lunaroute/deepseek-4.1-flash` (low/high/xhigh, 2 credits); uploaded/previewed/published POLIS with `{"sample":{"images":true}}`; the agent ran `eval` (`ide.patch("height: 6,", "height: 10,")`) and the scene rebuilt; Alice's own POLIS instance streamed thinking + answer.
- Grant renewal: restarted with `GRANT_TTL=1m`; after 75 s Bob's page still wrote `tasks/after-renewal`; 17 renewed grants for Bob in three minutes. Restored the 30 m default.
- `pkg/catalog/payments/service_test.go`: fake gateway tests for double-click idempotency, retry after uncertain creation, foreign session refusal, open-subscription redirect, delayed old-subscription event vs replacement, unknown Customer retry, terminal cancellation.

### Why
- Host networking lets one issuer URL (`http://localhost:18081`) work for both the browser and the server, and lets the portal serve `*.apps.localhost`, which Chromium resolves to loopback and treats as a secure context (`crypto.subtle` for PKCE). It is Linux-only; documented in the compose header.
- The Pi worker needs a writable agent directory (lock files, OAuth refresh), so `catalog-serve` copies only `auth.json`, `models.json`, `models-store.json`, `settings.json` into a private volume rather than giving the container write access to the operator's home.

### What worked
- Webhooks: `checkout.session.completed`, `invoice.paid`, `customer.subscription.created`, later `customer.subscription.updated` → all `processed` on attempt 1; projection `active, quantity 1, item_count 1, access_until 2026-11-13, cancel_at_period_end t` after the Portal cancellation.
- Usage ledger for the real POLIS call: `lunaroute/glm-5.3-flash:low`, completed, 1 credit, 4377 input / 60 output tokens, 2 rounds, provider cost 0 (OAuth route).
- `TEST_DATABASE_URL=... go test ./pkg/catalog/...` → all `ok` (catalog, billing, payments, releases).

### What didn't work
- `dependency failed to start: container francois-catalog-zitadel-1 is unhealthy`: `zitadel ready` probes the internal port, which does not match `ExternalDomain:ExternalPort` (`Output: "Error: not ready\n"`) although `curl http://localhost:18081/debug/ready` returned 200. Removed the healthcheck; setup polls the issuer.
- `unknown flag: --output` in the setup script (Glazed output flags not present on these commands). Removed the flag.
- Admin Releases list showed "No releases yet" after a successful draft (DB had the row). Cause: `cardinality($2::text[])=0` with a nil Go slice is `cardinality(NULL)` → NULL. Fixed with `COALESCE(...,0)` and added a regression assertion.
- Console: `Pattern attribute value [a-z0-9][a-z0-9_-]* is not a valid regular expression ... /v: Invalid character in character class`. Escaped the hyphen.
- Direct load of `/instances/<id>` showed "Missing or invalid CSRF token": the frame's launch handshake fired before `/api/session` set the token. The launcher now renders the frame only once the session token is loaded.
- `Refused to apply style from '.../static/style.css' because its MIME type ('text/plain')`: the example app links the local host's shared stylesheet. Instance origins now serve `style.css`/`icon.svg` like local artifact origins.
- `EROFS: read-only file system, mkdir '/etc/francois/pi/auth.json.lock'` starting the Pi worker from a read-only mount → copy into a private writable directory.
- Settings save → 422 `model lunaroute/glm-5.3-flash has unknown thinking level "max"`: the validator lacked Pi's `max` level. Added to validator and UI.
- Playwright automation quirks (not app bugs): MCP refused `/tmp` screenshot paths (allowed roots are the repo); `globalThis` did not persist between calls (looked pages up by session instead); background-context clicks on Stripe and POLIS buttons did not register until `bringToFront()` / in-page `click()`; a `confirm()` dialog was intercepted by the MCP's modal handling.

### What I learned
- ZITADEL v4 still ships the built-in login when `LOGINV2_REQUIRED=false`, which removes the need for the login container and Traefik in a local stack. MFA enrollment prompts on first login unless `LOGINPOLICY_MFAINITSKIPLIFETIME=0s` is set at instance creation (added for fresh stacks).
- The Stripe CLI in this account forwards `2026-06-24.dahlia` events, matching stripe-go v86's release train, so strict `ConstructEvent` verification works without ignoring version mismatches.

### What was tricky to build
- Checkout return vs webhook ordering: here the webhooks were already processed when the return page loaded, so the "checking status" state was never visible; the page still polls and requests an owner-scoped refresh, and the fake-gateway test asserts that a return alone never grants access.

### What warrants a second pair of eyes
- With a 1-minute TTL the runtime renews every ~5 s (renew at T−2 min, floor 5 s). Harmless at the 30 m default, but `--grant-ttl` below ~3 m is noisy; consider renewing at 75 % of TTL instead.
- `localhost` and `*.apps.localhost` are same-site; production must use a different registrable domain for apps.

### What should be done in the future
- Phase 7 (release notifications) remains open; Phase 8b production readiness and Phase 9 launch remain open.

### Code review instructions
- `docker compose -f deploy/compose/compose.yaml up --build`; read `deploy/compose/bin/catalog-setup` and `catalog-serve`.
- `TEST_DATABASE_URL=... go test ./pkg/catalog/...`.

### Technical details
- Sandbox objects: `prod_VQpH0Crg2Ztp5I` / `price_1UPxdFLeHmcfmvuCg0zsScj5` (test mode), Checkout attempt `chk_020806b270c68339703edb36` completed.
- Origins observed: preview `i-82f3669b96eb2f785857`, Alice's shared board `i-99127b164e4989bf88c3`, POLIS preview `i-8e19bb1e6c08af74bc87`, Alice's POLIS `i-e743c79ab92958abdfc8`.

## Step 6: Repository gates and logger conventions

Brought the new packages in line with the repository's CI gates: generated logcopter package loggers replace direct `zerolog/log` imports, golangci-lint findings (named returns, `max` shadowing, one gofmt) are fixed, and hosted grants now renew at 75 % of their remaining lifetime (minimum 15 s) instead of a fixed T−2 min.

### Prompt Context
**User prompt (verbatim):** see Step 4.
**Assistant interpretation:** Keep the implementation consistent with the repository's conventions and CI checks.
**Inferred user intent:** Mergeable, CI-clean work.

### Evidence and commits
- 0bee348 — "Use generated package loggers in catalog, lint fixes, proportional grant renewal".
- `make logcopter-check` (initially `generated file is not current: .../pkg/catalog/logcopter.go`) now passes; `golangci-lint run ./...` → `0 issues.`; `make glazed-lint` → no findings; `TEST_DATABASE_URL=... go test ./...` all `ok`; `pnpm test` 5 pass.
- Rebuilt stack: `/readyz` → `{"models":true,"status":"ready"}`; Bob's instance data persisted across restarts.

### Noteworthy decisions or failures
- The 9 golangci-lint findings were style-only; no behavior changed.

### Remaining requirements and next action
- Open: Phase 7 release notifications (`be57`), Phase 8b production readiness (`pezy`), Phase 9 launch (`dxkw`), and the umbrella Phase 8 (`i63k`).
