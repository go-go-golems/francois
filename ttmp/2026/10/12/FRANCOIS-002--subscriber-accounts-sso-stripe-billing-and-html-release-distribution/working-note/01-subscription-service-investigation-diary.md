---
Title: Subscription service investigation diary
Ticket: FRANCOIS-002
Status: active
Topics:
  - backend
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles: []
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
