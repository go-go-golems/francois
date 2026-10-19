import test from "node:test";
import assert from "node:assert/strict";
import { OutputBudget } from "../worker/src/output-budget";

test("all provider rounds share the admitted output budget", () => {
  const budget = new OutputBudget(100);
  assert.equal(budget.next(8192), 100);
  budget.consume(60);
  assert.equal(budget.next(8192), 40);
  budget.consume(40);
  assert.throws(() => budget.next(8192), /exhausted/);
});
test("unknown usage and provider budget violations fail closed", () => {
  for (const output of [undefined, -1, NaN, 11, 1.5]) {
    const budget = new OutputBudget(10);
    budget.next(8192);
    assert.throws(() => budget.consume(output), /unknown or invalid/);
  }
  assert.throws(() => new OutputBudget(0), /Invalid/);
});
