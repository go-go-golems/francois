---
Title: Email-only signup implementation guide
Ticket: FRANCOIS-013
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/email_signin.go
      Note: Shared request and confirmation flow
    - Path: repo://pkg/catalog/email_signup_test.go
      Note: Signup, recovery, collision and concurrency regression coverage
    - Path: repo://pkg/catalog/identity/email.go
      Note: ZITADEL verification and OTP client
    - Path: repo://pkg/catalog/identity/email_signup.go
      Note: Registration-owned provider identity and mailbox verification
    - Path: repo://pkg/catalog/store/email_registrations.go
      Note: Reservations, one-time claims and atomic account/session provisioning
    - Path: repo://pkg/catalog/store/users.go
      Note: Identity provisioning and email collision checks
ExternalSources: []
Summary: Passwordless email signup through the existing sign-in form, using ZITADEL email verification before local account creation.
LastUpdated: 2026-10-17T19:05:11.836226869-04:00
WhatFor: ""
WhenToUse: ""
---

# One email onboarding and sign-in flow

## Product behavior

The shared sign-in page offers Google, GitHub and email. Google (including Gmail accounts) and GitHub continue using the existing OIDC endpoints, provider routing, callback and sessions. Their buttons, native-provider fallback and role-neutral presentation remain. Only the email implementation changes.

Email has one workflow: enter an address, receive a link, open it in the requesting browser and press Continue. Confirmation signs into the matching local account or creates an ordinary account when signup is enabled. A person should not need to know whether ZITADEL already has their identity, Francois already has their account, or an email OTP factor exists.

This design replaces the previous split registration/session-OTP design. It introduces no compatibility adapter, queue, worker, mail service or alternate authentication UI. Existing provider factors remain untouched. Unconfirmed old links expire as part of the migration; account data and existing application sessions are preserved.

## Identity and proof

The canonical identity is issuer plus subject. Email locates a candidate identity, but a fresh provider-validated mailbox code establishes the proof needed to sign in. A local record's stale verification flag does not block obtaining that proof. Disabled accounts and identities in another organization remain ineligible, and multiple matching provider identities remain ambiguous.

ZITADEL v4.16.1 provides `POST /v2/users/{id}/email/send` to generate a fresh verification code and send the custom URL. Its command implementation does not require the address to be currently unverified or an OTP factor to be enrolled. `POST /v2/users/{id}/email/verify` checks that code. Use these two operations for every email login, including an already-verified provider account. Keep the existing server-side dedicated IAM_LOGIN_CLIENT credential and SMTP configuration.

The full local integration must include a provider identity that is verified, has no email OTP factor and has no local account. That reproduces the actual blocked-user case. A returning local account and a completely new address use the same proof operations.

## Resolve or create the provider identity

Search the configured organization for the normalized email with a result limit of two. Validate the chosen identity by its returned ID, resource owner, human type, email and active/initial state. Fetch the exact subject before issuance to avoid relying only on search-index profile data.

- One match: use that subject, irrespective of whether this application created it.
- Zero matches with signup enabled: create an unverified human whose username is the email. Request `returnCode:{}` to suppress the automatic creation message; discard the returned code. The normal `/email/send` operation sends the one actual login message.
- Zero matches with signup disabled: acknowledge generically without creating an identity.
- Multiple matches, inactive identity or a mismatching address/organization: fail closed.
- Concurrent creation conflict: search again once and use the unique matching identity. Mailbox proof is still mandatory; no provider accounts are merged.

There is no durable random-subject reservation. If creation succeeds but the response is lost, a retry discovers the identity through the same organization-scoped lookup. Provider username uniqueness handles overlapping creations; bounded re-resolution handles a conflict without a retry scheduler.

```text
resolve(email, signup_enabled):
    matches = search_organization_by_exact_email(email, limit=2)
    if exactly one: read and validate its subject; return identity
    if multiple: reject ambiguity
    if signup disabled: return unavailable
    create unverified human, suppress automatic email
    if creation conflicted: resolve once more without creating again
    read and validate identity; return identity
```

## One pending-link table

Migration 014 drops the obsolete pending `email_signins` and `email_registrations` tables and creates `email_links`. Keep historical migration files unchanged so existing migration checksums remain valid. These old tables hold only pending attempts/reservations; removing them does not remove users, sessions, artifacts or provider identities.

Each `email_links` row contains a flow digest, browser-binding digest, issuer, subject, email snapshot, expected role, trusted portal origin, safe return path, creation/expiry timestamps and a ready flag. It contains no provider session ID, factor enrollment state or plaintext mailbox code.

The lifetime is fifteen minutes. Insert an unready candidate after resolving the provider identity and checking local eligibility. Send the provider email, then mark that candidate ready. A failed send deletes only its own candidate and refunds its recipient quota reservation. It does not delete another local attempt before trying to send. ZITADEL itself controls replacement of verification codes; use the most recently received email.

GET confirmation checks a ready, unexpired link with the matching origin and browser digest and renders Continue. It does not validate or consume the provider code. POST atomically deletes and returns that link, validates the provider code, rechecks the provider identity and completes the local session transaction. Incorrect or consumed links require a fresh request.

## Local account reconciliation

The issuance and completion operations coordinate with ordinary OIDC provisioning using the existing normalized-email transaction lock. Lookup the local account by issuer and subject, not solely by an email flag. Reject a different local identity that owns the requested email; no automatic merging is added.

