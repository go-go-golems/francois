// Isolated visual fixture: serves the real built portal with synthetic API data.
// Run from the repository root. Never connects to the catalog database/providers.
import { createServer } from "node:http";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
const root = resolve(process.argv[2] ?? ".");
const assets = new Map([
  [
    "/static/catalog.js",
    ["pkg/catalog/web/static/catalog.js", "text/javascript"],
  ],
  ["/static/catalog.css", ["pkg/catalog/web/static/catalog.css", "text/css"]],
  ["/static/icon.svg", ["pkg/catalog/web/static/icon.svg", "image/svg+xml"]],
]);
const usage = (key, i = 1) => ({
  key,
  calls: i * 20,
  failed: i,
  credits: i * 30,
  inputTokens: i * 42000,
  outputTokens: i * 6000,
  cacheReadTokens: i * 10000,
  cacheWriteTokens: i * 400,
  cost: i * 0.45,
});
createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://localhost");
    if (url.pathname === "/fonts/fonts.css") {
      res.writeHead(200, { "Content-Type": "text/css" });
      res.end();
      return;
    }
    if (url.pathname === "/api/session") {
      res.setHeader("Content-Type", "application/json");
      res.end(
        JSON.stringify({
          authenticated: true,
          brand: "slop boutique",
          user: { id: "fixture-admin", name: "Preview admin", role: "admin" },
          typeCredit: false,
        }),
      );
      return;
    }
    if (url.pathname === "/api/admin/analytics") {
      const from = url.searchParams.get("from"),
        to = url.searchParams.get("to"),
        site = url.searchParams.get("site") ?? "",
        path = url.searchParams.get("path") ?? "",
        referrer = url.searchParams.get("referrer");
      const days = [];
      for (
        let date = new Date(from + "T00:00:00Z");
        date < new Date(to + "T00:00:00Z");
        date.setUTCDate(date.getUTCDate() + 1)
      )
        days.push(date.toISOString().slice(0, 10));
      const events = days
        .flatMap((day, i) =>
          ["/apps", "/apps/polis", "/", "/journal", "/pricing"].map(
            (page, j) => ({
              day,
              path: page,
              referrer: [
                "",
                "search.example",
                "github.com",
                "news.example",
                "",
              ][j],
              views: (i + 1) * (6 - j),
            }),
          ),
        )
        .filter(
          (event) =>
            (!path || event.path === path) &&
            (referrer === null || event.referrer === referrer),
        );
      const grouped = (key) => {
        const groups = new Map();
        for (const event of events)
          groups.set(event[key], (groups.get(event[key]) ?? 0) + event.views);
        return [...groups]
          .map(([key, views]) => ({
            key,
            views,
            visitors: Math.ceil(views / 3),
          }))
          .sort((a, b) => b.views - a.views || a.key.localeCompare(b.key));
      };
      const groups = {
        daily: grouped("day"),
        pages: grouped("path"),
        referrers: grouped("referrer"),
        sources: [],
        mediums: [],
        campaigns: [],
        devices: [],
        browsers: [],
        users: [],
      };
      const series = {};
      for (const [name, key] of [
        ["pages", "path"],
        ["referrers", "referrer"],
      ]) {
        const top = grouped(key)
          .slice(0, 3)
          .map((row) => row.key);
        const buckets = new Map();
        for (const event of events) {
          const other = !top.includes(event[key]),
            value = other ? "" : event[key],
            id = JSON.stringify([event.day, value, other]);
          const point = buckets.get(id) ?? {
            day: event.day,
            key: value,
            other,
            views: 0,
          };
          point.views += event.views;
          buckets.set(id, point);
        }
        series[name] = [...buckets.values()];
      }
      const views = events.reduce((sum, event) => sum + event.views, 0);
      res.setHeader("Content-Type", "application/json");
      res.end(
        JSON.stringify({
          from: from + "T00:00:00Z",
          to: to + "T00:00:00Z",
          site,
          path,
          referrer,
          traffic: {
            views,
            visitors: Math.ceil(views / 3),
            sessions: Math.ceil(views / 2),
            users: Math.min(24, Math.ceil(views / 3)),
            bounces: Math.min(42, Math.ceil(views / 2)),
          },
          groups,
          series,
          models: {
            daily: days.map((day, i) => usage(day, i + 1)),
            models: [
              usage(
                "lunaroute/glm-5.3-flash",
                Math.floor(((days.length * (days.length + 1)) / 2) * 0.6),
              ),
              usage(
                "lunaroute/deepseek-4.1-flash",
                (days.length * (days.length + 1)) / 2 - Math.floor(((days.length * (days.length + 1)) / 2) * 0.6),
              ),
            ],
            users: [
              usage("fixture-user", (days.length * (days.length + 1)) / 2),
            ],
            instances: [
              usage("fixture-instance", (days.length * (days.length + 1)) / 2),
            ],
            artifacts: [
              {
                ...usage(
                  "polis",
                  Math.floor(((days.length * (days.length + 1)) / 2) * 0.7),
                ),
                label: "Polis IDE",
              },
              {
                ...usage(
                  "tessera",
                  Math.floor(((days.length * (days.length + 1)) / 2) * 0.2),
                ),
                label: "Tessera",
              },
              usage("", (days.length * (days.length + 1)) / 2 - Math.floor(((days.length * (days.length + 1)) / 2) * 0.7) - Math.floor(((days.length * (days.length + 1)) / 2) * 0.2)),
            ],
          },
        }),
      );
      return;
    }
    const asset = assets.get(url.pathname);
    const [path, type] = asset ?? [
      "pkg/catalog/web/static/index.html",
      "text/html",
    ];
    res.writeHead(200, { "Content-Type": type });
    res.end(await readFile(resolve(root, path)));
  } catch (error) {
    res.writeHead(500);
    res.end(String(error));
  }
}).listen(18908, "127.0.0.1", () =>
  console.log(
    "Analytics preview at http://127.0.0.1:18908/admin/analytics (synthetic data)",
  ),
);
