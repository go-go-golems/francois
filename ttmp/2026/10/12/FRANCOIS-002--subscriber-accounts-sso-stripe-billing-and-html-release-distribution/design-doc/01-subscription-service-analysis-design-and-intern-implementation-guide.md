---
Title: Subscription service analysis design and intern implementation guide
Ticket: FRANCOIS-002
Status: draft
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: abs:///home/manuel/code/wesen/2026-07-25--zitadel-go-test/cmd/todo-demo/serve.go
      Note: Inspected ZITADEL OIDC and PKCE login integration
    - Path: abs:///home/manuel/code/wesen/2026-07-25--zitadel-go-test/internal/billing/stripe.go
      Note: Inspected predecessor Checkout Portal and current-state projection
    - Path: abs:///home/manuel/code/wesen/2026-07-25--zitadel-go-test/internal/store/postgres/billing.go
      Note: Inspected durable event claiming and projection transaction
    - Path: repo://cmd/francois/cmds/update.go
      Note: Existing authored HTML lifecycle
    - Path: repo://examples/polis.html
      Note: Maintained candidate subscriber application
    - Path: repo://pkg/host/server.go
      Note: Local owner and loopback boundary to preserve
ExternalSources:
    - https://docs.stripe.com/webhooks
    - https://docs.stripe.com/billing/subscriptions/webhooks
    - https://zitadel.com/docs/examples/login/go
Summary: Proposed subscriber distribution portal using ZITADEL OIDC, Stripe subscriptions, immutable HTML releases, authenticated downloads, and release notifications.
LastUpdated: 2026-10-12T21:16:55.331276-04:00
WhatFor: Teach a new intern the existing system, prior identity and billing patterns, the proposed product, and a phased implementation with explicit contracts.
WhenToUse: Before implementing subscription accounts or publicly distributing paid Francois artifacts.
---

# Subscription service: analysis, design, and intern implementation guide

## 1. What we are building

We are adding a website where a person can create an account, sign in through an existing identity provider, buy a recurring subscription, and download curated HTML applications such as POLIS IDE. An operator publishes immutable application releases. Subscribers browse the release catalog and receive links when new releases appear. The website authenticates the person and checks their subscription before sending the paid file bytes.

Francois already runs downloaded HTML applications locally. Its Go process supplies the applications' Claude-style browser APIs, SQLite state, and access to the owner's Pi model configuration. The new website has a different responsibility: distributing applications to subscribers. A customer can download an application, import it into their own Francois instance, and use their own Pi authentication. The website does not need access to that customer's local credentials.

This document is a **proposed design and implementation guide**, not a claim that user management or billing has been implemented. The existing repository baseline is `bd373eb`. No production identity configuration, Stripe resources, mailing campaigns, or remote deployment was changed during this investigation.

The recommended first release covers downloadable files and opted-in release notifications. A scope question was presented about also hosting applications and model calls; absent a different product choice, this guide proceeds with distribution. Hosted model usage would require a separate cost, isolation, and authorization design. It must not appear accidentally because a subscription portal shares a repository with a local model runner.

### Essential terms

An **identity provider** authenticates people. We propose the existing ZITADEL infrastructure, using OpenID Connect, or OIDC, to establish a verified identity. **Single sign-on**, or SSO, means people can authenticate through that provider and reuse its login across participating applications. This does not make a browser session cookie transferable between websites; each application establishes its own session.

A **local user** is the distribution service's record for a verified `(issuer, subject)` identity. The issuer identifies the trusted OIDC provider, and subject identifies a person within it. An email address is editable contact information, not that record's identity key. A **session** is the distribution site's time-limited browser login, represented by an opaque cookie and a database record.

A **subscription** is Stripe's recurring commercial relationship with a mapped customer. An **entitlement** is the application's decision that a local user may download subscriber releases now. Signing in establishes identity; it does not establish entitlement. Stripe's current subscription state is projected into the application's database, where download handlers can evaluate it cheaply.

A **catalog artifact** is a product entry such as POLIS. A **release** is an immutable published version of that artifact. A **release file** is the exact downloadable HTML or companion file associated with the release. The local runner's editable artifact versions and the subscriber site's curated releases are different records with different owners.

### One representative journey

Maya signs in with ZITADEL. The website receives verified claims and creates user `usr_...`, keyed by issuer and subject. Maya buys the configured subscription through Stripe-hosted Checkout. Stripe notifies the website through a signed HTTP request called a webhook. The site's worker retrieves current Stripe state and updates Maya's subscription projection. Maya's next download request succeeds if the account is enabled, the release is published, and the subscription qualifies.

The browser's return from Checkout can occur before the webhook has been processed. The account screen therefore initially says “Payment received; activating your subscription” only when verified billing information supports that message, or more generally “Checking subscription status.” It refreshes the local projection. It never grants access simply because a return URL contains `success=true`.

After downloading `polis-1.0.0.html`, Maya can import it locally:

```sh
go run ./cmd/francois import \
  --id polis --file ~/Downloads/polis-1.0.0.html \
  --manifest ~/Downloads/polis-capabilities.json
```

These existing import commands are executable today. The new catalog commands and HTTP endpoints later in this guide are proposed interfaces.

## 2. The current repository and the boundary we must preserve

The repository is `/home/manuel/code/wesen/go-go-golems/francois`. Its `serve`, `import`, `update`, and `models list` commands use Glazed. The main HTTP listener presents a local artifact library. Each imported application receives its own persisted loopback port, giving it a separate browser origin and storage namespace.

`pkg/host/server.go:69` creates one durable local owner for the whole host. At line 92, `Run` rejects a non-loopback listening address. `pkg/store/store.go:39` creates the SQLite artifact, version, document, asset, and comment tables. The local host's “user” capability does not represent authenticated subscriber accounts. Its stable owner identifier cannot be reused as a remote customer identity.

The existing artifact runtime has powerful operations. POLIS declares an `eval` tool that executes JavaScript in its browser page. Files can be read beneath an explicitly configured local root. The Pi worker reads private model/auth files through the SDK. These features are useful in the owner-run local process; they do not belong on the subscriber portal's origin or in its request handlers.

The new server will therefore be a separate command, proposed as `francois catalog serve`. It uses the same top-level Go module and binary but constructs a separate HTTP handler tree and service graph. Starting it will not construct `pkg/host.Host`, start artifact-origin listeners, load `worker/dist/main.js`, or read `~/.pi/agent`. The local commands remain local. This is a clear product boundary, not an adapter for the existing local identity API.

| Existing file | What the intern should learn from it | Relationship to new work |
|---|---|---|
| `cmd/francois/cmds/commands.go` | Glazed command registration and settings decoding. | Add a catalog command group alongside existing commands. |
| `cmd/francois/cmds/update.go` | Bounded complete-HTML reading and immutable local updates. | Share document validation where appropriate; do not turn update into remote billing logic. |
| `pkg/store/store.go:122` | Transactional compare-and-set publication. | A source of local authored versions for explicit catalog publication. |
| `pkg/host/server.go:69,92` | One owner and loopback-only service lifecycle. | Preserve this ownership boundary. |
| `pkg/piworker/backend.go` | Model runtime protocol and capabilities. | No dependency from the distribution server. |
| `examples/polis.html` | Maintained application, model controls, thinking, and Markdown. | Candidate release source; never execute under the portal origin. |
| `ui/src/app.tsx`, `pkg/web/embed.go` | React/RTK Query UI and embedded host assets. | Existing frontend conventions; add a distinct subscriber frontend entry point. |

The current `update` command preserves local identity, origin, and application data while replacing authored HTML. Catalog publication is similarly explicit, but it creates a subscriber release rather than silently promoting every local edit. Testing a local scene change must not send a release announcement to customers.

## 3. What go-go-parc already tells us

We reviewed five prior vault documents and preserved byte-identical copies in this ticket's `sources/` directory. `sources/manifest.json` records their original paths and SHA-256 hashes. The reports are evidence of prior work at their recorded dates; they are not proof of today's deployment status.

### ZITADEL TODO billing: the primary implementation reference

The closest predecessor is `/home/manuel/code/wesen/2026-07-25--zitadel-go-test`. Its vault report is `Projects/2026/07/26/PROJECT REPORT - Stripe Billing - End to End Subscription Infrastructure and Acceptance.md`. The local source checkout exists and was inspected, including:

