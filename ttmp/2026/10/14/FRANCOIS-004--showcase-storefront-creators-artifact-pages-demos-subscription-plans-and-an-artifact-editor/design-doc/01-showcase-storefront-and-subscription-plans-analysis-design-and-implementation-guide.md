---
Title: 'Showcase storefront and subscription plans: analysis, design and implementation guide'
Ticket: FRANCOIS-004
Status: active
Topics:
    - backend
    - frontend
    - billing
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: pkg/catalog/server.go
      Note: Site dispatch and the single authorizeInstance rule that plans replace
    - Path: pkg/catalog/billing/policy.go
      Note: Pure access policy; gains the price of the standing subscription
    - Path: pkg/catalog/settings/settings.go
      Note: Versioned settings document; gains plans, demo and storefront blocks
    - Path: pkg/catalog/releases/releases.go
      Note: Validation and content-addressed blob storage reused for media
    - Path: pkg/catalog/admin.go
      Note: Current release upload/check/preview/publish API
    - Path: pkg/catalog/hosted/sample.go
      Note: Credit reservation that becomes plan-aware
    - Path: ui/catalog/admin.tsx
      Note: Current publish form the studio editor replaces
ExternalSources: []
Summary: "Intern-oriented analysis and design for turning the hosted Francois catalog into a storefront for two creators: public artifact pages with write-ups and media, creators, a journal, visitor demos, subscription plans with entitlements, a studio editor, creator role, API tokens and a push CLI."
LastUpdated: 2026-10-14T19:30:00-04:00
WhatFor: "Understand the hosted catalog and implement the storefront and plans in phases."
WhenToUse: "Read before changing catalog content, public pages, entitlements, billing plans or the studio UI."
---

# Showcase storefront and subscription plans: analysis, design and implementation guide

## 1. Executive summary

Francois's hosted catalog (`francois catalog serve`) runs single-file HTML applications ("artifacts") for subscribers, each in its own isolated browser origin, with a model runtime paid for by the operator. Today it is a closed beta: one Stripe price grants all-or-nothing access, sign-up is invite-only, a visitor who is not signed in sees a bare list of titles, and the only content an artifact carries is a title, a short description and tags.

The owners want a storefront for two creators to showcase and sell their artifact work as **subscriptions to live artifacts**. This document designs the changes in eight phases:

1. **Content model.** Creators, a long Markdown write-up per artifact with a draft and a published copy, a media library, and journal posts.
2. **Public pages.** Server-rendered, indexable pages: front page, catalog, artifact pages, creator pages, journal with RSS, and pricing. A small script adds the buttons that need a session.
3. **Studio API and creator role.** A content API for people with the new `creator` role, scoped to their own artifacts; the release pipeline moves here from the admin API.
4. **Studio UI.** An artifact editor with Details, Write-up, Media, Releases and Demo tabs, a page preview, a journal editor, creator profiles and a media library.
5. **Plans and entitlements.** Plans defined in the versioned settings document. One resolver answers "what may this user do?" for instance creation, launch, sharing, downloads and model calls. Checkout takes a plan, and there is a pricing page.
6. **Visitor demos.** Short-lived instances for people without an account, with a separate small model budget, a global cap and a per-address rate limit.
7. **API tokens and `francois catalog push`.** Creators publish from their own machines without `kubectl exec`.
8. **Documentation and rollout.** The production playbook, the Kubernetes arguments and the order of the first rollout.

Multi-file artifacts (a zip upload with images, fonts and scripts) are designed only as a direction in §13; they are not implemented in this ticket.

## 2. Problem statement and scope

### 2.1 What the owners asked for

- Showcase and sell artifact design work made by two people.
- A write-up ("blog post") for each artifact, plus general posts.
- Multiple subscription tiers. The product is **ongoing access to live artifacts**, not one-off purchases, downloads or commissions.
- A real authoring workflow: upload, configure, preview, publish.

### 2.2 In scope

Everything in phases 1–8 above, implemented in this repository with tests, plus the production rollout notes.

### 2.3 Out of scope

- Revenue splitting between creators (Stripe Connect). Per-creator reporting is a possible follow-up.
- Live-mode Stripe, tax configuration and legal text changes. These are business decisions and production configuration.
- The ZITADEL login branding (a separate Paper design exists for it).
- Multi-file artifacts beyond the direction in §13.

## 3. Orientation: how the hosted catalog works

Read this section first. Everything later depends on these terms.

### 3.1 Terms

- **Portal origin.** The trusted website, such as `https://francois.yolo.scapegoat.dev`. It serves the catalog, account, admin pages and their JSON API. Sessions are cookies on this origin.
- **Site.** One portal origin plus the domain its instances live under. One process can serve several sites (production serves `francois.yolo.scapegoat.dev` and `slop.boutique`); they share one database, so users, artifacts and instances are the same on every site. `Site` is defined in `pkg/catalog/server.go:32`.
- **Artifact.** A catalog entry: an ID such as `polis`, a title, a description, tags and a visibility (`listed` or `hidden`). Table `catalog_artifacts`.
- **Release.** One immutable version of an artifact's HTML plus its capability manifest. States: `draft` → `published` → optionally `withdrawn`. Tables `releases` and `release_files`. The files themselves are content-addressed blobs on disk.
- **Capability manifest.** A JSON object naming the browser APIs an artifact may use: `sample` (model calls), `db` (documents), `room` (presence), `user`, `assets`, `comments`, `downloads`.
- **Instance.** A running copy of a release that belongs to a user, with its own data (documents, assets, comments). It is served on its own origin `i-<id>.<app domain>`, so one instance can never read another's browser storage or the portal's cookies. Kinds today: `workspace` (a subscriber's) and `preview` (an admin's test run of a draft).
- **Launch.** The handshake that lets an instance origin act for a signed-in user without sharing the portal cookie: the portal issues a one-use **launch code** bound to the session and a PKCE challenge, the instance page exchanges it for a short-lived **runtime grant** (a bearer token), and every runtime request carries the grant.
- **Access decision.** The answer to "may this user use hosted features now?", computed from Stripe subscription projections and complimentary grants by the pure function `billing.Policy.Access` (`pkg/catalog/billing/policy.go:65`).
- **Settings revision.** A versioned JSON document holding non-secret policy: approved models and their credit costs, allowances, limits. Each save is a new row in `platform_settings`; the newest is active. Defined in `pkg/catalog/settings/settings.go`.
- **Credits.** The unit of model usage. Each approved model costs a number of credits per call; a user has a monthly and a daily credit allowance. Reservation is atomic per user and period (`pkg/catalog/store/usage.go:43`).

### 3.2 The parts and how they connect

```text
                 Browser
        ┌──────────────┴──────────────────────────┐
        │ portal origin                           │ instance origin i-<id>.<app domain>
        │ francois.yolo.scapegoat.dev             │ (one per instance)
        ▼                                         ▼
  ┌───────────────────────────── one Go process: francois catalog serve ─────────────┐
  │ Server.Handler (server.go:190) dispatches by Host                                │
  │   portal host  → routes()  : SPA shell, /api/*, /auth/*, /privacy, /terms        │
  │   www.portal   → 308 to portal                                                   │
  │   i-x.<domain> → hosted.Runtime.Handler : /launch, /page, /api/views/*, assets   │
  │                                                                                  │
  │  store (PostgreSQL)   releases.Blobs (disk)   piworker (Node model worker)      │
  └──────────────────────────────────────────────────────────────────────────────────┘
        │                       │                          │
   PostgreSQL 16          PVC: releases/blobs        LunaRoute gateway
```

