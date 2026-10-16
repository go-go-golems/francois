---
Title: RSS and analytics intern architecture and implementation guide
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/analytics.go
      Note: Portal tracking, privacy, report API and cleanup
    - Path: repo://pkg/catalog/analytics_test.go
      Note: Feature integration tests
    - Path: repo://pkg/catalog/feed.go
      Note: Release RSS publication boundary
    - Path: repo://pkg/catalog/store/analytics.go
      Note: Traffic metrics and authoritative ledger aggregation
    - Path: repo://pkg/catalog/store/usage.go
      Note: Credit reservation and token settlement foundation
    - Path: repo://ui/catalog/admin.tsx
      Note: Operator analytics dashboard
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T11:26:00.501377053-04:00
WhatFor: ""
WhenToUse: ""
---

# RSS and analytics: an intern's guide to Francois

This guide describes the initial implementation checkpoint (`0a66114`). The follow-up [Paper designs and implementation guide](02-analytics-paper-designs-and-implementation-mapping.md) supersedes its table-only presentation with graphs, exact traffic filters, page/referrer stacks, a reusable date dialog, and durable model usage by artifact. It specifies the additional schema, APIs, algorithms, and acceptance tests.

The subsequent [minimal public experience and capacity proposal](03-minimal-public-experience-and-operational-capacity-controls.md) records the user's revised direction: keep usage reporting private, hide routine instance management, and protect six model lanes with fairness controls and reversible service pauses. Those additional policy/UI changes are implemented; see the current guide and its linked local rollout receipt.

## 1. What you are building on

Francois turns saved HTML applications into applications people can discover, subscribe to, and run. An **artifact** is a catalog entry such as POLIS or Board. A **release** is one immutable version of that artifact's HTML and capability manifest. An **instance** is a runnable workspace pinned to a release, with its own documents, uploaded assets, members, and model allowance checks. Publishing a new release does not silently upgrade existing instances.

There are two operating modes. The local owner service, started with `francois serve`, uses SQLite and the owner's Pi model configuration. The hosted catalog, started with `francois catalog serve`, uses PostgreSQL, browser sessions, subscriptions, and isolated application origins. This ticket changes the hosted catalog. Understanding that distinction prevents an easy mistake: adding hosted analytics to `pkg/host`, which serves the local product.

The hosted service has a trusted portal, for example `https://slop.boutique`, and a separate origin for every instance, such as `https://i-example.slop.boutique`. Browser origin boundaries keep subscriber HTML away from account cookies and administration APIs. A process may serve several configured portal domains; they share one database. The server accepts only configured hosts and registered instance hostnames. Never derive public feed URLs or analytics site identities directly from an arbitrary incoming Host header.

A subscriber sees public catalog pages rendered by Go templates, signs in through ZITADEL, subscribes through Stripe, and launches an instance. The instance receives a browser implementation of `window.claude`. Its `sample` calls pass through Go to a Node worker running the Pi SDK. Pi owns provider selection and authentication. Francois owns access checks, credit reservation, and durable usage accounting.

This ticket adds a release RSS feed and first-party portal analytics. It keeps the existing journal RSS feed and existing model ledger. The implementation uses Go's HTTP/XML libraries, PostgreSQL, and the existing React/Redux Toolkit Query administration app. There is no additional analytics service, browser SDK, queue, dependency, or process.

### A concrete example

Alice follows a newsletter link to `/apps?utm_source=newsletter&utm_medium=email&utm_campaign=autumn`, opens Board's details, signs in, and runs her Board instance. The portal records document views with a random visitor ID and a short-lived visit ID. A view after sign-in can also name Alice's internal user ID. A model call inside Board records input/output/cache tokens and provider-reported cost in the accounting ledger. These are separate measurements: a model call is not a page view, and a visitor cookie is not an account.

The operator opens `/admin/analytics` to inspect traffic and model usage. A feed reader polls `/apps/feed.xml` to discover published versions. Reading a feed does not create a page view or spend model credits.

```text
                          trusted portal origin
Browser / RSS reader ---> Server.Handler (exact Host routing)
                            |
                            +--> trackTraffic --> portal ServeMux
                            |      |                |
                            |      |                +-- Go storefront templates
                            |      |                +-- RSS handlers
                            |      |                +-- React administration app
                            |      |                +-- guarded JSON APIs
                            |      v
                            |    traffic_views ----------+
                            |                            |
                            |    model_calls ------------+--> analytics SQL
                            |         ^                       --> admin report
                            |         |
Instance browser ----------+--> hosted runtime --> Pi worker --> provider
  isolated origin               access / reserve / settle
```