- `internal/billing/stripe.go:24`: a persistence interface separating subscription orchestration from PostgreSQL.
- `internal/billing/stripe.go:101`: hosted subscription Checkout, mapped Customers, and attempt-scoped idempotency.
- `internal/billing/stripe.go:177`: signed event handling and current subscription retrieval.
- `internal/billing/stripe.go:267`: exact Price matching and subscription item period extraction.
- `internal/store/postgres/billing.go:52`: event claiming with processing tokens and reclaimable leases.
- `internal/store/postgres/billing.go:76`: transactional projection plus event completion.
- `cmd/todo-demo/serve.go:121`: ZITADEL authentication initialization and local user projection.
- `cmd/todo-demo/serve.go:251`: authorization-code flow with PKCE.
- `cmd/todo-demo/session_store.go`: application session persistence integration.

The inspected Stripe code uses `github.com/stripe/stripe-go/v86`. That is a verified predecessor version, not an instruction to select that version blindly for a new implementation. During implementation, pin a supported SDK and its matching Stripe API version, including the webhook endpoint's version, then run sandbox tests against that exact combination.

The predecessor has useful policy decisions and some domain assumptions we should not copy. It grants TODO Pro to `past_due` subscriptions and upserts subscription projection on `user_id`. This paid-file product proposes a stricter status policy and keeps subscription rows keyed by Stripe subscription ID. A delayed event for a canceled old subscription must not replace a newly purchased subscription's state simply because both belong to the same user.

### hyperblog: the download gate must withhold bytes

`Projects/2026/08/02/PROJECT REPORT - hyperblog - A Subscription Blog Whose Cursor Is a Glossary Term.md` describes a subscription blog that removes locked post bodies from responses. It also separates identity, credential scopes, and reader membership. Those are directly relevant to an HTML catalog: public metadata can be visible, but a paid HTML body must not reach an anonymous or unentitled browser.

The checkout path recorded in that report, `/home/manuel/workspaces/2026-07-30/transcript-agent/hyperblog`, is absent on this machine. We therefore use the preserved report as architectural evidence and do not claim to have verified its current source code. Its mention of membership is not evidence of an implemented Stripe integration; the explicit Stripe predecessor above supplies that evidence.

### Fortress: identity is independent of permissions

`Projects/2026/09/15/PROJ - Fortress - Developer Portal and Go Authentication Toolkit.md` documents ZITADEL-backed identities, application memberships, and separately classed API credentials. Its report also records unfinished interactive-login and deployment work. We use its separation of identity and application authorization, but do not introduce a Fortress validation service merely to authenticate browser subscribers. This product needs one website and ordinary server-managed sessions, not a fleet API-key control plane.

### serve-artifacts: publication and operations

`Projects/2026/07/21/PROJ - serve-artifacts - Write API, Production Deployment, and the JSON Contract.md` describes immutable/atomic file handling, metadata indexing, JSON errors, and persistent deployment volumes. It records an important operational failure: direct file writes did not automatically update its in-memory index in production. Our proposed publisher commits database catalog metadata explicitly; there is no watcher or search-index invalidation dependency in the first release.

### The reusable product playbook

`Projects/2026/07/27/ARTICLE - Playbook - Subscription Search Product Architecture.md` combines OIDC identity, a local application projection, Stripe Checkout, durable event handling, and server-side entitlement checks. Our new domain operation is a file download rather than a TODO write or a metered search. We reuse those verified responsibilities, not the predecessor's feature set.

### Practical reuse rule

The inspected predecessor code is under Go `internal/` directories and cannot be imported by Francois as a public library. Implement small domain-specific services in this repository using those source files and tests as references. Do not create a generic SaaS framework, copy its TODO schema, or add cross-repository compatibility adapters. An extracted shared package can be considered later when a genuine second implementation proves a stable contract.

## 4. Product contract for the first release

The proposed paid plan is one recurring membership that unlocks the published subscriber catalog. Public catalog pages show descriptions, screenshots, version notes, and runtime requirements. Subscribers can download all currently published releases while their access is valid. An operator may mark specific releases as free samples; free bytes have their own explicit download policy.

Pricing amount, currency, monthly/yearly intervals, trial availability, legal terms, and commercial license are product-owner decisions. The first implementation needs one server-configured recurring Price. Supporting yearly pricing later can be an additional accepted Price mapped to the same entitlement; it does not require a second product tier. Do not invent a price or copy the predecessor's USD 5 price.

The product decisions proposed here are:

- Downloads grant access to bytes, not remote execution or model credits.
- New paid downloads and release notifications require current entitlement.
- Already downloaded files remain on the customer's machine after cancellation; the service cannot revoke them.
- The first release does not impose download counts, seat counts, teams, organizations, or per-file purchases.
- Period-end cancellation preserves access through the already established access end while the subscription qualifies.
- Failed initial payments grant no access. The proposed first-release policy denies new paid downloads in `past_due`; a commercial grace policy can change this explicitly later.
- Published release files are immutable. Corrections produce a new release.
- Email notifications link to the authenticated release page. They contain no privileged file URL, session token, or inline paid application.

A subscription is an access policy, not DRM. A customer who already has the HTML can run, copy, or share it. License terms may define permitted use, but the server must not claim cancellation can delete a downloaded copy.

### Decision: distribute applications before hosting them

- **Context:** Francois contains a local owner runtime with powerful tools and private Pi configuration.
- **Options considered:** Add subscribers directly to the current runtime; build a distribution portal and let subscribers run locally.
- **Decision:** Propose a separate distribution command and HTTP origin first.
- **Rationale:** It meets the stated subscribe/receive/download use case without public execution or an operator-funded model budget.
- **Consequences:** Downloaded applications need documented local runtime requirements. Hosted execution is future work with a separate billing and isolation contract.
- **Status:** proposed, based on the documented first-release assumption.

## 5. Overall architecture

The simplest production shape is one distribution process, one PostgreSQL database, one private release-file directory, the existing ZITADEL provider, Stripe-hosted billing, and an email sender. The email sender can use the existing SES-backed infrastructure or an explicit SMTP configuration. No queue broker or separate billing microservice is needed; durable work is stored in PostgreSQL and drained by goroutines owned by the distribution process.

```text
Subscriber browser
   | HTTPS, site session cookie
   v
Francois catalog server ---- OIDC code exchange ---- ZITADEL
   |         |
   |         +---- Checkout / Portal requests ------ Stripe
   |                                                   |
   |                 signed webhooks <-----------------+
   |
   +---- PostgreSQL: users, sessions, subscriptions,
   |                 release metadata, durable jobs
   |
   +---- Private release files: immutable HTML/manifest bytes
   |
   +---- Email sender: verified recipient + release-page link

Subscriber downloads HTML -> local Francois -> subscriber's Pi config
```

ZITADEL is authoritative for authenticated identity. Stripe is authoritative for subscription/payment state. PostgreSQL owns local account status, sessions, Customer correlation, projected billing state, catalog metadata, and notification preferences. The release-file directory owns the stored bytes identified by a content hash. Browser components display the result of server policy; they do not grant access.

### Why PostgreSQL here while the runner uses SQLite

The existing SQLite database is local application state and already works well for that purpose. The distribution database has a separate lifecycle: subscriber records, durable webhook and email work, relational uniqueness, and recoverable billing transactions. PostgreSQL matches the inspected predecessor and its acceptance-tested transaction patterns. It is one database service, not a reason to add Redis, a message bus, or a service mesh.

The first deployment runs one replica with a persistent release directory. PostgreSQL may be the existing infrastructure service with a dedicated database/user. Back up both the database and release bytes. An S3-backed private file store can replace the directory when deployment requirements justify it; no second storage implementation is required for the first slice.

The executable may contain both local and catalog commands, but its runtime graph is selected by command. The catalog can run without Node and without any Pi credentials. A future standalone distribution archive can omit the worker; making that archive is an implementation phase, not an assumption that the existing package already does so.

## 6. User management and SSO

### Identity and local provisioning

Use one configured OIDC issuer, a dedicated web application client, and the authorization-code flow with PKCE. Request `openid`, `profile`, and `email`. Discovery determines provider endpoints and keys; never construct a discovery URL from a browser-supplied issuer. ZITADEL's current Go integration guide recommends PKCE for web applications. See [ZITADEL Go login](https://zitadel.com/docs/examples/login/go).