The portal's UI is a React single-page application (SPA) built from `ui/catalog/*.tsx` into `pkg/catalog/web/static/catalog.js` and embedded in the binary (`pkg/catalog/web/embed.go`). The server returns the same `index.html` shell for every SPA route (`pkg/catalog/routes.go:26`) and the SPA picks the page from the URL (`ui/catalog/main.tsx`).

### 3.3 One request followed end to end: Alice runs Polis

1. Alice opens `/apps/polis`. The SPA fetches `GET /api/artifacts/polis` (public) and `GET /api/session`.
2. She clicks **Run in a new private instance**. The SPA sends `POST /api/instances {"releaseId":"rel_…"}` with her cookie, an `Origin` header and the `X-CSRF-Token` from `/api/session`. `guard(enabled, …)` (`pkg/catalog/auth.go:75`) checks the session, the account state, the Origin and the CSRF token. `createInstance` (`pkg/catalog/instances.go:47`) calls `entitled`, which calls `access`, which runs `billing.Policy.Access` over her subscriptions and grants. It then creates an instance with hostname `i-…`.
3. The SPA navigates to `/instances/<id>`, which renders the **Launcher**: a frame whose source is the instance origin's `/launch`. The launcher asks the portal for a launch code (`POST /api/instances/<id>/launch`), which runs `authorizeInstance` (`pkg/catalog/server.go:229`) again.
4. The instance origin exchanges the code for a runtime grant (`hosted/routes.go`, `exchange`) and loads `/page`, which returns the release HTML with the runtime script injected into `<head>` (`hosted/routes.go:162`).
5. When Polis calls `window.claude.use("sample")`, the runtime posts to `/api/views/<view>/calls`. `rt.sample` resolves the model tier, reserves credits atomically and streams the answer.

Every hosted request re-runs `authorizeInstance` through `rt.principal`, so losing a subscription closes live views promptly.

### 3.4 Where the design changes things

- Step 1 becomes a **server-rendered public page** with a write-up, media and creators.
- Steps 2–5 ask an **entitlement resolver** ("which plan does Alice have, and what does it include?") instead of a yes/no decision.
- A visitor without an account can take a **demo** path through the same steps with a guest identity.
- Releases are uploaded through a **studio** by creators, from the browser or a CLI.

## 4. Current state, with evidence

### 4.1 Content

`catalog_artifacts` has `id, title, description, tags, visibility, created_at, updated_at` (`pkg/catalog/store/migrations/001_core.sql`). `PATCH /api/admin/artifacts/{id}` (`pkg/catalog/admin.go`, `adminArtifact`) limits the description to 2000 characters of plain text. There is no author, no long text, no image and no ordering. The public API `GET /api/catalog` (`routes.go`, `catalog`) lists listed artifacts that have a published release; `GET /api/artifacts/{id}` returns one artifact and its published releases.

### 4.2 Public pages

The SPA shell is served for `/`, `/apps/`, `/account`, `/instances/`, `/admin` (`routes.go:26`). All content is fetched by JavaScript after load, so search engines and link previews see an empty `<div id="root">`. The only server-rendered pages are `/privacy` and `/terms`, which use `html/template` with `pkg/catalog/web/legal.tmpl` (`routes.go:92`, `legalPage`). Every portal response carries this Content-Security-Policy (`routes.go:80`):

```text
default-src 'self'; frame-src <scheme>://*.<app domain>; img-src 'self' data:;
style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none';
base-uri 'none'; form-action 'self'
```

Inline styles and inline scripts are therefore impossible on portal pages, and all images must come from the portal origin. New templates must follow the same rules.

### 4.3 Authoring

The admin **Releases** tab (`ui/catalog/admin.tsx:122`) is a form with an HTML file, artifact ID, title, description, tags, version, release notes, a manifest textarea and a free-sample checkbox. Its endpoints (`pkg/catalog/admin.go:18`, `adminRoutes`):

| Endpoint | Handler | Behaviour |
| --- | --- | --- |
| `POST /api/admin/releases/check` | `adminCheck` | Parses the multipart form, runs `releases.Validate`, stores nothing |
| `POST /api/admin/releases` | `adminUpload` | `Publisher.CreateDraft`: validate, upsert the artifact, write HTML and manifest blobs, insert a `draft` release |
| `POST /api/admin/releases/{id}/preview` | `adminPreview` | Reuses or creates a `preview` instance owned by the admin |
| `POST /api/admin/releases/{id}/publish` | `adminPublish` | `draft` → `published` |
| `POST /api/admin/releases/{id}/withdraw` | `adminWithdraw` | `published` → `withdrawn`, optionally suspending pinned instances |
| `DELETE /api/admin/releases/{id}` | `adminDeleteDraft` | Deletes a draft |
| `PATCH /api/admin/artifacts/{id}` | `adminArtifact` | Title, description, tags, visibility |

`releases.Validate` (`pkg/catalog/releases/releases.go:141`) produces findings with levels `error` (blocks), `warning` and `info`: invalid ID or version, missing title, size, not a complete HTML document, an embedded SQLite database, an injected Francois bootstrap, credential-like text, invalid or unknown manifest capabilities, unavailable `files` and `artifact` capabilities, and external script hosts. `Blobs.Put` (`releases.go:46`) writes content-addressed files under `<release dir>/blobs/<xx>/<sha256>` with a temp file, fsync and rename; `Blobs.Read` verifies the hash on every read. The nightly backup copies exactly this directory.

Only administrators can publish, and the CLI equivalent (`francois catalog publish`) needs direct database access, which in production means `kubectl exec`.

### 4.4 Access and billing

- `billing.Policy` (`policy.go:32`) holds `AcceptedPrices` and `TrialAllowed`. `Server.policy()` (`server.go:172`) builds it from the single `--stripe-price-id`.
- `Policy.Access` returns a `Decision{Allowed, Reason, Until, Status, CancelScheduled}`. It considers subscriptions on an accepted price with quantity 1, and complimentary grants (`access_grants`). It does not say *which* price or plan applies.
- `checkout` (`routes.go:316`) accepts only `{"plan":"subscriber"}`.
- Limits are global, in the settings document: `limits.instancesPerUser`, `limits.membersPerInstance`, `allowance.monthlyCredits`, `allowance.dailyCredits` (`settings.go`).
- Yes/no checks happen in `entitled` (`instances.go:34`), `authorizeInstance` (`server.go:229`), `download` (`routes.go`) and `sessionInfo`. Credits are reserved in `rt.sample` with the global allowance (`hosted/sample.go:132`).
- Roles are `subscriber` and `admin` (`users.role` CHECK in `001_core.sql:9`). Administrators need a subscription or grant like anyone else to run workspaces.

### 4.5 Gaps