## 2. Scope and implementation status

The code described below is implemented in this working tree. Production deployment is a separate operator action; this ticket does not migrate or restart production.

Delivered behavior:

- Existing journal RSS remains at `GET /journal/feed.xml`.
- New release RSS is at `GET /apps/feed.xml`, with discovery metadata and a catalog subscription link.
- Portal document views are persisted in a new `traffic_views` table.
- Visitor, visit, and signed-in account counts appear in `/admin/analytics`.
- Traffic breakdowns cover UTC day, path, referrer hostname, UTM source/medium/campaign, device category, browser family, and internal user ID.
- Model breakdowns cover UTC day, actual model (requested model fallback), user, and instance, including calls, failures, charged credits, input/output/cache tokens, and reported provider cost.
- An administrator-only JSON endpoint supports bounded date ranges and portal filtering for traffic.
- DNT/GPC and common bot exclusions, bounded writes, private HTML caching, privacy text, and 90-day traffic retention are included.

This is intentionally a small server-side analytics implementation. It measures successful HTML document loads, including initial loads of account, studio, instance-shell, and admin pages. It does not record React-only history transitions, page dwell time, screen size, geographic location, downloads as conversion events, or cross-site identity. Those are explicit extension points rather than implicit claims about the data. It does not provide a general event ingestion API.

## 3. Source map and how to read the repository

Start with the lifecycle and ownership boundaries, then read the new feature files. Paths are relative to the repository root; function names are stable navigation anchors.

| File | What to look for |
| --- | --- |
| `cmd/francois/cmds/catalog.go` | Hosted command construction and service configuration. |
| `pkg/catalog/server.go:208` | `Server.Handler`: portal versus instance Host dispatch. |
| `pkg/catalog/server.go:156` | `New`: store/backend/runtime dependencies and portal middleware wiring. |
| `pkg/catalog/routes.go:21` | Portal `http.ServeMux`, storefront, feeds, static assets, account APIs. |
| `pkg/catalog/pages.go:130` | `render`: settings, session-aware navigation, templates, cache headers. |
| `pkg/catalog/pages.go` | `journalFeed`, `rssDoc`, `rssItem`, and `rssGUID`. |
| `pkg/catalog/feed.go` | `releaseFeed` visibility query and shared `writeRSS`. |
| `pkg/catalog/auth.go:102` | `guard`: roles, token scopes, session and mutation protections. |
| `pkg/catalog/analytics.go` | Tracking policy, cookie IDs, response wrapper, analytics API, cleanup loop. |
| `pkg/catalog/store/analytics.go` | Event insertion, metric DTOs, aggregate SQL, retention deletion. |
| `pkg/catalog/store/migrations/009_analytics.sql` | New traffic table and timestamp indexes. |
| `pkg/catalog/store/store.go:52` | Transactional, checksum-verified migration runner. |
| `pkg/catalog/admin.go:25` | Admin route registration, including analytics. |
| `pkg/catalog/hosted/sample.go:79` | Runtime model admission and request handling. |
| `pkg/catalog/hosted/sample.go:190` | Provider execution, accumulated usage, durable settlement. |
| `pkg/catalog/store/usage.go:54` | Atomic `Reserve`, then `StartCall` and `SettleCall`. |
| `pkg/piworker/backend.go:74` | Provider usage contract and cost limitations. |
| `ui/catalog/api.ts` | RTK Query `analytics` endpoint and TypeScript response shape. |
| `ui/catalog/admin.tsx` | Analytics navigation, date/site controls, metric tiles and tables. |
| `ui/catalog/store.ts` | Client-side navigation; explains which transitions do not reach Go. |
| `pkg/catalog/web/templates/layout.tmpl` | RSS discovery links shared by storefront pages. |
| `pkg/catalog/web/templates/catalog.tmpl` | Human-visible release subscription link. |
| `pkg/catalog/web/legal.tmpl` | Updated description of analytics cookies and records. |
| `scripts/build.mjs` | Rebuilds embedded portal JavaScript from TypeScript. |
| `pkg/catalog/analytics_test.go` | Policy, RSS serialization, database/reporting, access, retention tests. |

