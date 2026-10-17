---
Title: Investigation diary
Ticket: FRANCOIS-013
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Guide authorship, source investigation, design choices and delivery evidence."
LastUpdated: 2026-10-17T14:27:28-04:00
WhatFor: Preserve evidence and handoff decisions.
WhenToUse: Resuming or reviewing the guide.
---

# Investigation diary

## Step 1: Split the comparison into a concrete implementation guide

I created this ticket from the Final comparison and wrote a standalone intern guide. The explanation is detailed while the selected implementation is deliberately small. Runtime implementation and production configuration were not changed. This entry records the investigation and drafting already performed in this turn.

### Prompt Context

**User prompt (verbatim):**

> Ok, create two tickets: the visual part, and the signin email part. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable.
>
>
> (for each ticket)

**Additional user steering (verbatim):**

> the design doc is detailed, the design itself is simple, elegant, streamlined, concrete and pragmatic, no overengineering or future oriented, it is in the here and now.

**Assistant interpretation:** Create two independently usable intern guides, store each in its own docmgr ticket, and upload each to reMarkable.

**Inferred user intent:** Give a new contributor enough technical context to ship the present feature without introducing unnecessary architecture or speculative scope.

**Starting code revision:** `dbef98e`.

### What I did

- Created the ticket and design/diary documents through docmgr.
- Read the original comparison and existing Go/TypeScript source, following page, session, identity, release, runner and build contracts.
- Read the textbook-authoring, docmgr, diary and reMarkable delivery skills.
- Wrote foundational prose before the implementation details, then added concrete data flows, API tables, pseudocode, file references and acceptance tests.
- Generated two PNG diagrams with Graphviz using the tracked scripts/01-render-diagrams.py and visually reviewed them.
- Kept implementation tasks open and planned delivery bookkeeping separately.
- Validation completed: docmgr doctor passed; all relative Markdown image/document links resolve; fences are balanced; two diagram PNGs per guide were rendered and visually reviewed. sources/validation.json records the bounded check results. No application test run is claimed for documentation-only work.

### Why

The user explicitly separates detailed explanation from a small implementation. A new intern needs to understand current ownership boundaries, actual request flows and exact contracts; that does not require adding services or expanding scope.

### What worked

- Existing source provides concrete reuse points for the proposed feature.
- Graphviz generated both diagrams successfully; the guide references real PNG files rather than Mermaid source that might print as code.
- Both guides use the same measured Final reference and explicitly distinguish current behavior from proposed routes/data.

### What did not work

- Exploratory source reads initially guessed nonexistent pkg/catalog/config.go and pkg/catalog/store/identity.go; diagnostics were No such file or directory. Located Config in server.go and Identity in store/users.go instead. No source files were changed by those failed reads.
- One broad source read exceeded its output budget. Re-read relevant symbols in bounded searches and corrected the session constructor reference to store/sessions.go:49.

- The first staged whitespace check rejected blank quoted lines as trailing whitespace (`> ` at diary lines 29–30). The failure-gated sequence stopped before committing. Removed only those trailing spaces and reran the staged check.

### What I learned

The email design reuses UserByVerifiedEmail and existing session lifetimes. It prescribes one pending-link table, four HTTP routes and direct bounded SMTP for existing subscriber-role users. Staff retain OIDC; roles and identities are not merged by email. Transactional consumption, browser binding, scanner-safe confirmation and role-change session revocation are concrete requirements, not a new authentication framework. The current ZITADEL login documentation does not justify treating email OTP as a passwordless link toggle.

### What was tricky to build

The hard part was scoping the guide to an implementable present-day change without falsely claiming that all controls in a mockup already have backend support. Proposed choices and existing APIs are labeled separately, with concrete acceptance conditions.

### What warrants a second pair of eyes

Review the selected ticket boundary and the observable tests, then follow the cited entry points. In particular, verify that identity proof never changes account ownership/role and that visual changes preserve the existing instance isolation and resource limits.

### What should be done in the future

Implement this ticket’s listed tasks; no speculative roadmap is prescribed. Guide delivery does not mark implementation complete.

### Code review instructions

Start with the guide introduction and its architecture diagram; a new reader should be able to explain the main entities and request sequence from those alone. Check file references against the starting revision, run docmgr doctor, validate local links/images, and use the upload receipt to distinguish cloud delivery from physical device synchronization.

