---
Title: Analytics Paper designs and implementation mapping
Ticket: FRANCOIS-008
Status: active
Topics:
    - backend
DocType: design-doc
Intent: long-term
Owners: []
RelatedFiles:
    - Path: repo://pkg/catalog/hosted/sample.go
      Note: Trusted artifact ID source for call reservation
    - Path: repo://pkg/catalog/store/analytics.go
      Note: Filtered report and daily stack implementation target
    - Path: repo://pkg/catalog/store/instances.go
      Note: Physical deletion and same-artifact upgrade evidence
    - Path: repo://pkg/catalog/store/usage.go
      Note: Durable artifact attribution during model admission
    - Path: repo://ui/catalog/admin.tsx
      Note: Graphs filters and shared date dialog integration target
ExternalSources: []
Summary: ""
LastUpdated: 2026-10-16T11:46:56.875255014-04:00
WhatFor: ""
WhenToUse: ""
---

# Analytics Paper designs and implementation mapping

## Design references

The new Analytics group is separated by dividers in the existing unified design file, after Studio and before Archive. It contains:

- [Analytics — Traffic](https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-A-0): traffic overview plus the date-range dialog state.
- [Analytics — Model usage](https://app.paper.design/file/01M506DAH3CXP5QS28T2VQ4W3N/p-B-0): token/cost graphs and detailed usage table.

The user required Paper-first design. These artboards were created and reviewed before the follow-up graph/filter implementation. Their data is explicitly labeled SAMPLE DATA. The existing application commit `0a66114` has basic analytics tables; the new graphs/filters/dialog are the next implementation milestone.

## Visual system

Use the existing ABC Areal/Areal Mono typography, white paper, black ink, neutral muted labels, fine rules, and square controls. Avoid shadows, gradients, decorative chart cards, or a new accent palette. Main figures use 40px type, page titles 48px/52px, graph headings 24px/30px, axis labels 14px. Graph marks are black/gray with a light outlined Other series. All styling is available as exported source in `sources/paper/`.

The desktop artboards are 1440px wide with 40px page padding and content-fit heights. Charts sit immediately after metric tiles and before detail tables. The model chart row uses a wider input/output plot and a narrower cost plot. On narrower screens these should stack vertically while preserving readable axis labels.

## Graph contracts

### Daily page views

A bar chart uses UTC day buckets and zero-based vertical scale. A switch selects Total, Stack by page, or Stack by referrer. Stack the top three categories for the selected date/filter range, then aggregate every remaining category into Other. Every daily stack must equal that day's filtered view count. Do not draw a chart from a top-100 table truncated independently per day.

The visible page stack has four neutral styles: ink, muted, underline gray, and outlined tint for Other. The legend names the categories and their range totals. The underlying data table remains available so categories and values do not depend solely on visual shade. Empty referrers mean Direct/unspecified and must remain distinguishable from the Other bucket.

### Daily tokens and provider cost

Input and output use paired bars, not a single summed/stacked value. Cache counts stay separate in the detailed model table because provider definitions may overlap. Provider cost uses a plain line on the same UTC date axis, with explicit currency/provider-estimate wording. These reports stay catalog-wide even when traffic is filtered by site, path, or referring hostname.

The existing API already returns daily traffic/model groups; chronological sorting and date zero filling are client responsibilities. Page/referrer stacks require a dedicated daily categorical aggregation in the server response.

## Filters and dates

Reuse the catalog's underlined choices, admin selection-button language, and Studio picker's clear header/Close pattern. There was no reusable date-range dialog in the current code or Paper pages. Create a small shared dialog for both analytics sections using native date fields rather than a custom calendar package.

- Page and referrer selectors narrow all traffic counts, daily series, and breakdown tables.
- Allow a clear reset and visibly show active filters. Referrers include a distinct Direct/unspecified choice.
- Date button opens one dialog with Today, Yesterday, Last 7 days, Last 30 days, This month, and Custom range choices.
- Start and end labels use inclusive human dates. Applying the range converts the included end day to the next UTC midnight for the existing exclusive API boundary.
- Apply validates a positive range of at most 90 days. Cancel, Close, and Escape discard edits. Native modal focus handling should return focus to the trigger.
- Updating dates should be deliberate: draft dialog values do not send incomplete requests while the user edits.

## Implementation ownership

`pkg/catalog/store/analytics.go` owns filtered aggregates and complete daily stacks; `pkg/catalog/analytics.go` validates query fields and dates; `ui/catalog/api.ts` defines the report contract. The UI should keep chart preparation separate from rendering and reuse the existing RTK Query transport and styling variables.

The first guide remains the system-orientation source. This document owns the additional Paper-first interaction/graph decisions and should be updated with implementation/validation evidence when that milestone completes.

## Review evidence

Screenshot reviews covered spacing, typography, contrast, fixed-width data columns, chart axes, repetition, and artboard fit. A missing SVG hatch pattern was corrected to an outlined light bucket. Artboards were fitted to content and working indicators released. JSON snapshots contain clean Paper JSX without node IDs; these are design sources, not executable application modules.

## Implementation design: filtered reports and complete stacks

This section is the implementation contract for the requested follow-up. It supersedes the baseline guide's table-only presentation and its lack of traffic filters. The first guide remains the introduction to hosting, identity, RSS, and billing; this document explains the additional application work precisely enough for a new engineer to implement it without inventing aggregation or interaction semantics.

### One filtered population, one report snapshot

A report starts with a half-open UTC time range and optional exact-match traffic dimensions. The same filtered population must feed traffic totals, bounce counts, dimension tables, and both daily categorical series. Filtering only the SVG or a top-pages list would produce plausible but contradictory results. Apply every filter in PostgreSQL before counting anything.

Define the store request explicitly; change existing call sites rather than retaining a positional overload or adding an adapter:

```go
type TrafficFilter struct {
    Site     string  // empty means all configured portals
    Path     string  // empty means all paths
    Referrer *string // nil means all; pointer to "" means direct/unspecified
}

Analytics(ctx, from, to time.Time, filter TrafficFilter) (Analytics, error)
```

Do not overload an empty referrer to mean both all traffic and direct traffic. HTTP query presence carries this distinction: an absent `referrer` means all referrers; `referrer=` means only the stored empty hostname. A hostname value means an exact match, not a substring search or a URL. The page field is an absolute path such as `/apps/polis`, without query or fragment.

Retain the existing administrator/`admin:read` guard, maximum 90-day range, private JSON responses, and five-second total query budget. Validate path/hostname string size before SQL. A report should use a read-only repeatable-read transaction, so totals, ranking, and daily slices describe one database snapshot even when another visitor arrives during the query sequence. The new transaction owns all analytical reads; it never changes credit settlement.

```sql
-- Bind all five values. Do not interpolate caller strings.
WHERE created_at >= $1 AND created_at < $2
  AND ($3 = '' OR site = $3)
  AND ($4 = '' OR path = $4)
  AND ($5::text IS NULL OR referrer = $5)
```

PostgreSQL stores only referrer hostnames; no query value can ask the server to recover a discarded full URL. Traffic site/page/referrer filters do not filter model rows, because those rows have no authoritative portal/referrer history. The UI must keep that scope visible.

### Daily categorical series

Keep the existing top-100 dimension tables for browsing. Add a separate `series` response member so the chart does not infer history from those truncated tables. Rank categories once across the entire filtered range, not separately for each day. The top three keys remain in a fixed order/colors for that response; aggregate all other keys into an explicit Other series. This caps a chart at four stacks and its payload at 90 days times four categories per dimension.

```go
type TrafficPoint struct {
    Day   string `json:"day"`   // YYYY-MM-DD, UTC
    Key   string `json:"key"`   // raw path or hostname
    Other bool   `json:"other"` // distinguish remainder from a real empty key
    Views int64  `json:"views"`
}
// Analytics.Series: map[string][]TrafficPoint
// Exact map keys: "pages", "referrers".
```

Use a fixed server expression (`path` or `referrer`) selected by code. Never accept a SQL column/expression from the browser. An empty referrer can be a real top category; the boolean Other marker keeps it separate from the remaining-category bucket. The path cannot be inferred from the hostname and vice versa; compute the two distributions independently.

```sql
WITH filtered AS (
  SELECT created_at, path, referrer
  FROM traffic_views
  WHERE /* same parameterized population predicate */
), top_keys AS (
  SELECT path AS key
  FROM filtered
  GROUP BY path
  ORDER BY count(*) DESC, path ASC
  LIMIT 3
)
SELECT to_char(f.created_at AT TIME ZONE 'UTC', 'YYYY-MM-DD') AS day,
       COALESCE(k.key, '') AS key,
       k.key IS NULL AS other,
       count(*) AS views
FROM filtered f
LEFT JOIN top_keys k ON k.key = f.path
GROUP BY 1, 2, 3
ORDER BY 1, 3, 2;
```

The referrer query substitutes only the fixed identifier. Both queries contain every filtered event, including keys beyond the table limit. The sum of each day's stacks must equal its `groups.daily` count; the sum of all days must equal `traffic.views` in the same snapshot. When fewer than four categories exist, do not invent a nonempty Other series. The browser can omit a zero-sized remainder.

The API also echoes `path` and the nullable `referrer` next to the existing site/from/to metadata. Example:

```http
GET /api/admin/analytics?from=2026-10-01&to=2026-10-17&path=%2Fapps%2Fpolis&referrer=
```

This requests direct visits to `/apps/polis` in the included 1–16 October range. Model results remain global for the date interval. A successful response adds fields like these to the baseline contract:

```json
{
  "path": "/apps/polis",
  "referrer": "",
  "series": {
    "pages": [{"day":"2026-10-01","key":"/apps/polis","other":false,"views":8}],
    "referrers": [{"day":"2026-10-01","key":"","other":false,"views":8}]
  }
}
```

### Chart preparation and rendering

Keep data preparation in a small TypeScript module that can be tested without a browser. The response's daily tables are ordered by volume, so iterate calendar days from `from` through the day before `to`, look up each ISO date, and insert zero for missing days. Do not parse display labels, use local-midnight arithmetic, or sort days by count. UTC day arithmetic should remain correct across daylight-saving changes and month/year boundaries.

```text
prepare(report, traffic mode):
  days = all UTC dates in [report.from, report.to)
  if mode == total:
    values = daily view counts for each day, default zero
  otherwise:
    points = report.series[pages or referrers]
    collect distinct identities using (other, key)
    rank explicit keys by range total; place Other last
    for each day, emit each category value or zero
  assert stacked totals match the daily totals in tests

prepare model graphs:
  map report.models.daily by ISO day
  input/day  = row.inputTokens or zero
  output/day = row.outputTokens or zero
  cost/day   = row.cost or zero
  never add cache categories to input/output to create a false total
```

Render SVG through React using the exported Paper styles, mapped to the repository's existing CSS variables. Daily traffic uses zero-based bars; input/output use adjacent bars with distinct shades; cost uses a plain line and no filled area. Keep the axis region 220px high, fine gray horizontal rules, 14px mono labels, and no ornamental card borders/shadows. Observe container width so labels remain readable rather than scaling a 1360px SVG down to phone width. Reduce the number of date ticks at narrow widths. Put the two model charts into one flexible row that stacks on mobile.

Use `<title>` descriptions and per-mark exact-value titles; keep exact tabular data available. Empty/all-zero data should show an honest empty state or zero axis, never NaN/Infinity or a fabricated trend. For a one-day line, render a visible point instead of an invisible one-point path. A stack with many tiny categories still includes Other so its height stays correct.

### Filter interactions

Page/referrer selection updates the query arguments, not a client-only subset. Reuse the current RTK Query endpoint. A clear action resets the two traffic filters while preserving dates and portal. Empty selection means all; a separate Direct/unspecified option sends an explicitly present empty referrer. Existing table rows can act as convenient shortcuts to the same exact filters. Values displayed in a dropdown or table must remain text, never HTML.

The chart's Total/Page/Referrer mode is local presentation state and does not need another request because both bounded series arrive in one response. The table/group selector should remain independent from graph mode. Changing the traffic filter must not imply that model totals are attributed to those pages or referring sites.

## Implementation design: reusable date-range dialog

The dialog is shared by the Traffic and Model usage sections. It should be a small React component using the browser's native `<dialog>` behavior and `input type="date"` fields. Reuse the product's `tlink`, button, field, and selection-control classes where they fit; add only scoped analytics layout styles. Do not wrap or adapt Studio MediaPicker into a generic dialog framework: its upload/media behavior is unrelated. Reuse its visual pattern and clear Close action.

Keep **draft** dates inside the open dialog and **applied** dates in the report page. Opening copies the current range into draft state. Presets change draft state; only Apply commits a new query. Cancel, Close, and Escape discard draft edits, and the next open restores the applied values. Use native modal focus containment and restore focus to the trigger. Inputs need labels, and an invalid-range message must be exposed with an alert role or associated description. The dialog must fit a narrow viewport and let its content scroll if necessary.

Public UI dates are inclusive. The transport still uses an exclusive `to`; conversion happens once at the boundary:

```text
on open:
  draftStart = applied.from
  draftEnd   = UTC date(applied.to minus one calendar day)

on apply:
  require valid YYYY-MM-DD values
  start = UTC midnight(draftStart)
  endExclusive = UTC midnight(draftEnd plus one calendar day)
  require 0 < endExclusive - start <= 90 UTC days
  apply {from: draftStart, to: ISO-date(endExclusive)}
  close dialog
```

Define quick ranges against today's **UTC date**, not the viewer's local calendar. Today is `[today, tomorrow)`; Yesterday is `[yesterday, today)`; Last 7 days includes today and six preceding days; Last 30 days includes today and 29 preceding days; This month starts on the first UTC date of this month and ends tomorrow. Custom range exposes the existing draft fields without changing them. The date button displays the included end date so its wording matches the dialog. API validation remains authoritative even though the UI validates first.

Do not issue incomplete requests when one field is temporarily empty. RTK Query should retain a clear loading state during transitions and show request errors without presenting stale filter labels as though they describe an old response.

## Implementation design: model usage per artifact

An artifact is the catalog application; an instance is one workspace using a release of it. Operators need both views: an expensive artifact may have many cheap instances, while one expensive instance may belong to an otherwise quiet artifact. The new model report adds `models.artifacts` using the same usage metrics as models/users/instances. Paper now includes a separate Usage by artifact table below Usage by model, with sample Polis, Tessera, and Unknown artifact rows.

### Evidence and deletion boundary

`pkg/catalog/store/instances.go` obtains artifact identity by joining `instances.release_id` to `releases.artifact_id`. `UpgradeInstance` allows a new published release only within the same artifact. However, `DeleteInstance` executes a physical `DELETE`, and expired demos can also be physically removed in `store/demos.go`. The model ledger intentionally keeps its textual `instance_id` without a cascading foreign key. Therefore an inner join from historical model calls through the current instance table would silently drop usage after instance deletion.

Capture the immutable artifact ID at call admission. Add a **new migration 010**, rather than editing already committed migration 009:

```sql
ALTER TABLE model_calls ADD COLUMN artifact_id text NOT NULL DEFAULT '';
UPDATE model_calls c
SET artifact_id = r.artifact_id
FROM instances i JOIN releases r ON r.id = i.release_id
WHERE i.id = c.instance_id;
```

The default empty string means Unknown artifact. Backfill only when the surviving instance/release relationship proves the identity. Calls whose instances were already deleted cannot be reconstructed from existing data; retain them explicitly in Unknown artifact. Do not guess from a mutable instance title, a URL path, a user's current catalog choices, or model names.

No foreign key should cascade or prevent ledger retention. `artifact_id` is a historical identifier copied from server-authorized instance metadata, not a browser input. Add `ArtifactID` to `store.ReserveRequest`; `hosted.sample` passes `inst.ArtifactID`; the same INSERT that creates `model_calls` persists it. Credit reservation/start/settlement logic stays unchanged.

```text
hosted sample request:
  load authorized instance (already includes artifactId)
  Reserve({userId, instanceId, artifactId: instance.artifactId, ...})
    atomically reserve credits
    insert model call with artifactId snapshot
  execute provider and settle existing usage fields

artifact aggregation:
  group model_calls by stored artifact_id
  sum the same calls/failures/credits/token/cache/cost fields
  preserve empty artifact_id as a visible Unknown artifact row
```

The report key is the stable artifact ID. A display name can be resolved with a left lookup to current catalog metadata, falling back to the ID if metadata is missing. Do not use title as the grouping key because renames must not split historical usage. Aggregate first and resolve titles afterward or with a one-to-one left join, never a multiplying join through artifact creators or instance members.

For an untruncated fixture, artifact totals must reconcile with other model groupings and daily totals. Existing top-100 table limits still apply, so the UI must not sum a truncated artifact table and label it the grand total. New calls preserve attribution after deletion, release upgrades, and model changes. Artifact classification does not introduce portal/campaign attribution.

## Phased implementation and acceptance tests

1. Commit the Paper-first design, full implementation contract, and diary checkpoint independently from new runtime changes.
2. Add migration 010 and capture `ArtifactID` in `ReserveRequest` from trusted runtime metadata. Test deletion survival and unknown historical rows before modifying the chart UI.
3. Change the store report interface to `TrafficFilter`; add exact predicates, a read-only report snapshot, bounded complete daily stacks, and artifact groups. Update every existing call site directly.
4. Extend the guarded HTTP handler and TypeScript contract. Preserve explicit empty-referrer semantics. Keep model scope clear and reject malformed/oversized filter values.
5. Add chart data preparation, minimal SVG rendering, mode/filter controls, and the shared date dialog. Rebuild the embedded portal bundle.
6. Validate database behavior and focused chart/date contracts, run TypeScript/frontend checks, Go tests/build and relevant lint, then record a code checkpoint in the diary.

Required regression cases:

- Exact page and referrer filters affect totals, every dimension table, and daily stacks consistently; combined filters use AND semantics.
- Absent referrer selects all, present empty selects direct; a real hostname is not merged with Other.
- More than 100 page/referrer keys still reconcile in the four-series graph. Top keys are selected globally with deterministic tie-breaking, not per day.
- Each daily stack sums to its daily total; both stack dimensions sum to the same filtered overall total.
- Zero-traffic, one-day, unsorted-day, missing-day, and range-boundary reports produce valid graphs without numeric errors.
- UTC date arithmetic crosses daylight-saving and month/year boundaries; inclusive end conversion and all presets match the API's exclusive boundary.
- Dialog edits/cancel/Escape do not send a new report request; Apply validates and commits one range. Keyboard focus remains usable.
- Model snapshots capture artifact IDs from the authorized instance; deleting that instance does not erase artifact usage. Backfill preserves provable historical IDs and exposes irrecoverable rows as Unknown artifact.
- Artifact/model/user/instance aggregations use the same charged-state predicate and keep cache counts distinct.
- Anonymous/subscriber callers remain denied; filtering does not expand API-token scope or alter billing state.

## Completion boundary

At the design checkpoint these sections are the implementation plan, not a claim that the follow-up runtime code is already present. The existing RSS/basic analytics feature is committed as `0a66114`. The diary and changelog will name the later implementation commit and validation evidence once the graph/filter/date/artifact work is complete. The Paper references and clean source exports are the visual source of truth for that work.
