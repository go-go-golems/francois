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

## Step 4: Phase 0 refactors (no behavior change)

Resumed after the usage limit. Phase 0 prepares the code base: the token rules
of `POST /api/tokens` became a reusable function, the store can mint a token
inside a transaction, the client stops sending an empty bearer header, and
`releases push` now uses the shared connection code. All existing tests pass
unchanged, which was the acceptance criterion.

### Prompt Context

**User prompt (verbatim):** "continue" (resuming the implementation request in Step 3)
**Assistant interpretation:** Continue implementing phases 0-6 with commits and diary entries.
**Inferred user intent:** Finish the feature.
**Commit (code):** 3c36520 — "refactor(catalog): share token grant rules, transactional token insert, push uses connect"

### What I did
- `pkg/catalog/tokens.go`: new `grantError` and `tokenGrant(role, scopes, days)`; `createToken` keeps only its name check and calls `tokenGrant`.
- `pkg/catalog/store/tokens.go`: `rowQuerier` interface (`var _ rowQuerier = (pgx.Tx)(nil)`), `insertToken(ctx, db, …)`; `CreateToken` delegates with `s.Pool`.
- `pkg/catalog/client/client.go`: set `Authorization` only when the token is non-empty.
- `cmd/francois/cmds/push.go`: removed the `server`/`token-file` flags and the copied token loading; appended `connectionFlags()` and call `connect(in{p})`; new help text.
- `pkg/catalog/token_grant_test.go`: table test for `tokenGrant` (no database).

### Why
- Device approval and device minting must apply exactly the account page's rules; one function prevents drift.
- Minting at poll time requires "insert token + delete request" in one transaction (design §7.3).

### What worked
- `GOWORK=off go test ./cmd/... ./pkg/catalog/ ./pkg/catalog/client/ -count=1` with `TEST_DATABASE_URL` → all ok (catalog 18.0 s).
- `~/go/bin/golangci-lint run ./cmd/... ./pkg/catalog/...` → `0 issues.`

### What didn't work
- First build after the push edit:
  ```
  cmd/francois/cmds/push.go:6:2: "strings" imported and not used
  cmd/francois/cmds/push.go:65:12: cannot use 1st function result (value of type *"github.com/go-go-golems/francois/pkg/catalog/client".Client) as *CatalogPush value in multiple assignment
  cmd/francois/cmds/push.go:81:14: c.Studio undefined (type *CatalogPush has no field or method Studio)
  ```
  The method receiver is named `c`; renamed the local client to `api` and dropped the import.

### What I learned
- `push` was the only command not built through `single()`, which is why it had drifted.

### What was tricky to build
- Nothing substantial; keeping `createToken`'s original combined name/days error message preserved the existing test expectations.

### What warrants a second pair of eyes
- `releases push --server` is no longer required. That is a relaxation (it now also reads `FRANCOIS_SERVER`), not a break.

### What should be done in the future
- N/A

### Code review instructions
- `git show 3c36520`; run `go test ./pkg/catalog/ -run 'TestToken'` with `TEST_DATABASE_URL`.

### Technical details
- `tokenGrant` error codes: `invalid_request` (days), `tokens_unavailable` (role without scopes), `invalid_scopes`.

## Step 5: Phase 1, credentials store and `catalog auth` commands

This phase delivers "store and load" on its own: a token pasted once with
`francois catalog auth login --with-token` is kept per server and used by every
hosted-API command without flags or environment variables. Browser approval
(the device flow) is still missing; `auth login` without `--with-token` reports
that for now.

### Prompt Context

**User prompt (verbatim):** (see Step 4)
**Assistant interpretation:** Implement design phase 1.
**Inferred user intent:** Stop exporting tokens by hand.
**Commit (code):** c5240b0 — "feat(cli): store logins per server and add catalog auth commands"

### What I did
- New package `pkg/catalog/credentials`: `Dir`, `Path`, `Load` (missing file = empty, permission warning, rejects newer versions), `Save` (0700 dir, temp file + `Sync` + `chmod 0600` + `rename`), `File.Put/Remove`, `NormalizeOrigin`, `ResolveServer`, `Resolve` with `Source` values.
- `cmd/francois/cmds/api.go`: `connect` now calls `credentials.Resolve`, prints warnings, fails early on an expired stored token, warns when it expires within 72 h; new `hostOption` and `loginHint` (wraps `token_invalid`). Applied `loginHint` in `apiCommand`, `push` and `mcp`.
- `cmd/francois/cmds/auth.go`: `auth login --with-token` (hidden prompt on a TTY, else stdin; must start with `frt_`; verified with `GET /api/me`), `auth status [--check]` with a `used` column, `auth token` (refuses a terminal without `--yes`, decision 4), `auth logout [--all]`.
- Tests: `credentials_test.go` (normalization table, round trip, modes, no temp leftovers, version guard, precedence table), `auth_test.go` (login → whoami → status → token → env override → logout; bad input; rejected-token hint; expired stored token).
- Help topic `pkg/doc/topics/01-catalog-api.md` "Connecting".

