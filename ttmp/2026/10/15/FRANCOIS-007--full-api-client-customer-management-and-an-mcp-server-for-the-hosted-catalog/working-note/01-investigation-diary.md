---
Title: Investigation diary
Ticket: FRANCOIS-007
Status: active
Topics:
    - backend
    - cli
    - mcp
    - billing
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological record of the FRANCOIS-007 analysis: evidence gathered about the catalog API, auth, customer data and MCP prior art, the design decisions taken, and delivery."
LastUpdated: 2026-10-15T19:40:00-04:00
WhatFor: "Resuming FRANCOIS-007; understanding why the design guide chose scoped tokens, a typed client and a go-sdk MCP server."
WhenToUse: "Before implementing any phase of FRANCOIS-007."
---

# Investigation diary

## Step 1: Open the ticket and inventory the API, auth model, customer data and MCP prior art

The owner asked for the CLI to become a full client that can also manage customers, followed by an MCP server, and for an intern-oriented design guide stored in a new ticket and uploaded to reMarkable. This step created the ticket and gathered the evidence the guide rests on. No code was changed.

The evidence came from four parallel read-only investigations (HTTP route inventory, auth and customer data model, CLI and client structure, MCP libraries in the workspace), followed by direct checks of the claims the design depends on.

### Prompt Context

**User prompt (verbatim):** "Create a new docmgr ticket. turn it into full client. including managing customers and all that. then the MCP server. Create  a detailed analysis / design / implementation guide that is for a new intern, explaining all the parts of the system needed to understand what it is, with prose paragraphs and bullet point sand pseudocode and diagrams and api references and file references. It should be very clear and technical. Store in the ticket and the nupload to remarkable."

**Assistant interpretation:** Create FRANCOIS-007; analyse the catalog's API surfaces, authentication, customer data and CLI; design a full Go client (studio, admin/customer management, account), a complete command tree and an MCP server; write it as an intern guide; store it in the ticket; upload it to reMarkable. The deliverable is the guide, not the implementation.

**Inferred user intent:** The owner (and agents working for them) should be able to operate the live shop — content, customers, billing, settings — from scripts and from Claude, without `kubectl exec` and without a browser session, with a plan an implementer can follow phase by phase.

### What I did

- Added the docmgr topics `cli` and `mcp`; created ticket FRANCOIS-007 with a design doc and this diary.
- Inventoried every route on the portal mux and the instance mux, with guards and request/response shapes (`pkg/catalog/routes.go:21-102`, `admin.go:19-40`, `studio.go:35-74`, `hosted/routes.go:27-73`).
- Read `guard` and the token code directly: `pkg/catalog/auth.go:66-131`, `pkg/catalog/store/tokens.go:13-73`, `store/migrations/004_storefront.sql:80-89`, `studio.go:1200-1259`.
- Mapped the customer data model (migrations 001–007), store functions for users, grants, billing, usage, settings and instances, the payments service and gateway, and the audit log.
- Read the CLI (`cmd/francois/main.go`, `cmd/francois/cmds/*.go`), `pkg/catalog/studioclient/client.go`, the Postgres test harness (`pkg/catalog/catalog_test.go:34-196`), `pkg/catalog/tokens_test.go`, the TypeScript API definitions (`ui/catalog/api.ts`, `ui/catalog/studio-api.ts`) and the Makefile targets.
- Surveyed MCP code in the workspace (go-go-mcp, smailnail, jesus, corewars) and the module cache; checked go-sdk v1.7.0 signatures directly: `mcp/server.go:561` (`AddTool`), `mcp/tool.go:57` (`ToolHandlerFor`), `mcp/streamable.go:232` and the `Stateless` option, `mcp/transport.go:114,160`, `auth/auth.go:21-97` (`TokenInfo`, `RequireBearerToken`, `TokenInfoFromContext`, `AllowMissingExpiration`), and `ServerOptions.SchemaCache`.

### Why

- The design hinges on how `guard` treats bearer tokens, so that code was read line by line rather than taken from a summary.
- The MCP library choice depends on what is already in the module cache and on dependency weight under `CGO_ENABLED=0` (`Dockerfile:23`).

### What worked

- The four investigations returned file:line-referenced inventories that agreed with each other and with direct reads.
- go-sdk v1.7.0 is in the module cache and offers everything the design needs: typed tools with inferred schemas, stdio, Streamable HTTP with a stateless mode, bearer-token middleware, in-memory transports for tests, and a schema cache for per-request servers.

### What didn't work

- No failures. One limitation surfaced: the claude.ai smailnail MCP connector was unavailable this session (HTTP 502 from Cloudflare when the session started). It does not affect this ticket.

### What I learned

- Tokens are restricted by a **path prefix test inside `guard`** (`auth.go:89`), not by route declarations. Scopes therefore need a guard signature change at every route registration.
- Token management sits behind `guard(creator, …)`, so subscribers cannot create tokens at all today.
- `POST /api/instances/{id}/launch` binds the launch code to the browser session hash (`instances.go:206`, `SessionHash: p.Session.Hash`), so it can never work with a token and must be excluded explicitly.
- All nine admin GET handlers only read (no store mutation calls), which makes a method-derived `admin:read` / `admin:write` split sound.
- `PATCH /api/admin/settings` replaces the whole document with no concurrency check (`store/settings.go:36-63`), so a CLI that edits one plan could silently undo a browser edit.
- Plans and prices are not tables; they live in the settings document (`settings/settings.go:46-98`), and `Document.Plan` returns a value, not a pointer (`settings.go:289`).
- `settings` and `billing` import only the standard library and logcopter, so the client can reuse their types; `store` cannot be reused because it imports pgx.
- No library turns Glazed `GlazeCommand`s into MCP tools; go-go-mcp's config provider panics on them (`go-go-mcp/pkg/tools/providers/config-provider/tool-provider.go:362-367`).
- `RevokeUserSessions` does not revoke API tokens (`store/sessions.go:87-101`).

### What was tricky to build

- Deciding where admin reach should come from without undoing FRANCOIS-004's deliberate boundary ("a leaked publishing token cannot change users or billing"). Scopes keep that property for existing tokens because the migration gives every existing token exactly `{studio}`.

### What warrants a second pair of eyes

- The scope matrix: which account routes a token may call, and that launch and token management stay session-only.
- The decision to use go-sdk directly instead of the owner's go-go-mcp library.
- Exposing cancel-at-period-end (a Stripe write) to admin tokens and therefore to agents.

### What should be done in the future

- Implement the six phases in the guide (§10), one pull request each.

### Code review instructions

- Not applicable; no code changed. To check the evidence, start with `pkg/catalog/auth.go:78-131` and `pkg/catalog/admin.go:19-40`.

### Technical details

- Route counts: 19 admin routes, 39 studio routes, 13 account routes, plus public, auth, webhook and instance-origin routes.
- Token format: `frt_` + base64url(32 bytes); stored as SHA-256; `days` 0–366, where 0 means no expiry.

## Step 2: Write the guide and verify its claims

