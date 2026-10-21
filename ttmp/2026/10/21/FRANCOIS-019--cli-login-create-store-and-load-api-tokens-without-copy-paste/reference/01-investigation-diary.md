---
Title: Investigation diary
Ticket: FRANCOIS-019
Status: active
Topics:
    - cli
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological record of the FRANCOIS-019 investigation: current token creation and loading, and the device-login design."
LastUpdated: 2026-10-21T10:26:53.536977-04:00
WhatFor: "Tracing how the FRANCOIS-019 design was derived and what evidence backs it."
WhenToUse: "When resuming FRANCOIS-019 or reviewing the design doc's claims."
---

# Investigation diary

## Step 1: Answer the question and map current token handling

The user first asked whether francois can create a device token from the CLI.
I searched the server routes, the guard and the CLI connection code and answered
no: `POST /api/tokens` is guarded with `noToken`, and the CLI only reads a token
from `FRANCOIS_TOKEN` or `--token-file`. The user then asked for a ticket and an
intern-level design guide covering creating, storing and loading tokens.

### Prompt Context

**User prompt (verbatim):** "Does francois allow creating a device token from the CLI, like ask me to login and then give me a token?"

Follow-up (verbatim): "Create a new docmgr ticketk to be able to easily create at oken (and store / load it, dunno how that's handled right now) . Create a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet points and pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and upload to remarkable."

**Assistant interpretation:** Create docmgr ticket FRANCOIS-019, investigate token creation, validation and CLI loading, write a design and implementation guide for a device login flow plus credential storage, and upload it to reMarkable. Design only, no implementation.

**Inferred user intent:** Stop copying tokens out of the browser; have `login` once per machine and have every command find the token.

### What I did

- `grep` over `cmd`, `pkg`, `ui`, `doc` for device/token/login terms.
- Read `pkg/catalog/tokens.go`, `pkg/catalog/store/tokens.go`, `pkg/catalog/auth.go`,
  `pkg/catalog/store/sessions.go`, `pkg/catalog/store/email_links.go`,
  `pkg/catalog/routes.go`, `pkg/catalog/demos.go` (limiter, client address),
  `pkg/catalog/mcp.go`, migrations `001`, `004`, `008`, `014`.
- Read the CLI: `cmd/francois/cmds/api.go` (`connect`), `push.go`, `mcp.go`,
  `api_commands.go`, `api_test.go`, and `pkg/catalog/client/client.go`,
  `types.go`, `errors.go`.
- Read the UI: `ui/catalog/tokens.tsx`, `account.tsx`, `main.tsx`, `api.ts`,
  `ui.tsx` (`loginHref`), `studio.tsx` (old Tokens tab).
- Read `README.md`, `doc/production.md` (one replica, token policy) and
  `pkg/doc/topics/01-catalog-api.md`.
- Read the FRANCOIS-007 design doc's statements on token minting and deferred OAuth.
- `docmgr ticket create-ticket --ticket FRANCOIS-019 …`, then `docmgr doc add`
  for the design doc and this diary.

### Why

- The design had to reuse the current security model (session-minted, hashed,
  scoped tokens), so I read the guard and the token handler line by line before
  proposing any endpoint.
- The `login_flows` and `email_links` tables already implement short-lived,
  hashed, claim-once flows; the device table copies that shape instead of
  inventing one.

### What worked

- Evidence for every current-state claim was found with line numbers (design doc §4).
- Existing patterns cover nearly all building blocks: `insertSession` for
  transactional credential inserts, `limiter` for rate limits, `safeReturn` for
  post-login redirects, `setup(t)` / `fakeAPI` for tests.

### What didn't work

- `grep -rniE … --include=*.go` failed under zsh with `(eval):1: no matches found: --include=*.go`; quoting the glob (`--include='*.go'`) fixed it.
- `ls -d ttmp/2026/*/*/` listed date directories, not tickets; `ls -d ttmp/2026/*/*/*/` gave the ticket names (latest FRANCOIS-018, so the new ticket is FRANCOIS-019).

### What I learned

- `releases push` has its own copy of token loading, requires `--server`, and
  ignores `FRANCOIS_SERVER` (`push.go:32`, `69-79`); its help text still points
  at "Studio → Tokens" (`push.go:30`).
- The client always sends `Authorization: Bearer <token>` even when empty
  (`client.go:112`); harmless because `guard` only reacts to the `frt_` prefix.
- FRANCOIS-007 explicitly chose "a token can never mint or revoke tokens". The
  device flow keeps that (minting needs a browser approval), but a server-side
  logout needs a narrow self-revocation exception, which is an owner decision.
- Production runs a single replica, so in-memory rate limits are acceptable.

### What was tricky to build

- Deciding when the secret is created. Creating it at approval would require
  storing a usable secret in the database until the CLI polls. Creating it at
  the first poll after approval, inside one transaction that also deletes the
  request row, keeps "hashes only" and makes the claim exactly-once. That in turn
  requires `CreateToken` to accept a transaction (`insertToken`, like
  `insertSession`) and a re-check of the approver's role at mint time.

### What warrants a second pair of eyes

- Security analysis in design doc §9, especially remote phishing mitigations and
  the self-revocation exception (§7.5).