| Need | Today | Gap |
| --- | --- | --- |
| Rich artifact pages | Title, 2000-character description, tags | Write-up, cover, gallery, creators, ordering, featured flag |
| Indexable showcase | SPA renders everything client-side | Server-rendered public pages |
| Images and video | None | Media library served from the portal origin |
| Journal | None | Posts with draft and publish, RSS |
| Two creators | Admin-only publishing | Creator role scoped to own artifacts |
| Page preview | Only the app can be previewed | Draft write-up rendered as the public page |
| Tiers | One price, all-or-nothing | Plans with limits, entitlement resolver, pricing page, checkout per plan |
| Visitor demos | Login wall | Guest instances with caps |
| Publishing from a laptop | `kubectl exec` | API tokens and a push CLI |

## 5. Proposed architecture

### 5.1 Overview

```text
                         Public (no account)                      Signed in
 ┌──────────────────────────────────────────────────┐   ┌───────────────────────────────┐
 │ Server-rendered pages (html/template)            │   │ SPA (React)                    │
 │  /  /apps  /apps/{id}  /creators/{slug}           │   │  /instances  /account          │
 │  /journal  /journal/{slug}  /journal/feed.xml     │   │  /studio  (creators, admins)   │
 │  /pricing  /media/{id}                            │   │  /admin   (admins)             │
 │  + /static/public.js  (session-aware buttons)     │   └───────────────────────────────┘
 └──────────────────────────────────────────────────┘
                 │ reads                                        │ JSON
                 ▼                                              ▼
  content store: artifacts(page, page_draft) · creators · artifact_creators · posts · media
  entitlements: settings.plans + billing projections + grants + role  → Entitlement
  demos: guest users + demo instances (expiring) + demo budget
  studio API /api/studio/*  (cookie+CSRF, or API token)   admin API /api/admin/*
```

### 5.2 Data model

New migration `pkg/catalog/store/migrations/004_storefront.sql`. All IDs are text with a prefix, created by `store.NewID`.

```sql
-- Roles: creators edit their own content; guests are demo visitors.
ALTER TABLE users DROP CONSTRAINT users_role_check;
ALTER TABLE users ADD CONSTRAINT users_role_check
  CHECK (role IN ('subscriber', 'creator', 'admin', 'guest'));

-- Artifact page: published and draft copies of the page document, plus
-- settings that apply immediately.
ALTER TABLE catalog_artifacts
  ADD COLUMN page jsonb NOT NULL DEFAULT '{}',
  ADD COLUMN page_draft jsonb,
  ADD COLUMN featured boolean NOT NULL DEFAULT false,
  ADD COLUMN sort_order integer NOT NULL DEFAULT 0,
  ADD COLUMN required_plan text NOT NULL DEFAULT '',
  ADD COLUMN demo_enabled boolean NOT NULL DEFAULT false;

CREATE TABLE catalog_media (
  id text PRIMARY KEY,                       -- med_…
  sha256 text NOT NULL,
  storage_key text NOT NULL,                 -- blobs/<xx>/<sha256>, same store as releases
  media_type text NOT NULL,                  -- image/png|jpeg|webp|gif, video/mp4|webm
  size bigint NOT NULL,
  width integer NOT NULL DEFAULT 0,
  height integer NOT NULL DEFAULT 0,
  alt text NOT NULL DEFAULT '',
  uploaded_by text NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE creators (
  id text PRIMARY KEY,                       -- cr_…
  slug text NOT NULL UNIQUE,
  name text NOT NULL,
  bio text NOT NULL DEFAULT '',              -- Markdown
  avatar_media_id text REFERENCES catalog_media(id),
  links jsonb NOT NULL DEFAULT '[]',         -- [{"label":"…","url":"https://…"}]
  user_id text UNIQUE REFERENCES users(id),  -- the account that may edit as this creator
  sort_order integer NOT NULL DEFAULT 0,
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE artifact_creators (
  artifact_id text NOT NULL REFERENCES catalog_artifacts(id) ON DELETE CASCADE,
  creator_id text NOT NULL REFERENCES creators(id) ON DELETE CASCADE,
  position integer NOT NULL DEFAULT 0,
  PRIMARY KEY (artifact_id, creator_id)
);

CREATE TABLE posts (
  id text PRIMARY KEY,                       -- post_…
  slug text NOT NULL UNIQUE,
  title text NOT NULL,
  summary text NOT NULL DEFAULT '',
  body text NOT NULL DEFAULT '',             -- Markdown
  cover_media_id text REFERENCES catalog_media(id),
  author_id text REFERENCES creators(id),
  tags text[] NOT NULL DEFAULT '{}',
  state text NOT NULL DEFAULT 'draft' CHECK (state IN ('draft', 'published')),
  published_at timestamptz,
  created_by text NOT NULL REFERENCES users(id),
  created_at timestamptz NOT NULL DEFAULT now(),
  updated_at timestamptz NOT NULL DEFAULT now()
);

-- Complimentary access can name a plan ('' = the highest-ranked plan).
ALTER TABLE access_grants ADD COLUMN plan text NOT NULL DEFAULT '';

-- Demo instances expire.
ALTER TABLE instances DROP CONSTRAINT instances_kind_check;
ALTER TABLE instances ADD CONSTRAINT instances_kind_check
  CHECK (kind IN ('workspace', 'preview', 'demo'));
ALTER TABLE instances ADD COLUMN expires_at timestamptz;

CREATE TABLE api_tokens (
  id text PRIMARY KEY,                       -- tok_…
  user_id text NOT NULL REFERENCES users(id),
  name text NOT NULL,
  token_hash bytea NOT NULL UNIQUE,
  created_at timestamptz NOT NULL DEFAULT now(),
  last_used_at timestamptz,
  expires_at timestamptz,
  revoked_at timestamptz
);
```

The exact constraint names (`users_role_check`, `instances_kind_check`) are PostgreSQL's defaults for the inline `CHECK` constraints in `001_core.sql`; the migration test verifies them.

The **page document** stored in `page` and `page_draft`:

```json
{
  "summary": "One paragraph shown on cards and in link previews.",
  "body": "# How it works\n\nMarkdown …",
  "cover": "med_…",
  "gallery": ["med_…", "med_…"]
}
```

### Decision: page draft as a second column, not a revision table

- **Context:** Editors need to prepare a write-up and see it rendered without changing what visitors see. Releases already have draft and published states, but page text changes far more often and is small.
- **Options considered:** (a) edit the live page directly; (b) a `page_revisions` table with full history; (c) two JSON columns, `page` and `page_draft`.
- **Decision:** (c).
- **Rationale:** It gives the two states the workflow needs with one row per artifact and no joins on the public path. History can be added later by auditing publishes, which already go to `admin_audit`.
- **Consequences:** Only one draft at a time; publishing copies the draft and clears it. Title, tags, featured, ordering, required plan, demo flag and creators apply immediately, as visibility already does.
- **Status:** accepted

### Decision: media in the release blob store