This step turned the evidence into the design guide: orientation and terms, current state, a gap table, the proposed architecture with nine decision records, a route-to-client-method map, pseudocode for the key flows, a security checklist, six implementation phases and a test strategy.

After drafting, I re-checked the claims the decisions depend on and corrected two pieces of pseudocode that did not match the real `settings` API.

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Produce the guide in the ticket with prose, bullets, pseudocode, diagrams, API references and file references.

**Inferred user intent:** An implementer new to the codebase can build each phase from the guide alone.

### What I did

- Wrote `design-doc/01-full-api-client-customer-management-and-mcp-server-analysis-design-and-implementation-guide.md`.
- Checked that the nine admin GET handlers make no store mutation calls; that `instances.go` binds launch codes to `p.Session.Hash`; that `Dockerfile:23` builds with `CGO_ENABLED=0`; that `RevokeUserSessions` does not touch `api_tokens`.
- Corrected the pseudocode: `settings.Parse(raw)` instead of a non-existent `ParseReader`; plan editing by index into `doc.Plans`, because `Document.Plan` returns a copy; added `doc.Validate()` before saving.
- Tightened the statement about disabled users: tokens stop with `account_disabled` on routes at level `enabled` or above, while read-only account routes at level `session` remain readable, as they are for the browser session.

### Why

- Pseudocode that calls functions which do not exist sends an implementer looking for them; it was cheaper to fix than to explain.

### What worked

- The decision records stayed consistent with the evidence after the checks; no decision had to change.

### What didn't work

- No failures.

### What I learned

- The tool count is about 70 when a token has every scope (32 studio, 28 customer, 7 account, 1 identity). The guide addresses this with toolsets, a read-only mode and narrow tokens.

### What was tricky to build

- Remote MCP must not bypass `guard`. The design routes remote tool calls through the client with an in-process `http.Handler` transport, so a remote call takes exactly the path of an external HTTP request.

### What warrants a second pair of eyes

- §6.8: per-request MCP servers with a shared `SchemaCache`, and `AllowMissingExpiration` for non-expiring studio tokens.

### What should be done in the future

- Owner decisions listed in §12: go-go-mcp versus go-sdk, `catalog push` location, subscriber tokens, OAuth for remote clients.

### Code review instructions

- Read §3 first, then the decision records in §6; check §7 against `ui/catalog/api.ts` and `ui/catalog/studio-api.ts`.

### Technical details

- The guide's new endpoints: `GET /api/me`; `/api/tokens*`; admin token list and revoke; per-grant revoke; subscriptions list, cancel and resume; admin instance delete; settings `ETag`/`If-Match`; audit filters.

## Step 3: Validate the ticket and upload the guide

Ticket hygiene passed and the guide was uploaded to reMarkable as one PDF with a two-level table of contents.

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Deliver the guide to reMarkable after storing it in the ticket.

**Inferred user intent:** Read and annotate the design on the tablet.

### What I did

- `docmgr doctor --ticket FRANCOIS-007 --stale-after 30` → "All checks passed".
- `remarquee upload bundle <design doc> --name "FRANCOIS-007 API client customer management and MCP server guide" --remote-dir "/ai/2026/10/15/FRANCOIS-007" --toc-depth 2 --non-interactive` → `OK: uploaded FRANCOIS-007 API client customer management and MCP server guide.pdf -> /ai/2026/10/15/FRANCOIS-007`.

### Why

- One PDF of the guide is what was asked for; the diary stays in the ticket.

### What worked

- Upload succeeded on the first attempt. This proves cloud delivery, not device synchronization.

### What didn't work

- `docmgr doc relate --file-note "$R/$DD:Primary …"` failed in zsh because `$DD:P` is a zsh path modifier; `${DD}` fixed it.

### What I learned

- In zsh, write `${VAR}:note` in `--file-note` arguments.

### What was tricky to build

- N/A.

### What warrants a second pair of eyes

- N/A.

### What should be done in the future

- Owner decisions in guide §12, then phase 1.

### Code review instructions

- N/A; documentation only.

### Technical details

- Remote path: `/ai/2026/10/15/FRANCOIS-007/FRANCOIS-007 API client customer management and MCP server guide.pdf`.

## Step 4: Owner decisions and a measured comparison of go-go-mcp and go-sdk

The owner settled two open questions and asked what go-go-mcp would pull in and what it would offer. I measured the packages each option compiles and recorded the result in the guide's MCP library decision, which remains open.

### Prompt Context

**User prompt (verbatim):** "susbcriber tokens yes. What does go-go-mcp pull in and what would be the benefits of using it? catalog releases push. Not sure what claude.ai connectors man?"

**Assistant interpretation:** Record: subscribers may hold tokens; `catalog push` moves to `catalog releases push`. Quantify go-go-mcp's dependency cost and benefits. Explain claude.ai connectors.

**Inferred user intent:** Decide the MCP library on evidence, as the owner of go-go-mcp.

### What I did

- In `/home/manuel/code/wesen/go-go-golems/go-go-mcp` at `86db8ed`, ran `GOWORK=off GOFLAGS=-mod=mod GOPROXY=off go list -deps -f '{{if not .Standard}}{{.ImportPath}}{{end}}'` for `./pkg/embeddable` (318 packages), for go-sdk `mcp` + `auth` (29), and for the go-go-mcp packages `embeddable` imports other than `pkg/auth/oidc` (61).
- Confirmed with `go list -f '{{join .Imports " "}}' ./pkg/auth/oidc` that the embedded OIDC server imports `ory/fosite` and `mattn/go-sqlite3`; grouped the 318 packages by module (grpc 66, protobuf 37, OpenTelemetry exporters for Jaeger, Zipkin and OTLP, fosite, ory/x, watermill, testify, logrus).
- Checked francois's own dependency graph (`go list -deps ./cmd/francois`): it already has OpenTelemetry core and logrus, but not grpc, fosite, go-sqlite3, watermill or the exporters.
- Updated the guide: `catalog releases push` in the command decision and phase 3; the measurement table and a conditional path to go-go-mcp in §6.7; resolved items and a clearer remote-MCP/OAuth question in §12.

### Why

- The previous wording ("pulls in geppetto, clay, bubbletea…") came from the go.mod, which lists modules, not what is compiled. Compiled packages are what affect binary size, build time and vulnerability scanning.

### What worked

- The measurement isolates one import (`pkg/auth/oidc`) as the cause of about 257 of the 318 packages.

### What didn't work

- No failures.

### What I learned

- geppetto, clay and bubbletea appear in go-go-mcp's go.mod but bubbletea and clay are not compiled through `pkg/embeddable`; geppetto is, through the non-OIDC packages.

### What was tricky to build

- N/A.

### What warrants a second pair of eyes

- Whether go-go-mcp can register its `embedded_dev` auth mode from a separate package without breaking smailnail and jesus.

### What should be done in the future

- Owner choice: go-sdk directly now, or trim go-go-mcp first and use it.

### Code review instructions

- Guide §6.7 (decision and measurement table) and §12.

### Technical details

- Commands as above; run from the go-go-mcp checkout with the module cache offline (`GOPROXY=off`).

## Step 5: Would go-go-mcp's OIDC support help with remote harnesses later?

