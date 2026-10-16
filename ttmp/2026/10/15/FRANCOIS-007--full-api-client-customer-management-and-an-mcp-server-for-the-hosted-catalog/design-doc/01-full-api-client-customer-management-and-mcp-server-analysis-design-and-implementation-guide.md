---
Title: 'Full API client, customer management and MCP server: analysis, design and implementation guide'
Ticket: FRANCOIS-007
Status: active
Topics:
    - backend
    - cli
    - mcp
    - billing
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/catalog.go
      Note: CLI wiring; DB-direct commands move under catalog db
    - Path: repo://pkg/catalog/admin.go
      Note: Admin API routes for customer management
    - Path: repo://pkg/catalog/auth.go
      Note: guard and personal API tokens; the scope change lands here
    - Path: repo://pkg/catalog/store/tokens.go
      Note: Token storage, hashing, resolution
    - Path: repo://pkg/catalog/studio.go
      Note: Studio API routes and token endpoints
    - Path: repo://pkg/catalog/studioclient/client.go
      Note: Existing client replaced by pkg/catalog/client
ExternalSources: []
Summary: 'Intern-oriented guide: how the hosted catalog''s HTTP API, guard and personal API tokens work today, and the design for scoped tokens, customer-management endpoints, a typed Go client (pkg/catalog/client), a complete `francois catalog` command tree and an MCP server (local stdio first, remote /mcp second) built on that client.'
LastUpdated: 2026-10-16T00:30:00-04:00
WhatFor: Planning and implementing FRANCOIS-007; onboarding someone new to the catalog's API, auth model and customer data.
WhenToUse: Read before touching auth.go, admin.go, the token endpoints, the client package, the catalog CLI commands or the MCP server.
---

# Full API client, customer management and MCP server: analysis, design and implementation guide

## 1. Executive summary

The hosted Francois catalog (the storefront at `https://slop.boutique`) is managed today through two channels. People use the browser: the studio for content and the admin pages for users, billing and settings. Scripts use one command, `francois catalog push`, which uploads a release with a personal API token. Everything else a script or an AI agent might want to do — list customers, grant a complimentary plan, suspend an instance, edit a plan, write a journal post — needs either a browser session or a shell inside the production pod (`kubectl exec` and direct database access).

This ticket closes that gap in four layers, each built on the one below it:

1. **Scoped API tokens on the server.** Today a token works only under `/api/studio/`. Tokens gain *scopes* (`studio`, `account`, `admin:read`, `admin:write`), so an administrator can mint a token that also reaches the admin API, while a leaked publishing token still cannot touch customers or billing. A small set of new admin endpoints fills the customer-management gaps found in this analysis (revoke one grant, list subscriptions, cancel at period end, admin token oversight, safe settings edits, a filterable audit log).
2. **A typed Go client**, `pkg/catalog/client`, that covers the studio, admin and account APIs. It replaces `pkg/catalog/studioclient`, which covers three studio endpoints.
3. **A complete command tree**, `francois catalog <resource> <verb>`, written as Glazed commands over the client, so every operation is scriptable and its output can be printed as a table, JSON, YAML or CSV.
4. **An MCP server.** MCP (Model Context Protocol) is the protocol AI applications such as Claude Code use to discover and call tools. `francois mcp` exposes the client's operations as MCP tools over stdio on the operator's machine. A second phase mounts the same tools at `https://slop.boutique/mcp` for remote clients.

The design keeps one rule throughout: **every operation, from every channel, goes through the same HTTP handlers and the same `guard`**. The CLI and the MCP server are API clients, never database clients, so authorization lives in one place.

## 2. Problem statement and scope

### 2.1 What the owner asked for

> "turn it into full client. including managing customers and all that. then the MCP server."

The previous step in the conversation established that the CLI covers only release uploads, that the token cannot reach the admin API, and that there is no MCP server.

### 2.2 In scope

- Token scopes, token management for all signed-in users, and administrator oversight of tokens.
- Customer-management endpoints that are missing today (listed in §5).
- `pkg/catalog/client`: every studio, admin and account endpoint, plus the release push workflow.
- `francois catalog` API commands for every resource, sharing one connection section.
- Moving the existing direct-database operator commands under `francois catalog db`.
- `francois mcp` (stdio) and a remote `/mcp` endpoint inside `catalog serve`.
- Help pages, tests and the documentation updates these changes require.

### 2.3 Out of scope

- Creating or inviting users. Accounts come from ZITADEL sign-in; the bootstrap command `catalog db users promote` already pre-provisions an identity.
- Money-moving billing operations: refunds, immediate cancellation, price changes and coupons stay in the Stripe dashboard and the Stripe customer portal (decision in §6.4).
- OAuth 2.1 authorization for remote MCP clients (claude.ai connectors). §6.8 sketches it as a later phase; the first remote phase accepts personal API tokens only.
- The instance runtime API on `i-<id>.<app domain>` origins. It is what artifacts call from inside a running instance and is not an operator surface.

## 3. Orientation: the parts and how they relate

Read this section first. The later sections assume these terms.

### 3.1 Terms

- **Portal origin.** The trusted website of a site, such as `https://slop.boutique`. It serves public pages, the single-page application (SPA) for signed-in users, and every `/api/...` route. `Server.Handler` dispatches requests by `Host` (`pkg/catalog/server.go:205-240`): the portal host goes to the portal mux built by `routes()` (`pkg/catalog/routes.go:21-102`).
- **API surface.** A group of routes sharing a path prefix and an authorization rule:
  - *Public*: pages and JSON anyone may read (`/api/catalog`, `/api/plans`, …).
  - *Account API*: what a signed-in user does for themselves (`/api/account`, `/api/billing/*`, `/api/instances/*`).
  - *Studio API*: content management for creators and administrators, `/api/studio/*` (39 routes, `pkg/catalog/studio.go:36-74`).
  - *Admin API*: operator functions, `/api/admin/*` (19 routes, `pkg/catalog/admin.go:21-39`).
- **Role.** The column `users.role`: `subscriber`, `creator`, `admin` or `guest` (`pkg/catalog/store/migrations/004_storefront.sql:4-6`). Guests exist only for visitor demos.
- **Customer.** In this document, a user whose relationship with the shop is commercial: their Stripe customer record, subscriptions, complimentary grants, credit allowance and instances. Administrators manage customers through the admin API. The API itself says "user"; the CLI and MCP tool descriptions say "customer" where the operation is about the commercial relationship.
- **Principal.** The authenticated caller of one request: `Principal{Session, User, Token}` (`pkg/catalog/auth.go:18-22`). It is stored in the request context by `guard` and read with `principal(ctx)`.
- **Guard level.** Each route declares the minimum it requires: `public < session < enabled < creator < admin` (`auth.go:68-74`). `enabled` means signed in and not disabled.
- **Session.** A browser sign-in, held in the cookie `__Host-francois_session` (`auth.go:26-31`). Mutations made with a session must also carry `X-CSRF-Token` and a same-origin `Origin` header.
- **Personal API token.** A long random secret beginning `frt_`, created by a signed-in user, stored only as a SHA-256 hash, optionally expiring, revocable (`pkg/catalog/store/tokens.go`). Sent as `Authorization: Bearer frt_…`.
- **Token scope (new).** A named permission attached to a token that selects which API surfaces the token may call. A token never grants more than its owner's role allows.
- **Glazed command.** A CLI command written with `github.com/go-go-golems/glazed`: flags are declared as fields, parsed into a struct, and a `GlazeCommand` emits rows that Glazed formats as a table, JSON, YAML or CSV (`cmd/francois/cmds/push.go:22-120` is a complete example).
- **MCP (Model Context Protocol).** A JSON-RPC 2.0 protocol between an *MCP client* (an AI application such as Claude Code) and an *MCP server* (a program that offers tools). The client lists the server's tools, shows them to the model, and calls them with JSON arguments.
- **MCP tool.** A named operation with a description, a JSON Schema for its input, an optional output schema, annotations (read-only, destructive, idempotent) and a handler.
- **MCP transport.** How messages travel. With *stdio*, the client starts the server as a child process and exchanges messages on stdin and stdout. With *Streamable HTTP*, the server exposes one HTTP endpoint (conventionally `/mcp`) and the client POSTs JSON-RPC messages to it.

### 3.2 How the parts connect after this ticket

```text
  ┌────────────────────────┐  ┌────────────────────────┐  ┌──────────────────────────────┐
  │ shell, scripts, CI     │  │ Claude Code (local)    │  │ remote MCP client            │
  │ francois catalog …     │  │ starts `francois mcp`  │  │ HTTPS POST /mcp              │
  └───────────┬────────────┘  └───────────┬────────────┘  └───────────────┬──────────────┘
              │ Go calls                  │ MCP over stdio                │ MCP over HTTP
              │                           ▼                               ▼
              │               ┌────────────────────────┐  ┌──────────────────────────────┐
              │               │ pkg/catalog/mcpserver  │  │ same tools, mounted inside   │
              │               │ tools → client calls   │  │ catalog serve                │
              │               └───────────┬────────────┘  └───────────────┬──────────────┘
              ▼                           ▼                               ▼
  ┌─────────────────────────────────────────────────────┐  ┌──────────────────────────────┐
  │ pkg/catalog/client  (typed Go client)               │  │ pkg/catalog/client with an   │
  │ Me · Studio · Admin · Account · Push workflow       │  │ in-process transport         │
  └──────────────────────────┬──────────────────────────┘  └───────────────┬──────────────┘
                             │ HTTPS, Bearer frt_…                         │ http.Handler call
                             ▼                                             ▼
  ┌───────────────────────────────────────────────────────────────────────────────────────┐
  │ francois catalog serve — portal mux                                                   │
  │   guard(level, scope) → handler (admin.go, studio.go, routes.go, instances.go)        │
  │   → store (PostgreSQL) · releases.Publisher (blob store) · payments.Service (Stripe)  │
  └───────────────────────────────────────────────────────────────────────────────────────┘
```

Two properties of this picture matter for every later decision:

- **The client is the only way in.** The CLI commands and both MCP modes call `pkg/catalog/client`. The remote MCP endpoint runs inside the server process, but it still calls the portal's HTTP handler (through an in-process transport) rather than the store, so `guard`, role checks, object checks and auditing run exactly as for any other caller.
- **The token defines the reach.** The token's owner and scopes decide what a call may do. The MCP server additionally hides tools whose scopes the token lacks, so the model is not offered tools that would fail.

