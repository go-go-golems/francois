---
Title: Investigation diary
Ticket: FRANCOIS-014
Status: active
Topics:
    - frontend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T16:45:37.176940189-04:00
WhatFor: ""
WhenToUse: ""
---
# Investigation diary

## Step 1: Queue a download-only introduction after completing email sign-in

The user requested a short branded animation when opening downloaded HTML, explicitly excluding the copies running on the site. I created this ticket after the email-link implementation and investigated the two existing source-byte consumers before choosing the download handler as the narrow implementation boundary. This is a queued implementation design; application code has not been changed for the intro.

### Prompt Context

**User prompt (verbatim):** “Create a new docmgr ticket to add a "water mark intro animation" when opening a downloaded HTML (not the ones on the site themselves) which says "slop.boutique - explorations by manuel and ramon." in some fun animation and then just show the real app.

Do it after the login email link, or if you think a subagent can do it earlier.”

**Assistant interpretation:** Create a concrete ticket for a brief, self-contained download-only animation, sequenced after email login.
**Inferred user intent:** Give shared/downloaded experiments a recognizable playful attribution without adding friction to experiments viewed on the site.
**Preceding implementation commits:** 2f2fe66 and 0b1c302 finish the working email-link feature and browser qualification.

### What I did

- Created FRANCOIS-014 with docmgr and wrote the focused design/implementation notes.
- Read routes.go:download/mayDownload, the immutable release-file model and blob reader, and hosted/routes.go:page/inject.
- Specified exact wording, a roughly 1.6-second self-contained CSS intro and immediate reduced-motion bypass.
- Identified response checksum/length changes and GET/HEAD parity as required implementation details.
- Queued meaningful helper, authorization, source-integrity, file:// and hosted-absence checks.

### Why

Download-time transformation affects only the requested representation. Changing uploaded source or hosted injection would put the animation on the site, contrary to the user's explicit boundary. A CSS fragment avoids a new service, build system, database setting or app-script interception.

### What worked

The existing architecture already separates attachment downloads from authenticated hosted HTML. The current download handler is small and computes the response only after authorization, so the feature has one concrete integration point.

### What didn't work

One investigation probe guessed pkg/catalog/web/cache.go; it does not exist. Reading the actual download handler and hosted route supplied the relevant header and injection behavior. No application execution or animation test is claimed.

### What I learned

The download response currently advertises the stored blob checksum. A derived export must calculate the outgoing checksum and length rather than report the original source values. The immutable database metadata must remain source metadata.

### What was tricky to build

No code was built yet. The design addresses preserving authored startup behavior and parsing boundaries: show an overlay while the app initializes, do not rewrite script execution or embed the original app in another document.

### What warrants a second pair of eyes

Review missing-body fallback behavior, authored CSP interactions, reduced-motion fallback, outgoing checksum semantics and proof that hosted responses never invoke the export transformation.

### What should be done in the future

Implement the four scoped tasks, then capture the animation and completed app from a genuinely downloaded file. No extra presentation framework is proposed.

### Code review instructions

Start with the design's flowchart and the existing download handler. Verify that the planned helper is only called for release files whose role is html, after mayDownload succeeds. This checkpoint contains documentation only.

### Technical details

- Existing route: GET/HEAD /api/releases/{release}/files/{file}/download.
- No new route, database migration, external asset or permanent watermark.
- Hosted page boundary: pkg/catalog/hosted/routes.go:page.
- Exact wording: slop.boutique - explorations by manuel and ramon.
