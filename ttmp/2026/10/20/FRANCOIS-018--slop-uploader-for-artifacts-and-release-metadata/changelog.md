# Changelog

## 2026-10-20

- Initial workspace created

## 2026-10-20

Archived supplied uploader references and wrote the detailed intern implementation guide. Proposed one Studio form using existing mutations, explicit public-sample policy and partial-failure recovery; implementation remains open.

## 2026-10-20

Validated the ticket and local links; uploaded the guide to reMarkable cloud at /ai/2026/10/20/FRANCOIS-018. Application implementation tasks remain open.

## 2026-10-20 — Implementation and verification

Implemented `/studio/upload`, reusable Studio Markdown/media fields, complete upload limits and sequential recoverable publication. Added frontend orchestration tests and a browser interaction/screenshot script. PostgreSQL catalog tests, full Go suite, TypeScript/build, lint and generated-contract checks pass. See diary Steps 3–4 and `sources/implementation/browser.json`.

## 2026-10-20 — Production rollout

PR #18 merged as 21b8458. Rebased locally and deployed image sha-21b8458 through infrastructure PR #374 after successful backups and CI validation. Argo Synced/Healthy; both public domains serve exact merged assets. See diary Step 10 and sources/production-rollout.json.
