---
Title: Storefront browser in the html.moldandyeast.com style, per-app launch mode
Ticket: FRANCOIS-022
Status: active
Topics:
    - frontend
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/settings/copy.go
      Note: Editable site copy registry
    - Path: repo://pkg/catalog/web/templates/home.tmpl
      Note: Landing page browser
    - Path: repo://ui/catalog/launch-decision.ts
      Note: Launch rules (who may start, subscribe, inline/tab)
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-23T15:20:02.611202977-04:00
WhatFor: ""
WhenToUse: ""
---

# Storefront browser in the html.moldandyeast.com style, per-app launch mode

## Overview

Rebuild the slop.boutique landing page in the html.moldandyeast.com style (keeping header, support banner and footer), add a per-app launch mode (`inline` or `tab`), and make hard-coded site copy editable. Start with the design guide in design-doc/.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- frontend
- backend

## Tasks

See [tasks.md](./tasks.md) for the current task list.

## Changelog

See [changelog.md](./changelog.md) for recent changes and decisions.

## Structure

- design/ - Architecture and design documents
- reference/ - Prompt packs, API contracts, context summaries
- playbooks/ - Command sequences and test procedures
- scripts/ - Temporary code and tooling
- various/ - Working notes and research
- archive/ - Deprecated or reference-only artifacts
