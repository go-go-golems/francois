---
Title: Measure and improve slop.boutique page and application startup speed
Ticket: FRANCOIS-010
Status: active
Topics:
    - frontend
    - performance
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: "Measured public-page and portal transfer improvements, with a detailed vault report and a focused production release."
LastUpdated: 2026-10-16T20:36:11.385359511-04:00
WhatFor: ""
WhenToUse: ""
---

# Measure and improve slop.boutique page and application startup speed

## Overview

Improve the actual loading cost of slop.boutique using existing handlers and publishing APIs. The accepted shipping scope is content-versioned gzip/ETag static assets, smaller screenshot delivery copies and eager detail covers. Preserve authenticated runtime responses and hosting limits. The analysis teaches the complete request path and separates measured defects from conditional future work.

## Key Links

- [Technical report and implementation guide](analysis/01-page-speed-engineering-measured-costs-cache-design-and-implementation-guide.md)
- [Detailed implementation diary](reference/01-implementation-diary.md)
- [Cold/warm baseline](various/cache-before.json)
- [Startup probe: network tour limit](various/startup-before.json)
- Source PR: https://github.com/go-go-golems/francois/pull/9
- Vault note: /home/manuel/code/wesen/go-go-golems/go-go-parc/Projects/2026/10/16/PROJ - Francois - Page Speed and Browser Cache Engineering - A Technical Deep Dive.md
- Import sources, PNG originals and media delivery receipts remain with sibling ticket FRANCOIS-009.

## Status

Current status: **active**

The detailed report is published and pushed to go-go-parc. Source PR #9 passed all checks and merged as 393bf35; its normal production image/GitOps rollout is in progress. Final rollout and browser measurements will be retained here before completion. No end-to-end app-startup speedup is claimed from a declined tour probe.

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
