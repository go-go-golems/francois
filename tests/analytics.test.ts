import { test } from "node:test";
import assert from "node:assert/strict";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import type { Analytics } from "../ui/catalog/api";
import { AnalyticsChart } from "../ui/catalog/analytics-charts";
import {
  inclusiveRange,
  presetRange,
  rangeDays,
  rangeLabel,
  trafficSeries,
  modelSeries,
} from "../ui/catalog/analytics-data";

function report(): Analytics {
  return {
    from: "2026-10-01T00:00:00Z",
    to: "2026-10-04T00:00:00Z",
    site: "",
    path: "",
    referrer: null,
    traffic: { views: 9, visitors: 2, sessions: 2, users: 0, bounces: 1 },
    groups: {
      daily: [
        { key: "2026-10-03", views: 6, visitors: 2 },
        { key: "2026-10-01", views: 3, visitors: 1 },
      ],
    },
    series: {
      pages: [],
      referrers: [
        { day: "2026-10-01", key: "", other: false, views: 2 },
        { day: "2026-10-01", key: "", other: true, views: 1 },
        { day: "2026-10-03", key: "", other: true, views: 2 },
        { day: "2026-10-03", key: "search.example", other: false, views: 4 },
      ],
    },
    models: { daily: [] },
  };
}

test("date presets and inclusive boundaries use UTC across DST, leap years and year ends", () => {
  assert.deepEqual(inclusiveRange("2026-12-31", "2026-12-31"), {
    from: "2026-12-31",
    to: "2027-01-01",
  });
  assert.equal(
    rangeLabel({ from: "2026-12-31", to: "2027-01-01" }),
    "31 Dec–31 Dec 2026",
  );
  assert.match(
    rangeLabel({ from: "2026-12-31", to: "2027-01-03" }),
    /2026.*2027/,
  );
  assert.equal(rangeDays({ from: "2026-10-31", to: "2026-11-03" }).length, 3);
  assert.equal(rangeDays(inclusiveRange("2028-02-28", "2028-03-01")).length, 3);
  const now = new Date("2026-11-01T23:30:00-05:00"); // November 2 in UTC.
  assert.deepEqual(presetRange("Today", now), {
    from: "2026-11-02",
    to: "2026-11-03",
  });
  assert.deepEqual(presetRange("Yesterday", now), {
    from: "2026-11-01",
    to: "2026-11-02",
  });
  assert.equal(rangeDays(presetRange("Last 7 days", now)).length, 7);
  assert.equal(rangeDays(presetRange("Last 30 days", now)).length, 30);
  assert.deepEqual(presetRange("This month", now), {
    from: "2026-11-01",
    to: "2026-11-03",
  });
  assert.equal(
    rangeDays(inclusiveRange("2026-01-01", "2026-03-31")).length,
    90,
  );
  for (const [start, end] of [
    ["2026-02-30", "2026-03-01"],
    ["", "2026-01-01"],
    ["2026-01-02", "2026-01-01"],
    ["2026-01-01", "2026-04-01"],
  ])
    assert.throws(() => inclusiveRange(start, end));
});

test("daily charts sort by calendar day, zero-fill gaps and preserve direct versus Other", () => {
  const data = report();
  assert.deepEqual(trafficSeries(data, "total")[0].values, [3, 0, 6]);
  const stacks = trafficSeries(data, "referrers");
  assert.equal(stacks.length, 3);
  assert.equal(stacks.at(-1)?.label, "Other referrers");
  assert.deepEqual(
    stacks.find((s) => s.label === "Direct / unspecified")?.values,
    [2, 0, 0],
  );
  assert.deepEqual(
    [0, 1, 2].map((i) => stacks.reduce((sum, s) => sum + s.values[i], 0)),
    [3, 0, 6],
  );
  data.models.daily = [
    {
      key: "2026-10-03",
      calls: 1,
      failed: 0,
      credits: 2,
      inputTokens: 10,
      outputTokens: 3,
      cacheReadTokens: 1000,
      cacheWriteTokens: 500,
      cost: 0.01,
    },
  ];
  const models = modelSeries(data);
  assert.deepEqual(
    models.tokens.map((s) => s.values),
    [
      [0, 0, 10],
      [0, 0, 3],
    ],
  );
  assert.deepEqual(models.cost[0].values, [0, 0, 0.01]);
});

test("SVG graphs expose exact daily values and render zero and one-day data without invalid coordinates", () => {
  const empty = renderToStaticMarkup(
    React.createElement(AnalyticsChart, {
      title: "Traffic",
      days: ["2026-10-01"],
      series: [{ label: "Views", values: [0] }],
    }),
  );
  assert.doesNotMatch(empty, /NaN|Infinity/);
  assert.match(empty, /No recorded activity/);
  assert.match(empty, /View daily data/);
  const cost = renderToStaticMarkup(
    React.createElement(AnalyticsChart, {
      title: "Cost",
      days: ["2026-10-01"],
      series: [{ label: "Cost", values: [0.0123] }],
      kind: "line",
      money: true,
    }),
  );
  assert.match(cost, /<circle/);
  assert.match(cost, /\$0\.0123/);
  assert.doesNotMatch(cost, /NaN|Infinity/);
});