### Technical details

- Scope: documentation, diagrams and ticket bookkeeping only.
- Diagram generator: Python standard library calling installed dot; no dependency installation.
- reMarkable destination: /ai/2026/10/17/FRANCOIS-013/.
- Final command/output evidence belongs in sources/upload-receipt.txt and the delivery diary checkpoint.

## Step 2: Validate the guide and deliver it to reMarkable

The completed guide was uploaded as its own PDF with a depth-two table of contents. The successful CLI receipt confirms cloud delivery. Physical device synchronization was not independently checked, and no routine cloud listing or extra PDF render was added.

### Prompt Context

**User prompt:** See Step 1, including the explicit request for one upload per ticket.
**Assistant interpretation:** Deliver the finished, validated guide and retain a reviewable receipt.
**Inferred user intent:** Make the technical guide available for reading and annotation on reMarkable.
**Commit (documentation):** `84da5b6` — Split Final redesign into visual and email sign in intern guides.

### What I did

- Confirmed docmgr metadata, local image/document links, balanced code fences and the rendered diagrams before delivery.
- Used the remarkable-upload skill’s bounded finalization handoff for the frozen guide; no content changes were made during upload.
- Ran the normal bundle upload with a unique document name, ticket-specific destination, toc-depth 2 and non-interactive mode, without force.
- Retained command output in sources/upload-receipt.txt and the compact result in sources/delivery-summary.json.
- Marked only the design/validation/delivery tasks complete; runtime implementation remains open.

### Why

The requested deliverable includes both a repository guide and reMarkable delivery. A clear successful upload is sufficient cloud evidence under the delivery skill; it is not a claim about the physical device.

### What worked

`OK: uploaded FRANCOIS 013 Email Sign In Intern Guide.pdf -> /ai/2026/10/17/FRANCOIS-013`

### What did not work

The first attempt hit restricted-network DNS resolution (`lookup webapp-prod.cloud.remarkable.engineering: no such host`). The network-enabled retry succeeded. The CLI also reported transient remote-tree refresh warnings before its successful upload; the full sequence is retained in the receipt.

### What I learned

The guide’s local PNG diagrams and Markdown references were accepted by the normal bundle pipeline. No extra renderer or upload mechanism was necessary.

### What was tricky to build

Delivery needed to preserve the final guide and its evidence while avoiding overwriting an annotated document. A unique ticket folder and document name allowed an ordinary upload with no replacement.

### What warrants a second pair of eyes

Implementation should follow the current scoped guide rather than broader preliminary suggestions in FRANCOIS-011. The successful receipt does not qualify any unimplemented feature.

### What should be done in the future

Implement the existing open tasks. There is no additional design roadmap in this delivery.

### Code review instructions

Read the guide, sources/validation.json and the successful receipt. Check that the remaining tasks still describe implementation rather than delivery. No application test result is claimed.

### Technical details

- Remote destination: /ai/2026/10/17/FRANCOIS-013/.
- Source guide revision: 84da5b6.
- Evidence level: successful cloud upload; no physical device synchronization verification.

## Step 3: Establish the ZITADEL-backed magic-link implementation

The user asked whether ZITADEL could support the exact email-only journey, then requested saved sources and a concrete implementation path. The first answer was too conservative: hosted-login documentation alone did not establish whether the custom Session API could do this. Searching the upstream discussion and tracing the deployed release establishes a supported API path without requiring a password check.

The revised guide selects direct Session API authentication inside Francois, followed by the existing local portal session. ZITADEL generates, delivers and validates the email code. This keeps the same application/account model and avoids a custom OIDC proxy or independent SMTP/token implementation. Runtime qualification is still open; no real login is claimed from a source audit.

### Prompt Context

**User prompts (verbatim):**
- "how woul you implement the sign-in given zitadel"
- "can you search for email-only magic-link with zitadel?"
- "save relevant sources in the sources/ folder of the ticket"
- "ok, figure out how to build it, we do want magic link"

**Assistant interpretation:** Research email-only authentication in ZITADEL, retain primary sources and replace the ticket's unsupported first-party default with a concrete provider-backed design.
**Inferred user intent:** Deliver the Final magic-link experience with a simple implementation using the existing identity infrastructure.
**Starting revision:** 3ae52c7; clean working tree before this investigation.

