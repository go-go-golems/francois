import React from "react";
import test from "node:test";
import assert from "node:assert/strict";
import { renderToStaticMarkup } from "react-dom/server";
import { RecurringSupport } from "../ui/catalog/recurring-support";
import type { Subscription } from "../ui/catalog/api";

const subscription: Subscription = {
  id: "sub_example",
  priceId: "price_example",
  supportAmountCents: 4200,
  status: "active",
  accessUntil: "2030-11-01T12:00:00Z",
  cancelAtPeriodEnd: false,
  refreshedAt: "2030-10-01T12:00:00Z",
};
test("year funding preserves an existing monthly renewal in billing copy", () => {
  const html = renderToStaticMarkup(
    <RecurringSupport
      subscriptions={[subscription]}
      now={Date.parse("2030-10-01T00:00:00Z")}
    />,
  );
  assert.match(html, /does not cancel/);
  assert.match(html, /42 USD a month/);
  assert.match(html, /Next payment on/);
  const ending = renderToStaticMarkup(
    <RecurringSupport
      subscriptions={[{ ...subscription, cancelAtPeriodEnd: true }]}
      now={Date.parse("2030-10-01T00:00:00Z")}
    />,
  );
  assert.match(ending, /Scheduled to stop/);
  assert.doesNotMatch(ending, /Next payment/);
  assert.equal(
    renderToStaticMarkup(<RecurringSupport subscriptions={[]} />),
    "",
  );
});
