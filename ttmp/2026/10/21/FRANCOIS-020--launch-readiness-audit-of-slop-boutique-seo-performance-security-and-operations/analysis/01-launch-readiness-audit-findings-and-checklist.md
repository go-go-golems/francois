---
Title: 'Launch readiness audit: findings and checklist'
Ticket: FRANCOIS-020
Status: active
Topics:
    - frontend
    - performance
    - deployment
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://doc/production.md
      Note: Runbook and known gaps (section 12) that the P0 list draws on
    - Path: repo://pkg/catalog/analytics.go
      Note: Forces private, no-store on browser HTML (bfcache)
    - Path: repo://pkg/catalog/fonts.go
      Note: Unversioned font CSS and woff2 cache headers
    - Path: repo://pkg/catalog/pages.go
      Note: Canonical construction, 404/5xx handling and /media serving
    - Path: repo://pkg/catalog/routes.go
      Note: Portal routes and security-header wrapper; home of robots.txt, sitemap, catch-all 404 and HSTS
    - Path: repo://pkg/catalog/web/templates/layout.tmpl
      Note: Single <head> for all server-rendered pages (og:image, twitter, canonical, fonts)
ExternalSources:
    - https://slop.boutique/
Summary: 'Audit of the live slop.boutique deployment on 2026-10-21: transport security, SEO, performance, content, accessibility, email/DNS and operations, with a prioritized launch checklist and where each fix belongs in the code.'
LastUpdated: 2026-10-21T10:37:25.847562-04:00
WhatFor: Deciding what must change before slop.boutique is announced publicly, and where in the code or GitOps repo each change goes.
WhenToUse: 'Before a public launch, and after each fix: re-run scripts/01-live-audit.sh and compare against the findings here.'
---

# Launch readiness audit: findings and checklist

slop.boutique is fast, server-rendered and already well hardened at the application layer. The gaps are mostly in the plumbing around it:

- Plain HTTP is served without a redirect, and there is no HSTS.
- Crawlers get no `robots.txt` or sitemap, and a second host serves the whole catalog with its own canonicals.
- Homepage thumbnails are 10–20× larger than where they are shown.
- Nothing alerts anyone when the site goes down.

Four items already listed in `doc/production.md` §12 also need a decision before launch: Stripe test mode, same-site instance origins, an unrehearsed restore, and an untested Google sign-in.

Measured on 2026-10-21 against production from a US connection, with curl, `dig @1.1.1.1` and Lighthouse 12.8.2 (mobile, headless Chrome). `scripts/01-live-audit.sh` repeats every live check below.

## Launch blockers (P0)

