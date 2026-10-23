// enterprise-amount.ts holds the enterprise support amount rule: a power of
// two from 8 to 4096 USD a month, halved with >>1 and doubled with <<1. The
// server checks the same rule (billing.EnterpriseAmount).

export const ENTERPRISE_MIN = 8;
export const ENTERPRISE_MAX = 4096;
export const ENTERPRISE_DEFAULT = 256;

export function isEnterpriseAmount(usd: number): boolean {
  return (
    Number.isInteger(usd) &&
    usd >= ENTERPRISE_MIN &&
    usd <= ENTERPRISE_MAX &&
    (usd & (usd - 1)) === 0
  );
}

// shift moves the amount one step: "right" (>>1) halves, "left" (<<1)
// doubles. At either end the amount stays where it is.
export function shift(usd: number, direction: "left" | "right"): number {
  const next = direction === "left" ? usd << 1 : usd >> 1;
  return isEnterpriseAmount(next) ? next : usd;
}