### 3.3 One request today: `francois catalog push`

This is the only scripted path that works today. Following it shows every part involved.

1. `francois catalog push --server https://slop.boutique --artifact-id polis --version 1.3.0 --html dist/polis.html --publish` reads the token from `$FRANCOIS_TOKEN` or `--token-file` (`cmd/francois/cmds/push.go:66-79`).
2. `studioclient.Push` builds a multipart form in memory (`pkg/catalog/studioclient/client.go:116-137`) and sends `POST /api/studio/releases` with `Authorization: Bearer frt_…` (`client.go:78-114`).
3. The portal mux routes the request to `c(s.studioUpload)`, where `c` is `guard(creator, …)` (`studio.go:35`).
4. `guard` finds no session cookie, sees the `frt_` bearer, confirms the path starts with `/api/studio/`, and calls `Store.ResolveToken`, which in one `UPDATE … RETURNING` checks the hash, expiry and revocation and records `last_used_at` (`auth.go:88-99`, `store/tokens.go:65-73`). The principal becomes `{User, Token}`. The role check requires `creator` or `admin`. CSRF is skipped because a bearer token is not an ambient browser credential (`auth.go:116-118`).
5. `studioUpload` checks that the caller may upload to this artifact (`mayUpload`, `studio.go:532`), validates the HTML and manifest, and creates a draft release through `releases.Publisher`, which writes an audit row `release.draft`.
6. The client uploads each `--attach` file to `POST /api/studio/releases/{id}/attachments` and then calls `POST /api/studio/releases/{id}/publish`.
7. The command prints one row: release ID, artifact, version, state, attachment count.

### 3.4 One request after this ticket: an agent grants a complimentary plan

An administrator asks Claude Code: "Give ramon@example.com thirty days of Studio, reason: beta tester."

1. Claude Code has `francois mcp` configured with an admin token whose scopes are `admin:read admin:write`. It lists tools and sees `find_customers` and `grant_complimentary_access`.
2. The model calls `find_customers {"query":"ramon@example.com"}`. The tool calls `client.Admin.Users(ctx, UserQuery{Q: "ramon@example.com"})`, which sends `GET /api/admin/users?q=ramon%40example.com` with the bearer token.
3. `guard(admin, scopeAdmin, …)` resolves the token, computes the required scope from the method (`GET` → `admin:read`), confirms the token has it, confirms the owner's role is still `admin`, and records the token ID in the request context for auditing.
4. The model calls `grant_complimentary_access {"userId":"usr_…","days":30,"plan":"studio","reason":"beta tester"}`. The client sends `POST /api/admin/users/usr_…/access-grants`; `guard` now requires `admin:write`. `adminGrant` validates the plan, calls `Store.GrantAccess`, and writes an audit row `user.access_grant` whose new `token_id` column names the token.
5. The tool returns the grant as structured output and a one-line text summary. The administrator can later see in `catalog audit list --actor usr_admin` that the grant was made through that token.

## 4. Current state, with evidence

### 4.1 The guard

`Server.guard(level, h)` (`pkg/catalog/auth.go:78-131`) runs these checks in order. Each failure writes `{"error":{"code","message"}}` and stops.

| Step | Condition | Result |
| --- | --- | --- |
| 1 | Session lookup fails | 503 `unavailable` |
| 2 | No session, bearer starts with `frt_`, path not under `/api/studio/` | 401 `token_not_allowed` |
| 3 | Bearer token unknown, expired or revoked | 401 `token_invalid` |
| 4 | `level ≥ session` and no principal | 401 `login_required` |
| 5 | `level ≥ enabled` and user disabled | 403 `account_disabled` |
| 6 | `level ≥ creator` and role not creator/admin | 403 `creator_required` |
| 7 | `level ≥ admin` and role not admin | 403 `admin_required` |
| 8 | Mutation with a session: `Origin` not the portal | 403 `invalid_origin` |
| 9 | Mutation with a session: `X-CSRF-Token` wrong | 403 `csrf_failed` |

Three details matter for the design:

- A session cookie takes precedence: the bearer header is examined only when `p == nil` (`auth.go:88`).
- The token check is a **path prefix test inside guard**, not a property of the route. The route registration (`a(...)`, `c(...)`) says nothing about tokens.
- Role and disabled checks run on every request against the current `users` row, so demoting or disabling a user immediately disables their tokens beyond their new role.

### 4.2 Personal API tokens

- Table `api_tokens(id, user_id, name, token_hash UNIQUE, created_at, last_used_at, expires_at, revoked_at)` (`store/migrations/004_storefront.sql:80-89`). There is no scope column.
- `CreateToken` returns `frt_` + base64url of 32 random bytes; only `Hash(secret)` is stored (`store/tokens.go:36-45`).
- Management routes are `GET/POST /api/studio/tokens` and `DELETE /api/studio/tokens/{id}` (`studio.go:72-74`). They sit behind `guard(creator, …)`, so **subscribers cannot create tokens**, and `sessionOnly` (`studio.go:1203-1209`) rejects token callers so a token cannot mint more tokens.
- Creation accepts `{name ≤80, days 0..366}`; `0` means no expiry (`studio.go:1223-1247`). Creation and revocation are audited as `token.create` / `token.revoke`.
- The UI is the studio's Tokens section (`ui/catalog/studio.tsx:1225-1260`).
- There is no administrator view of other users' tokens, and `RevokeUserSessions` (`store/sessions.go:87-101`) does not revoke tokens.

### 4.3 The admin API: what "managing customers" can do today

All routes are `guard(admin, …)` and therefore session-only today.

| Endpoint | Purpose | Notes |
| --- | --- | --- |
| `GET /api/admin/overview` (`admin.go:49`) | Counts, usage, billing health | |
| `GET /api/admin/users?q=` (`:113`) | Search non-guest users, max 200 | Returns `[{user, access, plan, source}]` |
| `GET /api/admin/users/{id}` (`:136`) | The customer view | user, access decision, entitlement, subscriptions, complimentary grants, workspaces, session count, Stripe customer ID, allowance, last 20 model calls, last 20 audit entries |
| `PATCH /api/admin/users/{id}` (`:164`) | `{role?, disabled?}` | 409 `self_lockout` protects the caller; audited |
| `POST /api/admin/users/{id}/revoke-sessions` (`:198`) | Revoke sessions, grants and live views | Tokens untouched |
| `POST /api/admin/users/{id}/access-grants` (`:210`) | Complimentary access `{days 1..3660, reason, plan?}` | Audited `user.access_grant` |
| `DELETE /api/admin/users/{id}/access-grants` (`:244`) | Revoke **all** active grants | No per-grant revoke |
| `GET /api/admin/instances?q=` (`:259`), `GET …/{id}` (`:275`) | All instances, detail | |
| `POST /api/admin/instances/{id}/suspend`, `/resume` (`:304`) | Instance state | No admin delete |
| `GET /api/admin/settings` (`:319`) | Active settings, history, model list | |
| `PATCH /api/admin/settings` (`:335`) | Replace the whole settings document | No concurrency check |
| `GET /api/admin/billing?state=` (`:354`) | Stripe inbox, checkouts, health | |
| `POST /api/admin/billing/reconcile` (`:375`), `POST …/events/{id}/retry` (`:395`) | Re-sync one customer, retry one event | |
| `GET /api/admin/usage?user=&limit=` (`:407`) | Usage summary and model calls | |
| `POST /api/admin/usage/adjustments` (`:422`) | `{userId, credits ±100000, reason}` | Audited inside the store |
| `GET /api/admin/audit?subject=` (`:453`) | Last 200 audit rows | Subject filter only |

Plans and prices are not tables. They are part of the versioned settings document (`pkg/catalog/settings/settings.go:56-70` for `Plan`, `:90-98` for `Document`), and every plan edit is a full `PATCH /api/admin/settings`. Entitlements are computed per request by `resolveEntitlement` (`pkg/catalog/entitlement.go:42-70`) from the user's role, Stripe subscription projections and complimentary grants.

Creator profiles — needed before a creator can create artifacts (409 `creator_profile_required`, `studio.go:551-553`) — are managed under the studio API, admin-only for create, delete and linking a user (`studio.go:1100-1165`).

### 4.4 The studio and account APIs

The studio API covers artifacts and their public pages, releases and attachments, media, journal posts, creators, Markdown preview, plan names and tokens. The account API covers the caller's own account, Stripe Checkout and portal links, and instances (create, rename, delete, members, upgrade, launch). The complete route tables are reproduced in §7 where they are mapped to client methods.

One account route cannot work with a token by construction: `POST /api/instances/{id}/launch` issues a launch code **bound to the browser session** (`instances.go:182-212`), and a token principal has no session. The design excludes it from token access explicitly.

### 4.5 Response and error conventions

- Success responses are the bare JSON value, without a wrapper (`pkg/catalog/httpx/httpx.go:26`). Creation returns 201.
- Errors are `{"error":{"code":"…","message":"…"}}` (`httpx.go:29-31`). Release uploads add a sibling `"checks"` array (`studio.go:606-609`).
- Request bodies are read by `httpx.Read` with a byte limit; unknown JSON fields are tolerated everywhere except in the settings document, which is parsed strictly (`settings.go:123-131`).
- The client must treat `code` as the stable contract and `message` as text for humans. The codes in use, by status:

| Status | Codes |
| --- | --- |
| 400 | `invalid_request`, `invalid_upload`, `invalid_poster`, `invalid_signature`, `invalid_argument`, `unsupported_type`, `images_unavailable` |
| 401 | `login_required`, `token_not_allowed`, `token_invalid`, `grant_required`, `grant_invalid`, `launch_invalid`, `session_ended` |
| 402 | `allowance_exhausted` |
| 403 | `account_disabled`, `admin_required`, `creator_required`, `csrf_failed`, `invalid_origin`, `session_required`, `not_editor`, `not_uploader`, `not_member`, `owner_required`, `plan_required`, `email_unverified`, `guest`, `foreign_session`, `instance_suspended`, `demo_disabled`, `demo_expired`, `over_plan_limit`, `owner_inactive`, `release_unavailable`, `not_declared`, `read_only`, `permission_denied`, `model_not_allowed`, `model_not_in_plan` |
| 404 | `not_found`, `release_unavailable`, `user_not_found`, `no_customer`, `cancelled` |
| 409 | `conflict`, `self_lockout`, `artifact_exists`, `creator_profile_required`, `version_exists`, `no_draft`, `slug_taken`, `media_in_use`, `too_many_attachments`, `instance_limit`, `member_limit`, `subscription_open`, `no_billing_account`, `checkout_superseded` |
| 413, 415, 421 | `too_large`, `quota_or_state`; `media_type`; `misdirected` |
| 422 | `invalid_settings`, `invalid_release` |
| 429 | `rate_limited`, `demo_limit`, `daily_limit` |
| 5xx | `unavailable`, `billing_unavailable`, `database_unavailable`, `file_unavailable`, `login_unavailable`, `model_unavailable`, `sampling_disabled` |