| # | Finding | Evidence | Where to fix |
| --- | --- | --- | --- |
| 1 | Plain HTTP serves the full site. | `http://slop.boutique/` returns 200 with the page, not a redirect. | Traefik `web` entrypoint redirect in the GitOps repo `wesen/2026-03-27--hetzner-k3s` (manifests are not in this repo, `doc/production.md:17`). |
| 2 | No `Strict-Transport-Security` header on any response. | No match in `pkg`, `cmd`, `deploy` or `doc`. | Portal header wrapper `pkg/catalog/routes.go:107-114`, instance responses `pkg/catalog/hosted/routes.go:61-64`, or a Traefik headers middleware. Start with `max-age=86400; includeSubDomains` and raise it to 1–2 years once #1 is verified. |
| 3 | `/robots.txt` and `/sitemap.xml` are 404 (Go's plain-text default). | `scripts/01-live-audit.sh` | Add both in `pkg/catalog/routes.go` next to the feeds (:38-40). The sitemap can reuse `s.cards(ctx, listed)` (`pages.go:236`), posts and creators. Then submit it in Search Console, where the domain is already verified by TXT record. |
| 4 | `francois.yolo.scapegoat.dev` serves the same 7 experiments, each self-canonical. | Its homepage canonical is `https://francois.yolo.scapegoat.dev/`. Canonicals are built from `site.PortalOrigin()` (`pages.go:162`). | Pick slop.boutique as primary. Either 301 the old host (as `server.go:232-234` does for www) or keep it for staff with `X-Robots-Tag: noindex` and canonicals that point at slop.boutique. |
| 5 | Nothing watches production. There is no uptime check, alerting, error reporting or access log. | Section 8 of the code map; logs are `kubectl logs` only (`doc/production.md:495-505`). | An external uptime check on `https://slop.boutique/` and `/readyz` (public, returns `{"models":true,"status":"ready"}`), an alert on certificate expiry, and an error reporter or at least an alert on 5xx counts. |
| 6 | Backup restore has never been rehearsed. | `doc/production.md:672-688`; FRANCOIS-003 risk table, "Restore drill in Phase 8". | Restore the nightly `pg_dumpall` and the S3 blob sync into a scratch namespace once, and time it. |
| 7 | Decisions recorded as "before a public launch" are still open. | FRANCOIS-003 design doc risk table: "Same-site instance origins — Accepted for the beta — D1 option (c) before a public launch". `doc/production.md` §12 also lists Stripe test mode, Google sign-in never exercised, and `e2e-*` users in the production org. | Decide each: a separate registrable domain for `i-*` instances (or record that only staff publish HTML), Stripe live keys and Price if "Support the lab" launches, one real Google sign-in, and deleting the test users. |

## Should fix before launch (P1)

| # | Finding | Evidence | Where to fix |
| --- | --- | --- | --- |
| 8 | Homepage thumbnails are full-size screenshots. Five images, 782 KB in total (416 KB for Lattice Rift alone), are 1800×1125 shown at 76×54 CSS px. Lighthouse estimates 759 KiB savings on `/` and 375 KiB on `/apps/rag-workbench`. | Lighthouse `uses-responsive-images`. FRANCOIS-010 §10 lists "smaller thumbnails" as the next optimization. | Generate derived sizes when media is uploaded (for example 160w and 320w for thumbnails, 1200w for covers and `og:image`) and emit `srcset`/`sizes` with `width`/`height`. `/media/{id}` is at `pages.go:729-748`. |
| 9 | No default share image. `/`, `/pricing`, legal pages and covers-less experiments (Form Skate Workbench, Polis) have no `og:image`. `twitter:card` is only emitted with an image, and `og:type` is never set. | `layout.tmpl:12` | A site-wide 1200×630 fallback image, `og:type` (`website`, or `article` for posts), and `twitter:card=summary` when there is no image. |
| 10 | Unknown paths return Go's plain-text `404 page not found`, for example `/favicon.ico`, `/creators` and `/no-such-page`. Server errors are plain-text "Page unavailable". | The ServeMux has no catch-all. The styled `notfound.tmpl` only runs from handlers (`pages.go:213-220`). | Add `m.HandleFunc("/", …)` that calls `s.notFound` in `routes.go`, plus a styled 5xx template. |
| 11 | Only an SVG icon exists: no `/favicon.ico` (browsers request it on every new visit), no `apple-touch-icon`, no web manifest. | `layout.tmpl:14`, `web/embed.go:42-47` | Add a 32px ICO, a 180px PNG and a manifest to `portalAssets`. |
| 12 | SPA shell pages (`/studio`, `/account`, `/admin`, `/instances/`, `/thank-you`) are indexable and carry a stale head: "Francois — hosted HTML applications". | `pkg/catalog/web/static/index.html:6-7`, `routes.go:23-30` | Send `X-Robots-Tag: noindex` from the shell route, update the title, and disallow these paths in `robots.txt`. |
| 13 | Instance hosts (`i-<id>.slop.boutique`) have no `noindex`, and `/launch` puts the instance title in `<title>`. | `hosted/routes.go:41`, `:54-70` | `X-Robots-Tag: noindex, nofollow` on every instance response. |
| 14 | `/apps` and `/apps/` render the homepage with their own canonical. | `routes.go:32-34`, live canonical `https://slop.boutique/apps` | Canonical to `/`, or a 301. |
| 15 | `/privacy` and `/terms` have no description, canonical or OG tags, and reference unversioned `/static/*` URLs. | `pkg/catalog/web/legal.tmpl:3-9` | Render them through `layout.tmpl`, or add the head fields and the `asset` function. |
| 16 | The feed reads as internal: the channel description is "Applications.", item descriptions are release notes ("Public download access; application HTML is unchanged."), Form Skate Workbench has an empty description, and there is no `atom:link rel="self"`. | `/feed.xml`, `pkg/catalog/feed.go` | Use the storefront tagline for the channel and the artifact description (or editorial lead) for items. Keep release notes out of the public feed. |
| 17 | Experiment pages are thin for both visitors and crawlers: a one-line description, an empty `experiment-editorial` block, and "Sign in to run". | `/apps/form-skate-workbench` | Content work rather than code: a cover image, a paragraph or two and alt text for every listed experiment. The access policy for anonymous visitors is the open question from the FRANCOIS-008 diary, Step 19. |
| 18 | DMARC is missing. The wildcard `*.slop.boutique CNAME francois.yolo.scapegoat.dev` also answers `_dmarc.slop.boutique`. SPF and Google DKIM are present. | `dig @1.1.1.1 TXT _dmarc.slop.boutique` returns the CNAME | An explicit `_dmarc` TXT record (an explicit name wins over the wildcard), starting with `v=DMARC1; p=none; rua=mailto:…`. DNS is in a separate Cloudflare account, so its owner makes the change. Open question: sign-in mail is sent by ZITADEL through SES (`doc/production.md:773`). Check that its From domain aligns with SPF and DKIM. |
| 19 | Fonts load in two hops and are cached briefly. `fonts.css` is a separate render-blocking request (1 h cache), and five unversioned woff2 files (290 KB; the homepage needs Regular and Medium, 126 KB) get `max-age=604800`. Lighthouse: render-blocking 100–140 ms, and `uses-long-cache-ttl` fails on 3–4 resources. | `pkg/catalog/fonts.go:93-111`, `layout.tmpl:16` | Version fonts through the `Asset` scheme (`web/assets.go`, `immutable`), fold `@font-face` into `catalog.css`, and `preload` the two weights used above the fold. |
| 20 | Accessibility (Lighthouse mobile, 92 on `/`): `.archive-monogram` text fails contrast, and the sort `<select>` has no accessible name on mobile because its `<label>` is `display:none`. Images on experiment pages have no `width`/`height`. | Lighthouse `color-contrast`, `select-name`, `unsized-images` | Darken the monogram color, use the existing `visually-hidden` class instead of `display:none`, and emit image dimensions (stored at upload). |

## After launch (P2)

| # | Finding | Note |
| --- | --- | --- |
| 21 | Browsers receive `private, no-store` on all HTML, so pages are never eligible for the back/forward cache. | Forced by the analytics wrapper (`pkg/catalog/analytics.go:80-87`), which also overrides the legal pages' `max-age=3600`. `private, no-cache` keeps freshness and allows bfcache, but back navigations would no longer be counted as views. |
| 22 | HTML, RSS and JSON are not compressed. Only versioned static assets are gzipped. | Small gain (about 5 KB per page); a gzip middleware around the portal mux. |
| 23 | No structured data. | JSON-LD `WebSite` and `Organization` on `/`, and `SoftwareApplication` or `CreativeWork` on `/apps/{id}`. |
| 24 | The homepage `<title>` is just "slop.boutique". | Add the statement, e.g. "slop.boutique · a lab for computation and intelligence". |
| 25 | US visitors wait about 0.4 s for HTML: 0.12 s RTT to Falkenstein, 140 ms server time. | No CDN by decision; the Cloudflare proxy is unwanted (`doc/production.md:25, 511, 527-531`). Revisit only if traffic is mostly non-EU. |
| 26 | Every deploy is a short outage: single node, one replica, `Recreate` strategy. | Acceptable at launch scale; say so explicitly in the runbook. |
| 27 | No `Permissions-Policy`, COOP or `security.txt`. No CAA record. IPv4 only. | Hardening and hygiene. A CAA record would be `0 issue "letsencrypt.org"`. |
| 28 | Deleted media and attachment blobs are never collected. | `doc/production.md` §12. |

## What is already good

- **Server rendering.** The public pages are server-rendered with a title, description, canonical and RSS link on every page (`layout.tmpl`). The rest of this section is about those pages.
- **Lighthouse (clean runs).**

  | Page | Performance | LCP | Page weight |
  | --- | --- | --- | --- |
  | `/` | 98 | 1.7 s | 912 KiB |
  | `/apps/rag-workbench` | 94 | 2.6 s | 617 KiB |

  - Server response is 140 ms in both runs, with zero blocking time and zero layout shift.
  - A first run was distorted by local network trouble (a 4.3 s document request, not reproducible with curl) and was discarded.
- **Static assets.** They are content-versioned, `immutable` for a year, precompressed gzip, and served with strong ETags (`pkg/catalog/web/assets.go`).
- **Security headers.** The CSP is strict (no inline script or style), with `frame-ancestors 'none'`, `X-Frame-Options: DENY`, `nosniff` and `Referrer-Policy`.
- **Cookies and CSRF.**
  - Cookies are `__Host-` prefixed, `HttpOnly`, `Secure` and `SameSite=Lax` (`auth.go:40-44`).
  - CSRF is checked with a header and an Origin check.
- **Pages kept out of search.** `/signin`, drafts and previews send `noindex`.
- **Domain basics.**
  - `www` redirects 308 to the apex.
  - The wildcard Let's Encrypt certificate is valid until 2027-01-12.
  - The domain is verified in Google Search Console.

## Checklist

P0, before announcing:

- [ ] Redirect HTTP to HTTPS at Traefik (#1)
- [ ] Send HSTS, then raise its `max-age` (#2)
- [ ] Serve `robots.txt` (with a disallow list and a `Sitemap:` line) and `sitemap.xml`, then submit the sitemap in Search Console (#3)
- [ ] Choose the primary host; redirect or noindex `francois.yolo.scapegoat.dev` (#4)
- [ ] Set up uptime checks on `/` and `/readyz`, a certificate-expiry alert, and 5xx or error alerts (#5)
- [ ] Rehearse a restore from the nightly dump and the S3 blobs (#6)
- [ ] Decide on same-site instance origins, Stripe live mode, Google sign-in and the test users (#7)

P1, launch quality:

- [ ] Generate responsive thumbnail and cover sizes with `srcset` and dimensions (#8, #20)
- [ ] Add a default `og:image`, `og:type` and a `twitter:card` fallback (#9)
- [ ] Add a catch-all HTML 404 and a styled 5xx page (#10)
- [ ] Add `favicon.ico`, `apple-touch-icon` and a manifest (#11)
- [ ] Noindex the SPA shell and fix its title (#12)
- [ ] Noindex instance hosts (#13)
- [ ] Fix the `/apps` canonical (#14)
- [ ] Give the legal pages a full head and versioned assets (#15)
- [ ] Clean up the feed copy and add `atom:link self` (#16)
- [ ] Give every listed experiment a cover, editorial text and alt text (#17)
- [ ] Add a `_dmarc` TXT record and check the sign-in mail From alignment (#18)
- [ ] Version, inline and preload the fonts (#19)
- [ ] Fix monogram contrast and the mobile select label (#20)