`pkg/catalog/web/static/catalog.js` is generated and tracked because Go embeds it. Editing `admin.tsx` without running `pnpm build` changes source but leaves the served administration app unchanged. The embedding path and URL path are different concepts: browser assets are served under `/static/`.

## 4. Investigation: what already existed and what was missing

### Existing RSS

The README already advertised journal RSS, and investigation confirmed the route, shared layout discovery link, journal subscription link, and publication-state test. `journalFeed` queries up to 50 published posts and uses the configured request site's portal origin. It formats publication dates and serializes XML. Replacing this with another journal feed would duplicate working behavior.

The missing catalog capability was a release feed. The new route lives alongside the existing journal feed and reuses the XML envelope/serializer. It emits one item per release, because subscribers need to distinguish version 1.0 from version 1.1 even when they link to the same artifact page.

### Existing model accounting

`model_calls` already records requested and actual model, user, instance, lifecycle state, credits, tokens, cache reads/writes, rounds, error code, settings revision, and timestamps. `SettleCall` writes provider usage when converting a started reservation into charged usage. The browser does not supply authoritative token counts.

The older admin usage summary uses the allowance period key. Normal calls use a UTC month, while demos use `demo:<instance>` so demo budgets remain separate. A report filtered by the normal monthly period can therefore omit demos. The new analytical report filters `model_calls.created_at` by timestamps and includes every ledger period. Existing allowance and billing behavior is unchanged.

### Missing traffic measurement

There was no durable portal page-view table or traffic dashboard. Active runtime views measure currently connected application views, not historical visits. Session records describe authentication, not anonymous visitors. Neither is an accurate substitute for traffic events.

The code also had no trusted site field on model accounting rows. We therefore cannot honestly attribute historical model costs to a portal domain. The traffic `site` selector filters traffic only; the dashboard explicitly identifies model reporting as catalog-wide.

## 5. RSS contract and publication boundary

The feed reader performs an ordinary public GET. The server returns RSS 2.0 XML with `Content-Type: application/rss+xml; charset=utf-8` and a five-minute public cache lifetime. It needs no account and must never resolve private release files or draft previews.

The release query joins `releases` to `catalog_artifacts` and selects rows only when the release state is `published` and artifact visibility is `listed`. SQL sorts by publication timestamp descending, with release ID as the tie breaker, then limits to 50 items. This bounds response size independently of total catalog history.

A release item contains:

| RSS field | Source and meaning |
| --- | --- |
| `title` | Artifact title plus release version. |
| `link` | Configured portal origin plus `/apps/<escaped artifact ID>`. |
| `guid` | `urn:francois:release:<release ID>`, with `isPermaLink="false"`. |
| `pubDate` | Publication timestamp, UTC, RFC1123Z formatting. |
| `description` | Release changelog text, XML-escaped by `encoding/xml`. |

