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
Summary: ""
LastUpdated: 2026-10-11T18:38:50.115762318-04:00
WhatFor: ""
WhenToUse: ""
---

# Host Claude artifacts with Glazed and Pi model runtime

Francois will serve saved HTML artifacts and supply their browser capabilities, using a small Pi model worker for provider/auth access. The scaffold and detailed design are delivered; service implementation remains open.

## Reading order

- [Analysis, design, and intern implementation guide](design-doc/01-claude-artifact-host-analysis-design-and-intern-implementation-guide.md)
- [Investigation diary](working-note/01-investigation-diary.md)
- [Source manifest](sources/manifest.json)
- [Implementation tasks](tasks.md)
- [Changelog](changelog.md)

## Scope

First make POLIS sampling, streaming, and browser eval tools work. Then add documents, publishing, files, assets, local identity, rooms, and the comments shell. Keep platform-specific Claude conversation/connector operations explicitly unavailable until implemented.
