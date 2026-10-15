---
Title: Videos and release attachments design
Ticket: FRANCOIS-005
Status: active
Topics:
    - frontend
    - billing
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Video embedding, posters and streaming; per-release attachments with plan-gated downloads
LastUpdated: 2026-10-15T11:53:14.449750007-04:00
WhatFor: "Understanding how storefront videos and release attachments work and why"
WhenToUse: "Before changing media serving, the Markdown renderer, release downloads or the studio Releases tab"
---

# Videos and release attachments design

## Executive summary

FRANCOIS-004 gave every artifact a public page with a cover, a gallery and a Markdown write-up, and it already accepted MP4 and WebM uploads. Two things are missing for a creator who wants to *show* an application and *explain* it:

1. **Videos are second-class.** A video can be a cover or a gallery item, but a Markdown reference to it renders a broken `<img>`, there is no poster frame, the 25 MB upload limit is hard-coded (the `maxMediaBytes` setting is ignored), and every Range request reads the whole file into memory and hashes it.
2. **A version is only its HTML and manifest.** There is no place for the PDF manual, the source archive, a printable plate pack or design notes that belong to a particular version.

This ticket makes videos first-class media (Markdown embeds, posters, streaming, configurable size) and adds **release attachments**: any number of extra files (up to 20) on a release, managed in the studio and with `francois catalog push --attach`, downloaded under the same rule as the release itself.

## Terms

- **Media**: a public image or video in `catalog_media`, served at `/media/<id>` on the portal origin, used by pages, posts and creator profiles. Anyone can fetch it.
- **Release**: an immutable version of an artifact: `release_files` holds exactly one `html` and one `manifest` file. Publishing makes it visible; corrections are new versions.
- **Attachment** (new): a supplementary file on a release in `release_attachments`. Not part of what runs, so it is not covered by release immutability.
- **Download rule**: a published release's files can be downloaded by anyone if the release is a *free sample*, otherwise by signed-in users whose plan can run the artifact (`Entitlement.CanRun`). Editors of the artifact can always download, including drafts.

## Part 1: Videos

### 1.1 Markdown embeds

