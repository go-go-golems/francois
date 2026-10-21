---
Title: 'Launch readiness fixes: retire the old host, transport, crawling and polish'
Ticket: FRANCOIS-020
Status: active
Topics:
    - frontend
    - performance
    - deployment
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/catalog.go
      Note: redirect-host, hsts-max-age, contact-email flags
    - Path: repo://pkg/catalog/launch_test.go
      Note: Integration tests for the design
    - Path: repo://pkg/catalog/pages.go
      Note: Canonical, og:type, contact, fonts, 503 page
    - Path: repo://pkg/catalog/routes.go
      Note: New routes, catch-all 404, SPA noindex
    - Path: repo://pkg/catalog/seo.go
      Note: robots.txt, sitemap, manifest, root icons
    - Path: repo://pkg/catalog/server.go
      Note: Redirect hosts, HTTPS upgrade and HSTS in Handler
ExternalSources: []
Summary: 'Design for fixing the launch-readiness findings: retire the old portal host behind a configurable redirect, enforce HTTPS and HSTS, add robots.txt, a sitemap, icons and HTML error pages, keep non-public surfaces out of search, and take hostnames and contact addresses out of the code.'
LastUpdated: 2026-10-21T18:11:37.397649-04:00
WhatFor: What the launch-readiness PR changes, why each decision was made, and the deployment steps that have to happen outside this repository.
WhenToUse: When reviewing the launch-readiness PR, deploying it, or picking up the deferred items.
---

# Launch readiness fixes: retire the old host, transport, crawling and polish

This design fixes every finding in `analysis/01-launch-readiness-audit-findings-and-checklist.md` that can be fixed in this repository. Two decisions shape it:

- **The old host is retired.** `francois.yolo.scapegoat.dev` stops being a site. Every request to it is redirected to the primary portal.
- **Hosts and addresses come only from configuration.** Code and templates name no hostname or contact address.

Items that live in the GitOps repo, DNS, Stripe, or content are listed at the end with the order they must happen in.

## 1. Retire the old portal host

**Decision.** A new repeatable flag, `--redirect-host <host>`, names hosts that answer every request with **308** to the primary portal origin plus the original path and query.

- 308 keeps the method and body, so a `POST` from an old client reaches the new host as a `POST`.
- The redirect runs in `Server.Handler()` before any site lookup, so it covers every path: pages, `/api`, `/auth`, `/mcp` and feeds.
- `Config.Validate` rejects a redirect host that is also a portal host, `www.` + a portal host, or a host under a configured application domain.

**Why not keep the old host as a second site with noindex.** Two portals mean two session domains, two OIDC callbacks and two sets of absolute URLs to keep consistent. The audit found the second site only duplicated content. One portal is simpler and removes the duplicate-content problem at its root.

**Why not hardcode the hosts.** The server already learns every portal from `--public-url` and `--site`. The redirect follows the same rule: the code knows "this host redirects to the primary", never which host that is.

**Deployment change (GitOps repo `wesen/2026-03-27--hetzner-k3s`, `gitops/kustomize/francois/deployment.yaml`).**

```text
--public-url=https://slop.boutique
--app-domain=slop.boutique
--redirect-host=francois.yolo.scapegoat.dev
(remove --site=https://slop.boutique=slop.boutique)
```

The Ingress and Certificate for `francois.yolo.scapegoat.dev` stay, so the redirect is served over HTTPS. Instance hosts under `apps.francois.yolo.scapegoat.dev` stop resolving to a site and get 421. Instances themselves are site-independent: the same instance is reachable as `i-<id>.slop.boutique`.

**Order of operations.** The redirect breaks anything that cannot follow it:

1. **Stripe webhook.** Stripe does not follow redirects. Create the endpoint `https://slop.boutique/webhooks/stripe` with the same events. Add its signing secret as the *current* secret in `--stripe-webhook-secret-files`, keeping the old one as *previous*. Deploy, check that events arrive at the new endpoint, then disable the old endpoint.
2. **ZITADEL.** The slop.boutique callback is already registered, because it is a site today. Nothing to add. After the switch, remove the old host's redirect URIs.
3. **Swap the flags** as above and deploy.
4. **CLI logins.** Go's HTTP client follows 308 but drops the `Authorization` header when the host changes. A stored login for the old host then fails with 401. Users run `francois catalog auth login --server https://slop.boutique` once.
5. Update `doc/production.md` §2 (topology) to the single site.

