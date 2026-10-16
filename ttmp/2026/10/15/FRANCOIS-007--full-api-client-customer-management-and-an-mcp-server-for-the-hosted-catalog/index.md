---
Title: Full API client, customer management and an MCP server for the hosted catalog
Ticket: FRANCOIS-007
Status: active
Topics:
    - backend
    - cli
    - mcp
    - billing
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://ttmp/2026/10/15/FRANCOIS-007--full-api-client-customer-management-and-an-mcp-server-for-the-hosted-catalog/design-doc/01-full-api-client-customer-management-and-mcp-server-analysis-design-and-implementation-guide.md
      Note: Primary analysis, design and implementation guide
    - Path: repo://ttmp/2026/10/15/FRANCOIS-007--full-api-client-customer-management-and-an-mcp-server-for-the-hosted-catalog/working-note/01-investigation-diary.md
      Note: Investigation diary
ExternalSources: []
Summary: "Design (not yet implemented) for scoped API tokens, customer-management endpoints, a typed Go client, a complete francois catalog command tree and an MCP server (stdio, then remote /mcp). Guide written and uploaded; implementation phases 1-6 open."
LastUpdated: 2026-10-15T18:46:20.823652599-04:00
WhatFor: ""
WhenToUse: ""
---

# Full API client, customer management and an MCP server for the hosted catalog

## Overview

The hosted catalog can be operated from scripts only through `francois catalog push`; everything else needs a browser session or `kubectl exec`. This ticket designs the path to a full API client — including customer management by administrators — and an MCP server so agents can operate the shop through the same API and authorization checks.

- Guide: [design-doc/01](design-doc/01-full-api-client-customer-management-and-mcp-server-analysis-design-and-implementation-guide.md) — orientation, current state with evidence, gap table, nine decision records, client and command reference, pseudocode, security checklist, six phases, test strategy.
- Diary: [working-note/01](working-note/01-investigation-diary.md).

Status: the design is written; no code has changed. Phases 1–6 are open tasks.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- backend
- cli
- mcp
- billing

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
