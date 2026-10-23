import React from "react";
import type { Subscription } from "./api";

// Year funding is a separate payment; it never silently cancels recurring support.
export function RecurringSupport({
  subscriptions,
  now = Date.now(),
}: {
  subscriptions: Subscription[];
  now?: number;
}) {
  const active = subscriptions.filter(
    (s) =>
      s.status === "active" && s.accessUntil && Date.parse(s.accessUntil) > now,
  );
  if (!active.length) return null;
  return (
    <section aria-label="Existing recurring support">
      <p>Funding a year does not cancel your existing subscription.</p>
      {active.map((s) => (
        <p key={s.id}>
          {s.supportAmountCents > 0
            ? `${s.supportTier === "enterprise" ? "Enterprise support, " : ""}${s.supportAmountCents / 100} USD a month. `
            : "Your existing recurring subscription: "}
          {s.cancelAtPeriodEnd
            ? "Scheduled to stop on "
            : "Still active. Next payment on "}
          {new Date(s.accessUntil!).toLocaleDateString("en-GB", {
            day: "numeric",
            month: "long",
            year: "numeric",
          })}
          .
        </p>
      ))}
    </section>
  );
}
