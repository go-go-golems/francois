import test from "node:test";
import assert from "node:assert/strict";
import {
  ENTERPRISE_DEFAULT,
  isEnterpriseAmount,
  shift,
} from "../ui/catalog/enterprise-amount";

test("enterprise amounts are powers of two from 8 to 4096", () => {
  for (const usd of [8, 16, 32, 64, 128, 256, 512, 1024, 2048, 4096])
    assert.equal(isEnterpriseAmount(usd), true, String(usd));
  for (const usd of [0, 4, 7, 12, 42, 300, 8192, 256.5, NaN])
    assert.equal(isEnterpriseAmount(usd), false, String(usd));
});

test("<<1 halves down to 8 and >>1 doubles up to 4096", () => {
  const down = [ENTERPRISE_DEFAULT];
  while (shift(down.at(-1)!, "half") !== down.at(-1))
    down.push(shift(down.at(-1)!, "half"));
  assert.deepEqual(down, [256, 128, 64, 32, 16, 8]);
  const up = [ENTERPRISE_DEFAULT];
  while (shift(up.at(-1)!, "double") !== up.at(-1))
    up.push(shift(up.at(-1)!, "double"));
  assert.deepEqual(up, [256, 512, 1024, 2048, 4096]);
});
