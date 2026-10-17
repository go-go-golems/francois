---
Title: Screenshot and report evidence index
Ticket: FRANCOIS-009
Status: active
Topics:
    - backend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/scripts/05-verify-production.mjs
      Note: Assertions-behind-the-production-figures
    - Path: repo://ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/various/production-receipt.json
      Note: Published-release-and-functional-evidence
    - Path: repo://ttmp/2026/10/16/FRANCOIS-009--import-real-html-artifacts-as-hosted-proofs-of-concept/various/screenshots/lattice-rift-production.png
      Note: Production-layout-and-rendering-figure
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T19:30:46.553805934-04:00
WhatFor: ""
WhenToUse: ""
---

# Screenshot and report evidence index

This is the evidence map for the later go-go-parc vault report. It separates visible screenshots from assertions that require API/browser receipts. Screenshots are original PNG captures from Chromium at 1440×1000. They contain public application code, generated scenes and sample documents; no credentials, browser session state or private RAG corpus is included.

## Suggested narrative and figures

Start with the import boundary: Francois publishes immutable single-file HTML releases. The downloaded ZIPs contain classic scripts and static resources, so a deterministic Python packaging step is sufficient. Then explain the hosted capability boundary, the original private-corpus exclusion, the small runtime adaptations and actual local/production verification. Finish with what was tested and what remains outside this proof of concept.

| Figure | Local screenshot | Production screenshot | Suggested caption and evidence scope |
| --- | --- | --- | --- |
| Rift | [Local](../various/screenshots/lattice-rift-local.png) | [Production](../various/screenshots/lattice-rift-production.png) | Browser-rendered WebGL lattice, with space between Freeze and the host Close link. The receipt records a successful real pointer toggle. |
| Phantasmagoria | [Local](../various/screenshots/lattice-phantasmagoria-local.png) | [Production](../various/screenshots/lattice-phantasmagoria-production.png) | Local boot screen and production cartridge scene after A. Two canvases run without hosted model calls. |
| Tabula | [Local](../various/screenshots/tabula-workbench-local.png) | [Production](../various/screenshots/tabula-workbench-production.png) | Reactive 151-node workbook with editor, histogram and live input grid. Persistence is browser-local; the receipt proves the changed cell survives reload. |
| Deck | [Local](../various/screenshots/lattice-deck-local.png) | [Production](../various/screenshots/lattice-deck-production.png) | Live 3D stage, slide IR graph and scripted agent transcript. The successful command changes speed to 2.4; this is separate from the real model capability check. |
| RAG | [Local](../various/screenshots/rag-workbench-local.png) | [Production](../various/screenshots/rag-workbench-production.png) | Public retrieval corpus, editable pipeline and ranked results. Five bundled guides plus the uploaded public fixture demonstrate actual document persistence. |
| Catalog | — | [Public catalog](../various/screenshots/catalog-production.png) | Real production catalog showing the imported applications alongside the existing catalog. |
| Write-up | — | [RAG public detail](../various/screenshots/rag-workbench-public-page.png) | Public explanation of the imported RAG workbench and the sample-data boundary. |

The local screenshots were refreshed after the final toolbar-clearance edits. Production captures are made after explicit workspace upgrades, so they show the tested release rather than an older pinned workspace. Compare release SHA256 values in the build and production receipts before using the figures to describe a particular revision.

## Machine-readable evidence

- [Original input inventory](../sources/inventory.json): original Downloads paths, sizes, SHA256 values and archive entries. Original ZIPs/HTML files were never changed.
- [Build inventory](../sources/builds.json): generated release bytes, SHA256 values and requested manifests.
- [Local functional receipt](../various/local-functional-receipt.json): observed controls, workbook persistence, scripted agent action, RAG upload persistence and three sequential uncached model calls.
- [Production receipt](../various/production-receipt.json): actual published IDs/versions/digests, server validation, isolated origins, per-app interactions, sampling and bounded model ledger records. Only claim complete production qualification when browser contains all five successful entries.
- [Public receipt](../various/public-receipt.json): unauthenticated HTTP 200 pages, public catalog and all five applications in release RSS.
- [Implementation diary](01-implementation-diary.md): chronological decisions, failed attempts, fixes and commit checkpoints.
- [Import guide](../design-doc/01-artifact-import-adaptation-and-publication-guide.md): reproducible build/publication procedure, API references and limitations.

## Interpretation limits

