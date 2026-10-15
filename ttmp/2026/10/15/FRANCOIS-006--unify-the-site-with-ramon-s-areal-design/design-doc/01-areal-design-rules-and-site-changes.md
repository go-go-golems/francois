---
Title: Areal design rules and site changes
Ticket: FRANCOIS-006
Status: active
Topics:
    - frontend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Ramon's Areal rules, how the storefront deviates, and the changes to make
LastUpdated: 2026-10-15T13:49:58.241465069-04:00
WhatFor: ""
WhenToUse: ""
---
# Areal design rules and site changes

Ramon's Areal screens (Paper, page "Sign-in (Areal)" of the unified file https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N) are the reference for the whole site. This document records the rules they follow, where the storefront differs, and the changes this ticket makes. The "before" snapshot is in `sources/design-snapshot-2026-10-15-before/`.

## The rules

| Element | Rule |
|---|---|
| Typeface | ABC Areal Regular/Medium, ABC Areal Mono for codes and versions (self-hosted, see `doc/production.md` §3.8) |
| Header | 18px "F" mark, `francois/<section>` crumb in ink 17/22; navigation is one muted line, items separated by " · "; the crumb, not the nav, shows where you are |
| Titles | 48/52 Medium, −0.02em, ending with a period; only the catalogue statement is 60/62 |
| Subtitle | 17/22 muted, 12px under the title |
| Actions | one black button (15/22, 9×16 padding) per screen; everything else is an underlined text link (ink, underline #C4C4C4, offset 4px) or a muted word ("Back") |
| Lists | rows of 68px; the first rule ink, the others #EEEEEE; 20/24 title over 14/20 muted line; trailing status 14/20 or "→" |
| Colour | ink #111, muted #737373, paper #FFF; green #2F6F4F only for a live state, copper #B5603F only for errors |
| Footer | one line: Privacy · Terms left, a muted item right |
| Layout | one left column (560px for forms), 40px gutters; no cards, grey buttons, shadows or decoration |

## Changes

Paper pages (storefront): nav as one muted line; page titles 48; subtitles 17; links ink; footer "Privacy · Terms … Set in ABC Areal by Dinamo".

Live site:

1. Header: navigation becomes one muted line, no underlines; the crumb stays.
2. Titles: split "Title — description" into a 48px title and a 17px muted subtitle (artifact, catalogue, creator, journal, pricing).
3. Actions: no grey buttons; one black primary, the rest underlined links.
4. Footer: Ramon's single line, with the Areal credit as the right-hand item.
5. `?login_error`: Ramon's RF2 (copper message under an ink rule, with a Sign in link).
6. `/account` signed out: Ramon's RF1 ("Sign in to see your account.").

Later: ZITADEL login branding (label policy first; a custom Login V2 for the exact layouts), remaining pages in Paper (creator, account, demo, instances, studio, design-system sheet).
