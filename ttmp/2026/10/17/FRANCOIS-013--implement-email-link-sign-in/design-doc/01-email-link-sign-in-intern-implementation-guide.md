---
Title: Email link sign in intern implementation guide
Ticket: FRANCOIS-013
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/catalog.go
      Note: Glazed flags and secret file loading
    - Path: repo://doc/production.md
      Note: Recorded ZITADEL and invite-only deployment policy
    - Path: repo://pkg/catalog/auth.go
      Note: Current login cookies guards and logout
    - Path: repo://pkg/catalog/identity/oidc.go
      Note: Verified OIDC identity exchange
    - Path: repo://pkg/catalog/store/instances.go
      Note: Runtime grants derived from sessions
    - Path: repo://pkg/catalog/store/sessions.go
      Note: Existing session format expiry and revocation
    - Path: repo://pkg/catalog/store/users.go
      Note: Verified email lookup and role changes
ExternalSources:
    - https://zitadel.com/docs/guides/integrate/login-ui/login-app
    - https://cheatsheetseries.owasp.org/cheatsheets/Forgot_Password_Cheat_Sheet.html
    - https://cheatsheetseries.owasp.org/cheatsheets/Session_Management_Cheat_Sheet.html
Summary: ZITADEL Session API email challenges authenticate existing supporters into ordinary portal sessions.
LastUpdated: 2026-10-17T14:27:28-04:00
WhatFor: Teach a new contributor the current authentication system and a small complete email sign in implementation.
WhenToUse: Implementing the working Send link form from Ramon Final.
---
# Email-only magic links backed by ZITADEL

## 1. Outcome and selected design

A supporter enters an email address, receives a link, opens it in the same browser and presses Continue. No password or passkey is required. ZITADEL generates, sends and verifies the email code. Francois creates its normal local session only after ZITADEL confirms the email check for the expected existing identity.

Use the Session API directly inside the existing Go service. This is a custom login screen for our own application, not a separately deployed login service. The existing staff OIDC login remains available. There is no new identity provider, mail queue, local SMTP implementation, token-exchange impersonation or custom OIDC proxy.

This revises the original guide, preserved in `../archive/01-superseded-first-party-email-guide.md`. That guide selected locally generated email tokens because the hosted-login documentation describes email OTP as MFA. Further investigation found ZITADEL's explicit custom-login recommendation and verified the relevant implementation in the deployed version. The choice is now **ZITADEL email authentication, followed by the existing Francois session**.

The scope remains existing enabled subscriber-role accounts with one unambiguous verified email. A subscriber role is not proof of current payment: signing in restores account access; existing entitlement checks still control hosted functionality. Do not create users or attach accounts merely because two email strings match. The durable identity remains the existing configured issuer plus ZITADEL user ID.

## 2. Evidence and qualification

The cluster reports both ZITADEL and its login application at **v4.16.1**. The source snapshot is pinned to `cc3812a940d629984802fd1246f84fd91d07f6e8`, not today's main branch. Evidence is under `../sources/zitadel-email-login/`, including raw HTML, readable Markdown, provenance manifests, deployment image names and selected upstream source files.

The important findings are:

- `proto/zitadel/session/v2/challenge.proto` supports `otpEmail.sendCode.urlTemplate`, including a `{{.Code}}` placeholder. ZITADEL can send a link to our confirmation page.
- `internal/command/session_otp.go:createOTPEmailChallenge` requires a checked user and an enrolled email factor. It does not require a successful password check.
- `CheckOTPEmail` validates the session's challenge through ZITADEL's code checking and records `OTPEmailChecked`. The session model clears the challenge after success.
- `session_model.go:AuthenticationTime` includes the email check. `AuthMethodTypes` records email OTP. This is an authenticated factor, not merely an email-address verification request.
- The official custom-login OIDC guide explicitly says OIDC plumbing is unnecessary for a login UI used only by its own application. Use that smaller integration here.