The opaque release GUID survives title edits and alternate portal domains. It is not a navigable URL, so its permalink attribute must be false. The journal retains its URL GUID and declares that it is a permalink. RSS readers use GUIDs to recognize items; changing a GUID merely because wording changes produces duplicate notifications. These choices follow the [RSS specification](https://www.rssboard.org/rss-specification), including the channel and GUID contracts.

```text
GET /apps/feed.xml
  choose configured portal site from request context
  SELECT newest 50 published releases of listed artifacts
  for each row:
    create item with stable release GUID
    construct link from configured origin
  marshal complete document before writing response
  write XML header, RSS bytes, public cache headers
```

`writeRSS` marshals before writing the response so serialization failure can still return HTTP 503. Database/scan/iteration failures also return 503. Withdrawal or hiding removes an item from future feed responses after cache expiry; it cannot retract copies already saved by readers. The implementation does not add conditional ETag/Last-Modified responses, enclosures, or a historical pagination API.

## 6. Traffic event lifecycle

### Eligibility

`trackTraffic` wraps the portal handler after the dispatcher has selected a configured site. It does not wrap instance traffic. Before touching cookies or storage it excludes non-GET requests, known bot strings, DNT (`DNT: 1`), GPC (`Sec-GPC: 1`), and paths for APIs, authentication, static/media/font assets, webhooks, health probes, or XML feeds.

The wrapper lets the existing route handler run and captures the response status. It records an event only for HTTP 200 responses with an HTML content type. Thus a missing artifact, a redirect, a JSON response, or a failed database-backed storefront request is not a successful page view. This deliberately does not produce an HTTP error-rate metric; use server logs for errors.

The response wrapper implements `http.ResponseWriter` explicitly and exposes `Unwrap`. That keeps the standard [ResponseController](https://pkg.go.dev/net/http#ResponseController) mechanism available to handlers. Excluded API routes bypass the wrapper entirely, including streaming/MCP traffic.

### Identity

Two opaque cookie values identify browser activity:

- `francois_visitor`: 128 random bits, encoded as 32 hexadecimal characters; expires 90 days after issuance. It is not renewed on every view.
- `francois_visit`: another random value with a sliding 30-minute expiry, refreshed on eligible requests.

HTTPS adds the `__Host-` prefix, uses Secure, leaves Domain unset, and sets Path `/`. Both cookies are HttpOnly and SameSite=Lax using the existing `setCookie` helper. The server accepts existing values only when their length and hex encoding are valid. The identifiers are measurement hints, not authentication credentials or authorization inputs.

A separate lookup of the existing signed-in session can attach `user_id`. No email, display name, auth cookie value, bearer token, or OIDC code is copied into `traffic_views`. Anonymous history is not retroactively rewritten when a visitor signs in. User reports group only the views that were observed as authenticated.

Cookies are created before the downstream response commits headers; consequently an eligible path that ultimately returns an error can still issue or refresh cookies, although no view is inserted. DNT/GPC and explicitly excluded requests bypass cookie handling altogether. Concurrent first requests without cookies can receive different IDs; strict browser session reconstruction would need a more elaborate client protocol.

### Data minimization and write behavior

The stored event includes the configured portal origin, URL path without query or fragment, visitor and visit IDs, optional internal user ID, referrer hostname only, three explicitly selected campaign values, and broad device/browser labels. String dimensions are bounded to 200 Unicode characters. Campaign values are untrusted strings; React renders them as text, and SQL receives them as parameters.

For example, a referrer of `https://search.example/private?token=secret` becomes `search.example`. An incoming request's unrelated `token=secret` query is not recorded. UTM fields themselves can contain sensitive text if an author puts it there; callers should use campaign labels rather than personal information.

Once the handler completes, tracking uses `context.WithoutCancel` plus a 250 ms timeout. This lets a browser cancellation avoid immediately discarding an otherwise completed view, but bounds the combined identity lookup and insert. A storage error is logged and drops that view; it does not replace the successful page with an error. There is no per-view goroutine, channel, or retry queue. A slow database can still add up to roughly the timeout to handler completion, so this is an intentional low-volume tradeoff.

Tracked HTML receives `Cache-Control: private, no-store` before headers commit. This prevents a shared cache from replaying another visitor's Set-Cookie response. Non-tracked storefront rendering retains its existing cache policy. Feed caching remains public and independent.

```text
GET portal document
  eligibility / bot / privacy checks
  read or issue visitor cookie
  read or issue visit cookie; refresh its expiry
  run existing handler through status-capturing response writer
  if status != 200 or content type != HTML: stop
  with detached, 250 ms bounded context:
    resolve authenticated user if possible
    normalize path, referrer, campaign, browser/device
    insert one traffic_views row
    on failure: log and finish without changing the response
```

## 7. Database design and metric definitions

Migration 009 creates one event table with an identity primary key and a server timestamp. Events have text dimensions and no foreign key to users: deleting an account does not block traffic cleanup, and anonymous events use an empty user ID. Account erasure must therefore explicitly clear matching analytics user IDs or delete those rows as part of any future account-deletion workflow. Visitor/session IDs remain pseudonymous data; avoiding IP storage does not make the dataset anonymous.

The migration adds `traffic_views(created_at)` and `model_calls(created_at)` indexes. Time is the common filter across all report sections. There are no indexes on every dimension because inserts are more frequent than report reads and this initial deployment does not justify that write amplification. Measure real queries with EXPLAIN before adding more indexes.

`Store.Migrate` embeds SQL files, sorts their names, holds an advisory transaction lock, checks prior checksums, and applies pending migrations atomically. Never edit an already applied SQL migration; make a new numbered migration. The existing `Pending` startup check prevents serving with an incompatible schema.

| Metric | Exact definition |
| --- | --- |
| Views | Count of recorded successful portal HTML document loads in the range. |
| Visitors | Distinct `(site, visitor)` pairs in the range. Not people, devices, or accounts. |
| Sessions | Distinct `(site, session)` pairs in the range. Browser expiry approximates inactivity. |
| Users | Distinct nonempty authenticated `user_id` values in the range. |
| Bounces | Sessions with exactly one recorded view **inside the selected range**. |
| Single-page-session percentage | `100 * bounces / sessions`, or no value when sessions is zero. |
| Group views | Count within that group, with at most 100 groups returned. |
| Group visitors | Distinct `(site, visitor)` pairs within the group. |

Including site in visitor/session uniqueness avoids merging a coincidentally identical or deliberately copied cookie across different domains. Counting visitors across all domains can still count one person multiple times; cookies are intentionally host-only. Distinct visitor counts are not additive across pages: one visitor may contribute to many page groups.

The bounce metric is range-local, not a completed-session calculation. A session with one view before the range and another inside it can appear to bounce. A still-active session may also later gain more views. Interpret it as single-page sessions observed in this window, as the UI label says.

Empty group keys mean unspecified/anonymous/direct data depending on the dimension. Referrer reporting includes internal hostnames as well as external hostnames. Browser families use a short ordered substring matcher; it is intentionally approximate, and unrecognized clients are `other`. A bot can spoof a browser; bot filtering is not abuse prevention.

Campaign tags belong to the individual request carrying them. The implementation does not persist first-touch/last-touch attribution across later pages or assign later model spend to a campaign. This keeps measurement truthful and avoids inventing an attribution model.

Cleanup runs at service startup and hourly under the service errgroup, with a 30-second database timeout, deleting traffic older than 90 days. It does not delete billing/model rows. Failures log and retry next hour. Retention therefore depends on a running service and successful cleanup, and expired data can remain during outages or in backups subject to the backup lifecycle. A future high-volume table may need batched deletes or partitioning.

## 8. Model usage: why the ledger remains authoritative

The existing runtime validates the model against settings, checks access and concurrency, reserves credits transactionally, then starts the provider attempt. Usage events accumulate input, output, cache-read, cache-write, cost, and rounds. The final settlement converts reserved credits into used credits exactly once.

```text
reserved --not started / cancellation--> released (no charge)
    |
    +-- StartCall --> started --> completed (charged)
                         |----> failed    (charged)
                         +----> review    (restart recovery, charged)
```

A started call can cost money even if the browser cancels or the provider fails. Credit policy is therefore not equivalent to counting successful responses. `SettleCall` locks the row, moves the allowance counters, and ignores already settled calls. `RecoverCalls` releases unstarted calls and flags started ones for review after process restart.

The analytics query counts all ledger rows as calls, counts `failed` rows separately, and sums credits only for `completed`, `failed`, or `review`. It sums recorded usage fields, including cache tokens, without guessing missing usage. Model grouping chooses `model_used` when available and otherwise the requested model. Demo rows participate because filtering uses creation timestamps, not allowance-period names.

Provider cost is the existing worker-reported floating-point value. Some OAuth/subscription providers report zero or estimates. It is not a Stripe invoice or an exact accounting currency conversion. Input/output/cache categories should be displayed separately; blindly adding all categories can double-count provider-specific notions of input. Grouping a multi-round call by its final reported model is not a per-round allocation if a backend changes models during execution.

A call is assigned to the UTC day it was created, even if settlement occurs the next day. Historical report results can change when an in-flight call settles. `review` rows may have incomplete usage after a crash. Do not “repair” unknown token counts by inserting estimates into the allowance ledger.

## 9. HTTP and UI API reference

### Feed endpoints

```http
GET /journal/feed.xml
GET /apps/feed.xml
```

Both are public, bounded to 50 items, and return RSS XML. Go's GET route matching also supports HEAD; the HTTP server suppresses the body for HEAD. Feed requests are excluded from traffic measurement.

### Analytics endpoint

```http
GET /api/admin/analytics?from=2026-10-01&to=2026-10-17&site=https%3A%2F%2Fslop.boutique
```

`from` is inclusive midnight UTC and `to` is exclusive midnight UTC. Omitted dates default to a 30-day window ending at the next UTC midnight. The range must be positive and at most 90 days. Dates use `YYYY-MM-DD`. Omitted or empty `site` means all portals; a nonempty site is an exact origin filter. An unknown site yields empty traffic rather than changing the model query. The server applies a five-second timeout across all report queries.

An existing browser admin session or an admin-owned personal token with `admin:read` is required. The route uses the existing `guard(admin, scopeAdmin, ...)` registration; scope cannot elevate a subscriber into an administrator. No user-controlled field becomes a SQL expression. Grouping expressions come from a fixed list in Go; dates/site are bound parameters. [PostgreSQL aggregate semantics](https://www.postgresql.org/docs/current/functions-aggregate.html) explain `count`, `sum`, and the need for zero defaults on empty sets.

Successful response structure:

```json
{
  "from": "2026-10-01T00:00:00Z",
  "to": "2026-10-17T00:00:00Z",
  "site": "https://slop.boutique",
  "traffic": {"views": 120, "visitors": 40, "sessions": 55, "users": 12, "bounces": 18},
  "groups": {
    "pages": [{"key": "/apps", "views": 60, "visitors": 32}],
    "referrers": [{"key": "search.example", "views": 25, "visitors": 20}]
  },
  "models": {
    "models": [{"key": "provider/model", "calls": 20, "failed": 1,
      "credits": 40, "inputTokens": 10000, "outputTokens": 2000,
      "cacheReadTokens": 500, "cacheWriteTokens": 100, "cost": 0.15}]
  }
}
```

This example abbreviates the group maps. Actual traffic keys are `daily`, `pages`, `referrers`, `sources`, `mediums`, `campaigns`, `devices`, `browsers`, `users`. Actual model keys are `daily`, `models`, `users`, `instances`. Each table returns at most 100 groups, sorted by call/view count descending and then key. Missing days are omitted, not zero-filled. Traffic totals are independent of the table limits. Queries run separately, so concurrent activity can produce small differences between sections; this is a live operational report, not a transactionally frozen export.

Errors use the existing `{"error":{"code":"...","message":"..."}}` convention: 400 `invalid_range`, authentication/authorization denial from the guard, and 503 `unavailable` for database/query failures. Responses are non-cacheable through `httpx.OK`. The existing `/api/admin/usage` remains the operational call list and allowance-oriented summary.

The React app adds an Analytics tab with date inputs and an optional origin filter. RTK Query fetches the report; typed `Analytics` response fields drive tiles and traffic/model tables. The UI explains global model scope, exclusions, range-local sessions, and row limits. User and instance IDs can be cross-referenced through existing Users/Instances administration pages. No new Go client/CLI/MCP wrapper is added in this ticket; API consumers can call the HTTP endpoint with existing admin authentication.

## 10. Design decisions

### Decision: retain the existing feed and add a release feed

- **Context:** Journal RSS already worked; the catalog needed release syndication.
- **Options:** Replace it, combine unrelated content, or add a second focused feed.
- **Decision:** Keep journal RSS, add `/apps/feed.xml`, share XML serialization.
- **Rationale:** Clear subscriptions, small change, existing publication boundaries.
- **Consequences:** Two discoverable feeds; readers choose editorial or release updates.
- **Status:** accepted and implemented.

### Decision: first-party server document measurement

- **Context:** Need durable basic traffic analytics without adding operational services.
- **Options:** Hosted third-party tool, browser event SDK, or server middleware.
- **Decision:** Synchronous bounded server insertion with existing PostgreSQL.
- **Rationale:** Existing routes know configured site and successful responses; no new ingestion/auth surface.
- **Consequences:** No SPA-only transitions or dwell time; one DB insertion per eligible view; approximate browser classification.
- **Status:** accepted and implemented.

### Decision: separate traffic events from model accounting

- **Context:** Accounting already enforces allowances and settles provider usage.
- **Options:** Duplicate token events, change accounting writes, or query existing ledger.
- **Decision:** Read existing ledger and keep its lifecycle unchanged.
- **Rationale:** Reporting failures must not change credits; timestamp filters include demos.
- **Consequences:** No historical site/campaign cost attribution and no invented missing usage.
- **Status:** accepted and implemented.

### Decision: transparent approximations rather than extra infrastructure

- **Context:** A small catalog does not yet need a warehouse or analytics session processor.
- **Options:** Session materialization, rollups/queues, or direct event aggregation.
- **Decision:** Retained raw views plus bounded time-range SQL and explicit metric definitions.
- **Rationale:** Easy for a new engineer to trace, test, and operate.
- **Consequences:** Repeated scans and range-local bounces; measure scale before adding complexity.
- **Status:** accepted and implemented.

## 11. Implementation and rollout playbook

The following phases explain the actual dependency order and provide a template for future work.

1. **Investigate ownership.** Read `server.go`, `routes.go`, `auth.go`, `pages.go`, and `usage.go`. Confirm the two service modes, configured sites, publication states, and authoritative model data before changing anything.
2. **Add persistence.** Introduce migration 009 and `RecordView`/`Analytics`/`PruneTraffic`. Use bound parameters and fixed grouping expressions. Keep traffic retention separate from allowance retention.
3. **Wire measurement.** Wrap only the trusted portal, preserve host context, exclude credentials/assets/feeds, issue host-only cookies, capture successful HTML responses, and use a bounded context for best-effort writes.
4. **Expose reporting.** Register the GET endpoint with the standard admin guard. Validate dates before SQL. Add the RTK Query contract and UI without a parallel fetch abstraction.
5. **Add syndication.** Reuse the existing RSS model, implement the release visibility query, stable GUIDs, configured-domain links, and discovery/subscription links.
6. **Validate behavior.** Use unit tests for parsing/policy and PostgreSQL tests for aggregates and publication/access boundaries. Rebuild embedded JavaScript and verify lint.
7. **Deploy deliberately.** Back up the database using the established production procedure. Apply `francois catalog migrate` with the normal explicit database configuration, then restart/roll out the catalog binary and embedded frontend together. Startup will reject pending migrations.
8. **Observe.** Load the catalog and pricing page in an ordinary browser, then inspect Analytics with an administrator account. Poll both feeds and inspect links on every configured portal. Check logs for `record traffic view` and `prune traffic` failures.

Do not delete or rebuild the existing database to install analytics. Dropping the traffic table is not a safe application rollback: a running binary expects it. Reverting the application binary can leave the additional table/indexes in place; only remove schema later with an explicit migration and reviewed data-loss plan.

For local servers follow repository policy: use tmux and capture-pane, and stop a listener with `lsof-who -p <port> -k`. No production rollout was performed during this ticket. The tests use their own httptest listeners and isolated databases, rather than a manually started application server.

## 12. Validation and intern exercises

The new tests cover actual boundaries rather than mirroring helper implementation:

- URL exclusions, known bots, mobile/browser labels, Unicode bounds, and referrer secret removal.
- XML round-trip with `<`/`&` and non-permalink release GUIDs.
- Same-browser cookie reuse produces two views but one visitor and session.
- Authenticated page views name the internal subscriber; other portals are independently filtered.
- DNT/GPC, bots, HEAD, 404s, and feeds do not increase view counts.
- A settled demo-ledger call contributes credits/cache tokens to timestamp-based model analytics.
- Anonymous/subscriber callers cannot read the admin report; malformed/inverted/oversized ranges fail.
- Retention deletes old traffic rows.
- Draft releases and hidden artifacts stay out of RSS; links follow the selected alternate portal.

The existing storefront test's cache expectation changes to `private, no-store` for tracked browser-like requests. That assertion matters: visitor cookies must not leak through shared caches.

Verification commands:

```sh
pnpm check
pnpm test
pnpm build
GOCACHE=/tmp/francois-go-cache go build ./...
GOCACHE=/tmp/francois-go-cache \
  TEST_DATABASE_URL='postgres://francois:francois@127.0.0.1:5435/postgres?sslmode=disable' \
  go test ./...
GOCACHE=/tmp/francois-go-cache \
  GOLANGCI_LINT_CACHE=/tmp/francois-lint-cache \
  golangci-lint run ./pkg/catalog/...
git diff --check
```

The database URL above is the local compose development database, not a production credential. The existing harness reads `TEST_DATABASE_URL` and creates/drops a fresh `catalog_test_*` database per test. The session explicitly notified the user before using that environment-driven harness. A sandbox that denies listening sockets or local database connections needs the authorized execution path; skipping database tests is not equivalent to passing them.

Observed validation: the full Go suite passed against local PostgreSQL after correcting two new fixture mistakes; catalog lint passed with zero issues; TypeScript check, browser contract tests (5 tests), embedded build, and Go build passed. Node is v20 in this environment although package metadata requires v24 or later; the commands succeeded, but supported-version CI remains the deployment baseline. No browser visual inspection or live production test was performed. An existing unrelated `TestConcurrentNewArtifactUpload` failed once with a `version_exists` race response and passed on rerun; it was not modified.

Suggested intern exercises:

1. Explain why a session cookie cannot identify an anonymous visitor and why a visitor ID cannot authorize an API call.
2. Trace a feed item from SQL through `rssItem` to the XML response. Show why a draft cannot leak.
3. Compute views/visitors/sessions for one browser visiting three pages, then deleting cookies and visiting again.
4. Explain why a month-key usage report misses demo periods and why creation-time filtering fixes that report.
5. Run an empty-range report and inspect zero totals and empty groups. Demonstrate why distinct page visitors cannot simply be summed.
6. Create an opted-out request and show that it neither issues analytics cookies nor writes a view.
7. Read a `review` model-call row and describe which values are known and which cannot be reconstructed after a crash.

## 13. Limits, operating risks, and next extensions

The first slice is complete for the server document measurement described here. “Standard analytics” is a broad product category; these boundaries are deliberate and should remain visible to operators.

- **SPA navigation:** `navigate` in `ui/catalog/store.ts` changes history without requesting a new document. If product needs those views, add a small same-origin, bounded collector with an explicit route-event contract and avoid counting initial loads twice. It needs bot/privacy checks and abuse limits; do not reuse a model endpoint.
- **Engagement and conversions:** Time-on-page, funnels, download/demo/signup/checkout events, and revenue attribution need an event taxonomy and deduplication semantics. Existing billing state is authoritative for successful payment; a browser click is not a paid conversion.
- **Returning visitors:** Current reports count distinct visitors within the range; there is no persistent first-seen visitor profile. A returning/new split needs a defined lookback and retention policy before it can be accurate.
- **Geography and OS:** No IP collection/geolocation and no OS parser were added. Browser/device classification is coarse, and bots can evade it.
- **Attribution:** Referrers may be absent because browsers or source sites suppress them. Internal referrals appear as internal hostnames. UTM values apply only to the tagged view; cost is never inferred from them.
- **Availability:** Best-effort writes can undercount during database outages. No queue means there is nothing to replay. Do not treat this as financial/audit-grade event capture.
- **Query scale:** Reports perform one totals query plus nine traffic and four model grouping queries; each table is capped but scan cost is proportional to the selected retained data. All queries share a five-second timeout. Add measured indexes/rollups only when real workloads justify them.
- **Privacy operations:** Updated policy describes the implemented data, but the ticket does not supply a jurisdiction-specific legal assessment or consent manager. Honor existing privacy signals, avoid personal UTM values, and include traffic data in account-data handling procedures.
- **Billing scope:** Site filtering does not filter model spend. Per-site or per-campaign costs require new authoritative metadata at model admission and cannot be backfilled reliably from current records.
- **Presentation:** Tables are intentionally simple, no charts/CSV export/live polling. Daily rows are returned in volume order; consumers needing a chronological chart should sort and zero-fill explicitly.

## 14. References

Repository references are listed in Section 3; begin there for implementation review. Related background tickets are FRANCOIS-002 (subscriber platform and accounting), FRANCOIS-004 (storefront/journal), and FRANCOIS-007 (HTTP client, personal tokens, and MCP).

External primary references consulted on 16 October 2026:

- [RSS 2.0 specification](https://www.rssboard.org/rss-specification): channel/item fields, dates, stable GUID and permalink semantics.
- [Go net/http documentation](https://pkg.go.dev/net/http): `Handler`, `ResponseWriter`, `ResponseController`, cookies, GET/HEAD ServeMux matching.
- [PostgreSQL aggregate functions](https://www.postgresql.org/docs/current/functions-aggregate.html): counts, sums, null results on empty sets; the implementation uses broadly supported PostgreSQL SQL features.

For exact commands, failures, and delivery evidence see the ticket's `reference/01-implementation-diary.md` and changelog. The design explains final behavior; the diary preserves how it was established.
