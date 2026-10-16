---
Title: Initial implementation validation evidence
Ticket: FRANCOIS-008
Status: active
Topics: [backend]
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Test, lint and delivery receipts from the initial implementation.
LastUpdated: 2026-10-16T12:00:00-04:00
WhatFor: Evidence retained for code and documentation review.
WhenToUse: Reviewing the first implementation checkpoint.
---

# Validation evidence from the initial implementation

## francois-008-final-tests.log

```text
?   	github.com/go-go-golems/francois	[no test files]
?   	github.com/go-go-golems/francois/cmd/francois	[no test files]
ok  	github.com/go-go-golems/francois/cmd/francois/cmds	(cached)
?   	github.com/go-go-golems/francois/pkg	[no test files]
ok  	github.com/go-go-golems/francois/pkg/catalog	3.164s
ok  	github.com/go-go-golems/francois/pkg/catalog/billing	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/client	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/content	(cached)
?   	github.com/go-go-golems/francois/pkg/catalog/hosted	[no test files]
?   	github.com/go-go-golems/francois/pkg/catalog/httpx	[no test files]
ok  	github.com/go-go-golems/francois/pkg/catalog/identity	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/mcpserver	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/payments	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/releases	(cached)
ok  	github.com/go-go-golems/francois/pkg/catalog/settings	(cached)
?   	github.com/go-go-golems/francois/pkg/catalog/store	[no test files]
?   	github.com/go-go-golems/francois/pkg/catalog/web	[no test files]
?   	github.com/go-go-golems/francois/pkg/doc	[no test files]
ok  	github.com/go-go-golems/francois/pkg/host	(cached)
ok  	github.com/go-go-golems/francois/pkg/piworker	(cached)
ok  	github.com/go-go-golems/francois/pkg/store	(cached)
?   	github.com/go-go-golems/francois/pkg/web	[no test files]
?   	github.com/go-go-golems/francois/ttmp/2026/10/14/FRANCOIS-004--showcase-storefront-creators-artifact-pages-demos-subscription-plans-and-an-artifact-editor/scripts/03-parse-settings	[no test files]
```

## francois-008-focused.log

```text
ok  	github.com/go-go-golems/francois/pkg/catalog	0.392s
```

## francois-008-lint-final.log

```text
0 issues.
```

## remarkable-upload.log

```text
Cached tree found; checking for remote changes (a full resync may still be needed).
Synchronizing cloud tree… 0s | 0 HTTP responses, 0 active requests
Cloud tree synchronized. 3s | 8 HTTP responses
OK: uploaded FRANCOIS-008 RSS and Analytics Intern Guide.pdf -> /ai/2026/10/16/FRANCOIS-008
```
