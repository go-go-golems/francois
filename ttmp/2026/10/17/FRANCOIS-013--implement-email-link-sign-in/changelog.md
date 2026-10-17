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
