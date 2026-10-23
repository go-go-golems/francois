---
Title: 'Storefront browser, launch modes and site copy: analysis, design and implementation guide'
Ticket: FRANCOIS-022
Status: active
Topics:
    - frontend
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/pages.go
      Note: feedPage, artifactPage, cards, card view model
    - Path: repo://pkg/catalog/settings/settings.go
      Note: settings document; storefront.copy goes here
    - Path: repo://pkg/catalog/store/releases.go
      Note: Artifact model; launch_mode column
    - Path: repo://pkg/catalog/web/templates/home.tmpl
      Note: landing page markup to rewrite
    - Path: repo://ui/catalog/instance-launch.ts
      Note: launch handshake reused by Open and tab launch
    - Path: repo://ui/catalog/public.ts
      Note: runner actions, archive script, future viewer
ExternalSources: []
Summary: 'Intern guide: how the slop.boutique storefront, runner and settings work today, and the design and phased plan for an html.moldandyeast.com-style browser, a per-app launch mode (inline or tab) and editable site copy.'
LastUpdated: 2026-10-23T15:29:29.317425052-04:00
WhatFor: Understanding and implementing FRANCOIS-022
WhenToUse: Before touching the landing page, the runner/launch flow, artifact settings or the site copy
---

# Storefront browser, launch modes and site copy

*Analysis, design and implementation guide for FRANCOIS-022. Written for someone new to the codebase: it explains the parts of the system you need before changing anything, then what we will build and in which order.*

## 0. Summary

slop.boutique is the public storefront of **Francois**, a Go service that hosts single-file HTML applications ("artifacts") and runs them for visitors in sandboxed iframes on their own origins. Three changes are planned:

