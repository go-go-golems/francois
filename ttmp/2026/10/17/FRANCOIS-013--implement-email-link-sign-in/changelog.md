# Changelog

## 2026-10-17

- Initial workspace created

## 2026-10-17

Wrote a standalone intern guide for four email sign-in routes, one pending-link table, bounded direct mail and reuse of existing verified users/sessions. Added two rendered diagrams and concrete identity/session tests.

## 2026-10-17

Validated guide and diagrams; uploaded Email Sign In Intern Guide PDF to /ai/2026/10/17/FRANCOIS-013 with OK uploaded receipt after network-enabled retry. Guide revision 84da5b6; runtime tasks remain open.

## 2026-10-17

Confirmed ZITADEL v4.16.1 email-only Session API path; saved primary sources and replaced local-token design with provider-verified magic links. Runtime email qualification remains open.

## 2026-10-17

Implemented ZITADEL-backed magic links, Final forms, transactional flow/session controls and role revocation; real isolated v4.16.1 SMTP round trip passed. Full-suite and viewport validation follow.

## 2026-10-17

Implemented and qualified ZITADEL email-only sign-in with real isolated v4.16.1 SMTP, Chromium desktop/mobile flow, concurrency security tests and full checks; default disabled in production.

## 2026-10-17

Read-only remote verification found active SES SMTP and existing Vault credentials; documented actual sender/ownership without exposing secrets or sending mail.

## 2026-10-17

Provisioned dedicated production ZITADEL login identity/PAT in Vault; enabled the deployed form via Terraform #35 and k3s #364. Session/VSO/form checks pass. Existing supporter OTP enrollment was rejected by automatic approval review and remains pending explicit authorization.

## 2026-10-17

With explicit user approval, enrolled the single existing verified supporter in email OTP and verified provider readback plus preservation of existing authentication methods. No staff/subscription changes or login-email requests.

## 2026-10-17

Implemented shared Google/GitHub/email sign-in presentation and allowlisted ZITADEL provider routing; full Go tests/build/lint pass. Prepared bounded social-signup Terraform plan; production rollout pending.

## 2026-10-17

User approved public signup; applied/merged Terraform #36 (three in-place changes). Enabled role-neutral email eligibility with role-change race checks and fixed review P1 with a shared native-account fallback. Final Go suite/build/lint pass.

## 2026-10-17

Deployed reviewed shared sign-in source #12 (5721ee8) via k3s #366. Public social signup approved/applied through Terraform #36. Both real provider login forms, scopes/PKCE, shared page, signup availability and hidden-blog default verified; no real-user consent, account creation or email request submitted.

## 2026-10-17

Implemented email-only signup through the shared form with unverified ZITADEL registration, mailbox confirmation, OTP enrollment, atomic ordinary-account/session provisioning and browser-bound one-shot links. Full Go/database suite and build passed; final affected tests, lint/logger and actual pinned-provider first-signup/repeat-login journey passed. Detailed design, diary and sanitized browser evidence retained; no production deployment.

## 2026-10-17

Deployed email-only signup on k3s: source PR #13 / 5068ad1, official release 39078802326, validated image pin PR #369 / 18cdb9d. Verified exact image, ready replica, Synced/Healthy/Succeeded Argo, migration 013 and both public sites without sending production mail. Published the detailed 5,400-word textbook-style report with five figures and three diagrams to go-go-parc; committed and pushed vault main as 2731f8c.

## 2026-10-17

Replaced split email registration/OTP login with one ZITADEL resolve/send/verify flow and one pending-link table; Google/GitHub SSO unchanged. Existing provider-only and stale local accounts reconcile after fresh proof; same-identity SSO races succeed. Added fifteen-minute links, bounded realistic sending, honest provider errors and successful-send quota refunds. Full Go/database suite, build, lint/logger and one real local new/provider-only onboarding+return journey passed; no production mutation.
