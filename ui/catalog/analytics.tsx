import React, { useState } from "react";
import {
  api,
  errorMessage,
  type Analytics,
  type ModelUsage,
  type TrafficGroup,
} from "./api";
import { AnalyticsChart } from "./analytics-charts";
import {
  modelSeries,
  presetRange,
  rangeDays,
  trafficSeries,
  type TrafficMode,
} from "./analytics-data";
import { DateRangeDialog } from "./date-range-dialog";

type Filters = { site: string; path: string; referrer?: string };
const labels: Record<string, string> = {
  pages: "Top pages",
  referrers: "Referrers",
  sources: "Sources",
  mediums: "Mediums",
  campaigns: "Campaigns",
  devices: "Devices",
  browsers: "Browsers",
  users: "Signed-in users",
  daily: "Daily views",
};
const count = (n: number) => n.toLocaleString();
const tokenCount = (n: number) =>
  Intl.NumberFormat("en", {
    notation: "compact",
    maximumFractionDigits: 2,
  }).format(n);

export function AnalyticsPage() {
  const [range, setRange] = useState(() => presetRange("Last 30 days"));
  const [tab, setTab] = useState("traffic");
  const [filters, setFilters] = useState<Filters>({ site: "", path: "" });
  const [mode, setMode] = useState<TrafficMode>("total");
  const [extra, setExtra] = useState("campaigns"),
    [modelGroup, setModelGroup] = useState("models");
  const query = new URLSearchParams({
    ...range,
    site: filters.site,
    path: filters.path,
  });
  if (filters.referrer !== undefined) query.set("referrer", filters.referrer);
  // currentData never presents a previous response under newly applied filter labels.
  const {
    currentData: data,
    error,
    isFetching,
  } = api.useAnalyticsQuery(query.toString());
  const days = data ? rangeDays(data) : [],
    models = data ? modelSeries(data) : undefined;
  const selectGroup = (group: string, key: string) => {
    if (group === "pages") setFilters({ ...filters, path: key });
    if (group === "referrers") setFilters({ ...filters, referrer: key });
  };
  return (
    <section className="stats analytics">
      <div className="stats-head">
        <div>
          <h1>{tab === "traffic" ? "Traffic." : "Model usage."}</h1>
          <p className="sub">
            {tab === "traffic"
              ? "Understand how people find and explore the boutique."
              : "Tokens, credits and provider cost across the catalog."}
          </p>
        </div>
      </div>
      <div className="analytics-toolbar">
        <div className="seg" aria-label="Analytics section">
          {[
            ["traffic", "Traffic"],
            ["models", "Model usage"],
          ].map(([key, label]) => (
            <button
              type="button"
              key={key}
              aria-pressed={tab === key}
              onClick={() => setTab(key)}
            >
              {label}
            </button>
          ))}
        </div>
        <DateRangeDialog value={range} onApply={setRange} />
      </div>
      {tab === "traffic" && (
        <TrafficFilters
          key={JSON.stringify(filters)}
          value={filters}
          report={data}
          onApply={setFilters}
        />
      )}
      {error && (
        <p role="alert" className="warn">
          {errorMessage(error)}
        </p>
      )}
      {isFetching && (
        <p role="status" className="muted">
          Loading analytics…
        </p>
      )}
      {data && tab === "traffic" && (
        <>
          <Summary
            items={[
              [count(data.traffic.views), "page views"],
              [count(data.traffic.visitors), "visitors"],
              [count(data.traffic.sessions), "sessions"],
              [count(data.traffic.users), "signed-in users"],
              [
                data.traffic.sessions
                  ? `${((100 * data.traffic.bounces) / data.traffic.sessions).toFixed(1)}%`
                  : "—",
                "single-page sessions",
              ],
            ]}
          />
          <AnalyticsChart
            title="Daily page views"
            days={days}
            series={trafficSeries(data, mode)}
            controls={
              <div className="seg small" aria-label="Traffic chart mode">
                {(
                  [
                    ["total", "Total"],
                    ["pages", "Stack by page"],
                    ["referrers", "Stack by referrer"],
                  ] as const
                ).map(([key, label]) => (
                  <button
                    type="button"
                    key={key}
                    aria-pressed={mode === key}
                    onClick={() => setMode(key)}
                  >
                    {label}
                  </button>
                ))}
              </div>
            }
          />
          <div className="analytics-breakdowns">
            {["pages", "referrers"].map((group) => (
              <TrafficTable
                key={group}
                title={labels[group]}
                rows={data.groups[group]}
                onSelect={(key) => selectGroup(group, key)}
              />
            ))}
          </div>
          <section>
            <div className="analytics-chart-head">
              <h2>{labels[extra]}</h2>
              <label className="small">
                Breakdown{" "}
                <select
                  aria-label="Traffic breakdown"
                  value={extra}
                  onChange={(e) => setExtra(e.target.value)}
                >
                  {Object.entries(labels)
                    .filter(([key]) => key !== "pages" && key !== "referrers")
                    .map(([key, label]) => (
                      <option key={key} value={key}>
                        {label}
                      </option>
                    ))}
                </select>
              </label>
            </div>
            <TrafficTable rows={data.groups[extra]} />
          </section>
          <p className="analytics-note small muted">
            Successful portal document loads; known bots and DNT/GPC requests
            excluded. Traffic is retained for 90 days. Single-page sessions are
            counted within the selected range. All traffic counts and graphs use
            the applied filters. Detail tables show up to 100 groups; stacks
            include every group through Other.
          </p>
        </>
      )}
      {data && models && tab === "models" && (
        <>
          <Summary
            items={[
              [
                count(data.models.daily.reduce((sum, r) => sum + r.calls, 0)),
                "model calls",
              ],
              [
                tokenCount(
                  data.models.daily.reduce((sum, r) => sum + r.inputTokens, 0),
                ),
                "input tokens",
              ],
              [
                tokenCount(
                  data.models.daily.reduce((sum, r) => sum + r.outputTokens, 0),
                ),
                "output tokens",
              ],
              [
                count(data.models.daily.reduce((sum, r) => sum + r.credits, 0)),
                "credits charged",
              ],
              [
                `$${data.models.daily.reduce((sum, r) => sum + r.cost, 0).toFixed(2)}`,
                "reported provider cost",
              ],
            ]}
          />
          <div className="analytics-model-charts">
            <AnalyticsChart
              title="Daily tokens"
              days={days}
              series={models.tokens}
              kind="paired"
              note="Cache usage is listed separately below."
            />
            <AnalyticsChart
              title="Daily provider cost"
              days={days}
              series={models.cost}
              kind="line"
              money
              note="USD · Provider-reported, including estimates."
            />
          </div>
          <section>
            <div className="analytics-chart-head">
              <h2>Usage by {modelGroup}</h2>
              <div className="seg small" aria-label="Model usage grouping">
                {["models", "users", "instances", "daily"].map((group) => (
                  <button
                    type="button"
                    key={group}
                    aria-pressed={modelGroup === group}
                    onClick={() => setModelGroup(group)}
                  >
                    {group}
                  </button>
                ))}
              </div>
            </div>
            <ModelTable rows={data.models[modelGroup]} />
          </section>
          <section>
            <h2>Usage by artifact</h2>
            <p className="small muted">
              All releases and instances of each artifact. Attribution is
              retained after instance deletion.
            </p>
            <ModelTable rows={data.models.artifacts} artifacts />
          </section>
          <p className="analytics-note small muted">
            {count(data.models.daily.reduce((sum, r) => sum + r.failed, 0))}{" "}
            failed calls. Includes demos across all catalog sites; traffic
            filters do not filter model usage. Provider cost may be zero or
            estimated. Cache tokens stay separate from input/output. Calls are
            grouped by creation date in UTC. Unrecoverable historical
            attribution appears under Unknown artifact. Tables show up to 100
            groups.
          </p>
        </>
      )}
    </section>
  );
}

