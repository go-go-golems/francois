---
Title: 'Follow-up TODO after the launch-readiness PR'
Ticket: FRANCOIS-020
Status: active
Topics:
    - frontend
    - performance
    - deployment
DocType: reference
Intent: short-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Everything left after PR #20: the deployment steps outside this repository, operations, product decisions, content, and code follow-ups."
LastUpdated: 2026-10-21T19:00:00-04:00
WhatFor: "Checklist of the remaining launch work after PR #20 merges."
WhenToUse: "When deploying PR #20 or picking up the remaining launch-readiness work."
---

# Follow-up TODO after the launch-readiness PR

PR: https://github.com/go-go-golems/francois/pull/20. Design: `design-doc/01-launch-readiness-fixes-retire-the-old-host-transport-crawling-and-polish.md`. Audit: `analysis/01-launch-readiness-audit-findings-and-checklist.md`.

## 1. Deploy (in this order)

GitOps repo `wesen/2026-03-27--hetzner-k3s`, `gitops/kustomize/francois/deployment.yaml`.

- [ ] **Stripe webhook.** Stripe does not follow redirects, so this must happen before the flag swap.
  - [ ] Create the endpoint `https://slop.boutique/webhooks/stripe` with the same events as `we_1UQ9zfLeHmcfmvuC639e6BPZ`.
  - [ ] Store its signing secret as the *current* file in `--stripe-webhook-secret-files`, and keep the old secret as *previous*.
  - [ ] Deploy and confirm that an event (for example a test checkout) arrives at the new endpoint.
- [ ] **Flags.**
  - [ ] Set `--public-url=https://slop.boutique`.
  - [ ] Set `--app-domain=slop.boutique`.
  - [ ] Set `--redirect-host=francois.yolo.scapegoat.dev`.
  - [ ] Set `--contact-email=<address>`.
  - [ ] Remove `--site=…`.
  - [ ] Set `--hsts-max-age=24h` for the first rollout.
- [ ] **Probes.** Change `/readyz` and `/healthz` to send `Host: slop.boutique`; the old host now answers 308.
- [ ] **Keep the old host's TLS.** Keep the Ingress and Certificate for `francois.yolo.scapegoat.dev`, so the redirect itself is served over HTTPS.
- [ ] **Verify.** Run `scripts/01-live-audit.sh` against both hosts and check:
  - [ ] HTTP→HTTPS 308
  - [ ] HSTS header
  - [ ] the old host 308s to the same path on slop.boutique
  - [ ] `robots.txt` and `sitemap.xml`
  - [ ] `/share.png`
  - [ ] the styled 404
- [ ] **Clean up the old host.**
  - [ ] Disable the old Stripe webhook endpoint.
  - [ ] Remove the old host's redirect URIs from the ZITADEL application.
- [ ] **CLI logins.** Re-login: `francois catalog auth login --server https://slop.boutique`. Go drops `Authorization` on the cross-host redirect, so old stored logins get 401.
- [ ] **Full HSTS.** After a few days without HTTPS problems, remove `--hsts-max-age` to get the one-year default.
- [ ] **Topology docs.** Update `doc/production.md` §2 (topology table, probes text) to the single site.

## 2. Search and previews

- [ ] Add slop.boutique to Google Search Console and Bing Webmaster Tools, and submit `https://slop.boutique/sitemap.xml`.
- [ ] Ask Search Console to remove or re-crawl `francois.yolo.scapegoat.dev` URLs; the 308s consolidate them over time.
- [ ] Check link previews with a real unfurl (Slack, Bluesky, LinkedIn Post Inspector) for:
  - [ ] the homepage (generated `/share.png`)
  - [ ] an experiment with a cover (`?w=1200` variant)
  - [ ] a journal post
- [ ] Run Lighthouse on `/` again. The homepage thumbnails should drop from about 782 KB to a few tens of KB.

## 3. Operations

- [ ] **Uptime checks:** an external check on `https://slop.boutique/` and `/readyz`, with alerting.
- [ ] **Certificate-expiry alerts** for both certificates.
- [ ] **5xx-rate alert** from Traefik metrics.
- [ ] **Restore drill:** restore the nightly Postgres dump and the S3 blobs into a scratch namespace, start the server against them, and record the time taken in `doc/production.md` §10.
- [ ] **Variant cache backups:** `<release-dir>/variants/` is a cache. Exclude it from volume backups if those copy the whole PVC.

## 4. DNS (Cloudflare account)

- [ ] **DMARC.** Add an explicit `_dmarc.slop.boutique` TXT record, for example `v=DMARC1; p=none; rua=mailto:…`; it overrides the wildcard CNAME. Tighten to `quarantine` once reports are clean.
- [ ] **CAA.** Add a CAA record allowing `letsencrypt.org` only.

## 5. Product decisions (from `doc/production.md` §12)

- [ ] **Instance isolation.** Decide whether instances move to a dedicated registrable domain before announcing; they currently share it with the portal.
- [ ] **Stripe live mode:**
  - [ ] a live Price with an explicit tax decision
  - [ ] remove the bootstrap test-key check
  - [ ] configure the billing portal for every plan product and price
- [ ] **Google sign-in:** sign in with Google once for real, then delete the unused older client secret.
- [ ] **Test users:** remove or isolate `e2e-admin` and `e2e-alice` (Alice has a sandbox subscription) in the production organization.

## 6. Content

- [ ] **Experiment pages:**
  - [ ] a cover image
  - [ ] one paragraph of description
  - [ ] alt text on every image
- [ ] **Homepage description.** `pkg/catalog/pages.go` hardcodes the homepage description ("… Run by Manuel and Ramon."). Move it to the storefront settings, or use the tagline, so it can be edited without a release.
- [ ] **ZITADEL login branding.** Add a label policy so the login pages carry the brand.

## 7. Code follow-ups

- [ ] **Markdown images:** images embedded in Markdown bodies (journal posts, experiment pages) still load the original. Give the Markdown renderer `srcset` with the media dimensions.
- [ ] **Uploadable share image:** add an optional storefront setting for an uploaded share image (a media ID), used instead of the generated `/share.png` when set.
- [ ] **WebP/AVIF variants:** consider WebP or AVIF variants (cgo libwebp or an external encoder at upload) if JPEG variants are still too large in Lighthouse.
- [ ] **Variant cleanup:** variants of deleted media stay on the volume. Fold them into the blob garbage collector when one exists (see §12 "Blobs of deleted media").
- [ ] **bfcache:** the analytics response sends `no-store`, which blocks the back/forward cache. Decide whether that is needed.
- [ ] **HTML gzip:** pages are served uncompressed unless Traefik compresses them. Add the Traefik `compress` middleware or compress in Go.
- [ ] **JSON-LD:** `CreativeWork`/`SoftwareApplication` for experiments, `BlogPosting` for journal posts.
- [ ] **CDN:** consider a CDN in front of `/static/`, `/fonts/` and `/media/`. They are all immutable and versioned.
- [ ] **Flaky test:** `pkg/catalog/mcpserver` `TestLocalPathAllowDirs` fails on macOS (the `/private/var` symlink); fix the path comparison.
