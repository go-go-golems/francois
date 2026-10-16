import type { Analytics } from "./api";

const DAY = 86400000;
export type DateRange = { from: string; to: string };
export type Series = { label: string; values: number[]; other?: boolean };
export type TrafficMode = "total" | "pages" | "referrers";
export const isoDay = (date: Date) => date.toISOString().slice(0, 10);

function midnight(value: string) {
  const time = Date.parse(value + "T00:00:00Z");
  if (
    !/^\d{4}-\d{2}-\d{2}$/.test(value) ||
    !Number.isFinite(time) ||
    isoDay(new Date(time)) !== value
  )
    throw new Error("Choose valid start and end dates.");
  return time;
}
export function shiftDay(value: string, days: number) {
  return isoDay(new Date(midnight(value) + days * DAY));
}
export function inclusiveRange(start: string, end: string): DateRange {
  const from = midnight(start),
    to = midnight(end) + DAY;
  if (to <= from || to - from > 90 * DAY)
    throw new Error("Choose a range of 1 to 90 days.");
  return { from: start, to: isoDay(new Date(to)) };
}
export const presets = [
  "Today",
  "Yesterday",
  "Last 7 days",
  "Last 30 days",
  "This month",
] as const;
export function presetRange(
  preset: (typeof presets)[number],
  now = new Date(),
): DateRange {
  const today = isoDay(now),
    to = shiftDay(today, 1);
  switch (preset) {
    case "Today":
      return { from: today, to };
    case "Yesterday":
      return { from: shiftDay(today, -1), to: today };
    case "Last 7 days":
      return { from: shiftDay(today, -6), to };
    case "Last 30 days":
      return { from: shiftDay(today, -29), to };
    case "This month":
      return { from: today.slice(0, 8) + "01", to };
  }
}
export function rangeDays(range: DateRange) {
  const start = midnight(range.from.slice(0, 10)),
    end = midnight(range.to.slice(0, 10));
  if (end <= start || end - start > 90 * DAY)
    throw new Error("Invalid analytics range.");
  return Array.from({ length: (end - start) / DAY }, (_, i) =>
    isoDay(new Date(start + i * DAY)),
  );
}
export const shortDay = (value: string) =>
  new Date(value + "T00:00:00Z").toLocaleDateString("en-GB", {
    day: "2-digit",
    month: "short",
    timeZone: "UTC",
  });
export function rangeLabel(range: DateRange) {
  const end = shiftDay(range.to, -1);
  return `${shortDay(range.from)}${range.from.slice(0, 4) !== end.slice(0, 4) ? " " + range.from.slice(0, 4) : ""}–${shortDay(end)} ${end.slice(0, 4)}`;
}
export function trafficSeries(report: Analytics, mode: TrafficMode): Series[] {
  const days = rangeDays(report);
  if (mode === "total") {
    const values = new Map(
      report.groups.daily.map((row) => [row.key, row.views]),
    );
    return [
      { label: "Page views", values: days.map((day) => values.get(day) ?? 0) },
    ];
  }
  const groups = new Map<
    string,
    { key: string; other: boolean; total: number; days: Map<string, number> }
  >();
  for (const point of report.series[mode]) {
    const id = JSON.stringify([point.other, point.key]);
    const g = groups.get(id) ?? {
      key: point.key,
      other: point.other,
      total: 0,
      days: new Map(),
    };
    g.total += point.views;
    g.days.set(point.day, (g.days.get(point.day) ?? 0) + point.views);
    groups.set(id, g);
  }
  return [...groups.values()]
    .sort(
      (a, b) =>
        Number(a.other) - Number(b.other) ||
        b.total - a.total ||
        (a.key < b.key ? -1 : a.key > b.key ? 1 : 0),
    )
    .map((g) => ({
      label: g.other ? `Other ${mode}` : g.key || "Direct / unspecified",
      other: g.other,
      values: days.map((day) => g.days.get(day) ?? 0),
    }));
}
export function modelSeries(report: Analytics) {
  const days = rangeDays(report),
    rows = new Map(report.models.daily.map((row) => [row.key, row]));
  return {
    tokens: [
      {
        label: "Input tokens",
        values: days.map((day) => rows.get(day)?.inputTokens ?? 0),
      },
      {
        label: "Output tokens",
        values: days.map((day) => rows.get(day)?.outputTokens ?? 0),
      },
    ],
    cost: [
      {
        label: "Provider cost (USD)",
        values: days.map((day) => rows.get(day)?.cost ?? 0),
      },
    ],
  };
}
export function axisMax(max: number) {
  if (max <= 0) return 1;
  const power = 10 ** Math.floor(Math.log10(max)),
    unit = max / power;
  return ([1, 2, 4, 5, 8, 10].find((n) => n >= unit) ?? 10) * power;
}
