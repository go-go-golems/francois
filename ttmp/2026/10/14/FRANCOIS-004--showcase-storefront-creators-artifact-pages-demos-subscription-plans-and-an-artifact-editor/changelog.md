# Changelog

## 2026-10-14

- Initial workspace created

## 2026-10-14

Design doc written (orientation, current state, gaps, architecture with decision records, API reference, pseudocode, phases, tests) and uploaded to reMarkable /ai/2026/10/14/FRANCOIS-004

## 2026-10-14

Phase 1 content model (19ba56b): migration 004, store for pages/creators/media/posts, Markdown and media detection, upload keeps existing metadata

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/pkg/catalog/content/markdown.go — Safe Markdown rendering
- /home/manuel/code/wesen/go-go-golems/francois/pkg/catalog/store/migrations/004_storefront.sql — Schema for all storefront phases

## 2026-10-14

Phase 2 public pages (ab1598d): server-rendered storefront, /media, RSS, public.js, SPA catalog removed

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/pkg/catalog/pages.go — Storefront handlers

## 2026-10-14

Phase 3 studio API (68a7daf): creator guard level, /api/studio for artifacts, drafts, releases, media, posts, creators; admin release routes removed

### Related Files

- /home/manuel/code/wesen/go-go-golems/francois/pkg/catalog/studio.go — Studio API
