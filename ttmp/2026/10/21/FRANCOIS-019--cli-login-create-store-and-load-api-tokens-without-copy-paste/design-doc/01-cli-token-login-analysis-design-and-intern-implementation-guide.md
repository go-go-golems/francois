---
Title: CLI token login analysis design and intern implementation guide
Ticket: FRANCOIS-019
Status: active
Topics:
    - cli
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/api.go
      Note: connect resolves server and token; will use credentials.Resolve
    - Path: repo://cmd/francois/cmds/push.go
      Note: Duplicated token loading to replace with connect
    - Path: repo://pkg/catalog/auth.go
      Note: 'guard: token scopes, noToken, Origin and CSRF for session mutations'
    - Path: repo://pkg/catalog/store/email_links.go
      Note: Ready plus claim-by-delete pattern the device table copies
    - Path: repo://pkg/catalog/store/tokens.go
      Note: Token format, ScopesForRole, CreateToken to split into insertToken
    - Path: repo://pkg/catalog/tokens.go
      Note: Token routes and createToken validation to extract into tokenGrant
    - Path: repo://ui/catalog/tokens.tsx
      Note: Account-page token UI and scope descriptions reused by the approval page
ExternalSources: []
Summary: Add a browser-approved device login (francois catalog auth login) that mints a scoped personal API token from the user's portal session, and a local credentials file the CLI, push and MCP commands load automatically.
LastUpdated: 2026-10-21T10:26:53.330652-04:00
WhatFor: Understanding how francois authenticates CLI callers today and implementing device login plus credential storage.
WhenToUse: Before changing token creation, the guard, the catalog CLI connection code, or the account token UI.
---

# CLI token login: analysis, design and implementation guide

## 1. Executive summary

Today a person who wants to use the `francois catalog …` commands must open the
website, sign in, go to the account page, create a personal API token, copy the
secret out of the browser, and then either export it as `FRANCOIS_TOKEN` or save
it to a file they name with `--token-file` on every command. The CLI has no
`login` command, no place where it remembers a token, and no way to ask the
server for one. The server deliberately refuses to let any API token create
tokens.

This ticket adds two things that together remove the copy-and-paste step:

1. **A device login flow.** `francois catalog auth login --server https://slop.boutique`
   asks the server for a short *user code* (for example `WDJB-MJHT`), prints a
   link, and waits. The person opens the link in any browser where they are
   signed in, sees what is asking for access, picks scopes, and clicks
   **Approve**. The waiting CLI then receives a normal scoped personal API token
   (`frt_…`). The token is minted from the person's browser session, so the
   existing rule "a token cannot create tokens" still holds. This is the same
   shape as the OAuth 2.0 Device Authorization Grant (RFC 8628) used by
   `gh auth login`, the AWS CLI and smart TVs.
2. **A local credentials store.** The CLI writes the token, keyed by server
   origin, to `~/.config/francois/credentials.json` (mode `0600`). Every API
   command, `releases push` and `francois mcp` then find it automatically.
   `FRANCOIS_TOKEN` and `--token-file` keep working and take precedence, so
   scripts and CI do not change.

The work is split into phases so that the storage half (useful on its own, with
`auth login --with-token`) can ship before the server half. No existing route
changes behavior, the existing token table is reused unchanged, and one new
table holds short-lived pending device authorizations.

**Implementation status (2026-10-21):** implemented on branch `feat/cli-auth`
(commits 3c36520 … ed4c200) as designed, with the owner decisions of §11. Small
differences from the sketch: the store type is `DevicePoll` (not `PollResult`);
`GET /api/device/requests/{code}` also returns `suggestedScopes` and
`suggestedDays` (the role-maximum default of decision 3); `auth login --days`
is optional and the server defaults to 90. See the diary for evidence.

## 2. Problem statement and scope

### 2.1 What the user asked for

> "Does francois allow creating a device token from the CLI, like ask me to
> login and then give me a token?" — and then: create a ticket "to be able to
> easily create a token (and store / load it, dunno how that's handled right
> now)".

The answer to the first question is no (evidence in §4). This document designs
the feature and explains the current storage and loading behavior.

### 2.2 In scope

- A server-side device authorization flow on the hosted catalog
  (`francois catalog serve`).
- A browser approval page in the existing portal single-page application.
- CLI commands to log in (device flow, or by pasting an existing token), show
  login status, print the stored token, and log out.
- A credentials file and one shared resolution function used by every command
  that talks to the hosted API.