## 2. HTTPS and HSTS

**HTTP→HTTPS.** When the primary site is HTTPS and `--trust-proxy-headers` is set (as it is in production), a request carrying `X-Forwarded-Proto: http` gets a 308 to the same URL on `https`.

- This closes the live gap (plain HTTP served the full site) without waiting for an ingress change.
- Kubelet probes carry no forwarded header and are unaffected.
- A Traefik `redirectScheme` middleware on the `web` entrypoint is equivalent and can replace this later.

**HSTS.** New flag `--hsts-max-age` (a duration; default `8760h`; `0` disables).

- When the primary site is HTTPS, every response from `Handler()` sends `Strict-Transport-Security: max-age=<seconds>; includeSubDomains`. That covers portal, instance and redirect responses.
- `includeSubDomains` is safe because every subdomain the server answers for is served under the wildcard certificate.
- Rollout: deploy first with `--hsts-max-age=24h`. Once `https://` works everywhere, drop the flag to get the one-year default.

## 3. Crawling: robots.txt and sitemap.xml

Both are generated per request from the site the request arrived on. With one site after §1, that site is slop.boutique.

**`/robots.txt`** allows everything except the non-public surfaces, and points at the sitemap:

```text
User-agent: *
Disallow: /api/
Disallow: /auth/
Disallow: /account
Disallow: /admin
Disallow: /studio
Disallow: /instances/
Disallow: /signin
Disallow: /thank-you
Disallow: /mcp
Sitemap: <portal origin>/sitemap.xml
```

**`/sitemap.xml`** lists only pages that render 200 for a visitor, using the same rules as those pages:

| URL | Included when | `lastmod` |
| --- | --- | --- |
| `/` | always | newest listed release or published post |
| `/apps/{id}` | artifact `listed` with ≥1 published release (from `s.cards(ctx, listed)`) | that artifact's latest release |
| `/creators/{slug}` | credited on a listed artifact | their newest credited release |
| `/journal`, `/journal/{slug}` | journal enabled and post published | post `published_at` |
| `/pricing`, `/privacy`, `/terms` | always | omitted |

- Only `<loc>` and `<lastmod>` are emitted; search engines ignore `changefreq` and `priority`.
- `Cache-Control: public, max-age=3600`.
- A test requests every `<loc>` through the handler and expects 200, so the sitemap cannot drift from the pages.

**Instance hosts** answer `/robots.txt` with `Disallow: /` and send `X-Robots-Tag: noindex, nofollow` on every response.

## 4. Pages outside search

- **SPA shell** (`/account`, `/studio`, `/admin`, `/instances/`, `/thank-you`) sends `X-Robots-Tag: noindex, nofollow`.
  - Its static `<title>` becomes the product name, "Francois".
  - The stale marketing description is removed.
  - The SPA sets `document.title` to the configured brand once the session loads.
- **`/apps` and `/apps/`** render the homepage. Their canonical becomes `/`.

## 5. Error pages

- **Unknown portal paths.** A catch-all `"/"` route renders the styled 404 for `GET` and `HEAD`. Other methods get a plain 404.
  - The pattern is `"/"`, not `"GET /"`, because `"GET /"` would conflict with the `"/api/"` catch-all in Go's ServeMux.
  - The 404 template's "Back" link goes to `/`.
- **Server failures.** `pageFailed` renders a static "unavailable" template with status 503.
  - It needs neither the settings document nor the session, since either may be what failed.
  - The brand falls back to the site hostname from configuration.

## 6. Head, icons and share metadata

- **`og:type`.** `website`, or `article` on journal posts.
- **`twitter:card`.** `summary_large_image` when there is an image, otherwise `summary`.
- **Default share image (deferred).** There is no site-wide 1200×630 image to use yet; this needs a designed asset (see Deferred).
- **Icons.** `favicon.ico` (16 and 32 px), `apple-touch-icon.png` (180 px) and `icon-512.png`.
  - They are rendered once from `icon.svg` with `rsvg-convert` and `magick` and embedded with the other portal assets.
  - Served at `/favicon.ico`, `/apple-touch-icon.png` and as versioned `/static/` URLs.