Authors already paste `![alt](/media/<id>)` (the studio's *Copy Markdown* button). The renderer cannot tell an image from a video by the URL, so it needs the media types of the IDs a document references. The flow:

```text
source ──► content.MediaRefs(source) ──► ["med_a", "med_b"]
                                             │
                     store.MediaByIDs ◄──────┘
                            │
   map id → content.Embed{Type, Poster, Width, Height}
                            │
source ──► goldmark parse ──► ASTTransformer (reads map from parser.Context)
                                 marks video images with attributes
                            ──► renderImage: <video …> or <img …>
```

`Markdown.Render(source, embeds)` takes the map; a nil map renders every portal media reference as an image, as before. The server wraps it in `s.render(ctx, source)` so every call site (home intro, artifact page, creator bio, post, studio preview) behaves the same.

A video renders as:

```html
<video src="/media/med_v" controls preload="metadata" playsinline poster="/media/med_p" aria-label="alt"></video>
```

No raw HTML is ever accepted from authors; the element is produced by the renderer from a vetted ID.

### 1.2 Posters

`catalog_media.poster_media_id` (nullable, references an *image* in `catalog_media`, `ON DELETE SET NULL`) gives a video its still frame. It is used by Markdown embeds, galleries, covers and catalogue cards. A media item used as a poster counts as referenced, so it cannot be deleted while in use.

The studio can set a poster two ways, without any server-side video tooling:

- **Use current frame**: the media tile's video is paused at the frame the editor chose; the browser draws it to a canvas, uploads the PNG as new media (alt "Poster: …") and sets it. Media is same-origin, so the canvas is not tainted.
- **Choose from library**: pick any existing image.

### 1.3 Where videos appear

| Place | Rendering |
|---|---|
| Artifact cover | `autoplay muted loop playsinline` with poster: a moving hero, silent until the visitor acts |
| Gallery, Markdown | `controls preload="metadata"` with poster |
| Catalogue cards, creator page cards | the poster image when there is one, else a muted, preloaded video frame |

### 1.4 Serving and limits

- `Blobs.Open(key)` returns the file; `/media/<id>` uses `http.ServeContent` on it, so Range requests seek instead of reading and hashing the whole file. Integrity is checked when the blob is written (content-addressed key = SHA-256).
- Uploads stream: the multipart file is sniffed from its first 512 bytes, images are decoded for dimensions, and `Blobs.PutReader` copies to a temporary file while hashing, then renames into place.
- The upload limit is the active settings' `limits.maxMediaBytes`; its allowed range grows from 1–200 MiB to 1–1024 MiB.

## Part 2: Release attachments

### 2.1 Data model

```sql
CREATE TABLE release_attachments (
    id text PRIMARY KEY,                       -- att_…
    release_id text NOT NULL REFERENCES releases(id) ON DELETE CASCADE,
    name text NOT NULL,                        -- original file name, ≤ 200 chars
    label text NOT NULL DEFAULT '',            -- shown instead of the name when set
    media_type text NOT NULL,                  -- sniffed; informational only
    size bigint NOT NULL,
    sha256 text NOT NULL,
    storage_key text NOT NULL,
    position integer NOT NULL DEFAULT 0,
    created_by text NOT NULL REFERENCES users(id),
    created_at timestamptz NOT NULL DEFAULT now()
);
```

Bytes live in the same content-addressed blob store as releases and media. Settings gain `limits.maxAttachmentBytes` (default 100 MiB, 1 MiB–2 GiB); migration `006` backfills it into stored settings revisions, because settings parse strictly and a missing field would fail validation (the same hazard `005_plans.sql` handled). A release holds at most 20 attachments.

### 2.2 Why attachments are mutable

Release files are immutable because instances pin a release: changing its HTML would change running applications underneath their users. Attachments do not run. A creator should be able to add the manual to 1.1.0 after shipping it, fix a typo in a PDF or remove an archive, without cutting 1.1.1. So editors can add, relabel and remove attachments in any release state, including published.

### 2.3 Access

```text
GET /api/releases/{release}/attachments/{id}/download    public route
   release must be published
   → same rule as release files: free sample, or signed-in with CanRun
GET /api/studio/attachments/{id}/download                 editors, any state
```

Both reuse one authorization helper extracted from `download` (`mayDownload`). Responses use `Content-Disposition: attachment`, `application/octet-stream`, `nosniff`, `Cache-Control: private, no-store` and `X-Content-SHA256`: an attachment is never rendered on the portal origin, so any file type is safe to accept.

### 2.4 APIs

| Method | Path | Body | Notes |
|---|---|---|---|
| `POST` | `/api/studio/releases/{id}/attachments` | multipart `file`, optional `label` | editors; size and count limits |
| `PATCH` | `/api/studio/attachments/{id}` | `{label?, position?}` | editors |
| `DELETE` | `/api/studio/attachments/{id}` | | editors; the blob stays (shared, content-addressed) |
| `GET` | `/api/studio/releases` | | each release gains `attachments[]` |
| `GET` | `/api/artifacts/{id}` | | each public release gains `attachments[]` (no storage keys) |

Personal API tokens work on the studio routes, so `francois catalog push --attach docs/manual.pdf --attach plates.zip` creates the draft, uploads each attachment, then publishes when `--publish` is given.

### 2.5 Where attachments appear

- **Artifact page, Versions:** under each release, a list of attachments with label (or name) and size. When the visitor may download, each is a link; otherwise the names are shown with a note naming the plan that unlocks them, so visitors see what a subscription includes.
- **Studio, Releases tab:** each release row gets an *Attachments (n)* panel with a drop zone, inline label editing, download and delete.

## Implementation plan

1. Videos backend: migration `006` (poster column, attachments table, settings backfill), `Blobs.Open`/`PutReader`, streaming upload and serving, settings range, Markdown embeds, poster API. Tests.
2. Videos frontend: templates (cover/gallery/cards/posters), studio poster controls, limit text from settings. Browser check with a real video.
3. Attachments backend: store, studio and public routes, download helper, publicRelease, push `--attach`. Tests.
4. Attachments frontend: artifact page section, studio panel. Browser check.
5. Docs: production playbook (limits, ingress body size), README; seed the local showcase with a video and attachments.

## Risks and rollout notes

- **Ingress body size.** Uploads now go up to 1 GiB. Traefik has no default request body limit; if one is added later, it must exceed `maxAttachmentBytes` and `maxMediaBytes`.
- **Disk.** Videos and attachments share the release volume. Deleting media or attachments does not delete blobs (they may be shared); a blob garbage collector is future work.
- **MP4 sniffing.** Go's sniffer recognises ISO-BMFF files whose brands include `mp4…`; QuickTime `.mov` is rejected. Export as MP4 (H.264/AAC) or WebM.