### What I did

- Searched official docs and upstream GitHub discussion #2075. Found a November 2025 custom Session API recommendation and a March 2026 report of success.
- Saved five primary pages as raw HTML with timestamps, URLs, sizes and SHA-256 digests; used Defuddle for readable Markdown companions.
- Ran a read-only image query using `kubectl get deployments,statefulsets -A -o json`, filtering names and image references. Both deployed ZITADEL components report v4.16.1.
- Cloned upstream sparsely in /tmp, fetched v4.16.1 and checked it out detached at cc3812a940d629984802fd1246f84fd91d07f6e8. Saved relevant proto/command/API files and permission defaults under ticket sources.
- Traced createOTPEmailChallenge, CheckOTPEmail, session factor reduction, auth-request linking and code-exchange compliance. Verified that an enrolled email challenge requires a checked user but no password check, and that successful email checks become authentication factors.
- Noted the current MFA guide's obsolete boolean returnCode example; the deployed proto uses the sendCode/returnCode object oneof.
- Archived the earlier local-token design and rewrote the primary intern guide around four routes, one pending-flow table and a bounded provider client. Added a source-trace analysis and an explicit runtime qualification gate.
- Kept production users, factors, login policies, SMTP configuration and credentials untouched. No secret values were read or saved.

### Why

The user explicitly requires magic links. The relevant implementation boundary is custom Session API authentication, rather than the hosted login screen's available controls. For an own-application login, the official guide says additional OIDC plumbing is unnecessary. Direct provider verification followed by the existing local session is smaller than changing provider application routing and implementing custom OIDC request completion.

### What worked

Public source downloads succeeded with network-enabled execution. The pinned source resolves the password prerequisite and wire-format questions. Saved readable extracts and provenance manifests make those findings reviewable without relying on search snippets. The deployment query returned only public component identifiers/images, with no configuration export.

### What didn't work

The initial Python urllib download in the restricted sandbox failed with `urllib.error.URLError: <urlopen error [Errno -2] Name or service not known>`. Retrying the tracked download script with network-enabled execution succeeded. Initial source probes guessed nonexistent flat filenames (`internal/command/oidc_auth_request.go` and `proto/zitadel/session_service.proto`); rg reported file-not-found. Searching the actual tree found `internal/command/auth_request.go` and `proto/zitadel/session/v2/session_service.proto`.

### What I learned

Email OTP's placement under MFA in hosted-login documentation does not restrict a custom session to password-plus-email. Conversely, the low-level session-linking API does not automatically apply all hosted-login authentication decisions. Our handler must explicitly require the fresh successful email factor and intended identity. Merely selecting a user is never sufficient authentication.

### What was tricky to build

The design must preserve scanner-safe GET handling, same-browser binding, single use and local role safety while provider verification occurs outside a database transaction. The guide resolves this with atomic flow claiming before network calls and a locked user recheck before local session creation. One-time consumption deliberately sacrifices retrying the same link after a transient failure; the user requests another email instead.

Late provider responses are another boundary: a conditional provider-session attachment must fail if the pending flow was replaced, and then clean up the newly created provider session. No database transaction remains open across provider calls.

### What warrants a second pair of eyes

Review the trusted Session API credential's permissions, required successful otpEmail factor, issuer/subject/organization checks, provider email freshness and transaction ordering for promotion versus session creation. Email-authenticated sessions must never acquire admin access through later role promotion. Confirm the temporary provider session's expiry and cleanup semantics in the actual round trip.

### What should be done in the future

Implement the revised ticket tasks and run a real isolated v4.16.1 test-mail flow before enabling the feature. No extra authentication-service roadmap is implied. The earlier reMarkable PDF remains a historical snapshot of the superseded guide, explicitly identified in the ticket index.

### Code review instructions

Start with the source-trace analysis, then read the revised guide and inspect the pinned proto/command files. Validate saved-source digests and docmgr metadata. Validated all 19 saved raw page/source/license SHA-256 digests, checked balanced guide/analysis fences, ran docmgr doctor (passed). The pre-staging diff check passed for authored changes. Once raw source snapshots were staged, the full check reported upstream whitespace (for example `01-magic-links-discussion.html:6: trailing whitespace`). These are byte-preserved evidence, so they were not reformatted. A scoped diff check excluding the snapshot folder passed. No application test suite is claimed because application code was not changed; end-to-end verification remains an explicit implementation gate.