Several of these (`grant_*`, `launch_invalid`, `not_declared`, `read_only`, `permission_denied`) belong to the instance runtime and never reach the client.

### 4.6 The existing client and CLI

- `pkg/catalog/studioclient` (`client.go`, 215 lines) has `Check`, `Attach` and `Push`. Its `do` method sends the bearer token, reads at most 4 MiB, and turns a status ≥ 300 into `*Error{Status, Code, Message, Checks}` (`client.go:78-114`). It has a `Host` override for tests, because the server routes by `Host`. It has no retries and does no logging; the generated `logcopter.go` declares a `log` it never uses.
- Commands (`cmd/francois/cmds/`): `serve`, `import`, `update`, `models list` operate on the local single-user store. Under `catalog`: `serve`, `migrate`, `publish`, `settings`, `users list|promote|disable|enable|revoke-sessions|grant-access` all open PostgreSQL directly with `--database-url(-file)` (`catalog.go:36-64`). Only `catalog push` uses HTTP.
- The help system is set up but empty: `help.NewHelpSystem()` in `cmd/francois/main.go:29`, no embedded help pages.

### 4.7 Tests and tooling

- `pkg/catalog/catalog_test.go:34-64` creates a fresh PostgreSQL database per test when `TEST_DATABASE_URL` is set and skips otherwise. `setup(t)` (`:80-105`) starts the real server under `httptest`, with two sites (`portal.test`, `boutique.test`). `e.user(name, role, access)` creates a user and a session; `e.portal(...)` makes a JSON request with cookie and CSRF.
- `pkg/catalog/tokens_test.go:15-110` is the model for client tests: mint a token through the session API, build a client against `e.ts.URL` with `Host = "portal.test"`, call it, assert results and error codes.
- `cmd/francois/cmds/update_test.go:17-46` has `executeJSON(t, args...)`, which runs a command with `--format json` and decodes its rows.
- `make ci-check` runs `fmt-check lint logcopter-check test build` (`Makefile:46`). Every new package under `pkg/` needs a generated `logcopter.go` (`make logcopter-generate`); CI fails otherwise.

### 4.8 MCP prior art in this workspace

| Project | Library | Transport | Auth |
| --- | --- | --- | --- |
| `go-go-golems/go-go-mcp` (v0.2.x) `pkg/embeddable` | official `modelcontextprotocol/go-sdk` v1.7.0 | stdio, SSE, Streamable HTTP | none, embedded dev, external OIDC (JWT via JWKS) |
| `go-go-golems/smailnail` | go-go-mcp v0.0.19 (mark3labs backend) | Streamable HTTP mounted in its server | external OIDC; principal mapped to a local user in a tool middleware |
| `go-go-golems/jesus` | go-go-mcp v0.0.19 | stdio / HTTP | none |
| `2026-08-25--corewars` | `mark3labs/mcp-go` v0.58.0 directly | Streamable HTTP | none; tools are thin wrappers over the REST logic |

- The Go module cache has `modelcontextprotocol/go-sdk` v1.7.0, which provides `mcp.NewServer`, typed `mcp.AddTool[In, Out]` with JSON Schema inferred from struct tags (`mcp/server.go:561`), `mcp.StdioTransport`, `mcp.NewStreamableHTTPHandler` with a `Stateless` option (`mcp/streamable.go:232`), `mcp.NewInMemoryTransports` for tests (`mcp/transport.go:160`), and `auth.RequireBearerToken` / `auth.TokenInfoFromContext` (`auth/auth.go:83-97`).
- No library bridges Glazed `GlazeCommand`s to MCP tools: go-go-mcp's config provider runs only `WriterCommand`s and panics on the others (`go-go-mcp/pkg/tools/providers/config-provider/tool-provider.go:362-367`).

## 5. Gap analysis

| # | Need | Today | Proposed | Phase |
| --- | --- | --- | --- | --- |
| G1 | Scripts and agents reach the admin API | Tokens rejected outside `/api/studio/` | Token scopes; guard checks the route's scope | 1 |
| G2 | Subscribers script their own account | Only creators/admins can mint tokens | Token management moves to `/api/tokens` for every non-guest user; `account` scope | 1 |
| G3 | Know who a token is | No identity endpoint for tokens outside studio | `GET /api/me` for any valid token | 1 |
| G4 | Attribute automated changes | Audit rows record the user only | `admin_audit.token_id` | 1 |
| G5 | Respond to a leaked token | Admins cannot see or revoke others' tokens | Admin token list and revoke; `revoke-sessions` can include tokens | 1 |
| G6 | Revoke one complimentary grant | Only "revoke all" | `DELETE /api/admin/access-grants/{id}` | 2 |
| G7 | See all subscriptions | Only per user | `GET /api/admin/subscriptions` | 2 |
| G8 | Stop a renewal for a customer | Only the customer's own Stripe portal | Cancel / resume at period end | 2 |
| G9 | Edit one plan safely | Whole-document replace, last write wins | `ETag` / `If-Match` on settings | 2 |
| G10 | Clean up an instance as admin | Owner-only delete | `DELETE /api/admin/instances/{id}` | 2 |
| G11 | Search the audit log | Subject filter, newest 200 | `actor`, `action`, `before`, `limit` filters | 2 |
| G12 | Typed access from Go | Three studio calls | `pkg/catalog/client` | 3 |
| G13 | Every operation on the command line | `push` only | `francois catalog <resource> <verb>` | 4 |
| G14 | Agents operate the shop | No MCP | `francois mcp` (stdio) | 5 |
| G15 | Remote agents | No MCP endpoint | `/mcp` in `catalog serve` | 6 |

## 6. Proposed architecture

### 6.1 Token scopes

**Scopes and the routes they open.**

| Scope | Routes | Who may mint it |
| --- | --- | --- |
| `studio` | `/api/studio/*` | creator, admin |
| `account` | `GET /api/account`, `/api/billing/{checkout,portal,refresh}`, `/api/instances/*` except `…/launch` | any non-guest user |
| `admin:read` | `GET /api/admin/*` | admin |
| `admin:write` | every method under `/api/admin/*`; implies `admin:read` | admin |

`GET /api/me` accepts any valid token regardless of scope. Token management (`/api/tokens*`) and instance launch remain session-only. Public routes keep rejecting bearer tokens with `token_not_allowed`, as today, so a misconfigured client fails loudly instead of silently acting anonymously.

A scope is a ceiling, not a grant: the role checks in `guard` and the object checks in handlers still run. An `admin:write` token whose owner is later demoted to `creator` fails with `admin_required`.

**Minting rules** (enforced on `POST /api/tokens`):

- `scopes` must be a non-empty subset of what the caller's role may mint (table above).
- Tokens with an admin scope must expire within 1–90 days. Other tokens keep the current 0–366 range, where 0 means no expiry.
- A token cannot mint tokens (unchanged `sessionOnly`).

**Data.**

```sql
-- 008_token_scopes.sql
ALTER TABLE api_tokens ADD COLUMN scopes text[] NOT NULL DEFAULT '{studio}';
ALTER TABLE api_tokens ALTER COLUMN scopes DROP DEFAULT;
ALTER TABLE admin_audit ADD COLUMN token_id text;
CREATE INDEX admin_audit_actor ON admin_audit (actor_id, created_at DESC);
CREATE INDEX admin_audit_subject ON admin_audit (subject, created_at DESC);
```

Existing tokens receive `{studio}`, which is exactly what they can do today, so nothing changes for `catalog push` users. Dropping the default afterwards makes every later insert state its scopes.

**Route declarations.** The scope becomes part of the route, next to the level, instead of a path test inside `guard`:

```go
// pkg/catalog/auth.go
type scope string

const (
	noToken      scope = ""        // bearer tokens rejected (public pages, token management)
	anyToken     scope = "*"       // any valid token (GET /api/me)
	scopeStudio  scope = "studio"
	scopeAccount scope = "account"
	scopeAdmin   scope = "admin"   // admin:read for GET/HEAD, admin:write otherwise
)

func (s *Server) guard(level int, sc scope, h http.HandlerFunc) http.HandlerFunc
```

Every call site changes (`a(...)` becomes `guard(admin, scopeAdmin, h)`, `c(...)` becomes `guard(creator, scopeStudio, h)`, account routes pass `scopeAccount`, and so on). No compatibility wrapper is kept; the compiler finds every call site.

#### Decision: scopes on tokens rather than a second credential type

- **Context:** The admin API must be reachable by scripts and agents, but FRANCOIS-004 deliberately kept tokens away from it "so a leaked publishing token cannot change users or billing" (FRANCOIS-004 design §5.7).
- **Options considered:** (a) replay a browser session cookie plus CSRF token from scripts; (b) a separate "admin token" type with its own table; (c) scopes on the existing tokens; (d) OAuth access tokens from ZITADEL for every client.
- **Decision:** (c).
- **Rationale:** Scopes keep the FRANCOIS-004 property — a `studio` token still cannot reach `/api/admin` — while letting an administrator deliberately mint a narrow admin token. Cookie replay (a) puts a 30-day browser credential in scripts and requires CSRF handling. A second table (b) duplicates hashing, expiry, revocation and UI. OAuth (d) is the right tool for third-party remote clients and is kept for §6.8, but requires interactive flows that a cron job or a local agent does not need.
- **Consequences:** Every route declares a scope; tests must cover the scope matrix. Admin tokens are short-lived by rule, so long-running automation must rotate them.
- **Status:** proposed

#### Decision: required scope derived from the HTTP method for the admin API

