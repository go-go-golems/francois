---
Title: Diary
Ticket: FRANCOIS-026
Status: active
Topics:
    - catalog
    - storefront
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/content/resize.go
      Note: Card variant (1200x630 fill/crop)
    - Path: repo://pkg/catalog/instances.go
      Note: instancePage redirect for shared instance links
    - Path: repo://pkg/catalog/pages.go
      Note: media card variant, og:image selection
    - Path: repo://pkg/catalog/settings/settings.go
      Note: storefront.shareImage
    - Path: repo://ui/catalog/instances.tsx
      Note: Starter page for new-tab launch
    - Path: repo://ui/catalog/public.ts
      Note: openInNewTab and popup-blocked fallback
ExternalSources: []
Summary: 'Implementation diary: new-tab launch without about:blank, shared instance links that fall back to the app page, and link share cards.'
LastUpdated: 2026-10-24T10:39:02.629516658-04:00
WhatFor: ""
WhenToUse: ""
---

# Diary

## Goal

A chronological record of FRANCOIS-026: the "Starting…" launch tab, instance links that can be shared, and share cards (the main site card and the per-app cards).

## Step 1: Launch tab without about:blank, and shared instance links that fall back to the app

Run with the new-tab launch mode used to open `about:blank` and leave it blank while the opener created the instance (three round trips), then navigated the tab. I replaced that with a real page, `/instances/new`, that shows "Starting <title>…" at once, creates or resumes the instance itself and then replaces its URL with `/instances/{id}`. In the same change, `GET /instances/{id}` gained a server-side check: a visitor who cannot open the instance gets a 302 to the app's public page instead of "Instance not found".

Both work: in headless Chromium the tab moves from `/instances/new` to `/instances/ins_…` in about 0.2 s, and shared links redirect to `/apps/loom` for visitors who are signed out and for non-members. PR #28 is open and not merged yet.

### Prompt Context
**User prompt (verbatim):** "can we avoid the "about:blank" pause when running an artifact? and potentially make the link of an instance shareable so that people get  redirect to the artefact when they don't have access to the instance? Like if I go here https://slop.boutique/instances/ins_a3200fca7023211470db and the instance is "not found" it instaed brings me back to https://slop.boutique/apps/millwright ?"
**Assistant interpretation:** Make the new tab show something useful immediately, and turn unusable instance links into links to the app page.
**Inferred user intent:** Instance URLs get pasted to other people; they should land somewhere sensible, and starting an app should not look broken.
**Commit (code):** be4b4a7 — "feat(catalog): run opens a "Starting…" tab; shared instance links fall back to the app"

### What I did
- `ui/catalog/public.ts` `openInNewTab`: `window.open("/instances/new?artifact=…&release=…&title=…[&demo=1]")`. If the popup is blocked, it navigates the current tab to the same URL instead.
- `ui/catalog/instances.tsx` `Starter`: waits for the session, calls `api/demos` or `api/instances/open`, refetches the session (a first demo sets a guest cookie and a new CSRF token), then `navigate("/instances/"+id, true)`. On error it shows the message and a link back to the app.
- `ui/catalog/api.ts`: new `openInstance` and `startDemo` mutations. `ui/catalog/main.tsx`: route `instances/new` → `Starter`.
- `pkg/catalog/instances.go` `instancePage`, registered as `GET /instances/{id}` in `routes.go`. Members, and lookups that fail with an error other than not-found, get the SPA. Otherwise, if the instance exists and its artifact is `listed`, the visitor gets a 302 to `/apps/{id}`. Everything else gets the SPA.
- New test `pkg/catalog/instance_share_test.go`. Fixed `TestArtifactPageLatestDownload`, which now expects 2 download links (the phone copy from PR #27). Also fixed a gofmt slip in `releases.go` left by PR #26.

### Why
- Doing the work in the tab removes the blank wait and the opener-to-tab navigation. Errors now appear where the user is looking.
- The redirect is server-side so that link unfurlers and signed-out visitors (no JS session) get it too.

### What worked
- `TEST_DATABASE_URL=postgres://francois:francois@127.0.0.1:5491/francois?sslmode=disable go test ./pkg/catalog/... -count=1`: all ok. That ran against a throwaway `postgres:17` container on port 5491, since stopped.
- `pnpm -s check && pnpm -s build && pnpm -s test`: ok.
- Chromium run (seeded test server, `--host-resolver-rules` mapping portal.test and *.apps.test):
  ```
  stranger shared link -> http://portal.test/apps/loom
  none shared link -> http://portal.test/apps/loom
  tab first url http://portal.test/instances/new?artifact=loom&release=rel_…&title=Loom 159 ms
  tab became http://portal.test/instances/ins_055e7ec834e0c3e987aa 218 ms
  direct /instances/new -> http://portal.test/apps/loom
  ```

### What didn't work
- First test run used the already running `francois-email-013-postgres-1` (port 5437): `failed SASL auth: FATAL: password authentication failed for user "francois"`. That container belongs to another stack. I used a throwaway container instead.
- Initial `go test ./pkg/catalog/` failure: `storefront_test.go:182: expected one current HTML download`. It predates this change: the page has rendered the downloads twice since #27.
- Locally, the framed app showed `Cannot read properties of undefined (reading 'digest')`. `crypto.subtle` is missing on plain-http origins, which aren't secure contexts. The owner opening an existing instance (code this change doesn't touch) shows the same error. `--unsafely-treat-insecure-origin-as-secure` without a user-data-dir did not help. Production is https, so this is environment-only.
- `pkill -f TestZZManualServe` killed my own shell (exit 144) because the pattern matched the command line. I redid the cleanup without pkill.

