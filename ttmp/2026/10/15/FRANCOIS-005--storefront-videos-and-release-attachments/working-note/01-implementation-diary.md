---
Title: Implementation diary
Ticket: FRANCOIS-005
Status: active
Topics:
    - frontend
    - billing
DocType: working-note
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Step-by-step diary for FRANCOIS-005
LastUpdated: 2026-10-15T11:53:14.627720614-04:00
WhatFor: ""
WhenToUse: ""
---

# Implementation diary

## Summary

<!-- Brief summary for LLM ingestion -->

## Notes

<!-- Free-form notes, meeting summaries, or research findings -->

## Decisions

<!-- Any decisions made during this working session -->

## Next Steps

<!-- Actions or follow-ups -->

## Step 1: Investigation and design

The owners had just seeded a local showcase (FRANCOIS-004 storefront, Polis as a free sample with screenshots) and asked for two more features. I read the media, Markdown, release and download code before deciding anything, because both features touch paths that were built for small images and two fixed release files. The result is a short design document in this ticket and five tasks.

The investigation turned up two existing defects worth fixing on the way: the studio media upload ignored the `limits.maxMediaBytes` setting (a constant 25 MB was used), and catalogue cards rendered every cover as `<img>`, so a video cover would have shown a broken image.

### Prompt Context

**User prompt (verbatim):** "Ok, a couple of additional features: videos, and multiple file attachments to a version for additional information."

**Assistant interpretation:** Make videos work everywhere storefront media is used, and let a release carry any number of extra files (manuals, archives, notes) that visitors can download.

**Inferred user intent:** Creators want to *show* an application in motion and *explain* a version with supporting material, as part of selling subscriptions.

A later message, sent while this work ran (verbatim): "btw when done write a detailed project report for the obsidian vault as a deep dive technical analysis blog post using a textbook writing style (no analogies, see skill). Commit and push the bsidian vault when done (go-go-parc vault). About all the things you built"

### What I did

- Read `pkg/catalog/content/media.go`, `content/markdown.go`, `pages.go` (`media`, page handlers), `routes.go` (`download`, `publicRelease`), `studio.go` (media handlers, `checkMedia`), `releases/releases.go` (`Blobs`), `store/content.go`, `store/releases.go`, `settings/settings.go`, the templates and `ui/catalog/studio.tsx`.
- `docmgr ticket create-ticket --ticket FRANCOIS-005 --title "Storefront videos and release attachments" --topics frontend,billing`; added the design doc and this diary; wrote `tasks.md`.
- Committed the design: `8fea289`.

### Why

- Attachments get their own table instead of more `release_files` roles: `release_files` has `UNIQUE(release_id, role)`, is part of the immutable release, and the publish checks and `Missing` integrity sweep iterate it.
- Attachments are mutable after publishing: they do not run, so changing them cannot change a pinned instance; immutability exists to protect running applications.
- Markdown cannot tell a video from an image by URL, so the renderer must receive the media types of referenced IDs.

### What worked

- The settings precedent from FRANCOIS-004 (`005_plans.sql`) applied directly: a new required limit needs a migration that rewrites stored revisions.

### What didn't work

- N/A

### What I learned

- `settings.Parse` uses `DisallowUnknownFields` and then `Validate`; a field missing from a stored revision decodes as zero and fails the range check, so every new limit must be backfilled.

### What was tricky to build

- Nothing yet; the risk was in the shape of attachment access (published-only public route vs. editors on drafts), resolved by two routes sharing one rule.

### What warrants a second pair of eyes

- The mutability decision for attachments (design §2.2) is a product choice; it is easy to restrict to drafts later.

### What should be done in the future

- Blob garbage collection; deleting media and attachments leaves their blobs.

### Code review instructions

- Read `design-doc/01-videos-and-release-attachments-design.md`.

### Technical details

- Ticket: `ttmp/2026/10/15/FRANCOIS-005--storefront-videos-and-release-attachments/`.

## Step 2: Videos

Videos now work in every place storefront media appears. A Markdown reference to a video renders a player, a video can have a poster image (set from the current frame in the studio or chosen from the library), a video cover loops silently on the artifact page and is paused for visitors who prefer reduced motion, and catalogue cards show the poster. Serving and uploading stream from disk, and the upload limit is the `maxMediaBytes` setting, now allowed up to 1 GiB.

Commit `7f75b7e`. Migration `006_videos_attachments.sql` in the same commit also creates `release_attachments` and backfills `limits.maxAttachmentBytes`, so the schema for both features lands at once.

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Implement design Part 1.

**Inferred user intent:** Show applications in motion on their pages and in write-ups.

### What I did

