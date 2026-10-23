import test from "node:test";
import assert from "node:assert/strict";
import {
  autoStart,
  decide,
  inNewTab,
  type Plan,
} from "../ui/catalog/launch-decision";

const now = Date.parse("2026-10-23T12:00:00Z");
const future = "2026-11-12T00:00:00Z";
const past = "2026-10-01T00:00:00Z";
const plan = (source: string, paid: boolean): Plan => ({
  key: paid ? "subscriber" : "free",
  name: "",
  source,
  paid,
  instances: 1,
});

test("decide covers each visitor, demo and early access", () => {
  const cases: [string, Plan | undefined, object, string][] = [
    ["guest, no demo", undefined, {}, "signin"],
    ["guest, demo", undefined, { demo: "true" }, "start demo"],
    [
      "demo guest session",
      plan("guest", false),
      { demo: "true" },
      "start demo",
    ],
    ["free account", plan("free", false), {}, "start"],
    [
      "free, early access",
      plan("free", false),
      { earlyAccessUntil: future, demo: "true" },
      "subscribe",
    ],
    [
      "guest, early access",
      undefined,
      { earlyAccessUntil: future },
      "subscribe",
    ],
    [
      "free, early access over",
      plan("free", false),
      { earlyAccessUntil: past },
      "start",
    ],
    [
      "subscriber, early access",
      plan("subscription", true),
      { earlyAccessUntil: future },
      "start",
    ],
    [
      "staff, early access",
      plan("staff", true),
      { earlyAccessUntil: future },
      "start",
    ],
    ["disabled", plan("disabled", false), {}, "signin"],
  ];
  for (const [name, p, data, want] of cases) {
    const d = decide(p, data, now);
    const got = d.kind === "start" ? (d.demo ? "start demo" : "start") : d.kind;
    assert.equal(got, want, name);
  }
});

test("only inline artifacts start by themselves, and never as a demo", () => {
  const start = { kind: "start", demo: false } as const;
  const demo = { kind: "start", demo: true } as const;
  assert.equal(autoStart({ launch: "inline" }, start), true);
  assert.equal(autoStart({ launch: "inline" }, demo), false);
  assert.equal(autoStart({ launch: "tab" }, start), false);
  assert.equal(autoStart({ launch: "inline" }, { kind: "signin" }), false);
  assert.equal(inNewTab({ launch: "tab" }), true);
  assert.equal(inNewTab({}), true);
  assert.equal(inNewTab({ launch: "inline" }), false);
});