### Technical details

- Provider calls: GET /v2/users/{subject}, POST /v2/sessions, PATCH /v2/sessions/{id}, GET /v2/sessions/{id}, DELETE /v2/sessions/{id}.
- Enrollment: POST /v2/users/{subject}/otp_email during controlled supporter provisioning.
- Email URL: trusted origin, opaque flow and Code placeholder; maximum template length 200 characters.
- Provider session lifetime: 600s; no token returned to browser or retained in the pending table.
- Login credential: dedicated service account; IAM_LOGIN_CLIENT includes the needed permissions in v4.16.1, while ORG_USER_MANAGER lacks session.write.
- Main evidence: sources/zitadel-email-login/{manifest.json,source-manifest.json,deployment-version.json}.

## Step 4: Implement provider-backed magic links and prove the real email round trip

The user authorized implementation after the source investigation. The feature now exists in the Go service: a Final sign-in form, browser-bound pending attempts, direct ZITADEL email challenges, scanner-safe confirmation, single-use redemption and normal same-user portal sessions. It remains disabled by default and has not been enabled in production.

The decisive qualification succeeded against an isolated **ZITADEL v4.16.1**, a dedicated IAM_LOGIN_CLIENT credential, PostgreSQL and a Mailpit SMTP sink. A delivered email link completed sign-in without any password check, preserved the existing local identity and passed provider replay rejection and local logout. This replaces Step 3's source-only qualification with a real runtime result.

### Prompt Context

**User prompt (verbatim):** "Oh no it's fine then. work on the new ticket, commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"
**Assistant interpretation:** Implement FRANCOIS-013 locally, test it, update ticket evidence and commit at useful boundaries.
**Inferred user intent:** Ship the agreed simple ZITADEL-backed email-only journey with a detailed reviewable work history.
**Starting revision:** f47ab19, the source-backed design checkpoint.

### What I did

- Added identity/email.go: a fixed-origin HTTP client with a dedicated server-only credential, bounded bodies/timeouts, redirect refusal, active-user/email checks and required matching fresh email factor.
- Added migration 012 and store/email_signins.go: identity/origin/browser-bound pending rows, conditional late-send attachment, atomic one-time claim and user-locked final session creation.
- Reused the session insertion code and made role changes revoke sessions, runtime grants and pending email attempts under the same user lock. Admin role updates also close active runtime views.
- Added the four email routes and server-rendered Final form/sent/confirmation/error states. Sensitive responses are no-store/no-referrer/noindex and contain no public JavaScript. Authentication paths and /signin are excluded from traffic capture.
- Routed normal sign-in links to /signin, while preserving /auth/login for staff and fallback when email login is disabled. Added Glazed enablement/organization/credential-file flags and empty-ID-token local logout.
- Added security integration tests and an explicit opt-in real-provider test using a private fixture file rather than new environment lookup code.
- Started test infrastructure in tmux session francois-email-013-infra. Used a dedicated local stack and provisioner; all credentials remain beneath /tmp/francois-013-private.
- Updated the guide and production runbook to match implementation and the real observed expiry.

### Why

The direct Session API path verifies mailbox possession through the existing identity infrastructure and ends at our existing local sessions. It needs no custom OIDC proxy, independent SMTP client, local mailbox token scheme or new deployed application service. Browser and origin binding, a single atomic claim and a locked role recheck close the material trust boundaries.

### What worked

- `go test ./pkg/catalog/identity -count=1`: passed with network-enabled local listeners, including missing/wrong/stale factors, provider user eligibility, wire format and credential redirect prevention.
- `TEST_DATABASE_URL=postgres://emailtest:emailtest@127.0.0.1:5437/postgres?sslmode=disable go test ./pkg/catalog -run TestEmail -count=1`: passed; the harness created/disposed its own databases.
- `TEST_DATABASE_URL=... go test ./pkg/catalog -run '^TestEmailSigninRealZitadel$' -count=1 -v -args -email-login-fixture=/tmp/francois-013-private/fixture.json`: passed in 3.86s.
- Real-provider checks: SMTP delivery, scanner-safe GET, password-free verification, same-user portal session, rejected terminated-provider/code replay and local logout.