- A narrow "revoke the token I am using" endpoint so `logout` can clean up
  server-side (see the decision record in §7.5; this needs the owner's sign-off).
- Tests and documentation updates.

### 2.3 Out of scope

- OAuth 2.1 for third-party remote MCP clients (claude.ai connectors). That is
  the separate ZITADEL-backed plan recorded in FRANCOIS-007 §6.8. This ticket
  does not issue OAuth access tokens; it issues the existing personal API tokens.
- Operating-system keychain storage. It is designed as a later, optional backend
  (§7.4), not built in this ticket.
- Changing what scopes exist or what a scope may reach.
- The local single-user `francois serve` library server, which has no accounts.

## 3. System orientation for a new reader

This section explains the parts of francois you need before the design makes
sense. Skip it if you already know the hosted catalog.

### 3.1 What the hosted catalog is

francois has two products in one binary. The older one, `francois serve`, is a
local library that hosts saved HTML artifacts for one person. The newer one,
`francois catalog serve`, is a multi-user web service (production:
`https://slop.boutique`) where creators publish HTML applications ("artifacts")
and subscribers run them. This ticket is only about the hosted catalog.

The hosted catalog consists of:

- **One Go HTTP server** (`pkg/catalog`), started by `francois catalog serve`.
  It serves a server-rendered storefront (`/`, `/apps`, `/pricing`, …), a React
  single-page application (SPA) for signed-in pages (`/account`, `/studio`,
  `/admin`), and a JSON API under `/api/…`.
- **A PostgreSQL database** accessed only through `pkg/catalog/store`. Schema
  changes are numbered SQL files in `pkg/catalog/store/migrations/`, applied in
  name order.
- **An identity provider** (ZITADEL, via OpenID Connect) plus an email-link
  sign-in path. Either way, a successful sign-in ends with the server creating a
  *portal session* and setting a cookie.
- **A CLI** (`cmd/francois/cmds`). The `francois catalog <group> <verb>`
  commands are thin wrappers around a typed HTTP client (`pkg/catalog/client`).
  They never touch the database; they call the same API the browser calls.
- **An MCP server** (`francois mcp`, and optionally `/mcp` on the server) that
  exposes the same API as tools for coding agents. It also authenticates with a
  personal API token.

Production runs **one replica** (`doc/production.md:92`: "One replica,
`Recreate` strategy"). That matters later: in-memory rate limiters are
acceptable.

### 3.2 Terms used in this document

- **Portal origin.** The scheme, host and port of a site's portal, for example
  `https://slop.boutique`. The server can serve several sites from one process;
  each request is attributed to a site by its `Host` header, and
  `s.site(r).PortalOrigin()` returns that site's origin
  (`pkg/catalog/server.go:211`). Sessions are per origin.
- **Session.** A browser login. The browser holds a random cookie value; the
  database stores only its SHA-256 hash, a CSRF token, and idle/absolute expiry
  (7 and 30 days, `pkg/catalog/store/sessions.go:45-48`).
- **Personal API token.** A long-lived bearer credential for scripts and agents.
  Its secret looks like `frt_` followed by 43 base64url characters (32 random
  bytes). Only the hash is stored (`pkg/catalog/store/tokens.go:71-80`). A token
  belongs to one user, has a name, a list of scopes, an optional expiry, and can
  be revoked.
- **Scope.** A ceiling on which API surface a token may call: `studio`,
  `account`, `admin:read`, `admin:write`. A user may only put scopes on a token
  that their role allows (`store.ScopesForRole`, `tokens.go:26-37`).
- **Principal.** The authenticated caller of one request: either a session or a
  token, plus the user (`pkg/catalog/auth.go:18-22`).
- **Guard.** The wrapper every route is registered with. It takes an *access
  level* (`public`, `session`, `enabled`, `creator`, `admin`) and a *token
  scope* (`noToken`, `anyToken`, `scopeStudio`, …) and enforces authentication,
  role, account state, token scope, and Origin + CSRF for browser mutations
  (`auth.go:102-162`).
- **Device authorization grant.** A login pattern for devices that cannot easily
  show a browser (a TV, a CLI over SSH). The device asks the server for a
  *device code* (a secret it keeps) and a *user code* (short, human-typable).
  The human enters the user code on a *verification URI* in a browser where they
  are signed in and approves. Meanwhile the device *polls* the server with the
  device code until the server answers with a credential, a denial, or an
  expiry. Standardized as RFC 8628.
- **Credentials file.** The new local JSON file in which the CLI remembers
  tokens per portal origin.

### 3.3 The end-to-end picture

```text
  Terminal (CLI)                 francois catalog serve             Browser
  ---------------                ----------------------             -------
  auth login --server S
     |  POST /api/device/authorize  (no credential)
     |---------------------------------->|
     |  deviceCode, userCode=WDJB-MJHT,  |  INSERT device_authorizations
     |  verificationUri, interval=5s     |  (hashes only, expires 10 min)
     |<----------------------------------|
     |  prints: open S/device?code=WDJB-MJHT
     |                                   |   GET /device?code=WDJB-MJHT
     |                                   |<-------------------------------|
     |                                   |   (signs in first if needed)   |
     |                                   |   GET /api/device/requests/..  |
     |                                   |<-------------------------------|
     |                                   |   "francois CLI on ada-laptop  |
     |                                   |    wants: studio, 90 days"     |
     |  POST /api/device/token (poll)    |                                |
     |---------------------------------->|                                |
     |  400 authorization_pending        |   POST .../approve             |
     |<----------------------------------|   (session cookie + CSRF)      |
     |                                   |<-------------------------------|
     |  POST /api/device/token (poll)    |   status=approved, user=ada    |
     |---------------------------------->|                                |
     |                                   |  tx: insert api_tokens row,    |
     |                                   |      delete device row         |
     |  200 { token: frt_..., info }     |                                |
     |<----------------------------------|                                |
     |  writes ~/.config/francois/credentials.json (0600)
     |  GET /api/me with the new token -> "Logged in as Ada (creator)"
```

The important property is the arrow labelled "session cookie + CSRF": the token
is created because a signed-in human approved it in a browser session, exactly
like clicking **Create token** on the account page today. The CLI never sees the
session and the browser never sees the token.

## 4. Current state (with evidence)

### 4.1 How tokens are created today

Token management routes live in `pkg/catalog/tokens.go:16-23`:

```go
m.HandleFunc("GET /api/me", s.guard(session, anyToken, s.apiMe))
// Tokens are managed from a browser session only: a token cannot mint or
// revoke tokens.
m.HandleFunc("GET /api/tokens", s.guard(session, noToken, s.tokens))
m.HandleFunc("POST /api/tokens", s.guard(enabled, noToken, s.createToken))
m.HandleFunc("DELETE /api/tokens/{id}", s.guard(session, noToken, s.revokeToken))
```

`noToken` means a request carrying `Authorization: Bearer frt_…` is rejected
with `401 token_not_allowed` before the handler runs (`auth.go:114-118`). The
test `pkg/catalog/tokens_test.go:54-56` asserts exactly that. FRANCOIS-007
recorded this as a deliberate decision: "There is no command for managing one's
own tokens: `/api/tokens` stays session-only, so a token can never mint or
revoke tokens" (FRANCOIS-007 design doc, line 565).

`createToken` (`tokens.go:56-103`) validates the request and is the logic the
device flow must reuse:

- `name` is required, at most 80 characters; `days` is 0–366, 0 meaning no
  expiry (`tokens.go:62-64`).
- Scopes must be a non-empty subset of `store.ScopesForRole(role)`; duplicates
  are dropped (`tokens.go:66-86`).
- If any scope is `admin:read` or `admin:write`, `days` must be 1–90
  (`maxAdminTokenDays`, `tokens.go:14`, `87-90`).
- It calls `Store.CreateToken`, writes an audit entry `token.create`, and
  returns `201 {"token": "<secret>", "info": {…}}` (`tokens.go:96-102`).

The role-to-scope table (`store/tokens.go:26-37`):

| Role | Scopes the user may put on a token |
| --- | --- |
| `admin` | `studio`, `account`, `admin:read`, `admin:write` |
| `creator` | `studio`, `account` |
| `subscriber` | `account` |
| `guest` and others | none (`403 tokens_unavailable`) |

`Store.CreateToken` (`store/tokens.go:71-80`) generates 32 random bytes,
prefixes `frt_`, and inserts `sha256(secret)` into `api_tokens` using the pool
directly. It cannot currently run inside a caller's transaction; the device
flow needs that (§8.3). The session code already shows the pattern to copy:
`insertSession` takes a small `Exec` interface so that the email-link flow can
create a session inside its own transaction (`store/sessions.go:55-67`, used at
`store/email_links.go:132`).

The table was created in `store/migrations/004_storefront.sql:80-89` and gained
`scopes text[]` in `008_token_scopes.sql`. Columns: `id`, `user_id`, `name`,
`token_hash` (unique), `scopes`, `created_at`, `last_used_at`, `expires_at`,
`revoked_at`.

### 4.2 How tokens are checked on each request

`guard` (`auth.go:102-162`) runs on every route:

1. Resolve a session cookie, if any (`auth.go:104`, `resolve` at `47-60`).
2. If there is no session and the `Authorization` header starts with
   `Bearer frt_`, then: reject if the route is `noToken`; otherwise look the
   token up with `Store.ResolveToken` (which also bumps `last_used_at` and
   ignores revoked or expired tokens, `store/tokens.go:112-120`); reject with
   `403 token_scope` if it lacks the route's scope; build a token principal and
   tag the context so audit entries record the token ID (`auth.go:114-130`).
3. Enforce the access level (`login_required`, `account_disabled`, role).
4. For mutations by a *session* (not a token), require a same-origin `Origin`
   header and a matching `X-CSRF-Token` (`auth.go:147-159`). Bearer tokens skip
   this because they are not ambient browser credentials.

Two details matter for the new endpoints:

- A header such as `Authorization: Bearer ` (empty) or any value not starting
  with `frt_` is simply ignored by the guard. Public device endpoints therefore
  need no special casing.
- Browser mutations on `session`-level routes automatically get Origin and CSRF
  protection. The approve and deny endpoints get that for free by being
  `session`/`enabled` routes.

### 4.3 How the browser creates a token (the UI we are replacing for CLI users)

The account page renders `<ApiTokens role=… />` (`ui/catalog/account.tsx:208`),
implemented in `ui/catalog/tokens.tsx:31-118`. It shows a name field, a days
field, and a checkbox per allowed scope (`scopesFor(role)`, which mirrors the
server table at `tokens.tsx:14-25`). After creation it shows the secret once
with the text "Copy this token now; it is not shown again…" (`tokens.tsx:77`)
and a sample `export FRANCOIS_TOKEN=…` line. The studio's old **Tokens** tab now
only points to the account page (`ui/catalog/studio.tsx:1773-1790`). The RTK
Query hooks live in `ui/catalog/api.ts:640-653`; every SPA mutation sends the
session's CSRF token in `X-CSRF-Token` (`api.ts:355-363`).

### 4.4 How the CLI loads a token today

There is no stored login. All hosted API commands share three flags
(`cmd/francois/cmds/api.go:29-35`): `--server`, `--token-file`, `--host`. The
`connect` function (`api.go:135-159`) resolves them:

```go
server := a.S("server")
if server == "" { server = os.Getenv("FRANCOIS_SERVER") }
if server == "" { return nil, errors.New("set --server or FRANCOIS_SERVER …") }
token := os.Getenv("FRANCOIS_TOKEN")
if f := a.S("token-file"); f != "" {
    raw, err := os.ReadFile(f)   // trimmed
    token = strings.TrimSpace(string(raw))
}
if token == "" {
    return nil, errors.New("an API token is required (--token-file or " +
        "FRANCOIS_TOKEN); create one on the account page")
}
return client.New(server, token, opts...), nil
```

So today the precedence is `--token-file` > `FRANCOIS_TOKEN`, and
`--server` > `FRANCOIS_SERVER`. Nothing is written to disk by the CLI.

Two other commands load tokens:

- `francois mcp` calls the same `connect` and then `GET /api/me` to learn the
  token's scopes before registering tools (`cmd/francois/cmds/mcp.go:25-38`).
- `francois catalog releases push` has its **own copy** of the loading logic
  (`cmd/francois/cmds/push.go:69-79`), makes `--server` a required flag
  (`push.go:32`) so `FRANCOIS_SERVER` is ignored, and its help text still says
  "Create a token in the studio (Studio → Tokens)" (`push.go:30`), which is
  stale since tokens moved to `/account`.

`whoami` (`cmd/francois/cmds/api_commands.go:31-34`) prints `GET /api/me`, which
returns the user, the token metadata and the creator profile
(`pkg/catalog/tokens.go:27-42`; client type `pkg/catalog/client/types.go:16-40`).

The HTTP client always sends `Authorization: Bearer <token>`
(`pkg/catalog/client/client.go:112`), even if the token is empty. For the new
unauthenticated device calls the client should omit the header when the token
is empty (harmless today, but cleaner and easier to test).

The only on-disk convention in the docs is a suggestion, not code:
`doc/production.md:423` shows `--token-file ~/.config/francois/token`.

### 4.5 Existing building blocks we can reuse

| Need | Existing code to copy or reuse |
| --- | --- |
| Short-lived, hashed, single-use flow rows | `login_flows` (`store/sessions.go:17-34`, `001_core.sql:17-25`) |
| "Ready" flag + claim by deleting the row | `email_links` (`store/email_links.go:67-90`, `014_email_links.sql`) |
| Insert a credential inside a transaction | `insertSession` (`store/sessions.go:55-67`) |
| Random secrets and hashing | `store.RandomHex`, `store.Hash` (SHA-256), `store.NewID` (`store/store.go:136-152`) |
| Per-key sliding-window rate limiting | `limiter` (`pkg/catalog/demos.go:27-50`), used by `reserveEmail` (`email_signin.go:92-123`) |
| Client IP behind the proxy | `s.clientAddress(r)` (`demos.go:60-74`) |
| Safe post-login return path | `safeReturn` (`auth.go:174-184`), `loginHref` (`ui/catalog/ui.tsx:44-45`) |
| SPA route registration | `routes.go:28` (server) and `ui/catalog/main.tsx:11-38` (client) |
| Uniform JSON errors | `httpx.Fail` → `{"error":{"code","message"}}` (`pkg/catalog/httpx/httpx.go:28-31`); client side `*client.Error` with `Code` (`pkg/catalog/client/errors.go:19-26`) |
| Integration test harness with Postgres | `setup(t)`, `e.user`, `e.portal`, `e.do` (`pkg/catalog/catalog_test.go:80-193`), needs `TEST_DATABASE_URL` (`catalog_test.go:33-38`) |
| CLI test with a fake API | `fakeAPI` and `executeJSON` (`cmd/francois/cmds/api_test.go:15-100`) |

## 5. Gap analysis

| # | Gap | Consequence today | Addressed by |
| --- | --- | --- | --- |
| G1 | No way to obtain a token from the CLI | Manual copy from the browser for every machine | Device flow (§6.2), phases 2–4 |
| G2 | No place the CLI remembers a token | Users export env vars in shell profiles or manage files by hand | Credentials file (§6.4), phase 1 |
| G3 | `releases push` duplicates and diverges from `connect` | Ignores `FRANCOIS_SERVER`; any new loading logic would have to be added twice | Use `connect` everywhere, phase 0 |
| G4 | No logout | A forgotten laptop token stays valid until revoked on the web | `auth logout` + self-revoke (§6.3, §7.5), phase 5 |
| G5 | Server cannot mint a token outside a request handler's own transaction | Device claim cannot atomically "create token + consume request" | `insertToken` with a querier, phase 0 |
| G6 | Token request validation lives inline in `createToken` | Device approval would duplicate the scope and expiry rules | Extract `tokenGrant` (§8.1), phase 0 |
| G7 | Stale help text in `push` | Points users to a tab that no longer creates tokens | Docs, phase 6 |

## 6. Proposed architecture and APIs

### 6.1 Components and responsibilities

```text
+----------------------------- cmd/francois/cmds ------------------------------+
|  auth.go (new)          api.go: connect()          push.go       mcp.go       |
|  login/logout/status/   resolves server+token      uses connect  uses connect |
|  token commands         via credentials.Resolve                               |
+------------+--------------------------+---------------------------------------+
             |                          |
             v                          v
+--------------------------+   +--------------------------------------------+
| pkg/catalog/credentials  |   | pkg/catalog/client                         |
| (new, no network)        |   |  client.go  (omit empty Authorization)     |
|  Load/Save/Remove        |   |  device.go  (new: Authorize, PollToken)    |
|  Resolve(precedence)     |   |  studio.go  Me(); new RevokeSelf()         |
|  file: credentials.json  |   +--------------------+-----------------------+
+--------------------------+                        | HTTPS
                                                    v
+----------------------------------- pkg/catalog --------------------------------+
| device.go (new): deviceRoutes, authorize, poll, request, approve, deny          |
| tokens.go: tokenGrant() extracted; revokeSelf (new)                             |
| routes.go: "GET /device" added to SPA paths; s.deviceRoutes(m)                  |
| server.go: deviceLimiter                                                        |
+-------------------------------------+------------------------------------------+
                                      v
+------------------------------- pkg/catalog/store ------------------------------+
| device.go (new): CreateDeviceAuthorization, DeviceRequest, ApproveDevice,       |
|                  DenyDevice, PollDevice                                         |
| tokens.go: insertToken(querier, …) used by CreateToken and PollDevice           |
| migrations/019_device_authorizations.sql (new)                                  |
+---------------------------------------------------------------------------------+

+-------------------------------- ui/catalog (SPA) ------------------------------+
| device.tsx (new): /device approval page     main.tsx: route "device"            |
| api.ts: deviceRequest query, approveDevice / denyDevice mutations               |
| tokens.tsx: export scopeInfo for reuse                                          |
+---------------------------------------------------------------------------------+
```

### 6.2 HTTP API: the device flow

All bodies are JSON. Errors use the existing `{"error":{"code","message"}}`
shape so the Go client's `*client.Error` and `client.IsCode` work unchanged.
The paths follow francois's style (`/api/...`, camelCase JSON) rather than
RFC 8628's form-encoded `/device_authorization`; the semantics and error codes
follow the RFC so the flow is recognizable.

#### `POST /api/device/authorize` — start a login (CLI, unauthenticated)

Guard: `public`, `noToken`. Rate limited per client address and globally.

Request:

```json
{ "clientName": "francois CLI on ada-laptop",
  "scopes": ["studio", "account"],
  "days": 90 }
```

- `clientName`: 1–80 characters; becomes the token name. Required.
- `scopes`: what the CLI *suggests*. Unknown scope names are rejected
  (`400 invalid_scopes`). Role is unknown at this point, so a scope the approver
  cannot hold is not an error here; the approval page simply does not offer it.
  May be empty (the approver chooses).
- `days`: 0–366, default 90. Final validation happens at approval.

Response `201`:

```json
{ "deviceCode": "fdc_8bQ0mH…43 chars",
  "userCode": "WDJB-MJHT",
  "verificationUri": "https://slop.boutique/device",
  "verificationUriComplete": "https://slop.boutique/device?code=WDJB-MJHT",
  "expiresIn": 600,
  "interval": 5 }
```

`verificationUri` is built from `s.site(r).PortalOrigin()`, so a second site
gets its own origin. The row records that origin; approval and polling must
happen on the same site.

#### `POST /api/device/token` — poll (CLI, unauthenticated)

Guard: `public`, `noToken`. The device code is the credential.

Request: `{ "deviceCode": "fdc_…" }`

Responses:

| Status | `error.code` | Meaning | CLI action |
| --- | --- | --- | --- |
| 400 | `authorization_pending` | Nobody has approved yet | Wait `interval`, poll again |
| 400 | `slow_down` | Polled faster than `interval` | Add 5 s to the interval, then poll |
| 400 | `access_denied` | The user clicked Deny, or the approver lost the role/was disabled | Stop, report |
| 400 | `expired_token` | Unknown, expired or already used device code | Stop, suggest re-running login |
| 429 | `rate_limited` | Address-level limit | Stop, report |
| 200 | — | Approved: token minted now | Save, verify with `/api/me` |

Success body (same `info` shape as `POST /api/tokens`, plus the user so the CLI
can print who it is logged in as without a second call):

```json
{ "token": "frt_…",
  "info": { "id": "tok_…", "name": "francois CLI on ada-laptop",
            "scopes": ["studio"], "createdAt": "…", "expiresAt": "…" },
  "user": { "id": "usr_…", "name": "Ada", "email": "ada@example.com",
            "role": "creator", "disabled": false } }
```

The secret exists only in this response. The database keeps only its hash, as
for every other token.

#### `GET /api/device/requests/{userCode}` — what is asking? (browser)

Guard: `session`, `noToken`. Only pending, unexpired requests created on the
same portal origin are returned; everything else is `404 not_found` (do not
reveal whether a code ever existed). `{userCode}` is normalized (§8.2).

Response `200`:

```json
{ "userCode": "WDJB-MJHT",
  "clientName": "francois CLI on ada-laptop",
  "requestedScopes": ["studio", "account"],
  "requestedDays": 90,
  "requestedFrom": "203.0.113.7",
  "createdAt": "2026-10-21T14:02:11Z",
  "expiresAt": "2026-10-21T14:12:11Z",
  "allowedScopes": ["studio", "account"],
  "maxAdminDays": 90 }
```

`allowedScopes` is `store.ScopesForRole(me.Role)` so the page does not need its
own copy of the table (it currently has one in `tokens.tsx:14-25`).

#### `POST /api/device/requests/{userCode}/approve` (browser)

Guard: `enabled`, `noToken` — therefore Origin + CSRF are enforced by `guard`.

Request: `{ "scopes": ["studio"], "days": 90 }` — the approver's final choice,
validated with exactly the same rules as `POST /api/tokens` (`tokenGrant`,
§8.1). Response `200 {"approved": true}`. Audit: `token.device_approve` with the
user code hash prefix, client name, scopes and requester address.

No token is created at this moment. Approval records *who* approved and *what*;
the token is minted when the CLI next polls (§7.3).

#### `POST /api/device/requests/{userCode}/deny` (browser)

Guard: `session`, `noToken`. Response `200 {"denied": true}`. Audit:
`token.device_deny`.

### 6.3 HTTP API: revoke the calling token

#### `DELETE /api/me/token` (CLI, with the token being revoked)

Guard: `session`, `anyToken`. If the principal is a browser session (no token),
respond `400 token_required`. Otherwise revoke `p.Token.ID` for `p.User.ID` and
audit `token.revoke_self`. Response `200 {"revoked": true}`.

This is a deliberate, narrow exception to "a token cannot revoke tokens": a
token can only revoke *itself*, which can never widen access. See §7.5.

### 6.4 The credentials file

Location, in order:

1. `$FRANCOIS_CONFIG_DIR/credentials.json` if `FRANCOIS_CONFIG_DIR` is set
   (tests and multiple identities).
2. `$XDG_CONFIG_HOME/francois/credentials.json` if `XDG_CONFIG_HOME` is set.
3. `~/.config/francois/credentials.json` otherwise, on every OS (§7.4).

Format (version 1):

```json
{
  "version": 1,
  "default": "https://slop.boutique",
  "servers": {
    "https://slop.boutique": {
      "token": "frt_…",
      "tokenId": "tok_9f…",
      "name": "francois CLI on ada-laptop",
      "user": { "id": "usr_…", "name": "Ada", "email": "ada@example.com",
                "role": "creator" },
      "scopes": ["studio", "account"],
      "expiresAt": "2027-01-19T14:02:40Z",
      "savedAt": "2026-10-21T14:02:41Z",
      "host": ""
    }
  }
}
```

- Keys are **normalized origins**: lower-case scheme and host, default ports
  removed, no path or trailing slash (`https://Slop.Boutique:443/` →
  `https://slop.boutique`).
- `host` stores a `--host` override used at login (tunnels), so later commands
  reuse it.
- User, scopes and expiry are a cache for `auth status` and for friendly
  warnings ("token expires in 3 days"). The server stays the authority.
- The directory is created `0700`, the file written `0600` via a temporary file
  in the same directory plus `rename`, so a crash never leaves half a file. On
  load, a file readable by group or others produces a warning on stderr (like
  `ssh` does for keys) but still works.

Go API (new package `pkg/catalog/credentials`, no network code):

```go
package credentials

type User struct{ ID, Name, Email, Role string }

type Entry struct {
    Token     string     `json:"token"`
    TokenID   string     `json:"tokenId,omitempty"`
    Name      string     `json:"name,omitempty"`
    User      User       `json:"user"`
    Scopes    []string   `json:"scopes,omitempty"`
    ExpiresAt *time.Time `json:"expiresAt,omitempty"`
    SavedAt   time.Time  `json:"savedAt"`
    Host      string     `json:"host,omitempty"`
}

type File struct {
    Version int              `json:"version"`
    Default string           `json:"default,omitempty"`
    Servers map[string]Entry `json:"servers"`
}

func Path() (string, error)                       // §6.4 order
func Load() (File, error)                         // missing file = empty File
func (f *File) Put(origin string, e Entry, makeDefault bool)
func (f *File) Remove(origin string) bool
func Save(f File) error                           // 0700 dir, 0600 atomic file
func NormalizeOrigin(raw string) (string, error)

// Source says where a resolved value came from, for messages and auth status.
type Source string // "flag", "env", "file", "token-file"

type Resolved struct {
    Server, Token, Host       string
    ServerSource, TokenSource Source
    Entry                     *Entry // set when the token came from the file
}

// Inputs are the raw flag values; Resolve reads FRANCOIS_SERVER/FRANCOIS_TOKEN.
type Inputs struct{ Server, TokenFile, Host string }

func Resolve(in Inputs) (Resolved, error)
```

Resolution precedence (keeps every current invocation working):

| Value | 1st | 2nd | 3rd | 4th |
| --- | --- | --- | --- | --- |
| server | `--server` | `FRANCOIS_SERVER` | file `default` | error |
| token | `--token-file` | `FRANCOIS_TOKEN` | file entry for the resolved server | error |
| host | `--host` | file entry `host` | none | — |

The error when nothing is found names the next step:
`not logged in to https://slop.boutique: run "francois catalog auth login
--server https://slop.boutique", or set FRANCOIS_TOKEN`.

### 6.5 CLI commands

A new group `francois catalog auth` (decision on naming in §11):

```text
francois catalog auth login  [--server URL] [--scopes studio,account] [--days 90]
                             [--name TEXT] [--no-browser] [--with-token]
                             [--no-default] [--host H]
francois catalog auth status [--server URL]        # Glazed rows, no secrets
francois catalog auth token  [--server URL]        # prints the secret to stdout
francois catalog auth logout [--server URL] [--all] [--keep-token]
```

- `login` (default): device flow. Prints the URL and code to **stderr**, tries
  to open a browser unless `--no-browser`, `SSH_CONNECTION` is set, or stdout is
  not a terminal; polls; saves; verifies with `GET /api/me`; prints one row.
- `login --with-token`: reads a token from stdin (`francois catalog auth login
  --with-token < token.txt`), verifies it with `GET /api/me`, and saves it. This
  covers existing tokens, CI images and machines where the device flow is not
  wanted. It needs no server changes, so it ships in phase 1.
- `status`: one row per stored server: server, default, user, role, scopes,
  expires, token id, and the *effective* source for the current environment (so
  `FRANCOIS_TOKEN` overriding the file is visible).
- `token`: prints the stored secret for `export FRANCOIS_TOKEN=$(…)` or for
  configuring an MCP client. Refuses when stdout is a terminal unless `--yes`,
  to avoid secrets in scrollback (open question in §11).
- `logout`: calls `DELETE /api/me/token` (best effort; a 401 means the token was
  already invalid), then removes the entry. `--keep-token` only forgets it
  locally. `--all` iterates over every stored server.

Example session:

```text
$ francois catalog auth login --server https://slop.boutique
To authorize this terminal, open
  https://slop.boutique/device?code=WDJB-MJHT
and confirm the code  WDJB-MJHT  (expires in 10 minutes).
Waiting for approval...
+-----------------------+-------+---------+----------------+------------+
| server                | user  | role    | scopes         | expires    |
+-----------------------+-------+---------+----------------+------------+
| https://slop.boutique | Ada   | creator | studio account | 2027-01-19 |
+-----------------------+-------+---------+----------------+------------+
Saved to /Users/ada/.config/francois/credentials.json

$ francois catalog whoami          # no FRANCOIS_TOKEN, no --server needed
$ francois catalog releases push --artifact-id loom --version 1.0.1 --html dist/app.html
```

### 6.6 The approval page

A new SPA route `/device` (add `"GET /device"` to the list at `routes.go:28`,
and a branch in `ui/catalog/main.tsx`). Behavior:

1. **Signed out:** explain what the page is for and show **Sign in**, linking to
   `loginHref(location.pathname + location.search)`. `safeReturn` already allows
   `/device?code=…` as a return path (`auth.go:174-184`).
2. **No `code` query parameter:** a single input "Enter the code shown in your
   terminal", accepting `wdjbmjht`, `WDJB MJHT`, `wdjb-mjht`.
3. **With a code:** fetch `GET /api/device/requests/{code}`. Show:
   - "**francois CLI on ada-laptop** wants to act as **Ada (ada@example.com)**
     on **slop.boutique**."
   - "Requested from 203.0.113.7, 2 minutes ago. Expires in 8 minutes."
   - The code itself, large, with "Check that this matches your terminal."
   - Scope checkboxes for `allowedScopes`, pre-checked where requested, with the
     same descriptions as the account page (`scopeInfo`, `tokens.tsx:7-12`).
     Admin scopes are never pre-checked.
   - A days input with the same limits as the account page (1–90 when an admin
     scope is checked, otherwise 0–366).
   - A warning: "Only approve if you started this sign-in yourself, just now.
     Anyone holding the resulting token can act as you within these scopes."
   - **Approve** and **Deny** buttons.
4. **After approve:** "Done. You can return to your terminal." After deny:
   "Denied. The terminal will stop waiting." `404` shows "This code is invalid
   or has expired. Run the login command again."

## 7. Decision records

### 7.1 Decision: francois-native device flow instead of ZITADEL's device grant

- **Context:** ZITADEL supports RFC 8628 itself. The CLI could get a ZITADEL
  access token and trade it for a francois token. Alternatively francois can run
  the flow and mint from its own session.
- **Options considered:** (a) francois-native device flow minting personal API
  tokens from a portal session; (b) ZITADEL device grant plus a new
  token-exchange endpoint; (c) loopback redirect (the CLI listens on
  `127.0.0.1:<port>`, the browser redirects back with a code).
- **Decision:** (a).
- **Rationale:** (a) reuses everything that already works: every sign-in method
  (Google, GitHub, email link) ends in a portal session, and minting from a
  session is today's exact security model. (b) needs a new ZITADEL application,
  Terraform changes, token introspection, identity mapping, and does not work in
  local development without ZITADEL. (c) does not work over SSH or in
  containers, which is precisely where people run agents; it can be added later
  as a convenience on top of the same approval page.
- **Consequences:** francois owns a small amount of new security-sensitive code
  (user codes, polling, rate limits). It must be reviewed with RFC 8628 §5 in
  mind. OAuth for remote MCP connectors remains a separate project.
- **Status:** proposed.

### 7.2 Decision: approval is session-bound; tokens still cannot create tokens

- **Context:** FRANCOIS-007 decided tokens cannot mint tokens. The device flow
  must not quietly undo that.
- **Options considered:** (a) mint only after a browser session approves; (b)
  let a token with some scope call a "mint child token" endpoint.
- **Decision:** (a). The poll endpoint holds no credential except the device
  code, and the device code is worthless until a session approves it.
- **Rationale:** keeps the invariant and its existing test
  (`tokens_test.go:54-56`) intact. A stolen token still cannot multiply itself.
- **Consequences:** the approve route must be `noToken` and session-level so the
  guard enforces Origin and CSRF. Add a test that a bearer token cannot approve.
- **Status:** proposed.

### 7.3 Decision: mint at poll time, never store the secret

- **Context:** after approval, the CLI must receive the secret. The secret
  could be created at approval and parked in the row until polled, or created
  when the CLI polls.
- **Options considered:** (a) create at approval and store the secret
  (encrypted) in the device row; (b) create at the first poll after approval,
  inside one transaction that also deletes the device row.
- **Decision:** (b).
- **Rationale:** the database never holds a usable secret, matching how
  `api_tokens` and sessions work. The single transaction makes the claim
  exactly-once: a second poll finds no row and gets `expired_token`.
- **Consequences:** role and account state must be re-checked at poll time (the
  approver could be disabled in between). If the CLI dies between approval and
  poll, no token exists and nothing needs cleaning up. If the response is lost
  in transit after commit, a token exists that nobody holds; it is visible on
  the account page and can be revoked. That is acceptable and the same as
  closing the browser tab after **Create token** today.
- **Status:** proposed.

### 7.4 Decision: a plain 0600 JSON file under ~/.config, keychain later

- **Context:** the token must persist between CLI runs.
- **Options considered:** (a) `0600` JSON file; (b) OS keychain
  (`github.com/zalando/go-keyring` or similar) with file fallback; (c) keep
  env-only and document shell profiles.
- **Decision:** (a) now, with the file format leaving room for (b).
- **Rationale:** a file works identically on macOS, Linux, containers and over
  SSH, is easy to test (`FRANCOIS_CONFIG_DIR`), and matches what `gh`, `docker`
  and `kubectl` do by default. Keychains add a cgo or D-Bus dependency, prompt
  unexpectedly in headless sessions, and complicate tests. `~/.config/francois`
  (rather than `os.UserConfigDir()`, which is `~/Library/Application Support` on
  macOS) matches the path already shown in `doc/production.md:423` and is where
  CLI users look.
- **Consequences:** a token is as safe as the user's home directory. Mitigations:
  `0600`, permission warning, expiring tokens by default (90 days), `auth logout`
  revokes server-side. A later `"store": "keyring"` field in an entry can move
  the secret out of the file without changing the resolution API.
- **Status:** proposed.

### 7.5 Decision: allow a token to revoke itself

- **Context:** `auth logout` should invalidate the token on the server, not just
  forget it. `DELETE /api/tokens/{id}` is `noToken` by the FRANCOIS-007 rule.
- **Options considered:** (a) `DELETE /api/me/token`, `anyToken`, revokes only
  the calling token; (b) local-only logout plus a message pointing to
  `/account`; (c) open `DELETE /api/tokens/{id}` to tokens.
- **Decision:** (a), pending owner approval; (b) is the fallback if the owner
  wants the FRANCOIS-007 rule kept absolutely.
- **Rationale:** self-revocation can only reduce access; an attacker holding a
  stolen token gains nothing from it (they could stop using it anyway). (c)
  would let a stolen token revoke the owner's other tokens.
- **Consequences:** a one-line exception to document next to the token routes.
  Test that a token cannot revoke a *different* token and that a session gets
  `400 token_required`.
- **Status:** proposed (needs owner decision, see §11).

### 7.6 Decision: in-memory rate limits

- **Context:** the public endpoints (`authorize`, `token`) and code lookups need
  abuse limits.
- **Options considered:** (a) the existing in-memory `limiter`; (b) Postgres
  counters.
- **Decision:** (a).
- **Rationale:** production runs one replica (`doc/production.md:92`) and email
  sign-in already relies on the same limiter. Per-device-code polling limits are
  stored in the device row anyway (`last_polled_at`, `interval_seconds`), so they
  survive restarts.
- **Consequences:** if the deployment ever scales out, the address-level limits
  become per-replica. Note this next to the existing limiter.
- **Status:** proposed.

## 8. Detailed design and pseudocode

### 8.1 Phase 0 refactor: one place for token validation

Extract the checks from `createToken` (`tokens.go:62-95`) so the account page
and device approval share them:

```go
// tokenGrant validates a token request for a user with role. It returns the
// de-duplicated scopes and the expiry (nil = none), or an API failure.
type grantError struct{ status int; code, message string }

func tokenGrant(role string, scopes []string, days int) ([]string, *time.Time, *grantError) {
    if days < 0 || days > 366 { return nil, nil, &grantError{400, "invalid_request", "…"} }
    allowed := store.ScopesForRole(role)
    if len(allowed) == 0 { return nil, nil, &grantError{403, "tokens_unavailable", "…"} }
    var out []string
    admin := false
    for _, sc := range scopes {
        if !slices.Contains(allowed, sc) { return …"invalid_scopes"… }
        if !slices.Contains(out, sc) { out = append(out, sc) }
        admin = admin || store.IsAdminScope(sc)
    }
    if len(out) == 0 { return …"invalid_scopes"… }
    if admin && (days < 1 || days > maxAdminTokenDays) { return …"invalid_request"… }
    if days == 0 { return out, nil, nil }
    t := time.Now().Add(time.Duration(days) * 24 * time.Hour)
    return out, &t, nil
}
```

`createToken` keeps its name check and calls `tokenGrant`. The existing tests in
`tokens_test.go` and `token_scopes_test.go` must pass unchanged; that is the
acceptance criterion for the refactor.

In the store, split `CreateToken` like `insertSession`:

```go
type querier interface {
    QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

func insertToken(ctx context.Context, db querier, user, name string,
    scopes []string, expires *time.Time) (string, APIToken, error) {
    // body of today's CreateToken, using db instead of s.Pool
}

func (s *Store) CreateToken(ctx context.Context, user, name string,
    scopes []string, expires *time.Time) (string, APIToken, error) {
    return insertToken(ctx, s.Pool, user, name, scopes, expires)
}
```

### 8.2 Codes

- **Device code:** `"fdc_" + base64url(32 random bytes)`. Stored as
  `store.Hash(deviceCode)`. Never shown to the human.
- **User code:** 8 characters from the 20-letter alphabet
  `BCDFGHJKLMNPQRSTVWXZ` (no vowels, so no accidental words; no `0/O`, `1/I`
  confusion), displayed as `XXXX-XXXX`. This is the RFC 8628 §6.1 recommendation.
  Entropy: 20^8 ≈ 2^34.6. Stored as `store.Hash(normalized)` in a `UNIQUE`
  column; on the rare collision, regenerate.
- **Normalization:** upper-case, drop every character that is not in the
  alphabet (spaces, dashes), then require exactly 8 characters.

```go
const userCodeAlphabet = "BCDFGHJKLMNPQRSTVWXZ"

func newUserCode() string {
    b := make([]byte, 8)
    for i := range b {
        n, _ := rand.Int(rand.Reader, big.NewInt(int64(len(userCodeAlphabet))))
        b[i] = userCodeAlphabet[n.Int64()]
    }
    return string(b[:4]) + "-" + string(b[4:])
}

func normalizeUserCode(in string) (string, bool) {
    var b strings.Builder
    for _, r := range strings.ToUpper(in) {
        if strings.ContainsRune(userCodeAlphabet, r) { b.WriteRune(r) }
    }
    s := b.String()
    return s, len(s) == 8
}
```

Hash the normalized form *without* the dash, so `WDJB-MJHT` and `wdjbmjht` find
the same row.

### 8.3 Schema: `019_device_authorizations.sql`

The latest migrations are `017_daily_tokens.sql`, `017_year_payment_state.sql`
and `018_early_access.sql`; the runner applies files in name order
(`store/store.go:52-99`), so the next free name is `019_…`.

```sql
-- FRANCOIS-019: pending CLI device logins. Rows live for ten minutes and are
-- deleted when the CLI claims its token. Only hashes of both codes are stored.
CREATE TABLE device_authorizations (
    device_code_hash  bytea PRIMARY KEY,
    user_code_hash    bytea NOT NULL UNIQUE,
    portal_origin     text NOT NULL,
    client_name       text NOT NULL,
    requested_scopes  text[] NOT NULL,
    requested_days    integer NOT NULL,
    requester_address text NOT NULL,
    status            text NOT NULL DEFAULT 'pending'
                      CHECK (status IN ('pending', 'approved', 'denied')),
    user_id           text REFERENCES users(id),
    approved_scopes   text[],
    approved_days     integer,
    interval_seconds  integer NOT NULL DEFAULT 5,
    last_polled_at    timestamptz,
    created_at        timestamptz NOT NULL DEFAULT now(),
    expires_at        timestamptz NOT NULL
);
CREATE INDEX device_authorizations_expiry ON device_authorizations (expires_at);
```

Remember: once a migration is applied anywhere, its contents must never change
(the runner refuses, `store.go:81-84`). Get it right before merging.

### 8.4 Store API (`pkg/catalog/store/device.go`)

```go
const DeviceTTL = 10 * time.Minute

type DeviceAuthorization struct {
    UserCode         string // only set right after creation
    Origin           string
    ClientName       string
    RequestedScopes  []string
    RequestedDays    int
    RequesterAddress string
    Status           string // pending | approved | denied
    UserID           string
    ApprovedScopes   []string
    ApprovedDays     int
    Interval         time.Duration
    CreatedAt        time.Time
    ExpiresAt        time.Time
}

// CreateDeviceAuthorization stores a new pending request and deletes rows that
// expired more than an hour ago (the SaveLoginFlow pattern).
func (s *Store) CreateDeviceAuthorization(ctx context.Context, deviceCode,
    userCode string, d DeviceAuthorization) error

// DeviceRequest returns a pending, unexpired request on origin by user code.
func (s *Store) DeviceRequest(ctx context.Context, userCode, origin string) (DeviceAuthorization, error)

// ApproveDevice records the approver and final grant on a pending request.
// ErrNotFound when the request is gone, expired, decided, or on another origin.
func (s *Store) ApproveDevice(ctx context.Context, userCode, origin, userID string,
    scopes []string, days int) error

func (s *Store) DenyDevice(ctx context.Context, userCode, origin, userID string) error

type PollResult struct {
    State  string // pending | slow_down | denied | expired | issued
    Secret string
    Token  APIToken
    User   User
    Interval time.Duration
}

// PollDevice is the CLI's poll. On an approved request it re-validates the
// approver, mints the token and deletes the request in one transaction.
func (s *Store) PollDevice(ctx context.Context, deviceCode, origin string,
    validate func(User, []string, int) ([]string, *time.Time, error)) (PollResult, error)
```

`PollDevice` takes a `validate` callback so the store does not import the HTTP
package; the handler passes a closure around `tokenGrant` that also rejects
disabled users and guests.

Pseudocode:

```text
PollDevice(deviceCode, origin, validate):
  tx := begin
  row := SELECT … FROM device_authorizations
         WHERE device_code_hash = hash(deviceCode) AND portal_origin = origin
         FOR UPDATE
  if no row or row.expires_at <= now():          return {State: expired}
  if row.last_polled_at != nil and
     now() - row.last_polled_at < row.interval:
      UPDATE … SET interval_seconds = interval_seconds + 5,
                   last_polled_at = now()
      commit;                                     return {State: slow_down}
  UPDATE … SET last_polled_at = now()
  switch row.status:
    case pending:  commit;                        return {State: pending}
    case denied:   DELETE row; commit;            return {State: denied}
    case approved:
      user := SELECT user WHERE id = row.user_id
      scopes, expires, err := validate(user, row.approved_scopes, row.approved_days)
      if err != nil: DELETE row; commit;          return {State: denied}
      secret, tok := insertToken(tx, user.ID, row.client_name, scopes, expires)
      DELETE row
      commit
      return {State: issued, Secret: secret, Token: tok, User: user}
```

`ApproveDevice` in SQL:

```sql
UPDATE device_authorizations
   SET status = 'approved', user_id = $3, approved_scopes = $4, approved_days = $5
 WHERE user_code_hash = $1 AND portal_origin = $2
   AND status = 'pending' AND expires_at > now()
```

`RowsAffected() == 0` → `ErrNotFound`. `DenyDevice` is the same with
`status = 'denied'`.

### 8.5 Handlers (`pkg/catalog/device.go`)

```go
func (s *Server) deviceRoutes(m *http.ServeMux) {
    m.HandleFunc("POST /api/device/authorize", s.guard(public, noToken, s.deviceAuthorize))
    m.HandleFunc("POST /api/device/token", s.guard(public, noToken, s.deviceToken))
    m.HandleFunc("GET /api/device/requests/{code}", s.guard(session, noToken, s.deviceRequest))
    m.HandleFunc("POST /api/device/requests/{code}/approve",
        s.guard(enabled, noToken, s.deviceApprove))
    m.HandleFunc("POST /api/device/requests/{code}/deny",
        s.guard(session, noToken, s.deviceDeny))
}
```

Register `s.deviceRoutes(m)` next to `s.tokenRoutes(m)` (`routes.go:100`) and
`m.HandleFunc("DELETE /api/me/token", s.guard(session, anyToken, s.revokeSelf))`
inside `tokenRoutes`.

```text
deviceAuthorize(w, r):
  if !deviceLimiter allows ("addr:"+clientAddress(r), 20/h) and ("global", 600/h):
      Fail 429 rate_limited
  read {clientName, scopes, days} (limit 2 KiB)
  validate clientName 1..80, days 0..366, each scope is a known scope name
  origin := s.site(r).PortalOrigin()
  deviceCode := "fdc_" + base64url(rand 32)
  loop up to 3 times:
      userCode := newUserCode()
      err := Store.CreateDeviceAuthorization(deviceCode, userCode, {origin, …,
              RequesterAddress: clientAddress(r), ExpiresAt: now+DeviceTTL})
      if err is unique violation on user_code_hash: continue
      break
  JSON 201 {deviceCode, userCode, verificationUri: origin+"/device",
            verificationUriComplete: origin+"/device?code="+userCode,
            expiresIn: 600, interval: 5}

deviceToken(w, r):
  if !deviceLimiter allows ("poll:"+clientAddress(r), 1200/h): Fail 429 rate_limited
  read {deviceCode}; if missing or no "fdc_" prefix: Fail 400 expired_token
  res, err := Store.PollDevice(deviceCode, origin, func(u, scopes, days) {
      if u.Disabled: return error
      return tokenGrant(u.Role, scopes, days)   // also rejects guests
  })
  switch res.State:
    pending   -> Fail 400 authorization_pending
    slow_down -> Fail 400 slow_down
    denied    -> Fail 400 access_denied
    expired   -> Fail 400 expired_token
    issued    -> // no principal on this route: pass the approver explicitly
                 Store.Audit(ctx, res.User.ID, "token.create", res.Token.ID,
                       {via: "device", name, scopes, expiresAt})
                 JSON 200 {token: res.Secret, info: res.Token, user: meUser(res.User)}

deviceRequest(w, r):
  code, ok := normalizeUserCode(r.PathValue("code"))
  if !ok or !lookupLimiter allows ("user:"+me.ID, 30 misses/h): Fail 404 not_found
  d, err := Store.DeviceRequest(code, origin); ErrNotFound -> count a miss, 404
  JSON 200 {…, allowedScopes: store.ScopesForRole(me.Role), maxAdminDays: 90}

deviceApprove(w, r):
  read {scopes, days}
  scopes, _, gerr := tokenGrant(me.Role, in.Scopes, in.Days); gerr -> Fail gerr
  err := Store.ApproveDevice(code, origin, me.ID, scopes, in.Days); ErrNotFound -> 404
  Audit("token.device_approve", {clientName, scopes, days, requester})
  OK {approved: true}
```

Note: `deviceApprove` validates with `tokenGrant` for immediate user feedback,
and `PollDevice` validates again at mint time. Both are needed: the first gives
a clear error on the page, the second protects against role or account changes
in between.

### 8.6 CLI: client methods (`pkg/catalog/client/device.go`)

```go
type DeviceCode struct {
    DeviceCode, UserCode, VerificationURI, VerificationURIComplete string
    ExpiresIn, Interval int
}
type DeviceToken struct {
    Token string `json:"token"`
    Info  Token  `json:"info"`
    User  MeUser `json:"user"`
}

// Device calls the unauthenticated device endpoints; c may have an empty token.
func (c *Client) DeviceAuthorize(ctx context.Context, clientName string,
    scopes []string, days int) (DeviceCode, error)
func (c *Client) DevicePoll(ctx context.Context, deviceCode string) (DeviceToken, error)
func (c *Client) RevokeSelf(ctx context.Context) error   // DELETE /api/me/token
```

In `newRequest` (`client.go:112`), set `Authorization` only when `c.token != ""`.
JSON tags must match the server exactly; contract tests use
`WithStrictDecoding()`.

### 8.7 CLI: login command

```text
authLogin(flags):
  if --with-token:
      token := trim(read stdin)
      origin := NormalizeOrigin(flag --server or FRANCOIS_SERVER or file default)
      me := client.New(origin, token, host).Me()          // proves it works
      save(origin, token, me)
      emit row; return

  origin := NormalizeOrigin(--server or FRANCOIS_SERVER or file default)
            else error "pass --server"
  c := client.New(origin, "", WithHost(--host))
  name := --name or "francois CLI on " + os.Hostname()
  dc := c.DeviceAuthorize(name, --scopes, --days)
  stderr: "To authorize this terminal, open\n  " + dc.VerificationURIComplete
          + "\nand confirm the code  " + dc.UserCode + "  (expires in N minutes)."
  if shouldOpenBrowser(): openBrowser(dc.VerificationURIComplete)  // errors ignored
  interval := dc.Interval seconds; deadline := now + dc.ExpiresIn
  loop:
      select ctx.Done -> return ctx.Err()       // Ctrl-C stops cleanly
      case <-time.After(interval):
      res, err := c.DevicePoll(dc.DeviceCode)
      switch:
        err == nil:                         break loop
        IsCode(err, "authorization_pending"): continue
        IsCode(err, "slow_down"):            interval += 5s; continue
        IsCode(err, "access_denied"):        return "login was denied in the browser"
        IsCode(err, "expired_token"):        return "the code expired; run login again"
        network error:                       retry with backoff until deadline
        other:                               return err
      if now > deadline: return "the code expired; run login again"
  save(origin, res.Token, {tokenId: res.Info.ID, user: res.User, scopes, expiresAt,
       host}, makeDefault: !--no-default)
  emit row {server, user, role, scopes, expires}; stderr: "Saved to <path>"

shouldOpenBrowser():
  return !--no-browser && os.Getenv("SSH_CONNECTION") == "" &&
         term.IsTerminal(stdout) && (darwin || windows || DISPLAY/WAYLAND_DISPLAY set)

openBrowser(url): darwin "open"; linux "xdg-open"; windows
                  "rundll32 url.dll,FileProtocolHandler"; Start(), never Wait()
```

Use `exec.Command` with the URL as a separate argument (no shell). The URL comes
from our own server, but treat it as data anyway: only open it if it starts with
the normalized origin.

### 8.8 CLI: wiring `connect`

```go
// connect builds a client from the connection flags, the environment and the
// stored login, in that order (see credentials.Resolve).
func connect(a in) (*client.Client, error) {
    res, err := credentials.Resolve(credentials.Inputs{
        Server: a.S("server"), TokenFile: a.S("token-file"), Host: a.S("host")})
    if err != nil {
        return nil, err
    }
    if e := res.Entry; e != nil && e.ExpiresAt != nil && time.Until(*e.ExpiresAt) < 72*time.Hour {
        fmt.Fprintf(os.Stderr, "francois: the stored token for %s expires %s; "+
            "run francois catalog auth login\n", res.Server, e.ExpiresAt.Format(time.DateOnly))
    }
    var opts []client.Option
    if res.Host != "" {
        opts = append(opts, client.WithHost(res.Host))
    }
    return client.New(res.Server, res.Token, opts...), nil
}
```

Change `releases push` to use `connectionFlags()` and `connect` instead of its
own copy (`push.go:32-33`, `69-79`, `92`). Its `--server` stops being required,
which is a strict relaxation. When the server answers `401 token_invalid` and the
token came from the file, wrap the error with "run francois catalog auth login".

## 9. Security analysis

The device flow is the riskiest part of this ticket. Walk through these with a
reviewer.

- **Remote phishing (RFC 8628 §5.4).** An attacker starts a login and sends the
  victim the link: "verify your account at slop.boutique/device?code=…". If the
  victim approves, the attacker's CLI receives a token for the victim. Mitigations:
  the approval page names the requesting client and the requesting IP address,
  shows the code with "check that this matches your terminal", warns in plain
  words, never pre-checks admin scopes, and requires a click on **Approve**
  (opening the link alone never approves). Codes expire in 10 minutes. The
  account page lists the resulting token, and the audit log records
  `token.device_approve` with the requester address.
- **User code guessing.** A signed-in attacker could try codes to approve
  *someone else's* pending request. That gives the victim's CLI a token for the
  *attacker's* account (confusing, not a privilege gain). Still, lookups are
  limited to 30 misses per user per hour; with ~2^34.6 codes and a 10-minute
  life, guessing is impractical.
- **Device code theft.** The device code is 256 random bits, sent only over
  HTTPS in a POST body (never a URL, so it is not logged by proxies), stored
  hashed, single use.
- **CSRF on approve.** Approve and deny are `session`/`enabled` mutations, so
  `guard` enforces same Origin and `X-CSRF-Token` (`auth.go:147-159`). Add a
  test that a cross-origin approve fails.
- **Tokens minting tokens.** Approve is `noToken`; poll needs no token. Covered
  by §7.2 and a test.
- **Role or account changes after approval.** Re-validated at mint time
  (§7.3).
- **Cross-site confusion.** Requests are bound to `portal_origin`; a code from
  site A cannot be approved on site B or polled through site B.
- **Secrets at rest.** Server: hashes only. Client: `0600` file, warning on
  loose permissions, `auth token` refuses to print to a terminal without
  confirmation. Never log tokens; `client.send` logs method, path and status only
  (`client.go:148`), keep it that way.
- **Lost responses.** A minted token whose response is lost is orphaned but
  visible and revocable (§7.3).

## 10. Implementation phases

Each phase is a reviewable pull request with its own tests. Phases 0 and 1 have
no server-visible behavior change and can merge independently.

### Phase 0 — refactors (no behavior change)

1. `pkg/catalog/tokens.go`: extract `tokenGrant`; `createToken` uses it.
2. `pkg/catalog/store/tokens.go`: `insertToken(ctx, querier, …)`;
   `CreateToken` delegates.
3. `pkg/catalog/client/client.go`: omit `Authorization` when the token is empty.
4. `cmd/francois/cmds/push.go`: use `connectionFlags()` and `connect`; fix the
   help text to say "account page (/account → API tokens)".

Acceptance: `go test ./...` with `TEST_DATABASE_URL` set; existing
`TestTokensAndPush` and scope tests unchanged and passing; `releases push`
works with `FRANCOIS_SERVER` set and no `--server`.

### Phase 1 — credentials store and token-based login

1. New `pkg/catalog/credentials` package (§6.4) with unit tests.
2. `connect` uses `credentials.Resolve` (§8.8).
3. New `cmd/francois/cmds/auth.go`: `auth login --with-token`, `auth status`,
   `auth token`, `auth logout --keep-token` (local only until phase 5).
   Register the group in `registerAPI` (`api_commands.go:15-29`).
4. Help topic update (`pkg/doc/topics/01-catalog-api.md`, "Connecting").

Acceptance: with `FRANCOIS_CONFIG_DIR=$(mktemp -d)`,
`echo $TOKEN | francois catalog auth login --with-token --server S` then
`francois catalog whoami` works with no environment variables; existing
`FRANCOIS_TOKEN` tests still pass.

### Phase 2 — server device flow

1. `store/migrations/019_device_authorizations.sql`.
2. `store/device.go` (§8.4).
3. `pkg/catalog/device.go` handlers (§8.5); `deviceLimiter` on `Server`
   (`server.go:152-166`), initialized next to `emailLimiter: newLimiter()`
   (`server.go:183`). `Store.Audit(ctx, actor, action, subject, summary)` is
   at `store/users.go:183`.
4. Route registration in `routes.go`.

Acceptance: integration tests in §12.2 pass.

### Phase 3 — approval page

1. `"GET /device"` in the SPA path list (`routes.go:28`).
2. `ui/catalog/device.tsx`, route branch in `ui/catalog/main.tsx`.
3. `api.ts`: `deviceRequest` query, `approveDevice` and `denyDevice` mutations.
4. Export `scopeInfo` from `tokens.tsx` and reuse it.

Acceptance: `pnpm` typecheck and build pass; manual run-through against the
local compose stack (§12.4).

### Phase 4 — CLI device login

1. `client/device.go` (§8.6).
2. `auth login` device mode with polling, browser opening, Ctrl-C handling.

Acceptance: CLI test against a fake device API (§12.3); manual end-to-end.

### Phase 5 — logout revokes

1. `DELETE /api/me/token` (`tokens.go`), `client.RevokeSelf`.
2. `auth logout` calls it, tolerating `401 token_invalid`.

Acceptance: tests in §12.2; owner sign-off on §7.5.

### Phase 6 — documentation

1. `README.md` "Hosted subscriber catalog" token paragraph (lines ~223-235).
2. `doc/production.md` token section (lines ~420-440): mention device approvals
   in the audit log and `auth login`.
3. `pkg/doc/topics/01-catalog-api.md` and `02-catalog-mcp.md`: "Run
   `francois catalog auth login` once; `francois mcp` uses the stored token."
4. Account page token panel (`tokens.tsx:79-83`): show
   `francois catalog auth login --server <origin>` as the first suggestion.
5. A smoke playbook in this ticket.

## 11. Open questions (owner decisions)

**Resolved 2026-10-21 by the owner:** (1) self-revocation via `DELETE /api/me/token`
is accepted; (2) the command group is `francois catalog auth login`; (3) admin
scopes may be granted through the device flow, and the default scopes are the
maximum the user's role allows (the CLI requests no scopes by default and the
approval page pre-checks every allowed scope, admin scopes included, still
limited to 1-90 days); (4) `auth token` refuses to print to a terminal without
`--yes`. The original questions are kept below for context.

1. **Self-revocation (§7.5).** Accept the narrow `DELETE /api/me/token`
   exception, or keep "tokens never revoke tokens" and make logout local-only?
2. **Command names.** `francois catalog auth login` (proposed, groups four
   verbs) versus top-level `francois catalog login` / `logout`. Cobra aliases
   could offer both.
3. **Default requested scopes and lifetime.** Proposed: the CLI requests
   `studio,account` and 90 days by default; the page shows only what the role
   allows. Should admin scopes be grantable through the device flow at all?
   Proposed: yes, never pre-checked, still limited to 90 days.
4. **Printing secrets.** Should `auth token` refuse to print to a terminal
   without `--yes`, or always print (like `gh auth token`)?
5. **Recent sign-in.** Should approval require a session younger than, say,
   one hour (forcing re-authentication), as some providers do for credential
   creation? Not proposed for the first version.

## 12. Test strategy

### 12.1 Unit tests (no database)

- `credentials`: `NormalizeOrigin` table (case, default ports, trailing slash,
  paths rejected, non-http schemes rejected); `Save` then `Load` round trip;
  file mode is `0600` and directory `0700`; atomic write leaves no temp files;
  `Resolve` precedence table: every row of §6.4, including "env token with file
  server" and "flag server with no stored entry".
- `normalizeUserCode` / `newUserCode`: alphabet only, length, dash handling.
- `tokenGrant`: the same cases `createToken` rejects today (subscriber +
  studio, admin scope with 0 or 91 days, empty scopes, duplicates).

### 12.2 Server integration tests (`TEST_DATABASE_URL`)

Model them on `TestTokensAndPush` (`tokens_test.go:15-56`), using `setup(t)`,
`e.user`, `e.portal` (session + CSRF) and `e.do` (raw requests):

1. Happy path: authorize → poll returns `authorization_pending` → creator
   approves `studio` 30 days → poll returns `200` with an `frt_` secret → that
   secret works on `GET /api/studio/artifacts` and `GET /api/me` shows the
   client name → a second poll returns `expired_token`.
2. `slow_down`: two polls within one second → second is `slow_down`; the next
   allowed interval grows by 5 s.
3. Deny → `access_denied`; the row is gone afterwards.
4. Expiry: set `expires_at` in the past via SQL → poll `expired_token`, request
   page `404`.
5. Scope ceilings: subscriber approving `studio` → `400 invalid_scopes`; admin
   approving `admin:write` with 0 days → `400 invalid_request`.
6. Disabled after approval: approve, then disable the user through the store,
   then poll → `access_denied`, no token row created.
7. Token cannot approve: bearer token on `POST …/approve` →
   `401 token_not_allowed`.
8. Cross-origin approve without CSRF header → `403 csrf_failed`.
9. Second site (`TestSecondSite` shows how, `catalog_test.go:586`): code from
   site A is `404` on site B.
10. Audit entries: `token.device_approve` and `token.create` with `via: device`.
11. Self-revoke: token A revokes itself → subsequent call `401 token_invalid`;
    token A cannot revoke token B (no endpoint for it); a session calling
    `DELETE /api/me/token` gets `400 token_required`.
12. Rate limit: 21st authorize from one address within an hour → `429`.

### 12.3 CLI tests (`cmd/francois/cmds`)

Extend the `fakeAPI` pattern (`api_test.go:15-44`). The current fake requires
`Authorization: Bearer frt_test` on every request; add an allow-list of
unauthenticated paths for `/api/device/*`. Script the replies so the first poll
returns `authorization_pending` and the second returns a token. Run
`catalog auth login --no-browser --server <fake>` with
`FRANCOIS_CONFIG_DIR=t.TempDir()`, then assert the file contents and that
`catalog whoami` without `FRANCOIS_TOKEN` sends the stored token. Inject a short
poll interval for tests (the fake can answer `interval: 0`; the client should
treat values below 1 s as 1 s in production and allow 0 only in tests through an
unexported variable).

### 12.4 Manual end-to-end

Against the local compose stack (`README.md:167-177`, portal at
`http://localhost:8090`, test users from `zitadel-setup` logs):

```bash
export FRANCOIS_CONFIG_DIR=$(mktemp -d)
go run ./cmd/francois catalog auth login --server http://localhost:8090
# open the printed link, sign in as admin@francois.test, approve "studio"
go run ./cmd/francois catalog whoami
go run ./cmd/francois catalog auth status
cat "$FRANCOIS_CONFIG_DIR/credentials.json"; ls -l "$FRANCOIS_CONFIG_DIR"
go run ./cmd/francois catalog auth logout
go run ./cmd/francois catalog whoami      # expect "not logged in" guidance
```

Check the account page shows the device-created token with the client name, and
that `/admin` audit shows the approval.

## 13. Risks and alternatives

- **Phishing remains possible** for any device flow; we reduce it with page
  design and short expiry. If it becomes a concern, require the user to *type*
  the code instead of following `verificationUriComplete`.
- **Plain-text token file.** Acceptable for this audience with `0600` and
  expiry; the keychain backend is the documented next step.
- **Scaling out** turns in-memory address limits into per-replica limits;
  device state itself is in Postgres and unaffected.
- **Clock skew** is irrelevant to the server-side logic (database `now()`
  everywhere); the CLI's local expiry warning is advisory only.
- **Alternative: keep copy-paste, add only `--with-token` storage.** Phase 1
  alone delivers this. It solves "store and load" but not "easily create".

## 14. File reference

Existing files to read first, in this order:

| File | Why |
| --- | --- |
| `pkg/catalog/auth.go:18-162` | Principal, access levels, token scopes, `guard` |
| `pkg/catalog/tokens.go` | Token routes, `createToken` rules, `apiMe` |
| `pkg/catalog/store/tokens.go` | Token format, scopes per role, create/resolve/revoke |
| `pkg/catalog/store/sessions.go` | `login_flows` and `insertSession` patterns |
| `pkg/catalog/store/email_links.go:67-90` | Ready + claim-by-delete pattern |
| `pkg/catalog/routes.go:21-115` | SPA paths, route table, security headers |
| `pkg/catalog/demos.go:27-74` | `limiter`, `clientAddress` |
| `cmd/francois/cmds/api.go:29-159` | Connection flags and `connect` |
| `cmd/francois/cmds/push.go` | Duplicated token loading to remove |
| `cmd/francois/cmds/mcp.go:25-38` | Another `connect` consumer |
| `pkg/catalog/client/client.go:64-171` | Client construction, headers, retries |
| `ui/catalog/tokens.tsx` | Account-page token UI and scope descriptions |
| `ui/catalog/main.tsx` | SPA routing |
| `pkg/catalog/tokens_test.go`, `pkg/catalog/catalog_test.go:33-193` | Test harness |
| `cmd/francois/cmds/api_test.go` | CLI test harness |

New files:

| File | Contents |
| --- | --- |
| `pkg/catalog/store/migrations/019_device_authorizations.sql` | Table (§8.3) |
| `pkg/catalog/store/device.go` | Store API (§8.4) |
| `pkg/catalog/device.go` | Handlers and routes (§8.5) |
| `pkg/catalog/device_test.go` | Integration tests (§12.2) |
| `pkg/catalog/credentials/credentials.go` (+ `_test.go`) | Credentials file (§6.4) |
| `pkg/catalog/client/device.go` | Client calls (§8.6) |
| `cmd/francois/cmds/auth.go` (+ `auth_test.go`) | CLI commands (§6.5, §8.7) |
| `ui/catalog/device.tsx` | Approval page (§6.6) |

## 15. External references

- RFC 8628, OAuth 2.0 Device Authorization Grant:
  <https://www.rfc-editor.org/rfc/rfc8628> — §3.1–3.5 (requests, responses,
  polling errors), §5.4 (remote phishing), §6.1 (user code alphabet).
- FRANCOIS-007 design doc (scopes, "tokens cannot mint tokens", deferred OAuth):
  `ttmp/2026/10/15/FRANCOIS-007--full-api-client-customer-management-and-an-mcp-server-for-the-hosted-catalog/design-doc/01-full-api-client-customer-management-and-mcp-server-analysis-design-and-implementation-guide.md`
- `gh auth login` / `gh auth token` behavior is the closest UX precedent for
  §6.5.