The owner asked whether go-go-mcp's OIDC code would pay off later, when the server is offered to external harnesses such as claude.ai connectors, and noted that smailnail is old and may not be representative. I checked the current go-go-mcp code and ZITADEL's documentation instead of relying on smailnail, and recorded the findings in guide §6.8.

### Prompt Context

**User prompt (verbatim):** "so would the oidc stuff from go-go-mcp not help us, even later on when adding them to external harnesses?" — and, mid-turn: "smailnail is super old btw, and maybe not super indicaditve"

**Assistant interpretation:** Assess the value of go-go-mcp's external OIDC mode for a future OAuth-protected `/mcp` with ZITADEL.

**Inferred user intent:** Avoid choosing a library now that would have to be replaced at the connector phase.

### What I did

- Read `go-go-mcp/pkg/embeddable/auth_provider_external.go` (410 lines): discovery, JWKS cache (5-minute refresh), JWT verification; `grep -i introspect` finds nothing.
- Confirmed `~/go/pkg/mod/github.com/zitadel/oidc/v3@v3.47.8/pkg/client/rs/resource_server.go` provides `NewResourceServerClientCredentials`, `NewResourceServerJWTProfile` and an introspection URL.
- Web search on ZITADEL and RFC 7591: ZITADEL documents DCR (disabled by default); a report says it merged late July 2026; issue zitadel#12658 says DCR-created clients default to opaque access tokens; some sources say MCP now prefers Client ID Metadata Documents (unverified).
- Read the SMAILNAIL-016 index and changelog in the go-go-mcp repository for context only: Claude's login failed at the identity provider's client-registration policy, not in the MCP server.

### Why

- The library question depends on whether go-go-mcp's verifier would accept ZITADEL's tokens.

### What worked

- The check found a concrete mismatch: JWT-only verification versus opaque tokens from ZITADEL's dynamic registration.

### What didn't work

- Sources disagree on whether ZITADEL's registration support is in a stable release; unresolved until the deployed version is checked.

### What I learned

- The value of go-go-mcp at the connector phase is the resource-server plumbing (metadata endpoint, challenge header, scope checks). It would need introspection support to work with ZITADEL's default token type, or ZITADEL would need to issue JWT access tokens to registered clients.

### What was tricky to build

- N/A.

### What warrants a second pair of eyes

- The ZITADEL registration and token-type findings come partly from third-party reports; verify on the deployed ZITADEL.

### What should be done in the future

- At the connector phase: check ZITADEL's version and registration settings; decide between go-go-mcp with introspection added and go-sdk plus `zitadel/oidc` introspection.

### Code review instructions

- Guide §6.8, last bullet.

### Technical details

- Sources: https://zitadel.com/docs/guides/integrate/dynamic-client-registration, https://github.com/zitadel/zitadel/issues/12658, https://casys.ai/blog/zitadel-dcr-proxy-claude-mcp, https://zuplo.com/learn/mcp/compatibility/identity-providers.

## Step 6: Adopt go-go-mcp v0.3.0 in the guide

go-go-mcp PR #89 (GO-GO-MCP-EMBEDDABLE-AUTH) was merged and released as v0.3.0. That release removes both reasons the guide had preferred go-sdk directly: `pkg/embeddable` now compiles 65 packages instead of 318, and `external_oidc` verifies opaque tokens by introspection. The guide's MCP library decision is now go-go-mcp v0.3.0, and the MCP sections were rewritten for its API.

### Prompt Context

**User prompt (verbatim):** "commit. do PR." (after the proposal to commit the ticket, revise the MCP decision for v0.3.0, and open a PR)

**Assistant interpretation:** Revise the guide for go-go-mcp v0.3.0, commit the FRANCOIS-007 ticket on a branch, open a PR.

**Inferred user intent:** Get the plan reviewed and merged before phase 1 starts.

### What I did

- Read go-go-mcp v0.3.0's API:
  - `ToolHandler` is `func(ctx, map[string]any) (*protocol.ToolResult, error)` (`pkg/embeddable/server.go:19`);
  - the tool options are `WithSchema`, `WithOutputSchema` and `WithToolAnnotations` (`tools.go`);
  - results are built with `protocol.NewToolResult`, `WithText`, `WithStructuredContent` and `WithError(msg)` (`pkg/protocol/tools.go`);
  - the server is run with `NewBackend(cfg).Start(ctx)`;
  - `WithToolAuthorization` / `ToolAuthorizationPolicy{Public, RequiredScopes}` is enforced at call time and does not filter the tool list;
  - requirements are Glazed `v1.4.4-0.20260823170018-a7fdfda5e3c6`, `go 1.26.3`, go-sdk v1.7.0.
- Rewrote §6.7:
  - the decision record (now accepted), with the measurement table (29 / 318 / 65);
  - the package layout;
  - startup with francois owning the `mcp` command, so tools can be filtered by the token's scopes before registration;
  - a generic `tool[In, Out]` adapter that keeps tools typed over the map-based handler;
  - the example tool.
- Rewrote the §6.8 sketch:
  - `WithHTTPAuthVerifier(tokenVerifier)` and `MountHTTPHandlers`;
  - the raw token kept in `AuthPrincipal.Claims` for the in-process client;
  - a one-minute expiration for tokens without one;
  - per-tool `WithToolAuthorization`;
  - the OAuth/connector phase becoming `external_oidc` + `TokenValidation: auto` configuration.
- §10 phases 5–6 and §11 MCP tests updated; §12 question resolved.

### Why

- The library question was open only because of the two problems go-go-mcp v0.3.0 fixed.

### What worked

- The adapter design keeps the tool layer independent of the map-based handler signature.

### What didn't work

- N/A.

### What I learned

- go-go-mcp's tool policies run at call time, so a remote server shared by all callers lists every tool; local mode filters at registration because it knows the token first.

### What was tricky to build

- Remote mode needs the caller's raw token to call the API in-process; the verifier keeps it in the principal's `Claims`, which never leaves the process.

### What warrants a second pair of eyes

- The Glazed pseudo-version bump required by go-go-mcp v0.3.0, which affects all of francois's commands; it is scheduled as its own step in phase 5.

### What should be done in the future

- Phase 1 (scoped tokens).

### Code review instructions

- Guide §6.7–6.8, §10 phases 5–6, §11.

### Technical details

- go-go-mcp v0.3.0: tag on `ded30b2`, served by proxy.golang.org.

## Step 7: Phase 1 backend: scoped tokens, `/api/me`, admin token oversight, token audit

Phase 1 of the guide (§10) gives personal API tokens scopes, so an administrator can mint a token that reaches the admin API while existing publishing tokens keep exactly their current reach. This step implemented the server side: migration 008, scopes in the store, a scope on every route declaration, the new token and identity endpoints, administrator oversight, and token attribution in the audit log. The browser UI follows in the next step.

The implementation matches the design with one change in where the token-management endpoints sit relative to the guard levels, described below. All new tests passed on the first run. A mutation check confirmed that the scope matrix fails when scopes are ignored.

### Prompt Context