function Summary({ items }: { items: [string, string][] }) {
  return (
    <div className="analytics-summary">
      {items.map(([value, label]) => (
        <div key={label}>
          <span>{value}</span>
          <span className="small muted">{label}</span>
        </div>
      ))}
    </div>
  );
}

function TrafficFilters({
  value,
  report,
  onApply,
}: {
  value: Filters;
  report?: Analytics;
  onApply: (f: Filters) => void;
}) {
  const [referrerMode, setReferrerMode] = useState(
    value.referrer === undefined
      ? "all"
      : value.referrer === ""
        ? "direct"
        : "hostname",
  );
  return (
    <form
      className="analytics-filters"
      onSubmit={(event) => {
        event.preventDefault();
        const form = new FormData(event.currentTarget);
        onApply({
          site: String(form.get("site") ?? "").trim(),
          path: String(form.get("path") ?? "").trim(),
          referrer:
            referrerMode === "all"
              ? undefined
              : referrerMode === "direct"
                ? ""
                : String(form.get("referrer") ?? "")
                    .trim()
                    .toLowerCase(),
        });
      }}
    >
      <label className="field small">
        Portal origin
        <input
          name="site"
          placeholder="All portals"
          defaultValue={value.site}
        />
      </label>
      <label className="field small">
        Page
        <input
          name="path"
          list="analytics-pages"
          placeholder="All pages"
          defaultValue={value.path}
        />
      </label>
      <datalist id="analytics-pages">
        {report?.groups.pages.map((row) => (
          <option key={row.key} value={row.key} />
        ))}
      </datalist>
      <label className="field small">
        Referrer
        <select
          aria-label="Referrer filter"
          value={referrerMode}
          onChange={(e) => setReferrerMode(e.target.value)}
        >
          <option value="all">All referrers</option>
          <option value="direct">Direct / unspecified</option>
          <option value="hostname">Hostname…</option>
        </select>
      </label>
      {referrerMode === "hostname" && (
        <label className="field small">
          Referring hostname
          <input
            name="referrer"
            required
            list="analytics-referrers"
            defaultValue={value.referrer ?? ""}
            placeholder="search.example"
          />
        </label>
      )}
      <datalist id="analytics-referrers">
        {report?.groups.referrers
          .filter((row) => row.key)
          .map((row) => (
            <option key={row.key} value={row.key} />
          ))}
      </datalist>
      <div className="actions">
        <button className="btn btn-soft" type="submit">
          Apply filters
        </button>
        <button
          type="button"
          className="tlink"
          onClick={() => onApply({ site: value.site, path: "" })}
        >
          Clear traffic filters
        </button>
      </div>
      {(value.path || value.referrer !== undefined || value.site) && (
        <p className="small muted analytics-active-filters">
          Applied: {value.site || "all portals"} · {value.path || "all pages"} ·{" "}
          {value.referrer === undefined
            ? "all referrers"
            : value.referrer || "direct / unspecified"}
        </p>
      )}
    </form>
  );
}

