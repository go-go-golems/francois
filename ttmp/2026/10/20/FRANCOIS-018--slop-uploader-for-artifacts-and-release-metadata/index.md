---
Title: Slop uploader for artifacts and release metadata
Ticket: FRANCOIS-018
Status: active
Topics:
    - frontend
    - backend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-20T21:24:11.781943261-04:00
WhatFor: ""
WhenToUse: ""
---

# Slop uploader for artifacts and release metadata

## Overview

Design a single streamlined artifact uploader from the supplied JPEG and standalone prototype. The proposed implementation is a Studio screen that reuses existing authentication, creator permissions and upload APIs. The implementation guide is complete; application implementation remains open. Studio delivery is an explicit assumption pending any preference for the standalone local form.

## Documents

- [Analysis, design and intern implementation guide](design-doc/01-slop-uploader-analysis-design-and-intern-implementation-guide.md)
- [Investigation diary](reference/01-investigation-diary.md)
- [Source inventory](sources/README.md)
- [Implementation tasks](tasks.md)

## Scope

One form for HTML, metadata, capabilities, media, write-up and attachments. Existing endpoints handle check, draft creation and publication. Preserve public free-sample downloads; explain live metadata updates and partial publication honestly. No new database schema, service or queue.

## Delivery

The guide was uploaded to reMarkable cloud at `/ai/2026/10/20/FRANCOIS-018` as **FRANCOIS-018 Slop Uploader Implementation Guide**. See [delivery receipt](sources/remarkable-delivery.json).