### What didn't work

The first broad package check ran in the restricted sandbox: existing TestAPICommandsCallTheAPI failed with `httptest: failed to listen on a port: listen tcp6 [::1]:0: socket: operation not permitted`. This is a listener permission failure, not a feature diagnostic. Subsequent networked tests used escalation.

The original source snapshots had .go extensions under ttmp, which would make Go's recursive package discovery compile research evidence. Renamed them to .go.txt and updated the manifest without changing file bytes. The source URLs and checksums remain intact.

### What I learned

ZITADEL v4.16.1's default email OTP expiry is **five minutes**, while the initial guide proposed ten. The application and copy now use five minutes so a link does not appear valid after the provider code expires. A code check plus explicit fresh factor validation works with the dedicated login service role; no password request is needed.

### What was tricky to build

The provider call occurs outside a database transaction. Pending state is inserted before sending, and a conditional attach prevents a late response from resurrecting a replaced flow. The final transaction locks the same user row as role updates: either sign-in occurs first and promotion revokes it, or promotion occurs first and sign-in refuses the privileged user. A verification failure consumes the attempt and requires a new email, keeping retries explicit and the flow small.

### What warrants a second pair of eyes

Review provider proof validation, identity snapshots, no-secret diagnostics, CSRF/browser/portal binding, role-change revocation and user-lock ordering. A provider account change after the final provider read retains the normal external-identity revocation boundary; this is not a distributed transaction.

### What should be done in the future

Finish full repository checks and inspect desktop/mobile forms. Production enablement requires a separately provisioned dedicated credential, SMTP and enrolled supporters; no production auth settings were changed here.

### Code review instructions

Start with identity/email.go and email_signin.go, then inspect store/email_signins.go and the role/session changes. Run the focused TestEmail suite with the isolated PostgreSQL DSN. The real-provider test is opt-in and the fixed-loopback provisioner never accepts a production issuer. No secrets or live codes are in this diary.

### Technical details

- Provider and pending flow lifetime: 300s; portal session lifetime unchanged.
- Request acknowledgement/provider issuance deadline: 3s; verification deadline: 10s; cleanup bounded to 2s.
- The provider token is only in an HTTP Authorization header, never a cookie or email link.
- Local infrastructure ports: 18083, 18025, 5437; tmux session: francois-email-013-infra.
- The real email account is a disposable supporter fixture, with no production access.

## Step 5: Qualify real browser navigation, match Paper and finish verification

The first implementation checkpoint was committed as **2f2fe66 — Implement ZITADEL email-only supporter sign-in**. The subsequent real-browser check found a material difference from synthetic HTTP tests: Chromium makes the Origin header null for form navigations from our no-referrer authentication pages. The normal origin check consequently rejected our own form. This was fixed narrowly and tested against cross-site/null-origin cases before completing browser evidence.

The final browser journey now delivers real mail through isolated ZITADEL, opens a scanner-safe confirmation page, creates the same supporter's session, rejects replay and signs out locally. Ten screenshots cover five states at 1440px and 390px, without horizontal overflow. No credentials, codes or live sign-in URLs are saved in the evidence.

### Prompt Context

**User prompt:** See Step 4. During finalization the user additionally requested two subsequent tickets: a download-only watermark intro and a sandbox catalog API guide. Those requests do not replace finishing this authentication work.
**Assistant interpretation:** Finish qualification and a focused commit before moving to the new implementation/research tasks.
**Inferred user intent:** Complete working email login with an accurate diary and real browser evidence.
**Commit (implementation):** 2f2fe66.

### What I did

- Compared the Final sign-in desktop/mobile Paper snapshots with the local form. Matched the Sign in heading, 640px form column, muted label, 48px field, restrained button, mobile full-width button and supporter link.
- Refreshed the existing email binding cookie on admitted requests so an older open form does not shorten a newly issued link's usable lifetime.
- Added concurrent promotion/session-creation tests and ran the email suite with the race detector.
- Added the no-referrer navigation exception only for Origin null plus browser-declared same-origin/navigation Fetch Metadata; the separate host-cookie/form CSRF check still applies. Added rejection tests for cross-site, same-site-only, absent metadata, non-navigation requests and missing CSRF.
- Rebuilt checked-in frontend assets. The direct pnpm standalone executable embeds Node 20 despite shell PATH; reran check/test/build through the installed Node v24.18.0 Corepack entry point without engine warnings.
- Added repeatable local preview and browser verification scripts. Screenshots and redacted checks live under sources/implementation.
- Kept the earlier reMarkable upload explicitly marked as the superseded first-party guide; this implementation did not silently replace that PDF.

