---
Title: Expose browsable catalog to instance JavaScript and agents
Ticket: FRANCOIS-015
Status: active
Topics:
    - backend
    - frontend
DocType: index
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-17T16:44:13.084500789-04:00
WhatFor: ""
WhenToUse: ""
---

# Expose browsable catalog to instance JavaScript and agents

The design exposes published editorial content and currently authorized downloadable source through a small slop_boutique API. HTML JavaScript and every hosted model call share one read service and the existing current-user authorization boundary. This ticket is researched and ready for implementation after FRANCOIS-013; runtime changes are not implemented.

- [Intern implementation guide](design-doc/01-recursive-catalog-api-intern-implementation-guide.md)
- [Investigation diary](reference/01-investigation-diary.md)
- [Primary sources](sources/README.md)
- [Tasks](tasks.md)

The guide specifies three read operations, current implementation anatomy, public versus entitled content rules, automatic agent tool integration, bounded text chunks, pseudocode, diagrams and a concrete test matrix. Raw SQL, drafts, account data and automatic publishing are outside the browsing contract.

The guide was uploaded successfully to `/ai/2026/10/17/FRANCOIS-015/FRANCOIS 015 Recursive Catalog Intern Guide.pdf`; see [the receipt](sources/remarkable-upload.txt). Cloud delivery is confirmed; physical device synchronization was not checked.

The illustrated delivery is `/ai/2026/10/17/FRANCOIS-015/FRANCOIS 015 Recursive Catalog Intern Guide Illustrated.pdf`; see [its successful receipt](sources/remarkable-upload-illustrated.txt). It includes both rendered diagrams. The later local guide adds the FRANCOIS-014 original-source clarification; that small cross-ticket note was not re-uploaded, following the instruction to stop further upload churn.