function TrafficTable({
  title,
  rows,
  onSelect,
}: {
  title?: string;
  rows: TrafficGroup[];
  onSelect?: (key: string) => void;
}) {
  return (
    <section>
      {title && <h2>{title}</h2>}
      <div className="analytics-table">
        <table>
          <thead>
            <tr>
              <th>Group</th>
              <th>Views</th>
              <th>Visitors</th>
            </tr>
          </thead>
          <tbody>
            {rows.map((row) => (
              <tr key={row.key}>
                <th scope="row">
                  {onSelect ? (
                    <button
                      className="tlink"
                      type="button"
                      onClick={() => onSelect(row.key)}
                    >
                      {row.key || "Direct / unspecified"}
                    </button>
                  ) : (
                    row.key || "Unspecified"
                  )}
                </th>
                <td>{count(row.views)}</td>
                <td>{count(row.visitors)}</td>
              </tr>
            ))}
          </tbody>
        </table>
        {rows.length === 0 && (
          <p className="small muted">No recorded traffic.</p>
        )}
      </div>
    </section>
  );
}
function ModelTable({
  rows,
  artifacts = false,
}: {
  rows: ModelUsage[];
  artifacts?: boolean;
}) {
  return (
    <div className="analytics-table">
      <table>
        <thead>
          <tr>
            <th>{artifacts ? "Artifact" : "Group"}</th>
            {[
              "Calls",
              "Failed",
              "Credits",
              "Input",
              "Output",
              "Cache read",
              "Cache write",
              "Cost (USD)",
            ].map((label) => (
              <th key={label}>{label}</th>
            ))}
          </tr>
        </thead>
        <tbody>
          {rows.map((row) => (
            <tr key={row.key}>
              <th scope="row">
                {row.label
                  ? `${row.label} · ${row.key}`
                  : row.key || (artifacts ? "Unknown artifact" : "Unspecified")}
              </th>
              <td>{count(row.calls)}</td>
              <td>{count(row.failed)}</td>
              <td>{count(row.credits)}</td>
              <td>{count(row.inputTokens)}</td>
              <td>{count(row.outputTokens)}</td>
              <td>{count(row.cacheReadTokens)}</td>
              <td>{count(row.cacheWriteTokens)}</td>
              <td>${row.cost.toFixed(4)}</td>
            </tr>
          ))}
        </tbody>
      </table>
      {rows.length === 0 && (
        <p className="small muted">No recorded model calls.</p>
      )}
    </div>
  );
}
