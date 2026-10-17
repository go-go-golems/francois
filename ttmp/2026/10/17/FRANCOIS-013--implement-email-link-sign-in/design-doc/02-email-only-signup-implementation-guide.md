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
    - Path: repo://pkg/catalog/identity/email.go
      Note: ZITADEL verification and OTP client
    - Path: repo://pkg/catalog/store/users.go
      Note: Identity provisioning and email collision checks
ExternalSources: []
Summary: Passwordless email signup through the existing sign-in form, using ZITADEL email verification before local account creation.
LastUpdated: 2026-10-17T18:56:54.620052047-04:00
WhatFor: ""
WhenToUse: ""
---

# Email-only signup implementation guide

## Behavior and scope

The public email form must accept a new address as well as an existing account. A new visitor enters an address, receives a link, opens it in the requesting browser and presses Continue. Confirmation establishes mailbox ownership and creates an ordinary local account and session. There is no password, second signup screen, background worker or separate mail service. ZITADEL sends the message through its configured SMTP provider.

The existing `SignupAllowed` setting gates new email registrations and confirmation, just as it gates public social signup guidance. Existing email sign-in remains available when signup is disabled. New accounts have the database's ordinary `subscriber` role; that role alone grants neither payment entitlement nor staff privileges.

## Existing components

`pkg/catalog/email_signin.go` owns the GET sign-in page, POST request and GET/POST verification endpoints. The GET verification handler only renders a confirmation form; it cannot consume a code. Double-submit CSRF and a host-only HttpOnly cookie bind both request and confirmation to the browser. Every page containing authentication material is served with no-store and no-referrer headers. The return destination is restricted by `safeReturn`.

`pkg/catalog/identity/email.go` encapsulates the privileged server credential and ZITADEL API calls. Existing users authenticate using an enrolled email OTP factor in a temporary ZITADEL session. `pkg/catalog/store/email_signins.go` stores only hashed local flow/browser secrets and an opaque provider session ID, and rechecks the account under a user lock before issuing a session. Those existing safeguards stay in force.

New email addresses cannot use that OTP mechanism immediately: ZITADEL requires a verified email before email OTP can be enrolled. Registration therefore starts with ZITADEL's email-verification code. Successful verification is the mailbox proof for this first session. Only after it succeeds may the application enroll email OTP for subsequent sign-ins.

## Data model

Add migration `013_email_registrations.sql` with one `email_registrations` table. A row reserves a random provider subject for a normalized address and issuer. Its unique `(issuer,email)` key makes retries reuse the same subject instead of creating an identity per click. This row is not a local authenticated user and has no permissions or entitlements.

The same row holds the current pending attempt: flow digest, browser-binding digest, origin, return path, creation/expiry timestamps, proof kind (`verification` or `otp`), optional provider session ID and a ready flag. Replacing the attempt invalidates the old link. Claiming it atomically clears its flow digest, permitting one verification attempt. The row remains as the provider-identity reservation so a failed or expired attempt can be retried safely.

- No raw link token, provider bearer token or email code is stored.
- The expiry is five minutes, checked locally even if the provider's verification code lasts longer.
- A ready flag distinguishes a completed send operation from a reserved/incomplete attempt.
- The provider subject is application-generated and never accepted from the request.
- Durable reservations remain until deliberately removed; there is no cleanup worker. Global and per-address request limits bound creation, as with provider accounts themselves.

## Request algorithm

```text
normalize and validate address; check CSRF and existing request limits
look up all local users with this address (including disabled/unverified)
if any exist:
    require exactly one enabled, verified, matching-issuer human account
    use the existing email-OTP sign-in path
else:
    require SignupAllowed
    reserve/reuse random provider subject for (issuer, address)
    replace pending browser-bound attempt with new five-minute flow
    inspect only the reserved provider subject
    if absent:
        check organization-scoped provider email search for collisions
        create an unverified human with the reserved ID and custom email URL
    else if its email remains unverified:
        check subject, organization, address and usable state
        resend verification with the new URL
    else:
        check active matching verified identity
        ensure email OTP is enrolled on this owned registration identity
        send the existing temporary-session OTP challenge
    attach proof kind/session only if the flow is still current
acknowledge identically for unknown, existing, blocked and failed requests
```

An existing provider identity discovered by email is never adopted or linked. The lookup is a collision guard, not an authentication decision. Such users can use the existing Google/GitHub/native provider flow, which provisions the local user by issuer and subject. Disabled or ambiguous local records cannot fall through into signup. Creation conflicts fail closed; the next request can retry the reserved identity.

A provider user can become verified before a local transaction finishes. On retry, its durable reservation permits using OTP for a fresh proof instead of attempting to verify an already verified email again. OTP enrollment is limited to the exact identity reserved by this signup flow, after the provider reports its email verified. Existing unrelated human accounts are not enrolled automatically.

## Confirmation algorithm