A screenshot proves visible rendering at the captured viewport; it does not prove all presets, cross-browser behavior, multi-user collaboration or every export format. Model calls in the receipts are short explicit sampling probes, not full model-driven workbook/code-editing sessions. A scripted Deck agent result does prove executable stage manipulation, but it must not be described as model-generated output.

Reload/upgrade intentionally aborts old launch requests and event streams. The production receipt retains these ERR_ABORTED entries rather than presenting them as unexpected resource failures. Inspect any other failed request or page error separately.

The original RAG corpus is excluded, including notes marked publish:false. The public fixture is an intentional test upload. Initial qualification preserved the access policy and global model lane count. The later explicit public-access request enabled the existing bounded tours and free HTML downloads; the current guide and CLI receipt record that change and the preserved limits.


## Diagnostic figure

[Production RAG during the test API mismatch](../various/screenshots/rag-workbench-production-debug.png) preserves the earlier diagnostic capture. The application already rendered the correct ranked results; the test incorrectly read the JSON projection as the runtime object. Use the final production image for the main result and this figure only when explaining that investigation.


## Curated public covers and galleries

A second capture pass responds to the request for attractive screenshots and multiple views per app. Seventeen actual application images are published: one cover per app, two further gallery views for Rift/Phantasmagoria/RAG, and three further views for Deck and Tabula. Deck uses a scene-only cover crop so catalog thumbnails do not truncate the slide headline. Workspaces use the production HTML hashes, built-in examples and the public sample corpus. Deck/Tabula figures use direct slide-element crops; other figures use a 1800×1125 viewport. These are browser captures, not mockups or retouched illustrations.

The image-selection manifest [sources/showcase.json](../sources/showcase.json) records captions, dimensions, PNG hashes and source HTML hashes. The [CLI publication receipt](../various/gallery-publication-receipt.json) records actual media IDs, cover/gallery order, public release IDs and the preserved tour policy. The [public verification receipt](../various/public-gallery-verification.json) records unauthenticated page/image/download checks and the real bounded tour.

| Application | Selected views | Public page |
| --- | --- | --- |
| lattice-rift | [rift-title](../various/showcase/rift-title.png), [rift-arcade](../various/showcase/rift-arcade.png), [rift-world](../various/showcase/rift-world.png) | [View](https://slop.boutique/apps/lattice-rift) |
| lattice-phantasmagoria | [phantasmagoria-menu](../various/showcase/phantasmagoria-menu.png), [phantasmagoria-cartridge](../various/showcase/phantasmagoria-cartridge.png), [phantasmagoria-kipple](../various/showcase/phantasmagoria-kipple.png) | [View](https://slop.boutique/apps/lattice-phantasmagoria) |
| lattice-deck | [lattice-deck-cover](../various/showcase/lattice-deck-cover.png), [lattice-deck-procedural](../various/showcase/lattice-deck-procedural.png), [lattice-deck-motion](../various/showcase/lattice-deck-motion.png), [lattice-deck-charts](../various/showcase/lattice-deck-charts.png) | [View](https://slop.boutique/apps/lattice-deck) |
| tabula-workbench | [tabula-overview](../various/showcase/tabula-overview.png), [tabula-story](../various/showcase/tabula-story.png), [tabula-sensitivity](../various/showcase/tabula-sensitivity.png), [tabula-landscape](../various/showcase/tabula-landscape.png) | [View](https://slop.boutique/apps/tabula-workbench) |
| rag-workbench | [rag-search](../various/showcase/rag-search.png), [rag-trace](../various/showcase/rag-trace.png), [rag-corpus](../various/showcase/rag-corpus.png) | [View](https://slop.boutique/apps/rag-workbench) |

Use these curated images for the main vault-report figures. Retain the earlier startup/production screenshots as implementation evidence. Final public presentation captures include [the illustrated catalog](../various/showcase/catalog-public-covers.png), [the Deck gallery](../various/showcase/deck-public-gallery.png), and [an anonymous RAG tour](../various/showcase/rag-anonymous-tour.png).

The final [portal rollout verification](../various/portal-rollout-verification.json) supersedes the earlier image-count check: all seventeen images load, each app has exactly one current HTML download, and two fresh anonymous instance titles exactly match their public pages. It also verifies twenty-minute tour lifetimes and title cleanup on Close, without model calls. [Deployment evidence](../various/portal-deployment-receipt.json) identifies the source/GitOps commits, running image and healthy replica. [Deck instance](../various/showcase/lattice-deck-instance-title.png) and [RAG instance](../various/showcase/rag-workbench-instance-title.png) show the launched applications; document.title assertions come from the JSON receipt because browser viewport screenshots do not include tab chrome.
