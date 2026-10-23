// launch-decision.ts decides, from the session's plan and an artifact's
// data attributes, what a visitor may do with it on the storefront. It is
// pure so the rules can be tested without a page.

export type Plan = {
  key: string;
  name: string;
  source: string;
  paid: boolean;
  instances: number;
};

// LaunchData mirrors the data-* attributes on the runner and on list rows.
export type LaunchData = {
  demo?: string; // "true" when a visitor without a plan may start a demo
  earlyAccessUntil?: string; // RFC3339
  launch?: string; // "inline" | "tab"
};

export type Decision =
  | { kind: "subscribe" } // early access and the visitor does not pay
  | { kind: "signin" } // no plan that can run it and no demo
  | { kind: "start"; demo: boolean };

export function earlyAccess(d: LaunchData, now = Date.now()): boolean {
  return !!d.earlyAccessUntil && Date.parse(d.earlyAccessUntil) > now;
}

// canRun mirrors Entitlement.CanRun on the server plus the plan's instance
// allowance; the server decides again when the instance is opened.
export function canRun(plan: Plan | undefined, early: boolean): boolean {
  if (!plan) return false;
  if (plan.source === "staff") return true;
  return (
    plan.source !== "guest" &&
    plan.source !== "disabled" &&
    (!early || plan.paid) &&
    plan.instances > 0
  );
}

export function decide(
  plan: Plan | undefined,
  d: LaunchData,
  now = Date.now(),
): Decision {
  const early = earlyAccess(d, now);
  if (canRun(plan, early)) return { kind: "start", demo: false };
  if (early && !plan?.paid) return { kind: "subscribe" };
  if (d.demo === "true") return { kind: "start", demo: true };
  return { kind: "signin" };
}

// autoStart: inline artifacts start on page load for visitors who may run
// them. Demos never start by themselves; each one spends the demo budget.
export function autoStart(d: LaunchData, decision: Decision): boolean {
  return d.launch === "inline" && decision.kind === "start" && !decision.demo;
}

// inNewTab: tab artifacts open full screen in a new tab.
export function inNewTab(d: LaunchData): boolean {
  return d.launch !== "inline";
}