```text
validate CSRF, browser cookie, flow, origin and expiry
try existing sign-in claim; otherwise claim a ready registration
require signup still enabled
if verification proof:
    POST reserved subject/email/verify with supplied code
else:
    verify temporary session's fresh matching email factor
re-read provider user: active, same subject, organization and verified email
ensure email OTP is enrolled for this registration-owned identity
transaction:
    serialize local identity provisioning for the normalized email
    reject another local identity using that email
    insert ordinary verified local user by issuer+subject (or preserve existing role)
    reject disabled/guest accounts
    insert portal session and update last-login time
commit, clear browser-binding cookie, set portal-session cookie, redirect safely
```

The local provisioning transaction shares an email advisory lock with `UpsertIdentity`, the ordinary OIDC provisioning method. This small common critical section prevents a concurrent social callback and email signup from creating two local identities for the same address during registration completion. It does not introduce email-based identity merging. A matching issuer/subject preserves the existing role, and the user row is locked while the session is issued.

```mermaid
sequenceDiagram
    participant B as Browser
    participant F as Francois
    participant D as PostgreSQL
    participant Z as ZITADEL
    participant M as Mailbox
    B->>F: POST email + CSRF
    F->>D: Reserve subject; replace hashed pending flow
    F->>Z: Create unverified human + custom verification URL
    Z->>M: Verification email through existing SMTP
    F->>D: Mark current flow ready
    F-->>B: Generic check-email page
    M-->>B: Open link in same browser
    B->>F: GET link
    F-->>B: Confirmation form (no code consumption)
    B->>F: POST Continue + CSRF + code
    F->>D: Atomically claim current flow
    F->>Z: Verify email; check identity; enroll email OTP
    F->>D: Transaction: create ordinary user + session
    F-->>B: Session cookie + safe redirect
```

## API contracts and source references

These are the pinned ZITADEL v4.16.1 REST contracts, not assumptions based on a newer deployment. Existing service credentials stay server-side; no browser receives management access.

| Operation | Endpoint | Contract |
| --- | --- | --- |
| Read reserved identity | `GET /v2/users/{id}` | Check ID, state, resource owner and email |
| Check collision | `POST /v2/users/_search` | Organization ID and case-insensitive exact email; bounded results |
| Create unverified human | `POST /v2/users/human` | Explicit reserved `userId`, organization, minimal profile and `email.sendCode.urlTemplate`; never `isVerified:true` |
| Resend verification | `POST /v2/users/{id}/email/resend` | `sendCode.urlTemplate` points to existing application confirmation route |
| Verify address | `POST /v2/users/{id}/email/verify` | `verificationCode` from the claimed browser-bound attempt |
| Inspect factors | `GET /v2/users/{id}/authentication_methods` | Look for `AUTHENTICATION_METHOD_TYPE_OTP_EMAIL` |
| Enroll email OTP | `POST /v2/users/{id}/otp_email` | Only after verified ownership, on the reserved identity |
| Future sign-ins | `POST/PATCH/GET/DELETE /v2/sessions` | Existing fresh-email-factor proof and cleanup |

The human creation API requires given/family names, although the form collects only email. Use a neutral profile (`New`, `User`, display name `New user`) rather than inventing personal information. The API limits email to 200 characters; align form/server validation with that contract.

Primary references: [custom registration](https://zitadel.com/docs/guides/integrate/login-ui/username-password), [email verification](https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.VerifyEmail), [email OTP prerequisite](https://zitadel.com/docs/reference/api/user/zitadel.user.v2.UserService.AddOTPEmail). Pinned proto and command sources are retained under this ticket's `sources/zitadel-email-login/v4.16.1` and signup-specific source folder.

## Implementation and validation

1. Extend the bounded provider client with reserved-user creation, resend, verification and idempotent OTP enrollment. Preserve status-only error handling and redirect refusal.
2. Add the reservation/flow migration and store methods; share a normalized-email lock with ordinary identity provisioning.
3. Branch the existing request and verification handlers into signup only when no local address exists and signup is enabled. Reuse rendering, cookie handling, limits, expiry and redirects.
4. Update the form guidance to explain that email can create an account, with identical acknowledgement for all outcomes.
5. Test real PostgreSQL claims, replacement, expiry, disabled/collision behavior, concurrent completion and role preservation. Mock-provider tests cover proof ordering, wrong organization/address, failed codes and recovery after partial completion.
6. Exercise the full first-signup and second-login sequence against the isolated pinned ZITADEL and Mailpit stack. Assert that no local user exists before confirmation and one ordinary user/session exists afterward. Confirm scanner GETs send/verify nothing and replay fails.
7. Run the full database-backed Go suite, build and required lint. Record exact failures and repairs in the ticket diary. Commit design and implementation at meaningful boundaries.

No production credentials, signup mail or deployment changes are required to implement and validate this locally. Production rollout must include the migration and the existing signup flag, and operator documentation must distinguish local proof from actual SMTP delivery.