- **Context:** Read-only admin automation (reports, dashboards, an agent answering questions) should not be able to change anything.
- **Options considered:** per-route scopes for all 25 admin routes; one `admin` scope; method-derived `admin:read` / `admin:write`.
- **Decision:** method-derived. `GET` and `HEAD` need `admin:read`; every other method needs `admin:write`.
- **Rationale:** The admin API already follows HTTP semantics: the nine admin GET handlers only read (checked in `admin.go`: overview, users, user, instances, instance, settings, billing, usage, audit call no store mutation). A method rule cannot drift as routes are added.
- **Consequences:** A future admin GET with side effects would break the rule; the security checklist (§9) includes "no side effects in admin GET handlers".
- **Status:** proposed

### 6.2 Identity and token management endpoints

`GET /api/me` (guard `session`, scope `anyToken`):

```json
{
  "user": {"id": "usr_…", "name": "Manuel", "email": "…", "role": "admin", "disabled": false},
  "token": {"id": "tok_…", "name": "laptop agent", "scopes": ["admin:read", "admin:write"], "expiresAt": "2026-11-14T00:00:00Z"},
  "creator": {"id": "crt_…", "slug": "manuel", "name": "Manuel"}
}
```

`token` is `null` for a session caller and `creator` is `null` when no profile is linked. The client calls this once to discover what it may do; the MCP server uses `token.scopes` to decide which tools to register.

Token management moves from `/api/studio/tokens` to `/api/tokens` (guard `session`, scope `noToken`), so subscribers can create `account` tokens:

| Method and path | Body | Response |
| --- | --- | --- |
| `GET /api/tokens` | — | `[APIToken]` including `scopes` |
| `POST /api/tokens` | `{name, days, scopes[]}` | 201 `{token: "frt_…", info: APIToken}` |
| `DELETE /api/tokens/{id}` | — | `{revoked: true}` |

New error codes: 403 `token_scope` ("This token lacks the admin:write scope"), 400 `invalid_scopes`. The token form moves to the account page (`ui/catalog/account.tsx`), with scope checkboxes filtered by role; the studio's Tokens section (`ui/catalog/studio.tsx:1225`) links to it.

Administrator oversight (guard `admin`, scope `scopeAdmin`):

| Method and path | Purpose |
| --- | --- |
| `GET /api/admin/users/{id}/tokens` | A user's tokens: name, scopes, created, last used, expiry, revoked |
| `DELETE /api/admin/tokens/{id}` | Revoke any token; audited `token.revoke` with the owner as subject |
| `POST /api/admin/users/{id}/revoke-sessions` | Gains an optional body `{"tokens": true}` that also revokes all of the user's tokens |

### 6.3 Auditing token actions

`guard` stores the token ID in the request context with `store.WithAuditToken(ctx, id)`. Every audit insert — `Store.Audit` (`store/users.go:122-133`) and the four transactional inserts in `store/settings.go:59`, `store/releases.go:306,342` and `store/usage.go:230` — goes through one helper that reads it:

```go
// pkg/catalog/store/audit.go
func auditInsert(ctx context.Context, q querier, actor, action, subject string, summary any) error {
	var a, tok any
	if actor != "" { a = actor }
	if id := auditToken(ctx); id != "" { tok = id }
	raw, _ := json.Marshal(summary)
	_, err := q.Exec(ctx, `INSERT INTO admin_audit(actor_id,token_id,action,subject,summary) VALUES($1,$2,$3,$4,$5)`, a, tok, action, subject, raw)
	return err
}
```

The store does not read HTTP state; it reads a plain context value that the HTTP layer sets. `AuditEntry` gains `tokenId`.

### 6.4 Customer-management endpoints

All are guard `admin`, scope `scopeAdmin`, and audited.

| Method and path | Purpose | Store / gateway work |
| --- | --- | --- |
| `GET /api/admin/users?q=&role=&plan=&limit=` | Existing search gains role and plan filters and a limit (≤500) | `Users` gains a filter struct; plan filtering is done after entitlement resolution, as the existing handler already computes `plan` per row |
| `DELETE /api/admin/access-grants/{id}` | Revoke one complimentary grant; revoke live views if the user is no longer paid (same follow-up as the existing revoke-all) | `RevokeAccessGrant(ctx, id) (AccessGrant, error)` |
| `GET /api/admin/subscriptions?status=&plan=&limit=` | Every subscription projection with its user and plan | `AllSubscriptions(ctx, filter)` joining `billing_subscriptions`, `users` |
| `POST /api/admin/subscriptions/{id}/cancel` | Set `cancel_at_period_end` in Stripe | `Gateway.SetCancelAtPeriodEnd(ctx, id, true)`, then `EnqueueRefresh` so the projection updates through the normal worker |
| `POST /api/admin/subscriptions/{id}/resume` | Undo a pending cancellation | same with `false` |
| `DELETE /api/admin/instances/{id}` | Delete any instance and its data; revoke its views | existing `DeleteInstance` plus `Runtime.RevokeInstance`; audited `instance.delete` |
| `GET /api/admin/settings` | Adds response header `ETag: "<revision>"` | — |
| `PATCH /api/admin/settings` | Requires `If-Match: "<revision>"`; 412 `settings_conflict` when another revision was activated in between, 428 `revision_required` when the header is missing | `SaveSettings(ctx, actor, base, doc)` takes an advisory transaction lock and compares the newest revision with `base` |
| `GET /api/admin/audit?subject=&actor=&action=&before=&limit=` | Filtered, paged audit log (`before` is an entry ID; `limit` ≤ 500) | `AuditLog(ctx, AuditFilter)` |

#### Decision: only cancel-at-period-end is exposed for subscriptions

- **Context:** "Managing customers" could include refunds, immediate cancellation, plan changes and coupons. Each moves money or changes what a customer is charged, and Stripe already has audited interfaces for them (the dashboard for the owner, the customer portal for the customer).
- **Options considered:** expose all Stripe operations; expose none; expose the reversible ones.
- **Decision:** expose cancel-at-period-end and its undo.
- **Rationale:** Both are reversible until the period ends, cover the common support request ("stop my renewal"), and do not move money. Projections continue to flow only from Stripe through the webhook worker (`payments/service.go:246-305`), so the local database never disagrees with Stripe.
- **Consequences:** The Gateway interface (`payments/gateway.go:40-53`) gains one method and the fake gateway in `service_test.go` gains it too. Refunds remain a dashboard task, which the CLI help states.
- **Status:** proposed

#### Decision: optimistic concurrency on the settings document

- **Context:** Plans, prices, models, limits, demo settings and storefront wording all live in one document, replaced whole by `PATCH /api/admin/settings` (`admin.go:335-352`). A CLI command that edits one plan reads the document, changes one plan and writes it back. If an administrator saves in the browser in between, the CLI silently undoes their change.
- **Options considered:** per-section endpoints (`PATCH /api/admin/plans/{key}`, …); a JSON Patch endpoint; `If-Match` on the whole document.
- **Decision:** `ETag` / `If-Match` with the revision number, required on every PATCH, including from the browser UI.
- **Rationale:** It closes the race for every client with one mechanism, keeps validation in one place (`settings.Document.Validate`), and keeps the settings document as the single source of plan truth. Per-section endpoints would multiply validation paths.
- **Consequences:** The admin settings page must send `If-Match` and show a "settings changed, reload" message on 412. API callers always pass the revision they read; revision 0 (only defaults so far) is compared like any other, so two editors starting from the defaults cannot overwrite each other. The unconditional write is a separate store path (`SaveSettings`, a negative base internally) used only by the break-glass `catalog db settings` command, never by the HTTP API.
- **Status:** proposed

### 6.5 The Go client: `pkg/catalog/client`

The client replaces `pkg/catalog/studioclient` (moved, not aliased).

**Layout.**

```text
pkg/catalog/client/
  client.go        Client, Option, New, do, doMultipart, download
  errors.go        Error, IsCode, Checks
  transport.go     retry policy; handlerTransport for in-process use
  types.go         wire types not owned by a pure package (User, Customer, Instance, …)
  me.go            Me
  studio.go        Studio: artifacts, page drafts, releases, attachments, media, posts, creators, markdown, plans
  admin.go         Admin: overview, users, grants, tokens, subscriptions, instances, settings, billing, usage, audit
  account.go       Account: account, checkout/portal links, own instances and members
  push.go          the push workflow (check → draft → attachments → publish)
  logcopter.go     generated
```

**Construction.**

```go
type Client struct {
	Studio  Studio  // each holds a *Client
	Admin   Admin
	Account Account
	// unexported: base URL, token, *http.Client, host override
}

type Option func(*Client)

func WithHTTPClient(h *http.Client) Option
func WithHost(host string) Option          // tests and tunnels: the server routes by Host
func WithHandler(h http.Handler) Option    // in-process transport (remote MCP, §6.8)
func WithRetries(n int) Option             // default 3, GET/HEAD only

func New(server, token string, opts ...Option) *Client
func (c *Client) Me(ctx context.Context) (Me, error)
```

**Types.** Pure packages are reused: `settings.Document`, `settings.Plan` and `billing.Decision` / `billing.Subscription` import nothing but the standard library and logcopter. Everything else is a client-owned wire struct that mirrors the JSON (the TypeScript definitions in `ui/catalog/api.ts:3-255` and `ui/catalog/studio-api.ts:6-62` are a useful checklist). The client never imports `pkg/catalog/store`, which pulls in pgx.

#### Decision: client-owned wire types, checked by contract tests

- **Context:** Many handlers return ad-hoc `map[string]any` values, and the store types carry database concerns.
- **Options considered:** reuse `store` types; generate types from an OpenAPI document; hand-written client types with tests that detect drift.
- **Decision:** hand-written client types, plus contract tests that decode real server responses with `DisallowUnknownFields`.
- **Rationale:** There is no OpenAPI document to generate from, and writing one for ~85 routes is a project of its own. Contract tests catch the failure that matters — the server adds or renames a field and the client silently drops it — while production decoding stays tolerant.
- **Consequences:** Every response type has a contract test. A server change that adds a field fails the client tests until the client type is updated, which is the intended prompt.
- **Status:** proposed

**Behaviour.**