At issuance, record the current role for an existing enabled non-guest account, or the ordinary subscriber role if no local account exists. A new local account requires signup to be enabled. No local user is inserted before mailbox proof.

At completion, lock the local account if one now exists. A concurrent Google/GitHub callback may have provisioned the same identity; that is a successful reconciliation case, not a collision. Preserve its role, require it to match the expected role and reject disabled/guest accounts. Refresh its email, verified flag and provider display name after the fresh proof. If it remains absent, require signup still enabled and create the ordinary account. Insert the application session in the same transaction and remove other pending links for that identity.

Role changes invalidate pending links by issuer/subject, including links issued before local account creation. Existing user/session revocation semantics stay intact. Disabling an account also removes pending links. The account's privileges and paid access are never inferred from the email or changed by onboarding.

```text
complete(proven_identity, claimed_link, signup_enabled):
    begin transaction; lock normalized email
    reject different local identity owning this email
    account = select issuer/subject for update
    if absent:
        require signup enabled and expected ordinary role
        insert ordinary verified account
    else:
        require enabled, non-guest and unchanged expected role
        refresh email/name/verification without changing role
    insert application session
    delete remaining links for issuer/subject
    commit; set session cookie; redirect safely
```

## Timing, delivery feedback and limits

Remove the forced three-second acknowledgement wait. Provider work gets a bounded fifteen-second request context; individual HTTP calls remain bounded. Return as soon as issuance succeeds or fails. A provider failure displays a retryable send error, not an apparently successful check-email page. Ineligible/ambiguous addresses retain a generic acknowledgement so the form does not disclose account administration state.

Keep the existing process/client attempt limits (100/hour and 20/hour). Allow ten successful messages per recipient/hour. Reserve the recipient slot during provider work and refund that exact reservation if no usable link was issued. This avoids both unbounded concurrent sends and charging provider failures against the user's message allowance. Limit exhaustion gets a clear retry response. No distributed limiter is introduced for the current single-replica site.

The browser-binding cookie and CSRF checks remain. The page clearly tells the user to open the link in the requesting browser within fifteen minutes. There is no extra code-entry screen in this launch implementation. Sensitive responses remain no-store/no-referrer, and the confirmation form remains free of scripts. Safe local return paths and exact site origins stay enforced.

## Browser and data sequence

```mermaid
sequenceDiagram
    participant B as Browser
    participant F as Francois
    participant D as PostgreSQL
    participant Z as ZITADEL
    B->>F: Email + CSRF
    F->>Z: Find unique identity or create unverified human
    F->>D: Check local identity; insert unready hashed link
    F->>Z: email/send with current confirmation URL
    F->>D: Mark link ready
    F-->>B: Check your email
    B->>F: GET mailed link in requesting browser
    F-->>B: Continue form (read-only)
    B->>F: POST Continue + CSRF + code
    F->>D: Atomically claim link
    F->>Z: Check identity; email/verify; re-read verified identity
    F->>D: Reconcile issuer/subject + insert session transactionally
    F-->>B: Session cookie and safe redirect
```

## Code and API map

| File | Final responsibility |
| --- | --- |
| `pkg/catalog/email_signin.go` | One request/inspect/confirm path; quota reservation/refund and honest send feedback. |
| `pkg/catalog/identity/email.go` | Bounded authenticated transport and shared provider identity validation. |
| `pkg/catalog/identity/email_link.go` | Organization-scoped resolve/create, `/email/send` and `/email/verify`. |
| `pkg/catalog/store/email_links.go` | One pending link record, eligibility, claim and account/session reconciliation. |
| `pkg/catalog/store/migrations/014_email_links.sql` | Replace obsolete pending state with the unified table. |
| `pkg/catalog/store/users.go` | Existing OIDC ownership guard and role/disable invalidation against the new table. |
| `pkg/catalog/web/templates/signin.tmpl` | Existing Google/GitHub/email presentation; fifteen-minute guidance. |
| `pkg/catalog/auth.go` | Existing Google/GitHub routing and OIDC callback; unchanged by email simplification. |

Provider endpoints are `POST /v2/users` (search), `GET /v2/users/{id}`, `POST /v2/users/human`, `POST /v2/users/{id}/email/send` and `POST /v2/users/{id}/email/verify`. Payloads follow the retained v4.16.1 proto. The creation response's optional verification code is deliberately ignored and never stored/logged. No `/v2/sessions` or `/otp_email` call remains in this email-login flow.

Remove the old registration handler, separate registration store, email-session store and their OTP-specific clients/tests instead of maintaining adapters. Historical ticket diaries and production reports remain historical evidence; they are not the active design.

## Implementation and one final qualification boundary

Implement the design as one coherent replacement, then validate it. Retain focused regressions for new address, existing provider-only identity, stale local verification, existing account without OTP, same-identity OIDC completion, role/disabled/collision guards, scanner GET, browser/CSRF/origin checks, expiry/replay and failed-send quota refund. Preserve Google/GitHub sign-in tests.

Run the database-backed Go suite, required lint/build checks and one real isolated-provider browser journey after implementation. The browser journey must demonstrate both a new address and an already-verified provider-only account without OTP. Do not repeatedly run broad tests after each edit. Production mail is not sent as part of local qualification.
