---
Title: Analytics graph and filter browser validation
Ticket: FRANCOIS-008
Status: active
Topics: [backend]
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Chromium checks against real built assets and synthetic API data.
LastUpdated: 2026-10-16T12:20:00-04:00
WhatFor: Reproduce interaction and responsive-layout checks.
WhenToUse: Reviewing the completed analytics frontend.
---

# Chromium validation

The isolated preview serves the real embedded JavaScript and CSS with synthetic session/report data. PostgreSQL aggregation and migration behavior were tested independently in the Go integration suite; this fixture does not prove production connectivity or authentication.

Reproduction scripts: `scripts/01-analytics-preview.mjs` (run in tmux on localhost:18908) and `scripts/02-analytics-browser-check.js` (pass to Playwright browser_run_code).

### Result
{"checks":["Switching stack mode does not query the server","Every stacked segment has the same visible left and right edges","Stack includes Other pages","Direct and Other referrers remain distinct","Inverted date range is rejected inline","Editing and invalid Apply do not query","Cancel restores date-trigger focus","Escape closes the dialog without querying","Date presets remain drafts until Apply","Last seven days applies correct exclusive UTC boundary","Direct filter sends explicit empty referrer and exact path","Clear resets applied path and referring hostname","Model usage has token and cost graphs","Unattributed historical usage remains visible","Model graphs and tables fit mobile without page overflow","Date dialog fits mobile viewport","Traffic controls and stacks fit mobile","No browser page errors"],"requests":["?from=2026-09-17&to=2026-10-17&site=&path=","?from=2026-10-10&to=2026-10-17&site=&path=","?from=2026-10-10&to=2026-10-17&site=&path=%2Fapps&referrer="],"errors":[]}


## Visual review

- [Traffic desktop](screenshots/01-traffic-desktop.png): aligned stacked bars, readable dates, no clipped sections; top Other outline is inset.
- [Models desktop](screenshots/02-models-desktop.png): paired token bars and cost line match the monochrome design, compact summaries, exact artifact figures in the table.
- [Date dialog mobile](screenshots/03-date-mobile.png): native date controls, presets and actions fit 390px viewport; no page overflow.

The screenshot fixture uses fallback system fonts because licensed Areal web fonts are not included in the repository. The application uses the existing Areal font configuration when deployed.

