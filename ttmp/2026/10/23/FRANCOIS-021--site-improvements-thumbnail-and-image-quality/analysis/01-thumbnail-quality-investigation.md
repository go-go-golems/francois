---
Title: Thumbnail quality investigation
Ticket: FRANCOIS-021
Status: active
Topics:
    - frontend
    - performance
DocType: analysis
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/content/resize.go
      Note: variant encoding, quality and versioned URLs
    - Path: repo://pkg/catalog/launch_test.go
      Note: TestMediaVariants regression check
    - Path: repo://pkg/catalog/pages.go
      Note: media handler, srcset and variant cache names
    - Path: repo://pkg/catalog/web/templates/layout.tmpl
      Note: mini template feeds the archive row and hover preview
    - Path: repo://ui/catalog/public.ts
      Note: bindArchive clones the row image into the preview
ExternalSources: []
Summary: Why storefront thumbnails looked blurry and blocky, with measurements and the chosen fix.
LastUpdated: 2026-10-23T14:46:36.349073262-04:00
WhatFor: ""
WhenToUse: ""
---
# Thumbnail quality investigation

## Symptom

After the six Claude artifacts (Living Comic Studio, Hypercore Studio, Mimeo, Kiln Workbench, Octavo Lab, Form Skate Workbench) were published with 1600-2400 px covers and galleries, the thumbnails on slop.boutique looked compressed: blurry, with blocky text.

## Findings

### 1. The archive hover preview upscales the 160 px variant (main cause)

The home page archive row renders its cover with the `mini` template (`pkg/catalog/web/templates/layout.tmpl`), which emitted only `src=".../media/<id>?w=160"` and no `srcset`. That size is right for the 76x54 row thumbnail shown on narrow screens (`catalog.css`, `.archive-thumb`).

On desktop, `bindArchive` in `ui/catalog/public.ts` clones that same `<img>` into `[data-archive-preview]`, which is `width:432px; max-width:42vw; aspect-ratio:8/5`. Measured on production with Playwright (1440x900, DPR 1), hovering the Kiln Workbench row:

| element | CSS size | natural size |
|---|---|---|
| hover preview img | 432 x 270 | 160 x 100 |

A 2.7x upscale at DPR 1, 5.4x on a retina display. Every hover preview on the site was affected, not only the new apps; dense interface screenshots just show it most.

### 2. Variants are JPEG quality 82 with 4:2:0 chroma

`content.Resize` encodes opaque variants with `image/jpeg` at quality 82. Go's encoder always subsamples chroma 4:2:0. For screenshots of interfaces (code, tables, thin lines) this rings visibly. Measured on a 1200 px Kiln screenshot with Pillow (same quality scale):

| quality | subsampling | size | PSNR |
|---|---|---|---|
| 82 | 4:2:0 | 112 KiB | 39.4 dB |
| 90 | 4:2:0 | 142 KiB | 42.9 dB |
| 90 | 4:4:4 | 155 KiB | 44.0 dB |
| 95 | 4:2:0 | 184 KiB | 45.8 dB |
| PNG | - | 282 KiB | lossless |

Quality 90 buys ~3.5 dB for ~27% more bytes. Go's standard encoder cannot do 4:4:4, and there is no WebP encoder in the dependency set, so 90 at 4:2:0 is the cheap step.

### 3. Uploads were already lossy WebP (minor)

The gallery images were uploaded as WebP quality 86, so variants are a second lossy generation. Minor next to 1 and 2; future uploads of screenshots can be lossless WebP.

### Not a problem

Covers and galleries on `/apps/<id>` use the `media`/`hero` templates with a full `srcset` and `sizes="(max-width: 1200px) 100vw, 1200px"`, so browsers pick 1200 px or the original.

## Fix

- `mini` template: add `srcset` (via `srcset`, or `variantSet` for video posters whose width the page does not know) with `sizes="76px"`, so the row still loads the small file.
- `public.ts`: show the preview first, then set the clone's `sizes` to `preview.offsetWidth + "px"` before inserting it, so the browser picks the 640 or 1200 variant for the preview.
- `content.Resize`: JPEG quality 82 -> 90.
- Variant URLs carry a version (`?w=640&v=2`) built by `content.VariantURL`, and the cache name/ETag become `<sha>-w640-v2`. Variant responses are `Cache-Control: immutable` for a year, so without a new URL browsers would keep the old quality-82 files. The server ignores `v`; the old cache files are simply no longer read.

## Verification

- `TestMediaVariants` now publishes an artifact with a cover and asserts the archive row `<img>` has the srcset and `sizes="76px"`; with the template change reverted it fails with "archive thumbnail lacks a srcset for the hover preview".
- `TestVideos` checks the poster thumbnail srcset.
- Full `go test ./...` against a disposable PostgreSQL 17, `golangci-lint run ./...` (0 issues), `pnpm check`, `pnpm test`.
