---
Title: 'Site improvements: thumbnail and image quality'
Ticket: FRANCOIS-021
Status: active
Topics:
    - frontend
    - performance
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Storefront thumbnail/variant quality: hover preview upscaling, JPEG quality and versioned variant URLs."
LastUpdated: 2026-10-23T14:46:36.093348924-04:00
WhatFor: ""
WhenToUse: ""
---

# Site improvements: thumbnail and image quality

## Overview

Site improvements for slop.boutique. First item: thumbnails looked compressed because the archive hover preview drew the 160 px variant at 432 px, and variants were JPEG quality 82. See the analysis doc for measurements and the fix, and the diary for the work log.

## Key Links

- **Related Files**: See frontmatter RelatedFiles field
- **External Sources**: See frontmatter ExternalSources field

## Status

Current status: **active**

## Topics

- frontend
- performance

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
