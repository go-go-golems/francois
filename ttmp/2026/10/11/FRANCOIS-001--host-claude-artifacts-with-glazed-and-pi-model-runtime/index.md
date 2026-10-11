---
Title: Host Claude artifacts with Glazed and Pi model runtime
Ticket: FRANCOIS-001
Status: active
Topics:
    - backend
    - llm
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Working local artifact host with Pi sampling, browser tools, persistent capabilities, minimalist UI, and validated local packaging."
LastUpdated: 2026-10-11T18:38:50.115762318-04:00
WhatFor: ""
WhenToUse: ""
---

# Host Claude artifacts with Glazed and Pi model runtime

The local implementation is complete and tested: Glazed commands, embedded library, isolated artifact origins, Pi model worker, browser tools, images, persistent data, rooted files, assets, presence, and comments shell. Specific platform extensions remain open follow-ups in tasks.md.

## Reading order

- [Current runbook and support matrix](../../../../../README.md)
- [Detailed implementation diary](working-note/01-investigation-diary.md)
- [Original intern analysis and design](design-doc/01-claude-artifact-host-analysis-design-and-intern-implementation-guide.md) — historical research handoff written before implementation
- [Library screenshot](reference/02-francois-library.png)
- [Source snapshots and hashes](sources/manifest.json)
- [Tasks and remaining extensions](tasks.md)

## Verified behavior

Real Pi text streamed successfully. One real model eval changed POLIS tower height from 6 to 7. Browser checks verified shared state, pending snapshots, partial cancellation, asset/file operations, comments, image transformation, and room separation. Race-enabled Go tests, TypeScript checks, lint, generation, and local packaging passed. See the diary for exact evidence and limits.

## Open scope

Precise files-form publishing and permissions types, SVG sanitization, custom anchors, genuine multi-user identity, and conversation/MCP handoff remain separate follow-ups. This service uses one local owner and their Pi configuration.