### Why
- The file is read lazily, only when a flag or the environment leaves the server or token open, so explicit environments (CI, existing tests) never depend on the developer's real credentials file.
- `NormalizeOrigin` makes `https://Slop.Boutique:443/` and `https://slop.boutique` the same login; a bare host means https.

### What worked
- `GOWORK=off go test ./cmd/francois/cmds/ ./pkg/catalog/credentials/ -count=1` → ok.
- Manual: with an empty `FRANCOIS_CONFIG_DIR`, `catalog auth status` prints "Not logged in. Run francois catalog auth login --server URL." and `catalog whoami` explains how to log in.

### What didn't work
- `GOWORK=off go generate ./...` (to create the logcopter file) also runs the UI build and failed: `Error [ERR_MODULE_NOT_FOUND]: Cannot find package 'esbuild' imported from …/scripts/build.mjs` (no `node_modules`). Ran only the generator: `GOWORK=off go tool logcopter-gen -area-prefix go-go-golems.francois -strip-prefix github.com/go-go-golems/francois ./pkg/...`, then `-check` passed.
- Lint, first pass:
  ```
  cmd/francois/cmds/auth.go:224:2: missing cases in switch of type credentials.Source: credentials.FromFlag, credentials.FromFile (exhaustive)
  cmd/francois/cmds/auth.go:296:1: File is not properly formatted (gofmt)
  pkg/catalog/credentials/credentials.go:81:1: named return "f" with type "File" found (nonamedreturns)
  ```
  Fixed all three; second run `0 issues.`

### What I learned
- The repository lint config enforces `exhaustive` switches and forbids named returns.

### What was tricky to build
- Status needs to explain precedence: a stored login can exist yet be unused because `FRANCOIS_TOKEN` wins. `auth status` resolves exactly as a command would and marks each stored row `yes` or `no, overridden by FRANCOIS_TOKEN`, and adds a row when an environment token targets a server with no stored login.

### What warrants a second pair of eyes
- `NormalizeOrigin` now rejects `--server` values with a path (`https://host/prefix`). Previously the client would have used such a prefix. The portal is always served at the origin root, so this should only catch mistakes.

### What should be done in the future
- Device mode for `auth login` (phase 4) and server-side revocation in `logout` (phase 5).

### Code review instructions
- `pkg/catalog/credentials/credentials.go` `Resolve`; `cmd/francois/cmds/api.go` `connect`; `cmd/francois/cmds/auth.go`.
- `go test ./pkg/catalog/credentials/ ./cmd/francois/cmds/`.

### Technical details
- File format: `{"version":1,"default":"<origin>","servers":{"<origin>":{"token","tokenId","name","user":{id,name,email,role},"scopes","expiresAt","savedAt","host"}}}`.

## Step 6: Phase 2 server device flow, and moving to a separate worktree

I implemented the device endpoints, store and migration. While building, the
compile failed in a file I had not touched, which revealed another agent
session editing the same working tree (ticket FRANCOIS-021, "made-at dates").
To avoid mixing work, I moved my uncommitted Phase 2 files into a new git
worktree on a new branch and continued there.

### Prompt Context

**User prompt (verbatim):** "continue" (see Step 4)
**Assistant interpretation:** Continue phases 2-5.
**Inferred user intent:** Finish the feature.
**Commit (code):** 6e72bfa — "feat(catalog): device login endpoints that mint tokens from a browser approval"

### What I did
- `pkg/catalog/store/migrations/019_device_authorizations.sql` (table of design §8.3, `interval_seconds` without default: the server always sets it).
- `pkg/catalog/store/device.go`: `NewUserCode`, `NormalizeUserCode`, `FormatUserCode`, `CreateDeviceAuthorization` (returns `ErrConflict` on a user-code collision), `DeviceRequest`, `ApproveDevice`, `DenyDevice`, `PollDevice` with a `DeviceGrant` callback, `issueDevice`.
- `pkg/catalog/device.go`: `deviceRoutes`, `deviceAuthorize`, `deviceToken`, `findDevice`, `suggestGrant`, `deviceRequest`, `deviceApprove`, `deviceDeny`.
- `pkg/catalog/demos.go`: `limiter.left` and `limiter.take`. `server.go`: `deviceLimiter`. `routes.go`: `s.deviceRoutes(m)`. `tokens.go`: `meUser`.
- `pkg/catalog/device_test.go`: `TestUserCodes`, `TestSuggestGrant`, `TestDeviceLogin`, `TestDeviceLoginRefusals`, `TestDeviceAuthorizeRateLimit`.
- Worktree: `git worktree add -b feat/cli-auth ../francois-cli-auth HEAD`, copied my seven changed or new files there, restored them in the original tree (`git checkout --` on my four modified files, `rm` on my three new ones).

### Why
- Decision 3: when the CLI requests no scopes, `suggestGrant` pre-selects every scope the role allows, admin scopes included, and raises the lifetime to 90 days when admin scopes need one.
- `slow_down` and expiry use the database's `now()` in SQL, so the app server's clock does not matter.