The OIDC library must validate issuer, audience, signature, expiration, nonce, and the returned authorization flow. These checks establish the identity used by the local projection. The durable user key is `(issuer, subject)`; email and display name are mutable profile fields. See [OIDC Core](https://openid.net/specs/openid-connect-core-1_0.html), particularly ID-token validation and claim stability.

An existing ZITADEL login can reuse the provider session. The application still creates its own local session. Social login providers can be configured at ZITADEL later; Francois does not separately integrate Google, GitHub, or other identity systems in the first implementation.

### Login transactions

A login transaction stores a hash of the random state, the nonce, PKCE verifier, a browser-binding value, expiry, and a validated local return path. It is consumed once. The browser receives a short-lived, HttpOnly flow cookie. The callback must match both stored state and the initiating browser before exchanging the code.

```text
GET /auth/login:
    validate return path is a local catalog/account path
    create state, nonce, verifier and browser-binding secret
    save short-lived login transaction
    set flow cookie
    redirect to discovered authorize endpoint using S256 challenge

GET /auth/callback:
    require matching state and flow cookie
    atomically consume unexpired transaction
    exchange code with PKCE verifier
    validate ID token and nonce using the OIDC library
    upsert local user by issuer + subject
    preserve local role and account-disabled flags
    create new session; clear flow cookie
    redirect to validated local return path
```

The design sketches describe required behavior, not a new handwritten OIDC implementation. Select and use the provider library's supported validation helpers. The predecessor's `cmd/todo-demo/serve.go` is the first-party source reference, while the current SDK examples establish how that API is supported now.

### Login message sequence

The browser transports the authorization code, while the catalog exchanges and validates it server-side. Provider tokens are not given to the React application as its session credential.

```text
Browser                Catalog                         ZITADEL
   | GET /auth/login       |                               |
   |---------------------->| save flow, set binding cookie  |
   |<----------------------| redirect to authorize          |
   |------------------------------------------------------>|
   |                  provider sign-in / existing SSO       |
   |<------------------------------------------------------|
   | GET callback code     |                               |
   |---------------------->| consume bound login flow       |
   |                       | exchange code + PKCE verifier  |
   |                       |------------------------------>|
   |                       |<------------------------------|
   |                       | validate claims, upsert user   |
   |<----------------------| opaque site-session cookie     |
   | GET /api/account      |                               |
   |---------------------->| local session + account lookup |
```

### Sessions and request authentication

Create a random 32-byte session token. Store its SHA-256 digest, user ID, creation time, idle/absolute expiry, and revocation state. The cookie uses `__Host-francois_session`, Path `/`, Secure, HttpOnly, and SameSite Lax on the public HTTPS origin, with no Domain attribute. Development uses an explicitly configured localhost mode and a distinct non-Secure cookie name.

A request hashes the cookie, loads the unexpired session and local user, and constructs a principal. Account state and roles are read from the database so an administrator's suspension or role change takes effect without waiting for a new login. Existing site sessions do not require token introspection on every page view. New login fails when the identity provider is unavailable, but still-valid site sessions can continue.

Logout is a CSRF-protected POST that revokes the local session. Provider-global logout is optional and distinct. Do not accidentally log a user out of every participating website merely because they leave this catalog. The first release supports local logout and account session revocation.

### Account surface and administrative controls

The account page shows display name, verified contact email, membership status, scheduled end date, billing management, notification preference, and logout. Users change identity-owned details through the provider's profile/security UI. Password reset, email verification, MFA, and social-account linkage remain provider features rather than new local credential implementations. Notification settings and subscription display are application-owned.

Operators need a small set of commands: list users, inspect billing/download state, disable downloads for an account, re-enable it, revoke sessions, and assign the admin role. These are proposed Glazed operator commands; database access and the operating-system identity authorize their use. Bootstrap administration by exact issuer/subject, never by “first signup wins” or an email-domain rule.

Disabling a local account denies downloads and release notifications and revokes existing sessions. A subsequently verified identity may establish a restricted account session to access its own billing-management and support pages, so disabling downloads does not prevent the person from canceling a paid subscription. No route using that restricted principal may return paid file bytes. Logging in must not overwrite the disabled flag.

Deletion is an explicit lifecycle operation, not an ordinary profile edit. Revoke sessions, stop notifications, handle the open subscription, and then remove or anonymize application profile data under the chosen retention policy. Financial record retention and privacy policy need a product/operations decision; the first release may expose a support request rather than attempting automatic destructive deletion.

### Browser writes and CSRF

Cookie-authenticated mutation routes require a session-bound CSRF token and expected Origin. The React client obtains a token through the session response and includes it on POST/PATCH/DELETE requests. The webhook endpoint is the exception: it uses Stripe's signature rather than browser sessions or CSRF tokens. Every privileged route declares its authorization requirement in its handler/middleware wiring.

## 7. Stripe catalog, Checkout, and billing management

### Stable catalog and runtime objects

Use a dedicated Stripe Product and recurring Price for the HTML membership. Product/Price definitions can follow the existing Terraform catalog approach in `/home/manuel/code/wesen/terraform/stripe`, with separate sandbox/live state. The runtime service creates Customers, Checkout Sessions, and Portal Sessions; Terraform does not own user-specific subscriptions or ephemeral test fixtures.

The configured Price ID is server-controlled. A browser requests the logical plan `subscriber`, not an arbitrary Price ID, amount, customer ID, or discount. The server selects the accepted catalog entry and validates that runtime keys and webhook mode match the deployment environment.

### Customer mapping

`billing_customers` maps a local user ID to one Stripe Customer ID with uniqueness in both directions. Email may be supplied to Stripe as contact information, but it never establishes ownership. A webhook maps Stripe Customer back to the existing local user row. Client-supplied email, a callback parameter, or unvalidated subscription metadata cannot move a Customer between users.

Customer creation uses a stable idempotency key derived from the application's deployment identity and local user ID. Persist the mapping before accepting the billing journey as complete. If Stripe creation succeeds and the database write fails, retry the same operation/key and adopt the same Customer rather than minting another one. The prior billing service demonstrates this pattern; [Stripe idempotent requests](https://docs.stripe.com/api/idempotent_requests) documents the API behavior and parameter consistency requirements.

### Checkout attempt semantics

A Checkout attempt has a durable random ID, local owner, selected logical plan, state, Session ID, and expiry. Serialize admission for a user and reuse a still-open attempt. The same attempt uses the same Stripe idempotency key; a deliberately new attempt uses a new key. A network timeout leaves an uncertain attempt pending until retrieval/retry resolves it. It is not immediately converted into a new key.

Before creating Checkout, refresh enough Customer/subscription state to avoid offering a second subscription when an open one already exists. `active`, `trialing`, `past_due`, `unpaid`, and `paused` subscriptions go to billing management; `incomplete` goes to the pending-payment recovery flow rather than a second purchase. Only terminal/no-subscription state permits a replacement purchase. The predecessor's open-subscription helper is an example to inspect, not a policy to copy unchanged.

Do not hold a database transaction open while making a normal Checkout network request. Reserve the attempt with a short transaction, call Stripe with a bounded context, and finalize it using the attempt identity. A reconciler resolves pending/uncertain attempts after crashes. A double click and a request retry must resolve to the same usable Session.

The Checkout creation request uses `mode=subscription`, the mapped Customer, one configured Price at quantity one, validated success/cancel URLs, and diagnostic metadata containing local IDs. Use Stripe-hosted Checkout; the site never receives card numbers. Details such as billing address, Automatic Tax, Customer updates, and permitted payment methods are configured deliberately and exercised in sandbox. See [Checkout Session creation](https://docs.stripe.com/api/checkout/sessions/create).

### Return from Checkout

The return page shows the local billing status and polls a lightweight account endpoint for a bounded time. It may enqueue reconciliation using an authenticated, owner-checked Session reference. It must verify that the referenced Checkout Session belongs to the logged-in user's mapped Customer before reading it or requesting reconciliation.

The return route never sets an entitlement from a URL flag or treats an unverified browser redirect as proof of payment. A closed browser tab does not prevent activation: the webhook and worker are sufficient. An immediate redirect before webhook processing does not bypass activation: a pending account screen is expected.

### Customer Portal

Create Portal Sessions on a CSRF-protected authenticated POST. Look up the Customer from the principal's local user ID; ignore customer identifiers from the browser. Return the fresh Portal URL and let the browser navigate to Stripe. Configure payment-method updates, invoices, and period-end cancellation there instead of implementing card management screens. See [Stripe Customer Portal integration](https://docs.stripe.com/customer-management/integrate-customer-portal).

A return from Portal is also advisory. Subscription change webhooks and reconciliation update local access. Stripe customer billing email remains separate from OIDC identity and the app's notification preference. The first release uses the supported Customers v1 resource shape demonstrated by the predecessor; introducing Accounts v2 or Connect would be a separate product change.

## 8. Webhook handling and subscription convergence

A webhook is an authenticated notification that a Stripe resource may have changed. Delivery can be duplicated or occur out of order. An event's payload is not a safe replacement for retrieving current resource state. The implementation treats the event as a durable refresh request, rather than applying status transitions in delivery order.

The HTTP handler reads a bounded raw request body, verifies `Stripe-Signature` against configured current/previous endpoint secrets, checks expected sandbox/live mode, extracts supported routing identifiers, and inserts a durable inbox row. Return 2xx after successful persistence, not after all provider calls. Invalid signatures receive 400; database failure receives 5xx so Stripe can retry. Do not reserialize the JSON before verification. [Stripe's webhook guide](https://docs.stripe.com/webhooks) documents raw-body verification, duplicates, asynchronous handling, and delivery-order limitations.

```text
POST /webhooks/stripe:
    raw = read bounded original request body
    event = verify signature with matching SDK/API version
    reject wrong live/test mode
    classify supported event and extract safe resource identifiers
    insert inbox event id if not already present
    return 200 after durable insert
```

Store routing fields, event type, event ID, creation time, received time, and processing state. No raw event payload is needed for this first-release refresh strategy. Logs contain event ID/type, outcome code, attempts, and latency; they do not contain customer addresses, payment payloads, raw session tokens, or secrets.

### Event families and extraction

Support subscription created/updated/deleted, Checkout completed and asynchronous payment outcomes, and invoice paid/payment failed/action required/finalization failed. Every family maps to a current Customer/subscription refresh. Record unsupported signed events as ignored, rather than failing delivery indefinitely. The concrete field paths depend on the pinned API version; the implementation must use the matching SDK types and fixtures. [Subscription webhook guidance](https://docs.stripe.com/billing/subscriptions/webhooks) and the [event catalog](https://docs.stripe.com/api/events/types) are the API references.

Newer Stripe APIs use subscription item-level period fields. The predecessor already reads `item.CurrentPeriodEnd`. Do not revive older examples using a top-level subscription `current_period_end`. Verify the selected item's period in the pinned schema. See [Stripe's item-period change](https://docs.stripe.com/changelog/basil/2025-03-31/deprecate-subscription-current-period-start-and-end).

### One durable worker first

Run one billing worker goroutine under the process's errgroup. It claims a due inbox record with a processing token and lease, makes bounded Stripe retrievals, and transactionally writes subscription projections and marks the inbox item complete. A crash leaves the lease reclaimable; a failed provider request schedules backoff and exposes a sanitized error code. A wake-up channel reduces latency, but periodic database polling is the correctness path.

The first deployment is one replica and one billing worker. Use a rolling strategy that does not run two catalog instances over the private local file directory. Event claiming still protects retries and administrative job insertion; a distributed worker topology is not required. If replicas are introduced later, serializing Customer refresh across workers becomes part of that change's acceptance criteria.

```text
billing worker:
    repeat until context canceled:
        claim one due event with token and lease
        if no event: wait for wake-up or polling interval
        retrieve current Customer subscriptions with pagination
        resolve ownership by trusted Customer mapping
        derive each relevant subscription projection
        transaction:
            verify event processing token is still owned
            upsert projections by Stripe subscription ID
            mark event processed
        on retryable failure: record code and next_attempt_at
```

Fetch all relevant current subscriptions for the Customer when reconciling access, including replacement subscriptions. A deleted old subscription event must not grant or revoke access by overwriting one user-level “current subscription” row. The Customer's qualifying current rows determine access. Unknown Customer mappings go to an observable retry/error condition; they do not create users based on event email.

A scheduled reconciliation pass periodically enqueues refresh work for known nonterminal subscriptions and due Checkout attempts. It uses the same queue/projection path as webhooks, so an operator retry cannot race a second implementation of state updates. This handles missed deliveries and restores convergence after a provider outage without creating a polling request for every download.

### Purchase activation sequence

Checkout return and webhook delivery are independent. The diagram makes the activation boundary explicit: only persisted and reconciled billing facts can change download access.

```text
Browser                Catalog                         Stripe
   | POST Checkout         |                               |
   |---------------------->| reserve attempt; create Session|
   |                       |------------------------------>|
   |<----------------------|<-------------------------------|
   |------------------------------------------------------>|
   |                    hosted payment                      |
   |<------------------------------------------------------|
   | return page           |                               |
   |---------------------->| show pending/current projection|
   |                       |<------ signed webhook --------|
   |                       | persist event; reply 200        |
   |                       | worker retrieves current state |
   |                       |------------------------------>|
   |                       |<------------------------------|
   |                       | commit subscription projection |
   | GET download          |                               |
   |---------------------->| evaluate current entitlement   |
   |<----------------------| attachment bytes or JSON denial|
```

### Cancellation, renewal, and period expiry

Scheduled cancellation records an end date but preserves qualifying access until that date. Terminal cancellation removes access on projection. A renewal advances the item's access period after Stripe current-state retrieval. If the billing worker is unavailable beyond the stored period end, the handler does not invent a renewal; it temporarily denies new paid downloads until state can be refreshed.

Two requests arriving out of order still retrieve current Stripe state. Two events belonging to different subscription IDs update different rows. The user-level access calculation is derived from all accepted rows. These are separate protections; merely adding a unique event ID does not solve an old subscription overwriting a new subscription.

## 9. Entitlement policy

Implement one pure policy function and call it from downloads, account views, and notification recipient selection. Do not duplicate status strings in several handlers. Inputs are the principal's local account state, accepted Price IDs, release visibility, and current subscription projections; output includes allowed/denied and a stable reason code.

| Stripe status | Proposed paid-download access | Explanation |
|---|---|---|
| `active` | Allowed until the qualifying item's stored period end. | Normal paid membership. |
| `trialing` | Allowed only if an intentional configured trial exists, bounded by its end. | No trial is introduced by default. |
| `past_due` | Denied. | First release uses no implicit unpaid-download grace. |
| `incomplete`, `incomplete_expired` | Denied. | Purchase is not successfully active. |
| `unpaid`, `paused`, `canceled` | Denied. | No qualifying current service period. |
| Unknown status or unknown Price | Denied. | Unsupported commercial state does not grant access. |

Require exactly the configured membership item at quantity one, known recurring Price, and a valid period. Multiple conflicting membership subscriptions are surfaced for operator review. Do not silently resolve a malformed commercial state by choosing the most expensive item. A free sample release uses its explicit free policy and does not become paid merely because the viewer is a subscriber.

```text
CanDownload(user, release, subscriptions, now):
    if release not published: deny release_unavailable
    if user is authenticated and disabled: deny account_disabled
    if release explicitly free: allow free_sample
    if no authenticated user: deny login_required
    eligible = subscriptions matching accepted price and item shape
    if conflicting nonterminal membership rows: deny billing_conflict
    if one row is active and now < access_until: allow subscriber
    if configured trial and now < trial_until: allow subscriber_trial
    deny subscription_required
```

The concrete Go policy uses a principal containing `Authenticated` and optional user state. Anonymous sample downloads are permitted only by an explicitly free release; a missing session never satisfies the paid branch.

The proposed strict `past_due` policy differs from the TODO predecessor's grace policy. That is a product decision, not a claim that Stripe mandates denial. If the owner chooses grace, add a named bounded policy and tests. Do not infer grace from copied code.

Account suspension overrides commercial entitlement. Admin role allows publishing/operations, but should not silently fabricate a paid subscription record. If staff need catalog access, define a separate explicit staff-preview path on a distinct origin or an audited grant; no staff-preview execution is necessary for the first release.

Refund and dispute handling require a stated product policy. A refund alone does not guarantee a subscription changes status. The initial operational runbook should cancel/suspend access when required and reconcile, rather than pretending the status table automatically handles every financial event. This guide does not decide refund law or tax liability.

## 10. Curated HTML release publication

### Release model

A catalog artifact has a stable ID/slug, title, description, tags, screenshots, and visibility. A release has a distinct ID, operator-supplied version label, changelog, publication time, runtime requirements, and one or more immutable files. Version labels can be human-oriented strings; a release ID is the durable key. Published file bytes never change beneath that ID.

Start with two supported release files: the authored HTML and an optional capability manifest. A generated README can explain local import, runtime requirements, CDN dependencies, and model choices. ZIP bundles are a later format if customers need multi-file sites; Francois currently imports complete single HTML documents rather than automatically collecting sibling files.

`examples/polis.html` and `examples/polis.manifest.json` are known publication inputs. The published HTML should be authored bytes, not the response fetched from a running local artifact origin, because that response contains view/runtime bootstrap information that belongs to the owner's local process. The local store's `HTML` method can export a selected immutable authored version when that is the desired source.

### Private content storage

Compute SHA-256 for each file and use a server-generated storage key, for example `blobs/ab/<fullhash>`. The key is not a user-provided filesystem path. Keep the directory outside embedded assets, outside the SPA's static tree, and outside any unprotected file server. A hash is an identifier, not an access credential.

Write a temporary file, verify size/hash, sync if the storage policy requires durability, then atomically rename to its immutable key. Only after the file exists should the database transaction publish the release and reference its file records. A crash before metadata commit may leave an unused blob, which can be cleaned later; a committed release must never refer to a half-written file.

Do not build a full garbage-collection service for the first release. An operator verification command can report missing/unreferenced files. Backups retain all published releases. File removal requires checking every release reference; a withdrawal changes visibility without necessarily deleting bytes.

```text
catalog publish:
    read and validate authored HTML and manifest
    reject secret/bootstrap content and unsupported release shape
    compute file sizes and hashes
    write immutable private blobs atomically
    transaction:
        require unique artifact/version label
        create release and file metadata
        mark release published
        record announcement publication marker
    return release ID, hashes, and publication state
```

Secrets cannot be proven absent with a keyword scan alone. The operator must review the release and its dependencies. A publication validation check should detect known injected local-view bootstrap, credential-like configuration, accidental database copies, and invalid HTML, and surface runtime dependencies. It is an authoring check, not an arbitrary-code safety claim.

### Proposed operator commands

```sh
# Proposed; these commands do not exist yet.
francois catalog migrate --config-file /path/to/catalog.yaml
francois catalog serve --config-file /path/to/catalog.yaml
francois catalog publish --artifact-id polis --version 1.0.0 \
  --html examples/polis.html --manifest examples/polis.manifest.json
francois catalog users inspect --user-id usr_example
francois catalog users disable --user-id usr_example
francois catalog billing reconcile --user-id usr_example
francois catalog deliveries retry --release-id rel_example
```

All settings come through Glazed fields/sections. Secret values should be read from mounted files or an explicitly configured secret mechanism, not copied into command history or checked-in YAML. No new top-level environment reads should be introduced without following the repository's notification requirement.

## 11. Download delivery and browser-origin isolation

The download handler resolves the site session, loads local account state, resolves the immutable release/file, evaluates entitlement, and opens the private file through the store's safe root. It streams the exact bytes with `Content-Disposition: attachment`, a safe generated filename, `nosniff`, and `Cache-Control: private, no-store`. Use an inert binary media type for arbitrary HTML download rather than advertising a navigable portal document.

```text
GET /api/releases/{release}/files/{file}/download:
    principal = resolve session
    metadata = resolve published release and file IDs
    authorize before opening or writing bytes
    if denied: structured JSON error; no content prefix leaked
    file = open trusted immutable key beneath release root
    set attachment filename and private/no-store headers
    stream file using request context
```

Do not serve paid bytes under `/static/`, a public blob URL, a thumbnail route, a “source” endpoint, or an unauthenticated preview. Catalog metadata has its own view type that omits file bodies. The frontend must not preload an entire paid HTML application before checking a lock flag. This follows hyperblog's server-side byte-withholding principle.

The portal does not execute downloaded applications under its own authenticated origin. Merely placing paid HTML in an iframe on the portal origin would give application JavaScript access to same-origin requests and possibly account actions. For the first release, use operator-reviewed screenshots and descriptions for preview. If interactive preview becomes necessary, use a separate origin without subscriber cookies and design that execution boundary explicitly.

A download link containing only release/file IDs still requires a session and current entitlement. It is not a bearer grant. Email links go to the catalog page and use the same download handler. Do not introduce presigned public links for the local-filesystem implementation.

HEAD and range requests must pass the same authorization as full downloads. For the small initial HTML files, a full streaming GET is sufficient; range support is not required. If introduced later, it must not bypass entitlement. Browser/intermediary caching must not keep returning paid bytes through the service after access changes, while the already saved customer copy remains outside service control.

## 12. Receiving new releases by email

Publishing a release makes it visible in the subscriber catalog immediately after the publication transaction. Email is a separate durable consequence. A verified subscriber who opted into notifications receives a short message containing the release title, version, summary, and a link to its catalog page. There are no attachments or secret URLs in the first version.

Store user preferences separately from identity claims. Enabling notifications requires a verified address and records the preference timestamp. A changed or unverified address should not receive mail until verification is established. Global unsubscribe turns release notifications off while leaving identity and billing unchanged.

The email worker uses a narrow `Mailer` interface. Prefer the existing SES-backed infrastructure where practical; investigate sender verification, SPF/DKIM, bounce and complaint handling in that environment before launch. [SES deliverability documentation](https://docs.aws.amazon.com/ses/latest/dg/send-email-concepts-deliverability.html) is the operational reference. Billing receipts remain Stripe's responsibility; identity verification emails remain ZITADEL's responsibility.

### Durable recipients and retries

The release publication transaction records an announcement generation marker. Recipient generation considers existing users as of the publication timestamp and their eligibility when each page is scanned; later signups do not receive historical announcements automatically. A worker scans eligible users in pages and inserts deliveries with unique `(release_id, user_id, channel)`. It can resume after a crash because inserting the same recipient again is idempotent. A completion marker means recipient generation finished, not that every email was sent.

Immediately before sending, recheck preference, verified contact, account state, and entitlement. A user who canceled or unsubscribed after publication is skipped. Keep delivery state, attempt count, next attempt time, provider message ID, and a sanitized failure code. Retry temporary sender/network failures with bounded backoff. Suppress permanent bounces and complaints based on provider events or the selected SMTP/SES integration.

```text
notification worker:
    for each published release whose recipients are not fully generated:
        insert unique eligible-user deliveries in bounded pages
        persist generation cursor/completion
    for each due delivery:
        recheck recipient eligibility
        if not eligible: mark skipped
        send versioned release-page message
        persist provider message ID and sent state
```

There is no atomic transaction spanning the email provider and PostgreSQL. A crash after the provider accepts a message but before local completion can produce a duplicate on retry. Do not claim exactly-once email. Use a stable message ID, deduplicated jobs, and the provider's supported idempotency if available; otherwise accept rare duplicates and make their content harmless.

Use signed opaque unsubscribe tokens containing no email address. A confirmation page can be opened by GET; mutation uses POST. If standards-based one-click unsubscribe is enabled, implement its separate documented endpoint semantics and suppression policy. Do not make a casual GET mutate preferences because mail scanners follow links.

No messages were sent during this design task. The guide authorizes future implementation of this explicitly requested delivery workflow, not an unsolicited announcement to an existing list.

## 13. Proposed data model

The catalog database is distinct from local `francois.db`. The following table inventory is a design sketch, not an applied migration. Use UUIDs or cryptographically generated opaque IDs consistently, timestamps with timezone, foreign keys, and explicit migration files.

| Table | Key/important fields | Required invariant |
|---|---|---|
| `users` | id, issuer, subject, email, email_verified, display_name, role, disabled | Unique issuer/subject; profile updates never reset role/disabled. |
| `login_flows` | state_hash, verifier, nonce, binding_hash, return_path, expires_at | One-time consumption, short expiry. |
| `sessions` | token_hash, user_id, created_at, last_seen_at, expires_at, absolute_expires_at, revoked_at | No usable plaintext session token in storage. |
| `billing_customers` | user_id, stripe_customer_id | Unique both ways. |
| `billing_subscriptions` | stripe_subscription_id, user_id, customer_id, price_id, status, quantity, access_until, trial_until, cancel flags, refreshed_at | One row per Stripe subscription, not one mutable slot per person. |
| `checkout_attempts` | id, user_id, plan, state, session_id, expires_at, next_check_at | At most one admitted open attempt per user; retries keep its key. |
| `stripe_events` | event_id, type, customer/subscription/session IDs, mode, state, attempts, lease_token, lease_until, next_attempt_at, error_code | Durable deduplication and recoverable processing. |
| `catalog_artifacts` | id, slug, title, description, tags, visibility | Stable catalog identity; public metadata contains no HTML body. |
| `releases` | id, artifact_id, version, changelog, published_at, withdrawn_at, requirements, recipient generation cursor/state | Unique artifact/version; publication is explicit. |
| `release_files` | id, release_id, name, media_type, size, sha256, storage_key | Immutable private storage key and hash. |
| `notification_preferences` | user_id, enabled, verified contact revision, updated_at | Independent of identity/billing state. |
| `release_deliveries` | release_id, user_id, channel, state, attempts, next_attempt_at, provider_message_id, error_code | Unique release/user/channel; retryable delivery. |

The first migration should encode the important uniqueness in the database:

```sql
-- Selected invariants; incomplete schema sketch.
UNIQUE (issuer, subject)                         -- users
UNIQUE (user_id)                                 -- billing_customers
UNIQUE (stripe_customer_id)                      -- billing_customers
PRIMARY KEY (stripe_subscription_id)             -- billing_subscriptions
PRIMARY KEY (event_id)                           -- stripe_events
UNIQUE (artifact_id, version)                    -- releases
UNIQUE (release_id, user_id, channel)             -- release_deliveries
```

An explicit Checkout-attempt state machine is preferable to guessing from nullable fields. A partial uniqueness constraint on nonterminal admitted attempt states can prevent double clicks; terminalization/expiry must release that slot. If a crashed request's result is uncertain, reconciliation resolves its same Session before admitting a replacement. Test this against PostgreSQL rather than an in-memory map.

There is no separate permanent “entitlements” table in the first release. A policy derives access from account state and subscription projections. This avoids storing a boolean that can disagree with the facts used to compute it. Cached account views may display a computed result, but downloads evaluate the current database records.

## 14. Proposed Go package layout and integration contracts

Keep one top-level `go.mod`. A small domain layout is sufficient:

```text
cmd/francois/cmds/catalog/
  root.go, serve.go, migrate.go, publish.go
  users.go, reconcile.go, deliveries.go
pkg/catalog/
  server.go, routes.go, account.go, downloads.go
  identity/oidc.go, identity/sessions.go
  billing/service.go, billing/stripe.go, billing/policy.go
  releases/service.go, releases/files.go
  notifications/service.go, notifications/mail.go
  store/postgres.go, store/migrations/*.sql
  web/embed.go
ui/catalog/
  main.tsx, app.tsx, api.ts, account.tsx, library.tsx, style.css
```

Every path above is proposed. Existing `pkg/host` and `pkg/store` remain the local runtime implementation. Use standard `net/http` ServeMux and explicit middleware composition. Follow context propagation, `github.com/pkg/errors` wrapping, zerolog, errgroup ownership, and `_ "embed"` conventions already required by this repository.

Define interfaces at genuine external boundaries rather than wrapping every helper:

```go
// Proposed contracts, intentionally abbreviated.
type Principal struct {
    Authenticated bool
    UserID string
    Role string
    Disabled bool
}
type AccessDecision struct { Allowed bool; Reason string; Until time.Time }

type BillingGateway interface {
    EnsureCustomer(ctx context.Context, userID string) (Customer, error)
    CreateCheckout(ctx context.Context, attempt CheckoutAttempt) (Checkout, error)
    CreatePortal(ctx context.Context, customerID string) (Portal, error)
    CurrentSubscriptions(ctx context.Context, customerID string) ([]Subscription, error)
}
type Mailer interface {
    SendRelease(ctx context.Context, message ReleaseMessage) (MessageReceipt, error)
}
```

The actual Stripe SDK belongs in the gateway implementation, while the entitlement policy should consume small domain structs. Tests can provide deterministic gateways without fabricating Stripe HTTP responses for every account-screen test. Verify actual SDK behavior separately in sandbox.

The store owns transactions involving projections, event tokens, and delivery uniqueness. HTTP handlers own decoding, principal requirements, structured errors, and response headers. Domain services own the product operation. The UI owns presentation and redirects to provider-hosted pages. There is no universal plugin system or generic workflow engine.

### Process startup and shutdown

```text
catalog serve:
    decode Glazed config and validate public URL/secret paths
    connect PostgreSQL and verify migration compatibility
    open private release root
    discover configured OIDC provider with bounded startup context
    create Stripe client and mail sender
    construct catalog ServeMux with explicit policies
    start HTTP listener, billing worker, notification worker under errgroup
    on signal: stop accepting requests, cancel workers, drain bounded work,
               close listener/store/root
```

Workers take context arguments, claim work with recoverable leases, and stop on cancellation. No goroutine silently outlives the command. The initial process graph must be testable without Node, Pi config, or a local artifact library database.

## 15. HTTP and frontend API reference

The browser frontend uses React, TypeScript, Redux Toolkit, and RTK Query, with the same clean library styling already established in Francois. Embed its production assets and serve them under `/static/`. Public landing/catalog metadata can be server-rendered or served through the same SPA; do not add a second frontend framework.

All JSON errors use `{ "error": { "code": "...", "message": "..." } }`. Unknown resources use the same JSON 404 shape. HTTP status distinguishes missing authentication (401), authenticated denial (403), malformed requests (400), conflicts (409), bounded admission (429), and temporary backend failure (503).

| Method/path | Authentication/authorization | Contract |
|---|---|---|
| `GET /` | Public | Product explanation and public catalog UI shell. |
| `GET /static/{asset}` | Public | Embedded website assets only, never subscriber HTML. |
| `GET /auth/login` | Public, browser-bound login initiation | Redirect through configured OIDC issuer. |
| `GET /auth/callback` | Valid one-use login flow | Validate identity, provision local user/session, redirect locally. |
| `POST /auth/logout` | Session + CSRF | Revoke local session. |
| `GET /api/session` | Optional session | Identity summary, CSRF token when logged in, computed account access. |
| `GET /api/catalog` | Public | Metadata-only listing, availability labels, pagination. |
| `GET /api/artifacts/{id}/releases` | Public | Published release metadata and requirements; no file bytes. |
| `GET /api/account` | Session | Local account, notification preference, subscription projection. |
| `PATCH /api/account/notifications` | Session + CSRF | Enable/disable notifications subject to verified contact. |
| `POST /api/billing/checkout` | Enabled verified account + CSRF | Logical plan input; URL and durable attempt reference. |
| `POST /api/billing/portal` | Account owner + CSRF, including restricted billing account | Own mapped Customer Portal URL. |
| `GET /billing/return` | Session | Pending/current account UI, never a grant. |
| `POST /api/billing/refresh` | Session + CSRF, bounded | Enqueue owner-scoped reconciliation; never assign access from browser input. |
| `POST /webhooks/stripe` | Stripe signature and expected mode | Durable event ingress; no browser CSRF requirement. |
| `GET /api/releases/{release}/files/{file}/download` | Current file policy | Stream attachment after server authorization. |
| `GET /notifications/unsubscribe` | Opaque signed token | Confirmation page; no preference mutation on GET. |
| `POST /notifications/unsubscribe` | Valid signed token | Disable release notifications. |
| `GET /healthz`, `GET /readyz` | Public operational endpoints | Liveness and local storage/database readiness. |

Do not put administrator mutations in the public API in the first slice unless needed by an implemented operator screen. Operator CLI access is sufficient. If an admin API is introduced, each route needs admin-role plus CSRF checks and cannot be used by an ordinary subscriber.

### Representative JSON shapes

These are proposed domain payloads; IDs and dates are illustrative:

```json
{
  "user": {"id":"usr_example","name":"Maya","emailVerified":true},
  "membership": {
    "access":"subscriber",
    "status":"active",
    "accessUntil":"2026-11-12T00:00:00Z",
    "cancelScheduled":false
  },
  "notifications": {"enabled":true},
  "csrfToken":"opaque-session-bound-token"
}
```

```json
{
  "id":"rel_polis_100",
  "artifactId":"polis",
  "version":"1.0.0",
  "title":"POLIS IDE",
  "requirements":{"runtime":"francois","externalAssets":true},
  "files":[{"id":"file_html","name":"polis-1.0.0.html","size":123456,"sha256":"..."}],
  "downloadAccess":"subscriber_required"
}
```

Metadata does not include source text. `downloadAccess` guides UI presentation; it is not accepted as authorization input. The handler reevaluates access when the user clicks Download.

## 16. Deployment and configuration

The first production deployment needs a public HTTPS URL, a dedicated OIDC web client/callback, a PostgreSQL database, a persistent private release directory, Stripe sandbox/live configuration, and a verified sender. Reuse existing infrastructure patterns from the reviewed vault reports: immutable images, GitOps deployment definitions, Vault-delivered runtime secrets, and backups outside the node.

The proposed Glazed settings are `listen`, `public-url`, `database-url-file`, `release-dir`, `oidc-issuer`, `oidc-client-id`, `oidc-client-secret-file` when the client type requires it, `session-key-file` if the selected SDK needs encrypted flow state, `stripe-secret-key-file`, `stripe-webhook-secret-files`, `stripe-price-id`, `stripe-live-mode`, and email-sender settings. Client IDs, issuer URLs, Price IDs, and directories are nonsecret; secret files must not be emitted in structured command output.

Choose a pinned, compatible Go/toolchain and dependency set before production. The FRANCOIS-001 Step 10 vulnerability scan failed against the existing toolchain/dependencies; this ticket does not reinterpret a passing local functionality test as a clean production security result. Resolve those findings in an explicit implementation/deployment phase and retain evidence.

Local development can use a ZITADEL lab or dedicated development client, a disposable PostgreSQL instance, Stripe CLI forwarding, and a capture-only email sink. Do not connect a test mailer to the real subscriber list. Development insecure-cookie behavior is explicit and unavailable on a public HTTPS deployment.

Snapshot/sandbox webhooks are a practical first integration because they match the inspected v1 billing reference. Configure the event endpoint and SDK API versions together. Do not copy a preview API version shown in a new documentation example without selecting its supported SDK/type contract.

For Stripe, keep sandbox/live Product/Price IDs, API keys, endpoint secrets, and portal configurations separate. Secrets may rotate with bounded current/previous overlap. Endpoint creation returns a signing secret: follow the predecessor's direct-to-Vault operational pattern rather than silently writing it into Terraform state. No live financial resource is created by this guide.

Back up PostgreSQL consistently and the private release directory. Restore to an isolated environment, validate every published file hash, and reconcile billing with Stripe before reopening paid downloads if the projection may be stale. Keep webhooks/notifications paused during restore validation to avoid duplicate side effects.

An operational dashboard can be a small authenticated view or CLI output: oldest unprocessed Stripe event, failed attempts, reconciliation lag, missing release blobs, notification failures, and current account state. These observable counters are more useful than raw webhook payload logs.

## 17. Implementation phases for a new intern

The implementation should proceed as a sequence of working slices. Each phase has a concrete exit condition. Commit the code and full investigation diary at coherent milestones; do not mark later phases complete because this document exists.

### Phase 1: establish the catalog boundary and persistence

Create the `catalog` Glazed command group, configuration validation, PostgreSQL migrations, and a separate HTTP server. Add health/readiness and the embedded minimal UI shell. Ensure `catalog serve` never initializes the local host or model worker. Add a test that starts this graph with no Node executable or Pi files available.

**Exit:** migration runs on an empty database, server serves public metadata/health, command help is correct, and local commands still pass their existing tests. No paid files, identity, or billing are faked as complete.

### Phase 2: SSO, local sessions, and account controls

Provision a dedicated development OIDC web client. Implement code+PKCE login, one-use flow state, local user upsert, sessions, logout, CSRF, and the account page. Add operator list/inspect/disable/session-revoke commands. Preserve disabled/admin state on subsequent login.

**Exit:** two different verified subjects produce different users even when profile email text matches; one subject with an updated email remains the same user; callback replay fails; session expiry/logout work; an ordinary user cannot obtain administrator controls.

### Phase 3: curated release storage and download authorization

Implement explicit publisher, private atomic blob storage, release metadata, screenshots, and attachment-only download handler. Begin with a clearly marked free fixture and a subscriber fixture whose bytes are denied until entitlement exists. Implement the pure policy before billing integration.

**Exit:** unentitled requests never receive paid source bytes through any route; a valid fixture principal can download byte-identical HTML/manifest; metadata and screenshots do not disclose the source; a missing blob returns a controlled error.

### Phase 4: Stripe Customer, Checkout, and Portal

Create the selected sandbox catalog and portal configuration. Implement Customer mapping, durable Checkout attempts, idempotent retry, open-subscription handling, owner-scoped Portal Sessions, and pending-return UI. Add fake gateway tests and actual sandbox Checkout/Portal checks.

**Exit:** two rapid upgrade requests resolve to one attempt/Session, timeout recovery reuses the same key, another user's Customer cannot be selected from browser input, and a successful return alone still does not grant access.

### Phase 5: durable webhook projection and reconciliation

Implement signature verification, mode/version checks, durable inbox, worker lifecycle, current-state retrieval, subscription-ID projections, status policy, period expiry, and reconciliation. Use real signed Stripe CLI delivery and fixture tests for event extraction.

**Exit:** duplicate and out-of-order events converge; old canceled subscriptions do not hide replacements; crash/retry resumes work; initial failure denies; renewal extends access; scheduled cancellation preserves access until end; terminal cancellation removes new-download access. Record the exact chosen past_due/trial policy.

### Phase 6: subscriber notification delivery

Implement preferences, verified-contact checks, recipient generation marker/cursor, unique delivery jobs, rechecks, sender interface, retries, and unsubscribe. Begin with a capture sink; configure SES/SMTP only after the sink acceptance passes.

**Exit:** publish generates one logical delivery per eligible user, crash/retry resumes generation, unsubscribe before send suppresses delivery, inactive/disabled users are skipped, and all links pass through the authenticated download path. Do not claim exactly-once provider delivery.

### Phase 7: integrated customer journey and production readiness

Run full browser acceptance: sign up, subscribe, wait for projection, download, import locally, run the artifact, receive a captured release notice, manage/cancel billing, and verify access changes. Complete known vulnerability maintenance, consistent backups/restore, secret delivery, verified sender configuration, and production OIDC/Stripe settings.

**Exit:** the journey and denial cases are demonstrated with retained sanitized evidence; release files match their recorded hashes; no operator/customer model credential is present in portal responses or release bytes. Distinguish sandbox acceptance from live launch.

### Phase 8: launch and operational handoff

Deploy one catalog instance with its private storage. Configure the public webhook endpoint and sender only for the chosen environment. Perform an approved production acceptance journey, document reconciliation and support runbooks, and record image digest, application commit, API versions, and restore proof.

**Exit:** an operator can inspect a user's access, reconcile a missing webhook, withdraw a release, retry a delivery, rotate an endpoint secret, and restore metadata/files. Pricing, trial/grace, license, tax configuration, and email-policy decisions are explicit rather than embedded in accidental code defaults.

## 18. Test matrix and experiments

Use tests to prove externally observable invariants. A handler returning 403 is insufficient if another endpoint returns the protected bytes. A Checkout success screenshot is insufficient if subscription projection never changes. A send-provider 200 is insufficient proof of inbox delivery.

| Area | Essential cases | Evidence |
|---|---|---|
| Identity | State/binding mismatch, nonce failure, callback replay, expiry, changed email, same email/different subject. | OIDC fixtures plus browser login against the configured development issuer. |
| Sessions | Expired/revoked token, disabled account, profile update preserves roles, logout, CSRF failure. | Database-backed tests and route matrix. |
| Billing attempts | Double click, provider timeout after creation, DB failure after Customer creation, foreign Session/Customer input. | Fake gateway state plus sandbox object counts/IDs. |
| Webhooks | Invalid/raw-body signature, wrong mode/version, duplicate, irrelevant event, DB failure, worker crash, delayed old subscription event. | Signed fixtures, PostgreSQL transactions, Stripe CLI. |
| Entitlement | Exact Price, unknown Price/status, malformed items, active period, trial disabled, past_due denial, period-end and terminal cancellation. | Pure table-driven policy tests. |
| Downloads | Anonymous, free/nonfree, disabled, expired subscription, catalog/source/blob bypass, HEAD/range parity, missing bytes. | Whole-response byte assertions and byte hashes. |
| Publication | Duplicate version, interrupted file write, orphan blob, committed release/hash integrity, accidental injected runtime/secret input. | Filesystem fault fixtures and database tests. |
| Notifications | Unique jobs, partial generation restart, opt-out/cancel before send, invalid email, transient failure, duplicate-after-send ambiguity. | Capture mailer and durable job transitions. |
| Product journey | Login → Checkout → projection → download → local import → renewal/cancel → new-download denial. | Playwright and sandbox billing events with sanitized receipts. |
| Operations | Backup/restore, secret rotation, stale projection, known dependency vulnerabilities. | Runbook rehearsal and exact check results. |

For download-denial tests, put a distinctive marker inside a paid HTML fixture and assert it is absent from every anonymous/catalog/error response. Also assert that authorized downloaded bytes are identical to the stored hash. This directly transfers the useful hyperblog paywall test shape to the new product.

For billing replacement tests, create canceled subscription A and active subscription B for one Customer, process A's delayed event after B, and verify B still authorizes downloads. Store both rows and derive access from current state. This catches the predecessor's single-user-slot assumption before it enters the new service.

Use Stripe Test Clocks where supported by the chosen fixture setup to test renewal, failure, cancellation, and recovery without waiting for real billing periods. The prior ticket's `09-accept-stripe-test-clock.sh` is the first reference. No test uses live cards or a real customer mailing list.

Runtime checks should use tmux for long-lived services, and `lsof-who -p <port> -k` to stop test listeners. Standard checks include Go tests/race where appropriate, build, vet, Glazed lint, TypeScript checks, frontend production build, and an actual rendered browser review. A denied socket in the restricted execution environment is not an application assertion failure; retain the diagnostic and rerun only the affected integration check with authorized access.

## 19. Decisions, limitations, and open owner choices

### Decision: one service with small internal domains

- **Context:** Identity, billing, publication, and email have different external systems but one product.
- **Options considered:** Separate microservices; one catalog process with explicit package and transaction boundaries.
- **Decision:** One process and database, with private files and two durable workers.
- **Rationale:** It is straightforward to deploy and inspect, and avoids infrastructure that contributes no first-release behavior.
- **Consequences:** One-replica capacity and local release storage are explicit initial constraints. Scale requires a measured change.
- **Status:** proposed.

### Decision: entitlement is derived and file bytes are private

- **Context:** UI lock flags and browser redirects are manipulable; Stripe events are asynchronous.
- **Options considered:** Browser-only gates; a stored paid boolean; an application policy over verified current-state projections.
- **Decision:** Server policy on every download, exact accepted catalog IDs, private attachment delivery.
- **Rationale:** It matches observed predecessor responsibilities and makes the paid operation's boundary testable.
- **Consequences:** Access can temporarily remain pending during billing convergence; the UI must explain that state.
- **Status:** proposed.

### Owner decisions to settle before launch

The guide can be implemented with test fixtures before these commercial choices are finalized, but a live launch must resolve them:

1. Subscription amount/currency and whether yearly pricing is included.
2. The trusted ZITADEL issuer/project and allowed self-signup/social-provider policies.
3. Confirmation that the first release sells downloads/notifications, without hosted model calls.
4. Whether active members can download the entire catalog, or only releases during their membership; this guide proposes the entire currently published catalog.
5. Trial and payment-failure grace; this guide proposes no automatic trial and no past_due access.
6. Release notification opt-in and verified sending domain; this guide proposes opt-in messages linking to the website.
7. Commercial license, refund/dispute response, tax settings, retention, and support process.
8. Deployment domain, PostgreSQL/volume ownership, and backup destination.

These are visible assumptions, not reasons to delay the first technical slices. They should not be converted into a configurable policy framework with dozens of unused modes. Implement the chosen first-release contract and retain the tests that define it.

## 20. Reference map and review instructions

### Ticket-owned sources

- `sources/01-prior-stripe-billing-report.md`: predecessor subscription infrastructure, acceptance, and deployment boundaries.
- `sources/02-subscription-product-playbook.md`: combined identity/billing/product design conventions.
- `sources/03-hyperblog-report.md`: byte-withholding paywall and local account/session principles; source checkout not available here.
- `sources/04-fortress-report.md`: identity/authorization responsibilities, without adopting its API-key control plane.
- `sources/05-serve-artifacts-report.md`: curated artifact operations, atomic writes, and stale-index findings.
- `sources/manifest.json`: original vault paths, snapshot hashes, Francois and inspected predecessor Git revisions.
- `working-note/01-subscription-service-investigation-diary.md`: chronological decisions, failed probes, validation, and delivery receipt.

### Existing code to read in order

1. Francois `cmd/francois/cmds/commands.go` and `update.go`: existing command and authoring lifecycle.
2. Francois `pkg/host/server.go` and `pkg/store/store.go`: local owner/origin and immutable authored versions.
3. Prior TODO `cmd/todo-demo/serve.go` and `session_store.go`: OIDC integration and persistence.
4. Prior TODO `internal/billing/stripe.go` and `stripe_test.go`: external gateway contract, Checkout/Portal, current-state projection.
5. Prior TODO `internal/store/postgres/billing.go` and migrations `002_identity_billing_profile.sql`, `003_stripe_processing.sql`: uniqueness, processing-token claims, projection transactions.
6. Prior TODO `internal/app/csrf.go`, `billing.go`, and `billing_test.go`: browser-write and billing-route boundaries.

Absolute predecessor root: `/home/manuel/code/wesen/2026-07-25--zitadel-go-test`. Absolute current root: `/home/manuel/code/wesen/go-go-golems/francois`. All new catalog package paths in Section 14 are proposals, not files that already exist.

### External APIs verified for this design

| Reference | Use in implementation |
|---|---|
| [ZITADEL Go login](https://zitadel.com/docs/examples/login/go) | Supported web login integration and PKCE. |
| [OIDC Core](https://openid.net/specs/openid-connect-core-1_0.html) | Identity claims and ID-token validation. |
| [Stripe Checkout create](https://docs.stripe.com/api/checkout/sessions/create) | Hosted subscription Session parameters. |
| [Stripe idempotency](https://docs.stripe.com/api/idempotent_requests) | Stable retry identity for an external mutation. |
| [Stripe webhooks](https://docs.stripe.com/webhooks) | Signature verification, asynchronous processing, duplicate/order behavior. |
| [Subscription webhooks](https://docs.stripe.com/billing/subscriptions/webhooks) | Event families and subscription lifecycle integration. |
| [Stripe event catalog](https://docs.stripe.com/api/events/types) | Exact event/resource definitions for pinned API fixtures. |
| [Subscription item-period change](https://docs.stripe.com/changelog/basil/2025-03-31/deprecate-subscription-current-period-start-and-end) | Avoid obsolete top-level billing-period examples. |
| [Customer Portal integration](https://docs.stripe.com/customer-management/integrate-customer-portal) | Owner-authenticated portal creation and subsequent webhook handling. |
| [SES deliverability](https://docs.aws.amazon.com/ses/latest/dg/send-email-concepts-deliverability.html) | Verified sender, delivery feedback, and notification operations. |

A reviewer should first check the product boundary: the portal distributes authored bytes and never starts the owner's model runtime. Then review identity/session ownership, Customer correlation, the pure access policy, private download paths, subscription replacement convergence, and recoverable publication/notification work. The architecture is sufficient when those concrete operations compose correctly; adding extra middleware abstractions or infrastructure is not a substitute for those checks.
