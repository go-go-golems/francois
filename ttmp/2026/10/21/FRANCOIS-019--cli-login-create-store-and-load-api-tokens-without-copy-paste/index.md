---
Title: 'CLI login: create, store and load API tokens without copy-paste'
Ticket: FRANCOIS-019
Status: active
Topics:
    - cli
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://cmd/francois/cmds/api.go
      Note: CLI token loading entry point
    - Path: repo://pkg/catalog/tokens.go
      Note: Token creation entry point
ExternalSources: []
Summary: "Design for browser-approved CLI device login that mints scoped personal API tokens, plus a local credentials file loaded by every hosted-API command."
LastUpdated: 2026-10-21T10:26:53.097441-04:00
WhatFor: ""
WhenToUse: ""
---

# CLI login: create, store and load API tokens without copy-paste

## Overview

Today a CLI user must create a personal API token on `/account`, copy it, and pass it with
`FRANCOIS_TOKEN` or `--token-file` (the CLI stores nothing). This ticket designs:

- `francois catalog auth login`: an RFC 8628-style device flow. The CLI shows a code, the user
  approves it in a signed-in browser at `/device`, and the CLI receives a scoped `frt_` token.
  Tokens are still minted only from a browser session.
- A `0600` credentials file (`~/.config/francois/credentials.json`) keyed by portal origin,
  used automatically by API commands, `releases push` and `francois mcp`. `FRANCOIS_TOKEN` and
  `--token-file` keep precedence.

Status: design delivered; implementation not started (owner decisions in design doc §11).

## Key Links

- **Design doc**: [design-doc/01-cli-token-login-analysis-design-and-intern-implementation-guide.md](./design-doc/01-cli-token-login-analysis-design-and-intern-implementation-guide.md)
- **Diary**: [reference/01-investigation-diary.md](./reference/01-investigation-diary.md)
- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- cli
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