### What I learned
- The compose stack (`deploy/compose`) needs ZITADEL to start the server. The Go test harness (`setup()` in `catalog_test.go`), with sessions created directly, is the faster way to drive the UI.

### What was tricky to build
- A guest demo changes the session cookie and CSRF token. The Launcher's launch handshake is a CSRF-protected mutation, so the Starter awaits `api.endpoints.session.initiate(undefined, {forceRefetch: true})` before navigating.
- `/instances/new` creates an instance on load, which makes it a GET that mutates state. It only runs when `window.opener` is same-origin, so external links to it just redirect to the app.

### What warrants a second pair of eyes
- The redirect reveals which app an instance ID belongs to. IDs are random, and only listed apps are revealed.
- Signed-out *members* now land on the app page rather than on a sign-in prompt for the instance.

### What should be done in the future
- Merge PR #28 and the GitOps bump, then check on slop.boutique: https://slop.boutique/instances/ins_a3200fca7023211470db should redirect to /apps/millwright when you aren't signed in.

### Code review instructions
- Start at `pkg/catalog/instances.go` `instancePage` and `ui/catalog/instances.tsx` `Starter`/`openedHere`. Run `go test ./pkg/catalog/ -run TestSharedInstanceLink` with `TEST_DATABASE_URL` set.

### Technical details
- Starter URL contract: `/instances/new?artifact=<id>&release=<rel>&title=<title>[&demo=1]`.

## Step 2: Share cards — a 1200×630 card variant and a configurable site share image

App pages already used their cover as `og:image`, but resized to 1200 wide at the cover's own aspect ratio, so each platform cropped it differently and the meta tags gave no size. I added a `card` media variant that scales and center-crops to exactly 1200×630 JPEG, and used it for app covers, video posters and journal covers. I also added a `storefront.shareImage` setting (a media ID). When set, it replaces the drawn `/share.png` on every page without its own image. The user's "fill / crop" artwork is meant to fill that setting.

The user's PNG, run through `content.Card`, comes out 1200×630, edge to edge, with all lettering and the "NOW OPEN → slop.boutique" line intact. All catalog tests pass.

### Prompt Context
**User prompt (verbatim):** "also, for share cards, what would we need? like can we take the main thumbnail and share card it?" and then "also we should use the fill/crop version of ~/Downloads/slop-boutique-preview.html  for doing the main site share card"
**Assistant interpretation:** Make per-app link previews come from the app's main image, and let the main site card be the provided artwork, in its fill/crop variant.
**Inferred user intent:** Links to slop.boutique and its apps should unfurl with deliberate, consistent artwork on social platforms and in chat.
**Commit (code):** a79bf62 — "feat(catalog): 1200×630 share cards from covers and a configurable site share image"

### What I did
- `pkg/catalog/content/resize.go`: `CardWidth/CardHeight`, `CardURL(id)` (`/media/{id}?card=1&v=2`) and `Card(r)`. `Card` picks the largest 1200:630 rectangle centered in the source, scales it with CatmullRom, puts transparent areas on black and writes JPEG q90. It refuses GIFs and images over the decode limit with `ErrNoVariant`.
- `pkg/catalog/pages.go` `media`: `?card=1` goes through the same cache and single-flight path as width variants. `variant()` now takes a cache name and a resize function. The cache name is `<sha256>-card-v2`.
- `pageData.DefaultImage` → `CardImage` + `ImageAlt`. `layout.tmpl` emits `og:image:width/height` for any card and `og:image:alt` when one is known. The alt is the cover's alt text, the app title for video posters, or the brand for the default card.
- `settings.Storefront.ShareImage` (`json:"shareImage,omitempty"`), validated as `^med_[a-f0-9]{8,64}$`. `render()` uses `CardURL(shareImage)` when it is set. There is a new field in Admin → Settings → Storefront, with its type in `ui/catalog/api.ts`.
- Tests: `TestCardCrops` (content), plus card checks in `TestMediaVariants`: the size, ETag suffix and GIF passthrough; the app page meta; and the configured share image showing up on /pricing.