- **Manifest.** `/manifest.webmanifest` is generated per request with the configured brand as its name, so no name is hardcoded.
- **Legal pages.** `/privacy` and `/terms` get a description and canonical, and use the versioned asset URLs (the `asset` function is added to the legal template).

## 7. Fonts

- **Versioned URLs.**
  - Each font file and `fonts.css` gets a content hash at startup.
  - `fonts.css` refers to `/fonts/<name>?v=<hash>`.
  - Server-rendered pages link `/fonts/fonts.css?v=<hash>`.
  - A request whose `v` matches is served `public, max-age=31536000, immutable`. An unversioned one keeps today's headers, as the SPA index still uses them.
- **Preload.** Pages preload the Regular and Medium faces, the two used above the fold, with `<link rel="preload" as="font" type="font/woff2" crossorigin>`.
- **Not inlined.** The `@font-face` rules stay in a stylesheet. The CSP forbids inline styles, and the font set comes from a private directory at runtime, not from the embedded `catalog.css`.

## 8. Feeds

- **Channel description.** The combined feed (`/feed.xml`) takes it from the storefront tagline, falling back to the current text.
- **Item descriptions.** Release items in the combined feed use the artifact description; the release changelog stays in `/apps/feed.xml`, which is explicitly the release feed.
- **Self link.** All three feeds get `<atom:link rel="self">`.

## 9. Hostnames and contact address out of the code

- **New flag `--contact-email`.** It is shown in the footer, the support callout, `/pricing` and the SPA (through `/api/session`). Each link is omitted when the flag is unset.
- **Stripe line items.** They use neutral names ("Monthly support", "Year of funding") instead of a literal hostname.
- **SPA brand fallback.** When the session has not loaded, the SPA falls back to `window.location.hostname` instead of a literal.
- **Help-text examples are kept.** Flag help such as "e.g. https://slop.boutique" stays: it is documentation, not behavior.

## 10. Accessibility

- **Monogram text** changes from `--color-muted` (#737373 on #ededed, contrast about 4.2) to a darker gray that meets 4.5:1.
- **The mobile sort label** is visually hidden instead of `display:none`, so the `<select>` keeps its accessible name.

## Deferred (not in this change)

| Item | Why deferred | Next step |
| --- | --- | --- |
| Responsive thumbnails and cover sizes | Needs an image pipeline. The Go standard library cannot encode WebP; this is a choice between a cgo libwebp, an external tool at upload, or AVIF/JPEG derivatives. | Its own ticket. Store derived sizes and dimensions at upload, emit `srcset`. |
| Default `og:image` | Needs a designed 1200×630 asset. | Add it to the portal assets and use it when a page has no image. |
| Uptime and error alerting, certificate expiry alert | Platform monitoring, outside this repository. | External check on `/readyz` and `/`, plus alerting. |
| Restore drill | Operational. | Restore the nightly dump and S3 blobs into a scratch namespace. |
| Same-site instance origins, Stripe live mode, Google sign-in check, e2e users | Product and ops decisions in `doc/production.md` §12. | Decide before announcing. |
| DMARC (`_dmarc` TXT), CAA | DNS lives in a separate Cloudflare account. | An explicit `_dmarc` record overrides the wildcard CNAME. |
| Experiment editorial content, cover images | Content work. | Covers, a paragraph each, alt text. |
| bfcache (`no-store` from analytics), HTML gzip, JSON-LD, CDN | P2. Each has a tradeoff to decide separately. | Revisit after launch. |

## Testing

- **Integration tests** (`TEST_DATABASE_URL`):
  - redirect host: GET and POST, any path, 308 with path and query kept
  - `X-Forwarded-Proto` redirect, HSTS present and absent
  - `robots.txt` on portal and instance hosts
  - sitemap contents and visibility rules, plus every `<loc>` returning 200
  - catch-all 404 is HTML and 503 is styled
  - SPA and instance `noindex`
  - manifest uses the brand
  - versioned font caching
  - feed self links
  - the contact address rendered only when configured
- **Unit test:** `Config.Validate` for redirect hosts.
- **After deploy:** re-run `scripts/01-live-audit.sh` against both hosts.
