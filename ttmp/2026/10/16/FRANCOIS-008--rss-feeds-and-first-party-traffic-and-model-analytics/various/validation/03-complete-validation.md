---
Title: Completed analytics implementation validation
Ticket: FRANCOIS-008
Status: active
Topics: [backend]
DocType: reference
Intent: long-term
Owners: []
RelatedFiles: []
ExternalSources: []
Summary: Full Go PostgreSQL suite and lint receipts for the completed implementation.
LastUpdated: 2026-10-16T12:25:00-04:00
WhatFor: Review final code validation alongside the browser checks.
WhenToUse: Reviewing commits 2f53554 and f072af7.
---

# Final validation

The full Go suite used an explicit local PostgreSQL URL and isolated per-test databases. The catalog runtime test also asserts that the actual hosted sample path captures the artifact ID. Frontend check/build and eight tests passed on Node v24.18.0. All 18 browser checks passed after the visible stack-edge correction; browser setup uses synthetic API fixtures, with SQL validated separately.

## francois-008-complete-tests.log

```text
?   	github.com/go-go-golems/francois	[no test files]
?   	github.com/go-go-golems/francois/cmd/francois	[no test files]
ok  	github.com/go-go-golems/francois/cmd/francois/cmds	0.084s
?   	github.com/go-go-golems/francois/pkg	[no test files]
ok  	github.com/go-go-golems/francois/pkg/catalog	4.317s
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

## francois-008-complete-lint.log

```text
0 issues.
```