- **Context:** Covers, galleries, avatars and post images must be stored, backed up and served.
- **Options considered:** (a) per-instance assets (`instance_assets`); (b) a new directory; (c) the existing content-addressed `Blobs` store under the release directory.
- **Decision:** (c), with metadata in `catalog_media`.
- **Rationale:** `Blobs.Put` and `Blobs.Read` already give atomic writes and hash verification, and the backup CronJob already syncs `releases/blobs`. Instance assets belong to subscriber data and are served on instance origins, which public pages cannot reference.
- **Consequences:** Media and release files share the same storage key space; identical bytes are stored once. Deleting a media row does not delete its blob, which may be shared.
- **Status:** accepted

### 5.3 Markdown

`pkg/catalog/content/markdown.go` renders write-ups, bios and posts with `github.com/yuin/goldmark` (already in `go.sum` as an indirect dependency):

```go
md := goldmark.New(
    goldmark.WithExtensions(extension.GFM),       // tables, strikethrough, autolinks, task lists
    goldmark.WithParserOptions(parser.WithAutoHeadingID()),
    goldmark.WithRendererOptions(/* html.WithUnsafe() is NOT set */),
)
```

Without `WithUnsafe`, goldmark omits raw HTML and refuses `javascript:` and similar URLs. One more rule is needed because of the CSP: images must be on the portal origin, so the renderer replaces an image whose URL is not a relative `/media/…` path with a plain link. The result is passed to templates as `template.HTML` only after rendering, never from raw input.

### Decision: server-side Markdown, no raw HTML

- **Context:** Write-ups are authored by trusted creators but rendered on the trusted portal origin, where a script injection would expose every visitor's session-bound actions.
- **Options considered:** (a) Markdown with raw HTML plus a sanitizer (bluemonday); (b) Markdown without raw HTML; (c) a rich-text editor storing HTML.
- **Decision:** (b).
- **Rationale:** It removes the injection class entirely instead of filtering it, needs no second library, and keeps content portable.
- **Consequences:** Embeds (video players, iframes) are not possible in text; video goes in the gallery as uploaded media.
- **Status:** accepted

### 5.4 Public pages

New server-rendered routes on the portal origin, rendered with `html/template` from `pkg/catalog/web/templates/*.tmpl` embedded in the binary:

| Route | Template | Content |
| --- | --- | --- |
| `GET /{$}` | `home.tmpl` | Storefront title, tagline and intro (from settings), featured artifacts, creators, latest posts |
| `GET /apps` | `catalog.tmpl` | All listed artifacts with a published release, as cards; `?tag=` filter |
| `GET /apps/{id}` | `artifact.tmpl` | Cover, title, creators, summary, rendered write-up, gallery, version history, plan badge, action buttons |
| `GET /creators/{slug}` | `creator.tmpl` | Avatar, bio, links, artifacts, posts |
| `GET /journal` | `journal.tmpl` | Published posts, newest first |
| `GET /journal/{slug}` | `post.tmpl` | A post |
| `GET /journal/feed.xml` | (encoding/xml) | RSS 2.0 of published posts |
| `GET /pricing` | `pricing.tmpl` | Public plans with prices and highlights |
| `GET /media/{id}` | — | Media bytes |

`/apps/{id}` keeps the URL the SPA used for artifact details, so existing links still work. The SPA keeps `/instances/`, `/account`, `/admin` and gains `/studio`. The SPA's `Catalog` and `AppDetail` components are removed, and the SPA's links to server-rendered pages become ordinary navigations.

**Rendering a page.** Each handler loads data through the store, renders Markdown, and executes a template inside a shared layout (`layout.tmpl`) with the header, footer and the stylesheet `/static/catalog.css`. The header is rendered on the server from the request's principal (the session cookie is read with `guard(public, …)`), so navigation is correct without JavaScript. Responses carry `Cache-Control: no-cache` because the header is personalised.

**Draft preview.** `GET /apps/{id}?draft=1` renders `page_draft` instead of `page`, and also renders hidden artifacts, but only when the caller may edit that artifact (§5.7). Otherwise the parameter is ignored. The page shows a "Draft preview" banner.

**Action buttons.** Buttons that need CSRF-protected requests are rendered as placeholders:

```html
<div class="actions" data-artifact="polis" data-release="rel_…" data-demo="true"></div>
<script src="/static/public.js" defer></script>
```

`public.js` (new esbuild entry `ui/catalog/public.ts`) fetches `GET /api/session` and `GET /api/entitlement`, then renders:

| Visitor | Buttons |
| --- | --- |
| Signed out | **Try the demo** (if enabled), **Sign in to run** |
| Signed in, plan includes the artifact | **Run in a new instance**, **Try the demo** |
| Signed in, plan does not include it | **Upgrade to <plan>** (→ `/pricing`), **Try the demo** |

Run posts `POST /api/instances` with the CSRF token and goes to `/instances/<id>`. Demo posts `POST /api/demos` (§5.9) and goes to `/instances/<id>`.

**Link previews.** `layout.tmpl` emits `<title>`, `<meta name="description">` and Open Graph tags (`og:title`, `og:description`, `og:image` as the absolute `/media/<cover>` URL on the current site).

**CSP.** The existing policy applies. `media-src 'self'` is added so gallery videos play. No template uses inline styles or scripts.

**Not found.** A hidden artifact, an artifact without a published release, a draft post or an unknown creator returns a 404 page, identical to an unknown ID.

### 5.5 Media serving and validation

`POST /api/studio/media` takes a multipart file and an `alt` text. The server:

1. Reads at most `limits.maxMediaBytes` (new setting, default 25 MiB, max 200 MiB).
2. Detects the type from the bytes with `http.DetectContentType`, ignoring the client's `Content-Type`. Accepted: `image/png`, `image/jpeg`, `image/gif`, `image/webp`, `video/mp4`, `video/webm`. SVG is rejected because it can carry script.
3. For PNG, JPEG and GIF, reads the dimensions with `image.DecodeConfig` (WebP and video store 0).
4. Writes the blob with `Blobs.Put` and inserts `catalog_media`.

`GET /media/{id}` is public and returns the bytes with:

```text
Content-Type: <stored type>
Cache-Control: public, max-age=31536000, immutable
X-Content-Type-Options: nosniff
Content-Security-Policy: default-src 'none'; sandbox
```

A media ID always refers to the same bytes, so long caching is safe. Media is public by design: anything uploaded may be referenced from a public page. Deleting is refused while an artifact page, page draft, creator or post references the media.

### 5.6 Creators and journal

A **creator** is a public profile, optionally linked to one user account (`creators.user_id`). Linking is how authorization works: a user with role `creator` may edit the profile linked to them and any artifact whose `artifact_creators` include that profile. Administrators may edit everything and create, link or delete creators.

A **post** has a slug, a title, a summary, a Markdown body, an optional cover, an author (a creator), tags and a state. Posts are drafts until published; editing a published post changes it immediately. To rework a published post without visitors seeing intermediate states, unpublish it first. Creators may create posts authored as themselves and edit their own posts; administrators may edit all.

### 5.7 Studio API and the creator role

**Guard level.** `auth.go` gains a level between `enabled` and `admin`:

```go
const (
    public = iota
    session
    enabled
    creator   // role creator or admin
    admin
)
```

**Object authorization.** One helper answers whether a principal may edit an artifact:

