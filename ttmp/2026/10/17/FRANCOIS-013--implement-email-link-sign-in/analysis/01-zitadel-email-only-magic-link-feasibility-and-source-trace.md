---
Title: ZITADEL email-only magic-link feasibility and source trace
Ticket: FRANCOIS-013
Status: active
Topics:
    - backend
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/auth.go
      Note: Existing portal sessions and staff OIDC flow
    - Path: repo://ttmp/2026/10/17/FRANCOIS-013--implement-email-link-sign-in/sources/zitadel-email-login/source-manifest.json
      Note: Version pinned provider authentication evidence
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T16:16:42.256342148-04:00
WhatFor: ""
WhenToUse: ""
---
# Finding: email-only magic links are feasible with the Session API

The hosted login screen does not provide the requested ready-made journey in the sources inspected. That does not mean ZITADEL cannot authenticate it. Its custom Session API can issue an email challenge without a password, verify the resulting code and record an email factor. The implementation guide now selects this API directly for Francois supporter authentication.

## Evidence hierarchy

1. Deployed image metadata: ZITADEL and login application v4.16.1. This was a read-only cluster query, not a configuration export.
2. Version-pinned source: cc3812a940d629984802fd1246f84fd91d07f6e8. This proves API shape and implementation prerequisites for that release.
3. Official guides: custom login supports own-application sessions without OIDC plumbing. The MFA guide describes email checks but contains an obsolete boolean returnCode example relative to the pinned release.
4. GitHub discussion #2075: November 2025 reply recommends Session API/custom UI; a March 2026 comment reports a working implementation. These corroborate feasibility but supply no complete tested recipe.

## Source map

All paths below are relative to `../sources/zitadel-email-login/`.

| Saved source | What to inspect |
| --- | --- |
| `01-magic-links-discussion.md` | Distinguishes classic email magic links from FIDO enrollment links; recommends custom UI |
| `02-custom-login-oidc.md` | Own-app login does not require OIDC; also describes optional auth-request finalization |
| `03-email-otp-and-mfa.md` | Challenge/check workflow; compare wire format to pinned proto |
| `04-create-session.md` | Session creation and response contracts |
| `05-hosted-login.md` | Hosted authentication capabilities; does not establish email-only configuration |
| `v4.16.1/proto/zitadel/session/v2/challenge.proto` | otpEmail.sendCode.urlTemplate; Code placeholder; 200-character URL limit |
| `v4.16.1/proto/zitadel/session/v2/session_service.proto` | POST, GET, PATCH and DELETE routes; bounded session lifetime |
| `v4.16.1/internal/command/session_otp.go` | Enrollment requirement; no password prerequisite; code validation |
| `v4.16.1/internal/command/session_model.go` | Email-factor timestamp and challenge clearing |
| `v4.16.1/internal/api/grpc/session/v2/session.go` | Request-to-command conversion and sendCode oneof handling |
| `v4.16.1/internal/command/auth_request.go` | Optional linking of a session to an OIDC request |
| `v4.16.1/internal/command/auth_request_model.go` | OIDC state checks; does not itself enforce a password requirement |
| `v4.16.1/internal/api/oidc/token_code.go` | Code exchange checks for client, PKCE, redirect and request state |
| `v4.16.1/defaults.yaml` | IAM_LOGIN_CLIENT includes session read/write/delete; ORG_USER_MANAGER lacks session.write |

`manifest.json` records retrieval times, URLs, byte counts and SHA-256 digests for raw pages. `source-manifest.json` identifies pinned upstream source URLs and digests. Raw HTML is retained beside readable Defuddle Markdown. The latter is a convenience extraction, not an independently authoritative document.

## Chosen integration and rejected extra machinery

The Go service performs Session API calls using a dedicated login service credential. It requests a link pointing to its own scanner-safe confirmation page. On POST, it claims a browser-bound pending flow, verifies the code and requires a fresh email factor for the saved subject and organization. Only then does it create a standard portal session for the same existing local user.

We do not need OIDC authorization-request finalization for this same-application flow. Retaining the normal OIDC callback for magic links would require configuring a custom application login URL and managing its auth-request context. That adds work without changing the existing local session/access model. Staff retain the existing OIDC entry point.

An important correction to the earlier conversation is that custom Session API login is itself trusted authentication code. Keeping ZITADEL as the code verifier does not mean the hosted login's password/MFA screens or all its policy decisions run automatically. Francois must explicitly enforce eligibility, the successful email factor, same-user identity and privileged-account exclusion. A checked user factor alone must never authorize a local session.

## Required runtime qualification

No test email or login was sent to production and no production users, factors, policies, credentials or sessions were changed. The next gate is an isolated v4.16.1 user, dedicated service credential and test mailbox: enroll email OTP, request a challenge without a password, consume the delivered link in Francois and verify the same local user/session. Source inspection proves a concrete implementation path; SMTP delivery, permissions and the complete browser flow still require that test.

The current intern implementation guide owns the complete route/table/API/test design. The superseded first-party guide is archived; the earlier reMarkable PDF describes that older guide and has not been refreshed by this investigation.
