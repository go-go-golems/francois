---
Title: Add a download-only Slop Boutique intro animation to HTML exports
Ticket: FRANCOIS-014
Status: active
Topics:
    - frontend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: A short self-contained branded intro on downloaded HTML, with hosted instances and immutable source left unchanged.
LastUpdated: 2026-10-17T16:45:24.463209145-04:00
WhatFor: Implement the requested download-only attribution animation.
WhenToUse: Changing authorized HTML export presentation.
---
# Download-only Slop Boutique intro

Queue this after completed FRANCOIS-013 email sign-in. Show **“slop.boutique - explorations by manuel and ramon.”** in a short playful animation when opening a saved HTML download, then reveal the real app. Hosted instances on the site must never show it.

The proposed implementation is one self-contained CSS intro fragment inserted at the authorized download boundary. It does not change immutable uploaded source, authored app startup, hosted runtime injection, attachments or access policy. This ticket contains the implementation design; the animation is not implemented yet.

- [Design and implementation notes](design-doc/01-download-only-animated-intro-design-and-implementation-notes.md): request flow, animation behavior, pseudocode, file/API references and verification.
- [Current source evidence](sources/current-byte-paths.md), with a repeatable capture script.
- [Detailed investigation diary](reference/01-investigation-diary.md).
- [Implementation tasks](tasks.md) and [changelog](changelog.md).
- [Completed email sign-in prerequisite](../FRANCOIS-013--implement-email-link-sign-in/index.md).

The implementation must calculate the downloaded representation's actual length and checksum, while preserving source metadata. Verification must open a genuinely downloaded file offline and demonstrate that hosted HTML contains no intro.