```go
func (s *Server) canEditArtifact(ctx context.Context, p *Principal, artifactID string) (bool, error) {
    if p.User.Role == "admin" { return true, nil }
    if p.User.Role != "creator" { return false, nil }
    return s.Store.IsArtifactCreator(ctx, artifactID, p.User.ID) // joins artifact_creators → creators.user_id
}
```

**Endpoints.** All under `/api/studio/`, all JSON except uploads, all guarded at `creator` level. "Own" means linked through a creator profile.

| Method and path | Who | Purpose |
| --- | --- | --- |
| `GET /api/studio/artifacts` | creator: own; admin: all | List with release counts and page status |
| `POST /api/studio/artifacts` | creator, admin | Create `{id, title}`; a creator is linked automatically |
| `GET /api/studio/artifacts/{id}` | editor of it | Artifact, page, draft, creators, releases |
| `PATCH /api/studio/artifacts/{id}` | editor of it | `title, tags, visibility, featured, sortOrder, requiredPlan, demoEnabled, creators[]` |
| `PUT /api/studio/artifacts/{id}/draft` | editor of it | Save the page draft `{summary, body, cover, gallery}` |
| `POST /api/studio/artifacts/{id}/draft/publish` | editor of it | Copy draft to page, clear draft |
| `DELETE /api/studio/artifacts/{id}/draft` | editor of it | Discard the draft |
| `POST /api/studio/releases/check` | creator, admin | Dry-run validation (unchanged semantics) |
| `POST /api/studio/releases` | editor of the artifact (or new artifact) | Create a draft release |
| `POST /api/studio/releases/{id}/preview` | editor of its artifact | Preview instance |
| `POST /api/studio/releases/{id}/publish` | editor of its artifact | Publish |
| `POST /api/studio/releases/{id}/withdraw` | editor of its artifact | Withdraw |
| `DELETE /api/studio/releases/{id}` | editor of its artifact | Delete a draft |
| `GET /api/studio/media` | creator, admin | Media library |
| `POST /api/studio/media` | creator, admin | Upload |
| `PATCH /api/studio/media/{id}` | uploader, admin | Alt text |
| `DELETE /api/studio/media/{id}` | uploader, admin | Delete if unreferenced |
| `GET /api/studio/posts` | creator: own; admin: all | List |
| `POST /api/studio/posts` | creator, admin | Create |
| `GET/PATCH/DELETE /api/studio/posts/{id}` | author, admin | Read, edit, delete |
| `POST /api/studio/posts/{id}/publish`, `/unpublish` | author, admin | State |
| `GET /api/studio/creators` | creator, admin | All creators (to pick co-creators) |
| `POST /api/studio/creators` | admin | Create a profile |
| `PATCH /api/studio/creators/{id}` | linked user, admin | Edit; only admins change `userId` |
| `DELETE /api/studio/creators/{id}` | admin | Delete |
| `POST /api/studio/markdown` | creator, admin | Render Markdown with the public renderer, for the editor's preview |
| `GET/POST /api/studio/tokens`, `DELETE /api/studio/tokens/{id}` | self | Personal API tokens (§5.10) |

The `/api/admin/releases*` endpoints and `PATCH /api/admin/artifacts/{id}` are removed; the studio replaces them. `PATCH /api/admin/users/{id}` accepts the `creator` role.

### Decision: a separate studio API rather than widening the admin API

- **Context:** Creators need part of what administrators do, scoped to their own objects, and the CLI needs a token-authenticated surface.
- **Options considered:** (a) keep `/api/admin/*` and add per-route exceptions for creators; (b) a `/api/studio/*` namespace for content, leaving `/api/admin/*` for users, billing, settings and audit.
- **Decision:** (b).
- **Rationale:** The boundary becomes visible in the URL: everything under `/api/admin` is admin-only, everything under `/api/studio` is content with per-object checks. API tokens are accepted only under `/api/studio`, so a leaked publishing token cannot change users or billing.
- **Consequences:** The admin release endpoints move and their tests move with them. No compatibility aliases are kept.
- **Status:** accepted

### 5.8 Plans and entitlements

#### 5.8.1 Plans in the settings document

`settings.Document` gains `plans`, `demo` and `storefront`, and the global per-user limits move into plans:

```go
type PlanPrice struct {
    Interval string `json:"interval"` // "month" | "year"
    PriceID  string `json:"priceId"`  // Stripe recurring Price ID
    Label    string `json:"label"`    // display text, e.g. "€5 / month"
}

type Plan struct {
    Key                string      `json:"key"`         // "free", "supporter", "studio"
    Name               string      `json:"name"`
    Rank               int         `json:"rank"`        // 0 = free; higher includes lower
    Description        string      `json:"description"`
    Highlights         []string    `json:"highlights"`
    Public             bool        `json:"public"`      // shown on /pricing
    Prices             []PlanPrice `json:"prices"`      // empty only for the free plan
    Instances          int         `json:"instances"`
    MembersPerInstance int         `json:"membersPerInstance"`
    MonthlyCredits     int         `json:"monthlyCredits"`
    DailyCredits       int         `json:"dailyCredits"`
    Models             []string    `json:"models"`      // approved selectors; empty = all approved
}

type Demo struct {
    Enabled          bool     `json:"enabled"`
    TTLMinutes       int      `json:"ttlMinutes"`       // instance and guest session lifetime
    MaxActive        int      `json:"maxActive"`        // live demo instances, all visitors
    PerAddressPerHour int     `json:"perAddressPerHour"`
    Credits          int      `json:"credits"`          // per demo
    DailyBudget      int      `json:"dailyBudget"`      // all demos together
    Models           []string `json:"models"`
}

type Storefront struct {
    Title   string `json:"title"`
    Tagline string `json:"tagline"`
    Intro   string `json:"intro"` // Markdown
}
```

`Limits` keeps `documentsPerInstance`, `assetBytesPerInstance`, `maxHtmlBytes` and gains `maxMediaBytes`. `Allowance` keeps the concurrency and tool-round limits; its credit fields move into plans.

**Validation rules** (added to `Document.Validate`):

- Exactly one plan has key `free`, rank 0 and no prices. All ranks are distinct. Keys match `^[a-z][a-z0-9-]{0,31}$`.
- Every paid plan has at least one price; intervals are `month` or `year`, at most one of each; price IDs start with `price_` and are unique across plans.
- Every selector in `plan.models` and `demo.models` is an approved model.
- Numeric bounds as today (instances 0–1000, members 1–100, credits 0–1,000,000).

`Defaults()` contains only the free plan with zero instances and zero credits, so a fresh deployment runs nothing until an administrator saves plans.

#### 5.8.2 The entitlement resolver

```go
// Entitlement is everything a user may do now, and why.
type Entitlement struct {
    Plan     settings.Plan
    Source   string           // "staff" | "subscription" | "trial" | "complimentary" | "free" | "guest" | "disabled"
    Decision billing.Decision // standing details for the account page
}

func (e Entitlement) CanRun(a store.Artifact, plans []settings.Plan) bool
func (e Entitlement) AllowsModel(selector string) bool
```

Resolution order, implemented in `pkg/catalog/entitlement.go`:

