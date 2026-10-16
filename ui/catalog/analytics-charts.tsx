import React, { useEffect, useRef, useState } from "react";
import { axisMax, shortDay, type Series } from "./analytics-data";

const fills = [
  "var(--color-ink)",
  "var(--color-muted)",
  "var(--color-faint)",
  "var(--color-plinth)",
];
const compact = (n: number) =>
  Intl.NumberFormat("en", {
    notation: "compact",
    maximumFractionDigits: 1,
  }).format(n);

// Native SVG keeps these small, monochrome plots independent of a chart library.
// Width is measured so 14px labels stay readable on narrow screens.
export function AnalyticsChart({
  title,
  days,
  series,
  kind = "stack",
  money = false,
  note,
  controls,
}: {
  title: string;
  days: string[];
  series: Series[];
  kind?: "stack" | "paired" | "line";
  money?: boolean;
  note?: string;
  controls?: React.ReactNode;
}) {
  const ref = useRef<HTMLDivElement>(null);
  const [width, setWidth] = useState(680);
  useEffect(() => {
    const observer = new ResizeObserver(([entry]) =>
      setWidth(Math.max(280, Math.floor(entry.contentRect.width))),
    );
    if (ref.current) observer.observe(ref.current);
    return () => observer.disconnect();
  }, []);
  const values = days.map((_, i) =>
    kind === "stack"
      ? series.reduce((sum, s) => sum + s.values[i], 0)
      : Math.max(0, ...series.map((s) => s.values[i])),
  );
  const maximum = axisMax(Math.max(0, ...values));
  const left = 56,
    plotWidth = width - left - 12,
    slot = plotWidth / Math.max(1, days.length);
  const y = (n: number) => 176 - (n / maximum) * 160;
  const tickCount = Math.min(days.length, Math.max(2, Math.floor(width / 110)));
  const ticks = [
    ...new Set(
      Array.from({ length: tickCount }, (_, i) =>
        Math.round((i * (days.length - 1)) / Math.max(1, tickCount - 1)),
      ),
    ),
  ];
  const number = (n: number) =>
    money
      ? n.toLocaleString("en-US", {
          style: "currency",
          currency: "USD",
          maximumFractionDigits: Math.min(
            10,
            Math.max(2, 1 - Math.floor(Math.log10(maximum))),
          ),
        })
      : compact(n);
  const swatch = (s: Series, i: number) => ({
    background: s.other ? fills[3] : fills[i],
    border: s.other ? "1px solid var(--color-muted)" : undefined,
  });
  return (
    <section className="analytics-chart" ref={ref}>
      <div className="analytics-chart-head">
        <h2>{title}</h2>
        {controls}
      </div>
      <svg
        width="100%"
        height="220"
        viewBox={`0 0 ${width} 220`}
        role="img"
        aria-label={title}
      >
        <title>{`${title}. ${days[0]} to ${days.at(-1)}. Exact daily values follow in the data table.`}</title>
        {[0, maximum / 2, maximum].map((n) => (
          <g key={n}>
            <line
              x1={left}
              x2={width}
              y1={y(n)}
              y2={y(n)}
              className="analytics-rule"
            />
            <text x="0" y={y(n) + 5}>
              {number(n)}
            </text>
          </g>
        ))}
        {kind === "line"
          ? series.map((s, k) => (
              <g key={s.label}>
                <path
                  d={s.values
                    .map(
                      (n, i) =>
                        `${i ? "L" : "M"}${left + (i + 0.5) * slot},${y(n)}`,
                    )
                    .join(" ")}
                  fill="none"
                  stroke={fills[k]}
                  strokeWidth="2"
                />
                {s.values.map((n, i) => (
                  <circle
                    key={i}
                    cx={left + (i + 0.5) * slot}
                    cy={y(n)}
                    r={days.length === 1 ? 3 : 2}
                    fill={fills[k]}
                  >
                    <title>{`${days[i]} · ${s.label}: ${money ? `$${n.toFixed(4)}` : n.toLocaleString()}`}</title>
                  </circle>
                ))}
              </g>
            ))
          : days.map((day, i) => {
              let bottom = 0;
              return (
                <g key={day}>
                  {series.map((s, k) => {
                    const n = s.values[i],
                      base = kind === "stack" ? bottom : 0;
                    bottom += n;
                    const barWidth =
                      kind === "stack"
                        ? slot * 0.56
                        : (slot * 0.7) / Math.max(1, series.length);
                    const x =
                      left +
                      i * slot +
                      slot * (kind === "stack" ? 0.22 : 0.15) +
                      (kind === "paired" ? k * barWidth : 0);
                    return (
                      <rect
                        key={s.label}
                        x={x + (s.other ? 0.25 : 0)}
                        y={y(base + n) + (s.other ? 0.25 : 0)}
                        width={Math.max(0.1, barWidth - (s.other ? 0.5 : 0))}
                        height={Math.max(
                          0,
                          (n / maximum) * 160 - (s.other ? 0.5 : 0),
                        )}
                        fill={s.other ? fills[3] : fills[k]}
                        stroke={s.other ? fills[1] : "none"}
                        strokeWidth="0.5"
                      >
                        <title>{`${day} · ${s.label}: ${n.toLocaleString()}`}</title>
                      </rect>
                    );
                  })}
                </g>
              );
            })}
        {ticks.map((i) => (
          <text
            key={i}
            x={left + (i + 0.5) * slot}
            y="210"
            textAnchor={
              i === 0 ? "start" : i === days.length - 1 ? "end" : "middle"
            }
          >
            {shortDay(days[i])}
          </text>
        ))}
      </svg>
      {!values.some((n) => n > 0) && (
        <p className="small muted">No recorded activity in this range.</p>
      )}
      <div className="analytics-legend">
        {series.map((s, i) => (
          <span key={s.label}>
            <i style={swatch(s, i)} aria-hidden="true" />
            {s.label}
          </span>
        ))}
      </div>
      {note && <p className="small muted">{note}</p>}
      <details className="analytics-daily-data">
        <summary className="tlink">View daily data</summary>
        <div className="analytics-table">
          <table>
            <thead>
              <tr>
                <th>Date (UTC)</th>
                {series.map((s) => (
                  <th key={s.label}>{s.label}</th>
                ))}
              </tr>
            </thead>
            <tbody>
              {days.map((day, i) => (
                <tr key={day}>
                  <th scope="row">{day}</th>
                  {series.map((s) => (
                    <td key={s.label}>
                      {money
                        ? `$${s.values[i].toFixed(4)}`
                        : s.values[i].toLocaleString()}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </details>
    </section>
  );
}