### Why
- A server-side crop gives every platform the same picture. A card variant reuses the existing media store, cache and immutable URLs, so no new image pipeline is needed.
- The setting is a media ID, not an embedded asset: the server is multi-site and brand-neutral, and the image can be changed without a deploy.

### What worked
- `TEST_DATABASE_URL=… go test ./pkg/catalog/... -count=1`: all ok. `pnpm -s check`, `pnpm -s build`: ok.
- The user's fill/crop PNG (1659×948, RGBA) went through `content.Card` → `card-from-crop.jpg`, 1200×630. Visually checked: it scales to 1200×686 and crops 28 px from the top and the bottom.

### What didn't work
- My first `TestCardCrops` fixture was wrong: `content_test.go:149: tall: pixel (2,2) is 254,0,0`. A 400×400 source crops to rows 95–305, which reaches into the red band (rows 0–133). Making the tall fixture 400×1200 puts the crop inside the green band. The code was correct.

### What I learned
- The preview HTML embeds two images. The JPEG (1200×630) is the "recommended, full artwork" version with side bars. The PNG (1659×948) is the full-bleed version, which the page swaps in for "Compare · fill / crop".

### What was tricky to build
- Card URLs carry `v=VariantVersion` and the cache name carries the version too, so a future encoder change refreshes both browser caches and the variant cache, just like the width variants.

### What warrants a second pair of eyes
- `shareImage` is not checked against the media table when settings are saved (the settings package has no store). A deleted media ID would give a broken `og:image`.
- Center crop only. Covers with important content near the top or bottom edges lose it. A focal point could come later.

### What should be done in the future
- After deploy: upload `preview-2.png` (Studio → Media or `francois catalog media upload`), then set `storefront.shareImage` in Admin → Settings. The user applies settings themselves.
- Check unfurls with a platform debugger after deploy (e.g. opengraph.xyz).

### Code review instructions
- `content.Card`, `Server.media`/`variant`, `render()` default image, `layout.tmpl` line 13. Validate with `go test ./pkg/catalog/ -run 'TestMediaVariants|TestShareImage'` and `go test ./pkg/catalog/content/`.

### Technical details
- Crop math: `cw, ch = W, W*630/1200`; if `ch > H` then `cw, ch = H*1200/630, H`; origin `((W-cw)/2, (H-ch)/2)`.

## Step 3: Review fix — Run did nothing when the popup was blocked

A reviewer on PR #28 flagged (P2) that when `window.open` returns null, the fallback navigated the storefront tab to `/instances/new`. That tab has no `window.opener`, so the Starter redirected straight back to the app: Run silently did nothing. The fix keeps `/instances/new` for real popups. With the popup blocked, the storefront tab creates the instance itself, as before #28, and then navigates to it.

### Prompt Context
**User prompt (verbatim):** "for the previous pr, but it's fine to just push your current stuff to #28 again, rephrase the PR, and restart a code review anyway" (it came with the pasted review comment "Preserve instance launch when the popup is blocked")
**Assistant interpretation:** Fix the review finding, put the share-card work on the same PR, update its description and ask for a fresh review.
**Inferred user intent:** One PR with everything, reviewed again before merging.
**Commit (code):** 1465eeb — "fix(catalog): keep Run working when the popup is blocked"

### What I did
- `ui/catalog/public.ts` `openInNewTab`: `if (window.open(url, "_blank")) return;`. Otherwise it runs `openInstance(data, decision)` and then `location.assign("/instances/" + id)`. Errors propagate to the button's existing error display.

### Why
- Other ways to authorize a same-tab starter, such as a sessionStorage nonce, add a mechanism for a rare path. Creating the instance in the opener needs no new trust signal, and the `openedHere()` guard stays as strict as it was.

### What worked
- Chromium with `window.open = () => null` injected: `blocked popup: same tab -> http://portal.test/instances/ins_004947cbd1ff82570c42 new tabs: 0`.
- `pnpm -s check`, `pnpm -s build`, `pnpm -s test` (22 pass).

### What didn't work
- No failures observed.

### What I learned
- N/A.

### What was tricky to build
- N/A.

### What warrants a second pair of eyes
- With the popup blocked, the user sees the old pause, with the button showing "Starting…". That is acceptable for a fallback.

### What should be done in the future
- N/A.

### Code review instructions
- `ui/catalog/public.ts` `openInNewTab`.

### Technical details
- N/A.