- Every request carries `Authorization: Bearer <token>` and `Accept: application/json`; mutations carry `Content-Type: application/json` or a multipart boundary.
- Responses ≥ 300 decode into `*Error{Status, Code, Message, Checks}`; a body that is not the error envelope becomes `Code: "http_<status>"`. `client.IsCode(err, "not_found")` is the comparison helper.
- GET and HEAD retry up to three times on connection errors and on 502, 503 and 504, with exponential backoff from 250 ms. Rolling deployments produce brief 502s from Traefik. Mutations are never retried automatically.
- JSON responses are limited to 8 MiB; downloads (`attachments`, `release files`) stream to an `io.Writer` instead.
- Multipart uploads stream through `io.Pipe` (as `Attach` already does, `studioclient/client.go:154-180`), so large media and attachments are never held in memory.
- Debug logging goes through the package's logcopter logger: method, path, status and duration; never headers or bodies.

**Representative method.**

```go
// GrantAccess gives a customer complimentary access. plan "" means the highest plan.
func (a Admin) GrantAccess(ctx context.Context, userID string, in GrantRequest) (AccessGrant, error) {
	var out AccessGrant
	err := a.c.do(ctx, http.MethodPost, "/api/admin/users/"+url.PathEscape(userID)+"/access-grants", in, &out)
	return out, err
}
```

### 6.6 The command tree

#### Decision: API commands under `catalog`, database commands under `catalog db`

- **Context:** `catalog users list` already exists and opens PostgreSQL directly; the new API command of the same name would collide. The direct commands are still needed for bootstrap (promoting the first administrator before any token exists) and for break-glass access from inside the pod.
- **Options considered:** a new top-level group (`francois api …`); prefixing the new commands (`catalog remote-users …`); moving the database commands to `catalog db …`.
- **Decision:** move the database commands to `catalog db` (`catalog db users …`, `catalog db settings`, `catalog db publish`). `catalog serve` and `catalog migrate` stay where they are. `catalog push` becomes `catalog releases push`, next to the other release commands (owner decision, 2026-10-15).
- **Rationale:** The everyday commands get the short names; the dangerous ones are visibly separate. The production deployment uses only `catalog serve --migrate` (k3s `gitops/kustomize/francois/deployment.yaml:61-93`), so nothing deployed depends on the moved names.
- **Consequences:** `README.md:193-198`, `README.md:223-230` and `doc/production.md:394` change. AGENT.md forbids compatibility aliases without asking, so the old names, including `catalog push`, are removed.
- **Status:** proposed

**Connection section.** Every API command shares one Glazed section, `api`:

| Field | Environment | Meaning |
| --- | --- | --- |
| `--server` | `FRANCOIS_SERVER` | Portal origin, e.g. `https://slop.boutique` |
| `--token-file` | `FRANCOIS_TOKEN` (the token itself) | File holding the token |
| `--host` | — | Host header override (tunnels) |
| `--timeout` | — | Per-request timeout, default 2m |

**Commands.** Rows are printed through Glazed, so `--output json|yaml|csv|table` and `--fields` work everywhere.

```text
francois catalog whoami

francois catalog artifacts   list | show ID | create | update ID [--title …] [--visibility …] [--creators …]
                             page show ID | page set ID --body-file F [--cover M] [--gallery M,…] | page publish ID | page discard ID
francois catalog releases    list [--artifact ID] | check | push (workflow) | publish ID | withdraw ID [--suspend-pinned]
                             preview ID | delete ID
francois catalog attachments add RELEASE FILE [--label …] | update ID | delete ID | download ID [-o F]
francois catalog media       list | upload FILE [--alt …] | update ID [--alt …] [--poster M] | delete ID
francois catalog posts       list | show ID | create --slug … --title … [--body-file F] | update ID | publish ID | unpublish ID | delete ID
francois catalog creators    list | create | update ID | delete ID

francois catalog users       list [--q …] [--role …] [--plan …] | show ID | set-role ID ROLE | disable ID | enable ID
                             revoke-sessions ID [--tokens] | grant ID --days N --reason … [--plan K] | revoke-grant GRANT_ID
                             tokens ID
francois catalog subscriptions list [--status …] [--plan …] | cancel ID | resume ID
francois catalog instances   list [--q …] | show ID | suspend ID | resume ID | delete ID
francois catalog plans       list | show KEY | set KEY [--monthly-credits N] [--instances N] [--price-month price_…] …
francois catalog settings    show | history | apply --file F | edit (opens $EDITOR on the document, applies with If-Match)
francois catalog billing     status [--state …] | reconcile USER_ID | retry-event ID
francois catalog usage       summary | calls [--user ID] | adjust USER_ID --credits N --reason …
francois catalog audit       list [--subject …] [--actor …] [--action …] [--limit N]

francois catalog account     show | instances list | instances create --release ID | instances rename ID T
                             instances delete ID | members add ID EMAIL --role editor|viewer | members remove ID USER

francois catalog db          users list | users promote | users disable | users enable | users revoke-sessions
                             users grant-access | settings --file F | publish …
```

Destructive commands (`disable`, `revoke-*`, `withdraw`, `delete`, `cancel`) ask for confirmation on a terminal and accept `--yes` for scripts. There is no command for managing one's own tokens: `/api/tokens` stays session-only, so a token can never mint or revoke tokens. Owners manage tokens on the account page; administrators use `users tokens ID` and `users revoke-token ID`, which call the admin API.

### 6.7 The MCP server: `francois mcp`

#### Decision: go-go-mcp v0.3.0 `pkg/embeddable`

