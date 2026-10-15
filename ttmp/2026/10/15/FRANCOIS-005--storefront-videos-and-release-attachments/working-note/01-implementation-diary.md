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
RelatedFiles:
    - Path: repo://pkg/catalog/attachments_test.go
      Note: attachment tests
    - Path: repo://pkg/catalog/studioclient/client.go
      Note: streamed Attach for push
    - Path: repo://pkg/catalog/videos_test.go
      Note: video tests
    - Path: repo://ui/catalog/studio.tsx
      Note: VideoTile and Attachments panel
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

## Step 3: Release attachments

A release now carries up to 20 attachments in `release_attachments`. Editors add, relabel, reorder and remove them in any release state from the studio (an *Attachments (n)* panel on each release row) or upload them with `francois catalog push --attach`. Visitors download them under the release's own rule, which is now one helper shared with the HTML download; the artifact page lists them under each version, linked when the visitor may download them and otherwise with the plan that unlocks them.

Commits `e0691da` (feature) and `f7f5a4b` (page shows the file name only when a label differs).

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Implement design Part 2.

**Inferred user intent:** Let a version ship with its manual, source pack or notes, as part of what a subscription buys.

### What I did

- `store/attachments.go`: `Attachment`, `CreateAttachment` (locks the release row, counts, appends at `max(position)+1`, `ErrConflict` at the limit), `Attachment`, `UpdateAttachment(label?, position?)`, `DeleteAttachment`; `Release.Attachments` loaded with the release files.
- `routes.go`: `mayDownload(w, r, rel)` extracted from `download`; public `GET|HEAD /api/releases/{release}/attachments/{id}/download`; `publicRelease` includes `publicAttachments` (no storage key, no user IDs).
- `attachments.go`: upload (settings `maxAttachmentBytes`, file name = base name ≤200 chars, label ≤200, type sniffed from 512 bytes, streamed with `PutReader`), patch, delete, editor download, public download; `serveAttachment` uses `mime.FormatMediaType` for the `Content-Disposition` (RFC 2231 for non-ASCII names) and `http.ServeContent` (Range).
- Studio listing: attachment `href` points at the editor route so drafts can be downloaded.
- Artifact page: `releaseLine.Attached`, `Locked`; template list and plan note; `size` template function; the Open Graph image falls back to a video cover's poster.
- `studioclient.Attach` streams the multipart body through `io.Pipe`; `Push` attaches before publishing; `push --attach` (repeatable) checks the files exist before creating a draft.
- Studio UI: `Attachments` panel (drop zone with the limit from `studio/me`, inline labels, ↑/↓ that renumber the list, download, remove).
- Tests: `TestAttachments` (editor-only drafts, non-editor 403, RFC 2231 name, published rule 401/403/206, unknown ID 404, relabel after publish, page listing and order, links only for entitled visitors, public API hides storage and users, delete, 20-per-release 409, size 413, draft delete cascades) and `push --attach` in `TestTokensAndPush`.

### Why

- The release row lock: my first version used `INSERT … SELECT … WHERE count < limit` and a comment claiming concurrent uploads could not exceed the limit. Under READ COMMITTED two such statements both see the old count, so the claim was wrong; `SELECT … FOR UPDATE` on the release serializes them.
- Reordering renumbers 0…n-1 and patches only changed rows, because positions can have gaps after deletions and swapping two indices could collide with a third item.

### What worked

- `TEST_DATABASE_URL=… go test -count=1 ./pkg/catalog/...` → all `ok`; `--- PASS: TestAttachments (0.29s)`, `--- PASS: TestTokensAndPush (0.18s)` on the first run.
- `golangci-lint run ./...` → `0 issues.`; `pnpm check`, `pnpm build` clean.

### What didn't work

- N/A beyond the locking correction above (caught in review before any test).

### What I learned

- `http.ServeContent` sets `Accept-Ranges` and answers `Range` for any `io.ReadSeeker`; the `Content-Type` set beforehand (`application/octet-stream`) is kept, so downloads never sniff to `text/html` on the portal origin.

### What was tricky to build

- Keeping the two download paths consistent: the public route must refuse drafts while editors need them. One rule (`mayDownload`) plus two thin routes kept the policy in one place.

### What warrants a second pair of eyes

- Attachments are mutable after publishing (design §2.2). Subscribers could see a file change under a version label; a changelog of attachment changes might be wanted later.

### What should be done in the future

- Blob garbage collection for deleted attachments.

### Code review instructions

- `pkg/catalog/attachments.go`, `routes.go` (`mayDownload`), `store/attachments.go`, `pages.go` (`releaseLine`), `artifact.tmpl`, `studioclient/client.go` (`Attach`).
- Validate: `TEST_DATABASE_URL=… go test -count=1 -run 'TestAttachments|TestTokensAndPush' -v ./pkg/catalog/`.

