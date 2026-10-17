---
Title: Investigation diary
Ticket: FRANCOIS-011
Status: active
Topics:
    - frontend
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Evidence, decisions and limits of the Final-versus-production design comparison.
LastUpdated: 2026-10-17T14:03:27-04:00
WhatFor: Preserve why the authoritative design needs structural and functional implementation changes.
WhenToUse: Resuming the Final design alignment or reviewing its evidence.
---

# Investigation diary

## Step 1: Inspect the authoritative Final page and capture the current implementation

I compared Ramon's Final artboards with the public production site and the existing Go/TypeScript source. Final changes the public information layout and introduces controls whose behavior is not implemented by the current APIs. The deliverable is a concrete implementation plan and preserved evidence, with no frontend implementation or deployment claimed.

This entry backfills the investigation performed earlier in this turn. The source revision was `57c896c`; the workspace was clean before ticket creation. The old FRANCOIS-006 design reference is historical and should no longer dictate the public design.

### Prompt Context

**User prompt (verbatim):** "Look at Final in ramon's slop boutique paper design https://app.paper.design/file/01M4XVT9C83GGYK57CMAAAEJJT/p-6-0 and compare to what we currently have and how to update things. Ramon's the right design."

**Assistant interpretation:** Read the specified Final page, compare its desktop/mobile surfaces with production and source, and explain the smallest implementation changes needed.

**Inferred user intent:** Restore the intended minimal lab/archive presentation and use Ramon’s actual current work as authority. Continue the standing detailed-diary and periodic-commit workflow.

### What I did

- Read the Paper MCP guide, fetched basic file/page metadata, and inspected Final's ten artboards: archive, experiment, support, sign-in and signed-in account, each at desktop/mobile widths.
- Exported inline-style JSX for all ten, selected computed styles and file tokens. Reviewed eight artboard screenshots in the conversation and exported all ten PNGs into the ticket.
- Queried Final comments with `status: all`; the valid result contained zero threads.
- Inspected `layout.tmpl`, `home.tmpl`, `artifact.tmpl`, `pricing.tmpl`, `pages.go`, `routes.go`, `auth.go`, `demos.go`, store page/release models, the payment gateway and existing layout tests.
- Inspected React Header/Footer, Account, Launcher, public action enhancements and Studio editor entry points.
- Created FRANCOIS-011 and its design document/diary through docmgr.
- Ran `node ttmp/2026/10/17/FRANCOIS-011--align-slop-boutique-with-ramon-final-design/scripts/01-capture-live.mjs`. It captured eight public pages at 1440/390, recorded status/title/text/computed heading sizes, and checked two machine-readable destinations. Used an isolated browser context, without sign-in, payment or demo creation.
- Independently loaded the public archive through a separate Playwright context. Closed only the context created for this investigation; preserved existing tabs.
- Wrote the comparison, exact geometry table, source/API mapping, pseudocode, architecture diagram, staged shipping plan and acceptance checks.

### Why

The older design ticket uses another Paper file. Incrementally polishing that implementation would preserve the wrong structure. Reading exact Final JSX also prevents guessing dimensions from screenshots. Source inspection distinguishes simple visual changes from missing identity, payment and embedded-runner behavior.

### What worked

- Paper access succeeded, including all desktop/mobile exports. The returned token hash was `74b865ac`.
- All eight public page navigations returned 200 with no horizontal page overflow at capture time. The actual archive contains six experiments, not the mockup’s 21.
- Production `/llms.txt` and `/feed.json` returned 404, confirming that two Final footer destinations need implementation.
- Existing release files already contain filename/size. Existing hosted execution already supplies origin isolation, bounded guest demos and the launch handshake. Neither needs a replacement service.
- Existing source clearly establishes OIDC login, fixed-price Checkout and the old design-specific test expectations.
- Validation checkpoint: docmgr doctor reported All checks passed; relative Markdown links all resolve; inventory verified ten Paper PNGs, ten JSX exports, eight production PNGs and ten browser observations. The final staged whitespace check passed after normalizing the token export EOF. Runtime tests were not run for this documentation-only change.

### What didn't work

- The first `list_comment_threads` call used `previewLength: 2000`; Paper returned `maximum: 1000` and `message: Invalid input`. Retried at 1000 and obtained the empty inventory.
- An exploratory `rg` included `pkg/catalog/hosted/http.go`, which does not exist: `rg: pkg/catalog/hosted/http.go: No such file or directory (os error 2)`. Read the actual `hosted/routes.go` and existing runtime/launcher sources instead.
- A documentation patch attempted Delete File and Add File for the same path in one operation. `apply_patch verification failed: invalid patch: multiple operations target` the comparison document. No document content was applied by that failed patch; rewrote the intended file using a quoted heredoc.

- The first staged whitespace check reported `sources/paper-tokens.css:101: new blank line at EOF.` The shell sequence continued to commit because it lacked failure gating. Removed the extra blank line, used explicit failure gating for the correction, and amended the unpublished documentation commit.

### What I learned

Final is a compact experiment archive, not the current image-card feed. Most visible work fits templates and CSS. Real email links, arbitrary support amounts, receipt data and inline execution need explicit functional work. The signed-in account API exposes more internal data than the minimal current UI uses, but still lacks the exact supporter summary shown in Final.

### What was tricky to build

This was an investigation rather than an implementation. The main boundary was the live embed: automatically starting an existing demo consumes a visit and capacity. A screenshot labeled Live is not proof of an agreed startup policy. The plan preserves the target layout while making the automatic-versus-explicit-start choice visible. It does not quietly add unlimited instances or discard the current resource limits.

Stable archive numbers also cannot come from filtered row position. Proposed first-publication numbering/date rules are labeled as implementation choices and kept distinct from measured Paper properties.

### What warrants a second pair of eyes

- Whether root is experiments-only while journal writing remains separately reachable.
- Automatic embed startup versus an inline start state, given visit limits and active-slot caps.
- Real annual-funding amount and donation/entitlement separation.
- Existing identity provider support for the promised passwordless journey.
- Receipt and identity-email controls must lead to real capabilities, not invented data or billing-only email changes.

### What should be done in the future

Implement the three shipping slices in the comparison document, with screenshot review at each visible milestone and focused tests when runtime behavior changes. Keep the bounded hosting and cache improvements. This investigation does not authorize a new architecture or claim completed frontend work.

### Code review instructions

Start with the comparison table and measurements, then compare the paired `sources/paper-*` and `sources/live-*` images. Read `live-observations.json` for the captured titles/statuses and exact heading sizes. Follow the file map to current code. Run docmgr doctor and verify every relative Markdown image/document link exists. No Go test result is claimed for this documentation-only change.

### Technical details

- Artboards: 1440px desktop and 390px mobile; 10 JSX exports and 10 PNG exports.
- Production captures: archive `/`, detail `/apps/lattice-deck`, support `/pricing`, signed-out `/account`, at both widths.
- Final title sizes: archive 24/32 desktop, 20/24 mobile; experiment 96/92 desktop, 44/44 mobile.
- Final geometry: 48px desktop gutter, 20px mobile; 342px label plus 24px gap; pale blue literal `#CEE5FF`.
- Full-page browser captures can leave offscreen lazy covers unloaded. They establish layout, not image-loading performance.
- The capture script exports no cookies, local storage or grants. Signed-in account was source-reviewed only.