**User prompt (verbatim):** "ok, go ahead" (to: start phase 1, scoped tokens), and mid-turn: "commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

**Assistant interpretation:** Implement guide phase 1 on its own branch, commit at validated boundaries, and record each step here in investigation format.

**Inferred user intent:** Scripts and agents can use admin and account APIs with narrowly scoped tokens; the work is reviewable step by step.

**Commit (code):** 522f816 — "Scoped personal API tokens, /api/me, admin token oversight, token audit"

### What I did

- Branch `feat/francois-007-scoped-tokens` from `docs/francois-007-api-client-mcp` (PR #5), so the ticket can be updated with the code.
- Surveyed: `s.guard(` call sites (30 in `routes.go`, plus the `a` and `c` wrappers in `admin.go` and `studio.go`); five `INSERT INTO admin_audit` statements (`store/users.go:131`, `store/settings.go:59`, `store/usage.go:230`, `store/releases.go:306,342`); `httpx.Read` rejects an empty body (needed for the optional `revoke-sessions` body).
- `store/migrations/008_token_scopes.sql`: `api_tokens.scopes text[] NOT NULL DEFAULT '{studio}'`, then `DROP DEFAULT`; `admin_audit.token_id`; indexes `admin_audit_actor`, `admin_audit_subject`.
- `store/tokens.go`:
  - scope constants;
  - `ScopesForRole`, `IsAdminScope`;
  - `APIToken.Scopes`, `APIToken.Allows` (where `admin:write` implies `admin:read`);
  - `CreateToken(..., scopes, expires)`, `RevokeTokenByID`, `RevokeUserTokens`.
- `store/audit.go`: `WithAuditToken(ctx, id)` and `auditToken(ctx)`. All five audit inserts now write `token_id`; `AuditEntry.TokenID` and `AuditLog` select it.
- `auth.go`:
  - `type scope` with `noToken`, `anyToken`, `scopeStudio`, `scopeAccount` and `scopeAdmin`, plus `scope.required(method)`;
  - `guard(level, sc, h)` rejects tokens on `noToken` routes (401 `token_not_allowed`), resolves the token, checks the scope (403 `token_scope`), and stores the token ID for auditing;
  - role, disabled and CSRF checks are unchanged.
- `routes.go`:
  - public routes, logout and instance launch → `noToken` (launch codes are bound to the browser session);
  - account, billing and instance routes → `scopeAccount`;
  - `s.tokenRoutes(m)` added.
- `admin.go` → `scopeAdmin`. New endpoints:
  - `GET /api/admin/users/{id}/tokens`;
  - `DELETE /api/admin/tokens/{id}` (audited against the owner);
  - `revoke-sessions` accepts an optional `{"tokens": true}` and reports the count.
- `studio.go` → `scopeStudio`; removed the three `/api/studio/tokens*` routes, their handlers and `sessionOnly`.
- New `tokens.go`:
  - `GET /api/me` (guard `session`, `anyToken`);
  - `GET` and `DELETE /api/tokens` (guard `session`, `noToken`);
  - `POST /api/tokens` (guard `enabled`, `noToken`) with the minting rules: scopes must be ones the role allows; duplicates collapse; admin-scoped tokens need 1–90 days; guests get 403 `tokens_unavailable`; creation is audited with scopes and expiry.
- Tests:
  - `tokens_test.go` moved to `/api/tokens` with scopes;
  - new `token_scopes_test.go`: `TestTokenScopeMatrix` (18 calls covering three roles, six tokens and all surfaces, plus `/api/me`, a demoted admin, a disabled owner and an expired token), `TestTokenMintingRules`, `TestAuditRecordsToken` and `TestAdminTokenOversight`.

### Why

- Declaring the scope at registration makes each route's token reach readable in one place, and the compiler finds every call site when the signature changes. That is why no compatibility wrapper was kept (AGENT.md).
- `POST /api/tokens` uses guard level `enabled` so a disabled account cannot mint. Listing and revoking stay at `session` so a disabled user can still revoke a token.

### What worked

- `TEST_DATABASE_URL=…5434… go test ./pkg/catalog/ -run 'TestTokenScopeMatrix|TestTokenMintingRules|TestAuditRecordsToken|TestAdminTokenOversight|TestTokensAndPush' -v` → all five `PASS`.
- Mutation check: with `Allows` forced to return `true`, `TestTokenScopeMatrix` fails all seven `token_scope` cases (`…, want token_scope`); restored, it passes.
- `make ci-check` with the Postgres URL: `0 issues.`, logcopter check clean, every test package `ok`.

### What didn't work

- The first build failed as expected at the old caller: `pkg/catalog/studio.go:1240:91: not enough arguments in call to s.Store.CreateToken`. The old handler was removed, and that left `"time"` unused in `studio.go` (`"time" imported and not used`); the import was removed.
- `gosec` is not installed locally, so GoSec will first run in CI.

### What I learned

- `artifact.create` is audited with the artifact ID as subject, which made `TestAuditRecordsToken` a direct check of `token_id` on a real handler path.
- A subscriber now gets 400 `invalid_scopes` for `studio` instead of the old 403 `creator_required`, because minting moved out of the creator-only studio namespace.

### What was tricky to build

- The order inside `guard`. The scope check must come before the role check so that a token lacking a scope gets `token_scope`, not a misleading `admin_required`. A token that has the scope but whose owner lost the role must still get `admin_required`. The matrix covers both: the creator token on admin routes gives `token_scope`; the demoted admin's write token gives `admin_required`.

### What warrants a second pair of eyes

- `scopeAccount` on `POST /api/billing/checkout` and `/portal`: a token can obtain a Stripe Checkout or customer-portal URL for its owner. Neither moves money by itself, but the portal URL is a bearer link to the owner's billing.
- Read-only account routes (level `session`) remain readable with a disabled owner's token, matching the browser session's behaviour.

### What should be done in the future

- The account-page token UI (next step); README and `doc/production.md`.

### Code review instructions

- Start with `pkg/catalog/auth.go` (`scope`, `guard`), then `pkg/catalog/tokens.go`, `pkg/catalog/admin.go` (`adminRevoke`, `adminUserTokens`, `adminRevokeToken`), and `pkg/catalog/store/tokens.go`.
- Validate with `TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5434/francois_catalog?sslmode=disable' make ci-check`.

### Technical details

- Error codes added: 403 `token_scope`, 400 `invalid_scopes`, 403 `tokens_unavailable`. The message for 401 `token_not_allowed` is now "API tokens are not accepted here".
- Minting matrix: subscriber `account`; creator `studio`, `account`; admin `studio`, `account`, `admin:read`, `admin:write`.

## Step 8: Phase 1 UI, browser verification and documentation

Token management moved from the studio to the account page, so that subscribers, who have no studio, can create `account` tokens. The form offers a checkbox for each scope the user's role allows, with a description of each. The step ended with a browser round trip on the local stack: create an `admin:read` token in the form, use it from curl, revoke it, and confirm it stops working.

### Prompt Context

**User prompt (verbatim):** see Step 7.

**Assistant interpretation:** Finish phase 1: UI, verification and docs, committed separately.

**Inferred user intent:** Tokens usable by every role from the browser.

**Commit (code):** c36f489 — "Token management on the account page, with scopes"

### What I did

- `ui/catalog/api.ts`: `TokenScope` and `Token` (with `scopes`) types; the `tokens`, `createToken` (`{name, days, scopes}`) and `revokeToken` endpoints on `api/tokens`; tag `Tokens`.
- `ui/catalog/studio-api.ts`: removed the token type and endpoints.
- New `ui/catalog/tokens.tsx`:
  - `ApiTokens({role})`, with `scopesFor(role)` mirroring `store.ScopesForRole`;
  - the day limit switches to 1–90 when an admin scope is checked;
  - the secret is shown once with a curl example, plus a `catalog push` example for `studio` tokens;
  - the list shows each token's scopes, use and expiry.
- `ui/catalog/account.tsx`: an "API tokens" row with anchor `#api-tokens`. `ui/catalog/studio.tsx`: the Tokens section now links to `/account#api-tokens`.
- `pkg/catalog/web/static/catalog.css`: `.token-form`, `.scope-list`, `.scope-option`. `pnpm build` regenerated `catalog.js`.
- Rebuilt the local stack's francois service (`docker compose -f deploy/compose/compose.yaml up -d --build francois`); migration 008 applied (the `scopes` column exists on the 5435 database).
- Browser (Playwright, signed in as Ada Admin):
  - the panel listed all four scopes;
  - filled name "browser check", 30 days, `admin:read`, and created the token;
  - the secret had prefix `frt_` and length 47;
  - the token list showed the new token and the pre-existing "local showcase seed" token as `studio`.
- `README.md`, `doc/production.md`: tokens are created on the account page; scopes, oversight endpoints and audit attribution are described.

### Why

- Token management was under `/api/studio`, which only creators and admins can reach; subscriber tokens (owner decision) need a page every user has.

### What worked

- `pnpm -s check` passed; `pnpm -s test` → `ℹ pass 5`, `ℹ fail 0`.
- With the browser-created token:

  ```text
  GET /api/me -> 200 {"creator":null,"token":{"id":"tok_88f9…","name":"browser check","scopes":["admin:read"],…
  GET /api/admin/users -> 200 [...]
  GET /api/studio/artifacts -> 403 {"error":{"code":"token_scope","message":"This token lacks the studio scope"}}
  PATCH /api/admin/settings -> 403 {"error":{"code":"token_scope","message":"This token lacks the admin:write scope"}}
  ```

  After revoking it in the browser session (`DELETE /api/tokens/{id}` → 200): `GET /api/me` → 401 `token_invalid`.
- The element screenshot (`.playwright-mcp/account-tokens.png`, git-ignored) shows the form, the scope list, the revoked token and the migrated token.

### What didn't work

- The first curl loop used `set -- $p` to split `"GET /api/me"`; zsh does not word-split unquoted parameters, so every call went out malformed (`400 Bad Request`). It was rewritten with explicit arguments.
- The Playwright MCP refused to save a screenshot to the session scratchpad (`File access denied … is outside allowed roots`); it was saved under the git-ignored `.playwright-mcp/` instead.

### What I learned

- The migration's default worked on real data: the local stack's existing token became `studio` and kept working for its owner.

### What was tricky to build

- Keeping the scope list in the UI consistent with the server. The UI mirrors `ScopesForRole` only to offer choices; the server validates independently, so a stale UI could only offer fewer or invalid choices, never grant more.

### What warrants a second pair of eyes

- Showing admin scopes in a browser form makes it easy to create powerful tokens. The 90-day limit and the audit entries mitigate this; an extra confirmation step could be added.

### What should be done in the future

- Phase 2 (customer-management endpoints).

### Code review instructions

- `ui/catalog/tokens.tsx`; then try it on the local stack at `http://localhost:8090/account#api-tokens`.

### Technical details

- Local stack: `deploy/compose/compose.yaml`; francois on `:8090`, its Postgres on `127.0.0.1:5435`; tests use `127.0.0.1:5434`.

## Step 9: Phase 2: customer-management endpoints

Phase 2 fills the customer-management gaps from the guide's gap table (G6–G11):
- revoke a single complimentary grant;
- list subscriptions across customers;
- schedule or withdraw a cancellation at period end;
- delete an instance as an administrator;
- edit settings without losing a concurrent edit;
- filter the audit log and the user list.

Everything went in as one commit, as the owner asked for less granular steps.

### Prompt Context

**User prompt (verbatim):** "do 2-6, and then push to th ePR. No need to make it so grnaular. commit at appropriate intervals and keep a detailed diary as you work (using the diary format from the skill)"

**Assistant interpretation:** Implement phases 2–6 on `feat/francois-007-scoped-tokens`, commit per phase or so, keep this diary, push to PR #6.

**Inferred user intent:** Reach a working client, CLI and MCP server in one reviewable PR.

### What I did

- Store:
  - `SaveSettingsIf(ctx, actor, base, doc)` takes `pg_advisory_xact_lock(hashtext('platform_settings'))` and returns `ErrConflict` unless `base` is the newest revision. A negative `base` skips the check, which is what `SaveSettings` uses for the CLI and tests. Revision 0 means "only defaults so far" and is checked.
  - `AuditQuery(AuditFilter{Subject, Actor, Action, Before, Limit})`, with `AuditLog` kept as its subject-only form.
  - `RevokeAccessGrant(id)`.
  - `SubscriptionRow`, `AllSubscriptions(status, limit)` and `SubscriptionByID`.
- Payments: `Gateway.SetCancelAtPeriodEnd` (Stripe `V1Subscriptions.Update` with `CancelAtPeriodEnd`) and `Service.SetCancelAtPeriodEnd`, which looks up the projection, calls Stripe, then `EnqueueRefresh` + `Wake`. The projection only changes through the normal event path.
- Admin API:
  - `DELETE /api/admin/access-grants/{id}` revokes views when the customer is no longer paid;
  - `GET /api/admin/subscriptions?status=&limit=` returns rows with `userId`, `email` and the plan key;
  - `POST /api/admin/subscriptions/{id}/cancel|resume` gives 503 without billing, 502 when Stripe refuses, and is audited `billing.cancel_at_period_end` / `billing.resume`;
  - `DELETE /api/admin/instances/{id}` is audited `instance.delete`;
  - settings `GET` returns `ETag: "<revision>"`; `PATCH` needs `If-Match` (428 `revision_required`, 412 `settings_conflict`);
  - `GET /api/admin/audit?subject=&actor=&action=&before=&limit=`;
  - `GET /api/admin/users?q=&role=&plan=&limit=`, with rows gaining `planKey`.
- UI: `saveSettings` takes `{doc, revision}` and sends `If-Match`; the admin settings page passes `data.active.revision`.
- Tests:
  - `payments.TestSetCancelAtPeriodEnd` uses the fake gateway: cancel and resume each change the projection after a refresh, access continues until the period ends, and an unknown subscription gives `ErrNotFound`;
  - `catalog.TestCustomerManagement` covers per-grant revoke, subscriptions, cancel without billing, filters, admin instance delete and the audit filters with paging;
  - `catalog.TestSettingsIfMatch` covers 428, a save, a stale 412, and a save from the new revision.

### Why

- Cancel-at-period-end is the only Stripe write, per the guide's decision record: it is reversible and moves no money.
- Optimistic concurrency closes the read-modify-write race that the planned `plans set` command would otherwise have.

### What worked

- `make ci-check` (with `TEST_DATABASE_URL`): `0 issues.`, every package `ok`; `pnpm check`, `pnpm build`, `pnpm test` (5/0).

### What didn't work

- First run of `TestCustomerManagement`: `admin_customers_test.go:64: plan filter 200 [... "planKey":"pro","source":"staff" ...]`. The test was wrong, not the code: staff resolve to the highest plan (`entitlement.go:46-47`), so `plan=pro` matches the admin and the creator too. The test now combines `plan=pro&role=subscriber`.
- `make ci-check` failed at `fmt-check` (`make: *** [Makefile:29: fmt-check] Error 1`) on `service_test.go` and `admin_customers_test.go`; `gofmt -w` fixed both.

### What I learned

- The plan filter alone does not single out paying customers; combining it with the role does.

### What was tricky to build

- The meaning of revision 0. `0` cannot serve as "skip the check" because it is also the revision a fresh deployment reports (defaults, no rows). The sentinel is a negative base.

### What warrants a second pair of eyes

- `adminSubscriptionCancel` maps every Stripe error to 502 `billing_unavailable`, including "no such subscription" when Stripe and the projection disagree.

### What should be done in the future

- Phase 3: the client.

### Code review instructions

- `pkg/catalog/admin.go` (the new handlers), `pkg/catalog/store/settings.go` (`SaveSettingsIf`), `pkg/catalog/payments/service.go` (`SetCancelAtPeriodEnd`).

### Technical details

- New codes: 428 `revision_required`, 412 `settings_conflict`, 502 `billing_unavailable`.

## Step 10: Phase 3: the typed Go client

`pkg/catalog/client` replaces `pkg/catalog/studioclient`. It covers the identity, studio, admin and account APIs (§7 of the guide), using one transport with retries, streamed multipart uploads, streamed downloads and an in-process handler transport for the remote MCP phase. A contract test calls the methods against the real server with strict decoding.

### Prompt Context

**User prompt (verbatim):** see Step 9.

**Assistant interpretation:** Phase 3 of the guide.

**Inferred user intent:** One client that the CLI and the MCP server share.

### What I did

- `client.go`:
  - `New(server, token, opts...)` with the options `WithHTTPClient`, `WithHost`, `WithRetries` (default 3), `WithHandler` (in-process `http.Handler` transport) and `WithStrictDecoding`;
  - `send` retries GET/HEAD on connection errors and 502/503/504 with 250/500/1000 ms backoff, and never retries mutations;
  - `do` decodes JSON up to 8 MiB; `upload` streams multipart through `io.Pipe`; `download` streams to an `io.Writer`;
  - each call logs method, path, status and duration at debug level through the package's logcopter logger, never headers or bodies.
- `errors.go`: `Error{Status, Code, Message, Checks}`, `IsCode`, and decoding of the envelope (or `http_<status>` for other bodies).
- `types.go`:
  - wire types mirroring the server JSON (`Me`, `Token`, `Artifact`, `ArtifactRow`, `ArtifactDetail`, `Release`, `CheckResult`, `Media`, `Post`, `Creator`, `Instance`, `AccessGrant`, `AuditEntry`, `UserRow`, `Customer`, `SubscriptionRow`, `Settings`, `SettingsRevision`, …);
  - reuses `settings.Document` and `billing.Decision`/`billing.Subscription`;
  - nested values the client passes through (overview, usage, allowance) stay `json.RawMessage`.
- `studio.go` (all studio routes and `Push`: draft → attachments → publish, naming the draft on partial failure), `admin.go` (all admin routes; `SaveSettings(doc, base)` sends `If-Match`; `EditSettings`/`EditPlan` re-read and retry up to three times on `settings_conflict`), `account.go` (account, checkout/portal links, instances, members, upgrade).
- `cmd/francois/cmds/push.go` ported; `pkg/catalog/studioclient` removed (`git rm`).
- Tests:
  - `pkg/catalog/client/client_test.go`: a GET succeeds after two 503s (three attempts); a POST gets one attempt and returns `http_503`; error envelope decoding;
  - `pkg/catalog/client_contract_test.go` (`TestClientContract`): about 70 calls across every surface with a strict client, including the expected error codes (`version_exists`, `token_scope`, `settings_conflict`, `billing_unavailable`, `token_invalid`);
  - `TestTokensAndPush` now uses the new client.

### Why

- A single client means authorization lives only in the server; the CLI and MCP tools are thin.

### What worked

- The contract test passed on the first run. A mutation check showed strict decoding is effective: with `Release.DownloadAccess` removed, the test fails with `client_contract_test.go:64: create release: json: unknown field "downloadAccess"`.
- `make ci-check`: `0 issues.`, every package `ok` (including the new `pkg/catalog/client`).

### What didn't work

- `make ci-check` stopped once at `fmt-check` on `pkg/catalog/client/types.go` (struct field alignment); `gofmt -w` fixed it.

### What I learned

- The server's map-built responses (`publicRelease`, `instanceJSON`, studio list rows) were stable enough that hand-written types matched them exactly. Strict decoding is what keeps that true.

### What was tricky to build

- `Instance` is used for list rows, detail responses (`membersList`, `usage`, `release`, `upgrades`, `launchBlocked`) and admin detail (`activeViews`, `recentFailures`, `audit`). One struct with optional fields keeps strict decoding valid for all of them.

### What warrants a second pair of eyes

- `WithHandler` uses `httptest.NewRecorder` in production code (remote MCP). It buffers whole responses, which is fine for JSON but not for large downloads.

### What should be done in the future

- Phase 4: commands.

### Code review instructions

- `pkg/catalog/client/client.go` (`send`), then `types.go`; run `go test ./pkg/catalog/client/` and `TestClientContract`.

### Technical details

- Every route in guide §7 has a client method except `/api/billing/refresh` (a Checkout-return helper for the browser) and the public pages.

## Step 11: Phase 4: the `francois catalog` command tree

Every API operation now has a command under `francois catalog`, built on one generic Glazed command over the client. The direct-database commands moved to `catalog db`, `catalog push` moved to `catalog releases push`, and an embedded help topic (`francois help catalog-api`) explains tokens, scopes and the groups. Smoke-tested against the local stack. The test exposed two problems (a stale server image and a broken setup script), both fixed.

### Prompt Context

**User prompt (verbatim):** see Step 9.

**Assistant interpretation:** Phase 4 of the guide.

**Inferred user intent:** Operate the shop from a shell with a scoped token.

### What I did

- `cmd/francois/cmds/api.go`:
  - the generic `apiCommand` (Glazed `GlazeCommand`): reads the connection flags `--server`, `--token-file` and `--host`, with `$FRANCOIS_SERVER` and `$FRANCOIS_TOKEN` as fallbacks;
  - a `run(ctx, client, in)` hook, with `emit` turning lists into one row per element, objects into one row, and no body into `ok: true`;
  - the `in` accessors (`S`, `B`, `I`, `L`, plus tri-state `optS`, `optB`, `optI` for patch flags);
  - a `def` table and the `single`/`group` builders; destructive commands get `--yes`, and `confirm` uses `term.IsTerminal`.
- `cmd/francois/cmds/api_commands.go`: groups `whoami`, `artifacts` (list, show, create, update, page-set, page-publish, page-discard), `releases` (list, publish, withdraw, preview, delete, push), `attachments`, `media`, `posts`, `creators`, `users` (list, show, set-role, disable, enable, revoke-sessions, grant, revoke-grant, tokens, revoke-token), `subscriptions`, `instances`, `plans` (list, show, set: an `If-Match` read-modify-write through `EditPlan`), `settings` (show, history, apply), `billing`, `usage`, `audit` and `account`.
- `catalog.go`: new `db` group with `publish`, `settings` and `users …`; `serve` and `migrate` stay; `registerAPI` added.
- `pkg/doc` (embedded topics) loaded in `cmd/francois/main.go`; topic `01-catalog-api.md`.
- Callers of the moved commands updated: `deploy/compose/bin/catalog-setup` (`catalog db users promote`), `README.md`, `doc/production.md` (the `fc` helper now runs `catalog db`, plus a new section of API examples), and the UI hints (`catalog releases push`).
- `go.mod`: `golang.org/x/term` is now a direct dependency (`go mod tidy`).
- Tests (`api_test.go`): the command tree (including that `catalog push` is gone); `whoami` and `users grant` against a fake API, checking the request body; refusal without `--yes`; an API error code reaching the user.

### Why

- One generic command keeps around 60 commands to a table of short definitions; the client already holds the logic.

### What worked

- Against the rebuilt local stack:
  - `users list` with plan and source;
  - `users grant` → `agr_…`;
  - `users revoke-grant` without `--yes`: `revoke-grant changes or removes data; pass --yes to confirm`; with `--yes`: revoked;
  - `audit list --action user.access_revoke` shows `tokenId tok_5bbf…` (the CLI's token);
  - `plans set studio --daily-credits 120 --yes` → revision 5, and `plans show studio` → 120;
  - `subscriptions list` shows bob's Stripe-sandbox subscription.
- `make ci-check` → `0 issues.`, every package `ok`; `pnpm check`, `pnpm test` 5/0.

### What didn't work

- The first smoke run used `--output`/`--fields` (`unknown flag: --output`). This Glazed version names them `--format` and `--output-fields`.
- The first smoke run gave an empty plan column, `not_found: Unknown API route` for `revoke-grant`, and ignored audit filters. Cause: the local stack still ran the phase 1 image, without the phase 2 routes.
- Rebuilding the stack failed: `service "zitadel-setup" didn't complete successfully: exit 1`. The container log said `unknown flag: --database-url`, because the setup script still called `francois catalog users promote`, which had moved to `catalog db`. The script was updated, which also showed the docs needed updating.
- The confirmation first used `os.Stdin.Stat()` and `ModeCharDevice`. `/dev/null` is a character device, so the command prompted and read EOF ("cancelled") instead of saying "pass --yes". It was replaced with `term.IsTerminal`.
- `make ci-check` failed at `fmt-check` until `golangci-lint fmt` reflowed one-line closures in `api_commands.go`.

### What I learned

- Moving commands has callers outside Go: compose scripts, docs, UI hints, and the k3s repository README (`gitops/kustomize/francois/README.md:42`, which is not in this repository and still says `catalog users promote`).

### What was tricky to build

- Patch semantics on the command line: unset flags must mean "unchanged". String flags with tri-state parsing (`optB`, `optI`) avoid Glazed defaults that are indistinguishable from explicit values.

### What warrants a second pair of eyes

- `plans set` and `settings apply` are marked destructive (`--yes`) because they change what customers get.
- The k3s README still documents `catalog users promote`; it needs a one-line change in that repository.

### What should be done in the future

- Phase 5: the MCP server.

### Code review instructions

- `cmd/francois/cmds/api.go`, then the tables in `api_commands.go`; `francois catalog --help`; `francois help catalog-api`.

### Technical details

- Output: `--format json|yaml|csv|table`, `--output-fields a,b`.

## Step 12: Phase 5: `francois mcp` over stdio, on go-go-mcp v0.3.0

`pkg/catalog/mcpserver` exposes 68 tools on go-go-mcp v0.3.0's embeddable server, and `francois mcp` runs them over stdio for a local agent. Tools are filtered by the token's scopes, the selected toolsets and a read-only switch, and upload tools are confined to allowed directories. It was verified over real stdio JSON-RPC against the local stack, and by an automated test with an MCP client over streamable HTTP.

### Prompt Context

**User prompt (verbatim):** see Step 9.

**Assistant interpretation:** Phase 5 of the guide, with the go-go-mcp decision from Step 6.

**Inferred user intent:** Claude Code can manage the shop with a scoped token.

### What I did

- `GOWORK=off go get github.com/go-go-golems/go-go-mcp@v0.3.0`. This raised `go` from 1.26.1 to 1.26.3 and Glazed from v1.4.1 to `v1.4.4-0.20260823170018-a7fdfda5e3c6`. `make ci-check` ran on the bump alone before any MCP code: green.
- `pkg/catalog/mcpserver`:
  - `server.go`: `Settings{Mode, Scopes, Toolsets, ReadOnly, AllowDirs}`, `Options(ClientFunc, Settings)` and `ToolNames`. In remote mode, each tool also gets `embeddable.WithToolAuthorization` with its scope.
  - `typed.go`: the generic `tool[In, Out]` adapter. The input schema comes from `jsonschema.For[In]` and is converted to a map; results with a struct `Out` carry structured content plus an output schema; text content is the summary plus JSON, capped at 60,000 characters; errors become `IsError` results with `code: message` and release checks.
  - `files.go`: `localPath` (absolute path → `EvalSymlinks` → must lie under an `--allow-dir` root) and inline base64 uploads limited to 10 MiB.
  - `tools.go`: identity (1), studio (32), customers (28) and account (7) tools, with read-only and destructive annotations.
- `cmd/francois/cmds/mcp.go`: `francois mcp` checks the token with `GET /api/me`, filters by its scopes, and runs `embeddable.NewBackend(cfg).Start` with stdio. Flags: `--toolsets`, `--read-only`, `--allow-dir`, `--list-tools`, plus the connection flags.
- `pkg/doc/topics/02-catalog-mcp.md`: Claude Code setup (`claude mcp add francois -- francois mcp --server … --token-file … --allow-dir …`), scopes, read-only mode and file rules.
- `pkg/catalog/mcp_test.go` (`TestMCPTools`):
  - tool filtering for studio, read-only `admin:read` and account settings;
  - a go-sdk client over streamable HTTP: the tool list, the grant tool's schema and annotations, `find_customers`, `grant_complimentary_access` (audited with the token), a bad plan returned as a tool error;
  - `push_release` from an allowed file;
  - attachments refused both from outside the allowed directory and through a symlink pointing outside;
  - a creator's `create_creator` refused by the API.

### Why

- The adapter keeps tools typed even though go-go-mcp handlers take `map[string]any` (decision in guide §6.7).

### What worked

- `francois mcp --list-tools`: 68 tools for the admin token. With `--toolsets customers --read-only`: whoami plus 12 read tools.
- Real stdio session against the local stack: `initialize` → `{name: francois}`; `tools/list` → 68 tools; the grant tool's schema has `required: [userId, days, reason]` and annotations `destructiveHint false, readOnlyHint false`. `find_customers {query: alice}` → 1 customer; the grant → `Granted subscriber until 2026-10-19`; a bad plan → `isError=True invalid_request: Unknown plan`.
- `make ci-check` → `0 issues.`, every package `ok`; `CGO_ENABLED=0 go build ./cmd/francois` passes; `go list -deps ./cmd/francois` contains no `go-sqlite3`, `fosite` or `go-go-mcp/pkg/auth/oidc`. This confirms the point of GO-GO-MCP-EMBEDDABLE-AUTH.

### What didn't work

- Lint: `pkg/catalog/mcpserver/files.go:28:2: variable real has same name as predeclared identifier (predeclared)` (three times, plus one in `tools.go`). Renamed to `resolved`.

### What I learned

- go-go-mcp's `WithSchema` silently replaces anything that is neither a map nor a JSON string with an empty object schema (`ensureObjectSchema`). Passing `*jsonschema.Schema` directly would have produced tools with no parameters. The adapter converts schemas to maps.
- Structured content must be a JSON object, so list results are wrapped as `{items: [...]}`.

### What was tricky to build

- One catalogue serving both modes. Local mode filters tools by scope before registering them. Remote mode, with one server for all callers, keeps every tool and attaches per-tool scope policies; `guard` checks again on every call.

### What warrants a second pair of eyes

- 68 tools is a lot of model context. The help page recommends toolsets and `--read-only`.
- `revoke_customer_sessions`, `set_customer_disabled` and similar tools rely on the MCP client asking the human because of `destructiveHint`.

### What should be done in the future

- Phase 6: remote `/mcp`.

### Code review instructions

- `pkg/catalog/mcpserver/typed.go`, `files.go`, then skim `tools.go`; `francois mcp --list-tools`; `francois help catalog-mcp`; `TestMCPTools`.

### Technical details

- Claude Code: `claude mcp add francois -- francois mcp --server https://slop.boutique --token-file ~/.config/francois/agent-token --allow-dir ~/code/my-artifacts`.

## Step 13: Phase 6: remote `/mcp` inside `catalog serve`

With `catalog serve --mcp`, every portal serves the same tools at `/mcp` (streamable HTTP, stateless), authenticated by personal API tokens. Each tool call replays the caller's token through an in-process API client into `Server.Handler()`, so the API's checks and audit attribution are the ones every other caller gets. It is opt-in: production does not set the flag yet.

### Prompt Context

**User prompt (verbatim):** see Step 9.

**Assistant interpretation:** Phase 6 of the guide, then push everything to PR #6.

**Inferred user intent:** Remote agents can operate the shop without a local process.

### What I did

- `pkg/catalog/mcp.go`:
  - `mountMCP` builds the go-go-mcp config (streamable HTTP, `WithHTTPAuthVerifier(tokenVerifier)`, the remote-mode tool catalogue with per-tool scope policies) and calls `embeddable.MountHTTPHandlers` on the portal mux;
  - `mcpClient(ctx)` reads the verified principal (`embeddable.GetAuthPrincipal`) and the site from the request context, and returns `client.New(site origin, token, client.WithHandler(s.Handler()))`;
  - `tokenVerifier`: `Store.ResolveToken`; scopes are expanded so `admin:write` also carries `admin:read`, because go-go-mcp compares scope names literally; expiration is the token's, or one minute for non-expiring tokens; the raw token is kept in `Claims` (process memory only) for replay.
- `Config.MCP`; `routes()` mounts it before the `/api/` catch-all; the `catalog serve --mcp` flag; the local compose `catalog-serve` script passes `--mcp`.
- `setup(t, opts ...func(*Config))` in the tests. `TestRemoteMCP` covers: an anonymous `POST /mcp` → 401 with a `resource_metadata` challenge; the admin token's `whoami` and a grant, audited with that token's ID; `push_release` with `htmlPath` refused ("not accepted by the remote server"), inline `html` accepted; a studio token refused on `find_customers` but allowed `list_artifacts`.
- Docs: a remote section in `francois help catalog-mcp` (`claude mcp add --transport http … --header "Authorization: Bearer …"`), and a note in `doc/production.md`.

### Why

- Calling the API in-process instead of the store keeps one authorization path for browser, CLI, local MCP and remote MCP.

### What worked

- `TestRemoteMCP` passes; `make ci-check` → `0 issues.`, every package `ok`; `CGO_ENABLED=0` build passes.
- Local stack rebuilt with `--mcp`: a curl `initialize` → `"serverInfo":{"name":"francois","version":"1.0.0"}`; `tools/call list_plans` → `"3 plans."`; an anonymous POST → 401.

### What didn't work

- The first `TestRemoteMCP` run: `calling "initialize": sending "initialize": Forbidden`. Cause: go-sdk's DNS-rebinding protection (`streamable.go:326-331`) rejects requests whose connection arrived on a loopback address but whose `Host` is not localhost. `httptest` listens on 127.0.0.1 and the test sends `Host: portal.test`. Production does not meet this condition, because Traefik connects to the pod's IP. The test now serves through a wrapper that sets `http.LocalAddrContextKey` to a pod address (10.42.0.7:8080), which models the ingress.

### What I learned

- Running `catalog serve` on loopback behind a same-host reverse proxy with a public `Host` would trip the same protection. This is worth remembering for non-k3s deployments; the local compose stack is unaffected because browsers send `Host: localhost`.

### What was tricky to build

- The remote server is one go-go-mcp server for all callers, so tool filtering cannot depend on the caller. Scope enforcement happens per call, in go-go-mcp's tool policy and then in `guard`.

### What warrants a second pair of eyes

- `client.WithHandler` buffers each in-process response (`httptest.ResponseRecorder`); remote downloads are not offered as tools, so responses stay JSON-sized.
- The raw token travels in `AuthPrincipal.Claims` inside the process; go-go-mcp's logging redacts tool arguments but not principals. Check that nothing logs the principal's claims.

### What should be done in the future

- Enabling `--mcp` in production is a k3s change (deployment args) for the owner to approve.
- OAuth for claude.ai connectors (separate ticket): `external_oidc` with `TokenValidation: auto` against ZITADEL.
- The k3s README still documents `catalog users promote` (now `catalog db users promote`).

### Code review instructions

- `pkg/catalog/mcp.go`, then `TestRemoteMCP` in `pkg/catalog/mcp_test.go`.

### Technical details

- Claude Code (remote): `claude mcp add --transport http francois https://slop.boutique/mcp --header "Authorization: Bearer frt_…"`.