### Why

A correct HTTP-level proof test does not establish browser navigation correctness. The no-referrer policy is needed to protect link codes, and removing CSRF/origin validation would weaken the implementation. Fetch Metadata permits the specific legitimate navigation while rejecting cross-site forms; the cookie/form token remains an independent requirement.

### What worked

- Full `go test ./... -count=1` with the isolated PostgreSQL DSN passed, including database integration packages.
- `go build ./...` passed.
- `make lint logcopter-check` passed after resolving the reported issues.
- Node v24.18.0 Corepack `pnpm check`, `pnpm test` (9 passing tests) and `pnpm build` passed.
- Race-enabled focused email tests passed, including sixteen simultaneous promotion/session-creation attempts.
- Browser script returned six successful checks and ten screenshots: form, sent, confirm, account and used-link states at both viewport widths.
- Saved raw upstream source checksums still match after .go.txt renaming.

### What didn't work

- The first lint pass reported nonamedreturns on admitEmail, predeclared on a test variable named new, ST1013 for a literal HTTP status and QF1001 for the transport condition. Replaced named returns, renamed the variable, used status constants and simplified transport validation. The next lint pass reported zero issues.
- The first screenshot run began before go run had started listening: `page.goto: net::ERR_CONNECTION_REFUSED at http://localhost:18793/signin?return=/account`. Added a bounded readiness probe.
- The next real-browser submission reached the error page with `Please start sign-in again on this site.` Safe request diagnostics showed Origin null. The narrow Fetch Metadata handling fixed both send and confirmation POSTs; the full browser journey then passed.

- Final staging initially named a repository-root `.docmgr-operations/close-1fd61387569ba7bad46a1105130a6b1d.json`, and Git reported `pathspec ... did not match any files`. The receipt is inside the ticket; staging that ticket directory corrected the path without changing application files.
- A final sandboxed `go build ./...` tried to write module version metadata and reported `read-only file system` under the Go module download cache. The network/cache-enabled build succeeded. Final verification commit: **0b1c302 — Verify email magic links in Chromium and harden navigation checks**.

### What I learned

Referrer-Policy affects the browser's serialized Origin on POST navigation, not just the Referer header. Synthetic requests setting the portal Origin do not exercise that behavior. The explicit browser journey now prevents this regression from being hidden by API-only tests.

### What was tricky to build

The origin check has to distinguish a privacy-protected same-origin form navigation from an opaque or cross-site origin. Requiring both same-origin Fetch Metadata and navigation mode, then checking the HTTP-only binding cookie against the submitted form token, keeps the exception narrow. A null Origin alone is never sufficient.

### What warrants a second pair of eyes

Review emailForm's origin condition beside the negative tests and browser evidence. Inspect SetUserRole and CreateEmailSession locking together. Confirm any production rollout mounts a dedicated login credential, enables SMTP and enrolls only intended existing supporters.

### What should be done in the future

Production enablement is a separate operational action. The feature is implemented and tested locally; there is no remaining authentication implementation task in this ticket. Continue with the user's subsequent tickets.

### Code review instructions

Read the implementation commit and subsequent browser fix, then sources/implementation/checks.json and browser-validation.json. Use scripts/03-local-stack.yaml, 04-provision-local.py, 05-run-preview.sh and 06-verify-browser.mjs to reproduce the isolated flow. Physical reMarkable synchronization or a production deployment is not claimed.

### Technical details

- Preview: http://localhost:18793/signin, tmux francois-email-013-preview.
- Isolated provider/mail/DB stack: tmux francois-email-013-infra.
- Supported browser exception: Origin null AND Sec-Fetch-Site same-origin AND Sec-Fetch-Mode navigate, plus valid independent CSRF/binding.
- Test browser screenshots contain page content only; no address bar or live link/code/token captures.