### Technical details

- Count and position under lock: `SELECT 1 FROM releases WHERE id=$1 FOR UPDATE`, then `SELECT count(*), COALESCE(max(position)+1,0) FROM release_attachments WHERE release_id=$1`.

## Step 4: Browser verification on the local stack

I rebuilt the local compose stack from this branch and exercised both features through the real UI with a real video and real files. Migration 006 ran against the local database that already held four settings revisions and backfilled `maxAttachmentBytes=104857600`.

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Validate the features end to end, not only through tests.

**Inferred user intent:** Features that work when Manuel and Ramon use them.

### What I did

- `docker compose --profile stripe up -d --build francois`; checked `schema_migrations` (`006_videos_attachments.sql`) and the backfilled limit.
- Made a 13 s clip of Polis from 90 JPEG frame screenshots (`ffmpeg -framerate 7 … -c:v libx264 -crf 22 -movflags +faststart`, 1280×704, 1.3 MB).
- Studio → Media: uploaded the MP4 through the file input, scrubbed to 11 s, *Use this frame as poster* → toast "Poster set from this frame", `poster="/media/med_…"`.
- Set it as the Polis cover and embedded it in the write-up (token API).
- Studio → Polis → Releases → v1.1.0 → Attachments: uploaded a PDF, a ZIP and a TXT, labelled two, moved the changelog to the top.
- `push --attach` for capabilities 1.0.1 (subscriber plan): `attachments 2`.
- Signed out: cover `paused:false, muted:true, loop:true`, `readyState 4`; embed has controls, poster and aria-label; Polis (free sample) attachment downloads `200`; capabilities lists attachments without links plus "Attachments are included with the Subscriber plan."
- Reduced motion (`reducedMotion: 'reduce'`): cover `paused:true, currentTime:0`.
- As bob (Subscriber): two links, download `200`, `attachment; filename="Playground guide.md"`.
- Home card for Polis: `<img src="/media/<poster>" …>`.

### Why

- Playwright's own video recording could not be used for the clip: the WebGL canvas came out grey and the frame mis-scaled, while element screenshots capture WebGL correctly.

### What worked

- Every check above.

### What didn't work

- `recordVideo` clip: grey canvas; replaced by screenshot frames.
- `pandoc --pdf-engine=typst` → `Error producing PDF.`; the snap-confined `typst` could not read the scratch directory (`error: input file not found`). Rendered the PDF with Chromium (`page.pdf`) instead.
- The first recording attempt timed out on `getByText('TIMELAPSE')`, which matched hidden help text; `getByRole('button', {name: /timelapse/i})` worked.

### What I learned

- An unlabelled attachment rendered its name twice (title and file line); fixed in `f7f5a4b`.

### What was tricky to build

- N/A

### What warrants a second pair of eyes

- The media tile stacks three poster actions vertically; it works but is crowded at narrow tile widths.

### What should be done in the future

- N/A

### Code review instructions

- Reproduce with `scripts/01-seed-local-showcase.sh` (see its header) and the steps above.

### Technical details

- Clip: `ffmpeg -v error -y -framerate 7 -i frames/%04d.jpg -vf "scale=1280:-2,format=yuv420p" -c:v libx264 -preset slow -crf 22 -movflags +faststart polis-tour.mp4`.

## Step 5: Documentation

`doc/production.md` gains the attachment and video rows in the storefront content table, the limits and streaming behaviour, `push --attach`, the migration 006 rollout note (no flag changes; ingress body limits must stay above the upload limits) and two known gaps (no rollback below 006; no blob garbage collection). The README's storefront section mentions attachments, video embeds and posters. The ticket keeps the local showcase seed script and Tessera, the small app written for the showcase.

### Prompt Context

**User prompt (verbatim):** see Step 1.

**Assistant interpretation:** Leave the operators' playbook accurate.

**Inferred user intent:** Deployable work.

### What I did

- Edited `doc/production.md` (§3.7, §4.1, §5.3, §12) and `README.md`.
- Added `scripts/01-seed-local-showcase.sh`, `scripts/tessera.html`, `scripts/tessera.manifest.json`.

### Why

- Migration 006 changes stored settings; the playbook records it next to the FRANCOIS-004 note.

### What worked

- `bash -n scripts/01-seed-local-showcase.sh`.

### What didn't work

- My `sed` that rewrote the script header replaced `set -euo pipefail` with the comment; restored.

### What I learned

- N/A

### What was tricky to build

- N/A

### What warrants a second pair of eyes

- N/A

### What should be done in the future

- Deploy: merge `feat/storefront` with the k3s argument changes from FRANCOIS-004 §4.1.

### Code review instructions

- `git diff` of `doc/production.md` and `README.md`.

### Technical details

- N/A