- `content/markdown.go`: `Embed{Type, Poster}`, `MediaRefs(source)`, `Render(source, embeds)`. An AST transformer reads the embeds from the goldmark `parser.Context` and marks video image nodes with attributes; `renderImage` emits `<video src controls preload="metadata" playsinline poster aria-label>` for them. `portalMedia` factors the `/media/<id>` check.
- `pages.go`: `Server.markdown(ctx, source)` looks up referenced media and is used by the home intro, artifact page, creator bio, post and the studio preview. `/media/<id>` opens the blob and calls `http.ServeContent` with an `ETag` of the SHA-256.
- `releases.Blobs`: `PutReader(r, limit)` (copy to a temporary file while hashing; `ErrTooLarge`), `Open(key)`.
- `content.DetectMedia(io.ReadSeeker)`: sniffs the first 512 bytes, decodes image dimensions, rewinds.
- `studio.go`: `uploadedFile(w, r, limit)` (MaxBytesReader + multipart spooling, `413 too_large`), `studioUploadMedia` streams under the settings limit, `studioPatchMedia` accepts `{alt?, poster?}` (poster must be an image and the target a video), `checkImage` for avatars, `studio/me` returns `limits`.
- `store/content.go`: `Media.Poster`, `SetMediaPoster`; `MediaReferenced` counts posters and the active storefront intro.
- `settings`: `Limits.MaxAttachmentBytes` (1 MiB–2 GiB, default 100 MiB); `maxMediaBytes` range 1–1024 MiB.
- Templates: `media` (poster, aria-label), `hero` (cover: `autoplay muted loop playsinline controls`), `thumb` (cards: poster image, else muted first frame). CSS for `.ri-thumb video` and `.md video`. `public.ts` pauses autoplaying videos under `prefers-reduced-motion`.
- Studio UI: `VideoTile` (scrub, *Use this frame as poster* via canvas → JPEG upload → PATCH, *Choose poster* with an image-only `MediaPicker`, *No poster*), the uploader shows the limit from `studio/me`; admin settings gain *Max attachment MiB*.
- Test `TestVideos`: upload, Range `206` with `Content-Range: bytes 100-199/4096`, poster validation, poster in use cannot be deleted, Markdown preview and artifact page HTML, card uses the poster, video avatar rejected, 413 over a 1 MiB limit.

### Why

- Attributes on the AST node, set by a transformer, are the per-render channel goldmark offers: renderer functions receive only the writer, source and node, and the `Markdown` value is shared across requests.
- The cover autoplays muted because a looping clip is the usual way to show an application at a glance; `controls` stays so visitors can stop it (a moving element longer than five seconds needs a pause control).
- `Open` does not re-hash: hashing a 500 MB file on every Range request is not affordable; the key is the hash of what was written.

### What worked

- `TEST_DATABASE_URL=… go test -count=1 ./pkg/catalog/...` → all `ok`; `TestVideos` ran (`--- PASS: TestVideos (0.12s)`).
- `pnpm check`, `pnpm build` clean; `golangci-lint run ./pkg/catalog/...` → `0 issues.`

### What didn't work

- A regex rewrite of the content tests also rewrapped the test function signature: `func TestDetectMedia(bytes.NewReader(t *testing.T))`; fixed by hand.
- `golangci-lint` flagged `nonamedreturns` on the first `PutReader`; rewritten with plain returns.
- The LSP reported missing fields on `store.Artifact` throughout; it was indexing an older checkout. `go build ./...` was clean.

### What I learned

- Go's MP4 sniffer accepts ISO-BMFF files whose brands include an `mp4…` brand; a 24-byte `ftyp` box is enough for a fixture.

### What was tricky to build

- Getting media types into the renderer without per-request state on the shared `Markdown`. The answer was `parser.WithContext` plus an `ASTTransformer` that marks nodes.

### What warrants a second pair of eyes

- `PutReader` reuses an existing blob when a file of the same size is already at the key; the key is the SHA-256 of the new bytes, so equality is implied, but the temporary file is then discarded without comparing bytes.

### What should be done in the future

- A server-side poster/thumbnail generator (ffmpeg) if browsers' frame capture proves unreliable for some codecs.

### Code review instructions

- Start at `content/markdown.go` (`embedTransformer`, `renderImage`), then `pages.go` (`markdown`, `media`), `studio.go` (`uploadedFile`, `studioUploadMedia`, `studioPatchMedia`), `layout.tmpl` (`media`, `hero`, `thumb`).
- Validate: `TEST_DATABASE_URL=… go test -count=1 -run TestVideos -v ./pkg/catalog/`.

### Technical details

- Range check: `GET /media/<id>` with `Range: bytes=100-199` → `206`, 100 bytes, `Content-Range: bytes 100-199/4096`.
