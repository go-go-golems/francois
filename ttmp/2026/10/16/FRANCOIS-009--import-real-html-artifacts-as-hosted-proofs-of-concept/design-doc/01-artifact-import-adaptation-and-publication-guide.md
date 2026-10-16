---
Title: Artifact import, adaptation and publication guide
Ticket: FRANCOIS-009
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://examples/imports/README.md
      Note: Application inventory and reproduction entry point
    - Path: repo://examples/imports/rag-workbench/demo-corpus.json
      Note: Public replacement for the excluded private corpus
    - Path: repo://ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/scripts/02-build-imports.py
      Note: Deterministic packaging and hosting adaptations
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T19:07:30.710886884-04:00
WhatFor: ""
WhenToUse: ""
---

# Artifact import, adaptation and publication guide

## Scope and current checkpoint

This ticket imports real downloaded applications into Francois's existing hosted catalog. Two Lattice ZIPs contain separate visual applications; the other exports are Tabula Workbench, Lattice Deck and RAG Workbench. The source inventory records exact input filenames, SHA256 values and extracted program files. Original Downloads files are unchanged.

At this checkpoint all five single-file releases validate and have been published locally. Baseline browser startup succeeds for Rift, Phantasmagoria, Tabula and RAG. Deck exposed a collaboration presence-promise assumption; its packaged correction and RAG's improved initial query are published locally as 1.0.1 and their workspaces explicitly upgraded. Primary-interaction/model checks and production publication follow before final qualification.

## Why ZIPs need packaging

The hosted release is one HTML document plus a capability manifest. These ZIPs are exported classic HTML/JavaScript applications, not npm projects: there is no package.json or Vite build to run. Rift and Phantasmagoria each need their five local scripts embedded in the existing HTML. RAG additionally needs its presets, corpus and library source available without separate filesystem URLs.

`examples/imports/<artifact>/source/` keeps original program files. `scripts/02-build-imports.py` produces each `release.html` and `manifest.json`. The builder replaces local script tags in order and escapes closing-script sequences. RAG's embedded JSON escapes less-than signs and round-trips library text exactly.

## Public corpus boundary

The original RAG corpus contains customer/project notes marked publish:false. It is not copied into the public repository or release. The copy script excludes that entry, and `examples/imports/rag-workbench/demo-corpus.json` supplies five public Francois/RAG guides instead. Program presets are preserved as examples, with lexical minimal search selected initially and its query changed to model concurrency. Uploaded private user documents remain a separate runtime capability, never part of the public release.

## Adaptations and capability manifests

Rift and Phantasmagoria request no hosted capability. Tabula declares sample/downloads. Deck declares sample/db/user/room; RAG declares sample/db/user/downloads. Declaring only required capabilities avoids depending on default sample admission for browser-only applications.

Deck's room.presence call assumed a returned Promise. Francois's existing presence method is fire-and-forget; the builder wraps the two application call sites in Promise.resolve before their error handler. RAG's existing asynchronous map concurrency default is reduced from three to one to fit one model call per user. These are direct application edits, not new platform compatibility layers.

Pinned React/Three/HTM CDN references and Google Fonts remain as exported; the validator reports the scripts as informational external dependencies. Original application styling remains intact.

## Reproduction

Run from the Francois repository root:

```sh
python3 ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/scripts/01-copy-sources.py
python3 ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/scripts/02-build-imports.py
```

The copy step requires the five original Downloads files, validates archive paths/size and refuses to overwrite changed source copies. Building needs only Python's standard library. Hash/size manifests are in `sources/inventory.json` and `sources/builds.json`.

For local publication, an authenticated administrator opens the existing studio API. The browser uses `/api/session` for CSRF, sends multipart HTML/manifest to `POST /api/studio/releases/check`, uploads to `POST /api/studio/releases`, then calls `POST /api/studio/releases/{id}/publish`. This is the same service used by the supported catalog CLI. Revised workspaces need `POST /api/instances/{id}/upgrade` because releases are pinned deliberately.

The normal CLI equivalent is `go run ./cmd/francois catalog releases push --server ORIGIN --token-file PRIVATE_TOKEN --artifact-id ID --version VERSION --html examples/imports/ID/release.html --manifest examples/imports/ID/manifest.json --publish`. The CLI reads its existing environment fallback, so notify before use and keep credentials out of shell output. No browser cookies need to be exported to a file.

## Initial evidence and remaining work

`scripts/03-local-browser-check.js` opens each application through the trusted launcher on its isolated origin, records errors/network failures and captures screenshots. Baseline evidence shows real WebGL/canvas rendering, Tabula's 151-node workbook build, Deck's 11-node/two-slide build and RAG's five-document/15-chunk corpus. The detailed diary distinguishes startup from full functional validation and records the initial Deck error. Final interaction/persistence/model and production receipts will be added after execution.