- **Context:** The workspace has three ways to build an MCP server (§4.8). francois is security-scanned (GoSec, govulncheck, CodeQL) and builds a static binary with `CGO_ENABLED=0` (`Dockerfile:23`). The first version of this guide chose the official go-sdk directly because go-go-mcp v0.2.x compiled 318 packages, and because its `external_oidc` mode could not verify ZITADEL's opaque tokens.
- **Options considered:** go-go-mcp `pkg/embeddable`; `mark3labs/mcp-go`; `modelcontextprotocol/go-sdk` v1.7.0 directly.
- **Decision:** go-go-mcp v0.3.0 `pkg/embeddable` (on go-sdk v1.7.0).
- **Rationale:** GO-GO-MCP-EMBEDDABLE-AUTH (go-go-mcp PR #89, released as v0.3.0 on 2026-10-16) removed both obstacles. The embedded development OIDC server is now linked only through `pkg/embeddable/embeddeddev`, and `external_oidc` verifies opaque tokens by RFC 7662 introspection, accepting only bearer access tokens. What remains in go-go-mcp's favour is what francois would otherwise write itself:
  - `mcp start | list-tools | test-tool` commands;
  - `MountHTTPHandlers` for the remote phase;
  - per-tool scope policies (`WithToolAuthorization`);
  - hooks;
  - the external OIDC verifier the connector phase needs.
- **Measurement (`go list -deps`, non-standard packages compiled):**

  | Import | Packages |
  | --- | --- |
  | go-sdk `mcp` + `auth` | 29 |
  | go-go-mcp `pkg/embeddable` v0.2.5 | 318 |
  | go-go-mcp `pkg/embeddable` v0.3.0 | 65 |

  The remaining difference over go-sdk is mostly `geppetto/pkg/helpers` (22 packages, including watermill) used by two reflection helpers in go-go-mcp's `pkg/tools`; removing it is a go-go-mcp follow-up.
- **Consequences:**
  - go-go-mcp v0.3.0 requires Glazed `v1.4.4-0.20260823170018-a7fdfda5e3c6` (francois has v1.4.1) and `go 1.26.3` (francois declares 1.26.1 with toolchain 1.26.9). Both bumps happen in phase 5.
  - go-go-mcp tool handlers receive `map[string]any`. francois keeps typed tools through a small generic adapter (below).
  - `embedded_dev` is never linked, so francois carries no Fosite or SQLite.
- **Status:** accepted (owner merged and released go-go-mcp v0.3.0 for this purpose)

#### Decision: hand-written tools over the client, not generated from CLI commands

- **Context:** One could expose each Glazed command as a tool by converting its flags to a JSON Schema (`CommandDescription.ToJsonSchema`).
- **Options considered:** generate tools from Glazed commands; write tools against the client.
- **Decision:** write tools against the client.
- **Rationale:** No working bridge exists for `GlazeCommand` (§4.8). Tool descriptions for a model need different wording from CLI help (when to use the tool, what to call first). Tools need annotations and structured output, which flags do not describe. The client already holds all the logic, so a tool is 10–20 lines.
- **Consequences:** Adding an operation means adding a client method, a command and a tool. A test checks that every client method is reachable from at least one tool or is listed as intentionally excluded.
- **Status:** proposed

**Package.**

```text
pkg/catalog/mcpserver/
  server.go     Options(client, Settings) []embeddable.ServerOption: tools filtered by scopes, toolsets, read-only
  typed.go      tool[In, Out]: typed handlers over go-go-mcp's map-based ToolHandler
  tools_me.go   whoami
  tools_studio.go, tools_customers.go, tools_account.go
  files.go      path inputs restricted to --allow-dir roots (stdio mode only)
  logcopter.go
cmd/francois/cmds/mcp.go   `francois mcp` — flags: api section, --toolsets, --allow-dir, --read-only
```

**Startup.** francois builds the server configuration after it knows the token's scopes, so it owns the `mcp` command rather than using `embeddable.AddMCPCommand`, and runs go-go-mcp's backend directly:

```go
func runMCP(ctx context.Context, s MCPSettings) error {
	c := client.New(s.Server, s.Token)
	me, err := c.Me(ctx) // fails fast on a bad token
	if err != nil { return err }
	cfg := embeddable.NewServerConfig()
	opts := append([]embeddable.ServerOption{
		embeddable.WithName("francois"),
		embeddable.WithVersion(version),
		embeddable.WithDefaultTransport("stdio"),
	}, mcpserver.Options(c, mcpserver.Settings{
		Scopes:    me.Token.Scopes,
		Toolsets:  s.Toolsets,  // studio, customers, account; default: everything the scopes allow
		ReadOnly:  s.ReadOnly,  // register only tools annotated read-only
		AllowDirs: s.AllowDirs, // default: current directory
		Mode:      mcpserver.Local,
	})...)
	for _, o := range opts {
		if err := o(cfg); err != nil { return err }
	}
	backend, err := embeddable.NewBackend(cfg)
	if err != nil { return err }
	return backend.Start(ctx)
}
```

**Typed tools.** go-go-mcp's `ToolHandler` is `func(ctx, map[string]any) (*protocol.ToolResult, error)`. A generic adapter keeps francois's tools typed: the input schema is inferred from the input struct (with `github.com/google/jsonschema-go`, the library go-sdk itself uses), arguments are decoded with `encoding/json`, and the typed output becomes the result's structured content.

```go
func tool[In, Out any](name, description string, ann protocol.ToolAnnotations,
	fn func(ctx context.Context, in In) (Out, string, error)) embeddable.ServerOption {
	schema := must(jsonschema.For[In](nil))
	out := must(jsonschema.For[Out](nil))
	handler := func(ctx context.Context, args map[string]any) (*protocol.ToolResult, error) {
		var in In
		raw, _ := json.Marshal(args)
		if err := json.Unmarshal(raw, &in); err != nil {
			return protocol.NewToolResult(protocol.WithError("invalid arguments: " + err.Error())), nil
		}
		result, summary, err := fn(ctx, in)
		if err != nil {
			return protocol.NewToolResult(protocol.WithError(toolError(err))), nil
		}
		return protocol.NewToolResult(protocol.WithText(summary), protocol.WithStructuredContent(result)), nil
	}
	return embeddable.WithTool(name, handler,
		embeddable.WithDescription(description),
		embeddable.WithSchema(schema),
		embeddable.WithOutputSchema(out),
		embeddable.WithToolAnnotations(ann))
}
```

**A tool.**

```go
type grantIn struct {
	UserID string `json:"userId" jsonschema:"customer user ID from find_customers"`
	Days   int    `json:"days" jsonschema:"length of the grant in days, 1 to 3660"`
	Reason string `json:"reason" jsonschema:"why the grant is made; stored in the audit log"`
	Plan   string `json:"plan,omitempty" jsonschema:"plan key from list_plans; empty means the highest plan"`
}

tool("grant_complimentary_access",
	"Give a customer free access to a plan for a number of days. Use find_customers first to get the userId.",
	protocol.ToolAnnotations{DestructiveHint: ptr(false)},
	func(ctx context.Context, in grantIn) (client.AccessGrant, string, error) {
		g, err := c.Admin.GrantAccess(ctx, in.UserID, client.GrantRequest{Days: in.Days, Reason: in.Reason, Plan: in.Plan})
		if err != nil { return g, "", err }
		return g, fmt.Sprintf("Granted %s until %s.", planOr(g.Plan, "the highest plan"), g.Until.Format(time.DateOnly)), nil
	})
```

A failed call returns a result with `isError: true` and the text `code: message`, so the model sees `plan_required: …` and can correct itself. `toolError` formats `client.Error` and appends release checks when present.

**Tool catalogue.** Scope decides registration; annotations follow the HTTP semantics.

| Toolset | Tools (read-only in *italics*) | Scope |
| --- | --- | --- |
| — | *whoami* | any |
| studio | *list_artifacts*, *get_artifact*, create_artifact, update_artifact, save_page_draft, publish_page_draft, discard_page_draft, *list_releases*, *check_release*, push_release, publish_release, withdraw_release, delete_draft_release, preview_release, add_attachment, delete_attachment, *list_media*, upload_media, update_media, delete_media, *list_posts*, *get_post*, create_post, update_post, publish_post, unpublish_post, delete_post, *list_creators*, create_creator, update_creator, delete_creator, *render_markdown* | `studio` |
| customers | *admin_overview*, *find_customers*, *get_customer*, set_customer_role, set_customer_disabled, revoke_customer_sessions, grant_complimentary_access, revoke_access_grant, *list_customer_tokens*, revoke_token, *list_subscriptions*, cancel_subscription_at_period_end, resume_subscription, *list_instances*, *get_instance*, suspend_instance, resume_instance, delete_instance, *get_settings*, *list_plans*, update_plan, set_storefront_text, *billing_status*, reconcile_customer_billing, retry_billing_event, *usage_summary*, adjust_customer_credits, *audit_log* | read-only tools: `admin:read`; others: `admin:write` |
| account | *my_account*, *list_my_instances*, create_my_instance, rename_my_instance, delete_my_instance, add_instance_member, remove_instance_member | `account` |

Destructive tools (`DestructiveHint: true`): disabling, revoking, withdrawing, deleting, cancelling and suspending. MCP clients use the annotation to ask the human before running them.

**File inputs.** In local mode, tools that upload accept paths (`htmlPath`, `manifestPath`, `attachmentPaths`, `mediaPath`). Paths are resolved, symlinks evaluated, and the result must lie under one of the `--allow-dir` roots (default: the working directory). Without this rule a model could be talked into uploading `~/.ssh/id_ed25519` as a release attachment that subscribers can download. In remote mode path inputs are rejected and tools accept inline content: `html` as a string, the manifest as a JSON object, binary files as base64 limited to 10 MiB.

**Read-only mode.** `--read-only` registers only read-only tools, whatever the token allows. It is the recommended setting for an agent that answers questions about customers.

### 6.8 Remote MCP inside `catalog serve`

Phase 6 mounts the same tools at `/mcp` on each portal host when `catalog serve --mcp` is set, using go-go-mcp's `MountHTTPHandlers` and an application-owned verifier.

```go
// pkg/catalog/server.go, in routes()
if s.cfg.MCP {
	cfg := embeddable.NewServerConfig()
	opts := append([]embeddable.ServerOption{
		embeddable.WithDefaultTransport("streamable_http"), // stateless by default in go-go-mcp
		embeddable.WithHTTPAuthVerifier(&tokenVerifier{store: s.Store, resource: site.PortalOrigin() + "/mcp"}),
	}, mcpserver.Options(clientFromRequest(s.portal, site), mcpserver.Settings{Mode: mcpserver.Remote})...)
	// apply opts to cfg ...
	if err := embeddable.MountHTTPHandlers(m, cfg); err != nil { return err }
}

// tokenVerifier implements embeddable.HTTPAuthVerifier for frt_ tokens.
func (v *tokenVerifier) ValidateBearerToken(ctx context.Context, tok string) (embeddable.AuthPrincipal, error) {
	t, u, err := v.store.ResolveToken(ctx, tok)
	if err != nil { return embeddable.AuthPrincipal{}, err }
	scopes, _ := embeddable.ParseScopeSet(t.Scopes)
	exp := time.Now().Add(time.Minute) // re-checked on every request; real expiry when the token has one
	if t.ExpiresAt != nil { exp = *t.ExpiresAt }
	return embeddable.AuthPrincipal{Subject: u.ID, ClientID: t.ID, Scopes: scopes, Expiration: exp,
		Claims: map[string]any{"token": tok}}, nil
}
func (v *tokenVerifier) ProtectedResourceMetadata() map[string]any { return map[string]any{"resource": v.resource} }
func (v *tokenVerifier) WWWAuthenticateHeader() string {
	return embeddable.BearerChallenge(embeddable.ProtectedResourceMetadataURL(v.resource))
}
```

- **The caller's token reaches the API.** `clientFromRequest` builds the client per call from `embeddable.GetAuthPrincipal(ctx)`: the raw token (kept in process memory in `Claims`) becomes the bearer token of a client with `client.WithHandler(s.portal)`. The client's requests go straight into the portal mux without a network hop, so every tool call passes through `guard` with the same level, scope, role and audit handling as an external call.
- **Expiration.** go-sdk's middleware requires one. Tokens without an expiry report one minute ahead, which only records that the token was valid for this request (the same approach go-go-mcp uses for its static development key).
- **Scopes.** A single server serves every caller, so tools cannot be filtered per token at registration. Each tool gets `embeddable.WithToolAuthorization(name, embeddable.ToolAuthorizationPolicy{RequiredScopes: …})`, which go-go-mcp enforces when the tool is called; `guard` enforces them again. Unlike the local mode, the tool list is not filtered.
- **Later: OAuth for claude.ai connectors.** A remote MCP client that cannot be handed a token needs OAuth 2.1. With go-go-mcp v0.3.0 this becomes configuration more than code: `external_oidc` with ZITADEL as issuer and `TokenValidation: auto` (introspection for ZITADEL's opaque tokens, with francois's resource-server credentials), then mapping `(iss, sub)` to a francois user with `Store.UserByIdentity` and OAuth scopes to francois scopes. Findings to carry into that ticket (2026-10-15):
  - Hosted connectors register themselves with the authorization server at runtime (Dynamic Client Registration, RFC 7591). ZITADEL documents RFC 7591 support, disabled by default ([ZITADEL DCR guide](https://zitadel.com/docs/guides/integrate/dynamic-client-registration)); one report says the implementation merged in late July 2026 and may not be in a stable release yet. Check the deployed ZITADEL version first.
  - Clients created through ZITADEL's registration receive opaque access tokens by default ([zitadel#12658](https://github.com/zitadel/zitadel/issues/12658)); ZITADEL documents introspection responses with `token_type: Bearer`, which go-go-mcp v0.3.0 requires.
  - Some 2026 sources say the MCP specification revision of 2026-07-28 prefers Client ID Metadata Documents over Dynamic Client Registration; this was not verified against the specification.

#### Decision: local stdio first, remote second

- **Context:** The owner wants agents to operate the shop. The local mode needs only token scopes and the client; the remote mode also needs a new public endpoint and its hardening.
- **Options considered:** remote only; local only; local then remote.
- **Decision:** local first (phase 5), remote second (phase 6).
- **Rationale:** Local mode delivers the use case for the owner and Ramon immediately, from Claude Code, without adding an internet-facing endpoint. The remote mode reuses all tools unchanged.
- **Consequences:** Until phase 6, agents run where the token is. Remote mode is opt-in with `--mcp`.
- **Status:** proposed

## 7. API reference: client methods mapped to routes

This table is the implementation checklist for `pkg/catalog/client`. "New" marks routes this ticket adds.

### 7.1 Identity and tokens

| Client method | Route | Scope |
| --- | --- | --- |
| `Me` | `GET /api/me` (new) | any |
| — (browser only) | `GET/POST /api/tokens`, `DELETE /api/tokens/{id}` (new location) | session |
| `Admin.UserTokens` | `GET /api/admin/users/{id}/tokens` (new) | admin:read |
| `Admin.RevokeToken` | `DELETE /api/admin/tokens/{id}` (new) | admin:write |

### 7.2 Studio

| Client method | Route |
| --- | --- |
| `Studio.Artifacts`, `Artifact`, `CreateArtifact`, `PatchArtifact` | `GET/POST /api/studio/artifacts`, `GET/PATCH /api/studio/artifacts/{id}` |
| `Studio.SaveDraft`, `PublishDraft`, `DiscardDraft` | `PUT/DELETE /api/studio/artifacts/{id}/draft`, `POST …/draft/publish` |
| `Studio.Releases`, `CheckRelease`, `CreateRelease` | `GET /api/studio/releases?artifact=`, `POST /api/studio/releases/check`, `POST /api/studio/releases` (multipart) |
| `Studio.PublishRelease`, `WithdrawRelease`, `PreviewRelease`, `DeleteRelease` | `POST …/releases/{id}/publish`, `/withdraw` `{suspendPinned}`, `/preview`, `DELETE …/releases/{id}` |
| `Studio.AddAttachment`, `PatchAttachment`, `DeleteAttachment`, `DownloadAttachment` | `POST …/releases/{id}/attachments` (multipart `file`, `label`), `PATCH/DELETE …/attachments/{id}`, `GET …/attachments/{id}/download` |
| `Studio.Media`, `UploadMedia`, `PatchMedia`, `DeleteMedia` | `GET/POST /api/studio/media` (multipart `file`, `alt`), `PATCH/DELETE …/media/{id}` |
| `Studio.Posts`, `Post`, `CreatePost`, `PatchPost`, `PublishPost`, `UnpublishPost`, `DeletePost` | `/api/studio/posts…` |
| `Studio.Creators`, `CreateCreator`, `PatchCreator`, `DeleteCreator` | `/api/studio/creators…` (create, delete and linking are admin-only) |
| `Studio.Markdown`, `Studio.Plans`, `Studio.Me` | `POST /api/studio/markdown`, `GET /api/studio/plans`, `GET /api/studio/me` |
| `Push` (workflow) | check → create → attachments → publish |

### 7.3 Admin

| Client method | Route |
| --- | --- |
| `Admin.Overview` | `GET /api/admin/overview` |
| `Admin.Users`, `User`, `PatchUser`, `RevokeSessions` | `GET /api/admin/users`, `GET/PATCH …/users/{id}`, `POST …/users/{id}/revoke-sessions` |
| `Admin.GrantAccess`, `RevokeAllGrants`, `RevokeGrant` | `POST/DELETE …/users/{id}/access-grants`, `DELETE /api/admin/access-grants/{id}` (new) |
| `Admin.Subscriptions`, `CancelSubscription`, `ResumeSubscription` | `GET /api/admin/subscriptions`, `POST …/subscriptions/{id}/cancel`, `/resume` (all new) |
| `Admin.Instances`, `Instance`, `SuspendInstance`, `ResumeInstance`, `DeleteInstance` | `/api/admin/instances…` (delete is new) |
| `Admin.Settings`, `SaveSettings(doc, revision)` | `GET/PATCH /api/admin/settings` (ETag / If-Match new) |
| `Admin.Billing`, `Reconcile`, `RetryEvent` | `GET /api/admin/billing`, `POST …/billing/reconcile`, `POST …/billing/events/{id}/retry` |
| `Admin.Usage`, `AdjustCredits` | `GET /api/admin/usage`, `POST /api/admin/usage/adjustments` |
| `Admin.Audit` | `GET /api/admin/audit` (filters new) |

Plan commands are helpers over `Settings` / `SaveSettings`:

```go
func (a Admin) UpdatePlan(ctx context.Context, key string, edit func(*settings.Plan) error) (Revision, error) {
	for attempt := 0; attempt < 3; attempt++ {
		cur, err := a.Settings(ctx)                    // document + revision (from ETag)
		if err != nil { return Revision{}, err }
		doc := cur.Active.Document
		i := slices.IndexFunc(doc.Plans, func(p settings.Plan) bool { return p.Key == key })
		if i < 0 { return Revision{}, &Error{Status: 404, Code: "not_found", Message: "no plan " + key} }
		if err := edit(&doc.Plans[i]); err != nil { return Revision{}, err }
		if err := doc.Validate(); err != nil { return Revision{}, err } // same validation as the server
		rev, err := a.SaveSettings(ctx, doc, cur.Active.Revision)
		if IsCode(err, "settings_conflict") { continue } // someone saved in between: re-read, re-apply
		return rev, err
	}
	return Revision{}, errors.New("settings kept changing; try again")
}
```

### 7.4 Account

| Client method | Route |
| --- | --- |
| `Account.Get` | `GET /api/account` |
| `Account.Checkout`, `Portal`, `Refresh` | `POST /api/billing/checkout` `{plan, interval}`, `/portal`, `/refresh` |
| `Account.Instances`, `Instance`, `CreateInstance`, `RenameInstance`, `DeleteInstance` | `/api/instances…` |
| `Account.AddMember`, `RemoveMember`, `Upgrade` | `POST …/members`, `DELETE …/members/{user}`, `POST …/upgrade` |

## 8. Key flows in pseudocode

### 8.1 Guard with scopes

```go
func (s *Server) guard(level int, sc scope, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p, err := s.resolve(r) // session cookie
		if err != nil { fail(503, "unavailable"); return }
		ctx := r.Context()
		if bearer := r.Header.Get("Authorization"); p == nil && strings.HasPrefix(bearer, "Bearer "+store.TokenPrefix) {
			if sc == noToken { fail(401, "token_not_allowed"); return }
			t, u, err := s.Store.ResolveToken(ctx, strings.TrimPrefix(bearer, "Bearer "))
			if err != nil { fail(401, "token_invalid"); return }
			need := sc.required(r.Method) // studio | account | admin:read | admin:write | any
			if !t.Allows(need) { fail(403, "token_scope", "This token lacks the "+need+" scope"); return }
			p = &Principal{User: u, Token: &t}
			ctx = store.WithAuditToken(ctx, t.ID)
		}
		// unchanged: login_required, account_disabled, creator_required, admin_required,
		// and Origin + CSRF for session mutations
		h(w, r.WithContext(context.WithValue(ctx, principalKey{}, p)))
	}
}

func (t APIToken) Allows(need string) bool {
	if need == "*" { return true }
	for _, s := range t.Scopes {
		if s == need || (s == "admin:write" && need == "admin:read") { return true }
	}
	return false
}
```

### 8.2 Client request with retries

```go
func (c *Client) do(ctx context.Context, method, path string, in, out any) error {
	body, ctype := encodeJSON(in)
	for attempt := 0; ; attempt++ {
		req := newRequest(ctx, method, c.base+path, body, ctype)
		req.Header.Set("Authorization", "Bearer "+c.token)
		if c.host != "" { req.Host = c.host }
		resp, err := c.http.Do(req)
		if retryable(method, resp, err) && attempt < c.retries {
			if resp != nil {
				drainAndClose(resp.Body) // release the connection before retrying
			}
			sleep(ctx, backoff(attempt)) // 250ms, 500ms, 1s
			continue
		}
		if err != nil { return err }
		return decode(resp, out) // ≥300 → *Error; else JSON into out (≤8 MiB)
	}
}

func retryable(method string, resp *http.Response, err error) bool {
	if method != http.MethodGet && method != http.MethodHead { return false }
	return err != nil || resp.StatusCode == 502 || resp.StatusCode == 503 || resp.StatusCode == 504
}
```

### 8.3 In-process transport

```go
type handlerTransport struct{ h http.Handler }

func (t handlerTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	rec := httptest.NewRecorder()   // buffers the response; used only for JSON tool calls
	t.h.ServeHTTP(rec, r)
	return rec.Result(), nil
}
```

The request's `Host` must be the portal host so `Server.Handler` dispatches it to the portal mux; `client.New(site.PortalOrigin(), …)` sets it from the URL.

### 8.4 Settings with If-Match

```go
func (s *Server) adminSaveSettings(w http.ResponseWriter, r *http.Request) {
	base, ok := parseIfMatch(r.Header.Get("If-Match")) // `"42"` → 42
	if !ok { httpx.Fail(w, 428, "revision_required", "Send If-Match with the revision you edited"); return }
	raw, err := io.ReadAll(http.MaxBytesReader(w, r.Body, 64<<10))
	if err != nil { httpx.Fail(w, 400, "invalid_request", "Settings document too large"); return }
	doc, err := settings.Parse(raw) // strict: unknown fields rejected (settings.go:123)
	if err != nil { httpx.Fail(w, 422, "invalid_settings", err.Error()); return }
	rev, err := s.Store.SaveSettings(r.Context(), actor(r), base, doc)
	switch {
	case errors.Is(err, store.ErrConflict):
		httpx.Fail(w, 412, "settings_conflict", "Settings changed since you loaded them")
	case err != nil:
		httpx.Fail(w, 422, "invalid_settings", err.Error())
	default:
		w.Header().Set("ETag", strconv.Quote(strconv.FormatInt(rev.Revision, 10)))
		httpx.OK(w, rev)
	}
}

// store: inside the transaction
//   SELECT pg_advisory_xact_lock(hashtext('platform_settings'))
//   SELECT COALESCE(max(revision), 0) FROM platform_settings  → current
//   if base >= 0 && base != current { return ErrConflict }
//   (revision 0 is checked like any other; only the break-glass store path
//   SaveSettings passes a negative base, and the HTTP API never does)
//   INSERT … RETURNING revision; audit
```

### 8.5 One MCP tool call, end to end (local mode)

```text
model ──tools/call grant_complimentary_access {userId, days, reason, plan}──▶ francois mcp (stdio)
  mcpserver: input decoded and validated against the inferred JSON Schema
  tools.grant → client.Admin.GrantAccess
    POST https://slop.boutique/api/admin/users/usr_…/access-grants   Authorization: Bearer frt_…
      guard(admin, scopeAdmin): token ok, POST needs admin:write ✓, role admin ✓, ctx += token id
      adminGrant: validate plan, Store.GrantAccess, audit(user.access_grant, token_id)
    ◀── 201 {"id":"grt_…","until":"…","plan":"studio",…}
  tool result: text "Granted Studio until 2026-11-14." + structuredContent (the grant)
model ◀────────────────────────────────────────────────────────────────────────────┘
```

## 9. Security review checklist

- Scopes are ceilings: role, disabled, object and CSRF rules are unchanged and run after the scope check.
- Admin GET handlers have no side effects (the method-derived scope relies on it).
- Admin tokens always expire (≤ 90 days); creation is audited with the scopes.
- `POST /api/instances/{id}/launch` and `/api/tokens*` reject tokens.
- Revoking a user's sessions can revoke their tokens. Disabling a user stops their tokens with `account_disabled` on every route at level `enabled` or above (all studio and admin routes and account mutations); the read-only account routes at level `session` stay readable, as they are for the user's browser session today.
- The client never logs tokens, headers or bodies; `--token-file` is preferred over `--token` flags that appear in `ps`.
- MCP local mode restricts path inputs to `--allow-dir` roots after resolving symlinks.
- MCP remote mode is opt-in (`--mcp`), requires a bearer token on every request, and calls the API in-process through `guard`.
- Tool results never include secrets: the token list returns metadata only; `get_settings` already omits secrets (`admin.go:319-333` reports only whether they are configured).
- Destructive tools carry `DestructiveHint: true` so MCP clients ask before running them.
- The settings PATCH cannot lose a concurrent edit (`If-Match`).

## 10. Implementation phases

Each phase ends with `make ci-check` and the Postgres tests (`TEST_DATABASE_URL=… go test ./pkg/catalog/...`) passing, and is one pull request.

1. **Scoped tokens and identity.** Migration 008; `scope` in `guard` and every route; `APIToken.Scopes`, `Allows`; `/api/tokens` (move from studio, scopes, minting rules); `GET /api/me`; admin token list/revoke; `revoke-sessions {tokens}`; audit `token_id` via context; account-page token UI with scope checkboxes. Files: `auth.go`, `admin.go`, `studio.go`, `routes.go`, `instances.go`, `store/tokens.go`, `store/users.go`, `store/audit.go` (new), `store/migrations/008_token_scopes.sql`, `ui/catalog/account.tsx`, `ui/catalog/studio.tsx`, `ui/catalog/api.ts`. Acceptance: the scope matrix test (§11) passes; existing `catalog push` works unchanged with an old token.
2. **Customer-management endpoints.** Per-grant revoke, subscriptions list, cancel/resume (Gateway method, fake gateway), admin instance delete, settings ETag/If-Match (and the admin settings page), audit filters, user filters. Files: `admin.go`, `payments/gateway.go`, `payments/service.go`, `store/users.go`, `store/billing.go`, `store/settings.go`, `store/instances.go`, `ui/catalog/admin.tsx`. Acceptance: handler tests for each endpoint, including 412 on a stale revision.
3. **Client.** `pkg/catalog/client` with every method in §7; `studioclient` removed and `catalog push` ported to the new client (it is renamed to `catalog releases push` in phase 4); contract tests. Acceptance: every route in §7 has a client method and a contract test.
4. **Commands.** The `api` section; every command in §6.6; `catalog db` move; confirmation prompts; help pages (`pkg/doc` embedded Markdown loaded in `cmd/francois/main.go`). Acceptance: `executeJSON` tests for representative commands of each group; `README.md` and `doc/production.md` updated.
5. **MCP stdio.** Add `github.com/go-go-golems/go-go-mcp v0.3.0` (bumps Glazed to `v1.4.4-0.20260823170018-a7fdfda5e3c6` and the `go` directive to 1.26.3; run `make ci-check` and the Postgres tests after the bump on its own). Then `pkg/catalog/mcpserver` with the typed-tool adapter, `francois mcp`, toolsets, read-only mode, allow-dirs; a playbook for adding it to Claude Code. Acceptance: tests list tools per scope set and call representative tools against the test server.
6. **MCP remote.** `catalog serve --mcp`, `tokenVerifier` through `embeddable.WithHTTPAuthVerifier`, `embeddable.MountHTTPHandlers`, per-tool `WithToolAuthorization`, the in-process client transport; Traefik and k3s changes if `/mcp` needs any. Acceptance: Streamable HTTP test against `httptest`; manual test from Claude Code with `claude mcp add --transport http`.

## 11. Test strategy

- **Scope matrix (phase 1).** A table test in `pkg/catalog/tokens_test.go`: for each token scope set (`studio`, `account`, `admin:read`, `admin:write`, `studio+admin:write`) and each owner role, call one route of each surface with each relevant method and assert the status and code (`token_scope`, `token_not_allowed`, `admin_required`, success). Include: launch and `/api/tokens` reject every token; a demoted admin's token fails; an expired token fails; a revoked token fails; an old token migrated to `{studio}` behaves as before.
- **Audit attribution.** A mutation through a token writes `token_id`; the same mutation through a session writes `NULL`.
- **Handlers (phase 2).** One test per new endpoint, using `setup(t)` and the fake Stripe gateway pattern from `payments/service_test.go`. Settings: two clients read revision N, one saves, the other gets 412.
- **Client contract tests (phase 3).** In `package catalog` (to reuse `setup`), build `client.New(e.ts.URL, token, client.WithHost("portal.test"))` and call every method. A test-only decoder hook makes the client decode with `DisallowUnknownFields`, so a server field missing from the client type fails the test.
- **Retries.** An `httptest` server returning 503 twice then 200 for a GET (succeeds after three attempts) and 503 for a POST (fails immediately).
- **Commands (phase 4).** `executeJSON` (`cmd/francois/cmds/update_test.go:17-46`) for the output shape of representative commands; the server side is already covered by the client tests.
- **MCP (phase 5).** Build the server configuration with `mcpserver.Options`, mount it with `embeddable.MountHTTPHandlers` under `httptest`, and connect a go-sdk client (go-go-mcp runs on go-sdk v1.7.0); stdio itself needs no separate test. Assert the tool list for each scope set and for `--read-only`; call `whoami`, `find_customers`, `grant_complimentary_access` and `push_release` against the Postgres test server; assert that a path outside `--allow-dir` is refused, including through a symlink.
- **Remote MCP (phase 6).** Streamable HTTP against `httptest`: no token → 401 with `WWW-Authenticate`; a `studio` token calling a customer tool → refused by the tool policy (and by `guard`); a tool call is audited with the token ID.

## 12. Risks, alternatives and open questions

- **Tool count.** About 70 tools when every scope is present. Large tool lists cost model context and can confuse tool choice. Mitigations: toolsets, `--read-only`, and narrow tokens. If it remains a problem, group operations into fewer tools with an `action` argument; that trades schema precision for count.
- **Contract drift.** Hand-written types can lag the server; the `DisallowUnknownFields` contract tests are the guard. An OpenAPI document would be the long-term alternative.
- **Admin-token blast radius.** An `admin:write` token can disable users and cancel renewals for 90 days. The owner should keep such tokens in Vault, use `admin:read` for agents that only answer questions, and review `token_id` in the audit log.
- **Resolved (2026-10-16): go-go-mcp versus go-sdk.** go-go-mcp v0.3.0, after GO-GO-MCP-EMBEDDABLE-AUTH removed the embedded OIDC server from `pkg/embeddable` and added introspection (§6.7).
- **Resolved (2026-10-15): `catalog push` location.** It moves to `catalog releases push`.
- **Resolved (2026-10-15): subscriber tokens.** Yes: every non-guest user may mint `account` tokens.
- **Open question: remote MCP and OAuth.** Claude Code and similar tools can call a remote `/mcp` endpoint with a personal token in an `Authorization` header (phase 6). Connectors added in the claude.ai web, desktop or mobile apps are called from Anthropic's servers and sign in through OAuth rather than taking a pasted token; supporting them needs a ZITADEL-backed authorization phase after phase 6. Whether that is a goal is the owner's call.

## 13. References

**Server**

- `pkg/catalog/auth.go:18-131` — `Principal`, guard levels, `guard`, token handling, CSRF.
- `pkg/catalog/admin.go:19-460` — admin routes and handlers.
- `pkg/catalog/studio.go:35-74` — studio routes; `:1200-1259` token endpoints.
- `pkg/catalog/routes.go:21-102` — portal mux; `:129-150` `/api/session`; `:343-469` billing routes and webhook.
- `pkg/catalog/instances.go:20-307` — account instance routes; `:184` session-bound launch.
- `pkg/catalog/entitlement.go:42-70` — entitlement resolution.
- `pkg/catalog/httpx/httpx.go:19-45` — JSON envelope.
- `pkg/catalog/server.go:205-240` — host dispatch.

**Store and data**

- `pkg/catalog/store/tokens.go:13-73` — tokens.
- `pkg/catalog/store/users.go:41-184` — users, audit, grants.
- `pkg/catalog/store/settings.go:13-63` — settings revisions.
- `pkg/catalog/store/billing.go` — subscriptions, checkouts, Stripe inbox.
- `pkg/catalog/store/migrations/001_core.sql:50-66` — `admin_audit`, `platform_settings`; `004_storefront.sql:80-89` — `api_tokens`.
- `pkg/catalog/settings/settings.go:46-98` — plans and the settings document.
- `pkg/catalog/payments/gateway.go:40-53`, `payments/service.go:72-355` — Stripe gateway and billing service.

**Client, CLI and tests**

- `pkg/catalog/studioclient/client.go` — the client being replaced.
- `cmd/francois/main.go:18-34`, `cmd/francois/cmds/catalog.go:460-538`, `cmd/francois/cmds/push.go` — CLI wiring and the push command.
- `pkg/catalog/catalog_test.go:34-196`, `pkg/catalog/tokens_test.go:15-110`, `cmd/francois/cmds/update_test.go:17-46` — test harnesses.
- `ui/catalog/api.ts`, `ui/catalog/studio-api.ts` — the SPA's typed view of the API.
- `Makefile:22-67`, `.golangci.yml`, `logcopter_generate.go` — validation and code generation.

**MCP**

- `~/go/pkg/mod/github.com/modelcontextprotocol/go-sdk@v1.7.0/mcp/server.go:561` (`AddTool`), `mcp/streamable.go:232`, `mcp/transport.go:114,160`, `auth/auth.go:21-97`.
- `/home/manuel/code/wesen/go-go-golems/go-go-mcp/pkg/embeddable/` — the workspace's MCP library; `pkg/doc/topics/08-oidc-keycloak-coolify-hosted-mcp.md` for hosted OIDC.
- `/home/manuel/code/wesen/go-go-golems/smailnail/pkg/mcp/imapjs/server.go:49-73`, `cmd/smailnaild/commands/serve.go:157-174` — a hosted MCP server mounted in an application server.

**Earlier tickets**

- FRANCOIS-004 design doc §5.7 (studio API and the token boundary), §5.10 (tokens and `catalog push`).