The implementation now passes a real isolated v4.16.1 SMTP round trip, scanner-safe confirmation, same-user session creation, provider replay rejection and logout. No production identity, policy, SMTP configuration or session was mutated. The detailed execution record is in diary Step 4.

Sources: [ZITADEL magic-link discussion](https://github.com/zitadel/zitadel/discussions/2075), [custom-login integration](https://zitadel.com/docs/guides/integrate/login-ui/oidc-standard), and the pinned source manifest. The discussion recommends Session API glue; it is not itself a complete recipe.

## 3. Current system and responsibility boundaries

`pkg/catalog/auth.go` implements the current OIDC entry point, callback, cookie settings, session resolution, CSRF checks and logout. `pkg/catalog/identity/oidc.go` discovers the issuer, builds Authorization Code + PKCE requests and validates returned identity tokens. `pkg/catalog/store/users.go` stores users keyed by issuer and subject, and provides `UserByVerifiedEmail`, which refuses ambiguous enabled verified-email matches. `pkg/catalog/store/sessions.go` creates opaque portal sessions, stores only the digest of the cookie secret and applies idle and absolute expiration.

The email flow reuses the existing local user and session store. It must not call `UpsertIdentity` with browser-supplied values, invent an email issuer or treat an email OTP session as an OIDC ID token. Our server authenticates its Session API calls with a dedicated ZITADEL service credential. A successful Session API check is the authentication proof; the known existing issuer/subject selects the local user.

```mermaid
sequenceDiagram
    participant B as Browser
    participant F as Francois Go service
    participant D as Francois database
    participant Z as ZITADEL Session API
    participant M as Supporter mailbox
    B->>F: POST /auth/email/request (email, CSRF)
    F->>D: Find eligible existing identity; save browser-bound flow
    F->>Z: Read user; create session with email challenge
    Z->>M: Email link to Francois confirmation page
    F-->>B: Generic check-your-email response
    M-->>B: Open link
    B->>F: GET /auth/email/verify (flow, code)
    F-->>B: Continue form; no code verification yet
    B->>F: POST /auth/email/verify (flow, code, CSRF)
    F->>D: Atomically claim unexpired, browser-bound flow
    F->>Z: PATCH session with checks.otpEmail.code
    F->>Z: GET session; require expected user + fresh email factor
    F->>D: Revalidate identity; create normal session transactionally
    F->>Z: Delete temporary provider session
    F-->>B: Set portal cookie; redirect to saved local path
```

The provider session exists only to verify this login. It has a five-minute lifetime and is deleted after the attempt. Portal sessions retain the existing seven-day idle / thirty-day absolute bounds. Email login does not establish ZITADEL hosted-login SSO, and later ZITADEL logout does not automatically revoke this portal cookie. Local session revocation remains authoritative, just as for the application's existing session store.

## 4. Provider configuration and exact API requests

Provision one dedicated service account and mount its credential as a secret file. In v4.16.1 the built-in `IAM_LOGIN_CLIENT` role includes the required session read/write/delete and user-read permissions. It is a powerful login credential, not the public OIDC client ID. Do not reuse the Terraform administrator PAT in the application. The default org user-manager role lacks session.write. Validate permissions on the test deployment before shipping; do not invent an organization-scoped login role that is absent from this release.

Email OTP must be enrolled on each eligible ZITADEL user and ZITADEL must have working SMTP. Enroll explicitly during supporter provisioning using `POST /v2/users/{subject}/otp_email` with `{}` after a verified primary email exists. Do not silently enroll arbitrary browser-supplied identities, change provider policies or ignore all enrollment errors on every sign-in request. An already-enrolled factor is an idempotent provisioning case to classify deliberately.

All example API requests below are **server-to-server**, authenticated with `Authorization: Bearer <dedicated-service-token>` over HTTPS. The credential never enters the HTML or client JavaScript.

First read `GET /v2/users/{subject}`. Require the expected organization, an active human user, and `human.email.isVerified == true`. Its current primary address must match the eligible local account's verified address. Repeat this validation before final session creation so a local stale email cannot redirect proof to a changed account. Reject a local identity whose issuer differs from the configured ZITADEL issuer.

Create a temporary authentication session:

```http
POST /v2/sessions
Content-Type: application/json

{
  "checks": {"user": {"userId": "EXISTING_ZITADEL_SUBJECT"}},
  "lifetime": "300s",
  "challenges": {
    "otpEmail": {
      "sendCode": {
        "urlTemplate": "https://slop.boutique/auth/email/verify?flow=RANDOM_FLOW&code={{.Code}}"
      }
    }
  }
}
```

Construct the URL from `Site.PortalOrigin()` and a cryptographically random flow value. Do not accept a callback URL from the browser or use an untrusted Host header. The template has a 200-character limit in the proto; validate the finished template, including configured origin and random flow. Do not embed a provider session token, user ID or arbitrary return URL. The response contains `sessionId` and `sessionToken`; save the ID in the private flow record and discard the token, because privileged server calls authenticate with the dedicated service credential.

The currently indexed MFA guide still shows a boolean `returnCode`. The actual v4.16.1 email challenge uses the `sendCode` / `returnCode` object oneof above. Follow the pinned proto, not that outdated example.

After scanner-safe browser confirmation, check the code:

```http
PATCH /v2/sessions/PROVIDER_SESSION_ID
Content-Type: application/json

{"checks": {"otpEmail": {"code": "CODE_FROM_EMAIL"}}}
```

Then `GET /v2/sessions/{id}`. Require `session.factors.user.id` to equal the flow's saved subject, the factor's organization to match, `factors.otpEmail.verifiedAt` to be present and fresh for this flow, and the session not expired. A session with only `factors.user` identifies the selected account; **it is not proof of mailbox possession**. Never create a portal session from that state.

Delete the short-lived provider session using `DELETE /v2/sessions/{id}` after completing or abandoning redemption. Cleanup failure must not undo a committed local session; log the provider session ID at an appropriate private operational level and rely on its explicit five-minute expiry. Never log codes, cookies, service tokens or whole request URLs.

## 5. Four routes, one small flow table

Keep the feature in the existing Go service:

| Route | Contract |
| --- | --- |
| `GET /signin` | Render Final email form, browser binding and CSRF value |
| `POST /auth/email/request` | Validate form, rate limit, select eligible identity, request provider email |
| `GET /auth/email/verify` | Validate flow/browser; render Continue; no provider mutation |
| `POST /auth/email/verify` | Atomically claim flow; verify provider proof; create session |

The form works without React. Unknown, ambiguous, disabled and privileged addresses all receive the same public acknowledgement. Mail service disablement can show a global unavailable message independent of a submitted address. Use the existing OIDC entry point for creator/admin sign-in.

One table holds pending state:

```text
email_signins
    flow_hash             text primary key
    user_id               text unique references users
    issuer                text not null
    subject               text not null
    binding_hash          text not null
    email_snapshot        text not null
    provider_session_id   text null
    created_at            timestamptz not null
    expires_at            timestamptz not null
    return_path           text not null
```

A flow contains 32 CSPRNG bytes encoded as hex; only its SHA-256 digest is stored. The HTTP-only binding cookie contains a separate random value. Apply the existing secure-cookie conventions: Secure on HTTPS, SameSite=Lax, Path=/ and no Domain. Use a separate cookie name from the OIDC flow cookie so the two journeys do not overwrite each other's browser binding. A new request replaces the user's old pending flow; delete its old provider session best-effort outside the transaction.

Insert the pending record before asking ZITADEL to send the link. Save the returned provider session ID with a conditional update against that exact flow hash. Until this update succeeds, verification reports that the request is not ready. A replaced or expired row must not be resurrected by a late API response. Delete the newly created provider session if its conditional update loses the race. Do not keep database locks open during network calls.

GET verification returns a no-store, no-referrer confirmation page without third-party resources or analytics. Codes are escaped into a hidden POST field; they are never used as HTML. Require the original binding cookie and same-origin CSRF validation on POST. Chromium sends Origin: null for form navigation from a no-referrer document. The handler permits that only with Sec-Fetch-Site: same-origin and Sec-Fetch-Mode: navigate, and still requires the independent binding cookie and matching CSRF value. Cross-site, same-site-only and missing-metadata null origins are refused. This intentionally requires the same browser; explain that in the email/form. Email scanners opening GET must not authenticate or consume the challenge.

## 6. Single use, failures and transaction boundaries

Claim the flow before contacting ZITADEL using an atomic delete conditioned on the digest, browser binding, expiry and non-null provider session ID. `DELETE ... RETURNING` gives one caller the complete saved row. A replay or concurrent request cannot reach provider verification through this portal route. A wrong code or temporary provider failure burns this attempt; the user requests another email. This is a deliberate simple failure contract, not a retry queue.

```text
request_email(form):
    require_valid_input_origin_and_csrf(form)
    admit_under_mail_limits()
    user = unique_enabled_verified_subscriber(form.email)
    if user absent: return generic_acknowledgement()
    require user.issuer == configured_zitadel_issuer
    provider_user = zitadel.get_user(user.subject)
    require active_verified_matching_user(provider_user, user)
    flow = replace_pending_flow(user, browser_binding, safe_return)
    provider = zitadel.create_session(user.subject, link_template(flow))
    if not attach_provider_session_if_flow_still_current(flow, provider.id):
        zitadel.delete_session_best_effort(provider.id)
    return generic_acknowledgement()

confirm_email(form):
    require_same_origin_csrf_and_browser_cookie()
    flow = delete_ready_unexpired_flow_returning_row(form.flow, cookie)
    if absent: return invalid_or_used_link()
    defer delete_provider_session_best_effort(flow.provider_session_id)
    zitadel.check_email_code(flow.provider_session_id, form.code)
    proof = zitadel.get_session(flow.provider_session_id)
    require proof matches subject, organization, fresh otpEmail factor
    current_provider_user = zitadel.get_user(flow.subject)
    require active_verified_matching_user(current_provider_user, flow)
    transaction:
        lock local user row
        require same issuer, subject, email, enabled, subscriber role
        create ordinary portal session with empty OIDC id_token
    set existing portal cookie
    redirect to stored safe_return
```

The implemented table also saves issuer and subject with the flow. The final transaction compares those snapshots to the locked local user, as well as checking the email and role. No browser-supplied identity can replace them.

Provider proof and the database cannot share an atomic transaction. After proof succeeds but before local commit, a crash means the user requests a new email. That is acceptable. A local role/disable change during the network calls is caught by locking and revalidating the local user in the final transaction. The provider can change immediately after its read; this is the same general external-identity revocation boundary as OIDC, not a claim of a distributed lock.

SetUserRole must revoke existing portal sessions and derived runtime grants transactionally on role changes. Otherwise an email-authenticated subscriber session could become an admin session after promotion. Session creation and role changes must lock the same user row so concurrent promotion cannot skip revocation. Tests must exercise both lock orders.

Logout revokes the local session as usual. When its ID token is empty, clear the cookie and return locally instead of attempting provider OIDC end-session. The temporary ZITADEL proof session has already been deleted; there is no provider session to keep alive for this login method.

## 7. Limits and configuration

Reuse the small rate-limiter utility with a separate email instance: three requests per normalized recipient per hour, twenty per client address per hour, one hundred globally per hour. Reserve counters before lookup or network work. Bound submitted code length and all request bodies; each consumed flow permits one code attempt. The v4.16.1 default OTP email code expires after five minutes; the portal flow and provider session use that same bound. These application limits complement ZITADEL's code verification protections. Do not trust forwarded client addresses except through the existing trusted-proxy policy.

Use Glazed fields and secret-file configuration, not new environment lookups:

- `--email-signin-enabled`: explicit feature switch, false until the real round trip passes.
- `--zitadel-login-token-file`: mounted dedicated service credential, never printed.
- Existing issuer URL and organization selection: reuse the configured ZITADEL identity settings; make organization ID explicit if presently only embedded in the OIDC scope.

One small HTTP client uses the configured issuer as its fixed base URL, bearer authentication, bounded response bodies, request contexts and timeouts. It only needs get-user, create-session, check-email, get-session and delete-session methods. It must not follow arbitrary redirects carrying the credential. Use a shared request deadline for issuance rather than allowing every upstream call to consume a full independent timeout. Preserve a uniform generic response policy for admitted valid addresses; record provider failure privately and let the user request again.

ZITADEL owns SMTP and code generation. Francois has no SMTP username/password flags and generates no independent mailbox-authentication token. The random flow value binds and locates our login attempt; it cannot authenticate without a successful provider email check and browser binding.

## 8. File-level implementation plan

| File | Concrete change |
| --- | --- |
| `pkg/catalog/identity/email.go` (new) | Typed bounded Session API client and response validation |
| `pkg/catalog/email_signin.go` (new) | Four handlers, browser binding, limits and provider orchestration |
| `pkg/catalog/routes.go` | Register email handlers and explicit availability |
| `pkg/catalog/auth.go` | Empty-ID-token logout branch; reuse existing cookies and sessions |
| `pkg/catalog/store/email_signins.go` (new) | Replace pending flow, conditional attach, atomic claim, locked final session transaction |
| `pkg/catalog/store` existing migration registration | Add one pending-flow table through existing migration mechanism |
| `pkg/catalog/store/users.go` | Lock user and revoke sessions/grants on role change |
| `pkg/catalog/store/sessions.go` | Reuse session insertion from the locked final transaction |
| `cmd/francois/cmds/catalog.go` | Glazed switch, organization and credential-file wiring |
| Existing public templates/CSS | Final form, sent, confirmation, unavailable and expired states |
| `doc/production.md` | Dedicated service credential, supporter factor enrollment, SMTP ownership and release checks |

Before editing, inspect current migration and template registration rather than inventing parallel mechanisms. The user authorized implementation after the investigation. Production changes remain separate from the local implementation and validation.

## 9. Validation and shipping criteria

The useful tests exercise trust boundaries, not each line of the client:

- Fake provider returns a session with only a user factor: login must fail.
- Wrong subject, wrong organization, stale/missing email factor, changed email and disabled provider user: login must fail.
- Unknown/ambiguous/staff account requests: generic acknowledgement and no provider email.
- GET link preview, including an automated mail scanner: no code verification or local session.
- Parallel POST redemption: exactly one claimant and at most one local session.
- Replacement while a send is in flight: late provider response cannot attach to the newer flow.
- Provider timeout/incorrect code: attempt consumed, safe retry by requesting a new link, no session.
- Promotion/disable racing finalization: no privileged email session survives.
- Empty-ID-token logout: local cookie and derived grants revoked, no hosted provider logout redirect.

Then run a **real isolated** ZITADEL v4.16.1 email flow with a test user and a test mail sink. Enroll the factor, create the session without any password check, retrieve the delivered link from the mailbox, and complete the actual portal confirmation. Verify the same preexisting local user ID, existing instance ownership and entitlement behavior. Confirm provider challenge reuse fails and no temporary provider session remains active after completion. Never save service credentials, OTP codes or live links in ticket evidence; record redacted outcomes and screenshots.

Run affected Go tests plus repository-required checks. The isolated round trip now passes. Production enablement still requires its dedicated credential, functioning SMTP and explicit supporter factor enrollment; no production configuration was changed.

## 10. Why this is the smallest suitable integration

OIDC completion through `POST /v2/oidc/auth_requests/{id}` is available if we build a login UI serving multiple applications. It would require provider application login-URL configuration and an OIDC authorization context in addition to the email flow. Our supporter sign-in only serves Francois, so none of that is required now. The existing staff OIDC path remains intact.

The result is a Final email screen, a short-lived browser-bound pending record and several server-side calls to the identity provider we already run. ZITADEL performs the actual email authentication; Francois keeps the account/session/access responsibilities it already has.