```text
resolve(user, settings, now):
  if user.disabled            → {Source: disabled, Plan: none}            // denies everything
  if role ∈ {admin, creator}  → {Source: staff, Plan: highest-ranked plan}
  if role == guest            → {Source: guest, Plan: demo plan derived from settings.demo}
  d := Policy{AcceptedPrices: all plan price IDs}.Access(account, subs, comps, now)
  if d.Allowed && d.Reason ∈ {subscriber, subscriber_trial}
                               → {Source: subscription|trial, Plan: plan owning d.PriceID}
  if d.Allowed && d.Reason == complimentary
                               → {Source: complimentary, Plan: grant.plan or highest-ranked}
  otherwise                    → {Source: free, Plan: free plan, Decision: d}
```

`billing.Decision` gains `PriceID` (the price of the standing subscription) and `Plan` (the grant's plan key), so the pure policy stays the single place that interprets Stripe states. `Policy.Access` remains pure and table-tested.

**Running an artifact.** `required_plan` on the artifact names the lowest plan that includes it; empty means the lowest-ranked *paid* plan.

```text
CanRun(artifact):
  required := plan(artifact.requiredPlan) or lowest-ranked plan with prices
  return e.Source != disabled && e.Plan.Rank >= required.Rank
```

An artifact can therefore be offered to free accounts by setting its required plan to `free`.

#### 5.8.3 Where entitlements are checked

| Place | Today | New rule |
| --- | --- | --- |
| `createInstance` | `entitled` yes/no; global `instancesPerUser` | `CanRun(artifact)`; limit `plan.Instances` (workspaces only) |
| `authorizeInstance` (launch and every runtime request) | caller and owner `Allowed` | `preview`: editor of the artifact. `demo`: owner is caller, not expired. `workspace`: caller and owner `CanRun`; the instance is within the owner's first `plan.Instances` workspaces by creation time |
| `addMember` | global `membersPerInstance` | owner's `plan.MembersPerInstance` |
| `rt.sample` | global monthly and daily credits | the caller's plan credits; the resolved selector must satisfy `AllowsModel`; demos use `demo.credits`, `demo.models` and `demo.dailyBudget` |
| `download` | free sample or `Allowed` | free sample, or `CanRun(artifact)` |
| `sessionInfo`, `account` | `access` decision | plus the plan, its limits and usage |
| `checkout` | plan must be `subscriber` | `{plan, interval}` → server resolves the price |

**Downgrades.** No data is deleted. If a subscriber on a lower plan owns more workspaces than the plan allows, the oldest ones up to the limit keep working and launching the others fails with `over_plan_limit` and a message pointing to the pricing page. Upgrading restores them.

#### 5.8.4 Checkout, changes and Stripe

```text
POST /api/billing/checkout {"plan":"studio","interval":"year"}
  plan := settings.plan(key); require plan.prices[interval]
  require user email verified (unchanged)
  if user has an open subscription on any plan price → 409 subscription_open  (use the billing portal)
  Payments.Checkout(user, priceID, portalOrigin) → Stripe Checkout URL
```

`payments.Service` stops reading a single configured price: `Checkout` takes the price ID, and the "already subscribed" check compares against the set of all plan prices. Plan changes, cancellations and payment methods go through the Stripe billing portal, which must be configured (in Stripe, for the products and prices involved) to allow switching between them. The webhook already records `price_id` on `billing_subscriptions`, so a change in the portal changes the resolved plan on the next request.

`--stripe-price-id` is removed from `catalog serve`; prices live in plans. The production rollout (§10, Phase 8) saves a plans revision containing the existing price.

### Decision: plans in the versioned settings document

- **Context:** Plans combine Stripe prices with limits and credits that the owners will tune.
- **Options considered:** (a) a `plans` table with its own admin API; (b) command-line flags; (c) a block in the existing settings document.
- **Decision:** (c).
- **Rationale:** Settings already provide validation, revision history, an admin editor and atomic activation. Model approvals and credit costs, which plans refer to, live in the same document, so one validation pass can check references between them.
- **Consequences:** Changing a plan is one revision; old revisions remain readable. Plans cannot be referenced by foreign keys, so artifacts reference plans by key and an unknown key falls back to the lowest paid plan.
- **Status:** accepted

#### 5.8.5 Pricing page and account page

`/pricing` renders public plans by rank with name, description, highlights and price labels. The buttons depend on the visitor: sign in, **Choose** (checkout through `public.js`), **Current plan** or **Manage billing** (the Stripe portal). The SPA account page shows the plan name and source, limits and usage, and links to `/pricing`.

### 5.9 Visitor demos

A demo lets someone without an account try an artifact for a limited time.

```text
POST /api/demos {"artifactId":"polis"}            (public; Origin must be the portal)
  require settings.demo.enabled and artifact.demoEnabled and a published release
  require rate limit for client address and live demos < demo.maxActive
  principal := current session or create guest:
      user  := users(role guest, issuer "urn:francois:guest", subject random)
      session cookie with lifetime demo.ttlMinutes
  inst := instances(kind demo, owner guest/user, latest published release,
                    expires_at now + ttl, title "Demo · <title>")
  → 201 {"id": inst.id}   then the browser opens /instances/<id> (normal launch)
```

- **Authorization.** `authorizeInstance` allows a `demo` instance only for its owner and only before `expires_at`. A guest can reach `/instances/<id>` and launch because the existing launch path needs only a session; guests cannot create workspaces (their entitlement plan has zero instances) and cannot open checkout.
- **Model budget.** Guests reserve credits against `demo.credits` (both monthly and daily allowance for that guest). Before reserving, the server checks that today's credits charged to guests are below `demo.dailyBudget`.
- **Rate limit.** An in-memory limiter keyed by client address allows `demo.perAddressPerHour` demos. Behind Traefik the client address comes from `X-Real-Ip`, honoured only when `catalog serve --trust-proxy-headers` is set.
- **Clean-up.** A loop started in `Server.Run` deletes expired demo instances every minute (revoking live views first). Guest users stay as rows so model-call history keeps its foreign keys; their sessions expire.
- **Signed-in visitors** get a demo instance under their own account, with the same expiry and demo budget.

### Decision: guest users instead of anonymous instances

- **Context:** The runtime assumes every request has a user (`hosted.Principal`): documents are owned, grants are bound to a session, model calls are charged to a user.
- **Options considered:** (a) teach the runtime an anonymous principal; (b) create a short-lived guest user and session per visitor.
- **Decision:** (b).
- **Rationale:** It reuses the launch handshake, grants, ownership checks and credit accounting unchanged, so the security review covers one path.
- **Consequences:** Guest rows accumulate; they are small and filtered out of the admin user list. A periodic purge of guests without instances can be added later.
- **Status:** accepted

### 5.10 API tokens and `francois catalog push`

A personal API token lets a creator use the studio API from scripts. Tokens are created in the studio (shown once), stored as a SHA-256 hash, can expire and be revoked, and record `last_used_at`. Format: `frt_` followed by 43 base64url characters.

`guard` accepts `Authorization: Bearer frt_…` **only for routes under `/api/studio/`**. A bearer request carries no ambient browser credential, so the Origin and CSRF checks do not apply to it; everything else (role, disabled account, object checks) does.

```text
francois catalog push \
  --server https://slop.boutique --token-file ~/.config/francois/token \
  --artifact-id polis --version 1.2.0 --html dist/polis.html \
  [--manifest polis.manifest.json] [--title …] [--changelog …] [--publish]
```

`push` uploads with `POST /api/studio/releases` (the same multipart form as the browser), prints the findings, and with `--publish` publishes the new draft. It never needs database access.

### 5.11 Studio UI

A new SPA area `/studio`, visible to creators and administrators:

```text
Studio ─┬─ Artifacts ── list (title, status, latest version, page status)
        │               └─ editor: [Details] [Write-up] [Media] [Releases] [Demo]  + "Preview page" + "Publish page"
        ├─ Journal ──── list → post editor (title, slug, summary, cover, body, tags, author; Save, Publish/Unpublish, Preview)
        ├─ Creators ─── profiles (admins: all, link to users; creators: own profile)
        ├─ Media ────── library: upload, alt text, copy Markdown, delete
        └─ Tokens ───── create, list, revoke
```

- **Details:** title, tags, visibility, featured, sort order, required plan (from settings), creators.
- **Write-up:** summary, Markdown body with a side-by-side preview rendered by `POST /api/studio/markdown`, cover and gallery pickers from the media library; **Save draft**, **Preview page** (opens `/apps/<id>?draft=1`), **Publish page**, **Discard draft**.
- **Media:** upload area and grid; inserting an image adds `![alt](/media/<id>)` to the body.
- **Releases:** today's flow from `admin.tsx`, scoped to the artifact.
- **Demo:** the artifact's demo flag, with the global demo settings shown for context.

The admin area keeps Overview, Users (with the new role), Instances, Billing, Usage, Settings (with plans, demo and storefront sections) and Audit. Its Releases tab is removed.

## 6. API reference (new and changed)

### 6.1 Public

| Method | Path | Notes |
| --- | --- | --- |
| GET | `/`, `/apps`, `/apps/{id}`, `/creators/{slug}`, `/journal`, `/journal/{slug}`, `/pricing` | HTML |
| GET | `/journal/feed.xml` | RSS 2.0 |
| GET | `/media/{id}` | Bytes |
| GET | `/api/catalog` | Gains `summary`, `cover`, `creators`, `requiredPlan`, `demo` per artifact |
| GET | `/api/plans` | Public plans for scripts and the SPA |
| GET | `/api/entitlement` | Signed-in caller's plan, source, limits and usage; `{authenticated:false}` otherwise |
| POST | `/api/demos` | `{artifactId}` → `{id}`; errors `demo_disabled`, `demo_limit`, `rate_limited` |

### 6.2 Changed

| Method | Path | Change |
| --- | --- | --- |
| POST | `/api/billing/checkout` | Body `{plan, interval}` |
| PATCH | `/api/admin/users/{id}` | `role` may be `creator` |
| POST | `/api/admin/users/{id}/access-grants` | Optional `plan` |
| GET | `/api/session` | `user.role` may be `creator` or `guest`; adds `plan` |
| — | `/api/admin/releases*`, `PATCH /api/admin/artifacts/{id}` | Removed (moved to `/api/studio`) |

### 6.3 Error codes added

`over_plan_limit`, `plan_required`, `model_not_in_plan`, `demo_disabled`, `demo_limit`, `demo_expired`, `rate_limited`, `creator_required`, `not_editor`, `media_type`, `media_in_use`, `slug_taken`, `token_invalid`.

## 7. Key flows in pseudocode

### 7.1 Rendering an artifact page

```go
func (s *Server) artifactPage(w http.ResponseWriter, r *http.Request) {
    p := principal(r.Context())
    a, err := s.Store.Artifact(ctx, r.PathValue("id"))
    draft := r.URL.Query().Get("draft") == "1" && p != nil && s.canEdit(ctx, p, a.ID)
    if err != nil || (!draft && (a.Visibility != "listed" || !hasPublishedRelease(a))) {
        s.notFound(w, r); return
    }
    page := a.Page
    if draft && a.PageDraft != nil { page = *a.PageDraft }
    view := artifactView{
        Artifact: a, Summary: page.Summary, Body: s.md.Render(page.Body),
        Cover: s.media(page.Cover), Gallery: s.mediaList(page.Gallery),
        Creators: s.Store.ArtifactCreators(ctx, a.ID), Releases: published(a.ID),
        RequiredPlan: plans.Required(a), Demo: a.DemoEnabled && settings.Demo.Enabled,
        Draft: draft,
    }
    s.render(w, r, "artifact", view)  // layout + header from p + OG tags
}
```

### 7.2 Entitlement check when launching

```go
func (s *Server) authorizeInstance(ctx context.Context, u store.User, inst store.Instance) error {
    rev := s.Store.ActiveSettings(ctx)
    switch inst.Kind {
    case "preview":
        if !s.canEditArtifactOfRelease(ctx, u, inst.ReleaseID) { return Denied(403, "not_editor", …) }
        return nil
    case "demo":
        if inst.OwnerUserID != u.ID { return Denied(403, "not_member", …) }
        if inst.ExpiresAt.Before(now) { return Denied(403, "demo_expired", …) }
        return nil
    }
    a := artifactOf(inst)
    caller := s.entitlement(ctx, u, rev)
    if !caller.CanRun(a, rev.Plans) { return Denied(403, "plan_required", …) }
    if inst.OwnerUserID != u.ID {
        owner := s.entitlement(ctx, ownerOf(inst), rev)
        if !owner.CanRun(a, rev.Plans) { return Denied(403, "owner_inactive", …) }
    }
    if !s.Store.WithinWorkspaceLimit(ctx, inst, ownerPlan.Instances) {
        return Denied(403, "over_plan_limit", …)
    }
    return nil
}
```

### 7.3 Reserving credits for a model call

```go
plan := s.entitlementOf(p.User)              // via a callback from the portal, like Authorize
if !plan.AllowsModel(selector) { fail(403, "model_not_in_plan") }
included, daily := plan.Plan.MonthlyCredits, plan.Plan.DailyCredits
if p.Instance.Kind == "demo" {
    included, daily = rev.Demo.Credits, rev.Demo.Credits
    if s.Store.GuestCreditsToday(ctx)+model.Credits > rev.Demo.DailyBudget { fail(429, "demo_limit") }
}
res := s.Store.Reserve(ctx, ReserveRequest{…, Credits: model.Credits, Included: included, Daily: daily})
```

The runtime receives the entitlement through a second callback next to `Authorizer`, so `pkg/catalog/hosted` keeps no policy of its own.

### 7.4 Publishing a write-up

```text
editor saves      PUT /api/studio/artifacts/polis/draft   {summary, body, cover, gallery}
                  validate: summary ≤ 400, body ≤ 100 KB, media IDs exist
editor previews   GET /apps/polis?draft=1                 (editor only)
editor publishes  POST /api/studio/artifacts/polis/draft/publish
                  UPDATE catalog_artifacts SET page = page_draft, page_draft = NULL
                  audit "artifact.page.publish"
```

## 8. Security review checklist

- Public pages render Markdown without raw HTML; images only from `/media/`; templates auto-escape everything else.
- `/media/{id}` serves only detected raster or video types with `nosniff` and a sandboxing CSP; SVG is refused.
- Draft content renders only for editors of that artifact; the `draft` parameter is ignored for others.
- Creators cannot change users, billing, settings or other creators' artifacts; the studio checks objects, not only roles.
- API tokens work only under `/api/studio/`; they are hashed, expire and are revocable.
- Demo creation checks Origin explicitly (it is a public POST), is rate limited and globally capped, and has its own model budget.
- Guests cannot create workspaces, check out or reach `/api/studio`.
- The entitlement resolver is the only place that maps billing state to permissions.

## 9. Test strategy

**Unit tests (no database):**

- `settings`: plan validation (free plan rules, ranks, prices, model references), demo bounds, defaults.
- `billing`: `Policy.Access` returns `PriceID` and grant plans; existing table extended.
- Entitlement resolver: a table over roles, subscriptions, grants, disabled accounts and plan ranks; `CanRun` with empty, unknown and explicit required plans.
- `content`: Markdown renders GFM, drops raw HTML and `javascript:` links, turns external images into links, adds heading IDs.
- Media detection: PNG, JPEG, WebP, MP4 accepted; SVG, HTML and plain text rejected.

**Integration tests** (`TEST_DATABASE_URL`, pattern of `pkg/catalog/catalog_test.go`):

- Public pages: listed artifact renders with write-up; hidden or unpublished artifact is 404; `?draft=1` renders the draft for an editor and is ignored for others; script in Markdown does not appear in output; RSS lists only published posts.
- Studio: a creator edits their own artifact and gets `not_editor` on someone else's; creators cannot reach `/api/admin`; media upload and delete-while-referenced.
- Plans: subscriber on a lower plan is refused a higher-plan artifact; instance limit per plan; over-limit workspaces cannot launch; member limit from the owner's plan; credits from the plan; staff run everything; complimentary grant with a plan.
- Demos: disabled → 403; guest creation, launch, expiry, global cap, rate limit, budget.
- Tokens: bearer works on `/api/studio`, not on `/api/admin` or `/api/instances`; revoked token fails.

**Front end:** `pnpm check` (TypeScript), `pnpm build`, and a browser pass against a local stack (`docker compose -f deploy/compose/compose.yaml` provides ZITADEL and PostgreSQL) covering the public pages, the studio editor and a demo launch.

## 10. Implementation phases

Each phase ends with passing tests and a commit; the diary records each step.

| Phase | Scope | Main files |
| --- | --- | --- |
| 1 Content model | Migration 004; store for artifacts page, creators, media, posts; `content` Markdown; media detection | `store/migrations/004_storefront.sql`, `store/content.go`, `content/markdown.go`, `content/media.go` |
| 2 Public pages | Templates, handlers, `/media`, RSS, `public.ts`, CSP `media-src`, SPA route changes | `web/templates/*.tmpl`, `pages.go`, `ui/catalog/public.ts`, `routes.go`, `ui/catalog/main.tsx` |
| 3 Studio API | `creator` level, `canEditArtifact`, `/api/studio/*`, release endpoints moved | `auth.go`, `studio.go`, `admin.go` |
| 4 Studio UI | Editor tabs, journal, creators, media, tokens UI | `ui/catalog/studio.tsx`, `api.ts`, `catalog.css` |
| 5 Plans | Settings blocks, `Decision.PriceID`, resolver, gating, checkout, pricing page, account and admin settings UI | `settings/settings.go`, `billing/policy.go`, `entitlement.go`, `instances.go`, `hosted/sample.go`, `payments/service.go` |
| 6 Demos | `/api/demos`, guests, expiry loop, limiter, budget | `demos.go`, `server.go`, `store/instances.go` |
| 7 Tokens and push | `api_tokens`, bearer in guard, `catalog push` | `auth.go`, `store/tokens.go`, `cmd/francois/cmds/catalog.go` |
| 8 Docs and rollout | `doc/production.md`, README, k3s args, rollout order | docs |

**Rollout order for production (Phase 8 detail).** The new binary rejects `--stripe-price-id`, and the old binary's strict settings parser rejects revisions containing `plans`. So: (1) merge a k3s change that removes `--stripe-price-id` together with the image bump; (2) immediately save a settings revision with the free plan and a paid plan using the existing price ID, and the previous credits; (3) link the creators to their accounts and set their roles. Between (1) and (2) subscribers resolve to the free plan; with test users only this is acceptable.

## 11. Risks, alternatives and open questions

- **Risk: the portal now serves user-visible content.** Mitigated by server-side Markdown without raw HTML and the unchanged strict CSP.
- **Risk: demos as an abuse vector** (model spend, instance creation). Mitigated by the global cap, per-address limit, daily budget and the feature being off by default.
- **Risk: plan misconfiguration locks subscribers out.** Validation rejects unknown price formats and missing free plans; the account page shows the resolved plan and source, and staff always resolve to the highest plan.
- **Alternative not taken: separate storefront site** (static generator). Rejected because entitlements, demos and runs need the same session and database.
- **Open: revenue split** between the creators (reporting or Stripe Connect).
- **Open: per-site storefronts.** Both sites show the same storefront today; per-site titles could be added to `Site` later.
- **Open: demo seed data.** Demos start empty; a per-artifact seed document set is a natural extension.

## 12. References

- `pkg/catalog/server.go` — `Site`, `Config`, `Handler`, `authorizeInstance`, `Run`
- `pkg/catalog/routes.go` — SPA routes, CSP, `catalog`, `artifact`, `download`, `account`, `checkout`
- `pkg/catalog/auth.go` — `guard`, sessions, `sameOrigin`
- `pkg/catalog/admin.go` — admin routes and the release upload handlers
- `pkg/catalog/instances.go` — `entitled`, `createInstance`, `launch`, `addMember`
- `pkg/catalog/releases/releases.go` — `Blobs`, `Validate`, `Publisher.CreateDraft`
- `pkg/catalog/billing/policy.go` — `Policy.Access`, reason codes
- `pkg/catalog/settings/settings.go` — `Document`, `Validate`, `Defaults`
- `pkg/catalog/hosted/routes.go`, `runtime.go`, `sample.go` — instance origin, grants, model calls
- `pkg/catalog/store/*.go`, `store/migrations/*.sql` — persistence
- `ui/catalog/*.tsx` — the SPA; `scripts/build.mjs` — bundles
- `doc/production.md` — production deployment
- Ticket FRANCOIS-002 (hosted platform design) and FRANCOIS-003 (deployment)

## 13. Direction for multi-file artifacts (not in this ticket)

Today a release is one HTML file and a manifest (`release_files.role IN ('html','manifest')`). A later ticket can accept a zip upload: validate paths (no `..`, no absolute paths, bounded count and size), store each file as a blob with `role = 'file'` and a relative `name`, keep `index.html` as the entry, and serve the files on the instance origin under `/page/<path>` after the same grant check as `/page`. `Validate` would scan every HTML and JavaScript file for the existing error findings. Asset requests need the grant without a header (for `<img src>`), which the runtime already solves for assets with signed short-lived URLs (`hosted/runtime.go`, `signBlob`).