### What worked
- `go test ./pkg/catalog/ -run 'TestDevice|TestUserCodes|TestSuggestGrant' -v`: all five PASS. Full `./pkg/catalog/` ok (15.1 s). Lint `0 issues.`

### What didn't work
- `GOWORK=off go build ./...` in the shared tree:
  ```
  pkg/catalog/client/types.go:89:2: MadeAt redeclared
  	pkg/catalog/client/types.go:86:2: other declaration of MadeAt
  ```
  `git status` showed changes I did not make (`client/types.go`, `pages.go`, `store/releases.go`, `studio.go`, templates, `ui/catalog/studio*.ts(x)`, migration `020_artifact_made_at.sql`, ticket FRANCOIS-021). I left them untouched.

### What I learned
- Two agents can share one checkout. A worktree per ticket keeps commits and builds separate.

### What was tricky to build
- Tests poll twice within the 5-second interval, which correctly returns `slow_down`. `devicePoll` in the tests moves `last_polled_at` an hour back before each poll that should not count as too fast. One test asserts the `slow_down` path itself.
- `tokenGrant` returns `*grantError`. The grant callback returns it as `error` only when it is non-nil, avoiding a non-nil interface that wraps a nil pointer.

### What warrants a second pair of eyes
- `PollDevice` deletes an approved row before re-checking the approver. A denial at that point (role or disabled change) therefore consumes the request; the CLI sees `access_denied`.
- Rate limits: 20 starts per hour per address, 600 per hour globally, 1200 polls per hour per address, and 30 unknown-code lookups per hour per user.

### What should be done in the future
- The other session's work in the original tree is not mine. Merging the branches is a human decision.

### Code review instructions
- `git show 6e72bfa`; start at `PollDevice` in `pkg/catalog/store/device.go`.

### Technical details
- The worktree is at `/Users/manuel.odendahl/code/go-go-golems/francois-cli-auth`, branch `feat/cli-auth`, branched from `5503396`. `feat/cli-auth-login` stays at `5503396` in the original tree.

## Step 7: Self-revocation, client calls, CLI device login

These steps finish the server and CLI halves: `DELETE /api/me/token` (decision
1), typed client calls, and `auth login` running the device flow by default,
with `auth logout` revoking the token on the server.

### Prompt Context

**User prompt (verbatim):** (see Step 4)
**Assistant interpretation:** Phases 4 and 5.
**Inferred user intent:** One command to log in and one to log out.
**Commit (code):** 5b30603 — "feat(catalog): tokens may revoke themselves; client calls for device login"; ba9761c — "feat(cli): browser device login and revoking logout"

### What I did
- `pkg/catalog/tokens.go`: route `DELETE /api/me/token` (`session`, `anyToken`) and `revokeSelf` (`400 token_required` for sessions, audit `token.revoke_self`).
- `pkg/catalog/client/device.go`: `DeviceAuthorize`, `DevicePoll`, `RevokeSelf`. `TestDeviceLoginClient` runs the whole flow through the client with `WithStrictDecoding`. `TestTokenRevokesItself` checks that another token is unaffected and that a session is refused.
- `cmd/francois/cmds/auth.go`: `authLogin.device`, `pollDevice`, `shouldOpenBrowser`, `openBrowser`, `revoke`; new flags `--scopes --days --name --no-browser` for login and `--keep-token` for logout.
- `cmd/francois/cmds/auth_test.go`: `deviceAPI` scripted fake, `TestAuthLoginDevice` (pending, slow_down, pending, issued; checks the request body and that no bearer header is sent), `TestAuthLoginDeviceRefusals`.

### Why
- The browser opens only for interactive desktop sessions (no `SSH_CONNECTION`, stderr is a TTY, macOS or Windows or a display), and only for a URL under the server's own origin.
- Logout still removes the local entry when the server is unreachable, and prints a warning that names the token to revoke on the account page.

### What worked
- `go test ./cmd/francois/cmds/` ok (1.07 s); catalog device and revoke tests PASS; lint `0 issues.`

### What didn't work
- The first `TestAuthLoginDevice` run took 10.02 s: after `slow_down` the real 5-second increment applied to every later poll. Made `deviceSlowDown` a package variable next to `devicePollFloor`; tests lower both.

### What I learned
- The scripted fake must return `interval: 0` and rely on `devicePollFloor`. Otherwise the client's one-second floor makes the tests slow.

### What was tricky to build
- `pollDevice` must tell API refusals apart from network errors. Known codes are handled first, any other `*client.Error` aborts, and transport errors are retried until the code expires.

### What warrants a second pair of eyes
- `DELETE /api/me/token` is the agreed exception to "tokens never revoke tokens" (owner decision 1).

### What should be done in the future
- Phase 3 UI (`/device` page), then documentation.

### Code review instructions
- `git show 5b30603 ba9761c`; `go test ./cmd/francois/cmds/ -run TestAuth -v`.

### Technical details
- Login request body: `{"clientName":"francois CLI on <host>","scopes":null|[…],"days":N?}`. Without `--days` the field is omitted and the server uses 90.