1. **A new landing page / browser.** `/` (and its alias `/apps`) gets the layout of [html.moldandyeast.com](https://html.moldandyeast.com): a large display sentence, a sort select and a tag line with counts, then the list of experiments grouped by month, each row with a thumbnail, a large title, the description, tags and the actions **Open · Download · path · size**. Our header, the support ("subscribe") banner and the footer stay exactly as they are.
2. **A per-app launch mode** with two values:
   - `inline`: like html.moldandyeast.com, **Open** starts the application immediately in a viewer over the index; the app page also starts it immediately for visitors who may run it.
   - `tab`: the app page shows its cover (today's look) and **Start** opens the application full screen in a new browser tab.
3. **Editable site copy.** The hero sentence and other strings that are hard-coded in templates today become editable by staff in the admin settings, with history, through a small registry of "copy keys".

Decisions confirmed by the product owner (Manuel, 2026-10-23):

| Question | Decision |
|---|---|
| Launch modes | Exactly two: `inline` and `tab`. |
| Header | Untouched (wordmark, statement, Account/Sign in, Support). Search stays in the controls row. |
| Hero | A *different* sentence from the header statement: “These artifacts are the results of our research. Play with them, download the HTML, and drop them into your coding agent.” Editable in the backend. |
| Download in the list | Shown, except while the app is in early access and the visitor is not a paying subscriber. |
| CMS | There is none; build a CMS-like mechanism so this and other hard-coded copy can be edited. |
| Early access | When an app is in early access and the visitor may not run it, say so and invite them to subscribe (list, viewer and app page). |
| Launch button | Blue, to stand out, through a design token (the blue already used by "Support the lab"). |

## 1. The system you are working in

### 1.1 One binary, two kinds of origin

Francois is one Go binary (`cmd/francois`) that serves several hostnames. Read `README.md` and `doc/production.md` for the deployment; for this ticket you need this picture:

```
                     ┌─────────────────────────── Francois (Go) ───────────────────────────┐
 browser ──HTTPS──▶  │ portal  slop.boutique                                                │
                     │   server-rendered pages: /, /apps/{id}, /pricing, /privacy …        │
                     │   (html/template in pkg/catalog/web/templates, layout.tmpl)          │
                     │   React SPA for signed-in tools: /account /studio /admin /instances/ │
                     │   JSON API: /api/session, /api/instances/…, /api/demos, /api/studio… │
                     │                                                                     │
 iframe   ──HTTPS──▶ │ app origins  <instance>.<app-domain>                                 │
                     │   /launch bootstrap → handshake → /page (the artifact's HTML)        │
                     └───────────────┬─────────────────────────────────────────────────────┘
                                     │ pgx
                              PostgreSQL (catalog_artifacts, releases, media,
                              platform_settings, instances, demos, users …)
                              + blob store on disk (release HTML, media, variants)
```

- **Portal pages** are rendered on the server with Go's `html/template`. `pkg/catalog/pages.go` has the handlers and the template functions (`pageFuncs`), `pkg/catalog/web/templates/*.tmpl` the markup, `pkg/catalog/web/static/catalog.css` the styles. A small script, `ui/catalog/public.ts` (bundled by `scripts/build.mjs` into `pkg/catalog/web/static/public.js`), adds the session-dependent parts: run buttons, sort auto-submit, the archive hover preview.
- **The SPA** (`ui/catalog/main.tsx`, React + RTK Query in `ui/catalog/api.ts`) is used for `/account`, `/studio` (editors manage artifacts), `/admin` (staff settings) and `/instances/{id}` (a full-viewport viewer).
- **Applications never run on the portal origin.** Each running copy (an *instance*) gets its own subdomain under the app domain. The portal embeds it in an iframe and hands it a short-lived code through `postMessage` (the *launch handshake*, §1.5). This isolation is the reason a "new tab" launch must open a portal page, not the app origin (§4.4).

### 1.2 Artifacts, releases, pages, media

| Concept | Where | What it is |
|---|---|---|
| Artifact | `store.Artifact`, table `catalog_artifacts` (`pkg/catalog/store/releases.go`) | The product: `ID` (slug), `Title`, `Description`, `Tags`, `Visibility` (`listed`/`hidden`), `Page` (published Markdown + `Cover` + `Gallery` media IDs), `PageDraft`, `Featured`, `SortOrder`, `EarlyAccessUntil`, `DemoEnabled`. |
| Release | `store.Release` | An immutable version: files (one `role:"html"` file with `Name`, `Size`, `ID`), attachments, manifest (capabilities), `State` (`draft`/`published`/…), `FreeSample` (`downloadAccess`), `PublishedAt`. |
| Media | `store.Media`, `/media/{id}` | Uploaded images/videos used as covers and gallery items; still images get resized variants (`?w=160|320|640|1200&v=2`, see FRANCOIS-021). |
| Settings | `settings.Document`, table `platform_settings` | One versioned JSON document of platform policy: operations, models, limits, plans, demo, **storefront** (§1.6). |

An artifact is **on the storefront** when it is `listed` and has at least one published release (`Server.cards` with the `listed` filter in `pages.go`).

### 1.3 Who may run or download: entitlements

`pkg/catalog/entitlement.go` turns a user into an `Entitlement` with a `Source` (`guest`, `disabled`, `free`, `paid`, `staff`, …) and a plan. The rules this ticket relies on:

```go
// entitlement.go
func (e Entitlement) CanRun(a store.Artifact, doc settings.Document) bool {
    switch e.Source {
    case SourceDisabled, SourceGuest: return false
    case SourceStaff:                 return true
    }
    return !a.EarlyAccess(time.Now()) || e.Paid()   // early access = paying only
}
```

- `a.EarlyAccess(now)` is true while `EarlyAccessUntil` is in the future (`store/releases.go`).
- **Download** (`Server.mayDownload` in `routes.go`): anyone may download a `FreeSample` release; otherwise the caller must be signed in and `CanRun` must hold. Unauthenticated requests get `401 login_required`, others `403 plan_required` (JSON).
- **Demos** let guests try an app: settings `demo.enabled` **and** artifact `DemoEnabled` (`demos.go`, `POST /api/demos`), rate-limited per user and address, with a TTL and a global `maxActive`. Each demo start creates a guest session and an instance, so demos are a budget, not free.

### 1.4 The pages involved today

**Landing page** (`GET /`, `GET /apps`): handler `Server.feedPage` (`pages.go`).

```
feedPage(r):
  cards  := s.cards(ctx, listed)          // latest published release per artifact, cover, number
  q, tag := query params; order := "oldest" | "newest"
  tags   := every tag of every card, A–Z, each with a link keeping q and sort
  rows   := cards matching tag (exact) and q (substring of title+description+tags)
  if newest: reverse rows                 // cards are sorted by first publication
  render "home" with Body{Rows, Total, Tags, Tag, Query, Sort, AllHref, LoginError}
```

Template `home.tmpl` renders `main.archive`: a GET `form.archive-controls` (search, categories, sort select), `a.archive-row` per card (number, title, tags, date, a 76 px thumbnail) and the hover preview, then `{{template "support-callout" .}}`.

**App page** (`GET /apps/{id}`): `Server.artifactPage` → `artifact.tmpl`. Header (meta line, title, "Get the file ↓" when allowed, description, byline), then the **runner**:

```html
<section class="experiment-runner" data-runner>
  toolbar: [data-runner-state]"Preview" "in your browser"  [Reload][Full screen] (hidden)
  <div data-runner-surface> cover (hero partial) </div>
  <div class="runner-start" data-actions data-artifact data-release data-title
       [data-demo="true"] [data-early-access-until]>  "Sign in to run" (no-JS fallback) </div>
  <a data-runner-link hidden>Open full screen ↗</a>
</section>
```

then the Markdown sections, gallery, attachments and the support callout.

**Layout** (`layout.tmpl`, template `layout`): `<header class="site-header">` with wordmark, the hard-coded `site-statement`, `site-nav` (Account/Sign in, Support the lab or the supporter thank-you); the page content; `<footer class="site-footer">` (brand, contact, For humans, Information, type credit). Partials: `support-callout` (hidden for supporters), `media`, `hero`, `thumb`, `mini`, `cards` (legacy, used only by `creator.tmpl`).

### 1.5 Running an application: the launch flow

`ui/catalog/public.ts` → `renderActions(container)` decides which button to show, from `GET /api/session` (`plan`, `csrfToken`, …):

```
canRun = plan && (plan.source == "staff" ||
         plan.source ∉ {guest, disabled} && (!earlyAccess || plan.paid) && plan.instances > 0)
if earlyAccess && !plan.paid      → "Get early access →" (/pricing)
else if !canRun && demo != "true" → "Sign in to run" (/signin?return=…)
else                              → button "Start experiment →"
```

On click:

```
sequenceDiagram (text)
 user        portal page (public.ts)            portal API                      app origin iframe
  │ click ─────▶│ POST /api/instances/open {releaseId,title}  (subscriber)          │
  │             │   or POST /api/demos {artifactId}           (guest + demo)        │
  │             │◀── {id} ────────────────────────────────────│                     │
  │             │ GET /api/instances/{id} ──▶ {origin, launchBlocked}               │
  │             │ create <iframe sandbox src=origin+"/launch">─────────────────────▶│
  │             │◀────────────── postMessage francois-launch-ready {challenge} ─────│
  │             │ POST /api/instances/{id}/launch {challenge} ─▶ {code} (60 s)      │
  │             │──────────────── postMessage francois-launch-code {code} ─────────▶│
  │             │                                     POST /api/launch/exchange ◀───│
  │             │◀────────────── postMessage francois-launch-started ───────────────│
  │ sees app    │ state "Live", Reload / Full screen / "Open full screen ↗" shown    │
```

- Instance creation: `instances.go` `instancesOpen` (checks `CanRun`, plan instances, `operations.acceptNewVisits`; `Store.OpenWorkspace` resumes the user's existing workspace for the artifact or creates one).
- Demo creation: `demos.go` `createDemo` (403 `demo_disabled` unless both demo flags are on and the artifact is listed).
- Handshake helper: `ui/catalog/instance-launch.ts` `bindInstanceLaunch(frame, origin, launch, onState)`; child side `ui/catalog/launch.ts` + `launch-protocol.ts` (PKCE S256); server side `pkg/catalog/hosted/routes.go` (`/launch` with `frame-ancestors <portal>`, `/api/launch/exchange`, `/page`).
- **Important constraint:** `launch.ts` refuses to start when it is not inside a frame ("Open this application from your Francois account."). The full-screen experience is therefore the portal page `/instances/{id}` (`ui/catalog/instances.tsx` `Launcher`), which frames the instance in a full-viewport `.viewer`.

### 1.6 Settings: the closest thing to a CMS today

`platform_settings` stores **revisions** of one JSON document (`settings.Document`). `Store.ActiveSettings` returns the newest; `Store.SaveSettingsIf(actor, base, doc)` validates and appends a revision only if `base` is still the newest (optimistic concurrency). The admin API is `GET /api/admin/settings` and `PATCH /api/admin/settings` with `If-Match: <revision>` (`admin.go`), edited in the SPA (`ui/catalog/admin.tsx`, "Storefront" section around the journal/title/tagline/intro fields), by the CLI (`francois catalog settings show|apply`) and by the MCP tools (`pkg/catalog/mcpserver/tools.go`).

```go
type Storefront struct {          // settings.go
    JournalEnabled bool   `json:"journalEnabled"`
    Title          string `json:"title"`    // brand in header/titles
    Tagline        string `json:"tagline"`  // meta description, RSS, share card
    Intro          string `json:"intro"`    // Markdown; currently not rendered anywhere
}
```

`settings.Parse` is strict (unknown fields fail) and `Document.Validate` bounds every field. Hard-coded copy outside the document today: the header statement and the archive intro (`layout.tmpl`, `home.tmpl`, and the `Description` in `feedPage`), the support callout, the pricing page (`pricing.tmpl`), the sign-in page, the "Sign in to run"/"Start experiment" labels (`public.ts`).

## 2. The reference: html.moldandyeast.com

Its `index.html`, `styles.css` and `js/app.js` were studied on 2026-10-23; they are not copied into this public repository (see `sources/README.md`). Anatomy at 1440 px:

```
┌──────────────────────────────────────────────────────────────────────────────────┐
│ ■ 21:20  html.moldandyeast.com/type to search …                Index  Agents  ⌘K │  .bar
│                                                                                  │
│ Stop prompting. Start pointing at artefacts:                                     │  .hero h1
│ single HTML files you hand to your agent with “like this, but…”                  │  68px/1.03, -0.035em
│                                                                                  │
│ Newest first ▾                                                                   │  .controls .sort
│ All 22 · Procedural 2 · 2.5d 1 · Animation 1 · …                                 │  .tags .tag
│ ──────────────────────────────────────────────────────────────────────────────── │
│ October 2026    ┌──────────────┐  Weft                                           │  .group-label | .ri
│ 22 artefacts    │  4:3 thumb   │  A shader language whose pixels are glyphs, …   │  .ri-thumb .ri-title
│                 │              │                                                 │  .ri-desc
│                 └──────────────┘  Shader · Tpy · Glyphs · Terminal               │  .ri-tags
│                                   Open  Download  /a/weft · 1018 KB              │  .ri-actions .ri-path
│                 ─────────────────────────────────────────────────────────────── │
│                 ┌──────────────┐  Shop …                                         │
└──────────────────────────────────────────────────────────────────────────────────┘
```

- **Tokens** (`:root` in `styles.css`): white wall, ink `#111`, text `#1D1D1D`, muted `#737373`, line `#EEE`, plinth `#F4F4F4`; ABC Areal / Areal Mono; body 17/22, heading 40/42 (-0.03em), display `clamp(32px, 4.72vw, 68px)` / 1.03 / -0.035em; label column 300 px, gaps 40 px.
- **Rows**: thumbnail `aspect-ratio: 4/3`, a 1.015 zoom on hover; text column with head (title, description) and foot (tags, actions) pushed apart; a `data-mode="narrow"` variant stacks image over text.
- **Open** (`/?open=slug`) shows a full-viewport `dialog.viewer`: title + description, **Close · Esc**, the iframe in a grey "table", device sizes **Phone / Tablet / Desktop / your screen** with the pixel size, and **Info · Copy HTML · Download · size**. The URL changes so the open state can be shared and Back closes it.
- It is a client-rendered SPA (rows are absolutely positioned by a text-measuring layout engine). **We will not copy that**; we render on the server and use CSS grid. Our `catalog.css` already contains an older port of these tokens and `.ri`/`.viewer` rules (FRANCOIS-002), which we reuse.

## 3. Design: the landing page browser

### 3.1 Page structure

```
layout header (unchanged)  ── slop.boutique  A lab for computation …  Account  Support the lab
main.browser
  section.browser-hero      h1 = copy "home.hero"
  form.browser-controls     GET /   (works without JavaScript)
     [search input q]  [sort select: Newest | Oldest | A–Z | Smallest | Largest]  "14 experiments"
     nav.tags: All 14 · 2.5d 1 · 3d 2 · comics 2 · …     (links keep q and sort)
  section.browser-list
     for each group:
       div.group  ┌ group-label: "October 2026" / "9 experiments" ┐  rows…
       article.ri  thumb | title, description | tags | Open · Download · path · size
  p.browser-empty (no matches) + "Clear filters"
support-callout (unchanged, hidden for supporters)
layout footer (unchanged)
```

The old number/date columns and the hover preview go away (the thumbnail is now always visible). `ordinal .Number` stays on the app page meta line.

### 3.2 View model and server logic

Add the fields the row needs to `card` (`pages.go`), filled in `cards()`/`cardOf()` from the latest release:

```go
type card struct {
    … existing fields …
    LaunchMode   string // "inline" | "tab"                       (§4)
    EarlyAccess  bool   // a.EarlyAccess(now)
    HTMLName     string // release html file name, e.g. "mimeo-0.1.0.html"
    HTMLSize     int64  // bytes, for "· 973 KB" and size sorting
    DownloadHref string // "/api/releases/{rel}/files/{file}/download"
    FreeSample   bool
}
```

`feedPage` becomes (pseudocode):

```
feedPage(r):
  cards := s.cards(ctx, listed)
  q, tag := params; order := one of newest|oldest|az|small|large (default newest)
  e := entitlement of the principal (nil for guests)
  matches := filter(cards, tag, q)               // unchanged rules
  sort matches by order:
     newest/oldest: FirstPublished (then ID)      az: lower(Title)
     small/large:   HTMLSize (then Title)
  groups := order in {newest, oldest} ? groupBy(month(FirstPublished)) // "October 2026"
          : order == az ? groupBy(upper(first letter of Title))
          : one group labelled "By size"
  tagCounts := for each tag: number of cards carrying it (over all cards, like the reference)
  for each row: row.ShowDownload = !(row.EarlyAccess && !(e != nil && (e.Paid() || e.Source == staff)))
                row.DownloadHref = direct link if mayDownload-equivalent, else "/signin?return=/apps/{id}"
  render "home" Body{Hero: copy("home.hero"), Groups, Total, Count, Tags(with counts), Tag, Query, Sort, AllHref, LoginError}
```

**Download rule (decided):** the link is hidden only when the app is in early access and the visitor is not a paying subscriber (staff count as paying). Otherwise it is shown. For visitors who cannot download directly (a guest and a subscriber-only release) the link goes to sign-in with a return to the app page, so nobody lands on a JSON 401. The server keeps enforcing `mayDownload`.

**Early access in the list (decided):** when `row.EarlyAccess` and the visitor is not paying, the row shows, in place of Download, a line *"Early access for subscribers until 12 November. Subscribe →"* (copy key `earlyaccess.row`, link to `/pricing`). **Open** on such a row goes to the app page, which shows the same invitation next to its cover (§4.1). Signed-in free users and guests get the same message; staff and paying users never see it.

**Path · size:** `slop.boutique/apps/{id} · 973 KB`, the host from `s.site(r).PublicURL.Host`, size formatted by the existing `size` template function.

### 3.3 Template sketch (`home.tmpl`)

```html
<main class="browser" aria-label="Experiments">
<section class="browser-hero"><h1>{{copy "home.hero"}}</h1></section>
<form class="browser-controls" method="get" action="/" role="search">
  <div class="browser-views">
    <input class="browser-search" name="q" value="{{.Body.Query}}" type="search" placeholder="{{copy "home.search"}}">
    <select name="sort" data-archive-sort> … newest/oldest/az/small/large … </select>
    <span class="result-count" data-count>{{.Body.Count}} experiments</span>
  </div>
  {{if .Body.Tag}}<input type="hidden" name="tag" value="{{.Body.Tag}}">{{end}}
  <nav class="tags" aria-label="Filter by tag">
    <a class="tag" href="{{.Body.AllHref}}" {{if not .Body.Tag}}aria-current="page"{{end}}>All <span class="tag-count">{{.Body.Total}}</span></a>
    {{range .Body.Tags}}<a class="tag" href="{{.Href}}" …>{{.Name}} <span class="tag-count">{{.Count}}</span></a>{{end}}
  </nav>
</form>
<section class="browser-list" data-browser>
{{range .Body.Groups}}<div class="group">
  <p class="group-label"><span>{{.Label}}</span><span>{{.Count}} experiments</span></p>
  <div class="group-rows">{{range .Rows}}
  <article class="ri" data-row data-id="{{.ID}}" data-launch="{{.LaunchMode}}" data-search="{{lower (print .Title " " .Description " " (join .Tags " "))}}">
    <a class="ri-thumb" href="/apps/{{.ID}}" tabindex="-1">{{if .Cover}}{{template "thumb" .Cover}}{{else}}<div class="mono-thumb">…</div>{{end}}</a>
    <div class="ri-text">
      <div class="ri-head"><h2 class="ri-title"><a href="/apps/{{.ID}}">{{.Title}}</a></h2><p class="ri-desc">{{.Description}}</p></div>
      <div class="ri-foot"><p class="ri-tags">{{join .Tags " · "}}</p>
        <p class="ri-actions">
          <a class="u" href="/apps/{{.ID}}" data-open data-artifact="{{.ID}}" data-release="{{.ReleaseID}}" data-title="{{.Title}}" {{if .Demo}}data-demo="true"{{end}} {{if .EarlyAccessUntil}}data-early-access-until="…"{{end}}>Open</a>
          {{if .ShowDownload}}<a class="u" href="{{.DownloadHref}}" {{if .Direct}}download="{{.HTMLName}}"{{end}}>Download</a>{{end}}
          <span class="ri-path mono">{{$.Host}}/apps/{{.ID}} · {{size .HTMLSize}}</span></p></div>
    </div>
  </article>{{end}}</div></div>
{{else}}<div class="browser-empty"><p>No experiments match your search.</p><a class="u" href="/">Clear filters</a></div>{{end}}
</section>
{{template "support-callout" .}}
</main>
```

Without JavaScript every link works: **Open** goes to the app page, search/sort/tags submit the GET form.

### 3.4 CSS

- New section `/* Browser (FRANCOIS-022) */` in `catalog.css`, scoped under `.browser` so the app page, pricing and the SPA are unaffected.
- Port from the reference: `.hero h1` (display size/tracking), `.controls/.views/.sort/.result-count`, `.tags/.tag/.tag-count` (dot separators via `::after`, `aria-current` underline), `.group-label`, `.ri*` and the narrow-mode rules. Reuse the existing tokens of `.final-site` (warm greys, `--pad: 48px`) so the page sits with our header and footer; do not import their colours wholesale.
- Layout with CSS grid instead of absolute positioning:

```css
.browser .group { display:grid; grid-template-columns: var(--label, 300px) 1fr; column-gap: var(--gap, 40px);
                  border-top: 1px solid var(--color-ink); }
.browser .ri    { display:grid; grid-template-columns: minmax(200px, 402px) 1fr; gap: 40px;
                  padding: 24px 0 32px; border-bottom: 1px solid var(--color-line); }
.browser .ri-thumb { aspect-ratio: 4/3; }         /* covers are 8:5; object-fit: cover crops the sides */
@media (max-width: 900px) { .browser .group { grid-template-columns: 1fr; }
                            .browser .ri { grid-template-columns: 1fr; } }
```

- Thumbnails use the `thumb` partial (640 px + srcset, `sizes` updated to the 402 px column), which benefits from the FRANCOIS-021 variant fixes.
- Delete the archive rules that become unused (`.archive-row`, `.archive-thumb`, `.archive-preview`, …) and the dead templates `catalog.tmpl` and `journal.tmpl` (never rendered). Keep `.archive-*` rules that the journal branch of `home.tmpl` still uses, or move the journal to its own small template.

### 3.5 Client behaviour (`public.ts` → `bindBrowser`)

- **Live filter:** typing in the search box hides rows whose `data-search` does not contain the query, hides empty groups, and updates the count; Enter submits the form (server result is identical). Debounce 80 ms. `history.replaceState` keeps `?q=` in the URL.
- **Sort and tags** stay server-side links (simple, cacheable, no state duplication).
- **Open** (§4.3) is intercepted only when JavaScript can start the app; otherwise the link navigates.
- Remove the hover-preview code from `bindArchive`.

## 4. Design: per-app launch mode

### 4.1 Semantics

| | `inline` | `tab` |
|---|---|---|
| Intent | Small, self-contained pieces that read well in a window (games, toys, single tools). | Large tools that want the whole screen (IDEs, workbenches). |
| **Open** in the list | Opens the viewer overlay on the index and starts the app there. | Opens `/instances/{id}` full screen in a new tab. |
| App page, visitor who may run | Runner starts on page load (no cover, no click). | Cover; **Start** opens a new tab full screen. |
| App page, guest with demo | Cover + "Try it →"; starts in the runner on click. | Cover + "Try it →"; opens a new tab on click. |
| Guest without demo | "Sign in to run" (as today). | Same. |
| Early access, visitor not paying (guest or free) | No start. Message *"In early access for subscribers until {date}. Subscribe to run it now."* with a blue **Subscribe →** button to `/pricing` (copy keys `earlyaccess.runner`, `earlyaccess.button`). Replaces today's muted note + "Get early access →". | Same. |
| Default | — | **Default for all existing artifacts** (no visible change until an editor switches). |

Rationale for the guest rule: a demo creates a guest session and an instance and is rate-limited; starting one on every page view would burn the demo budget on crawlers and drive-by visits. Explicit clicks (Open, Try it) are fine.

### 4.1a The launch button and the blue token

Today the blue `#cee5ff` is hard-coded three times in `catalog.css` (`.site-support`, `.support-callout`, `.support-monthly > .site-button`). Introduce a token and use it for every primary call to action:

```css
.final-site { --color-accent: #cee5ff; --color-accent-hover: #b9daff; }
.site-button.is-primary { background: var(--color-accent); }
.site-button.is-primary:hover { background: var(--color-accent-hover); filter: none; }
```

- Replace the three literals with `var(--color-accent)`.
- `public.ts` renders the runner's **Start experiment →**, **Try it →** and **Subscribe →** with `class="site-button is-primary"`; the list's **Open** stays a text link (the row is already a strong target), and the viewer has no button.
- The SPA (`/instances/{id}`) does not show a launch button, so it needs no change.

### 4.2 Data model and API (end to end)

1. **Migration** `pkg/catalog/store/migrations/020_launch_mode.sql`:

```sql
ALTER TABLE catalog_artifacts
  ADD COLUMN launch_mode text NOT NULL DEFAULT 'tab'
  CHECK (launch_mode IN ('inline','tab'));
```

2. **Store** (`store/releases.go`): `Artifact.LaunchMode string`; add to `artifactColumns`, `scanArtifact`, `CreateArtifact` (default `tab`), `UpdateArtifact`.
3. **Studio API** (`studio.go`): `PATCH /api/studio/artifacts/{id}` accepts `"launchMode": "inline"|"tab"`; validate (`422 invalid_request` otherwise); include in `artifactSummary` and the audit entry. `GET /api/catalog` (public JSON, `routes.go`) adds `launchMode`.
4. **Go client / CLI / MCP**: `client/types.go` (`Artifact`, `ArtifactRow`, `ArtifactPatch.LaunchMode *string`); CLI `francois catalog artifacts update <id> --launch-mode inline|tab` (`cmd/francois/cmds/api_commands.go`); MCP `update_artifact` input `launchMode` (`mcpserver/tools.go`).
5. **Studio UI**: `ui/catalog/studio-api.ts` types; a "Launch" select in the Details form (`studio.tsx`) with help text: "Inline: starts immediately in the page. New tab: shows the cover; starting opens full screen in a new tab."
6. **Pages**: `card.LaunchMode` → `data-launch` on list rows and on `div.runner-start` in `artifact.tmpl`.
7. **Contract tests**: `tests/contracts.test.ts`, `client_contract_test.go`, `storefront_test.go` `TestContentStore` round-trip.

### 4.3 Client flows (pseudocode)

Shared helper extracted from today's `renderActions`:

```ts
// ui/catalog/public.ts
async function openInstance(el: DataSet): Promise<{id: string, origin: string}> {
  const s = await session();
  const viaDemo = !canRun(s, el) && el.demo === "true";
  const created = viaDemo ? await post("/api/demos", {artifactId: el.artifact})
                          : await post("/api/instances/open", {releaseId: el.release, title: el.title});
  if (viaDemo) { resetSession(); await session(); }       // new guest CSRF token
  const inst = await getJSON("/api/instances/" + created.id);
  if (inst.launchBlocked) throw new Error(inst.launchBlocked);
  return {id: created.id, origin: inst.origin};
}
function mountFrame(surface, inst, onState) { /* iframe + bindInstanceLaunch, as today */ }
```

**List row Open** (`bindBrowser`):

```
on click [data-open]:
  if !jsCanStart(row) → let the link navigate to /apps/{id}           // sign-in / early access UI lives there
  preventDefault
  if row.launch == "tab":
     w = window.open("about:blank", "_blank")      // must happen synchronously in the click (popup blockers)
     try { inst = await openInstance(row); w.location = "/instances/" + inst.id }
     catch (e) { w.close(); showError(row, e) }
  else: // inline
     viewer.open(row)                                // dialog, history.pushState("?open=" + id)
     inst = await openInstance(row); mountFrame(viewer.surface, inst, viewer.setState)
on popstate / Esc / Close: viewer.close()            // remove iframe, history back to the list URL
on page load with ?open={id} and an inline row: open the viewer only if canRun (no demo on load)
```

`jsCanStart(row)` = `canRun(session, row)` or (`row.demo == "true"` and not early-access-blocked).

**App page** (`renderActions`), adding the launch mode:

```
mode = container.dataset.launch
if mode == "inline" && canRun:            start immediately (no button), as the click handler does today
else show the button ("Start experiment →", or "Try it →" for demos); on click:
     mode == "tab"    → window.open about:blank, openInstance, w.location = /instances/{id}
     mode == "inline" → today's in-page start
```

`/instances/{id}` already provides the full-screen viewer (Close, Esc, launch handshake). Its Close link goes to `/`; for tab launches it should go to the app page (`/apps/{artifactId}`) or close the tab; add `artifactId` to the instance JSON if it is not there.

### 4.4 The index viewer overlay

A server-rendered empty `<dialog class="viewer" data-viewer>` in `home.tmpl`, filled by `public.ts`:

```
┌ dialog.viewer (full viewport, grey table) ──────────────────────────────────────┐
│ Title  description…                                          Details · Close · Esc │  .v-top
│ ┌───────────────────────── iframe (instance origin /launch) ───────────────────┐ │  .v-table .v-device
│ │                                                                              │ │
│ └──────────────────────────────────────────────────────────────────────────────┘ │
│ Phone  Tablet  Desktop                    Reload · Full screen · Download · size │  .v-bottom
└──────────────────────────────────────────────────────────────────────────────────┘
```

- Reuses `.viewer`/`.v-*` CSS that already exists for `/instances/{id}`; device sizes set the iframe width (390 / 834 / 100 %).
- Uses the same `openInstance` + `mountFrame`; a failure shows the message inside the table with a link to the app page.
- The portal CSP already allows framing the app domain (`frame-src` in `routes.go` security headers); nothing changes server-side.
- Accessibility: `dialog.showModal()`, focus the Close button, `Esc` closes unless focus is inside the iframe (the same rule as `Launcher`), `aria-label` from the title.

## 5. Design: editable site copy (a small CMS)

### 5.1 Approach

We do not need pages, layouts or a WYSIWYG editor; we need **named strings** that staff can change without a deploy, with history. The settings document already provides storage, revisions, optimistic concurrency, an admin screen, CLI and MCP access, so copy lives there:

```go
// settings.go
type Storefront struct {
    … existing …
    Copy map[string]string `json:"copy,omitempty"` // overrides of registered copy keys
}
```

A Go **registry** defines which keys exist, their default text, their kind and limits. Unknown keys are rejected by `Validate`, so a typo cannot silently do nothing.

```go
// pkg/catalog/settings/copy.go
type CopyKey struct {
    Key, Label, Help string
    Kind     string // "text" (single line) | "markdown" (rendered with the page Markdown renderer)
    Max      int    // characters
    Default  string
}
var CopyKeys = []CopyKey{
  {Key: "home.hero", Label: "Landing page headline", Kind: "text", Max: 240,
   Default: "These artifacts are the results of our research. Play with them, download the HTML, and drop them into your coding agent."},
  {Key: "home.search",          Kind: "text", Max: 80,  Default: "Search experiments"},
  {Key: "header.statement",     Kind: "text", Max: 200, Default: "A lab for computation and intelligence as material and medium. Run by Manuel and Ramon."},
  {Key: "callout.label",        Kind: "text", Max: 60,  Default: "Support the lab"},
  {Key: "callout.heading",      Kind: "text", Max: 120, Default: "Keep the lab running."},
  {Key: "callout.body",         Kind: "markdown", Max: 2000, Default: "We prototype and explore, and share the work here. …"},
  {Key: "callout.button",       Kind: "text", Max: 60,  Default: "Support the lab →"},
  {Key: "runner.start",         Kind: "text", Max: 60,  Default: "Start experiment →"},
  {Key: "runner.demo",          Kind: "text", Max: 60,  Default: "Try it →"},
  {Key: "runner.signin",        Kind: "text", Max: 60,  Default: "Sign in to run"},
  {Key: "earlyaccess.row",      Kind: "text", Max: 160, Default: "Early access for subscribers until {date}."},
  {Key: "earlyaccess.runner",   Kind: "text", Max: 200, Default: "In early access for subscribers until {date}. Subscribe to run it now."},
  {Key: "earlyaccess.button",   Kind: "text", Max: 60,  Default: "Subscribe →"},
}
// {date} is replaced with the early-access end date ("12 November 2026").
func (d Document) CopyText(key string) string   // override if non-empty, else default
```

The first version registers the hero, the header statement and the support callout (the strings visible on every public page); pricing and sign-in copy can be added later by appending keys, with no schema change.

### 5.2 Rendering

- `render()` (`pages.go`) already loads the active settings; it puts a `copy` function into the template data (or a `Copy` map with defaults applied) so templates write `{{copy "home.hero"}}`. Markdown keys go through the existing Markdown renderer (`s.md`) and are emitted as `template.HTML`.
- `feedPage`'s hard-coded `Description` and the `layout.tmpl` statement switch to `header.statement`. The unused `Storefront.Intro` stays as is (out of scope) or is removed in a follow-up.
- Runner labels needed by `public.ts` are passed as `data-label-*` attributes on the runner container, so the script needs no second request.

### 5.3 Editing

- **Admin UI** (`ui/catalog/admin.tsx`): a "Site copy" panel in the settings screen listing every registered key (label, help, default as placeholder, a textarea for Markdown keys, a character counter, "Reset to default" clears the override). It saves through the existing `PATCH /api/admin/settings` with `If-Match`, so concurrent edits are detected.
- `GET /api/admin/copy-keys` (staff) returns the registry so the UI does not duplicate defaults.
- CLI: `francois catalog settings show` / `apply` already round-trip the whole document; add `francois catalog copy list` and `francois catalog copy set <key> <text|--file>` as conveniences (read, modify `storefront.copy`, apply with the base revision).
- MCP: `update_settings` gains `copy` (map) in its storefront section.

### 5.4 Validation

```
for key, value in doc.Storefront.Copy:
    k := registry[key] or error "storefront.copy: unknown key %q"
    utf8.ValidString(value) and RuneCount(value) <= k.Max  or error
    k.Kind == "text" → no newlines
len(json(doc)) stays well under the 64 KiB PATCH limit (all defaults together are < 4 KiB)
```

## 6. Implementation plan

Work on branch `feat/storefront-browser`; one commit per phase; keep the diary up to date.

**Phase 1 — Site copy foundation**
- `settings/copy.go` registry + `Storefront.Copy` + validation + unit tests (`settings_test.go`: unknown key, too long, newline in text, defaults).
- `copy` template function, header statement and support callout switched to keys; `feedPage` description.
- `GET /api/admin/copy-keys`; admin "Site copy" panel; CLI `catalog copy list|set`; MCP field.
- Tests: `layout_test.go` (statement override renders, default otherwise), `support_test.go` (callout copy).

**Phase 2 — Launch mode field**
- Migration 020, store, studio PATCH + validation + audit, `/api/catalog`, client types, CLI flag, MCP, Studio select.
- Tests: `studio_test.go` (patch inline/tab, reject other values), `TestContentStore` round-trip, contract tests.

**Phase 3 — Landing page browser (server)**
- `card` fields (size, download, early access, launch mode), sort orders `az|small|large`, groups, tag counts, download rule, `home.tmpl` rewrite, CSS section, delete dead templates/CSS.
- Early-access row message and the `--color-accent` token (replace the three literals).
- Tests: extend `final_archive_test.go` (early-access row shows the subscribe line for guests and free users and not for paid/staff; groups by month, counts, each sort order, tag counts, download hidden in early access for guests/free and shown for paid/staff, sign-in link for guests on subscriber-only releases, Open/Details links, no-JS form), `TestSupporterPresentation` still passes.

**Phase 4 — Client: live filter, Open, viewer, tab launch**
- `public.ts`: early-access branch shows the message + blue **Subscribe →**; primary buttons get `is-primary`; `openInstance`/`mountFrame` refactor, `bindBrowser` (filter, Open), viewer dialog with history and device sizes, app-page inline auto-start and tab launch; Close on `/instances/{id}` returns to the app page.
- Tests: TypeScript unit tests for the decision table (`canRun`, `jsCanStart`, mode × visitor → action) in `tests/`; existing `instance-launch.test.ts` unchanged.
- Manual/Playwright check against a local server with a subscriber, a free user, a guest with demo and a guest without demo; screenshots side by side with the reference at 1440 px and 390 px.

**Phase 5 — Rollout**
- Deploy (GitOps image bump in the k3s repo), set `launchMode` per app with the CLI, e.g. `inline` for small pieces, `tab` for Kiln/Octavo/Form Skate/Mimeo/Living Comic/Hypercore; set the hero via the admin panel or leave the default.
- Smoke test on slop.boutique: list renders, filter, Open in both modes as subscriber and guest, downloads per rule.

## 7. Risks and open points

- **Popup blockers**: the new tab must be opened synchronously inside the click handler (`window.open` before any `await`), then pointed at `/instances/{id}`; otherwise browsers block it.
- **Auto-start cost**: inline auto-start on the app page creates/resumes a workspace for every visit by a subscriber; `OpenWorkspace` resumes the existing one, so this is bounded by one workspace per user and artifact.
- **Back/forward cache**: when the overlay is open and the user navigates away, the `pagehide` handler must unbind the launch listener (the runner already does this).
- **Search index for JS filter**: `data-search` repeats title/description/tags per row; fine for tens of rows. Revisit with a JSON index if the catalogue grows to hundreds.

## 8. Reference

### 8.1 Files

| File | Role in this ticket |
|---|---|
| `pkg/catalog/pages.go` | `feedPage`, `artifactPage`, `cards`, `cardOf`, `card`, `pageFuncs`, `render` |
| `pkg/catalog/web/templates/home.tmpl` | Landing page markup (rewritten) |
| `pkg/catalog/web/templates/artifact.tmpl` | App page runner (`data-launch`) |
| `pkg/catalog/web/templates/layout.tmpl` | Header/footer/partials; statement and callout to copy keys |
| `pkg/catalog/web/static/catalog.css` | New `.browser` section; legacy `.ri`/`.viewer` rules |
| `ui/catalog/public.ts` | `renderActions`, `bindArchive` → `bindBrowser`, viewer |
| `ui/catalog/instance-launch.ts` | `bindInstanceLaunch` (parent side of the handshake) |
| `ui/catalog/instances.tsx` | `/instances/{id}` full-screen `Launcher` |
| `pkg/catalog/instances.go`, `demos.go` | `POST /api/instances/open`, `GET /api/instances/{id}`, `POST /api/instances/{id}/launch`, `POST /api/demos` |
| `pkg/catalog/hosted/routes.go`, `ui/catalog/launch.ts` | App-origin side of the launch |
| `pkg/catalog/entitlement.go` | `CanRun`, `Paid` |
| `pkg/catalog/routes.go` | Route table, `mayDownload`, `download`, `/api/catalog`, security headers |
| `pkg/catalog/store/releases.go`, `store/migrations/` | `Artifact`, artifact SQL, migration 020 |
| `pkg/catalog/studio.go` | Studio artifact API |
| `pkg/catalog/settings/settings.go` (+ `copy.go`) | Settings document, validation, copy registry |
| `pkg/catalog/admin.go`, `ui/catalog/admin.tsx` | Settings API and editor |
| `pkg/catalog/client/*`, `cmd/francois/cmds/api_commands.go`, `pkg/catalog/mcpserver/tools.go` | Go client, CLI, MCP |
| Tests: `final_archive_test.go`, `storefront_test.go`, `layout_test.go`, `support_test.go`, `studio_test.go`, `demos_test.go`, `early_access_test.go`, `tests/*.test.ts` | Existing coverage to extend |

### 8.2 HTTP API touched

| Method and path | Auth | Change |
|---|---|---|
| `GET /` , `/apps` | public | New layout, `sort ∈ newest,oldest,az,small,large`, groups, tag counts |
| `GET /apps/{id}` | public | `data-launch` on the runner |
| `GET /api/catalog` | public | `launchMode` per artifact |
| `PATCH /api/studio/artifacts/{id}` | editor | `launchMode` |
| `GET/PATCH /api/admin/settings` | staff | `storefront.copy` |
| `GET /api/admin/copy-keys` | staff | New: copy registry |
| `POST /api/instances/open`, `POST /api/demos`, `GET /api/instances/{id}`, `POST /api/instances/{id}/launch` | session | Unchanged; reused by Open and tab launch |
| `GET /api/releases/{release}/files/{file}/download` | public | Unchanged rule; linked from the list |

### 8.3 Related tickets

- FRANCOIS-002: first port of html.moldandyeast.com tokens (`:root`, `.ri`, `.viewer`).
- FRANCOIS-006 / 011 / 012: Ramon's Areal "Final" design that produced today's archive, header, footer and runner; 011 already discussed auto-start vs click-to-start for demos.
- FRANCOIS-021: image variants (`thumb` partial quality) used by the new rows.
