---
Title: Diary
Ticket: FRANCOIS-021
Status: active
Topics:
    - frontend
    - performance
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Chronological diary for the thumbnail quality fix."
LastUpdated: 2026-10-23T14:46:36.486279502-04:00
WhatFor: ""
WhenToUse: ""
---
# Diary

## 2026-10-23 — Thumbnails look compressed

**Trigger.** After publishing six new apps with 1600-2400 px covers, the user reported that thumbnails on slop.boutique look compressed.

**Investigation.**
- Read `content.Resize` and the media handler: variants are fixed widths 160/320/640/1200, JPEG quality 82 for opaque images, cached as `<sha>-w<width>` and served `immutable`.
- Fetched `?w=640` and `?w=1200` of two new gallery images: baseline JPEGs at the requested width, as designed.
- The home page `<img>` tags carry only `?w=160`. Hovering a row with Playwright showed the preview box at 432x270 drawing a 160x100 image. The `mini` template has no srcset, and `bindArchive` clones it into the preview. This is the main cause.
- Compared JPEG settings with Pillow on a 1200 px interface screenshot (table in the analysis doc). Quality 90 is +3.5 dB for +27% bytes; Go's encoder cannot avoid 4:2:0.

**Decisions.**
- Fix the preview by giving `mini` a srcset with `sizes="76px"` and setting the clone's `sizes` to the preview width. That keeps the 76 px row cheap and lets the browser choose the preview variant.
- Version the variant URL (`&v=2`) and cache name because responses are immutable; otherwise browsers that already hold quality-82 files would keep them for a year.
- Video posters have no known width in the template, so they get `variantSet` (all widths; widths beyond the original fall back to the original).

**Pitfalls.**
- The variant tests need `TEST_DATABASE_URL`; without it they skip and `go test` reports `ok`. Used a disposable local PostgreSQL 17 container; the harness creates and drops its own database.
- Confirmed the new assertion fails with the template change reverted.

**Validation.** `go test ./... -count=1` (with the test database), `golangci-lint run ./...` 0 issues, `pnpm build`, `pnpm check`, `pnpm test`.

**Next.** Deploy; re-measure the hover preview natural size on production; consider lossless WebP uploads for screenshot galleries.