- Whether admin scopes should be grantable through the device flow (§11 Q3).

### What should be done in the future

- Owner decisions in design doc §11, then implementation phases 0–6.

### Code review instructions

- Start with design doc §3.3 (end-to-end picture), §4 (evidence), §6.2 (API),
  §8.4 (store pseudocode). Verify the cited lines in `pkg/catalog/auth.go`,
  `pkg/catalog/tokens.go`, `cmd/francois/cmds/api.go`.

### Technical details

- Token secret: `frt_` + base64url(32 bytes); `api_tokens.token_hash` = SHA-256.
- Proposed device code `fdc_` + base64url(32 bytes); user code 8 letters from
  `BCDFGHJKLMNPQRSTVWXZ`, shown `XXXX-XXXX`, 10-minute life, 5-second interval.
- Proposed migration name: `019_device_authorizations.sql` (latest is `018_early_access.sql`).

## Step 2: Ticket bookkeeping, validation and reMarkable delivery

Recorded the design in the ticket, related the primary files, added phase tasks,
validated with `docmgr doctor`, and uploaded the design guide to reMarkable.

### Prompt Context

**User prompt (verbatim):** (see Step 1)
**Assistant interpretation:** Store the guide in the ticket and upload it.
**Inferred user intent:** Read the guide on the reMarkable.

### Evidence and commits

- `docmgr doc relate` (7 files on the design doc, 2 on the index), 8 tasks added, changelog entry.
- `docmgr doctor --ticket FRANCOIS-019 --stale-after 30` → "All checks passed".
- `remarquee upload bundle … --name "FRANCOIS-019 CLI Token Login Guide" --remote-dir "/ai/2026/10/21/FRANCOIS-019" --toc-depth 2 --non-interactive` → `OK: uploaded FRANCOIS-019 CLI Token Login Guide.pdf -> /ai/2026/10/21/FRANCOIS-019` (cloud upload result; no device listing performed).
- No commit made; ticket files are uncommitted in the working tree.

### Noteworthy decisions or failures

- `docmgr changelog update … --file-note "$R/$D:Primary …"` failed under zsh: `$D:P` was parsed as the zsh `:P` path modifier, producing `malformed --file-note value …`. Re-ran with `${PWD}/${D}:Primary design deliverable`.

### Remaining requirements and next action

- Owner decisions in design doc §11 (task mef6), then phases 0–6 (tasks m14m … bos5).

## Step 3: Owner decisions, implementation setup and baseline tests

The owner answered the design's open questions and asked for implementation with
commits at sensible points and a detailed diary. I recorded the decisions in the
design doc (section 11), created the feature branch, started a throwaway
Postgres for the integration tests, and ran the baseline suite. The session hit
its usage limit before any implementation code was written.

### Prompt Context

**User prompt (verbatim):** "1. yes\n2. auth login\n3. admin scopes can be granted, default scopes should be the max for the user kind\n4. yes\n\nImplement, btw commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)."
**Assistant interpretation:** Accept self-revocation, name the group `auth login`, allow admin scopes with role-maximum defaults, refuse terminal printing in `auth token`; implement phases 0-6 with commits and a diary.
**Inferred user intent:** Ship the device login and credentials store.

### What I did
- Recorded the decisions in design doc section 11.
- `git checkout -b feat/cli-auth-login`.
- `docker run -d --name francois-019-pg -e POSTGRES_USER=francois -e POSTGRES_PASSWORD=francois -p 127.0.0.1:55439:5432 postgres:17`.
- Baseline: `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:55439/postgres?sslmode=disable' GOWORK=off go test ./... -count=1`.

### Why
- A baseline separates existing failures from regressions this ticket might introduce.

### What worked
- Token tests (`TestTokensAndPush`, `TestTokenScopeMatrix`, `TestTokenMintingRules`, `TestAuditRecordsToken`, `TestAdminTokenOversight`) pass; all packages pass except one.

### What didn't work
- Existing failure, unrelated to this ticket: `pkg/catalog/mcpserver` `TestLocalPathAllowDirs` (`files_test.go:31`) returns `/private/var/folders/...` where `/var/folders/...` is expected (the macOS `/var` -> `/private/var` symlink).
- `golangci-lint` is not on PATH; `~/go/bin/golangci-lint` v2.12.2 matches `.golangci-lint-version`. Lefthook hooks are not installed in `.git/hooks`.
- `node_modules` is missing; `pnpm install` is needed before the UI build (built `pkg/catalog/web/static/catalog.js` is committed).

### What I learned
- New packages under `pkg/` need a generated `logcopter.go` (`go generate ./...`, checked by `make logcopter-check`).

### What was tricky to build
- N/A (no code yet).

### What warrants a second pair of eyes
- N/A.

### What should be done in the future
- Implement phases 0-6 on `feat/cli-auth-login` (tasks m14m ... bos5). Stop the `francois-019-pg` container when finished.

### Code review instructions
- Design doc section 11 for the recorded decisions.

### Technical details
- Test database: `postgres://francois:francois@127.0.0.1:55439/postgres?sslmode=disable` (container `francois-019-pg`).
